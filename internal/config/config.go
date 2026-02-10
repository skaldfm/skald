package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Port           string
	DataDir        string
	DBType         string
	DBURL          string
	SecretKey      string
	BackupInterval time.Duration
	BackupRetain   int
}

func Load() *Config {
	cfg := &Config{
		Port:           envOr("SKALD_PORT", "7707"),
		DataDir:        envOr("SKALD_DATA_DIR", "./data"),
		DBType:         envOr("SKALD_DB_TYPE", "sqlite"),
		DBURL:          os.Getenv("SKALD_DB_URL"),
		SecretKey:      os.Getenv("SKALD_SECRET_KEY"),
		BackupInterval: parseDuration(envOr("SKALD_BACKUP_INTERVAL", "24h")),
		BackupRetain:   parseInt(envOr("SKALD_BACKUP_RETAIN", "14")),
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

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

func parseInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 14
	}
	return n
}
