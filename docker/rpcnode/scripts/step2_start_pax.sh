#!/usr/bin/env sh
set -eu

command -v paxd >/dev/null 2>&1 || { echo "Missing executable: paxd" >&2; exit 1; }
for file in app.toml config.toml genesis.json; do
  [ -s "$HOME/.pax/config/$file" ] && [ -r "$HOME/.pax/config/$file" ] || {
    echo "Missing readable node configuration: $file" >&2
    exit 1
  }
done

# The supervisor observes the node's logs, signals and exit status directly.
exec paxd start --chain-id pax
