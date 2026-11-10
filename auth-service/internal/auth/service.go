package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/model"
)

const (
	// PurposePasswordReset dan PurposeEmailVerify: nilai kolom user_action_tokens.purpose.
	PurposePasswordReset = "password_reset"
	PurposeEmailVerify   = "email_verify"

	resetTokenTTL = 30 * time.Minute
	// Batas per akun per jam, di atas batas per IP di nginx.
	resetMaxPerHour  = 3
	verifyTokenTTL   = 24 * time.Hour
	verifyMaxPerHour = 3

	// verificationSendTimeout membatasi pengiriman email verifikasi di latar belakang saat registrasi.
	verificationSendTimeout = 45 * time.Second

	minPasswordRunes = 8
	// bcrypt hanya memakai 72 byte pertama (dan menolak lebih dari itu).
	maxPasswordBytes = 72
	bcryptCost       = 10
)

// UserStore: akses tabel users. Dipenuhi store.UserStore.
type UserStore interface {
	Create(ctx context.Context, email, username, passwordHash string) (model.User, error)
	FindByLogin(ctx context.Context, identifier string) ([]model.User, error)
	// FindByEmail: id, email, username untuk alur reset; apperror.ErrNotFound bila tidak ada.
	FindByEmail(ctx context.Context, email string) (model.User, error)
	// FindForVerification: id, email, username, email_verified_at; apperror.ErrNotFound bila tidak ada.
	FindForVerification(ctx context.Context, userID int64) (model.User, error)
	// PasswordChangedAt: nil bila belum pernah; apperror.ErrNotFound bila user tidak ada.
	PasswordChangedAt(ctx context.Context, userID int64) (*time.Time, error)
}

// TokenStore: token sekali pakai (user_action_tokens). Dipenuhi store.TokenStore.
type TokenStore interface {
	CountRecentTokens(ctx context.Context, userID int64, purpose string, since time.Time) (int, error)
	// CreateToken menyimpan token baru dan membatalkan token lama (purpose sama) dalam satu transaksi.
	CreateToken(ctx context.Context, userID int64, purpose, tokenHash string, expiresAt time.Time, requestIP string) error
	// ResetPassword memakai token, mengganti password, mengisi password_changed_at; ErrInvalidResetToken bila tidak sah.
	ResetPassword(ctx context.Context, tokenHash, passwordHash string) error
	// VerifyEmail memakai token dan mengisi email_verified_at; ErrInvalidResetToken bila tidak sah.
	VerifyEmail(ctx context.Context, tokenHash string) error
}

// Mailer mengirim email lewat mail-service (mailclient.Client).
type Mailer interface {
	Send(ctx context.Context, to, template string, data map[string]any) error
}

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
	users UserStore
	// tokens: akses tabel token sekali pakai (reset password dan verifikasi email).
	tokens TokenStore
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
}

// NewService merangkai Service. mailer boleh nil (email mati).
func NewService(users UserStore, tokens TokenStore, issuer *TokenIssuer, mailer Mailer, opts Options) *Service {
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
	}
}

// ValidatePassword: aturan password untuk registrasi dan reset (8 karakter sampai 72 byte).
func ValidatePassword(password string) error {
	// hitung karakter (rune) dan byte; di luar batas berarti lemah
	if utf8.RuneCountInString(password) < minPasswordRunes || len(password) > maxPasswordBytes {
		return apperror.ErrWeakPassword
	}
	return nil
}

// Register membuat akun dan mengembalikan token login. Email verifikasi dikirim di latar belakang.
func (s *Service) Register(ctx context.Context, email, username, password string) (string, error) {
	// periksa kekuatan password lebih dulu; yang lemah ditolak sebelum menyentuh database
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	// ubah password menjadi hash bcrypt agar tidak disimpan dalam bentuk asli
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	// simpan akun baru; email atau username yang sudah dipakai menghasilkan ErrDuplicate
	user, err := s.users.Create(ctx, email, username, string(hash))
	if err != nil {
		return "", err
	}
	// terbitkan token login untuk akun yang baru dibuat
	token, err := s.issuer.Issue(user.ID)
	if err != nil {
		return "", err
	}
	// kirim email verifikasi tanpa menunggu hasilnya
	s.sendVerificationInBackground(user.ID)
	return token, nil
}

// sendVerificationInBackground mengirim email verifikasi tanpa menahan respons registrasi.
func (s *Service) sendVerificationInBackground(userID int64) {
	// verifikasi mati: tidak ada email yang dikirim
	if !s.verifyOn {
		return
	}
	// jalankan di goroutine agar registrasi tidak menunggu SMTP yang lambat
	go func() {
		// context sendiri dengan batas waktu, tidak mengikuti request yang sudah selesai
		ctx, cancel := context.WithTimeout(context.Background(), verificationSendTimeout)
		defer cancel()
		// gagal kirim hanya dicatat (tanpa alamat email); user bisa kirim ulang dari Profil
		if err := s.SendVerification(ctx, userID, ""); err != nil {
			s.log.Error("send verification email after register failed", "user_id", userID, "err", err)
		}
	}()
}

// Login menerima email atau username. Kandidat dicocokkan satu per satu dengan password; username yang sama
// bisa dimiliki lebih dari satu user, jadi yang cocok passwordnya yang masuk.
func (s *Service) Login(ctx context.Context, identifier, password string) (string, error) {
	// cari semua akun yang cocok dengan email atau username yang dimasukkan
	users, err := s.users.FindByLogin(ctx, identifier)
	if err != nil {
		return "", err
	}
	// cocokkan password setiap kandidat satu per satu
	for _, user := range users {
		// Akun tanpa password (hanya login Google) dilewati.
		if !user.PasswordHash.Valid {
			continue
		}
		// hash tidak cocok dengan password: coba kandidat berikutnya
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(password)) != nil {
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
func (s *Service) Authenticate(ctx context.Context, raw string) (int64, error) {
	// periksa tanda tangan RS256 dan masa berlaku
	claims, err := s.issuer.Verify(raw)
	if err != nil {
		return 0, apperror.ErrUnauthenticated
	}
	// klaim `sub` harus berupa string
	sub, ok := claims["sub"].(string)
	if !ok {
		return 0, apperror.ErrUnauthenticated
	}
	// ubah `sub` menjadi id user berupa angka
	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return 0, apperror.ErrUnauthenticated
	}
	// pencabutan sesi: JSON mengubah angka menjadi float64, jadi iat dibaca sebagai float64
	if iat, ok := claims["iat"].(float64); ok {
		// waktu terakhir password diganti, dibaca dari database
		changedAt, err := s.users.PasswordChangedAt(ctx, userID)
		// user sudah tidak ada: sesinya dianggap tidak sah
		if errors.Is(err, apperror.ErrNotFound) {
			return 0, apperror.ErrUnauthenticated
		}
		// gangguan database: kembalikan apa adanya (pemanggil membalas 500)
		if err != nil {
			return 0, err
		}
		// Dibandingkan dalam detik: iat berdetik, dan login tepat setelah reset tidak boleh ikut dicabut.
		if changedAt != nil && changedAt.Unix() > int64(iat) {
			return 0, apperror.ErrUnauthenticated
		}
	}
	return userID, nil
}

// RequestReset membuat token dan mengirim tautan ke email akun. Akun yang tidak ada, atau yang sudah mencapai
// batas per jam, dikembalikan sebagai nil supaya pemanggil tidak bisa membedakannya (tidak ada enumerasi).
func (s *Service) RequestReset(ctx context.Context, email, requestIP string) error {
	// tanpa pengirim email, reset tidak mungkin; rute juga tidak dipasang
	if s.mailer == nil {
		return errors.New("mailer not configured")
	}
	// cari akun berdasarkan email; email tidak terdaftar tidak dianggap galat (tidak ada enumerasi)
	user, err := s.users.FindByEmail(ctx, strings.TrimSpace(email))
	if errors.Is(err, apperror.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := s.now()
	// hitung token reset yang sudah dibuat akun ini dalam satu jam terakhir
	recent, err := s.tokens.CountRecentTokens(ctx, user.ID, PurposePasswordReset, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	// batas per jam tercapai: catat lalu kembalikan nil tanpa mengirim email
	if recent >= resetMaxPerHour {
		s.log.Warn("password reset rate limited", "user_id", user.ID)
		return nil
	}
	// token mentah acak yang hanya ada di email
	raw, err := newActionToken()
	if err != nil {
		return err
	}
	// simpan hash token (bukan token mentah); token lama yang belum dipakai dibatalkan
	if err := s.tokens.CreateToken(ctx, user.ID, PurposePasswordReset, hashActionToken(raw), now.Add(resetTokenTTL), requestIP); err != nil {
		return err
	}
	// kirim tautan reset yang memuat token mentah dan lama berlakunya dalam menit
	return s.mailer.Send(ctx, user.Email, "password-reset", map[string]any{
		"Name":    user.Username,
		"Link":    s.publicURL + "/reset-password?token=" + raw,
		"Minutes": int(resetTokenTTL / time.Minute),
	})
}

// ResetPassword memasang password baru bila token sah. Password diperiksa sebelum token disentuh.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	// password lemah ditolak sebelum token disentuh, jadi tidak menghabiskan token
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	// buang spasi di ujung token; token kosong tidak mungkin sah
	token = strings.TrimSpace(token)
	if token == "" {
		return apperror.ErrInvalidResetToken
	}
	// hash password baru, lalu tandai token terpakai dan simpan password secara atomik di store
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.tokens.ResetPassword(ctx, hashActionToken(token), string(hash))
}

// SendVerification membuat token baru dan mengirim tautan verifikasi. Akun yang sudah terverifikasi: tidak
// mengirim apa pun. Batas per jam habis: apperror.ErrTooManyVerifyEmails.
func (s *Service) SendVerification(ctx context.Context, userID int64, requestIP string) error {
	// tanpa pengirim email, tidak ada yang bisa dikirim
	if s.mailer == nil {
		return errors.New("mailer not configured")
	}
	// ambil data user (email, username, status verifikasi)
	user, err := s.users.FindForVerification(ctx, userID)
	if err != nil {
		return err
	}
	// sudah terverifikasi: tidak ada yang perlu dikirim
	if user.EmailVerifiedAt.Valid {
		return nil
	}
	now := s.now()
	// hitung token verifikasi yang sudah dibuat akun ini dalam satu jam terakhir
	recent, err := s.tokens.CountRecentTokens(ctx, userID, PurposeEmailVerify, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	// batas per jam tercapai: tolak dengan ErrTooManyVerifyEmails
	if recent >= verifyMaxPerHour {
		s.log.Warn("verification email rate limited", "user_id", userID)
		return apperror.ErrTooManyVerifyEmails
	}
	// token mentah acak yang hanya ada di email
	raw, err := newActionToken()
	if err != nil {
		return err
	}
	// simpan hash token; token lama yang belum dipakai dibatalkan dalam satu transaksi
	if err := s.tokens.CreateToken(ctx, userID, PurposeEmailVerify, hashActionToken(raw), now.Add(verifyTokenTTL), requestIP); err != nil {
		return err
	}
	// kirim tautan verifikasi yang memuat token mentah dan lama berlakunya dalam jam
	return s.mailer.Send(ctx, user.Email, "email-verify", map[string]any{
		"Name":  user.Username,
		"Link":  s.publicURL + "/verify-email?token=" + raw,
		"Hours": int(verifyTokenTTL / time.Hour),
	})
}

// VerifyEmail menandai email terverifikasi bila token sah; token kosong atau tidak sah: ErrInvalidResetToken.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	// buang spasi di ujung token; token kosong tidak mungkin sah
	token = strings.TrimSpace(token)
	if token == "" {
		return apperror.ErrInvalidResetToken
	}
	// cocokkan hash token di database, tandai email terverifikasi dan token terpakai
	return s.tokens.VerifyEmail(ctx, hashActionToken(token))
}

// newActionToken: 32 byte acak dari crypto/rand, base64url tanpa padding (43 karakter).
func newActionToken() (string, error) {
	b := make([]byte, 32)
	// isi dengan byte acak dari crypto/rand; gagal berarti sumber keacakan bermasalah
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashActionToken: SHA-256 token mentah (hex). Hanya hash ini yang disimpan di database.
func hashActionToken(raw string) string {
	// hitung SHA-256 lalu ubah menjadi string hex
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
