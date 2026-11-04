package service

import (
	"context"
	"qms/internal/apperror"
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

type UserService struct {
	repository UserRepository
	jwtSecret  string
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
	return profile, nil
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
