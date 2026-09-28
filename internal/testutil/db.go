// Package testutil holds shared helpers for backend tests that need a real Postgres database.
//
// Tests that call SetupDB are skipped unless TEST_DATABASE_URL is set. The database name must end
// in "_test" because every table is truncated before each test. scripts/test-backend.sh creates
// the guac_test database in the docker-compose Postgres and runs the tests against it.
package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-guacamole/db"
)

var (
	setupOnce sync.Once
	setupErr  error
)

// SetupDB points db.Pool at the test database (running migrations on first use) and empties every
// table, so each test starts from a clean database.
func SetupDB(t *testing.T) context.Context {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database test (run scripts/test-backend.sh)")
	}

	setupOnce.Do(func() { setupErr = connect(url) })
	if setupErr != nil {
		t.Fatalf("test database setup failed: %v", setupErr)
	}

	ctx := context.Background()
	if err := truncateAll(ctx); err != nil {
		t.Fatalf("resetting test database: %v", err)
	}
	return ctx
}

func connect(url string) error {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("parsing TEST_DATABASE_URL: %w", err)
	}

	// every test wipes the database, so never run against anything that isn't clearly a test db
	if name := config.ConnConfig.Database; !strings.HasSuffix(name, "_test") {
		return fmt.Errorf("refusing to run tests against database %q: name must end in _test", name)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return err
	}
	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("connecting to test database: %w", err)
	}

	db.Pool = pool
	db.RunMigrations(pool)
	return nil
}

func truncateAll(ctx context.Context) error {
	rows, err := db.Pool.Query(ctx,
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations'`)
	if err != nil {
		return err
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return nil
	}

	for i, table := range tables {
		tables[i] = pgx.Identifier{table}.Sanitize()
	}
	_, err = db.Pool.Exec(ctx, `TRUNCATE `+strings.Join(tables, ", ")+` RESTART IDENTITY CASCADE`)
	return err
}

// User is a user row created for a test.
type User struct {
	ID   int
	UUID string
}

// CreateUser inserts a user and returns its id and uuid (the friend code).
func CreateUser(t *testing.T, ctx context.Context, name string) User {
	t.Helper()

	var u User
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO users (display_name, email, google_id) VALUES ($1, $2, $3) RETURNING id, uuid::text`,
		name, strings.ToLower(name)+"@example.com", "google-"+name,
	).Scan(&u.ID, &u.UUID)
	if err != nil {
		t.Fatalf("creating user %q: %v", name, err)
	}
	return u
}

// Follow makes follower follow followee.
func Follow(t *testing.T, ctx context.Context, follower, followee User) {
	t.Helper()

	_, err := db.Pool.Exec(ctx,
		`INSERT INTO users_follows (follower_id, followee_id) VALUES ($1, $2)`, follower.ID, followee.ID)
	if err != nil {
		t.Fatalf("following user: %v", err)
	}
}

// Count returns the result of a SELECT COUNT(*) query.
func Count(t *testing.T, ctx context.Context, query string, args ...any) int {
	t.Helper()

	var n int
	if err := db.Pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	return n
}
