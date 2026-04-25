-- name: CreateTimeEntry :exec
INSERT INTO time_entries (id, task_id, started_at, ended_at, duration_sec, note, source, created_at)
VALUES (
    sqlc.arg(id),
    sqlc.arg(task_id),
    sqlc.arg(started_at),
    sqlc.arg(ended_at),
    sqlc.arg(duration_sec),
    sqlc.arg(note),
    sqlc.arg(source),
    sqlc.arg(created_at)
);

-- name: ListTimeEntries :many
SELECT te.id, te.task_id, te.started_at, te.ended_at, te.duration_sec, te.note, te.source, te.created_at,
       t.title       AS task_title,
       p.id          AS project_id,
       p.name        AS project_name,
       p.slug        AS project_slug
FROM time_entries te
JOIN tasks    t ON t.id = te.task_id
JOIN projects p ON p.id = t.project_id
WHERE te.started_at >= sqlc.arg(min_started_at)
  AND (sqlc.arg(project_id) = '' OR p.id = sqlc.arg(project_id))
ORDER BY te.started_at DESC
LIMIT sqlc.arg(max_rows);
