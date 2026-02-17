package router

import (
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	"gorm.io/gorm"

	"tinyurl/internal/config"
	"tinyurl/internal/handler"
	"tinyurl/internal/middleware"
	"tinyurl/internal/repository"
	"tinyurl/internal/service"
	"tinyurl/pkg/snowflake"
)

// New создаёт и настраивает chi-роутер со всеми маршрутами и middleware.
func New(cfg *config.Config, db *gorm.DB) chi.Router {
	sf, err := snowflake.New(cfg.App.SnowflakeNode)
	if err != nil {
		panic("роутер: ошибка инициализации snowflake: " + err.Error())
	}

	repo := repository.NewURLRepository(db)
	svc := service.NewURLService(repo, sf, cfg.App.BaseURL)

	shortenH := handler.NewShortenHandler(svc)
	redirectH := handler.NewRedirectHandler(svc)
	healthH := handler.NewHealthHandler(svc)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(middleware.Logging)

	r.Get("/health", healthH.Health)
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
	r.Post("/api/v1/shorten", shortenH.Shorten)
	r.Get("/{shortURL}", redirectH.Redirect)

	return r
}
