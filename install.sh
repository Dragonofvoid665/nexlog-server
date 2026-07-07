#!/bin/bash
# ============================================================
#  NexLog — Production install (Arch Linux + PostgreSQL)
#  Usage: sudo bash install.sh
# ============================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[OK]${NC}   $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERR]${NC}  $*"; exit 1; }

[[ $EUID -ne 0 ]] && error "Run as root: sudo bash install.sh"

DOMAIN="${DOMAIN:-}"
INSTALL_DIR="/opt/nexlog"
JWT_SECRET=$(openssl rand -hex 32)
PG_PASS=$(openssl rand -hex 20)

echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║   NexLog — Arch Linux Production Installer  ║"
echo "╚══════════════════════════════════════════════╝"
echo ""

# 1. Packages
info "Installing packages..."
pacman -Sy --noconfirm --needed \
    docker docker-compose nginx certbot certbot-nginx \
    ufw fail2ban curl wget unzip openssl
systemctl enable --now docker
success "Packages installed"

# 2. User
if ! id nexlog &>/dev/null; then
    useradd -r -s /usr/bin/nologin -d "$INSTALL_DIR" nexlog
    usermod -aG docker nexlog
fi

# 3. Dirs
mkdir -p "$INSTALL_DIR/public/uploads"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp -r "$SCRIPT_DIR"/* "$INSTALL_DIR/" 2>/dev/null || true
chown -R nexlog:nexlog "$INSTALL_DIR"

# 4. .env
cat > "$INSTALL_DIR/.env" << ENV
POSTGRES_PASSWORD=$PG_PASS
JWT_SECRET=$JWT_SECRET
PORT=3000
APP_ENV=production
PUBLIC_DIR=/app/public
MIGRATIONS_DIR=/app/migrations
DB_MAX_OPEN=50
DB_MAX_IDLE=25
RATE_LIMIT_PER_MIN=120
ENV
chmod 600 "$INSTALL_DIR/.env"
success ".env created (secrets auto-generated)"

# 5. Docker build & start
info "Building Docker image..."
cd "$INSTALL_DIR"
docker compose build --no-cache
docker compose up -d
sleep 5

if docker compose ps | grep -qE "running|Up|healthy"; then
    success "Containers running"
else
    error "Container failed — check: docker compose logs"
fi

# 6. Firewall
info "Configuring UFW..."
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
success "Firewall: 22, 80, 443 open"

# 7. Fail2ban
cat > /etc/fail2ban/jail.local << 'F2B'
[DEFAULT]
bantime  = 3600
findtime = 600
maxretry = 5
[sshd]
enabled = true
[nginx-http-auth]
enabled = true
F2B
systemctl enable --now fail2ban
success "Fail2ban active"

# 8. Nginx
mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled
grep -q "sites-enabled" /etc/nginx/nginx.conf || \
    sed -i '/http {/a\    include /etc/nginx/sites-enabled/*;' /etc/nginx/nginx.conf

NGINX_CONF="/etc/nginx/sites-available/nexlog"
cat > "$NGINX_CONF" << 'NGINX'
upstream nexlog_backend {
    server 127.0.0.1:3000;
    keepalive 64;
}
limit_req_zone $binary_remote_addr zone=api:10m rate=60r/m;
limit_conn_zone $binary_remote_addr zone=conn:10m;

server {
    listen 80;
    server_name _;
    client_max_body_size 10M;
    limit_conn conn 200;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript;
    gzip_min_length 1000;

    location /uploads/ {
        proxy_pass http://nexlog_backend;
        proxy_cache_valid 200 7d;
        add_header Cache-Control "public, max-age=604800, immutable";
    }
    location /api/ {
        limit_req zone=api burst=30 nodelay;
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
NGINX

ln -sf "$NGINX_CONF" /etc/nginx/sites-enabled/nexlog
nginx -t && systemctl enable --now nginx && systemctl reload nginx
success "Nginx configured"

# 9. SSL
if [[ -n "$DOMAIN" ]]; then
    info "Installing SSL for $DOMAIN..."
    sed -i "s/server_name _;/server_name $DOMAIN;/" "$NGINX_CONF"
    nginx -t && systemctl reload nginx
    certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "admin@$DOMAIN"
    success "SSL installed for $DOMAIN"
else
    warn "DOMAIN not set — SSL skipped. Run: DOMAIN=example.com bash install.sh"
fi

# 10. Systemd service
cat > /etc/systemd/system/nexlog.service << SVC
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
SVC
systemctl daemon-reload
systemctl enable nexlog
success "Systemd service: nexlog"

IP=$(ip route get 1 | awk '{print $7; exit}' 2>/dev/null || echo "YOUR_SERVER_IP")
echo ""
echo "╔══════════════════════════════════════════════════╗"
echo "║         ✅ Installation complete!                ║"
echo "╠══════════════════════════════════════════════════╣"
echo "║  🌐  Site:   http://$IP"
echo "║  🔧  Admin:  http://$IP/admin"
echo "║  🔑  Password: password  ← CHANGE IMMEDIATELY"
echo "╠══════════════════════════════════════════════════╣"
echo "║  Commands:"
echo "║  docker compose -f $INSTALL_DIR/docker-compose.yml logs -f"
echo "║  docker compose -f $INSTALL_DIR/docker-compose.yml restart"
echo "╚══════════════════════════════════════════════════╝"
echo ""
warn "⚠️  Change admin password at /admin → Security tab!"
