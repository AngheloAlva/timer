-- name: CreateTask :exec
INSERT INTO tasks (id, project_id, title, description, status, external_ref, created_at, updated_at)
VALUES (
    sqlc.arg(id),
    sqlc.arg(project_id),
    sqlc.arg(title),
    sqlc.arg(description),
    sqlc.arg(status),
    sqlc.arg(external_ref),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);

-- name: GetTaskByID :one
SELECT * FROM tasks WHERE id = ? LIMIT 1;

-- name: FindTasksByIDPrefix :many
SELECT * FROM tasks
WHERE id LIKE sqlc.arg(prefix)
LIMIT 10;

-- name: UpdateTaskStatus :exec
UPDATE tasks
SET status = sqlc.arg(status),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: ListTasks :many
SELECT t.id, t.project_id, t.title, t.description, t.status, t.external_ref, t.created_at, t.updated_at,
       p.name AS project_name,
       p.slug AS project_slug
FROM tasks t
JOIN projects p ON p.id = t.project_id
WHERE (sqlc.arg(include_done) = 1 OR t.status NOT IN ('done', 'archived'))
ORDER BY p.name COLLATE NOCASE ASC, t.created_at ASC;

-- name: ListTasksByProject :many
SELECT t.id, t.project_id, t.title, t.description, t.status, t.external_ref, t.created_at, t.updated_at,
       p.name AS project_name,
       p.slug AS project_slug
FROM tasks t
JOIN projects p ON p.id = t.project_id
WHERE t.project_id = sqlc.arg(project_id)
  AND (sqlc.arg(include_done) = 1 OR t.status NOT IN ('done', 'archived'))
ORDER BY t.created_at ASC;
