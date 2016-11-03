package messages

const (
	// AckFailureMessageSignature is the signature byte for the ACK_FAILURE message
	AckFailureSignature = 0x0E
)

// AckFailure Represents an ACK_FAILURE message
type AckFailure struct{}

// Signature gets the signature byte for the struct
func (i AckFailure) Signature() uint8 {
	return AckFailureSignature
}

// Fields gets the fields to encode for the struct
func (i AckFailure) Fields() []interface{} {
	return nil
}
