#!/bin/sh

if ! [[ "$PUID" == 0 && "$PGID" == 0 ]]; then
  # Change UID and GID based on environment variables
  usermod -u "$PUID" abc >/dev/null
  groupmod -g "$PGID" abc >/dev/null

  exec su-exec abc "$@"
else
  exec "$@"
fi
