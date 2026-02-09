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

## Начало работы

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

3. Поднимите базу данных и примените миграции:
   ```bash
   docker compose up -d
   make migrate-up
   ```

4. Сгенерируйте модели и запустите сервер:
   ```bash
   make generate
   make run
   ```

5. Проверьте, что сервер работает:
   ```bash
   curl http://localhost:8080/internal/hello_world
   ```

## Команды Makefile

```bash
make run                        # Запустить сервер
make build                      # Собрать бинарник
make generate                   # Сгенерировать модели SQLBoiler
make test                       # Запустить тесты
make migrate-up                 # Применить все миграции
make migrate-down               # Откатить последнюю миграцию
make migrate-version            # Текущая версия миграции
make migrate-create NAME=...    # Создать новую миграцию
```

## Структура проекта

```
cmd/bytebattle/          # Точка входа приложения
internal/
  config/                # Конфигурация (env-переменные)
  database/              # Репозитории и подключение к БД
    models/              # Сгенерированные модели SQLBoiler (в .gitignore)
  server/                # HTTP-сервер, роуты, хендлеры
  service/               # Бизнес-логика
migrations/              # SQL-миграции (golang-migrate)
scripts/                 # Вспомогательные скрипты
.env.example             # Пример переменных окружения
sqlboiler.toml.example   # Пример конфига SQLBoiler
```

## Конфигурация

Приложение настраивается через переменные окружения.
Все доступные переменные и их значения по умолчанию описаны в `.env.example`.

`sqlboiler.toml` используется для генерации моделей — формат описан в `sqlboiler.toml.example`.

Оба файла (`.env`, `sqlboiler.toml`) содержат креды и **не должны коммититься** в репозиторий.

## Built With

- [Go](https://golang.org/) - Programming language
- [Echo](https://echo.labstack.com/) - Web framework
- [PostgreSQL](https://www.postgresql.org/) - Database
- [SQLBoiler](https://github.com/aarondl/sqlboiler) - ORM
- [golang-migrate](https://github.com/golang-migrate/migrate) - Database migrations
