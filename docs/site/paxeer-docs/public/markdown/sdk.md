<!-- Source: https://docs.paxeer.app/sdk/ -->

# Cosmos SDK Fork

Paxeer X’s in-tree Cosmos SDK fork with Paxeer X-specific modifications and extensions.

**Source:** `Paxeer-X-Network/sdk/`

## Choose the right development surface

Paxeer X has three complementary SDK surfaces. The in-tree Cosmos SDK powers `paxd` and native chain modules. The [XDK](https://docs.paxeer.app/xdk) provides application-facing clients for LayerX transactions, proofs, identity, exchange, bridge, launchpad, and native precompile calls. The [platform API](https://docs.paxeer.app/platform-api) connects paid resources, merchant checkout, agent spending, receipts, and webhooks.

Chain maintainers work in `sdk/`, `modules/`, and `node/`. Application developers use `services/agent/sdk/` (TypeScript, Python), `services/agent/crates/layerx-sdk` (Rust), `sdk/platform/` (Go, JVM, Swift, .NET), and `sdk/wallet`. `docs/system/sdk.md` indexes exported SDK symbols and their source paths.

### Cross-surface application lifecycle

1. Resolve the unified EVM, Cosmos, and LayerX identity and select the asset denomination or pointer.
2. Use the matching client or native ABI to deposit custody funds, submit an intent, transfer an asset, or create a market.
3. Track transaction receipts together with kernel receipts, checkpoint finality, and withdrawal or settlement status.
4. Verify proofs against the appropriate finalized root before fulfilling a payment or releasing funds.

The [network overview](https://docs.paxeer.app/unified-network) explains how these execution surfaces compose. An EVM transaction receipt and a finalized kernel checkpoint represent different stages of an application lifecycle.

## Overview

Paxeer X vendors a fork of the Cosmos SDK directly in the monorepo under `Paxeer-X-Network/sdk/`. This is **not** a published Go module or npm package. It is an in-tree dependency for the `paxd` binary and Paxeer chain modules.

### Why a Fork?

Paxeer X maintains a fork to support:

- Custom storage engines

- EVM integration and pointer contracts

- Fast iteration without waiting for upstream releases

- Paxeer X-specific consensus and execution optimizations

- Co-location with Paxeer modules and WASM runtime

## Upstream Base

The fork is based on Cosmos SDK but diverges significantly. It is not compatible with upstream Cosmos SDK releases without careful rebasing and testing.

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
| `platform/`, `wallet/`, `js/` | Platform language SDKs, wallet, and JavaScript support |

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

Plus Paxeer X-specific modules under `Paxeer-X-Network/modules/`:

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

The SDK's `store/` abstraction is implemented by Paxeer X’s custom storage engines. The engine implementations live under `storage/`:

- Database engines under `storage/db_engine/`: `litt`, `pebbledb`, `rocksdb`, and `dbcache`.

See [Storage](https://docs.paxeer.app/storage) for engine details.

## Protobuf Definitions

SDK types are defined in `sdk/proto/`. Regenerate Go code with:

```
cd Paxeer-X-Network
./scripts/protoc.sh
```

The script builds the pinned `protoc-gen-gocosmos` plugin and runs `buf generate` via `go run github.com/bufbuild/buf/cmd/buf@v1.58.0`. See `Paxeer-X-Network/api/README.md`.

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

There is no `sdk/docs/` tree in the fork. Use `docs/system/sdk.md` in the monorepo as the index of exported SDK symbols and source paths.

### Upstream Resources

- [Cosmos SDK Docs](https://docs.cosmos.network/) (upstream, may diverge from Paxeer)

- [Cosmos Tutorials](https://tutorials.cosmos.network/) (upstream concepts apply)

- [GoDoc](https://pkg.go.dev/github.com/cosmos/cosmos-sdk) (upstream SDK, not Paxeer X’s fork)

**Warning:** Upstream documentation describes the standard Cosmos SDK, not Paxeer X’s fork. Treat it as reference only.

## Contributing to the Fork

Changes to `Paxeer-X-Network/sdk/` affect the entire Paxeer chain. Test thoroughly:

1. Edit code under `sdk/`
2. Rebuild `paxd`: `make paxeer-build`
3. Run unit tests: `make test`
4. Test with [Docker cluster](https://docs.paxeer.app/docker): `make docker-cluster-start`
5. Run integration tests: `make human-test-integration` (there is no `test-integration` target)

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
