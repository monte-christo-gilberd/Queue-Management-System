package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/model"
)

// maxUsernameLen: batas panjang username dalam karakter (rune), sama dengan backend.
const maxUsernameLen = 50

// LinkRequiredError dikembalikan LoginWithGoogle bila email sudah terdaftar dengan password. Membawa link token
// untuk dikirim kembali bersama password di LinkGoogle. Cocok dengan apperror.ErrGoogleLinkRequired lewat errors.Is.
type LinkRequiredError struct {
	// Token: link token (JWT RS256, 5 menit, purpose google_link).
	Token string
}

// Error mengembalikan teks sentinel apperror.ErrGoogleLinkRequired.
func (e *LinkRequiredError) Error() string {
	// pesan sama dengan sentinel supaya log dan perbandingan konsisten
	return apperror.ErrGoogleLinkRequired.Error()
}

// Is membuat errors.Is(err, apperror.ErrGoogleLinkRequired) bernilai true.
func (e *LinkRequiredError) Is(
	target error,
) bool {
	// cocok hanya dengan sentinel ErrGoogleLinkRequired
	return target == apperror.ErrGoogleLinkRequired
}

// WithGoogle memasang pemeriksa ID token Google dan menyalakan login Google.
func (s *Service) WithGoogle(v GoogleVerifier) *Service {
	// Simpan pemeriksa; nilai non-nil menyalakan login Google.
	s.google = v
	// Kembalikan Service agar bisa dirangkai berantai.
	return s
}

// GoogleEnabled melaporkan apakah login Google dinyalakan.
func (s *Service) GoogleEnabled() bool {
	// login Google hanya aktif bila pemeriksa ID token sudah dipasang
	return s.google != nil
}

// LoginWithGoogle memeriksa ID token lalu:
//  1. akun yang tertaut ke google_id ini ada: terbitkan token login;
//  2. email sudah dipakai akun lain: JANGAN tautkan otomatis. Kembalikan *LinkRequiredError (link token) supaya user
//     membuktikan password akun itu lebih dulu (pengambilalihan akun lewat email dicegah);
//  3. email belum ada: buat akun baru tanpa password, emailnya terverifikasi (Google sudah menjaminnya).
func (s *Service) LoginWithGoogle(
	ctx context.Context,
	idToken string,
) (string, error) {
	// Login Google belum dipasang: tolak.
	if s.google == nil {
		// Kembalikan error khusus agar handler membalas bahwa fitur tidak tersedia.
		return "", apperror.ErrGoogleUnavailable
	}
	// Periksa ID token ke Google; hanya identitas yang terverifikasi yang dipercaya.
	ident, err := s.google.Verify(ctx, idToken)
	// ID token tidak sah atau pemeriksaan gagal.
	if err != nil {
		// Kembalikan error pemeriksa apa adanya.
		return "", err
	}

	// Dua kali percobaan: pendaftaran serentak untuk akun Google yang sama bisa menabrak UNIQUE di CreateGoogle.
	for attempt := 0; attempt < 2; attempt++ {
		// 1. Sudah tertaut: cocokkan lewat google_id.
		user, err := s.users.FindByGoogleID(ctx, ident.Subject)
		// Akun sudah tertaut ke google_id ini: tidak perlu langkah lain.
		if err == nil {
			// Terbitkan token login untuk akun itu.
			return s.issuer.Issue(user.ID)
		}
		// Galat selain ErrNotFound adalah gangguan database.
		if !errors.Is(err, apperror.ErrNotFound) {
			// Kembalikan error database.
			return "", err
		}

		// 2. Email sudah terdaftar: perlu konfirmasi password.
		existing, err := s.users.FindAccountByEmail(ctx, ident.Email)
		// Tentukan hasil pencarian akun lewat email.
		switch {
		// Email sudah dipakai akun lain.
		case err == nil:
			// Jangan tautkan otomatis; minta konfirmasi password (atau tolak bila tidak bisa dibuktikan).
			return "", s.linkRequired(existing.ID, existing.PasswordHash.Valid, existing.GoogleID.Valid, ident.Subject)
		// Gangguan database saat mencari akun.
		case !errors.Is(err, apperror.ErrNotFound):
			// Kembalikan error database.
			return "", err
		}

		// 3. Email baru: buat akun Google.
		created, err := s.users.CreateGoogle(ctx, ident.Email, usernameFor(ident), ident.Subject)
		// Akun Google baru berhasil dibuat.
		if err == nil {
			// Terbitkan token login untuk akun baru.
			return s.issuer.Issue(created.ID)
		}
		// Duplikat: ada yang membuat akun itu bersamaan; ulangi keputusan dari awal.
		if !errors.Is(err, apperror.ErrDuplicate) {
			// Galat bukan duplikat: kembalikan apa adanya.
			return "", err
		}
	}
	// Dua percobaan sama-sama bertabrakan: kembalikan ErrDuplicate (409).
	return "", apperror.ErrDuplicate
}

// linkRequired membuat galat konfirmasi password untuk akun yang sudah ada. Akun yang sudah tertaut ke akun Google
// lain, atau tidak punya password untuk dibuktikan, tidak bisa ditautkan: ErrDuplicate (409 CONFLICT).
func (s *Service) linkRequired(
	userID int64,
	hasPassword,
	hasGoogle bool,
	googleID string,
) error {
	// Sudah tertaut ke akun Google lain, atau tanpa password untuk dibuktikan: tolak.
	if hasGoogle || !hasPassword {
		// ErrDuplicate dibalas 409 CONFLICT.
		return apperror.ErrDuplicate
	}
	// Terbitkan link token yang mengikat user id dan google id.
	token, err := s.issuer.IssueLink(userID, googleID)
	// Gagal menerbitkan link token.
	if err != nil {
		// Kembalikan error penerbitan.
		return err
	}
	// Kembalikan error khusus yang membawa link token untuk dikirim ke klien.
	return &LinkRequiredError{Token: token}
}

// LinkGoogle menautkan akun Google setelah user membuktikan password akunnya, lalu menerbitkan token login.
// Salah password menghasilkan apperror.ErrNotFound (401); setelah maxLinkAttempts kali salah, link token dibakar.
func (s *Service) LinkGoogle(
	ctx context.Context,
	linkToken,
	password string,
) (string, error) {
	// Periksa tanda tangan, masa berlaku, dan tujuan link token.
	userID, googleID, err := s.issuer.VerifyLink(linkToken)
	// Link token tidak sah atau kedaluwarsa.
	if err != nil {
		// Balas ErrInvalidLinkToken tanpa menjelaskan sebabnya.
		return "", apperror.ErrInvalidLinkToken
	}
	// Kunci penghitung percobaan dibuat dari hash link token.
	key := linkKey(linkToken)
	// Token sudah dibakar atau terlalu sering salah password: tolak untuk mencegah tebak-tebakan password.
	if s.links.exhausted(key, s.now()) {
		// Balas ErrInvalidLinkToken yang sama dengan token tidak sah.
		return "", apperror.ErrInvalidLinkToken
	}

	// Ambil akun yang dituju link token.
	user, err := s.users.FindByID(ctx, userID)
	// Akun sudah tidak ada: link token tidak berguna lagi.
	if errors.Is(err, apperror.ErrNotFound) {
		// Balas ErrInvalidLinkToken.
		return "", apperror.ErrInvalidLinkToken
	}
	// Gangguan database saat mengambil akun.
	if err != nil {
		// Kembalikan error database.
		return "", err
	}
	// Akun tanpa password tidak bisa membuktikan apa pun.
	if !user.PasswordHash.Valid {
		// Balas ErrInvalidLinkToken karena tidak ada password untuk dibuktikan.
		return "", apperror.ErrInvalidLinkToken
	}
	// Password salah: catat kegagalan agar token terbakar setelah terlalu banyak percobaan.
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(password)) != nil {
		// Tambah hitungan salah password untuk link token ini.
		s.links.fail(key, s.now())
		// Balas ErrNotFound (401) seperti login biasa agar tidak membocorkan detail.
		return "", apperror.ErrNotFound
	}

	// Sudah tertaut ke akun Google yang sama: idempoten, cukup terbitkan token.
	switch {
	// Sudah tertaut ke google_id yang sama.
	case user.GoogleID.Valid && user.GoogleID.String == googleID:
		// Terbitkan token login tanpa menautkan ulang.
		return s.issuer.Issue(user.ID)
	// Sudah tertaut ke akun Google yang berbeda: tolak.
	case user.GoogleID.Valid:
		// Kembalikan ErrDuplicate (409).
		return "", apperror.ErrDuplicate
	}
	// Simpan tautan akun Google ke akun ini.
	if err := s.users.LinkGoogle(ctx, user.ID, googleID); err != nil {
		// Gagal menautkan (mis. google_id sudah dipakai akun lain): kembalikan error.
		return "", err
	}
	// Link token sekali pakai: setelah berhasil, tidak bisa dipakai lagi.
	s.links.burn(key, s.now())
	// Terbitkan token login untuk akun yang baru tertaut.
	return s.issuer.Issue(user.ID)
}

// usernameFor memilih username untuk akun Google baru: nama di akun Google, atau bagian depan email bila kosong.
// Karakter '<' dan '>' dibuang (sama dengan aturan no_html_tags), spasi dirapikan, dan panjang dibatasi 50 karakter.
func usernameFor(ident model.GoogleIdentity) string {
	// Pakai nama dari akun Google bila ada setelah dibersihkan.
	if name := cleanName(ident.Name); name != "" {
		// Kembalikan nama itu sebagai username.
		return name
	}
	// Nama kosong: ambil bagian email sebelum tanda '@'.
	if local, _, ok := strings.Cut(ident.Email, "@"); ok {
		// Bersihkan bagian depan email dengan aturan yang sama.
		if name := cleanName(local); name != "" {
			// Pakai bagian depan email sebagai username.
			return name
		}
	}
	// Cadangan terakhir bila nama dan email tidak menghasilkan username.
	return "user"
}

// cleanName membuang '<' '>' dan karakter kontrol, merapikan spasi, dan memotong ke maxUsernameLen karakter.
func cleanName(s string) string {
	// Buang karakter yang tidak boleh ada di username.
	s = strings.Map(func(r rune) rune {
		if r == '<' || r == '>' || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	// Rapikan spasi berlebih menjadi satu spasi.
	s = strings.Join(strings.Fields(s), " ")
	// Username terlalu panjang: potong per karakter, bukan per byte.
	if utf8.RuneCountInString(s) > maxUsernameLen {
		// Potong ke batas maksimum karakter.
		s = string([]rune(s)[:maxUsernameLen])
		// Buang spasi di ujung akibat pemotongan.
		s = strings.TrimSpace(s)
	}
	// Kembalikan nama yang sudah bersih.
	return s
}
