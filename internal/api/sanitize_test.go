package api

import (
	"strings"
	"testing"
)

func TestSanitizeLog(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string", "hello world", "hello world"},
		{"newline injection", "admin\nFAKE LOG: hacked", "admin\\nFAKE LOG: hacked"},
		{"carriage return", "user\rEVIL", "user\\rEVIL"},
		{"tab", "value\there", "value\\there"},
		{"CRLF", "line\r\ninjection", "line\\r\\ninjection"},
		{"null byte", "zero\x00byte", "zerobyte"},
		{"other control chars", "bell\x07char", "bellchar"},
		{"empty string", "", ""},
		{"truncation", strings.Repeat("a", 300), strings.Repeat("a", 256) + "..."},
		{"exactly 256", strings.Repeat("b", 256), strings.Repeat("b", 256)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLog(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeLog(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
