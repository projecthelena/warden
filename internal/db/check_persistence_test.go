package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
)

func TestRetryableCheckError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		retry bool
	}{
		{"lost commit response", io.EOF, true},
		{"serialization rollback", &pq.Error{Code: "40001"}, true},
		{"constraint failure", &pq.Error{Code: "23514"}, false},
		{"SQLite busy", sqlite3.Error{Code: sqlite3.ErrBusy}, true},
		{"SQLite constraint", sqlite3.Error{Code: sqlite3.ErrConstraint}, false},
		{"shutdown", context.Canceled, false},
		{"deadline", context.DeadlineExceeded, false},
		{"bad connection", driver.ErrBadConn, true},
		{"partial response", io.ErrUnexpectedEOF, true},
		{"deadlock", &pq.Error{Code: "40P01"}, true},
		{"connection lost", &pq.Error{Code: "08006"}, true},
		{"lock unavailable", &pq.Error{Code: "55P03"}, true},
		{"server restarting", &pq.Error{Code: "57P03"}, true},
		{"SQLite locked", sqlite3.Error{Code: sqlite3.ErrLocked}, true},
		{"syntax", &pq.Error{Code: "42601"}, false},
		{"unknown", errors.New("unknown failure"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RetryableCheckError(fmt.Errorf("persist: %w", tc.err)); got != tc.retry {
				t.Fatalf("retry=%v want=%v", got, tc.retry)
			}
		})
	}
}
