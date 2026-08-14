package db

import "testing"

// The type column is added by a paired migration, so it is worth proving on both
// dialects rather than only on the SQLite the rest of these tests use.
func TestMultiDB_MonitorTypeRoundTrip(t *testing.T) {
	RunTestWithBothDBs(t, "MonitorTypeRoundTrip", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
			t.Fatalf("failed to seed group: %v", err)
		}

		monitors := []Monitor{
			{ID: "m-http", GroupID: "g1", Name: "API", URL: "https://example.com", Interval: 60},
			{ID: "m-tcp", Type: MonitorTypeTCP, GroupID: "g1", Name: "DB", URL: "db.example.com:5432", Interval: 60},
			{ID: "m-ping", Type: MonitorTypePing, GroupID: "g1", Name: "Router", URL: "192.168.1.1", Interval: 60},
			{ID: "m-dns", Type: MonitorTypeDNS, GroupID: "g1", Name: "Zone", URL: "example.com", Interval: 60},
		}
		for _, m := range monitors {
			if err := s.CreateMonitor(m); err != nil {
				t.Fatalf("failed to create %s: %v", m.ID, err)
			}
		}

		stored, err := s.GetMonitors()
		if err != nil {
			t.Fatalf("failed to read monitors: %v", err)
		}

		byID := make(map[string]Monitor, len(stored))
		for _, m := range stored {
			byID[m.ID] = m
		}

		// An unset type is stored as http so monitors created before check types
		// existed keep behaving the way they always did.
		if got := byID["m-http"].Type; got != MonitorTypeHTTP {
			t.Errorf("expected an unset type to default to http, got %q", got)
		}
		if got := byID["m-tcp"].Type; got != MonitorTypeTCP {
			t.Errorf("expected tcp, got %q", got)
		}
		if got := byID["m-ping"].Type; got != MonitorTypePing {
			t.Errorf("expected ping, got %q", got)
		}
		if got := byID["m-dns"].Type; got != MonitorTypeDNS {
			t.Errorf("expected dns, got %q", got)
		}

		// A pre-existing row keeps the column's own default, with no help from Go.
		if err := s.UpdateMonitor("m-tcp", "", "DB", "db.example.com:5432", 60, nil, nil, nil, nil); err != nil {
			t.Fatalf("failed to update monitor: %v", err)
		}
		stored, _ = s.GetMonitors()
		for _, m := range stored {
			if m.ID == "m-tcp" && m.Type != MonitorTypeHTTP {
				t.Errorf("expected an explicitly blank type to normalize to http, got %q", m.Type)
			}
		}
	})
}

func TestUpdateMonitorChangesType(t *testing.T) {
	s := newTestStoreWithGroup(t)

	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "API", URL: "https://example.com", Interval: 60}); err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	if err := s.UpdateMonitor("m1", MonitorTypeTCP, "API", "example.com:443", 60, nil, nil, nil, nil); err != nil {
		t.Fatalf("failed to update monitor: %v", err)
	}

	stored, _ := s.GetMonitors()
	if stored[0].Type != MonitorTypeTCP {
		t.Errorf("expected tcp after the update, got %q", stored[0].Type)
	}
}

func TestGetMonitorsNormalizesEmptyStoredType(t *testing.T) {
	s := newTestStoreWithGroup(t)

	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "API", URL: "https://example.com", Interval: 60}); err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	// Simulate a row written before the column carried a value.
	if _, err := s.db.Exec(s.rebind("UPDATE monitors SET type = '' WHERE id = ?"), "m1"); err != nil {
		t.Fatalf("failed to blank the type: %v", err)
	}

	stored, err := s.GetMonitors()
	if err != nil {
		t.Fatalf("failed to read monitors: %v", err)
	}
	if stored[0].Type != MonitorTypeHTTP {
		t.Errorf("expected a blank stored type to read back as http, got %q", stored[0].Type)
	}
}

func TestRequestConfigIsEmptyCoversDNSFields(t *testing.T) {
	if !(&RequestConfig{}).IsEmpty() {
		t.Fatal("expected a zero RequestConfig to be empty")
	}
	if (&RequestConfig{DNSRecordType: "MX"}).IsEmpty() {
		t.Error("expected a config with a record type not to be empty")
	}
	if (&RequestConfig{DNSResolver: "1.1.1.1"}).IsEmpty() {
		t.Error("expected a config with a resolver not to be empty")
	}
}

func TestRequestConfigDNSFieldsSurviveRoundTrip(t *testing.T) {
	s := newTestStoreWithGroup(t)

	cfg := &RequestConfig{DNSRecordType: "MX", DNSResolver: "1.1.1.1:53"}
	m := Monitor{ID: "m1", Type: MonitorTypeDNS, GroupID: "g1", Name: "Zone", URL: "example.com", Interval: 60, RequestConfig: cfg}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	stored, _ := s.GetMonitors()
	if stored[0].RequestConfig == nil {
		t.Fatal("expected the DNS config to be stored")
	}
	if stored[0].RequestConfig.DNSRecordType != "MX" {
		t.Errorf("expected record type MX, got %q", stored[0].RequestConfig.DNSRecordType)
	}
	if stored[0].RequestConfig.DNSResolver != "1.1.1.1:53" {
		t.Errorf("expected resolver 1.1.1.1:53, got %q", stored[0].RequestConfig.DNSResolver)
	}
}

// newTestStoreWithGroup returns a clean store with a group to hang monitors off,
// since the shared helper wipes the seeded default group.
func newTestStoreWithGroup(t *testing.T) *Store {
	s := newTestStore(t)
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("failed to seed group: %v", err)
	}
	return s
}
