<!-- Source: https://docs.paxeer.app/node-troubleshooting/ -->

# Troubleshoot a Node

Diagnose Paxeer X sync stalls, peer failures, RPC errors, database locks, and AppHash mismatches before changing node state.

Collect a small set of evidence before changing configuration: the running binary, configured network, current height, peer count, and recent logs. This lets you distinguish a local service problem from a network-wide halt or an application-state mismatch.

## Diagnose kernel and application dependencies

Paxeer mainnet-beta runs kernel execution inside `paxd`. A healthy EVM RPC confirms one access surface. Trace an affected operation through its gateway or Human API, chain submission, execution receipt, and committed block. Keep the operation ID, transaction hash, receipt digest, and block height together.

| Symptom | Evidence to inspect | Recovery direction |
| --- | --- | --- |
| Gateway rejects a request | Public method, authentication, registry route, dependency readiness, and receipt verification error | Correct the failing authorization or upstream dependency |
| Node or hosted service fails startup | Node configuration, genesis, chain identity, state-directory ownership, and store lock | Use the matching artifacts and one process per state directory |
| One node falls behind | Committed height, peers, storage latency, and the earliest sync error | Restore connectivity or recover from a matching verified snapshot |
| Transaction remains pending | Original transaction hash, mempool admission, nonce, fees, and block progress | Resolve the original transaction before submitting another payment |
| Committed payment proof fails verification | Transaction binding, receipt digest, block headers, commit signatures, and trusted validator set | Use matching proof material and the configured chain trust anchor |
| Human or agent operation stalls | Identity, owner approval, provider readiness, key-management channel, and canonical receipt | Repair the responsible service boundary and resume the recorded operation |
| Historical state is unavailable | Requested height, configured retention, and node profile; see [history access](https://docs.paxeer.app/relay-archive) and the `paxd` sync checks below | Use an endpoint retaining the required state |

A receipt is final when its block is committed. Verify application evidence against the intended chain and transaction. Distinguish a pending submission from a rejected transaction or a receipt-verification error. Continue with [settlement](https://docs.paxeer.app/settlement) and [operator responsibilities](https://docs.paxeer.app/operators).

## Confirm the process and endpoints

The examples use the systemd service named `paxd` installed by HPX. Set `CONSENSUS_HTTP_URL` to the consensus HTTP origin and `EVM_RPC_URL` to the EVM JSON-RPC URL for your node. Substitute your service name when using another deployment.

```
paxd version --long
systemctl status paxd --no-pager
systemctl show paxd -p ExecStart -p User
ss -lntp
curl --fail --silent --show-error "${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/status" | jq '.result | {node_info, sync_info}'
curl --fail --silent --show-error "${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/net_info" | jq '.result.n_peers'
curl --fail-with-body --silent --show-error "${EVM_RPC_URL:?Set the EVM JSON-RPC URL}"   -H 'Content-Type: application/json'   --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

For the Paxeer X network documented here, compare the Cosmos network with `hyperpax_125-1` and the EVM result with `0x7d`. A local development cluster can use a different identifier. The binary found in your shell can differ from the absolute path in the service’s `ExecStart`.

## Match the symptom to the next check

| Symptom | Inspect | Next action |
| --- | --- | --- |
| Connection refused | Service status and listening sockets | Confirm process startup and the bound address before investigating peers |
| Zero peers | Network ID, configured peer IDs, DNS, and P2P connectivity | Use peers supplied for that exact network and release |
| Peers present, height stalled | Earliest error before the stall, upgrade height, disk and memory pressure | Compare the same height with a known-good node |
| Cosmos status works, EVM RPC fails | EVM enablement, port, and RPC namespace configuration | Check [application configuration](https://docs.paxeer.app/configuration) |
| Historical query fails | Pruning and state retention at the requested height | Use an archive-capable endpoint with that state |

## Capture a sync stall

Capture the height and latest block time twice, a short interval apart. Compare both samples with a known-good node on the same network. A changing height with increasing lag points toward throughput; an unchanged height with a repeating error points toward a specific blocker.

```
curl --fail --silent --show-error "${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/status"   | jq '.result.sync_info | {latest_block_height, latest_block_time, catching_up}'
journalctl -u paxd --since '15 minutes ago' --no-pager -o short-iso
df -h
df -i
free -h
```

Check storage capacity and inode exhaustion separately. For slow disks, use the host’s I/O monitoring to compare read latency, write latency, and saturation with the stalled period. Avoid changing several performance settings at once; preserve one baseline to compare against.

## Investigate AppHash mismatches

An AppHash mismatch means the node’s computed application state disagrees with the block header. Record the exact height, expected and actual hashes, binary revision, genesis identity, and snapshot provenance. Use the binary assigned to that network’s upgrade schedule at the failing height.

```
journalctl -u paxd --since '1 hour ago' --no-pager   | rg -n -i 'apphash|app hash|panic|corrupt|upgrade|handshake'

# Set PAXEER_NODE_HOME to the data directory used by the service.
sha256sum "${PAXEER_NODE_HOME:?Set the active node home}/config/genesis.json"
```

Handshake errors can follow a state mismatch; start with the earliest application error rather than repeatedly rotating peers. Reconcile the release, upgrade schedule, and snapshot with a known-good operator before choosing recovery steps.

## Handle database locks and recovery

A database lock often means another process still holds the same data directory. Inspect the service and process list, including containers, before attempting another start. Keep one writer per data directory; stop the process holding the database before recovery.

Rollback, snapshot restore, and reset commands change durable state and depend on the storage engine and release. Preserve a backup and use the recovery procedure for that exact node version. For validator recovery, preserve the existing signing state and confirm that only one signer is active.

For a duplicate-signing alert, stop the affected signer and establish which instance owns signing before restarting. Retain the validator keys and signing history throughout recovery.

## Prepare an actionable report

- Node role, network identifiers, binary version and checksum, and startup command.

- Stalled or failing height and a comparison with a healthy node.

- At least the logs leading into the first failure, including the complete panic stack when present.

- Disk, memory, peer count, recent upgrade or restore activity, and snapshot checksum.

- The relevant configuration changes, with RPC credentials and signing material removed.

Use [Operators Guide](https://docs.paxeer.app/operators) for ongoing monitoring, [Configuration](https://docs.paxeer.app/configuration) for settings, and [Run a Node](https://docs.paxeer.app/run-node) for initial setup.

**Paxeer source:** monorepo root — `daemon/paxd/cmd/root.go` (command registration), `node/app.go` (DefaultNodeHome), and `consensus/internal/rpc/core/status.go` (Status).
