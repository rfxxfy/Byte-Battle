# Byte-Battle

Byte-Battle — это онлайн-платформа, где программисты могут доказывать, кто быстрее решает алгоритмические задачи. В
игровом режиме от 2 до 50 участников получают одну задачу — кто первым её решает, получает очки и сразу получает следующую.
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

2. Создайте файлы конфигурации из примеров и заполните своими значениями:
   ```bash
   cp .env.example .env
   cp sqlboiler.toml.example sqlboiler.toml
   ```

3. Поднимите базу данных:
   ```bash
   docker compose up -d --wait
   ```

4. Примените миграции:
   ```bash
   make migrate-up
   ```

5. Сгенерируйте модели SQLBoiler:
   ```bash
   make generate
   ```

## Запуск

```bash
make run      # запустить сервер
make build    # собрать бинарник в bin/bytebattle
```

Проверьте, что сервер работает:
```bash
curl http://localhost:8080/
```

## Тесты

```bash
make test
```

`make test` автоматически ждёт готовности БД, применяет миграции и запускает все тесты, включая интеграционные.
Требует запущенного Docker Compose и `.env`.

## Команды Makefile

```bash
make run                        # Запустить сервер
make build                      # Собрать бинарник
make test                       # Запустить все тесты (unit + интеграционные)
make generate                   # Сгенерировать модели SQLBoiler
make clean-models               # Удалить сгенерированные модели
make lint                       # Запустить линтер
make migrate-up                 # Применить все миграции
make migrate-down               # Откатить последнюю миграцию
make migrate-down-all           # Откатить все миграции
make migrate-drop               # Удалить все таблицы
make migrate-version            # Текущая версия миграции
make migrate-create NAME=...    # Создать новую миграцию
make migrate-force VERSION=...  # Принудительно задать версию миграции
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
