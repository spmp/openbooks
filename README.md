# openbooks

> NOTE: Going forward only the latest release will be supported. If you encounter any issues, be sure you are using the latest version.

[![Docker Pulls](https://img.shields.io/docker/pulls/evanbuss/openbooks.svg)](https://hub.docker.com/r/evanbuss/openbooks/)

Openbooks allows you to download ebooks from irc.irchighway.net quickly and easily.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./.github/home_v3_dark.png">
  <img alt="openbooks screenshot" src="./.github/home_v3.png">
</picture>


## Getting Started

### Binary

1. Download the latest release for your platform from the [releases page](https://github.com/evan-buss/openbooks/releases).
2. Run the binary
   - Linux users may have to run `chmod +x [binary name]` to make it executable
3. `./openbooks --help`
   - This will display all possible configuration values and introduce the two modes; CLI or Server.

### Docker

- Basic config
  - `docker run -p 8080:5228 evanbuss/openbooks`
- Config to persist all eBook files to disk
  - `docker run -p 8080:5228 -v /home/evan/Downloads/openbooks:/books evanbuss/openbooks --persist`

Docker image defaults:

- Server mode starts with `./openbooks server`.
- Default port is `5228`.
- Default download directory is `/books`.
- Environment variables can override defaults without needing a `command:` override.

### Setting the Base Path

OpenBooks server doesn't have to be hosted at the root of your webserver. The basepath value allows you to host it behind a reverse proxy. The base path value must have opening and closing forward slashes (default "/").

- Docker
  - `docker run -p 8080:80 -e BASE_PATH=/openbooks/ evanbuss/openbooks`
- Binary
  - `./openbooks server --basepath /openbooks/`

## Environment Variables

OpenBooks supports environment-variable configuration. CLI flags still take precedence.

### Global

- `OPENBOOKS_DEBUG`
- `OPENBOOKS_NAME`
- `OPENBOOKS_SERVER`
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
- `OPENBOOKS_ASSIGN_RANDOM_USERNAME_AFTER`
- `OPENBOOKS_NO_BROWSER_DOWNLOADS`
- `OPENBOOKS_PERSIST`
- `OPENBOOKS_BROWSER`
- `OPENBOOKS_BASEPATH` (legacy alias: `BASE_PATH`)

### CLI

- `OPENBOOKS_DIR`

## Post-Download Hook

Use `--post-download-hook` to run an executable after each completed server-side book download.

- Hook receives file path as first argument.
- Hook inherits container/process environment variables.
- OpenBooks also sets: `OPENBOOKS_FILE_PATH`, `OPENBOOKS_FILENAME`, `OPENBOOKS_AUTHOR`, `OPENBOOKS_TITLE`.
- Optional controls:
  - `--post-download-hook-timeout` (default `20` seconds)
  - `--post-download-hook-workers` (default `1`, queued execution)

Example:

- `./openbooks server --post-download-hook /opt/hooks/ebook.sh --post-download-hook-timeout 30 --post-download-hook-workers 2`

## Random Username Rotation

Use `--assign-random-username-after N` to rotate IRC usernames after every N searches+downloads.

- Mutually exclusive with `--name`.
- Generates a random initial username automatically when enabled.
- Use a larger N value. Some IRC servers may dislike frequent nickname changes.

## Usage

For a complete list of features use the `--help` flags on all subcommands.
For example `openbooks cli --help or openbooks cli download --help`. There are
two modes; Server or CLI. In CLI mode you interact and download books through
a terminal interface. In server mode the application runs as a web application
that you can visit in your browser.

Double clicking the executable will open the UI in your browser. In the future it may use [webviews](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) to provide a "native-like" desktop application. 

## Development

### Install the dependencies

- `go get`
- `cd server/app && npm install`
- `cd ../..`
- `go run main.go`

### Build the React SPA and compile binaries for multiple platforms.

- Run `./build.sh`
- This will install npm packages, build the React app, and compile the executable.

### Build the go binary (if you haven't changed the frontend)

- `go build`

### Mock Development Server

- The mock server allows you to debug responses and requests to simplified IRC / DCC
  servers that mimic the responses received from IRC Highway.
- ```bash
  cd cmd/mock_server
  go run .
  # Another Terminal
  cd cmd/openbooks
  go run . server --server localhost --log
  ```

### Desktop App
Compile OpenBooks with experimental webview support:

``` shell
cd cmd/openbooks
go build -tags webview
```


## Why / How

- I wrote this as an easier way to search and download books from irchighway.net. It handles all the extraction and data processing for you. You just have to click the book you want. Hopefully you find it much easier than the IRC interface.
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
