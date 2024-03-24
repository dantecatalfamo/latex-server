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
