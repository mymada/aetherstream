# AetherStream Deployment Guide

This guide covers deploying AetherStream with Docker, systemd, and Kubernetes.

---

## Prerequisites

| Dependency | Minimum Version | Notes |
|------------|-----------------|-------|
| Go         | 1.25            | CGO required for SQLite |
| FFmpeg     | 5.x             | With ffprobe |
| SQLite     | 3.35+           | FTS5 support recommended |

Optional:
- **NVIDIA GPU** + drivers for NVENC
- **Intel GPU** for QuickSync (QSV)
- **AMD GPU** for VAAPI

---

## Docker (Recommended)

### Quick Start

```bash
git clone https://github.com/mymada/aetherstream.git
cd aetherstream
docker compose up --build -d
```

The server will be available at `http://localhost:8080`.

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AETHERSTREAM_AUTH_SECRET` | Yes | — | JWT signing secret (min 32 chars) |
| `AETHERSTREAM_MASTER_KEY` | No | — | AES-256-GCM master key for secure store |
| `AETHERSTREAM_DATABASE_PATH` | No | `./data/aetherstream.db` | SQLite database file |
| `AETHERSTREAM_SERVER_PORT` | No | `8096` | HTTP API port |
| `AETHERSTREAM_SERVER_HOST` | No | `0.0.0.0` | Bind address |
| `AETHERSTREAM_FFMPEG_PATH` | No | `ffmpeg` | FFmpeg binary path |
| `AETHERSTREAM_FFMPEG_MAX_JOBS` | No | `4` | Max concurrent transcodes |
| `AETHERSTREAM_FFMPEG_HWACCEL` | No | `auto` | Hardware acceleration mode |
| `AETHERSTREAM_SWIFTFLOW_URL` | No | — | SwiftFlow API base URL |
| `AETHERSTREAM_SWIFTFLOW_KEY` | No | — | SwiftFlow API key |

### Docker Compose (Production)

```yaml
services:
  aetherstream:
    image: aetherstream:latest
    container_name: aetherstream
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "8097:8097"
    volumes:
      - /media:/media
      - aetherstream-data:/data
    environment:
      - AETHERSTREAM_AUTH_SECRET=${JWT_SECRET}
      - AETHERSTREAM_MASTER_KEY=${MASTER_KEY}
      - AETHERSTREAM_DATABASE_PATH=/data/aetherstream.db
      - AETHERSTREAM_SERVER_PORT=8080
      - AETHERSTREAM_FFMPEG_MAX_JOBS=4
      - AETHERSTREAM_FFMPEG_HWACCEL=auto
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/system/info"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 15s

volumes:
  aetherstream-data:
```

### GPU Support (Docker)

#### NVIDIA (NVENC)

```yaml
services:
  aetherstream:
    image: aetherstream:latest
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

Requires NVIDIA Container Toolkit installed on host.

#### Intel (QSV/VAAPI)

```yaml
services:
  aetherstream:
    image: aetherstream:latest
    devices:
      - /dev/dri:/dev/dri
```

#### AMD (VAAPI)

```yaml
services:
  aetherstream:
    image: aetherstream:latest
    devices:
      - /dev/dri:/dev/dri
    group_add:
      - video
```

### Reverse Proxy (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name media.example.com;

    ssl_certificate /etc/letsencrypt/live/media.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/media.example.com/privkey.pem;

    client_max_body_size 0;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## systemd

Create a service file at `/etc/systemd/system/aetherstream.service`:

```ini
[Unit]
Description=AetherStream Media Server
After=network.target

[Service]
Type=simple
User=aetherstream
Group=aetherstream
WorkingDirectory=/opt/aetherstream
ExecStart=/opt/aetherstream/aetherstream
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/aetherstream/data /opt/aetherstream/thumbnails
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Environment
Environment="AETHERSTREAM_AUTH_SECRET=change-me-in-production"
Environment="AETHERSTREAM_DATABASE_PATH=/opt/aetherstream/data/aetherstream.db"
Environment="AETHERSTREAM_SERVER_PORT=8096"
Environment="AETHERSTREAM_FFMPEG_PATH=/usr/bin/ffmpeg"
Environment="AETHERSTREAM_FFMPEG_MAX_JOBS=4"

[Install]
WantedBy=multi-user.target
```

### Setup

```bash
# Create user
sudo useradd -r -s /bin/false -d /opt/aetherstream aetherstream

# Install binary
sudo mkdir -p /opt/aetherstream/data /opt/aetherstream/thumbnails
sudo cp aetherstream /opt/aetherstream/
sudo chown -R aetherstream:aetherstream /opt/aetherstream

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now aetherstream
sudo systemctl status aetherstream
```

---

## Kubernetes

### Namespace and ConfigMap

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: aetherstream
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: aetherstream-config
  namespace: aetherstream
data:
  config.yaml: |
    server:
      port: 8080
      host: 0.0.0.0
    ffmpeg:
      path: ffmpeg
      max_jobs: 4
      hwaccel: auto
```

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aetherstream-secrets
  namespace: aetherstream
type: Opaque
stringData:
  AETHERSTREAM_AUTH_SECRET: "change-me-in-production-min-32-chars"
  AETHERSTREAM_MASTER_KEY: ""
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aetherstream
  namespace: aetherstream
spec:
  replicas: 1
  selector:
    matchLabels:
      app: aetherstream
  template:
    metadata:
      labels:
        app: aetherstream
    spec:
      containers:
        - name: aetherstream
          image: aetherstream:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 8097
              name: dlna
          envFrom:
            - secretRef:
                name: aetherstream-secrets
          env:
            - name: AETHERSTREAM_DATABASE_PATH
              value: "/data/aetherstream.db"
            - name: AETHERSTREAM_SERVER_PORT
              value: "8080"
          volumeMounts:
            - name: data
              mountPath: /data
            - name: media
              mountPath: /media
            - name: config
              mountPath: /app/config.yaml
              subPath: config.yaml
          resources:
            requests:
              memory: "256Mi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "4000m"
          livenessProbe:
            httpGet:
              path: /system/info
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /system/info
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: aetherstream-data
        - name: media
          hostPath:
            path: /media
            type: Directory
        - name: config
          configMap:
            name: aetherstream-config
```

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: aetherstream
  namespace: aetherstream
spec:
  selector:
    app: aetherstream
  ports:
    - port: 80
      targetPort: 8080
      name: http
    - port: 8097
      targetPort: 8097
      name: dlna
  type: ClusterIP
```

### Ingress (nginx-ingress)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: aetherstream
  namespace: aetherstream
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "600"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - media.example.com
      secretName: media-tls
  rules:
    - host: media.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: aetherstream
                port:
                  number: 80
```

### PVC

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: aetherstream-data
  namespace: aetherstream
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

---

## Health Checks

| Endpoint | Expected | Interval |
|----------|----------|----------|
| `GET /system/info` | HTTP 200 + JSON | 30s |
| Docker HEALTHCHECK | wget | 30s |
| K8s liveness | HTTP 200 | 30s |
| K8s readiness | HTTP 200 | 10s |

---

## Troubleshooting Deployment

| Symptom | Cause | Fix |
|---------|-------|-----|
| `bind: address already in use` | Port conflict | Change `AETHERSTREAM_SERVER_PORT` |
| `permission denied` on /data | Wrong volume permissions | `chown 1000:1000 /data` |
| FFmpeg not found | Missing in PATH | Set `AETHERSTREAM_FFMPEG_PATH` |
| SQLite locked | Multiple writers | Ensure single process; use WAL mode |
| GPU transcode fails | Missing drivers | Install NVIDIA/Intel/AMD drivers on host |

---

## TLS / Let's Encrypt

AetherStream includes `AutoTLSManager` in `pkg/api/security.go`. To enable:

1. Set `AETHERSTREAM_SERVER_HOST` to your domain
2. Ensure port 443 is exposed
3. Pass domains to `AutoTLSManager(domains, cacheDir)` in a custom main wrapper

For Docker/systemd, it is recommended to terminate TLS at the reverse proxy (nginx/traefik) instead.
