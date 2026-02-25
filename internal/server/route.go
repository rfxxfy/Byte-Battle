package server

func (s *HTTPServer) registerRoutes() {
	s.echo.GET("/", s.handleRoot)
	s.echo.GET("/internal/hello_world", s.handleHello)

	// Game routes
	s.echo.POST("/games", s.handleCreateGame)
	s.echo.GET("/games/:id", s.handleGetGame)
	s.echo.GET("/games", s.handleListGames)
	s.echo.POST("/games/:id/start", s.handleStartGame)
	s.echo.POST("/games/:id/complete", s.handleCompleteGame)
	s.echo.POST("/games/:id/cancel", s.handleCancelGame)
	s.echo.DELETE("/games/:id", s.handleDeleteGame)

	// Session routes
	s.echo.POST("/sessions", s.handleCreateSession)
	s.echo.GET("/sessions/:id", s.handleGetSession)
	s.echo.GET("/sessions/validate", s.handleValidateSession)
	s.echo.POST("/sessions/:id/refresh", s.handleRefreshSession)
	s.echo.DELETE("/sessions/:id", s.handleEndSession)
	s.echo.GET("/users/:user_id/sessions", s.handleGetUserSessions)
	s.echo.DELETE("/users/:user_id/sessions", s.handleEndAllUserSessions)
	s.echo.POST("/sessions/cleanup", s.handleCleanupExpiredSessions)

	// Execution routes
	s.echo.POST("/execute", s.handleExecute)
}
