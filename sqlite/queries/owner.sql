-- name: CountOwnerNamespaces :one
SELECT COUNT(*) AS count
FROM namespace AS n
WHERE n.owner = ?;

-- name: CountOwnerOperators :one
SELECT COUNT(*) AS count
FROM operator AS o
         JOIN namespace AS n ON n.id = o.namespace_id
WHERE n.owner = ?;

-- name: CountOwnerAccounts :one
SELECT COUNT(*) AS count
FROM account AS a
         JOIN namespace AS n ON n.id = a.namespace_id
WHERE n.owner = ? AND a.name != 'SYS';

-- name: CountOwnerUsers :one
SELECT COUNT(*) AS count
FROM "user" AS u
         JOIN namespace AS n ON n.id = u.namespace_id
WHERE n.owner = ? AND u.name != 'sys';
