-- name: CreateProject :exec
INSERT INTO projects (id, name, slug, color, archived, created_at, updated_at)
VALUES (
    sqlc.arg(id),
    sqlc.arg(name),
    sqlc.arg(slug),
    sqlc.arg(color),
    sqlc.arg(archived),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);

-- name: ListProjects :many
SELECT * FROM projects
WHERE archived = 0 OR sqlc.arg(include_archived) = 1
ORDER BY name COLLATE NOCASE ASC;

-- name: GetProjectBySlug :one
SELECT * FROM projects WHERE slug = ? LIMIT 1;

-- name: CountProjects :one
SELECT COUNT(*) AS total FROM projects;

-- name: ArchiveProject :exec
UPDATE projects
SET archived = 1,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = ?;

-- name: CountTasksByProject :one
SELECT COUNT(*) AS total FROM tasks WHERE project_id = ?;

-- name: CountTimeEntriesByProject :one
SELECT COUNT(*) AS total
FROM time_entries te
JOIN tasks t ON t.id = te.task_id
WHERE t.project_id = ?;

-- name: ListActiveTimersByProject :many
SELECT tm.id, tm.task_id, tm.started_at, tm.note, tm.source,
       tm.paused_at, tm.paused_total_sec,
       t.title AS task_title
FROM timers tm
JOIN tasks t ON t.id = tm.task_id
WHERE t.project_id = sqlc.arg(project_id)
ORDER BY tm.started_at ASC;
