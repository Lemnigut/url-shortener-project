package dto

// ShortenResponse — ответ с короткой ссылкой.
type ShortenResponse struct {
	ShortURL string `json:"short_url"`
}

// HealthResponse — ответ проверки здоровья сервиса.
type HealthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

// ErrorResponse — ответ с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}
