-- name: CreateNamespace :exec
INSERT INTO namespace (id, name, owner)
VALUES ($1, $2, $3);

-- name: ReadNamespaceByName :one
SELECT *
FROM namespace
WHERE name = $1
  AND owner = $2;

-- name: ReadNamespace :one
SELECT *
FROM namespace
WHERE id = $1;

-- name: BatchReadNamespaces :many
SELECT *
FROM namespace
WHERE id = ANY (sqlc.arg('ids')::TEXT[]);

-- name: ListNamespaces :many
SELECT *
FROM namespace
WHERE (sqlc.narg('cursor')::TEXT IS NULL OR id <= sqlc.narg('cursor')::TEXT)
  AND owner = $1
ORDER BY id DESC
LIMIT @size;

-- name: DeleteNamespace :exec
SELECT delete_namespace_and_nkeys($1);

