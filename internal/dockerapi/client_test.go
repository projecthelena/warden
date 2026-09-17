package dockerapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestInspectResolvesContainerNameBeforeEveryCheck(t *testing.T) {
	listCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/containers/json":
			listCalls++
			_, _ = fmt.Fprintf(w, `[{"Id":"new-id-%d","Names":["/warden"],"Image":"warden:latest","State":"running","Status":"Up 5 seconds (healthy)"}]`, listCalls)
		case fmt.Sprintf("/containers/new-id-%d/json", listCalls):
			_, _ = w.Write([]byte(`{"RestartCount":2,"State":{"Status":"running","Running":true,"Paused":false,"Restarting":false,"OOMKilled":false,"ExitCode":0,"Error":"","Health":{"Status":"healthy","Log":[]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(db.DockerHost{Endpoint: server.URL}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		state, err := client.Inspect(context.Background(), "warden")
		if err != nil {
			t.Fatal(err)
		}
		if !state.Running || state.Health != "healthy" || state.RestartCount != 2 {
			t.Fatalf("unexpected state: %+v", state)
		}
	}
	if listCalls != 2 {
		t.Fatalf("expected selector to be resolved twice, got %d", listCalls)
	}
}

func TestContainersReturnsStoppedContainersAndHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("all") != "1" {
			t.Error("container discovery must include stopped containers")
		}
		_, _ = w.Write([]byte(`[{"Id":"abc","Names":["/db"],"Image":"postgres:18","State":"exited","Status":"Exited (137) 2 minutes ago"},{"Id":"def","Names":["/api"],"Image":"api:latest","State":"running","Status":"Up 1 minute (unhealthy)"}]`))
	}))
	defer server.Close()

	client, err := NewClient(db.DockerHost{Endpoint: server.URL}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	containers, err := client.Containers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 2 || containers[0].Name != "api" || containers[0].Health != "unhealthy" || containers[1].Name != "db" {
		t.Fatalf("unexpected containers: %+v", containers)
	}
}

func TestNewClientRejectsIncompleteTLSCredentials(t *testing.T) {
	_, err := NewClient(db.DockerHost{Endpoint: "https://docker.example.com", ClientCert: "certificate only"}, time.Second)
	if err == nil {
		t.Fatal("expected incomplete mTLS credentials to be rejected")
	}
}
