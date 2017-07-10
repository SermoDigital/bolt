package bolt

import (
	"database/sql/driver"
	"errors"
	"io"

	"github.com/sermodigital/bolt/structures/graph"
	"github.com/sermodigital/bolt/structures/messages"
)

var _ driver.Rows = (*rows)(nil)

type rows struct {
	conn     *conn
	cols     []string
	closed   bool // true if Close successfully called.
	finished bool // true if all rows have been read.
	md       map[string]interface{}
}

// Columns returns the 'fields' returned from the server. It helps implement
// driver.Rows.
func (r *rows) Columns() []string {
	return r.cols
}

// Close closes the rows. It helps implement driver.Rows.
func (r *rows) Close() error {
	if r.closed {
		return nil
	}
	// We haven't read all the rows.
	if !r.finished {
		if err := r.conn.dec.Discard(); err != nil {
			return err
		}
		r.finished = true
	}
	r.closed = true
	return nil
}

// ErrRowsClosed is returned when the Rows have already been closed.
var ErrRowsClosed = errors.New("bolt: rows have been closed")

// Next returns the next row. It helps implement driver.Rows.
func (r *rows) Next(dest []driver.Value) error {
	if r.closed {
		return ErrRowsClosed
	}

	resp, err := r.conn.consume()
	if err != nil {
		return err
	}

	switch t := resp.(type) {
	case messages.Success:
		r.md = t.Metadata
		r.finished = true
		return io.EOF
	case messages.Record:
		for i, item := range t.Values {
			switch item := item.(type) {
			case driver.Value,
				Array,
				Map,
				graph.Node,
				graph.Path,
				graph.Relationship,
				graph.UnboundRelationship:
				dest[i] = item
			default:
				dest[i], err = driver.DefaultParameterConverter.ConvertValue(item)
				if err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return UnrecognizedResponseErr{v: resp}
	}
}
