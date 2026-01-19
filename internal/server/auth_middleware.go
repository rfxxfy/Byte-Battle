package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type contextKey string

const contextKeyUserID contextKey = "userID"

func (s *HTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cfg.CookieName)
		if err != nil || cookie.Value == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "unauthorized"})
			return
		}

		session, err := s.sessionService.ValidateToken(r.Context(), cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "unauthorized"})
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, int(session.UserID))
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func userIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(contextKeyUserID).(int)
	return id, ok
}

func setSessionCookie(w http.ResponseWriter, token, name string, secure bool, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
