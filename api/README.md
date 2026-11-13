# Protobuf API definitions

This directory holds the protobuf definitions used by the Paxeer X node (`paxd`), including chain modules, program types and runtime administration, together with Go tests for their descriptors and REST bindings. These definitions supply generated Go types, gRPC services and HTTP annotations for the modules of the merged Paxeer network; the node registers their routes in [`node/app.go`](../node/app.go).

## Layout

- [`epoch/`](epoch/) — epoch state, genesis, parameters and queries.
- [`eth/`](eth/) — Ethereum transaction envelopes and access lists.
- [`evm/`](evm/) — EVM transactions, receipts, genesis, parameters, governance and queries.
- [`launchpad/`](launchpad/) — launchpad parameters, treasury messages and market queries.
- [`layerxagent/`](layerxagent/) — agent jobs, genesis, events, parameters and queries.
- [`layerxanchor/`](layerxanchor/) — anchor parameter updates.
- [`layerxauth/`](layerxauth/) — authorization scopes, parameters and genesis.
- [`layerxbridge/`](layerxbridge/) — bridge governance proposals and chain, attestor and transfer-cap messages.
- [`layerxcustody/`](layerxcustody/) — custody state, events, genesis, parameters, proposals and withdrawal messages.
- [`layerxexchange/`](layerxexchange/) — exchange state, events, genesis, parameters and market messages.
- [`layerxgov/`](layerxgov/) — governance proposal content.
- [`layerxledger/`](layerxledger/) — ledger parameters and genesis.
- [`layerxpay/`](layerxpay/) — payment parameters and queries.
- [`layerxprograms/`](layerxprograms/) — program genesis, fee and metering parameters, and state, balance and event queries.
- [`mint/`](mint/) — mint state, genesis, governance and queries under `v1beta1/`.
- [`oracle/`](oracle/) — exchange-rate votes, state, genesis and queries.
- [`pax/`](pax/) — runtime logging administration under `admin/v0/`.
- [`perps/`](perps/) — perpetual-market parameters, market settings and pause messages.
- [`pricefeed/`](pricefeed/) — price observations, market settings and queries.
- [`spot/`](spot/) — spot parameters and reserve synchronization messages.
- [`tokenfactory/`](tokenfactory/) — token authority, genesis, parameters, queries and token-management messages.
- [`xweb/`](xweb/) — attestor administration and request, result and program queries.
- [`buf_build_test.go`](buf_build_test.go) — workspace descriptor-image coverage.
- [`buf_apidir_test.go`](buf_apidir_test.go) — workspace-image equivalence when invoked from this directory.
- [`buf_lint_test.go`](buf_lint_test.go) — Buf lint and lint configuration checks.
- [`buf_breaking_test.go`](buf_breaking_test.go) — wire and JSON compatibility against the root descriptor baseline.
- [`buf_generate_test.go`](buf_generate_test.go) — generated Go parity with checked-in files.
- [`protosource_helpers_test.go`](protosource_helpers_test.go) — descriptor comparison and query/gateway helpers used by the module tests.
- [`gateway_mount_test.go`](gateway_mount_test.go) — node REST route registration tests.

The other `*_test.go` files check module descriptor parity, reserved fields, REST responses and SDK shadow definitions.

Each file's `go_package` determines the generated Go package. Most definitions target `modules/<name>/types` under [`modules/`](../modules); the exceptions are:

| Protobuf source | Generated Go package directory |
| --- | --- |
| [`eth/`](eth) | [`modules/evm/types/ethtx`](../modules/evm/types/ethtx) |
| [`layerxprograms/`](layerxprograms) | [`precompiles/layerxprograms/types`](../precompiles/layerxprograms/types) |
| [`pax/admin/v0/admin.proto`](pax/admin/v0/admin.proto) | [`admin/types`](../admin/types) |

## Build and test

The directory contains schema sources and tests, with no standalone binary. From this directory, run `buf build ..` to build the full root workspace and its local imports; plain `buf build` uses the same configuration but selects only `api/` and its imports. The workspace-image tests use the pinned Buf CLI through `go run github.com/bufbuild/buf/cmd/buf@v1.58.0`.

Regenerate the Go code from the repository root:

```bash
./scripts/protoc.sh
```

[`scripts/protoc.sh`](../scripts/protoc.sh) builds the pinned `protoc-gen-gocosmos` plugin into `build/proto/gocosmos`, runs `buf generate` (through `go run github.com/bufbuild/buf/cmd/buf@v1.58.0`) with the root template and the `consensus/internal/buf.gen.yaml` and `consensus/internal/wireguard.buf.gen.yaml` templates, then copies the generated Go files from `build/proto/` into their module directories. The temporary `build/proto/` directory is removed after copying. The `go:generate` entry point is the first directive in [`tools/repotest/gen.go`](../tools/repotest/gen.go), which runs this script from the repository root. The script also accepts `--wasm-swagger` to generate the Wasm query Swagger document under `build/swagger/wasm` without running the Go generation path.

The root [`Makefile`](../Makefile) provides the node build and the Go test target that includes this directory:

```bash
make paxeer-build
make paxeer-test
```

These targets delegate to [`chain.mk`](../chain.mk). The test target selects Go packages with tests, including external test packages such as `api_test`, and runs them in groups. The API tests cover Buf build, lint, compatibility and generation as well as module descriptors and REST handlers. The root targets use `GOPROXY` in offline mode, so Go dependencies must already be cached.

## Configuration

The root [`buf.yaml`](../buf.yaml) declares this directory as one Buf module next to `sdk/proto`, `sdk/third_party/proto`, `interchain/proto`, `interchain/third_party/proto`, `consensus/proto`, `consensus/internal` and `wasm/proto`, and the root [`buf.gen.yaml`](../buf.gen.yaml) lists it as a generation input. These root files are the only Buf configuration for `api/`.

The generation script also reads [`consensus/internal/buf.gen.yaml`](../consensus/internal/buf.gen.yaml) and [`consensus/internal/wireguard.buf.gen.yaml`](../consensus/internal/wireguard.buf.gen.yaml). Generation tests read the root [`go.mod`](../go.mod) for plugin versions, and compatibility tests read [`buf.baseline.binpb`](../buf.baseline.binpb). The protobuf definitions have no runtime environment variables of their own; runtime configuration belongs to the node and the generated services' consumers.

## Documentation

- [Paxeer Network documentation](../docs/paxeer-chain-docs.md) — chain documentation and OpenAPI/Swagger generation.
- [REST & gRPC APIs](../docs/site/paxeer-docs/src/app/rest-grpc/page.tsx) — documentation-site page for these definitions, queries and administration.
- [REST & gRPC Markdown reference](../docs/site/paxeer-docs/public/markdown/rest-grpc.md) — the corresponding Markdown reference.
