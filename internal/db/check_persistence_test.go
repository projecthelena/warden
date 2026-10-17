package db

import (
	"context"
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RetryableCheckError(fmt.Errorf("persist: %w", tc.err)); got != tc.retry {
				t.Fatalf("retry=%v want=%v", got, tc.retry)
			}
		})
	}
}
