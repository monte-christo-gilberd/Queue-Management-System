package service

import (
	"context"
	"errors"
	"io"
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
}

const maxUsernameLen = 50

// AvatarStore: penyimpanan foto profil. Implementasi sekarang di Postgres (repository.AvatarRepository);
// antarmuka ini jalur pindah ke object storage bila foto sudah puluhan ribu.
type AvatarStore interface {
	Put(ctx context.Context, userID int64, data []byte) (time.Time, error)
	Get(ctx context.Context, userID int64) (model.Avatar, error)
	Delete(ctx context.Context, userID int64) error
	UpdatedAt(ctx context.Context, userID int64) (*time.Time, error)
}

type UserService struct {
	repository UserRepository
	jwtSecret  string
	avatars    AvatarStore
}

// WithAvatars memasang penyimpanan foto profil. Tanpa ini, Profile tidak melaporkan foto dan
// operasi foto mengembalikan ErrNotFound.
func (s *UserService) WithAvatars(store AvatarStore) *UserService {
	s.avatars = store
	return s
}

func NewUserService(
	repository UserRepository,
	jwtSecret string,
) *UserService {
	return &UserService{
		repository: repository,
		jwtSecret:  jwtSecret,
	}
}

func (s *UserService) Register(
	ctx context.Context,
	email,
	username,
	password string,
) (string, error) {

	var user model.User

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		10,
	)
	if err != nil {
		return "", err
	}

	user, err = s.repository.Create(
		ctx,
		email,
		username,
		string(passwordHash),
	)
	if err != nil {
		return "", err
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		return "", err
	}

	return token, nil
}

// Login menerima email atau username. Kandidat dicocokkan satu per satu dengan password;
// username yang sama bisa dimiliki lebih dari satu user, jadi yang cocok passwordnya yang masuk.
func (s *UserService) Login(
	ctx context.Context,
	identifier,
	password string,
) (string, error) {

	users, err := s.repository.FindByLogin(
		ctx,
		identifier,
	)
	if err != nil {
		return "", err
	}

	for _, user := range users {
		if !user.PasswordHash.Valid {
			// akun Google-only tidak punya password
			continue
		}
		if bcrypt.CompareHashAndPassword(
			[]byte(user.PasswordHash.String),
			[]byte(password),
		) != nil {
			continue
		}
		return s.issueToken(user.ID)
	}

	return "", apperror.ErrNotFound
}

func (s *UserService) issueToken(
	userID int64,
) (string, error) {
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)
	return token.SignedString([]byte(s.jwtSecret))
}

// Profile mengembalikan user yang sedang login, apakah ia owner, dan (untuk owner) akhir masa langganan.
func (s *UserService) Profile(
	ctx context.Context,
	userID int64,
) (model.Profile, error) {

	user, err := s.repository.FindByID(ctx, userID)
	if err != nil {
		return model.Profile{}, err
	}

	isOwner, err := s.repository.IsOwner(ctx, userID)
	if err != nil {
		return model.Profile{}, err
	}

	profile := model.Profile{User: user, IsOwner: isOwner}
	if isOwner {
		profile.SubscriptionExpiresAt, err = s.repository.SubscriptionExpiresAt(ctx, userID)
		if err != nil {
			return model.Profile{}, err
		}
	}
	if s.avatars != nil {
		profile.AvatarUpdatedAt, err = s.avatars.UpdatedAt(ctx, userID)
		if err != nil {
			return model.Profile{}, err
		}
	}
	return profile, nil
}

// SetAvatar mengolah unggahan (validasi isi, crop, perkecil, encode ulang) lalu menggantikan foto user.
func (s *UserService) SetAvatar(ctx context.Context, userID int64, upload io.Reader) error {
	if s.avatars == nil {
		return apperror.ErrNotFound
	}
	data, err := imageproc.Process(upload)
	switch {
	case errors.Is(err, imageproc.ErrTooLarge):
		return apperror.ErrImageTooLarge
	case errors.Is(err, imageproc.ErrUnsupported):
		return apperror.ErrUnsupportedImage
	case err != nil:
		return apperror.ErrInvalidImage
	}
	_, err = s.avatars.Put(ctx, userID, data)
	return err
}

// Avatar mengembalikan foto user; ErrNotFound bila belum ada.
func (s *UserService) Avatar(ctx context.Context, userID int64) (model.Avatar, error) {
	if s.avatars == nil {
		return model.Avatar{}, apperror.ErrNotFound
	}
	return s.avatars.Get(ctx, userID)
}

// DeleteAvatar menghapus foto user. Idempoten.
func (s *UserService) DeleteAvatar(ctx context.Context, userID int64) error {
	if s.avatars == nil {
		return nil
	}
	return s.avatars.Delete(ctx, userID)
}

// UpdateUsername mengganti username user yang login. Spasi di tepi dibuang; panjang 1-50 karakter (rune).
// Username tidak unik, jadi tidak ada pengecekan duplikat.
func (s *UserService) UpdateUsername(
	ctx context.Context,
	userID int64,
	username string,
) (model.User, error) {

	username = strings.TrimSpace(username)
	if n := utf8.RuneCountInString(username); n < 1 || n > maxUsernameLen {
		return model.User{}, apperror.ErrInvalidUsername
	}
	return s.repository.UpdateUsername(ctx, userID, username)
}
