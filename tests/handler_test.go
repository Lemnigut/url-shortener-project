package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"tinyurl/internal/dto"
	"tinyurl/internal/handler"
	"tinyurl/internal/service"
)

// --- мок ---

type mockURLService struct {
	shortenFn     func(ctx context.Context, longURL string) (*service.ShortenResult, error)
	resolveFn     func(ctx context.Context, shortCode string) (string, error)
	healthCheckFn func(ctx context.Context) error
}

func (m *mockURLService) Shorten(ctx context.Context, longURL string) (*service.ShortenResult, error) {
	if m.shortenFn != nil {
		return m.shortenFn(ctx, longURL)
	}
	return nil, errors.New("не реализовано")
}

func (m *mockURLService) Resolve(ctx context.Context, shortCode string) (string, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, shortCode)
	}
	return "", errors.New("не реализовано")
}

func (m *mockURLService) HealthCheck(ctx context.Context) error {
	if m.healthCheckFn != nil {
		return m.healthCheckFn(ctx)
	}
	return errors.New("не реализовано")
}

// --- вспомогательные функции ---

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("ошибка декодирования ответа: %v", err)
	}
}

func chiRequest(method, path, paramKey, paramVal string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramKey, paramVal)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// --- сокращение ссылок ---

func TestShorten_Success(t *testing.T) {
	mock := &mockURLService{
		shortenFn: func(_ context.Context, _ string) (*service.ShortenResult, error) {
			return &service.ShortenResult{ShortURL: "http://localhost:8080/abc123"}, nil
		},
	}
	h := handler.NewShortenHandler(mock)

	body := `{"long_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Shorten(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusCreated)
	}

	var resp dto.ShortenResponse
	decodeJSON(t, rec, &resp)
	if resp.ShortURL != "http://localhost:8080/abc123" {
		t.Errorf("short_url = %q, ожидался %q", resp.ShortURL, "http://localhost:8080/abc123")
	}
}

func TestShorten_InvalidJSON(t *testing.T) {
	h := handler.NewShortenHandler(&mockURLService{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusBadRequest)
	}
}

func TestShorten_InvalidURL(t *testing.T) {
	h := handler.NewShortenHandler(&mockURLService{})

	body := `{"long_url":"not-a-url"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusBadRequest)
	}
}

func TestShorten_MissingURL(t *testing.T) {
	h := handler.NewShortenHandler(&mockURLService{})

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusBadRequest)
	}
}

func TestShorten_ServiceError(t *testing.T) {
	mock := &mockURLService{
		shortenFn: func(_ context.Context, _ string) (*service.ShortenResult, error) {
			return nil, errors.New("бд недоступна")
		},
	}
	h := handler.NewShortenHandler(mock)

	body := `{"long_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Shorten(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- редирект ---

func TestRedirect_Success(t *testing.T) {
	mock := &mockURLService{
		resolveFn: func(_ context.Context, code string) (string, error) {
			if code == "abc123" {
				return "https://example.com", nil
			}
			return "", service.ErrNotFound
		},
	}
	h := handler.NewRedirectHandler(mock)

	req := chiRequest(http.MethodGet, "/abc123", "shortURL", "abc123")
	rec := httptest.NewRecorder()

	h.Redirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com" {
		t.Errorf("Location = %q, ожидался %q", loc, "https://example.com")
	}
}

func TestRedirect_NotFound(t *testing.T) {
	mock := &mockURLService{
		resolveFn: func(_ context.Context, _ string) (string, error) {
			return "", service.ErrNotFound
		},
	}
	h := handler.NewRedirectHandler(mock)

	req := chiRequest(http.MethodGet, "/nonexistent", "shortURL", "nonexistent")
	rec := httptest.NewRecorder()

	h.Redirect(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusNotFound)
	}
}

func TestRedirect_ServiceError(t *testing.T) {
	mock := &mockURLService{
		resolveFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("бд недоступна")
		},
	}
	h := handler.NewRedirectHandler(mock)

	req := chiRequest(http.MethodGet, "/abc123", "shortURL", "abc123")
	rec := httptest.NewRecorder()

	h.Redirect(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- проверка здоровья ---

func TestHealth_OK(t *testing.T) {
	mock := &mockURLService{
		healthCheckFn: func(_ context.Context) error { return nil },
	}
	h := handler.NewHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusOK)
	}

	var resp dto.HealthResponse
	decodeJSON(t, rec, &resp)
	if resp.DB != "connected" {
		t.Errorf("db = %q, ожидался %q", resp.DB, "connected")
	}
}

func TestHealth_DBDown(t *testing.T) {
	mock := &mockURLService{
		healthCheckFn: func(_ context.Context) error { return errors.New("соединение отклонено") },
	}
	h := handler.NewHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("статус = %d, ожидался %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp dto.HealthResponse
	decodeJSON(t, rec, &resp)
	if resp.DB != "disconnected" {
		t.Errorf("db = %q, ожидался %q", resp.DB, "disconnected")
	}
}
