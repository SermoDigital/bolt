package graph

import (
	"database/sql"
	"fmt"
)

const (
	// UnboundRelationshipSignature is the signature byte for a UnboundRelationship object
	UnboundRelationshipSignature = 0x72
)

// UnboundRelationship Represents a UnboundRelationship structure
type UnboundRelationship struct {
	RelIdentity int64
	Type        string
	Properties  map[string]interface{}
}

// Signature gets the signature byte for the struct
func (r UnboundRelationship) Signature() uint8 {
	return UnboundRelationshipSignature
}

// Fields gets the fields to encode for the struct
func (r UnboundRelationship) Fields() []interface{} {
	return []interface{}{r.RelIdentity, r.Type, r.Properties}
}

func (u *UnboundRelationship) Scan(val interface{}) error {
	u0, ok := val.(UnboundRelationship)
	if !ok {
		return fmt.Errorf("UnboundRelationship.Scan: unknown type: %T", val)
	}
	*u = u0
	return nil
}

var _ sql.Scanner = (*UnboundRelationship)(nil)
