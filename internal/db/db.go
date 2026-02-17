package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"tinyurl/internal/model"
)

// Init подключается к PostgreSQL и выполняет автомиграцию схемы.
func Init(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("бд: ошибка подключения: %w", err)
	}

	if err := db.AutoMigrate(&model.URL{}); err != nil {
		return nil, fmt.Errorf("бд: ошибка миграции: %w", err)
	}

	return db, nil
}
