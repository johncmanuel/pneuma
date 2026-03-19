# pneuma

pneuma is an open-source, self-hostable, and local-first music project, designed to give a Spotify-like experience. It is composed of a desktop application for local music playback and a server for music storage and streaming.

<!-- TODO: add demo as mp4, add screenshots -->

> NOTE: This project is currently under active development. Expect bugs and possibly breaking changes.

## Highlights

- **Self-organizing music library**: A music library is organized using metadata from the music files themselves. Playlists can be created by users to create custom collections of music. When using the server to store and stream your music, it makes use of fingerprinting via metadata and hashing to detect duplicate songs.
- **Real-time playback sync**: WebSocket-driven playback engines keep playback state, queues, and progress tightly in sync between the server and the local client. This is useful when playing music in a playlist with a mix of local and remote audio tracks.
- **Automatic library monitoring**: Background directory watchers automatically detect newly added or removed music files and update your library in real-time.
- **Cross-platform**: pneuma natively supports Windows, macOS, and Linux.
- **Offline-first**: pneuma is designed to work entirely offline. The desktop application can be used without the server, focusing on local playback for music on your own machine.
- **Multi-user ready**: The server includes a built-in admin web dashboard to manage itself and allows multiple users to maintain their own isolated profiles and custom playlists on a single instance.

## Why make this?

pneuma was built to address some problems I've had with Spotify.

As a premium user since 2018, I've noticed that Spotify's UX gradually worsened. It worsened by bloating the service with features such as short-form content, social integration (combining a music streaming and social media service into one), and the sudden increase in AI-generated content. These changes have made it difficult to find and listen to music I enjoy.

## Technology

pneuma is built with:

1. Go
2. TypeScript
3. Wails
4. SQLite
5. sqlc
6. Svelte
7. Docker (for server deployment)

## Getting Started

Install the following:

1. Go 1.24+
2. Node.js 20+
3. Wails https://wails.io/docs/gettingstarted/installation
4. sqlc https://docs.sqlc.dev/en/stable/overview/install.html
5. (OPTIONAL): golang-migrate CLI tool https://github.com/golang-migrate/migrate/tree/master/cmd/migrate

### Running the desktop application

Run `wails dev` to start the desktop application in development mode. By default, it runs both the frontend and the Go process. It will also install frontend dependencies if not already installed.

Run `wails build` to build the desktop application. The output executable will be under `build/bin/pneuma`.

Upon first start, the desktop application will create a directory `${OS_CACHE}/pneuma/` for storing its SQLite database and other types of data. See https://pkg.go.dev/os#UserCacheDir to find the appropriate cache directory for your OS.

### Running the server

First, run the following to build the UI:

```bash
cd web
npm run build
```

The build output is placed under `dist/`.

Run `go run ./cmd/server` to start the server. The default port is 8989.

Run `go build -o build/bin/server ./cmd/server` to create the server executable at `./build/bin/` with the name, `server` (on Unix) or `server.exe` (on Windows).

Upon first start, the server will create a directory `${HOME}/.pneuma/` for storing its SQLite database and other types of data. Visit `localhost:8989` to register an admin user and perform operations like managing music files for others to stream.

By default, it embeds the admin dashboard UI from `./web/dist/` (if it exists). To exclude the UI, run `go build -tags no_embed -o build/bin/server ./cmd/server`.

#### Docker

```bash
docker build -t pneuma:latest .
docker run -d -p 8989:8989 pneuma
# use docker stop <container id> if you want to stop it
```

To build the server without the UI:

```bash
docker build -t pneuma:latest --build-arg EMBED_UI=false .
docker run -p 8989:8989 pneuma
```

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
