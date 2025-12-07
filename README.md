# free5GC MCP (Model Context Protocol) Server

This project is an MCP (Model Context Protocol) server implemented in Go that provides AI assistants (like GitHub Copilot) with tools to manage free5GC 5G core network subscribers and configurations.

## Features

- **Core Network Control**: Start, stop, and monitor all free5GC network functions (NRF, AMF, SMF, UPF, etc.)
- **Subscriber Management**: Full CRUD operations for 5G subscribers
- **Tenant User Management**: Query users within tenants
- **JWT Authentication**: Automatic authentication with free5GC webconsole
- **VS Code Integration**: Works seamlessly with GitHub Copilot in VS Code

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Setting Up free5GC Backend](#setting-up-free5gc-backend)
3. [Configuring Sudo for MCP Server](#configuring-sudo-for-mcp-server)
4. [Patching free5GC run.sh](#patching-free5gc-runsh)
5. [Setting Up the MCP Server](#setting-up-the-mcp-server)
6. [VS Code MCP Configuration](#vs-code-mcp-configuration)
7. [Tool Summary](#tool-summary)
8. [Configuration Reference](#configuration-reference)
9. [Usage Examples](#usage-examples)

---

## Prerequisites

- **Go**: Version 1.22 or later (1.23+ recommended)
- **MongoDB**: Version 4.4 or later
- **free5GC**: v4.1.0 or compatible version
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

## Configuring Sudo for MCP Server

> ⚠️ **IMPORTANT DISCLAIMER**: The MCP server requires passwordless sudo privileges to start and stop free5GC network functions. This is because free5GC's `run.sh` script requires root privileges to configure network interfaces and start the UPF (User Plane Function).

### Step 1: Configure visudo

The user running the MCP server must have passwordless sudo access for the free5GC scripts. Run `sudo visudo` and add the following lines:

> **⚠️ IMPORTANT**: Replace `<YOUR_USERNAME>` with your actual Linux username in ALL instances below. The configuration will not work if you don't replace the placeholder!

### Recommended: Restrict to specific commands

```bash
# Allow passwordless sudo for free5GC operations (RECOMMENDED)
<YOUR_USERNAME> ALL=(ALL) NOPASSWD: /home/<YOUR_USERNAME>/free5gc/run.sh
<YOUR_USERNAME> ALL=(ALL) NOPASSWD: /home/<YOUR_USERNAME>/free5gc/force_kill.sh
<YOUR_USERNAME> ALL=(ALL) NOPASSWD: /usr/bin/ip
<YOUR_USERNAME> ALL=(ALL) NOPASSWD: /usr/sbin/ip
```

### Step 2: Verify sudo Configuration

```bash
# Test that sudo works without password (replace <YOUR_USERNAME>)
sudo -n /home/<YOUR_USERNAME>/free5gc/run.sh --help
```

---

## Patching free5GC run.sh

> ⚠️ **REQUIRED**: The default `run.sh` script in free5GC contains a sudo credential check that blocks non-interactive execution. This must be removed for the MCP server to work properly.

### Step 1: Edit run.sh

Open the free5GC `run.sh` script:

```bash
cd ~/free5gc
nano run.sh
```

### Step 2: Remove the Sudo Check Block

Find and **remove** or **comment out** the following block near the beginning of the script:

```bash
# Remove this block:
if ! sudo -n true 2>/dev/null; then
    echo "This script requires sudo privileges. Please enter your password:"
    sudo -v
    if [ $? -ne 0 ]; then
        echo "Failed to obtain sudo privileges. Exiting."
        exit 1
    fi
fi
```

### Step 3: Verify the Patch

After patching, the script should be able to run non-interactively:

```bash
# This should not prompt for password if visudo is configured correctly
sudo ./run.sh &
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

Edit `config.yaml`:

```yaml
server:
  addr: "127.0.0.1:8080"  # Bind to localhost only for security

free5gc:
  webui_base_url: "http://127.0.0.1:5000"
  username: "admin"
  password: "free5gc"  # CHANGE THIS IN PRODUCTION!
  free5gc_path: "/home/<YOUR_USERNAME>/free5gc"  # Required for start/stop/status tools
```

> ⚠️ **IMPORTANT**: 
> - Replace `<YOUR_USERNAME>` with your actual Linux username in `free5gc_path`
> - The `free5gc_path` must be set to the absolute path of your free5GC installation directory
> - For production, change the default password and consider enabling authentication (see Security section)

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

### Core Network Control Tools

| Tool Name | Description |
|-----------|-------------|
| `free5gc_start` | Start all free5GC network functions (NRF, AMF, SMF, UPF, etc.) and the webconsole |
| `free5gc_stop` | Stop all free5GC network functions and webconsole gracefully |
| `free5gc_status` | Get the current status of all free5GC network functions and webconsole |

#### Network Function Ports

When running, the free5GC network functions listen on the following addresses:

| NF | Address | Protocol |
|----|---------|----------|
| NRF | 127.0.0.10:8000 | HTTP |
| AMF | 127.0.0.18:8000 | HTTP |
| SMF | 127.0.0.2:8000 | HTTP |
| UDR | 127.0.0.4:8000 | HTTP |
| PCF | 127.0.0.7:8000 | HTTP |
| UDM | 127.0.0.3:8000 | HTTP |
| NSSF | 127.0.0.31:8000 | HTTP |
| AUSF | 127.0.0.9:8000 | HTTP |
| UPF | 127.0.0.8:8805 | PFCP (UDP) |
| CHF | 127.0.0.113:8000 | HTTP |
| NEF | 127.0.0.5:8000 | HTTP |
| Webconsole | 0.0.0.0:5000 | HTTP |

### Subscriber Management Tools

| Tool Name | API Endpoint | Method | Description |
|-----------|--------------|--------|-------------|
| `subscriber_list` | `/api/subscriber` | GET | Get all subscribers from free5GC |
| `subscriber_get` | `/api/subscriber/:ueId/:servingPlmnId` | GET | Get a specific subscriber by UE ID and PLMN ID |
| `subscriber_create` | `/api/subscriber/:ueId/:servingPlmnId` | POST | Create a new subscriber |
| `subscriber_create_multiple` | `/api/subscriber/:ueId/:servingPlmnId/:userNumber` | POST | Create multiple subscribers at once |
| `subscriber_update` | `/api/subscriber/:ueId/:servingPlmnId` | PUT | Full replacement update of a subscriber |
| `subscriber_patch` | `/api/subscriber/:ueId/:servingPlmnId` | PATCH | Partial update of a subscriber |
| `subscriber_delete` | `/api/subscriber/:ueId/:servingPlmnId` | DELETE | Delete a specific subscriber |
| `subscriber_delete_multiple` | `/api/subscriber` | DELETE | Delete multiple subscribers at once |

### Tenant Management Tools

| Tool Name | API Endpoint | Method | Description |
|-----------|--------------|--------|-------------|
| `tenant_users_get` | `/api/tenant/:tenantId/user` | GET | Get all users for a specific tenant |

---

## Configuration Reference

The configuration file `config/config.yaml` supports the following options:

### `server`

| Option | Description | Default |
|--------|-------------|---------|  
| `addr` | Listen address for MCP server | `127.0.0.1:8080` |
| `api_token_type` | Auth type: `static`, `jwt`, or empty | `static` |
| `api_token` | Static bearer token (when `api_token_type` is `static`) | |
| `jwt_secret` | HS256 secret for JWT validation | |
| `jwt_public_key_path` | Path to RSA public key for JWT | |

> **⚠️ Security Warning:**  
> The default configuration binds to localhost (`127.0.0.1:8080`) with authentication enabled.  
> Binding to all interfaces (`:8080`) or disabling authentication is **NOT RECOMMENDED** unless you are in a tightly controlled environment.  
> Any network client that can reach the MCP server will be able to manage subscribers and control free5GC operations.

### `free5gc`

| Option | Description | Default |
|--------|-------------|---------|  
| `webui_base_url` | URL of free5GC webconsole backend | `http://127.0.0.1:5000` |
| `username` | Webconsole login username | `admin` |
| `password` | Webconsole login password | `free5gc` |
| `free5gc_path` | Absolute path to free5GC installation directory | `~/free5gc` |
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

**Core Network Control:**
- **"Start free5GC"** → Uses `free5gc_start`
- **"Stop free5GC"** → Uses `free5gc_stop`
- **"What's the status of free5GC?"** → Uses `free5gc_status`
- **"Check if all NFs are running"** → Uses `free5gc_status`

**Subscriber Management:**
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
# Edit config.yaml and set free5gc_path to your free5GC installation path
./bin/free5gc-mcp --config config.yaml

# 4. Configure VS Code
echo '{"servers":{"free5gc-mcp":{"type":"http","url":"http://127.0.0.1:8080"}}}' > ~/.vscode/mcp.json

# 5. Reload VS Code and start using Copilot with free5GC tools!
```

---

## Notes / Next Steps

- The `free5gc_start`, `free5gc_stop`, and `free5gc_status` tools are fully implemented and control the bare-metal free5GC installation.
- `pkg/infrastructure/microk8s.go` is reserved for k8s automation when you containerize or move away from bare metal.
- Docker and systemd unit files are provided for when you want long-running services.

## Troubleshooting

### "sudo: a password is required" Error

If you see this error when using `free5gc_start` or `free5gc_stop`, ensure:
1. You have configured visudo correctly (see [Configuring Sudo for MCP Server](#configuring-sudo-for-mcp-server))
2. The `run.sh` script has been patched (see [Patching free5GC run.sh](#patching-free5gc-runsh))

### Webconsole Not Detected as Running

The webconsole binds to `0.0.0.0:5000`. If status shows it as stopped but it's actually running, check:
```bash
ss -tlnp | grep :5000
```

### Network Functions Not Starting

Ensure MongoDB is running before starting free5GC:
```bash
sudo systemctl status mongodb
sudo systemctl start mongodb
```
