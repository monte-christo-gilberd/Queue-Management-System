// Package app merangkai komponen backend: Wire memasang dependensi (repository → service → handler → router),
// dan NewEngine menyiapkan engine HTTP (Gin) dari router itu. Package main hanya memanggil keduanya lalu menjalankan server.
package app

import (
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"

	"qms/internal/config"
	"qms/internal/database"
	"qms/internal/handler"
	"qms/internal/middleware"
	"qms/internal/qrclient"
	"qms/internal/repository"
	"qms/internal/router"
	"qms/internal/service"
	"qms/internal/sse"
)

// App adalah hasil Wire: router yang sudah lengkap dan hub SSE yang harus dihentikan saat server berhenti.
type App struct {
	// Router: semua handler sudah terpasang; berikan ke NewEngine.
	Router *router.AppRouter
	// Hub: pengelola koneksi SSE. Hub.Run sudah berjalan di goroutine; panggil Hub.Shutdown saat server berhenti.
	Hub *sse.Hub
}

// Wire merangkai semua komponen dari bawah ke atas: hub SSE → repository → service → handler → router.
//
// Service dirangkai dengan pola With* (memasang dependensi pilihan setelah konstruktor). Service yang belum dipasang
// guard-nya (pemeriksa hak akses) menolak akses, bukan membukanya. Push notification hanya dipasang ke router
// bila config-nya diisi. Koneksi database dimiliki pemanggil, yang juga menutupnya.
func Wire(cfg *config.Config, db *sqlx.DB) *App {
	// membuat hub SSE (pengelola koneksi SSE per antrian dan penyebar event
	// SSE = koneksi satu arah dari server ke browser)
	hub := sse.NewHub()

	// cache snapshot layar publik, dibatalkan oleh hub SEBELUM tiap event disebar (lihat Hub.Notify)
	// snapshot = salinan keadaan antrian; cache = penyimpanan sementara
	displayCache := service.
		NewDisplayCache(service.DisplayCacheTTL)

	// memasang hook (fungsi yang dipanggil otomatis pada titik tertentu) di hub: Invalidate membuang cache snapshot
	// layar publik setiap ada event yang disebar, sebelum event itu
	// dikirim ke klien, agar layar tidak menampilkan data lama
	// invalidate = membuang agar data dimuat ulang
	hub.SetBeforeNotify(displayCache.Invalidate)

	// menjalankan loop hub di goroutine: memproses gabung, keluar, dan siaran event
	// goroutine = fungsi yang berjalan bersamaan dengan program utama
	go hub.Run()

	// guard verifikasi email: hanya dibuat bila EMAIL_VERIFICATION_REQUIRED (bawaan true). Backend hanya MEMBACA status
	// verifikasi dari database; pengiriman email dan penandaannya dilakukan auth-service. nil berarti guard mati.
	// Dibuat sebelum service lain karena queue dan ticket service memakainya.
	var verification *service.EmailVerificationGuard
	if cfg.EmailVerificationRequired {
		verification = service.NewEmailVerificationGuard(repository.NewVerificationRepository(db))
	} else {
		slog.Info("email verification disabled: new accounts do not need to verify")
	}

	// inisialisasi transaksi sqlx
	txRunner := database.
		NewSqlxTxRunner(db)

	// inisialisasi layer repository user
	userRepo := repository.
		NewUserRepository(db)

	// inisialisasi layer repository avatar
	avatarRepo := repository.
		NewAvatarRepository(db)

	// inisialisasi layer service user (profil, preferensi, foto). Login dan registrasi ada di auth-service.
	userService := service.
		NewUserService(userRepo).

		// memasang fitur avatar
		WithAvatars(avatarRepo)

	// memasang status verifikasi di profil (false = semua akun dilaporkan terverifikasi)
	userService.WithEmailVerification(verification != nil)

	// inisialisasi layer handler user
	userHandler := handler.
		NewUserHandler(userService)

	// inisialisasi layer repository queue
	queueRepo := repository.
		NewQueueRepository(db)

	// inisialisasi layer repository langganan owner
	subscriptionChecker := repository.
		NewSubscriptionChecker(db)

	// inisialisasi layer repository tiket
	ticketRepo := repository.
		NewTicketRepository(db)

	// inisialisasi layer service queue
	queueService := service.
		NewQueueService(
			queueRepo,
			subscriptionChecker,
			txRunner,
			hub,
		).

		// memasang ticketRepo agar pemegang tiket (pelanggan yang punya tiket di antrian) boleh membaca antriannya (lihat ViewQueue)
		WithTicketHolderChecker(ticketRepo)

	// membuat antrian butuh email terverifikasi
	if verification != nil {
		queueService.WithVerificationGuard(verification)
	}

	// service QR: memanggil microservice qr-service lewat qrclient
	// microservice = layanan terpisah yang dipanggil lewat HTTP
	qrService := service.
		NewQRService(
			queueRepo,

			// klien HTTP ke qr-service
			qrclient.New(
				cfg.QRServiceURL,
				cfg.QRServiceToken,
				cfg.QRHTTPTimeout,
			),
			cfg.PublicAppURL,
			hub,
		).

		// TTL token join QR dinamis: 120 detik (sama dengan default)
		// TTL = time to live, masa berlaku
		WithDynamicTTL(120 * time.Second)

	// inisialisasi layer handler queue, termasuk endpoint QR
	queueHandler := handler.
		NewQueueHandlerWithQR(
			queueService,
			qrService,
		)

	// inisialisasi layer service ticket
	ticketService := service.
		NewTicketService(ticketRepo, txRunner, hub).

		// memasang queueService sebagai queue guard; tanpa guard, join, next, dan list tiket ditolak
		WithQueueGuard(queueService).

		// memasang pencarian akun untuk fitur sisip; tanpa ini, InsertTicket ditolak
		// fitur sisip = menambahkan pelanggan ke antrian secara manual oleh owner
		WithAccountFinder(userRepo)

	// bergabung ke antrian (nomor baru) butuh email terverifikasi
	if verification != nil {
		ticketService.WithVerificationGuard(verification)
	}

	// inisialisasi layer handler ticket dengan qr-service
	ticketHandler := handler.
		NewTicketHandlerWithQR(
			ticketService,
			qrService,
		)

	// inisialisasi layer handler sse
	sseHandler := handler.
		NewSSEHandler(hub, queueService)

	// Pencabutan sesi: token yang terbit sebelum password diganti ditolak. password_changed_at dibaca langsung dari
	// database (bukan lewat auth-service), jadi pencabutan tetap bekerja saat auth-service mati.
	sessionRepo := repository.
		NewSessionRepository(db)

	// pemeriksa token lokal: hanya RS256, diverifikasi dengan kunci publik
	tokenVerifier := middleware.
		NewTokenVerifier(cfg.JWTPublicKey)

	// inisialisasi router
	appRouter := router.NewAppRouter(
		ticketHandler,
		userHandler,
		queueHandler,

		// middleware autentikasi JWT dengan pencabutan sesi (revocation): menolak token yang terbit sebelum password diganti
		middleware.RequireAuthWithRevocation(
			tokenVerifier,
			sessionRepo,
		),
		sseHandler,
		cfg.AllowedOrigins,
	)

	// inisialisasi layer repository note
	noteRepo := repository.
		NewNoteRepository(db)

	// inisialisasi layer service note
	noteService := service.
		NewNoteService(noteRepo).

		// memasang queue guard: hanya owner antrian yang boleh mengakses
		WithQueueGuard(queueService)

	// inisialisasi layer handler note
	noteHandler := handler.
		NewNoteHandler(noteService)

	// memasang endpoint catatan per nomor tiket ke appRouter
	appRouter.
		WithNotes(noteHandler)

	// inisialisasi layer repository counter
	counterRepo := repository.
		NewCounterRepository(db)

	// inisialisasi layer service counter
	counterService := service.
		NewCounterService(
			counterRepo,
			txRunner,
			hub,
		).

		// memasang queue guard ke service counter
		WithQueueGuard(queueService).

		// memasang pelepasan saat counter dihapus
		// release = melepas nomor yang dipegang counter
		WithTicketReleaser(ticketService)

	// inisialisasi layer handler counter
	counterHandler := handler.
		NewCounterHandler(counterService)

	// memasang endpoint CRUD (create, read, update, delete) counter ke appRouter
	appRouter.
		WithCounters(counterHandler)

	// inisialisasi layer repository display
	displayRepo := repository.
		NewDisplayRepository(db)

	// inisialisasi layer service display
	displayService := service.
		NewDisplayService(displayRepo, counterRepo, displayCache).

		// memasang queue guard ke service display
		WithQueueGuard(queueService).

		// memasang notifikasi ke service display
		WithNotifier(hub)

	// inisialisasi layer handler display
	displayHandler := handler.
		NewDisplayHandler(
			displayService,
			hub,
		)

	// memasang endpoint display ke appRouter: konfigurasi owner (pemilik antrian, /queues/:id/display) dan layar publik (/display/:token)
	appRouter.
		WithDisplay(displayHandler)

	// inisialisasi layer repository langganan: tambah 30 hari tanpa pembayaran (pengganti payment-service)
	subscriptionRepo := repository.
		NewSubscriptionRepository(db)

	// inisialisasi layer service langganan
	subscriptionService := service.
		NewSubscriptionService(subscriptionRepo)

	// subscribe (menjadi owner) butuh email terverifikasi
	if verification != nil {
		subscriptionService.WithVerificationGuard(verification)
	}

	// inisialisasi layer handler langganan
	subscriptionHandler := handler.
		NewSubscriptionHandler(subscriptionService)

	// memasang endpoint langganan ke appRouter: POST /api/v1/owners/subscription/extend
	appRouter.
		WithSubscription(subscriptionHandler)

	// inisialisasi layer repository riwayat
	historyRepo := repository.
		NewHistoryRepository(db)

	// inisialisasi layer service riwayat
	historyService := service.
		NewHistoryService(historyRepo)

	// inisialisasi layer handler riwayat
	historyHandler := handler.
		NewHistoryHandler(historyService)

	// memasang endpoint riwayat antrian (owner) dan riwayat tiket (customer) ke appRouter
	appRouter.
		WithHistory(historyHandler)

	// inisialisasi layer repository push notification
	pushRepo := repository.
		NewPushRepository(db)

	// inisialisasi layer handler push notification (dibuat selalu; yang bersyarat hanya pemasangan ke router)
	pushHandler := handler.NewPushHandler(
		pushRepo,
		cfg.VAPIDPublicKey,
	)

	// jika VAPID_PUBLIC_KEY diisi, fitur push notification dipasang
	// VAPID = pasangan kunci untuk push notification di browser
	if cfg.VAPIDPublicKey != "" {

		// memasang fitur push notification ke appRouter
		appRouter.
			WithPush(pushHandler)
	}

	return &App{
		Router: appRouter,
		Hub:    hub,
	}
}
