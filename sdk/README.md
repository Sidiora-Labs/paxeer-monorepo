# sdk

`sdk/` contains the Cosmos SDK fork embedded by `paxd` and the network's client SDKs. Paxeer X Network is one BFT network with EVM chain ID `125` (`0x7d`). The LayerX kernel runs inside `paxd`, participating in block proposal and execution. Finality occurs when the block covering a transaction is committed.

The Go fork supplies application execution, stores, modules, encoding and client context. [`platform/`](platform/) contains language clients for native transactions and receipt verification; [`chainflow/`](chainflow/) contains chain-facing clients. They use the same network and transaction identity.

## Native transactions and receipts

Construct and sign an EIP-1559 (type 2) transaction for chain ID `125`, then submit its exact bytes with `eth_sendRawTransaction`. Native precompiles route calls through the modules and embedded kernel; the Go Programs client implements this path in [`platform/go/programs.go`](platform/go/programs.go). Keep the signed bytes and returned transaction hash.

Query `pax_sequencerReceipt` with that hash. Its evidence progresses through `submitted`, `receipted` and `committed`; a signed receipt alone does not establish block commitment. Verify receipt signatures against independently trusted receipt keys, bind the receipt to the transaction, and verify committed block evidence before claiming finality. Inspect the execution status as well: a committed failure remains a failure. See [receipt evidence](../docs/site/docs/concepts/receipts.md).

Connect to the node JSON-RPC at the root HTTPS endpoint `https://api-mainnet-beta.paxeer.network` or root WebSocket endpoint `wss://api-mainnet-beta.paxeer.network`. Check `eth_chainId` before signing. The [public node endpoint reference](../docs/site/docs/reference/public-rpc.md) lists numbered nodes and method availability; hosted gateway methods have their own contract.

The code here is part of the root Go module, `github.com/Sidiora-Labs/Paxeer-X-Network` (see [`go.mod`](../go.mod)). Other packages import it as `github.com/Sidiora-Labs/Paxeer-X-Network/sdk/...`. Only [`cosmovisor/`](cosmovisor/README.md) and `ics23/` have their own `go.mod`.

## Layout

| Path | What it holds |
| ---- | ------------- |
| `baseapp/` | The ABCI application base that `paxd` embeds: transaction execution, commit, halt handling and state sync snapshots |
| `tasks/` | The concurrent transaction scheduler (`tasks.NewScheduler`) that `node/` uses for parallel `DeliverTx` |
| `store/` | KV store wrappers and the original `rootmulti` multistore ([README](store/README.md)) |
| `storev2/` | The `rootmulti` multistore that `paxd` mounts, built on the state stores in [`storage/`](../storage/README.md) |
| `snapshots/` | State sync snapshot manager and on-disk snapshot store ([README](snapshots/README.md)) |
| `server/` | The `start`, `export`, `rollback` and `tendermint` commands and `app.toml` handling ([README](server/README.md)) |
| `x/` | Base modules: auth, authz, bank, capability, distribution, evidence, feegrant, genutil, gov, params, slashing, staking, upgrade ([README](x/README.md)) |
| `client/`, `codec/`, `crypto/`, `types/`, `std/`, `version/`, `telemetry/`, `utils/` | CLI client context, encoding, keys and keyring, core types, version command, metrics, helpers |
| `proto/`, `third_party/` | Protobuf definitions for the SDK types and the vendored proto dependencies |
| `ics23/` | A copy of the ICS-23 proof library (module `github.com/confio/ics23/go`). The root `go.mod` resolves that path through its `replace` directive, not through this directory |
| `cosmovisor/` | Process manager that swaps binaries at upgrade heights ([README](cosmovisor/README.md)) |
| `contrib/`, `scripts/` | Developer tooling inherited from upstream ([scripts README](scripts/README.md)) |
| `testutil/`, `tests/` | Test helpers, fixtures and mocks |

The chain application that wires these modules together is [`node/app.go`](../node/app.go). Chain-specific modules (for example `evm`, `mint`, `oracle`, `tokenfactory` and the `layerx*` modules) live in [`modules/`](../modules/README.md). IBC lives in [`interchain/`](../interchain/README.md). The consensus engine is in [`consensus/`](../consensus/README.md).

## Build and test

`paxd` is built from the repository root with the chain makefile:

```bash
make -f chain.mk build      # writes ./build/paxd
make -f chain.mk install    # installs paxd into $GOPATH/bin
```

The SDK package build and install targets also run from `sdk/`, using the root Go manifests and fork linker packages:

```bash
make -C sdk build
make -C sdk install
```

Run the SDK unit tests with the build tags the SDK makefile uses:

```bash
make -C sdk test-unit
```

or with plain Go from the repository root:

```bash
go test ./sdk/...
```

## Documentation

- Hosted documentation: [docs.paxeer.app](https://docs.paxeer.app/)
- Repository layout and build ownership: [`docs/MONOREPO.md`](../docs/MONOREPO.md)
- Module specifications: the `spec/` directory under each module in [`x/`](x/README.md)
