package main

var migrations = []string{
`
CREATE TABLE IF NOT EXISTS users (
  id INTEGER NOT NULL PRIMARY KEY,
  name TEXT
);

CREATE TABLE IF NOT EXISTS projects (
  id INTEGER NOT NULL PRIMARY KEY,
  name TEXT,
  user_id INTEGER NOT NULL,
  public INTEGER DEFAULT FALSE,
  created_at TEXT NOT NULL DEFAULT (datetime('now', 'utc')),

  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS projects_user_name_index ON projects(user_id, name);

CREATE TABLE IF NOT EXISTS builds (
  id INTEGER NOT NULL PRIMARY KEY,
  project_id INTEGER NOT NULL,
  build_start TEXT DEFAULT (datetime('now', 'utc', 'subsecond')),
  build_time REAL,
  status TEXT,
  options TEXT,

  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS builds_project_index ON builds(project_id);

CREATE TABLE IF NOT EXISTS files (
  id INTEGER NOT NULL PRIMARY KEY,
  project_id INTEGER NOT NULL,
  subdir TEXT,
  path TEXT,
  sha512hash TEXT,
  size INTEGER,

  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS files_projects_subdir_index ON files(project_id, subdir);

CREATE TABLE IF NOT EXISTS schema_migration (
  version INTEGER
);

INSERT INTO schema_migration (version) VALUES (1);
`,
}
