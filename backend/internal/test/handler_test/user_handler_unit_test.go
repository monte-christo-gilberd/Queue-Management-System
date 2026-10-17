package handler_test

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"qms/internal/apperror"
	"qms/internal/handler"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func specUserEngine(svc *specUserSvc) *gin.Engine {
	e := specEngine(0)
	h := handler.NewUserHandler(svc)
	e.POST("/auth/register", h.Register)
	e.POST("/auth/login", h.Login)
	return e
}

func TestSpec_Register_NewAccountGetsTokenForDirectEntry(t *testing.T) {
	// TC-01-06: registrasi berhasil -> token langsung diberikan (tanpa login ulang).
	svc := &specUserSvc{token: "jwt-token"}
	e := specUserEngine(svc)

	w := specRequest(e, http.MethodPost, "/auth/register",
		`{"email":"baru@example.com","username":"baru","password":"rahasia123"}`)

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "jwt-token", data["token"])
	assert.Equal(t, "baru@example.com|baru", svc.gotID)
	assert.Equal(t, "rahasia123", svc.gotPass)
}

func TestSpec_Register_DuplicateEmailIs409(t *testing.T) {
	// TC-01-07: email sudah terdaftar -> 409, pengguna tetap di halaman register (tanpa token).
	svc := &specUserSvc{err: apperror.ErrDuplicate}
	e := specUserEngine(svc)

	w := specRequest(e, http.MethodPost, "/auth/register",
		`{"email":"ada@example.com","username":"ada","password":"rahasia123"}`)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "CONFLICT", specErrorCode(t, w))
	assert.NotContains(t, w.Body.String(), `"token"`)
}

func TestSpec_Register_MissingFieldsAreRejectedBeforeService(t *testing.T) {
	// TC-01-05 / 4.17: isian wajib tidak boleh kosong; service tidak dipanggil.
	cases := map[string]string{
		"missing email":    `{"username":"u","password":"p"}`,
		"missing username": `{"email":"e@x.com","password":"p"}`,
		"missing password": `{"email":"e@x.com","username":"u"}`,
		"html in username": `{"email":"e@x.com","username":"<b>u</b>","password":"p"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &specUserSvc{token: "x"}
			e := specUserEngine(svc)

			w := specRequest(e, http.MethodPost, "/auth/register", body)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, "BAD_REQUEST", specErrorCode(t, w))
			assert.Equal(t, 0, svc.calls)
		})
	}
}

func TestSpec_Login_EmptyCredentialsAreRejectedBeforeService(t *testing.T) {
	// TC-01-05: login dengan isian kosong -> ditolak, tidak ada permintaan login yang berhasil.
	cases := map[string]string{
		"empty identifier and password": `{"identifier":"","password":""}`,
		"missing password":              `{"identifier":"owner@example.com"}`,
		"missing identifier":            `{"password":"rahasia123"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &specUserSvc{token: "x"}
			e := specUserEngine(svc)

			w := specRequest(e, http.MethodPost, "/auth/login", body)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, 0, svc.calls)
		})
	}
}

func TestSpec_Login_WrongOrUnknownCredentialsGiveSameResponse(t *testing.T) {
	// TC-01-03 / TC-01-04: password salah dan akun tidak terdaftar menghasilkan respons 401 yang sama.
	svc := &specUserSvc{err: apperror.ErrNotFound}
	e := specUserEngine(svc)

	wrong := specRequest(e, http.MethodPost, "/auth/login", `{"identifier":"owner@example.com","password":"salah"}`)
	unknown := specRequest(e, http.MethodPost, "/auth/login", `{"identifier":"tidak@ada.com","password":"salah"}`)

	assert.Equal(t, http.StatusUnauthorized, wrong.Code)
	assert.Equal(t, "UNAUTHORIZED", specErrorCode(t, wrong))
	assert.Equal(t, wrong.Code, unknown.Code)
	assert.Equal(t, wrong.Body.String(), unknown.Body.String())
	assert.NotContains(t, wrong.Body.String(), `"token"`)
}

func TestSpec_Login_EmailOrUsernameBothReachService(t *testing.T) {
	// TC-01-01 / TC-01-02: login memakai email atau username sebagai identifier.
	svc := &specUserSvc{token: "tok"}
	e := specUserEngine(svc)

	w := specRequest(e, http.MethodPost, "/auth/login", `{"identifier":"owner","password":"rahasia123"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "owner", svc.gotID)
}
