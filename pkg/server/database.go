package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	SQLiteTime = "2006-01-02 15:04:05"
	SQLiteTimeNano = "2006-01-02 15:04:05.999999"
)

type Database struct {
	conn *sql.DB
}

func NewDatabse(path string) (*Database, error) {
	dbSpec := fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL&_foreign_keys=true", path)
	db, err := sql.Open("sqlite3", dbSpec)
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
	CreatedAt time.Time `json:"createdAt"`
	LatestBuild BuildInfo `json:"latestBuild"`
}

type BuildInfo struct {
	BuildStart time.Time `json:"buildStart"`
	BuildTime float64 `json:"buildTime"`
	Status string     `json:"status"`
	Options ProjectBuildOptions `json:"options"`
	BuildOut string    `json:"buildOut"`
}

func (db *Database) ListUserProjects(user string) ([]ProjectInfo, error) {
	userId, err := db.GetUserId(user)
	if err != nil {
		return nil, fmt.Errorf("ListUserProjects: %w", err)
	}

	// We select the latest build using max(b.id), even through we
	// don't actually care about that colummn
	// https://www.sqlite.org/lang_select.html#bareagg
	query := `
SELECT
  p.name,
  p.public,
  p.created_at,
  COALESCE(b.build_start, datetime(0, 'unixepoch')),
  COALESCE(b.build_time, 0),
  COALESCE(b.status, ''),
  COALESCE(b.options, '{}'),
  COALESCE(max(b.id), 0)
FROM
  projects p
LEFT JOIN
  builds b
ON
  p.id = b.project_id
WHERE
  p.user_id = ?
GROUP BY
  p.id
ORDER BY
  p.id DESC
`
	rows, err := db.conn.Query(query, userId)
	if err != nil {
		return nil, fmt.Errorf("ListUserProjects query: %w", err)
	}
	defer rows.Close()

	var infos []ProjectInfo

	for rows.Next() {
		var projectInfo ProjectInfo
		var unparsedOptions string
		var createdAt string
		var buildStart string
		var buildId int
		if err := rows.Scan(
			&projectInfo.Name,
			&projectInfo.Public,
			&createdAt,
			&buildStart,
			&projectInfo.LatestBuild.BuildTime,
			&projectInfo.LatestBuild.Status,
			&unparsedOptions,
			&buildId,
		); err != nil {
			return nil, fmt.Errorf("ListUserProjects scan: %w", err)
		}

		projectInfo.CreatedAt, err = time.Parse(SQLiteTime, createdAt)
		if err != nil {
			return nil, fmt.Errorf("ListUserProject parse createdAt time: %w", err)
		}

		projectInfo.LatestBuild.BuildStart, err = time.Parse(SQLiteTimeNano, buildStart)
		if err != nil {
			return nil, fmt.Errorf("ListUserProject parse buildStart time: %w", err)
		}

		if err := json.Unmarshal([]byte(unparsedOptions), &projectInfo.LatestBuild.Options); err != nil {
			return nil, fmt.Errorf("ListUserProjects unmarshal last build options: %w", err)
		}

		infos = append(infos, projectInfo)
	}

	return infos, nil
}

func (db *Database) GetProjectInfo(user string, project string) (ProjectInfo, error) {
	projectId, err := db.GetProjectId(user, project)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("GetProjectInfo: %w", err)
	}

	query := `
SELECT
  p.name,
  p.public,
  p.created_at,
  COALESCE(b.build_start, datetime(0, 'unixepoch')),
  COALESCE(b.build_time, 0),
  COALESCE(b.status, ''),
  COALESCE(b.options, '{}'),
  COALESCE(b.build_out, '')
FROM
  projects p
LEFT JOIN
  builds b
ON
  p.id = b.project_id
WHERE
  p.id = ?
ORDER BY
  b.id DESC
LIMIT 1
`
	row := db.conn.QueryRow(query, projectId)
	if row.Err() != nil {
		return ProjectInfo{}, fmt.Errorf("GetProjectInfo row query: %w", err)
	}

	var projectInfo ProjectInfo
	var unparsedOptions string
	var createdAt string
	var buildStart string
	if err := row.Scan(
		&projectInfo.Name,
		&projectInfo.Public,
		&createdAt,
		&buildStart,
		&projectInfo.LatestBuild.BuildTime,
		&projectInfo.LatestBuild.Status,
		&unparsedOptions,
		&projectInfo.LatestBuild.BuildOut,
	); err != nil {
		return ProjectInfo{}, fmt.Errorf("GetProjectInfo scan: %w", err)
	}

	projectInfo.CreatedAt, err = time.Parse(SQLiteTime, createdAt)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("GetProjectInfo parse createdAt time: %w", err)
	}

	projectInfo.LatestBuild.BuildStart, err = time.Parse(SQLiteTimeNano, buildStart)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("GetProjectInfo parse buildStart time: %w", err)
	}

	if err := json.Unmarshal([]byte(unparsedOptions), &projectInfo.LatestBuild.Options); err != nil {
		return ProjectInfo{}, fmt.Errorf("GetProjectInfo unmarshal last build options: %w", err)
	}

	return projectInfo, nil
}

// ListProjectFiles returns a list of files in the subdir of a project
// directory.
func (db *Database) ListProjectFiles(user string, projectName string, subdir string) ([]FileInfo, error) {
	projectId, err := db.GetProjectId(user, projectName)
	if err != nil {
		return nil, fmt.Errorf("ListProjectFiles: %w", err)
	}

	rows, err := db.conn.Query("SELECT path, size, sha256sum FROM files WHERE project_id = ? AND subdir = ?", projectId, subdir)
	if err != nil {
		return nil, fmt.Errorf("ListProjectFiles query: %w", err)
	}

	defer rows.Close()

	var fileInfo []FileInfo

	for rows.Next() {
		info := FileInfo{}
		if err := rows.Scan(&info.Path, &info.Size, &info.Sha256Sum); err != nil {
			return nil, fmt.Errorf("ListProjectFiles row scan: %w", err)
		}
		fileInfo = append(fileInfo, info)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("ListProjectFiles rows error: %w", rows.Err())
	}

	return fileInfo, nil
}


func (db *Database) GetProjectId(user string, project string) (int, error) {
	row := db.conn.QueryRow(
		"SELECT p.id FROM projects p JOIN users u ON p.user_id = u.id WHERE u.name = ? AND p.name = ?",
		user,
		project,
	)
	if row.Err() != nil {
		return 0, fmt.Errorf("Database.GetProjectId query: %w", row.Err())
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
