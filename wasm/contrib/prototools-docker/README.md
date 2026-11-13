# Protobuf generation

Use the repository's root protobuf generator for the Wasm module. The
[Dockerfile](Dockerfile) in this directory is an upstream tool definition;
the root generator uses the versions and workspace configuration below.

## Generate Go code

From the repository root, with the Go toolchain declared in
[`go.mod`](../../../go.mod) (Go `1.25.8`), run:

```sh
bash scripts/protoc.sh
```

[`scripts/protoc.sh`](../../../scripts/protoc.sh) builds
`protoc-gen-gocosmos` from `github.com/regen-network/cosmos-proto` v0.3.1,
then invokes Buf v1.58.0 through `go run`. It uses the root
[`buf.yaml`](../../../buf.yaml) and [`buf.gen.yaml`](../../../buf.gen.yaml),
both with configuration version `v2`, plus the consensus internal templates.

The root configuration includes `wasm/proto`, uses the local
`build/proto/gocosmos/protoc-gen-gocosmos` generator, and maps SDK types to this
repository's fork, including `google/protobuf/any.proto` to
`github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types`. The workspace includes
[`sdk/proto`](../../../sdk/proto) and
[`sdk/third_party/proto`](../../../sdk/third_party/proto) as import inputs.

The script copies generated output into the source trees, including `sdk/`,
`wasm/` and `consensus/`, and removes `build/proto`. It downloads dependencies,
uses `go get` to obtain the generator source and can update module metadata.
Generated files are overwritten.

## Generate Swagger

```sh
bash scripts/protoc.sh --wasm-swagger
```

This command uses protoc-gen-swagger v1.16.0 and writes the Wasm query
service schema to `build/swagger/wasm/cosmwasm/wasm/v1/query.swagger.json`.
See the [Wasm script guide](../../scripts/README.md) for wrapper scripts and
the output location.

See also the [development tools](../devtools/README.md) and
[local examples](../local/README.md).
