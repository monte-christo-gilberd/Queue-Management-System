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

type UserService interface {
	Register(
		ctx context.Context,
		email,
		username,
		password string,
	) (string, error)
	Login(
		ctx context.Context,
		email,
		password string,
	) (string, error)
	Profile(
		ctx context.Context,
		userID int64,
	) (model.Profile, error)
	UpdateUsername(
		ctx context.Context,
		userID int64,
		username string,
	) (model.User, error)
	SetAvatar(
		ctx context.Context,
		userID int64,
		upload io.Reader,
	) error
	Avatar(
		ctx context.Context,
		userID int64,
	) (model.Avatar, error)
	DeleteAvatar(
		ctx context.Context,
		userID int64,
	) error
}

// avatarFormOverhead: ruang untuk bagian multipart di luar berkas (batas, header field).
const avatarFormOverhead = 64 << 10

type UserHandler struct {
	service UserService
}

func NewUserHandler(
	service UserService,
) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

func (h *UserHandler) Register(
	c *gin.Context,
) {

	ctx := c.Request.Context()
	var request dto.RegisterRequest

	err := c.BindJSON(&request)
	if err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid request body",
		)
		return
	}

	token, err := h.service.Register(
		ctx,
		request.Email,
		request.Username,
		request.Password,
	)
	if err != nil {
		if errors.Is(err, apperror.ErrDuplicate) {
			respondError(
				c,
				http.StatusConflict,
				"CONFLICT",
				"conflict",
			)
			return
		}
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	respondData(
		c,
		http.StatusOK,
		dto.AuthResponse{
			Token: token,
		},
	)

}

func (h *UserHandler) Login(
	c *gin.Context,
) {

	ctx := c.Request.Context()
	var request dto.LoginRequest

	err := c.BindJSON(&request)
	if err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid request body",
		)
		return
	}

	identifier := request.Login()
	if identifier == "" {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"identifier (email or username) is required",
		)
		return
	}

	token, err := h.service.Login(
		ctx,
		identifier,
		request.Password,
	)
	if err != nil {
		respondError(
			c,
			http.StatusUnauthorized,
			"UNAUTHORIZED",
			"unauthorized user",
		)
		return
	}

	respondData(
		c,
		http.StatusOK,
		dto.AuthResponse{
			Token: token,
		},
	)
}

// Me: GET /api/v1/users/me — profil user yang login (butuh token).
func (h *UserHandler) Me(
	c *gin.Context,
) {
	profile, err := h.service.Profile(
		c.Request.Context(),
		c.GetInt64("user_id"),
	)
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	respondData(c, http.StatusOK, dto.NewMeResponse(profile))
}

// UpdateMe: PUT /api/v1/users/me — ganti username user yang login. Email tidak bisa diubah di sini.
func (h *UserHandler) UpdateMe(
	c *gin.Context,
) {
	var request dto.UpdateMeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	userID := c.GetInt64("user_id")
	if _, err := h.service.UpdateUsername(c.Request.Context(), userID, request.Username); err != nil {
		switch {
		case errors.Is(err, apperror.ErrInvalidUsername):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}

	// Baca ulang lewat Profile supaya bentuk respons sama persis dengan GET /users/me.
	h.respondProfile(c, userID)
}

// UploadAvatar: PUT /api/v1/users/me/avatar — multipart, field "file". Mengganti foto profil.
func (h *UserHandler) UploadAvatar(
	c *gin.Context,
) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, imageproc.MaxUploadBytes+avatarFormOverhead)
	header, err := c.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			respondError(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", apperror.ErrImageTooLarge.Error())
			return
		}
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "field \"file\" is required")
		return
	}
	file, err := header.Open()
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "cannot read the uploaded file")
		return
	}
	defer file.Close()

	userID := c.GetInt64("user_id")
	if err := h.service.SetAvatar(c.Request.Context(), userID, file); err != nil {
		switch {
		case errors.Is(err, apperror.ErrImageTooLarge):
			respondError(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", err.Error())
		case errors.Is(err, apperror.ErrUnsupportedImage):
			respondError(c, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", err.Error())
		case errors.Is(err, apperror.ErrInvalidImage):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	h.respondProfile(c, userID)
}

// GetAvatar: GET /api/v1/users/me/avatar — foto profil milik sendiri. Hanya pemilik yang bisa membacanya
// (butuh token); tidak ada URL publik.
func (h *UserHandler) GetAvatar(
	c *gin.Context,
) {
	avatar, err := h.service.Avatar(c.Request.Context(), c.GetInt64("user_id"))
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "no profile photo")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	etag := fmt.Sprintf(`"%x"`, avatar.UpdatedAt.UnixNano())
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, avatar.ContentType, avatar.Data)
}

// DeleteAvatar: DELETE /api/v1/users/me/avatar — menghapus foto profil. Idempoten (204 walau belum ada).
func (h *UserHandler) DeleteAvatar(
	c *gin.Context,
) {
	if err := h.service.DeleteAvatar(c.Request.Context(), c.GetInt64("user_id")); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}

// respondProfile membaca ulang profil dan menjawab dengan bentuk yang sama dengan GET /users/me.
func (h *UserHandler) respondProfile(c *gin.Context, userID int64) {
	profile, err := h.service.Profile(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	respondData(c, http.StatusOK, dto.NewMeResponse(profile))
}
