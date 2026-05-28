#!/bin/bash
set -e
cd ~/free5gc-MCP
git pull origin main && make build && sudo install -Dm755 bin/free5gc-mcp /usr/local/bin/free5gc-mcp && sudo systemctl restart free5gc-mcp
echo "Deploy done — $(git log --oneline -1)"
