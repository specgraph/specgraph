#!/usr/bin/env bash
set -euo pipefail

echo "=== SpecGraph E2E Smoke Test ==="

cleanup() {
    echo "Cleaning up..."
    kill $SERVER_PID 2>/dev/null || true
    docker rm -f specgraph-e2e-memgraph 2>/dev/null || true
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

# Start Memgraph
echo "Starting Memgraph..."
docker run -d --name specgraph-e2e-memgraph -p 7688:7687 memgraph/memgraph:latest
sleep 5

# Temp dir for config and binary
TMPDIR=$(mktemp -d)

# Server config
cat > "$TMPDIR/config.yaml" <<'YAML'
server:
  mode: external
  host: "127.0.0.1"
  port: 9091
storage:
  backend: memgraph
  memgraph:
    bolt_uri: "bolt://localhost:7688"
YAML

# Build
echo "Building specgraph..."
go build -o "$TMPDIR/specgraph" ./cmd/specgraph

# Start server
echo "Starting server..."
"$TMPDIR/specgraph" serve --config "$TMPDIR/config.yaml" &
SERVER_PID=$!
sleep 2

# Client config
cat > "$TMPDIR/client.yaml" <<'YAML'
server:
  remote: "http://127.0.0.1:9091"
YAML

SG="$TMPDIR/specgraph --config=$TMPDIR/client.yaml"

# Test: create
echo "Test: create..."
$SG create login-api --intent "REST endpoint for OAuth2 login" --priority p1
echo "PASS: create"

# Test: list
echo "Test: list..."
OUTPUT=$($SG list)
echo "$OUTPUT"
echo "$OUTPUT" | grep -q "login-api" || { echo "FAIL: login-api not in list"; exit 1; }
echo "PASS: list"

# Test: show
echo "Test: show..."
OUTPUT=$($SG show login-api)
echo "$OUTPUT"
echo "$OUTPUT" | grep -q "REST endpoint for OAuth2 login" || { echo "FAIL: intent not shown"; exit 1; }
echo "PASS: show"

echo "=== All E2E tests passed ==="
