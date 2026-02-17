package handler

import (
	"context"

	"tinyurl/internal/service"
)

// URLService — интерфейс сервиса сокращения ссылок.
// Хендлеры зависят от интерфейса, а не от конкретной реализации,
// что позволяет подставлять моки в тестах.
type URLService interface {
	Shorten(ctx context.Context, longURL string) (*service.ShortenResult, error)
	Resolve(ctx context.Context, shortCode string) (string, error)
	HealthCheck(ctx context.Context) error
}
