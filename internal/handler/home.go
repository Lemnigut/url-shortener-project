package handler

import (
	"net/http"

	"tinyurl/web"
)

// HomeHandler — хендлер главной страницы.
type HomeHandler struct {
	html []byte
}

func NewHomeHandler() *HomeHandler {
	data, err := web.StaticFS.ReadFile("static/index.html")
	if err != nil {
		panic("home: не удалось прочитать index.html: " + err.Error())
	}
	return &HomeHandler{html: data}
}

// Home отдаёт лендинг-страницу.
func (h *HomeHandler) Home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(h.html)
}
