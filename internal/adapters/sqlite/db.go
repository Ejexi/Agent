package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/SecDuckOps/shared/infra/sqlite/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

// Open connection to SQLite  configure usage with WAL mode
// foreign keys explicitly enabled
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn) //connection pool
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure SQLite for concurrent use and relational integrity
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA strict=ON;",
	}

	for _, pragma := range pragmas {
		_, err := db.Exec(pragma)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set %s: %w", pragma, err)
		}
	}

	return db, nil
}

// RunMigrations executes the initial schema definition
func RunMigrations(db *sql.DB) error {
	// 1) Load the embedded SQL files
	d, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to init iofs for migrations: %w", err)
	}

	// 2. Connect the migrator to SQLite
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create sqlite migration driver: %w", err)
	}
	
	m, err := migrate.NewWithInstance("iofs", d, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrator: %w", err)
	}

	// 3. Carefully run only the missing migrations!
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}