<!-- Source: https://docs.paxeer.app/run-node/ -->

# Run a Node

Initialize a Paxeer X node, choose its role, connect to a network, and verify synchronization.

## Chain node and hosted services

The chain node is `paxd`. The kernel executes inside the node, and the proposer executes transactions and proposes their block. A receipt is final once its block commits. Hosted applications such as the gateway and registry live under `services/platform/hosted/` and have their own configuration and state. This page covers the chain node.

Initialize the node for its intended role, install the verified network genesis and configure peers before starting. Verify synchronization and receipt reads before admitting application traffic. See [operators](https://docs.paxeer.app/operators), [configuration](https://docs.paxeer.app/configuration), [chain history and archives](https://docs.paxeer.app/relay-archive), and [kernel architecture](https://docs.paxeer.app/layerx-kernel).

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

Place the verified network genesis at `~/.paxeer/config/genesis.json`. Confirm its chain ID and expected checksum. Configure `bootstrap-peers` and `persistent-peers` under `[p2p]` in `config.toml` using the network's peer entries in `node-id@host:port` form. The default peer port is 26656.

[HPX tooling](https://docs.paxeer.app/admin-hpx) provides distribution manifests, genesis, and peer registry handling for managed node setup.

## Sync Methods

### Block Replay

A node with compatible genesis and an available block history can replay blocks. Use the correct binaries and upgrades for that history.

### State Sync

State sync imports an application snapshot at a recent height. In `config.toml`, the `[statesync]` section uses `enable`, `rpc-servers`, `trust-height`, `trust-hash`, and `trust-period`. RPC-based state sync requires at least two RPC servers and a trusted checkpoint within the trust period. It starts only when the node has no local application state.

Set `TM_RPC_URL` to your consensus HTTP endpoint and `TRUST_HEIGHT` to the checkpoint height. These are client-side variables for the following queries:

```
curl -sS "$TM_RPC_URL/status" | jq '.result.sync_info'
curl -sS "$TM_RPC_URL/block?height=$TRUST_HEIGHT" | jq '.result.block.header, .result.block_id.hash'
```

Consensus JSON-RPC data is inside `result`. Check the response for a matching height and valid block hash before entering the checkpoint in your configuration.

### Snapshot Restore

For a filesystem snapshot, stop the node first and verify the archive's layout, checksum, revision, and storage backend. Preserve the validator signing key and its last-sign state. Restore to the provider's documented directories, including any SC or SS paths configured outside the default home.

Choose a snapshot covering the historical range your application needs. Archive retention preserves that imported history and subsequent state.

## Start Node

```
paxd start --home "$HOME/.paxeer"
```

To use a different data directory, set the example variable `NODE_HOME` to the initialized node home. The `--home` option selects that directory:

```
paxd start --home "$NODE_HOME"
```

## Verify Synchronization

```
paxd status --node "$TM_NODE_URL"
curl -sS "$TM_RPC_URL/status" | jq '.result.sync_info'
curl -sS "$TM_RPC_URL/net_info" | jq '.result.n_peers'
```

Set `TM_NODE_URL` to the consensus RPC address accepted by `paxd status --node`, with its transport scheme. The default consensus RPC port is 26657. Confirm the expected chain identity, advancing block height and completed synchronization before directing application traffic to the node.

## Validator Setup

After synchronization, create or select the operator account and fund it with the intended stake and fees. The consensus signing key is separate from that account. Use [Operators Guide](https://docs.paxeer.app/operators) for registration and last-sign-state handling.

## Local Docker Development

From the repository root, start the local Docker cluster with the target forwarded to `chain.mk`:

```
make docker-cluster-start
```

See [Docker](https://docs.paxeer.app/docker) for the local cluster and monitoring targets.

**Source:** `daemon/paxd/cmd/init.go`, `node/params/config.go`, `consensus/config/config.go`, `consensus/rpc/jsonrpc/types/types.go`, `consensus/privval/file.go`.
