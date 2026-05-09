// Package main contains dbmigrate, a small CLI wrapper around golang-migrate for managing the server's SQLite migrations.
//
// If wanted, use golang-migrate's CLI tool to do this instead at https://github.com/golang-migrate/migrate/tree/master/cmd/migrate.
// This tool is for those that don't want to install another external tool.
//
// Usage:
//
// go run ./cmd/dbmigrate [-config path] [-db path] <command> [args]
//
// Commands:
//
// up              Apply all pending migrations
// down [N]        Roll back N steps (default 1)
// force <version> Force schema version and clear the dirty flag
// version         Print current version and dirty status
//
// Either -config or -db must resolve to the database file. -db takes
// precedence and lets you skip having a valid config.toml on the machine.
package main
