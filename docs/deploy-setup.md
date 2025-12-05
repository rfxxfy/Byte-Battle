# Настройка деплоя на VPS

Инструкция по первоначальной настройке VPS и GitHub-репозитория для деплоя Byte-Battle.

## 1. Подготовка VPS

### Установить Docker

```bash
# Docker Engine
curl -fsSL https://get.docker.com | sh

# Добавить пользователя в группу docker (чтобы не нужен sudo)
sudo usermod -aG docker $USER
# Перелогиниться чтобы группа применилась
```

### Создать директорию проекта

```bash
sudo mkdir -p /opt/bytebattle
sudo chown $USER:$USER /opt/bytebattle
```

### Скопировать compose-файл и создать .env

```bash
cd /opt/bytebattle

# Скопировать docker-compose.prod.yaml из репозитория
# (можно через scp или просто создать файл)

# Создать .env с продовыми значениями
cat > .env << 'EOF'
# Database
DB_USER=bytebattle
DB_PASSWORD=СГЕНЕРИРОВАТЬ_НАДЁЖНЫЙ_ПАРОЛЬ
DB_NAME=bytebattle

# HTTP
HTTP_PORT=8080

# Задаётся автоматически при деплое (не менять вручную)
IMAGE=ghcr.io/OWNER/byte-battle
EOF
```

> ⚠️ Пароль `DB_PASSWORD` должен быть уникальным. Сгенерировать: `openssl rand -base64 24`

## 2. Настройка GitHub

### Создать Personal Access Token (CR_PAT)

Нужен чтобы VPS мог pull'ить Docker-образ из GitHub Container Registry.

1. GitHub → Settings → Developer Settings → Personal Access Tokens → Tokens (classic)
2. Generate new token
3. Scope: **`read:packages`**
4. Скопировать токен

### Добавить Secrets в репозиторий

GitHub → Repo → Settings → Secrets and variables → Actions → New repository secret

| Secret | Значение |
|---|---|
| `SSH_HOST` | IP-адрес VPS |
| `SSH_USER` | Пользователь на VPS (напр. `deploy`) |
| `SSH_PRIVATE_KEY` | Приватный SSH-ключ (содержимое `~/.ssh/id_ed25519`) |
| `CR_PAT` | Personal Access Token из шага выше |

### Настроить SSH-доступ

```bash
# На локальной машине — сгенерировать ключ для деплоя
ssh-keygen -t ed25519 -f ~/.ssh/bytebattle-deploy -C "github-actions-deploy"

# Скопировать публичный ключ на VPS
ssh-copy-id -i ~/.ssh/bytebattle-deploy.pub USER@VPS_IP

# Содержимое приватного ключа → в secret SSH_PRIVATE_KEY
cat ~/.ssh/bytebattle-deploy
```

## 3. Первый деплой

1. Убедиться, что VPS подготовлен (Docker, `/opt/bytebattle/.env`, compose-файл)
2. GitHub → Actions → **Deploy** → **Run workflow**
3. Дождаться выполнения

### Проверить что работает

```bash
# На VPS
docker compose -f docker-compose.prod.yaml ps
docker compose -f docker-compose.prod.yaml logs app

# С любой машины
curl http://VPS_IP:8080/
```

## 4. Как устроен деплой

```
Run workflow (кнопка в GitHub)
    │
    ▼
GitHub Actions runner:
    1. Собирает Docker-образ из Dockerfile
    2. Пушит образ в ghcr.io
    3. Заходит на VPS по SSH
    │
    ▼
VPS:
    4. docker login ghcr.io (через CR_PAT)
    5. docker compose pull (стягивает новый образ)
    6. migrate up (миграции внутри контейнера)
    7. docker compose up -d (перезапускает приложение)
```

## Полезные команды на VPS

```bash
cd /opt/bytebattle

# Статус
docker compose -f docker-compose.prod.yaml ps

# Логи приложения
docker compose -f docker-compose.prod.yaml logs -f app

# Логи БД
docker compose -f docker-compose.prod.yaml logs -f postgres

# Перезапуск
docker compose -f docker-compose.prod.yaml restart app

# Остановить всё
docker compose -f docker-compose.prod.yaml down

# Остановить всё + удалить данные БД
docker compose -f docker-compose.prod.yaml down -v
```
