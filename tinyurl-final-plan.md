# TinyURL — Полный план разработки

---

## Обзор

```
Инфраструктура:
  Этапы 1-6:  Timeweb VPS 16 ГБ (Docker Compose)       ~3000 руб/мес
              В день нагрузочных тестов: апгрейд до 64 ГБ  +300 руб
  Этап 7:     Timeweb Managed K8s (только ноды, всё ставим сами)

Этапы:
  1. Базовый бекенд + домен + HTTPS                     2 недели
  2. Мониторинг: Prometheus + Grafana + Alertmanager     1 неделя
  3. Redis кэш + Bloom filter                           1 неделя
  4. Аналитика: Kafka + ClickHouse                      1.5 недели
  5. Масштабирование (кластеры на день тестов)           2 недели
  6. Cassandra                                           1 неделя
  7. Kubernetes + CI/CD + ArgoCD                         2.5 недели
  ──────────────────────────────────────────────────────────────────
  Итого:                                                 ~11 недель
```

---

## Финальная архитектура

```
┌─────────────────── Git ──────────────────────────┐
│                                                   │
│  tinyurl-app (код)       tinyurl-infra (K8s)     │
│       │                         ▲                 │
│       ▼                         │                 │
│  GitLab CI ─── build ──────► update tag           │
│                                                   │
└──────────────────────┬────────────────────────────┘
                       │
                  ┌────▼─────┐
                  │  ArgoCD  │
                  └────┬─────┘
                       │
         ┌─────────────▼──────────────┐
         │   Timeweb Managed K8s      │
         │                            │
         │   ┌─── Ingress (TLS) ───┐  │
         │   └─────────┬───────────┘  │
         │             │              │
         │   ┌─────────▼─────────┐    │
         │   │  API Pods (3-10)  │    │   ← HPA автоскейл
         │   └──┬────┬────┬──────┘    │
         │      │    │    │           │
         │   ┌──▼┐ ┌─▼──┐│┌───▼──┐   │
         │   │PGB│ │Rds ││ │Kafka │   │
         │   │x2 │ │x6  ││ │x3   │   │
         │   └─┬─┘ └────┘│ └──┬──┘   │
         │     │          │    │      │
         │   ┌─▼────┐    │ ┌──▼───────────┐
         │   │PG    │    │ │Consumer (x3) │
         │   │pri+rp│    │ └──┬────────┬──┘
         │   └──────┘    │    │        │   │
         │               │ ┌──▼──┐ ┌───▼─┐│
         │               │ │Cass │ │ CH  ││
         │               │ │x3   │ │x2   ││
         │               │ └─────┘ └─────┘│
         │                                │
         │   ┌── Monitoring ───────────┐  │
         │   │ Prometheus → Grafana    │  │
         │   │ Alertmanager → Telegram │  │
         │   └─────────────────────────┘  │
         └────────────────────────────────┘

PGB = PGBouncer    Rds = Redis Cluster
PG  = PostgreSQL   CH  = ClickHouse
Cass = Cassandra
```

### Потоки данных

```
СОЗДАНИЕ ССЫЛКИ:
  Client → Ingress → API Pod
    → Redis Bloom (BF.EXISTS — есть ли longURL?)
      ├── "точно НЕТ" → Snowflake ID → base62 → PGBouncer → PostgreSQL INSERT
      └── "возможно ДА" → PGBouncer → PostgreSQL SELECT (проверка)
    → Redis Cache SET (прогрев)
    → 201 { short_url }

РЕДИРЕКТ:
  Client → Ingress → API Pod
    → Redis Cache GET
      ├── HIT → 302 redirect (микросекунды)
      └── MISS → PGBouncer → PostgreSQL SELECT → Redis SET → 302
    → Kafka Producer (async, не блокирует ответ)
      → Consumer Pod
        → ClickHouse INSERT (батч)
        → Cassandra INSERT

АНАЛИТИКА:
  Grafana → ClickHouse (SQL по дашбордам)
  Grafana → Prometheus (метрики инфраструктуры)
  API → GET /api/v1/stats/:shortURL → Cassandra (последние клики)
```

---

## Структура проекта

```
tinyurl/
├── cmd/
│   ├── api/
│   │   └── main.go                  # HTTP-сервер
│   └── consumer/
│       └── main.go                  # Kafka consumer
│
├── internal/
│   ├── config/
│   │   └── config.go                # Конфигурация из env
│   ├── handler/
│   │   ├── shorten.go               # POST /api/v1/shorten
│   │   ├── redirect.go              # GET /:shortURL
│   │   ├── stats.go                 # GET /api/v1/stats/:shortURL
│   │   └── health.go                # GET /health
│   ├── service/
│   │   └── url_service.go           # Бизнес-логика
│   ├── repository/
│   │   ├── postgres.go              # SQL-запросы
│   │   ├── redis_cache.go           # Redis кэш
│   │   ├── redis_bloom.go           # Redis Bloom filter
│   │   └── cassandra.go             # Cassandra (статистика)
│   ├── kafka/
│   │   └── producer.go              # Kafka producer
│   ├── snowflake/
│   │   └── generator.go             # Snowflake ID генератор
│   ├── base62/
│   │   └── base62.go                # Encode / Decode
│   ├── metrics/
│   │   └── prometheus.go            # Кастомные метрики
│   └── middleware/
│       ├── logging.go
│       └── metrics.go               # Prometheus middleware
│
├── consumer/
│   ├── clickhouse_writer.go         # Запись в ClickHouse
│   └── cassandra_writer.go          # Запись в Cassandra
│
├── migrations/
│   ├── postgres/
│   │   └── 001_init.sql
│   ├── cassandra/
│   │   └── 001_init.cql
│   └── clickhouse/
│       └── 001_init.sql
│
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile.api
│   │   ├── Dockerfile.consumer
│   │   ├── docker-compose.yml           # повседневный (1 реплика)
│   │   ├── docker-compose.cluster.yml   # день нагрузочных тестов
│   │   ├── nginx/
│   │   │   └── nginx.conf
│   │   ├── prometheus/
│   │   │   ├── prometheus.yml
│   │   │   └── alerts.yml
│   │   └── alertmanager/
│   │       └── alertmanager.yml
│   └── k8s/
│       ├── apps/
│       │   ├── namespace.yaml
│       │   ├── secrets.yaml
│       │   ├── api/
│       │   │   ├── deployment.yaml
│       │   │   ├── service.yaml
│       │   │   └── hpa.yaml
│       │   ├── consumer/
│       │   │   ├── deployment.yaml
│       │   │   └── service.yaml
│       │   └── ingress.yaml
│       ├── helm-values/
│       │   ├── postgres-values.yaml     # + PGBouncer
│       │   ├── redis-values.yaml
│       │   ├── kafka-values.yaml
│       │   ├── clickhouse-values.yaml
│       │   ├── cassandra-values.yaml
│       │   └── prometheus-values.yaml
│       └── argocd/
│           └── app-of-apps.yaml
│
├── dashboards/
│   ├── overview.json                # Grafana: RPS, латентность, кэш
│   └── analytics.json               # Grafana: ClickHouse аналитика
│
├── loadtest/
│   ├── k6_create.js
│   ├── k6_redirect.js
│   └── k6_mixed.js
│
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

### Go-зависимости

```
github.com/go-chi/chi/v5                # роутер
github.com/jackc/pgx/v5                 # PostgreSQL
github.com/redis/go-redis/v9            # Redis
github.com/segmentio/kafka-go           # Kafka
github.com/ClickHouse/clickhouse-go/v2  # ClickHouse
github.com/gocql/gocql                  # Cassandra
github.com/bwmarrin/snowflake           # ID генератор
github.com/prometheus/client_golang     # метрики
```

---

# Этап 1: Базовый бекенд + домен + HTTPS

**Цель:** рабочий сокращатель на одном VPS. Go + PostgreSQL + Nginx.
Никаких кэшей, очередей, аналитики.

```
Пользователь
      │
      ▼
┌────────────┐
│   Nginx    │  ← HTTPS (Let's Encrypt)
│  :80/:443  │
└─────┬──────┘
      │
      ▼
┌────────────┐
│  Go API    │  ← 1 инстанс
│   :8080    │
└─────┬──────┘
      │
      ▼
┌────────────┐
│ PostgreSQL │  ← 1 инстанс
│   :5432    │
└────────────┘
```

---

### 1.1 Арендовать VPS

```
Timeweb → Cloud Servers → Создать

  CPU:   4 vCPU
  RAM:   16 ГБ          ← сразу 16, чтобы хватило до этапа 6
  Disk:  100 ГБ SSD
  OS:    Ubuntu 24.04

Стоимость: ~3000 руб/мес
```

### 1.2 Купить домен

```
Timeweb → Домены → Зарегистрировать

  Например: mylinks.ru (~200 руб/год)

  DNS-записи:
    A      @      185.xx.xx.xx     3600
    A      www    185.xx.xx.xx     3600

Проверка: nslookup mylinks.ru → должен показать IP VPS
Ждём 15 мин — 48 часов.
```

### 1.3 Настроить сервер

```bash
ssh root@185.xx.xx.xx

apt update && apt upgrade -y

# Docker
curl -fsSL https://get.docker.com | sh
apt install -y docker-compose-plugin

# Go
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc

# Инструменты
apt install -y git make nginx certbot python3-certbot-nginx
```

### 1.4 Миграция PostgreSQL

```sql
-- migrations/postgres/001_init.sql

CREATE TABLE urls (
    id         BIGINT PRIMARY KEY,
    short_url  VARCHAR(10) UNIQUE NOT NULL,
    long_url   TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_urls_long_url ON urls USING hash(long_url);
```

### 1.5 API endpoints

```
POST /api/v1/shorten
  Body:     { "long_url": "https://amazon.com/very-long-page" }
  Response: { "short_url": "https://mylinks.ru/zn9edcu" }
  Логика:
    1. Валидация URL
    2. SELECT short_url FROM urls WHERE long_url = $1 (дедупликация)
    3. Если есть → вернуть
    4. Если нет → Snowflake ID → base62 → INSERT → вернуть

GET /:shortURL
  Response: 302 redirect
  Логика:
    1. SELECT long_url FROM urls WHERE short_url = $1
    2. Найден → 302 Location: long_url
    3. Не найден → 404

GET /health
  Response: { "status": "ok", "db": "connected" }
```

### 1.6 Dockerfile

```dockerfile
# deploy/docker/Dockerfile.api

FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /api /api
EXPOSE 8080
CMD ["/api"]
```

### 1.7 Docker Compose (минимальный)

```yaml
# deploy/docker/docker-compose.yml

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: tinyurl
      POSTGRES_USER: app
      POSTGRES_PASSWORD: secret
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - pg_data:/var/lib/postgresql/data
      - ../../migrations/postgres:/docker-entrypoint-initdb.d
    restart: always

  api:
    build:
      context: ../../
      dockerfile: deploy/docker/Dockerfile.api
    environment:
      APP_PORT: "8080"
      APP_BASE_URL: "https://mylinks.ru"
      POSTGRES_DSN: "postgres://app:secret@postgres:5432/tinyurl?sslmode=disable"
      SNOWFLAKE_NODE: "1"
    ports:
      - "127.0.0.1:8080:8080"
    depends_on:
      - postgres
    restart: always

volumes:
  pg_data:
```

### 1.8 Nginx + HTTPS

```nginx
# /etc/nginx/sites-available/mylinks

server {
    listen 80;
    server_name mylinks.ru www.mylinks.ru;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```bash
ln -s /etc/nginx/sites-available/mylinks /etc/nginx/sites-enabled/
rm /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx

# HTTPS
certbot --nginx -d mylinks.ru -d www.mylinks.ru
```

### 1.9 Запуск и проверка

```bash
cd tinyurl
docker compose -f deploy/docker/docker-compose.yml up -d --build

curl https://mylinks.ru/health
# → {"status":"ok","db":"connected"}

curl -X POST https://mylinks.ru/api/v1/shorten \
  -H "Content-Type: application/json" \
  -d '{"long_url":"https://amazon.com/dp/B017V4NTFA"}'
# → {"short_url":"https://mylinks.ru/zn9edcu"}

curl -v https://mylinks.ru/zn9edcu
# → 302 Location: https://amazon.com/dp/B017V4NTFA
```

### ✅ Результат этапа 1

```
✅ Go API работает
✅ PostgreSQL хранит ссылки
✅ Домен mylinks.ru с HTTPS
✅ Nginx проксирует трафик
```

---

# Этап 2: Мониторинг

**Цель:** видеть состояние системы ДО того, как начнём масштабировать.

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│ Go API   │────▶│Prometheus│────▶│ Grafana  │
│ /metrics │     │  :9090   │     │  :3000   │
└──────────┘     └────┬─────┘     └──────────┘
                      │
                      ▼
                ┌──────────────┐     ┌──────────┐
                │ Alertmanager │────▶│ Telegram │
                └──────────────┘     └──────────┘
```

### 2.1 Метрики в Go-приложении

```
Зависимость: github.com/prometheus/client_golang
Endpoint:    GET /metrics

Метрики:
  tinyurl_links_created_total                        # счётчик созданных ссылок
  tinyurl_redirects_total{status}                    # редиректы (hit/miss/not_found)
  tinyurl_errors_total{handler}                      # ошибки
  tinyurl_request_duration_seconds{handler,method}   # латентность (гистограмма)
  tinyurl_db_query_duration_seconds{query}           # время SQL
  tinyurl_active_connections                         # текущие соединения
```

### 2.2 Добавить в Docker Compose

```yaml
# Добавить к docker-compose.yml

  prometheus:
    image: prom/prometheus:v2.50.0
    ports:
      - "127.0.0.1:9090:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./prometheus/alerts.yml:/etc/prometheus/alerts.yml:ro
      - prom_data:/prometheus
    restart: always

  grafana:
    image: grafana/grafana:10.3.0
    ports:
      - "127.0.0.1:3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin123
      GF_SERVER_ROOT_URL: https://mylinks.ru/grafana/
    volumes:
      - grafana_data:/var/lib/grafana
    restart: always

  alertmanager:
    image: prom/alertmanager:v0.27.0
    ports:
      - "127.0.0.1:9093:9093"
    volumes:
      - ./alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
    restart: always

  node-exporter:
    image: prom/node-exporter:v1.7.0
    ports:
      - "127.0.0.1:9100:9100"
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
    command:
      - '--path.procfs=/host/proc'
      - '--path.sysfs=/host/sys'
    restart: always

  postgres-exporter:
    image: prometheuscommunity/postgres-exporter:v0.15.0
    environment:
      DATA_SOURCE_NAME: "postgres://app:secret@postgres:5432/tinyurl?sslmode=disable"
    ports:
      - "127.0.0.1:9187:9187"
    restart: always
```

### 2.3 Prometheus конфигурация

```yaml
# deploy/docker/prometheus/prometheus.yml

global:
  scrape_interval: 15s

rule_files:
  - "alerts.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets: ["alertmanager:9093"]

scrape_configs:
  - job_name: "tinyurl-api"
    static_configs:
      - targets: ["api:8080"]

  - job_name: "node"
    static_configs:
      - targets: ["node-exporter:9100"]

  - job_name: "postgres"
    static_configs:
      - targets: ["postgres-exporter:9187"]
```

### 2.4 Алерты

```yaml
# deploy/docker/prometheus/alerts.yml

groups:
  - name: tinyurl
    rules:
      - alert: APIDown
        expr: up{job="tinyurl-api"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "API недоступен!"

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(tinyurl_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "P95 > 500ms: {{ $value }}s"

      - alert: HighErrorRate
        expr: rate(tinyurl_errors_total[5m]) > 1
        for: 3m
        labels:
          severity: critical
        annotations:
          summary: "Ошибки: {{ $value }}/сек"

      - alert: HighCPU
        expr: 100 - (avg(irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "CPU > 80%"

      - alert: DiskLow
        expr: node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"} < 0.15
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Диск < 15%"

      - alert: PGHighConnections
        expr: pg_stat_activity_count > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "PostgreSQL > 80 соединений"
```

### 2.5 Alertmanager → Telegram

```
Создать бота:
  1. @BotFather → /newbot → tinyurl_alerts_bot
  2. Получить token
  3. Создать группу, добавить бота
  4. Получить chat_id
```

```yaml
# deploy/docker/alertmanager/alertmanager.yml

route:
  receiver: "telegram"
  group_wait: 10s
  group_interval: 5m

receivers:
  - name: "telegram"
    telegram_configs:
      - bot_token: "123456:ABC-DEF..."
        chat_id: -100123456789
        message: |
          {{ if eq .Status "firing" }}🔥{{ else }}✅{{ end }}
          <b>{{ .GroupLabels.alertname }}</b>
          {{ range .Alerts }}{{ .Annotations.summary }}{{ end }}
        parse_mode: "HTML"
```

### 2.6 Grafana через Nginx

```nginx
# Добавить в /etc/nginx/sites-available/mylinks

    location /grafana/ {
        proxy_pass http://127.0.0.1:3000/;
        proxy_set_header Host $host;
    }
```

### 2.7 Grafana дашборд "Обзор системы"

```
Панель                          PromQL
──────────────────────────────────────────────────────────────────
Ссылок создано всего            tinyurl_links_created_total
Создания/мин                    rate(tinyurl_links_created_total[1m]) * 60
Редиректы/мин                   rate(sum(tinyurl_redirects_total)[1m]) * 60
RPS                             rate(tinyurl_request_duration_seconds_count[1m])
P50 латентность                 histogram_quantile(0.50, rate(tinyurl_request_duration_seconds_bucket[5m]))
P95 латентность                 histogram_quantile(0.95, rate(tinyurl_request_duration_seconds_bucket[5m]))
P99 латентность                 histogram_quantile(0.99, rate(tinyurl_request_duration_seconds_bucket[5m]))
CPU %                           100 - avg(irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100
RAM                             node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes
Диск                            node_filesystem_avail_bytes{mountpoint="/"}
PG соединения                   pg_stat_activity_count
```

### ✅ Результат этапа 2

```
✅ Prometheus собирает метрики с API и сервера
✅ Grafana дашборд: RPS, латентность, CPU, RAM, PG
✅ Alertmanager → Telegram
✅ Каждый следующий этап будет виден на дашборде
```

---

# Этап 3: Redis (кэш + Bloom filter)

**Цель:** ускорить редиректы кэшем, снизить нагрузку на PG через Bloom filter.

```
              ┌────────────┐
              │  Go API    │
              └──┬───┬───┬─┘
                 │   │   │
        ┌────────┘   │   └────────┐
        ▼            ▼            ▼
  ┌──────────┐ ┌──────────┐ ┌────────────┐
  │  Redis   │ │  Redis   │ │ PostgreSQL │
  │  Cache   │ │  Bloom   │ │            │
  └──────────┘ └──────────┘ └────────────┘
```

### 3.1 Добавить Redis

```yaml
# Добавить в docker-compose.yml

  redis:
    image: redis/redis-stack-server:7.2.0-v9    # включает RedisBloom
    ports:
      - "127.0.0.1:6379:6379"
    command: >
      redis-server
      --maxmemory 1gb
      --maxmemory-policy allkeys-lru
      --appendonly yes
    volumes:
      - redis_data:/data
    restart: always

  redis-exporter:
    image: oliver006/redis_exporter:v1.58.0
    environment:
      REDIS_ADDR: "redis:6379"
    ports:
      - "127.0.0.1:9121:9121"
    restart: always
```

### 3.2 Кэш для редиректов (cache-aside)

```
GET /abc1234:
  1. Redis GET "short:abc1234"
     ├── HIT  → 302 redirect (~1ms)
     └── MISS → PostgreSQL SELECT → Redis SET TTL 24h → 302 (~5-10ms)
```

### 3.3 Bloom filter для создания

```
POST /api/v1/shorten:
  1. BF.EXISTS "bloom:urls" longURL
     ├── "точно НЕТ"  → Snowflake → base62 → PG INSERT → BF.ADD → Redis SET
     └── "возможно ДА" → PG SELECT (проверяем)
                          ├── найден → вернуть существующий
                          └── не найден (false positive) → создать

Инициализация:
  BF.RESERVE bloom:urls 0.01 10000000    # 10 млн, 1% false positive
```

### 3.4 Метрики

```
tinyurl_cache_hits_total
tinyurl_cache_misses_total
tinyurl_bloom_checks_total{result="definitely_not"|"probably_yes"}
```

```yaml
# Добавить в prometheus.yml
  - job_name: "redis"
    static_configs:
      - targets: ["redis-exporter:9121"]
```

### 3.5 Grafana панели

```
Cache hit rate (%):   rate(tinyurl_cache_hits_total[5m]) / (hits + misses) * 100
Redis память:         redis_memory_used_bytes
Redis ops/sec:        rate(redis_commands_processed_total[1m])
```

### 3.6 Алерт

```yaml
      - alert: RedisHighMemory
        expr: redis_memory_used_bytes / redis_memory_max_bytes > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Redis > 90% памяти"
```

### ✅ Результат этапа 3

```
✅ Редиректы из Redis (~1ms вместо ~5-10ms)
✅ Bloom filter снижает нагрузку на PG
✅ Grafana: cache hit rate, Redis метрики
```

---

# Этап 4: Аналитика (Kafka + ClickHouse)

**Цель:** каждый клик → Kafka → Consumer → ClickHouse. Дашборды аналитики.

```
┌────────────┐
│  Go API    │──▶ Kafka (topic: clicks)
│ (redirect) │        │
└────────────┘        ▼
                 Consumer (Go) ──▶ ClickHouse
```

### 4.1 Добавить Kafka (1 брокер)

```yaml
  kafka:
    image: confluentinc/cp-kafka:7.6.0
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      CLUSTER_ID: "MkU3OEVBNTcwNTJENDM2Qk"
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_LOG_RETENTION_HOURS: 168
    ports:
      - "127.0.0.1:9092:9092"
    volumes:
      - kafka_data:/var/lib/kafka/data
    restart: always

  kafka-ui:
    image: provectuslabs/kafka-ui:v0.7.2
    ports:
      - "127.0.0.1:8081:8080"
    environment:
      KAFKA_CLUSTERS_0_NAME: tinyurl
      KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS: kafka:9092
    restart: always

  kafka-exporter:
    image: danielqsj/kafka-exporter:v1.7.0
    command: --kafka.server=kafka:9092
    ports:
      - "127.0.0.1:9308:9308"
    restart: always
```

### 4.2 Создать топик

```bash
docker exec kafka kafka-topics --create \
  --topic clicks --partitions 3 --replication-factor 1 \
  --bootstrap-server localhost:9092
```

### 4.3 Producer в API

```
При GET /:shortURL → асинхронно в Kafka:
  {
    "short_url": "abc1234",
    "clicked_at": "2026-02-13T15:04:05Z",
    "ip": "85.143.12.44",
    "user_agent": "Chrome/120",
    "referer": "https://twitter.com/...",
    "country": "NL"
  }

Отправка НЕ блокирует 302 ответ пользователю.
```

### 4.4 ClickHouse

```yaml
  clickhouse:
    image: clickhouse/clickhouse-server:24.1
    ports:
      - "127.0.0.1:8123:8123"
      - "127.0.0.1:9000:9000"
    volumes:
      - ch_data:/var/lib/clickhouse
    ulimits:
      nofile: { soft: 262144, hard: 262144 }
    restart: always
```

### 4.5 Миграция ClickHouse

```sql
CREATE DATABASE IF NOT EXISTS tinyurl;

CREATE TABLE tinyurl.clicks (
    short_url    String,
    clicked_at   DateTime,
    ip           String,
    country      LowCardinality(String),
    user_agent   String,
    referer      String,
    date         Date DEFAULT toDate(clicked_at)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (short_url, clicked_at)
TTL date + INTERVAL 1 YEAR;

-- Автоагрегация: клики по часам
CREATE MATERIALIZED VIEW tinyurl.clicks_hourly
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (short_url, hour, date)
AS SELECT
    short_url,
    toStartOfHour(clicked_at) AS hour,
    toDate(clicked_at) AS date,
    count() AS click_count,
    uniqExact(ip) AS unique_visitors
FROM tinyurl.clicks
GROUP BY short_url, hour, date;

-- Автоагрегация: клики по странам
CREATE MATERIALIZED VIEW tinyurl.clicks_by_country
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (short_url, country, date)
AS SELECT
    short_url, country,
    toDate(clicked_at) AS date,
    count() AS click_count
FROM tinyurl.clicks
GROUP BY short_url, country, date;
```

### 4.6 Consumer

```yaml
  consumer:
    build:
      context: ../../
      dockerfile: deploy/docker/Dockerfile.consumer
    environment:
      KAFKA_BROKERS: "kafka:9092"
      KAFKA_TOPIC: "clicks"
      KAFKA_GROUP: "tinyurl-clicks"
      CLICKHOUSE_DSN: "clickhouse://clickhouse:9000/tinyurl"
    depends_on:
      - kafka
      - clickhouse
    restart: always
```

Consumer читает из Kafka, батчит по 100 сообщений или 1 секунде, batch INSERT в ClickHouse.

### 4.7 Метрики

```
tinyurl_kafka_messages_sent_total
tinyurl_kafka_errors_total
tinyurl_kafka_send_duration_seconds
```

### 4.8 Grafana — аналитический дашборд (ClickHouse datasource)

```sql
-- Топ-20 популярных ссылок сегодня
SELECT short_url, count() AS clicks
FROM tinyurl.clicks WHERE date = today()
GROUP BY short_url ORDER BY clicks DESC LIMIT 20

-- Клики по часам
SELECT toStartOfHour(clicked_at) AS hour, count() AS clicks
FROM tinyurl.clicks WHERE clicked_at > now() - INTERVAL 24 HOUR
GROUP BY hour ORDER BY hour

-- Топ стран за неделю
SELECT country, count() AS clicks
FROM tinyurl.clicks WHERE date >= today() - 7
GROUP BY country ORDER BY clicks DESC LIMIT 10

-- Уники vs всего
SELECT toStartOfHour(clicked_at) AS hour,
       count() AS total, uniqExact(ip) AS unique_visitors
FROM tinyurl.clicks WHERE clicked_at > now() - INTERVAL 24 HOUR
GROUP BY hour ORDER BY hour

-- Мобильные vs десктоп
SELECT multiIf(
    position(user_agent, 'Mobile') > 0, 'Mobile',
    position(user_agent, 'Tablet') > 0, 'Tablet',
    'Desktop') AS device, count() AS clicks
FROM tinyurl.clicks WHERE date = today()
GROUP BY device
```

### 4.9 Алерты Kafka

```yaml
      - alert: KafkaConsumerLag
        expr: kafka_consumergroup_lag_sum{consumergroup="tinyurl-clicks"} > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Kafka consumer lag > 10000"

      - alert: KafkaSendErrors
        expr: rate(tinyurl_kafka_errors_total[5m]) > 0
        for: 3m
        labels:
          severity: critical
        annotations:
          summary: "Ошибки записи в Kafka"
```

### ✅ Результат этапа 4

```
✅ Каждый клик → Kafka → Consumer → ClickHouse
✅ Materialized views агрегируют автоматически
✅ Grafana: топ ссылок, страны, устройства, клики/час
✅ Алерты на Kafka lag
```

---

# Этап 5: Масштабирование

**Цель:** в обычный день работаем с 1 репликой. В день нагрузочного
тестирования апгрейдим VPS до 64 ГБ и поднимаем полные кластеры.

```
Повседневно:        VPS 16 ГБ + docker-compose.yml
День тестирования:  VPS 64 ГБ + docker-compose.cluster.yml
```

---

## 5.1 Несколько инстансов API

```yaml
# В docker-compose.cluster.yml

  api-1:
    build: { context: ../../, dockerfile: deploy/docker/Dockerfile.api }
    environment: &app-env
      APP_PORT: "8080"
      APP_BASE_URL: "https://mylinks.ru"
      POSTGRES_DSN: "postgres://app:secret@postgres:5432/tinyurl?sslmode=disable"
      REDIS_ADDR: "redis:6379"
      KAFKA_BROKERS: "kafka-1:9092,kafka-2:9092,kafka-3:9092"
      SNOWFLAKE_NODE: "1"
    restart: always

  api-2:
    build: { context: ../../, dockerfile: deploy/docker/Dockerfile.api }
    environment:
      <<: *app-env
      SNOWFLAKE_NODE: "2"      # уникальный!
    restart: always

  api-3:
    build: { context: ../../, dockerfile: deploy/docker/Dockerfile.api }
    environment:
      <<: *app-env
      SNOWFLAKE_NODE: "3"
    restart: always
```

Nginx upstream:
```nginx
upstream api_backend {
    least_conn;
    server api-1:8080;
    server api-2:8080;
    server api-3:8080;
}
```

---

## 5.2 Redis Cluster (6 нод: 3 master + 3 replica)

```
Как работает:
  Ключи разбиты на 16384 слота:
    Master 1: слоты 0-5460      ←→ Replica 1
    Master 2: слоты 5461-10922  ←→ Replica 2
    Master 3: слоты 10923-16383 ←→ Replica 3

  CRC16("short:abc1234") % 16384 = слот → нужный Master
  Master упал → Replica автоматически становится Master
```

```yaml
# В docker-compose.cluster.yml (6 нод redis-1..redis-6)

  redis-1: &redis-node
    image: redis:7-alpine
    command: >
      redis-server --port 6379
      --cluster-enabled yes --cluster-config-file nodes.conf
      --cluster-node-timeout 5000 --appendonly yes
      --maxmemory 512mb --maxmemory-policy allkeys-lru
    restart: always

  redis-2: { <<: *redis-node }
  redis-3: { <<: *redis-node }
  redis-4: { <<: *redis-node }
  redis-5: { <<: *redis-node }
  redis-6: { <<: *redis-node }

  redis-init:
    image: redis:7-alpine
    command: >
      sh -c "sleep 5 && redis-cli --cluster create
      redis-1:6379 redis-2:6379 redis-3:6379
      redis-4:6379 redis-5:6379 redis-6:6379
      --cluster-replicas 1 --cluster-yes"
    restart: "no"
```

В Go: `redis.NewClient()` → `redis.NewClusterClient()`. Код не меняется.

---

## 5.3 Kafka Cluster (3 брокера KRaft)

```
Как Kafka масштабируется:

  Topic "clicks" — 6 партиций, replication-factor 3:

    Брокер 1: P0(leader), P1(replica), P2(replica), P3(leader)...
    Брокер 2: P0(replica), P1(leader), P2(replica), P4(leader)...
    Брокер 3: P0(replica), P1(replica), P2(leader), P5(leader)...

    Каждая партиция: 1 leader + 2 replicas
    Брокер упал → leaders переехали на живых

  Масштабирование записи: больше партиций
  Масштабирование чтения: больше consumer-ов (макс = кол-во партиций)

    Consumer group "tinyurl-clicks":
      Consumer 1: P0, P1      (6 партиций / 3 consumer-а = по 2)
      Consumer 2: P2, P3
      Consumer 3: P4, P5

      Consumer 2 упал → Kafka ребалансирует:
        Consumer 1: P0, P1, P2
        Consumer 3: P3, P4, P5
```

```yaml
# В docker-compose.cluster.yml

  kafka-1:
    image: confluentinc/cp-kafka:7.6.0
    environment: &kafka-env
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka-1:9093,2@kafka-2:9093,3@kafka-3:9093
      CLUSTER_ID: "MkU3OEVBNTcwNTJENDM2Qk"
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 3
      KAFKA_DEFAULT_REPLICATION_FACTOR: 3
      KAFKA_MIN_INSYNC_REPLICAS: 2
      KAFKA_LOG_RETENTION_HOURS: 168
      KAFKA_NODE_ID: 1
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka-1:9092
    volumes:
      - kafka_1:/var/lib/kafka/data
    restart: always

  kafka-2:
    image: confluentinc/cp-kafka:7.6.0
    environment:
      <<: *kafka-env
      KAFKA_NODE_ID: 2
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka-2:9092
    volumes:
      - kafka_2:/var/lib/kafka/data
    restart: always

  kafka-3:
    image: confluentinc/cp-kafka:7.6.0
    environment:
      <<: *kafka-env
      KAFKA_NODE_ID: 3
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka-3:9092
    volumes:
      - kafka_3:/var/lib/kafka/data
    restart: always
```

Consumer-ы (3 штуки, одна consumer group):
```yaml
  consumer-1: &consumer
    build: { context: ../../, dockerfile: deploy/docker/Dockerfile.consumer }
    environment: &consumer-env
      KAFKA_BROKERS: "kafka-1:9092,kafka-2:9092,kafka-3:9092"
      KAFKA_TOPIC: "clicks"
      KAFKA_GROUP: "tinyurl-clicks"
      CLICKHOUSE_DSN: "clickhouse://clickhouse:9000/tinyurl"
    restart: always

  consumer-2: { <<: *consumer }
  consumer-3: { <<: *consumer }
```

---

## 5.4 Нагрузочное тестирование (k6)

```javascript
// loadtest/k6_create.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '30s', target: 50 },
        { duration: '2m',  target: 200 },
        { duration: '5m',  target: 500 },
        { duration: '1m',  target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'],
        http_req_failed: ['rate<0.01'],
    },
};

export default function () {
    const res = http.post('https://mylinks.ru/api/v1/shorten',
        JSON.stringify({ long_url: `https://example.com/${__VU}-${__ITER}-${Date.now()}` }),
        { headers: { 'Content-Type': 'application/json' } }
    );
    check(res, { 'status 201': (r) => r.status === 201 });
    sleep(0.1);
}
```

```javascript
// loadtest/k6_redirect.js
import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';

const urls = new SharedArray('urls', () => JSON.parse(open('./short_urls.json')));

export const options = {
    stages: [
        { duration: '30s', target: 100 },
        { duration: '3m',  target: 1000 },
        { duration: '5m',  target: 5000 },
        { duration: '1m',  target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<100'],
        http_req_failed: ['rate<0.001'],
    },
};

export default function () {
    const res = http.get(`https://mylinks.ru/${urls[Math.floor(Math.random() * urls.length)]}`,
        { redirects: 0 });
    check(res, { 'status 302': (r) => r.status === 302 });
}
```

---

## 5.5 Makefile

```makefile
# Повседневная работа
up:
	docker compose -f deploy/docker/docker-compose.yml up -d

down:
	docker compose -f deploy/docker/docker-compose.yml down

logs:
	docker compose -f deploy/docker/docker-compose.yml logs -f

# День нагрузочного тестирования (после апгрейда VPS до 64 ГБ)
cluster-up:
	docker compose -f deploy/docker/docker-compose.cluster.yml up -d
	@echo "Ждём 60 сек пока кластеры соберутся..."
	@sleep 60
	@make cluster-init

cluster-down:
	docker compose -f deploy/docker/docker-compose.cluster.yml down

cluster-init:
	docker exec redis-1 redis-cli --cluster create \
	  redis-1:6379 redis-2:6379 redis-3:6379 \
	  redis-4:6379 redis-5:6379 redis-6:6379 \
	  --cluster-replicas 1 --cluster-yes
	docker exec kafka-1 kafka-topics --create \
	  --topic clicks --partitions 6 --replication-factor 3 \
	  --bootstrap-server kafka-1:9092

# Нагрузка
load-create:
	k6 run loadtest/k6_create.js

load-redirect:
	k6 run loadtest/k6_redirect.js
```

### День тестирования — пошагово

```bash
# 1. Timeweb панель → Сервер → Изменить → 64 ГБ (5-10 мин, ребут)
# 2. ssh root@185.xx.xx.xx
make cluster-up
# 3. Нагрузка
make load-create
make load-redirect
# 4. Смотрим Grafana: RPS, латентность, cache hit rate, Kafka lag
# 5. Закончили:
make cluster-down
make up
# 6. Timeweb панель → Сервер → Изменить → 16 ГБ
```

### ✅ Результат этапа 5

```
✅ 3 инстанса API за Nginx
✅ Redis Cluster 6 нод (шардирование + failover)
✅ Kafka Cluster 3 брокера (репликация + параллельная обработка)
✅ 3 consumer-а параллельно
✅ k6 нагрузочные тесты
✅ Всё видно в Grafana
```

---

# Этап 6: Cassandra

**Цель:** быстрое чтение кликов по конкретной ссылке. ClickHouse для агрегаций,
Cassandra для точечных запросов.

```
Kafka → Consumer ─┬→ ClickHouse  (GROUP BY, топ ссылок, дашборды)
                   └→ Cassandra   (SELECT * WHERE short_url = X за 1-5ms)
```

### 6.1 Зачем обе

```
ClickHouse: "топ-100 ссылок за неделю"          → 500ms ✅
ClickHouse: "последние 100 кликов по abc1234"    → 50-500ms (full scan)

Cassandra:  "последние 100 кликов по abc1234"    → 1-5ms ✅ (прямой lookup)
Cassandra:  "топ-100 ссылок за неделю"           → невозможно ❌
```

### 6.2 Cassandra (1 нода повседневно, 3 в день тестов)

```yaml
# docker-compose.yml (1 нода)
  cassandra:
    image: cassandra:4.1
    environment:
      CASSANDRA_CLUSTER_NAME: tinyurl
      CASSANDRA_DC: dc1
      MAX_HEAP_SIZE: 2G
      HEAP_NEWSIZE: 400M
    ports:
      - "127.0.0.1:9042:9042"
    volumes:
      - cass_data:/var/lib/cassandra
    restart: always
```

```yaml
# docker-compose.cluster.yml (3 ноды) — поднимаются последовательно!
  cassandra-1: &cass
    image: cassandra:4.1
    environment: &cass-env
      CASSANDRA_CLUSTER_NAME: tinyurl
      CASSANDRA_DC: dc1
      CASSANDRA_SEEDS: cassandra-1
      MAX_HEAP_SIZE: 2G
      HEAP_NEWSIZE: 400M
    restart: always

  cassandra-2:
    <<: *cass
    depends_on: [cassandra-1]

  cassandra-3:
    <<: *cass
    depends_on: [cassandra-2]
```

### 6.3 Миграция

```cql
CREATE KEYSPACE tinyurl WITH replication = {
    'class': 'NetworkTopologyStrategy', 'dc1': 3
};

CREATE TABLE tinyurl.clicks (
    short_url   text,
    clicked_at  timestamp,
    ip          text,
    country     text,
    user_agent  text,
    referer     text,
    PRIMARY KEY (short_url, clicked_at)
) WITH CLUSTERING ORDER BY (clicked_at DESC)
  AND default_time_to_live = 7776000
  AND compaction = {
    'class': 'TimeWindowCompactionStrategy',
    'compaction_window_size': 1,
    'compaction_window_unit': 'DAYS'
  };
```

### 6.4 Consumer пишет в обе базы параллельно

```
Kafka message →
  ├── goroutine 1: batch INSERT в ClickHouse
  └── goroutine 2: INSERT в Cassandra
```

### 6.5 API endpoint

```
GET /api/v1/stats/:shortURL
  → Cassandra: SELECT * FROM clicks WHERE short_url = ? LIMIT 100
  → JSON: { "short_url": "abc1234", "total_clicks": 1523, "recent_clicks": [...] }
```

### ✅ Результат этапа 6

```
✅ Cassandra хранит сырые клики
✅ Consumer пишет в ClickHouse + Cassandra параллельно
✅ /stats/:shortURL читает из Cassandra за 1-5ms
✅ Полный стек собран
```

---

# Этап 7: Kubernetes + CI/CD + ArgoCD

**Цель:** переехать на Timeweb Managed K8s. Все сервисы ставим сами через
Helm. CI/CD через GitLab. Автодеплой через ArgoCD.

---

## 7.1 Что такое ArgoCD

```
Без ArgoCD:
  git push → CI → kubectl apply → кластер     (push-модель, CI нужен доступ)

С ArgoCD:
  git push → ArgoCD ВИДИТ изменение → применяет к кластеру    (pull-модель)

Git = единственный источник правды.
Что в Git — то и в кластере. Всегда.
Откат = git revert.
Дрифт-детекция: кто-то руками поменял → ArgoCD вернёт как в Git.
```

```
Developer → git push → GitLab CI → build → push image → update tag в infra-репо
                                                              │
                                              ArgoCD следит ──┘
                                              ArgoCD применяет к кластеру
```

---

## 7.2 Два Git-репозитория

```
tinyurl-app (код):
  ├── cmd/ internal/ Dockerfile ...
  └── .gitlab-ci.yml

tinyurl-infra (K8s манифесты):
  ├── apps/
  │   ├── api/ (deployment, service, hpa)
  │   ├── consumer/ (deployment, service)
  │   └── ingress.yaml
  ├── helm-values/
  │   ├── postgres-values.yaml
  │   ├── redis-values.yaml
  │   ├── kafka-values.yaml
  │   ├── clickhouse-values.yaml
  │   ├── cassandra-values.yaml
  │   └── prometheus-values.yaml
  └── argocd/
      └── app-of-apps.yaml
```

---

## 7.3 Создать K8s кластер на Timeweb

```
Timeweb Cloud → Kubernetes → Создать кластер

  Node Group 1 "app":         3 ноды × 4 ГБ = 12 ГБ
    → API, Consumer, Ingress, ArgoCD

  Node Group 2 "data":        3 ноды × 16 ГБ = 48 ГБ
    → PostgreSQL, Redis, Kafka, ClickHouse, Cassandra

  Node Group 3 "monitoring":  1 нода × 8 ГБ = 8 ГБ
    → Prometheus, Grafana, Alertmanager

  Итого: 7 нод, 68 ГБ RAM

Скачать kubeconfig из панели Timeweb → ~/.kube/config
```

```bash
kubectl get nodes
# → 7 нод Ready

helm version
# → v3.x
```

---

## 7.4 K8s манифесты для своего кода

### API Deployment

```yaml
# tinyurl-infra/apps/api/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tinyurl-api
  namespace: tinyurl
spec:
  replicas: 3
  selector:
    matchLabels: { app: tinyurl-api }
  template:
    metadata:
      labels: { app: tinyurl-api }
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
    spec:
      nodeSelector:
        node-group: app
      containers:
        - name: api
          image: registry.gitlab.com/yourname/tinyurl-app/api:latest
          ports: [{ containerPort: 8080 }]
          envFrom:
            - secretRef: { name: tinyurl-secrets }
          resources:
            requests: { cpu: "250m", memory: "256Mi" }
            limits:   { cpu: "1000m", memory: "512Mi" }
          readinessProbe:
            httpGet: { path: /health, port: 8080 }
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet: { path: /health, port: 8080 }
            initialDelaySeconds: 15
            periodSeconds: 20
```

### HPA (автоскейл от 3 до 10 подов)

```yaml
# tinyurl-infra/apps/api/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: tinyurl-api-hpa
  namespace: tinyurl
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: tinyurl-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target: { type: Utilization, averageUtilization: 70 }
```

### Ingress

```yaml
# tinyurl-infra/apps/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: tinyurl-ingress
  namespace: tinyurl
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
spec:
  tls:
    - hosts: [mylinks.ru]
      secretName: tinyurl-tls
  rules:
    - host: mylinks.ru
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service: { name: tinyurl-api, port: { number: 80 } }
```

---

## 7.5 Stateful сервисы через Helm (ставим сами, не managed)

### PostgreSQL + PGBouncer

```yaml
# tinyurl-infra/helm-values/postgres-values.yaml

auth:
  database: tinyurl
  username: app
  password: secret

primary:
  nodeSelector:
    node-group: data
  persistence:
    size: 50Gi

readReplicas:
  replicaCount: 1
  nodeSelector:
    node-group: data
  persistence:
    size: 50Gi

pgbouncer:
  enabled: true
  poolMode: transaction
  maxClientConnections: 500
  defaultPoolSize: 20
  reservePoolSize: 5
  replicas: 2
```

```bash
helm install postgres bitnami/postgresql -n tinyurl -f helm-values/postgres-values.yaml
```

**Подключение приложения:**
```
Без PGBouncer:  postgres://app:secret@postgres-primary:5432/tinyurl
С PGBouncer:    postgres://app:secret@postgres-pgbouncer:6432/tinyurl

PGBouncer мультиплексирует: 500 клиентских → 20 реальных к PostgreSQL.
HPA масштабирует API до 10 подов → 200+ соединений → PGBouncer держит.
```

**Метрики PGBouncer:**
```
pgbouncer_pools_client_active_connections
pgbouncer_pools_client_waiting_connections    ← если > 0, пул маловат
pgbouncer_pools_server_active_connections
pgbouncer_stats_avg_query_duration
```

**Алерт:**
```yaml
- alert: PGBouncerWaitingClients
  expr: pgbouncer_pools_client_waiting_connections > 10
  for: 2m
  annotations:
    summary: "{{ $value }} клиентов ждут — увеличить pool_size"
```

### Redis Cluster

```yaml
# tinyurl-infra/helm-values/redis-values.yaml

cluster:
  nodes: 6
  replicas: 1

persistence:
  size: 5Gi

redis:
  nodeSelector:
    node-group: data

maxmemory: 512mb
maxmemoryPolicy: allkeys-lru
```

```bash
helm install redis bitnami/redis-cluster -n tinyurl -f helm-values/redis-values.yaml
```

### Kafka (KRaft)

```yaml
# tinyurl-infra/helm-values/kafka-values.yaml

kraft:
  enabled: true

controller:
  replicaCount: 3
  nodeSelector:
    node-group: data
  persistence:
    size: 20Gi

broker:
  replicaCount: 3     # controller и broker на тех же подах в combined mode
```

```bash
helm install kafka bitnami/kafka -n tinyurl -f helm-values/kafka-values.yaml
```

### ClickHouse

```yaml
# tinyurl-infra/helm-values/clickhouse-values.yaml

shards: 1
replicaCount: 2

nodeSelector:
  node-group: data

persistence:
  size: 50Gi
```

```bash
helm install clickhouse bitnami/clickhouse -n tinyurl -f helm-values/clickhouse-values.yaml
```

### Cassandra

```yaml
# tinyurl-infra/helm-values/cassandra-values.yaml

replicaCount: 3

nodeSelector:
  node-group: data

persistence:
  size: 50Gi

cluster:
  name: tinyurl
  datacenter: dc1

resources:
  limits:
    memory: 4Gi
  requests:
    memory: 2Gi
```

```bash
helm install cassandra bitnami/cassandra -n tinyurl -f helm-values/cassandra-values.yaml
```

### Prometheus + Grafana

```bash
helm install monitoring prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace \
  -f helm-values/prometheus-values.yaml
```

---

## 7.6 GitLab CI/CD Pipeline

```yaml
# tinyurl-app/.gitlab-ci.yml

stages:
  - test
  - build
  - deploy

variables:
  REGISTRY: registry.gitlab.com/yourname/tinyurl-app

test:
  stage: test
  image: golang:1.22
  script:
    - go test ./... -v -race -cover

build-api:
  stage: build
  image: docker:24
  services: [docker:24-dind]
  script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - docker build -t $REGISTRY/api:$CI_COMMIT_SHORT_SHA -f Dockerfile.api .
    - docker push $REGISTRY/api:$CI_COMMIT_SHORT_SHA
  only: [main]

build-consumer:
  stage: build
  image: docker:24
  services: [docker:24-dind]
  script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - docker build -t $REGISTRY/consumer:$CI_COMMIT_SHORT_SHA -f Dockerfile.consumer .
    - docker push $REGISTRY/consumer:$CI_COMMIT_SHORT_SHA
  only: [main]

deploy:
  stage: deploy
  image: alpine:3.19
  script:
    - apk add --no-cache git
    - git clone https://oauth2:$INFRA_TOKEN@gitlab.com/yourname/tinyurl-infra.git
    - cd tinyurl-infra
    - sed -i "s|image:.*api:.*|image: ${REGISTRY}/api:${CI_COMMIT_SHORT_SHA}|" apps/api/deployment.yaml
    - sed -i "s|image:.*consumer:.*|image: ${REGISTRY}/consumer:${CI_COMMIT_SHORT_SHA}|" apps/consumer/deployment.yaml
    - git add . && git commit -m "Deploy ${CI_COMMIT_SHORT_SHA}" && git push
  only: [main]
```

### Поток деплоя

```
1. git push в tinyurl-app (main)
2. GitLab CI: go test → docker build → docker push
3. GitLab CI: обновляет image tag в tinyurl-infra
4. ArgoCD: видит изменение → kubectl apply
5. K8s: rolling update (старые поды → новые)
```

---

## 7.7 ArgoCD

```bash
# Установить
kubectl create namespace argocd
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Пароль
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d

# CLI
curl -sSL -o /usr/local/bin/argocd \
  https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
chmod +x /usr/local/bin/argocd

# Подключить репо
argocd repo add https://gitlab.com/yourname/tinyurl-infra.git \
  --username git --password $INFRA_TOKEN
```

### App of Apps

```yaml
# tinyurl-infra/argocd/app-of-apps.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tinyurl
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://gitlab.com/yourname/tinyurl-infra.git
    targetRevision: main
    path: apps
  destination:
    server: https://kubernetes.default.svc
    namespace: tinyurl
  syncPolicy:
    automated:
      prune: true        # удалять ресурсы, которых нет в Git
      selfHeal: true     # исправлять дрифт
    syncOptions:
      - CreateNamespace=true
```

```bash
kubectl apply -f argocd/app-of-apps.yaml
```

### Откат

```bash
# Через Git (правильный способ)
cd tinyurl-infra
git revert HEAD
git push
# → ArgoCD откатит кластер

# Через ArgoCD CLI
argocd app history tinyurl
argocd app rollback tinyurl 5
```

---

## 7.8 Итоговая K8s архитектура

```
Timeweb Managed K8s
├── Node Group "app" (3 × 4 ГБ)
│   ├── tinyurl-api         Deployment    3-10 подов (HPA)
│   ├── tinyurl-consumer    Deployment    3 пода
│   ├── nginx-ingress       DaemonSet     TLS termination
│   └── argocd              Deployment    3 пода
│
├── Node Group "data" (3 × 16 ГБ)
│   ├── postgresql          StatefulSet   primary + replica
│   ├── pgbouncer           Deployment    2 пода
│   ├── redis-cluster       StatefulSet   6 подов (3m + 3r)
│   ├── kafka               StatefulSet   3 пода (KRaft)
│   ├── clickhouse          StatefulSet   2 пода
│   └── cassandra           StatefulSet   3 пода
│
└── Node Group "monitoring" (1 × 8 ГБ)
    ├── prometheus           StatefulSet
    ├── grafana              Deployment
    └── alertmanager         Deployment

Всего: 7 нод, 68 ГБ RAM, ~30 подов
Стоимость: ~13 500 руб/мес
```

### ✅ Результат этапа 7

```
✅ Всё в K8s на Timeweb (ставим сами, не managed)
✅ PGBouncer мультиплексирует соединения (500 → 20)
✅ HPA автоскейл API от 3 до 10 подов
✅ GitLab CI: тесты → сборка → push
✅ ArgoCD: автодеплой из Git, откат = git revert
✅ Мониторинг через kube-prometheus-stack
✅ Один провайдер, один аккаунт
```

---

# Полезные команды

```bash
# PostgreSQL
kubectl exec -it postgres-0 -n tinyurl -- psql -U app -d tinyurl
SELECT count(*) FROM urls;

# PGBouncer
kubectl exec -it pgbouncer-0 -n tinyurl -- psql -p 6432 -U app pgbouncer
SHOW POOLS;
SHOW STATS;

# Redis
kubectl exec -it redis-0 -n tinyurl -- redis-cli
CLUSTER INFO
DBSIZE

# Kafka
kubectl exec -it kafka-0 -n tinyurl -- kafka-topics.sh --list --bootstrap-server localhost:9092
kubectl exec -it kafka-0 -n tinyurl -- kafka-consumer-groups.sh \
  --describe --group tinyurl-clicks --bootstrap-server localhost:9092

# Cassandra
kubectl exec -it cassandra-0 -n tinyurl -- cqlsh
SELECT count(*) FROM tinyurl.clicks;

# ClickHouse
kubectl exec -it clickhouse-0 -n tinyurl -- clickhouse-client
SELECT short_url, count() FROM tinyurl.clicks GROUP BY short_url ORDER BY count() DESC LIMIT 5;

# ArgoCD
argocd app list
argocd app get tinyurl
argocd app sync tinyurl

# Логи
kubectl logs -f deployment/tinyurl-api -n tinyurl
kubectl logs -f deployment/tinyurl-consumer -n tinyurl

# HPA статус
kubectl get hpa -n tinyurl
```

---

# Стоимость по этапам

```
Этапы 1-6:   Timeweb VPS 16 ГБ                     ~3000 руб/мес
             + домен .ru                            ~200 руб/год
             + день тестов (апгрейд до 64 ГБ)       ~300 руб разово

Этап 7:      Timeweb Managed K8s (7 нод, 68 ГБ)    ~13 500 руб/мес
             VPS выключаем

Итого на весь курс (~3 мес):
  3 мес VPS      = 9 000 руб
  Домен           = 200 руб
  Тесты           = 300 руб
  1 мес K8s      = 13 500 руб
  ──────────────────────────
  ~23 000 руб (весь проект)
```
