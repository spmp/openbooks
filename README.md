# openbooks

> NOTE: This is an agentically assisted fork building on the excellent work of [evanbuss](https://github.com/evan-buss/openbooks) and may be relevant until all changes are merged upstream. Until then, enjoy.

> NOTE: Going forward only the latest release will be supported. If you encounter any issues, be sure you are using the latest version.

Openbooks allows you to download ebooks from irc.irchighway.net quickly and easily.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./.github/home_v3_dark.png">
  <img alt="openbooks screenshot" src="./.github/home_v3.png">
</picture>

## Features

- Easily search and download books from our oldest Internet friend, IRC
- Download direct in browser, and/or persist books in the back end
- Supports multiple concurrent connections, useful if behind a proxy
- Can assign a random username every N actions, keep N large to avoid angering IRCHighway ;)
- Filter search results on author, title, and format, and sort by file size
- Search results and downloads persist in browser
- Support a post-download hook script to run on successful download
- In browser notifications for IRC connection, search acceptance, download comletion, and optional post-download hook status messages

### Bundled scripts

This release bundles a post-download hook script, [ebook resolve and move](https://github.com/spmp/ebook-resolve-move) to enhance book metadata and move to a finally library location, including hooks to trigger Kavita scans on successful move.

## Getting Started

The simplest way to get started is with Docker (or equivalent)

### Single/incidental use, _Desktop mode_

For useage just from your local machine, downloading from the browser:

```bash
docker run --rm -p 8080:5228 ghcr.io/spmp/openbooks
```

To persist the files do disk automatically (no browser download popup):

```bash
docker run --rm -p 8080:5228 -v /home/evan/Downloads/openbooks:/books ghcr.io/spmp/openbooks --persist
```

### Docker compose, _Server mode_

To integrate Openbooks into your media stack it is recommended to use `docker-compose`.
This example includes Docker _labels_ for Traefik (proxy) and Authelia (authentication),
downloads books into a staging location `/Media/EBooks-incoming` which the post-download hook script processes into `/Media/EBooks` and triggers a Kavita scan on move.

```yml
services:
  openbooks:
    image: ghcr.io/spmp/openbooks 
    container_name: openbooks
    user: 1000:1000
    environment:
      - OPENBOOKS_NO_BROWSER_DOWNLOADS=True
      - OPENBOOKS_PERSIST=True
      - OPENBOOKS_DIR=/Media/EBooks-incoming
      - OPENBOOKS_POST_DOWNLOAD_HOOK=ebook-resolve-move
      - OPENBOOKS_POST_DOWNLOAD_HOOK_TIMEOUT=45
      - OPENBOOKS_POST_DOWNLOAD_HOOK_WORKERS=1
      - OPENBOOKS_ASSIGN_RANDOM_USERNAME_AFTER=200
      - EBOOK_LIBRARY_ROOT=/Media/EBooks/
      - EBOOK_DRY_RUN=False
      - EBOOK_KAVITA_SCAN=True
      - EBOOK_KAVITA_URL=http://kavita:5000
      - EBOOK_KAVITA_API_KEY=XYZ
      - EBOOK_OVERWRITE_EXISTING=True 
    volumes:
      - /Media/:/Media/
    networks:
      - your-proxy-network
    labels:
      traefik.enable: true
      traefik.http.routers.openbooks.middlewares: auth@file
      traefik.http.routers.openbooks.entryPoints: https
    restart: unless-stopped
```
NOTE: The `EBBOK_...` env-vars are used by the `ebook-resolve-move` script.
    
The included [docker-compose.yml](docker-compose.yml) builds and runs a basic server mode for use/testing

### Binary

1. Download the latest release for your platform from the [releases page](https://github.com/spmp/openbooks/releases).
2. Run the binary
   - Linux users may have to run `chmod +x [binary name]` to make it executable
3. `./openbooks --help`
   - This will display all possible configuration values and introduce the two modes; CLI or Server.

## Run modes

Openbooks has a number of _modes_ of operation as:
Usage:
  openbooks \[flags\]
  openbooks \[command\]

Available Commands:
  cli         Run openbooks from the terminal in interactive CLI mode - default if no _command_ given
  completion  Generate the autocompletion script for the specified shell
  server      Run OpenBooks in server mode.

## Configuration

Configuration is via CLI options or environment variables. CLI options take precedence.
See [configuration.md](docs/docs/configuration.md) for more detail.

### CLI options

#### Global

```
      --debug              Enable debug mode.
  -l, --log                Save raw IRC logs for each client connection.
  -n, --name string        Username used to connect to IRC server.
      --searchbot string   The IRC bot that handles search queries. Try 'searchook' if 'search' is down. (default "search")
  -s, --server string      IRC server to connect to. (default "irc.irchighway.net:6697")
      --tls                Connect to server using TLS. (default true)
  -u, --useragent string   UserAgent / Version Reported to IRC Server. (default "OpenBooks 4.3.0")
```

#### Desktop/Server

```
      --assign-random-username-after int   Rotate to a random IRC username after N searches and downloads. Disabled when set to 0.
      --basepath string                    Base path where the application is accessible. For example "/openbooks/". (default "/"). Not needed if using a subdomain style proxy
  -b, --browser                            Open the browser on server start.
  -d, --dir string                         The directory where eBooks are saved when persist enabled. (default "/books")
  -h, --help                               help for server
      --no-browser-downloads               The browser won't recieve and download eBook files, but they are still saved to the defined 'dir' path.
      --persist                            Persist eBooks in 'dir'. Default is to delete after sending.
  -p, --port string                        Set the local network port for browser mode. (default "5228")
      --post-download-hook string          Executable path to run after a book download completes.
      --post-download-hook-timeout int     Seconds to wait before terminating post-download-hook. (default 20)
      --post-download-hook-workers int     Maximum number of post-download-hook processes running at once. (default 1)
  -r, --rate-limit int                     The number of seconds to wait between searches to reduce strain on IRC search servers. Minimum is 10 seconds. (default 10)
```

#### Completion

```
  bash        Generate the autocompletion script for bash
  fish        Generate the autocompletion script for fish
  powershell  Generate the autocompletion script for powershell
  zsh         Generate the autocompletion script for zsh
```

### Environment Variables

See above for meaning

#### Global

- `OPENBOOKS_DEBUG`
- `OPENBOOKS_NAME`
- `OPENBOOKS_ASSIGN_RANDOM_USERNAME_AFTER`
- `OPENBOOKS_SERVER` (server mode)
- `OPENBOOKS_TLS`
- `OPENBOOKS_LOG`
- `OPENBOOKS_SEARCHBOT`
- `OPENBOOKS_USERAGENT`

### Server/Desktop

- `OPENBOOKS_PORT`
- `OPENBOOKS_RATE_LIMIT`
- `OPENBOOKS_DIR`
- `OPENBOOKS_POST_DOWNLOAD_HOOK`
- `OPENBOOKS_POST_DOWNLOAD_HOOK_TIMEOUT`
- `OPENBOOKS_POST_DOWNLOAD_HOOK_WORKERS`
- `OPENBOOKS_NO_BROWSER_DOWNLOADS`
- `OPENBOOKS_PERSIST`
- `OPENBOOKS_BROWSER`
- `OPENBOOKS_BASEPATH`

### CLI

- `OPENBOOKS_DIR`

## Post-Download Hook

Use `--post-download-hook` to run an executable after each completed server-side book download.

- Hook receives file path as first argument.
- Hook inherits container/process environment variables.
- Openbooks also sets: `OPENBOOKS_FILE_PATH`, `OPENBOOKS_FILENAME`, `OPENBOOKS_AUTHOR`, `OPENBOOKS_TITLE`.
- Optional controls:
  - `--post-download-hook-timeout` (default `20` seconds)
  - `--post-download-hook-workers` (default `1`, queued execution)

Hook script notifications:

- Hook output is parsed when script execution completes.
- To send popup notifications, print lines in this format:
  - `OPENBOOKS_NOTIFY {"level":"info|warning|error","title":"...","detail":"..."}`
- Timeout behavior:
  - If hook exceeds timeout, Openbooks terminates it and sends an error notification.

Example:

- `./openbooks server --post-download-hook /opt/hooks/ebook.sh --post-download-hook-timeout 30 --post-download-hook-workers 2`

Detailed docs (interface, output format, Docker + Python venv setup, packaging hooks from git) at [post-download-hooks.md](docs/docs/post-download-hooks.md)

## Random Username Rotation

Use `--assign-random-username-after N` to rotate IRC usernames after every N searches+downloads.

- Mutually exclusive with `--name`.
- Generates a random initial username automatically when enabled.
- Use a larger N value. Some IRC servers may dislike frequent nickname changes.

## Development

### Docker

Build and test is most simply achieved via the included `docker-compose.yml` as:

```bash
docker compose build
docker compose up
```

Browse to [http://localhost:5228](http://localhost:5228)

### Otherwise

#### Install the dependencies

- `go get`
- `cd server/app && npm install`
- `cd ../..`
- `go run main.go`

#### Build the React SPA and compile binaries for multiple platforms.

- Run `./build.sh`
- This will install npm packages, build the React app, and compile the executable.

#### Build the go binary (if you haven't changed the frontend)

- `go build`

#### Mock Development Server

- The mock server allows you to debug responses and requests to simplified IRC / DCC
  servers that mimic the responses received from IRC Highway.
- ```bash
  cd cmd/mock_server
  go run .
  # Another Terminal
  cd cmd/openbooks
  go run . server --server localhost --log
  ```

#### Desktop App

Compile OpenBooks with experimental webview support:

```shell
cd cmd/openbooks
go build -tags webview
```

## Why / How

- I (Evan Buss) wrote this as an easier way to search and download books from irchighway.net. It handles all the extraction and data processing for you. You just have to click the book you want. Hopefully you find it much easier than the IRC interface.
- It was also interesting to learn how the [IRC](https://en.wikipedia.org/wiki/Internet_Relay_Chat) and [DCC](https://en.wikipedia.org/wiki/Direct_Client-to-Client) protocols work and write custom implementations.

## Technology

- Backend
  - Golang
  - Chi
  - gorilla/websocket
  - Archiver (extract files from various archive formats)
- Frontend
  - React.js
  - TypeScript
  - Redux / Redux Toolkit
  - Mantine UI / @emotion/react
  - Framer Motion
  - 
