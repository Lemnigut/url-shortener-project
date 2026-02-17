package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"tinyurl/internal/model"
)

// URLRepository — репозиторий для работы с таблицей urls через GORM.
type URLRepository struct {
	db *gorm.DB
}

// NewURLRepository создаёт новый экземпляр репозитория.
func NewURLRepository(db *gorm.DB) *URLRepository {
	return &URLRepository{db: db}
}

// Create сохраняет новую запись URL в базу данных.
func (r *URLRepository) Create(ctx context.Context, url *model.URL) error {
	result := r.db.WithContext(ctx).Create(url)
	if result.Error != nil {
		return fmt.Errorf("репозиторий: создание url: %w", result.Error)
	}
	return nil
}

// FindByShortURL ищет запись по короткому коду.
func (r *URLRepository) FindByShortURL(ctx context.Context, shortURL string) (*model.URL, error) {
	var url model.URL
	result := r.db.WithContext(ctx).Where("short_url = ?", shortURL).First(&url)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("репозиторий: поиск по короткому url: %w", result.Error)
	}
	return &url, nil
}

// FindByLongURL ищет запись по оригинальному URL (для дедупликации).
func (r *URLRepository) FindByLongURL(ctx context.Context, longURL string) (*model.URL, error) {
	var url model.URL
	result := r.db.WithContext(ctx).Where("long_url = ?", longURL).First(&url)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("репозиторий: поиск по длинному url: %w", result.Error)
	}
	return &url, nil
}

// Ping проверяет доступность базы данных.
func (r *URLRepository) Ping(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("репозиторий: получение sql.DB: %w", err)
	}
	return sqlDB.PingContext(ctx)
}
