package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"qms/internal/apperror"
	"qms/internal/imageproc"
	"qms/internal/model"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository: penyimpanan akun, pencarian login (email atau username), dan status owner beserta
// langganannya. Dipenuhi repository (user_repo.go dan owner_repo.go).
type UserRepository interface {
	Create(
		ctx context.Context,
		email,
		username,
		passwordHash string,
	) (model.User, error)
	FindByLogin(
		ctx context.Context,
		identifier string,
	) ([]model.User, error)
	FindByID(
		ctx context.Context,
		id int64,
	) (model.User, error)
	IsOwner(
		ctx context.Context,
		userID int64,
	) (bool, error)
	SubscriptionExpiresAt(
		ctx context.Context,
		userID int64,
	) (*time.Time, error)
	UpdateUsername(
		ctx context.Context,
		userID int64,
		username string,
	) (model.User, error)
	SetEmailNotifications(
		ctx context.Context,
		userID int64,
		enabled bool,
	) error
}

// maxUsernameLen: batas panjang username dalam karakter (rune).
const maxUsernameLen = 50

// AvatarStore: penyimpanan foto profil. Implementasi sekarang di Postgres (repository.AvatarRepository);
// antarmuka ini jalur pindah ke object storage bila foto sudah puluhan ribu.
type AvatarStore interface {
	Put(ctx context.Context, userID int64, data []byte) (time.Time, error)
	Get(ctx context.Context, userID int64) (model.Avatar, error)
	Delete(ctx context.Context, userID int64) error
	UpdatedAt(ctx context.Context, userID int64) (*time.Time, error)
}

// EmailVerifier mengirim email verifikasi ke akun. Dipenuhi *EmailVerificationService.
type EmailVerifier interface {
	SendVerification(ctx context.Context, userID int64, requestIP string) error
}

// verificationSendTimeout membatasi pengiriman email verifikasi di latar belakang saat registrasi.
const verificationSendTimeout = 45 * time.Second

// UserService: akun user (registrasi, login, profil), foto profil, username, dan preferensi email notifikasi.
// Token login berupa JWT HS256 yang ditandatangani dengan jwtSecret.
type UserService struct {
	repository UserRepository
	jwtSecret  string
	avatars    AvatarStore
	verifier   EmailVerifier
}

// WithEmailVerifier menyalakan verifikasi email: Register mengirim tautan verifikasi dan Profile melaporkan
// status verifikasi. Tanpa ini (verifikasi mati), setiap akun dilaporkan terverifikasi dan tidak ada email dikirim.
func (s *UserService) WithEmailVerifier(v EmailVerifier) *UserService {
	s.verifier = v
	return s
}

// WithAvatars memasang penyimpanan foto profil. Tanpa ini, Profile tidak melaporkan foto dan
// operasi foto mengembalikan ErrNotFound.
func (s *UserService) WithAvatars(store AvatarStore) *UserService {
	s.avatars = store
	return s
}

// NewUserService membuat UserService dengan repository dan secret JWT. Verifier dan penyimpanan foto
// dipasang sesudahnya lewat WithEmailVerifier dan WithAvatars.
func NewUserService(
	repository UserRepository,
	jwtSecret string,
) *UserService {
	return &UserService{
		repository: repository,
		jwtSecret:  jwtSecret,
	}
}

// Register memvalidasi password, meng-hash-nya dengan bcrypt (cost 10), membuat akun, lalu mengembalikan token
// login. Email verifikasi dikirim di latar belakang dan tidak menahan respons.
func (s *UserService) Register(
	ctx context.Context,
	email,
	username,
	password string,
) (string, error) {

	var user model.User

	// Periksa kekuatan password; bila tidak memenuhi syarat, kembalikan error.
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	// Ubah password menjadi hash bcrypt dengan cost 10 agar tidak disimpan dalam bentuk asli.
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		10,
	)
	// Gagal membuat hash: kembalikan error.
	if err != nil {
		return "", err
	}

	// Simpan akun baru beserta hash password ke repository.
	user, err = s.repository.Create(
		ctx,
		email,
		username,
		string(passwordHash),
	)
	// Gagal membuat akun (misalnya email sudah dipakai): kembalikan error.
	if err != nil {
		return "", err
	}

	// Terbitkan token login (JWT) untuk akun yang baru dibuat.
	token, err := s.issueToken(user.ID)
	// Gagal membuat token: kembalikan error.
	if err != nil {
		return "", err
	}

	// Kirim email verifikasi di latar belakang tanpa menunggu hasilnya.
	s.sendVerificationInBackground(user.ID)

	return token, nil
}

// sendVerificationInBackground mengirim email verifikasi tanpa menahan respons registrasi (SMTP bisa lambat).
// Gagal kirim hanya dicatat di log tanpa alamat email: registrasi tetap berhasil dan user bisa kirim ulang dari Profil.
func (s *UserService) sendVerificationInBackground(userID int64) {
	// Verifikasi email dimatikan (tanpa verifier): tidak ada email yang dikirim.
	if s.verifier == nil {
		return
	}
	// Jalankan pengiriman di goroutine terpisah agar registrasi tidak menunggu SMTP.
	go func() {
		// Buat context dengan batas waktu verificationSendTimeout untuk pengiriman email.
		ctx, cancel := context.WithTimeout(context.Background(), verificationSendTimeout)
		// Lepas resource context begitu fungsi goroutine ini selesai.
		defer cancel()
		// Kirim email verifikasi; bila gagal, catat di log tanpa alamat email.
		if err := s.verifier.SendVerification(ctx, userID, ""); err != nil {
			slog.Error("send verification email after register failed", "user_id", userID, "err", err)
		}
	}()
}

// Login menerima email atau username. Kandidat dicocokkan satu per satu dengan password;
// username yang sama bisa dimiliki lebih dari satu user, jadi yang cocok passwordnya yang masuk.
func (s *UserService) Login(
	ctx context.Context,
	identifier,
	password string,
) (string, error) {

	// Cari semua akun yang cocok dengan email atau username yang dimasukkan.
	users, err := s.repository.FindByLogin(
		ctx,
		identifier,
	)
	// Gagal mencari akun: kembalikan error.
	if err != nil {
		return "", err
	}

	// Cocokkan password setiap kandidat satu per satu.
	for _, user := range users {
		// Akun tanpa password (hanya login Google): lewati kandidat ini.
		if !user.PasswordHash.Valid {
			// akun Google-only tidak punya password
			continue
		}
		// Bandingkan hash tersimpan dengan password yang dimasukkan; hasil bukan nil berarti tidak cocok.
		if bcrypt.CompareHashAndPassword(
			[]byte(user.PasswordHash.String),
			[]byte(password),
		) != nil {
			continue
		}
		// Password cocok: terbitkan token login untuk user ini.
		return s.issueToken(user.ID)
	}

	return "", apperror.ErrNotFound
}

// issueToken membuat JWT HS256 dengan klaim sub (id user), iat (waktu terbit, dipakai pencabutan sesi), dan
// exp (berlaku 24 jam).
func (s *UserService) issueToken(
	userID int64,
) (string, error) {
	// Ambil waktu sekarang sebagai dasar perhitungan klaim iat dan exp.
	now := time.Now()
	// Susun klaim JWT: sub berisi ID user sebagai string, iat waktu terbit, dan exp 24 jam dari sekarang.
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		// iat dipakai pencabutan sesi setelah ganti/reset password (D6).
		"iat": now.Unix(),
		"exp": now.Add(24 * time.Hour).Unix(),
	}
	// Buat token baru dengan metode penandatanganan HS256 dan klaim di atas.
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)
	// Tandatangani token dengan jwtSecret dan kembalikan string token-nya.
	return token.SignedString([]byte(s.jwtSecret))
}

// Profile mengembalikan user yang sedang login, apakah ia owner, dan (untuk owner) akhir masa langganan.
func (s *UserService) Profile(
	ctx context.Context,
	userID int64,
) (model.Profile, error) {

	// Ambil data akun user yang sedang login.
	user, err := s.repository.FindByID(ctx, userID)
	// Akun tidak bisa dibaca: kembalikan error.
	if err != nil {
		return model.Profile{}, err
	}

	// Cek apakah user ini memiliki peran owner.
	isOwner, err := s.repository.IsOwner(ctx, userID)
	// Gagal mengecek status owner: kembalikan error.
	if err != nil {
		return model.Profile{}, err
	}

	// Susun profil dari data akun, status owner, dan status verifikasi email (terverifikasi bila sudah
	// dikonfirmasi, atau bila verifikasi email dimatikan).
	profile := model.Profile{
		User:    user,
		IsOwner: isOwner,
		// Verifikasi mati (tanpa verifier) = tidak ada yang perlu diverifikasi, jadi klien tidak menampilkan banner.
		EmailVerified: user.EmailVerifiedAt.Valid || s.verifier == nil,
	}
	// Hanya owner yang punya langganan: ambil tanggal akhir langganannya.
	if isOwner {
		profile.SubscriptionExpiresAt, err = s.repository.SubscriptionExpiresAt(ctx, userID)
		// Gagal membaca masa langganan: kembalikan error.
		if err != nil {
			return model.Profile{}, err
		}
	}
	// Penyimpanan foto dipasang: ambil waktu terakhir foto diubah untuk ditampilkan di profil.
	if s.avatars != nil {
		profile.AvatarUpdatedAt, err = s.avatars.UpdatedAt(ctx, userID)
		// Gagal membaca waktu foto: kembalikan error.
		if err != nil {
			return model.Profile{}, err
		}
	}
	return profile, nil
}

// SetAvatar mengolah unggahan (validasi isi, crop, perkecil, encode ulang) lalu menggantikan foto user.
func (s *UserService) SetAvatar(ctx context.Context, userID int64, upload io.Reader) error {
	// Penyimpanan foto belum dipasang: operasi foto tidak tersedia, kembalikan ErrNotFound.
	if s.avatars == nil {
		return apperror.ErrNotFound
	}
	// Proses unggahan (cek isi, crop, perkecil, encode ulang) menjadi data gambar siap simpan.
	data, err := imageproc.Process(upload)
	// Pilih pesan error sesuai jenis kegagalan pemrosesan gambar.
	switch {
	// Ukuran file terlalu besar: kembalikan ErrImageTooLarge.
	case errors.Is(err, imageproc.ErrTooLarge):
		return apperror.ErrImageTooLarge
	// Format gambar tidak didukung: kembalikan ErrUnsupportedImage.
	case errors.Is(err, imageproc.ErrUnsupported):
		return apperror.ErrUnsupportedImage
	// Kegagalan lain saat memproses gambar: kembalikan ErrInvalidImage.
	case err != nil:
		return apperror.ErrInvalidImage
	}
	// Simpan foto baru menggantikan foto lama milik user.
	_, err = s.avatars.Put(ctx, userID, data)
	return err
}

// Avatar mengembalikan foto user; ErrNotFound bila belum ada.
func (s *UserService) Avatar(ctx context.Context, userID int64) (model.Avatar, error) {
	// Penyimpanan foto belum dipasang: tidak ada foto yang bisa dibaca.
	if s.avatars == nil {
		return model.Avatar{}, apperror.ErrNotFound
	}
	// Ambil foto user dari penyimpanan.
	return s.avatars.Get(ctx, userID)
}

// DeleteAvatar menghapus foto user. Idempoten.
func (s *UserService) DeleteAvatar(ctx context.Context, userID int64) error {
	// Penyimpanan foto belum dipasang: tidak ada yang perlu dihapus, anggap berhasil.
	if s.avatars == nil {
		return nil
	}
	// Hapus foto user dari penyimpanan.
	return s.avatars.Delete(ctx, userID)
}

// UpdateUsername mengganti username user yang login. Spasi di tepi dibuang; panjang 1-50 karakter (rune).
// Username tidak unik, jadi tidak ada pengecekan duplikat.
func (s *UserService) UpdateUsername(
	ctx context.Context,
	userID int64,
	username string,
) (model.User, error) {

	// Buang spasi di awal dan akhir username.
	username = strings.TrimSpace(username)
	// Hitung panjang username dalam karakter (rune); di luar rentang 1 sampai maxUsernameLen berarti tidak valid.
	if n := utf8.RuneCountInString(username); n < 1 || n > maxUsernameLen {
		return model.User{}, apperror.ErrInvalidUsername
	}
	// Simpan username baru untuk user yang login.
	return s.repository.UpdateUsername(ctx, userID, username)
}

// SetEmailNotifications menyalakan atau mematikan email notifikasi tiket untuk user ini. Berlaku untuk kejadian
// berikutnya; email yang sudah masuk antrian kirim tetap terkirim.
func (s *UserService) SetEmailNotifications(ctx context.Context, userID int64, enabled bool) error {
	// Simpan preferensi email notifikasi (aktif atau tidak) ke repository.
	return s.repository.SetEmailNotifications(ctx, userID, enabled)
}
