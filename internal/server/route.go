package server

func (s *HTTPServer) registerRoutes() {
	s.echo.GET("/", s.handleRoot)
	s.echo.GET("/internal/hello_world", s.handleHello)

	// Duel routes
	s.echo.POST("/duels", s.handleCreateDuel)
	s.echo.GET("/duels/:id", s.handleGetDuel)
	s.echo.GET("/duels", s.handleListDuels)
	s.echo.POST("/duels/:id/start", s.handleStartDuel)
	s.echo.POST("/duels/:id/complete", s.handleCompleteDuel)
	s.echo.DELETE("/duels/:id", s.handleDeleteDuel)

	// Session routes
	s.echo.POST("/sessions", s.handleCreateSession)
	s.echo.GET("/sessions/:id", s.handleGetSession)
	s.echo.GET("/sessions/validate", s.handleValidateSession)
	s.echo.POST("/sessions/:id/refresh", s.handleRefreshSession)
	s.echo.DELETE("/sessions/:id", s.handleEndSession)
	s.echo.GET("/users/:user_id/sessions", s.handleGetUserSessions)
	s.echo.DELETE("/users/:user_id/sessions", s.handleEndAllUserSessions)
	s.echo.POST("/sessions/cleanup", s.handleCleanupExpiredSessions)
}
