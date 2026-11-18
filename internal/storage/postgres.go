package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Postgres{Pool: pool}, nil
}

func (p *Postgres) Close() {
	if p != nil && p.Pool != nil {
		p.Pool.Close()
	}
}

// migrationAdvisoryLockKey is the pg_advisory_lock key that serialises
// migration runs (the migrate container and API instances started with
// RUN_MIGRATIONS_ON_START). It is an arbitrary constant ("KOOMMIGR" in ASCII);
// do not change it, or old and new runners would stop excluding each other.
const migrationAdvisoryLockKey int64 = 0x4b4f4f4d4d494752

// migrationFile is one *.sql file from the migrations directory.
type migrationFile struct {
	Name     string
	Content  []byte
	Checksum string
}

// migrationChecksum returns the hex sha256 of a migration file. Line endings
// are normalised to LF first so that a Windows checkout (core.autocrlf=true)
// and a Linux CI checkout of the same commit produce the same checksum.
func migrationChecksum(content []byte) string {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

// loadMigrationFiles reads every *.sql file in dir, sorted by name.
func loadMigrationFiles(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	files := make([]migrationFile, 0, len(names))
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		files = append(files, migrationFile{Name: name, Content: content, Checksum: migrationChecksum(content)})
	}
	return files, nil
}

// appliedMigration is a row of schema_migrations. Checksum is empty for rows
// recorded by the legacy runner (NULL column).
type appliedMigration struct {
	Checksum string
}

// migrationPlan is the outcome of comparing migration files with
// schema_migrations.
type migrationPlan struct {
	// Pending are files that have not been applied yet, in order.
	Pending []migrationFile
	// Backfill are applied files whose recorded checksum is NULL (legacy rows);
	// their current checksum is recorded without re-running them.
	Backfill []migrationFile
}

// ErrMigrationChecksumMismatch is returned when an already applied migration
// file was edited after it ran.
var ErrMigrationChecksumMismatch = errors.New("applied migration file was modified")

// planMigrations decides which files to apply and which legacy rows to
// backfill. It fails if the file of an applied migration no longer matches the
// checksum recorded when it ran. Applied rows whose file no longer exists are
// ignored, as are legacy rows (empty checksum).
func planMigrations(files []migrationFile, applied map[string]appliedMigration) (migrationPlan, error) {
	var plan migrationPlan
	var mismatched []string
	for _, file := range files {
		row, ok := applied[file.Name]
		if !ok {
			plan.Pending = append(plan.Pending, file)
			continue
		}
		switch {
		case row.Checksum == "":
			plan.Backfill = append(plan.Backfill, file)
		case !strings.EqualFold(row.Checksum, file.Checksum):
			mismatched = append(mismatched, fmt.Sprintf("%s (recorded %s, file %s)", file.Name, row.Checksum, file.Checksum))
		}
	}
	if len(mismatched) > 0 {
		return migrationPlan{}, fmt.Errorf("%w: %s; never edit an applied migration, add a new one instead", ErrMigrationChecksumMismatch, strings.Join(mismatched, ", "))
	}
	return plan, nil
}

// RunMigrations applies pending migrations from dir. The whole run holds a
// session-level Postgres advisory lock so concurrent runners (e.g. the migrate
// container and an API started with RUN_MIGRATIONS_ON_START) cannot race.
// Each applied migration is recorded with a sha256 checksum; editing an
// already applied migration makes the run fail.
func (p *Postgres) RunMigrations(ctx context.Context, dir string) error {
	files, err := loadMigrationFiles(dir)
	if err != nil {
		return err
	}

	conn, err := p.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		// Use a fresh context: ctx may already be cancelled. If unlocking fails
		// the lock is released anyway when the session ends, so make sure the
		// connection is not returned to the pool still holding it.
		unlockCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockKey); err != nil {
			_ = conn.Conn().Close(unlockCtx)
		}
	}()

	return runMigrationsLocked(ctx, conn.Conn(), files)
}

func runMigrationsLocked(ctx context.Context, conn *pgx.Conn, files []migrationFile) error {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT`); err != nil {
		return fmt.Errorf("add schema_migrations.checksum: %w", err)
	}

	applied, err := loadAppliedMigrations(ctx, conn)
	if err != nil {
		return err
	}
	plan, err := planMigrations(files, applied)
	if err != nil {
		return err
	}

	for _, file := range plan.Backfill {
		if _, err := conn.Exec(ctx, `UPDATE schema_migrations SET checksum = $2 WHERE version = $1 AND checksum IS NULL`, file.Name, file.Checksum); err != nil {
			return fmt.Errorf("backfill checksum for migration %s: %w", file.Name, err)
		}
	}

	for _, file := range plan.Pending {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", file.Name, err)
		}
		if _, err := tx.Exec(ctx, string(file.Content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", file.Name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version, checksum) VALUES ($1, $2)`, file.Name, file.Checksum); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", file.Name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", file.Name, err)
		}
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, conn *pgx.Conn) (map[string]appliedMigration, error) {
	rows, err := conn.Query(ctx, `SELECT version, COALESCE(checksum, '') FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[string]appliedMigration)
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = appliedMigration{Checksum: strings.TrimSpace(checksum)}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	return applied, nil
}
