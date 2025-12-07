package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/q1317540161/free5gc-MCP/pkg/api"
	"github.com/q1317540161/free5gc-MCP/pkg/auth"
	"github.com/q1317540161/free5gc-MCP/pkg/config"
	"github.com/q1317540161/free5gc-MCP/pkg/control"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	fmt.Printf("Loading config: %s\n", *configPath)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// override addr if provided via flag
	if addr != nil && *addr != "" {
		cfg.Server.Addr = *addr
	}

	fmt.Printf("Starting free5GC MCP on %s\n", cfg.Server.Addr)

	client := control.NewFree5GCClient(
		cfg.Free5GC.BaseURL, 
		cfg.Free5GC.Username, 
		cfg.Free5GC.Password,
		cfg.Free5GC.Free5GCPath,
	)

	// setup auth
	// translate server config into auth config for API package
	authCfg := &auth.AuthConfig{
		Type:             cfg.Server.APITokenType,
		StaticToken:      cfg.Server.APIToken,
		JWTSecret:        cfg.Server.JWTSecret,
		JWTPublicKeyPath: cfg.Server.JWTPublicKeyPath,
	}
	if err := authCfg.Load(); err != nil {
		log.Fatalf("failed to load auth config: %v", err)
	}

	r := api.SetupRouter(client, authCfg)
	if err := r.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
