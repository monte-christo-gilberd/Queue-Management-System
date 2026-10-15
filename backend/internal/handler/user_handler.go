package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"

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
