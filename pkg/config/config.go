package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Addr     string `yaml:"addr"`
	APIToken string `yaml:"api_token"`
	APITokenType string `yaml:"api_token_type"` // "static" or "jwt" or "" (none)
	JWTSecret    string `yaml:"jwt_secret"`    // optional, for HS256
	JWTPublicKeyPath string `yaml:"jwt_public_key_path"` // optional, for RS256 (PEM file)
}

type Free5GCConfig struct {
	BaseURL         string `yaml:"webui_base_url"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	SubscribersPath string `yaml:"subscribers_path"`
}

type Config struct {
	Server      ServerConfig `yaml:"server"`
	Free5GC     Free5GCConfig `yaml:"free5gc"`
	Infrastructure map[string]interface{} `yaml:"infrastructure"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	// set defaults
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Free5GC.SubscribersPath == "" {
		cfg.Free5GC.SubscribersPath = "/api/subscribers"
	}
	return &cfg, nil
}
