// dbmigrate is a small CLI for managing the server's SQLite migrations.
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

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang-migrate/migrate/v4"

	"pneuma/internal/config"
	"pneuma/internal/store/sqlite"
)

const usageHeader = `Usage: dbmigrate [flags] <command> [args]

Commands:
  up              Apply all pending migrations
  down [N]        Roll back N steps (default 1)
  force <version> Force schema version and clear dirty flag
  version         Print current version and dirty status

Flags:
`

func main() {
	dataDir := flag.String("data", "", "path to data directory (default: $PNEUMA_DATA_DIR or ~/.pneuma)")
	cfgPath := flag.String("config", "", "path to config.toml (default: <data-dir>/config.toml)")
	dbPath := flag.String("db", "", "direct path to SQLite database file (overrides -config)")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usageHeader)
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	dir := *dataDir
	if dir == "" {
		dir = config.DefaultDataDir()
	}

	cPath := *cfgPath
	if cPath == "" {
		cPath = filepath.Join(dir, "config.toml")
	}

	path := *dbPath
	if path == "" {
		cfg, err := config.Load(cPath, dir)
		if err != nil {
			fatalf("load config: %v", err)
		}
		path = cfg.Database.Path
	}

	// FK enforcement must be off for migration 003, involving a recreation of the table, tracks.
	// The pragma must be set on the connection before a transaction begins; it cannot be changed from inside a transaction.
	db, err := sqlite.OpenRaw(path, false)
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()

	m, err := sqlite.NewMigrator(db)
	if err != nil {
		fatalf("build migrator: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Fprintf(os.Stderr, "migrator source close: %v\n", srcErr)
		}
		if dbErr != nil {
			fmt.Fprintf(os.Stderr, "migrator db close: %v\n", dbErr)
		}
	}()

	cmd := args[0]

	switch cmd {
	case "up":
		err = m.Up()
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("already up to date")
			err = nil
		}

	case "down":
		n := 1
		if len(args) > 1 {
			parsed, parseErr := strconv.Atoi(args[1])
			if parseErr != nil || parsed < 1 {
				fatalf("down: N must be a positive integer, got %q", args[1])
			}
			n = parsed
		}
		err = m.Steps(-n)
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("nothing to roll back")
			err = nil
		}

	case "force":
		if len(args) < 2 {
			fatalf("force: requires a version number")
		}
		v, parseErr := strconv.Atoi(args[1])
		if parseErr != nil {
			fatalf("force: version must be an integer, got %q", args[1])
		}
		err = m.Force(v)

	case "version":
		ver, dirty, verErr := m.Version()
		if errors.Is(verErr, migrate.ErrNilVersion) {
			fmt.Println("version: none (no migrations have run)")
		} else if verErr != nil {
			fatalf("version: %v", verErr)
		} else {
			fmt.Printf("version: %d  dirty: %v\n", ver, dirty)
		}
		return

	default:
		fatalf("unknown command %q — run with -help for usage", cmd)
	}

	if err != nil {
		fatalf("%s: %v", cmd, err)
	}

	ver, dirty, _ := m.Version()
	fmt.Printf("done  version: %d  dirty: %v\n", ver, dirty)
}

// fatalf is a wrapper over fmt.Fprintf but triggers os.Exit(1) after printing.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
