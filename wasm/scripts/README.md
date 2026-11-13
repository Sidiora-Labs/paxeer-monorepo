# Scripts

Run generation from the repository root:

```sh
bash scripts/protoc.sh
bash scripts/protoc.sh --wasm-swagger
```

The first command generates Go protobuf and gateway code through the root
[`buf.gen.yaml`](../../buf.gen.yaml), including Wasm types under
`wasm/x/wasm/types`. It uses gocosmos v0.3.1, grpc-gateway v1.16.0 and Buf
v1.58.0, with the monorepo SDK Any mapping.

[`protocgen.sh`](protocgen.sh) and
[`protoc-swagger-gen.sh`](protoc-swagger-gen.sh) delegate to these commands.
They resolve the root from their own location, independently of the caller's
working directory. They require Bash, Go and the dependencies of the root
generator.

The Swagger command uses protoc-gen-swagger v1.16.0 and generates only the
Wasm query service into
`build/swagger/wasm/cosmwasm/wasm/v1/query.swagger.json`. Both commands resolve
imports through the root [`buf.yaml`](../../buf.yaml) workspace, including
`wasm/proto`, `sdk/proto` and `sdk/third_party/proto`.

The Swagger output is a generated schema artifact. The node's combined
[`docs/swagger-ui/swagger.yml`](../../docs/swagger-ui/swagger.yml) and embedded
bundle are published separately.

The repository uses the [Regen Network protobuf fork](https://github.com/regen-network/protobuf).
See the `replace` directive for `github.com/gogo/protobuf` in the root
[`go.mod`](../../go.mod) for its version.
