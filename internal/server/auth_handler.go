package server

import (
	"errors"
	"net/http"
	"time"

	"bytebattle/internal/database/models"
	"bytebattle/internal/service"

	"github.com/labstack/echo/v4"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Rating    int       `json:"rating"`
	CreatedAt time.Time `json:"created_at"`
}

func userToResponse(u *models.User) UserResponse {
	var createdAt time.Time
	if u.CreatedAt.Valid {
		createdAt = u.CreatedAt.Time
	}

	rating := 0
	if u.Rating.Valid {
		rating = u.Rating.Int
	}

	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Rating:    rating,
		CreatedAt: createdAt,
	}
}

func (s *HTTPServer) handleAuthRegister(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": "error",
			"error":  "invalid request body",
		})
	}

	result, err := s.auth.Register(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return s.handleAuthError(c, err)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  "ok",
		"user_id": result.UserID,
	})
}

func (s *HTTPServer) handleAuthConfirm(c echo.Context) error {
	var req ConfirmRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": "error",
			"error":  "invalid request body",
		})
	}

	result, err := s.auth.ConfirmEmail(c.Request().Context(), req.Email, req.Code)
	if err != nil {
		return s.handleAuthError(c, err)
	}

	s.setSessionCookie(c, result.SessionToken)

	return c.JSON(http.StatusOK, echo.Map{
		"status":  "ok",
		"user_id": result.UserID,
	})
}

func (s *HTTPServer) handleAuthLogin(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": "error",
			"error":  "invalid request body",
		})
	}

	result, err := s.auth.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return s.handleAuthError(c, err)
	}

	s.setSessionCookie(c, result.SessionToken)

	return c.JSON(http.StatusOK, echo.Map{
		"status":  "ok",
		"user_id": result.UserID,
	})
}

func (s *HTTPServer) handleAuthLogout(c echo.Context) error {
	cookie, err := c.Cookie(s.authCfg.CookieName)
	if err == nil && cookie.Value != "" {
		_ = s.auth.Logout(c.Request().Context(), cookie.Value)
	}

	s.clearSessionCookie(c)

	return c.JSON(http.StatusOK, echo.Map{
		"status": "ok",
	})
}

func (s *HTTPServer) handleAuthMe(c echo.Context) error {
	user, ok := c.Get("user").(*models.User)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"status": "error",
			"error":  "unauthorized",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status": "ok",
		"user":   userToResponse(user),
	})
}

func (s *HTTPServer) setSessionCookie(c echo.Context, token string) {
	cookie := &http.Cookie{
		Name:     s.authCfg.CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(s.authCfg.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.authCfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	c.SetCookie(cookie)
}

func (s *HTTPServer) clearSessionCookie(c echo.Context) {
	cookie := &http.Cookie{
		Name:     s.authCfg.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.authCfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	c.SetCookie(cookie)
}

func (s *HTTPServer) handleAuthError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrPasswordTooShort):
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrEmailAlreadyExists):
		return c.JSON(http.StatusConflict, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrInvalidCredentials):
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrEmailNotVerified):
		return c.JSON(http.StatusForbidden, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrInvalidCode):
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrTooManyAttempts):
		return c.JSON(http.StatusTooManyRequests, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrUserNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	case errors.Is(err, service.ErrSessionNotFound):
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	default:
		c.Logger().Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": "error",
			"error":  "internal server error",
		})
	}
}