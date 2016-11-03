package messages

const (
	// PullAllMessageSignature is the signature byte for the PULL_ALL message
	PullAllSignature = 0x3F
)

// PullAll Represents an PULL_ALL message
type PullAll struct{}

// NewPullAll Gets a new PullAll struct
func NewPullAllMessage() PullAll {
	return PullAll{}
}

// Signature gets the signature byte for the struct
func (i PullAll) Signature() uint8 {
	return PullAllSignature
}

// Fields gets the fields to encode for the struct
func (i PullAll) Fields() []interface{} {
	return nil
}
