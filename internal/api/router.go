package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/static"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

// NewRouter builds the HTTP router serving both JSON APIs and static assets.
func NewRouter(manager *uptime.Manager, store *db.Store) http.Handler {
	uptimeH := NewUptimeHandler(manager, store)
	authH := NewAuthHandler(store)
	statusPageH := NewStatusPageHandler(store, manager)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	// Update CORS to allow credentials
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"}, // Add frontend dev URL
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(api chi.Router) {
		// Public Routes
		api.Get("/health", Health)
		api.Get("/status", uptimeH.GetHistory) // Legacy singular endpoint? Maybe repurpose or keep.
		// Actually, let's keep it for now as "public data" if authentication is not enforced there yet?
		// Wait, checking line 37: api.Get("/status", uptimeH.GetHistory) // Public status page data
		// This endpoints logic inside uptimeH.GetHistory doesn't check for status_pages config yet.
		// We will need to guard this or create new endpoints.

		api.Get("/s/{slug}", statusPageH.GetPublicStatus) // New public endpoint for status pages

		// Public Auth Routes
		api.Post("/auth/login", authH.Login)
		api.Post("/auth/logout", authH.Logout)

		// Protected Routes
		api.Group(func(protected chi.Router) {
			protected.Use(authH.AuthMiddleware)
			protected.Get("/auth/me", authH.Me)
			protected.Patch("/auth/me", authH.UpdateUser)
			protected.Get("/uptime", uptimeH.GetHistory)                          // Dashboard data (authenticated)
			protected.Get("/monitors/{id}/uptime", uptimeH.GetMonitorUptimeStats) // Uptime Stats

			// CRUD operations
			crudH := NewCRUDHandler(store, manager)
			protected.Route("/groups", func(r chi.Router) {
				r.Post("/", crudH.CreateGroup)
				r.Get("/", crudH.GetGroups)
				r.Delete("/{id}", crudH.DeleteGroup)
				r.Put("/{id}", crudH.UpdateGroup)
			})
			protected.Post("/monitors", crudH.CreateMonitor)
			protected.Put("/monitors/{id}", crudH.UpdateMonitor)
			protected.Delete("/monitors/{id}", crudH.DeleteMonitor)

			// Status Pages Management
			protected.Get("/status-pages", statusPageH.GetAll)
			protected.Patch("/status-pages/{slug}", statusPageH.Toggle)

			// API Keys
			apiKeyH := NewAPIKeyHandler(store)
			protected.Get("/api-keys", apiKeyH.ListKeys)
			protected.Post("/api-keys", apiKeyH.CreateKey)
			protected.Delete("/api-keys/{id}", apiKeyH.DeleteKey)

			// Admin
			adminH := NewAdminHandler(store, manager)
			protected.Post("/admin/reset", adminH.ResetDatabase)

			// Settings
			settingsH := NewSettingsHandler(store, manager)
			protected.Get("/settings", settingsH.GetSettings)
			protected.Patch("/settings", settingsH.UpdateSettings)
		})
	})

	r.Handle("/*", static.Handler())

	return r
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
