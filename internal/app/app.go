package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"

	"tinyurl/internal/config"
	"tinyurl/internal/db"
	"tinyurl/internal/router"
)

// Application — основная структура приложения, содержащая конфигурацию, БД и HTTP-сервер.
type Application struct {
	cfg    *config.Config
	db     *gorm.DB
	server *http.Server
}

// Init инициализирует приложение: загружает конфигурацию, подключается к БД, создаёт HTTP-сервер.
func Init() (*Application, error) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	slog.Info("конфигурация загружена",
		"port", cfg.App.Port,
		"base_url", cfg.App.BaseURL,
		"snowflake_node", cfg.App.SnowflakeNode,
	)

	database, err := db.Init(cfg.Postgres.DSN())
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации базы данных: %w", err)
	}
	slog.Info("база данных готова")

	app := &Application{
		cfg: cfg,
		db:  database,
		server: &http.Server{
			Addr:    ":" + cfg.App.Port,
			Handler: router.New(cfg, database),
		},
	}

	return app, nil
}

// Run запускает HTTP-сервер и ожидает сигнала завершения.
func (app *Application) Run() {
	defer app.cleanup()

	go func() {
		slog.Info("запуск сервера", "addr", app.server.Addr)
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("ошибка сервера", "error", err)
			os.Exit(1)
		}
	}()

	app.waitForShutdown()
}

// waitForShutdown ожидает SIGINT/SIGTERM и выполняет graceful shutdown.
func (app *Application) waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("остановка сервера...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.server.Shutdown(ctx); err != nil {
		slog.Error("ошибка при остановке сервера", "error", err)
	}

	slog.Info("сервер остановлен")
}

// cleanup освобождает ресурсы (закрывает соединение с БД).
func (app *Application) cleanup() {
	if app.db != nil {
		sqlDB, err := app.db.DB()
		if err != nil {
			return
		}
		if err := sqlDB.Close(); err != nil {
			slog.Error("ошибка закрытия соединения с БД", "error", err)
		}
	}
}
