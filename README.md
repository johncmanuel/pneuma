# pneuma

pneuma is an open-source, self-hostable, and local-first music project, designed to give a Spotify-like experience. It is composed of a desktop application for local music playback, a server for music storage and streaming, and a web player for accessible music playback.

<!-- TODO: add demo as mp4 -->

Public demo: https://pneuma.johncarlomanuel.com/

Register an account, and play a couple of songs from the [Library of Congress](https://www.loc.gov/) in the web! The songs can also be streamed on the desktop application.

## Screenshots

### Desktop application

![Screenshot 1](.github/imgs/Screenshot_20260319_105737.webp)
![Screenshot 2](.github/imgs/Screenshot_20260319_110300.webp)
![Screenshot 3](.github/imgs/Screenshot_20260319_113205.webp)

> Web player is aesthetically identical to the desktop application.

### Admin server interface

![Screenshot 4](.github/imgs/Screenshot_20260319_113102.webp)

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

## Metadata Structure

pneuma supports the following metadata for each individual track.

1. Title
2. Artist
3. Album
4. Album Artist
5. Track Number
6. Disc Number
7. Duration
8. Album Artwork
9. Sample Rate
10. Bitrate
11. Genre

## Getting Started

Install the following:

1. [Go](https://go.dev/) 1.24+
2. [Bun](https://bun.sh/)
3. [Wails](https://wails.io/docs/gettingstarted/installation)
4. [sqlc](https://docs.sqlc.dev/en/stable/overview/install.html)
5. (OPTIONAL): [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) CLI tool
6. [Docker](https://www.docker.com/)

For Docker, ensure you have Docker's [BuildX](https://github.com/docker/buildx) installed. Run the following to verify it is installed:

```bash
docker buildx version
```

### Setting up the environment

Run to set up frontend dependencies:

```bash
bun install
```

### Running the desktop application

Run `wails dev` to start the desktop application in development mode. By default, it runs both the frontend and the Go process. It will also install frontend dependencies if not already installed.

Run `wails build` to build the desktop application. The output executable will be `build/bin/pneuma` (or whatever executable your OS supports).

Desktop data is profile-aware:

- `wails dev` uses the `dev` profile by default and stores data under `${OS_CONFIG}/pneuma-dev/`
- `wails build` executables use the `prod` profile by default and store data under `${OS_CONFIG}/pneuma/`

For cache data (thumbnail artwork), the app uses `${OS_CACHE}/pneuma-dev/` for `dev` and `${OS_CACHE}/pneuma/` for `prod`.

You can override profile detection at runtime with `PNEUMA_DESKTOP_PROFILE=dev` or `PNEUMA_DESKTOP_PROFILE=prod`.

See [os.UserConfigDir()](https://pkg.go.dev/os#UserConfigDir) and [os.UserCacheDir()](https://pkg.go.dev/os#UserCacheDir) for OS-specific directory locations.

##### Linux Runtime Requirements

On Linux, the desktop application requires GStreamer plugins for audio playback. This addresses the error: `GStreamer element autoaudiosink not found`. Install it using your distribution's package manager:

```bash
# Debian/Ubuntu
sudo apt-get install gstreamer1.0-plugins-good

# Fedora
sudo dnf install gstreamer1-plugins-good

# Arch Linux
sudo pacman -S gst-plugins-good
```

Source: https://wails.io/docs/guides/linux/#gstreamer-error-when-using-audio-or-video-elements

### Running the server

To run the server, run:

```bash
# this will compile web/ and dashboard/, embed them into the server binary,
# and run the server
bun run server
```

Upon first start, the server will create a directory `${HOME}/.pneuma/` for storing its SQLite database and other types of data. Visit `localhost:8989` to register an admin user and perform operations like managing music files for others to stream.

#### Docker

Then run the commands below.

```bash
# build normally
docker build -t pneuma:latest .

# or if you want to be more explicit with the platform:
# supported OS/arch:
# 1. linux/amd64
# 2. linux/arm64
docker build --platform <placeholder> -t pneuma-server .

docker run -d -p 8989:8989 pneuma
# use docker stop <container id> if you want to stop it
```

To build the server without the UI (for faster builds for testing):

```bash
docker build -t pneuma:latest --build-arg EMBED_UI=false .
docker run -p 8989:8989 pneuma
```

Some useful Docker sanity check methods:

```bash
# is the container running?
docker ps

# review logs
docker logs -f <container id>

# test if container can be reached
curl http://localhost:8989 # or whatever port you set it to

# access container filesystem
docker exec -it <container id> /bin/sh
```

##### Docker Compose (for the server)

```yaml
services:
  server:
    image: ghcr.io/johncmanuel/pneuma/server:latest
    container_name: pneuma-server
    restart: unless-stopped
    ports:
      - "8989:8989"
    volumes:
      # Persistent application data (database, cached artwork, uploads, etc.)
      - pneuma_data:/data

      # Mount your music directory (read-only recommended)
      # Replace `./music` with the actual path to your local music directory
      - ./music:/music:ro

    environment:
      # Core configuration
      - PNEUMA_SERVER_HOST=0.0.0.0
      - PNEUMA_DATA_DIR=/data

      # Point the music scanner to the mounted volume
      - PNEUMA_LIBRARY_WATCH_FOLDERS=/music

      # Full rescan cadence in minutes with 120 as the default, but increase for low-power devices
      # - PNEUMA_LIBRARY_SCAN_INTERVAL_MINUTES=240

      # Security (this'll be auto generated if not provided and placed in the config file)
      # - PNEUMA_AUTH_SECRET_KEY=change-this-to-a-secure-random-string

      # Rate limiting (defaults to true)
      # - PNEUMA_RATE_LIMITING_ENABLED=true

      # Increase upload limit if needed (500 MB is default)
      # - PNEUMA_UPLOAD_MAX_SIZE_MB=500

volumes:
  pneuma_data:
```

##### Test in an HTTPS environment

There are a handful of ways to test HTTPS locally. One way is to use [tailscale](https://tailscale.com/) to enable testing across not just the current machine, but through other devices, including mobile. [Configure HTTPS](https://tailscale.com/kb/1153/enabling-https) for your tailnet.

1. Build and start the staging stack:

```bash
docker compose -f .staging/docker-compose.staging.yml \
  --project-directory .staging \
  up --build -d
```

2. Expose the app to your tailnet:

```bash
sudo tailscale serve http://0.0.0.0:8989
```

3. Access from another device via the designated:

```text
https://<hostname>.<your tailnet domain>
```

4. Stop staging when done:

```bash
docker compose -f .staging/docker-compose.staging.yml \
  --project-directory .staging \
  down

# ctrl+c to exit tailscale serve
```

Notes:

- Always use `/player/` (with trailing slash) for proper service worker scope.
- If using `tailscale serve` for production, use `tailscale serve --bg http://0.0.0.0:8989` (or whatever port you're using). Use `tailscale serve --https=443 off` to turn it off if needed.

### Formatting

Run `bun fmt` to format TypeScript, Svelte, and Go code.

### Linting

Run `bun lint` to lint TypeScript, Svelte, and Go code.

Run `bun knip` to check for unused TypeScript and Svelte code, dependencies, and exports.

### Running sqlc

sqlc is used to generate Go code from SQL queries.

Add SQL query files under `internal/store/sqlite/<desktop or server>/query/`.

Once done so, run `sqlc generate` to generate the Go code equivalent of the queries. The generated code will be placed under `internal/store/sqlite/<desktop or server>db/` with the file extension `.sql.go`. They can be imported from the the package, `<desktop or server>db`.

The config file, `sqlc.yaml` is found at the root.

### Database Migrations

Create new migration SQL files under `internal/store/sqlite/<desktop or server>/migrations/`.

Run `go run ./cmd/dbmigrate up` to apply all pending migrations.  
Run `go run ./cmd/dbmigrate down [N]` to roll back N steps (default 1).  
Run `go run ./cmd/dbmigrate force <version>` to force schema version and clear the dirty flag.  
Run `go run ./cmd/dbmigrate version` to print current version and dirty status.

If wanted, use [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)'s CLI tool to do this instead. The custom CLI in `/cmd/dbmigrate` is a wrapper over the Go library version; it is for those that don't want to install another external tool.

## FAQ

### Will there be a mobile version?

Eventually, yes. There are three options I'm looking at: wait for Wails to support mobile platforms, optimize the web player to be a [progressive web app (PWA)](https://en.wikipedia.org/wiki/Progressive_web_app), or look into tools like [Capacitor](https://capacitorjs.com/) for building mobile applications with Svelte support.
