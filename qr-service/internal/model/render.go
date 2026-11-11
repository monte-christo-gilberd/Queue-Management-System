package model

// RenderOptions: parameter render QR. Nilai kosong atau nol memakai bawaan.
type RenderOptions struct {
	// Data: isi yang dikodekan ke QR.
	Data string
	// Size: lebar gambar dalam piksel. Dibatasi ke rentang 128 sampai 2048.
	Size int
	// RecoveryLevel: tingkat koreksi error: L, M, Q, atau H. Kosong = M.
	RecoveryLevel string
	// ForegroundColor: warna depan #rrggbb. Kosong = warna bawaan library.
	ForegroundColor string
	// BackgroundColor: warna latar #rrggbb. Kosong = warna bawaan library.
	BackgroundColor string
}
