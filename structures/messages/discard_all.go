package messages

const (
	// DiscardAllMessageSignature is the signature byte for the DISCARD_ALL message
	DiscardAllMessageSignature = 0x2F
)

// DiscardAll Represents an DISCARD_ALL message
type DiscardAll struct{}

// NewDiscardAll Gets a new DiscardAll struct
func NewDiscardAllMessage() DiscardAll {
	return DiscardAll{}
}

// Signature gets the signature byte for the struct
func (i DiscardAll) Signature() uint8 {
	return DiscardAllMessageSignature
}

// Fields gets the fields to encode for the struct
func (i DiscardAll) Fields() []interface{} {
	return nil
}
