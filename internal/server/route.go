package server

func (s *HTTPServer) registerRoutes() {
	s.echo.GET("/", s.handleRoot)
	s.echo.GET("/internal/hello_world", s.handleHello)

	s.echo.POST("/auth/register", s.handleAuthRegister)
	s.echo.POST("/auth/confirm", s.handleAuthConfirm)
	s.echo.POST("/auth/login", s.handleAuthLogin)
	s.echo.POST("/auth/logout", s.handleAuthLogout)

	s.echo.GET("/auth/me", s.handleAuthMe, s.authMiddleware)

	duels := s.echo.Group("/duels", s.authMiddleware)
	duels.POST("", s.handleCreateDuel)
	duels.GET("/:id", s.handleGetDuel)
	duels.GET("", s.handleListDuels)
	duels.POST("/:id/start", s.handleStartDuel)
	duels.POST("/:id/complete", s.handleCompleteDuel)
	duels.DELETE("/:id", s.handleDeleteDuel)
}