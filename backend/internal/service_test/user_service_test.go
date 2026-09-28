package service_test

import (
	"context"
	"database/sql"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	createUser model.User
	createErr  error
	findUser   model.User
	findErr    error
}

func (r *fakeUserRepo) Create(
	ctx context.Context,
	email,
	username,
	passwordHash string,
) (model.User, error) {
	return r.createUser, r.createErr
}

func (r *fakeUserRepo) FindByEmail(
	ctx context.Context,
	email string,
) (model.User, error) {
	return r.findUser, r.findErr
}

func TestLogin_Success(t *testing.T) {
	// langkah 1: siapkan hash bcrypt dari password yang akan dites
	hashBytes, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	// langkah 2: buat fake, isi findUser dengan hash itu
	repo := &fakeUserRepo{
		findUser: model.User{
			ID:           1,
			PasswordHash: sql.NullString{String: string(hashBytes), Valid: true},
		},
	}

	// langkah 3: buat service ASLI (bukan fake), suntik fake repo ke dalamnya
	svc := service.NewUserService(repo, "test-secret")

	// langkah 4: panggil Login ASLI (dari UserService, bukan dari fake)
	token, err := svc.Login(context.Background(), "test@test.com", "password123")

	// langkah 5: assert
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestLogin_WrongPassword(t *testing.T) {
	// langkah 1: siapkan hash bcrypt dari password yang akan dites
	hashBytes, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	// langkah 2: buat fake, isi findUser dengan hash itu
	repo := &fakeUserRepo{
		findUser: model.User{
			ID:           1,
			PasswordHash: sql.NullString{String: string(hashBytes), Valid: true},
		},
	}

	// langkah 3: buat service ASLI (bukan fake), suntik fake repo ke dalamnya
	svc := service.NewUserService(repo, "test-secret")

	// langkah 4: panggil Login ASLI (dari UserService, bukan dari fake) dengan password salah
	token, err := svc.Login(context.Background(), "test@test.com", "salah_password")

	// langkah 5: assert
	assert.Error(t, err)
	assert.Empty(t, token)
}

func TestLogin_EmailNotFound(t *testing.T) {

	repo := &fakeUserRepo{
		findErr: apperror.ErrNotFound,
	}

	svc := service.NewUserService(repo, "test-secret")

	token, err := svc.Login(context.Background(), "test@test.com", "salah_password")

	assert.Error(t, err)
	assert.Empty(t, token)
}

func TestRegister_Success(t *testing.T) {

	repo := &fakeUserRepo{
		createUser: model.User{ID: 1}, // minimal ID yang valid
	}

	svc := service.NewUserService(repo, "test-secret")

	token, err := svc.Register(context.Background(), "test@test.com", "budi", "password123")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestRegister_EmailAlreadyExist(t *testing.T) {

	repo := &fakeUserRepo{
		createErr: apperror.ErrDuplicate,
	}

	svc := service.NewUserService(repo, "test-secret")

	token, err := svc.Register(context.Background(), "test@test.com", "budi", "password123")

	assert.Error(t, err)
	assert.Empty(t, token)
}
