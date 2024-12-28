# Scaling example

This folder contains a minimal setup to quickly explore Coro scaling capabilities with multiple **Controller** service
replicas and a **Broker** cluster behind Nginx load balancing.

## Steps

### 1. Spin up the environment

Spin up the environment with Docker Compose to start the following services:

- **Postgres** (database)
- **Coro Controller** service (2 replicas)
- **Coro Broker** cluster (2 nodes)
- **Nginx** for load balancing
- **NATS** managed by the example Operator

```shell
docker compose -p coro-scaling up -d
```

### 2. Run the Proxy

Create a Proxy authorization token for the example Operator initialized during setup.

```shell
NAMESPACE_ID=$(curl -sS "http://localhost:8080/controller-svc/api/v1/namespaces" | jq -r '.data[] | select(.name == "default") | .id')
echo "$NAMESPACE_ID"
OPERATOR_ID=$(curl -sS "http://localhost:8080/controller-svc/api/v1/namespaces/$NAMESPACE_ID/operators" | jq -r '.data[0].id')
echo "$OPERATOR_ID"
PROXY_TOKEN=$(curl -sS -X POST "http://localhost:8080/controller-svc/api/v1/namespaces/$NAMESPACE_ID/operators/$OPERATOR_ID/proxy/token"| jq -r '.data.token')
echo "$PROXY_TOKEN"
```

Build and run the Proxy to forward updates received from the Notifier cluster to the NATS server. Nginx will route the
Proxy WebSocket connection to one of the Notifier cluster nodes automatically.

```shell
docker build -t coro-proxy -f ../../cmd/proxy/Dockerfile ../..
docker run --rm --network host --name coro-scaling-proxy -d coro-proxy \
  --nats-url nats://localhost:4222 \
  --broker-url ws://localhost:8080/broker-svc/api/v1/broker \
  --token $PROXY_TOKEN
```

### 3. Validate the setup

Send a request to create a new Account.

```shell
curl -sS -X POST \
  "http://localhost:8080/controller-svc/api/v1/namespaces/$NAMESPACE_ID/operators/$OPERATOR_ID/accounts" \
  -H "Content-Type: application/json" \
  -d '{"name": "foo"}'
```

A successful response confirms that:

1. One of the Broker service replicas handled the API request.
2. The request triggered a notification in one of the Notifier cluster nodes.
3. The Proxy received and forwarded the notification to the NATS server.

### 4. Teardown environment

```shell
docker stop coro-scaling-proxy
docker compose -p coro-scaling down -v
```