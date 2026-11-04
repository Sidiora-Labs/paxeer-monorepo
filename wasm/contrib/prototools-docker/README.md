Prototool Docker Helper
=======================

This is a historical upstream protobuf helper, based on the
[prototool-docker](https://github.com/charithe/prototool-docker) project.
It is not the toolchain for generating the merged repository's protobuf code.

## Current repository generation

From the repository root, use the existing generation script:

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
repository's fork. The script copies generated output back into the source
trees and removes `build/proto`; its `go get` step can update module metadata.
Use the Go toolchain declared in the root [`go.mod`](../../../go.mod).

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

### Build and inspect the historical helper

For historical tooling work, run these commands from the repository root.
The build depends on the old base images, repositories, and package sources
remaining available; it is not a qualification of current protobuf generation.

```sh
docker build -t cosmwasm/prototools-docker -f wasm/contrib/prototools-docker/Dockerfile .
docker run --rm -it \
  -v "$PWD:/workspace" \
  -v "$PWD/sdk:/workspace/cosmos_sdk_dir" \
  --workdir /workspace \
  --env COSMOS_SDK_DIR=/workspace/cosmos_sdk_dir \
  cosmwasm/prototools-docker sh
```

Both commands use the same image tag. The SDK mount comes from this checkout's
[`sdk/`](../../../sdk), and `COSMOS_SDK_DIR` names that exact container path.
The Dockerfile itself does not consume that variable; it is available to
commands run inside the shell. Use the root script above for current generation.

See also the [devtools notes](../devtools/README.md) and
[upstream local examples](../local/README.md).
