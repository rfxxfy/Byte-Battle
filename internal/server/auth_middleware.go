package server

import (
	"context"
	"net/http"
	"strings"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/google/uuid"
)

type contextKey string

const (
	contextKeyUserID  contextKey = "userID"
	contextKeySession contextKey = "session"
)

func (s *HTTPServer) optionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := bearerToken(r); token != "" {
			if session, err := s.sessionService.ValidateToken(r.Context(), token); err == nil {
				s.sessionService.TryRefresh(r.Context(), session)
				ctx := context.WithValue(r.Context(), contextKeyUserID, session.UserID)
				ctx = context.WithValue(ctx, contextKeySession, session)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *HTTPServer) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := userIDFromContext(r.Context()); !ok {
			writeHTTPError(w, apierr.New(apierr.ErrInvalidToken, "unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *HTTPServer) strictAuthMiddleware(publicOps map[string]bool) api.StrictMiddlewareFunc {
	return func(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		if publicOps[operationID] {
			return f
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error) {
			if _, ok := userIDFromContext(ctx); !ok {
				return nil, apierr.New(apierr.ErrInvalidToken, "unauthorized")
			}
			return f(ctx, w, r, req)
		}
	}
}

func publicOpsFromSpec() map[string]bool {
	swagger, err := api.GetSwagger()
	if err != nil {
		panic("failed to load embedded swagger spec: " + err.Error())
	}
	public := map[string]bool{}
	for _, pathItem := range swagger.Paths.Map() {
		for _, op := range pathItem.Operations() {
			if op.Security != nil && len(*op.Security) == 0 {
				public[op.OperationID] = true
			}
		}
	}
	return public
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return after
	}
	return ""
}

func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(contextKeyUserID).(uuid.UUID)
	return id, ok
}

func sessionFromContext(ctx context.Context) (sqlcdb.Session, bool) {
	s, ok := ctx.Value(contextKeySession).(sqlcdb.Session)
	return s, ok
}
