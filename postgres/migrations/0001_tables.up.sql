CREATE TYPE nkey_type AS ENUM ('operator', 'account', 'user');

CREATE TABLE namespace
(
    id   TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE operator
(
    id           TEXT NOT NULL PRIMARY KEY,
    namespace_id TEXT NOT NULL REFERENCES namespace (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    public_key   TEXT NOT NULL UNIQUE,
    jwt          TEXT NOT NULL,
    UNIQUE (namespace_id, name)
);

CREATE TABLE account
(
    id                TEXT NOT NULL PRIMARY KEY,
    namespace_id      TEXT NOT NULL REFERENCES namespace (id) ON DELETE CASCADE,
    operator_id       TEXT NOT NULL REFERENCES operator (id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    public_key        TEXT NOT NULL UNIQUE,
    jwt               TEXT NOT NULL,
    user_jwt_duration INTERVAL,
    UNIQUE (operator_id, name)
);

CREATE TABLE "user"
(
    id           TEXT NOT NULL PRIMARY KEY,
    namespace_id TEXT NOT NULL REFERENCES namespace (id) ON DELETE CASCADE,
    operator_id  TEXT NOT NULL REFERENCES operator (id) ON DELETE CASCADE,
    account_id   TEXT NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    jwt          TEXT NOT NULL,
    jwt_duration INTERVAL,
    UNIQUE (operator_id, account_id, name)
);

CREATE TABLE user_jwt_issuances
(
    user_id     TEXT   NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    issue_time  BIGINT NOT NULL, -- unix
    expire_time BIGINT NOT NULL  -- unix
);

CREATE TABLE nkey
(
    id   TEXT      NOT NULL PRIMARY KEY,
    type nkey_type NOT NULL,
    seed BYTEA     NOT NULL
);

CREATE TABLE signing_key
(
    id   TEXT      NOT NULL PRIMARY KEY REFERENCES nkey (id) ON DELETE CASCADE,
    type nkey_type NOT NULL,
    seed BYTEA     NOT NULL
);

CREATE TABLE operator_token
(
    operator_id TEXT NOT NULL REFERENCES operator (id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    token       TEXT NOT NULL UNIQUE,
    PRIMARY KEY (operator_id, type)
);
