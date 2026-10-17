package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
)

// These are the interfaces implemented by the pinned pq driver. Embedding them
// preserves cancellation, connection validation and named parameter conversion.
type postgresConn interface {
	driver.Conn
	driver.QueryerContext
	driver.ExecerContext
	driver.ConnPrepareContext
	driver.ConnBeginTx
	driver.Pinger
	driver.SessionResetter
	driver.Validator
	driver.NamedValueChecker
}

type postgresStmt interface {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext
}

type postgresRows interface {
	driver.Rows
	driver.RowsNextResultSet
	driver.RowsColumnTypeScanType
	driver.RowsColumnTypeDatabaseTypeName
	driver.RowsColumnTypeLength
	driver.RowsColumnTypePrecisionScale
}

type traceConnector struct {
	driver.Connector
	run      string
	sequence atomic.Uint64
	emit     func(traceEvent)
}

type traceEvent struct {
	At          string  `json:"at"`
	Run         string  `json:"run"`
	Connection  uint64  `json:"connection"`
	PID         int64   `json:"pid"`
	Span        uint64  `json:"span"`
	Transaction uint64  `json:"transaction,omitempty"`
	Statement   uint64  `json:"statement,omitempty"`
	Operation   string  `json:"operation"`
	Phase       string  `json:"phase"`
	DurationMS  float64 `json:"duration_ms,omitempty"`
	Outcome     string  `json:"outcome,omitempty"`
	SQLState    string  `json:"sqlstate,omitempty"`
}

func openTracedPostgres(dsn string) (*sql.DB, error) {
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(&traceConnector{Connector: connector, run: rand.Text()[:12], emit: func(e traceEvent) {
		data, _ := json.Marshal(e)
		log.Printf("db_trace %s", data)
	}}), nil
}

func (c *traceConnector) Connect(ctx context.Context) (driver.Conn, error) {
	raw, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	conn := raw.(postgresConn)
	// One extra round trip per physical connection, never per statement.
	rows, err := conn.QueryContext(ctx, "SELECT pg_backend_pid()", nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	values := make([]driver.Value, 1)
	err = rows.Next(values)
	closeErr := rows.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &traceConn{postgresConn: conn, owner: c, id: c.sequence.Add(1), pid: values[0].(int64)}, nil
}

type traceConn struct {
	postgresConn
	owner       *traceConnector
	id          uint64
	pid         int64
	transaction uint64
}

func (c *traceConn) start(operation string, statement uint64) (uint64, func(error)) {
	span := c.owner.sequence.Add(1)
	started := time.Now()
	event := traceEvent{At: started.UTC().Format(time.RFC3339Nano), Run: c.owner.run,
		Connection: c.id, PID: c.pid, Span: span, Transaction: c.transaction,
		Statement: statement, Operation: operation, Phase: "start"}
	c.owner.emit(event)
	var once sync.Once
	return span, func(err error) {
		once.Do(func() {
			event.At = time.Now().UTC().Format(time.RFC3339Nano)
			event.Phase = "end"
			event.DurationMS = float64(time.Since(started)) / float64(time.Millisecond)
			event.Outcome = "success"
			if err != nil {
				event.Outcome = "error"
				var pgErr *pq.Error
				if errors.As(err, &pgErr) {
					event.SQLState = string(pgErr.Code)
				}
				if errors.Is(err, context.Canceled) {
					event.Outcome = "canceled"
				}
				if errors.Is(err, context.DeadlineExceeded) {
					event.Outcome = "timeout"
				}
			}
			c.owner.emit(event)
		})
	}
}

func (c *traceConn) tagged(query string, span uint64) string {
	return fmt.Sprintf("/*warden:%s:%d*/ %s", c.owner.run, span, query)
}

func (c *traceConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	span, finish := c.start(traceOperation(query), 0)
	result, err := c.postgresConn.ExecContext(ctx, c.tagged(query, span), args)
	finish(err)
	return result, err
}

func (c *traceConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	span, finish := c.start(traceOperation(query), 0)
	rows, err := c.postgresConn.QueryContext(ctx, c.tagged(query, span), args)
	return tracedRows(rows, err, finish)
}

func (c *traceConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *traceConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	span, finish := c.start("prepare", 0)
	stmt, err := c.postgresConn.PrepareContext(ctx, c.tagged(query, span))
	finish(err)
	if err != nil {
		return nil, err
	}
	return &traceStmt{postgresStmt: stmt.(postgresStmt), conn: c, id: span, operation: traceOperation(query)}, nil
}

func (c *traceConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *traceConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.transaction = c.owner.sequence.Add(1)
	_, finish := c.start("begin", 0)
	tx, err := c.postgresConn.BeginTx(ctx, opts)
	finish(err)
	if err != nil {
		c.transaction = 0
		return nil, err
	}
	return &traceTx{Tx: tx, conn: c}, nil
}

type traceTx struct {
	driver.Tx
	conn *traceConn
}

func (t *traceTx) Commit() error {
	_, finish := t.conn.start("commit", 0)
	err := t.Tx.Commit()
	finish(err)
	t.conn.transaction = 0
	return err
}
func (t *traceTx) Rollback() error {
	_, finish := t.conn.start("rollback", 0)
	err := t.Tx.Rollback()
	finish(err)
	t.conn.transaction = 0
	return err
}

type traceStmt struct {
	postgresStmt
	conn      *traceConn
	id        uint64
	operation string
}

func (s *traceStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	_, finish := s.conn.start(s.operation, s.id)
	result, err := s.postgresStmt.ExecContext(ctx, args)
	finish(err)
	return result, err
}
func (s *traceStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	_, finish := s.conn.start(s.operation, s.id)
	rows, err := s.postgresStmt.QueryContext(ctx, args)
	return tracedRows(rows, err, finish)
}
func (s *traceStmt) Exec(args []driver.Value) (driver.Result, error) {
	_, finish := s.conn.start(s.operation, s.id)
	result, err := s.postgresStmt.Exec(args)
	finish(err)
	return result, err
}
func (s *traceStmt) Query(args []driver.Value) (driver.Rows, error) {
	_, finish := s.conn.start(s.operation, s.id)
	rows, err := s.postgresStmt.Query(args)
	return tracedRows(rows, err, finish)
}

type traceRows struct {
	postgresRows
	finish func(error)
}

func tracedRows(rows driver.Rows, err error, finish func(error)) (driver.Rows, error) {
	if err != nil {
		finish(err)
		return nil, err
	}
	return &traceRows{postgresRows: rows.(postgresRows), finish: finish}, nil
}
func (r *traceRows) Next(values []driver.Value) error {
	err := r.postgresRows.Next(values)
	if err != nil && (err != io.EOF || !r.HasNextResultSet()) {
		if err == io.EOF {
			r.finish(nil)
		} else {
			r.finish(err)
		}
	}
	return err
}
func (r *traceRows) NextResultSet() error {
	err := r.postgresRows.NextResultSet()
	if err != nil {
		if err == io.EOF {
			r.finish(nil)
		} else {
			r.finish(err)
		}
	}
	return err
}
func (r *traceRows) Close() error {
	err := r.postgresRows.Close()
	r.finish(err)
	return err
}

// Only fixed labels leave the process: no SQL, arguments, error messages or URLs.
func traceOperation(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	if strings.HasPrefix(q, "insert into monitor_checks") {
		return "checks_insert"
	}
	if strings.HasPrefix(q, "insert into monitor_uptime_daily") {
		return "daily_upsert"
	}
	if strings.Contains(q, "from monitor_checks") {
		if strings.HasPrefix(q, "select count(*)") {
			return "checks_count"
		}
		if strings.Contains(q, "order by latency asc") {
			return "baseline_percentile"
		}
		return "checks_read"
	}
	for _, verb := range []string{"select", "insert", "update", "delete", "create", "alter", "drop"} {
		if strings.HasPrefix(q, verb+" ") {
			return verb
		}
	}
	return "other"
}
