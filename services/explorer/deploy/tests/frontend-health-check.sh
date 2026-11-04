#!/usr/bin/env bash
# Exercise the Compose probe against an already-built frontend image, offline.
set -euo pipefail

deploy_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
settings="$(docker compose --file "$deploy_dir/docker-compose.local.yml" config --format json | python3 -c '
import json, sys

def frontend_settings(config):
    frontend = config["services"]["frontend"]
    probe = frontend["healthcheck"]["test"]
    assert len(probe) == 2 and probe[0] == "CMD-SHELL"
    return frontend["image"], probe[1]

print(*frontend_settings(json.load(sys.stdin)), sep="\n")
')"
mapfile -t fields <<< "$settings"
[ "${#fields[@]}" -eq 2 ]
image="${fields[0]}"
probe="${fields[1]}"
docker image inspect "$image" > /dev/null

container=""
cleanup() {
  if [ -n "$container" ]; then
    docker rm --force "$container" > /dev/null
  fi
}
trap cleanup EXIT

# Keep a container alive without a server so the negative probe executes wget,
# rather than merely failing because docker exec cannot reach a stopped image.
container="$(docker run --detach --pull=never --network none \
  --env HOSTNAME=127.0.0.1 --env PORT=3000 --entrypoint sleep "$image" 120)"
if docker exec "$container" sh -c "$probe"; then
  echo "FAIL frontend probe succeeded without a server" >&2
  exit 1
fi

# Start the built standalone server directly: health needs no runtime asset
# downloads, backend, RPC credentials or entrypoint-generated browser config.
docker exec --detach "$container" node server.js
deadline=$((SECONDS + 60))
until docker exec "$container" sh -c "$probe"; do
  if [ "$SECONDS" -ge "$deadline" ]; then
    docker logs "$container" >&2
    echo "FAIL frontend never answered its Compose health probe" >&2
    exit 1
  fi
  sleep 1
done
echo "frontend image health check: ok"
