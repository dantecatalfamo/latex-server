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
  created_at TEXT NOT NULL DEFAULT (datetime('now', 'utc')),

  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS projects_user_index ON projects(user_id);

CREATE TABLE IF NOT EXISTS builds (
  id INTEGER NOT NULL PRIMARY KEY,
  project_id INTEGER NOT NULL,
  build_start TEXT DEFAULT (datetime('now', 'utc', 'subsecond')),
  build_time  TEXT,
  status TEXT,
  options TEXT,

  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS builds_project_index ON builds(project_id);

CREATE TABLE IF NOT EXISTS files (
  id INTEGER NOT NULL PRIMARY KEY,
  project_id INTEGER NOT NULL,
  project_dir TEXT,
  file_path TEXT,
  file_hash TEXT,
  file_size INTEGER,

  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS files_projects_dir_hash_index ON files(project_id, project_dir, file_hash);

CREATE TABLE IF NOT EXISTS schema_info (
  version INTEGER
);

INSERT INTO schema_info (version) VALUES (1);
`,
}
