#!/usr/bin/env bash
# Builds the app and deploys it to Unikraft Cloud.
#
#   ./deploy.sh            # deploy to fra
#   METRO=dal ./deploy.sh  # deploy elsewhere
#
# The public address lives on a persistent service and the SQLite database on a
# persistent volume, so the instance itself is disposable: each deploy stops it
# (SIGTERM closes the database cleanly), deletes it and starts a new one on the
# fresh image. A volume attaches to one instance at a time, so expect a few
# seconds of downtime.
set -euo pipefail

METRO="${METRO:-fra}"
APP=secret-santa # instance, service and subdomain name
IMAGE=jesi-rgb/secret-santa
VOLUME=secret-santa-data
URL="https://$APP.$METRO.unikraft.app"

cd "$(dirname "$0")"

for cmd in unikraft jq curl; do
  command -v "$cmd" >/dev/null || { echo "missing required command: $cmd" >&2; exit 1; }
done

echo "==> Building and pushing $IMAGE:latest"
unikraft build . --output "$IMAGE:latest"
digest=$(unikraft image get "$IMAGE:latest" -o json | jq -r 'first(.[].digest)')
[[ $digest == sha256:* ]] || { echo "could not resolve image digest" >&2; exit 1; }

if ! unikraft volume get "$METRO/$VOLUME" -o quiet >/dev/null 2>&1; then
  echo "==> Creating volume $VOLUME"
  unikraft volume create --metro "$METRO" --name "$VOLUME" --size 64 -o quiet
fi

if ! unikraft service get "$METRO/$APP" -o quiet >/dev/null 2>&1; then
  echo "==> Creating service $APP ($URL)"
  unikraft service create --metro "$METRO" --name "$APP" --domain "$APP" \
    --service 443:8080/http+tls --service 80:443/http+redirect -o quiet
fi

if unikraft instance get "$METRO/$APP" -o quiet >/dev/null 2>&1; then
  echo "==> Stopping and removing the current instance"
  unikraft instance stop "$METRO/$APP" -o quiet
  unikraft instance delete "$METRO/$APP" -o quiet
fi

echo "==> Starting $IMAGE@$digest"
unikraft run --metro "$METRO" --name "$APP" --image "$IMAGE@$digest" \
  --service "$APP" -v "$VOLUME:/data" -m 256MiB \
  --scale-to-zero policy=on,stateful=true,cooldown-time=2000 \
  --restart on-failure -o quiet

echo "==> Checking $URL"
curl -fsS --retry 5 --retry-all-errors --retry-delay 2 -o /dev/null "$URL/"
echo "Deployed: $URL"
