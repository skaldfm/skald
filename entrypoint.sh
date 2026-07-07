#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Resolve a group for the requested GID, reusing any existing one (alpine ships
# built-in groups like "users" at gid 100, so blindly addgroup-ing collides).
group_name=$(getent group "$PGID" | cut -d: -f1)
if [ -z "$group_name" ]; then
    group_name=skald
    addgroup -g "$PGID" "$group_name"
fi

# Same for the user: reuse an existing account with this UID rather than failing.
user_name=$(getent passwd "$PUID" | cut -d: -f1)
if [ -z "$user_name" ]; then
    adduser -u "$PUID" -G "$group_name" -s /bin/sh -D -H skald
fi

# Recursive chown is O(files under data/); skip it once ownership is already
# correct so startup doesn't scale with the size of the uploads directory.
if [ "$(stat -c '%u' /app/data)" != "$PUID" ]; then
    chown -R "$PUID:$PGID" /app/data
fi

exec su-exec "$PUID:$PGID" ./skald "$@"
