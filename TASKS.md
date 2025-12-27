# Byte-Battle — Backend Tasks

## Задача 1 — Рефактор БД-слоя: sqlboiler → sqlc + pgx

**Почему:** sqlboiler требует живой БД для кодогенерации (неудобно в CI), скрывает SQL за ORM-стилем, плохо дружит с кастомными запросами. sqlc — пишешь SQL, получаешь типобезопасные Go-функции. pgx быстрее и активнее поддерживается чем `lib/pq`.

**Что сделать:**

- Добавить `github.com/jackc/pgx/v5` и `github.com/sqlc-dev/sqlc`, убрать sqlboiler и `lib/pq`
- Создать `sqlc.yaml` — указать схему (существующие миграции) и папку с запросами
- Создать `internal/db/queries/` — переписать все запросы как `.sql` файлы с аннотациями sqlc:
  - `games.sql` — все запросы из `game_repository.go`
  - `sessions.sql` — из `session_repository.go`
  - `users.sql` — из `user_repository.go`
- Запустить `sqlc generate` → получить `internal/db/sqlc/`
- Переписать репозитории под сгенерированные функции (убрать `internal/database/models/`)
- Обновить `database/postgres.go` — заменить `sql.Open` на `pgxpool.New`
- Обновить тесты репозиториев
- Обновить `Makefile`: добавить `make generate` (вызывает sqlc), убрать sqlboiler таргеты

**Зависимости:** независима, можно делать сейчас.

---

## Задача 2 — Рефактор HTTP-слоя: echo → chi + oapi-codegen (Swagger)

**Почему:** oapi-codegen — contract-first подход: пишешь `openapi.yaml`, получаешь типы, серверный интерфейс и embedded swagger spec. chi — stdlib-совместимый роутер, любая `net/http` middleware работает без обёрток. Фронт получает живую документацию.

**Что сделать:**

- Добавить `github.com/go-chi/chi/v5` и `github.com/oapi-codegen/oapi-codegen/v2`, убрать echo
- Написать `api/openapi.yaml` — описать все текущие эндпоинты (`/games`, `/sessions`), форматы запросов/ответов, коды ошибок
- Настроить `api/oapi-codegen.yaml`:
  ```yaml
  package: api
  generate:
    chi-server: true
    strict-server: true
    models: true
    embedded-spec: true
  output: ../internal/api/api.gen.go
  ```
- Запустить `oapi-codegen` → получить `internal/api/api.gen.go`
- Переписать `internal/server/`:
  - `handler.go` — реализовать сгенерированный `StrictServerInterface`
  - `http_server.go` — заменить echo на chi роутер
  - `route.go` — подключить сгенерированные wrapper'ы
- Добавить `swaggerui` или `redoc` эндпоинт для документации (`GET /docs`)
- Обновить `Makefile`: `make generate` вызывает и sqlc, и oapi-codegen

**Зависимости:** независима, можно делать параллельно с задачей 1. Лучше делать после задачи 1 чтобы в openapi.yaml сразу описать финальные типы.

---

## Задача 3 — Унифицированная обработка ошибок

**Почему:** сейчас ошибки возвращаются как строка `err.Error()`. Фронту нужен машиночитаемый код ошибки чтобы показывать правильные сообщения.

**Что сделать:**

- Создать `internal/apierr/errors.go`:
  ```go
  type AppError struct {
      Code    string `json:"error_code"`
      Message string `json:"message"`
  }

  const (
      ErrGameNotFound    = "GAME_NOT_FOUND"
      ErrNotEnoughPlayers = "NOT_ENOUGH_PLAYERS"
      // ...
  )

  func New(code, message string) *AppError
  func HTTPStatus(code string) int  // маппинг кода → HTTP статус
  ```
- Обновить хендлеры — использовать `apierr` вместо `jsonError(c, 404, err)`
- Описать все коды ошибок в `openapi.yaml` (делать вместе с задачей 2)

**Зависимости:** независима. Лучше делать вместе с задачей 2.

---

## Задача 4 — Migrations as code + embed.FS

**Почему:** сейчас миграции запускаются через `make migrate` (shell). Если встроить их в бинарник через `embed.FS`, `RunMigrations` можно вызывать из кода — это нужно для e2e тестов (задача 5) и чистого старта приложения.

**Что сделать:**

- Создать `internal/migrations/embed.go`:
  ```go
  //go:embed *.sql
  var FS embed.FS
  ```
- Перенести `migrations/*.sql` в `internal/migrations/`
- Написать `func RunMigrations(pool *pgxpool.Pool) error` в `internal/app/` или `internal/migrations/` — использует `iofs` + `golang-migrate`
- Вызывать `RunMigrations` из `main.go` при старте вместо отдельного make-таргета

**Зависимости:** лучше делать после задачи 1 (pgxpool нужен для сигнатуры функции).

---

## Задача 5 — E2E тесты через testcontainers

**Почему:** сейчас e2e тесты запускаются через `.sh` скрипт — не code, нельзя гонять в CI без доп. настройки. testcontainers поднимает Postgres-контейнер прямо из Go-теста.

**Что сделать:**

- Добавить `github.com/testcontainers/testcontainers-go` и `testcontainers-go/modules/postgres`
- Создать `internal/e2e/e2e_test.go` с `TestMain`:
  ```go
  func TestMain(m *testing.M) {
      pgCtr, _ := tcpostgres.Run(ctx, "postgres:16-alpine", ...)
      dsn, _   := pgCtr.ConnectionString(ctx, "sslmode=disable")
      pool, _  := dbpkg.NewPool(ctx, dsn)
      migrations.RunMigrations(pool)
      testSrv  = httptest.NewServer(app.NewRouter(pool, cfg))
      m.Run()
      pgCtr.Terminate(ctx)
  }
  ```
- Написать тесты на основные сценарии:
  - Создание и жизненный цикл игры
  - Подключение к игре по WS (когда будет)
  - Отправка кода и получение результата
- Удалить `.sh` скрипт
- Добавить `make test-e2e` в Makefile
- Обновить CI workflow: разделить `test` (unit, без Docker) и `test-e2e` (с Docker)

**Зависимости:** требует задачи 4 (RunMigrations).

---

## Задача 6 — Хранение задач в файлах

**Почему:** хранить условия задач и тесты в БД неудобно — сложно добавлять/редактировать задачи, большие входные данные раздувают БД. Файловый подход стандартен для тест-систем и более универсален.

**Структура на диске:**
```
problems/
  001-two-sum/
    problem.json        # title, difficulty, time_limit_ms, memory_limit_mb
    tests/
      01.in
      01.out
      02.in
      02.out
  002-fizzbuzz/
    problem.json
    tests/
      01.in
      01.out
```

**Что сделать:**

- Определить формат `problem.json`:
  ```json
  {
    "id": "001-two-sum",
    "title": "Two Sum",
    "description": "...",
    "difficulty": "easy",
    "time_limit_ms": 2000,
    "memory_limit_mb": 256
  }
  ```
- Написать `internal/problems/loader.go` — загружает задачи из директории при старте:
  ```go
  type Problem struct {
      Meta      ProblemMeta
      TestCases []TestCase  // { Input, ExpectedOutput }
  }

  type Loader struct { problems map[string]*Problem }

  func NewLoader(dir string) (*Loader, error)
  func (l *Loader) Get(id string) (*Problem, error)
  func (l *Loader) List() []*Problem
  ```
- Убрать таблицу `problems` из схемы (или оставить пустой если нужна FK для `games.problem_id`)
- Удалить `test_cases` из планируемых миграций (таблица так и не была создана — хорошо)
- Настроить mount директории `problems/` в `docker-compose.yml`
- Передать путь к директории через конфиг (`PROBLEMS_DIR`)
- Написать 2-3 реальные задачи для тестирования

**Зависимости:** независима. Влияет на задачу 7 (game session использует Loader вместо ProblemRepository).

---

## Задача 7 — WebSocket: игровая сессия

**Зависит от:** executor PR #15, задача 6 (файловое хранилище задач).

**Что сделать:**

### 7.1 Добавить зависимость
```
go get github.com/gorilla/websocket
go mod vendor
```

### 7.2 GameHub (`internal/hub/hub.go`)

Менеджер активных WS-соединений. Один на всё приложение, запускается горутиной.

```go
type Client struct {
    GameID int
    UserID int
    conn   *websocket.Conn
    send   chan []byte
}

type GameHub struct {
    rooms      map[int]map[*Client]struct{}
    register   chan *Client
    unregister chan *Client
    broadcast  chan broadcastMsg
}

func (h *GameHub) Run()
func (h *GameHub) Register(c *Client)
func (h *GameHub) Unregister(c *Client)
func (h *GameHub) Broadcast(gameID int, msg []byte)
func (h *GameHub) ClientCount(gameID int) int
```

### 7.3 Протокол (`internal/hub/message.go`)

```
client → server:  submit_code  { code, language }
server → client:  code_result  { passed, stdout, stderr, time_ms }
server → all:     game_started { problem: { title, description, time_limit_ms } }
server → all:     game_over    { winner_id }
server → all:     player_joined { user_id }
```

### 7.4 WS-хендлер (`internal/server/ws_handler.go`)

- Маршрут: `GET /games/{id}/ws`
- Проверить что игра в статусе `pending` или `active`
- Upgrade HTTP → WebSocket
- Запустить `readPump` (читает сообщения → `GameSessionService.HandleMessage`) и `writePump` (пишет из `client.send`)
- При разрыве: `GameSessionService.HandleDisconnect`

### 7.5 GameSessionService (`internal/service/game_session_service.go`)

```go
type GameSessionService struct {
    hub         *hub.GameHub
    gameService *GameService
    execService *ExecutionService
    problems    *problems.Loader
}
```

**HandleConnect(gameID, userID, conn):**
1. Зарегистрировать клиента в hub
2. Broadcast `player_joined`
3. Если все участники подключились → `StartGame()` → broadcast `game_started` с задачей

**HandleSubmit(client, payload):**
1. Загрузить задачу и тест-кейсы через `problems.Loader`
2. Для каждого тест-кейса: `ExecutionService.Execute(code, language, input, timeLimit)`
3. Сравнить stdout с expected_output (trim whitespace)
4. Если все прошли: `CompleteGame(winnerID)` → broadcast `game_over` → закрыть соединения комнаты
5. Если упал: отправить `code_result{passed:false}` только этому игроку

**HandleDisconnect(client):**
1. Unregister из hub
2. Если игра `active` → `CancelGame()`
3. Broadcast о завершении игры

---

## Отложено

**Регистрация / авторизация (PR #9):** нужно командное решение по архитектуре. Варианты:
- email + password (текущий PR, но есть замечания по архитектуре)
- email-only magic code: вводишь email → получаешь код → сервер смотрит есть ли юзер → регистрирует или логинит

Пока не блокирует остальные задачи.
