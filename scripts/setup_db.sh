#!/bin/bash

# Конфигурация базы данных
DB_NAME="bytebattle"
DB_USER="bytebattle"
DB_PASS="bytebattle"

# Запуск миграций
echo "Запуск миграций..."
go run github.com/pressly/goose/v3/cmd/goose -dir schema postgres "user=$DB_USER dbname=$DB_NAME password=$DB_PASS sslmode=disable" up

echo "Настройка базы данных завершена!"