package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/imageproc"
	"qms/internal/model"

	"github.com/gin-gonic/gin"
)

// UserService adalah logika profil user, preferensi, dan foto profil (dipenuhi oleh service).
type UserService interface {
	// Profile mengambil profil user.
	Profile(
		ctx context.Context,
		userID int64,
	) (model.Profile, error)
	// UpdateUsername mengganti username user.
	UpdateUsername(
		ctx context.Context,
		userID int64,
		username string,
	) (model.User, error)
	// SetEmailNotifications menyalakan atau mematikan email notifikasi tiket.
	SetEmailNotifications(
		ctx context.Context,
		userID int64,
		enabled bool,
	) error
	// SetAvatar memproses dan menyimpan foto profil dari upload.
	SetAvatar(
		ctx context.Context,
		userID int64,
		upload io.Reader,
	) error
	// Avatar mengambil foto profil user.
	Avatar(
		ctx context.Context,
		userID int64,
	) (model.Avatar, error)
	// DeleteAvatar menghapus foto profil user.
	DeleteAvatar(
		ctx context.Context,
		userID int64,
	) error
}

// avatarFormOverhead: ruang untuk bagian multipart di luar berkas (batas, header field).
const avatarFormOverhead = 64 << 10

// UserHandler menangani rute akun user: profil, preferensi, dan foto profil. Daftar dan login ada di auth-service.
type UserHandler struct {
	// service berisi logika akun user.
	service UserService
}

// NewUserHandler membuat UserHandler dengan service yang diberikan.
func NewUserHandler(
	service UserService,
) *UserHandler {
	// isi handler dengan service
	return &UserHandler{
		service: service,
	}
}

// Me: GET /api/v1/users/me — profil user yang login (butuh token).
func (h *UserHandler) Me(
	c *gin.Context,
) {
	// ambil profil user yang login
	profile, err := h.service.Profile(
		c.Request.Context(),
		c.GetInt64("user_id"),
	)
	// akun tidak ditemukan: sesi dianggap tidak sah
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}
	// error lain: 500 tanpa detail
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	// kirim profil
	respondData(c, http.StatusOK, dto.NewMeResponse(profile))
}

// UpdateMe: PUT /api/v1/users/me — ganti username user yang login. Email tidak bisa diubah di sini.
func (h *UserHandler) UpdateMe(
	c *gin.Context,
) {
	// baca body JSON yang berisi username baru; gagal berarti format request salah
	var request dto.UpdateMeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// ganti username; error diterjemahkan ke respons HTTP
	userID := c.GetInt64("user_id")
	if _, err := h.service.UpdateUsername(c.Request.Context(), userID, request.Username); err != nil {
		switch {
		// username tidak memenuhi panjang 1 sampai 50 karakter: 400
		case errors.Is(err, apperror.ErrInvalidUsername):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		// akun tidak ditemukan: sesi dianggap tidak sah
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		// error lain: 500 tanpa detail
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}

	// Baca ulang lewat Profile supaya bentuk respons sama persis dengan GET /users/me.
	h.respondProfile(c, userID)
}

// UpdatePreferences: PUT /api/v1/users/me/preferences — sakelar email notifikasi tiket.
func (h *UserHandler) UpdatePreferences(
	c *gin.Context,
) {
	// baca body JSON; gagal, atau field email_notifications tidak ada, berarti request tidak lengkap
	var request dto.UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.EmailNotifications == nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// simpan preferensi untuk user yang login; error diterjemahkan ke respons HTTP
	userID := c.GetInt64("user_id")
	if err := h.service.SetEmailNotifications(c.Request.Context(), userID, *request.EmailNotifications); err != nil {
		// akun tidak ditemukan: sesi dianggap tidak sah
		if errors.Is(err, apperror.ErrNotFound) {
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
			return
		}
		// error lain: 500 tanpa detail
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// berhasil: kirim profil terbaru
	h.respondProfile(c, userID)
}

// UploadAvatar: PUT /api/v1/users/me/avatar — multipart, field "file". Mengganti foto profil.
func (h *UserHandler) UploadAvatar(
	c *gin.Context,
) {
	// batasi ukuran body agar unggahan terlalu besar berhenti sebelum memenuhi memori
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, imageproc.MaxUploadBytes+avatarFormOverhead)
	// ambil berkas dari field "file"; gagal bisa karena terlalu besar atau field tidak ada
	header, err := c.FormFile("file")
	if err != nil {
		// body melewati batas: 413
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			respondError(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", apperror.ErrImageTooLarge.Error())
			return
		}
		// field "file" tidak ada: 400
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "field \"file\" is required")
		return
	}
	// buka berkas untuk dibaca; gagal berarti berkas tidak bisa dibaca
	file, err := header.Open()
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "cannot read the uploaded file")
		return
	}
	// tutup berkas saat fungsi selesai
	defer file.Close()

	// proses dan simpan foto profil; error diterjemahkan ke respons HTTP
	userID := c.GetInt64("user_id")
	if err := h.service.SetAvatar(c.Request.Context(), userID, file); err != nil {
		switch {
		// foto terlalu besar setelah diproses: 413
		case errors.Is(err, apperror.ErrImageTooLarge):
			respondError(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", err.Error())
		// format bukan JPEG, PNG, atau WebP: 415
		case errors.Is(err, apperror.ErrUnsupportedImage):
			respondError(c, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", err.Error())
		// berkas rusak atau tidak valid: 400
		case errors.Is(err, apperror.ErrInvalidImage):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		// error lain: 500 tanpa detail
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	// berhasil: kirim profil terbaru
	h.respondProfile(c, userID)
}

// GetAvatar: GET /api/v1/users/me/avatar — foto profil milik sendiri. Hanya pemilik yang bisa membacanya
// (butuh token); tidak ada URL publik.
func (h *UserHandler) GetAvatar(
	c *gin.Context,
) {
	// ambil foto profil user yang login
	avatar, err := h.service.Avatar(c.Request.Context(), c.GetInt64("user_id"))
	// belum punya foto profil: 404
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "no profile photo")
		return
	}
	// error lain: 500 tanpa detail
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	// ETag dari waktu terakhir foto diubah, dipakai browser untuk cache
	etag := fmt.Sprintf(`"%x"`, avatar.UpdatedAt.UnixNano())
	c.Header("ETag", etag)
	// hanya boleh di-cache oleh browser pemilik, dan harus dicek ulang
	c.Header("Cache-Control", "private, no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	// foto belum berubah sejak terakhir diambil: kirim 304 tanpa isi
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	// kirim isi foto dengan tipe konten yang tersimpan
	c.Data(http.StatusOK, avatar.ContentType, avatar.Data)
}

// DeleteAvatar: DELETE /api/v1/users/me/avatar — menghapus foto profil. Idempoten (204 walau belum ada).
func (h *UserHandler) DeleteAvatar(
	c *gin.Context,
) {
	// hapus foto profil user yang login; error berarti error server
	if err := h.service.DeleteAvatar(c.Request.Context(), c.GetInt64("user_id")); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// berhasil dihapus (atau memang belum ada): 204 tanpa isi
	c.Status(http.StatusNoContent)
}

// respondProfile membaca ulang profil dan menjawab dengan bentuk yang sama dengan GET /users/me.
func (h *UserHandler) respondProfile(c *gin.Context, userID int64) {
	// baca profil terbaru; error berarti error server
	profile, err := h.service.Profile(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// kirim profil dengan bentuk yang sama seperti GET /users/me
	respondData(c, http.StatusOK, dto.NewMeResponse(profile))
}
