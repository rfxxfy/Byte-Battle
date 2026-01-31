package server

import (
	"errors"
	"net/http"

	"bytebattle/internal/service"

	"github.com/labstack/echo/v4"
)

func (s *HTTPServer) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, err := c.Cookie(s.authCfg.CookieName)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{
				"status": "error",
				"error":  "unauthorized",
			})
		}

		user, err := s.auth.ValidateSession(c.Request().Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, service.ErrSessionNotFound) {
				return c.JSON(http.StatusUnauthorized, echo.Map{
					"status": "error",
					"error":  "unauthorized",
				})
			}
			c.Logger().Error("auth middleware error: ", err)
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"status": "error",
				"error":  "internal server error",
			})
		}

		c.Set("user", user)
		return next(c)
	}
}