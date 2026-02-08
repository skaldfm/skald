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
		Port:    envOr("PODFORGE_PORT", "8080"),
		DataDir: envOr("PODFORGE_DATA_DIR", "./data"),
		DBType:  envOr("PODFORGE_DB_TYPE", "sqlite"),
		DBURL:   os.Getenv("PODFORGE_DB_URL"),
	}

	// For SQLite, default DB path is inside data dir
	if cfg.DBType == "sqlite" && cfg.DBURL == "" {
		cfg.DBURL = filepath.Join(cfg.DataDir, "podforge.db")
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
