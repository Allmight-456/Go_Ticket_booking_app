package handler

import (
	"errors"
	"net/http"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/Allmight-456/ticketflow/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decode(w, r, &req) {
		return
	}

	user, token, err := h.auth.Register(r.Context(), req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDuplicateEmail):
			renderError(w, http.StatusConflict, err.Error())
		default:
			renderError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	render(w, http.StatusCreated, map[string]any{
		"user":  user,
		"token": token,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decode(w, r, &req) {
		return
	}

	user, token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			renderError(w, http.StatusUnauthorized, "invalid email or password")
		case errors.Is(err, domain.ErrUserInactive):
			renderError(w, http.StatusForbidden, err.Error())
		default:
			renderError(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	render(w, http.StatusOK, map[string]any{
		"user":  user,
		"token": token,
	})
}
