-- name: CreateNamespace :exec
INSERT INTO namespace (id, name)
VALUES (?1, ?2);

-- name: ReadNamespaceByName :one
SELECT *
FROM namespace
WHERE name = ?1;

-- name: ReadNamespace :one
SELECT *
FROM namespace
WHERE id = ?1;

-- name: BatchReadNamespaces :many
SELECT *
FROM namespace
WHERE id IN (sqlc.slice('ids'));

-- name: ListNamespaces :many
SELECT *
FROM namespace
WHERE (@cursor IS NULL OR id <= @cursor)
  AND name != 'coro_internal'
ORDER BY id DESC
LIMIT @size;

-- name: DeleteNamespace :exec
DELETE
FROM namespace
WHERE id = ?1;
