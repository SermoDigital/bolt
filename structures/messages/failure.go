package messages

const (
	// FailureMessageSignature is the signature byte for the FAILURE message
	FailureSignature = 0x7F
)

// Failure Represents an FAILURE message
type Failure struct {
	Metadata map[string]interface{}
}

// NewFailure Gets a new Failure struct
func NewFailureMessage(metadata map[string]interface{}) Failure {
	return Failure{Metadata: metadata}
}

// Signature gets the signature byte for the struct
func (i Failure) Signature() uint8 {
	return FailureSignature
}

// Fields gets the fields to encode for the struct
func (i Failure) Fields() []interface{} {
	return []interface{}{i.Metadata}
}
