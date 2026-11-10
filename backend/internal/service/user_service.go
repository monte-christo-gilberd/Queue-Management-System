package service

import (
	"context"
	"errors"
	"io"
	"qms/internal/apperror"
	"qms/internal/imageproc"
	"qms/internal/model"
	"strings"
	"time"
	"unicode/utf8"
)

// UserRepository: pembacaan akun, status owner beserta langganannya, dan pembaruan profil. Pembuatan akun dan
// pencarian login ada di auth-service, bukan di sini. Dipenuhi repository (user_repo.go dan owner_repo.go).
type UserRepository interface {
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

// UserService: profil user, foto profil, username, dan preferensi email notifikasi. Registrasi dan login ada di
// auth-service; backend hanya memverifikasi token yang diterbitkannya (middleware.TokenVerifier).
type UserService struct {
	// repository: penyimpanan akun dan langganan owner (UserRepository).
	repository UserRepository
	// avatars: penyimpanan foto profil. Nil = fitur foto tidak aktif.
	avatars AvatarStore
	// verificationOn: true bila verifikasi email berlaku. false = semua akun dilaporkan terverifikasi.
	verificationOn bool
}

// WithEmailVerification menyatakan apakah verifikasi email berlaku. Bila false, setiap akun dilaporkan
// terverifikasi sehingga klien tidak menampilkan banner verifikasi.
func (s *UserService) WithEmailVerification(enabled bool) *UserService {
	s.verificationOn = enabled
	return s
}

// WithAvatars memasang penyimpanan foto profil. Tanpa ini, Profile tidak melaporkan foto dan
// operasi foto mengembalikan ErrNotFound.
func (s *UserService) WithAvatars(store AvatarStore) *UserService {
	s.avatars = store
	return s
}

// NewUserService membuat UserService dengan repository. Penyimpanan foto dan status verifikasi dipasang
// sesudahnya lewat WithAvatars dan WithEmailVerification.
func NewUserService(
	repository UserRepository,
) *UserService {
	return &UserService{
		repository: repository,
	}
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
		// Verifikasi mati = tidak ada yang perlu diverifikasi, jadi klien tidak menampilkan banner.
		EmailVerified: user.EmailVerifiedAt.Valid || !s.verificationOn,
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
