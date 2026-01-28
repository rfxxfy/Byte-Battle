package server

import (
	"bytebattle/internal/service"
	"context"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type HTTPServer struct {
	echo           *echo.Echo
	users          *service.UserService
	duelService    *service.DuelService
	sessionService *service.SessionService
}

func NewHTTPServer(users *service.UserService, duelService *service.DuelService, sessionService *service.SessionService) *HTTPServer {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	s := &HTTPServer{
		echo:           e,
		users:          users,
		duelService:    duelService,
		sessionService: sessionService,
	}

	s.registerRoutes()
	return s
}

func (s *HTTPServer) Run(addr string) error {
	return s.echo.Start(addr)
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}
