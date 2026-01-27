-- name: CountOwnerNamespaces :one
SELECT COUNT(*)::BIGINT
FROM namespace AS n
WHERE n.owner = $1;

-- name: CountOwnerOperators :one
SELECT COUNT(*)::BIGINT
FROM operator o
         JOIN namespace n ON n.id = o.namespace_id
WHERE n.owner = $1;

-- name: CountNamespaceOperators :one
SELECT COUNT(*)::BIGINT
FROM operator o
WHERE o.namespace_id = $1;

-- name: CountOperatorAccounts :one
SELECT COUNT(*)::BIGINT
FROM account a
WHERE a.operator_id = $1
  AND a.name != 'SYS';

-- name: CountOperatorUsers :one
SELECT COUNT(*)::BIGINT
FROM "user" u
WHERE u.operator_id = $1
  AND u.name != 'sys';
