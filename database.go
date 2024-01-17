package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	conn *sql.DB
}

func NewDatabse(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("NewDatabase: %w", err)
	}

	database := &Database{ conn: db }
	if err := database.Migrate(); err != nil {
		return nil, fmt.Errorf("NewDatabse: %w", err)
	}

	return database, nil
}

func (db *Database) Migrate() error {
	row := db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migration'")
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
		row = db.conn.QueryRow("SELECT version FROM schema_migration")
		if row.Err() != nil {
			return fmt.Errorf("Migrate query row schema_migration: %w", row.Err())
		}

		if err := row.Scan(&lowestMigration); err != nil {
			return fmt.Errorf("Migrate scan schema_migration: %w", err)
		}
	}

	// No migrations needed
	if lowestMigration == len(migrations) {
		return nil
	}

	for index, migration := range migrations[lowestMigration:] {
		log.Printf("Running database migration %d", index)
		if _, err := db.conn.Exec(migration); err != nil {
			return fmt.Errorf("Migrate applying migration: %w", err)
		}

		if _, err := db.conn.Exec("UPDATE schema_migration SET version = ?", index + 1); err != nil {
			return fmt.Errorf("Migrate: applying migration: %w", err)
		}
	}

	return nil
}

type ProjectInfo struct {
	Name string `json:"name"`
	Public bool `json:"public"`
	CreatedAt string `json:"createdAt"`
	LatestBuild BuildInfo `json:"latestBuild"`
}

type BuildInfo struct {
	BuildStart string `json:"buildStart"`
	BuildTime float64 `json:"buildTime"`
	Status string `json:"status"`
	Options ProjectBuildOptions `json:"options"`
}

func (db *Database) GetProjectInfo (ProjectInfo, error) {

}

func (db *Database) GetProjectId(user string, project string) (int, error) {
	userId, err := db.GetUserId(project)
	if err != nil {
		return 0, fmt.Errorf("Database.GetProjectId: %w", err)
	}

	// TODO This should be a join instead of two separate queries
	row := db.conn.QueryRow("SELECT id FROM projects WHERE user_id = ? AND name = ?", userId, project)
	if row.Err() != nil {
		return 0, fmt.Errorf("Database.GetProjectId query: %w", err)
	}

	var id int
	if err := row.Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}

func (db *Database) GetUserId(user string) (int, error) {
	row := db.conn.QueryRow("SELECT id FROM users WHERE name = ?", user)
	if row.Err() != nil {
		return 0, fmt.Errorf("Database.GetUserId query: %w", row.Err())
	}

	var id int
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("Database.GetUserId scan: %w", err)
	}

	return id, nil
}
