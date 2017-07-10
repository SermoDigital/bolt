package bolt

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
)

var neo4jConnStr = ""

func expect(t *testing.T, r *sql.Rows, n int) {
	if r == nil {
		if n != 0 {
			panic(fmt.Sprintf("wanted %d rows, got nil r", n))
		}
		return
	}
	var err error
	defer func() {
		err2 := r.Close()
		if err2 != nil {
			if err != nil {
				panic(fmt.Sprintf("two errors (close and orig): %#v %#v", err2, err))
			}
			panic(err2)
		}
	}()
	var i int
	for r.Next() {
		i++
	}
	if err = r.Err(); err != nil && (err != sql.ErrNoRows && n != 0) {
		panic(err)
	}
	if i != n {
		panic(fmt.Sprintf("expected %d rows, got %d", n, i))
	}
}

func TestMain(m *testing.M) {
	neo4jConnStr = os.Getenv("NEO4J_BOLT")
	if neo4jConnStr != "" {
		if testing.Verbose() {
			fmt.Println("Using NEO4J for tests:", neo4jConnStr)
		}
	} else if os.Getenv("ENSURE_NEO4J_BOLT") != "" {
		fmt.Println("Must give NEO4J_BOLT environment variable")
		os.Exit(1)
	}

	if neo4jConnStr != "" {
		// If we're using a DB for testing neo, clear it out after all the test runs
		defer clearNeo()
	}

	output := m.Run()

	os.Exit(output)
}

func clearNeo() {
	db, err := sql.Open(DefaultDriver, neo4jConnStr)
	if err != nil {
		panic(fmt.Sprintf("error getting conn to clear DB: %s\n", err))
	}
	defer db.Close()

	_, err = db.Exec("MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		panic("Error running query to clear DB")
	}
}
