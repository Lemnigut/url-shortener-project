package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tinyurl/internal/dto"
	"tinyurl/internal/service"
)

// RedirectHandler — хендлер редиректа по короткой ссылке.
type RedirectHandler struct {
	svc URLService
}

func NewRedirectHandler(svc URLService) *RedirectHandler {
	return &RedirectHandler{svc: svc}
}

// Redirect разрешает короткую ссылку и перенаправляет на оригинальный URL.
// @Summary     Редирект по короткой ссылке
// @Description Разрешает код короткой ссылки и выполняет 302-редирект на оригинальный URL.
// @Tags        urls
// @Param       shortURL path string true "Код короткой ссылки"
// @Success     302
// @Failure     404 {object} dto.ErrorResponse
// @Failure     500 {object} dto.ErrorResponse
// @Router      /{shortURL} [get]
func (h *RedirectHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortURL")
	if shortCode == "" {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "отсутствует короткая ссылка"})
		return
	}

	longURL, err := h.svc.Resolve(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: "короткая ссылка не найдена"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "не удалось разрешить ссылку"})
		return
	}

	http.Redirect(w, r, longURL, http.StatusFound)
}
