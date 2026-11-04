package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"
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
}

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
	profile, err := h.service.Profile(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	respondData(c, http.StatusOK, dto.NewMeResponse(profile))
}
