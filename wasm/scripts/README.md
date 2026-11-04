# Scripts

Run generation from the repository root:

```sh
bash scripts/protoc.sh
bash scripts/protoc.sh --wasm-swagger
```

The first command generates Go protobuf and gateway code through the root
`buf.gen.yaml`, including wasm types under `wasm/x/wasm/types`. It uses
gocosmos v0.3.1, grpc-gateway v1.16.0 and Buf v1.58.0, with the monorepo SDK
Any mapping. The two scripts in this directory delegate to these commands
and resolve the root from their own location, independently of the caller's
working directory. They require Bash, Go and the dependencies of the root
generator; they do not install a plugin at `@latest`.

The Swagger command uses protoc-gen-swagger v1.16.0 and generates only the
wasm query service into
`build/swagger/wasm/cosmwasm/wasm/v1/query.swagger.json`. Both commands resolve
imports through the root `buf.yaml` workspace, including `wasm/proto`,
`sdk/proto` and `sdk/third_party/proto`; no `wasm/third_party/proto` is needed.
Swagger output is a staging artifact. Publishing it into the node's combined
`docs/swagger-ui/swagger.yml` and embedded bundle is a separate documentation
wiring step. The offline repair gate compares the committed combined routes
and response fields with the proto declarations and registered Go gateway;
it does not run generation or establish that a fresh artifact is identical.

The legacy `wasm/Makefile` Docker `proto-gen` target mounts only `wasm`, so it
cannot access the root pipeline. Use the root command above until the owning
build domain wires that target to the root (tracked in repair 22.33.1252).

Please note that a custom [fork](https://github.com/regen-network/protobuf) of gogo/protobuf by the Regen Network team
is used. See the `replace` directive for `github.com/gogo/protobuf` in the root [`go.mod`](../../go.mod) for the version.
