package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"bytebattle/internal/service"
)

func (s *HTTPServer) handleSendCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.entrance.SendCode(r.Context(), req.Email); err != nil {
		s.handleEntranceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleVerifyCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Code == "" {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	token, err := s.entrance.VerifyCode(r.Context(), req.Email, req.Code)
	if err != nil {
		s.handleEntranceError(w, err)
		return
	}

	setSessionCookie(w, token, s.cfg.CookieName, s.cfg.CookieSecure, s.cfg.SessionTTL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(s.cfg.CookieName)
	if err == nil && cookie.Value != "" {
		session, err := s.sessionService.ValidateToken(r.Context(), cookie.Value)
		if err == nil {
			_ = s.sessionService.EndSession(r.Context(), int(session.ID))
		}
	}

	clearSessionCookie(w, s.cfg.CookieName, s.cfg.CookieSecure)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "user_id": userID})
}

func (s *HTTPServer) handleEntranceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		writeAuthError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidCode):
		writeAuthError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrTooManyAttempts):
		writeAuthError(w, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		writeAuthError(w, http.StatusNotFound, err.Error())
	default:
		writeAuthError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": message})
}
