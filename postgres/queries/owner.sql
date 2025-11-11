-- name: CountOwnerNamespaces :one
SELECT COUNT(*)::BIGINT
FROM namespace AS n
WHERE n.owner = $1;

-- name: CountOwnerOperators :one
SELECT COUNT(*)::BIGINT
FROM operator o
         JOIN namespace n ON n.id = o.namespace_id
WHERE n.owner = $1;

-- name: CountOwnerAccounts :one
SELECT COUNT(*)::BIGINT
FROM account a
         JOIN namespace n ON n.id = a.namespace_id
WHERE n.owner = $1;

-- name: CountOwnerUsers :one
SELECT COUNT(*)::BIGINT
FROM "user" u
         JOIN namespace n ON n.id = u.namespace_id
WHERE n.owner = $1;