package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"net"

	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
)

// RetryableCheckError includes connection failures whose commit outcome may be unknown.
func RetryableCheckError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var pg *pq.Error
	if errors.As(err, &pg) {
		switch pg.Code.Class() {
		case "08", "40", "53":
			return true
		}
		return pg.Code == "55P03" || pg.Code == "57P01" || pg.Code == "57P02" || pg.Code == "57P03"
	}
	var sqlite sqlite3.Error
	if errors.As(err, &sqlite) {
		return sqlite.Code == sqlite3.ErrBusy || sqlite.Code == sqlite3.ErrLocked
	}
	var network net.Error
	return errors.Is(err, driver.ErrBadConn) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &network)
}
