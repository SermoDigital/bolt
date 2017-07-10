// +build go1.8

package bolt

import (
	"context"
	"database/sql/driver"
)

var (
	_ driver.QueryerContext     = (*conn)(nil)
	_ driver.ExecerContext      = (*conn)(nil)
	_ driver.ConnBeginTx        = (*conn)(nil)
	_ driver.ConnPrepareContext = (*conn)(nil)
)

// QueryContext implements driver.QueryerContext.
func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	params, err := driverArgsToMap(args)
	if err != nil {
		return nil, err
	}
	return c.query(ctx, query, params)
}

// ExecContext implements driver.ExecerContext.
func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	params, err := driverArgsToMap(args)
	if err != nil {
		return nil, err
	}
	return c.exec(ctx, query, params)
}

// BeginTx implements driver.ConnBeginTx.
func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.begin()
}
