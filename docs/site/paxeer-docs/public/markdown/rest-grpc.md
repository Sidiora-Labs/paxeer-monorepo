<!-- Source: https://docs.paxeer.app/rest-grpc/ -->

# REST & gRPC APIs

Cosmos SDK REST and gRPC interfaces for Paxeer chain modules and administration.

**Source:** `api/` proto definitions, `docs/`

## Overview

Paxeer exposes Cosmos SDK-style gRPC and REST (gRPC-Gateway) endpoints for chain modules. These APIs complement the [JSON-RPC interface](https://docs.paxeer.app/json-rpc) and provide access to chain state, module parameters, and administrative operations.

### Protocol Buffers

All services are defined in `api/` using Protocol Buffers v3. The proto files generate:

- Go types under `modules/*/types`

- gRPC server and client interfaces

- REST endpoints via `google.api.http` annotations

- OpenAPI/Swagger documentation

### Code Generation

Regenerate Go code from proto files, from the repository root:

```
./scripts/protoc.sh
```

The script builds the pinned `protoc-gen-gocosmos` plugin and runs `buf generate` (through `buf@v1.58.0`) with the root `buf.yaml` / `buf.gen.yaml` configuration — the only Buf configuration for `api/` — then copies the generated Go files into their module directories. See `api/README.md`. Ignite is not used.

## EVM Module Query Service

The EVM module provides address mapping, static calls, and pointer queries.

### Address Mapping

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `PaxAddressByEVMAddress` | `GET /pax-protocol/paxchain/evm/pax_address` | Get Cosmos address from EVM address |
| `EVMAddressByPaxAddress` | `GET /pax-protocol/paxchain/evm/evm_address` | Get EVM address from Cosmos address |

### Static Calls

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `StaticCall` | `GET /pax-protocol/paxchain/evm/static_call` | Execute read-only contract call |

### Pointer Queries

Pointers bridge Cosmos-native assets (CW20/CW721/CW1155) to EVM addresses:

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `Pointer` | `GET /pax-protocol/paxchain/evm/pointer` | Get EVM pointer for Cosmos contract |
| `Pointee` | `GET /pax-protocol/paxchain/evm/pointee` | Get Cosmos contract from EVM pointer |
| `PointerVersion` | `GET /pax-protocol/paxchain/evm/pointer_version` | Get current pointer contract version |

**Proto:** `api/evm/query.proto`

## Epoch Module Query Service

The Epoch module tracks the current chain epoch for time-based operations:

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `Epoch` | `GET /pax-protocol/paxchain/epoch/epoch` | Get current epoch number and metadata |
| `Params` | `GET /pax-protocol/paxchain/epoch/params` | Get epoch module parameters |

**Proto:** `api/epoch/query.proto`

## Oracle Module Query Service

The Oracle module provides exchange rates, price feeds, and validator vote tracking:

### Exchange Rates & Prices

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `ExchangeRate` | `GET /pax-protocol/pax-chain/oracle/denoms/{denom}/exchange_rate` | Get exchange rate for a denom |
| `ExchangeRates` | `GET /pax-protocol/pax-chain/oracle/denoms/exchange_rates` | Get all exchange rates |
| `Actives` | `GET /pax-protocol/pax-chain/oracle/denoms/actives` | List all active denoms |
| `VoteTargets` | `GET /pax-protocol/pax-chain/oracle/denoms/vote_targets` | List vote target denoms |
| `PriceSnapshotHistory` | `GET /pax-protocol/pax-chain/oracle/denoms/price_snapshot_history` | Get historical price snapshots |
| `Twaps` | `GET /pax-protocol/pax-chain/oracle/denoms/twaps/{lookback_seconds}` | Get time-weighted average prices |

### Validator Vote Tracking

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `FeederDelegation` | `GET /pax-protocol/pax-chain/oracle/validators/{validator_addr}/feeder` | Get feeder delegation for validator |
| `VotePenaltyCounter` | `GET /pax-protocol/pax-chain/oracle/validators/{validator_addr}/vote_penalty_counter` | Get oracle miss counter |
| `SlashWindow` | `GET /pax-protocol/pax-chain/oracle/slash_window` | Get slash window information |
| `Params` | `GET /pax-protocol/pax-chain/oracle/params` | Get oracle module parameters |

**Proto:** `api/oracle/query.proto`

## TokenFactory Module Query Service

The TokenFactory module manages custom token creation and metadata:

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `Params` | `GET /pax-protocol/paxchain/tokenfactory/params` | Get tokenfactory parameters |
| `DenomAuthorityMetadata` | `GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/authority_metadata` (additional binding: `GET …/tokenfactory/denom_authority_metadata`) | Get denom authority metadata |
| `DenomMetadata` | `GET /pax-protocol/paxchain/tokenfactory/denoms/metadata` | Get denom metadata |
| `DenomsFromCreator` | `GET /pax-protocol/paxchain/tokenfactory/denoms_from_creator/{creator}` | List denoms created by an address |
| `DenomAllowList` | `GET /pax-protocol/paxchain/tokenfactory/denoms/allow_list` | Get denom allow list |
| `DenomPaused` | `GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused` (additional binding: `GET …/tokenfactory/denom_paused`) | Query the pause status and issuer DID of a denom |

**Proto:** `api/tokenfactory/query.proto`

## Mint Module Query Service

The Mint module exposes minting parameters and state:

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `Params` | `GET /paxchain/mint/v1beta1/params` | Get mint module parameters |
| `Minter` | `GET /paxchain/mint/v1beta1/minter` | Get minter state (start/end dates, amounts) |

**Proto:** `api/mint/v1beta1/query.proto`

## Other Module Query Services

The services above are not the whole REST/gRPC surface. The `api/` tree also defines `Query` services with `google.api.http` bindings for `launchpad`, `layerxagent`, `layerxpay`, `layerxprograms`, `pricefeed` and `xweb`, alongside the `layerxanchor`, `layerxauth`, `layerxbridge`, `layerxcustody`, `layerxexchange`, `layerxgov`, `layerxledger`, `spot`, `perps` and `eth` proto trees. Read each module's `query.proto` for its methods and REST paths; this page details the EVM, Epoch, Oracle, TokenFactory and Mint services.

## Admin gRPC Service

The Admin service provides runtime log level control. It runs on a separate loopback-only gRPC server (default `127.0.0.1:9095`):

| Method | Purpose |
| --- | --- |
| `SetLogLevel` | Change log level for loggers matching a pattern |
| `GetLogLevel` | Get current log level for a logger |
| `ListLoggers` | List all registered loggers and their levels |

Enable in `app.toml`:

```
[admin_server]
admin_enabled = true
admin_address = "127.0.0.1:9095"
```

**Proto:** `api/pax/admin/v0/admin.proto`
 **Implementation:** `admin/`

## OpenAPI / Swagger Documentation

Paxeer generates OpenAPI documentation from proto annotations. To regenerate:

```
./scripts/update-swagger-ui-statik.sh
```

This generates `docs/swagger-ui/swagger.yml` and embeds it in `docs/swagger/statik.go`.

### Serving Swagger UI

Enable in `app.toml`:

```
[api]
enable = true
swagger = true
```

Access at `http://<node-ip>:<port>/swagger/`.

**See:** `docs/README.md` for generation instructions

## gRPC Endpoints

gRPC services are exposed on the Cosmos SDK gRPC port (default `9090`). Use any gRPC client (e.g., `grpcurl`):

```
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 paxprotocol.paxchain.evm.Query/Pointer
```

## REST Gateway

REST endpoints are served via gRPC-Gateway on the API port (default `1317`). All gRPC methods have corresponding REST endpoints defined by `google.api.http` annotations in the proto files.

## Transaction Services

Each module also defines a `Msg` service for transactions (e.g., `evm/tx.proto`, `epoch/tx.proto`). These are not query endpoints but message types for state-changing operations. Submit via standard Cosmos SDK transaction APIs.
