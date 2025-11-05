package server

func (s *HTTPServer) registerRoutes() {
	s.echo.GET("/", s.handleRoot)
	s.echo.GET("/internal/hello_world", s.handleHello)

	s.echo.POST("/duels", s.handleCreateDuel)
	s.echo.GET("/duels/:id", s.handleGetDuel)
	s.echo.GET("/duels", s.handleListDuels)
	s.echo.POST("/duels/:id/start", s.handleStartDuel)
	s.echo.POST("/duels/:id/complete", s.handleCompleteDuel)
	s.echo.DELETE("/duels/:id", s.handleDeleteDuel)
}
