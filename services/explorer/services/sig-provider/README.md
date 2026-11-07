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

The revisioned [embedded inventory](sig-provider/src/sources/precompiles.json)
contains an `inventory` manifest alongside the `abi` array. Each manifest entry
records its `name`, repository `origin` (or Go file and ABI constant), and `status`:

- `versioned`: ABI files declared by `precompileInfoSources` in the node's
  [precompile registry](../../../../precompiles/setup.go), including bank, staking,
  launchpad, perps, pricefeed and spot. The generator checks the
  `GetCustomPrecompilesE` constructors and the `v6120Contracts` version switch in
  [node/app.go](../../../../node/app.go).
- `merge`: ABIs selected by `MergeAddressTable`, including ledger, pay, auth,
  programs and chains, plus the served ABI constants in
  [layerxagent](../../../../precompiles/layerxagent). Constructor ABIs include
  wrapper-added events; cold metadata alone is insufficient.
- `historical`: the previous exchange ABI, the
  [launchpad predecessor](../../../../precompiles/launchpad/legacy/v610/abi.json),
  and the [audit snapshot](sig-provider/src/sources/precompiles_historical.json)
  retained for decoding older transactions.

The [generator](sig-provider/src/sources/generate_precompiles.py) derives this
union from the repository's real producers, keeping overloads and distinct
indexed event layouts. From this directory, regenerate it with
`python3 sig-provider/src/sources/generate_precompiles.py`; `--stdout` emits the
same inventory without writing. The
[inventory comparison](sig-provider/src/sources/precompiles_abi_test.rs) checks
the embedded signatures against the declared producer and historical union.
The Git revision of `precompiles.json`, shipped with the service binary, identifies
the decoding inventory. A node ABI change requires regeneration and a new service
binary to update local decoding.

Inventory status describes provenance, not current executability: a versioned
entry can expose a retired refusal surface. Consult the node's upgrade/version
selection for active contracts at a given height. The source matches selectors
and topics without an address parameter, so a candidate decoding does not
establish which contract or network produced the data.

[Service construction](sig-provider-server/src/run.rs) puts local signatures
before Openchain and 4byte signatures, and local complete event ABIs before the
bytecode database. The aggregator preserves priority when removing duplicates.
The local source retains complete event ABIs, including argument names, order
and indexed flags. Available complete ABIs determine exact layouts; only events
without complete ABIs use inferred signature-only layouts, which can be ambiguous.
Signature uploads do not change the read-only local inventory.

An **unknown selector or topic** returns no local candidates. In the aggregator's
coverage report this is `OutsideCoverage`, allowing external directories to
contribute. A successful lookup can return an empty candidate list for an
unknown key or data that does not decode; a successful source also allows partial
results when another source fails. Such a result does not prove every source was
available. The `*_with_coverage` methods expose `Succeeded`, `Failed` and
`OutsideCoverage` outcomes per source.

An **unavailable-source outcome** occurs when a capable source fails and no
capable source succeeds. The aggregator returns typed `LookupUnavailable`
(`signature lookup unavailable`), rather than converting the failure to an empty
candidate list. A miss in the finite local inventory does not count as a
successful directory lookup. The current service maps this lookup error to gRPC
`INTERNAL` (HTTP 500); the candidate-array API does not expose the internal
coverage report. A known local signature can still decode while external sources
are unavailable.

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
the coverage and unavailable-source behavior described above.

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
