package dto

// RenderRequest: body untuk merender QR umum menjadi PNG.
type RenderRequest struct {
	// Data: isi yang dikodekan ke QR. Wajib diisi, maksimal 1024 karakter.
	Data string `json:"data"`
	// Size: lebar gambar dalam piksel. 0 = bawaan 512; nilai di luar 128 sampai 2048 dibatasi ke rentang itu.
	Size int `json:"size"`
	// Level: tingkat koreksi error: L, M, Q, atau H. Kosong = M.
	Level string `json:"level"`
	// FG: warna depan dalam format #rrggbb. Kosong = hitam.
	FG string `json:"fg"`
	// BG: warna latar dalam format #rrggbb. Kosong = putih.
	BG string `json:"bg"`
}
