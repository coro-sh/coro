-- name: CreateUser :exec
INSERT INTO user (id, namespace_id, operator_id, account_id, name, jwt, jwt_duration)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7);

-- name: UpdateUser :exec
UPDATE user
SET name         = ?2,
    jwt          = ?3,
    jwt_duration = ?4
WHERE id = ?1;

-- name: ReadUser :one
SELECT *
FROM user
WHERE id = ?1;

-- name: ReadUserByName :one
SELECT *
FROM user
WHERE operator_id = ?1
  AND account_id = ?2
  AND name = ?3;

-- name: ListUsers :many
SELECT *
FROM user
WHERE account_id = ?1
  AND (@cursor IS NULL OR id <= @cursor)
ORDER BY id DESC
LIMIT @size;

-- name: DeleteUser :exec
DELETE
FROM user
WHERE id = ?1;

-- name: CreateUserJWTIssuance :exec
INSERT INTO user_jwt_issuances (user_id, issue_time, expire_time)
VALUES (?1, ?2, ?3);

-- name: ListUserJWTIssuances :many
SELECT *
FROM user_jwt_issuances
WHERE user_id = ?1
  AND (@cursor IS NULL OR issue_time <= @cursor)
ORDER BY issue_time DESC
LIMIT @size;