package api

import (
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
	"testing"
)

func TestNewHTTPMonitorDefaultsToAutomaticRetry(t *testing.T) {
	s, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	m := uptime.NewManager(s)
	defer m.Reset()
	h := NewCRUDHandler(s, m)
	for _, tc := range []struct {
		name      string
		cfg       *db.RequestConfig
		automatic bool
	}{{"default", nil, true}, {"explicit none", &db.RequestConfig{}, false}, {"manual", &db.RequestConfig{RetryCount: 2}, false}} {
		monitor, err := h.AddMonitor(MonitorInput{Name: tc.name, URL: "http://127.0.0.1:1", GroupID: "g-default", Interval: 60, RequestConfig: tc.cfg})
		if err != nil {
			t.Fatal(err)
		}
		if (monitor.RequestConfig != nil && monitor.RequestConfig.AutoRetry) != tc.automatic {
			t.Fatalf("wrong default for %s", tc.name)
		}
	}
	if validateRequestConfig(&db.RequestConfig{AutoRetry: true, RetryCount: 2}) == nil {
		t.Fatal("accepted conflicting retry policies")
	}
}
