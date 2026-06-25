#!/bin/sh
set -eu

ALLOW_FILE=/etc/nginx/conf.d/allow_ips.conf
: > "$ALLOW_FILE"

if [ -n "${ADMIN_ALLOWED_IPS:-}" ]; then
  echo "# ADMIN_ALLOWED_IPS" >> "$ALLOW_FILE"
  IFS=,
  for ip in $ADMIN_ALLOWED_IPS; do
    ip=$(echo "$ip" | tr -d ' ')
    [ -n "$ip" ] || continue
    echo "allow $ip;" >> "$ALLOW_FILE"
  done
  echo "deny all;" >> "$ALLOW_FILE"
else
  echo "# ADMIN_ALLOWED_IPS not set — allow all" >> "$ALLOW_FILE"
  echo "allow all;" >> "$ALLOW_FILE"
fi

exec nginx -g 'daemon off;'
