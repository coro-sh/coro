-- name: CreateOperator :exec
INSERT INTO operator (id, namespace_id, name, public_key, jwt, last_connect_time)
VALUES (?1, ?2, ?3, ?4, ?5, ?6);

-- name: UpdateOperator :exec
UPDATE operator
SET name              = ?2,
    jwt               = ?3,
    last_connect_time = ?4
WHERE id = ?1;

-- name: UpdateOperatorLastConnectTime :exec
UPDATE operator
SET last_connect_time = ?2
WHERE id = ?1;

-- name: ReadOperator :one
SELECT *
FROM operator
WHERE id = ?1;

-- name: ReadOperatorByPublicKey :one
SELECT *
FROM operator
WHERE public_key = ?1;

-- name: ReadOperatorByName :one
SELECT *
FROM operator
WHERE name = ?1;

-- name: ListOperators :many
SELECT *
FROM operator
WHERE namespace_id = ?1
  AND (@cursor IS NULL OR id <= @cursor)
ORDER BY id DESC
LIMIT @size;

-- name: DeleteOperator :exec
DELETE
FROM operator
WHERE id = ?1;
