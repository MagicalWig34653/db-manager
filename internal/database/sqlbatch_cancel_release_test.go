package database

import (
	"context"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// When a request's context ends, database/sql rolls the transaction back and closes
// the connection in a background goroutine. The SQLite file (exclusive flock) must
// nevertheless be released once closeDB returns; otherwise the next request sees
// database_busy (CI flake in TestSQLiteSQLBatchRollbackAndSandbox).
func TestSQLiteCloseWaitsForCanceledTransaction(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE target (id INTEGER PRIMARY KEY)")
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		db, closeDB, err := e.open(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO target VALUES(?)", i); err != nil {
			t.Fatal(err)
		}
		cancel()
		closeDB()
		if _, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "SELECT count(*) FROM target"}); err != nil {
			t.Fatalf("round %d: database still held after close: %v", i, err)
		}
	}
}
