#!/bin/bash
set -e

# AetherStream VPS Deployment Script
# Usage: curl -fsSL https://raw.githubusercontent.com/mymada/aetherstream/master/deploy/vps/install.sh | bash

AETHERSTREAM_VERSION="${AETHERSTREAM_VERSION:-latest}"
DOMAIN="${DOMAIN:-localhost}"
EMAIL="${EMAIL:-admin@localhost}"
DATA_DIR="${DATA_DIR:-/opt/aetherstream/data}"
MEDIA_DIR="${MEDIA_DIR:-/opt/aetherstream/media}"

echo "=== AetherStream VPS Installer ==="
echo "Version: $AETHERSTREAM_VERSION"
echo "Domain: $DOMAIN"
echo "Data: $DATA_DIR"
echo "Media: $MEDIA_DIR"
echo ""

# Update system
echo "[1/7] Updating system..."
apt-get update -qq
apt-get install -y -qq curl wget git nginx certbot python3-certbot-nginx ufw

# Install Docker
echo "[2/7] Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    usermod -aG docker $USER
fi

# Create directories
echo "[3/7] Creating directories..."
mkdir -p "$DATA_DIR" "$MEDIA_DIR"

# Clone repo
echo "[4/7] Cloning AetherStream..."
if [ -d /opt/aetherstream/app ]; then
    cd /opt/aetherstream/app
    git pull origin master
else
    git clone https://github.com/mymada/aetherstream.git /opt/aetherstream/app
    cd /opt/aetherstream/app
fi

# Build and start
echo "[5/7] Building Docker image..."
docker build -t aetherstream:$AETHERSTREAM_VERSION .

echo "[6/7] Starting services..."
export DATA_DIR MEDIA_DIR
export AETHERSTREAM_AUTH_SECRET=$(openssl rand -hex 32)
docker-compose up -d

# Configure nginx + SSL
echo "[7/7] Configuring nginx + SSL..."
cat > /etc/nginx/sites-available/aetherstream << 'EOF'
server {
    listen 80;
    server_name _;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
    
    location /ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
EOF

ln -sf /etc/nginx/sites-available/aetherstream /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl restart nginx

# SSL (if domain is not localhost)
if [ "$DOMAIN" != "localhost" ]; then
    certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$EMAIL"
fi

# Firewall
echo "Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 8097/tcp
ufw --force enable

echo ""
echo "=== AetherStream installed ==="
echo "URL: http://${DOMAIN}"
echo "Data: ${DATA_DIR}"
echo "Media: ${MEDIA_DIR}"
echo ""
echo "Next steps:"
echo "1. Place media files in ${MEDIA_DIR}"
echo "2. Login with admin/admin123"
echo "3. Change password immediately"
