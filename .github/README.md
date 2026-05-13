# SISHC

`sishc` is a daemon-first CLI for managing [sish](https://docs.ssi.sh/) tunnels.
It reads a config file as source of truth, keeps tunnels reconciled in the background, and exposes a Unix socket for `status` and `reconcile`.

Runtime dependencies:
- `autossh`
- `ssh`

## What it does

- runs tunnels as a long-lived daemon
- keeps one daemon per config file
- manages tunnel config through the CLI
- writes daemon events to `daemon.log`
- writes per-tunnel logs with rotation
- supports temporary `oneoff` tunnels without touching config

## Commands

```text
sishc daemon
sishc status [--verbose] [<name>]
sishc logs [--tail N] [--follow] <name|daemon>
sishc validate
sishc reconcile
sishc add [flags] <name> [<local_host>][:<local_port>]
sishc update [flags] [--new-name NAME] <name> [<local_host>][:<local_port>]
sishc remove <name>
sishc start <name>
sishc stop <name>
sishc oneoff [flags] [<name>] [<local_host>:]<local_port>
sishc init [--config PATH]
```

### Tunnel flags

```text
--ssh-key PATH
--remote-port PORT
--remote-server HOST
--local-protocol tcp|https
```

### Notes

- `add` and `update` accept shorthand host/port forms and use globals when fields are omitted
- `update` uses `--new-name` for rename
- `start` and `stop` toggle `enabled`
- `oneoff` prints the remote server output directly and does not write config
- `status` can show one tunnel in detail
- `logs --follow` follows rotated log files

## Config

Default config path:

```text
~/.config/sishc/config.yaml
```

Useful environment variables:

- `SISHC_CONFIG_FILE`
- `SISHC_LOG_DIR`
- `SISHC_SOCKET`

Example:

```yaml
ssh_key: "~/.ssh/id_rsa"
remote_port: 1433
remote_server: sish.example.com
local_host: caddy
local_port: 80

tunnels:
  - name: test1.example.com
  - name: test2
    enabled: false
    local_host: example_host
  - name: test3
    local_port: 1443
  - name: test4
    local_host: example_host2
    local_port: 3000
    ssh_key: "~/.ssh/id_rsa2"
    remote_server: sish2.example.com
    remote_port: 1723
  - name: "2512" # Expose ssh port to example.com:2512
    local_host: 192.168.50.80
    local_port: 22
    local_protocol: tcp
```

## Logs

- `daemon.log` contains daemon lifecycle messages
- tunnel logs live next to it, one file per tunnel
- logs rotate by size

## Web UI

The daemon can start the web UI automatically when these config keys are set:

```yaml
web_enabled: true
web_listen: 127.0.0.1:5000
```

If `web_enabled` is false or omitted, the daemon runs tunnels only.
