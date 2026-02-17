package handler

import (
	"net/http"

	"tinyurl/internal/dto"
)

// HealthHandler — хендлер проверки состояния сервиса.
type HealthHandler struct {
	svc URLService
}

func NewHealthHandler(svc URLService) *HealthHandler {
	return &HealthHandler{svc: svc}
}

// Health проверяет состояние API и подключение к БД.
// @Summary     Проверка здоровья
// @Description Возвращает статус API и подключения к базе данных.
// @Tags        система
// @Produce     json
// @Success     200 {object} dto.HealthResponse
// @Failure     503 {object} dto.HealthResponse
// @Router      /health [get]
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "connected"
	statusCode := http.StatusOK

	if err := h.svc.HealthCheck(r.Context()); err != nil {
		dbStatus = "disconnected"
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, dto.HealthResponse{
		Status: "ok",
		DB:     dbStatus,
	})
}
