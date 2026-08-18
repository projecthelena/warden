package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
	_ "github.com/projecthelena/warden/internal/docs"
	wardenmcp "github.com/projecthelena/warden/internal/mcp"
	"github.com/projecthelena/warden/internal/static"
	"github.com/projecthelena/warden/internal/uptime"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"golang.org/x/time/rate"
)

type Router struct {
	*chi.Mux
	manager *uptime.Manager
	store   *db.Store
	config  *config.Config
}

// SecurityHeaders middleware adds essential security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Permissions-Policy restricts access to browser features
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

// SecureHeadersWithConfig returns middleware that adds security headers including HSTS when HTTPS is enabled.
func SecureHeadersWithConfig(cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			// HSTS: Only enable when using secure cookies (HTTPS deployment)
			if cookieSecure {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewRouter builds the HTTP router serving both JSON APIs and static assets.
func NewRouter(manager *uptime.Manager, store *db.Store, cfg *config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)

	// SECURITY: Only trust X-Forwarded-For headers when behind a trusted reverse proxy.
	// If TrustProxy is false (default), we use the direct connection IP to prevent
	// attackers from spoofing their IP address and bypassing rate limiting.
	if cfg.TrustProxy {
		r.Use(RealIPFromProxy)
	}

	r.Use(SecureHeadersWithConfig(cfg.CookieSecure))

	// Rate limiter for general API requests (100 requests/second with burst of 200)
	// This is high enough to not interfere with normal usage but prevents abuse
	apiLimiter := NewIPRateLimiter(rate.Limit(100), 200)

	// Auth rate limiter: stricter in production, relaxed in dev/test mode
	// When ADMIN_SECRET is set, we're in dev/test mode (E2E tests, local dev)
	var authLimiter *IPRateLimiter
	if cfg.AdminSecret != "" {
		// Dev/test mode: 100 requests/second (effectively no limit for tests)
		authLimiter = NewIPRateLimiter(rate.Limit(100), 200)
	} else {
		// Production: strict limit (10 requests/minute with burst of 10)
		authLimiter = NewIPRateLimiter(rate.Limit(10.0/60.0), 10)
	}

	// Login-specific limiter with exponential backoff
	// Disabled in dev/test mode (when ADMIN_SECRET is set) to avoid E2E test failures
	var loginLimiter *LoginRateLimiter
	if cfg.AdminSecret == "" {
		loginLimiter = NewLoginRateLimiter()
	}

	// Base Router for setup methods attached to *Router
	apiRouter := &Router{
		Mux:     r,
		manager: manager,
		store:   store,
		config:  cfg,
	}

	// Instantiate Handlers
	authH := NewAuthHandler(store, cfg, loginLimiter)
	ssoH := NewSSOHandler(store, cfg)
	uptimeH := NewUptimeHandler(manager, store)
	crudH := NewCRUDHandler(store, manager)
	statsH := NewStatsHandler(store)
	settingsH := NewSettingsHandler(store, manager)
	apiKeyH := NewAPIKeyHandler(store)
	adminH := NewAdminHandler(store, manager, cfg)
	incidentH := NewIncidentHandler(store)
	maintH := NewMaintenanceHandler(store, manager)
	eventH := NewEventHandler(store, manager)
	insightH := NewInsightHandler(store)
	statusPageH := NewStatusPageHandler(store, manager, authH)
	notifH := NewNotificationChannelsHandler(store)
	userH := NewUserHandler(store)

	// Kubernetes health probes (unauthenticated, no rate limiting)
	r.Get("/healthz", Healthz)
	r.Get("/readyz", Readyz(store))

	r.Route("/api", func(api chi.Router) {
		// Apply general rate limiting to all API routes
		api.Use(RateLimitMiddleware(apiLimiter))

		// Public routes with stricter rate limiting for auth
		api.Group(func(auth chi.Router) {
			auth.Use(RateLimitMiddleware(authLimiter))
			auth.Post("/auth/login", authH.Login)
			auth.Post("/auth/logout", authH.Logout)
			auth.Get("/setup/status", apiRouter.CheckSetup)
			auth.Post("/setup", apiRouter.PerformSetup)

			// SSO routes (public)
			auth.Get("/auth/sso/status", ssoH.GetSSOStatus)
			auth.Get("/auth/sso/google", ssoH.GoogleLogin)
			auth.Get("/auth/sso/google/callback", ssoH.GoogleCallback)
		})

		// Public Status Pages
		api.Get("/s/{slug}", statusPageH.GetPublicStatus)
		api.Get("/s/{slug}/rss", statusPageH.GetRSSFeed)

		// API Documentation (Swagger UI)
		api.Get("/docs/*", httpSwagger.Handler(
			httpSwagger.URL("/api/docs/doc.json"),
		))
		// api.Get("/status", statusPageH.GetPublicStatus) // Legacy - omitting for now or map to default?

		// Admin operations (requires ADMIN_SECRET, not session auth)
		// This must be outside protected group so it works before any user exists
		api.Post("/admin/reset", adminH.ResetDatabase)

		api.Group(func(protected chi.Router) {
			protected.Use(authH.AuthMiddleware)

			// Auth endpoints - accessible by ALL authenticated users (including status_viewer)
			protected.Get("/auth/me", authH.Me)
			protected.Patch("/auth/me", authH.UpdateUser)
			protected.Get("/my/status-pages", statusPageH.GetMyStatusPages)

			// All other endpoints require at least viewer role (blocks status_viewer)
			protected.Group(func(dashboard chi.Router) {
				dashboard.Use(RequireViewerMiddleware)

				// Model Context Protocol. The tool set follows the API key's role: a
				// viewer key only ever sees the read tools, an editor key also gets the
				// ones that create and change things.
				if cfg.MCPEnabled {
					mcpServer := wardenmcp.NewServer(store, manager, Version, mcpWriter{crudH}, func(r *http.Request) bool {
						return hasPermission(getUserRole(r), RoleEditor)
					})
					dashboard.Mount("/mcp", requireSameOrigin(mcpServer.Handler()))
				}

				// Dashboard Overview
				dashboard.Get("/overview", uptimeH.GetOverview)

				// Groups
				dashboard.Post("/groups", crudH.CreateGroup)
				dashboard.Put("/groups/{id}", crudH.UpdateGroup)
				dashboard.Delete("/groups/{id}", crudH.DeleteGroup)

				// Monitors
				dashboard.Get("/uptime", uptimeH.GetHistory)
				dashboard.Post("/monitors", crudH.CreateMonitor)
				dashboard.Put("/monitors/{id}", crudH.UpdateMonitor)
				dashboard.Delete("/monitors/{id}", crudH.DeleteMonitor)
				dashboard.Post("/monitors/{id}/alerts", crudH.SetMonitorAlerts)
				dashboard.Post("/monitors/{id}/pause", crudH.PauseMonitor)
				dashboard.Post("/monitors/{id}/resume", crudH.ResumeMonitor)
				dashboard.Get("/monitors/{id}/uptime", uptimeH.GetMonitorUptime)
				dashboard.Get("/monitors/{id}/latency", uptimeH.GetMonitorLatency)
				dashboard.Get("/monitors/{id}/events", uptimeH.GetMonitorEvents)
				dashboard.Get("/monitors/{id}/insights", insightH.GetMonitorInsights)

				// Incidents
				dashboard.Get("/incidents", incidentH.GetIncidents)
				dashboard.Post("/incidents", incidentH.CreateIncident)
				dashboard.Get("/incidents/{id}", incidentH.GetIncident)
				dashboard.Put("/incidents/{id}", incidentH.UpdateIncident)
				dashboard.Delete("/incidents/{id}", incidentH.DeleteIncident)
				dashboard.Patch("/incidents/{id}/visibility", incidentH.SetVisibility)
				dashboard.Get("/incidents/{id}/updates", incidentH.GetUpdates)
				dashboard.Post("/incidents/{id}/updates", incidentH.AddUpdate)

				// Outages (promote to incident)
				dashboard.Post("/outages/{id}/promote", incidentH.PromoteOutage)

				// Maintenance
				dashboard.Post("/maintenance", maintH.CreateMaintenance)
				dashboard.Get("/maintenance", maintH.GetMaintenance)
				dashboard.Put("/maintenance/{id}", maintH.UpdateMaintenance)
				dashboard.Delete("/maintenance/{id}", maintH.DeleteMaintenance)

				// Settings
				dashboard.Get("/settings", settingsH.GetSettings)
				dashboard.Patch("/settings", settingsH.UpdateSettings)

				// SSO Settings (admin only)
				dashboard.Post("/settings/sso/test", ssoH.TestSSOConfig)

				// API Keys
				dashboard.Get("/api-keys", apiKeyH.ListKeys)
				dashboard.Post("/api-keys", apiKeyH.CreateKey)
				dashboard.Delete("/api-keys/{id}", apiKeyH.DeleteKey)

				// Stats
				dashboard.Get("/stats", statsH.GetStats)

				// Notifications
				dashboard.Get("/notifications/channels", notifH.GetChannels)
				dashboard.Post("/notifications/channels", notifH.CreateChannel)
				dashboard.Post("/notifications/channels/test", notifH.TestChannel)
				dashboard.Put("/notifications/channels/{id}", notifH.UpdateChannel)
				dashboard.Delete("/notifications/channels/{id}", notifH.DeleteChannel)

				// Events (for history)
				dashboard.Get("/events", eventH.GetSystemEvents)

				// Pattern findings across every monitor
				dashboard.Get("/insights", insightH.GetInsights)

				// Users (admin only)
				dashboard.Post("/users", userH.CreateUser)
				dashboard.Get("/users", userH.ListUsers)
				dashboard.Patch("/users/{id}/role", userH.UpdateUserRole)
				dashboard.Post("/users/{id}/password", userH.ResetUserPassword)
				dashboard.Delete("/users/{id}", userH.DeleteUser)
				dashboard.Get("/users/{id}/status-pages", userH.GetUserStatusPages)
				dashboard.Put("/users/{id}/status-pages", userH.SetUserStatusPages)

				// Status Pages Management
				dashboard.Get("/status-pages", statusPageH.GetAll)
				dashboard.Patch("/status-pages/{slug}", statusPageH.Toggle)
			})
		})
	})

	// Workaround for Vite Proxy stripping /api prefix for api-keys
	r.Group(func(r chi.Router) {
		r.Use(authH.AuthMiddleware)
		r.Get("/api-keys", apiKeyH.ListKeys)
		r.Post("/api-keys", apiKeyH.CreateKey)
		r.Delete("/api-keys/{id}", apiKeyH.DeleteKey)
	})

	// Static Assets (Frontend)
	r.Handle("/*", static.Handler())

	return r
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
