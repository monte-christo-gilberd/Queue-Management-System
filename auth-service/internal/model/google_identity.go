package model

// GoogleIdentity: identitas dari ID token Google yang sudah diperiksa (tanda tangan, issuer, audience, masa berlaku,
// dan email_verified). Hanya ini yang boleh dipercaya; tidak ada data dari klien.
type GoogleIdentity struct {
	// Subject: id akun Google yang stabil (klaim `sub`). Dipakai untuk mencocokkan akun, bukan email (FR 4.17).
	Subject string
	// Email: email akun Google, sudah diverifikasi Google.
	Email string
	// Name: nama tampilan di akun Google; boleh kosong.
	Name string
}
