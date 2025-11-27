FROM golang:1.20-alpine AS builder
WORKDIR /src
COPY . .
RUN apk add --no-cache git && \
    go build -o /out/free5gc-mcp ./cmd/server

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/free5gc-mcp /usr/local/bin/free5gc-mcp
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/free5gc-mcp"]
