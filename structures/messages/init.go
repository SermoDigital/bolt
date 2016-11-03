package messages

const (
	// InitMessageSignature is the signature byte for the INIT message
	InitSignature = 0x01
)

// Init Represents an INIT message
type Init struct {
	clientName string
	authToken  map[string]interface{}
}

// NewInit Gets a new Init struct
func NewInitMessage(clientName string, user string, password string) Init {
	msg := Init{clientName: clientName}
	if user == "" {
		msg.authToken = map[string]interface{}{"scheme": "none"}
	} else {
		msg.authToken = map[string]interface{}{
			"scheme":      "basic",
			"principal":   user,
			"credentials": password,
		}
	}
	return msg
}

// Signature gets the signature byte for the struct
func (i Init) Signature() uint8 {
	return InitSignature
}

// Fields gets the fields to encode for the struct
func (i Init) Fields() []interface{} {
	return []interface{}{i.clientName, i.authToken}
}
