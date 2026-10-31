#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
cd "$root"
case "${1:-verify}" in
build)
    make -j5 LXP_REVISION="$(git rev-parse HEAD)" PAXEER_GO_JOBS=5 paxeer-build
    cargo build --locked --manifest-path services/platform/Cargo.toml -p layerx-platform-paxeer-boundary --bin layerx-paxeer-boundary
    forge build services/platform/hosted/paxeer/contracts/BetaUsdl.sol \
        tools/benchmark/loadtest/contracts/evm/lib/solmate/src/tokens/WETH.sol \
        --threads 5 --out build/withdraw-contracts/artifacts --cache-path build/withdraw-contracts/cache
    ;;
verify)
    test -f build/withdraw-contracts/artifacts/BetaUsdl.sol/BetaUsdl.json
    test -f build/withdraw-contracts/artifacts/WETH.sol/WETH.json
    ;;
*) echo 'usage: 24.16.sh build|verify' >&2; exit 2 ;;
esac
