#!/usr/bin/env bash
set -euo pipefail

# Clean, repeatable end-to-end test against the mock WebUI
# Usage:
#   bash scripts/test-mcp.sh            # run with defaults (mock backend + mock config)
#   KEEP=1 bash scripts/test-mcp.sh     # keep processes running after tests
#   AUTH_TOKEN=xyz bash scripts/test-mcp.sh  # add Authorization header to requests

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)
BIN_MCP="$ROOT_DIR/bin/free5gc-mcp"
BIN_MOCK="$ROOT_DIR/bin/mockwebui"
CFG_MOCK="$ROOT_DIR/config/mock-config.yaml"

LOG_DIR="$ROOT_DIR/.logs"
mkdir -p "$LOG_DIR"
LOG_MOCK="$LOG_DIR/mockwebui.log"
LOG_MCP="$LOG_DIR/mcp.log"

MCP_ADDR=127.0.0.1:8080
MOCK_ADDR=127.0.0.1:5050

tick() { printf "\033[32m✔\033[0m %s\n" "$*"; }
cross() { printf "\033[31m✘\033[0m %s\n" "$*"; }
info() { printf "\033[34m▶\033[0m %s\n" "$*"; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

pretty_json() {
  if command_exists jq; then jq -C .; 
  elif command_exists python3; then python3 -m json.tool; 
  else cat; fi
}

curl_json() {
  local method=$1; shift
  local url=$1; shift
  local data=${1:-}
  local extra=(-sS --max-time 5)
  if [[ -n "${AUTH_TOKEN:-}" ]]; then
    extra+=( -H "Authorization: Bearer ${AUTH_TOKEN}" )
  fi
  if [[ -n "$data" ]]; then
    curl "${extra[@]}" -H 'Content-Type: application/json' -X "$method" "$url" -d "$data"
  else
    curl "${extra[@]}" -X "$method" "$url"
  fi
}

wait_http() {
  local url=$1; shift
  local name=${1:-service}
  for i in {1..50}; do
    if curl -sS --max-time 1 "$url" >/dev/null 2>&1; then
      tick "$name is up: $url"
      return 0
    fi
    sleep 0.2
  done
  cross "Timed out waiting for $name at $url"; return 1
}

shutdown() {
  local p
  for p in ${MCP_PID:-} ${MOCK_PID:-}; do
    if [[ -n "$p" ]] && kill -0 "$p" 2>/dev/null; then
      kill "$p" 2>/dev/null || true
    fi
  done
}
trap '[[ "${KEEP:-0}" != 1 ]] && shutdown || true' EXIT

info "Building binaries (mock + MCP)"
make -C "$ROOT_DIR" build-mock >/dev/null
make -C "$ROOT_DIR" build >/dev/null
tick "Build complete"

info "Starting mock WebUI at http://$MOCK_ADDR"
"$BIN_MOCK" >"$LOG_MOCK" 2>&1 & MOCK_PID=$!
wait_http "http://$MOCK_ADDR/api/subscribers" "mockwebui"

info "Starting MCP against mock config at http://$MCP_ADDR"
"$BIN_MCP" --config "$CFG_MOCK" --addr :8080 >"$LOG_MCP" 2>&1 & MCP_PID=$!
wait_http "http://$MCP_ADDR/health" "mcp"

echo
info "REST: GET /health"
curl_json GET "http://$MCP_ADDR/health" | pretty_json || true

echo
info "REST: POST /tools/convert-time"
curl_json POST "http://$MCP_ADDR/tools/convert-time" '{"time":"2025-11-29T01:00:00Z","from":"UTC","to":"Asia/Kuala_Lumpur"}' | pretty_json || true

echo
info "REST: GET /tools/subscribers (expect [])"
curl_json GET "http://$MCP_ADDR/tools/subscribers" | pretty_json || true

echo
info "REST: POST /tools/subscribers (create)"
curl_json POST "http://$MCP_ADDR/tools/subscribers" '{"id":"001010123456789","imsi":"001010123456789","authType":"5g_aka"}' | pretty_json || true

echo
info "REST: GET /tools/subscribers (should contain one)"
curl_json GET "http://$MCP_ADDR/tools/subscribers" | pretty_json || true

echo
info "MCP: tools/list"
curl_json POST "http://$MCP_ADDR/" '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | \
  (command -v jq >/dev/null && jq '{jsonrpc,id,tools: .result.tools|map(.name)}' || cat) || true

echo
info "MCP: tools/call subscriber_list"
resp=$(curl_json POST "http://$MCP_ADDR/" '{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"subscriber_list","arguments":{}}}')
if command -v jq >/dev/null; then
  echo "$resp" | jq -r '.result.content[0].text' | jq . || echo "$resp" | pretty_json
else
  echo "$resp" | pretty_json
fi

echo
info "MCP: tools/call subscriber_get"
resp=$(curl_json POST "http://$MCP_ADDR/" '{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"subscriber_get","arguments":{"id":"001010123456789"}}}')
if command -v jq >/dev/null; then
  echo "$resp" | jq -r '.result.content[0].text' | jq . || echo "$resp" | pretty_json
else
  echo "$resp" | pretty_json
fi

echo
info "MCP: tools/call subscriber_update"
resp=$(curl_json POST "http://$MCP_ADDR/" '{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"subscriber_update","arguments":{"id":"001010123456789","patch":{"authType":"5g_aka_prime"}}}}')
if command -v jq >/dev/null; then
  echo "$resp" | jq -r '.result.content[0].text' | jq . || echo "$resp" | pretty_json
else
  echo "$resp" | pretty_json
fi

echo
info "MCP: tools/call subscriber_delete"
curl_json POST "http://$MCP_ADDR/" '{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"subscriber_delete","arguments":{"id":"001010123456789"}}}' | pretty_json || true

echo
info "REST: GET /tools/subscribers (expect empty after delete)"
curl_json GET "http://$MCP_ADDR/tools/subscribers" | pretty_json || true

echo
tick "All test calls executed. Logs: $LOG_DIR"
if [[ "${KEEP:-0}" == 1 ]]; then
  info "KEEP=1 set. Leaving processes running. PIDs: mock=$MOCK_PID mcp=$MCP_PID"
else
  shutdown
  info "Processes stopped."
fi
