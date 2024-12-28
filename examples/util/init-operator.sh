#!/bin/sh

SERVER_URL=${1:-"http://server:5400"}

echo "Using server URL: $SERVER_URL"

echo 'Installing dependencies...'
apk add --no-cache curl jq >/dev/null 2>&1 || {
  echo "Failed to install dependencies. Exiting."
  exit 1
}
echo 'Dependencies installed'

# Add a timeout for the health check loop
echo "Waiting for Coro to be healthy..."
MAX_RETRIES=30
RETRY_COUNT=0

while ! curl -s "$SERVER_URL/healthz" >/dev/null; do
  RETRY_COUNT=$((RETRY_COUNT + 1))
  if [ "$RETRY_COUNT" -ge "$MAX_RETRIES" ]; then
    echo "Health check timed out after $MAX_RETRIES retries. Exiting."
    exit 1
  fi
  sleep 1
  echo "Retrying... ($RETRY_COUNT/$MAX_RETRIES)"
done

echo "Coro is healthy!"

echo "Initializing default namespace..."

# Fetch Namespaces
NAMESPACES_RESPONSE=$(curl -sS -w "\n%{http_code}" "$SERVER_URL/api/v1/namespaces" \
  -H "Content-Type: application/json")

# Extract response body and HTTP status code
NAMESPACES_BODY=$(echo "$NAMESPACES_RESPONSE" | head -n -1)
HTTP_STATUS=$(echo "$NAMESPACES_RESPONSE" | tail -n 1)

# Find the 'default' namespace
if [ "$HTTP_STATUS" -eq 200 ]; then
  echo "Successfully fetched namespaces."
  NAMESPACE_ID=$(echo "$NAMESPACES_BODY" | jq -r '.data[] | select(.name == "default") | .id')
  if [ -z "$NAMESPACE_ID" ]; then
    echo "Error: Default namespace not found. Exiting."
    exit 1
  fi
else
  echo "Failed to fetch Namespaces. HTTP status: $HTTP_STATUS"
  echo "Response: $NAMESPACES_BODY"
  exit 1
fi

echo "Initializing example operator..."

# Fetch Operators
OPERATORS_RESPONSE=$(curl -sS -w "\n%{http_code}" "$SERVER_URL/api/v1/namespaces/$NAMESPACE_ID/operators" \
  -H "Content-Type: application/json")

# Extract response body and HTTP status code
OPERATORS_BODY=$(echo "$OPERATORS_RESPONSE" | head -n -1)
HTTP_STATUS=$(echo "$OPERATORS_RESPONSE" | tail -n 1)

# Find or create the 'example' Operator
if [ "$HTTP_STATUS" -eq 200 ]; then
  echo "Successfully fetched Operators."
  OPERATOR_ID=$(echo "$OPERATORS_BODY" | jq -r '.data[] | select(.name == "example") | .id')
  if [ -n "$OPERATOR_ID" ]; then
    echo "Operator 'example' already exists with ID: $OPERATOR_ID"
  else
    echo "Operator 'example' not found. Creating a new Operator."
    OPERATORS_RESPONSE=$(curl -sS -w "\n%{http_code}" -X POST "$SERVER_URL/api/v1/namespaces/$NAMESPACE_ID/operators" \
      -H "Content-Type: application/json" \
      -d '{"name": "example"}')

    OPERATORS_BODY=$(echo "$OPERATORS_RESPONSE" | head -n -1)
    HTTP_STATUS=$(echo "$OPERATORS_RESPONSE" | tail -n 1)

    if [ "$HTTP_STATUS" -eq 201 ]; then
      OPERATOR_ID=$(echo "$OPERATORS_BODY" | jq -r '.data.id')
      echo "Created new Operator 'example' with ID: $OPERATOR_ID"
    else
      echo "Failed to create Operator. HTTP status: $HTTP_STATUS"
      echo "Response: $OPERATORS_BODY"
      exit 1
    fi
  fi
else
  echo "Failed to fetch Operators. HTTP status: $HTTP_STATUS"
  echo "Response: $OPERATORS_BODY"
  exit 1
fi

# Fetch Operator NATS configuration
mkdir -p nats-config
NATS_CONFIG_RESPONSE=$(curl -sS -w "\n%{http_code}" "$SERVER_URL/api/v1/namespaces/$NAMESPACE_ID/operators/$OPERATOR_ID/nats-config" -o nats-config/nats.conf)
HTTP_STATUS=$(echo "$NATS_CONFIG_RESPONSE" | tail -n 1)

if [ "$HTTP_STATUS" -eq 200 ]; then
  echo "NATS configuration saved to nats-config/nats.conf"
else
  echo "Failed to fetch NATS configuration. HTTP status: $HTTP_STATUS"
  exit 1
fi

echo "Initialization completed successfully."
