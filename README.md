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

Under the hood, Coro consists of two services:

- **Controller**: Issues and manages Operators, Accounts, and Users.
- **Broker**: Facilitates messaging between the Controller and connected Operator NATS servers.

Coro also provides a **Proxy Agent** to connect Operator NATS servers to the **Broker**.

## Getting started

### Demo

https://github.com/user-attachments/assets/63bdafb0-a45f-4494-a13f-699f7f46b14b

### Quickstart

The fastest way to get up and running is by using all-in-one Coro. Check out
the [quickstart example](examples/quickstart) to see how to run Coro with Docker Compose.

### Scaling

The quickstart guide runs an all-in-one Coro server using flag `-service=all`. For high-availability scenarios, you can
run the **Controller** and **Broker** services separately, allowing you
to scale them independently.

See the [scaling example](examples/scaling/) for a simple Docker and Nginx based setup.

## Disclaimer

Coro is under active development and may undergo significant changes. While it is available for exploration and testing,
it is not recommended for production use at this time. Features may be incomplete, and breaking changes may occur as the
platform is improved.
