# devtools

The [Makefile](Makefile) in this directory is inherited from the Cosmos SDK and provides
standalone development tools. Its `tools` target depends on `statik`, `runsim`, and
`golangci-lint` through `tools-stamp`.

## Installation

Use the Go 1.25.8 toolchain declared in the root [`go.mod`](../../../go.mod).
The recipes install these pinned executable packages with `go install package@version`:

| Executable | Package | Pinned version |
| --- | --- | --- |
| `statik` | `github.com/rakyll/statik` | `v0.1.8` |
| `runsim` | `github.com/cosmos/tools/cmd/runsim` | `v1.0.0` |
| `golangci-lint` | `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` | `v2.5.0` |

The linter uses the v2 module and configuration format. Its
[release history](https://github.com/golangci/golangci-lint/blob/v2.5.0/CHANGELOG.md)
records Go 1.25 support starting with v2.4.0; v2.5.0 matches the repository's v2
configuration. `statik` and `runsim` retain their existing command interfaces.

`TOOLS_DESTDIR` defaults to `$(GOPATH)/bin`. Each recipe sets `GOBIN` to the absolute
path of `TOOLS_DESTDIR`, so Go writes the binary at its Make prerequisite path,
including when the destination is overridden. Version-qualified installation leaves
the project's `go.mod` and `go.sum` unchanged and needs no temporary working directory.
Installing requires access to the Go module proxy or a populated module cache.

From the repository root, install into a writable directory and put it on `PATH`:

```sh
make -C wasm/contrib/devtools tools TOOLS_DESTDIR="$PWD/.devtools/bin"
export PATH="$PWD/.devtools/bin:$PATH"
```

Existing binaries satisfy Make's prerequisites. To refresh them after a pin changes,
run `make -C wasm/contrib/devtools tools-clean TOOLS_DESTDIR="$PWD/.devtools/bin"` before installing
again, using the same destination. `tools-clean` removes the three binaries there
and `tools-stamp`. Invoke the installed commands through `PATH`, for example with
`golangci-lint run`.

These helpers belong to the vendored Wasm development tree. See the
[upstream local scripts](../local/README.md) for their standalone `wasmd` prerequisites
and the [protobuf helper notes](../prototools-docker/README.md) for the root generation path.

## Contributors

Thanks to the entire [Cosmos SDK](https://github.com/cosmos/cosmos-sdk) team and the contributors who put their efforts
into making simulation testing easier to implement. This Makefile is derived from the Cosmos SDK's `contrib/devtools/Makefile`.
