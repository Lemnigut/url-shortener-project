package model

import "time"

// URL — модель таблицы urls в базе данных.
type URL struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	ShortURL  string    `gorm:"uniqueIndex;size:12;not null" json:"short_url"`
	LongURL   string    `gorm:"not null;index:idx_long_url" json:"long_url"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName возвращает имя таблицы в БД.
func (URL) TableName() string {
	return "urls"
}
