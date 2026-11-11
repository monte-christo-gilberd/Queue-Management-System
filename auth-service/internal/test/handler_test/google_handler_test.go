package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/model"
	"qms/auth-service/internal/test/testutil"
	"qms/auth-service/internal/validator"
)

// googleRouter: router dengan login Google menyala dan satu ID token uji "tok" (budi@gmail.com).
func googleRouter(t *testing.T) (*gin.Engine, *testutil.Env) {
	t.Helper()
	e, env := newRouter(t, true, false)
	env.Google.Identities["tok"] = model.GoogleIdentity{Subject: "g-1", Email: "budi@gmail.com", Name: "Budi"}
	return e, env
}

func googleLogin(e *gin.Engine, idToken string) *httptest.ResponseRecorder {
	return do(e, http.MethodPost, "/api/v1/users/google", `{"id_token":"`+idToken+`"}`, "")
}

func linkTokenOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	return decode(t, w)["error"].(map[string]any)["link_token"].(string)
}

func TestGoogleRoutes_AbsentWhenGoogleIsNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registerOnce.Do(validator.RegisterValidators)
	env := testutil.NewServiceWithoutGoogle(t)
	e := testutil.NewEngine(env, true)

	assert.Equal(t, http.StatusNotFound, do(e, http.MethodPost, "/api/v1/users/google", `{"id_token":"x"}`, "").Code)
	assert.Equal(t, http.StatusNotFound, do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"x","password":"y"}`, "").Code)
	// rute lain tidak terpengaruh
	assert.Equal(t, http.StatusOK, do(e, http.MethodGet, "/health", "", "").Code)
}

func TestGoogleLogin_NewAccountGetsTokenInDataEnvelopeAndNoStore(t *testing.T) {
	e, env := googleRouter(t)

	w := googleLogin(e, "tok")

	token := tokenOf(t, w)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"), "respons yang memuat token tidak boleh di-cache")
	id, err := env.Svc.Authenticate(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, "budi@gmail.com", env.Store.User(id).Email)
}

func TestGoogleLogin_ExistingPasswordAccountIs409WithLinkTokenAndNoSession(t *testing.T) {
	e, env := googleRouter(t)
	register(t, e, "budi@gmail.com", "budi", "rahasia123")

	w := googleLogin(e, "tok")

	assert.Equal(t, http.StatusConflict, w.Code)
	errObj := decode(t, w)["error"].(map[string]any)
	assert.Equal(t, "GOOGLE_LINK_REQUIRED", errObj["code"])
	assert.NotEmpty(t, errObj["link_token"])
	assert.NotContains(t, w.Body.String(), `"data"`, "belum ada token login")
	assert.False(t, env.Store.User(1).GoogleID.Valid, "tidak tertaut otomatis")
}

func TestGoogleLogin_ErrorsMapToStatusAndCode(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
		code   string
	}{
		"token tidak sah":          {apperror.ErrGoogleInvalidToken, http.StatusUnauthorized, "UNAUTHORIZED"},
		"email belum diverifikasi": {apperror.ErrGoogleEmailNotVerified, http.StatusForbidden, "GOOGLE_EMAIL_UNVERIFIED"},
		"Google tidak terjangkau":  {apperror.ErrGoogleUnavailable, http.StatusServiceUnavailable, "GOOGLE_UNAVAILABLE"},
		"tabrakan akun":            {apperror.ErrDuplicate, http.StatusConflict, "CONFLICT"},
		"galat tak terduga":        {testutil.ErrBoom, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for name, tc := range cases {
		e, env := googleRouter(t)
		env.Google.Err = tc.err

		w := googleLogin(e, "tok")

		assert.Equal(t, tc.status, w.Code, name)
		assert.Equal(t, tc.code, errCode(t, w), name)
		assert.NotContains(t, w.Body.String(), "boom", name+": detail galat tidak bocor")
		assert.Empty(t, env.Store.Users, name)
	}
}

func TestGoogleLogin_InvalidBodyIs400AndSkipsVerification(t *testing.T) {
	e, env := googleRouter(t)

	for name, body := range map[string]string{
		"kosong": `{}`, "id_token kosong": `{"id_token":""}`, "bukan json": `bukan json`,
		"terlalu besar": `{"id_token":"` + string(make([]byte, 0)) + `x` + repeat("a", 8200) + `"}`,
	} {
		w := do(e, http.MethodPost, "/api/v1/users/google", body, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}
	assert.Zero(t, env.Google.Calls, "verifikasi tidak dijalankan untuk body yang salah")
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestLinkGoogle_Outcomes(t *testing.T) {
	e, env := googleRouter(t)
	register(t, e, "budi@gmail.com", "budi", "rahasia123")
	link := linkTokenOf(t, googleLogin(e, "tok"))

	wrong := do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"`+link+`","password":"salah"}`, "")
	assert.Equal(t, http.StatusUnauthorized, wrong.Code)
	assert.Equal(t, "UNAUTHORIZED", errCode(t, wrong))
	assert.False(t, env.Store.User(1).GoogleID.Valid)

	bad := do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"sampah","password":"rahasia123"}`, "")
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Equal(t, "INVALID_TOKEN", errCode(t, bad))

	ok := do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"`+link+`","password":"rahasia123"}`, "")
	token := tokenOf(t, ok)
	assert.Equal(t, "no-store", ok.Header().Get("Cache-Control"))
	id, err := env.Svc.Authenticate(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
	assert.Equal(t, "g-1", env.Store.User(1).GoogleID.String)

	again := do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"`+link+`","password":"rahasia123"}`, "")
	assert.Equal(t, "INVALID_TOKEN", errCode(t, again), "link token sekali pakai")

	// setelah tertaut, login Google langsung berhasil
	assert.NotEmpty(t, tokenOf(t, googleLogin(e, "tok")))
}

func TestLinkGoogle_MissingFieldsIs400(t *testing.T) {
	e, _ := googleRouter(t)
	for _, body := range []string{`{}`, `{"link_token":"x"}`, `{"password":"x"}`, `{"link_token":"` + repeat("a", 2100) + `","password":"x"}`} {
		assert.Equal(t, http.StatusBadRequest, do(e, http.MethodPost, "/api/v1/users/google/link", body, "").Code, body[:min(len(body), 40)])
	}
}

func TestLinkGoogle_BurnedAfterFiveWrongPasswordsIs400(t *testing.T) {
	e, _ := googleRouter(t)
	register(t, e, "budi@gmail.com", "budi", "rahasia123")
	link := linkTokenOf(t, googleLogin(e, "tok"))

	for i := 0; i < 5; i++ {
		w := do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"`+link+`","password":"salah"}`, "")
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
	w := do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"`+link+`","password":"rahasia123"}`, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "INVALID_TOKEN", errCode(t, w))
}

func TestLinkToken_CannotBeUsedAsSessionOnProtectedRoute(t *testing.T) {
	// Link token memuat sub dan ditandatangani kunci yang sama dengan token login. Ia tidak boleh membuka rute
	// yang butuh login (kirim ulang verifikasi), karena itu berarti sesi tanpa password.
	e, env := googleRouter(t)
	register(t, e, "budi@gmail.com", "budi", "rahasia123")
	link := linkTokenOf(t, googleLogin(e, "tok"))

	w := do(e, http.MethodPost, "/api/v1/users/me/verify-email/resend", "", link)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, env.Mailer.Sent(), "rute tidak dijalankan")
}

func TestGoogleLogin_DoesNotBreakPasswordLoginOfTheSameAccount(t *testing.T) {
	e, _ := googleRouter(t)
	register(t, e, "budi@gmail.com", "budi", "rahasia123")
	link := linkTokenOf(t, googleLogin(e, "tok"))
	tokenOf(t, do(e, http.MethodPost, "/api/v1/users/google/link", `{"link_token":"`+link+`","password":"rahasia123"}`, ""))

	w := do(e, http.MethodPost, "/api/v1/users/login", `{"identifier":"budi","password":"rahasia123"}`, "")

	assert.NotEmpty(t, tokenOf(t, w))
}
