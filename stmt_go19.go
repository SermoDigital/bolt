// +build go1.9

package bolt

import (
	"database/sql/driver"
	"errors"
)

const isGo19 = true

// ErrNotMap is returned when one argument is passed to a Query, Exec, etc.
// method and its type isn't Map.
var ErrNotMap = errors.New("bolt: if one argument is passed it must of type Map")

// maybeConv is a dummy type embedded in stmt, used in Go versions > 1.8.
type maybeConv struct{}

func (s *stmt) CheckNamedValue(v *driver.NamedValue) error {
	_, ok := v.Value.(Map)
	if ok || v.Name != "" {
		return nil
	}
	return errors.New("argument name cannot be empty")
}

var _ driver.NamedValueChecker = (*stmt)(nil)
