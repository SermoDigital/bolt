package bolt

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/sermodigital/bolt/encoding"
)

// UnrecognizedResponseErr is an error used when the server sends a reply this
// library cannot recognize. It might indicate a version mismatch or a library
// bug.
type UnrecognizedResponseErr struct {
	v interface{}
}

func (u UnrecognizedResponseErr) Error() string {
	return fmt.Sprintf("unrecognized response from server: %#v", u.v)
}

// Array implements sql.Scanner and should be used to retrieve, e.g., a list
// from sql.Rows.
type Array []interface{}

func (a *Array) Scan(val interface{}) error {
	a0, ok := val.([]interface{})
	if !ok {
		return fmt.Errorf("Array.Scan: unknown type: %T", val)
	}
	*a = a0
	return nil
}

// Map is a utility type. See the package docs for its use in Query, Exec, etc.
// calls. It also implements sql.Scanner and should be used to retrieve, e.g.,
// properties from sql.Rows.
type Map map[string]interface{}

func (m *Map) Scan(val interface{}) error {
	m0, ok := val.(Map)
	if !ok {
		return fmt.Errorf("Map.Scan: unknown type: %T", val)
	}
	*m = m0
	return nil
}

var (
	_ sql.Scanner = (*Array)(nil)
	_ sql.Scanner = (*Map)(nil)
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

var (
	errOddLength = errors.New("bolt: odd number of arguments")
	errNotString = errors.New("bolt: odd-numbered arguments must be strings")
)

func driverArgsToMap2(args []driver.Value) (map[string]interface{}, error) {
	if len(args) == 0 {
		return nil, nil
	}

	if len(args) == 1 {
		m, ok := args[0].(Map)
		if !ok {
			return nil, ErrNotMap
		}
		return m, nil
	}

	if len(args)%2 != 0 {
		return nil, errOddLength
	}

	out := make(map[string]interface{}, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			return nil, errNotString
		}
		out[key] = args[i+1]
	}
	return out, nil
}

func parseCols(md map[string]interface{}) []string {
	val, ok := md["fields"]
	if !ok {
		return nil
	}

	fifc, ok := val.([]interface{})
	if !ok {
		return nil
	}

	cols := make([]string, len(fifc))
	for i, col := range fifc {
		cols[i], ok = col.(string)
		if !ok {
			return nil
		}
	}
	return cols
}

type multiError []error

func (m multiError) Error() string {
	switch n := len(m); n {
	case 0:
		return "<nil>"
	case 1:
		return m[0].Error()
	case 2:
		return fmt.Sprintf("%s and %s", m[0].Error(), m[1].Error())
	default:
		var str string
		for _, err := range m {
			str += fmt.Sprintf("*%s\n", err.Error())
		}
		return str
	}
}

func multi(errs ...error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	e := make(multiError, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			e = append(e, err)
		}
	}
	if len(e) == 1 {
		return e[0]
	}
	return e
}
