# Coro Configuration

## Overview

Coro provides a flexible and modular configuration system for running its various services. Configuration can be managed
using YAML files and environment variables.

This guide outlines the available configuration options and their purpose.

---

## Configuration Methods

Coro configurations can be set using:

1. **YAML File**: Define configurations in a structured file (passed via `--config` flag)
2. **Environment Variables**: Override settings dynamically using the environment variable names listed below

---

## Service Modes

Coro can be run in different service modes using the `--service` flag:

- `--service all`: Runs Controller, Broker, and UI in a single process (all-in-one mode)
- `--service backend`: Runs Controller and Broker together
- `--service controller`: Runs only the Controller service
- `--service broker`: Runs only the Broker service
- `--service ui`: Runs only the UI service

---

## Configuration Options by Service

### `--service all` (All-in-One Mode)

Runs Controller, Broker, and UI together using an in-memory SQLite database (dev/testing only).

| YAML Field                        | Type     | Environment Variable                | Required | Default | Description                                                    |
|-----------------------------------|----------|-------------------------------------|----------|---------|----------------------------------------------------------------|
| `port`                            | int      | `PORT`                              | false    | `5400`  | Port to run the server on                                      |
| `logger.level`                    | string   | `LOGGER_LEVEL`                      | false    | `info`  | Logging level: `debug`, `info`, `warn`, `error`                |
| `logger.structured`               | bool     | `LOGGER_STRUCTURED`                 | false    | `true`  | Enable structured JSON logging                                 |
| `tls.certFile`                    | string   | `TLS_CERT_FILE`                     | false    |         | Path to TLS certificate file                                   |
| `tls.keyFile`                     | string   | `TLS_KEY_FILE`                      | false    |         | Path to TLS private key file                                   |
| `tls.caCertFile`                  | string   | `TLS_CA_CERT_FILE`                  | false    |         | Path to TLS CA certificate file                                |
| `tls.insecureSkipVerify`          | bool     | `TLS_INSECURE_SKIP_VERIFY`          | false    | `false` | Skip TLS verification (not recommended for production)         |
| `encryptionSecretKey`             | string   | `ENCRYPTION_SECRET_KEY`             | false    |         | Hex-encoded AES key (16, 24, or 32 bytes) for encrypting nkeys |
| `postgres.hostPort`               | string   | `POSTGRES_HOST_PORT`                | true     |         | PostgreSQL host and port (`host:port`)                         |
| `postgres.database`               | string   | `POSTGRES_DATABASE`                 | false    | `coro`  | PostgreSQL database name                                       |
| `postgres.user`                   | string   | `POSTGRES_USER`                     | true     |         | PostgreSQL username                                            |
| `postgres.password`               | string   | `POSTGRES_PASSWORD`                 | false    |         | PostgreSQL password                                            |
| `postgres.tls.certFile`           | string   | `POSTGRES_TLS_CERT_FILE`            | false    |         | Path to PostgreSQL client TLS certificate                      |
| `postgres.tls.keyFile`            | string   | `POSTGRES_TLS_KEY_FILE`             | false    |         | Path to PostgreSQL client TLS key                              |
| `postgres.tls.caCertFile`         | string   | `POSTGRES_TLS_CA_CERT_FILE`         | false    |         | Path to PostgreSQL CA certificate                              |
| `postgres.tls.insecureSkipVerify` | bool     | `POSTGRES_TLS_INSECURE_SKIP_VERIFY` | false    | `false` | Skip PostgreSQL TLS verification                               |
| `corsOrigins`                     | []string | `CORS_ORIGINS`                      | false    |         | Allowed CORS origins (comma-separated in env)                  |

**Example:**

```yaml
port: 5400
logger:
  level: info
  structured: true
encryptionSecretKey: "00deaa689d7b85e4a68d416678e206cb"
postgres:
  hostPort: "localhost:5432"
  database: "coro"
  user: "coro"
  password: "securepassword"
corsOrigins:
  - "http://localhost:8080"
  - "http://localhost:5173"
```

---

### `--service controller`

Runs the Controller service which manages Operators, Accounts, and Users.

| YAML Field                        | Type     | Environment Variable                | Required    | Default | Description                                                           |
|-----------------------------------|----------|-------------------------------------|-------------|---------|-----------------------------------------------------------------------|
| `port`                            | int      | `PORT`                              | false       | `5400`  | Port to run the Controller on                                         |
| `logger.level`                    | string   | `LOGGER_LEVEL`                      | false       | `info`  | Logging level: `debug`, `info`, `warn`, `error`                       |
| `logger.structured`               | bool     | `LOGGER_STRUCTURED`                 | false       | `true`  | Enable structured JSON logging                                        |
| `tls.certFile`                    | string   | `TLS_CERT_FILE`                     | false       |         | Path to TLS certificate file                                          |
| `tls.keyFile`                     | string   | `TLS_KEY_FILE`                      | false       |         | Path to TLS private key file                                          |
| `tls.caCertFile`                  | string   | `TLS_CA_CERT_FILE`                  | false       |         | Path to TLS CA certificate file                                       |
| `tls.insecureSkipVerify`          | bool     | `TLS_INSECURE_SKIP_VERIFY`          | false       | `false` | Skip TLS verification (not recommended for production)                |
| `encryptionSecretKey`             | string   | `ENCRYPTION_SECRET_KEY`             | false       |         | Hex-encoded AES key (16, 24, or 32 bytes) for encrypting nkeys        |
| `postgres.hostPort`               | string   | `POSTGRES_HOST_PORT`                | true        |         | PostgreSQL host and port (`host:port`)                                |
| `postgres.database`               | string   | `POSTGRES_DATABASE`                 | false       | `coro`  | PostgreSQL database name                                              |
| `postgres.user`                   | string   | `POSTGRES_USER`                     | true        |         | PostgreSQL username                                                   |
| `postgres.password`               | string   | `POSTGRES_PASSWORD`                 | false       |         | PostgreSQL password                                                   |
| `postgres.tls.certFile`           | string   | `POSTGRES_TLS_CERT_FILE`            | false       |         | Path to PostgreSQL client TLS certificate                             |
| `postgres.tls.keyFile`            | string   | `POSTGRES_TLS_KEY_FILE`             | false       |         | Path to PostgreSQL client TLS key                                     |
| `postgres.tls.caCertFile`         | string   | `POSTGRES_TLS_CA_CERT_FILE`         | false       |         | Path to PostgreSQL CA certificate                                     |
| `postgres.tls.insecureSkipVerify` | bool     | `POSTGRES_TLS_INSECURE_SKIP_VERIFY` | false       | `false` | Skip PostgreSQL TLS verification                                      |
| `corsOrigins`                     | []string | `CORS_ORIGINS`                      | false       |         | Allowed CORS origins (comma-separated in env)                         |
| `broker.natsURLs`                 | []string | `BROKER_NATS_URLS`                  | conditional |         | Broker embedded NATS URLs (required if connecting to external Broker) |

**Note:** The `broker` section is only required when the Controller needs to communicate with a separate Broker service.
When running in all-in-one mode, this is not needed.

**Example:**

```yaml
port: 5400
logger:
  level: info
  structured: true
encryptionSecretKey: "00deaa689d7b85e4a68d416678e206cb"
tls:
  certFile: "/etc/coro/tls/cert.pem"
  keyFile: "/etc/coro/tls/key.pem"
  caCertFile: "/etc/coro/tls/ca.pem"
postgres:
  hostPort: "db.example.com:5432"
  database: "coro"
  user: "coro"
  password: "securepassword"
  tls:
    caCertFile: "/etc/coro/tls/postgres-ca.pem"
broker:
  natsURLs:
    - "nats://broker1.example.com:4222"
    - "nats://broker2.example.com:4222"
corsOrigins:
  - "https://app.example.com"
```

---

### `--service broker`

Runs the Broker service which manages WebSocket connections from proxy agents and routes commands.

| YAML Field                        | Type     | Environment Variable                | Required | Default | Description                                                                                     |
|-----------------------------------|----------|-------------------------------------|----------|---------|-------------------------------------------------------------------------------------------------|
| `port`                            | int      | `PORT`                              | false    | `5400`  | Port to run the Broker on                                                                       |
| `logger.level`                    | string   | `LOGGER_LEVEL`                      | false    | `info`  | Logging level: `debug`, `info`, `warn`, `error`                                                 |
| `logger.structured`               | bool     | `LOGGER_STRUCTURED`                 | false    | `true`  | Enable structured JSON logging                                                                  |
| `tls.certFile`                    | string   | `TLS_CERT_FILE`                     | false    |         | Path to TLS certificate file                                                                    |
| `tls.keyFile`                     | string   | `TLS_KEY_FILE`                      | false    |         | Path to TLS private key file                                                                    |
| `tls.caCertFile`                  | string   | `TLS_CA_CERT_FILE`                  | false    |         | Path to TLS CA certificate file                                                                 |
| `tls.insecureSkipVerify`          | bool     | `TLS_INSECURE_SKIP_VERIFY`          | false    | `false` | Skip TLS verification (not recommended for production)                                          |
| `encryptionSecretKey`             | string   | `ENCRYPTION_SECRET_KEY`             | false    |         | Hex-encoded AES key (16, 24, or 32 bytes) for encrypting tokens                                 |
| `postgres.hostPort`               | string   | `POSTGRES_HOST_PORT`                | true     |         | PostgreSQL host and port (`host:port`)                                                          |
| `postgres.database`               | string   | `POSTGRES_DATABASE`                 | false    | `coro`  | PostgreSQL database name (overriding default value is discouraged for compatibility with pgtool |
| `postgres.user`                   | string   | `POSTGRES_USER`                     | true     |         | PostgreSQL username                                                                             |
| `postgres.password`               | string   | `POSTGRES_PASSWORD`                 | false    |         | PostgreSQL password                                                                             |
| `postgres.tls.certFile`           | string   | `POSTGRES_TLS_CERT_FILE`            | false    |         | Path to PostgreSQL client TLS certificate                                                       |
| `postgres.tls.keyFile`            | string   | `POSTGRES_TLS_KEY_FILE`             | false    |         | Path to PostgreSQL client TLS key                                                               |
| `postgres.tls.caCertFile`         | string   | `POSTGRES_TLS_CA_CERT_FILE`         | false    |         | Path to PostgreSQL CA certificate                                                               |
| `postgres.tls.insecureSkipVerify` | bool     | `POSTGRES_TLS_INSECURE_SKIP_VERIFY` | false    | `false` | Skip PostgreSQL TLS verification                                                                |
| `embeddedNats.hostPort`           | string   | `EMBEDDED_NATS_HOST_PORT`           | true     |         | Host and port for embedded NATS server (`host:port`)                                            |
| `embeddedNats.nodeRoutes`         | []string | `EMBEDDED_NATS_NODE_ROUTES`         | false    |         | Routes to other Broker nodes for clustering (NATS URLs)                                         |

**Example (Single Broker):**

```yaml
port: 5400
logger:
  level: info
  structured: true
encryptionSecretKey: "00deaa689d7b85e4a68d416678e206cb"
postgres:
  hostPort: "db.example.com:5432"
  database: "coro"
  user: "coro"
  password: "securepassword"
embeddedNats:
  hostPort: "0.0.0.0:4222"
```

**Example (Clustered Brokers):**

```yaml
port: 5400
logger:
  level: info
  structured: true
encryptionSecretKey: "00deaa689d7b85e4a68d416678e206cb"
postgres:
  hostPort: "db.example.com:5432"
  database: "coro"
  user: "coro"
  password: "securepassword"
embeddedNats:
  hostPort: "0.0.0.0:4222"
  nodeRoutes:
    - "nats://broker2.example.com:4222"
    - "nats://broker3.example.com:4222"
```

---

### `--service ui`

Runs the UI service which serves the web interface and proxies API requests to the Controller.

| YAML Field               | Type   | Environment Variable       | Required | Default                 | Description                                                      |
|--------------------------|--------|----------------------------|----------|-------------------------|------------------------------------------------------------------|
| `port`                   | int    | `PORT`                     | false    | `8400`                  | Port to run the UI on                                            |
| `logger.level`           | string | `LOGGER_LEVEL`             | false    | `info`                  | Logging level: `debug`, `info`, `warn`, `error`                  |
| `logger.structured`      | bool   | `LOGGER_STRUCTURED`        | false    | `true`                  | Enable structured JSON logging                                   |
| `apiAddress`             | string | `API_ADDRESS`              | false    | `http://localhost:5400` | Controller API address to proxy requests to                      |
| `tls.certFile`           | string | `TLS_CERT_FILE`            | false    |                         | Path to TLS certificate file                                     |
| `tls.keyFile`            | string | `TLS_KEY_FILE`             | false    |                         | Path to TLS private key file                                     |
| `tls.caCertFile`         | string | `TLS_CA_CERT_FILE`         | false    |                         | Path to TLS CA certificate file (for proxying to Controller API) |
| `tls.insecureSkipVerify` | bool   | `TLS_INSECURE_SKIP_VERIFY` | false    | `false`                 | Skip TLS verification when proxying to Controller                |

**Example:**

```yaml
port: 8400
logger:
  level: info
  structured: true
apiAddress: "https://api.example.com"
tls:
  certFile: "/etc/coro/tls/cert.pem"
  keyFile: "/etc/coro/tls/key.pem"
  caCertFile: "/etc/coro/tls/ca.pem"
```

---

## Configuration Notes

### Encryption Secret Key

The `encryptionSecretKey` is used to encrypt sensitive data like nkeys and proxy tokens in the database. It must be a
hex-encoded string that decodes to 16, 24, or 32 bytes for AES-128, AES-192, or AES-256 respectively.

Generate a key using OpenSSL:

```bash
# AES-128 (16 bytes)
openssl rand -hex 16

# AES-192 (24 bytes)
openssl rand -hex 24

# AES-256 (32 bytes)
openssl rand -hex 32
```

**Important:** Keep this key secure and consistent across all service instances. Changing the key will make existing
encrypted data unreadable.

### TLS Configuration

TLS can be configured at multiple levels:

1. **Service TLS** (`tls.*`): Enables HTTPS for the service endpoint
2. **PostgreSQL TLS** (`postgres.tls.*`): Secures connection to PostgreSQL
3. **Controller TLS for Broker communication** (`tls.*` in Controller): Secures Controller → Broker NATS connections

For production deployments, TLS is strongly recommended for all connections.

### CORS Origins

When running the Controller with a separate UI or allowing web applications to access the API, configure `corsOrigins`
to specify allowed origins.

Environment variable format (comma-separated):

```bash
export CORS_ORIGINS="https://app.example.com,https://staging.example.com"
```

### Embedded NATS Clustering

When running multiple Broker instances for high availability, configure `embeddedNats.nodeRoutes` to list the NATS URLs
of other brokers. Each broker will form a cluster and share WebSocket connections and messages.

---

## Environment Variable Override

All configuration values can be overridden using environment variables. The variable names follow the pattern shown in
the tables above with prefixes based on the nesting level.

**Example:**

```bash
export PORT=5400
export LOGGER_LEVEL=debug
export LOGGER_STRUCTURED=true
export POSTGRES_HOST_PORT=localhost:5432
export POSTGRES_DATABASE=coro
export POSTGRES_USER=coro
export POSTGRES_PASSWORD=secret
export ENCRYPTION_SECRET_KEY=00deaa689d7b85e4a68d416678e206cb
```

For array values (like `corsOrigins` or `broker.natsURLs`), use comma-separated values:

```bash
export CORS_ORIGINS="http://localhost:8080,http://localhost:5173"
export BROKER_NATS_URLS="nats://broker1:4222,nats://broker2:4222"
```
