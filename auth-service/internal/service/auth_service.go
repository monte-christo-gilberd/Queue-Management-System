package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"qms/auth-service/internal/apperror"
)

const (
	minPasswordRunes = 8
	// bcrypt hanya memakai 72 byte pertama (dan menolak lebih dari itu).
	maxPasswordBytes = 72
	bcryptCost       = 10
)

// Options: pengaturan Service.
type Options struct {
	// PublicAppURL: alamat dasar aplikasi untuk tautan di email.
	PublicAppURL string
	// VerificationRequired: kirim email verifikasi saat registrasi. Butuh Mailer.
	VerificationRequired bool
}

// Service: registrasi, login, reset password, dan verifikasi email.
type Service struct {
	// users: akses tabel users.
	users UserRepository
	// tokens: akses tabel token sekali pakai (reset password dan verifikasi email).
	tokens TokenRepository
	// issuer: penerbit dan pemeriksa JWT RS256.
	issuer *TokenIssuer
	// mailer: pengirim email lewat mail-service. Nil = email mati.
	mailer Mailer
	// publicURL: alamat dasar aplikasi untuk tautan di email (tanpa "/" di akhir).
	publicURL string
	// verifyOn: true bila email verifikasi dikirim saat registrasi (membutuhkan mailer).
	verifyOn bool
	// now: sumber waktu; bisa diganti di tes.
	now func() time.Time
	// log: pencatat peristiwa, mis. saat batas permintaan per jam tercapai.
	log *slog.Logger
	// google: pemeriksa ID token Google. Nil = login Google mati (rute tidak dipasang).
	google GoogleVerifier
	// links: penghitung salah password per link token (lihat google.go).
	links *linkAttempts
}

// NewService merangkai Service. mailer boleh nil (email mati).
func NewService(
	users UserRepository,
	tokens TokenRepository,
	issuer *TokenIssuer,
	mailer Mailer,
	opts Options,
) *Service {
	// "/" di akhir URL dibuang agar tidak ada "//" di tautan; verifikasi hanya menyala bila ada pengirim email
	return &Service{
		users:     users,
		tokens:    tokens,
		issuer:    issuer,
		mailer:    mailer,
		publicURL: strings.TrimRight(opts.PublicAppURL, "/"),
		verifyOn:  opts.VerificationRequired && mailer != nil,
		now:       time.Now,
		log:       slog.Default(),
		links:     newLinkAttempts(),
	}
}

// ValidatePassword: aturan password untuk registrasi dan reset (8 karakter sampai 72 byte).
func ValidatePassword(password string) error {
	// hitung karakter (rune) dan byte; di luar batas berarti lemah
	if utf8.RuneCountInString(password) < minPasswordRunes || len(password) > maxPasswordBytes {
		// Password terlalu pendek atau terlalu panjang: tolak dengan ErrWeakPassword.
		return apperror.ErrWeakPassword
	}
	// Password memenuhi aturan.
	return nil
}

// Register membuat akun dan mengembalikan token login. Email verifikasi dikirim di latar belakang.
func (s *Service) Register(
	ctx context.Context,
	email,
	username,
	password string,
) (string, error) {
	// periksa kekuatan password lebih dulu; yang lemah ditolak sebelum menyentuh database
	if err := ValidatePassword(password); err != nil {
		// Password lemah: kembalikan ErrWeakPassword.
		return "", err
	}
	// ubah password menjadi hash bcrypt agar tidak disimpan dalam bentuk asli
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	// Gagal membuat hash.
	if err != nil {
		// Kembalikan error pembuatan hash.
		return "", err
	}
	// simpan akun baru; email atau username yang sudah dipakai menghasilkan ErrDuplicate
	user, err := s.users.Create(ctx, email, username, string(hash))
	// Gagal menyimpan akun.
	if err != nil {
		// Kembalikan error apa adanya, termasuk ErrDuplicate bila email atau username sudah ada.
		return "", err
	}
	// terbitkan token login untuk akun yang baru dibuat
	token, err := s.issuer.Issue(user.ID)
	// Gagal menerbitkan token.
	if err != nil {
		// Kembalikan error penerbitan token.
		return "", err
	}
	// kirim email verifikasi tanpa menunggu hasilnya
	s.sendVerificationInBackground(user.ID)
	// Kembalikan token login. Registrasi sudah berhasil walau email verifikasi belum terkirim.
	return token, nil
}

// Login menerima email atau username. Kandidat dicocokkan satu per satu dengan password; username yang sama
// bisa dimiliki lebih dari satu user, jadi yang cocok passwordnya yang masuk.
func (s *Service) Login(
	ctx context.Context,
	identifier,
	password string,
) (string, error) {
	// cari semua akun yang cocok dengan email atau username yang dimasukkan
	users, err := s.users.FindByLogin(ctx, identifier)
	// Gagal membaca database.
	if err != nil {
		// Kembalikan error database; pemanggil membalas 500.
		return "", err
	}
	// cocokkan password setiap kandidat satu per satu
	for _, user := range users {
		// Akun tanpa password (hanya login Google) dilewati.
		if !user.PasswordHash.Valid {
			// Lanjut ke kandidat berikutnya.
			continue
		}
		// hash tidak cocok dengan password: coba kandidat berikutnya
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(password)) != nil {
			// Lanjut ke kandidat berikutnya; password tidak cocok.
			continue
		}
		// password cocok: terbitkan token untuk user ini
		return s.issuer.Issue(user.ID)
	}
	// tidak ada kandidat yang cocok; pemanggil membalas 401 yang sama untuk semua kegagalan
	return "", apperror.ErrNotFound
}

// Authenticate memeriksa token dan pencabutan sesi, lalu mengembalikan id user. apperror.ErrUnauthenticated
// untuk token tidak sah atau sesi dicabut; galat lain (database) dikembalikan apa adanya.
func (s *Service) Authenticate(
	ctx context.Context,
	raw string,
) (int64, error) {
	// periksa tanda tangan RS256 dan masa berlaku
	claims, err := s.issuer.Verify(raw)
	// Token tidak sah atau kedaluwarsa.
	if err != nil {
		// Tolak sebagai tidak terautentikasi tanpa menjelaskan sebabnya.
		return 0, apperror.ErrUnauthenticated
	}
	// klaim `sub` harus berupa string
	sub, ok := claims["sub"].(string)
	// Klaim sub hilang atau bukan string.
	if !ok {
		// Token dianggap tidak sah.
		return 0, apperror.ErrUnauthenticated
	}
	// ubah `sub` menjadi id user berupa angka
	userID, err := strconv.ParseInt(sub, 10, 64)
	// Isi sub bukan angka yang valid.
	if err != nil {
		// Token dianggap tidak sah.
		return 0, apperror.ErrUnauthenticated
	}
	// pencabutan sesi: JSON mengubah angka menjadi float64, jadi iat dibaca sebagai float64
	if iat, ok := claims["iat"].(float64); ok {
		// waktu terakhir password diganti, dibaca dari database
		changedAt, err := s.users.PasswordChangedAt(ctx, userID)
		// user sudah tidak ada: sesinya dianggap tidak sah
		if errors.Is(err, apperror.ErrNotFound) {
			// Sesi dianggap tidak sah karena user sudah dihapus.
			return 0, apperror.ErrUnauthenticated
		}
		// gangguan database: kembalikan apa adanya (pemanggil membalas 500)
		if err != nil {
			// Gangguan database: kembalikan error asli.
			return 0, err
		}
		// Dibandingkan dalam detik: iat berdetik, dan login tepat setelah reset tidak boleh ikut dicabut.
		if changedAt != nil && changedAt.Unix() > int64(iat) {
			// Token terbit sebelum password diganti: sesi lama dicabut.
			return 0, apperror.ErrUnauthenticated
		}
	}
	// Token sah dan sesi belum dicabut: kembalikan id user.
	return userID, nil
}
