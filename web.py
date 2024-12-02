#!/usr/bin/python3

import os
import re
import subprocess
import threading

import yaml
from flask import Flask, redirect, render_template, request, url_for

# Initialize Flask app
app = Flask(__name__)

# Config and log paths
CONFIG_FILE = os.getenv(
    "SISHC_CONFIG_FILE",
    f"{os.getenv('XDG_CONFIG_HOME', os.path.join(os.getenv('HOME'), '.config'))}/sishc/config.yaml",
)
LOG_FILE = os.getenv(
    "SISHC_OUTPUT_LOG",
    f"{os.getenv('XDG_DATA_HOME', os.path.join(os.getenv('HOME'), '.local/share'))}/sishc/sishc.log",
)


# Function to remove ANSI escape sequences from log lines
def strip_ansi_codes(text):
    ansi_escape = re.compile(r"\x1B[@-_][0-?]*[ -/]*[@-~]")
    return ansi_escape.sub("", text)


# Load configuration
def load_config():
    with open(CONFIG_FILE, "r") as file:
        config = yaml.safe_load(file)
        return config if config else {}


# Save configuration
def save_config(config):
    # Remove empty keys from config
    config = {k: v for k, v in config.items() if v != ""}
    with open(CONFIG_FILE, "w") as file:
        yaml.dump(config, file)


# Load tunnel configurations
def load_tunnels():
    config = load_config()
    return config.get("tunnels", [])


# Save tunnel configurations
def save_tunnels(tunnels):
    # Remove empty keys from each tunnel
    tunnels = [{k: v for k, v in tunnel.items() if v != ""} for tunnel in tunnels]
    config = load_config()
    config["tunnels"] = tunnels
    save_config(config)


# Get tunnel logs
def get_logs():
    with open(LOG_FILE, "r") as file:
        return file.readlines()


@app.route("/")
def index():
    tunnels = load_tunnels()
    global_config = load_config()
    for tunnel in tunnels:
        for key in [
            "ssh_key",
            "local_protocol",
            "local_host",
            "local_port",
            "remote_port",
            "remote_server",
        ]:
            if key not in tunnel or tunnel[key] == "":
                tunnel[key] = global_config.get(key, "")
    return render_template("index.html", tunnels=tunnels, global_config=global_config)


@app.route("/edit/<string:tunnel_name>", methods=["GET", "POST"])
def edit_tunnel(tunnel_name):
    tunnels = load_tunnels()
    tunnel = next((t for t in tunnels if t["name"] == tunnel_name), None)
    global_config = load_config()
    if request.method == "POST":
        updated_tunnel = request.form.to_dict()
        for key, value in updated_tunnel.items():
            if key != "name" and value == global_config.get(key, ""):
                updated_tunnel[key] = ""
        tunnel.update(updated_tunnel)
        save_tunnels(tunnels)
        return redirect(url_for("index"))
    return render_template("edit.html", tunnel=tunnel, global_config=global_config)


@app.route("/delete/<string:tunnel_name>", methods=["POST"])
def delete_tunnel(tunnel_name):
    tunnels = load_tunnels()
    tunnels = [t for t in tunnels if t["name"] != tunnel_name]
    save_tunnels(tunnels)
    return redirect(url_for("index"))


@app.route("/logs/<string:tunnel_name>")
def view_logs(tunnel_name):
    logs = get_logs()
    stripped_logs = [strip_ansi_codes(log) for log in logs if tunnel_name in log]
    return render_template("logs.html", logs=stripped_logs, tunnel_name=tunnel_name)


@app.route("/config", methods=["GET", "POST"])
def config():
    config = load_config()
    if request.method == "POST":
        config["ssh_key"] = request.form["ssh_key"]
        config["local_protocol"] = request.form["local_protocol"]
        config["local_host"] = request.form["local_host"]
        config["local_port"] = int(request.form["local_port"])
        config["remote_port"] = int(request.form["remote_port"])
        config["remote_server"] = request.form["remote_server"]
        save_config(config)
        return redirect(url_for("index"))
    return render_template("config.html", config=config)


@app.route("/add", methods=["GET", "POST"])
def add_tunnel():
    global_config = load_config()
    if request.method == "POST":
        tunnels = load_tunnels()
        new_tunnel = request.form.to_dict()
        for key, value in new_tunnel.items():
            if key != "name" and value == global_config.get(key, ""):
                new_tunnel[key] = ""
        tunnels.append(new_tunnel)
        save_tunnels(tunnels)
        return redirect(url_for("index"))
    return render_template("add.html", global_config=global_config)


@app.route("/edit_raw", methods=["GET", "POST"])
def edit_raw():
    if request.method == "POST":
        raw_content = request.form["raw_content"]
        with open(CONFIG_FILE, "w") as file:
            file.write(raw_content)
        return redirect(url_for("index"))
    with open(CONFIG_FILE, "r") as file:
        raw_content = file.read()
    return render_template("edit_raw.html", raw_content=raw_content)


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=False)
