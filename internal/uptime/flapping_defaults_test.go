package uptime

import "testing"

func TestFlappingNotificationsAreOptIn(t *testing.T) {
	m, store := newAlertTestManager(t)
	if m.eventFilter.FlappingEnabled {
		t.Fatal("new manager enables flapping notices")
	}
	for _, value := range []string{"", "false", "true"} {
		if err := store.SetSetting("notification.event.flapping.enabled", value); err != nil {
			t.Fatal(err)
		}
		filter := m.loadEventFilter()
		if filter.FlappingEnabled != (value == "true") {
			t.Fatalf("flapping setting %q: enabled=%v", value, filter.FlappingEnabled)
		}
		if !filter.DownEnabled || !filter.UpEnabled {
			t.Fatal("flapping preference silenced outage or recovery alerts")
		}
	}
}
