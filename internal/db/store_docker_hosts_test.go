package db

import (
	"strings"
	"testing"
)

func TestDockerHostCRUDAndMonitorProtection(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateGroup(Group{ID: "g-default", Name: "Default"}); err != nil {
		t.Fatal(err)
	}
	host := DockerHost{ID: "dh-local", Name: "Local", Endpoint: "unix:///var/run/docker.sock", ClientKey: "secret"}
	if err := s.CreateDockerHost(host); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetDockerHost(host.ID)
	if err != nil || got.ClientKey != "secret" {
		t.Fatalf("GetDockerHost() = %+v, %v", got, err)
	}

	monitor := Monitor{ID: "m-docker", Type: MonitorTypeDocker, GroupID: "g-default", Name: "API", URL: "api", Active: true, Interval: 60, RequestConfig: &RequestConfig{DockerHostID: host.ID}}
	if err := s.CreateMonitor(monitor); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDockerHost(host.ID); err == nil || !strings.Contains(err.Error(), "used by monitor") {
		t.Fatalf("expected referenced host deletion to fail, got %v", err)
	}
	if err := s.DeleteMonitor(monitor.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDockerHost(host.ID); err != nil {
		t.Fatal(err)
	}
}
