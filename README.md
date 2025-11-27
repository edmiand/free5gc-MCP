# free5GC MCP (Minimal Control Plane)

This project is a scaffold for an MCP (Model Context Protocol) server implemented in Go.
It provides a Tools Provider REST API for controlling free5GC (subscriber CRUD, core start/stop, config operations) and infrastructure helpers for microk8s + Helm.

Defaults:
- HTTP port: 8080
- Config: `config/config.yaml`

Quick start (local):

1. Build:

```bash
cd free5gc-MCP
make build
```

2. Run:

```bash
./bin/free5gc-mcp --config config/config.yaml
```

3. Example API:

- `GET /health`
- `GET /tools/subscribers`
- `POST /tools/subscribers` {"id":"imsi-001","name":"user1"}

Notes:
- This is a scaffold. Several control functions are stubs and need to be wired to the free5GC WebUI and microk8s.
