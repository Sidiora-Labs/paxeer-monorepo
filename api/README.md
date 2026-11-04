# Protobuf API definitions

This directory holds the protobuf definitions used by the Paxeer X node (`paxd`), including chain modules, program types and runtime administration. The source directories are `epoch`, `eth`, `evm`, `launchpad`, `layerxagent`, `layerxanchor`, `layerxauth`, `layerxbridge`, `layerxcustody`, `layerxexchange`, `layerxgov`, `layerxledger`, `layerxpay`, `layerxprograms`, `mint`, `oracle`, `pax`, `perps`, `pricefeed`, `spot`, `tokenfactory` and `xweb`.

Each file's `go_package` determines the generated Go package. Most definitions target `modules/<name>/types` under [`modules/`](../modules); the exceptions are:

| Protobuf source | Generated Go package directory |
| --- | --- |
| [`eth/`](eth) | [`modules/evm/types/ethtx`](../modules/evm/types/ethtx) |
| [`layerxprograms/`](layerxprograms) | [`precompiles/layerxprograms/types`](../precompiles/layerxprograms/types) |
| [`pax/admin/v0/admin.proto`](pax/admin/v0/admin.proto) | [`admin/types`](../admin/types) |

The root [`buf.yaml`](../buf.yaml) declares this directory as one Buf module next to `sdk/proto`, `sdk/third_party/proto`, `interchain/proto`, `interchain/third_party/proto`, `consensus/proto`, `consensus/internal` and `wasm/proto`, and the root [`buf.gen.yaml`](../buf.gen.yaml) lists it as a generation input. These root files are the only Buf configuration for `api/`. From this directory, run `buf build ..` to build the full root workspace and its local imports; plain `buf build` uses the same configuration but selects only `api/` and its imports.

## Regenerate the Go code

From the repository root:

```bash
./scripts/protoc.sh
```

[`scripts/protoc.sh`](../scripts/protoc.sh) builds the pinned `protoc-gen-gocosmos` plugin into `build/proto/gocosmos`, runs `buf generate` (through `go run github.com/bufbuild/buf/cmd/buf@v1.58.0`) with the root template and the `consensus/internal/buf.gen.yaml` and `consensus/internal/wireguard.buf.gen.yaml` templates, then copies the generated Go files from `build/proto/` into their module directories. The `go:generate` entry point is the first directive in [`tools/repotest/gen.go`](../tools/repotest/gen.go), which runs this script from the repository root.
