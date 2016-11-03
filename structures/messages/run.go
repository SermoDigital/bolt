package messages

const (
	// RunMessageSignature is the signature byte for the RUN message
	RunSignature = 0x10
)

// Run Represents an RUN message
type Run struct {
	statement  string
	parameters map[string]interface{}
}

// NewRun Gets a new Run struct
func NewRunMessage(statement string, parameters map[string]interface{}) Run {
	return Run{statement: statement, parameters: parameters}
}

// Signature gets the signature byte for the struct
func (i Run) Signature() uint8 {
	return RunSignature
}

// Fields gets the fields to encode for the struct
func (i Run) Fields() []interface{} {
	return []interface{}{i.statement, i.parameters}
}
