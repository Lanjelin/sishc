# sishc.sh

`sishc.sh` is a Bash script designed to manage [sish](https://docs.ssi.sh/getting-started) tunnels based on configurations specified in a YAML file. It provides features such as automatic tunnel management, logging, and configuration monitoring.

## Features

- **Automatic Tunnel Management**: Start and stop SSH tunnels based on configurations defined in a YAML file.
- **Configuration Monitoring**: Automatically reload tunnels when the configuration file changes.
- **Logging**: Log output to a specified log file with timestamps.
- **Input Validation**: Ensure that all required parameters are provided and valid before starting tunnels.
- **Colorized Output**: Optionally display colored output in the terminal for better visibility.

## Prerequisites

Before using `sishc.sh`, ensure you have the following installed:

- [Bash](https://www.gnu.org/software/bash/)
- [yq](https://github.com/mikefarah/yq) - A command-line YAML processor.
- [autossh](https://github.com/haifux/autossh) - A tool to automatically restart SSH sessions.

## Installation

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
## Docker

Create a config file at `./config/config.yaml` and edit it as specified below. Ensure that PUID and PGID is set as the same user that owns the config-file and the private key(s) used.

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
  #      - SISHC_OUTPUT_LOG="/config/sishc.log" # enable logging to file
      restart: on-failure:10
```

### docker cli

```bash
docker run --name sishc --rm -d -v ./config:/config -v ~/.ssh:/config/.ssh:ro -e TZ=Europe/Oslo -e PUID=${UID} -e PGID=${GID} ghcr.io/lanjelin/sishc:latest
```


## Configuration

Create a configuration file at `~/.config/sishc/config.yaml` with the following structure:

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
```

### Configuration Parameters

- `ssh_key`: Path to your SSH private key.
- `local_protocol`: Protocol to use for the local service (e.g., `http`, `https`).
- `local_host`: Hostname or IP address of the local service.
- `local_port`: Port number of the local service.
- `remote_port`: Port number on the remote server.
- `remote_server`: Hostname or IP address of the remote server.
- `tunnels`: A list of tunnel configurations, each with a unique `name`.

## Usage

Run the script to start managing your sish tunnels:

```bash
./sishc.sh
```

You can also run it in the background or as a service to keep your tunnels active.

## Logging

Logs are written to `~/.local/share/sishc/sishc.log` by default. You can change the log file location by setting the `SISHC_OUTPUT_LOG` environment variable.

## Color Output

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

