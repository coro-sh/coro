-- name: UpsertOperatorToken :exec
INSERT INTO operator_token (operator_id, type, token)
VALUES ($1, $2, $3)
ON CONFLICT (operator_id, type)
    DO UPDATE SET token = excluded.token;

-- name: ReadOperatorToken :one
SELECT operator_token.token
FROM operator_token
WHERE operator_id = $1
  AND type = $2;
