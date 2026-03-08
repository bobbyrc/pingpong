package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDashboardReturnsHTML(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %q", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "PingPong") {
		t.Error("expected body to contain 'PingPong'")
	}
}

func TestDashboardNotFoundForOtherPaths(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestConfigAPIRoundTrip(t *testing.T) {
	// Create temp .env file
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\nBAZ=qux\n"), 0644); err != nil {
		t.Fatalf("write temp env: %v", err)
	}

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, envPath)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// GET /api/config — verify returns JSON with env vars
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config: expected 200, got %d", rec.Code)
	}

	var env map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if env["FOO"] != "bar" || env["BAZ"] != "qux" {
		t.Errorf("unexpected env: %v", env)
	}

	// POST /api/config with updates
	body := strings.NewReader(`{"FOO":"updated","NEW_KEY":"new_val"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/config: expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// GET /api/config again — verify updates are reflected
	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode second GET response: %v", err)
	}
	if env["FOO"] != "updated" {
		t.Errorf("expected FOO=updated, got %q", env["FOO"])
	}
	if env["NEW_KEY"] != "new_val" {
		t.Errorf("expected NEW_KEY=new_val, got %q", env["NEW_KEY"])
	}
	if env["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux preserved, got %q", env["BAZ"])
	}
}

func TestConfigAPINoEnvPath(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no env path, got %d", rec.Code)
	}
}

func TestStaticFileServing(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "PingPong") {
		t.Error("expected CSS file to contain 'PingPong' comment")
	}
}

func TestAlertsPageNilQueue(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Alert History") {
		t.Error("expected alerts page to contain 'Alert History'")
	}
}

func TestAlertsAPINilQueue(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["total"].(float64) != 0 {
		t.Errorf("expected total=0, got %v", resp["total"])
	}
}
