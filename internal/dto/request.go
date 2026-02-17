package dto

// ShortenRequest — запрос на сокращение ссылки.
type ShortenRequest struct {
	LongURL string `json:"long_url" validate:"required,url"`
}
