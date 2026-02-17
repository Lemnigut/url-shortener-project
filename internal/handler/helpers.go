package handler

import (
	"encoding/json"
	"net/http"
)

// writeJSON записывает JSON-ответ с указанным статус-кодом.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
