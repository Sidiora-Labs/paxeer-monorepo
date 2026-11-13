<!-- Source: https://docs.paxeer.app/rest-grpc/ -->

# REST & gRPC APIs

Cosmos SDK REST and gRPC interfaces for Paxeer chain modules and administration.

**Source:** `api/` proto definitions, `node/app.go` route registration

## Overview

Paxeer exposes Cosmos SDK-style gRPC and REST (gRPC-Gateway) endpoints for chain modules. These APIs complement the [JSON-RPC interface](https://docs.paxeer.app/json-rpc) and provide access to chain state, module parameters, and administrative operations.

### Protocol Buffers

Chain-module and admin service definitions use Protocol Buffers v3 under `api/`. SDK, consensus and Wasm definitions also live in their own trees. Generation provides:

- Go types at each file’s go_package path, including modules, precompiles and admin types

- gRPC server and client interfaces

- REST endpoints via `google.api.http` annotations

- Swagger input from HTTP-annotated query definitions

### Code Generation

Regenerate Go code from proto files, from the repository root:

```
./scripts/protoc.sh
```

The script builds `protoc-gen-gocosmos` from cosmos-proto v0.3.1 and runs `buf generate` through `buf@v1.58.0`. It uses root `buf.yaml` / `buf.gen.yaml` and the two consensus/internal templates, then copies generated Go files into their package directories. See `api/README.md`.

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

Pointer queries resolve native denominations and CW20, CW721, CW1155 or ERC token mappings according to the requested pointer type:

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `Pointer` | `GET /pax-protocol/paxchain/evm/pointer` | Get a registered pointer for the requested type and pointee |
| `Pointee` | `GET /pax-protocol/paxchain/evm/pointee` | Get the registered pointee for the requested pointer type and address |
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
| `DenomAuthorityMetadata` | `GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/authority_metadata` (additional binding: `GET /pax-protocol/paxchain/tokenfactory/denom_authority_metadata`) | Get denom authority metadata |
| `DenomMetadata` | `GET /pax-protocol/paxchain/tokenfactory/denoms/metadata` | Get denom metadata |
| `DenomsFromCreator` | `GET /pax-protocol/paxchain/tokenfactory/denoms_from_creator/{creator}` | List denoms created by an address |
| `DenomAllowList` | `GET /pax-protocol/paxchain/tokenfactory/denoms/allow_list` | Get denom allow list |
| `DenomPaused` | `GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused` (additional binding: `GET /pax-protocol/paxchain/tokenfactory/denom_paused`) | Query the pause status and issuer DID of a denom |

**Proto:** `api/tokenfactory/query.proto`

## Mint Module Query Service

The Mint module exposes minting parameters and state:

| Method | REST Endpoint | Purpose |
| --- | --- | --- |
| `Params` | `GET /paxchain/mint/v1beta1/params` | Get mint module parameters |
| `Minter` | `GET /paxchain/mint/v1beta1/minter` | Get minter state (start/end dates, amounts) |

**Proto:** `api/mint/v1beta1/query.proto`

## Other Module Query Services

The `api/` tree also contains definitions for `launchpad`, `layerxagent`, `layerxpay`, `layerxprograms`, `pricefeed`, `xweb` and other chain modules. Read each service’s `query.proto` and its registration in `node/app.go` before selecting a method. HTTP bindings are declared per method; a gRPC method does not automatically have a REST route. This page details EVM, Epoch, Oracle, TokenFactory and Mint queries.

## Admin gRPC Service

The Admin service provides runtime log level control. It runs on a separate loopback-only gRPC server (default port `9095`) and is disabled by default:

| Method | Purpose |
| --- | --- |
| `SetLogLevel` | Change log level for loggers matching a pattern |
| `GetLogLevel` | Get current log level for a logger |
| `ListLoggers` | List all registered loggers and their levels |

Enable in `app.toml`, replacing the address placeholder with a loopback IP and port:

```
[admin_server]
admin_enabled = true
admin_address = "<loopback-ip>:9095"
```

**Proto:** `api/pax/admin/v0/admin.proto`
 **Implementation:** `admin/`

## OpenAPI / Swagger Documentation

The Swagger script generates OpenAPI documentation with Ignite, then embeds prepared assets with statik. To regenerate from the repository root:

```
./scripts/update-swagger-ui-statik.sh
```

The script prepares `docs/swagger-ui/`, including `swagger.yml`, and writes the embedded assets to `docs/swagger/statik.go`. Use `./scripts/update-swagger-ui-statik.sh --assets-only` to embed existing assets without OpenAPI generation.

### Serving Swagger UI

Enable in `app.toml`:

```
[api]
enable = true
swagger = true
```

The enabled API listener serves the embedded assets at `/swagger/`.

**See:** `scripts/update-swagger-ui-statik.sh` and `scripts/swagger_assets.py`

## gRPC Endpoints

Enabled gRPC listeners use port `9090` by default. Set `GRPC_ADDRESS` to your listener’s host and port; it is an example client variable. For a plaintext listener, `grpcurl` can list services and request epoch parameters:

```
grpcurl -plaintext "$GRPC_ADDRESS" list
grpcurl -plaintext "$GRPC_ADDRESS" paxprotocol.paxchain.epoch.Query/Params
```

## REST Gateway

REST query routes are served via gRPC-Gateway on the API listener, whose default port is `1317`. The API is enabled by the full and archive initialization profiles. Only registered methods with HTTP bindings have the annotated REST paths.

## Transaction Services

State-changing messages are declared in `Msg` services, including `api/evm/tx.proto` and `api/tokenfactory/tx.proto`. These differ from query methods. Encode and sign the appropriate transaction, then submit through the Cosmos SDK transaction service.
