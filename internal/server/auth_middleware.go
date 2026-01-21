package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const contextKeyUserID contextKey = "userID"

func (s *HTTPServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "unauthorized"})
			return
		}

		session, err := s.sessionService.ValidateToken(r.Context(), token)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "unauthorized"})
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, int(session.UserID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return after
	}
	return ""
}

func userIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(contextKeyUserID).(int)
	return id, ok
}
