package bolt

import (
	"context"
	"database/sql"
	"testing"
)

func newRecorder(t *testing.T, name, dsn string) *sql.DB {
	sql.Register(name, &Recorder{Name: name})
	db, err := sql.Open(name, dsn)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBoltConn_parseURL(t *testing.T) {
	v := make(values)

	err := parseURL(v, "bolt://john@foo:1234")
	if err == nil {
		t.Fatal("expected error from missing password")
	}

	err = parseURL(v, "bolt://john:password@foo:7687")
	if err != nil {
		t.Fatal("should not error on valid url")
	}

	if v.get("username") != "john" {
		t.Fatal("expected user to be 'john'")
	}
	if v.get("password") != "password" {
		t.Fatal("expected password to be 'password'")
	}

	err = parseURL(v, "bolt://john:password@foo:7687?tls=true")
	if err != nil {
		t.Fatal("should not error on valid url")
	}
	if v.get("tls") != "true" {
		t.Fatal("expected to use TLS")
	}

	err = parseURL(v, "bolt://john:password@foo:7687?tls=true&tls_no_verify=1&tls_ca_cert_file=ca&tls_cert_file=cert&tls_key_file=key")
	if err != nil {
		t.Fatal("should not error on valid url")
	}
	if v.get("tls") != "true" {
		t.Fatal("expected to use TLS")
	}
	if v.get("tls_no_verify") != "1" {
		t.Fatal("expected to use TLS with no verification")
	}
	if v.get("tls_ca_cert_file") != "ca" {
		t.Fatal("expected ca cert file 'ca'")
	}
	if v.get("tls_cert_file") != "cert" {
		t.Fatal("expected cert file 'cert'")
	}
	if v.get("tls_key_file") != "key" {
		t.Fatal("expected key file 'key'")
	}
}

func TestBoltConn_Close(t *testing.T) {
	// Records session for testing
	rec := newRecorder(t, "TestBoltConn_Close", neo4jConnStr)

	err := rec.Close()
	if err != nil {
		t.Fatalf("an error occurred closing conn: %s", err)
	}
}

func TestBoltConn_SelectOne(t *testing.T) {
	// Records session for testing
	rec := newRecorder(t, "TestBoltConn_SelectOne", neo4jConnStr)

	var out int64
	err := rec.QueryRow("RETURN 1;").Scan(&out)
	if err != nil {
		t.Fatalf("an error occurred getting next row: %s", err)
	}

	if out != 1 {
		t.Fatalf("unexpected output. Expected 1. Got: %d", out)
	}

	err = rec.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltConn_SelectAll(t *testing.T) {
	// Records session for testing
	rec := newRecorder(t, "TestBoltConn_SelectAll", neo4jConnStr)

	results, err := rec.Exec("CREATE (f:NODE {a: 1}), (b:NODE {a: 2})")
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}

	affected, err := results.RowsAffected()
	if err != nil {
		t.Fatalf("an error occurred getting rows affected: %s", err)
	}
	if affected != int64(2) {
		t.Fatalf("incorrect number of rows affected: %d", affected)
	}

	ctx, fn := WithSummary(context.Background())
	rows, err := rec.QueryContext(
		ctx,
		"MATCH (n:NODE) RETURN n.a ORDER BY n.a",
	)
	if err != nil {
		t.Fatal(err)
	}

	if rows, err := rows.Columns(); err != nil || rows[0] != "n.a" {
		t.Fatalf("unexpected result from Columns: (%v, %#v)", rows, err)
	}

	var (
		out []int64
		x   int64
	)
	for rows.Next() {
		if err := rows.Scan(&x); err != nil {
			t.Fatal(err)
		}
		out = append(out, x)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(out) != 2 {
		t.Fatalf("wanted len == 2, got %d", len(out))
	}

	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	metadata := fn()

	if out[0] != int64(1) {
		t.Fatalf("incorrect data returned for first row: %#v", out[0])
	}
	if out[1] != int64(2) {
		t.Fatalf("incorrect data returned for second row: %#v", out[1])
	}

	if metadata.Type != Read {
		t.Fatalf("unexpected request metadata: %#v", metadata)
	}

	results, err = rec.Exec("MATCH (n:NODE) DELETE n")
	if err != nil {
		t.Fatalf("an error occurred querying Neo: %s", err)
	}
	affected, err = results.RowsAffected()
	if err != nil {
		t.Fatalf("an error occurred getting rows affected: %s", err)
	}
	if affected != int64(2) {
		t.Fatalf("incorrect number of rows affected: %d", affected)
	}

	err = rec.Close()
	if err != nil {
		t.Fatalf("error closing connection: %s", err)
	}
}

func TestBoltConn_Ignored(t *testing.T) {
	// Records session for testing
	rec := newRecorder(t, "TestBoltConn_Ignored", neo4jConnStr)

	defer rec.Close()

	// This will make two calls at once - Run and Pull All. The pull all should
	// be ignored, which is what we're testing.
	_, err := rec.Query("syntax error", Map{"foo": 1, "bar": 2.2})
	if err == nil {
		t.Fatal("expected an error on syntax error.")
	}

	rows, err := rec.Query("RETURN 1;")
	if err != nil {
		t.Fatalf("got error when running next query after a failure: %#v", err)
	}
	defer rows.Close()

	var out int64
	for rows.Next() {
		rows.Scan(&out)
	}

	if out != 1 {
		t.Fatalf("expected different data from output: %#v", out)
	}
}
