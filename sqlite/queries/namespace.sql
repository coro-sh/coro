-- name: CreateNamespace :exec
INSERT INTO namespace (id, name, owner)
VALUES (?1, ?2, ?3);

-- name: UpdateNamespace :exec
UPDATE namespace
SET name = ?2
WHERE id = ?1;

-- name: ReadNamespaceByName :one
SELECT *
FROM namespace
WHERE name = ?1
  AND owner = ?2;

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
  AND owner = ?1
ORDER BY id DESC
LIMIT @size;

-- name: DeleteNamespace :exec
DELETE
FROM namespace
WHERE id = ?1;
