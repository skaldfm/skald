package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	DataDir          string
	DBType           string
	DBURL            string
	BackupInterval   time.Duration
	BackupRetain     int
	OpenRegistration bool
	SecureCookies    bool
	MaxUploadBytes   int64
	LogLevel         string
	LogFormat        string
}

func Load() *Config {
	cfg := &Config{
		Port:             envOr("SKALD_PORT", "7707"),
		DataDir:          envOr("SKALD_DATA_DIR", "./data"),
		DBType:           envOr("SKALD_DB_TYPE", "sqlite"),
		DBURL:            os.Getenv("SKALD_DB_URL"),
		BackupInterval:   parseDuration(envOr("SKALD_BACKUP_INTERVAL", "24h")),
		BackupRetain:     parseInt(envOr("SKALD_BACKUP_RETAIN", "14")),
		OpenRegistration: os.Getenv("SKALD_OPEN_REGISTRATION") == "true",
		// Default to secure cookies; operators serving plain HTTP on localhost
		// can set SKALD_SECURE_COOKIES=false.
		SecureCookies: envOr("SKALD_SECURE_COOKIES", "true") != "false",
		LogLevel:      envOr("SKALD_LOG_LEVEL", "info"),
		LogFormat:     envOr("SKALD_LOG_FORMAT", "text"),
	}

	// For SQLite, default DB path is inside data dir
	if cfg.DBType == "sqlite" && cfg.DBURL == "" {
		cfg.DBURL = filepath.Join(cfg.DataDir, "skald.db")
	}

	// Max request body size (default 512 MB — generous enough for audio assets,
	// but bounds unbounded uploads spooling to disk).
	maxMB := 512
	if n, err := strconv.Atoi(envOr("SKALD_MAX_UPLOAD_MB", "512")); err == nil && n > 0 {
		maxMB = n
	}
	cfg.MaxUploadBytes = int64(maxMB) << 20

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
