package encoding

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/sermodigital/bolt/structures"
)

const (
	packedLower = 0x80 // lower bound of a packed length

	// TinyString marks the beginning of a string
	TinyString = 0x80
	// TinySlice marks the beginning of a slice object
	TinySlice = 0x90
	// TinyMap marks the beginning of a map object
	TinyMap = 0xA0
	// TinyStruct marks the beginning of a struct object
	TinyStruct = 0xB0

	packedUpper = 0xC0 // upper bound of a packed length

	// Nil marks the beginning of a nil
	Nil = 0xC0

	// Float marks the beginning of a float64
	Float = 0xC1

	// False marks the beginning of a false boolean
	False = 0xC2
	// True marks the beginning of a true boolean
	True = 0xC3

	// Int8 marks the beginning of an int8
	Int8 = 0xC8
	// Int16 marks the beginning of an int16
	Int16 = 0xC9
	// Int32 marks the beginning of an int32
	Int32 = 0xCA
	// Int64 marks the beginning of an int64
	Int64 = 0xCB

	// String8 marks the beginning of a string
	String8 = 0xD0
	// String16 marks the beginning of a string
	String16 = 0xD1
	// String32 marks the beginning of a string
	String32 = 0xD2

	// Slice8 marks the beginning of a slice object
	Slice8 = 0xD4
	// Slice16 marks the beginning of a slice object
	Slice16 = 0xD5
	// Slice32 marks the beginning of a slice object
	Slice32 = 0xD6

	// Map8 marks the beginning of a map object
	Map8 = 0xD8
	// Map16 marks the beginning of a map object
	Map16 = 0xD9
	// Map32 marks the beginning of a map object
	Map32 = 0xDA

	// Struct8 marks the beginning of a struct object
	Struct8 = 0xDC
	// Struct16 marks the beginning of a struct object
	Struct16 = 0xDD
)

// endMessage is the data to send to end a message
var endMessage = [2]byte{0, 0}

// MaybeMap returns true if m might be a chunked, bolt-encoded map.
func MaybeMap(m []byte) bool {
	const (
		length = 2               // chunk length
		marker = 1               // type marker
		endlen = len(endMessage) // chunk ending bytes
	)

	// Must have room for a chunk length, marker, and ending bytes.
	if len(m) < length+marker+endlen {
		return false
	}

	// Must have a map marker.
	switch adjust(m[2 /* 0-indexed marker */]) {
	case TinyMap, Map8, Map16, Map32:
		// OK
	default:
		return false
	}

	// Read through chunks to ensure they're all accurate.
	for {
		clen := int(binary.BigEndian.Uint16(m))
		if clen > len(m) {
			return false
		}
		m = m[length+clen:]
		if len(m) <= length {
			if len(m) == length {
				return bytes.HasSuffix(m, endMessage[:])
			}
			return false // malformed
		}
	}
}

// Encoder encodes objects of different types to the given stream.
// Attempts to support all builtin golang types, when it can be confidently
// mapped to a data type from:
// http://alpha.neohq.net/docs/server-manual/bolt-serialization.html#bolt-packstream-structures
// (version v3.1.0-M02 at the time of writing this.
//
// Maps and Slices are a special case, where only map[string]interface{} and
// []interface{} are supported. The interface for maps and slices may be more
// permissive in the future.
type Encoder struct {
	w *chunkWriter
}

const DefaultChunkSize = math.MaxUint16

// NewEncoder initializes a new Encoder with the provided chunk size.
func NewEncoder(w io.Writer) *Encoder {
	const size = DefaultChunkSize
	return &Encoder{w: &chunkWriter{w: w, buf: make([]byte, size), size: size}}
}

// SetChunkSize sets the Encoder's chunk size. It flushes any pending writes
// using the new chunk size if the new chunl size is smaller than the current
// pending write(s).
func (e *Encoder) SetChunkSize(size uint16) error {
	if e.w.size == size {
		return nil
	}
	e.w.size = size

	// Create a new buffer if necessary.
	if int(e.w.size) > len(e.w.buf) {
		e.w.buf = make([]byte, e.w.size)
		return nil
	}

	// Flush what we have so far if our current chunk is >= size.
	for e.w.n >= e.w.size {
		e.w.n = e.w.size
		err := e.w.writeChunk()
		if err != nil {
			return err
		}
		// Slide our buffer down.
		e.w.n = uint16(copy(e.w.buf[:e.w.size], e.w.buf[e.w.n:]))
	}
	return nil
}

// Marshal is used to marshal an object to the bolt interface encoded bytes.
func Marshal(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	err := NewEncoder(&b).Encode(v)
	return b.Bytes(), err
}

type chunkWriter struct {
	w    io.Writer
	buf  []byte
	n    uint16
	size uint16
}

// Write writes to the Encoder. Writes are not necessarily written to the
// underlying Writer until Flush is called.
func (w *chunkWriter) Write(p []byte) (n int, err error) {
	for n < len(p) {
		m := copy(w.buf[w.n:], p[n:])
		w.n += uint16(m)
		n += m
		if w.n == w.size {
			if err = w.writeChunk(); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

// Write writes a string to the Encoder. Writes are not necessarily written to
// the underlying Writer until Flush is called.
func (w *chunkWriter) WriteString(s string) (n int, err error) {
	for n < len(s) {
		m := copy(w.buf[w.n:], s[n:])
		w.n += uint16(m)
		n += m
		if w.n == w.size {
			if err = w.writeChunk(); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

// Flush writes the existing data to the underlying writer and then ends
// the stream.
func (w *chunkWriter) Flush() error {
	if err := w.writeChunk(); err != nil {
		return err
	}
	_, err := w.w.Write(endMessage[:])
	return err
}

func (w *chunkWriter) write(marker uint8) error {
	w.buf[w.n] = marker
	w.n++
	if w.n == w.size {
		return w.writeChunk()
	}
	return nil
}

func (w *chunkWriter) writeChunk() error {
	if w.n == 0 {
		return nil
	}
	if err := binary.Write(w.w, binary.BigEndian, uint16(w.n)); err != nil {
		return err
	}
	_, err := w.w.Write(w.buf[:w.n])
	w.n = 0
	return err
}

func (e *Encoder) write(v interface{}) error {
	return binary.Write(e.w, binary.BigEndian, v)
}

// Encode encodes an object to the stream
func (e *Encoder) Encode(val interface{}) error {
	if err := e.encode(val); err != nil {
		return err
	}
	return e.w.Flush()
}

// Encode encodes an object to the stream
func (e *Encoder) encode(val interface{}) error {
	const maxInt = math.MaxInt64
	switch val := val.(type) {
	case nil:
		return e.w.write(Nil)
	case bool:
		if val {
			return e.w.write(True)
		}
		return e.w.write(False)
	case int:
		if val > maxInt {
			return fmt.Errorf("integer too big: %d. Max integer supported: %d", val, math.MaxInt64)
		}
		return e.encodeInt(int64(val))
	case int8:
		return e.encodeInt(int64(val))
	case int16:
		return e.encodeInt(int64(val))
	case int32:
		return e.encodeInt(int64(val))
	case int64:
		return e.encodeInt(val)
	case uint:
		if val > maxInt {
			return fmt.Errorf("integer too big: %d. Max integer supported: %d", val, math.MaxInt64)
		}
		return e.encodeInt(int64(val))
	case uint8:
		return e.encodeInt(int64(val))
	case uint16:
		return e.encodeInt(int64(val))
	case uint32:
		return e.encodeInt(int64(val))
	case uint64:
		if val > maxInt {
			return fmt.Errorf("integer too big: %d. Max integer supported: %d", val, math.MaxInt64)
		}
		return e.encodeInt(int64(val))
	case float32:
		return e.encodeFloat(float64(val))
	case float64:
		return e.encodeFloat(val)
	case string:
		return e.encodeString(val)
	case []interface{}:
		return e.encodeSlice(val)
	case map[string]interface{}:
		return e.encodeMap(val)
	case structures.Structure:
		return e.encodeStructure(val)
	default:
		return fmt.Errorf("unrecognized type when encoding data for Bolt transport: %T", val)
	}
}

func (e *Encoder) encodeInt(val int64) (err error) {
	switch {
	case val < math.MinInt32:
		// Write as INT_64
		if err = e.w.write(Int64); err != nil {
			return err
		}
		return e.write(val)
	case val < math.MinInt16:
		// Write as INT_32
		if err = e.w.write(Int32); err != nil {
			return err
		}
		return e.write(int32(val))
	case val < math.MinInt8:
		// Write as INT_16
		if err = e.w.write(Int16); err != nil {
			return err
		}
		return e.write(int16(val))
	case val < -16:
		// Write as INT_8
		if err = e.w.write(Int8); err != nil {
			return err
		}
		return e.write(int8(val))
	case val < math.MaxInt8:
		// Write as TINY_INT
		return e.write(int8(val))
	case val < math.MaxInt16:
		// Write as INT_16
		if err = e.w.write(Int16); err != nil {
			return err
		}
		return e.write(int16(val))
	case val < math.MaxInt32:
		// Write as INT_32
		if err = e.w.write(Int32); err != nil {
			return err
		}
		return e.write(int32(val))
	case val <= math.MaxInt64:
		// Write as INT_64
		if err = e.w.write(Int64); err != nil {
			return err
		}
		return e.write(val)
	default:
		return fmt.Errorf("Int too long to write: %d", val)
	}
}

func (e *Encoder) encodeFloat(val float64) error {
	if err := e.w.write(Float); err != nil {
		return err
	}
	return e.write(val)
}

func (e *Encoder) encodeString(str string) (err error) {
	switch length := len(str); {
	case length <= 15:
		err = e.w.write(TinyString + uint8(length))
		if err != nil {
			return err
		}
		_, err = e.w.WriteString(str)
		return err
	case length <= math.MaxUint8:
		if err = e.w.write(String8); err != nil {
			return err
		}
		if err = e.write(uint8(length)); err != nil {
			return err
		}
		_, err = e.w.WriteString(str)
		return err
	case length <= math.MaxUint16:
		if err = e.w.write(String16); err != nil {
			return err
		}
		if err = e.write(uint16(length)); err != nil {
			return err
		}
		_, err = e.w.WriteString(str)
		return err
	case length > math.MaxUint16 && length <= math.MaxUint32:
		if err = e.w.write(String32); err != nil {
			return err
		}
		if err = e.write(uint32(length)); err != nil {
			return err
		}
		_, err = e.w.WriteString(str)
		return err
	default:
		return errors.New("string too long to write")
	}
}

func (e *Encoder) encodeSlice(val []interface{}) (err error) {
	switch length := len(val); {
	case length <= 15:
		err = e.w.write(TinySlice + uint8(length))
		if err != nil {
			return err
		}
	case length <= math.MaxUint8:
		if err = e.w.write(Slice8); err != nil {
			return err
		}
		if err = e.write(uint8(length)); err != nil {
			return err
		}
	case length <= math.MaxUint16:
		if err = e.w.write(Slice16); err != nil {
			return err
		}
		if err = e.write(uint16(length)); err != nil {
			return err
		}
	case length <= math.MaxUint32:
		if err := e.w.write(Slice32); err != nil {
			return err
		}
		if err = e.write(uint32(length)); err != nil {
			return err
		}
	default:
		return errors.New("slice too long to write")
	}

	// Encode Slice values
	for _, item := range val {
		if err = e.encode(item); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encodeMap(val map[string]interface{}) (err error) {
	switch length := len(val); {
	case length <= 15:
		err = e.w.write(TinyMap + uint8(length))
		if err != nil {
			return err
		}
	case length <= math.MaxUint8:
		if err = e.w.write(Map8); err != nil {
			return err
		}
		if err = e.write(uint8(length)); err != nil {
			return err
		}
	case length <= math.MaxUint16:
		if err = e.w.write(Map16); err != nil {
			return err
		}
		if err = e.write(uint16(length)); err != nil {
			return err
		}
	case length <= math.MaxUint32:
		if err = e.w.write(Map32); err != nil {
			return err
		}
		if err = e.write(uint32(length)); err != nil {
			return err
		}
	default:
		return errors.New("map too long to write")
	}

	// Encode Map values
	for k, v := range val {
		if err := e.encode(k); err != nil {
			return err
		}
		if err := e.encode(v); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encodeStructure(val structures.Structure) (err error) {
	fields := val.Fields()
	switch length := len(fields); {
	case length <= 15:
		err = e.w.write(TinyStruct + uint8(length))
		if err != nil {
			return err
		}
	case length <= math.MaxUint8:
		if err = e.w.write(Struct8); err != nil {
			return err
		}
		if err = e.write(uint8(length)); err != nil {
			return err
		}
	case length <= math.MaxUint16:
		if err = e.w.write(Struct16); err != nil {
			return err
		}
		if err = e.write(uint16(length)); err != nil {
			return err
		}
	default:
		return errors.New("structure too large to write")
	}

	if err = e.w.write(val.Signature()); err != nil {
		return err
	}

	for _, field := range fields {
		if err = e.encode(field); err != nil {
			return err
		}
	}
	return nil
}
