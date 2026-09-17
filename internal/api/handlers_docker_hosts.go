package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/dockerapi"
	"github.com/projecthelena/warden/internal/uptime"
)

type DockerHostHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewDockerHostHandler(store *db.Store, manager *uptime.Manager) *DockerHostHandler {
	return &DockerHostHandler{store: store, manager: manager}
}

type dockerHostResponse struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Endpoint             string    `json:"endpoint"`
	TLSVerify            bool      `json:"tlsVerify"`
	HasCACertificate     bool      `json:"hasCaCertificate"`
	HasClientCertificate bool      `json:"hasClientCertificate"`
	CreatedAt            time.Time `json:"createdAt"`
}

type dockerHostRequest struct {
	Name       string  `json:"name"`
	Endpoint   string  `json:"endpoint"`
	TLSVerify  bool    `json:"tlsVerify"`
	CACert     *string `json:"caCert"`
	ClientCert *string `json:"clientCert"`
	ClientKey  *string `json:"clientKey"`
}

func dockerHostJSON(host db.DockerHost) dockerHostResponse {
	return dockerHostResponse{ID: host.ID, Name: host.Name, Endpoint: host.Endpoint, TLSVerify: host.TLSVerify, HasCACertificate: host.CACert != "", HasClientCertificate: host.ClientCert != "", CreatedAt: host.CreatedAt}
}

func (h *DockerHostHandler) List(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	hosts, err := h.store.GetDockerHosts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load Docker hosts")
		return
	}
	response := make([]dockerHostResponse, 0, len(hosts))
	for _, host := range hosts {
		response = append(response, dockerHostJSON(host))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *DockerHostHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	request, ok := decodeDockerHostRequest(w, r)
	if !ok {
		return
	}
	host := db.DockerHost{ID: generateID(request.Name, "dh-"), Name: strings.TrimSpace(request.Name), Endpoint: strings.TrimSpace(request.Endpoint), TLSVerify: request.TLSVerify}
	host.TLSVerify = strings.HasPrefix(host.Endpoint, "https://")
	applyDockerHostSecrets(&host, request)
	if err := validateDockerHost(host); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.CreateDockerHost(host); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			writeError(w, http.StatusConflict, "A Docker host with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to save Docker host")
		return
	}
	writeJSON(w, http.StatusCreated, dockerHostJSON(host))
}

func (h *DockerHostHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	host, err := h.store.GetDockerHost(chi.URLParam(r, "id"))
	if err != nil {
		writeDockerHostError(w, err)
		return
	}
	request, ok := decodeDockerHostRequest(w, r)
	if !ok {
		return
	}
	host.Name = strings.TrimSpace(request.Name)
	host.Endpoint = strings.TrimSpace(request.Endpoint)
	host.TLSVerify = strings.HasPrefix(host.Endpoint, "https://")
	applyDockerHostSecrets(&host, request)
	if !host.TLSVerify {
		host.CACert = ""
		host.ClientCert = ""
		host.ClientKey = ""
	}
	if err := validateDockerHost(host); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.UpdateDockerHost(host); err != nil {
		writeDockerHostError(w, err)
		return
	}
	if h.manager != nil {
		h.manager.Sync()
	}
	writeJSON(w, http.StatusOK, dockerHostJSON(host))
}

func (h *DockerHostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	if err := h.store.DeleteDockerHost(chi.URLParam(r, "id")); err != nil {
		if strings.Contains(err.Error(), "is used by monitor") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeDockerHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DockerHostHandler) Test(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	host, err := h.store.GetDockerHost(chi.URLParam(r, "id"))
	if err != nil {
		writeDockerHostError(w, err)
		return
	}
	client, err := dockerapi.NewClient(host, 5*time.Second)
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		err = client.Ping(ctx)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *DockerHostHandler) Containers(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	host, err := h.store.GetDockerHost(chi.URLParam(r, "id"))
	if err != nil {
		writeDockerHostError(w, err)
		return
	}
	client, err := dockerapi.NewClient(host, 5*time.Second)
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		var containers any
		containers, err = client.Containers(ctx)
		if err == nil {
			writeJSON(w, http.StatusOK, containers)
			return
		}
	}
	writeError(w, http.StatusBadGateway, err.Error())
}

func decodeDockerHostRequest(w http.ResponseWriter, r *http.Request) (dockerHostRequest, bool) {
	var request dockerHostRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return dockerHostRequest{}, false
	}
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Endpoint) == "" {
		writeError(w, http.StatusBadRequest, "name and endpoint are required")
		return dockerHostRequest{}, false
	}
	if len(request.Name) > maxNameLength {
		writeError(w, http.StatusBadRequest, "name is too long")
		return dockerHostRequest{}, false
	}
	return request, true
}

func applyDockerHostSecrets(host *db.DockerHost, request dockerHostRequest) {
	if request.CACert != nil {
		host.CACert = *request.CACert
	}
	if request.ClientCert != nil {
		host.ClientCert = *request.ClientCert
	}
	if request.ClientKey != nil {
		host.ClientKey = *request.ClientKey
	}
}

func validateDockerHost(host db.DockerHost) error {
	_, err := dockerapi.NewClient(host, 5*time.Second)
	return err
}

func writeDockerHostError(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrDockerHostNotFound) {
		writeError(w, http.StatusNotFound, "Docker host not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "Docker host operation failed")
}
