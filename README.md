<br>
<p align="center">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="img/logo-dark.png" width="280">
  <source media="(prefers-color-scheme: light)" srcset="img/logo-light.png" width="280">
  <img alt="Coro logo" src="img/logo-light.png" width="280">
</picture>
</p>
<br>

**Coro** is a platform that makes it simple to issue and manage Operators, Accounts, and Users
for [NATS](https://nats.io) servers.

It is distributed as a single binary but consists of modular services that can be deployed independently:

- **Controller**: Issues and manages Operators, Accounts, and Users.
- **Broker**: Facilitates messaging between the **Controller** and connected Operator NATS servers.
- **UI**: Web user interface for the **Controller**.

In addition to its core services, Coro provides the following tools:

- **`pgtool`**: A CLI tool for initializing and managing the Coro Postgres database.
- **`proxy-agent`**: A **Proxy Agent** that connects Operator NATS servers to the **Broker**.

## Getting started

### Demo

https://github.com/user-attachments/assets/63bdafb0-a45f-4494-a13f-699f7f46b14b

### Quickstart

The fastest way to get started is by running Coro in the all-in-one mode. Follow
the [quickstart example](examples/quickstart) to run Coro using Docker Compose.

### Scaling

The quickstart example runs an all-in-one Coro server using flag `-service=all`. For a high-availability setup, you can
run the **Controller**, **Broker**, and **UI** services separately, allowing you to scale them independently.

See the [scaling example](examples/scaling/) for a simple Docker and Nginx based setup.

### Configuration

Refer to the [configuration guide](docs/config.md) for a full list of configuration options.

## Disclaimer

Coro is under active development and may undergo significant changes. While it is available for exploration and testing,
it is not recommended for production use at this time. Features may be incomplete, and breaking changes may occur as the
platform is improved.
