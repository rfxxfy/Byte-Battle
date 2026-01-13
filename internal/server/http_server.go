package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	"bytebattle/internal/service"
	"bytebattle/internal/ws"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	gorillaws "github.com/gorilla/websocket"
)

var upgrader = gorillaws.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true }, //nolint:gosec // origin check intentionally disabled; restrict in production
}

type HTTPServer struct {
	users            *service.UserService
	gameService      *service.GameService
	sessionService   *service.SessionService
	executionService *service.ExecutionService
	hub              *ws.Hub
}

func New(users *service.UserService, gameService *service.GameService, sessionService *service.SessionService, executionService *service.ExecutionService, hub *ws.Hub) http.Handler {
	s := &HTTPServer{
		users:            users,
		gameService:      gameService,
		sessionService:   sessionService,
		executionService: executionService,
		hub:              hub,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", s.handleRoot)
	r.Get("/internal/hello_world", s.handleHello)
	r.Post("/execute", s.handleExecute)
	r.Get("/games/{id}/ws", s.handleGameWS)

	strictOpts := api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestErrorHandler,
		ResponseErrorHandlerFunc: responseErrorHandler,
	}
	strictHandler := api.NewStrictHandlerWithOptions(s, nil, strictOpts)
	api.HandlerFromMux(strictHandler, r)

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
	var ae *apierr.AppError
	if errors.As(err, &ae) {
		http.Error(w, ae.Message, ae.HTTPStatus)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
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
