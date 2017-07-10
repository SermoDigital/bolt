package bolt

import (
	"context"
	"database/sql/driver"

	"github.com/sermodigital/bolt/structures/messages"
)

var (
	_ driver.Stmt             = (*stmt)(nil)
	_ driver.StmtExecContext  = (*stmt)(nil)
	_ driver.StmtQueryContext = (*stmt)(nil)
)

type stmt struct {
	conn   *conn
	query  string
	closed bool

	// maybeConv is either an empty struct (> Go 1.9) or a type that causes
	// stmt to implement driver.ColumnConverter (< Go 1.8).
	maybeConv
}

// Close closes the statement and helps implement driver.Stmt.
func (s *stmt) Close() error {
	if s.closed {
		return nil
	}
	if s.conn.bad {
		return driver.ErrBadConn
	}
	s.closed = true
	return nil
}

// NumInput returns the number of placeholder parameters. It currently returns
// -1, indicating the number of input placeholders is unknown.
func (s *stmt) NumInput() int {
	return -1 // TODO: need a cypher parser for this.
}

// Exec helps implement driver.Stmt.
func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	params, err := driverArgsToMap2(args)
	if err != nil {
		return nil, err
	}
	return s.exec(context.Background(), params)
}

// ExecContext implements driver.StmtExecContext.
func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	params, err := driverArgsToMap(args)
	if err != nil {
		return nil, err
	}
	return s.exec(ctx, params)
}

type result struct {
	*Counters
}

// LastInsertId returns -1, nil. It helps implement driver.Result.
func (r result) LastInsertId() (int64, error) {
	// TODO: Is this possible?
	return -1, nil
}

// exec is the common implementation of Exec and ExecContext.
func (s *stmt) exec(ctx context.Context, args map[string]interface{}) (driver.Result, error) {
	if s.closed {
		return nil, ErrStatementClosed
	}

	_, err := s.pull(ctx, args)
	if err != nil {
		return nil, err
	}

	// Discard any results.
	_, pull, err := s.conn.consumeAll()
	if err != nil {
		return nil, err
	}

	success, ok := pull.(messages.Success)
	if !ok {
		return nil, UnrecognizedResponseErr{v: pull}
	}

	sum := fromContext(ctx)
	sum.parseSuccess(success.Metadata)
	return result{Counters: &sum.Counters}, nil
}

// Query helps implement driver.Stmt.
func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	params, err := driverArgsToMap2(args)
	if err != nil {
		return nil, err
	}
	return s.runquery(context.Background(), params)
}

// QueryContext implements driver.StmtQueryContext.
func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	params, err := driverArgsToMap(args)
	if err != nil {
		return nil, err
	}
	return s.runquery(ctx, params)
}

// runquery is the common implementation of Query and QueryContext. Its naming
// is different because stmt has a query member.
func (s *stmt) runquery(ctx context.Context, args map[string]interface{}) (driver.Rows, error) {
	if s.closed {
		return nil, ErrStatementClosed
	}
	cols, err := s.pull(ctx, args)
	if err != nil {
		return nil, err
	}
	return &rows{conn: s.conn, cols: cols}, nil
}

// pull executes a query and returns any errors that occur. It does not pull
// any results other than the 'RUN' command.
func (s *stmt) pull(ctx context.Context, args map[string]interface{}) ([]string, error) {
	resp, err := s.conn.sendRunPullAllConsumeRun(s.query, args)
	if err != nil {
		s.closed = true
		return nil, err
	}
	success, ok := resp.(messages.Success)
	if !ok {
		s.closed = true
		return nil, UnrecognizedResponseErr{v: resp}
	}
	md := success.Metadata
	sum := fromContext(ctx)
	sum.parseSuccess(md)
	sum.Query = s.query
	return parseCols(md), nil
}
