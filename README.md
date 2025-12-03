# free5GC MCP (Model Context Protocol) Server

This project is an MCP (Model Context Protocol) server implemented in Go that provides AI assistants (like GitHub Copilot) with tools to manage free5GC 5G core network subscribers and configurations.

## Features

- **Subscriber Management**: Full CRUD operations for 5G subscribers
- **Tenant User Management**: Query users within tenants
- **JWT Authentication**: Automatic authentication with free5GC webconsole
- **VS Code Integration**: Works seamlessly with GitHub Copilot in VS Code

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Setting Up free5GC Backend](#setting-up-free5gc-backend)
3. [Setting Up the MCP Server](#setting-up-the-mcp-server)
4. [VS Code MCP Configuration](#vs-code-mcp-configuration)
5. [Tool Summary](#tool-summary)
6. [Configuration Reference](#configuration-reference)
7. [Usage Examples](#usage-examples)

---

## Prerequisites

- **Go**: Version 1.21 or later
- **MongoDB**: Version 4.4 or later
- **free5GC**: v3.4.x or compatible version
- **VS Code**: With GitHub Copilot extension installed

---

## Setting Up free5GC Backend

### Step 1: Install Dependencies

```bash
# Install Go (if not installed)
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install MongoDB
sudo apt-get install -y mongodb
sudo systemctl start mongodb
sudo systemctl enable mongodb

# Verify MongoDB is running
mongosh --eval "db.adminCommand('ping')"
```

### Step 2: Clone and Build free5GC

```bash
# Clone free5GC
git clone --recursive -b v4.1.0 -j `nproc` https://github.com/free5gc/free5gc.git
cd free5gc

# Build all NFs
make all

# Build webconsole
cd webconsole
go build -o bin/webconsole ./server.go
```

### Step 3: Start free5GC Components

```bash
# Terminal 1: Start the webconsole (backend API server)
cd free5gc/webconsole
./bin/webconsole

# The webconsole will be available at http://127.0.0.1:5000
# Default login: admin / free5gc
```

### Step 4: Verify free5GC is Running

```bash
# Check webconsole health
curl -s http://127.0.0.1:5000/api/health

# Login to get a token (for manual testing)
curl -s -X POST http://127.0.0.1:5000/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"free5gc"}'
```

---

## Setting Up the MCP Server

### Step 1: Build the MCP Server

```bash
cd free5gc-MCP

# Build the binary
make build

# to run the web server + mongodb
make test-up

# Or clean previous artifacts
make clean 
```

### Step 2: Configure the MCP Server

Edit `config/config.yaml`:

```yaml
server:
  addr: ":8080"

free5gc:
  webui_base_url: "http://127.0.0.1:5000"
  username: "admin"
  password: "free5gc"
```

### Step 3: Run the MCP Server

```bash
# Run in foreground
./bin/free5gc-mcp --config config/config.yaml

# Or run in background
nohup ./bin/free5gc-mcp --config config/config.yaml > mcp.log 2>&1 &
```

### Step 4: Verify MCP Server is Running

```bash
# Test MCP initialize handshake
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

# List available tools
curl -s http://127.0.0.1:8080/ \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'
```

---

## VS Code MCP Configuration

To use this MCP server with GitHub Copilot in VS Code, create a configuration file:

### Step 1: Create the MCP Configuration File

Create or edit `~/.vscode/mcp.json`:

```json
{
  "servers": {
    "free5gc-mcp": {
      "type": "http",
      "url": "http://127.0.0.1:8080"
    }
  }
}
```

### Step 2: Reload VS Code

After creating the configuration file:
1. Restart VS Code or reload the window (`Ctrl+Shift+P` → `Developer: Reload Window`)
2. The MCP server tools will now be available to GitHub Copilot

### Step 3: Verify Connection

In VS Code, open a Copilot chat and ask:
- "List all free5GC subscribers"
- "Get tenant users for tenant ID xxx"

Copilot will automatically use the MCP tools to query the free5GC backend.

---

## Tool Summary

The MCP server exposes the following tools to AI assistants:

| Tool Name | API Endpoint | Method | Description |
|-----------|--------------|--------|-------------|
| `tenant_users_get` | `/api/tenant/:tenantId/user` | GET | Get all users for a specific tenant |
| `subscriber_list` | `/api/subscriber` | GET | Get all subscribers from free5GC |
| `subscriber_get` | `/api/subscriber/:ueId/:servingPlmnId` | GET | Get a specific subscriber by UE ID and PLMN ID |
| `subscriber_create` | `/api/subscriber/:ueId/:servingPlmnId` | POST | Create a new subscriber |
| `subscriber_create_multiple` | `/api/subscriber/:ueId/:servingPlmnId/:userNumber` | POST | Create multiple subscribers at once |
| `subscriber_update` | `/api/subscriber/:ueId/:servingPlmnId` | PUT | Full replacement update of a subscriber |
| `subscriber_patch` | `/api/subscriber/:ueId/:servingPlmnId` | PATCH | Partial update of a subscriber |
| `subscriber_delete` | `/api/subscriber/:ueId/:servingPlmnId` | DELETE | Delete a specific subscriber |
| `subscriber_delete_multiple` | `/api/subscriber` | DELETE | Delete multiple subscribers at once |

---

## Configuration Reference

The configuration file `config/config.yaml` supports the following options:

### `server`

| Option | Description | Default |
|--------|-------------|---------|
| `addr` | Listen address for MCP server | `:8080` |
| `api_token_type` | Auth type: `static`, `jwt`, or empty | (empty) |
| `api_token` | Static bearer token (when `api_token_type` is `static`) | |
| `jwt_secret` | HS256 secret for JWT validation | |
| `jwt_public_key_path` | Path to RSA public key for JWT | |

### `free5gc`

| Option | Description | Default |
|--------|-------------|---------|
| `webui_base_url` | URL of free5GC webconsole backend | `http://127.0.0.1:5000` |
| `username` | Webconsole login username | `admin` |
| `password` | Webconsole login password | `free5gc` |

---

## Usage Examples

### Using curl to test MCP tools

```bash
# List all subscribers
curl -s http://127.0.0.1:8080/ \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "subscriber_list",
      "arguments": {}
    }
  }'

# Get a specific subscriber
curl -s http://127.0.0.1:8080/ \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "subscriber_get",
      "arguments": {
        "ueId": "imsi-208930000000001",
        "servingPlmnId": "20893"
      }
    }
  }'
```

### Using GitHub Copilot in VS Code

Once the MCP server is configured and running, you can interact with it naturally through Copilot:

- **"Show me all 5G subscribers"** → Uses `subscriber_list`
- **"Create a new subscriber with IMSI 208930000000099"** → Uses `subscriber_create`
- **"Delete subscriber imsi-208930000000001"** → Uses `subscriber_delete`
- **"Get all users for tenant ID xxx"** → Uses `tenant_users_get`

---

## Quick Start (TL;DR)

```bash
# 1. Start MongoDB
sudo systemctl start mongodb

# 2. Start free5GC webconsole (in another terminal)
cd free5gc/webconsole && ./bin/webconsole

# 3. Build and run MCP server
cd free5gc-MCP
make build
./bin/free5gc-mcp --config config/config.yaml

# 4. Configure VS Code
echo '{"servers":{"free5gc-mcp":{"type":"http","url":"http://127.0.0.1:8080"}}}' > ~/.vscode/mcp.json

# 5. Reload VS Code and start using Copilot with free5GC tools!
```

---

## Notes / Next Steps

- Core start/stop/status handlers are stubs; wire them to `pkg/control` once the operational flow is defined.
- `pkg/infrastructure/microk8s.go` is reserved for k8s automation when you containerize or move away from bare metal.
- Docker and systemd unit files are provided for when you want long-running services.
