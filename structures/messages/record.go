package messages

const (
	// RecordSignature is the signature byte for the RECORD message
	RecordSignature = 0x71
)

// Record Represents an RECORD message
type Record struct {
	Values []interface{}
}

// NewRecord Gets a new Record struct
func NewRecord(fields []interface{}) Record {
	return Record{Values: fields}
}

// Signature gets the signature byte for the struct
func (i Record) Signature() uint8 {
	return RecordSignature
}

// Fields gets the fields to encode for the struct
func (i Record) Fields() []interface{} {
	return i.Values
}
