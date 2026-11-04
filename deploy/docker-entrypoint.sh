#!/bin/sh
set -e

mkdir -p /data/gitfox-community

for f in prod.env prod.yaml; do
  if [ -d "/data/gitfox-community/$f" ]; then
    rm -rf "/data/gitfox-community/$f"
  fi
  if [ ! -f "/data/gitfox-community/$f" ]; then
    cp "/app/defaults/$f" "/data/gitfox-community/$f"
  fi
done

exec /app/gitfox "$@"
