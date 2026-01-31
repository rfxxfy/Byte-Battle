# Byte-Battle

Byte-Battle — это онлайн-платформа, где программисты могут доказывать, кто быстрее решает алгоритмические задачи. В
дуэльном режиме двум участникам выдаётся одна задача — кто первым её решает, получает очки и сразу получает следующую.
Побеждает быстрый и сообразительный.

Для тех, кто хочет соло-практику, есть режим на время: сам выбираешь старт и набор задач, а система фиксирует твой
результат для сравнения с другими. Такой формат помогает готовиться к собеседованиям — учит думать под давлением времени
и стресса.

Byte-Battle — это не просто кодинг, а азартные и полезные баттлы, идеально подходящие для учёбы, соревнований с друзьями
или быстрого роста как инженера.

## Цели проекта Byte-Battle:

- Реализовать онлайн-дуэли для решения алгоритмических задач на скорость.
- Ввести режим индивидуальных тайм-трейлов для самостоятельной практики.
- Автоматизировать проверку решений и начисление очков.
- Создать систему рейтинга пользователей.
- Сделать удобный и простой интерфейс.

## Задачи проекта Byte-Battle:

- Разработать серверную часть с автоматической проверкой решений.
- Реализовать фронтенд для дуэлей и соло-режима.
- Сделать систему регистрации, профилей и рейтинга.
- Настроить хранение и выборку задач.
- Обеспечить быстрый матчмейкинг для дуэлей.
- Внедрить таймеры и учёт времени прохождения.
- Организовать логику начисления очков и отображения результатов.

## Начало работы

Эти инструкции помогут вам запустить копию проекта на вашей локальной машине.

### Предварительные требования

- Go 1.25 или выше
- Git
- Docker и Docker Compose

### Установка

1. Клонируйте репозиторий:
   ```bash
   git clone <repository-url>
   cd Byte-Battle
   ```

2. Создайте файлы конфигурации из примеров:
   ```bash
   cp .env.example .env
   cp sqlboiler.toml.example sqlboiler.toml
   ```

3. Установите зависимости:
   ```bash
   make tidy
   ```

4. Поднимите базу данных:
   ```bash
   docker compose up -d
   ```

5. Примените миграции:
   ```bash
   make migrate-up
   ```

6. Сгенерируйте модели SQLBoiler:
   ```bash
   make generate
   ```

7. Запустите сервер:
   ```bash
   make run
   ```

8. Проверьте, что сервер работает:
   ```bash
   curl -X GET localhost:8080/internal/hello_world
   ```

## Команды Makefile

### База данных и миграции

```bash
make migrate-up        # Применить все миграции
make migrate-down      # Откатить последнюю миграцию
make migrate-version   # Показать текущую версию миграции
make migrate-create NAME=add_field  # Создать новую миграцию
```

### Тестирование

```bash
make test-prepare      # Подготовить окружение для тестов
make test              # Запустить тесты
```

### Сборка и запуск

```bash
make run               # Запустить сервер
make build             # Собрать бинарник
make generate          # Сгенерировать модели SQLBoiler
```

## Конечные точки API

- `GET /` - Приветственное сообщение
- `GET /internal/hello_world` - JSON ответ Hello World с интеграцией базы данных

## Конфигурация

Приложение использует переменные окружения из файлов `.env` и `sqlboiler.toml`. 
Скопируйте примеры и настройте под себя (см. шаг 2 в разделе "Установка").

Основные переменные `.env`:

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `HTTP_HOST` | Хост сервера | `localhost` |
| `HTTP_PORT` | Порт сервера | `8080` |
| `DB_HOST` | Хост базы данных | `localhost` |
| `DB_PORT` | Порт базы данных | `5432` |
| `DB_USER` | Пользователь БД | `bytebattle` |
| `DB_PASSWORD` | Пароль БД | `bytebattle` |
| `DB_NAME` | Имя базы данных | `bytebattle` |

## Схема базы данных

Приложение использует следующие таблицы базы данных:

- `users`: Хранит информацию о пользователях (id, username, email, password_hash, rating, created_at, updated_at)
- `problems`: Содержит задачи по программированию (id, title, description, difficulty, time_limit, memory_limit,
  created_at, updated_at)
- `duels`: Отслеживает дуэли между пользователями (id, player1_id, player2_id, problem_id, winner_id, status,
  started_at, completed_at, created_at, updated_at)
- `solutions`: Хранит отправленные решения (id, user_id, problem_id, duel_id, code, language, status, execution_time,
  memory_used, created_at, updated_at)

- `sessions`: Хранит сессии пользователей (id, user_id, token, expires_at, created_at, updated_at)

Миграции находятся в папке `migrations/` и управляются через [golang-migrate](https://github.com/golang-migrate/migrate).

## Built With

- [Go](https://golang.org/) - Programming language
- [Echo](https://echo.labstack.com/) - Web framework
- [PostgreSQL](https://www.postgresql.org/) - Database
- [SQLBoiler](https://github.com/aarondl/sqlboiler) - ORM
- [golang-migrate](https://github.com/golang-migrate/migrate) - Database migrations
