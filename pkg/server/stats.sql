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
