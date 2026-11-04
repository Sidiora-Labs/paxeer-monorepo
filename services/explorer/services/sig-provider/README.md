# <h1 align="center"> Sig-provider </h1>

**Sig-provider** aggregates function and event signatures and returns candidate
decodings for transaction input and logs. Blockscout uses it to display decoded
transaction data and determine transaction actions on the merged Paxeer network
(EVM chain id **125**).

Supported decoding sources include:

- The local `PrecompileSource`, enabled by default, with signatures embedded in
  [precompiles.json](sig-provider/src/sources/precompiles.json).
- [Openchain signatures](https://openchain.xyz/signatures), configured as `sigeth`.
- [4byte directory](https://www.4byte.directory/).
- [Ethereum bytecode database](https://docs.blockscout.com/about/features/ethereum-bytecode-database-microservice#solution-ethereum-bytecode-database-blockscout-ebd),
  which supplies complete event ABIs when enabled.

## Requirements

The service does not require its own database. External sources need outbound
HTTP access; the embedded precompile lookup itself needs no remote service.

## How to enable

Set these environment variables on the Blockscout backend:

```sh
MICROSERVICE_SIG_PROVIDER_ENABLED=true
MICROSERVICE_SIG_PROVIDER_URL=http://sig-provider:8050
```

The URL must resolve to the sig-provider service from the backend. Its HTTP
listener defaults to `0.0.0.0:8050`; gRPC is disabled by default and uses port 8051
when enabled. This is an explorer decoding API. The network's public JSON-RPC is
served separately at the root of node hosts such as
`https://api1.mainnet-beta.paxeer.network/`, with chain id 125 (`0x7d`).

## Envs

[Settings](sig-provider-server/src/settings.rs) defines service-specific defaults;
[config/base.toml](sig-provider-server/config/base.toml) is the sample file.
Set `SIG_PROVIDER__CONFIG` to load a configuration file. Environment variables
with prefix `SIG_PROVIDER` and `__` separators override file values; missing
settings use Serde defaults.

The workspace declares the registry dependency `blockscout-service-launcher = "0.10.0"`,
resolved to **0.10.1** in `Cargo.lock`. Common server, metrics, tracing
and Jaeger settings are documented by that
[launcher release](https://docs.rs/blockscout-service-launcher/0.10.1/blockscout_service_launcher/),
which is the dependency used here.

[anchor]: <> (anchors.envs.start)

| Variable | Required | Description | Default value |
| --- | --- | --- | --- |
| `SIG_PROVIDER__SOURCES__PRECOMPILES__ENABLED` | | Use the embedded local precompile signatures | `true` |
| `SIG_PROVIDER__SOURCES__FOURBYTE` | | 4byte directory HTTP URL | `https://www.4byte.directory/` |
| `SIG_PROVIDER__SOURCES__SIGETH` | | Openchain signature database HTTP URL | `https://sig.eth.samczsun.com/` |
| `SIG_PROVIDER__SOURCES__ETH_BYTECODE_DB__ENABLED` | | Use the Ethereum bytecode database for complete event ABIs | `true` |
| `SIG_PROVIDER__SOURCES__ETH_BYTECODE_DB__URL` | | Ethereum bytecode database HTTP URL | `https://eth-bytecode-db.services.blockscout.com/` |

[anchor]: <> (anchors.envs.end)

The local source can be configured in TOML:

```toml
[sources.precompiles]
enabled = true
```

Set `SIG_PROVIDER__SOURCES__PRECOMPILES__ENABLED=false` in the service's deployment
environment to disable it. The backend's `MICROSERVICE_*` variables enable the
client integration; the `SIG_PROVIDER__*` variables configure the service itself.

## Local precompile signatures

[PrecompileSource](sig-provider/src/sources/precompiles.rs) loads the embedded ABI
once and indexes canonical function selectors and event topics. Lookups accept
hex with or without `0x` and normalize letter case. The inventory is versioned
with the service binary; it is not fetched from the node at runtime and signature
uploads do not modify it.

The [inventory comparison](sig-provider/src/sources/precompiles_abi_test.rs)
currently compares the embedded signatures with the union of these repository
ABI files:

- [addr/abi.json](../../../../precompiles/addr/abi.json)
- [layerxexchange/abi.json](../../../../precompiles/layerxexchange/abi.json)
- [layerxbridge/abi.json](../../../../precompiles/layerxbridge/abi.json)
- [launchpad/abi.json](../../../../precompiles/launchpad/abi.json)

This is the shipped signature inventory, including historical signatures. It is
not a declaration that all these contracts are active, nor a complete inventory
of the merged node's precompiles. The source matches selectors and topics without
an address parameter, so a candidate decoding does not establish which contract
or network produced the data.

[Service construction](sig-provider-server/src/run.rs) puts local signatures
before Openchain and 4byte signatures. The aggregator preserves that priority
when removing duplicate function signatures. Event decoding tries complete ABIs
from the bytecode database before signature-only candidates. The local source
retains signature text, not indexed-argument metadata; signature-only event
layouts are inferred and can be ambiguous.

An unknown selector or topic returns no local candidates and allows other sources
to contribute. A remote source error is logged and omitted by the aggregator;
it is not returned as a distinct unavailable-source result. Consequently an empty
candidate list can also result from failed sources or data that does not decode.
It does not prove that every source was available.

## API

The [HTTP mapping](sig-provider-proto/proto/api_config_http.yaml) and
[protobuf contract](sig-provider-proto/proto/sig-provider.proto) define:

| Method | Route | Purpose |
| --- | --- | --- |
| POST | `/api/v1/signatures` | Submit an ABI to external signature sources asynchronously |
| GET | `/api/v1/abi/function` | Decode `txInput` into candidate function ABIs |
| GET | `/api/v1/abi/event` | Decode comma-separated `topics` and hex `data` |
| POST | `/api/v1/abi/events:batch-get` | Decode a `requests` array of events |
| GET | `/health` | Service health |

Function and event HTTP responses are candidate arrays. Malformed hex or a topic
whose decoded length is not 32 bytes produces gRPC `INVALID_ARGUMENT`. Other
aggregator errors, including transaction input shorter than four bytes, are
mapped to gRPC `INTERNAL` by the service handler. Source lookup failures follow
the logging behavior described above.

## In this repository

From this directory, `cargo check` checks the workspace. Its launcher dependency
comes from the registry, and its service and protobuf crates are local workspace
members. The repository-root
[Dockerfile](../../../../docker/explorer-sig-provider/Dockerfile) builds the
`sig-provider-server` binary; the
[image workflow](../../../../.github/workflows/explorer-images.yml) publishes the
`paxeer-x-explorer-sig-provider` image. The [explorer README](../../README.md)
covers the local stack and lint script.

For environment-table maintenance, see [env-collector](../libs/env-collector/README.md).
Source-code verification is handled by the separate
[smart-contract verifier API](../smart-contract-verifier/smart-contract-verifier-server/README.md#http-api).

## Upstream links

- [Demo](https://sig-provider.services.blockscout.com)
- [Swagger](https://blockscout.github.io/swaggers/services/sig-provider/index.html)
- [Packages](https://github.com/blockscout/blockscout-rs/pkgs/container/sig-provider)
- [Releases](https://github.com/blockscout/blockscout-rs/releases?q=sig-provider&expanded=true)
