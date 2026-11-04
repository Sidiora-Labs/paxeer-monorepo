#!/usr/bin/env sh

umask 077

SKIP_BUILD=${SKIP_BUILD:-""}

# Set up env
export GOPATH="$HOME/go"
export GOBIN="$GOPATH/bin"
export BUILD_PATH=/pax-protocol/pax-chain/build
export PATH="$GOBIN:$PATH:/usr/local/go/bin:$BUILD_PATH"
mkdir -p "$GOBIN"

# Step 1 build paxd
if [ -z "$SKIP_BUILD" ]
then
  /usr/bin/build.sh
fi
cp build/paxd "$GOBIN"/

# Run init to set up state sync configurations
/usr/bin/configure_init.sh

# Start the chain
/usr/bin/start_pax.sh

tail -f /dev/null
