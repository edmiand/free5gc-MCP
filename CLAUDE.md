# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

An MCP (Model Context Protocol) server written in Go that exposes free5GC 5G core network management as tools consumable by AI assistants (e.g., GitHub Copilot via VS Code). It bridges JSON-RPC 2.0 MCP requests to the free5GC webconsole REST API and to local shell/Kubernetes operations.

## Common Commands

```bash
# Build the server binary to bin/free5gc-mcp
make build

# Run the server (requires bin/free5gc-mcp to exist)
make run                   # uses config/config.yaml, listens on :8080
make run-mcp-mock          # uses config/mock-config.yaml (points to mock webui)

# Build and run the mock free5GC WebUI (for local development without free5GC)
make build-mock
make run-mock

# Run all tests (unit + e2e; e2e requires Docker for dockertest)
go test ./...

# Run a single test package
go test ./pkg/control/...
go test ./cmd/server/...

# Run a specific test
go test ./pkg/control/... -run TestLogin_Success

# Integration/e2e test using docker-compose
make test-up    # start test containers
make test-logs  # tail logs
make test-down  # teardown
```

## Architecture

### Request flow

```
AI Client (e.g. Copilot)
  → HTTP POST / (JSON-RPC 2.0)
    → pkg/api/router.go  (Gin router, optional auth middleware)
      → pkg/mcp/server.go  (MCP protocol handler: initialize / tools/list / tools/call)
        → pkg/control/free5gc_client.go  (webconsole REST calls + local process control)
```

### Key packages

| Package | Role |
|---|---|
| `cmd/server` | Entry point; loads config, wires dependencies, starts Gin |
| `cmd/mockwebui` | Lightweight fake webconsole for offline development/testing |
| `pkg/config` | Loads `config.yaml` (YAML → structs) |
| `pkg/auth` | Bearer-token or JWT middleware for the MCP endpoint |
| `pkg/api` | Gin router; exposes `/` (MCP), `/health`, `/tools/*` REST shim |
| `pkg/mcp` | Full MCP JSON-RPC 2.0 implementation; tool schemas live here |
| `pkg/control` | `Free5GCClient` — authenticates with webconsole, calls subscriber/tenant APIs, and controls local NFs via shell scripts |

### Authentication

`Free5GCClient` authenticates against the free5GC webconsole with username/password on first use, caches the JWT, and automatically re-logs in on 401 responses (`doRequestWithRetry`). The `Token` header (not `Authorization`) is used by the webconsole API.

The MCP server itself optionally enforces incoming auth (configured via `server.api_token_type`): `static` (shared bearer token) or `jwt` (HS256 secret or RSA public key).

### Adding new MCP tools

1. Add the tool schema to `handleListTools` in `pkg/mcp/server.go`.
2. Add a `case` for it in `handleCallTool`.
3. Implement the handler method (`callXxx`) on `*Server`.
4. If the tool needs a new webconsole call, add the method to `Free5GCClient` in `pkg/control/free5gc_client.go`.

### Config file

`config/config.yaml` — two top-level sections:
- `server` — listen addr, auth type/token/JWT settings
- `free5gc` — webconsole URL, credentials, path to local free5GC install (for start/stop/status)

`config/mock-config.yaml` points to the mock webui for development without a real free5GC.

### Tests

- `pkg/control/free5gc_client_test.go` — unit tests using `httptest.NewServer` to mock the webconsole.
- `cmd/server/main_test.go` — e2e test (`TestE2E_ServerWithWebUI`) that spins up real MongoDB and free5gc/webui Docker containers via `dockertest`, builds the binary, and runs full JSON-RPC scenarios.
