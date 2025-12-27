package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	"bytebattle/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type HTTPServer struct {
	users          *service.UserService
	gameService    *service.GameService
	sessionService *service.SessionService
}

func New(users *service.UserService, gameService *service.GameService, sessionService *service.SessionService) http.Handler {
	s := &HTTPServer{
		users:          users,
		gameService:    gameService,
		sessionService: sessionService,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", s.handleRoot)
	r.Get("/internal/hello_world", s.handleHello)

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
