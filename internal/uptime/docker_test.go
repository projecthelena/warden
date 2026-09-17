package uptime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projecthelena/warden/internal/db"
)

func TestProbeDockerStates(t *testing.T) {
	tests := []struct {
		name      string
		stateJSON string
		wantUp    bool
		wantError string
	}{
		{name: "healthy", stateJSON: `{"RestartCount":0,"State":{"Status":"running","Running":true,"Health":{"Status":"healthy","Log":[]}}}`, wantUp: true},
		{name: "running without healthcheck", stateJSON: `{"RestartCount":0,"State":{"Status":"running","Running":true}}`, wantUp: true},
		{name: "starting", stateJSON: `{"RestartCount":0,"State":{"Status":"running","Running":true,"Health":{"Status":"starting","Log":[]}}}`, wantError: "still starting"},
		{name: "unhealthy", stateJSON: `{"RestartCount":0,"State":{"Status":"running","Running":true,"Health":{"Status":"unhealthy","Log":[{"Output":"connection refused"}]}}}`, wantError: "connection refused"},
		{name: "oom killed", stateJSON: `{"RestartCount":0,"State":{"Status":"exited","Running":false,"OOMKilled":true,"ExitCode":137}}`, wantError: "out of memory"},
		{name: "restarting", stateJSON: `{"RestartCount":6,"State":{"Status":"restarting","Running":true,"Restarting":true}}`, wantError: "restart count: 6"},
		{name: "exited", stateJSON: `{"RestartCount":0,"State":{"Status":"exited","Running":false,"ExitCode":2,"Error":"bad config"}}`, wantError: "exit code 2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/containers/json" {
					_, _ = w.Write([]byte(`[{"Id":"container-id","Names":["/api"],"Image":"api:latest","State":"running","Status":"Up"}]`))
					return
				}
				_, _ = w.Write([]byte(tc.stateJSON))
			}))
			defer server.Close()

			out := probeDocker(Job{Type: db.MonitorTypeDocker, URL: "api", DockerHost: &db.DockerHost{Endpoint: server.URL}}, defaultCheckTimeout)
			if out.up != tc.wantUp {
				t.Fatalf("up = %v, want %v; error: %s", out.up, tc.wantUp, out.err)
			}
			if tc.wantError != "" && !strings.Contains(out.err, tc.wantError) {
				t.Fatalf("error %q does not contain %q", out.err, tc.wantError)
			}
		})
	}
}
