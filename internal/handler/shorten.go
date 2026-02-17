package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"

	"tinyurl/internal/dto"
)

// ShortenHandler — хендлер создания короткой ссылки.
type ShortenHandler struct {
	svc      URLService
	validate *validator.Validate
}

func NewShortenHandler(svc URLService) *ShortenHandler {
	return &ShortenHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Shorten создаёт короткую ссылку из длинного URL.
// @Summary     Сокращение ссылки
// @Description Создаёт короткую ссылку. Если длинный URL уже сокращался — возвращает существующую.
// @Tags        urls
// @Accept      json
// @Produce     json
// @Param       request body     dto.ShortenRequest  true "Длинный URL для сокращения"
// @Success     201     {object} dto.ShortenResponse
// @Failure     400     {object} dto.ErrorResponse
// @Failure     500     {object} dto.ErrorResponse
// @Router      /api/v1/shorten [post]
func (h *ShortenHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	var req dto.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "некорректное тело запроса"})
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "некорректный или отсутствующий long_url"})
		return
	}

	result, err := h.svc.Shorten(r.Context(), req.LongURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "не удалось сократить ссылку"})
		return
	}

	writeJSON(w, http.StatusCreated, dto.ShortenResponse{ShortURL: result.ShortURL})
}
