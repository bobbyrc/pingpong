package web

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bobbyrc/pingpong/internal/alerter"
	"github.com/prometheus/client_golang/prometheus"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var validEnvKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Handler serves the web UI pages, API endpoints, and static assets.
type Handler struct {
	pages       map[string]*template.Template // keyed by page filename
	broadcaster *Broadcaster
	queue       *alerter.Queue // may be nil
	envPath     string         // may be empty
}

// pageData is the data structure passed to all page templates.
type pageData struct {
	Title      string
	Active     string // "dashboard", "alerts", "config"
	Alerts     []alerter.Alert
	Page       int
	TotalPages int
}

// NewHandler creates a Handler that renders templates, broadcasts SSE
// metrics, and serves the config and alerts APIs.
func NewHandler(reg *prometheus.Registry, queue *alerter.Queue, envPath string) (*Handler, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}

	// Parse the layout as a base template, then clone it per page so that
	// each page's {{define "content"}} block doesn't overwrite the others.
	layout, err := template.New("layout.html").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html")
	if err != nil {
		return nil, err
	}

	pageFiles := []string{"dashboard.html", "alerts.html", "config.html"}
	pages := make(map[string]*template.Template, len(pageFiles))
	for _, pf := range pageFiles {
		clone, err := layout.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := clone.ParseFS(templateFS, "templates/"+pf); err != nil {
			return nil, err
		}
		pages[pf] = clone
	}

	b := NewBroadcaster(reg, 5*time.Second)

	return &Handler{
		pages:       pages,
		broadcaster: b,
		queue:       queue,
		envPath:     envPath,
	}, nil
}

// RegisterRoutes registers all web UI and API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Page routes
	mux.HandleFunc("GET /", h.dashboardPage)
	mux.HandleFunc("GET /config", h.configPage)
	mux.HandleFunc("GET /alerts", h.alertsPage)

	// API routes
	mux.Handle("GET /api/events", h.broadcaster)
	mux.HandleFunc("GET /api/config", h.configAPI)
	mux.HandleFunc("POST /api/config", h.configAPI)
	mux.HandleFunc("GET /api/alerts", h.alertsAPI)

	// Static files
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		slog.Error("failed to create static sub-filesystem", "error", err)
	} else {
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))
	}
}

// Start launches the SSE broadcaster in a background goroutine.
func (h *Handler) Start(ctx context.Context) {
	go h.broadcaster.Run(ctx)
}

// dashboardPage renders the main dashboard. Only matches exact "/" path.
func (h *Handler) dashboardPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.pages["dashboard.html"].ExecuteTemplate(w, "layout.html", pageData{
		Title:  "Dashboard",
		Active: "dashboard",
	}); err != nil {
		slog.Error("failed to render dashboard", "error", err)
	}
}

// configPage renders the configuration editor page.
func (h *Handler) configPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.pages["config.html"].ExecuteTemplate(w, "layout.html", pageData{
		Title:  "Config",
		Active: "config",
	}); err != nil {
		slog.Error("failed to render config page", "error", err)
	}
}

// alertsPage renders the paginated alert history page.
func (h *Handler) alertsPage(w http.ResponseWriter, r *http.Request) {
	const perPage = 20

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	data := pageData{
		Title:  "Alerts",
		Active: "alerts",
		Page:   page,
	}

	if h.queue != nil {
		alerts, total, err := h.queue.RecentAlerts(perPage, offset)
		if err != nil {
			slog.Error("failed to query alerts", "error", err)
		} else {
			data.Alerts = alerts
			data.TotalPages = int(math.Ceil(float64(total) / float64(perPage)))
			if data.TotalPages < 1 {
				data.TotalPages = 1
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.pages["alerts.html"].ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("failed to render alerts page", "error", err)
	}
}

// jsonError writes a JSON-encoded error response with the given status code.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// configAPI handles GET (read env file) and POST (write env file) for the
// configuration API.
func (h *Handler) configAPI(w http.ResponseWriter, r *http.Request) {
	if h.envPath == "" {
		jsonError(w, "no env file configured", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		env, err := ReadEnvFile(h.envPath)
		if err != nil {
			slog.Error("failed to read env file", "error", err)
			jsonError(w, "failed to read env file", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(env)

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

		var updates map[string]string
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			jsonError(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		for k, v := range updates {
			if !validEnvKey.MatchString(k) || strings.ContainsAny(v, "\n\r") {
				jsonError(w, "invalid key or value", http.StatusBadRequest)
				return
			}
		}

		if err := WriteEnvFile(h.envPath, updates); err != nil {
			slog.Error("failed to write env file", "error", err)
			jsonError(w, "failed to write env file", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// alertsAPI returns paginated alerts as JSON.
func (h *Handler) alertsAPI(w http.ResponseWriter, r *http.Request) {
	if h.queue == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"alerts": []interface{}{},
			"total":  0,
		})
		return
	}

	const perPage = 20
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	alerts, total, err := h.queue.RecentAlerts(perPage, offset)
	if err != nil {
		slog.Error("failed to query alerts API", "error", err)
		jsonError(w, "failed to query alerts", http.StatusInternalServerError)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts":     alerts,
		"total":      total,
		"page":       page,
		"totalPages": totalPages,
	})
}
