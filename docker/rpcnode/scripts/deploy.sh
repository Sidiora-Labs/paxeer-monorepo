#!/usr/bin/env sh
set -eu
umask 077

SKIP_BUILD=${SKIP_BUILD:-""}

# Set up env
export GOPATH="$HOME/go"
export GOBIN="$GOPATH/bin"
export BUILD_PATH=/pax-protocol/pax-chain/build
export PATH="$GOBIN:$PATH:/usr/local/go/bin:$BUILD_PATH"

for script in /usr/bin/configure_init.sh /usr/bin/start_pax.sh; do
  [ -x "$script" ] || { echo "Missing executable: $script" >&2; exit 1; }
done
mkdir -p "$GOBIN"

# Step 1 build paxd; enforce failure propagation inside the build script too.
if [ -z "$SKIP_BUILD" ]; then
  [ -r /usr/bin/build.sh ] || { echo "Missing build script" >&2; exit 1; }
  sh -e /usr/bin/build.sh
fi
[ -x build/paxd ] || { echo "Missing executable: build/paxd" >&2; exit 1; }
cp build/paxd "$GOBIN"/

# Run init to set up state sync configurations.
/usr/bin/configure_init.sh

# Keep the node attached to the container supervisor.
exec /usr/bin/start_pax.sh
