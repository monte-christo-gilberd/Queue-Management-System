package service

import (
	"context"
	"qms/internal/model"
	"strconv"
	"time"

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
	FindByEmail(
		ctx context.Context,
		email string,
	) (model.User, error)
}

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

func (s *UserService) Login(
	ctx context.Context,
	email,
	password string,
) (string, error) {

	user, err := s.repository.FindByEmail(
		ctx,
		email,
	)
	if err != nil {
		return "", err
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash.String),
		[]byte(password),
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
