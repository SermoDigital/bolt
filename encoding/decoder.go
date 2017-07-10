package encoding

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"

	"github.com/sermodigital/bolt/structures/graph"
	"github.com/sermodigital/bolt/structures/messages"
)

type reader interface {
	io.Reader
	io.ByteReader
}

type byteReader struct {
	io.Reader
	b [1]byte
}

func (b *byteReader) ReadByte() (byte, error) {
	_, err := io.ReadFull(b.Reader, b.b[:1])
	return b.b[0], err
}

type chunkReader struct {
	r      reader
	length uint16 // remaining bytes in the chunk to read
}

func (b *chunkReader) next() error {
	if err := binary.Read(b.r, binary.BigEndian, &b.length); err != nil {
		return err
	}
	if b.length == 0 {
		return io.EOF
	}
	return nil
}

// Read implements io.Reader.
func (r *chunkReader) Read(p []byte) (n int, err error) {
	if r.length <= 0 {
		if err = r.next(); err != nil {
			return 0, err
		}
	}
	if len(p) > int(r.length) {
		p = p[0:r.length]
	}
	n, err = r.r.Read(p)
	r.length -= uint16(n)
	return n, err
}

// ReadByte implements io.ByteReader.
func (r *chunkReader) ReadByte() (c byte, err error) {
	if r.length <= 0 {
		if err = r.next(); err != nil {
			return 0, err
		}
	}
	c, err = r.r.ReadByte()
	if err != nil {
		return 0, err
	}
	r.length--
	return c, err
}

// Decoder decodes a message from the bolt protocol stream. It attempts to
// support all builtin Go types, when it can be confidently mapped to a data
// type from:
// http://alpha.neohq.net/docs/server-manual/bolt-serialization.html#bolt-packstream-structures
// (version v3.1.0-M02 at the time of writing this.
//
// Maps and Slices are a special case, where only map[string]interface{} and
// []interface{} are supported. The interface for maps and slices may be more
// permissive in the future.
type Decoder struct {
	r       *chunkReader
	scratch [512]byte
	lastErr error
}

// NewDecoder creates a new Decoder object
func NewDecoder(r io.Reader) *Decoder {
	if rr, ok := r.(reader); ok {
		return &Decoder{r: &chunkReader{r: rr}}
	}
	return &Decoder{r: &chunkReader{r: &byteReader{Reader: r}}}
}

// Unmarshal is used to marshal an object to the bolt interface encoded bytes
func Unmarshal(b []byte) (interface{}, error) {
	return NewDecoder(bytes.NewReader(b)).Decode()
}

// Discard drains the rest of the data from the Decoder.
func (d *Decoder) Discard() error {
	// We use this instead of io.Copy(ioutil.Discard, conn) since b.Reads
	// messages in chunks and stops when the next chunk does not exist.
	// That causes timeouts since the TCP connection is never sent EOF.
	_, err := io.Copy(ioutil.Discard, d.r)
	return err
}

// Decode returns the next object from the stream.
func (d *Decoder) Decode() (interface{}, error) {
	if d.lastErr != nil {
		return nil, d.lastErr
	}

	v, err := d.decode()
	if err != nil {
		d.lastErr = err
		return nil, err
	}

	var eof uint16
	if err = d.read(&eof); err != nil {
		// io.EOF means 0 bytes read, so we've got more to read.
		if err == io.EOF {
			return v, nil
		}
		d.lastErr = err
		return nil, err
	}

	if eof != 0 {
		d.lastErr = errors.New("invalid eof")
		return nil, d.lastErr
	}
	return v, nil
}

// More reports whether there the stream contains more usable data.
func (d *Decoder) More() bool {
	return d.lastErr == nil
}

func (d *Decoder) read(v interface{}) error {
	return binary.Read(d.r, binary.BigEndian, v)
}

func (d *Decoder) int8() (int64, error) {
	_, err := io.ReadFull(d.r, d.scratch[:1])
	return int64(int8(d.scratch[0])), err
}

func (d *Decoder) int16() (int64, error) {
	_, err := io.ReadFull(d.r, d.scratch[:2])
	return int64(int16(binary.BigEndian.Uint16(d.scratch[:2]))), err
}

func (d *Decoder) int32() (int64, error) {
	_, err := io.ReadFull(d.r, d.scratch[:4])
	return int64(int32(binary.BigEndian.Uint32(d.scratch[:4]))), err
}

func (d *Decoder) int64() (int64, error) {
	_, err := io.ReadFull(d.r, d.scratch[:8])
	return int64(binary.BigEndian.Uint64(d.scratch[:8])), err
}

func (d *Decoder) float() (float64, error) {
	_, err := io.ReadFull(d.r, d.scratch[:8])
	return math.Float64frombits(binary.BigEndian.Uint64(d.scratch[:8])), err
}

func adjust(c byte) byte {
	if c < packedUpper && c > packedLower {
		c -= c % 16
	}
	return c
}

func (d *Decoder) decode() (interface{}, error) {
	marker, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch adjust(marker) {
	// Basic nil, true, and false.
	case Nil:
		return nil, nil
	case True:
		return true, nil
	case False:
		return false, nil

	// Numbers
	default:
		if m := int8(marker); m >= -16 && m <= 127 {
			return int64(m), nil
		}
		return nil, fmt.Errorf("unrecognized marker byte: %x", marker)
	case Int8:
		return d.int8()
	case Int16:
		return d.int16()
	case Int32:
		return d.int32()
	case Int64:
		return d.int64()
	case Float:
		return d.float()

	// Strings
	case TinyString:
		return d.decodeString(int(marker) - TinyString)
	case String8:
		length, err := d.int8()
		if err != nil {
			return nil, err
		}
		return d.decodeString(int(length))
	case String16:
		length, err := d.int16()
		if err != nil {
			return nil, err
		}
		return d.decodeString(int(length))
	case String32:
		length, err := d.int32()
		if err != nil {
			return nil, err
		}
		return d.decodeString(int(length))

	// Slices
	case TinySlice:
		return d.decodeSlice(int(marker) - TinySlice)
	case Slice8:
		length, err := d.int8()
		if err != nil {
			return nil, err
		}
		return d.decodeSlice(int(length))
	case Slice16:
		length, err := d.int16()
		if err != nil {
			return nil, err
		}
		return d.decodeSlice(int(length))
	case Slice32:
		length, err := d.int32()
		if err != nil {
			return nil, err
		}
		return d.decodeSlice(int(length))

	// Maps
	case TinyMap:
		return d.decodeMap(int(marker) - TinyMap)
	case Map8:
		slots, err := d.int8()
		if err != nil {
			return nil, err
		}
		return d.decodeMap(int(slots))
	case Map16:
		slots, err := d.int16()
		if err != nil {
			return nil, err
		}
		return d.decodeMap(int(slots))
	case Map32:
		slots, err := d.int32()
		if err != nil {
			return nil, err
		}
		return d.decodeMap(int(slots))

	// Structures
	case TinyStruct:
		return d.decodeStruct()
	case Struct8:
		if _, err := d.int8(); err != nil {
			return nil, err
		}
		return d.decodeStruct()
	case Struct16:
		if _, err := d.int16(); err != nil {
			return nil, err
		}
		return d.decodeStruct()
	}
}

func (d *Decoder) decodeString(size int) (string, error) {
	if size == 0 {
		return "", nil
	}
	var buf []byte
	if size <= cap(d.scratch) {
		buf = d.scratch[0:size]
	} else {
		buf = make([]byte, size)
	}
	_, err := io.ReadFull(d.r, buf)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (d *Decoder) decodeSlice(size int) ([]interface{}, error) {
	slice := make([]interface{}, size)
	for i := 0; i < size; i++ {
		item, err := d.decode()
		if err != nil {
			return nil, err
		}
		slice[i] = item
	}
	return slice, nil
}

func (d *Decoder) decodeMap(size int) (map[string]interface{}, error) {
	m := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		kv, err := d.decode()
		if err != nil {
			return nil, err
		}
		key, ok := kv.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected key type: %T", kv)
		}

		val, err := d.decode()
		if err != nil {
			return nil, err
		}
		m[key] = val
	}
	return m, nil
}

func (d *Decoder) decodeStruct() (interface{}, error) {
	signature, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch signature {
	case graph.NodeSignature:
		return d.decodeNode()
	case graph.RelationshipSignature:
		return d.decodeRelationship()
	case graph.PathSignature:
		return d.decodePath()
	case graph.UnboundRelationshipSignature:
		return d.decodeUnboundRelationship()
	case messages.RecordSignature:
		return d.decodeRecordMessage()
	case messages.FailureSignature:
		return d.decodeFailureMessage()
	case messages.IgnoredSignature:
		return messages.Ignored{}, nil
	case messages.SuccessSignature:
		return d.decodeSuccessMessage()
	case messages.AckFailureSignature:
		return messages.AckFailure{}, nil
	case messages.DiscardAllMessageSignature:
		return messages.DiscardAll{}, nil
	case messages.PullAllSignature:
		return messages.PullAll{}, nil
	case messages.ResetSignature:
		return messages.Reset{}, nil
	default:
		return nil, fmt.Errorf("unrecognized type decoding struct with signature %x", signature)
	}
}

func (d *Decoder) decodeNode() (graph.Node, error) {
	var node graph.Node
	nodeIdentityInt, err := d.decode()
	if err != nil {
		return node, err
	}
	var ok bool
	node.NodeIdentity, ok = nodeIdentityInt.(int64)
	if !ok {
		return node, fmt.Errorf("unexpected type")
	}

	labelInt, err := d.decode()
	if err != nil {
		return node, err
	}
	labelIntSlice, ok := labelInt.([]interface{})
	if !ok {
		return node, fmt.Errorf("expected: Labels []string, but got %T", labelInt)
	}
	node.Labels, err = sliceInterfaceToString(labelIntSlice)
	if err != nil {
		return node, err
	}

	propertiesInt, err := d.decode()
	if err != nil {
		return node, err
	}
	node.Properties, ok = propertiesInt.(map[string]interface{})
	if !ok {
		return node, fmt.Errorf("expected: Properties map[string]interface{}, but got %T", propertiesInt)
	}
	return node, nil
}

func (d *Decoder) decodeRelationship() (graph.Relationship, error) {
	var rel graph.Relationship

	relIdentityInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	rel.RelIdentity = relIdentityInt.(int64)

	startNodeIdentityInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	rel.StartNodeIdentity = startNodeIdentityInt.(int64)

	endNodeIdentityInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	rel.EndNodeIdentity = endNodeIdentityInt.(int64)

	var ok bool
	typeInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	rel.Type, ok = typeInt.(string)
	if !ok {
		return rel, fmt.Errorf("expected: Type string, but got %T", typeInt)
	}

	propertiesInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	rel.Properties, ok = propertiesInt.(map[string]interface{})
	if !ok {
		return rel, fmt.Errorf("expected: Properties map[string]interface{}, but got %T", propertiesInt)
	}
	return rel, nil
}

func (d *Decoder) decodePath() (graph.Path, error) {
	var path graph.Path

	nodesInt, err := d.decode()
	if err != nil {
		return path, err
	}
	nodesIntSlice, ok := nodesInt.([]interface{})
	if !ok {
		return path, fmt.Errorf("expected: Nodes []Node, but got %T", nodesInt)
	}
	path.Nodes, err = sliceInterfaceToNode(nodesIntSlice)
	if err != nil {
		return path, err
	}

	relsInt, err := d.decode()
	if err != nil {
		return path, err
	}
	relsIntSlice, ok := relsInt.([]interface{})
	if !ok {
		return path, fmt.Errorf("expected: Relationships []Relationship, but got %T", relsInt)
	}
	path.Relationships, err = sliceInterfaceToUnboundRelationship(relsIntSlice)
	if err != nil {
		return path, err
	}

	seqInt, err := d.decode()
	if err != nil {
		return path, err
	}
	seqIntSlice, ok := seqInt.([]interface{})
	if !ok {
		return path, fmt.Errorf("expected: Sequence []int, but got %T", seqInt)
	}
	path.Sequence, err = sliceInterfaceToInt(seqIntSlice)

	return path, err
}

func (d *Decoder) decodeUnboundRelationship() (graph.UnboundRelationship, error) {
	var rel graph.UnboundRelationship

	relIdentityInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	var ok bool
	rel.RelIdentity, ok = relIdentityInt.(int64)
	if !ok {
		return rel, fmt.Errorf("expected int64, got %T", relIdentityInt)
	}

	typeInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	rel.Type, ok = typeInt.(string)
	if !ok {
		return rel, fmt.Errorf("expected: Type string, but got %T", typeInt)
	}

	propertiesInt, err := d.decode()
	if err != nil {
		return rel, err
	}
	rel.Properties, ok = propertiesInt.(map[string]interface{})
	if !ok {
		return rel, fmt.Errorf("expected: Properties map[string]interface{}, but got %T", propertiesInt)
	}
	return rel, nil
}

func (d *Decoder) decodeRecordMessage() (messages.Record, error) {
	fieldsInt, err := d.decode()
	if err != nil {
		return messages.Record{}, err
	}
	vals, ok := fieldsInt.([]interface{})
	if !ok {
		return messages.Record{}, fmt.Errorf("expected: Fields []interface{}, but got %T", fieldsInt)
	}
	return messages.Record{Values: vals}, nil
}

func (d *Decoder) decodeFailureMessage() (messages.Failure, error) {
	metadataInt, err := d.decode()
	if err != nil {
		return messages.Failure{}, err
	}
	metadata, ok := metadataInt.(map[string]interface{})
	if !ok {
		return messages.Failure{}, fmt.Errorf("expected: Metadata map[string]interface{}, but got %T", metadataInt)
	}
	return messages.Failure{Metadata: metadata}, nil
}

func (d *Decoder) decodeSuccessMessage() (messages.Success, error) {
	metadataInt, err := d.decode()
	if err != nil {
		return messages.Success{}, err
	}
	metadata, ok := metadataInt.(map[string]interface{})
	if !ok {
		return messages.Success{}, fmt.Errorf("expected: Metadata map[string]interface{}, but got %T", metadataInt)
	}
	return messages.Success{Metadata: metadata}, nil
}
