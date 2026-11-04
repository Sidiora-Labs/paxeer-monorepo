#!/bin/bash

set -euo pipefail

# Resolve the workspace before any generation or cleanup, even when invoked by
# a compatibility script from another directory.
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "$repo_root"

case "${1:-}" in
  --wasm-swagger)
    if [[ $# -ne 1 ]]; then
      echo "Usage: bash scripts/protoc.sh [--wasm-swagger]" >&2
      exit 2
    fi
    # JSON is also a Buf YAML template. Keep the Swagger plugin on the same
    # grpc-gateway release as buf.gen.yaml; Buf resolves imports via buf.yaml.
    swagger_template='{
      "version": "v2",
      "inputs": [{"directory": "wasm/proto"}],
      "plugins": [{
        "local": ["go", "run", "github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger@v1.16.0"],
        "out": "./build/swagger/wasm",
        "opt": ["logtostderr=true", "fqn_for_swagger_name=true", "simple_operation_ids=true"]
      }]
    }'
    exec go run github.com/bufbuild/buf/cmd/buf@v1.58.0 generate \
      --template "$swagger_template" \
      --path wasm/proto/cosmwasm/wasm/v1/query.proto
    ;;
  "")
    if [[ $# -ne 0 ]]; then
      echo "Usage: bash scripts/protoc.sh [--wasm-swagger]" >&2
      exit 2
    fi
    ;;
  *)
    echo "Usage: bash scripts/protoc.sh [--wasm-swagger]" >&2
    exit 2
    ;;
esac

echo "Generating protobuf code..."

rm -rf ./build/proto

# We have to build regen-network protoc-gen-gocosmos from source because
# the module uses replace directive, which makes it impossible to use
# go install like a healthy human being.
#
# As a workaround, we download the source code to a temporary location
# and build the binary. buf.gen.yaml then implicitly uses the path to the
# built binary. This is ugly but it works, and results in the least amount
# of changes across the repo to have _a_ working solution without accidentally
# breaking anything else or introduce too much change as part of automating
# the proto generation.
go get github.com/regen-network/cosmos-proto/protoc-gen-gocosmos@v0.3.1
mkdir -p ./build/proto/gocosmos
build_out="${PWD}/build/proto/gocosmos"
pushd "$(go env GOMODCACHE)/github.com/regen-network/cosmos-proto@v0.3.1" &&
  go build -o "${build_out}/protoc-gen-gocosmos" ./protoc-gen-gocosmos &&
  popd

go run github.com/bufbuild/buf/cmd/buf@v1.58.0 generate
go run github.com/bufbuild/buf/cmd/buf@v1.58.0 generate --template consensus/internal/buf.gen.yaml
go run github.com/bufbuild/buf/cmd/buf@v1.58.0 generate --template consensus/internal/wireguard.buf.gen.yaml

# We can't manipulate the outputs enough to eliminate the extra move-abouts.
# So we just copy the files we want to the right places manually.
# The repo restructure should help this in the future.
cp -rf ./build/proto/gocosmos/github.com/Sidiora-Labs/Paxeer-X-Network/* ./
cp -rf ./build/proto/gocosmos/github.com/Sidiora-Labs/Paxeer-X-Network/sdk/* ./sdk
cp -rf ./build/proto/gocosmos/github.com/Sidiora-Labs/Paxeer-X-Network/wasm/* ./wasm

# Use gogofaster for Tendermint and IAVL because that is their established generator.
# See ./consensus/internal/buf.gen.yaml.
cp -rf ./build/proto/gogofaster/github.com/Sidiora-Labs/Paxeer-X-Network/consensus/* ./consensus

rm -rf ./build/proto

echo "Protobuf code generation complete."
