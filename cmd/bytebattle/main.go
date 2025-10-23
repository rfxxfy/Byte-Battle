package main

import (
	"fmt"
	"log"
	"net/http"

	"bytebattle/internal/config"
	"bytebattle/internal/database"
	"bytebattle/internal/routes"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Загружаем конфиг бд
	dbConfig := config.LoadDatabaseConfig()

	// Создаем клиент для работы с бд
	dbClient, err := database.NewClient(dbConfig)
	if err != nil {
		log.Fatalf("Не удалось создать клиент базы данных: %v", err)
	}
	defer dbClient.Close()

	// Создаем новый экземпляр Echo
	e := echo.New()

	// Подключаем промежуточные обработчики
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Регистрируем маршруты
	routes.RegisterRoutes(e, dbClient)

	// Маршрут по умолчанию
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Добро пожаловать в Byte-Battle!")
	})

	// Запускаем сервер
	fmt.Println("Запуск сервера Byte-Battle на порту 8080...")
	log.Fatal(e.Start(":8080"))
}
