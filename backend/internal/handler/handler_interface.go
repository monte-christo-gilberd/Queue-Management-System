// Catatan: interface di file ini adalah sisa lama dan tidak dipakai di mana pun; router memakai interface di
// router/router_interface.go.
package handler

import "github.com/gin-gonic/gin"

// QueueHandlerInterface adalah kontrak handler antrian yang dipakai oleh router.
type QueueHandlerInterface interface {
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
	// Info menampilkan informasi publik antrian.
	Info(c *gin.Context)
	// GetQR menampilkan kode QR antrian.
	GetQR(c *gin.Context)
	// SetQRType mengubah jenis QR antrian.
	SetQRType(c *gin.Context)
	// ReplaceQR mengganti kode QR statis dengan yang baru.
	ReplaceQR(c *gin.Context)
}

// TicketHandlerInterface adalah kontrak handler tiket yang dipakai oleh router.
type TicketHandlerInterface interface {
	// Join menambahkan pelanggan ke antrian lewat token join.
	Join(c *gin.Context)
	// Preview menampilkan info antrian sebelum pelanggan bergabung.
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
	// List menampilkan daftar tiket di antrian.
	List(c *gin.Context)
	// Status menampilkan status antrian.
	Status(c *gin.Context)
	// MyTickets menampilkan tiket milik user yang sedang login.
	MyTickets(c *gin.Context)
}

// UserHandlerInterface adalah kontrak handler user yang dipakai oleh router.
type UserHandlerInterface interface {
	// Register mendaftarkan akun baru.
	Register(c *gin.Context)
	// Login memverifikasi kredensial dan menerbitkan token login.
	Login(c *gin.Context)
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

// SSEHandlerInterface adalah kontrak handler SSE yang dipakai oleh router.
type SSEHandlerInterface interface {
	// ServeSSE membuka koneksi SSE untuk antrian di path.
	ServeSSE(c *gin.Context)
}
