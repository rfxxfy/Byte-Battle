package server

func (s *HTTPServer) registerRoutes() {
	s.echo.GET("/", s.handleRoot)
	s.echo.GET("/internal/hello_world", s.handleHello)
}
