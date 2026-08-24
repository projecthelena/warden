package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

type CRUDHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewCRUDHandler(store *db.Store, manager *uptime.Manager) *CRUDHandler {
	return &CRUDHandler{store: store, manager: manager}
}

// generateID creates a slug + hash ID from a name
// e.g. "My Group" -> "g-my-group-a1b2c3"
func generateID(name, prefix string) string {
	slug := generateSlug(name, prefix)

	// 2. Generate random suffix (3 bytes = 6 hex chars)
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return slug + "-rnd"
	}
	hash := hex.EncodeToString(b)

	return slug + "-" + hash
}

// generateSlug creates a clean slug ID from a name without hash
// e.g. "My Group" -> "g-my-group"
func generateSlug(name, prefix string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	reg := regexp.MustCompile("[^a-z0-9-]+")
	slug = reg.ReplaceAllString(slug, "")
	return prefix + slug
}

// maxNameLength is the maximum allowed length for names (groups, monitors)
const maxNameLength = 255

// CreateGroup creates a new monitor group.
// @Summary      Create group
// @Tags         groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string} true "Group payload"
// @Success      201  {object} db.Group
// @Failure      400  {string} string "Name is required"
// @Failure      409  {object} object{error=string} "Group already exists"
// @Router       /groups [post]
func (h *CRUDHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate name length
	if len(req.Name) > maxNameLength {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	id := generateSlug(req.Name, "g-")

	g := db.Group{
		ID:   id,
		Name: req.Name,
	}

	if err := h.store.CreateGroup(g); err != nil {
		// Handle Duplicate ID error
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "Group with this name already exists (ID: "+id+")")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(g)
}

// DeleteGroup deletes a monitor group by ID.
// @Summary      Delete group
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Group ID"
// @Success      200  "OK"
// @Failure      400  {string} string "ID required"
// @Router       /groups/{id} [delete]
func (h *CRUDHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteGroup(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.manager.Sync()
	w.WriteHeader(http.StatusOK)
}

// UpdateGroup renames a monitor group.
// @Summary      Update group
// @Tags         groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Group ID"
// @Param        body body object{name=string} true "New name"
// @Success      200  {object} object{name=string}
// @Failure      400  {string} string "Name is required"
// @Router       /groups/{id} [put]
func (h *CRUDHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate name length
	if len(req.Name) > maxNameLength {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateGroup(id, req.Name); err != nil {
		http.Error(w, "Failed to update group", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(req)
}

// CreatedMonitor is the create response: the stored monitor, plus the reason when its
// first check failed.
type CreatedMonitor struct {
	db.Monitor
	Warning string `json:"warning,omitempty"`
}

// CreateMonitor creates a new monitor of the requested check type.
// @Summary      Create monitor
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,type=string,url=string,groupId=string,interval=int} true "Monitor payload. type is one of http, tcp, ping, dns (default http)"
// @Success      201  {object} CreatedMonitor
// @Failure      400  {string} string "Validation error"
// @Failure      404  {string} string "Group not found"
// @Failure      409  {string} string "Monitor name already exists"
// @Router       /monitors [post]
func (h *CRUDHandler) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	var req struct {
		Name                    string            `json:"name"`
		Type                    string            `json:"type"`
		URL                     string            `json:"url"`
		GroupID                 string            `json:"groupId"`
		Interval                int               `json:"interval"`
		ConfirmationThreshold   *int              `json:"confirmationThreshold,omitempty"`
		NotificationCooldownMin *int              `json:"notificationCooldownMinutes,omitempty"`
		LatencyThreshold        *int              `json:"latencyThreshold,omitempty"`
		RequestConfig           *db.RequestConfig `json:"requestConfig,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m, err := h.AddMonitor(MonitorInput{
		Name:                    req.Name,
		Type:                    req.Type,
		URL:                     req.URL,
		GroupID:                 req.GroupID,
		Interval:                req.Interval,
		ConfirmationThreshold:   req.ConfirmationThreshold,
		NotificationCooldownMin: req.NotificationCooldownMin,
		LatencyThreshold:        req.LatencyThreshold,
		RequestConfig:           req.RequestConfig,
	})
	if err != nil {
		writeMonitorError(w, err)
		return
	}

	// Wait for the first ping results (max 5 seconds) to ensure "Wow effect" in UI
	// This ensures that when the frontend fetches the list immediately after this returns,
	// the first check is likely already done.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mon := h.manager.GetMonitor(m.ID)
		if mon != nil && len(mon.GetHistory()) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// We waited for that first check anyway, so report it when it failed rather than
	// leaving a monitor to sit red on the dashboard with the reason a few clicks away.
	warning := ""
	if mon := h.manager.GetMonitor(m.ID); mon != nil {
		if history := mon.GetHistory(); len(history) > 0 {
			if last := history[len(history)-1]; !last.IsUp && last.Error != "" {
				warning = last.Error
			}
		}
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CreatedMonitor{Monitor: m, Warning: warning})
}

// writeMonitorError maps the shared creation errors onto the status codes the API has
// always returned for them.
func writeMonitorError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrMonitorInvalid):
		http.Error(w, strings.TrimPrefix(err.Error(), "invalid monitor: "), http.StatusBadRequest)
	case errors.Is(err, ErrGroupNotFound):
		http.Error(w, "Selected group does not exist", http.StatusNotFound)
	case errors.Is(err, ErrMonitorNameTaken):
		http.Error(w, "A monitor with this name already exists", http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *CRUDHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.GetGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(groups)
}

// UpdateMonitor updates a monitor's name, type, target or interval.
// @Summary      Update monitor
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Param        body body object{name=string,type=string,url=string,interval=int} true "Fields to update. Omit type to keep the stored one"
// @Success      200  "OK"
// @Failure      400  {string} string "ID required"
// @Router       /monitors/{id} [put]
func (h *CRUDHandler) UpdateMonitor(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Name                    string            `json:"name"`
		Type                    string            `json:"type"`
		URL                     string            `json:"url"`
		Interval                int               `json:"interval"`
		ConfirmationThreshold   *int              `json:"confirmationThreshold,omitempty"`
		NotificationCooldownMin *int              `json:"notificationCooldownMinutes,omitempty"`
		LatencyThreshold        *int              `json:"latencyThreshold,omitempty"`
		RequestConfig           *db.RequestConfig `json:"requestConfig,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// An update that doesn't mention the type keeps the stored one, so an API client
	// that only changes the name can't silently turn a ping monitor into an HTTP one.
	monitorType := req.Type
	if monitorType == "" {
		monitorType = h.storedMonitorType(id)
	}
	if !db.IsValidMonitorType(monitorType) {
		http.Error(w, "type must be one of http, tcp, ping, dns", http.StatusBadRequest)
		return
	}
	if err := validateTarget(monitorType, req.URL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate per-monitor overrides
	if req.ConfirmationThreshold != nil && (*req.ConfirmationThreshold < 1 || *req.ConfirmationThreshold > 100) {
		http.Error(w, "confirmationThreshold must be between 1 and 100", http.StatusBadRequest)
		return
	}
	if req.NotificationCooldownMin != nil && (*req.NotificationCooldownMin < 0 || *req.NotificationCooldownMin > 1440) {
		http.Error(w, "notificationCooldownMinutes must be between 0 and 1440", http.StatusBadRequest)
		return
	}
	if req.LatencyThreshold != nil && *req.LatencyThreshold < 1 {
		http.Error(w, "latencyThreshold must be at least 1", http.StatusBadRequest)
		return
	}

	if err := validateRequestConfig(req.RequestConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateMonitor(id, monitorType, req.Name, req.URL, req.Interval, req.ConfirmationThreshold, req.NotificationCooldownMin, req.LatencyThreshold, req.RequestConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.manager.Sync()
	w.WriteHeader(http.StatusOK)
}

// DeleteMonitor removes a monitor and its history.
// @Summary      Delete monitor
// @Tags         monitors
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Success      200  "OK"
// @Failure      400  {string} string "ID required"
// @Router       /monitors/{id} [delete]
func (h *CRUDHandler) DeleteMonitor(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteMonitor(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.manager.RemoveMonitor(id)
	h.manager.Sync()
	w.WriteHeader(http.StatusOK)
}

// SetMonitorAlerts mutes or unmutes a monitor's alerts. A muted monitor is still checked,
// still records outages and still appears in the daily digest and on the dashboard — it
// simply never interrupts anyone. Meant for test and staging targets.
// @Summary      Mute or unmute a monitor's alerts
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path string true "Monitor ID"
// @Param        body  body object{muted=bool} true "Mute state"
// @Success      200  {object} object{message=string,alertsMuted=bool}
// @Failure      400  {object} object{error=string} "ID required or invalid body"
// @Failure      404  {object} object{error=string} "Monitor not found"
// @Router       /monitors/{id}/alerts [post]
func (h *CRUDHandler) SetMonitorAlerts(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID required")
		return
	}

	var body struct {
		Muted bool `json:"muted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	monitors, err := h.store.GetMonitors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load monitors")
		return
	}
	found := false
	for _, m := range monitors {
		if m.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}

	if err := h.store.SetMonitorAlertsMuted(id, body.Muted); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update monitor alerts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "monitor alerts updated", "alertsMuted": body.Muted})
}

// SetMonitorGroup moves a monitor into another group. Like pausing and muting, it takes
// effect on its own rather than as part of an edit: the monitor keeps its id, so its
// history, uptime and incidents move with it and nothing else about it changes.
// @Summary      Move a monitor to another group
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path string true "Monitor ID"
// @Param        body  body object{groupId=string} true "Destination group"
// @Success      200  {object} object{message=string,groupId=string}
// @Failure      400  {object} object{error=string} "ID required, invalid body or missing groupId"
// @Failure      404  {object} object{error=string} "Monitor or group not found"
// @Router       /monitors/{id}/group [post]
func (h *CRUDHandler) SetMonitorGroup(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID required")
		return
	}

	var body struct {
		GroupID string `json:"groupId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.MoveMonitor(id, body.GroupID); err != nil {
		switch {
		case errors.Is(err, db.ErrMonitorNotFound):
			writeError(w, http.StatusNotFound, "monitor not found")
		case errors.Is(err, ErrGroupNotFound):
			writeError(w, http.StatusNotFound, "selected group does not exist")
		case errors.Is(err, ErrMonitorInvalid):
			writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "invalid monitor: "))
		default:
			writeError(w, http.StatusInternalServerError, "failed to move monitor")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "monitor moved", "groupId": body.GroupID})
}

// PauseMonitor stops checking a monitor without deleting it.
// @Summary      Pause monitor
// @Tags         monitors
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Success      200  {object} object{message=string,active=bool}
// @Failure      400  {object} object{error=string} "ID required"
// @Failure      404  {object} object{error=string} "Monitor not found"
// @Router       /monitors/{id}/pause [post]
func (h *CRUDHandler) PauseMonitor(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID required")
		return
	}

	if err := h.store.SetMonitorActive(id, false); err != nil {
		if errors.Is(err, db.ErrMonitorNotFound) {
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to pause monitor")
		return
	}

	h.manager.Sync() // Immediately stop the monitor
	writeJSON(w, http.StatusOK, map[string]any{"message": "monitor paused", "active": false})
}

// ResumeMonitor restarts checking a paused monitor.
// @Summary      Resume monitor
// @Tags         monitors
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Success      200  {object} object{message=string,active=bool}
// @Failure      400  {object} object{error=string} "ID required"
// @Failure      404  {object} object{error=string} "Monitor not found"
// @Router       /monitors/{id}/resume [post]
func (h *CRUDHandler) ResumeMonitor(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID required")
		return
	}

	if err := h.store.SetMonitorActive(id, true); err != nil {
		if errors.Is(err, db.ErrMonitorNotFound) {
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resume monitor")
		return
	}

	h.manager.Sync() // Immediately start the monitor
	writeJSON(w, http.StatusOK, map[string]any{"message": "monitor resumed", "active": true})
}

var validMethods = map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "DELETE": true}
var acceptedCodesRe = regexp.MustCompile(`^[1-5][0-9]{2}(-[1-5][0-9]{2})?(,[1-5][0-9]{2}(-[1-5][0-9]{2})?)*$`)

func validateRequestConfig(cfg *db.RequestConfig) error {
	if cfg == nil {
		return nil
	}
	if cfg.Method != "" && !validMethods[cfg.Method] {
		return fmt.Errorf("method must be one of GET, HEAD, POST, PUT, DELETE")
	}
	if cfg.TimeoutSeconds != 0 && (cfg.TimeoutSeconds < 1 || cfg.TimeoutSeconds > 120) {
		return fmt.Errorf("timeoutSeconds must be between 1 and 120")
	}
	if len(cfg.Headers) > 50 {
		return fmt.Errorf("maximum 50 custom headers allowed")
	}
	for k, v := range cfg.Headers {
		if len(k) > 256 || len(v) > 4096 {
			return fmt.Errorf("header key max 256 chars, value max 4096 chars")
		}
	}
	if len(cfg.Body) > 10240 {
		return fmt.Errorf("request body max 10KB")
	}
	if cfg.AcceptedStatusCodes != "" && !acceptedCodesRe.MatchString(cfg.AcceptedStatusCodes) {
		return fmt.Errorf("acceptedStatusCodes must match format like '200-299,301,302'")
	}
	if cfg.RetryCount < 0 || cfg.RetryCount > 5 {
		return fmt.Errorf("retryCount must be between 0 and 5")
	}
	if cfg.DNSRecordType != "" && !uptime.ValidDNSRecordType(strings.ToUpper(cfg.DNSRecordType)) {
		return fmt.Errorf("dnsRecordType must be one of A, AAAA, MX, NS, TXT")
	}
	if cfg.DNSResolver != "" && !isValidResolver(cfg.DNSResolver) {
		return fmt.Errorf("dnsResolver must be a host or host:port")
	}
	return nil
}

// storedMonitorType returns the check type currently stored for a monitor. An unknown
// monitor falls back to http; the update that follows fails on its own with "monitor
// not found".
func (h *CRUDHandler) storedMonitorType(id string) string {
	monitors, err := h.store.GetMonitors()
	if err != nil {
		return db.MonitorTypeHTTP
	}
	for _, m := range monitors {
		if m.ID == id {
			return db.NormalizeMonitorType(m.Type)
		}
	}
	return db.MonitorTypeHTTP
}

const maxTargetLength = 2048

// validateTarget checks the target against the addressing rules of the check that will
// probe it. Each type reads the same `url` column but expects a different shape there.
func validateTarget(monitorType, target string) error {
	if len(target) > maxTargetLength {
		return fmt.Errorf("target too long (max %d characters)", maxTargetLength)
	}

	switch db.NormalizeMonitorType(monitorType) {
	case db.MonitorTypeHTTP:
		parsed, err := url.ParseRequestURI(target)
		if err != nil {
			return fmt.Errorf("invalid URL format")
		}
		// SECURITY: Only allow http and https protocols to prevent SSRF
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("only HTTP and HTTPS URLs are allowed")
		}
		if parsed.Host == "" {
			return fmt.Errorf("invalid URL format")
		}
	case db.MonitorTypeTCP:
		if !isValidHostPort(target) {
			return fmt.Errorf("tcp target must be host:port (e.g. db.example.com:5432)")
		}
	case db.MonitorTypePing:
		if !isValidHost(target) {
			return fmt.Errorf("ping target must be a hostname or IP address, without scheme or port")
		}
	case db.MonitorTypeDNS:
		if !isValidHost(target) {
			return fmt.Errorf("dns target must be a hostname, without scheme or port")
		}
		// Resolving an IP literal short-circuits before any query leaves the process,
		// so such a monitor would report up forever. Require a name to look up.
		if net.ParseIP(target) != nil {
			return fmt.Errorf("dns target must be a hostname, not an IP address")
		}
	default:
		return fmt.Errorf("type must be one of http, tcp, ping, dns")
	}

	return nil
}

// isValidHostPort accepts host:port where the port is a real port number.
func isValidHostPort(target string) bool {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return false
	}
	return isValidHost(host)
}

// isValidHost accepts an IP literal or a hostname, and nothing that smuggles in a
// scheme, a port, a path or credentials.
func isValidHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if strings.ContainsAny(host, ":/@?# ") {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		if !hostLabelRe.MatchString(label) {
			return false
		}
	}
	return true
}

// Underscores are not legal in a strict reading of DNS, but Docker's embedded resolver
// serves compose service names that contain them, and Warden ships as a container that
// people point at exactly those names.
var hostLabelRe = regexp.MustCompile(`^[a-zA-Z0-9_]([a-zA-Z0-9_-]*[a-zA-Z0-9_])?$`)

// isValidResolver accepts a DNS resolver address with or without an explicit port.
func isValidResolver(addr string) bool {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return isValidHostPort(addr)
	}
	return isValidHost(addr)
}
