package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// TicketHandler adalah handler untuk operasi tiket dan antrian yang dipakai petugas maupun pelanggan.
type TicketHandler interface {
	// Join menambahkan pelanggan ke antrian lewat token join.
	Join(c *gin.Context)
	// Preview menampilkan info antrian sebelum pelanggan benar-benar bergabung.
	Preview(c *gin.Context)
	// Next memanggil tiket berikutnya untuk dilayani.
	Next(c *gin.Context)
	// Skip melewati tiket yang sedang dilayani.
	Skip(c *gin.Context)
	// Cancel membatalkan tiket.
	Cancel(c *gin.Context)
	// Remove menghapus satu tiket dari antrian.
	Remove(c *gin.Context)
	// Insert menyisipkan tiket ke posisi tertentu di antrian.
	Insert(c *gin.Context)
	// Recall memanggil ulang tiket yang sedang dilayani.
	Recall(c *gin.Context)
	// List menampilkan daftar tiket di antrian.
	List(c *gin.Context)
	// Status menampilkan status antrian.
	Status(c *gin.Context)
	// MyTickets menampilkan tiket milik user yang sedang login.
	MyTickets(c *gin.Context)
}

// UserHandler adalah handler untuk akun user: profil, preferensi, dan foto profil. Daftar dan login ada di auth-service.
type UserHandler interface {
	// Me menampilkan profil user yang sedang login.
	Me(c *gin.Context)
	// UpdateMe mengubah profil user yang sedang login.
	UpdateMe(c *gin.Context)
	// UpdatePreferences mengubah preferensi user.
	UpdatePreferences(c *gin.Context)
	// UploadAvatar mengunggah foto profil.
	UploadAvatar(c *gin.Context)
	// GetAvatar mengambil foto profil.
	GetAvatar(c *gin.Context)
	// DeleteAvatar menghapus foto profil.
	DeleteAvatar(c *gin.Context)
}

// DisplayHandler adalah handler untuk layar publik antrian: konfigurasi oleh owner dan tampilan tanpa login.
type DisplayHandler interface {
	// Get menampilkan konfigurasi layar publik antrian.
	Get(c *gin.Context)
	// Update mengubah konfigurasi layar publik.
	Update(c *gin.Context)
	// Rotate membuat token baru untuk layar publik, sehingga tautan lama tidak berlaku lagi.
	Rotate(c *gin.Context)
	// Disable mematikan layar publik.
	Disable(c *gin.Context)
	// Snapshot mengambil data layar publik memakai token di path.
	Snapshot(c *gin.Context)
	// Events membuka koneksi SSE untuk layar publik memakai token di path.
	Events(c *gin.Context)
}

// QueueHandler adalah handler untuk antrian: data antrian, kode QR, dan pengaturan skip.
type QueueHandler interface {
	// Create membuat antrian baru.
	Create(c *gin.Context)
	// Get menampilkan detail satu antrian.
	Get(c *gin.Context)
	// List menampilkan daftar antrian milik user.
	List(c *gin.Context)
	// Update mengubah data antrian.
	Update(c *gin.Context)
	// Close menutup antrian.
	Close(c *gin.Context)
	// GetQR menampilkan kode QR antrian.
	GetQR(c *gin.Context)
	// SetQRType mengubah jenis QR antrian.
	SetQRType(c *gin.Context)
	// ReplaceQR mengganti kode QR statis dengan yang baru.
	ReplaceQR(c *gin.Context)
	// GetSkipSettings menampilkan pengaturan skip antrian.
	GetSkipSettings(c *gin.Context)
	// UpdateSkipSettings mengubah pengaturan skip antrian.
	UpdateSkipSettings(c *gin.Context)
}

// PushHandler adalah handler untuk notifikasi Web Push.
type PushHandler interface {
	// PublicKey memberikan kunci publik VAPID untuk browser saat berlangganan.
	PublicKey(c *gin.Context)
	// Subscribe menyimpan langganan push dari browser.
	Subscribe(c *gin.Context)
	// Unsubscribe menghapus langganan push.
	Unsubscribe(c *gin.Context)
}

// SubscriptionHandler adalah handler untuk penambahan masa langganan owner (30 hari, tanpa pembayaran).
type SubscriptionHandler interface {
	// Extend menambah 30 hari ke langganan user yang login.
	Extend(c *gin.Context)
}

// SSEHandler adalah handler untuk koneksi Server-Sent Events antrian.
type SSEHandler interface {
	// ServeSSE membuka koneksi SSE untuk antrian di path.
	ServeSSE(c *gin.Context)
}

// CounterHandler adalah handler untuk CRUD counter (loket) antrian.
type CounterHandler interface {
	// List menampilkan daftar counter antrian.
	List(c *gin.Context)
	// Create membuat counter baru.
	Create(c *gin.Context)
	// Rename mengganti nama counter.
	Rename(c *gin.Context)
	// Delete menghapus counter.
	Delete(c *gin.Context)
}

// NoteHandler adalah handler untuk catatan per nomor tiket.
type NoteHandler interface {
	// Get menampilkan catatan tiket.
	Get(c *gin.Context)
	// Save menyimpan catatan tiket.
	Save(c *gin.Context)
}

// HistoryHandler adalah handler untuk riwayat antrian yang sudah ditutup.
type HistoryHandler interface {
	// ListQueues menampilkan daftar antrian yang sudah ditutup milik owner.
	ListQueues(c *gin.Context)
	// GetQueue menampilkan riwayat satu antrian.
	GetQueue(c *gin.Context)
	// Export mengunduh riwayat antrian sebagai CSV atau XLSX.
	Export(c *gin.Context)
	// ListMine menampilkan riwayat tiket milik user yang sedang login.
	ListMine(c *gin.Context)
}

// AppRouter menyimpan semua handler dan middleware yang akan dipasang ke rute. Dibuat lewat NewAppRouter.
// Handler yang berawalan opsional bernilai nil bila tidak dipasang, dan rutenya juga tidak didaftarkan.
type AppRouter struct {
	// counterHandler menangani CRUD counter (opsional).
	counterHandler CounterHandler
	// historyHandler menangani riwayat antrian (opsional).
	historyHandler HistoryHandler
	// noteHandler menangani catatan tiket (opsional).
	noteHandler NoteHandler
	// displayHandler menangani layar publik (opsional).
	displayHandler DisplayHandler
	// pushHandler menangani Web Push (opsional).
	pushHandler PushHandler
	// subscriptionHandler menangani penambahan masa langganan owner (opsional).
	subscriptionHandler SubscriptionHandler
	// ticketHandler menangani tiket dan join antrian.
	ticketHandler TicketHandler
	// userHandler menangani akun user.
	userHandler UserHandler
	// queueHandler menangani antrian.
	queueHandler QueueHandler
	// sseHandler menangani koneksi SSE antrian.
	sseHandler SSEHandler
	// authMiddleware memeriksa login sebelum rute yang butuh login diproses.
	authMiddleware gin.HandlerFunc
	// allowedOrigins adalah daftar origin browser yang boleh memanggil API (dipakai CORS).
	allowedOrigins []string
	// logger mencatat log akses dan panic (opsional; nil berarti slog.Default()).
	logger *slog.Logger
}
