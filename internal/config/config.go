// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Address      string
	Token        string
	DataDir      string
	AllowedHosts []string
	CAFile       string
}

func Load() (Config, error) {
	cfg := Config{Address: os.Getenv("DBM_ADDRESS")}
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:8080"
	}
	cfg.DataDir = os.Getenv("DBM_DATA_DIR")
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if raw := os.Getenv("DBM_ALLOWED_HOSTS"); raw != "" {
		cfg.AllowedHosts = strings.Split(raw, ",")
	}
	cfg.CAFile = os.Getenv("DBM_DB_CA_FILE")
	tokenFile := os.Getenv("DBM_TOKEN_FILE")
	if tokenFile == "" {
		return cfg, fmt.Errorf("DBM_TOKEN_FILE ist erforderlich")
	}
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return cfg, fmt.Errorf("API-Token-Datei konnte nicht gelesen werden")
	}
	cfg.Token = strings.TrimSpace(string(data))
	if len(cfg.Token) < 32 {
		return cfg, fmt.Errorf("API-Token muss mindestens 32 Zeichen enthalten")
	}
	return cfg, nil
}
