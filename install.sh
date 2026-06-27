#!/bin/bash
# ============================================================
#  NexLog — установка на Arch Linux
#  Использование: sudo bash install.sh
# ============================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[OK]${NC}  $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

[[ $EUID -ne 0 ]] && error "Запусти от root: sudo bash install.sh"

DOMAIN="${DOMAIN:-}"
INSTALL_DIR="/opt/nexlog"
JWT_SECRET=$(openssl rand -hex 32)
APP_USER="nexlog"

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║   NexLog Go Production Installer        ║"
echo "║   Arch Linux                             ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# ─── 1. Пакеты через pacman ──────────────────────────────
info "Обновление системы и установка пакетов..."
pacman -Sy --noconfirm --needed \
    docker docker-compose \
    nginx certbot certbot-nginx \
    ufw fail2ban \
    curl wget unzip openssl
success "Пакеты установлены"

# ─── 2. Docker ────────────────────────────────────────────
info "Запуск Docker..."
systemctl enable --now docker
success "Docker запущен"

# ─── 3. Пользователь ─────────────────────────────────────
if ! id "$APP_USER" &>/dev/null; then
    info "Создание системного пользователя: $APP_USER"
    useradd -r -s /usr/bin/nologin -d "$INSTALL_DIR" "$APP_USER"
    usermod -aG docker "$APP_USER"
fi

# ─── 4. Директории ───────────────────────────────────────
info "Создание директорий..."
mkdir -p "$INSTALL_DIR"/{data,public/uploads,logs}

# Копируем файлы проекта из текущей папки
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp -r "$SCRIPT_DIR"/* "$INSTALL_DIR/" 2>/dev/null || true
chown -R "$APP_USER:$APP_USER" "$INSTALL_DIR"

# ─── 5. .env файл ─────────────────────────────────────────
info "Создание .env..."
cat > "$INSTALL_DIR/.env" << EOF
JWT_SECRET=$JWT_SECRET
PORT=3000
DATA_DIR=/app/data
PUBLIC_DIR=/app/public
EOF
chmod 600 "$INSTALL_DIR/.env"
success ".env создан (JWT secret сгенерирован)"

# ─── 6. Сборка Docker образа ─────────────────────────────
info "Сборка Docker образа (1-3 мин)..."
cd "$INSTALL_DIR"
docker compose build --no-cache
success "Образ собран"

# ─── 7. Запуск ────────────────────────────────────────────
info "Запуск контейнера..."
docker compose up -d
sleep 3

if docker compose ps | grep -qE "running|Up"; then
    success "Контейнер запущен!"
else
    error "Контейнер не запустился. Смотри: docker compose logs"
fi

# ─── 8. Firewall (ufw) ───────────────────────────────────
info "Настройка UFW..."
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
success "Firewall настроен (22, 80, 443)"

# ─── 9. Fail2ban ─────────────────────────────────────────
info "Настройка fail2ban..."
cat > /etc/fail2ban/jail.local << 'EOF'
[DEFAULT]
bantime  = 3600
findtime = 600
maxretry = 5

[sshd]
enabled = true

[nginx-http-auth]
enabled = true
EOF
systemctl enable --now fail2ban
success "Fail2ban настроен"

# ─── 10. Nginx ───────────────────────────────────────────
info "Настройка Nginx..."
mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled

# Arch nginx не включает sites-enabled по умолчанию — добавляем
if ! grep -q "sites-enabled" /etc/nginx/nginx.conf; then
    sed -i '/http {/a\    include /etc/nginx/sites-enabled/*;' /etc/nginx/nginx.conf
fi

cat > /etc/nginx/sites-available/nexlog << 'NGINXEOF'
upstream nexlog_backend {
    server 127.0.0.1:3000;
    keepalive 64;
}

limit_req_zone $binary_remote_addr zone=api:10m rate=30r/m;
limit_conn_zone $binary_remote_addr zone=conn:10m;

server {
    listen 80;
    server_name _;

    client_max_body_size 10M;
    limit_conn conn 100;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript;
    gzip_min_length 1000;

    location /api/ {
        limit_req zone=api burst=20 nodelay;
        proxy_pass http://nexlog_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
        proxy_connect_timeout 5s;
    }

    location / {
        proxy_pass http://nexlog_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    add_header X-Frame-Options SAMEORIGIN always;
    add_header X-Content-Type-Options nosniff always;
}
NGINXEOF

ln -sf /etc/nginx/sites-available/nexlog /etc/nginx/sites-enabled/nexlog

nginx -t && systemctl enable --now nginx && systemctl reload nginx
success "Nginx настроен"

# ─── 11. SSL (если задан домен) ──────────────────────────
if [[ -n "$DOMAIN" ]]; then
    info "Установка SSL для $DOMAIN..."
    sed -i "s/server_name _;/server_name $DOMAIN;/" /etc/nginx/sites-available/nexlog
    nginx -t && systemctl reload nginx
    certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "admin@$DOMAIN"
    success "SSL установлен для $DOMAIN"
else
    warn "DOMAIN не задан — SSL пропущен. Позже: DOMAIN=example.com bash install.sh"
fi

# ─── 12. Systemd сервис ───────────────────────────────────
info "Создание systemd сервиса..."
cat > /etc/systemd/system/nexlog.service << EOF
[Unit]
Description=NexLog Logistics CMS
After=docker.service network-online.target
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=$INSTALL_DIR
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=120

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable nexlog
success "Systemd сервис создан (автозапуск при перезагрузке)"

# ─── Итог ─────────────────────────────────────────────────
IP=$(ip route get 1 | awk '{print $7; exit}' 2>/dev/null || echo "your-server-ip")
echo ""
echo "╔══════════════════════════════════════════════════╗"
echo "║          ✅ Установка завершена!                 ║"
echo "╠══════════════════════════════════════════════════╣"
echo "║  🌐 Сайт:    http://$IP"
echo "║  🔧 Админ:   http://$IP/admin"
echo "║  🔑 Пароль:  password  (смени немедленно!)"
echo "╠══════════════════════════════════════════════════╣"
echo "║  Команды:"
echo "║  docker compose -f $INSTALL_DIR/docker-compose.yml logs -f"
echo "║  docker compose -f $INSTALL_DIR/docker-compose.yml restart"
echo "╚══════════════════════════════════════════════════╝"
echo ""
warn "⚠️  Смени пароль в /admin → Security сразу после входа!"
