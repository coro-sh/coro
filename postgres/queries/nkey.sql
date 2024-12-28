-- name: CreateNkey :exec
INSERT INTO nkey (id, type, seed)
VALUES ($1, $2, $3);

-- name: ReadNkey :one
SELECT *
FROM nkey
WHERE id = $1;

-- name: CreateSigningKey :exec
INSERT INTO signing_key (id, type, seed)
VALUES ($1, $2, $3);

-- name: ReadSigningKey :one
SELECT *
FROM signing_key
WHERE id = $1;
