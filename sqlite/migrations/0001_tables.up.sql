CREATE TABLE namespace
(
    id   TEXT PRIMARY KEY NOT NULL,
    name TEXT             NOT NULL UNIQUE
);

CREATE TABLE operator
(
    id           TEXT PRIMARY KEY NOT NULL,
    namespace_id TEXT             NOT NULL,
    name         TEXT             NOT NULL,
    public_key   TEXT             NOT NULL UNIQUE,
    jwt          TEXT             NOT NULL,
    UNIQUE (namespace_id, name),
    FOREIGN KEY (namespace_id) REFERENCES namespace (id) ON DELETE CASCADE
);

CREATE TABLE account
(
    id                TEXT PRIMARY KEY NOT NULL,
    namespace_id      TEXT             NOT NULL,
    operator_id       TEXT             NOT NULL,
    name              TEXT             NOT NULL,
    public_key        TEXT             NOT NULL UNIQUE,
    jwt               TEXT             NOT NULL,
    user_jwt_duration INTEGER, -- seconds
    UNIQUE (operator_id, name),
    FOREIGN KEY (namespace_id) REFERENCES namespace (id) ON DELETE CASCADE,
    FOREIGN KEY (operator_id) REFERENCES operator (id) ON DELETE CASCADE
);

CREATE TABLE user
(
    id           TEXT PRIMARY KEY NOT NULL,
    namespace_id TEXT             NOT NULL,
    operator_id  TEXT             NOT NULL,
    account_id   TEXT             NOT NULL,
    name         TEXT             NOT NULL,
    jwt          TEXT             NOT NULL,
    jwt_duration INTEGER, -- seconds
    UNIQUE (operator_id, account_id, name),
    FOREIGN KEY (namespace_id) REFERENCES namespace (id) ON DELETE CASCADE,
    FOREIGN KEY (operator_id) REFERENCES operator (id) ON DELETE CASCADE,
    FOREIGN KEY (account_id) REFERENCES account (id) ON DELETE CASCADE
);

CREATE TABLE user_jwt_issuances
(
    user_id     TEXT    NOT NULL,
    issue_time  INTEGER NOT NULL, -- unix epoch seconds
    expire_time INTEGER NOT NULL, -- unix epoch seconds
    FOREIGN KEY (user_id) REFERENCES "user" (id) ON DELETE CASCADE
);

CREATE TABLE nkey
(
    id   TEXT PRIMARY KEY NOT NULL,
    type TEXT             NOT NULL CHECK (type IN ('operator', 'account', 'user')),
    seed BLOB             NOT NULL
);

CREATE TABLE signing_key
(
    id   TEXT PRIMARY KEY NOT NULL,
    type TEXT             NOT NULL CHECK (type IN ('operator', 'account', 'user')),
    seed BLOB             NOT NULL,
    FOREIGN KEY (id) REFERENCES nkey (id) ON DELETE CASCADE
);

CREATE TABLE operator_token
(
    operator_id TEXT NOT NULL,
    type        TEXT NOT NULL,
    token       TEXT NOT NULL UNIQUE,
    PRIMARY KEY (operator_id, type),
    FOREIGN KEY (operator_id) REFERENCES operator (id) ON DELETE CASCADE
);