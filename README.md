# Byte-Battle

Byte-Battle — это онлайн-платформа, где программисты могут доказывать, кто быстрее решает алгоритмические задачи. В игровом режиме от 2 участников получают матч из нескольких задач (до 20) — кто первым решает текущую задачу, продвигает игру к следующей.
Побеждает быстрый и сообразительный.

Для тех, кто хочет соло-практику, есть режим на время: сам выбираешь старт и набор задач, а система фиксирует твой
результат для сравнения с другими. Такой формат помогает готовиться к собеседованиям — учит думать под давлением времени
и стресса.

Byte-Battle — это не просто кодинг, а азартные и полезные баттлы, идеально подходящие для учёбы, соревнований с друзьями
или быстрого роста как инженера.

## Цели проекта Byte-Battle:

- Реализовать онлайн-игры для решения алгоритмических задач на скорость (2–50 игроков).
- Ввести режим индивидуальных тайм-трейлов для самостоятельной практики.
- Автоматизировать проверку решений и начисление очков.
- Создать систему рейтинга пользователей.
- Сделать удобный и простой интерфейс.

## Предварительные требования

- Go 1.25 или выше
- Docker и Docker Compose

## Настройка

1. Клонируйте репозиторий:
   ```bash
   git clone <repository-url>
   cd Byte-Battle
   ```

2. Поднимите инфраструктуру (PostgreSQL + Swagger UI):
   ```bash
   docker compose up -d --wait
   ```

3. Примените миграции:
   ```bash
   make migrate-up
   ```

По умолчанию приложение подключается к `postgres://bytebattle:bytebattle@localhost:5432/bytebattle?sslmode=disable` — это значение `docker-compose.yaml`. Для другого окружения задайте переменную `DB_DSN` (или создайте `.env` на основе `.env.example`).

## Email-коды входа (Resend)

По умолчанию (без `RESEND_API_KEY`) коды подтверждения не отправляются во внешнюю почту и печатаются в логах сервера.

Чтобы включить реальную отправку email:

1. Создайте аккаунт в Resend и получите API ключ.
2. Добавьте в `.env`:
   - `RESEND_API_KEY=re_...`
   - `FROM_EMAIL=Byte Battle <noreply@yourdomain.com>`
3. Убедитесь, что домен отправителя верифицирован в Resend.

После этого `SendCode` будет отправлять код на указанный email через Resend API.

## Запуск

```bash
make run      # запустить сервер
make build    # собрать бинарник в bin/bytebattle
```

Проверьте, что сервер работает:
```bash
curl http://localhost:8080/
```

Swagger UI доступен на [http://localhost:8081](http://localhost:8081).

## Тесты

```bash
make test        # unit + e2e
make test-unit   # только unit
make test-e2e    # только e2e (testcontainers, требует Docker)
```

E2e-тесты автоматически поднимают PostgreSQL через testcontainers — Docker Compose для тестов не нужен.

## Команды Makefile

```bash
make run                        # Запустить сервер
make build                      # Собрать бинарник
make test                       # Запустить все тесты (unit + e2e)
make test-unit                  # Запустить unit-тесты
make test-e2e                   # Запустить e2e-тесты (testcontainers)
make generate                   # Сгенерировать API + sqlc код
make generate-api               # Сгенерировать API из openapi.yaml
make generate-sqlc              # Сгенерировать sqlc код из SQL-запросов
make clean                      # Удалить сгенерированные файлы и бинарники
make fmt                        # Отформатировать код
make lint                       # Запустить линтер
make migrate-up                 # Применить все миграции
make migrate-rollback           # Откатить последнюю миграцию
make migrate-down               # Откатить все миграции
make migrate-drop               # Удалить все таблицы
make migrate-version            # Текущая версия миграции
make migrate-create NAME=...    # Создать новую миграцию
make migrate-force VERSION=...  # Принудительно задать версию миграции
```

## Структура проекта

```
cmd/bytebattle/          # Точка входа приложения
internal/
  api/                   # Сгенерированные типы и интерфейсы из openapi.yaml
  apierr/                # Типизированные ошибки API
  config/                # Конфигурация (env-переменные)
  db/
    queries/             # SQL-запросы для sqlc
    sqlc/                # Сгенерированный sqlc-код
  migrations/            # SQL-миграции (embed в binary, golang-migrate)
  problems/              # Загрузка задач и тест-кейсов из файловой системы
  server/                # HTTP-сервер, роуты, хендлеры
  service/               # Бизнес-логика
api/                     # OpenAPI-спецификация
problems/                # Файловый банк задач (problem.json + tests/*.in/*.out)
.env.example             # Пример переменных окружения
sqlc.yaml                # Конфиг генератора sqlc
```

## Конфигурация

Приложение настраивается через переменные окружения:

| Переменная | По умолчанию | Описание |
|---|---|---|
| `DB_DSN` | `postgres://bytebattle:bytebattle@localhost:5432/bytebattle?sslmode=disable` | DSN подключения к PostgreSQL |
| `HTTP_HOST` | `0.0.0.0` | Хост HTTP-сервера |
| `HTTP_PORT` | `8080` | Порт HTTP-сервера |
| `PROBLEMS_DIR` | `./problems` | Путь к каталогу задач |
| `RESEND_API_KEY` | `` | API ключ Resend; если пустой, используется dev-mailer (код в логах) |
| `FROM_EMAIL` | `noreply@bytebattle.dev` | Email отправителя для писем с кодом |

Для dev-окружения значения по умолчанию совпадают с `docker-compose.yaml` — никаких `.env` не нужно.
Для задания кастомных значений создайте `.env` на основе `.env.example` — он загружается автоматически.

## Built With

- [Go](https://golang.org/) - Programming language
- [chi](https://github.com/go-chi/chi) - HTTP router
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) - OpenAPI code generation
- [PostgreSQL](https://www.postgresql.org/) - Database
- [pgx](https://github.com/jackc/pgx) - PostgreSQL driver
- [sqlc](https://sqlc.dev/) - SQL-to-Go code generation
- [golang-migrate](https://github.com/golang-migrate/migrate) - Database migrations
