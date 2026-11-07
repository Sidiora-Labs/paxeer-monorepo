# WAL

The write-ahead log and signer state protect consensus recovery inside `paxd`.
Paxeer is one network, including the kernel that proposes blocks; a receipt is
final when its block is committed. Recovery restores that network's consensus
and application state. The storage contract depends on the selected engine
mode.

| Mode | Consensus recovery state |
| --- | --- |
| Classic consensus, including pinned proposer rounds | Consensus message WAL, private-validator last-sign state, block store and application/state stores |
| Optional Autobahn (`autobahn-config-file` is set) | Persisted consensus votes/certificates, availability prune anchor and lane logs, global block and full-certificate logs, plus application/state stores |

See [proposer selection](proposer-selection.md) for mode configuration and
[validator signing](signing.md) for the signature rules each mode protects.
These logs serve different purposes and cannot be substituted for one another.

## Classic Consensus WAL

[`internal/consensus/wal.go`](../../internal/consensus/wal.go) records messages
processed by the consensus receive loop, timeout events, round-state events and
`EndHeightMessage` boundaries. This is not a log of every message exchanged on
the peer network. The state machine appends messages before handling them and
calls `WAL.Sync` at safety-relevant boundaries, including before signing a
proposal or vote and after recording the end of a height. `Append` alone does
not guarantee durability; `Sync` flushes buffers and calls `fsync`.

The implementation uses
[`internal/libs/wal`](../../internal/libs/wal/), not the historical
`autofile.Group`. `DefaultConfig` sets a 10 MiB segment rotation threshold and
a 1 GiB retained-tail size limit. Rotation occurs before appending when the
current head has reached the threshold; pruning removes old tail segments to
meet the retained-tail budget. These are byte limits, not a guarantee that
exactly two heights are retained. The active head can add to total disk use.

The TOML key `[consensus].wal-file` is resolved by
[`ConsensusConfig.WalFile`](../../config/config.go). Its default is
`data/tendermint/cs.wal/wal` under the node home. When either recognized default
is configured and the legacy `data/cs.wal` directory exists, that directory is
used instead. Custom relative paths are resolved against the node home;
absolute paths remain absolute. Preserve the complete log group, including
rotated segments, rather than copying only the active `wal` file.

### Persisted Proposal Receive Time

Classic PBTS depends on the time a proposal arrived, not the time it was
processed or appended to disk. The WAL encodes `msgInfo.ReceiveTime` in its
wire extension field 3. `TimedWALMessage.Time` remains a debugging timestamp.
Replay restores the original receive time before feeding a proposal back into
the consensus state machine.

A legacy proposal entry without that field returns
`ErrWALProposalReceiveTimeMissing`. Recovery must start from committed state
rather than inventing a new receive time or using the debugging timestamp.
The [PBTS runtime checks](proposer-based-timestamp/v1/pbts_001_draft.md#classic-runtime-checks)
explain why preserving this input matters.

## Replay

The classic [`catchupReplay`](../../internal/consensus/replay.go) reads messages
after the last `EndHeightMessage`. The marker identifies the next consensus
height. If the recorded height matches the current consensus height, replay
passes the messages and timeouts through the state machine. An older WAL can
be expected after state or block sync and is not replayed into the newer height;
an unexpected higher height is an error.

During replay the validator can attempt to sign again. Its
[`FilePVLastSignState`](../../privval/file.go) rejects regressions, reuses an
identical signature, and for timestamp-only vote retries restores the original
timestamp and signature. A rejected replay-time signing attempt does not mean
that the last-sign file should be reset: later WAL messages may advance the
state to the already signed step. See [double signing](signing.md#double-signing).

A crash while applying a block is handled through the application handshake
and block replay in the same `replay.go`, using stored heights and application
hashes. Message-WAL replay alone does not replace block/application recovery.

## Autobahn Persistence

Autobahn does not recover by replaying the classic last-height message WAL.
Its JSON configuration uses `persistent_state_dir`, validated in
[`config/autobahn.go`](../../config/autobahn.go). The directory is required;
a blank value is rejected. `unsafe_test_only_disable_persistence` is an explicit
opt-out for throwaway tests and cannot be combined with a persistence directory.
It provides no restart-safety guarantee.

[`node/setup.go`](../../node/setup.go) resolves a relative persistence directory
against the node's `--home`. The consensus persister requires an existing,
writable directory. Node assembly shares this root with availability and data
persistence; their files and subdirectories are separate:

| Location under `persistent_state_dir` | Contents |
| --- | --- |
| `inner_a.pb`, `inner_b.pb` | `PersistedInner`: commit, prepare and timeout certificates plus this node's prepare, commit and timeout votes |
| `avail_inner_a.pb`, `avail_inner_b.pb` | Availability prune anchor containing an `AppQC` and matching `CommitQC` |
| `blocks/` | Signed lane proposals in per-lane indexed WALs |
| `commitqcs/` | Availability-layer commit certificates indexed by road position |
| `globalblocks/` | Data-layer global block payload log |
| `fullcommitqcs/` | Full commit certificates covering contiguous global block ranges |

The implementations are
[`consensus/persist`](../../internal/autobahn/consensus/persist/),
[`consensus/persisted_inner.go`](../../internal/autobahn/consensus/persisted_inner.go),
[`avail/state.go`](../../internal/autobahn/avail/state.go) and
[`data/state.go`](../../internal/autobahn/data/state.go).

### Durability and Pruning

The consensus output loop persists `PersistedInner` before publishing vote
outputs. This keeps the signed votes and the certificates justifying the current
view together. The A/B persister alternates between two files, uses CRC32-C and
a sequence number, and synchronizes each write. Startup selects the valid file
with the highest sequence number. One interrupted or corrupt copy can be
recovered from the other; unreadable files, no valid copy, equal sequence
numbers or invalid decoded state cause an error. Absence of both files means a
fresh state, which is only appropriate for an actually fresh validator.

The indexed WAL adapter uses `storage/wal` with synchronous, unbatched writes
and `FsyncEnabled: true`. Availability persists its prune anchor before pruning
lane and certificate logs. That anchor is the recovery watermark: after a
crash, stale entries below it are filtered and then truncated by the startup
path. Blocks become available for voting through persistence cursors; cursor
advancement follows successful writes.

### Startup Invariants

Startup verifies persisted certificates and signatures against the same
committee context used before the crash. Among the enforced invariants:

- The timeout certificate's index is the next index justified by the commit
  certificate (or index zero when no commit certificate exists).
- Persisted votes and prepare certificates belong to the restored current view;
  a commit vote requires a prepare certificate.
- Availability certificates and lane blocks are contiguous after applying the
  prune anchor, and lane history and certificate relationships validate.
- Global blocks stay within the retained full-certificate range after
  reconciliation. Startup aligns pruned prefixes and truncates block tails
  beyond the certified range. Certificates ahead of available blocks allow
  the blocks to catch up;
  blocks present with an empty certificate WAL are rejected as corruption.

Restored votes and timeout certificates feed the consensus output watches for
rebroadcast. Commit certificates are served through the data layer's
`StreamCommitQCs` path rather than rebroadcast directly from the restored view.
Autobahn's committee is fixed for this runtime; changing keys, weights or chain
context is not a recovery procedure.

## Recovery Procedure

1. Stop the node and keep the error log. Preserve the validator key, last-sign
   state, configuration, genesis and a consistent copy of the complete
   persistence root together with application, block and consensus stores.
2. For an ordinary crash, restart with the same node home, key and committee
   configuration. Let the selected mode's startup code perform replay,
   certificate validation and cursor reconciliation.
3. If startup rejects corruption, missing proposal receive time or inconsistent
   stores, keep the failing data for diagnosis. Restore a consistent known-good
   snapshot or use the node's applicable committed-state synchronization
   procedure. The data WAL explicitly reports that state sync is required when
   blocks survive but their certificates are lost; this page does not define a
   command that can bypass that invariant.

Deleting signer state, removing individual certificates or clearing an Autobahn
persistence directory to make startup succeed loses the evidence that prevents
conflicting votes. Do not roll back safety state independently of the data and
application state it protects. A successful classic WAL replay does not qualify
Autobahn recovery, and an A/B recovery does not reconstruct missing application
data.
