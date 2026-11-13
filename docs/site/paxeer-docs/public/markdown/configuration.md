<!-- Source: https://docs.paxeer.app/configuration/ -->

# Configuration

The current paxd configuration files, node modes, RPC settings, and storage controls.

## Configuration across the unified network

The chain configuration below controls `paxd`. Hosted service configuration is maintained separately under `services/platform/hosted/` and the component workspaces. Paxeer mainnet-beta is one BFT chain with EVM chain ID 125. Its kernel runs within the network. Configure hosted dependencies for that network and preserve their service-specific authorization settings.

| Surface | Configure | Boundary |
| --- | --- | --- |
| Core boundary | Network ID, node and replica connections, TLS material, and separate admin credentials | Core and admin listeners are separate planes |
| Gateway | Public TLS, upstream dependencies, receipt verification, request policy, and shared coordination storage | Public application ingress |
| Registry | Published service identities, network records, route destinations, and signing policy | Discovery records identify the authorized network services |
| Node | Consensus identity, peer topology, execution, storage retention, and API listeners | Block commitment establishes receipt finality |
| Agent and Human | Provider destinations, identity and approval policy, key-management channels, and receipt dependencies | User-facing authorization reaches protected provider boundaries |

Generate `config.toml` and `app.toml` using `paxd` as shown below, then review the configured listeners and storage settings. Persistent data and runtime socket directories are distinct. Keep administrative listeners and signer channels off public ingress; route applications through the [gateway API](https://docs.paxeer.app/platform-api). See [on-chain parameters](https://docs.paxeer.app/network-parameters) for governance-controlled economic settings.

## Configuration Files

`paxd init <moniker> --chain-id <chain-id>` generates configuration under `~/.paxeer/config/`. An explicit `--home` changes that location. Generate files using the binary revision you will run, then edit the settings you need.

| File | Purpose |
| --- | --- |
| `config.toml` | Consensus, P2P, mempool, consensus RPC, state-sync client, and validator signer |
| `app.toml` | Application APIs, EVM RPC, execution, snapshots, and storage |
| `client.toml` | CLI node, chain, output, and keyring defaults |
| `genesis.json` | Initial chain identity, consensus parameters, and application state |

## Node Modes

```
paxd init my-node --chain-id hyperpax_125-1 --mode full
```

| Mode | Application profile | History |
| --- | --- | --- |
| `full` | API, gRPC, EVM RPC, and KV transaction index | 100,000 recent SS versions and consensus blocks by default |
| `archive` | Full-node services; consensus mode is written as `full` | SS and block retention set to zero, meaning keep available history |
| `validator` | API, gRPC, EVM RPC, SS queries, and transaction indexing disabled by this profile | State needed to validate and execute |
| `seed` | Discovery profile with a 1,000-connection setting; public application APIs disabled | Peer discovery role |

The default init mode is `full`. It binds consensus RPC and P2P to all interfaces. Validator and seed profiles retain the base loopback listeners; configure the intended peer topology explicitly.

**Source:** `daemon/paxd/cmd/init.go`; `node/params/config.go` — `SetTendermintConfigByMode`, `SetAppConfigByMode`, `SetEVMConfigByMode`.

## Consensus and P2P

These example settings belong in `config.toml`. Replace the angle-bracket listener placeholders with your chosen bind interfaces and use peer IDs from your network. Consensus options use hyphenated keys.

```
log-level = "info"
log-format = "text"

[rpc]
laddr = "tcp://<consensus-listen-host>:26657"

[p2p]
laddr = "tcp://<p2p-listen-host>:26656"
bootstrap-peers = ""
persistent-peers = ""
private-peer-ids = ""
max-connections = 100

[mempool]
broadcast = true
size = 5000
max-txs-bytes = 1073741824
max-tx-bytes = 1048576
```

`max-txs-bytes` limits total mempool bytes; `max-tx-bytes` limits one transaction. Use `bootstrap-peers` for initial discovery, `persistent-peers` for maintained connections, and `max-connections` for the combined peer limit.

**Source:** `consensus/config/config.go` and `consensus/config/toml.go`.

## Application and EVM RPC

These settings belong in `app.toml`. EVM options use underscores, unlike the consensus options above. The host and port are separate EVM fields.

```
minimum-gas-prices = "0.01uhpx"
min-retain-blocks = 100000
occ-enabled = true

[api]
enable = true
swagger = true
address = "tcp://<api-listen-host>:1317"

[grpc]
enable = true
address = "<grpc-listen-host>:9090"

[evm]
http_enabled = true
http_address = "<evm-listen-host>"
http_port = 8545
ws_enabled = true
ws_port = 8546
simulation_gas_limit = 10000000
```

Replace the listener placeholders with your chosen bind interfaces. Set `http_address` and `http_port` for the EVM HTTP listener, and `ws_port` for the WebSocket listener. Configure public routing to match your deployment.

The default worker count is twice the detected CPU count, clamped to 10–128. Override `concurrency-workers` only after measuring your workload. The node admission setting `minimum-gas-prices` is separate from the EVM module's on-chain fee parameters.

**Source:** `rpc/config/config.go`, `rpc/server.go`, `sdk/server/config/config.go`, `daemon/paxd/cmd/root.go`.

## Storage and Retention

Keep root-level settings before TOML section headers. State-store keys have the `ss-` prefix; state-commit keys have `sc-`.

```
# app.toml root setting
min-retain-blocks = 100000

[state-commit]
sc-enable = true
sc-write-mode = "memiavl_only"

[state-store]
ss-enable = true
ss-backend = "pebbledb"
ss-keep-recent = 100000

[receipt-store]
rs-backend = "pebbledb"
```

Archive mode sets both `min-retain-blocks` and `ss-keep-recent` to zero to preserve available history. Start from data covering the historical range you need. RocksDB is an optional SS backend that requires a compatible build; receipt storage uses Pebble.

Storage routing modes control state migration. Select the mode specified by your chain's migration plan and existing data layout. See [Storage](https://docs.paxeer.app/storage).

**Source:** `storage/config/toml.go`, `storage/config/sc_config.go`, `storage/config/ss_config.go`, `storage/config/receipt_config.go`.

## Snapshots and State Sync

A snapshot provider sets `[state-sync]` in `app.toml`. A joining node enables `[statesync]` in `config.toml`. These are different roles.

```
# app.toml: application snapshot production
[state-sync]
snapshot-interval = 0
snapshot-keep-recent = 2

# config.toml: snapshot consumption
[statesync]
enable = false
rpc-servers = ""
trust-height = 0
trust-hash = ""
```

Snapshot interval zero disables periodic application snapshots. Before enabling a joining node, supply trusted RPC servers, a valid checkpoint height and hash, and the required trust period. See [Run a Node](https://docs.paxeer.app/run-node).

## Metrics

```
# config.toml
[instrumentation]
prometheus = true
prometheus-listen-addr = "<metrics-listen-host>:26660"
```

Read `/metrics` on the configured listener. Application telemetry is configured separately in `app.toml` under `[telemetry]`.

## Validation

Review generated files using the selected binary's help and configuration templates. Start the configured node with `paxd start`, check its startup logs, then confirm its chain identity and synchronization with `paxd status`.
