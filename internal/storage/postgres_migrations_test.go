package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationChecksumIgnoresLineEndings(t *testing.T) {
	lf := migrationChecksum([]byte("CREATE TABLE a (id int);\nSELECT 1;\n"))
	crlf := migrationChecksum([]byte("CREATE TABLE a (id int);\r\nSELECT 1;\r\n"))
	if lf != crlf {
		t.Fatalf("checksum differs between LF and CRLF: %s vs %s", lf, crlf)
	}
	if len(lf) != 64 {
		t.Fatalf("expected hex sha256, got %q", lf)
	}
	if other := migrationChecksum([]byte("CREATE TABLE b (id int);\n")); other == lf {
		t.Fatal("different content produced the same checksum")
	}
}

func TestLoadMigrationFilesSortsAndFilters(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("0002_b.sql", "SELECT 2;")
	write("0001_a.sql", "SELECT 1;")
	write("README.md", "not a migration")
	if err := os.Mkdir(filepath.Join(dir, "0003_dir.sql"), 0o700); err != nil {
		t.Fatal(err)
	}

	files, err := loadMigrationFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Name != "0001_a.sql" || files[1].Name != "0002_b.sql" {
		t.Fatalf("unexpected files: %+v", files)
	}
	if files[0].Checksum != migrationChecksum([]byte("SELECT 1;")) {
		t.Fatal("checksum not computed from file content")
	}
}

func TestPlanMigrations(t *testing.T) {
	a := migrationFile{Name: "0001_a.sql", Checksum: migrationChecksum([]byte("a"))}
	b := migrationFile{Name: "0002_b.sql", Checksum: migrationChecksum([]byte("b"))}
	c := migrationFile{Name: "0003_c.sql", Checksum: migrationChecksum([]byte("c"))}
	files := []migrationFile{a, b, c}

	t.Run("fresh database applies everything", func(t *testing.T) {
		plan, err := planMigrations(files, map[string]appliedMigration{})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Pending) != 3 || len(plan.Backfill) != 0 {
			t.Fatalf("unexpected plan: %+v", plan)
		}
	})

	t.Run("legacy rows are backfilled, not re-applied", func(t *testing.T) {
		plan, err := planMigrations(files, map[string]appliedMigration{
			a.Name: {},
			b.Name: {Checksum: b.Checksum},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Backfill) != 1 || plan.Backfill[0].Name != a.Name {
			t.Fatalf("expected backfill of %s, got %+v", a.Name, plan.Backfill)
		}
		if len(plan.Pending) != 1 || plan.Pending[0].Name != c.Name {
			t.Fatalf("expected %s pending, got %+v", c.Name, plan.Pending)
		}
	})

	t.Run("checksum comparison is case-insensitive", func(t *testing.T) {
		_, err := planMigrations([]migrationFile{a}, map[string]appliedMigration{
			a.Name: {Checksum: strings.ToUpper(a.Checksum)},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("modified applied migration fails loudly", func(t *testing.T) {
		_, err := planMigrations(files, map[string]appliedMigration{
			a.Name: {Checksum: migrationChecksum([]byte("original a"))},
		})
		if !errors.Is(err, ErrMigrationChecksumMismatch) {
			t.Fatalf("expected checksum mismatch, got %v", err)
		}
		if !strings.Contains(err.Error(), a.Name) {
			t.Fatalf("error should name the migration: %v", err)
		}
	})

	t.Run("applied rows without a file are ignored", func(t *testing.T) {
		plan, err := planMigrations([]migrationFile{a}, map[string]appliedMigration{
			a.Name:          {Checksum: a.Checksum},
			"0000_gone.sql": {Checksum: "deadbeef"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Pending) != 0 || len(plan.Backfill) != 0 {
			t.Fatalf("unexpected plan: %+v", plan)
		}
	})
}

// TestRunMigrationsAgainstPostgres exercises the runner end to end. It creates
// and drops a scratch schema, so point TEST_DATABASE_URL at a disposable
// database only.
func TestRunMigrationsAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema := fmt.Sprintf("migtest_%d", time.Now().UnixNano())
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
	}()

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	db := &Postgres{Pool: pool}

	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Simulate a legacy schema_migrations table with an applied row and no checksum column.
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE legacy_table (id int); INSERT INTO schema_migrations(version) VALUES ('0001_legacy.sql')`); err != nil {
		t.Fatal(err)
	}
	write("0001_legacy.sql", "CREATE TABLE legacy_table (id int);")
	write("0002_counter.sql", "CREATE TABLE counter (n int); INSERT INTO counter VALUES (1);")

	// Concurrent runs must not apply 0002 twice.
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- db.RunMigrations(ctx, dir)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent run failed: %v", err)
		}
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM counter`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("expected one counter row, got %d (%v)", rows, err)
	}
	var nullChecksums int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE checksum IS NULL`).Scan(&nullChecksums); err != nil || nullChecksums != 0 {
		t.Fatalf("expected legacy checksum backfilled, got %d NULL rows (%v)", nullChecksums, err)
	}

	write("0002_counter.sql", "CREATE TABLE counter (n int); INSERT INTO counter VALUES (2);")
	if err := db.RunMigrations(ctx, dir); !errors.Is(err, ErrMigrationChecksumMismatch) {
		t.Fatalf("expected checksum mismatch after editing an applied migration, got %v", err)
	}
}
