-- name: CreateNamespace :exec
INSERT INTO namespace (id, name)
VALUES ($1, $2);

-- name: ReadNamespaceByName :one
SELECT *
FROM namespace
WHERE name = $1;

-- name: ReadNamespace :one
SELECT *
FROM namespace
WHERE id = $1;

-- name: ListNamespaces :many
SELECT *
FROM namespace
WHERE (sqlc.narg('cursor')::TEXT IS NULL OR id <= sqlc.narg('cursor')::TEXT)
  AND name != 'coro_internal'
ORDER BY id DESC
LIMIT @size;

-- name: DeleteNamespace :exec
SELECT delete_namespace_and_nkeys($1);
