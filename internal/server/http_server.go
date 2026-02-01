package server

import (
	"bytebattle/internal/service"
	"context"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type HTTPServer struct {
	echo        *echo.Echo
	users            *service.UserService
	duelService      *service.DuelService
	executionService *service.ExecutionService
}

func NewHTTPServer(users *service.UserService, duelService *service.DuelService, executionService *service.ExecutionService) *HTTPServer {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	s := &HTTPServer{
		echo:             e,
		users:            users,
		duelService:      duelService,
		executionService: executionService,
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
