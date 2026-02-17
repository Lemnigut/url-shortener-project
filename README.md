# TinyURL

Сервис сокращения ссылок на Go.

## Стек технологий

- **Роутер:** chi/v5
- **ORM:** GORM + PostgreSQL
- **Конфигурация:** koanf (YAML + переменные окружения)
- **Валидация:** go-playground/validator
- **Генерация ID:** Snowflake → base62
- **Документация:** Swagger (swaggo)
- **Task Runner:** [Task](https://taskfile.dev)

## Быстрый старт

### Требования

- Go 1.22+
- Docker
- [Task](https://taskfile.dev) (`go install github.com/go-task/task/v3/cmd/task@latest`)

### Запуск

```bash
# 1. Поднять PostgreSQL
task up

# 2. Запустить API-сервер (миграции выполняются автоматически через GORM AutoMigrate)
task run

# API доступен по адресу http://localhost:8080
# Swagger UI: http://localhost:8080/swagger/index.html
```

### Остановка

```bash
task down
```

## API

| Метод  | Эндпоинт             | Описание                          |
|--------|----------------------|-----------------------------------|
| POST   | `/api/v1/shorten`    | Создать короткую ссылку           |
| GET    | `/{shortURL}`        | Редирект на оригинальный URL (302)|
| GET    | `/health`            | Проверка здоровья сервиса         |
| GET    | `/swagger/*`         | Swagger UI                        |

### Примеры

```bash
# Сокращение ссылки
curl -X POST http://localhost:8080/api/v1/shorten \
  -H "Content-Type: application/json" \
  -d '{"long_url":"https://example.com/very/long/path"}'
# → {"short_url":"http://localhost:8080/2PV1ZxXo12W"}

# Редирект
curl -v http://localhost:8080/2PV1ZxXo12W
# → 302 Location: https://example.com/very/long/path

# Проверка здоровья
curl http://localhost:8080/health
# → {"status":"ok","db":"connected"}
```

## Конфигурация

Конфигурация загружается из YAML-файла, затем переопределяется переменными окружения.

По умолчанию используется `internal/config/configs/local.yaml`. Путь можно переопределить через `CONFIG_PATH`.

### Переменные окружения

| Переменная           | По умолчанию (local.yaml)  | Описание                    |
|----------------------|----------------------------|-----------------------------|
| `APP_PORT`           | `8080`                     | Порт сервера                |
| `APP_BASE_URL`       | `http://localhost:8080`    | Базовый URL для ссылок      |
| `APP_SNOWFLAKE_NODE` | `1`                        | ID узла Snowflake           |
| `POSTGRES_HOST`      | `localhost`                | Хост PostgreSQL             |
| `POSTGRES_PORT`      | `5432`                     | Порт PostgreSQL             |
| `POSTGRES_USER`      | `app`                      | Пользователь PostgreSQL     |
| `POSTGRES_PASSWORD`  | `secret`                   | Пароль PostgreSQL           |
| `POSTGRES_DB_NAME`   | `tinyurl`                  | Имя базы данных             |
| `POSTGRES_SSL_MODE`  | `disable`                  | Режим SSL                   |

## Миграции

Миграции выполняются автоматически при запуске приложения через GORM `AutoMigrate`.

Справочный SQL-файл: `migrations/postgres/001_init.sql` (для ручного использования при необходимости):

```bash
# Применить вручную (если не используется AutoMigrate)
docker exec -i <postgres-container> psql -U app -d tinyurl < migrations/postgres/001_init.sql
```

## Тесты

```bash
# Запустить все тесты
task test

# С подробным выводом
task test -- -v

# С детектором гонок
task test -- -race

# Отчёт покрытия (откроется в браузере)
task test-cover
```

## Все задачи

```bash
task --list
```

| Задача          | Описание                                    |
|-----------------|---------------------------------------------|
| `task up`       | Запустить PostgreSQL (Docker Compose)        |
| `task down`     | Остановить Docker Compose                   |
| `task logs`     | Просмотр логов контейнеров                  |
| `task run`      | Запустить API-сервер локально               |
| `task build`    | Собрать бинарный файл в `bin/api`           |
| `task test`     | Запустить тесты                             |
| `task test-cover` | Тесты + HTML-отчёт покрытия              |
| `task swag`     | Перегенерировать Swagger-документацию       |
| `task lint`     | Запустить линтер                            |
| `task lint-fix` | Запустить линтер с автоисправлением         |
| `task fmt`      | Форматировать код                           |
| `task tools`    | Установить все инструменты разработки       |

## Структура проекта

```
├── cmd/api/main.go          # Точка входа (минимальный)
├── internal/
│   ├── app/                 # Инициализация и жизненный цикл приложения
│   ├── config/              # Конфигурация (koanf: YAML + env)
│   ├── db/                  # Инициализация БД (GORM + AutoMigrate)
│   ├── router/              # Chi-роутер, регистрация маршрутов
│   ├── handler/             # HTTP-хендлеры
│   ├── service/             # Бизнес-логика
│   ├── repository/          # Слой доступа к данным (GORM)
│   ├── model/               # GORM-сущности
│   ├── dto/                 # DTO запросов/ответов
│   └── middleware/          # HTTP-middleware
├── pkg/
│   ├── base62/              # Кодирование/декодирование base62
│   └── snowflake/           # Генератор Snowflake ID
├── tests/                   # Все тесты
├── migrations/postgres/     # Справочные SQL-миграции
├── deploy/docker/           # Docker Compose, Dockerfile
├── docs/                    # Сгенерированная Swagger-документация
└── Taskfile.yml             # Конфигурация Task Runner
```
