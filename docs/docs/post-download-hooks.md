# Post-Download Hooks

Post-download hooks let you run external automation after a server-side book download completes.

Typical use cases:

- Move files into a library structure.
- Normalize metadata.
- Trigger external tooling (Calibre, indexers, notifications).

## Overview

- Hook runs only for completed server-side book downloads.
- Hook does not run for browser-only fetches that do not perform a server-side download.
- Hook failures do not block the OpenBooks backend.
- Hook execution is queued/concurrency-limited by `--post-download-hook-workers`.

## Configuration

- `--post-download-hook <path>`: executable/script path.
- `--post-download-hook-timeout <seconds>`: kill script if it exceeds timeout (default `20`).
- `--post-download-hook-workers <n>`: max concurrent hook processes (default `1`).

Environment equivalents:

- `OPENBOOKS_POST_DOWNLOAD_HOOK`
- `OPENBOOKS_POST_DOWNLOAD_HOOK_TIMEOUT`
- `OPENBOOKS_POST_DOWNLOAD_HOOK_WORKERS`

## Hook Interface

OpenBooks executes the hook directly (no shell parsing).

- Command invocation: `<hook_path> <file_path>`
- `argv[1]` is the downloaded file path.

Hook environment includes process/container environment plus:

- `OPENBOOKS_FILE_PATH`
- `OPENBOOKS_FILENAME`
- `OPENBOOKS_AUTHOR`
- `OPENBOOKS_TITLE`

## Hook Output Notifications

Hook output is parsed **after the hook process finishes** (not streamed live).

To send UI popups, print one or more lines with this prefix:

`OPENBOOKS_NOTIFY ` followed by JSON:

```text
OPENBOOKS_NOTIFY {"level":"info","title":"Processed","detail":"Metadata updated"}
OPENBOOKS_NOTIFY {"level":"warning","title":"Skipped","detail":"Missing ISBN"}
OPENBOOKS_NOTIFY {"level":"error","title":"Failed","detail":"Move failed"}
```

Fields:

- `level`: `info`, `warning`, `error` (`warn`/`err` are also accepted)
- `title`: short message title
- `detail`: optional detail text

Malformed notification lines are ignored.

## Timeout Behavior

- If hook runtime exceeds `--post-download-hook-timeout`, OpenBooks terminates the process.
- A UI error notification is sent indicating script + file were terminated due to timeout.
- Any hook output collected before termination is still logged and parsed.

## Python Example (Docker)

OpenBooks Docker image includes Python and bash.

### 1) Mount script directory

```yaml
services:
  openbooks:
    image: evanbuss/openbooks:latest
    volumes:
      - /srv/openbooks/books:/books
      - /srv/openbooks/scripts:/scripts
    environment:
      - OPENBOOKS_POST_DOWNLOAD_HOOK=/scripts/ebook-hooks/process.py
      - OPENBOOKS_POST_DOWNLOAD_HOOK_TIMEOUT=30
      - OPENBOOKS_POST_DOWNLOAD_HOOK_WORKERS=1
```

### 2) Build venv from inside running container (recommended)

This ensures venv Python matches container Python.

```bash
docker exec -it openbooks bash
cd /scripts/ebook-hooks
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
```

### 3) Verify interpreter path

Inside container, confirm venv interpreter resolves correctly:

```bash
readlink -f /scripts/ebook-hooks/.venv/bin/python3
```

Expected target should be container Python path (typically `/usr/local/bin/python3`).

### 4) Script shebang

Use the venv interpreter in shebang:

```python
#!/scripts/ebook-hooks/.venv/bin/python3
```

## Packaging Hook Scripts From Remote Git

If scripts come from a remote repository, package them in a derived image so startup is deterministic.

Example pattern:

```dockerfile
FROM evanbuss/openbooks:latest

RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*

WORKDIR /scripts
RUN git clone https://github.com/example/ebook-hooks.git
RUN python3 -m venv /scripts/ebook-hooks/.venv \
 && /scripts/ebook-hooks/.venv/bin/pip install -r /scripts/ebook-hooks/requirements.txt

ENV OPENBOOKS_POST_DOWNLOAD_HOOK=/scripts/ebook-hooks/process.py
```

Notes:

- Prefer building dependencies at image build time instead of runtime pull/init.
- Keep hook path explicit; OpenBooks does not do shell-based PATH lookup/parsing.
- If you need command wrappers, make wrapper executable and point `--post-download-hook` to it.
