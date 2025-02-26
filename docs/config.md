# Coro Configuration

## Overview

Coro provides a flexible and modular configuration system for running its various services. Configuration can be managed
using YAML files and environment variables.

This guide outlines the available configuration options and their purpose.

---

## Service Configuration

### Flag `--service all` (Controller, Broker, and UI)

| YAML Field               | Type   | Environment Variable       | Required | Default | Description                                                        |
|--------------------------|--------|----------------------------|----------|---------|--------------------------------------------------------------------|
| `port`                   | int    | `PORT`                     | true     | `5400`  | The port on which to run the services on.                          |
| `logger.level`           | string | `LOGGER_LEVEL`             | false    | `info`  | Logging level (`debug`, `info`, `warn`, `error`).                  |
| `logger.structured`      | bool   | `LOGGER_STRUCTURED`        | false    | `true`  | Enables structured logging.                                        |
| `tls.certFile`           | string | `TLS_CERT_FILE`            | false    |         | Path to the TLS certificate file.                                  |
| `tls.keyFile`            | string | `TLS_KEY_FILE`             | false    |         | Path to the TLS key file.                                          |
| `tls.caCertFile`         | string | `TLS_CA_CERT_FILE`         | false    |         | Path to the TLS CA certificate file.                               |
| `tls.insecureSkipVerify` | bool   | `TLS_INSECURE_SKIP_VERIFY` | false    | `false` | Allows skipping TLS verification (not recommended for production). |
| `encryptionSecretKey`    | string | `ENCRYPTION_SECRET_KEY`    | false    |         | AES key for encrypting sensitive data (16, 24, or 32 bytes).       |
| `postgres.hostPort`      | string | `POSTGRES_HOST_PORT`       | true     |         | PostgreSQL host and port in the form `host:port`.                  |
| `postgres.user`          | string | `POSTGRES_USER`            | true     |         | PostgreSQL username.                                               |
| `postgres.password`      | string | `POSTGRES_PASSWORD`        | true     |         | PostgreSQL password.                                               |

### Flag `--service backend` (Controller and Broker)

Same as above.

### Flag `--service controller`

| YAML Field               | Type   | Environment Variable       | Required   | Default | Description                                                        |
|--------------------------|--------|----------------------------|------------|---------|--------------------------------------------------------------------|
| `port`                   | int    | `PORT`                     | true       | `5400`  | The port on which the service runs.                                |
| `logger.level`           | string | `LOGGER_LEVEL`             | false      | `info`  | Logging level (`debug`, `info`, `warn`, `error`).                  |
| `logger.structured`      | bool   | `LOGGER_STRUCTURED`        | false      | `true`  | Enables structured logging.                                        |
| `tls.certFile`           | string | `TLS_CERT_FILE`            | false      |         | Path to the TLS certificate file.                                  |
| `tls.keyFile`            | string | `TLS_KEY_FILE`             | false      |         | Path to the TLS key file.                                          |
| `tls.caCertFile`         | string | `TLS_CA_CERT_FILE`         | false      |         | Path to the TLS CA certificate file.                               |
| `tls.insecureSkipVerify` | bool   | `TLS_INSECURE_SKIP_VERIFY` | false      | `false` | Allows skipping TLS verification (not recommended for production). |
| `encryptionSecretKey`    | string | `ENCRYPTION_SECRET_KEY`    | false      |         | AES key for encrypting sensitive data (16, 24, or 32 bytes).       |
| `postgres.hostPort`      | string | `POSTGRES_HOST_PORT`       | true       |         | PostgreSQL host and port in the form `host:port`.                  |
| `postgres.user`          | string | `POSTGRES_USER`            | true       |         | PostgreSQL username.                                               |
| `postgres.password`      | string | `POSTGRES_PASSWORD`        | true       |         | PostgreSQL password.                                               |
| `broker.natsURLs`        | array  | `BROKER_NATS_URLS`         | at least 1 |         | URLs of Broker cluster embedded NATS                               |
| `corsOrigins`            | array  | `CORS_ORIGINS`             | false      |         | Allowed CORS origins.                                              |

### Flag `--service broker`

| YAML Field                | Type   | Environment Variable        | Required | Default | Description                                                        |
|---------------------------|--------|-----------------------------|----------|---------|--------------------------------------------------------------------|
| `port`                    | int    | `PORT`                      | true     | `5400`  | The port on which the service runs.                                |
| `logger.level`            | string | `LOGGER_LEVEL`              | false    | `info`  | Logging level (`debug`, `info`, `warn`, `error`).                  |
| `logger.structured`       | bool   | `LOGGER_STRUCTURED`         | false    | `true`  | Enables structured logging.                                        |
| `tls.certFile`            | string | `TLS_CERT_FILE`             | false    |         | Path to the TLS certificate file.                                  |
| `tls.keyFile`             | string | `TLS_KEY_FILE`              | false    |         | Path to the TLS key file.                                          |
| `tls.caCertFile`          | string | `TLS_CA_CERT_FILE`          | false    |         | Path to the TLS CA certificate file.                               |
| `tls.insecureSkipVerify`  | bool   | `TLS_INSECURE_SKIP_VERIFY`  | false    | `false` | Allows skipping TLS verification (not recommended for production). |
| `encryptionSecretKey`     | string | `ENCRYPTION_SECRET_KEY`     | false    |         | AES key for encrypting sensitive data (16, 24, or 32 bytes).       |
| `postgres.hostPort`       | string | `POSTGRES_HOST_PORT`        | true     |         | PostgreSQL host and port in the form `host:port`.                  |
| `postgres.user`           | string | `POSTGRES_USER`             | true     |         | PostgreSQL username.                                               |
| `postgres.password`       | string | `POSTGRES_PASSWORD`         | true     |         | PostgreSQL password.                                               |
| `embeddedNats.hostPort`   | string | `EMBEDDED_NATS_HOST_PORT`   | true     |         | Port to run the embedded NATS server on.                           |
| `embeddedNats.nodeRoutes` | array  | `EMBEDDED_NATS_NODE_ROUTES` | false    |         | URLs of other Broker node's embedded NATS (to form a cluster).     |

### Flag `--service ui`

| YAML Field               | Type   | Environment Variable       | Required | Default                 | Description                                                        |
|--------------------------|--------|----------------------------|----------|-------------------------|--------------------------------------------------------------------|
| `port`                   | int    | `PORT`                     | true     | `8400`                  | The port on which to serve the UI.                                 |
| `apiAddress`             | string | `API_ADDRESS`              | true     | `http://localhost:5400` | Address of the Controller API.                                     |
| `logger.structured`      | bool   | `LOGGER_STRUCTURED`        | false    | `true`                  | Enables structured logging.                                        |
| `tls.certFile`           | string | `TLS_CERT_FILE`            | false    |                         | Path to the TLS certificate file.                                  |
| `tls.keyFile`            | string | `TLS_KEY_FILE`             | false    |                         | Path to the TLS key file.                                          |
| `tls.caCertFile`         | string | `TLS_CA_CERT_FILE`         | false    |                         | Path to the TLS CA certificate file.                               |
| `tls.insecureSkipVerify` | bool   | `TLS_INSECURE_SKIP_VERIFY` | false    | `false`                 | Allows skipping TLS verification (not recommended for production). |

---

## Configuration Methods

Coro configurations can be set using:

1. **YAML File**: Define configurations in a structured file.
2. **Environment Variables**: Override settings dynamically.

### Example Controller Configuration

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
  user: "coro"
  password: "securepassword"
broker:
  natsURLs:
    - "nats://localhost:5222"
```
