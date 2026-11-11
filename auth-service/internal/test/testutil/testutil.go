package testutil

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/handler"
	"qms/auth-service/internal/middleware"
	"qms/auth-service/internal/model"
	"qms/auth-service/internal/router"
	"qms/auth-service/internal/service"
)

// Key: kunci RSA bersama untuk semua tes (membuat kunci 2048 bit cukup lambat, jadi dibuat sekali).
var Key = func() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return k
}()

// Token: satu baris user_action_tokens di memori.
type Token struct {
	UserID    int64
	Purpose   string
	ExpiresAt time.Time
	CreatedAt time.Time
	UsedAt    *time.Time
}

// FakeStore memenuhi service.UserRepository dan service.TokenRepository di memori.
type FakeStore struct {
	mu sync.Mutex
	// Users: akun yang ada; ID diisi saat Create (mulai dari 1).
	Users []model.User
	// Changed: waktu password terakhir diganti per user id.
	Changed map[int64]*time.Time
	// tokens: kunci = hash token.
	tokens map[string]*Token
	// Err: bila terisi, PasswordChangedAt mengembalikannya (mensimulasikan database bermasalah).
	Err error
}

// NewFakeStore membuat FakeStore kosong.
func NewFakeStore() *FakeStore {
	return &FakeStore{Changed: map[int64]*time.Time{}, tokens: map[string]*Token{}}
}

// AddUser menambah akun langsung (tanpa melalui Register) dan mengembalikan id-nya.
func (s *FakeStore) AddUser(email, username, passwordHash string, verified bool) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := model.User{
		ID:       int64(len(s.Users) + 1),
		Email:    email,
		Username: username,
	}
	if passwordHash != "" {
		u.PasswordHash = sql.NullString{String: passwordHash, Valid: true}
	}
	if verified {
		u.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}
	s.Users = append(s.Users, u)
	return u.ID
}

// Create: email unik; duplikat menghasilkan ErrDuplicate seperti pelanggaran UNIQUE.
func (s *FakeStore) Create(_ context.Context, email, username, passwordHash string) (model.User, error) {
	s.mu.Lock()
	for _, u := range s.Users {
		if u.Email == email {
			s.mu.Unlock()
			return model.User{}, apperror.ErrDuplicate
		}
	}
	s.mu.Unlock()
	id := s.AddUser(email, username, passwordHash, false)
	return s.byID(id), nil
}

func (s *FakeStore) byID(id int64) model.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Users[id-1]
}

// User mengembalikan salinan akun dengan id tertentu (untuk pemeriksaan di tes).
func (s *FakeStore) User(id int64) model.User { return s.byID(id) }

// FindByLogin: email atau username sama; email didahulukan.
func (s *FakeStore) FindByLogin(_ context.Context, identifier string) ([]model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var byEmail, byName []model.User
	for _, u := range s.Users {
		switch {
		case u.Email == identifier:
			byEmail = append(byEmail, u)
		case u.Username == identifier:
			byName = append(byName, u)
		}
	}
	return append(byEmail, byName...), nil
}

// FindByEmail: email persis; ErrNotFound bila tidak ada.
func (s *FakeStore) FindByEmail(_ context.Context, email string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.Email == email {
			return u, nil
		}
	}
	return model.User{}, apperror.ErrNotFound
}

// FindForVerification: akun berdasarkan id; ErrNotFound bila tidak ada.
func (s *FakeStore) FindForVerification(_ context.Context, userID int64) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if userID < 1 || int(userID) > len(s.Users) {
		return model.User{}, apperror.ErrNotFound
	}
	return s.Users[userID-1], nil
}

// PasswordChangedAt: nil bila belum pernah diganti; ErrNotFound bila user tidak ada.
func (s *FakeStore) PasswordChangedAt(_ context.Context, userID int64) (*time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	if userID < 1 || int(userID) > len(s.Users) {
		return nil, apperror.ErrNotFound
	}
	return s.Changed[userID], nil
}

// FindByGoogleID: akun yang tertaut ke akun Google ini; ErrNotFound bila tidak ada.
func (s *FakeStore) FindByGoogleID(_ context.Context, googleID string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.GoogleID.Valid && u.GoogleID.String == googleID {
			return u, nil
		}
	}
	return model.User{}, apperror.ErrNotFound
}

// FindAccountByEmail: email tanpa membedakan huruf besar atau kecil; yang pertama (id terkecil) menang.
func (s *FakeStore) FindAccountByEmail(_ context.Context, email string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return model.User{}, apperror.ErrNotFound
}

// FindByID: akun berdasarkan id; ErrNotFound bila tidak ada.
func (s *FakeStore) FindByID(_ context.Context, id int64) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id < 1 || int(id) > len(s.Users) {
		return model.User{}, apperror.ErrNotFound
	}
	return s.Users[id-1], nil
}

// CreateGoogle membuat akun tanpa password dan terverifikasi. Email atau google_id yang sudah ada = ErrDuplicate.
func (s *FakeStore) CreateGoogle(_ context.Context, email, username, googleID string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.Email == email || (u.GoogleID.Valid && u.GoogleID.String == googleID) {
			return model.User{}, apperror.ErrDuplicate
		}
	}
	u := model.User{
		ID: int64(len(s.Users) + 1), Email: email, Username: username,
		GoogleID:        sql.NullString{String: googleID, Valid: true},
		EmailVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true},
	}
	s.Users = append(s.Users, u)
	return u, nil
}

// LinkGoogle menautkan google_id bila akun belum tertaut; selain itu ErrDuplicate.
func (s *FakeStore) LinkGoogle(_ context.Context, userID int64, googleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.Users {
		if u.GoogleID.Valid && u.GoogleID.String == googleID {
			return apperror.ErrDuplicate
		}
	}
	u := &s.Users[userID-1]
	if u.GoogleID.Valid {
		return apperror.ErrDuplicate
	}
	u.GoogleID = sql.NullString{String: googleID, Valid: true}
	if !u.EmailVerifiedAt.Valid {
		u.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}
	return nil
}

// SetPasswordChanged mengatur waktu penggantian password (untuk tes pencabutan sesi).
func (s *FakeStore) SetPasswordChanged(userID int64, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Changed[userID] = &at
}

// CountRecentTokens menghitung token purpose ini milik user yang dibuat setelah since.
func (s *FakeStore) CountRecentTokens(_ context.Context, userID int64, purpose string, since time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, t := range s.tokens {
		if t.UserID == userID && t.Purpose == purpose && t.CreatedAt.After(since) {
			n++
		}
	}
	return n, nil
}

// CreateToken membatalkan token lama (purpose sama) lalu menyimpan token baru.
func (s *FakeStore) CreateToken(_ context.Context, userID int64, purpose, hash string, expiresAt time.Time, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, t := range s.tokens {
		if t.UserID == userID && t.Purpose == purpose && t.UsedAt == nil {
			t.UsedAt = &now
		}
	}
	s.tokens[hash] = &Token{UserID: userID, Purpose: purpose, ExpiresAt: expiresAt, CreatedAt: now}
	return nil
}

// consume menandai token terpakai bila sah (purpose cocok, belum dipakai, belum kedaluwarsa).
func (s *FakeStore) consume(hash, purpose string) (*Token, error) {
	t, ok := s.tokens[hash]
	if !ok || t.Purpose != purpose || t.UsedAt != nil || !t.ExpiresAt.After(time.Now()) {
		return nil, apperror.ErrInvalidResetToken
	}
	now := time.Now()
	t.UsedAt = &now
	return t, nil
}

// ResetPassword memakai token reset, mengganti password, mengisi waktu penggantian, dan mencabut token reset lain.
func (s *FakeStore) ResetPassword(_ context.Context, hash, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.consume(hash, service.PurposePasswordReset)
	if err != nil {
		return err
	}
	s.Users[t.UserID-1].PasswordHash = sql.NullString{String: passwordHash, Valid: true}
	now := time.Now()
	s.Changed[t.UserID] = &now
	for _, other := range s.tokens {
		if other.UserID == t.UserID && other.Purpose == service.PurposePasswordReset && other.UsedAt == nil {
			other.UsedAt = &now
		}
	}
	return nil
}

// VerifyEmail memakai token verifikasi dan mengisi email_verified_at bila belum terisi.
func (s *FakeStore) VerifyEmail(_ context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.consume(hash, service.PurposeEmailVerify)
	if err != nil {
		return err
	}
	if u := &s.Users[t.UserID-1]; !u.EmailVerifiedAt.Valid {
		u.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}
	return nil
}

// ExpireTokens membuat semua token kedaluwarsa (mensimulasikan waktu berlalu).
func (s *FakeStore) ExpireTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tokens {
		t.ExpiresAt = time.Now().Add(-time.Minute)
	}
}

// StoredHashes mengembalikan hash token yang tersimpan (untuk memastikan token mentah tidak pernah disimpan).
func (s *FakeStore) StoredHashes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.tokens))
	for h := range s.tokens {
		out = append(out, h)
	}
	return out
}

// Mail: satu email terkirim yang direkam FakeMailer.
type Mail struct {
	To       string
	Template string
	Data     map[string]any
}

// FakeMailer merekam email dan bisa dipaksa gagal.
type FakeMailer struct {
	mu   sync.Mutex
	sent []Mail
	// Err: bila terisi, Send mengembalikannya (mail-service mati).
	Err error
	// Notify: bila tidak nil, menerima satu nilai tiap Send (untuk menunggu pengiriman di latar belakang).
	Notify chan struct{}
}

// Send merekam email lalu mengembalikan Err.
func (m *FakeMailer) Send(_ context.Context, to, template string, data map[string]any) error {
	m.mu.Lock()
	m.sent = append(m.sent, Mail{To: to, Template: template, Data: data})
	notify := m.Notify
	err := m.Err
	m.mu.Unlock()
	if notify != nil {
		notify <- struct{}{}
	}
	return err
}

// Sent mengembalikan salinan email yang sudah terkirim.
func (m *FakeMailer) Sent() []Mail {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Mail(nil), m.sent...)
}

// FakeGoogle memenuhi service.GoogleVerifier: ID token "uji" dipetakan ke identitas di memori.
type FakeGoogle struct {
	mu sync.Mutex
	// Identities: token → identitas. Token yang tidak ada = ErrGoogleInvalidToken.
	Identities map[string]model.GoogleIdentity
	// Err: bila terisi, Verify mengembalikannya untuk semua token (email belum terverifikasi, Google mati, dst.).
	Err error
	// Calls: jumlah panggilan Verify.
	Calls int
}

// Verify mengembalikan identitas untuk token yang dikenal.
func (g *FakeGoogle) Verify(_ context.Context, idToken string) (model.GoogleIdentity, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Calls++
	if g.Err != nil {
		return model.GoogleIdentity{}, g.Err
	}
	ident, ok := g.Identities[idToken]
	if !ok {
		return model.GoogleIdentity{}, apperror.ErrGoogleInvalidToken
	}
	return ident, nil
}

// Env: Service beserta fake-nya.
type Env struct {
	Svc    *service.Service
	Store  *FakeStore
	Mailer *FakeMailer
	Issuer *service.TokenIssuer
	Google *FakeGoogle
}

// NewService merangkai Service dengan FakeStore, FakeMailer, dan kunci RSA tes. verification menyalakan email
// verifikasi saat registrasi.
func NewService(t *testing.T, verification bool) *Env {
	t.Helper()
	store := NewFakeStore()
	mailer := &FakeMailer{}
	issuer := service.NewTokenIssuer(Key, "test-kid")
	svc := service.NewService(store, store, issuer, mailer, service.Options{
		PublicAppURL:         "https://app.example/",
		VerificationRequired: verification,
	})
	google := &FakeGoogle{Identities: map[string]model.GoogleIdentity{}}
	svc.WithGoogle(google)
	return &Env{Svc: svc, Store: store, Mailer: mailer, Issuer: issuer, Google: google}
}

// NewServiceWithoutGoogle seperti NewService, tetapi login Google mati (GOOGLE_CLIENT_IDS kosong).
func NewServiceWithoutGoogle(t *testing.T) *Env {
	t.Helper()
	store := NewFakeStore()
	mailer := &FakeMailer{}
	issuer := service.NewTokenIssuer(Key, "test-kid")
	svc := service.NewService(store, store, issuer, mailer, service.Options{PublicAppURL: "https://app.example/"})
	return &Env{Svc: svc, Store: store, Mailer: mailer, Issuer: issuer}
}

// ErrBoom: galat generik untuk mensimulasikan kegagalan infrastruktur.
var ErrBoom = errors.New("boom")

// NewEngine merangkai router HTTP seperti di main: registrasi dan login selalu ada, rute email hanya bila emailOn,
// rute Google hanya bila Google dinyalakan di Svc.
func NewEngine(env *Env, emailOn bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewAuthHandler(env.Svc)
	app := router.NewAppRouter(h, middleware.Auth(env.Svc), nil)
	if emailOn {
		app.WithEmail(h)
	}
	if env.Svc.GoogleEnabled() {
		app.WithGoogle(h)
	}
	return router.SetupRouter(app)
}
