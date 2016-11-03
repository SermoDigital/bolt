package messages

const (
	// IgnoredMessageSignature is the signature byte for the IGNORED message
	IgnoredSignature = 0x7E
)

// Ignored Represents an IGNORED message
type Ignored struct{}

// Signature gets the signature byte for the struct
func (i Ignored) Signature() uint8 {
	return IgnoredSignature
}

// Fields gets the fields to encode for the struct
func (i Ignored) Fields() []interface{} {
	return nil
}
