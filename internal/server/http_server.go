package server

import (
	"context"

	"bytebattle/internal/config"
	"bytebattle/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type HTTPServer struct {
	echo        *echo.Echo
	users       *service.UserService
	duelService *service.DuelService

	auth    *service.AuthService
	authCfg *config.AuthConfig
}

func NewHTTPServer(
	users *service.UserService,
	duelService *service.DuelService,
	auth *service.AuthService,
	authCfg *config.AuthConfig,
) *HTTPServer {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	s := &HTTPServer{
		echo:        e,
		users:       users,
		duelService: duelService,
		auth:        auth,
		authCfg:     authCfg,
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
