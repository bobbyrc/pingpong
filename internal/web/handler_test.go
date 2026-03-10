package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bobbyrc/pingpong/internal/alerter"
	"github.com/prometheus/client_golang/prometheus"
)

func TestDashboardReturnsHTML(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, nil, "")
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
	h, err := NewHandler(reg, nil, nil, "")
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
	h, err := NewHandler(reg, nil, nil, envPath)
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
	h, err := NewHandler(reg, nil, nil, "")
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
	h, err := NewHandler(reg, nil, nil, "")
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
	h, err := NewHandler(reg, nil, nil, "")
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

func TestResolvePerPage(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		cookie   string
		expected int
	}{
		{"default", "", "", 20},
		{"query param", "?perPage=50", "", 50},
		{"cookie", "", "50", 50},
		{"query overrides cookie", "?perPage=10", "50", 10},
		{"invalid query falls to cookie", "?perPage=abc", "30", 30},
		{"zero query falls to cookie", "?perPage=0", "30", 30},
		{"negative query falls to default", "?perPage=-5", "", 20},
		{"over max falls to cookie", "?perPage=101", "30", 30},
		{"over max falls to default", "?perPage=200", "", 20},
		{"invalid cookie falls to default", "", "abc", 20},
		{"boundary lower", "?perPage=1", "", 1},
		{"boundary upper", "?perPage=100", "", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/alerts"+tt.query, nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "pingpong_alerts_per_page", Value: tt.cookie})
			}
			got := resolvePerPage(req)
			if got != tt.expected {
				t.Errorf("resolvePerPage() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestConfigAPIMissingEnvFile(t *testing.T) {
	// Point envPath at a valid directory but non-existent file.
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, nil, envPath)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for missing env file, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var env map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(env) != 0 {
		t.Errorf("expected empty JSON object, got %v", env)
	}
}

func TestAlertsAPINilQueue(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, nil, "")
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

func TestHistoryAPINilStore(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty object, got %v", resp)
	}
}

func TestHistoryAPIWithData(t *testing.T) {
	db := openTestDB(t)
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}

	store.Record("ping_latency", "1.1.1.1", 12.5)
	store.Record("ping_latency", "1.1.1.1", 13.0)
	store.Record("download_speed", "", 95.2)

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, store, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Parse the nested structure
	var resp map[string]map[string][]HistoryPoint
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	pingData, ok := resp["ping_latency"]
	if !ok {
		t.Fatal("missing ping_latency in response")
	}
	points := pingData["1.1.1.1"]
	if len(points) != 2 {
		t.Fatalf("expected 2 ping points, got %d", len(points))
	}
	if points[0].Value != 12.5 {
		t.Errorf("first ping value = %v, want 12.5", points[0].Value)
	}

	dlData, ok := resp["download_speed"]
	if !ok {
		t.Fatal("missing download_speed in response")
	}
	if len(dlData[""]) != 1 {
		t.Fatalf("expected 1 download point, got %d", len(dlData[""]))
	}
}

func TestDeleteAlertAPI(t *testing.T) {
	db := openTestDB(t)
	q, err := alerter.NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	if err := q.Enqueue("key1", "latency", "Alert 1", "Body 1"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q.Enqueue("key2", "speed", "Alert 2", "Body 2"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, q, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Get alerts to find an ID
	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/alerts: expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode GET /api/alerts response: %v", err)
	}
	alerts, ok := resp["alerts"].([]interface{})
	if !ok || len(alerts) == 0 {
		t.Fatal("expected non-empty alerts array")
	}
	first, ok := alerts[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected alert object")
	}
	alertID := int64(first["ID"].(float64))

	// DELETE single alert
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/alerts/%d", alertID), nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Verify only 1 remains
	req = httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode verification response: %v", err)
	}
	if resp["total"].(float64) != 1 {
		t.Errorf("expected 1 alert remaining, got %v", resp["total"])
	}
}

func TestDeleteAllAlertsAPI(t *testing.T) {
	db := openTestDB(t)
	q, err := alerter.NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	if err := q.Enqueue("key1", "latency", "Alert 1", "Body 1"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q.Enqueue("key2", "speed", "Alert 2", "Body 2"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, q, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Verify 0 remain
	req = httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode verification response: %v", err)
	}
	if resp["total"].(float64) != 0 {
		t.Errorf("expected 0 alerts, got %v", resp["total"])
	}
}

func TestDeleteAlertAPIBadID(t *testing.T) {
	db := openTestDB(t)
	q, err := alerter.NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, q, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/alerts/abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric ID, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteAllAlertsAPINilQueue(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 with nil queue, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestConfigAPIInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\n"), 0644); err != nil {
		t.Fatalf("write temp env: %v", err)
	}

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, nil, envPath)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := strings.NewReader(`{invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAlertsPageWithData(t *testing.T) {
	db := openTestDB(t)
	q, err := alerter.NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	if err := q.Enqueue("key1", "latency", "High Latency Alert", "Latency exceeded threshold"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, q, nil, "")
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
	if !strings.Contains(body, "High Latency Alert") {
		t.Error("expected alerts page to contain the alert title 'High Latency Alert'")
	}
}

func TestAlertsAPIPagination(t *testing.T) {
	db := openTestDB(t)
	q, err := alerter.NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := q.Enqueue(fmt.Sprintf("key%d", i), "latency", fmt.Sprintf("Alert %d", i), "body"); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, q, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/alerts?page=1&perPage=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	totalPages := int(resp["totalPages"].(float64))
	if totalPages != 3 {
		t.Errorf("expected totalPages=3, got %d", totalPages)
	}

	alerts := resp["alerts"].([]interface{})
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts on page 1, got %d", len(alerts))
	}
}

func TestDeleteAlertAPINilQueue(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, err := NewHandler(reg, nil, nil, "")
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/alerts/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 with nil queue, got %d", rec.Code)
	}
}
