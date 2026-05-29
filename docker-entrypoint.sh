#!/bin/ash
set -eu

user_name="${SISHC_USER:-sishc}"
puid="${PUID:-1000}"
pgid="${PGID:-1000}"
group_name="${user_name}"

if [ "$(id -u)" = "0" ]; then
  if existing_group="$(getent group "${pgid}" 2>/dev/null | cut -d: -f1)"; [ -n "${existing_group:-}" ]; then
    group_name="${existing_group}"
  else
    addgroup -S -g "${pgid}" "${user_name}"
  fi
  if ! getent passwd "${puid}" >/dev/null 2>&1; then
    adduser -S -D -u "${puid}" -G "${group_name}" -h "${HOME}" "${user_name}"
  fi
  mkdir -p "${HOME}" "${HOME}/.ssh" "${SISHC_LOG_DIR}"
  chown -R "${puid}:${pgid}" "${HOME}" "${SISHC_LOG_DIR}"
  exec su-exec "${puid}:${pgid}" "$@"
fi

exec "$@"
