// +build !go1.9

package bolt

import (
	"bytes"
	"database/sql/driver"
	"errors"

	"github.com/sermodigital/bolt/encoding"
)

const isGo19 = false

// ErrNotMap is returned when one argument is passed to a Query, Exec, etc.
// method and its type isn't []byte and a bolt-encoded map.
var ErrNotMap = errors.New("bolt: if one argument is passed it must of type []byte and a bolt-encoded map")

var (
	_ driver.ValueConverter  = (*maybeConv)(nil)
	_ driver.ColumnConverter = (*stmt)(nil)
)

// maybeConv implements driver.ValueConverter to allow the sql.DB interface
// to work with bolt. It's legacy and only used in Go versions < 1.9.
type maybeConv struct {
	b     *bytes.Buffer
	e     *encoding.Encoder
	ismap bool
	idx   int
}

// ColumnConverter implements driver.ColumnConverter.
func (m *maybeConv) ColumnConverter(idx int) driver.ValueConverter {
	m.idx = idx
	return m
}

// encode returns an encoded v and any errors that may have occurred.
func (m *maybeConv) encode(v interface{}) ([]byte, error) {
	if m.b == nil {
		m.b = new(bytes.Buffer)
	}
	if m.e == nil {
		m.e = encoding.NewEncoder(m.b)
	}
	if err := m.e.Encode(v); err != nil {
		return nil, err
	}
	p := make([]byte, m.b.Len())
	copy(p, m.b.Bytes())
	m.b.Reset()
	return p, nil
}

// ConvertValue implements driver.ValueConverter.
func (m *maybeConv) ConvertValue(v interface{}) (driver.Value, error) {
	if m.idx == 0 {
		if p, ok := v.(Map); ok {
			m.ismap = true
			return m.encode((map[string]interface{})(p))
		}
	}

	// If our first value was a map then we've finished and any new values are
	// an error.
	if m.ismap {
		return nil, errors.New("if value at index 0 is Map no other values are allowed")
	}

	// Even entries should be strings (keys).
	if m.idx%2 == 0 {
		key, ok := v.(string)
		if !ok {
			return nil, errors.New("even-indexed values must be string keys")
		}
		return key, nil
	}

	// Odd entries can be anything. The sql package handles the driver.Valuer
	// case for us. If v is a valid driver.Value return it. Otherwise, use
	// bolt's encoding and return it as a []byte.
	//
	// TODO: is there something more efficient?
	if driver.IsValue(v) {
		return v, nil
	}
	return m.encode(v)
}
