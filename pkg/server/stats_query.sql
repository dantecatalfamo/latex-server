-- Per-user, per-project stats
WITH user_projects(user_id, user_name, project_id, project_name) AS (
  SELECT
    u.id,
    u.name,
    p.id,
    p.name
  FROM
    users u
  LEFT JOIN
    projects p
    ON p.user_id = u.id
),
user_projects_builds(project_id, total_builds, total_build_time) AS (
  SELECT
    up.project_id,
    COUNT(*),
    SUM(b.build_time)
  FROM
    user_projects up
  LEFT JOIN
    builds b
    ON b.project_id = up.project_id
  GROUP BY
    up.project_id
),
user_projects_files(project_id, total_files, total_file_size) AS (
  SELECT
    up.project_id,
    COUNT(*),
    SUM(f.size)
  FROM
    user_projects up
  LEFT JOIN
    files f
    ON f.project_id = up.project_id
  GROUP BY
    up.project_id
)
SELECT
  up.user_id,
  up.user_name,
  up.project_id,
  up.project_name,
  upb.total_builds,
  upb.total_build_time,
  upf.total_files,
  upf.total_file_size
FROM
  user_projects up
LEFT JOIN
  user_projects_builds upb
  ON upb.project_id = up.project_id
LEFT JOIN
  user_projects_files upf
  ON upf.project_id = up.project_id


-- Per-user, per-project build stats
SELECT
  u.name user_name,
  p.name project_name,
  COUNT(*) builds,
  SUM(b.build_time) build_time
FROM
  builds b
JOIN
  projects p
  ON b.project_id = p.id
JOIN
  users u
  ON p.user_id = u.id
GROUP BY
  p.id

-- Per-user, per-project file stats
SELECT
  u.id user_id,
  u.name user_name,
  p.name project_name,
  COUNT(*) files,
  SUM(f.size) file_size
FROM
  files f
JOIN
  projects p
  ON f.project_id = p.id
JOIN
  users u
  ON p.user_id = u.id
GROUP BY u.id, p.id

-- Per-user project stats
SELECT
  u.id user_id,
  u.name user_name,
  COUNT(DISTINCT p.id) AS projects,
  COUNT(DISTINCT b.id) AS builds,
  COUNT(DISTINCT f.id) AS files
FROM
  users u
LEFT JOIN
  projects p
  ON p.user_id = u.id
LEFT JOIN
  builds b
  ON b.project_id = p.id
LEFT JOIN
  files f
  ON f.project_id = p.id
GROUP BY u.id
