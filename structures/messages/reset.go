package messages

const (
	// ResetMessageSignature is the signature byte for the RESET message
	ResetSignature = 0x0F
)

// Reset Represents an RESET message
type Reset struct{}

// Signature gets the signature byte for the struct
func (i Reset) Signature() uint8 {
	return ResetSignature
}

// Fields gets the fields to encode for the struct
func (i Reset) Fields() []interface{} {
	return nil
}
