#!/bin/bash
set -e
cd ~/free5gc-MCP

if [ "$1" != "--updated" ]; then
    git pull origin main
    exec bash ~/free5gc-MCP/update.sh --updated
fi

make build && sudo install -Dm755 bin/free5gc-mcp /usr/local/bin/free5gc-mcp && sudo systemctl restart free5gc-mcp
echo "Deploy done — $(git log --oneline -1)"
