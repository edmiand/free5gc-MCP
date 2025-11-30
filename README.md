# free5GC MCP (Minimal Control Plane)

This project is a scaffold for an MCP (Model Context Protocol) server implemented in Go.
It exposes a REST Tools Provider that fronts the free5GC WebUI backend so an MCP client (for example a Copilot agent) can:

- proxy subscriber CRUD to the WebUI backend
- expose simple health/status endpoints for MCP orchestration
- enforce either static bearer tokens or JWT validation for inbound MCP calls

## Quick start (bare metal)

```bash
//to run the web server + mongodb
make test-up

cd free5gc-MCP
make build
./bin/free5gc-mcp --config config/config.yaml
```

Once running:

- `GET /health` to check liveness
- `GET /tools/subscribers` to mirror `free5gc/webconsole/backend` subscriber list
- `POST /tools/subscribers` with a JSON payload to create a subscriber via the WebUI backend
- `POST /tools/convert-time` to convert timestamps between time zones/formats

### MCP JSON-RPC entrypoint

GitHub Copilot (or any MCP-compliant host) connects to the root path `/` using JSON-RPC 2.0 (and keeps a GET `/` SSE stream alive for notifications). You can verify the handshake manually:

```bash
curl -s http://127.0.0.1:8080/ \
	-H 'Content-Type: application/json' \
	-d '{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {},
			"clientInfo": {"name": "curl", "version": "0.1"}
		}
	}'
```

Call the new MCP tool without going through the REST helper:

```bash
curl -s http://127.0.0.1:8080/ \
	-H 'Content-Type: application/json' \
	-d '{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/call",
		"params": {
			"name": "convert_time",
			"arguments": {
				"time": "2025-11-29T01:00:00",
				"from": "UTC",
				"to": "Asia/Kuala_Lumpur"
			}
		}
	}'
```

Example request:

```bash
curl -s http://127.0.0.1:8080/tools/convert-time \
	-H 'Content-Type: application/json' \
	-d '{
		"time": "2025-11-29T01:00:00",
		"from": "UTC",
		"to": "Asia/Kuala_Lumpur"
	}'
```

## Configuration

The default config lives at `config/config.yaml` (a copy also exists at the repo root for convenience). Override values via flags or environment-specific files.

### `server`

- `addr`: listen address (default `:8080`).
- `api_token_type`: `"static"` to require a fixed bearer, `"jwt"` to validate inbound JWTs, or empty to disable auth.
- `api_token`: token MCP clients must send when `api_token_type` is `static`.
- `jwt_secret` / `jwt_public_key_path`: HS256 secret or PEM-encoded RSA public key used when `api_token_type` is `jwt`.

### `free5gc`

- `webui_base_url`: point this to your running `free5gc/webconsole/backend` instance (default `http://127.0.0.1:5000`).
- `token`: bearer token sent from MCP to the WebUI backend if that backend enforces auth.
- `subscribers_path`: relative path for subscriber CRUD (default `/api/subscribers`; set it to whatever the WebUI backend expects).

### `infrastructure`

Today this is a placeholder map (e.g., `use_microk8s: false`). Future microk8s/Helm helpers will live under `pkg/infrastructure`.

## Bare metal workflow

1. Start free5GC core + WebUI backend locally (see the `free5gc` repo scripts like `run.sh` and the `webconsole/backend`).
2. Update `config/config.yaml` so `webui_base_url` references the WebUI backend (for example `http://127.0.0.1:5000`).
3. Optionally set a static MCP token or JWT trust material.
4. `make build && ./bin/free5gc-mcp --config config/config.yaml`.
5. Call the MCP endpoints with the same subscriber payloads you would send to the WebUI backend.

## Notes / next steps

- Core start/stop/status handlers are stubs; wire them to `pkg/control` once the operational flow is defined.
- `pkg/infrastructure/microk8s.go` is reserved for k8s automation when you containerize or move away from bare metal.
- Docker and systemd unit files are provided for when you want long-running services.

## Using this server with GitHub Copilot Agents

1. **Build & run the MCP server**
	- `make build`
	- `./bin/free5gc-mcp --config config/config.yaml`
	- Ensure the new `/tools/convert-time` endpoint is reachable (see curl example above).
2. **Expose it to Copilot** (VS Code Insiders ≥ 1.93 with Copilot Agents enabled)
	- Open the Command Palette → `GitHub Copilot: Manage MCP Servers` → `Add HTTP server`.
	- Name it `free5gc-mcp`, set the base URL to `http://127.0.0.1:8080` (requires access to `/` for JSON-RPC and SSE).
	- If you enabled MCP auth, add `Authorization: Bearer <token>` under headers.
3. **Use the tool in chat**
	- Open a Copilot chat session, pick the agent you want (e.g., `@workspace`).
	- Ask: `@workspace convert 2025-11-29T01:00:00 UTC to Asia/Kuala_Lumpur using the free5gc time tool` (Copilot issues `tools/call` → `convert_time`).
	- Copilot calls `/tools/convert-time` (and any other registered tool) and streams back the formatted results.

You can declare additional tools by adding handlers in `pkg/api`, registering the route in `pkg/api/router.go`, and documenting them in this section so Copilot users know what payloads to send.
