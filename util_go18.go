// +build go1.8

package bolt

import (
	"database/sql/driver"
	"errors"

	"github.com/sermodigital/bolt/encoding"
)

func driverArgsToMap(args []driver.NamedValue) (map[string]interface{}, error) {
	if len(args) == 0 {
		return nil, nil
	}

	if len(args) == 1 {
		v := args[0].Value
		// In Go 1.9 we can pass a Map itself, < 1.9 we can't.
		if isGo19 {
			if m, ok := v.(Map); ok {
				return m, nil
			}
		} else if p, ok := v.([]byte); ok && encoding.MaybeMap(p) {
			if v, err := encoding.Unmarshal(p); err == nil {
				if m, ok := v.(map[string]interface{}); ok {
					return m, nil
				}
			}
		}
		return nil, ErrNotMap
	}

	out := make(map[string]interface{}, len(args))
	for _, arg := range args {
		if arg.Name == "" {
			return nil, errors.New("bolt: cannot have an empty name")
		}
		out[arg.Name] = arg.Value
	}
	return out, nil
}
