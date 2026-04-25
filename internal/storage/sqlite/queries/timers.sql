-- name: CreateTimer :exec
INSERT INTO timers (id, task_id, started_at, note, source, paused_at, paused_total_sec)
VALUES (
    sqlc.arg(id),
    sqlc.arg(task_id),
    sqlc.arg(started_at),
    sqlc.arg(note),
    sqlc.arg(source),
    NULL,
    0
);

-- name: GetTimerByID :one
SELECT * FROM timers WHERE id = ? LIMIT 1;

-- name: GetTimerByTaskID :one
SELECT * FROM timers WHERE task_id = ? LIMIT 1;

-- name: FindTimersByIDPrefix :many
SELECT * FROM timers
WHERE id LIKE sqlc.arg(prefix)
LIMIT 10;

-- name: ListActiveTimers :many
SELECT tm.id, tm.task_id, tm.started_at, tm.note, tm.source, tm.paused_at, tm.paused_total_sec,
       t.title       AS task_title,
       p.id          AS project_id,
       p.name        AS project_name,
       p.slug        AS project_slug
FROM timers tm
JOIN tasks    t ON t.id = tm.task_id
JOIN projects p ON p.id = t.project_id
ORDER BY tm.started_at ASC;

-- name: PauseTimer :exec
UPDATE timers SET paused_at = sqlc.arg(paused_at) WHERE id = sqlc.arg(id);

-- name: ResumeTimer :exec
UPDATE timers
SET paused_at = NULL,
    paused_total_sec = paused_total_sec + sqlc.arg(extra_paused_sec)
WHERE id = sqlc.arg(id);

-- name: DeleteTimer :exec
DELETE FROM timers WHERE id = ?;
