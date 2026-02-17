package service

import (
	"context"
	"fmt"

	"tinyurl/internal/model"
	"tinyurl/internal/repository"
	"tinyurl/pkg/base62"
	"tinyurl/pkg/snowflake"
)

// URLService — сервис сокращения ссылок.
type URLService struct {
	repo    *repository.URLRepository
	sf      *snowflake.Generator
	baseURL string
}

// NewURLService создаёт новый экземпляр сервиса.
func NewURLService(repo *repository.URLRepository, sf *snowflake.Generator, baseURL string) *URLService {
	return &URLService{
		repo:    repo,
		sf:      sf,
		baseURL: baseURL,
	}
}

// ShortenResult — результат сокращения ссылки.
type ShortenResult struct {
	ShortURL string `json:"short_url"`
}

// Shorten сокращает длинный URL. Если URL уже был сокращён — возвращает существующий.
func (s *URLService) Shorten(ctx context.Context, longURL string) (*ShortenResult, error) {
	// Дедупликация: проверяем, существует ли уже такой URL
	existing, err := s.repo.FindByLongURL(ctx, longURL)
	if err != nil {
		return nil, fmt.Errorf("сервис: проверка существующего url: %w", err)
	}
	if existing != nil {
		return &ShortenResult{
			ShortURL: s.baseURL + "/" + existing.ShortURL,
		}, nil
	}

	// Генерация нового короткого URL
	id := s.sf.Generate()
	shortCode := base62.Encode(id)

	url := &model.URL{
		ID:       id,
		ShortURL: shortCode,
		LongURL:  longURL,
	}

	if err := s.repo.Create(ctx, url); err != nil {
		return nil, fmt.Errorf("сервис: создание url: %w", err)
	}

	return &ShortenResult{
		ShortURL: s.baseURL + "/" + shortCode,
	}, nil
}

// Resolve разрешает короткий код в оригинальный URL.
func (s *URLService) Resolve(ctx context.Context, shortCode string) (string, error) {
	url, err := s.repo.FindByShortURL(ctx, shortCode)
	if err != nil {
		return "", fmt.Errorf("сервис: разрешение url: %w", err)
	}
	if url == nil {
		return "", ErrNotFound
	}
	return url.LongURL, nil
}

// HealthCheck проверяет подключение к базе данных.
func (s *URLService) HealthCheck(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

// ErrNotFound — ошибка: URL не найден.
var ErrNotFound = fmt.Errorf("url не найден")
