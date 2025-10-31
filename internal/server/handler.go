package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (s *HTTPServer) handleHello(c echo.Context) error {
	user, err := s.users.GetOrCreateTestUser(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  "success",
		"message": "Привет, Byte-Battle!",
		"user":    user,
	})
}

func (s *HTTPServer) handleRoot(c echo.Context) error {
	return c.String(http.StatusOK, "Добро пожаловать в Byte-Battle!")
}
