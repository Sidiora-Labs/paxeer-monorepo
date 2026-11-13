<!-- Source: https://docs.paxeer.app/sdk/ -->

# Cosmos SDK Fork

Paxeer X’s in-tree Cosmos SDK fork with Paxeer X-specific modifications and extensions.

**Source:** `sdk/`

## Choose the right development surface

The in-tree Cosmos SDK powers `paxd` and native chain modules. The [XDK](https://docs.paxeer.app/xdk) provides application clients for node transactions, receipts, identity, and native precompiles. The [platform API](https://docs.paxeer.app/platform-api) connects paid resources, merchant checkout, agent spending, receipts, and webhooks.

Chain maintainers work in `sdk/`, `modules/`, and `node/`. Application developers use `services/agent/sdk/` (TypeScript, Python), `services/agent/crates/layerx-sdk` (Rust), `sdk/platform/` (Go, JVM, Swift, .NET), and `sdk/wallet`. `docs/system/sdk.md` indexes exported SDK symbols and their source paths.

### Cross-surface application lifecycle

1. Resolve the account identity and select the native asset denomination or ERC-20 pointer.
2. Use the matching client or native ABI to submit a signed transaction for the intended operation.
3. Track the transaction hash, execution receipt, and covering committed block.
4. Verify receipt signatures and committed inclusion before treating a payment as final.

The [network overview](https://docs.paxeer.app/unified-network) explains how these execution surfaces compose. The kernel executes within the BFT network; an execution receipt becomes final when its covering block is committed.

## Overview

Paxeer X vendors a fork of the Cosmos SDK directly in the monorepo under `sdk/`. These packages are **part of** the root Go module. It is an in-tree dependency for the `paxd` binary and Paxeer chain modules.

### Fork capabilities

Paxeer X maintains a fork to support:

- Custom storage engines

- EVM integration and pointer contracts

- Chain-specific module and precompile integration

- Paxeer X-specific consensus and execution optimizations

- Co-location with Paxeer modules and WASM runtime

## Upstream Base

The fork includes its own application framework, stores, account types, and module implementations. Its imports resolve through the root Go module.

**Warning:** Do not assume drop-in compatibility with standard Cosmos SDK tooling or libraries. Paxeer X’s SDK is a custom fork.

## Directory Structure

| Path | Purpose |
| --- | --- |
| `baseapp/` | Application framework (ABCI, routing, ante handlers) |
| `client/` | CLI framework and transaction builders |
| `server/` | Node server, gRPC, REST gateway |
| `types/` | Core SDK types (messages, coins, errors, context) |
| `x/` | Standard Cosmos modules (bank, auth, staking, gov, etc.) |
| `proto/` | Protobuf definitions for SDK modules |
| `store/` | State store abstraction and implementations |
| `storev2/`, `codec/`, `crypto/` | Store v2, codec, and cryptography support |
| `platform/`, `wallet/`, `js/` | Platform language SDKs, wallet SDK, and the first-party @pax-js/cosmos package |

## Key Modules

Paxeer X’s SDK includes standard Cosmos modules:

| Module | Purpose |
| --- | --- |
| `x/auth` | Account authentication and transaction signing |
| `x/bank` | Token transfers and balance management |
| `x/staking` | Proof-of-Stake validator set and delegation |
| `x/distribution` | Staking rewards and fee distribution |
| `x/gov` | On-chain governance and proposals |
| `x/slashing` | Validator penalties for misbehavior |
| `x/authz`, `x/feegrant` | Authorization grants and fee grants |
| `x/capability`, `x/genutil`, `x/simulation` | Capability keeper, genesis utilities, and simulation |
| `x/evidence` | Double-sign evidence submission |
| `x/params` | Module parameter management |
| `x/upgrade` | Coordinated chain upgrades |

Chain module source directories under `modules/`:

- `evm` — EVM execution and JSON-RPC

- `epoch` — Epoch tracking

- `oracle`, `pricefeed` — Price feeds

- `tokenfactory` — Custom token creation

- `mint` — Token minting

- `launchpad`, `spot`, `perps`, `sequencer`, `store`, `xweb`, `lxphost`

- `layerxagent`, `layerxanchor`, `layerxauth`, `layerxbridge`, `layerxchains`, `layerxcustody`, `layerxexchange`, `layerxgov`, `layerxledger`, `layerxpay`

## BaseApp

The core application framework (`baseapp/`) handles:

- ABCI interface to Tendermint consensus

- Message routing to module handlers

- Ante handlers (gas, signatures, nonces)

- Transaction execution and state commits

- Query routing

Paxeer extends BaseApp for EVM transaction routing and concurrent execution (OCC).

## Client & Server

The SDK provides:

- **CLI:** `paxd` command-line interface (transaction builders, queries)

- **gRPC:** Module query and transaction services

- **REST:** HTTP gateway (gRPC-Gateway)

- **Tendermint RPC:** Block and transaction queries

See [REST & gRPC](https://docs.paxeer.app/rest-grpc) for API documentation.

## State Store

The SDK's `sdk/store/` abstraction is implemented by Paxeer X’s custom storage engines. The engine implementations live under `storage/`:

- Database engines under `storage/db_engine/`: `litt`, `pebbledb`, `rocksdb`, and `dbcache`.

See [Storage](https://docs.paxeer.app/storage) for engine details.

## Protobuf Definitions

SDK types are defined in `sdk/proto/`. Regenerate Go code with:

```
# From the repository root
bash scripts/protoc.sh
```

The script builds the pinned `protoc-gen-gocosmos` plugin and runs `buf generate` via `go run github.com/bufbuild/buf/cmd/buf@v1.58.0`. See `api/README.md`.

## Building Against the SDK

Paxeer modules import the SDK from the in-tree path:

```
import (
    sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
    "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
)
```

There is no `sdk/go.mod`: the SDK packages belong to the single root module `github.com/Sidiora-Labs/Paxeer-X-Network`, so no replacement directives are needed.

## Makefile Targets

The repository Makefile includes SDK-related targets:

```
# Build paxd from the unified repository root
make paxeer-build

# Run the chain tests
make paxeer-test
```

## Documentation

Use `docs/system/sdk.md` for exported SDK symbols and source paths. The first-party @pax-js/cosmos package in `sdk/js` is edited in place and aligned to the protos of this repository.

### Upstream Resources

- [Cosmos SDK Docs](https://docs.cosmos.network/) (upstream, may diverge from Paxeer)

- [Cosmos Tutorials](https://tutorials.cosmos.network/) (upstream concepts apply)

- [GoDoc](https://pkg.go.dev/github.com/cosmos/cosmos-sdk) (upstream SDK, not Paxeer X’s fork)

**Warning:** Upstream documentation describes the standard Cosmos SDK, not Paxeer X’s fork. Treat it as reference only.

## Contributing to the Fork

Changes to `sdk/` affect the entire Paxeer chain. Test thoroughly:

1. Edit code under `sdk/`
2. Rebuild `paxd`: `make paxeer-build`
3. Run chain tests: `make paxeer-test`
4. Test with [Docker cluster](https://docs.paxeer.app/docker): `make docker-cluster-start`
5. Run Human service integration tests: `make human-test-integration`

## Differences from Upstream Cosmos SDK

Paxeer X’s SDK fork diverges from upstream in several areas:

- Custom storage backends

- EVM module integration and pointer contracts

- Optimistic concurrency control (OCC) for parallel execution

- Receipt indexing and synthetic transactions

- Paxeer X-specific ante handlers and gas metering

- Chain-specific upgrade logic

Do not assume compatibility with standard Cosmos SDK tooling (e.g., CosmJS, Keplr) without testing.

## Go Module Path

```
github.com/Sidiora-Labs/Paxeer-X-Network
```

There is no separate SDK module. Paths such as `github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types` are package paths inside the single root module, not a standalone published module.
