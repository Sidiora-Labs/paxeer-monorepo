<!-- Source: https://docs.paxeer.app/operators/ -->

# Operators Guide

Manage Paxeer X node services, validator accounts, monitoring, and recovery.

For stalled heights, peer failures, or state mismatches, start with [Troubleshoot a Node](https://docs.paxeer.app/node-troubleshooting). Record the binary revision and failing height before changing state.

## Operate the node and application services

Paxeer mainnet-beta is one BFT chain with EVM chain ID 125. Kernel execution is embedded in `paxd` and participates in block proposal. A receipt is final when its block is committed. Hosted gateways and user services support applications around the chain. Assign each deployed service its own health checks, storage policy, and recovery procedure.

| Role | Responsibility | Operational evidence |
| --- | --- | --- |
| Chain node and validator | Execute EVM and Cosmos transactions; enforce active exchange, bridge, launchpad, ledger, and XWeb rules | Chain identity, synchronization, peers, application hash, upgrade version, and RPC health |
| Embedded kernel | Execute ordered activities as part of chain block processing | Committed blocks, execution results, state roots, and receipts |
| Receipt authority | Provide verified receipt material to hosted clients | Authorized receipt keys, transaction binding, and receipt digests |
| Consensus validator set | Vote on blocks and establish BFT commits | Validator membership, voting power, commit signatures, and block identity |
| Gateway and registry | Verify public receipts, route requests, and publish discovery records | Dependency readiness, receipt verification, registry identity, and routing health |
| Agent and Human services | Serve identity, delegated execution, owner approval, security, movement, and key-management boundaries | Authorization outcomes, signed activity and receipt continuity, provider readiness |
| Historical queries | Serve retained node state; see [history access](https://docs.paxeer.app/relay-archive) | Earliest available height and archive retention settings |

Applications access node JSON-RPC or the hosted gateway methods appropriate to their integration. Core administration, signing APIs, and provider internals have separate authentication boundaries. Consensus and kernel execution remain within the node.

See [kernel execution](https://docs.paxeer.app/layerx-kernel), [settlement](https://docs.paxeer.app/settlement), [public APIs](https://docs.paxeer.app/platform-api), and [configuration](https://docs.paxeer.app/configuration).

## Choose the Node Profile

Full nodes serve queries and retain recent history. Archive nodes preserve available historical state. Validators use a signing profile with public application APIs disabled, and seed nodes serve peer discovery. Hardware sizing depends on traffic, retained history, and the selected stores.

Use [Configuration](https://docs.paxeer.app/configuration) to choose a node profile and its enabled services. Validator participation also depends on on-chain registration and the active set.

## HPX Node Management

HPX manages a native `paxd` systemd service and verifies installation artifacts against the selected distribution mirror’s checksum manifest. The mirror distributes installation material.

From the monorepo root, inspect and run the checked-out bootstrap script:

```
less tools/hpx/get-hpx.sh
sudo bash tools/hpx/get-hpx.sh
# If setup was skipped during bootstrap:
sudo env HPX_TYPE=fullnode hpx setup
```

The bootstrap script offers the setup wizard after installation. Select `HPX_TYPE=validator` instead of `fullnode` for validator setup. Run the HPX management commands below from a root shell. The selected artifact set includes the binary, architecture-specific Wasm runtimes, genesis, and configuration. Use the selected mirror's manifest to confirm the release and artifact checksums.

```
hpx                 # interactive menu
hpx status
hpx info
hpx logs
hpx start
hpx stop
hpx restart
hpx peers show
hpx statesync
hpx validator keygen
hpx validator stake
hpx validator status
hpx help
```

`hpx start|stop|restart` control the systemd service. `hpx validator keygen` generates this node's operator account, `hpx validator stake` submits create-validator from the funded operator account, and `hpx validator status` reports validator state. `hpx peers refresh` changes the peer configuration (a peers-refresh systemd timer is also installed); `hpx register` announces the node to the registry. Use `hpx update` as a planned version change after checking network compatibility.

**Source:** `tools/hpx/get-hpx.sh`, `tools/hpx/hpx`, `tools/hpx/README.md`.

## Local Development Cluster

From the monorepo root:

```
DOCKER_DETACH=true make -f chain.mk docker-cluster-start
# Or start the cluster with monitoring:
DOCKER_DETACH=true make -f chain.mk docker-cluster-start-monitoring
```

The targets in `chain.mk` stop the corresponding local cluster and replace generated configuration before startup. See [Docker](https://docs.paxeer.app/docker) for compose services and generated paths.

## Validator Accounts and Keys

The operator account submits staking and governance transactions. The consensus key signs blocks. Use an existing operator account, or create one with the following commands:

```
paxd keys add operator
paxd keys show operator -a
paxd keys show operator --bech val -a
paxd tendermint show-validator
```

A `pax1…` account address, `paxvaloper1…` validator address, and consensus public key are distinct arguments. Use the form required by each command.

### Register a Validator

After the node is synchronized and the operator is funded, register it with `create-validator`. This example uses one PAX in base units. Choose your stake, moniker, and commission settings, taking the network's active validator set into account.

```
paxd tx staking create-validator \
  --amount=1000000uhpx \
  --pubkey="$(paxd tendermint show-validator)" \
  --moniker="My Validator" \
  --chain-id=hyperpax_125-1 \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas=auto \
  --gas-adjustment=1.5 \
  --gas-prices=0.02uhpx \
  --from=operator
```

`--min-self-delegation` is expressed in base units. Set gas prices for the connected network. Choose either `--gas-prices` with a gas limit or an explicit `--fees` amount.

### Inspect Validator State

```
paxd query staking validator "$(paxd keys show operator --bech val -a)"
paxd query staking delegations-to "$(paxd keys show operator --bech val -a)"
paxd query slashing signing-info "$(paxd tendermint show-validator)"
paxd query staking params
paxd query slashing params
```

The staking module also exposes `edit-validator`, `delegate`, `redelegate`, and `unbond`. `paxd tx slashing unjail` is subject to the keeper's eligibility checks; it cannot reverse permanent tombstoning.

**Source:** `sdk/x/staking/client/cli/tx.go`, `sdk/x/staking/client/cli/query.go`, `sdk/x/slashing/client/cli/query.go`, `sdk/client/tx/factory.go`.

## Monitoring

```
curl -sS "${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/status" | jq '.result.sync_info'
curl -sS "${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/net_info" | jq '.result.n_peers'
journalctl -u paxd -f
```

Enable the configured Prometheus listener as shown in [Configuration](https://docs.paxeer.app/configuration). Consensus metrics include block height, last-signed height, missed-block gauges, and syncing state. Build alerts from the emitted metric names and choose thresholds for your workload. Treat missed-block values as gauges.

## Peer and Signer Configuration

Use `bootstrap-peers`, `persistent-peers`, and `private-peer-ids` under `[p2p]`. The peer ID can be read with `paxd tendermint show-node-id`. Keep signer access separate from public query services.

Remote signers connect through the configured `[priv-validator]` socket or gRPC interface. For a hardware-backed signer, use an integration compatible with that interface and verify signing before activating the validator.

## Backup and Recovery

| Data | Purpose |
| --- | --- |
| `config/priv_validator_key.json` | Consensus identity and signing key |
| `data/priv_validator_state.json` | Last signed height, round, step, and signature |
| `config/node_key.json` | Peer identity |
| Selected keyring backend | Operator account keys |
| Genesis, TOML configuration, and configured stores | Matching chain and state data |

The node defaults to `~/.paxeer`. HPX defaults to `/root/.paxeer` and reads `HPX_HOME` to override that home. Stop the node before replacing its stores. Preserve the key and last-sign state together and ensure the original signer is offline before activating a replacement. Never start a second active signer with the same validator key.

For snapshot recovery, verify the archive revision, checksum, backend, and layout before extraction. Reserve `unsafe-reset-all` for a planned state reset with complete backups. Use retention settings for routine pruning and `paxd rollback` for supported stopped-node state reconciliation.

**Source:** `consensus/config/config.go`, `consensus/privval/file.go`, `sdk/server/rollback.go`, `sdk/x/evidence/keeper/infraction.go`.

## Planned Upgrades

Match the network's upgrade plan and the release revision before replacing a binary. Record the installed version, preserve the previous binary and required state, stop the service, install the compatible release, and confirm synchronization after restart. Check the release's upgrade and snapshot compatibility requirements before installation.

Governance upgrade proposals use the registered `software-upgrade` command. Expedited proposal support uses `--is-expedited`; query current deposit and voting parameters before submitting.
