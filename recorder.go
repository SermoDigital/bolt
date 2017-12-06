package bolt

import (
	"bytes"
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

var (
	_ driver.Driver = (*Recorder)(nil)
	_ net.Conn      = (*Recorder)(nil)
)

// Recorder allows for recording and playback of a session. The Name field can
// be set and when closed the Recorder will write out the session to a gzipped
// JSON file with the specified name. For example
//
//  const name = "TestRecordedSession"
//  sql.Register(name, &Recorder{Name: name})
//
//  db, err := sql.Open(name)
//  if err != nil { ... }
//
//  // Lots of code
//
//  // The recording will be saved as "TestRecordedSession.json.gzip"
//  if err := db.Close(); err != nil { ... }
//
// Do note: a Recorder where Name == "" has already been registered using the
// name
//
//  bolt-recorder
//
// and will create a random, timestamped file name.
type Recorder struct {
	Name string

	net.Conn
	events []*Event
	cur    int
}

// Open opens a simulated Neo4j connection using pre-recorded data if name is
// an empty string. Otherwise, it opens up an actual connection using that to
// create a new recording.
func (r *Recorder) Open(name string) (driver.Conn, error) {
	if name == "" {
		err := r.load()
		if err != nil {
			return nil, err
		}
		return newConn(r, nil)
	}
	conn, v, err := open(&dialer{}, name)
	if err != nil {
		return nil, err
	}
	r.Conn = conn
	return newConn(r, v)
}

func (r *Recorder) lastEvent() *Event {
	if len(r.events) > 0 {
		return r.events[len(r.events)-1]
	}
	return nil
}

// Read reads from the net.Conn, recording the interaction.
func (r *Recorder) Read(p []byte) (n int, err error) {
	if r.Conn != nil {
		n, err = r.Conn.Read(p)
		r.record(p[:n], false)
		r.recordErr(err, false)
		return n, err
	}

	if r.cur >= len(r.events) {
		return 0, fmt.Errorf("trying to read past all of the events in the recorder %#v", r)
	}
	event := r.events[r.cur]
	if event.IsWrite {
		return 0, fmt.Errorf("recorder expected Read, got Write %#v, Event: %#v", r, event)
	}

	if len(p) > len(event.Event) {
		return 0, fmt.Errorf("attempted to read past current event in recorder Bytes: %s. Recorder %#v, Event; %#v", p, r, event)
	}

	n = copy(p, event.Event)
	event.Event = event.Event[n:]
	if len(event.Event) == 0 {
		r.cur++
	}
	return n, nil
}

// Close the net.Conn, outputting the recording.
func (r *Recorder) Close() error {
	if r.Conn != nil {
		err := r.flush()
		if err != nil {
			return err
		}
		return r.Conn.Close()
	}
	if len(r.events) > 0 {
		if r.cur != len(r.events) {
			return fmt.Errorf("didn't read all of the events in the recorder on close %#v", r)
		}
		if len(r.events[len(r.events)-1].Event) != 0 {
			return fmt.Errorf("left data in an event in the recorder on close %#v", r)
		}
	}
	return nil
}

// Write to the net.Conn, recording the interaction.
func (r *Recorder) Write(b []byte) (n int, err error) {
	if r.Conn != nil {
		n, err = r.Conn.Write(b)
		r.record(b[:n], true)
		r.recordErr(err, true)
		return n, err
	}

	if r.cur >= len(r.events) {
		return 0, fmt.Errorf("trying to write past all of the events in the recorder %#v", r)
	}
	event := r.events[r.cur]
	if !event.IsWrite {
		return 0, fmt.Errorf("recorder expected Write, got Read %#v, Event: %#v", r, event)
	}

	if len(b) > len(event.Event) {
		return 0, fmt.Errorf("attempted to write past current event in recorder Bytes: %s. Recorder %#v, Event; %#v", b, r, event)
	}

	event.Event = event.Event[len(b):]
	if len(event.Event) == 0 {
		r.cur++
	}
	return len(b), nil
}

func (r *Recorder) record(data []byte, isWrite bool) {
	if len(data) == 0 {
		return
	}

	event := r.lastEvent()
	if event == nil || event.Completed || event.IsWrite != isWrite {
		event = newEvent(isWrite)
		r.events = append(r.events, event)
	}

	event.Event = append(event.Event, data...)
	event.Completed = bytes.HasSuffix(data, endMessage)
}

var endMessage = []byte{0, 0}

func (r *Recorder) recordErr(err error, isWrite bool) {
	if err == nil {
		return
	}

	event := r.lastEvent()
	if event == nil || event.Completed || event.IsWrite != isWrite {
		event = newEvent(isWrite)
		r.events = append(r.events, event)
	}
	event.Error = err
	event.Completed = true
}

func (r *Recorder) ensureName() {
	if r.Name != "" {
		return
	}
	var buf [16]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		copy(buf[:], "rand_read_failed") // 16 chars
	}
	dst := make([]byte, hex.EncodedLen(len(buf)))
	hex.Encode(dst, buf[:])
	r.Name = string(time.Now().AppendFormat(dst, "2006-01-02-15-04-05"))
}

func (r *Recorder) load() error {
	r.ensureName()
	path := filepath.Join("recordings", r.Name+".json")
	file, err := os.OpenFile(path, os.O_RDONLY, 0660)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&r.events)
}

func (r *Recorder) writeRecording() error {
	r.ensureName()
	path := filepath.Join("recordings", r.Name+".json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(r.events)
}

func (r *Recorder) flush() error {
	if os.Getenv("RECORD_OUTPUT") != "" {
		return r.writeRecording()
	}
	return nil
}

// Event represents a single recording (read or write) event in the recorder
type Event struct {
	Timestamp int64 `json:"-"`
	Event     []byte
	IsWrite   bool
	Completed bool
	Error     error
}

func newEvent(isWrite bool) *Event {
	return &Event{
		Timestamp: time.Now().UnixNano(),
		IsWrite:   isWrite,
	}
}
