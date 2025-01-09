# SISHC Tunnel Manager

SISHC Tunnel Manager is a lightweight, web app for managing [sish tunnels](https://docs.ssi.sh/).
The app enables you to add, edit, delete, and monitor SSH tunnels conveniently through a user-friendly interface.
The project is built using Flask, Bulma, and Codemirror.

<div align="center">
  <a href="https://github.com/Lanjelin/sishc/blob/main/.github/sishc.png">
    <img src="https://raw.githubusercontent.com/Lanjelin/sishc/refs/heads/main/.github/sishc.png" title="screenshot" width="450" />
  </a>
</div>

## Features

- **Add New Tunnels**: Create SSH tunnels with configurable local and remote settings.
- **Edit Configurations**: Update global and individual tunnel configurations via a streamlined interface.
- **Manage Tunnels**: Edit raw configurations directly or delete tunnels when no longer needed.
- **View Logs**: Access logs for individual tunnels or view aggregated logs.
- **CLI update supported**: Tunnels with be updated when a change is detected in the config-file.

## Docker

Ensure that PUID and PGID is set as the same user that owns the config-dir and the private key(s) used.
After starting the container, access the web ui at port 5000, eg. `http://127.0.0.1:5000`

### docker compose

```yaml
services:
  sishc:
    image: ghcr.io/lanjelin/sishc:latest
    container_name: sishc
    volumes:
      - ./config:/config
      - ~/.ssh:/config/.ssh:ro
    environment:
      - TZ=Europe/Oslo
      - PUID=1000 # defaults to 1000
      - PGID=1000 # defaults to 1000
    #      - USE_COLOR=false # toggle color in logs
    #      - SISHC_OUTPUT_LOG="/config/sishc.log" # change log path
    ports:
      - 5000:5000
    restart: on-failure:10
```

### docker cli

```bash
docker run --name sishc --rm -d -v ./config:/config -v ~/.ssh:/config/.ssh:ro -e TZ=Europe/Oslo -e PUID=${UID} -e PGID=${GID} -p 5000:5000 ghcr.io/lanjelin/sishc:latest
```

## Configuration

The configuration file at `~/.config/sishc/config.yaml` should have the following structure:

```yaml
# Global Configuration
ssh_key: "~/.ssh/id_rsa"
local_protocol: "http"
local_host: "localhost"
local_port: 8080
remote_port: 2222
remote_server: "example.com"
# Tunnel Specific Configurations
tunnels:
  - name: "first_tunnel"
    local_protocol: "http"
    local_host: "localhost"
    local_port: 8080
    remote_port: 2222
    remote_server: "example.com"
  - name: "second_tunnel"
    local_protocol: "https"
    local_port: 4433
    disabled: True
  - name: "2512" # Expose ssh port to example.com:2512
    local_host: 192.168.1.101
    local_port: 22
    local_protocol: tcp
```

### Configuration Parameters

- `ssh_key`: Path to your SSH private key.
- `local_protocol`: Protocol to use for the local service (e.g., `http`, `https`).
- `local_host`: Hostname or IP address of the local service.
- `local_port`: Port number of the local service.
- `remote_port`: Port number on the remote server.
- `remote_server`: Hostname or IP address of the remote server.
- `tunnels`: A list of tunnel configurations, each with a unique `name`.

## How do I configure sish for this?

I've attached an example as how I run sish in `docker-compose-sish-example.yaml`, for full instructions, see the [docs](https://docs.ssi.sh/getting-started#docker-compose).

## Running outside Docker

Before using `sishc.sh`, ensure you have the following installed:

- [Bash](https://www.gnu.org/software/bash/)
- [yq](https://github.com/mikefarah/yq) - A command-line YAML processor.
- [autossh](https://github.com/haifux/autossh) - A tool to automatically restart SSH sessions.

Requirements for `web.py` is listed in `requiments.txt`, if you want to use the web frontend.

### Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/Lanjelin/sishc.git
   cd sishc
   ```

2. Make the script executable:

   ```bash
   chmod +x sishc.sh
   ```

3. (Optional) Move the script to a directory in your PATH for easier access:

   ```bash
   mv sishc.sh /usr/local/bin/sishc
   ```

### Usage

Run the script to start managing your sish tunnels:

```bash
./sishc.sh
```

You can also run it in the background or as a service to keep your tunnels active.

### Logging

Logs are written to `~/.local/share/sishc/sishc.log` by default. You can change the log file location by setting the `SISHC_OUTPUT_LOG` environment variable.

### Color Output

By default, the script uses colored output. You can disable this by running the script with the `--no-color` flag:

```bash
./sishc.sh --no-color
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for any bugs or feature requests.

## License

This project is licensed under the GPL-3.0 License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [sish](https://docs.ssi.sh/) for the tunnel software itself.
- [autossh](https://github.com/haifux/autossh) for maintaining SSH tunnels.
- [yq](https://github.com/mikefarah/yq) for YAML processing.

## Nota Bene

This README was mostly written by GPT-4o

<div align="center">
  <a href="https://imgur.com/k4VWmn7">
    <img src="https://user-images.githubusercontent.com/74038190/216644507-4f06ea29-bf55-4356-aac0-d42751461a9d.gif" title="source: imgur.com" width="150" />
  </a>
</div>
