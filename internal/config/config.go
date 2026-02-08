package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Port    string
	DataDir string
	DBType  string
	DBURL   string
}

func Load() *Config {
	cfg := &Config{
		Port:    envOr("SKALD_PORT", "7707"),
		DataDir: envOr("SKALD_DATA_DIR", "./data"),
		DBType:  envOr("SKALD_DB_TYPE", "sqlite"),
		DBURL:   os.Getenv("SKALD_DB_URL"),
	}

	// For SQLite, default DB path is inside data dir
	if cfg.DBType == "sqlite" && cfg.DBURL == "" {
		cfg.DBURL = filepath.Join(cfg.DataDir, "skald.db")
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
