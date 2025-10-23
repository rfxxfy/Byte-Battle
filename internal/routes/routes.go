package routes

import (
	"bytebattle/internal/database"
	"bytebattle/internal/handlers"

	"github.com/labstack/echo/v4"
)

// RegisterRoutes настраивает все маршруты для приложения
func RegisterRoutes(e *echo.Echo, dbClient *database.Client) {
	// Конечная точка Hello World
	e.GET("/internal/hello_world", func(c echo.Context) error {
		return handlers.HelloWorld(c, dbClient)
	})
}
