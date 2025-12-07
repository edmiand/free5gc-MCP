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
	Free5GCPath     string `yaml:"free5gc_path"` // Path to free5gc directory for core control
	SubscribersPath string `yaml:"subscribers_path"`
}

type K8sConfig struct {
	K8sTool           string `yaml:"k8s_tool"`            // "microk8s" | "kubectl" | "k3s"
	HelmBasePath      string `yaml:"base_path"`           // path to free5gc-helm
	ChartPath         string `yaml:"chart_path"`          // path to free5gc-helm chart
	UeransimChartPath string `yaml:"ueransim_chart_path"` // path to ueransim chart
	Namespace         string `yaml:"namespace"`           // k8s namespace
	ReleaseName       string `yaml:"release_name"`        // helm release name
}

type Config struct {
	Server         ServerConfig              `yaml:"server"`
	Free5GC        Free5GCConfig             `yaml:"free5gc"`
	Infrastructure map[string]interface{}    `yaml:"infrastructure"`
	K8s            K8sConfig                 `yaml:"k8s"`
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
		cfg.Server.Addr = "127.0.0.1:8080"
	}
	if cfg.K8s.K8sTool == "" {
		cfg.K8s.K8sTool = "microk8s"
	}
	if cfg.K8s.Namespace == "" {
		cfg.K8s.Namespace = "free5gc"
	}
	if cfg.K8s.ReleaseName == "" {
		cfg.K8s.ReleaseName = "free5gc"
	}
	return &cfg, nil
}
