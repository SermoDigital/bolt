package messages

const (
	// SuccessMessageSignature is the signature byte for the SUCCESS message
	SuccessSignature = 0x70
)

// Success Represents an SUCCESS message
type Success struct {
	Metadata map[string]interface{}
}

// Signature gets the signature byte for the struct
func (i Success) Signature() uint8 {
	return SuccessSignature
}

// Fields gets the fields to encode for the struct
func (i Success) Fields() []interface{} {
	return []interface{}{i.Metadata}
}
