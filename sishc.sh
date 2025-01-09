#!/bin/bash

# Respect XDG Base Directory Specification and allow overrides via environment variables
CONFIG_FILE="${SISHC_CONFIG_FILE:-${XDG_CONFIG_HOME:-$HOME/.config}/sishc/config.yaml}"
OUTPUT_LOG="${SISHC_OUTPUT_LOG:-${XDG_DATA_HOME:-$HOME/.local/share}/sishc/sishc.log}"

# Color settings
USE_COLOR="${USE_COLOR:-true}"
for arg in "$@"; do
  if [[ "$arg" == "--no-color" ]]; then
    USE_COLOR=false
  fi
done

# Verify the existence of config and log files
verify_files() {

  # Create necessary directories if they do not exist
  mkdir -p "$(dirname "$CONFIG_FILE")"
  mkdir -p "$(dirname "$OUTPUT_LOG")"

  # Create the log file if it does not exist
  if [[ ! -f "$OUTPUT_LOG" ]]; then
    touch "$OUTPUT_LOG"
  fi

  # Create the config file if it does not exist
  if [[ ! -s "$CONFIG_FILE" ]]; then
    colored_echo "31" "ERROR: Configuration file is empty. Please edit it at $CONFIG_FILE" | output_handler
    touch "$CONFIG_FILE"
    # exit 1
  fi
}

declare -A running_tunnels
declare -A previous_configs

# Function to handle output
output_handler() {
  truncate_logs # Check if we need to truncate the log
  while read -r line; do
    echo -e "$(date +'%Y-%m-%d %H:%M:%S') - $line" >>"$OUTPUT_LOG"
    echo -e "$(date +'%Y-%m-%d %H:%M:%S') - $line"
  done
}

# Function to validate inputs for starting a tunnel
validate_inputs() {
  local name="$1"
  local ssh_key="$2"
  local local_proto="$3"
  local local_host="$4"
  local local_port="$5"
  local remote_port="$6"
  local remote_server="$7"

  # Check if all required parameters are provided
  if [[ -z "$name" || -z "$ssh_key" || -z "$local_host" || -z "$local_port" || -z "$remote_port" || -z "$remote_server" || -z "$local_proto" ]]; then
    colored_echo "31" "ERROR: Missing required parameters for tunnel '$name'." | output_handler
    return 1
  fi

  # Check if the local port is a valid number
  if ! [[ "$local_port" =~ ^[0-9]+$ ]] || ((local_port < 1 || local_port > 65535)); then
    colored_echo "31" "ERROR: Invalid local port '$local_port' for tunnel '$name'. It must be a number between 1 and 65535." | output_handler
    return 1
  fi

  # Check if the remote port is a valid number
  if ! [[ "$remote_port" =~ ^[0-9]+$ ]] || ((remote_port < 1 || remote_port > 65535)); then
    colored_echo "31" "ERROR: Invalid remote port '$remote_port' for tunnel '$name'. It must be a number between 1 and 65535." | output_handler
    return 1
  fi

  # Check if local_host is a valid hostname or IP address
  if ! [[ "$local_host" =~ ^[a-zA-Z0-9._-]+$ || "$local_host" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    colored_echo "31" "ERROR: Invalid local host '$local_host'. It must be a valid hostname or IP address." | output_handler
    return 1
  fi

  # Check if remote_server is a valid hostname or IP address
  if ! [[ "$remote_server" =~ ^[a-zA-Z0-9._-]+$ || "$remote_server" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    colored_echo "31" "ERROR: Invalid remote server '$remote_server'. It must be a valid hostname or IP address." | output_handler
    return 1
  fi

  # Check if the SSH key file exists
  if [[ ! -f "$ssh_key" ]]; then
    colored_echo "31" "ERROR: SSH key file '$ssh_key' does not exist for tunnel '$name'." | output_handler
    return 1
  fi

  return 0
}

# Function to start an SSH tunnel
start_tunnel() {
  local name="$1"
  local ssh_key="$2"
  local local_proto="$3"
  local local_host="$4"
  local local_port="$5"
  local remote_port="$6"
  local remote_server="$7"

  # Prepend default SSH key directory if we don't have a valid path
  if [[ "$ssh_key" != /* ]]; then
    ssh_key="${HOME}/.ssh/$ssh_key"
  fi

  # Validate inputs before starting the tunnel
  if ! validate_inputs "$name" "$ssh_key" "$local_proto" "$local_host" "$local_port" "$remote_port" "$remote_server"; then
    return 1
  fi

  echo "INFO: Starting tunnel: $name -> $local_proto"://$local_host:$local_port | output_handler

  # Specify the protocol according to sish docs
  if [[ "$local_proto" == "https" ]]; then
    local_proto=443
  elif [[ "$local_proto" == "tcp" ]]; then
    # Might not be able to get this to work, defaulting to http for now
    # local_proto=$local_port
    local_proto=80
  else
    local_proto=80
  fi

  {
    AUTOSSH_POLL=10 AUTOSSH_GATETIME=5 autossh -M 0 -o ServerAliveInterval=10 -o ServerAliveCountMax=3 -o StrictHostKeyChecking=no \
      -T -i "$ssh_key" -p "$remote_port" -R "$name:$local_proto:$local_host:$local_port" "$remote_server" \
      2>&1 | while read -r line; do
      if ! [[ -n $(echo "$line" | grep "Press Ctrl-C") ||
      -n $(echo "$line" | grep '^[[:space:]]*$') ||
      -n $(echo "$line" | grep "Starting SSH Forwarding service") ||
      -n $(echo "$line" | grep ": http://$name") ]]; then
        if [[ -n $(echo "$line" | grep ": https://$name") ]]; then
          url=$(echo "$line" | grep -o 'http[s]\?://[^ ]\+') #P '(?<=https://).*')
          echo "$(colored_echo '32' 'INFO: Tunnel '$name' started successfully. Access it at') $(colored_echo '34' ''$url'')" | output_handler
        else
          if [[ -n $(echo "$line" | sed -r 's/\x1B\[[0-9;]*m//g' | grep -E "\| ($name+(\.[a-zA-Z0-9-]+)*) \| [1-5][0-9][0-9] \|") ]]; then
            echo "$line" | sed -E 's/^[0-9]{4}\/[0-9]{2}\/[0-9]{2} - [0-9]{2}:[0-9]{2}:[0-9]{2} \| /LOG: /' | output_handler
          else
            colored_echo "33" "WARNING: $name: $line" | output_handler
          fi
        fi
      fi
    done
  } &

  # Store the PID of the tunnel (subshell really....)
  running_tunnels["$name"]=$!
}

# Function to stop a specific tunnel
stop_tunnel() {
  local name="$1"
  if [[ -n "${running_tunnels[$name]}" ]]; then
    echo "INFO: Stopping tunnel: $name" | output_handler

    # Attempt to gracefully stop the tunnel
    kill $(pgrep -f "autossh.*$name\:") 2>/dev/null
    kill $(pgrep -f "/usr/bin/ssh.*$name\:") 2>/dev/null

    # Wait for 3 seconds to allow the processes to terminate
    sleep 3

    # If the tunnel is still running, force it to stop
    if [[ -n $(pgrep -f "autossh.*$name\:") || -n $(pgrep -f "/usr/bin/ssh.*$name\:") ]]; then
      colored_echo "33" "WARNING: Tunnel did not stop gracefully. Forcing stop: $name" | output_handler
      kill -9 $(pgrep -f "autossh.*$name\:") 2>/dev/null
      kill -9 $(pgrep -f "/usr/bin/ssh.*$name\:") 2>/dev/null
    else
      echo "INFO: Tunnel stopped: $name" | output_handler
    fi

    # Clean up the running_tunnels array
    unset running_tunnels["$name"]
  fi
}

# Function to load tunnels from the config file
load_tunnels() {

  # Create an associative array to track tunnels defined in the config
  declare -A defined_tunnels

  # Read the whole config file and parse common settings
  local config
  readarray config < <(yq --output-format=json --indent=0 '.' "$CONFIG_FILE")
  config_ssh_key=$(echo "$config" | yq eval '.ssh_key // ""' -)
  config_local_proto=$(echo "$config" | yq '.local_protocol // "http"')
  config_local_host=$(echo "$config" | yq eval '.local_host // ""' -)
  config_local_port=$(echo "$config" | yq eval '.local_port // ""' -)
  config_remote_port=$(echo "$config" | yq eval '.remote_port // ""' -)
  config_remote_server=$(echo "$config" | yq eval '.remote_server // ""' -)

  # Iterate over the different tunnels
  local tunnels
  readarray tunnels < <(echo "$config" | yq '.tunnels[]')
  for tunnel in "${tunnels[@]}"; do
    name=$(echo "$tunnel" | yq eval '.name' -)
    ssh_key=$(echo "$tunnel" | yq eval '.ssh_key // '\"$config_ssh_key\"'' -)
    local_proto=$(echo "$tunnel" | yq eval '.local_protocol // '\"$config_local_proto\"'' -)
    local_host=$(echo "$tunnel" | yq eval '.local_host // '\"$config_local_host\"'' -)
    local_port=$(echo "$tunnel" | yq eval '.local_port // '\"$config_local_port\"'' -)
    remote_port=$(echo "$tunnel" | yq eval '.remote_port // '\"$config_remote_port\"'' -)
    remote_server=$(echo "$tunnel" | yq eval '.remote_server // '\"$config_remote_server\"'' -)
    tunnel_disabled=$(echo "$tunnel" | yq eval '.disabled // "false"')

    # Create a unique identifier for the tunnel configuration
    current_config=$(echo "$ssh_key:$local_proto:$local_host:$local_port:$remote_port:$remote_server")
    defined_tunnels["$name"]=1

    # Check if the tunnel is disabled
    if [[ "$tunnel_disabled" == true || "${tunnel_disabled,,}" == "true" ]]; then
      # Stop it if it's running
      if [[ -n "${running_tunnels[$name]}" ]]; then
        stop_tunnel "$name"
      fi
      continue
      echo "Continue"
    fi

    # Check if the tunnel is already running and if the configuration has changed
    if [[ -n "${running_tunnels[$name]}" ]]; then
      if [[ "${previous_configs[$name]}" != "$current_config" ]]; then
        echo "INFO: Configuration changed for tunnel: $name. Restarting..." | output_handler
        stop_tunnel "$name"
        start_tunnel "$name" "$ssh_key" "$local_proto" "$local_host" "$local_port" "$remote_port" "$remote_server"
      fi
    else

      # Start the tunnel if it's not already running
      start_tunnel "$name" "$ssh_key" "$local_proto" "$local_host" "$local_port" "$remote_port" "$remote_server"
    fi

    # Store the current configuration
    previous_configs["$name"]="$current_config"
  done

  # Stop tunnels that are no longer defined in the config
  for name in "${!running_tunnels[@]}"; do
    if [[ -z "${defined_tunnels[$name]}" ]]; then
      stop_tunnel "$name"
    fi
  done
}

# Wrap text in color
colored_echo() {
  local color_code="$1"
  shift
  if [[ "$USE_COLOR" == "true" ]]; then
    echo -e "\033[${color_code}m$*\033[0m"
  else
    echo "$*"
  fi
}

# Function to watch the config file for changes
watch_config() {
  local last_mod_time=$(stat -c %Y "$CONFIG_FILE")
  while true; do
    sleep 1
    local new_mod_time=$(stat -c %Y "$CONFIG_FILE")
    if [[ "$new_mod_time" != "$last_mod_time" ]]; then
      last_mod_time="$new_mod_time"
      load_tunnels
    fi
  done
}

# Function to truncate log file when it exceeds a limit
truncate_logs() {
  local max_size=26214400 # 25 MB
  if [[ -f "$OUTPUT_LOG" && $(stat -c%s "$OUTPUT_LOG") -ge $max_size ]]; then
    tail -n 1000 "$OUTPUT_LOG" >"${OUTPUT_LOG}.tmp"
    mv "${OUTPUT_LOG}.tmp" "$OUTPUT_LOG"
    colored_echo "33" "INFO: Log file exceeded $((max_size / 1024 / 1024)) MB. Truncated to the last 1000 lines." | output_handler
  fi
}

# Main function
main() {
  output_handler & # Start the output handler
  verify_files     # Verify existence of config file
  load_tunnels     # Load and unload tunnels
  watch_config     # Start watching the config file
}

# Run the main function
main
