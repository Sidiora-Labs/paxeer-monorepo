Prototool Docker Helper
=======================

This is a historical upstream protobuf helper, based on the
[prototool-docker](https://github.com/charithe/prototool-docker) project.
The [Dockerfile](Dockerfile) is retired from supported repository generation
and retained only as a historical upstream reference. Use the root workflow
below for all current protobuf generation, including `wasm/proto`.

## Current repository generation

From the repository root, with the Go toolchain declared in the root
[`go.mod`](../../../go.mod) (currently Go `1.25.8`), run:

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
`github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types`. The root workspace
includes the local [`sdk/proto`](../../../sdk/proto) and
[`sdk/third_party/proto`](../../../sdk/third_party/proto) inputs; no external
Cosmos SDK checkout or `COSMOS_SDK_DIR` mount is needed.

The script copies generated output back into the source trees, including
`sdk/`, `wasm/` and `consensus/`, and removes `build/proto`. Its `go get` step can
update module metadata. It requires dependency downloads and overwrites
generated files, so review the resulting source and module changes.

## Historical image contents

The [Dockerfile](Dockerfile) uses Go 1.16.8 for its build stage and Go 1.17.3
for its final image. Its default tool versions are:

| Tool | Version |
| --- | --- |
| Buf | `0.30.0` |
| gRPC gateway and Swagger generators | `1.16.0` |
| Regen `protoc-gen-gocosmos` | `0.3.0` |
| Regen protobuf replacement | `1.3.2-alpha.regen.4` |
| `protoc-gen-doc` | `1.3.2` |

These come from [bufbuild/buf](https://github.com/bufbuild/buf),
[grpc-ecosystem/grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway),
[regen-network/cosmos-proto](https://github.com/regen-network/cosmos-proto),
[regen-network/protobuf](https://github.com/regen-network/protobuf), and
[pseudomuto/protoc-gen-doc](https://github.com/pseudomuto/protoc-gen-doc).
The final image also installs Alpine's `protoc` package. Buf 0.30.0 cannot read
the root v2 configuration, and the image's generator versions differ from the
root workflow. Mounting the current sources does not resolve those differences.

There are no supported build or run instructions for this retired image.
Use `bash scripts/protoc.sh` from the repository root instead.

See also the [devtools notes](../devtools/README.md) and
[upstream local examples](../local/README.md).
