package bolt

import (
	"context"
	"database/sql"
	"log"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sermodigital/bolt/structures/graph"
)

func ifcs(n int) []interface{} {
	x := make([]interface{}, n)
	for i := range x {
		x[i] = new(interface{})
	}
	return x
}

func deref(x ...interface{}) {
	for i := range x {
		x[i] = *(x[i]).(*interface{})
	}
}

func columns(t *testing.T, rows *sql.Rows) []string {
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	return cols
}

func TestBoltStmt_SelectOne(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SelectOne", neo4jConnStr)

	stmt, err := driver.Prepare("RETURN 1;")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	ctx, fn := WithSummary(context.Background())
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	if rows, err := rows.Columns(); err != nil || rows[0] != "1" {
		t.Fatalf("unexpected Columns result: (%v, %#v)", rows, err)
	}

	var out int64
	for rows.Next() {
		if err := rows.Scan(&out); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if out != 1 {
		t.Fatalf("unexpected output. Expected 1. Got: %d", out)
	}

	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	sum := fn()
	if sum.Type != Read {
		t.Fatalf("wanted Read, got %s", sum.Type)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SelectMany(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SelectMany", neo4jConnStr)

	stmt, err := driver.Prepare(`RETURN 1, 34234.34323, "string", [1, "2", 3, true, null], true, null;`)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	ctx, fn := WithSummary(context.Background())
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	gcols := columns(t, rows)
	wcols := []string{
		"1",
		"34234.34323",
		"\"string\"",
		"[1, \"2\", 3, true, null]",
		"true",
		"null",
	}
	if !reflect.DeepEqual(gcols, wcols) {
		t.Fatalf("unexpected columns, wanted: %#v, got %#v", wcols, gcols)
	}

	output := struct {
		a int64
		b float64
		c string
		d Array
		e bool
		f interface{}
	}{d: Array{}}

	for rows.Next() {
		err := rows.Scan(&output.a, &output.b, &output.c, &output.d, &output.e, &output.f)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if output.a != 1 {
		t.Fatalf("unexpected output. Expected 1. Got: %#v", output.a)
	}
	if output.b != 34234.34323 {
		t.Fatalf("unexpected output. Expected 34234.34323. Got: %#v", output.b)
	}
	if output.c != "string" {
		t.Fatalf("unexpected output. Expected string. Got: %#v", output.c)
	}
	if !reflect.DeepEqual(
		output.d, Array{int64(1), "2", int64(3), true, interface{}(nil)}) {
		t.Fatalf("unexpected output. Expected []interface{}{1, '2', 3, true, nil}. Got: %#v", output.d)
	}
	if !output.e {
		t.Fatalf("unexpected output. Expected true. Got: %#v", output.e)
	}
	if output.f != nil {
		t.Fatalf("unexpected output. Expected nil. Got: %#v", output.f)
	}

	err = rows.Close()
	if err != nil {
		t.Fatal(err)
	}

	sum := fn()
	if sum.Type != Read {
		t.Fatalf("wanted Read, got %s", sum.Type)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_InvalidArgs(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_InvalidArgs", neo4jConnStr)

	stmt, err := driver.Prepare("CREATE (f:FOO {a: {a}, b: {b}, c: {c}, d: {d}, e: {e}, f: {f}}) RETURN f")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	args := Map{
		"a": 1,
		"b": 34234.34323,
		"c": "string",
		"d": []interface{}{int64(1), "2", int64(3), true, interface{}(nil)},
		"e": true,
		"f": nil,
	}

	_, err = stmt.Query(args)

	const expected = "Collections containing mixed types can not be stored in properties"
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("did not receive expected error, got: %s", err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_ExecNeo(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_ExecNeo", neo4jConnStr)

	stmt, err := driver.Prepare("CREATE (f:FOO {a: {a}, b: {b}, c: {c}, d: {d}, e: {e}, f: {f}, g: {g}, h: {h}})-[b:BAR]->(c:BAZ)")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	params := Map{
		"a": "foo",
		"b": 1,
		"c": true,
		"d": nil,
		"e": []interface{}{1, 2, 3},
		"f": 3.4,
		"g": -1,
		"h": false,
	}
	result, err := stmt.Exec(params)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("error getting rows affected: %s", err)
	}

	if affected != 3 {
		t.Fatalf("unexpected rows affected from create node. Expected %#v. Got: %#v", 3, affected)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare(`MATCH (f:FOO) SET f.a = "bar";`)
	if err != nil {
		t.Fatalf("an error occurred preparing update statement: %s", err)
	}

	result, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on update query to Neo: %s", err)
	}

	affected, err = result.RowsAffected()
	if err != nil {
		t.Fatalf("error getting update rows affected: %s", err)
	}

	if affected != 0 {
		t.Fatalf("unexpected rows affected from update node. Expected %#v. Got: %#v", 0, affected)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO)-[b:BAR]->(c:BAZ) DELETE f, b, c")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	result, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	affected, err = result.RowsAffected()
	if err != nil {
		t.Fatalf("error getting delete rows affected: %s", err)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	if affected != 3 {
		t.Fatalf("unexpected rows affected from delete node. Expected %#v. Got: %#v", 3, affected)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_CreateArgs(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_CreateArgs", neo4jConnStr)

	stmt, err := driver.Prepare("CREATE (f:FOO {a: {a}, b: {b}, c: {c}, d: {d}, e: {e}, f: {f}}) RETURN f.a, f.b, f.c, f.d, f.e, f.f")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	args := Map{
		"a": 1,
		"b": 34234.34323,
		"c": "string",
		"d": []interface{}{1, 2, 3},
		"e": true,
		"f": nil,
	}
	rows, err := stmt.Query(args)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output := ifcs(6)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}
	deref(output...)
	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	if output[0].(int64) != 1 {
		t.Fatalf("unexpected output. Expected 1. Got: %#v", output[0])
	}
	if output[1].(float64) != 34234.34323 {
		t.Fatalf("unexpected output. Expected 34234.34323. Got: %#v", output[1])
	}
	if output[2].(string) != "string" {
		t.Fatalf("unexpected output. Expected string. Got: %#v", output[2])
	}
	if !reflect.DeepEqual(output[3].([]interface{}), []interface{}{int64(1), int64(2), int64(3)}) {
		t.Fatalf("unexpected output. Expected []interface{}{1, 2, 3}. Got: %#v", output[3])
	}
	if !output[4].(bool) {
		t.Fatalf("unexpected output. Expected true. Got: %#v", output[4])
	}
	if output[5] != nil {
		t.Fatalf("unexpected output. Expected nil. Got: %#v", output[5])
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) DELETE f")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_Discard(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_Discard", neo4jConnStr)

	stmt, err := driver.Prepare(`CREATE (f:FOO {a: "1"}), (b:FOO {a: "2"}) RETURN f, b`)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	if _, err := stmt.Query(); err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	// Closing stmt should discard stream when it wasn't yet consumed
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) RETURN f.a ORDER BY f.a")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	rows, err := stmt.Query()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	var output []int64
	var x int64
	for rows.Next() {
		err := rows.Scan(&x)
		if err != nil {
			t.Fatal(err)
		}
		output = append(output, x)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if output[0] != 1 {
		t.Fatalf("unexpected return data: %#v", err)
	}

	// Closing in middle of record stream
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) RETURN f.a ORDER BY f.a")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	rows, err = stmt.Query()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output = output[0:0]
	for rows.Next() {
		err := rows.Scan(&x)
		if err != nil {
			t.Fatal(err)
		}
		output = append(output, x)
	}

	if output[0] != 1 {
		t.Fatalf("unexpected return data: %#v", output[0])
	}

	if output[1] != 2 {
		t.Fatalf("unexpected return data: %#v", output[1])
	}

	// Ensure we're getting proper data in subsequent queries
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) DELETE f")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_Failure(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_Failure", neo4jConnStr)

	stmt, err := driver.Prepare(`CREATE (f:FOO {a: "1"}), (b:FOO {a: "2"}) RETURN f, b`)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Check a failure from an invalid query
	stmt, err = driver.Prepare("This is an invalid query")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	_, err = stmt.Query()
	if err == nil {
		t.Fatalf("invalid query should return an error")
	}

	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) RETURN f.a ORDER BY f.a")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	rows, err := stmt.Query()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	var output []int64
	var x int64
	for rows.Next() {
		err := rows.Scan(&x)
		if err != nil {
			t.Fatal(err)
		}
		output = append(output, x)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}
	if err := rows.Close(); err != nil {
		log.Fatal(err)
	}

	if output[0] != 1 {
		t.Fatalf("unexpected return data: %#v", output[0])
	}

	if output[1] != 2 {
		t.Fatalf("unexpected return data: %#v", output[1])
	}

	// Closing in middle of record stream
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) RETURN f.a ORDER BY f.a")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	rows, err = stmt.Query()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output = output[0:0]
	for rows.Next() {
		err := rows.Scan(&x)
		if err != nil {
			t.Fatal(err)
		}
		output = append(output, x)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if output[0] != 1 {
		t.Fatalf("unexpected return data: %#v", output[0])
	}

	if output[1] != 2 {
		t.Fatalf("unexpected return data: %#v", output[1])
	}

	// Ensure we're getting proper data in subsequent queries
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) DELETE f")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_MixedObjects(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_MixedObjects", neo4jConnStr)

	stmt, err := driver.Prepare(`CREATE (f:FOO {a: "1"})-[b:TO]->(c:BAR)<-[d:FROM]-(e:BAZ) RETURN f, b, c, d, e`)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	rows, err := stmt.Query()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output := ifcs(5)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}
	deref(output...)
	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if output[0].(graph.Node).Labels[0] != "FOO" {
		t.Fatalf("unexpected return data: %s", err)
	}
	if output[1].(graph.Relationship).Type != "TO" {
		t.Fatalf("unexpected return data: %s", err)
	}
	if output[2].(graph.Node).Labels[0] != "BAR" {
		t.Fatalf("unexpected return data: %s", err)
	}
	if output[3].(graph.Relationship).Type != "FROM" {
		t.Fatalf("unexpected return data: %s", err)
	}
	if output[4].(graph.Node).Labels[0] != "BAZ" {
		t.Fatalf("unexpected return data: %s", err)
	}

	// Closing in middle of record stream
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO)-[b:TO]->(c:BAR)<-[d:FROM]-(e:BAZ) DELETE f, b, c, d, e")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_Path(t *testing.T) {

	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_Path", neo4jConnStr)

	stmt, err := driver.Prepare(`CREATE path=(f:FOO {a: "1"})-[b:TO]->(c:BAR)<-[d:FROM]-(e:BAZ) RETURN path`)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	rows, err := stmt.Query()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output := ifcs(1)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}
	deref(output...)
	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	path, ok := output[0].(graph.Path)
	if !ok {
		t.Fatalf("unrecognized return type for path: %#v", output[0])
	}

	if path.Nodes[0].Labels[0] != "FOO" {
		t.Fatalf("unexpected node return data 1: %#v", path)
	}
	if path.Nodes[1].Labels[0] != "BAR" {
		t.Fatalf("unexpected node return data 2: %#v", path)
	}
	if path.Nodes[2].Labels[0] != "BAZ" {
		t.Fatalf("unexpected node return data 3: %#v", path)
	}
	if path.Relationships[0].Type != "TO" {
		t.Fatalf("unexpected relationship return data: %#v", path)
	}
	if path.Relationships[1].Type != "FROM" {
		t.Fatalf("unexpected relationship return data: %#v", path)
	}
	if path.Sequence[0] != 1 {
		t.Fatalf("unexpected sequence return data: %#v", path)
	}
	if path.Sequence[1] != 1 {
		t.Fatalf("unexpected sequence return data: %#v", path)
	}
	if path.Sequence[2] != -2 {
		t.Fatalf("unexpected sequence return data: %#v", path)
	}
	if path.Sequence[3] != 2 {
		t.Fatalf("unexpected sequence return data: %#v", path)
	}

	// Closing in middle of record stream
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO)-[b:TO]->(c:BAR)<-[d:FROM]-(e:BAZ) DELETE f, b, c, d, e")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SingleRel(t *testing.T) {

	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SingleRel", neo4jConnStr)

	stmt, err := driver.Prepare("CREATE (f:FOO)-[b:BAR {a: {a}, b: {b}, c: {c}, d: {d}, e: {e}, f: {f}, g: {g}, h: {h}}]->(c:BAZ) return b")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	params := Map{
		"a": "foo",
		"b": int64(math.MaxInt64),
		"c": true,
		"d": nil,
		"e": []interface{}{int64(1), int64(2), int64(3)},
		"f": 3.4,
		"g": int32(math.MaxInt32),
		"h": false,
	}
	rows, err := stmt.Query(params)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output := ifcs(1)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}
	deref(output...)
	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if output[0].(graph.Relationship).Properties["a"].(string) != "foo" {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Relationship).Properties["b"].(int64) != math.MaxInt64 {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if !output[0].(graph.Relationship).Properties["c"].(bool) {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Relationship).Properties["d"] != nil {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if !reflect.DeepEqual(output[0].(graph.Relationship).Properties["e"].([]interface{}), []interface{}{int64(1), int64(2), int64(3)}) {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Relationship).Properties["f"].(float64) != 3.4 {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Relationship).Properties["g"].(int64) != math.MaxInt32 {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Relationship).Properties["h"].(bool) {
		t.Fatalf("unexpected return data: %#v", output)
	}

	// Closing in middle of record stream
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO)-[b:BAR]->(c:BAZ) DELETE f, b, c")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SingleNode(t *testing.T) {

	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SingleNode", neo4jConnStr)

	stmt, err := driver.Prepare("CREATE (f:FOO {a: {a}, b: {b}, c: {c}, d: {d}, e: {e}, f: {f}, g: {g}, h: {h}}) return f")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	params := Map{
		"a": "foo",
		"b": 1,
		"c": true,
		"d": nil,
		"e": []interface{}{int64(1), int64(2), int64(3)},
		"f": 3.4,
		"g": -1,
		"h": false,
	}
	rows, err := stmt.Query(params)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output := ifcs(1)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}
	deref(output...)
	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if output[0].(graph.Node).Properties["a"].(string) != "foo" {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Node).Properties["b"].(int64) != 1 {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if !output[0].(graph.Node).Properties["c"].(bool) {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Node).Properties["d"] != nil {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if !reflect.DeepEqual(output[0].(graph.Node).Properties["e"].([]interface{}), []interface{}{int64(1), int64(2), int64(3)}) {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Node).Properties["f"].(float64) != 3.4 {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Node).Properties["g"].(int64) != -1 {
		t.Fatalf("unexpected return data: %#v", output)
	}
	if output[0].(graph.Node).Properties["h"].(bool) {
		t.Fatalf("unexpected return data: %#v", output)
	}

	// Closing in middle of record stream
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err = driver.Prepare("MATCH (f:FOO) DELETE f")
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred on delete query to Neo: %s", err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SelectIntLimits(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SelectIntLimits", neo4jConnStr)

	query := "RETURN {min64} as min64, {min32} as min32, {min16} as min16, {min8} as min8, -16, {max8} as max8, {max16} as max16, {max32} as max32, {max64} as max64"
	stmt, err := driver.Prepare(query)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	params := Map{
		"min64": math.MinInt64,
		"min32": math.MinInt32,
		"min16": math.MinInt16,
		"min8":  math.MinInt8,
		"max8":  math.MaxInt8,
		"max16": math.MaxInt16,
		"max32": math.MaxInt32,
		"max64": math.MaxInt64,
	}
	rows, err := stmt.Query(params)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	output := ifcs(9)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}

	deref(output...)

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if output[0].(int64) != math.MinInt64 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MinInt64, output[0])
	}
	if output[1].(int64) != math.MinInt32 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MinInt32, output[1])
	}
	if output[2].(int64) != math.MinInt16 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MinInt16, output[2])
	}
	if output[3].(int64) != math.MinInt8 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MinInt8, output[3])
	}
	if output[4].(int64) != -16 {
		t.Fatalf("unexpected output. Expected -16. Got: %d", output[4])
	}
	if output[5].(int64) != math.MaxInt8 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MaxInt8, output[5])
	}
	if output[6].(int64) != math.MaxInt16 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MaxInt16, output[6])
	}
	if output[7].(int64) != math.MaxInt32 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MaxInt32, output[7])
	}
	if output[8].(int64) != math.MaxInt64 {
		t.Fatalf("unexpected output. Expected %d. Got: %d", math.MaxInt64, output[8])
	}

	if rows.Next() {
		t.Fatalf("unexpected row closed output. Expected false. Got true")
	}

	err = rows.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SelectStringLimits(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SelectStringLimits", neo4jConnStr)

	query := "RETURN {a} as a, {b} as b, {c} as c, {d} as d"
	stmt, err := driver.Prepare(query)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	params := Map{
		"a": strings.Repeat("A", 15),
		"b": strings.Repeat("A", 16),
		"c": strings.Repeat("A", int(math.MaxUint8)+1),
		"d": strings.Repeat("A", int(math.MaxUint16)+1),
	}
	rows, err := stmt.Query(params)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}
	cols := columns(t, rows)

	output := ifcs(4)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}

	deref(output...)

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if !reflect.DeepEqual(cols, []string{"a", "b", "c", "d"}) {
		t.Fatalf("unexpected columns. %#v", columns(t, rows))
	}

	if output[0].(string) != params["a"].(string) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["a"], output[0])
	}
	if output[1].(string) != params["b"].(string) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["b"], output[1])
	}
	if output[2].(string) != params["c"].(string) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["c"], output[2])
	}
	if output[3].(string) != params["d"].(string) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["d"], output[3])
	}

	if rows.Next() {
		t.Fatalf("unexpected row output, wanted false got true")
	}

	err = rows.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SelectSliceLimits(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SelectSliceLimits", neo4jConnStr)

	query := "RETURN {a} as a, {b} as b, {c} as c, {d} as d"
	stmt, err := driver.Prepare(query)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	params := Map{
		"a": make([]interface{}, 15),
		"b": make([]interface{}, 16),
		"c": make([]interface{}, int(math.MaxUint8)+1),
		"d": make([]interface{}, int(math.MaxUint16)+1),
	}
	for _, v := range params {
		for i := range v.([]interface{}) {
			v.([]interface{})[i] = "A"
		}
	}

	rows, err := stmt.Query(params)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}
	cols := columns(t, rows)

	output := ifcs(4)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}

	deref(output...)

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if !reflect.DeepEqual(cols, []string{"a", "b", "c", "d"}) {
		t.Fatalf("unexpected columns. %#v", columns(t, rows))
	}

	if !reflect.DeepEqual(output[0].([]interface{}), params["a"].([]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["a"], output[0])
	}
	if !reflect.DeepEqual(output[1].([]interface{}), params["b"].([]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["b"], output[1])
	}
	if !reflect.DeepEqual(output[2].([]interface{}), params["c"].([]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["c"], output[2])
	}
	if !reflect.DeepEqual(output[3].([]interface{}), params["d"].([]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["d"], output[3])
	}

	if rows.Next() {
		t.Fatalf("unexpected row output, wanted false got true")
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SelectMapLimits(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_SelectMapLimits", neo4jConnStr)

	query := "RETURN {a} as a, {b} as b, {c} as c, {d} as d"
	stmt, err := driver.Prepare(query)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	params := Map{
		"a": make(map[string]interface{}, 15),
		"b": make(map[string]interface{}, 16),
		"c": make(map[string]interface{}, int(math.MaxUint8)+1),
		"d": make(map[string]interface{}, int(math.MaxUint16)+1),
	}

	for i := 0; i < int(math.MaxUint16)+1; i++ {
		key := strconv.Itoa(i)
		if i <= 15 {
			params["a"].(map[string]interface{})[key] = "-"
		}
		if i <= 16 {
			params["b"].(map[string]interface{})[key] = "-"
		}
		if i <= int(math.MaxUint8)+1 {
			params["c"].(map[string]interface{})[key] = "-"
		}
		if i <= int(math.MaxUint16)+1 {
			params["d"].(map[string]interface{})[key] = "-"
		}
	}

	rows, err := stmt.Query(params)
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}
	cols := columns(t, rows)

	output := ifcs(4)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}

	deref(output...)

	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if !reflect.DeepEqual(cols, []string{"a", "b", "c", "d"}) {
		t.Fatalf("unexpected columns. %#v", columns(t, rows))
	}

	if !reflect.DeepEqual(output[0].(map[string]interface{}), params["a"].(map[string]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["a"], output[0])
	}
	if !reflect.DeepEqual(output[1].(map[string]interface{}), params["b"].(map[string]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["b"], output[1])
	}
	if !reflect.DeepEqual(output[2].(map[string]interface{}), params["c"].(map[string]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["c"], output[2])
	}
	if !reflect.DeepEqual(output[3].(map[string]interface{}), params["d"].(map[string]interface{})) {
		t.Fatalf("unexpected output. Expected: %#v Got: %#v", params["d"], output[3])
	}

	if rows.Next() {
		t.Fatalf("unexpected row output, wanted false got true")
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_ManyChunks(t *testing.T) {
	// Records session for testing
	driver := newRecorder(t, "TestBoltStmt_ManyChunks", neo4jConnStr)

	//driver.SetChunkSize(10)
	query := `RETURN "1 2 3 4 5 6 7 8 9 10" as a,  "1 2 3 4 5 6 7 8 9 10" as b, "1 2 3 4 5 6 7 8 9 10" as c`
	stmt, err := driver.Prepare(query)
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}

	rows, err := stmt.Query()
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}
	cols := columns(t, rows)

	output := ifcs(3)
	for rows.Next() {
		err := rows.Scan(output...)
		if err != nil {
			t.Fatal(err)
		}
	}
	deref(output...)
	if err := rows.Err(); err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if !reflect.DeepEqual(cols, []string{"a", "b", "c"}) {
		t.Fatalf("unexpected columns. %#v", columns(t, rows))
	}

	if output[0].(string) != "1 2 3 4 5 6 7 8 9 10" {
		t.Fatalf("unexpected output. %#v", output[0])
	}
	if output[1].(string) != "1 2 3 4 5 6 7 8 9 10" {
		t.Fatalf("unexpected output. %#v", output[1])
	}
	if output[2].(string) != "1 2 3 4 5 6 7 8 9 10" {
		t.Fatalf("unexpected output. %#v", output[2])
	}

	if rows.Next() {
		t.Fatalf("unexpected row output, wanted false got true")
	}

	err = driver.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltStmt_SqlQueryAndExec(t *testing.T) {
	if neo4jConnStr == "" {
		t.Skip("Cannot run this test when in recording mode")
	}

	db, err := sql.Open("bolt", neo4jConnStr)
	if err != nil {
		t.Fatalf("an error occurred opening conn: %s", err)
	}
	defer db.Close()

	args := Map{
		"a": 1,
		"b": 34234.34323,
		"c": "string",
		"d": []interface{}{1, 2, 3},
		"e": true,
		"f": nil,
	}

	stmt, err := db.Prepare("CREATE path=(f:FOO {a: {a}, b: {b}, c: {c}, d: {d}, e: {e}, f: {f}})-[b:BAR]->(c:BAZ) RETURN f.a, f.b, f.c, f.d, f.e, f.f, f, b, path")
	if err != nil {
		t.Fatalf("an error occurred preparing statement: %s", err)
	}
	rows, err := stmt.Query(args)
	if err != nil {
		t.Fatalf("an error occurred querying statement: %s", err)
	}
	var a int
	var b float64
	var c string
	var d []interface{}
	var e bool
	var f interface{}
	var node graph.Node
	var rel graph.Relationship
	var path graph.Path
	if !rows.Next() {
		t.Fatalf("rows.Next failed")
	}
	err = rows.Scan(&a, &b, &c, &d, &e, &f, &node, &rel, &path)
	if err != nil {
		t.Fatalf("an error occurred scanning row: %s", err)
	}
	defer rows.Close()

	if a != 1 {
		t.Fatalf("unexpected value for a. Expected: %#v  Got: %#v", 1, a)
	}
	if b != 34234.34323 {
		t.Fatalf("unexpected value for b. Expected: %#v  Got: %#v", 34234.34323, b)
	}
	if c != "string" {
		t.Fatalf("unexpected value for c. Expected: %#v  Got: %#v", "string", b)
	}

	if !reflect.DeepEqual(d, []interface{}{int64(1), int64(2), int64(3)}) {
		t.Fatalf("unexpected value for d. Expected: %#v  Got: %#v", []interface{}{1, 2, 3}, d)
	}

	if !e {
		t.Fatalf("unexpected value for e. Expected: %#v  Got: %#v", true, e)
	}

	if f != nil {
		t.Fatalf("unexpected value for f. Expected: %#v  Got: %#v", nil, f)
	}

	if node.Labels[0] != "FOO" {
		t.Fatalf("unexpected label for node. Expected: %#v  Got: %#v", "FOO", node)
	}
	if node.Properties["a"] != int64(1) {
		t.Fatalf("unexpected value for node. Expected: %#v  Got: %#v", int64(1), node)
	}

	if rel.Type != "BAR" {
		t.Fatalf("unexpected label for node. Expected: %#v  Got: %#v", "FOO", rel)
	}

	if path.Nodes[0].Labels[0] != "FOO" {
		t.Fatalf("unexpected label for path node 0. Expected: %#v  Got: %#v", "FOO", path)
	}
	if path.Nodes[1].Labels[0] != "BAZ" {
		t.Fatalf("unexpected label for path node 1. Expected: %#v  Got: %#v", "BAZ", path)
	}
	if path.Relationships[0].Type != "BAR" {
		t.Fatalf("unexpected label for path relationship 1. Expected: %#v  Got: %#v", "BAR", path)
	}
	if path.Sequence[0] != 1 {
		t.Fatalf("unexpected label for path sequence 0. Expected: %#v  Got: %#v", 1, path)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatalf("an error occurred closing statement: %s", err)
	}

	stmt, err = db.Prepare("MATCH (f:FOO)-[b:BAR]->(c:BAZ) DELETE f, b, c")
	if err != nil {
		t.Fatal(err)
	}

	result, err := stmt.Exec()
	if err != nil {
		t.Fatalf("an error occurred preparing delete statement: %s", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("an error occurred getting affected rows: %s", err)
	}
	if affected != 3 {
		t.Fatalf("expected to delete 3 items, got %#v", affected)
	}

	err = stmt.Close()
	if err != nil {
		t.Fatalf("an error occurred closing statement: %s", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}
