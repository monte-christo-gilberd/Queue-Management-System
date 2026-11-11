package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/auth-service/internal/test/testutil"
	"qms/auth-service/internal/validator"
)

var registerOnce sync.Once

// newRouter merangkai router dengan Service fake. emailOn = mail-service terpasang.
func newRouter(t *testing.T, emailOn, verification bool) (*gin.Engine, *testutil.Env) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	registerOnce.Do(validator.RegisterValidators)
	env := testutil.NewService(t, verification)
	return testutil.NewEngine(env, emailOn), env
}

func do(e *gin.Engine, method, path, body, bearer string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	e.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m), w.Body.String())
	return m
}

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	return decode(t, w)["error"].(map[string]any)["code"].(string)
}

func tokenOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return decode(t, w)["data"].(map[string]any)["token"].(string)
}

func register(t *testing.T, e *gin.Engine, email, username, password string) string {
	t.Helper()
	body := `{"email":"` + email + `","username":"` + username + `","password":"` + password + `"}`
	return tokenOf(t, do(e, http.MethodPost, "/api/v1/users/register", body, ""))
}

func TestHealth_IsOKWithoutTouchingDatabase(t *testing.T) {
	e, _ := newRouter(t, true, false)

	w := do(e, http.MethodGet, "/health", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

// --- Register ---

func TestRegister_NewAccountGetsTokenInDataEnvelope(t *testing.T) {
	// TC-01-06: kontrak respons {"data":{"token":...}} sama dengan yang dulu dilayani backend.
	e, env := newRouter(t, true, false)

	token := register(t, e, "baru@example.com", "baru", "rahasia123")

	id, err := env.Svc.Authenticate(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
}

func TestRegister_DuplicateEmailIs409(t *testing.T) {
	// TC-01-07
	e, _ := newRouter(t, true, false)
	register(t, e, "ada@example.com", "ada", "rahasia123")

	w := do(e, http.MethodPost, "/api/v1/users/register", `{"email":"ada@example.com","username":"x","password":"rahasia123"}`, "")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "CONFLICT", errCode(t, w))
	assert.NotContains(t, w.Body.String(), `"token"`)
}

func TestRegister_WeakPasswordIs400(t *testing.T) {
	e, _ := newRouter(t, true, false)

	w := do(e, http.MethodPost, "/api/v1/users/register", `{"email":"a@example.com","username":"a","password":"pendek"}`, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "WEAK_PASSWORD", errCode(t, w))
}

func TestRegister_MissingFieldsAndHTMLAreRejected(t *testing.T) {
	// TC-01-05
	e, env := newRouter(t, true, false)
	for name, body := range map[string]string{
		"missing email":    `{"username":"u","password":"rahasia123"}`,
		"missing username": `{"email":"e@x.com","password":"rahasia123"}`,
		"missing password": `{"email":"e@x.com","username":"u"}`,
		"html in username": `{"email":"e@x.com","username":"<b>u</b>","password":"rahasia123"}`,
		"html in email":    `{"email":"<e@x.com>","username":"u","password":"rahasia123"}`,
		"bukan json":       `bukan json`,
	} {
		w := do(e, http.MethodPost, "/api/v1/users/register", body, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
		assert.Equal(t, "BAD_REQUEST", errCode(t, w), name)
	}
	assert.Empty(t, env.Store.Users, "tidak ada akun yang dibuat")
}

// --- Login ---

func TestLogin_AcceptsIdentifierEmailOrUsernameAndLegacyEmailField(t *testing.T) {
	e, _ := newRouter(t, true, false)
	register(t, e, "budi@mail.com", "budi", "rahasia123")

	for _, body := range []string{
		`{"identifier":"budi@mail.com","password":"rahasia123"}`,
		`{"identifier":"budi","password":"rahasia123"}`,
		`{"email":"budi@mail.com","password":"rahasia123"}`,
		`{"identifier":"budi","email":"lain@mail.com","password":"rahasia123"}`, // identifier menang
	} {
		w := do(e, http.MethodPost, "/api/v1/users/login", body, "")
		assert.NotEmpty(t, tokenOf(t, w), body)
	}
}

func TestLogin_EmptyCredentialsAreRejectedBeforeLookup(t *testing.T) {
	// TC-01-05
	e, _ := newRouter(t, true, false)
	for name, body := range map[string]string{
		"identifier dan password kosong": `{"identifier":"","password":""}`,
		"password hilang":                `{"identifier":"owner@example.com"}`,
		"identifier hilang":              `{"password":"rahasia123"}`,
	} {
		w := do(e, http.MethodPost, "/api/v1/users/login", body, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}
}

func TestLogin_WrongOrUnknownCredentialsGiveIdenticalResponse(t *testing.T) {
	// TC-01-03 / TC-01-04
	e, _ := newRouter(t, true, false)
	register(t, e, "owner@example.com", "owner", "rahasia123")

	wrong := do(e, http.MethodPost, "/api/v1/users/login", `{"identifier":"owner@example.com","password":"salah"}`, "")
	unknown := do(e, http.MethodPost, "/api/v1/users/login", `{"identifier":"tidak@ada.com","password":"salah"}`, "")

	assert.Equal(t, http.StatusUnauthorized, wrong.Code)
	assert.Equal(t, "UNAUTHORIZED", errCode(t, wrong))
	assert.Equal(t, wrong.Code, unknown.Code)
	assert.Equal(t, wrong.Body.String(), unknown.Body.String(), "tidak membocorkan akun yang terdaftar")
	assert.NotContains(t, wrong.Body.String(), `"token"`)
}

// --- Lupa dan reset password ---

func TestForgotPassword_Always202ForValidEmailAndSendsAsync(t *testing.T) {
	e, env := newRouter(t, true, false)
	register(t, e, "budi@mail.com", "budi", "rahasia123")
	env.Mailer.Notify = make(chan struct{}, 1)

	known := do(e, http.MethodPost, "/api/v1/users/forgot-password", `{"email":"budi@mail.com"}`, "")
	unknown := do(e, http.MethodPost, "/api/v1/users/forgot-password", `{"email":"tidak@ada.com"}`, "")

	assert.Equal(t, http.StatusAccepted, known.Code)
	assert.Equal(t, http.StatusAccepted, unknown.Code)
	assert.Equal(t, known.Body.String(), unknown.Body.String(), "respons seragam, tidak ada enumerasi akun")
	select {
	case <-env.Mailer.Notify:
	case <-time.After(2 * time.Second):
		t.Fatal("email reset tidak terkirim")
	}
}

func TestForgotPassword_InvalidBodyIs400(t *testing.T) {
	e, env := newRouter(t, true, false)
	for name, body := range map[string]string{
		"kosong":        `{}`,
		"bukan email":   `{"email":"bukan-email"}`,
		"html":          `{"email":"<a@b.c>"}`,
		"terlalu besar": `{"email":"` + strings.Repeat("a", 260) + `@b.c"}`,
	} {
		w := do(e, http.MethodPost, "/api/v1/users/forgot-password", body, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}
	assert.Empty(t, env.Mailer.Sent())
}

func TestResetPassword_Outcomes(t *testing.T) {
	e, env := newRouter(t, true, false)
	register(t, e, "budi@mail.com", "budi", "rahasia123")
	require.NoError(t, env.Svc.RequestReset(t.Context(), "budi@mail.com", ""))
	raw := strings.TrimPrefix(env.Mailer.Sent()[0].Data["Link"].(string), "https://app.example/reset-password?token=")

	weak := do(e, http.MethodPost, "/api/v1/users/reset-password", `{"token":"`+raw+`","password":"pendek"}`, "")
	assert.Equal(t, http.StatusBadRequest, weak.Code)
	assert.Equal(t, "WEAK_PASSWORD", errCode(t, weak))

	bad := do(e, http.MethodPost, "/api/v1/users/reset-password", `{"token":"salah","password":"password-baru"}`, "")
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Equal(t, "INVALID_TOKEN", errCode(t, bad))

	ok := do(e, http.MethodPost, "/api/v1/users/reset-password", `{"token":"`+raw+`","password":"password-baru"}`, "")
	assert.Equal(t, http.StatusOK, ok.Code)
	assert.Equal(t, "no-store", ok.Header().Get("Cache-Control"), "respons yang memakai token tidak boleh di-cache")
	assert.JSONEq(t, `{"data":{"status":"ok"}}`, ok.Body.String())

	again := do(e, http.MethodPost, "/api/v1/users/reset-password", `{"token":"`+raw+`","password":"password-baru"}`, "")
	assert.Equal(t, "INVALID_TOKEN", errCode(t, again), "token sekali pakai")
}

func TestResetPassword_MissingFieldsIs400(t *testing.T) {
	e, _ := newRouter(t, true, false)
	for _, body := range []string{`{}`, `{"token":"x"}`, `{"password":"password-baru"}`, `{"token":"` + strings.Repeat("a", 201) + `","password":"password-baru"}`} {
		assert.Equal(t, http.StatusBadRequest, do(e, http.MethodPost, "/api/v1/users/reset-password", body, "").Code, body)
	}
}

// --- Verifikasi email ---

func TestVerifyEmail_Outcomes(t *testing.T) {
	e, env := newRouter(t, true, false)
	id := env.Store.AddUser("budi@mail.com", "budi", "x", false)
	require.NoError(t, env.Svc.SendVerification(t.Context(), id, ""))
	raw := strings.TrimPrefix(env.Mailer.Sent()[0].Data["Link"].(string), "https://app.example/verify-email?token=")

	env.Mailer.Notify = make(chan struct{}, 2)
	bad := do(e, http.MethodPost, "/api/v1/users/verify-email", `{"token":"salah"}`, "")
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Equal(t, "INVALID_TOKEN", errCode(t, bad))

	ok := do(e, http.MethodPost, "/api/v1/users/verify-email", `{"token":"`+raw+`"}`, "")
	assert.Equal(t, http.StatusOK, ok.Code)
	assert.Equal(t, "no-store", ok.Header().Get("Cache-Control"))
	assert.True(t, env.Store.User(id).EmailVerifiedAt.Valid)
	assert.JSONEq(t, `{"data":{"status":"ok"}}`, ok.Body.String())
	select {
	case <-env.Mailer.Notify:
	case <-time.After(2 * time.Second):
		t.Fatal("email konfirmasi tidak terkirim")
	}
	require.Len(t, env.Mailer.Sent(), 2)
	assert.Equal(t, "email-verified", env.Mailer.Sent()[1].Template)

	assert.Equal(t, http.StatusBadRequest, do(e, http.MethodPost, "/api/v1/users/verify-email", `{}`, "").Code)
}

func TestResendVerification_RequiresLoginAndUsesUserFromToken(t *testing.T) {
	e, env := newRouter(t, true, false)
	token := register(t, e, "budi@mail.com", "budi", "rahasia123")

	noToken := do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", "")
	assert.Equal(t, http.StatusUnauthorized, noToken.Code)
	basic := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/me/verify-email/resend", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	e.ServeHTTP(basic, req)
	assert.Equal(t, http.StatusUnauthorized, basic.Code)
	assert.Equal(t, http.StatusUnauthorized, do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", "sampah").Code)
	assert.Empty(t, env.Mailer.Sent())

	ok := do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", token)
	assert.Equal(t, http.StatusAccepted, ok.Code)
	sent := env.Mailer.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "budi@mail.com", sent[0].To, "memakai user dari token login")
}

func TestResendVerification_LimitedToThreePerHourIs429(t *testing.T) {
	e, _ := newRouter(t, true, false)
	token := register(t, e, "budi@mail.com", "budi", "rahasia123")

	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusAccepted, do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", token).Code)
	}
	w := do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", token)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "TOO_MANY_REQUESTS", errCode(t, w))
}

func TestResendVerification_RevokedSessionIs401AndDatabaseErrorIs500(t *testing.T) {
	e, env := newRouter(t, true, false)
	token := register(t, e, "budi@mail.com", "budi", "rahasia123")

	env.Store.SetPasswordChanged(1, time.Now().Add(2*time.Second))
	revoked := do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", token)
	assert.Equal(t, http.StatusUnauthorized, revoked.Code)
	assert.Equal(t, "UNAUTHORIZED", errCode(t, revoked))

	env.Store.Err = testutil.ErrBoom
	broken := do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", token)
	assert.Equal(t, http.StatusInternalServerError, broken.Code)
	assert.NotContains(t, broken.Body.String(), "boom", "detail galat tidak bocor ke klien")
}

// --- Tanpa mail-service ---

func TestEmailRoutes_AbsentWithoutMailService(t *testing.T) {
	e, _ := newRouter(t, false, false)

	for _, path := range []string{
		"/api/v1/users/forgot-password", "/api/v1/users/reset-password",
		"/api/v1/users/verify-email", "/api/v1/users/me/verify-email/resend",
	} {
		assert.Equal(t, http.StatusNotFound, do(e, http.MethodPost, path, `{}`, "").Code, path)
	}
	// register dan login tetap ada
	register(t, e, "a@example.com", "a", "rahasia123")
	assert.Equal(t, http.StatusUnauthorized, do(e, http.MethodPost, "/api/v1/users/login", `{"identifier":"a","password":"salah"}`, "").Code)
}

func TestRoutesNotOwnedByAuthService_AreNotRegistered(t *testing.T) {
	// Profil, avatar, dan tiket tetap di backend: auth-service tidak boleh melayaninya.
	e, _ := newRouter(t, true, false)

	for _, path := range []string{"/api/v1/users/me", "/api/v1/users/me/avatar", "/api/v1/users/me/tickets", "/api/v1/queues"} {
		assert.Equal(t, http.StatusNotFound, do(e, http.MethodGet, path, "", "").Code, path)
	}
}
