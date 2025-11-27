BINARY=bin/free5gc-mcp

.PHONY: build docker run install

build:
	go build -o $(BINARY) ./cmd/server

docker:
	docker build -t free5gc-mcp:latest .

run:
	$(BINARY) --config config/config.yaml --addr :8080

install:
	install -Dm755 $(BINARY) /usr/local/bin/free5gc-mcp
	install -Dm644 systemd/free5gc-mcp.service /etc/systemd/system/free5gc-mcp.service
	install -Dm644 config/config.yaml /etc/free5gc-mcp/config.yaml
	systemctl daemon-reload
	systemctl enable --now free5gc-mcp
