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
