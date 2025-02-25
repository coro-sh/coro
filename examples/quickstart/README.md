# Quickstart example

This folder contains a minimal setup to quickly explore Coro. 
The setup runs the all-in-one mode that combines all
services into a single process.

## Steps

1. Spin up the environment with Docker Compose to start the following services:

    - **Postgres** initialized with `coro-pgtool`
    - **Coro** (all-in-one mode)

    ```shell
    docker compose -p coro up -d
    ```

2. Open http://localhost:5400 in your browser.
3. Create a new Operator and open it.
4. Head to the `NATS` tab and follow the instructions on how to set up a NATS server and Coro Proxy Agent.
   - Use the following flags when running the Proxy Agent:
      - `--token <PROXY_TOKEN>`
      - `--nats-url nats://host.docker.internal:4222`
      - `--broker-url ws://host.docker.internal:5400/api/v1/broker`
5. Once your NATS server is connected, create a new Account and User.
6. Open the User, head to the `Connect` tab, and download the User's credentials file
7. Connect to the NATS server using the credentials.
8. Teardown environment.
   ```shell
   docker compose -p coro down -v
   ```
