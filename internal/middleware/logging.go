package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// wrappedWriter — обёртка над ResponseWriter для перехвата статус-кода.
type wrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *wrappedWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Logging — middleware для логирования HTTP-запросов.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &wrappedWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		slog.Info("запрос",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.statusCode,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}
