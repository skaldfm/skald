#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Create group and user if they don't already exist
if ! getent group skald >/dev/null 2>&1; then
    addgroup -g "$PGID" skald
fi
if ! getent passwd skald >/dev/null 2>&1; then
    adduser -u "$PUID" -G skald -s /bin/sh -D -H skald
fi

chown -R skald:skald /app/data

exec su-exec skald:skald ./skald "$@"
