package main

import (
	"log/slog"
	"os"

	_ "tinyurl/docs"
	"tinyurl/internal/app"
)

// @title       TinyURL API
// @version     1.0
// @description Сервис сокращения ссылок.
// @host        localhost:8080
// @BasePath    /
func main() {
	a, err := app.Init()
	if err != nil {
		slog.Error("ошибка инициализации приложения", "error", err)
		os.Exit(1)
	}

	a.Run()
}
