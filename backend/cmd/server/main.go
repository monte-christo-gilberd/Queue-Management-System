// Command server adalah API utama Queue Management System: sistem antrian untuk layanan tatap muka.
//
// Owner membuat antrian, memanggil nomor berikutnya, dan mengelola counter serta layar publik. Customer bergabung
// lewat tautan atau QR, lalu melihat posisi dan nomor yang sedang dilayani secara real-time (SSE) tanpa refresh.
// Layar publik menampilkan nomor tanpa login lewat token di URL. Riwayat antrian dan tiket dapat dibaca kembali.
//
// Binary ini hanya menyambungkan komponen dan menjalankan HTTP server. Logika ada di internal/service,
// akses data di internal/repository, dan endpoint di internal/handler. Login, registrasi, reset password, dan
// verifikasi email ada di auth-service (backend hanya memverifikasi tokennya secara lokal, tanpa memanggilnya).
// Hanya qr-service yang dipanggil backend lewat HTTP; layanan notifikasi, email, dan pembayaran berjalan terpisah
// dan bekerja lewat database.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"qms/internal/config"
	"qms/internal/database"
	"qms/internal/handler"
	"qms/internal/httpserver"
	"qms/internal/middleware"
	"qms/internal/qrclient"
	"qms/internal/repository"
	"qms/internal/router"
	"qms/internal/service"
	"qms/internal/sse"
	noHTMLValidator "qms/internal/validator"

	"syscall"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// main hanya memanggil run dan mencatat error-nya. Pekerjaan sebenarnya ada di run agar defer di dalamnya
// (termasuk db.Close) sudah berjalan sebelum log.Fatal mengakhiri proses.
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run merangkai semua komponen dari bawah ke atas: config → database → repository → service → handler → router,
// lalu menjalankan server sampai proses menerima SIGINT atau SIGTERM.
//
// Service dirangkai dengan pola With* (memasang dependensi pilihan setelah konstruktor). Service yang belum dipasang
// guard-nya (pemeriksa hak akses) menolak akses, bukan membukanya. Push notification hanya dipasang ke router
// bila config-nya diisi.
//
// Berhenti dengan rapi: koneksi SSE ditutup, request yang sedang berjalan dituntaskan (paling lama
// SHUTDOWN_TIMEOUT), lalu koneksi database ditutup oleh defer. Error saat start (config, database) masih langsung
// log.Fatal karena belum ada resource yang perlu ditutup. Kembali dengan error bila server gagal start atau
// shutdown tidak selesai tepat waktu.
func run() error {
	// membaca variabel env sebagai config
	cfg, err := config.Load()

	// jika ada error saat load config
	if err != nil {

		// keluar dan print di log
		log.Fatal(err)
	}

	// membuka koneksi ke database dan cek koneksi (Ping)
	db, err := database.Connect(
		database.Config{
			// menggunakan env DatabaseURL sebagai DSN database
			DSN: cfg.DatabaseURL,
		},
	)

	// jika ada error saat connect database
	if err != nil {
		// keluar dan print di log
		log.Fatal(err)
	}

	// menutup koneksi database saat run selesai, yaitu setelah server berhenti dan request terakhir dituntaskan
	defer db.Close()

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
		log.Println("verifikasi email mati: akun baru tidak perlu verifikasi")
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

	// pemeriksa token lokal: RS256 dengan kunci publik, plus HS256 lama hanya selama masa transisi (JWT_LEGACY_HS256_SECRET)
	tokenVerifier := middleware.
		NewTokenVerifier(cfg.JWTPublicKey, cfg.JWTLegacySecret)

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

	// membuat engine gin (router HTTP dari framework Gin) dan mendaftarkan semua rute dari appRouter
	engine := router.SetupRouter(appRouter)

	// daftarkan validator kustom no_html_tags ke validator bawaan gin
	// RegisterValidation = mendaftarkan aturan validasi; no_html_tags menolak input berisi tag HTML
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation(
			"no_html_tags",
			noHTMLValidator.NoHTMLTags,
		)
	}

	// Tanpa daftar, header X-Forwarded-For dari klien tidak dipercaya (IP klien = alamat koneksi).
	//
	// memasang trusted proxies dari config ke engine
	// trusted proxies = proxy yang header X-Forwarded-For-nya dipercaya untuk membaca IP klien asli
	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {

		// jika ada error saat pemasangan, keluar dan print log
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}

	// membuat string port dari config
	port := fmt.Sprintf(":%s", cfg.Port)

	// ctx dibatalkan saat proses menerima SIGINT (Ctrl+C) atau SIGTERM (docker stop)
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// menjalankan server HTTP dengan timeout dasar; hub.Shutdown menutup koneksi SSE saat server berhenti,
	// agar graceful shutdown tidak menunggu koneksi yang berumur panjang
	srv := httpserver.New(port, engine)
	log.Printf("server listen di %s", port)

	err = httpserver.Run(ctx, srv, cfg.ShutdownTimeout, hub.Shutdown)
	if err == nil {
		log.Println("server berhenti dengan rapi")
	}
	return err
}
