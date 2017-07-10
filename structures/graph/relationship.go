package graph

import (
	"database/sql"
	"fmt"
)

const (
	// RelationshipSignature is the signature byte for a Relationship object
	RelationshipSignature = 0x52
)

// Relationship Represents a Relationship structure
type Relationship struct {
	RelIdentity       int64
	StartNodeIdentity int64
	EndNodeIdentity   int64
	Type              string
	Properties        map[string]interface{}
}

// Signature gets the signature byte for the struct
func (r Relationship) Signature() uint8 {
	return RelationshipSignature
}

// Fields gets the fields to encode for the struct
func (r Relationship) Fields() []interface{} {
	return []interface{}{r.RelIdentity, r.StartNodeIdentity, r.EndNodeIdentity, r.Type, r.Properties}
}

func (r *Relationship) Scan(val interface{}) error {
	r0, ok := val.(Relationship)
	if !ok {
		return fmt.Errorf("Relationship.Scan: unknown type: %T", val)
	}
	*r = r0
	return nil
}

var _ sql.Scanner = (*Relationship)(nil)
