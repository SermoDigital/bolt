package bolt

import (
	"context"
	"fmt"
	"time"
)

// Summary lists details of the query, like profiling information and what type
// of statement was run.
type Summary struct {
	Query          string
	Counters       Counters
	Type           Type
	Plan           Plan
	Notifications  []Notification
	AvailableAfter time.Duration
	ConsumedAfter  time.Duration
	ServerInfo     ServerInfo
}

func (s *Summary) parseSuccess(md map[string]interface{}) {
	if typ, ok := md["type"].(string); ok {
		s.Type.SetString(typ)
	}

	if stats, ok := md["stats"].(map[string]interface{}); ok {
		s.Counters.parse(stats)
	}

	for _, key := range [...]string{"plan", "profile"} {
		if plan, ok := md[key].(map[string]interface{}); ok {
			s.Plan.parse(plan)
			break
		}
	}

	nots, ok := md["notifications"].([]interface{})
	if ok {
		s.Notifications = make([]Notification, len(nots))
		for i, not := range nots {
			if nmd, ok := not.(map[string]interface{}); ok {
				s.Notifications[i].parse(nmd)
			}
		}
	}

	if v, ok := md["result_available_after"].(int64); ok {
		s.AvailableAfter = time.Duration(v) * time.Millisecond
	}
	if v, ok := md["result_consumed_after"].(int64); ok {
		s.ConsumedAfter = time.Duration(v) * time.Millisecond
	}

	if vers, ok := md["server"].(string); ok {
		s.ServerInfo.Version = vers
	}
}

// ServerInfo describes basic information on the server that ran the query.
type ServerInfo struct {
	// Address is the remote address of the server where the query was executed.
	Address string
	// Version is a string indicating which version of the server executed the
	// query.
	Version string
}

// Counters counts the number of different operations the query performed.
type Counters struct {
	NodesCreated         int64
	NodesDeleted         int64
	RelationshipsCreated int64
	RelationshipsDeleted int64
	PropertiesSet        int64
	LabelsAdded          int64
	LabelsRemoved        int64
	IndicesAdded         int64
	IndicesRemoved       int64
	ConstraintsAdded     int64
	ConstraintsRemoved   int64
}

func (c *Counters) parse(md map[string]interface{}) {
	statsFor := func(s string) int64 {
		v, _ := md[s].(int64)
		return v
	}
	c.NodesCreated = statsFor("nodes-created")
	c.NodesDeleted = statsFor("nodes-deleted")
	c.RelationshipsCreated = statsFor("relationships-created")
	c.RelationshipsDeleted = statsFor("relationships-deleted")
	c.PropertiesSet = statsFor("properties-set")
	c.LabelsAdded = statsFor("labels-added")
	c.LabelsRemoved = statsFor("labels-removed")
	c.IndicesAdded = statsFor("indices-added")
	c.IndicesRemoved = statsFor("indices-removed")
	c.ConstraintsAdded = statsFor("constraints-added")
	c.ConstraintsRemoved = statsFor("donstraints-removed")
}

// RowsAffected returns the number of nodes and relationships created and
// deleted during the query. It partially implements driver.Result.
func (c Counters) RowsAffected() (int64, error) {
	return c.NodesCreated + c.NodesDeleted +
		c.RelationshipsCreated + c.RelationshipsDeleted, nil
}

// Type describes the type of query.
type Type uint8

const (
	Read        Type = iota // read only
	ReadWrite               // read and write
	Write                   // write only
	SchemaWrite             // schema write only
)

func (t *Type) SetString(s string) {
	switch s {
	case "r":
		*t = Read
	case "rw":
		*t = ReadWrite
	case "w":
		*t = Write
	case "s":
		*t = SchemaWrite
	}
}

func (t Type) String() string {
	switch t {
	case Read:
		return "r"
	case ReadWrite:
		return "rw"
	case Write:
		return "w"
	case SchemaWrite:
		return "s"
	default:
		return fmt.Sprintf("unknown Type: %d", t)
	}
}

// Notification represents a notification that might occur during the
// exectution of a query.
type Notification struct {
	// Code is the notification code.
	Code string
	// Title is a short summary of the notification.
	Title string
	// Description is a longer description of the notification.
	Description string
	// Position is the position in a query this notificaiton points to.
	Position Position
	// Severity is the severity level of the notification.
	Severity string
}

func (n *Notification) parse(md map[string]interface{}) {
	n.Code, _ = md["code"].(string)
	n.Title, _ = md["title"].(string)
	n.Description, _ = md["description"].(string)
	if pos, ok := md["position"].(map[string]interface{}); ok {
		n.Position.parse(pos)
	}

	var ok bool
	n.Severity, ok = md["severity"].(string)
	if !ok {
		n.Severity = "N/A"
	}
}

// Position is the position in a query a notification points to.
type Position struct {
	// Offset is the character offset this position points to, starting at 0.
	Offset int64
	// Line is the line number this position points to, starting at 1.
	Line int64
	// Column is the column number this position points to, starting at 1.
	Column int64
}

func (p *Position) parse(md map[string]interface{}) {
	p.Offset, _ = md["offset"].(int64)
	p.Line, _ = md["line"].(int64)
	p.Column, _ = md["column"].(int64)
}

// Plan describes the plan the database planner used when executing the query.
type Plan struct {
	// Operation is the type of operation the plan is performing.
	Operation string
	// Args contains the arguments the planner uses to during its execution.
	Args map[string]interface{}
	// Identifiers are identifiers used by the plan and can be generated by
	// either the user or planner.
	Identifiers []string
	// Profile is the executed plan.
	Profile *Profile
	// Children returns the next level of the planning tree.
	Children []Plan
}

// Profile describes an executed plan.
type Profile struct {
	// Hits is the number of time the plan touched the underlying data stores.
	Hits int64
	// Records is the number of records the plan produced.
	Records int64
}

func (p *Plan) parse(md map[string]interface{}) {
	p.Operation, _ = md["operatorType"].(string)
	p.Args, _ = md["args"].(map[string]interface{})

	ids, ok := md["identifiers"].([]interface{})
	if ok {
		p.Identifiers = make([]string, len(ids))
		for i, id := range ids {
			p.Identifiers[i], _ = id.(string)
		}
	}

	hits, hok := md["dbHits"].(int64)
	records, rok := md["rows"].(int64)
	if hok || rok {
		p.Profile = &Profile{Hits: hits, Records: records}
	}

	kids, ok := md["children"].([]interface{})
	if ok {
		p.Children = make([]Plan, len(kids))
		for i, kid := range kids {
			if kmd, ok := kid.(map[string]interface{}); ok {
				p.Children[i].parse(kmd)
			}
		}
	}
}

// summaryKey is used to access a channel for recieving row metadata.
type summaryKey struct{}

// WithSummary returns a context.Context that facilitates passing the summary
// of a Cypher query. The returned function is only valid after Rows.Close has
// been called or after Exec/ExecContext has returned. The function is
// idempotent.
func WithSummary(ctx context.Context) (context.Context, func() *Summary) {
	var s Summary
	return context.WithValue(ctx, summaryKey{}, &s), func() *Summary { return &s }
}

// fromContext returns the metadata channel. It never returns nil.
func fromContext(ctx context.Context) *Summary {
	s, ok := ctx.Value(summaryKey{}).(*Summary)
	if !ok {
		s = new(Summary)
	}
	return s
}
