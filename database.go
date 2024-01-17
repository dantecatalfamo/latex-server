package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func NewDatabse(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("NewDatabase: %w", err)
	}

	database := &Database{ db: db }
	if err := database.Migrate(); err != nil {
		return nil, fmt.Errorf("NewDatabse: %w", err)
	}

	return database, nil
}

func (db *Database) Migrate() error {
	row := db.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_info'")
	if row.Err() != nil {
		return fmt.Errorf("Migrate query row sqlite_master: %w", row.Err())
	}

	var tablesExist int
	if err := row.Scan(&tablesExist); err != nil {
		return fmt.Errorf("Migrate scanning version: %w", err)
	}

	// The lowest migration is the index into the migrations array + 1
	var lowestMigration int

	// Tables don't exist, start migrating from 0
	if tablesExist > 0 {
		row = db.db.QueryRow("SELECT version FROM schema_info")
		if row.Err() != nil {
			return fmt.Errorf("Migrate query row schema_info: %w", row.Err())
		}

		if err := row.Scan(&lowestMigration); err != nil {
			return fmt.Errorf("Migrate scan schema_info: %w", err)
		}
	}

	// No migrations needed
	if lowestMigration == len(migrations) {
		return nil
	}

	for index, migration := range migrations[lowestMigration:] {
		if _, err := db.db.Exec(migration); err != nil {
			return fmt.Errorf("Migrate applying migration: %w", err)
		}

		if _, err := db.db.Exec("UPDATE schema_info SET version = ?", index + 1); err != nil {
			return fmt.Errorf("Migrate: applying migration: %w", err)
		}
	}

	return nil
}
