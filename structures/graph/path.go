package graph

import (
	"database/sql"
	"fmt"
)

const (
	// PathSignature is the signature byte for a Path object
	PathSignature = 0x50
)

// Path Represents a Path structure
type Path struct {
	Nodes         []Node
	Relationships []UnboundRelationship
	Sequence      []int
}

// Signature gets the signature byte for the struct
func (p Path) Signature() uint8 {
	return PathSignature
}

// Fields gets the fields to encode for the struct
func (p Path) Fields() []interface{} {
	nodes := make([]interface{}, len(p.Nodes))
	for i, node := range p.Nodes {
		nodes[i] = node
	}
	relationships := make([]interface{}, len(p.Relationships))
	for i, relationship := range p.Relationships {
		relationships[i] = relationship
	}
	sequences := make([]interface{}, len(p.Sequence))
	for i, sequence := range p.Sequence {
		sequences[i] = sequence
	}
	return []interface{}{nodes, relationships, sequences}
}

func (p *Path) Scan(val interface{}) error {
	p0, ok := val.(Path)
	if !ok {
		return fmt.Errorf("Path.Scan: unknown type: %T", val)
	}
	*p = p0
	return nil
}

var _ sql.Scanner = (*Path)(nil)
