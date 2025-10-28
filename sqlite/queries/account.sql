-- name: CreateAccount :exec
INSERT INTO account (id, namespace_id, operator_id, name, public_key, jwt, user_jwt_duration)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7);

-- name: UpdateAccount :exec
UPDATE account
SET name              = ?2,
    jwt               = ?3,
    user_jwt_duration = ?4
WHERE id = ?1;

-- name: ReadAccount :one
SELECT *
FROM account
WHERE id = ?1;

-- name: ReadAccountByPublicKey :one
SELECT *
FROM account
WHERE public_key = ?1;

-- name: ListAccounts :many
SELECT *
FROM account
WHERE operator_id = ?1
  AND (@cursor IS NULL OR id <= @cursor)
  AND name != 'SYS'
ORDER BY id DESC
LIMIT @size;

-- name: DeleteAccount :exec
DELETE
FROM account
WHERE id = ?1;
