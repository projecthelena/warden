package api

import (
	"bytes"
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

func TestDockerHostListNeverReturnsCertificatesOrKeys(t *testing.T) {
	store := newTestStore(t)
	handler := NewDockerHostHandler(store, nil)
	if err := store.CreateDockerHost(db.DockerHost{ID: "dh-local", Name: "Local", Endpoint: "https://docker.example.com", TLSVerify: true, CACert: "SECRET_CA", ClientCert: "SECRET_CERT", ClientKey: "SECRET_KEY"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/docker/hosts", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleEditor))
	recorder := httptest.NewRecorder()
	handler.List(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, secret := range []string{"SECRET_CA", "SECRET_CERT", "SECRET_KEY"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaked %s: %s", secret, body)
		}
	}
	if !strings.Contains(body, `"hasCaCertificate":true`) || !strings.Contains(body, `"hasClientCertificate":true`) {
		t.Fatalf("response should expose only credential presence: %s", body)
	}
}

func TestDockerHostUpdatePreservesOmittedSecrets(t *testing.T) {
	store := newTestStore(t)
	handler := NewDockerHostHandler(store, nil)
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	if err := store.CreateDockerHost(db.DockerHost{ID: "dh-local", Name: "Local", Endpoint: server.URL, TLSVerify: true, CACert: ca}); err != nil {
		t.Fatal(err)
	}

	router := chi.NewRouter()
	router.Use(roleMiddleware(RoleEditor))
	router.Put("/docker/hosts/{id}", handler.Update)
	req := httptest.NewRequest(http.MethodPut, "/docker/hosts/dh-local", bytes.NewBufferString(`{"name":"Renamed","endpoint":"`+server.URL+`","tlsVerify":true}`))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	host, err := store.GetDockerHost("dh-local")
	if err != nil {
		t.Fatal(err)
	}
	if host.CACert != ca {
		t.Fatalf("update erased credentials: %+v", host)
	}
}

func TestDockerHostUpdateClearsTLSSecretsForNonHTTPSHost(t *testing.T) {
	store := newTestStore(t)
	host := db.DockerHost{ID: "dh-remote", Name: "Remote", Endpoint: "https://docker.example.com:2376", TLSVerify: true, CACert: "ca", ClientCert: "cert", ClientKey: "key"}
	if err := store.CreateDockerHost(host); err != nil {
		t.Fatal(err)
	}

	handler := NewDockerHostHandler(store, nil)
	router := chi.NewRouter()
	router.Use(roleMiddleware(RoleEditor))
	router.Put("/docker/hosts/{id}", handler.Update)
	request := httptest.NewRequest(http.MethodPut, "/docker/hosts/dh-remote", strings.NewReader(`{"name":"Local","endpoint":"unix:///var/run/docker.sock"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	updated, err := store.GetDockerHost(host.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TLSVerify || updated.CACert != "" || updated.ClientCert != "" || updated.ClientKey != "" {
		t.Fatalf("expected TLS configuration to be cleared, got %#v", updated)
	}
}

func TestDockerHostMutationRequiresEditor(t *testing.T) {
	store := newTestStore(t)
	handler := NewDockerHostHandler(store, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/docker/hosts", bytes.NewBufferString(`{"name":"Local","endpoint":"unix:///var/run/docker.sock"}`))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleViewer))
	recorder := httptest.NewRecorder()
	handler.Create(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}
