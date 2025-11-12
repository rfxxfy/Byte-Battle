package handlers

import (
	"context"
	"net/http"

	"bytebattle/internal/database"

	"github.com/labstack/echo/v4"
)

// HelloWorld обрабатывает конечную точку /internal/hello_world
func HelloWorld(c echo.Context, dbClient *database.Client) error {
	// Получаем пример пользователя для демонстрации использования базы данных
	ctx := context.Background()
	user, err := dbClient.GetUserByUsername(ctx, "testuser")
	if err != nil {
		// Проверяем, связана ли ошибка с тем, что пользователь не существует
		if err.Error() == "не удалось получить пользователя по имени: models: unable to select from users: sql: no rows in result set" {
			// Если пользователь не существует, создаем его
			user, err = dbClient.CreateUser(ctx, "testuser", "test@example.com", "hashedpassword")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"message": "Не удалось создать пользователя",
					"status":  "error",
					"error":   err.Error(),
				})
			}
		} else {
			// Если это другая ошибка, возвращаем ее
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": "Не удалось получить пользователя",
				"status":  "error",
				"error":   err.Error(),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Привет, Byte-Battle!",
		"status":  "success",
		"user":    user,
	})
}
