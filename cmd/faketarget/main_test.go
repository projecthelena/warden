package main

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestRequestLog(t *testing.T) {
	var output bytes.Buffer
	writer, flags := log.Writer(), log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(writer)
		log.SetFlags(flags)
	})

	for _, tt := range []struct {
		name   string
		quiet  bool
		format string
		args   []any
		want   string
	}{
		{"normal", false, "→ %s %s [%d]", []any{"GET", "/status/503", 503}, "→ GET /status/503 [503]\n"},
		{"request injection", false, "→ %s %s", []any{"GET", "/healthy\r\nforged entry"}, "→ GET /healthyforged entry\n"},
		{"format injection", false, "first\r\n%s", []any{"second\nthird"}, "firstsecondthird\n"},
		{"quiet", true, "%s", []any{"/healthy\nforged entry"}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output.Reset()
			requestLog(tt.quiet, tt.format, tt.args...)
			if got := output.String(); got != tt.want {
				t.Fatalf("requestLog() = %q, want %q", got, tt.want)
			}
			if strings.Count(output.String(), "\n") > 1 {
				t.Fatal("request created multiple log lines")
			}
		})
	}
}
