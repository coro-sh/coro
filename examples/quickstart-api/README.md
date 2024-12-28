# Quickstart API example

This folder contains a minimal setup to quickly explore Coro backend API. The setup runs the `backend` mode that
combines all backend services into a single process.

## Steps

### 1. Spin up the environment

Spin up the environment with Docker Compose to start the following services:

- **Postgres** initialized with `coro-pgtool`
- **Coro** (backend mode)
- **NATS** managed by the example Operator

```shell
docker compose -p coro up -d
```

### 2. Retrieve the 'default' Namespace ID

A 'default' Namespace is created in Coro on server startup.

We can retrieve the Namespace ID using the List Namespaces API.

```shell
NAMESPACE_ID=$(curl -sS "http://localhost:5400/api/v1/namespaces" | jq -r '.data[] | select(.name == "default") | .id')
echo "$NAMESPACE_ID"
```

### 3. Retrieve the example Operator ID

An example Operator will have been created in Coro and configured with the NATS server during setup.

We can retrieve the Operator ID using the List Operators API.

```shell
OPERATOR_ID=$(curl -sS "http://localhost:5400/api/v1/namespaces/$NAMESPACE_ID/operators" | jq -r '.data[] | select(.name == "example") | .id')
echo "$OPERATOR_ID"
```

### 4. Create a Proxy authorization token

```shell
PROXY_TOKEN=$(curl -sS -X POST "http://localhost:5400/api/v1/namespaces/$NAMESPACE_ID/operators/$OPERATOR_ID/proxy/token"| jq -r '.data.token')
echo "$PROXY_TOKEN"
```

### 5. Build and run the Proxy Agent

The Proxy Agent connects to the Coro Broker service via WebSocket and forwards any received notifications to the
Operator NATS server.

```shell
# Build image
docker build -t coro-proxy -f ../../cmd/proxy/Dockerfile ../..
# Run the container
docker run --rm --network host --name coro-proxy -d coro-proxy \
  --nats-url nats://localhost:4222 \
  --broker-url ws://localhost:5400/api/v1/broker \
  --token $PROXY_TOKEN
```

### 6. Create an Account

When you create or update an Account belonging to an Operator, Coro notifies the Operator's connected NATS server via
the Proxy Agent.

```shell
ACCOUNT_ID=$(curl -sS -X POST "http://localhost:5400/api/v1/namespaces/$NAMESPACE_ID/operators/$OPERATOR_ID/accounts" -H "Content-Type: application/json" -d '{"name": "foo"}' | jq -r '.data.id')
echo "$ACCOUNT_ID"
```

### 7. Create a User

Once we have an Account, we can create a User within it.

```shell
USER_ID=$(curl -sS -X POST "http://localhost:5400/api/v1/namespaces/$NAMESPACE_ID/accounts/$ACCOUNT_ID/users" -H "Content-Type: application/json" -d '{"name": "bar"}' | jq -r '.data.id')
echo "$USER_ID"
```

When connecting to a NATS server, Users authenticate using credentials (`.creds` files). Download the created user's
`.creds` file so that we can get it connected.

```shell
mkdir -p tmp/ && curl -sS "http://localhost:5400/api/v1/namespaces/$NAMESPACE_ID/users/$USER_ID/creds" -o tmp/user.creds
cat tmp/user.creds
```

### 8. Test the connection

You now have credentials (`user.creds`) that grant access to the NATS server.

In one terminal, subscribe using the credentials:

```shell
nats -s nats://localhost:4222 --creds ./tmp/user.creds sub test
```

In another terminal, publish a message to the test subject:

```shell
nats -s nats://localhost:4222 --creds ./tmp/user.creds pub test 'Hello, World!'
```

### 9. Teardown environment

```shell
docker stop coro-proxy
docker compose -p coro down -v
```