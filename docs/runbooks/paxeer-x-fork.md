# Paxeer X fork validator runbook

This is the `v7.0` merged-network upgrade and recovery procedure for mainnet-beta
`hyperpax_125-1`, EVM chain ID `125` (`0x7d`). Its rehearsed starting point is
`v6.12.0`. Confirm the starting chain state and release revision before scheduling it;
a network that has already applied `v7.0` must not schedule it again.

Paxeer is one BFT network running `paxd`. The kernel runs inside the node and
participates in block proposal; a transaction receipt is final when its block is
committed. Sub-chains are tenant domains in the database and require no additional
node infrastructure. Use the public node JSON-RPC at the root HTTPS path:
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, or
`https://api-mainnet-beta.paxeer.network`. Compare heights across nodes before
selecting a reference. `node.hyperpaxeer.com` is only the hpx library mirror, not an RPC endpoint.
The `px` namespace is served at the node root from node state; only
`px_register` and `px_requestFunds` are hosted operations. The gateway's
methods have their own contract in
[openrpc.json](../../services/platform/hosted/gateway/openrpc.json).

The implementation authority is [node/upgrades.go](../../node/upgrades.go),
[node/app.go](../../node/app.go) and
[the precompile registration](../../precompiles/setup.go). `MergeUpgrade` is
registered explicitly as `v7.0`; the last entry in `node/tags` does not select it.
At H, the store loader adds `sequencer`, `layerxledger`, `layerxauth`, `layerxpay`,
`layerxchains`, `layerxprograms`, `layerxagent` and `lxphost`. The merge handler
validates the required precompiles, runs migrations, initializes the sequencing
state, retires anchor activity, returns guarantor bonds and releases remaining
custody balances to the custody authority. Check these balance changes against
the rehearsal evidence; they are migration effects, not a continuing finality mechanism.

The halt is scheduled **by height**, using the governance upgrade plan. Wall-clock
time is an estimate. Do not substitute `halt-time` or `halt-height`: the old
binary's upgrade handler must write `data/upgrade-info.json`, which
`SetStoreUpgradeHandlers` reads when the replacement binary starts.

This procedure assumes a manual binary swap under systemd. Cosmovisor operators
must rehearse their binary layout and provisioning separately: the merge plan's
`info` is module genesis, not the historical binaries-only download document.

## Local values

Keep hosts, units, paths, hashes, proposal IDs and operational evidence in a
private inventory. Load a reviewed local environment file before using the
examples. Examples use placeholders for deployment-specific values.

| Key | Meaning |
| --- | --- |
| `UPGRADE_NAME` | `v7.0` for the merge; the later deletion plan is a separate operation |
| `COSMOS_CHAIN_ID` | `hyperpax_125-1` |
| `EVM_CHAIN_ID` | `125` |
| `RELEASE_BINARY` | Downloaded and checksum-verified `paxd-v7.0.0-linux-amd64` asset |
| `PAXD_BIN` | Absolute path of the installed `paxd` |
| `PAXD_HOME` | Absolute node home for this operation |
| `PAXD_HOMES` | Space-separated node homes for discovery; use paths without spaces |
| `PAXD_UNIT` | systemd unit for this home |
| `COMET_RPC_PORT` | This home's local CometBFT RPC port |
| `COMET_RPC_URL` | Synced reference CometBFT URL for CLI queries, e.g. `tcp://127.0.0.1:<port>` |
| `EVM_RPC_URL` | This node's root JSON-RPC URL, e.g. `http://127.0.0.1:<port>` |
| `T_HALT_UTC` | Estimated target time in ISO 8601 UTC |
| `HALT_HEIGHT` | Agreed height H, calculated in section 1 |
| `PLAN_INFO_FILE` | Reviewed JSON file containing the merge genesis described in section 2 |
| `OPERATOR_KEY` | Local keyring name used for governance transactions |
| `DEPOSIT` / `FEES` | Coin strings selected from live parameters and fee policy |
| `PROPOSAL_ID` | Confirmed on-chain proposal ID |
| `BACKUP_ROOT` | Private backup directory outside the node home |
| `STATESYNC_RPC_SERVERS` | At least two trusted CometBFT HTTP URLs for recovery |
| `TRUST_HEIGHT` / `TRUST_HASH` | Independently verified recent block height and hash for state sync |

Use one inventory row per home. On a multi-home host, override the unit, home and
endpoint values together before each operation. Protect key backups as carefully
as the live keys; do not publish inventory or raw qualification logs.

## 0. Facts to confirm first

Map every active validator consensus key to a host, home, signer and unit. Query
`/validators` from a synced CometBFT node, following pagination until the returned
validator count equals `total`:

```sh
curl --fail --silent --show-error \
  "http://127.0.0.1:$COMET_RPC_PORT/validators?page=1&per_page=100" | jq .result
```

For each local home, inspect its public validator address and unit configuration:

```sh
for node_home in $PAXD_HOMES; do
  [ -f "$node_home/config/priv_validator_key.json" ] || continue
  jq -r .address "$node_home/config/priv_validator_key.json"
done
systemctl show -p ExecStart --value "$PAXD_UNIT"
```

Never print private key fields. For remote signers, map the remote key and its
persistent signing state too. Have a second operator check the completed table.

| Validator address | Voting power | Host | Home | Unit / signer | CometBFT / EVM endpoints |
| --- | --- | --- | --- | --- | --- |
| `<validator>` | | | | | |

Do not continue with duplicate signing identities or an unmapped active key.
Record current height, app hash, chain ID and `catching_up` from `/status`.
Confirm the chain has applied the prerequisite release plans, including
`v6.12.0`, using `paxd q upgrade applied <plan-name>`. A binary's version alone
does not prove a plan was applied.

### PaxDB and persistent state

[SetupPaxDB](../../node/paxdb.go) requires `[state-commit] sc-enable = true` in
`config/app.toml`; disabling it panics because the IAVL backend is deprecated.
Preserve the deployed backend and storage configuration across the binary swap.
Inventory `state-commit.sc-directory`, `state-store.ss-db-directory` and
`state-store.evm-ss-db-directory`, including external paths and mount points.
A backup of `data/` alone is insufficient when storage lives elsewhere.

State-sync snapshot settings are in `[state-sync]` in `app.toml`:
`snapshot-interval` and `snapshot-keep-recent`. With snapshots enabled,
`[state-store] ss-enable` must also be true. These exported snapshots are distinct
from internal state-commit snapshots. Keep the node's configured sequencing WAL,
receipt key, consensus key and signing high-water mark in the recovery inventory.

## 1. Halt height

Measure a synced node's height progression over a representative interval.
Record both samples and UTC epoch times; reject samples with no height increase
or a node that is catching up. Calculate:

```text
seconds_per_block = (t2 - t1) / (h2 - h1)
H = h2 + round((target_utc_epoch - t2) / seconds_per_block)
```

Read live governance parameters before choosing H. Allow the full deposit and
voting windows, staging time, and a cancellation voting window with margin.
Record the agreed H and expected time in the private inventory. Re-measure before
submission and during staging. If the target drifts outside the agreed window,
cancel by governance and propose a new height; never edit H on individual nodes.

## 2. Proposal

The former hosted/paxeer governance scripts are retired. Use the registered
`paxd` governance CLI from
[sdk/x/upgrade/client/cli](../../sdk/x/upgrade/client/cli/tx.go).
Read current values rather than copying historical mainnet deposits or periods:

```sh
"$PAXD_BIN" q gov params --node "$COMET_RPC_URL" -o json
"$PAXD_BIN" q upgrade plan --node "$COMET_RPC_URL" -o json
```

### Merge plan info

`mergeUpgradePlan` consumes `plan.info` as JSON module genesis. Prepare and review
`PLAN_INFO_FILE` using the exact encoding in
[the merge rehearsal](../../node/merge_rehearsal_test.go) and
[ReceiptKeyEntry](../../modules/sequencer/keeper/receipt_keys.go):

| Field | Required content |
| --- | --- |
| `leaders` | Non-empty ordered list of distinct, nonzero 32-byte consensus public keys; primary first; each byte string encoded as base64 |
| `receipt_keys` | Non-empty array of objects with `key_id`, `public_key`, `first_height`, `last_height` |
| `key_id` / `public_key` | Base64-encoded 32-byte values; key ID nonzero and unique, public key a canonical Ed25519 key |
| `first_height` / `last_height` | Positive inclusive numeric height range, not inverted; at least one key covers H+1 |
| `activation_height` | Set to H+1 for this procedure; omitted or zero defaults to H+1 |
| `last_seq` | Zero, or omitted |
| `last_seq_hash` | Zero 32-byte hash encoded as base64, or omitted |

Use real, reviewed public keys; never copy the deterministic keys from tests.
Keep receipt signing seeds out of the proposal. A receipt key is separate from
the consensus signing key. Invalid genesis, missing leaders, a nonzero initial
sequence, or missing H+1 key coverage makes activation fail.

Do not replace this JSON with `{"binaries": ...}`. Record release URLs, source
revision and binary checksums separately in section 3. Schedule only the reviewed
merge plan, using the live deposit and fee values:

```sh
"$PAXD_BIN" tx gov submit-proposal software-upgrade "$UPGRADE_NAME" \
  --upgrade-height "$HALT_HEIGHT" --upgrade-info "$(cat "$PLAN_INFO_FILE")" \
  --title "Paxeer merge $UPGRADE_NAME" --description "<reviewed release and plan details>" \
  --deposit "$DEPOSIT" --fees "$FEES" --from "$OPERATOR_KEY" \
  --home "$PAXD_HOME" --chain-id "$COSMOS_CHAIN_ID" --node "$COMET_RPC_URL"
```

Confirm transaction inclusion and a successful application result before taking
the proposal ID from its events. Each voter submits and confirms its own vote:

```sh
"$PAXD_BIN" tx gov vote "$PROPOSAL_ID" yes --from "$OPERATOR_KEY" \
  --home "$PAXD_HOME" --chain-id "$COSMOS_CHAIN_ID" \
  --node "$COMET_RPC_URL" --fees "$FEES"
"$PAXD_BIN" q gov proposal "$PROPOSAL_ID" --node "$COMET_RPC_URL" -o json
"$PAXD_BIN" q upgrade plan --node "$COMET_RPC_URL" -o json
```

Require a passed proposal and exact agreement on plan name, H and decoded `info`.
Compute quorum and tally requirements from live voting power and governance
parameters; do not assume four equal validators or a fixed voting period.
The v6.10 handler writes one-hour standard and twenty-minute expedited periods,
but governance may have changed them since. The expedited CLI flag is
`--is-expedited`; it requires its own deposit and tally checks.

### Consensus timeouts

Keep the rehearsed consensus parameters during the merge. Inspect live
`/consensus_params` and record the values. A timeout change is a separate
parameter-change proposal with its own qualification and governance timeline;
do not include an unqualified change in the binary swap.

### Abort

Make the no-go decision while cancellation can still pass **before H**. Submit
`tx gov submit-proposal cancel-software-upgrade` with a reviewed title,
description, live deposit, fees, chain ID and operator key, then vote and confirm
that `q upgrade plan` reports no scheduled plan. Cancelling is a governance
transaction, not an immediate local action. A T−2 h check is too late if the
remaining voting window is longer. If cancellation cannot finish, coordinate
the network halt and recovery; do not independently skip the upgrade.

## 3. Binary

Record privately the source revision, release URL, binary SHA-256, dependent
shared-library hashes, reviewed plan-info hash and the per-host storage layout.
Stage the exact rehearsed binary on every validator and full node without
replacing the running executable:

```sh
install -m 0755 "$RELEASE_BINARY" "$PAXD_BIN.$UPGRADE_NAME"
sha256sum "$PAXD_BIN.$UPGRADE_NAME"
cp -p "$PAXD_BIN" "$PAXD_BIN.pre-$UPGRADE_NAME"
"$PAXD_BIN.$UPGRADE_NAME" version --long
ldd "$PAXD_BIN.$UPGRADE_NAME"
```

Compare the checksum and reported revision with the release record. Resolve
shared-library requirements, including libwasmvm when dynamically linked, before
H; keep old dependencies available for recovery.

Review the embedded node runtime configuration against
[node/sequencer/config.go](../../node/sequencer/config.go). Under `[sequencer]`,
`enable`, `role`, `wal-dir`, `receipt-key-file`, `standby-address` and
`replication-timeout` must match the rehearsed primary/standby roles. Enabled
nodes need positive `nonce-queue-max-per-sender`, `nonce-queue-max-total` and
`lease-duration` (or the supported derivation from replication timeout).
Receipt seeds must have mode `0600`; retain the WAL at its configured path.
Use `mode = "sequential"` unless microbatch configuration was separately qualified.
Ordinary nodes keep `enable = false`; review `receipt-forward-url` only when
receipt forwarding is required. These are settings inside `paxd`.

## 4. Rehearsal (must pass before go)

Release qualification must already have run the programme rehearsal on the
exact candidate source revision. The release gate is:

```sh
PYTHONPATH=spec/layerx-merge python3 -m pytest spec/layerx-merge/programme/tests/test_rehearsal_run.py::test_rehearsal_passes_on_merged_revision -q
```

The test drives the four ordered steps in
[rehearsal.kvx](../../spec/layerx-merge/programme/data/rehearsal.kvx):

1. Apply `v7.0` on the `v6.12.0` starting state and seal the first sequenced block:
   `node/merge_rehearsal_test.go::TestMergeRehearsalFirstSequencedBlock`.
2. Export and re-import with sequencing state preserved:
   `node/upgrade_export_test.go::TestExportAfterMergeKeepsSequencerState`.
3. Activate the gated precompiles:
   `node/upgrades_merge_activation_test.go::TestMergePlanActivatesGatedPrecompiles`.
4. Apply the later `v7.0-anchor-store-delete` plan:
   `node/store_delete_test.go::TestAnchorStoreDeletedAtStep`.

All four steps must pass. A missing step or a nonzero exit is a no-go. Keep exact
revision, command, exit code and private log location for each step. Retain
additional release evidence for bond return, custody release, retired-method
refusals, receipt order matching block order, and restart recovery with the
configured storage and signing identities. No source change after qualification
is covered by the earlier evidence.

### Later anchor-store deletion

`v7.0` retains the `layerxanchor` store. Only the separately scheduled
`v7.0-anchor-store-delete` loader deletes it. Apply that plan at an agreed later
height after merge acceptance, with its own proposal, halt, backup and release
evidence. Do not substitute it for the merge plan or schedule it before the merge.
The anchor address remains registered after deletion and all its methods refuse.

## 5. Announcement

> **Paxeer mainnet-beta upgrade: v7.0**
> Chain `hyperpax_125-1` (EVM 125) upgrades at block **H = <height>**, expected
> near **<UTC estimate>**. Height is authoritative. Operators stage release
> **<revision>**, SHA-256 **<hash>**, and swap binaries only after the upgrade
> halt and upgrade-info file are confirmed. Public RPC availability may be
> interrupted until more than two thirds of voting power resumes. Status:
> **<operator channel>**. Anchor-store deletion is a later operation.

Announce when voting starts and repeat before the cancellation deadline and H.
Publish the agreed height and binary identity; keep private host and signer
inventories out of the announcement.

## 6. Per-validator procedure (at height H)

Follow the section 0 inventory. Coordinate validators holding **more than two
thirds** of voting power, plus margin, to restart promptly; remaining validators
and full nodes follow. Never run one consensus identity in two places.

1. Observe the old binary's `UPGRADE "v7.0" NEEDED at height: H` halt. Confirm
   the last committed height is H−1 and inspect the upgrade file:

   ```sh
   journalctl -u "$PAXD_UNIT" -n 200 --no-pager
   jq '{name, height}' "$PAXD_HOME/data/upgrade-info.json"
   ```

   Require the agreed name and height. Do not fabricate this file if missing.
2. Stop the unit, prevent automatic restarts and confirm the process and signer
   have stopped. Capture the latest signing high-water mark from
   `data/priv_validator_state.json` or the external signer. A key may have signed
   at H even when H has not committed.
3. Take a consistent stopped-node backup under `BACKUP_ROOT`: configuration,
   genesis, upgrade-info, consensus/node/receipt keys, signing state, all PaxDB
   directories and the sequencing WAL, including paths outside the home.
   Record the backup height and hashes. Never copy a live database as a backup.
4. Verify the staged checksum again and install the replacement binary:

   ```sh
   install -m 0755 "$PAXD_BIN.$UPGRADE_NAME" "$PAXD_BIN"
   sha256sum "$PAXD_BIN"
   ```

   Coordinate every unit sharing that executable before replacing it. Use the
   libraries and runtime configuration reviewed in section 3.
5. Start the unit and inspect startup/migration logs:

   ```sh
   systemctl start "$PAXD_UNIT"
   journalctl -u "$PAXD_UNIT" -n 200 --no-pager
   curl --fail --silent --show-error \
     "http://127.0.0.1:$COMET_RPC_PORT/status" | jq .result.sync_info
   "$PAXD_BIN" q upgrade applied "$UPGRADE_NAME" --node "$COMET_RPC_URL" -o json
   "$PAXD_BIN" q upgrade module_versions sequencer --node "$COMET_RPC_URL" -o json
   ```

   Require applied height H, advancing committed blocks beyond H, eventual
   `catching_up: false`, and matching block/app hashes against trusted peers.
   H+1 is the first sequenced height in this procedure. Use the local node as
   `COMET_RPC_URL` for these checks; a remote node cannot establish local health.
6. Check the local root JSON-RPC:

   ```sh
   curl --fail --silent --show-error "$EVM_RPC_URL" \
     -H 'content-type: application/json' \
     --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
   curl --fail --silent --show-error "$EVM_RPC_URL" \
     -H 'content-type: application/json' \
     --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
   ```

   Require `result: "0x7d"` and an advancing hexadecimal height above H. HTTP
   success alone is insufficient: inspect each JSON-RPC result or error.
7. Apply the version-specific acceptance checks below, then record halt seen,
   upgrade-info identity, checksum, backup location, restart UTC, first signed
   and first committed heights. Before restoring traffic, perform a controlled
   restart and confirm the same applied-plan and committed-state checks persist.

### Merge acceptance and retired precompiles

Use ABI-encoded calls at a recorded committed block height, with calldata from
that release's checked-in ABI. Decode revert data as Solidity `Error(string)`;
a generic transport error or empty-account success does not prove retirement.
The addresses stay registered. Keep actual request and response evidence.

| Surface | Expected after `v7.0` | Expected after anchor-store deletion |
| --- | --- | --- |
| `0x1012` (`layerxVerify`) | Retired methods revert with `LXP_MODULE_DISABLED` and `layerxVerify retired` | Same |
| `0x1013` (`layerxCustody`) | All methods, including `depositCount()`, revert with `LXP_MODULE_DISABLED` and `layerxCustody retired` | Same |
| `0x1014` (`layerxAnchor`) writes | Revert with `LXP_MODULE_DISABLED` and `layerxAnchor retired` | Same |
| `0x1014` reads, including `latestFinalized()` | Historical reads remain served; no new finality is derived from them | Revert with `LXP_MODULE_DISABLED` and `layerxAnchor retired` |

The ABI sources are [custody](../../precompiles/layerxcustody/abi.json),
[anchor](../../precompiles/layerxanchor/abi.json) and
[verify](../../precompiles/layerxverify/abi.json). Do not require successful old
custody/anchor calls as merge activation checks. A successful historical anchor
read between plans is compatible with retirement of anchor writes.

Require the release's merge activation evidence for native ledger, pay, auth,
programs and agent functions; arbitrary no-argument calls do not prove those
interfaces work. Compare a known transaction's committed receipt and block order
with the release evidence, and verify the expected bond and custody balance
changes. Preserve the export/import and restart evidence with the plan record.

## 7. Rollback

The default response to an activation failure is to keep affected nodes stopped,
preserve logs, databases, WAL and signing state, and coordinate a release repair.
Do not delete data, reset signing history, independently use
`--unsafe-skip-upgrades`, or restart the old binary against migrated stores.

Before H, a staged binary can be withdrawn after governance cancellation is
confirmed. At or after H, restoring a pre-upgrade snapshot is a network-wide
recovery decision requiring an agreed canonical state and evidence of every
validator's signing high-water mark. If any key signed H or later, restoring its
H−1 signing-state file can permit double signing even if H never committed.
Preserve the newest signing state, including an external signer's state.

If H committed, continue from that committed state with a compatible repair
binary or an explicitly coordinated recovery plan. A local snapshot restore
cannot undo a network commit. Retain old binaries and consistent snapshots as
recovery inputs; they are not permission to roll the chain back.

## 8. Full nodes and RPC fleet

Synced full nodes follow section 6 with their own homes and endpoints. Withdraw
an upgrading upstream from the TLS proxy, then restore it only after local
chain ID, committed height, peer hash and restart checks pass. Root-path
JSON-RPC forwarding remains the client entry point.

For lagging nodes, first determine whether the home has ever held a signing
identity. A validator follows the identity-preserving procedure below even when
it is currently inactive in the validator set.

### Non-signing full-node recovery

1. Use a trusted upgraded node as snapshot server. In its `app.toml`, configure
   `[state-sync] snapshot-interval` and `snapshot-keep-recent`, and ensure
   `[state-store] ss-enable = true`. Stage and restart this configuration early.
2. Wait for an available exported snapshot after H (and after the deletion
   height if recovering the post-deletion state). Independently verify a recent
   trust block hash against trusted peers within the configured trust period.
   Trust height need not equal the snapshot height or be an interval multiple.
3. Stop the recovering node and back up its home and external database paths.
   Prepare an empty recovery home using the same chain genesis, reviewed
   configuration and compatible binary. Keep its own `node_key.json`; never
   copy a donor's validator key, signing-state file or receipt seed.
4. Set the actual CometBFT keys in `config/config.toml`:

   ```toml
   [statesync]
   enable = true
   rpc-servers = "<trusted CometBFT HTTP URL 1>,<trusted CometBFT HTTP URL 2>"
   trust-height = <verified recent block height>
   trust-hash = "<verified block ID hash>"
   ```

   Ensure P2P connectivity to snapshot providers. Public EVM JSON-RPC URLs
   cannot replace the CometBFT state-sync servers.
5. Start, wait for `catching_up: false`, compare committed hashes, applied plans
   and JSON-RPC results, then set `enable = false` for subsequent starts.
   Prove restart recovery before returning the node to the RPC pool.

If state sync fails, preserve the error and use a verified, consistent stopped
full-node snapshot of **all** configured databases. Copying only a donor's
`data/` may omit external PaxDB state; copying a running database is unsafe.

### Validator recovery

Stop and fence the original signer before preparing any replacement. Securely
preserve the validator's own key and **latest** signing high-water mark together;
never reset `priv_validator_state.json` to height zero or replace it with a
donor's file. Preserve receipt identity and sequencing WAL where applicable.
Recover chain state as a non-signing node first. Before enabling the original
signer, verify the canonical recovered height is beyond the highest signed
height, hashes agree with trusted peers, and the old signer cannot restart.
If these conditions cannot be established, keep signing disabled and coordinate
recovery. A replacement machine must never create a second active copy of a key.

## 9. Go / no-go (before cancellation deadline, T−2 h and H−100)

- [ ] Section 0 inventory checked; each consensus identity has exactly one active signer.
- [ ] Starting state is the rehearsed `v6.12.0` state; `v7.0` has not already applied.
- [ ] Proposal passed; scheduled name, H and decoded genesis exactly match the reviewed plan.
- [ ] Receipt-key pins cover H+1 and match staged public keys; runtime roles and WAL paths are ready.
- [ ] All four rehearsal steps and migration/refusal/recovery evidence passed on the release revision.
- [ ] Binary and shared-library checksums agree on every validator and RPC host.
- [ ] More than two thirds of voting power, with margin, is staged and operators are available.
- [ ] Consistent backups cover all databases and signing state, with sufficient disk space.
- [ ] Snapshot recovery is ready; RPC upstream withdrawal and restoration are coordinated.
- [ ] Anchor-store deletion has its own later height and acceptance record.

Before the cancellation deadline, any unchecked item is a no-go: cancel and
re-announce. Later failures require coordinated halt/recovery decisions.
After restart, close the upgrade only when section 6 acceptance holds and exact
release revision, commands, exits and private evidence locations are recorded.
