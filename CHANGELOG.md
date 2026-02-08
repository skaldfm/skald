# Changelog

All notable changes to PodForge will be documented in this file.

## [Unreleased]

### Added
- Project scaffolding: Go module, directory structure, Makefile
- Configuration loading from environment variables
- SQLite database with WAL mode, foreign keys, and busy timeout
- Migration runner with version tracking
- Initial database schema: shows, episodes, guests, assets, tags with linking tables
- Chi router with logging and recovery middleware
- Static file serving
- Health check endpoint (`/health`)
- Cross-compilation targets for linux/darwin/windows (amd64 + arm64)
