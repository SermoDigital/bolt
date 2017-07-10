package bolt

import (
	"bufio"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/sermodigital/bolt/encoding"
	"github.com/sermodigital/bolt/structures/messages"
)

// ErrInFailedTransaction is returned when an operation is attempted inside
// a failed transaction.
var ErrInFailedTransaction = errors.New("bolt: operation inside failed transaction")

// ErrStatementClosed is returned when an operation is attempted on a closed
// statement.
var ErrStatementClosed = errors.New("bolt: statement is closed")

// status indicates the state of a conn, particularly whether it's in a
// (potentially failed) transcation or not.
type status uint8

const (
	statusIdle    status = iota // idle
	statusInTx                  // in a transaction
	statusInBadTx               // in a bad transaction
)

type conn struct {
	conn net.Conn
	buf  *bufio.Reader

	// dec and enc should not be used outright. The decode and encode methods
	// should be used instead.
	dec *encoding.Decoder
	enc *encoding.Encoder

	timeout time.Duration
	size    uint16
	status  status
	bad     bool
}

var (
	_ driver.Queryer = (*conn)(nil)
	_ driver.Execer  = (*conn)(nil)
	_ driver.Conn    = (*conn)(nil)
)

// Query implements driver.Queryer.
func (c *conn) Query(query string, args []driver.Value) (driver.Rows, error) {
	params, err := driverArgsToMap2(args)
	if err != nil {
		return nil, err
	}
	return c.query(context.Background(), query, params)
}

// query is the common implementation of Query and QueryContext.
func (c *conn) query(ctx context.Context, query string, args map[string]interface{}) (driver.Rows, error) {
	if c.bad {
		return nil, driver.ErrBadConn
	}
	s := &stmt{conn: c, query: query}
	defer s.Close()
	return s.runquery(ctx, args)
}

// Exec implements driver.Execer.
func (c *conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	params, err := driverArgsToMap2(args)
	if err != nil {
		return nil, err
	}
	return c.exec(context.Background(), query, params)
}

// exec is the common implementation of Exec and ExecContext.
func (c *conn) exec(ctx context.Context, query string, args map[string]interface{}) (driver.Result, error) {
	if c.bad {
		return nil, driver.ErrBadConn
	}
	s := &stmt{conn: c, query: query}
	defer s.Close()
	return s.exec(ctx, args)
}

// Prepare helps implement driver.Conn.
func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

// PrepareContext implements driver.ConnPrepareContext.
func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if c.bad {
		return nil, driver.ErrBadConn
	}
	return &stmt{conn: c, query: query}, nil
}

// Close closes the connection. It helps implement driver.Conn.
func (c *conn) Close() error {
	if c.bad {
		return driver.ErrBadConn
	}
	c.status = statusIdle
	err := c.conn.Close()
	c.bad = err == nil
	return err
}

// Begin begins a new transaction. It helps implement driver.Conn.
func (c *conn) Begin() (driver.Tx, error) {
	return c.begin()
}

// begin is the implementaiton of Begin and BeginTx.
func (c *conn) begin() (driver.Tx, error) {
	if c.bad {
		return nil, driver.ErrBadConn
	}
	if err := c.checktx(false); err != nil {
		return nil, err
	}
	if err := c.transac(begin); err != nil {
		return nil, err
	}
	c.status = statusInTx
	return c, nil
}

// Commit commits and closes the transaction. It helps implement driver.Tx.
func (c *conn) Commit() error {
	if c.bad {
		return driver.ErrBadConn
	}
	if err := c.checktx(true); err != nil {
		return err
	}
	if c.status == statusInBadTx {
		if err := c.Rollback(); err != nil {
			return err
		}
		return ErrInFailedTransaction
	}
	return c.transac(commit)
}

// Rollback rolls back and closes the transaction. It helps implement driver.Tx.
func (c *conn) Rollback() error {
	if c.bad {
		return driver.ErrBadConn
	}

	if err := c.checktx(true); err != nil {
		return err
	}
	if err := c.transac(rollback); err != nil {
		return err
	}
	c.status = statusIdle
	return nil
}

// decode returns the next message from the connection if it exists. It returns
// io.EOF when the stream has finished.
func (c *conn) decode() (interface{}, error) {
	if c.dec == nil {
		c.dec = encoding.NewDecoder(c)
	}
	if !c.dec.More() {
		return nil, io.EOF
	}
	return c.dec.Decode()
}

// encode writes the bolt-encoded form of v to the connection.
func (c *conn) encode(v interface{}) error {
	if c.enc == nil {
		c.enc = encoding.NewEncoder(c)
		c.enc.SetChunkSize(c.size)
	}
	return c.enc.Encode(v)
}

// newConn creates a new Neo4j connection using the provided values.
func newConn(netcn net.Conn, v values) (*conn, error) {
	timeout, err := parseTimeout(v.get("timeout"))
	if err != nil {
		return nil, err
	}

	c := &conn{
		conn:    netcn,
		buf:     bufio.NewReader(netcn),
		timeout: timeout,
		size:    encoding.DefaultChunkSize,
	}
	if err := c.handshake(); err != nil {
		return nil, multi(err, c.Close())
	}

	resp, err := c.sendInit(v.get("username"), v.get("password"))
	if err != nil {
		return nil, multi(err, c.Close())
	}

	_, ok := resp.(messages.Success)
	if !ok {
		return nil, multi(
			UnrecognizedResponseErr{v: resp},
			c.Close(),
		)
	}
	return c, nil
}

// handshake completes the bolt protocol's version handshake.
func (c *conn) handshake() error {
	_, err := c.Write(handshake[:])
	if err != nil {
		return err
	}
	var vers version
	_, err = io.ReadFull(c, vers[:])
	if err != nil {
		return err
	}
	if vers == noSupportedVersions {
		return errors.New("server does not support any versions")
	}
	return nil
}

// Read implements io.Reader with conn's timeout.
func (c *conn) Read(p []byte) (n int, err error) {
	if c.timeout != 0 {
		err = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
		if err != nil {
			return 0, err
		}
	}
	return c.buf.Read(p)
}

// Write implements io.Writer with conn's timeout.
func (c *conn) Write(p []byte) (n int, err error) {
	if c.timeout != 0 {
		err = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
		if err != nil {
			return 0, err
		}
	}
	return c.conn.Write(p)
}

// ackFailure responds to a failure message allowing the connection to proceed.
// https://github.com/neo4j-contrib/boltkit/blob/b2739a15871aae8469363b0298f8765a4ec77a9a/boltkit/driver.py#L662
func (c *conn) ackFailure() error {
	if err := c.encode(messages.AckFailure{}); err != nil {
		return err
	}

	for {
		resp, err := c.decode()
		if err != nil {
			return err
		}

		switch resp := resp.(type) {
		case messages.Ignored:
			// OK
		case messages.Success:
			return nil
		case messages.Failure:
			return c.reset()
		default:
			return multi(
				UnrecognizedResponseErr{v: resp},
				c.Close(),
			)
		}
	}
}

// reset clears the connection.
// https://github.com/neo4j-contrib/boltkit/blob/c81e36a602a44c6fc64e4a088cfed3b2a2449329/boltkit/driver.py#L672
func (c *conn) reset() error {
	if err := c.encode(messages.Reset{}); err != nil {
		return err
	}

	for {
		resp, err := c.decode()
		if err != nil {
			return err
		}

		switch resp := resp.(type) {
		case messages.Ignored:
			// OK
		case messages.Success:
			return nil
		case messages.Failure:
			return multi(
				fmt.Errorf("error resetting session: %#v ", resp),
				err,
			)
		default:
			return multi(
				UnrecognizedResponseErr{v: resp},
				c.Close(),
			)
		}
	}
}

type txQuery string

const (
	begin    txQuery = "BEGIN"
	commit   txQuery = "COMMIT"
	rollback txQuery = "ROLLBACK"
)

func (t txQuery) String() string {
	const tx = "transaction"
	switch t {
	case begin:
		return "beginning " + tx
	case commit:
		return "committing " + tx
	case rollback:
		return "rolling back " + tx
	default:
		return string(t)
	}
}

// transac executes the given transaction query.
func (c *conn) transac(query txQuery) error {
	switch query {
	case begin, commit, rollback:
		// OK
	default:
		return fmt.Errorf("bug: invalid transaction query: %s", query)
	}

	run, pull, err := c.sendRunPullAllConsumeSingle(string(query), nil)
	if err != nil {
		return err
	}

	_, ok := run.(messages.Success)
	if !ok {
		return UnrecognizedResponseErr{v: run}
	}

	_, ok = pull.(messages.Success)
	if !ok {
		c.status = statusInBadTx
		return UnrecognizedResponseErr{v: pull}
	}
	return nil
}

func (c *conn) checktx(intx bool) error {
	if (c.status == statusInTx || c.status == statusInBadTx) != intx {
		c.bad = true
		return fmt.Errorf("unexpected transaction status: %v", c.status)
	}
	return nil
}

// SetChunkSize sets the size of the chunks that are written to the connection.
func (c *conn) SetChunkSize(chunkSize uint16) {
	c.size = chunkSize
}

// SetTimeout sets the timeout for reading and writing to the connection.
func (c *conn) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// consume returns the next value from the connection, acknowledging any
// failures that occurred.
func (c *conn) consume() (interface{}, error) {
	resp, err := c.decode()
	if err != nil {
		return resp, err
	}
	if fail, ok := resp.(messages.Failure); ok {
		if err := c.ackFailure(); err != nil {
			return nil, err
		}
		return fail, nil
	}
	return resp, err
}

func (c *conn) consumeAll() ([]interface{}, interface{}, error) {
	var responses []interface{}
	for {
		resp, err := c.consume()
		if err != nil {
			return nil, resp, err
		}
		smg, ok := resp.(messages.Success)
		if ok {
			return responses, smg, nil
		}
		responses = append(responses, resp)
	}
}

func (c *conn) consumeAllMultiple(mult int) ([][]interface{}, []interface{}, error) {
	responses := make([][]interface{}, mult)
	successes := make([]interface{}, mult)
	for i := 0; i < mult; i++ {
		resp, success, err := c.consumeAll()
		if err != nil {
			return responses, successes, err
		}
		responses[i] = resp
		successes[i] = success
	}
	return responses, successes, nil
}

func (c *conn) sendInit(user, pass string) (interface{}, error) {
	initMessage := messages.NewInitMessage(ClientID, user, pass)
	if err := c.encode(initMessage); err != nil {
		return nil, err
	}
	return c.consume()
}

func (c *conn) run(query string, args map[string]interface{}) error {
	runMessage := messages.NewRunMessage(query, args)
	return c.encode(runMessage)
}

func (c *conn) sendRunConsume(query string, args map[string]interface{}) (interface{}, error) {
	if err := c.run(query, args); err != nil {
		return nil, err
	}
	return c.consume()
}

func (c *conn) pullAll() error {
	return c.encode(messages.NewPullAllMessage())
}

func (c *conn) pullAllConsume() (interface{}, error) {
	if err := c.pullAll(); err != nil {
		return nil, err
	}
	return c.consume()
}

func (c *conn) sendRunPullAll(query string, args map[string]interface{}) error {
	if err := c.run(query, args); err != nil {
		return err
	}
	return c.pullAll()
}

func (c *conn) sendRunPullAllConsumeRun(query string, args map[string]interface{}) (interface{}, error) {
	if err := c.sendRunPullAll(query, args); err != nil {
		return nil, err
	}
	return c.consume()
}

func (c *conn) sendRunPullAllConsumeSingle(query string, args map[string]interface{}) (interface{}, interface{}, error) {
	err := c.sendRunPullAll(query, args)
	if err != nil {
		return nil, nil, err
	}

	runSuccess, err := c.consume()
	if err != nil {
		return runSuccess, nil, err
	}

	pullSuccess, err := c.consume()
	return runSuccess, pullSuccess, err
}

func (c *conn) sendRunPullAllConsumeAll(query string, args map[string]interface{}) (interface{}, interface{}, []interface{}, error) {
	err := c.sendRunPullAll(query, args)
	if err != nil {
		return nil, nil, nil, err
	}

	runSuccess, err := c.consume()
	if err != nil {
		return runSuccess, nil, nil, err
	}

	records, pullSuccess, err := c.consumeAll()
	return runSuccess, pullSuccess, records, err
}

func (c *conn) sendDiscardAll() error {
	return c.encode(messages.DiscardAll{})
}

func (c *conn) sendDiscardAllConsume() (interface{}, error) {
	if err := c.sendDiscardAll(); err != nil {
		return nil, err
	}
	return c.consume()
}

func (c *conn) sendRunDiscardAll(query string, args map[string]interface{}) error {
	err := c.run(query, args)
	if err != nil {
		return err
	}
	return c.sendDiscardAll()
}

func (c *conn) sendRunDiscardAllConsume(query string, args map[string]interface{}) (interface{}, interface{}, error) {
	runResp, err := c.sendRunConsume(query, args)
	if err != nil {
		return runResp, nil, err
	}
	discardResp, err := c.sendDiscardAllConsume()
	return runResp, discardResp, err
}
