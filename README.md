# pneuma

pneuma is an open-source, self-hostable, and local-first music player and server, designed to give a Spotify-like experience.

## Demo

TBA

## Features

- Self-hostable remote server for streaming music to desktop app

## Why?

## Technology

pneuma is built with:

1. Go
2. TypeScript
3. Wails
4. SQLite
5. sqlc
6. Svelte

## Getting Started

Install the following:

1. Go 1.24+
2. Node.js 20+
3. Wails https://wails.io/docs/gettingstarted/installation
4. sqlc https://docs.sqlc.dev/en/stable/overview/install.html

### Running the desktop application

Run `wails dev` to start the desktop application in development mode. By default, it runs both the frontend and the Go process. It will also install frontend dependencies if not already installed.

Run `wails build` to build the desktop application. The output executable will be under `build/bin/pneuma`.

Upon first start, the desktop application will create a directory `${OS_CACHE}/pneuma/` for storing its SQLite database and other types of data. See https://pkg.go.dev/os#UserCacheDir to find the appropriate cache directory for your OS.

### Running the server

Run `go run ./cmd/server` to start the server. The default port is 8989.

Run `go build -o build/bin/server ./cmd/server` to create the server executable at `./build/bin/` with the name, `server` (on Unix) or `server.exe` (on Windows).

Upon first start, the server will create a directory `${HOME}/.pneuma/` for storing its SQLite database and other types of data. Visit `localhost:8989` to register an admin user.

To make changes to the admin dashboard UI, `cd ./web` and run `npm run build`. Restart the server. The server will load the new changes from `./web/dist/`.

By default, it embeds the admin dashboard UI from `./web/dist/` (if it exists). To exclude the UI, run `go build -tags no_embed -o build/bin/server ./cmd/server`.

### Formatting

Run `go fmt ./...` to format Go code.

Run `npm i` at the root directory to install `prettier`, then run `npm run fmt` to format TypeScript and Svelte code.

### Running sqlc

sqlc is used to generate Go code from SQL queries.

Add SQL query files under `internal/store/sqlite/<desktop or server>/query/`.

Once done so, run `sqlc generate` to generate the Go code equivalent of the queries. The generated code will be placed under `internal/store/sqlite/<desktop or server>db/` with the file extension `.sql.go`. They can be imported from the the package, `<desktop or server>db`.

### Database Migrations

Create new migration SQL files under `internal/store/sqlite/<desktop or server>/migrations/`.

Run `go run ./cmd/dbmigrate up` to apply all pending migrations.  
Run `go run ./cmd/dbmigrate down [N]` to roll back N steps (default 1).  
Run `go run ./cmd/dbmigrate force <version>` to force schema version and clear the dirty flag.  
Run `go run ./cmd/dbmigrate version` to print current version and dirty status.
