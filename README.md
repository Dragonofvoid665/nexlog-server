# NexLog — Go Production Server

Высокопроизводительный сервер на Go для логистического CMS-сайта.  
Рассчитан на **10 000 одновременных пользователей** на сервере с **2 CPU / 2 GB RAM**.

## Стек
- **Go 1.22** — HTTP-сервер (net/http, горутины)
- **SQLite + WAL** — single-writer, многопоточное чтение
- **In-memory cache** — 30-секундное кэширование API-ответов
- **Gzip** — встроенная компрессия ответов
- **JWT** — авторизация админки
- **bcrypt** — хэширование паролей
- **Nginx** — reverse proxy + SSL + rate limiting
- **Docker** — контейнеризация

---

## Быстрый старт (Docker)

```bash
# 1. Клонируй или распакуй проект
cd nexlog

# 2. Создай .env
cp .env.example .env
# Отредактируй JWT_SECRET на случайную строку:
# openssl rand -hex 32

# 3. Запусти
docker compose up -d

# 4. Открой браузер
# Сайт:  http://localhost:3000
# Админ: http://localhost:3000/admin   (пароль: password)
```

---

## Установка на продакшен-сервер (Ubuntu 22.04 / 24.04)

```bash
# Загрузи архив на сервер
scp nexlog.zip root@your-server-ip:/opt/

# Зайди на сервер
ssh root@your-server-ip

# Распакуй
cd /opt && unzip nexlog.zip && cd nexlog

# Запусти автоматическую установку
sudo bash install.sh

# Если есть домен (установит SSL автоматически):
DOMAIN=yourdomain.com sudo bash install.sh
```

---

## Команды управления

```bash
# Статус контейнера
docker compose ps

# Логи в реальном времени
docker compose logs -f

# Перезапуск
docker compose restart

# Остановка
docker compose down

# Обновление (после изменений)
docker compose down
docker compose build --no-cache
docker compose up -d

# Бэкап базы данных
docker run --rm -v nexlog_nexlog_data:/data alpine \
  tar czf - /data > backup_$(date +%Y%m%d).tar.gz

# Восстановление бэкапа
docker run --rm -v nexlog_nexlog_data:/data -i alpine \
  tar xzf - < backup_20250101.tar.gz
```

---

## Переменные окружения

| Переменная   | По умолчанию | Описание |
|-------------|-------------|----------|
| `PORT`      | `3000`      | Порт сервера |
| `JWT_SECRET`| (генерируется) | Секрет для JWT токенов |
| `DATA_DIR`  | `./data`    | Директория с БД |
| `PUBLIC_DIR`| `./public`  | Статические файлы |

---

## Производительность

| Метрика | Значение |
|---------|---------|
| Пиковые RPS | ~8 000–12 000 |
| Кэш API | 30 сек (in-memory) |
| SQLite WAL | параллельное чтение |
| Горутины Go | не ограничены |
| Rate limit | 100 req / 15 мин / IP |
| Gzip | включён автоматически |

---

## Структура проекта

```
nexlog/
├── cmd/server/
│   ├── main.go          # entrypoint, HTTP server
│   └── middleware.go    # rate limit, gzip, security headers
├── internal/
│   ├── db/db.go         # SQLite: схема, модели, запросы
│   ├── cache/cache.go   # in-memory TTL cache
│   ├── handlers/        # HTTP handlers (API + uploads)
│   └── middleware/      # JWT auth middleware
├── public/
│   ├── index.html       # SPA (фронтенд)
│   ├── admin.html       # Админ-панель
│   └── uploads/         # Загруженные изображения
├── data/
│   └── nexlog.db        # SQLite база данных
├── vendor/              # Go зависимости
├── Dockerfile
├── docker-compose.yml
├── nginx.conf           # Nginx config для ручной настройки
└── install.sh           # Автоматическая установка на Ubuntu
```

---

## Admin по умолчанию

- URL: `/admin`
- Пароль: `password`
- **Сразу смени пароль в Admin → Security!**
