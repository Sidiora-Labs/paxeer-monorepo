# Fuzz Tests

Native Go fuzz targets exercise packages of the consensus engine embedded in Paxeer's `paxd`. These targets use an in-process test application, paired connections or an HTTP test handler; they do not require a running testnet. For live consensus-network invariants, see the [end-to-end harness](../e2e/README.md).

The entrypoints in [`tests/`](tests/) cover:

- `FuzzMempool`: submits arbitrary bytes to mempool `CheckTx` using an in-process kvstore application.
- `FuzzP2PSecretConnection`: checks that bytes written through a paired secret connection are read back unchanged.
- `FuzzRPCJSONRPCServer`: sends arbitrary request bodies to a registered JSON-RPC handler and checks that nonempty responses decode as JSON-RPC responses, individually or in a batch.

## Running

Use the toolchain required by the repository-root [`go.mod`](../../../go.mod), which declares Go 1.25.8. Run these commands from `consensus/test/fuzz/`:

```sh
go test -tags=gofuzz -fuzz='^FuzzMempool$' ./tests
go test -tags=gofuzz -fuzz='^FuzzP2PSecretConnection$' ./tests
go test -tags=gofuzz -fuzz='^FuzzRPCJSONRPCServer$' ./tests
```

Every fuzz source file has a `//go:build gofuzz` constraint, so `-tags=gofuzz` is required to include the entrypoints. Each anchored selector names exactly one fuzz function. Run one command at a time; fuzzing continues until interrupted unless you set a limit, such as `-fuzztime=30s`.

See the [Go fuzzing introduction](https://go.dev/doc/fuzz/) for corpus management and replaying failing inputs.

Use the tagged native commands above for this repository. The targets and their seed corpora are under [`tests/`](tests/).
