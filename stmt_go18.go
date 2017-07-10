// +build go1.8

package bolt

import (
	"context"
	"database/sql/driver"
)

var (
	_ driver.StmtExecContext  = (*stmt)(nil)
	_ driver.StmtQueryContext = (*stmt)(nil)
)

// ExecContext implements driver.StmtExecContext.
func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	params, err := driverArgsToMap(args)
	if err != nil {
		return nil, err
	}
	return s.exec(ctx, params)
}

// QueryContext implements driver.StmtQueryContext.
func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	params, err := driverArgsToMap(args)
	if err != nil {
		return nil, err
	}
	return s.runquery(ctx, params)
}
