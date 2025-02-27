# Scaling example

This folder provides a minimal setup to explore Coro's scaling capabilities. It demonstrates how to run multiple
**Controller** service replicas and a **Broker** cluster behind an **Nginx** load balancer.

Each **Broker** node includes an embedded NATS server, which enables communication between nodes to form a cluster.

## Steps

1. Spin up the environment with Docker Compose to start the following services:

    - **Postgres** (database)
    - **Coro Controller** (2 replicas)
    - **Coro Broker** cluster (2 nodes)
    - **Coro UI**
    - **Nginx** (load balancer)

    ```shell
    docker compose -p coro-scaling up -d
    ```
2. Open http://localhost:8400 in your browser.
3. Create a new Operator and open it.
4. Head to the `NATS` tab and follow the instructions on how to set up a NATS server and Coro Proxy Agent.
    - Use the following flags when running the Proxy Agent:
        - `--token <PROXY_TOKEN>`
        - `--nats-url nats://host.docker.internal:4222`
        - `--broker-url ws://host.docker.internal:8080/broker-svc/api/v1/broker`
    - Nginx will take care of routing the Proxy Agent's WebSocket connection to one of the Broker
      cluster nodes.
5. Once your NATS server is connected, create a new Account.
6. A successful Account creation confirms that:
    - One of the Controller service replicas handled the API request.
    - The request triggered a notification in one of the Broker cluster nodes.
    - The Proxy Agent received and forwarded the notification to the connected NATS server.
7. Teardown environment.
   ```shell
   docker compose -p coro-scaling down -v
   ```
