#!/bin/sh

if ! [[ "$PUID" == 0 && "$PGID" == 0 ]]; then
  # Change UID and GID based on environment variables
  usermod -u "$PUID" abc >/dev/null
  groupmod -g "$PGID" abc >/dev/null

  find / -maxdepth 1 ! -name '.*' -exec chown abc:abc {} + >/dev/null 2>&1

  su-exec abc python3 /web.py >/dev/null 2>&1 &
  su-exec abc /sishc.sh
else
  python3 /web.py >/dev/null 2>&1 &
  /sishc.sh
fi
