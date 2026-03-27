package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	"bytebattle/internal/service"
	"bytebattle/internal/ws"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	gorillaws "github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

func allowedOrigins() []string {
	v := os.Getenv("ALLOWED_ORIGINS")
	if v == "" {
		return nil // dev mode — allow all
	}
	return strings.Split(v, ",")
}

func newUpgrader() gorillaws.Upgrader {
	origins := allowedOrigins()
	return gorillaws.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if origins == nil {
				return true
			}
			origin := r.Header.Get("Origin")
			for _, o := range origins {
				if o == origin {
					return true
				}
			}
			return false
		},
	}
}

var upgrader = newUpgrader() //nolint:gochecknoglobals // package-level for performance, initialized once at startup

type HTTPServer struct {
	pool              *pgxpool.Pool
	users             *service.UserService
	gameService       *service.GameService
	problemService    *service.ProblemService
	sessionService    *service.SessionService
	executionService  *service.ExecutionService
	submissionService *service.SubmissionService
	hub               *ws.Hub
	entrance          service.EntranceService
}

func New(
	pool *pgxpool.Pool,
	users *service.UserService,
	gameService *service.GameService,
	problemService *service.ProblemService,
	sessionService *service.SessionService,
	executionService *service.ExecutionService,
	submissionService *service.SubmissionService,
	hub *ws.Hub,
	entrance service.EntranceService,
) http.Handler {
	s := &HTTPServer{
		pool:              pool,
		users:             users,
		gameService:       gameService,
		problemService:    problemService,
		sessionService:    sessionService,
		executionService:  executionService,
		submissionService: submissionService,
		hub:               hub,
		entrance:          entrance,
	}

	origins := allowedOrigins()
	corsAllowed := origins
	if corsAllowed == nil {
		corsAllowed = []string{"*"}
	}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: corsAllowed,
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         300,
	}))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", s.handleHealth)
	r.Get("/", s.handleRoot)
	r.Get("/internal/hello_world", s.handleHello)
	r.Get("/api/games/{id}/ws", s.handleGameWS)

	strictOpts := api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestErrorHandler,
		ResponseErrorHandlerFunc: responseErrorHandler,
	}
	publicOps := publicOpsFromSpec()
	strictHandler := api.NewStrictHandlerWithOptions(s, []api.StrictMiddlewareFunc{s.strictAuthMiddleware(publicOps)}, strictOpts)
	api.HandlerFromMuxWithBaseURL(strictHandler, r, "/api")

	return r
}

func requestErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{
		ErrorCode: apierr.ErrValidation,
		Message:   err.Error(),
	})
}

func writeHTTPError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var ae *apierr.AppError
	if errors.As(err, &ae) {
		w.WriteHeader(ae.HTTPStatus)
		_ = json.NewEncoder(w).Encode(api.ErrorResponse{
			ErrorCode: ae.ErrorCode,
			Message:   ae.Message,
		})
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{
		ErrorCode: apierr.ErrInternal,
		Message:   "internal error",
	})
}

func responseErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	var ae *apierr.AppError
	if errors.As(err, &ae) {
		w.WriteHeader(ae.HTTPStatus)
		_ = json.NewEncoder(w).Encode(api.ErrorResponse{
			ErrorCode: ae.ErrorCode,
			Message:   ae.Message,
		})
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{
		ErrorCode: apierr.ErrInternal,
		Message:   err.Error(),
	})
}
