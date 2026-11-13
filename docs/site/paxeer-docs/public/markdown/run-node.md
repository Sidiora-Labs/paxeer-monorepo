<!-- Source: https://docs.paxeer.app/run-node/ -->

# Run a Node

Initialize a Paxeer X node, choose its role, connect to a network, and verify synchronization.

## Chain node and hosted services

The chain node is `paxd`. Hosted services (gateway, core, authority, registry, Human and related components) live under `services/platform/hosted/` in the monorepo, with the hosted node under `services/platform/hosted/node/`; each has its own configuration, credentials, and state. This page covers the `paxd` chain node.

The former `layerxd` C daemon (`cmd/layerxd`) is retired: its sources are absent from the build and retirement tests assert that absence. There is no `layerxd --check-config`, `--serve`, or `--authority-replica` startup path. Order startup so dependencies exist first, then verify synchronization and an end-to-end receipt before admitting traffic. See [operators](https://docs.paxeer.app/operators), [configuration](https://docs.paxeer.app/configuration), [relay/archive status](https://docs.paxeer.app/relay-archive), and [kernel architecture](https://docs.paxeer.app/layerx-kernel).

## Before You Start

Install a `paxd` revision compatible with the network you intend to join. Obtain its genesis file, peer information, and any snapshot or state-sync checkpoint from that network's operator. Keep EVM RPC and consensus RPC endpoints separate.

Size disk, memory, and CPU for your retained history and query workload. See [Installation](https://docs.paxeer.app/installation) for build requirements and [Configuration](https://docs.paxeer.app/configuration) for node profiles.

## Node Types

- **Full:** Serves application and EVM queries and retains recent history by default.

- **Archive:** Serves full-node APIs and retains available historical state and blocks.

- **Validator:** Runs the signing node profile; active-set membership also requires an on-chain validator and sufficient stake.

- **Seed:** Uses the peer-discovery profile.

## Initialize Node

```
paxd init my-node --chain-id hyperpax_125-1 --mode full
```

The default home is `~/.paxeer`. Init writes configuration and a local genesis. To join an existing network, install that network's verified genesis before starting. Validator operators should select `--mode validator` before configuring the signer and peer topology.

## Genesis and Peers

Place the verified network genesis at `~/.paxeer/config/genesis.json`. Confirm its chain ID and expected checksum. Configure `bootstrap-peers` and `persistent-peers` under `[p2p]` in `config.toml` using the network's `node-id@host:26656` entries.

[HPX tooling](https://docs.paxeer.app/admin-hpx) provides distribution manifests, genesis, and peer registry handling for managed node setup.

## Sync Methods

### Block Replay

A node with compatible genesis and an available block history can replay blocks. Use the correct binaries and upgrades for that history.

### State Sync

State sync imports an application snapshot at a recent height. In `config.toml`, the `[statesync]` section accepts `rpc-servers`, `trust-height`, `trust-hash`, and `trust-period`. Use independent trusted RPC sources and a checkpoint within the trust period.

Inspect a chosen RPC checkpoint before configuring it:

```
TM_RPC_URL=http://127.0.0.1:26657
curl -sS "$TM_RPC_URL/status" | jq '.result.sync_info'
curl -sS "$TM_RPC_URL/block?height=100000" | jq '.result.block.header, .result.block_id.hash'
```

Replace the example height with your trusted checkpoint height. JSON-RPC data is inside `result`. Check the response for a matching height and valid block hash before entering the checkpoint in your configuration.

### Snapshot Restore

For a filesystem snapshot, stop the node first and verify the archive's layout, checksum, revision, and storage backend. Preserve the validator signing key and its last-sign state. Restore to the provider's documented directories, including any SC or SS paths configured outside the default home.

Choose a snapshot covering the historical range your application needs. Archive retention preserves that imported history and subsequent state.

## Start Node

```
paxd start --home "$HOME/.paxeer"
```

For a service, use an explicit binary path, operating-system user, and home that match the initialized node. Create the user and directories shown in the example before starting the service.

```
[Unit]
Description=Paxeer Node
After=network-online.target

[Service]
Type=simple
User=paxeer
ExecStart=/usr/local/bin/paxd start --home /var/lib/paxeer
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

## Verify Synchronization

```
paxd status --node tcp://127.0.0.1:26657
curl -sS http://127.0.0.1:26657/status | jq '.result.sync_info'
curl -sS http://127.0.0.1:26657/net_info | jq '.result.n_peers'
journalctl -u paxd -f
```

Confirm the expected chain identity, advancing block height, and completed synchronization before directing application traffic to the node.

## Validator Setup

After synchronization, create or select the operator account and fund it with the intended stake and fees. The consensus signing key is separate from that account. Use [Operators Guide](https://docs.paxeer.app/operators) for registration and last-sign-state handling.

## Local Docker Development

From the monorepo root (the repository is flattened; there is no `Paxeer-X-Network/` subdirectory):

```
make docker-cluster-start
```

See [Docker](https://docs.paxeer.app/docker) for the local cluster and monitoring targets.

**Source:** `daemon/paxd/cmd/init.go`, `node/params/config.go`, `consensus/config/config.go`, `consensus/rpc/jsonrpc/types/types.go`, `consensus/privval/file.go`.
