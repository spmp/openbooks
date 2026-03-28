# Configuration

All configuration options can be supplied by command line flags, and most can also be set via environment variables.
When both are provided, command line flags take precedence over environment variables.

## Environment Variables

### Global

- `OPENBOOKS_DEBUG`
- `OPENBOOKS_NAME`
- `OPENBOOKS_SERVER`
- `OPENBOOKS_TLS`
- `OPENBOOKS_LOG`
- `OPENBOOKS_SEARCHBOT`
- `OPENBOOKS_USERAGENT`

### Server/Desktop mode

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

### CLI mode

- `OPENBOOKS_DIR`

## Global Options

These options apply to both Server and CLI mode.

| Flag             | Default                   | Description                                                          |
|------------------|---------------------------|----------------------------------------------------------------------|
| `--debug`        | `false`                   | Display additional debug information, including all config values.   |
| `--help`/ `-h`   |                           | Display all commands and flags.                                      |
| `--log`/`-l`     | `false`                   | Save raw IRC logs for each client connection.                        |
| `--name`/`-n`    | **REQUIRED**              | Username used to connect to IRC server.                              |
| `--searchbot`    | `search`                  | The IRC search operator to use. Try `searchook` if `search` is down. |
| `--server`/`-s`  | `irc.irchighway.net:6697` | The IRC `server:port` to connect to.                                 |
| `--tls`          | `true`                    | Connect to IRC server over TLS.                                      |
| `--useragent/-u` | `OpenBooks v4.5.0`        | UserAgent / Version Reported to IRC Server.                          |

## Server Mode Options

| Flag                     | Default     | Description                                               |
|--------------------------|-------------|-----------------------------------------------------------|
| `--basepath`             | `/`         | Web UI Path. Must have trailing `/`. (Ex. `/openbooks/`)  |
| `--browser`/`-b`         | `false`     | Open the browser on startup.                              |
| `--dir`/`-d`             | `/books`[^1] | Directory where search results and eBooks are saved.      |
| `--no-browser-downloads` | `false`     | Don't send files to browser but save them to disk.        |
| `--post-download-hook`   |             | Executable path to run after a book download completes.   |
| `--post-download-hook-timeout` | `20`  | Seconds to wait before terminating post-download-hook.    |
| `--post-download-hook-workers` | `1`   | Maximum number of post-download-hook processes at once.   |
| `--assign-random-username-after` | `0` | Rotate IRC username after N searches + downloads (`0` disables). |
| `--persist`              | `false`     | Save eBook files after sending to browser.                |
| `--port`/`-p`            | `5228`      | The port that the server listens on.                      |
| `--rate-limit`/`-r`      | `10`        | Seconds to wait between IRC search requests. (minimum 10) |

`--post-download-hook` notes:

- The hook inherits container environment variables and receives these additional variables: `OPENBOOKS_FILE_PATH`, `OPENBOOKS_FILENAME`, `OPENBOOKS_AUTHOR`, and `OPENBOOKS_TITLE`.
- The hook is executed directly, so the configured value must be an executable path (it does not parse shell arguments).
- Hook failures or timeouts are logged and do not interrupt download delivery.

`--assign-random-username-after` notes:

- This option is mutually exclusive with `--name`.
- When enabled, OpenBooks generates a random initial username and rotates usernames every N search/download requests.
- Some IRC servers may treat frequent nickname changes as suspicious behavior. Use a larger N value for better stability.

## CLI Mode Options

| Flag         | Default           | Description                                          |
|--------------|-------------------|------------------------------------------------------|
| `--dir`/`-d` | Working Directory | Directory where search results and eBooks are saved. |

[^1]: `/books` works with the default Docker image out of the box.

## Server `/library` Endpoint

- `GET /library` returns the current files in the server download directory (`--dir`).
- This is a live filesystem view and can be used to inspect files that have not yet been processed by other tooling.
- Availability:
  - Enabled when `--persist` is true.
  - Also enabled when `--no-browser-downloads` is true, even if `--persist` is false.
- This endpoint reflects files currently on disk; it is not a browser-local history store.
