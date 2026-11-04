# devtools

The [Makefile](Makefile) in this directory is inherited from the Cosmos SDK and included by
[`wasm/Makefile`](../../Makefile). Its `tools` target depends on `statik`, `runsim`, and
`golangci-lint` through `tools-stamp`.

## Installation limits

The recipes still describe the upstream toolchain:

| Executable | Package | Pinned version |
| --- | --- | --- |
| `statik` | `github.com/rakyll/statik` | `v0.1.6` |
| `runsim` | `github.com/cosmos/tools/cmd/runsim` | `v1.0.0` |
| `golangci-lint` | `github.com/golangci/golangci-lint/cmd/golangci-lint` | `v1.28.0` |

Each installation recipe runs `go get` from `/tmp`. Modern Go does not install executables
with `go get`, and this outside-module invocation is incompatible with the Go 1.25.8
toolchain declared in the root [`go.mod`](../../../go.mod). Consequently, `make tools`
is not a working installer for the current repository toolchain. The pinned versions
above record the recipes; they are not a claim of compatibility with current Go.

The Makefile looks for executables in `TOOLS_DESTDIR`, defaulting to `$(GOPATH)/bin`.
Overriding `TOOLS_DESTDIR` changes those prerequisite paths, but the recipes do not pass
that directory to Go as `GOBIN`. Provision compatible tool versions separately in the
chosen directory; a modern installation uses `go install package@version` with `GOBIN`
set explicitly. `tools-clean` removes the three binaries at those paths and `tools-stamp`.

These helpers belong to the vendored Wasm development tree. See the
[upstream local scripts](../local/README.md) for their standalone `wasmd` prerequisites
and the [protobuf helper notes](../prototools-docker/README.md) for the root generation path.

## Contributors

Thanks to the entire [Cosmos SDK](https://github.com/cosmos/cosmos-sdk) team and the contributors who put their efforts
into making simulation testing easier to implement. This Makefile is derived from the Cosmos SDK's `contrib/devtools/Makefile`.
