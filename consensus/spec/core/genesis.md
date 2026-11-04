# Genesis

The genesis document defines the initial conditions of the consensus engine embedded
in `paxd`. Paxeer mainnet-beta is one BFT network, with EVM chain ID **125**
(`0x7d`); the kernel participates in the network and proposes blocks. A transaction
receipt is final when its block is committed. Tenant sub-chains are database domains,
not independently deployed consensus networks.

This page describes the Go `GenesisDoc` and its classic initialization path. Autobahn
uses the same application initialization document but has its own committee and
recovery state; see [mode applicability](#mode-applicability). The inherited
Tendermint [accountability research](../light-client/accountability/README.md)
does not prove either initialization path correct.

## Genesis Fields

The source of the JSON shape and validation is
[`types/genesis.go`](../../types/genesis.go). Fields are:

| Field | Meaning and validation |
| --- | --- |
| `genesis_time` | Initial chain time. A zero value is filled with the current time by `ValidateAndComplete`; distribute an agreed genesis document rather than relying on each node's local default. Node startup waits if this time is in the future. |
| `chain_id` | Nonempty consensus chain identifier, at most `MaxChainIDLen` (50) bytes. This is a string used by consensus, distinct from the numeric EVM chain ID. |
| `initial_height` | Starting block height, encoded as a JSON string. Negative values are rejected; zero is normalized to 1. A restart at a higher initial height requires a matching application state. |
| `consensus_params` | Consensus limits and timings. If absent, `DefaultConsensusParams` is used; supplied parameters are completed and validated. |
| `validators` | Initial `GenesisValidator` entries: `address`, typed `pub_key`, string-encoded `power`, and `name`. Zero power is rejected. An omitted address is derived from the key; a supplied address must match it. The classic handshake also checks the application's returned validator set. |
| `app_hash` | Initial application state commitment (`HexBytes`). A nonempty hash returned by classic `InitChain` replaces this value in the initial consensus state. |
| `app_state` | Application-specific JSON passed as `AppStateBytes` to `InitChain`. Consensus does not interpret or validate its application schema. |

### Consensus parameters

The authoritative keys, defaults and constraints live in
[`types/params.go`](../../types/params.go). A configured genesis can override the
defaults, so these are not claims about a deployed node's current values.

- `block`: `max_bytes`, `max_gas`, `max_gas_wanted`, and the deprecated
  `min_txs_in_block`. The default byte limit is 22,020,096; the absolute
  `MaxBlockSizeBytes` bound is 104,857,600. Negative gas limits are interpreted
  as unlimited by the corresponding conversion helpers.
- `evidence`: `max_age_num_blocks`, `max_age_duration`, `max_bytes`. These bound
  admissible evidence, as described in [State](state.md#evidence).
- `validator.pub_key_types`: accepted consensus public-key types. The supported
  type map currently contains `ed25519`.
- `version.app_version`: application protocol version.
- `synchrony`: `message_delay` and `precision`, used for proposal timestamp
  validity in the classic protocol.
- `timeout`: `propose`, `propose_delta`, `vote`, `vote_delta`, `commit`, and
  `bypass_commit_timeout`. These configure the classic consensus round steps;
  they are not Autobahn view-timeout settings.
- `abci`: `vote_extensions_enable_height` and `recheck_tx`.

Integer limits and durations use the JSON encodings declared on the Go types;
most are string-encoded integers, and durations represent nanoseconds. In
particular, `abci.vote_extensions_enable_height` is a JSON number and
`abci.recheck_tx` is a boolean.

## Application initialization

`MakeGenesisState` in [`internal/state/state.go`](../../internal/state/state.go)
creates the initial consensus state with `LastBlockHeight = 0`, the configured
initial height, validator sets, parameters and application hash. There is no
block at height zero.

In classic mode, the in-process handshake in
[`internal/consensus/replay.go`](../../internal/consensus/replay.go) calls
`InitChain` when the application reports height zero. `ToRequestInitChain`
passes time, consensus chain ID, initial height, parameters and application bytes.
If genesis specifies validators, the `ResponseInitChain.Validators` entries must
match them after sorting. The returned set must be nonempty even when genesis
omits validators. A nonempty returned `AppHash` updates the initial state.

Subsequent block execution uses `FinalizeBlock` followed by `Commit`, as detailed
in [State](state.md#execution). Initialization and execution use the Go
[`Application` interface](../../abci/types/application.go) within the node.

## Mode applicability

A nonempty `autobahn-config-file` selects Autobahn in
[`node/node.go`](../../node/node.go). Its JSON configuration is defined in
[`config/autobahn.go`](../../config/autobahn.go), including committee validator
and node keys and `persistent_state_dir`. The genesis validator list and classic
round timeouts do not specify Autobahn's committee, certificates or view changes.

Autobahn's `runExecute` in
[`internal/p2p/giga_router.go`](../../internal/p2p/giga_router.go) calls `InitChain`
on a fresh application (reported height zero) and starts at `initial_height`.
On restart it resumes from the application's committed height instead of
reinitializing it. See [State](state.md#autobahn-state-and-recovery) for the separate
persistence boundary; classic handshake replay is not its recovery procedure.
