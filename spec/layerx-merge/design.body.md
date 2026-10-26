## Reading this design

This body describes the merged design of the layerx-merge specification at repository revision `2a1033aa2`. Every `path:line` names the defining line at that revision. **HEAD** means code that exists at the revision; **planned** means work declared by a pending task of this specification. A planned statement names the task that delivers it. The kvx blocks are the authority: where this text and a block differ, the block governs and the difference is a defect of this text.

The specification is merged mechanically from per-job fragments. It carries 2545 tasks, 2388 requirements with 4025 acceptance criteria (one test each), 77723 interface blocks, 615 decisions, 536 wiring requests, 40 services and 200 contract contexts.

| Interface status | Count | Meaning |
|---|---|---|
| stable | 68958 | Code and intent agree; no task modifies it. |
| change | 2945 | Exists at HEAD; behaviour or shape must change. |
| new | 1461 | Absent at HEAD; a task creates it. |
| retire | 4359 | A task removes it or makes it refuse. |

Status counts include the 1333 `[override.*]` blocks that later jobs used to correct an inventoried status without editing the inventory fragment.

## One execution domain

The Paxeer X Network runs one execution domain: paxd, with the Cosmos SDK modules, the EVM, CosmWasm and optimistic concurrency control (OCC) in one state machine. The LayerX kernel stops being a second ledger beside the chain. Its ledger, payment, authority, program and trading functions become chain modules and precompiles that run inside paxd transactions (decision.1). The kernel daemon, its WAL, its replicas and its checkpoint anchoring are retired because block commit is finality.

### Node assembly at HEAD

- The application constructs the EVM keeper at node/app.go:823 and the native keepers beside it at node/app.go:828; the launchpad keeper at node/app.go:837; pricefeed, perps and spot together at node/app.go:841.
- Module services are registered at node/app.go:1133, and the proposal and finalization handlers are installed at node/app.go:1212.
- The custom precompile set is constructed and its activation entries installed at node/app.go:990-997. `PrecompileKeepers` carries the venue, custody, bridge and xweb keepers at node/precompiles.go:26-54, built by `NewPrecompileKeepers` at node/precompiles.go:56.
- RPC construction receives the application and keeper handles at rpc/server.go:167. BaseApp commits through the multistore at sdk/baseapp/abci.go:296.
- An EVM transaction creates its state database in `EVMTransaction` (modules/evm/keeper/msg_server.go:69-86), and `applyEVMMessage` passes `CustomPrecompiles` to the EVM (modules/evm/keeper/msg_server.go:231-243). Merge surfaces keep this route.

### Kernel host inside paxd (decision.1)

The kernel C modules and the Rust program runtime are hosted through cgo inside the node, not ported.

- `HostExecute` is the host entry (modules/lxphost/host.go:678). It loads the SDK branch, serializes access with its mutex, locks the operating-system thread, binds authority and assets, and enters the native activity executor (modules/lxphost/host.go:718).
- The cgo link inputs already include the kernel and program sandbox archives (modules/lxphost/host.go:6-7). `NewHost` validates its configuration at modules/lxphost/host.go:600-609.
- The C bridge calls `lxp_kernel_execute_activity` (modules/lxphost/host.go:474), defined at src/protocol/lxp_kernel.c:6950. Module-context initialization and dispatch are at src/protocol/lxp_kernel.c:7158-7162 and src/protocol/lxp_kernel.c:7221-7225.
- Changes are staged through the module context (modules/lxphost/ctx.go:232) into the Cosmos context's KV store obtained at modules/lxphost/ctx.go:95. `HostExecute` reads the outcome and commits rows and blobs at modules/lxphost/host.go:723-742. The writes stay in the transaction context until BaseApp commit.
- The Rust sandbox keeps its FFI entry points at programs/crates/layerx-programs-sandbox/src/lib.rs:18.
- Planned: task.5.19.2 constructs the shared host and the ledger, auth and agent keepers; task.5.19.4 links the merge constructor packages; task.1.21.3 widens the returned receipt; task.1.22.1 scopes restoration; task.1.23.1 verifies binding authority.

Determinism is a gate, not an assumption. The same block replayed through the host on every validator build must give an identical app hash and per-transaction results; OCC with forced aborts must equal sequential execution; no kernel in-memory state may survive between transactions outside the committed store; no execute path reachable from a keeper may depend on wall-clock time, allocator addresses, threads or floating point.

### OCC, CosmWasm and the EVM

- OCC stays on for validators: `DefaultOccEnabled = true` (sdk/server/config/config.go:29), with `DefaultConcurrencyWorkers` derived by `getConcurrencyWorkers` (sdk/server/config/config.go:41-45).
- CosmWasm keeps its own cgo runtime under wasm-runtime/ and the wasmd precompile at 0x1002 (precompiles/wasmd/wasmd.go:30).
- The EVM keeps the precompile dispatch of precompiles/common: `Precompile.Run` decodes input and calls `Execute` with the state database context (precompiles/common/precompiles.go:187-215); `Prepare` is at precompiles/common/precompiles.go:227-250.

## Native constructs

A native construct is a fixed precompile address served by a Go executor over a chain keeper, or over the shared kernel host. The common registration map is at precompiles/setup.go:164-192. Height-dependent venue replacement is at node/app.go:3528-3535 and node/precompiles_v6120.go:70-93. Each short address below means its 20-byte zero-padded form.

### Addresses served at HEAD

| Address | Construct | Defining line | Merged disposition |
|---|---|---|---|
| `0x1001` | bank | precompiles/bank/bank.go:36 | kept; ABI freeze task.6.9.1, pause refusal task.6.3.2 |
| `0x1002` | wasmd | precompiles/wasmd/wasmd.go:30 | kept |
| `0x1003` | json | precompiles/json/json.go:28 | kept |
| `0x1004` | addr (EVM to DID binding) | precompiles/addr/addr.go:58 | kept; the binding is the authority source of ledger, auth and pay |
| `0x1005` | staking | precompiles/staking/staking.go:49 | kept |
| `0x1006` | gov | precompiles/gov/gov.go:30 | kept |
| `0x1007` | distribution | precompiles/distribution/distribution.go:37 | kept |
| `0x1008` | oracle | precompiles/oracle/oracle.go:26 | refusal surface: both queries return the retired error (precompiles/oracle/oracle.go:93, precompiles/oracle/oracle.go:104) |
| `0x1009` | ibc | precompiles/ibc/ibc.go:30 | kept |
| `0x100A` | pointerview | precompiles/pointerview/pointerview.go:24 | kept |
| `0x100B` | pointer | precompiles/pointer/pointer.go:29 | kept |
| `0x100C` | solo | precompiles/solo/solo.go:32 | kept |
| `0x1011` | p256 | precompiles/p256/p256.go:24 | kept |
| `0x1012` | layerxVerify | precompiles/layerxverify/layerxverify.go:52 | retired executor: task.6.71.1 returns LXP_MODULE_DISABLED from the merge plan, address stays registered |
| `0x1013` | layerxCustody | modules/layerxcustody/types/ids.go:17 | retired executor: task.6.45.1 refuses every method; task.6.46.1 releases stranded balances (decision.642) |
| `0x1014` | layerxAnchor | precompiles/layerxanchor/layerxanchor.go:65 | writes retire at merge (task.6.32.1); reads retire at the store-delete step (task.6.32.2); bonds returned (task.6.33.1) |
| `0x1015` | perps (historical exchange below v6.12.0) | precompiles/perps/perps.go:27 | native perps engine (decision.301); legacy exchange writes refused (task.3.27.3) |
| `0x1016` | layerxBridge | modules/layerxbridge/types/keys.go:27 | kept |
| `0x1017` | launchpad | modules/launchpad/types/keys.go:26 | native launchpad from v6.12.0 (decision.1704) |
| `0x1018` | feeToken | precompiles/feetoken/feetoken.go:24 | kept; SID self-pay |
| `0x1019` | xweb | precompiles/xweb/xweb.go:59 | kept; program-origin requests added (task.13.32.1) |
| `0x1020` | pricefeed | precompiles/pricefeed/pricefeed.go:23 | venue price source (decision.1702) |
| `0x1021-0x1025` | spot (PECOR, Orders, StopOrders, Quoter, Vault) | modules/spot/types/keys.go:12 | served with the gas wrapper (task.3.21.1, task.3.21.2) |

### Merge addresses

Twelve addresses are declared in `MergeAddressTable` (precompiles/setup.go:61-73) and gated by `mergePrecompileGate` (node/upgrades.go:521-536). None is served at HEAD: constructors register through precompiles/setup.go:89-103, and `newMergePrecompiles` skips every entry without a registered constructor (precompiles/setup.go:117-139). The tee-attestor package shows the registration pattern (precompiles/layerxagent/teeattestor/teeattestor.go:103-106) but the production node does not import it.

| Address | Construct | Planned executor |
|---|---|---|
| `0x101A` | layerxledger: assets, accounts, send, payer grants, draw, payExact | task.2.10.1 |
| `0x101B` | layerxpay: escrow, budget, stream, service | task.2.28.1 |
| `0x101C` | layerxauth: session and capability grants | task.2.11.1 |
| `0x101D` | layerxprograms: deploy, upgrade, call, wind-down, retireStorage | task.13.22.1 |
| `0x0901-0x0908` | agent functions: orob, clearing, oracle aggregator, pofq, scheduler, streams, tee attestor, eip712 | domain 2 (decision.22) |

A declared address, a linked library and a registered constructor are three separate facts; an address is live only when its constructor is imported by the node and its gate is active at the height.

### Gas

Every new precompile method charges a fixed per-method row (decision.30). `GasTable` maps an ABI method name to its fixed row (precompiles/common/precompiles.go:65), and `WithGasTable` charges every method from that table and refuses an unknown method (precompiles/common/precompiles.go:115). Dynamic-gas precompiles use the same table through precompiles/common/precompiles.go:307.

## The proposer is the executor

An operator-run primary and hot standby execute every transaction at receipt on a speculative branch of committed state, return a canonical ProtocolReceipt v2 signed by a receipt key, serve pending reads from the open block, and propose blocks containing exactly that sequence. Other validators re-execute and commit. Block time is settlement only (decision.18, decision.19).

### Components at HEAD

- `NewExecutor` builds the executor (node/sequencer/executor.go:194), and `sequential.Open` opens the speculative block on committed state (node/sequencer/executor.go:226-259).
- `Receipter.Accept` signs an accepted result (node/sequencer/receipt.go:325-370); the local producer calls `SignReceipt` at node/sequencer/receipt.go:366.
- `Replicator.Release` syncs locally and awaits the standby (node/sequencer/standby.go:214-262).
- `ConsensusSource.Seal` constructs the signed tx-0 batch (node/sequencer/consensus_source.go:56-109).
- The keeper admits a committed batch at modules/sequencer/keeper/keeper.go:56 and writes `LastSeq`, `LastSeqHash` and the last batch height only after authentication (modules/sequencer/keeper/keeper.go:98-104).

These components do not connect themselves. At HEAD the daemon reads and discards the sequencer config (daemon/paxd/cmd/root.go:249-251), and consensus looks for `types.SequencerSource` on the application (consensus/node/node.go:263-264) whose `Sequenced` signature differs (node/app.go:3099-3102, consensus/types/sequencer.go:10-16). Planned: task.5.0.1 adds the application source, RPC option and commit-listener seams; task.5.5.12 assembles the runtime and hands the source to consensus; task.5.5.19 loads keys and replays the WAL before admitting sends; task.5.46.7 starts the runtime from parsed config.

### Receipts and rungs

- The receipt method name is fixed at rpc/sequencer_receipt.go:26 and the three rungs `submitted`, `receipted` and `committed` at rpc/sequencer_receipt.go:28-37. Clients report only these rungs (decision.17).
- Receipt keys are separate from validator keys (decision.15). Loading refuses exposed files, a shared key file, a key equal to the consensus key, malformed pins and missing coverage (node/sequencer/receiptkey.go:42-84).
- `ReceiptKeys.VerifyReceipt` checks canonical bytes, height coverage and the signature (modules/sequencer/keeper/receipt_keys.go:112). Primary and standby key ranges may overlap (modules/sequencer/keeper/receipt_keys.go:74-76; decision.36).
- HEAD `sequential.Accept` delivers on a transaction cache and advances the sequence only for code zero or an EVM execution with a VM error (node/sequencer/accept.go:16-78); block position zero is reserved for the batch (node/sequencer/accept.go:33).
- `ErrReceiptInvariant` marks execution accepted but receipt construction failed (node/sequencer/receipt.go:363-368); it is not an ordinary refusal.

### Proposal validation and delivery

At a sequenced height a block is `[MsgSequenceBatch, tx1, ..., txN]`. `CreateProposalBlock` obtains it from `SequencerSource.Seal` and preserves `OpenTime` (consensus/internal/state/execution.go:266-275). `MsgSequenceBatch` carries height, range, previous and final sequence hashes, transaction root, key id and signature (modules/sequencer/types/msgs.go:33-42).

HEAD proposal validation checks that tx 0 exists and decodes, checks the transaction root and applies total-block-gas validation to the remaining transactions (node/app.go:1533-1585). It does not yet run batch authentication and sequence continuity before accepting the proposal. Planned order: decode tx 0 from keeper activation state; `VerifyBatchTxs` checks count, root and the `seqHash` fold (task.5.22.4); `checkSequencerOrder` runs `DeliverSequenceBatch` on a disposable cache (task.5.1.1); `checkSequencerProposer` checks membership in the leader list (task.5.1.2); task.5.0.2 installs the checks and rejection counters.

Validator re-execution runs BeginBlock, delivers the batch at index zero, executes the remaining transactions at their block positions (node/app.go:2235-2275), then MidBlock, Giga flush, deferred balances and EndBlock (node/app.go:2278-2308). Planned task.5.23.1, task.5.23.2 and task.5.9.1 store decided votes so speculative and validator BeginBlock see the same votes; HEAD executor `beginBlock` passes nil votes (node/sequencer/executor.go:265-271).

### WAL and standby

- WAL records are defined at node/sequencer/wal/wal.go:107-141; frames carry CRC32C, an eight-byte header and an 8 MiB entry bound (node/sequencer/wal/wal.go:38-49). `GroupSync` flushes a group and issues `fdatasync` (node/sequencer/wal/wal.go:557-590).
- `Replay` begins after the last `COMMITTED`, re-executes each `ACCEPT` and stops on `ErrReceiptMismatch` (node/sequencer/wal/wal.go:717-778).
- Standby acknowledgements are `AckOK`, `AckGap` or `AckRefused` in 25 bytes (node/sequencer/standby.go:25-29); task.5.27.2 adds the epoch and grows the ack to 33 bytes. `NewStandby` requires distinct primary and standby keys (node/sequencer/standby.go:494-521).
- A send without the standby acknowledgement returns an error and no receipt; after a primary stop the standby proposes the replicated slice with receipts unchanged.

### Activation height

HEAD `App.Sequenced` compares with strict `>` against the upgrade done height (node/app.go:3099-3102) while the leader helper uses an inclusive gate (node/upgrades.go:472-483), and the merge plan checks key coverage at the plan height (node/upgrades.go:458-463). The merged rule is keeper state: `ActivationHeightKey = 0x07`, sequencing when `S > 0 && h >= S`, with genesis `activation_height = 0` defaulting to the initialization height plus one (decision.521; task.5.24.2, task.5.25.2, task.5.0.2). task.5.6.2 checks key coverage at the first sequenced height; task.5.6.4 removes the old leader helper. `ReceiptKey.Covers` is the inclusive interval at modules/sequencer/types/genesis.go:20-21.

## Fee policy

Fees are policy, not security.

- The EVM minimum fee per gas is 0.067 gwei at HEAD: `DefaultMinFeePerGas` (modules/evm/types/params.go:52), set into `DefaultParams` (modules/evm/types/params.go:85). The draw sender class pays at most that price (decision.10). Cosmos transactions keep the node minimum gas price; the daemon's default app config sets `MinGasPrices = "0.01uhpx"` (daemon/paxd/cmd/root.go:404). Because the ante chain runs in deliver mode on the sequencer and never checks the node floor there, `FeeFloor` applies the floor before Accept with the CheckTx checker (node/sequencer/fee.go:18-22).
- SID is self-paid through feeToken 0x1018 at the governed rate. `DefaultMaxFeeTokenRateAge` permits a governed rate for 1000 blocks (modules/evm/types/params.go:39); the rate age is raised by governance for the sequencer cadence (decision.7).
- Sponsored writes are type-4 transactions calling `executeSponsored` (contracts/src/BatchCallAndSponsor.sol:135) with one paymaster nonce; the gas station keeps per-DID budgets (services/interop/crates/layerx-gas-station/src/policy.rs:124) resolved through `getLayerXDid` at 0x1004 (precompiles/addr/addr.go:41; decision.14).
- Paid search is drawn at 0x101A (decision.10). The xweb module default fee is one PAX in base units at HEAD (modules/xweb/types/state.go:79); the activation value is set from the governed price.
- The program fee (metering times prices) stays on top of gas (decision.9).

## Authority model

- Every kernel account is a derived bank address with its own margin account (decision.2). Balances without an addr binding at activation follow decision.5.
- The EVM to DID binding is the 0x1004 surface: `bindLayerX`, `unbindLayerX` and `getLayerXDid` (precompiles/addr/addr.go:39-41). Write precompiles resolve the sender to the granting DID before execute.
- Session keys and delegated authorities live in authz as LayerX-scope authorizations (decision.3); `LayerXScopeAuthorization.MsgTypeURL` is at modules/layerxauth/types/authorization.go:41.
- Payment payloads are typed per family, for example `StreamOpen` (modules/layerxpay/types/payload_stream.go:28); there is one stream state (decision.29).
- Custody claims and guarantor bonds are settled at the activation height (decision.4, superseded by decision.642): bonds return to their posters and stranded custody balances are released to the custody authority.

## Activation and retirement

Activation is a bring-up at the activation height; no account-state migration is required. Four plans are recognized from an upgrade info file by `knownUpgradePlans` (node/upgrades.go:543).

| Plan | Constant | HEAD behaviour | Merged behaviour |
|---|---|---|---|
| `v6.9` | `ActivationUpgrade` (node/upgrades.go:95) | gates 0x1012-0x1019 through `activationPrecompileGate` (node/upgrades.go:501) | preserved |
| `v6.10` | `V610Upgrade` (node/upgrades.go:104) | operating parameters | preserved |
| `v6.12.0` | `V6120Upgrade` (node/upgrades_v6120.go:32) | adds perps, spot and pricefeed | rehearsal starts after this plan |
| `v7.0` | `MergeUpgrade` (node/upgrades.go:112) | `mergeUpgradePlan` (node/upgrades.go:432) adds only the sequencer store (node/upgrades.go:418-421) | task.5.6.5 adds ledger, auth, pay, programs, agent and host stores; bonds refunded; custody released |
| `v7.0-anchor-store-delete` | planned | none | task.5.7.1 deletes the anchor store and keeps the 0x1014 executor |

The merge plan name is declared three times: node/upgrades.go:112, precompiles/setup.go:50 and precompiles/common/precompiles.go:30. task.5.6.1 derives the node constant from the common constant; decision.51 keeps the setup declaration under an equality test.

Retired executors stay registered and refuse. `RetiredAt` (precompiles/common/retire.go:24) and `RetiredRevert` (precompiles/common/retire.go:34) are the shared helpers; at HEAD no executor calls them. Legacy surfaces follow decision.20; layerxAnchor reads stay served until bond return completes (decision.21).

Components that existed only to join two domains are retired after their replacements land: the intent ingester under services/interop/crates/layerx-intent-ingester/, the kernel finality path, and the anchor-reading mirror input. Kernel governance halt and resume become per-module pause params set through governance.

## Off-chain services and clients

Services read chain state and sequencer receipts; none of them mutates the node store directly.

- The platform gateway relays JSON-RPC at services/platform/hosted/gateway/src/paxeer.rs:82.
- The agent shared client submits signed bytes with `eth_sendRawTransaction` (services/agent/crates/layerx-client/src/sequencer.rs:321), performs EVM reads (services/agent/crates/layerx-client/src/sequencer.rs:303) and requests receipt keys (services/agent/crates/layerx-client/src/sequencer.rs:367).
- The interop web watcher performs EVM calls (services/interop/crates/x-websearch/src/watch.rs:164) and reads logs (services/interop/crates/x-websearch/src/watch.rs:580); the chain verifies program answers at modules/xweb/keeper/program_fulfil.go:117.
- The wallet gateway registers its routes at services/wallet/gateway/src/index.ts:91, authenticates service requests at services/wallet/gateway/src/middleware/auth.ts:21 and broadcasts signed transactions at services/wallet/gateway/src/rpc/pool.ts:347.
- The explorer backend reaches the node over HTTP and JSON-RPC (services/explorer/README.md:21); the web app fetches the explorer API at apps/web/src/explorer/client.ts:99.
- 40 `[service.*]` blocks declare each process with its route, inputs, outputs, state, dependencies, failure policy, completion task and operation task.

## Domains

Seventeen domains partition the repository by exclusive path prefixes. Each domain has one feature manager role, one `[feature.N]` block and five anchor tasks: N.0.1 foundation (wave 0), N.0.2 surface frozen (wave 2), N.0.3 integrated (wave 4), N.0.4 clients repointed (wave 5) and N.0.9 completion gate (wave 7). Cross-domain `requires` name only anchors; a change in another domain's path is a `[wiring.*]` request fulfilled by a task of the owning domain.

| N | Slug | Owner role | Requirements | Criteria | Tasks | Interfaces |
|---|---|---|---|---|---|---|
| 1 | kernel | fm-kernel | 142 | 244 | 141 | 2755 |
| 2 | ledger-auth-pay | fm-ledger-auth-pay | 153 | 283 | 153 | 617 |
| 3 | markets | fm-markets | 111 | 225 | 170 | 4202 |
| 4 | launchpad | fm-launchpad | 186 | 263 | 186 | 14315 |
| 5 | node-runtime | fm-node-runtime | 196 | 337 | 232 | 2946 |
| 6 | chain-modules | fm-chain-modules | 193 | 344 | 253 | 5092 |
| 7 | rpc-api | fm-rpc-api | 70 | 147 | 116 | 864 |
| 8 | platform | fm-platform | 214 | 378 | 213 | 7469 |
| 9 | agent | fm-agent | 148 | 280 | 148 | 4994 |
| 10 | human | fm-human | 125 | 199 | 125 | 2554 |
| 11 | interop | fm-interop | 125 | 236 | 125 | 3077 |
| 12 | wallet | fm-wallet | 117 | 168 | 117 | 3511 |
| 13 | programs | fm-programs | 128 | 206 | 128 | 5152 |
| 14 | web-explorer-bridge | fm-web-explorer-bridge | 163 | 274 | 163 | 7355 |
| 15 | cosmos-fork | fm-cosmos-fork | 95 | 136 | 57 | 6070 |
| 16 | tools-ops | fm-tools-ops | 157 | 216 | 153 | 6709 |
| 17 | programme | fm-programme | 65 | 89 | 65 | 41 |

### 1. kernel

- Paths: `include/layerx/`, `src/`, `modules/lxphost/`, `layerxproof/`, `custodyproof/` and the kernel C tests.
- The C kernel and its cgo host. `HostExecute` (modules/lxphost/host.go:678) and the state writeback (modules/lxphost/ctx.go:232) are the boundary between SDK transaction state and C activity execution. The kernel result-code registry adds -737 to -739 (decision.40).
- Tasks by wave: w0 18, w1 17, w2 19, w3 7, w4 9, w5 19, w6 51, w7 1; anchors 1.0.1, 1.0.2, 1.0.3, 1.0.4, 1.0.9.
- Interfaces: stable 2183, change 69, new 22, retire 481; incoming wiring requests: 27.

### 2. ledger-auth-pay

- Paths: `modules/layerxledger/`, `modules/layerxauth/`, `modules/layerxpay/`, `modules/layerxagent/` and their precompiles and protos.
- Chain adapters for kernel accounts, authorization, payment constructs and the agent functions 0x0901-0x0908. Typed exact payment at 0x101A follows decision.27; agent functions follow decision.22, decision.31 and decision.32.
- Tasks by wave: w0 27, w1 15, w2 79, w4 15, w5 2, w6 14, w7 1; anchors 2.0.1, 2.0.2, 2.0.3, 2.0.4, 2.0.9.
- Interfaces: stable 232, change 56, new 324, retire 5; incoming wiring requests: 4.

### 3. markets

- Paths: `modules/perps/`, `modules/spot/`, `modules/pricefeed/`, `modules/oracle/`, `modules/layerxexchange/`, `apps/perps/` and their precompiles.
- The served venues are native Go. The perps keeper owns the EVM collateral and pricefeed dependencies (modules/perps/keeper/keeper.go:20); spot has five addresses (modules/spot/types/keys.go:12). decision.301 and decision.334 fix the served topology; trading semantics where kernel and exchange differ follow decision.8.
- Tasks by wave: w0 11, w1 40, w2 25, w3 6, w4 46, w5 12, w6 29, w7 1; anchors 3.0.1, 3.0.2, 3.0.3, 3.0.4, 3.0.9.
- Interfaces: stable 3798, change 283, new 99, retire 22; incoming wiring requests: 5.

### 4. launchpad

- Paths: `modules/launchpad/`, `precompiles/launchpad/`, `services/launchpad/`, `apps/launchpad/`, `sdk/launchpad/`, `contracts/launchpad/`.
- The launchpad precompile binds its address at precompiles/launchpad/launchpad.go:37 over the keeper built at node/app.go:837; the statistics process starts at services/launchpad/stats-workers/cmd/statsd/main.go:16. Service projections serve reads and never establish consensus state.
- Tasks by wave: w0 37, w1 43, w2 44, w3 7, w4 25, w5 25, w6 4, w7 1; anchors 4.0.1, 4.0.2, 4.0.3, 4.0.4, 4.0.9.
- Interfaces: stable 13934, change 128, new 195, retire 58; incoming wiring requests: 0.

### 5. node-runtime

- Paths: `node/`, `modules/sequencer/`, `storage/`, `engine/`, `consensus/`, `daemon/`, `cmd/`, `go.mod`.
- Execution assembly, consensus hooks, receipt production and node persistence; the sequencer runtime described above. Shared files `node/app.go` and `node/precompiles.go` belong here.
- Tasks by wave: w0 15, w1 44, w2 38, w3 27, w4 31, w5 42, w6 34, w7 1; anchors 5.0.1, 5.0.2, 5.0.3, 5.0.4, 5.0.9.
- Interfaces: stable 2563, change 119, new 109, retire 155; incoming wiring requests: 113.

### 6. chain-modules

- Paths: `modules/evm/`, `modules/tokenfactory/`, `modules/layerxgov/`, `modules/layerxbridge/`, `modules/layerxcustody/`, `modules/layerxanchor/`, `modules/xweb/`, `precompiles/` (rest), `contracts/` (Solidity).
- EVM execution and shared precompile registration: `precompiles/setup.go:89` registers merge constructors and precompiles/setup.go:117 builds the registered set. The retired executors 0x1012-0x1014 and the custody release belong here.
- Tasks by wave: w0 46, w1 31, w2 33, w3 7, w4 38, w5 39, w6 58, w7 1; anchors 6.0.1, 6.0.2, 6.0.3, 6.0.4, 6.0.9.
- Interfaces: stable 4134, change 49, new 181, retire 728; incoming wiring requests: 36.

### 7. rpc-api

- Paths: `rpc/`, `api/eth`, `api/pax`, `buf.*`, `sdk/proto/`.
- Node listeners for chain reads, submission, subscriptions and receipts. rpc/server.go:220 registers the EVM namespace; the receipt rungs are fixed at rpc/sequencer_receipt.go:33.
- Tasks by wave: w0 8, w1 53, w2 15, w3 8, w4 23, w5 3, w6 5, w7 1; anchors 7.0.1, 7.0.2, 7.0.3, 7.0.4, 7.0.9.
- Interfaces: stable 812, change 14, new 28, retire 10; incoming wiring requests: 2.

### 8. platform

- Paths: `services/platform/` (except perps-data), `tools/platform/`, `sdk/platform/`.
- Gateway, core, authority, indexer and supporting services in the workspace at services/platform/Cargo.toml:2. Service databases and caches stay outside replicated execution.
- Tasks by wave: w0 10, w1 17, w2 22, w3 32, w4 17, w5 59, w6 55, w7 1; anchors 8.0.1, 8.0.2, 8.0.3, 8.0.4, 8.0.9.
- Interfaces: stable 6327, change 558, new 93, retire 491; incoming wiring requests: 41.

### 9. agent

- Paths: `services/agent/`.
- Agent daemon, agent API and shared clients (services/agent/crates/layerx-client/src/sequencer.rs:321). agent-api major 2 replaces major 1.
- Tasks by wave: w0 10, w1 26, w2 32, w3 22, w4 18, w5 13, w6 26, w7 1; anchors 9.0.1, 9.0.2, 9.0.3, 9.0.4, 9.0.9.
- Interfaces: stable 4174, change 396, new 29, retire 395; incoming wiring requests: 21.

### 10. human

- Paths: `services/human/`.
- Human identity, intents and KMS services that reuse the agent clients through workspace dependencies (services/human/Cargo.toml:31). decision.1081 takes the three retired crates out of the human workspace members.
- Tasks by wave: w0 6, w1 19, w2 14, w3 14, w4 20, w5 38, w6 13, w7 1; anchors 10.0.1, 10.0.2, 10.0.3, 10.0.4, 10.0.9.
- Interfaces: stable 1588, change 159, new 8, retire 799; incoming wiring requests: 16.

### 11. interop

- Paths: `services/interop/`.
- Relayers, gas station, x402, web attestors and portable receipts. The gas station keeps per-DID budgets (services/interop/crates/layerx-gas-station/src/policy.rs:124).
- Tasks by wave: w0 13, w1 17, w2 22, w3 21, w4 14, w5 11, w6 26, w7 1; anchors 11.0.1, 11.0.2, 11.0.3, 11.0.4, 11.0.9.
- Interfaces: stable 2671, change 164, new 80, retire 162; incoming wiring requests: 6.

### 12. wallet

- Paths: `services/wallet/`, `apps/wallet/`, `sdk/wallet/`.
- Wallet UI, gateway and signing service; route registration at services/wallet/gateway/src/index.ts:91.
- Tasks by wave: w0 9, w1 15, w2 13, w3 14, w4 16, w5 21, w6 28, w7 1; anchors 12.0.1, 12.0.2, 12.0.3, 12.0.4, 12.0.9.
- Interfaces: stable 3139, change 234, new 54, retire 84; incoming wiring requests: 6.

### 13. programs

- Paths: `programs/`, `precompiles/layerxprograms/`.
- The Rust program runtime, sandbox and evidence verifiers; task.13.0.3 connects runtime and host and task.13.22.1 builds the 0x101D executor. Reference guests deploy through MsgDeploy at activation (decision.13).
- Tasks by wave: w0 12, w1 20, w2 31, w3 21, w4 20, w5 8, w6 15, w7 1; anchors 13.0.1, 13.0.2, 13.0.3, 13.0.4, 13.0.9.
- Interfaces: stable 4858, change 168, new 99, retire 27; incoming wiring requests: 11.

### 14. web-explorer-bridge

- Paths: `apps/web/`, `apps/explorer/`, `apps/bridge/`, `services/explorer/`, `services/bridge/`, `sdk/js/`.
- Client applications and index services that use node APIs; the explorer status UI moves to the three rungs.
- Tasks by wave: w0 11, w1 32, w2 35, w3 22, w4 17, w5 28, w6 17, w7 1; anchors 14.0.1, 14.0.2, 14.0.3, 14.0.4, 14.0.9.
- Interfaces: stable 6779, change 354, new 66, retire 156; incoming wiring requests: 7.

### 15. cosmos-fork

- Paths: `sdk/` (everything not owned by 4, 7, 8, 12, 14).
- The in-tree SDK supplies BaseApp, contexts and the multistore: sdk/baseapp/abci.go:254 selects the proposal state for commitment and sdk/baseapp/abci.go:269 implements Commit.
- Tasks by wave: w0 1, w1 22, w2 1, w3 1, w4 1, w5 13, w6 17, w7 1; anchors 15.0.1, 15.0.2, 15.0.3, 15.0.4, 15.0.9.
- Interfaces: stable 5933, change 27, new 18, retire 92; incoming wiring requests: 3.

### 16. tools-ops

- Paths: `tools/`, `deploy/`, `docker/`, `scripts/`, `Makefile`, `chain.mk`, `.github/`, `tests/` (rest).
- Build, release, deployment and qualification tooling. Makefile:3145 declares the native prerequisites and Makefile:3147 the node build target.
- Tasks by wave: w0 14, w1 20, w2 9, w3 6, w4 6, w5 64, w6 33, w7 1; anchors 16.0.1, 16.0.2, 16.0.3, 16.0.4, 16.0.9.
- Interfaces: stable 6326, change 251, new 39, retire 93; incoming wiring requests: 231.

### 17. programme

- Paths: `spec/`, `docs/`, root `*.md`, `GOTCHA.kvx`, `codify.kvx`.
- The specification, documentation and aggregate delivery gates. Its wave-7 gate 17.0.9 requires every domain completion gate.
- Tasks by wave: w0 12, w1 10, w2 7, w3 6, w4 4, w5 11, w6 7, w7 8; anchors 17.0.1, 17.0.2, 17.0.3, 17.0.4, 17.0.9.
- Interfaces: stable 16, change 9, new 16, retire 0; incoming wiring requests: 7.

## Execution model

Tasks run in waves 0 to 7. Every task of a wave runs concurrently in its own sandbox; a wave starts when the previous wave's gate passed.

| Wave | Meaning | Tasks |
|---|---|---|
| 0 | foundations: type, ABI, proto and error-registry freezes, fixtures, scaffolds | 260 |
| 1 | core logic: keeper, kernel and crate internals, unit tests | 441 |
| 2 | surfaces: precompile methods, Msg and Query handlers, JSON-RPC, HTTP routes, CLI, events and errors | 439 |
| 3 | services: off-chain services and daemons against wave-2 surfaces | 221 |
| 4 | integration: node/app.go wiring, activation lists, genesis, params, SDK regeneration | 320 |
| 5 | clients: apps, SDKs, wallets and explorers repointed | 408 |
| 6 | conformance: parity goldens, retirements, migration rehearsal, performance bounds | 432 |
| 7 | completion: per-domain gates N.0.9 and the programme gate 17.0.9 | 24 |

- One acceptance criterion is one test is one task: each `[acceptance.N.k.a]` names exactly one test, each test belongs to exactly one task, and the task's `verify_cmd` runs exactly its listed tests.
- A task is bounded: one sandbox, 30 minutes, one build, one verify run, at most two repair cycles, at most 12 touched files, touches inside its own domain only.
- `gate_kind` is `planned` when any listed test does not exist at HEAD; planned test files appear in `planned_paths`.
- A wiring request names the owner domain, the insertion point `path:line`, the change and the requesting task; the build adds the fulfilling task to the requester's `requires`. The build checks the expanded graph for cycles with a linear strongly-connected-component pass.
- The Hyperfide CLI (`cg`) reads the merged document; the build asks it to parse the document and compute eligibility (`cg spec status`, `cg spec ready`) as a bounded acceptance step.

## Carried-forward decisions

Decisions 1 to 94 carry the earlier specification's rulings forward where the code at HEAD still agrees; each block's `basis` cites the code. Later decisions use `decision.<N*100+k>` per domain. The list below names the carried-forward set.

| Decision | Title |
|---|---|
| decision.1 | Kernel C modules and the Rust program runtime inside paxd |
| decision.2 | Kernel account and margin model |
| decision.3 | Home of session keys and delegated authorities |
| decision.4 | Custody claims and guarantor bonds at the activation height (superseded by decision.642) |
| decision.5 | Kernel balances without an addr binding at activation |
| decision.6 | Precompile error encoding |
| decision.7 | Time semantics under the sequencer cadence |
| decision.8 | Trading semantics where kernel and exchange differ |
| decision.9 | Program fee versus gas |
| decision.10 | Paid search settlement and lxpay placement |
| decision.11 | Program-origin web request terms |
| decision.12 | Program-origin observation admission |
| decision.13 | Reference guest deployment |
| decision.14 | Sponsorship path and per-identity budget home |
| decision.15 | Sequencer receipt signing key |
| decision.16 | DID key rotation and the embedded-wallet ed25519 share |
| decision.17 | Client-facing contract |
| decision.18 | Sequencer consensus scope |
| decision.19 | Sequencer execution mode |
| decision.20 | Legacy surfaces |
| decision.21 | layerxAnchor reads after the fold |
| decision.22 | Agent functions 0x0901-0x0904 |
| decision.23 | Revert reasons for addr and bank |
| decision.24 | Service dispute resolution |
| decision.25 | Exchange withdrawMargin |
| decision.26 | Signer of the agent-host web payer |
| decision.27 | Typed exact payment at 0x101A |
| decision.28 | xweb error naming |
| decision.29 | One stream state |
| decision.30 | Gas rule across the new precompiles |
| decision.31 | computeClearing and scoreBatch ABI form |
| decision.32 | submitPrice signer rule |
| decision.33 | TEE attestor roots |
| decision.34 | TEE signer threshold and key type |
| decision.35 | Result-code retryability source of truth |
| decision.36 | Receipt key range overlap |
| decision.37 | Stream rate update ordinal 8 |
| decision.38 | Blob store key layout |
| decision.39 | Payments and exchange governance surfaces |
| decision.40 | Kernel result codes -737 to -739 |
| decision.41 | Subscription lag signalling |
| decision.42 | Cross-module grant state stage |
| decision.43 | Activity proof verifier |
| decision.44 | Agent signing and client interfaces |
| decision.45 | Committed proof codecs |
| decision.46 | Sequencer handover encoding |
| decision.47 | .NET and Swift gates |
| decision.48 | PXVE v1 dev attestation envelope |
| decision.49 | Pure agent helpers gate on the agent module |
| decision.50 | AI reputation history retention prune runs at epoch opening |
| decision.51 | Merge plan name constant in the precompiles package |
| decision.52 | Gas station: a finalized earlier publication never consumes the renewal tick |
| decision.53 | Autobahn receiptsRoot is the same-block results root |
| decision.54 | KMS secp256k1 key kind for EVM sender handles |
| decision.55 | Wallet SDK formatting configuration |
| decision.56 | addr 0x1004 revert reasons use the registry LXP_ADDR codes |
| decision.57 | CosmWasm transaction counter is the block position from the merge plan height |
| decision.58 | Stream rate change is activity ordinal 8 end to end |
| decision.59 | Checkpoint settlement header without the retired policy field |
| decision.60 | One serde_json pin |
| decision.61 | FinalizeBlock response retention |
| decision.62 | Fatal kernel codes in the exchange code table |
| decision.63 | Intent plan vector holder |
| decision.64 | transactionsRoot verifier |
| decision.65 | History finality API removal order |
| decision.66 | Explorer paxeer-x page retirement |
| decision.67 | Guarantor header trim order |
| decision.68 | xweb program pair and the applied v6.10 state |
| decision.69 | newHeads receiptsRoot |
| decision.70 | lxphost raw-key scan scope |
| decision.71 | layerxagent params |
| decision.72 | Pay contract record views and stream rate event |
| decision.73 | Retained exchange intent selectors |
| decision.74 | Committed-proof verifier per language |
| decision.75 | Sequencer log ordinal |
| decision.76 | Indexer position migration |
| decision.77 | Prepared EIP-1559 fields order |
| decision.78 | native_terminal_outcome evidence |
| decision.79 | Explorer receipts heights |
| decision.80 | 8.73 acceptance vector |
| decision.81 | Gateway position cursor ownership |
| decision.82 | Wallet SDK curve library version |
| decision.83 | Oracle feeder transaction path |
| decision.84 | Model runner engine binding |
| decision.85 | Giga block fee-token charge destination |
| decision.86 | Curve library resolution in the wallet app |
| decision.87 | Vendor-native quotes in the exchange interface |
| decision.88 | Malformed snapshot ring refusal in launchpad price history |
| decision.89 | Relay journal restore admits the local signer beyond the observation cap |
| decision.90 | eth_subscribe relays over the WebSocket entry; HTTP answers WebSocket required |
| decision.91 | Explorer signature route vectors |
| decision.92 | Router sequence batch read |
| decision.93 | Hosted registry retirement is one compile unit |
| decision.94 | AI market guest size is met by a binaryen post-pass |

