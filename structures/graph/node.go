package graph

import (
	"database/sql"
	"fmt"
)

const (
	// NodeSignature is the signature byte for a Node object
	NodeSignature = 0x4E
)

// Node Represents a Node structure
type Node struct {
	NodeIdentity int64
	Labels       []string
	Properties   map[string]interface{}
}

// Signature gets the signature byte for the struct.
func (n Node) Signature() uint8 {
	return NodeSignature
}

// Fields gets the fields to encode for the struct.
func (n Node) Fields() []interface{} {
	labels := make([]interface{}, len(n.Labels))
	for i, label := range n.Labels {
		labels[i] = label
	}
	return []interface{}{n.NodeIdentity, labels, n.Properties}
}

func (n *Node) Scan(val interface{}) error {
	n0, ok := val.(Node)
	if !ok {
		return fmt.Errorf("Node.Scan: unknown type: %T", val)
	}
	*n = n0
	return nil
}

var _ sql.Scanner = (*Node)(nil)
