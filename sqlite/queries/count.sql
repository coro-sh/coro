-- name: CountOwnerNamespaces :one
SELECT COUNT(*) AS count
FROM namespace AS n
WHERE n.owner = ?;

-- name: CountOwnerOperators :one
SELECT COUNT(*) AS count
FROM operator AS o
         JOIN namespace AS n ON n.id = o.namespace_id
WHERE n.owner = ?;

-- name: CountNamespaceOperators :one
SELECT COUNT(*) AS count
FROM operator AS o
WHERE o.namespace_id = ?;

-- name: CountOperatorAccounts :one
SELECT COUNT(*) AS count
FROM account AS a
WHERE a.operator_id = ? AND a.name != 'SYS';

-- name: CountOperatorUsers :one
SELECT COUNT(*) AS count
FROM "user" AS u
WHERE u.operator_id = ? AND u.name != 'sys';
