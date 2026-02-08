package config

import (
	"encoding/json"
	"fmt"
	"os"

	"openclaw-bridge/shared/protocol"
)

type Config struct {
	RelayURL         string        `json:"relay_url"`
	AccessCode       string        `json:"access_code"`
	AccessCodeHash   string        `json:"access_code_hash"`
	Generation       int           `json:"generation"`
	Caps             protocol.Caps `json:"caps"`
	ReconnectSeconds int           `json:"reconnect_seconds"`
	Gateway          GatewayConfig `json:"gateway"`
}

type GatewayConfig struct {
	URL                     string              `json:"url"`
	Auth                    GatewayAuthConfig   `json:"auth"`
	Client                  GatewayClientConfig `json:"client"`
	MinProtocol             int                 `json:"min_protocol"`
	MaxProtocol             int                 `json:"max_protocol"`
	Scopes                  []string            `json:"scopes"`
	Locale                  string              `json:"locale"`
	UserAgent               string              `json:"user_agent"`
	ChallengeTimeoutSeconds int                 `json:"challenge_timeout_seconds"`
	ReconnectInitialSeconds int                 `json:"reconnect_initial_seconds"`
	ReconnectMaxSeconds     int                 `json:"reconnect_max_seconds"`
	SendMethod              string              `json:"send_method"`
	SendTo                  string              `json:"send_to"`
	CancelMethod            string              `json:"cancel_method"`
}

type GatewayAuthConfig struct {
	Token string `json:"token"`
}

type GatewayClientConfig struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Mode        string `json:"mode"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if cfg.RelayURL == "" {
		return Config{}, fmt.Errorf("relay_url is required")
	}
	if cfg.Generation == 0 {
		cfg.Generation = 1
	}
	if cfg.ReconnectSeconds <= 0 {
		cfg.ReconnectSeconds = 2
	}
	if cfg.AccessCodeHash == "" {
		if cfg.AccessCode == "" {
			return Config{}, fmt.Errorf("access_code or access_code_hash is required")
		}
		cfg.AccessCodeHash = protocol.HashAccessCode(cfg.AccessCode)
	}
	if cfg.Gateway.URL == "" {
		cfg.Gateway.URL = "ws://127.0.0.1:18789"
	}
	if cfg.Gateway.Client.ID == "" {
		cfg.Gateway.Client.ID = "cli"
	}
	if cfg.Gateway.Client.DisplayName == "" {
		cfg.Gateway.Client.DisplayName = "OpenClaw Bridge Connector"
	}
	if cfg.Gateway.Client.Version == "" {
		cfg.Gateway.Client.Version = "0.1.0"
	}
	if cfg.Gateway.Client.Platform == "" {
		cfg.Gateway.Client.Platform = "linux"
	}
	if cfg.Gateway.Client.Mode == "" {
		cfg.Gateway.Client.Mode = "operator"
	}
	if cfg.Gateway.MinProtocol <= 0 {
		cfg.Gateway.MinProtocol = 3
	}
	if cfg.Gateway.MaxProtocol <= 0 {
		cfg.Gateway.MaxProtocol = cfg.Gateway.MinProtocol
	}
	if len(cfg.Gateway.Scopes) == 0 {
		cfg.Gateway.Scopes = []string{"operator.read", "operator.write"}
	}
	if cfg.Gateway.Locale == "" {
		cfg.Gateway.Locale = "en-US"
	}
	if cfg.Gateway.UserAgent == "" {
		cfg.Gateway.UserAgent = "openclaw-bridge-connector/0.1.0"
	}
	if cfg.Gateway.ChallengeTimeoutSeconds <= 0 {
		cfg.Gateway.ChallengeTimeoutSeconds = 8
	}
	if cfg.Gateway.ReconnectInitialSeconds <= 0 {
		cfg.Gateway.ReconnectInitialSeconds = 1
	}
	if cfg.Gateway.ReconnectMaxSeconds <= 0 {
		cfg.Gateway.ReconnectMaxSeconds = 30
	}
	if cfg.Gateway.SendMethod == "" {
		cfg.Gateway.SendMethod = "send"
	}
	if cfg.Gateway.CancelMethod == "" {
		cfg.Gateway.CancelMethod = "cancel"
	}

	return cfg, nil
}
