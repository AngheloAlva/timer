package sqlite

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migrateSqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps a *sql.DB connection with helpers specific to this project.
// We keep the underlying connection unexported so callers can't bypass
// our pragmas/migrations setup.
type DB struct {
	conn *sql.DB
}

// Open creates (or opens) the SQLite database at path, applies pragmas,
// and runs any pending migrations. Caller is responsible for calling Close.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Pin the pool to a single connection. SQLite serialises writes
	// anyway, and this guarantees PRAGMAs (notably busy_timeout) apply
	// to every query — pragmas in modernc.org/sqlite are per-connection
	// and a multi-connection pool would silently lose them on new conns,
	// surfacing as SQLITE_BUSY under the MCP's concurrent request handling.
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := applyPragmas(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply pragmas: %w", err)
	}

	if err := runMigrations(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close releases the underlying connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn exposes the raw *sql.DB so service/storage layers can run queries.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// applyPragmas configures the connection with the project-wide SQLite
// settings: foreign keys ON, WAL journaling, busy timeout.
func applyPragmas(db *sql.DB) error {
	prgmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
	}

	for _, pragma := range prgmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("apply pragma %s: %w", pragma, err)
		}
	}

	return nil
}

// runMigrations applies any pending migrations from the embedded FS.
func runMigrations(db *sql.DB) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	driver, err := migrateSqlite.WithInstance(db, &migrateSqlite.Config{})
	if err != nil {
		return fmt.Errorf("build migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
