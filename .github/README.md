# SISHC

`sishc` is a small daemon-first CLI for managing [sish](https://docs.ssi.sh/) tunnels.

It keeps a config file as the source of truth, runs tunnels in the background, and exposes a web UI when enabled.

The binary is called `sishc`. The release image is `ghcr.io/lanjelin/sishc:latest`.

## Quick start

Docker is the easiest way to run it.

1. Put your SSH key somewhere under `/config`, for example `/config/id_rsa`.
2. If you already have a `config.yaml`, place it at `/config/config.yaml`.
3. Start the container.
4. If the config is empty, the container creates a minimal config.
5. Open the web UI at `http://yourhost:5000` to complete the setup.

```bash
docker run \
  -e PUID=$(id -u) \
  -e PGID=$(id -g) \
  -v "$(pwd)/config:/config" \
  -p 5000:5000 \
  ghcr.io/lanjelin/sishc:latest
```

If you want the local `sishc` binary to talk to the daemon inside Docker:

```bash
export SISHC_CONFIG_FILE=/path/to/config/config.yaml
export SISHC_LOG_DIR=/path/to/config/logs
export SISHC_SOCKET=/path/to/config/sishc.sock
sishc status
```

## Screenshots

<table>
  <tr>
    <td><img src="./screenshots/sishc.png" alt="Dashboard"></td>
    <td><img src="./screenshots/config.png" alt="Config"></td>
  </tr>
  <tr>
    <td><img src="./screenshots/tunnels_new.png" alt="Add tunnel"></td>
    <td><img src="./screenshots/logs_sishc.png" alt="Logs"></td>
  </tr>
</table>

## What it does

- keeps tunnels alive and reconnects on connection drops
- manages tunnel config through the CLI or the web UI
- supports `status`, `logs`, `add`, `update`, `remove`, `start`, `stop`, and `oneoff`
- writes per-tunnel logs with rotation
- can start the web UI from config
- quickly expose local endpoints for testing using `sishc o <port>`

## Commands

```text
sishc daemon     Run the tunnel daemon
sishc status, ls Show tunnel status
sishc logs       Show tunnel logs
sishc validate   Validate config and exit
sishc reconcile  Reconcile config now
sishc add, a     Add a tunnel
sishc update, u  Update a tunnel
sishc remove, rm Remove a tunnel
sishc start      Enable a tunnel
sishc stop       Disable a tunnel
sishc oneoff, o  Run a temporary tunnel
sishc init       Create config interactively
```

Helpful flags:

```text
--ssh-key PATH
--remote-port PORT
--remote-server HOST
--local-protocol http|https|tcp
```

## CLI examples

```bash
sishc add test229 localhost
```

Uses the global local port and only overrides the host.

```bash
sishc update test229 :9090
```

Updates only the local port. The host stays as-is.

```bash
sishc ls test229
```

Shows one tunnel instead of the full table.

```bash
sishc logs --follow test229
```

Follows the tunnel log live.

```bash
sishc o --local-protocol https cockpit.example.com 192.0.2.10:9090
```

One-off HTTPS tunnel to a remote host.

## Defaults and paths

```text
Config: XDG_CONFIG_HOME/sishc/config.yaml or ~/.config/sishc/config.yaml
Logs:   XDG_DATA_HOME/sishc/logs or ~/.local/share/sishc/logs
Socket: XDG_RUNTIME_DIR/sishc.sock or XDG_DATA_HOME/sishc/sishc.sock
        or ~/.local/share/sishc/sishc.sock
```

Useful environment variables:

- `SISHC_CONFIG_FILE`
- `SISHC_LOG_DIR`
- `SISHC_SOCKET`
- `SISHC_KNOWN_HOSTS`

## Server side

For the server side that `sishc` connects to, see:

- https://github.com/Lanjelin/sish-starter

## Install

### Download a release

Grab the `sishc` binary from GitHub Releases and put it on your `PATH`.

### Build locally

```bash
go build ./cmd/sishc
```

### Run from source

```bash
go run ./cmd/sishc
```

## About this rewrite

This is a Codex-assisted rewrite of the original bash+python version:

- https://github.com/Lanjelin/sishc/tree/22ea2b3b55dc6ec9d120b19588c6a9ca740f7fa8
