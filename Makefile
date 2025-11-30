BINARY=bin/free5gc-mcp

.PHONY: build docker run install

build:
	go build -o $(BINARY) ./cmd/server

build-mock:
	go build -o bin/mockwebui ./cmd/mockwebui

docker:
	docker build -t free5gc-mcp:latest .

run:
	$(BINARY) --config config/config.yaml --addr :8080

run-mock:
	go run ./cmd/mockwebui

run-mcp-mock:
	$(BINARY) --config config/mock-config.yaml --addr :8080

.PHONY: test-mock
test-mock:
	bash scripts/test-mcp.sh

install:
	install -Dm755 $(BINARY) /usr/local/bin/free5gc-mcp
	install -Dm644 systemd/free5gc-mcp.service /etc/systemd/system/free5gc-mcp.service
	install -Dm644 config/config.yaml /etc/free5gc-mcp/config.yaml
	systemctl daemon-reload
	systemctl enable --now free5gc-mcp

.PHONY: test-up test-down test-logs

test-up:
	docker compose -f docker-compose.yaml up -d --build

test-down:
	docker compose -f docker-compose.yaml down -v

test-logs:
	docker compose -f docker-compose.yaml logs -f