# Programs

A LayerX kernel program is deterministic WASM that runs against protocol state. It can compute, use principal-scoped and shared storage, call other programs, and request transfers through explicitly granted authority. It never writes a balance.

## The monetary law

Guest code stages typed 402LXP transfer requests without mutating balances. The host's `TransferCapability` checks those requests against the invocation's granted authority and authorizes an atomic effect set. The kernel transfer primitive applies that set.

## Program-owned value

A program may hold value in an ordinary protocol account derived from its
program identifier and a public seed. Anyone can reproduce the account
identifier; nobody can turn that identifier into authority. Funding is an
ordinary principal-funded 402LXP transfer to the derived account. Spending is
a separate `ProgramSpend` grant binding the owner program, seed, rederived
source, asset, destination and cumulative amount ceiling. Only the deriving
program's own frame can stage that debit, and the kernel transfer primitive is
still the only balance writer.

The Rust SDK examples are complete custody patterns:

- `programs/sdk/rust/examples/escrow` records distinct immutable release and
  refund receipt digests in shared state, takes payment into its derived
  account, and pays exactly once after the selected receipt verifies a
  successful settlement for the escrow's exact asset and amount.
- `programs/sdk/rust/examples/vault` credits each caller in principal-scoped
  storage while maintaining the pooled total in shared storage. A withdrawal
  debits both ledgers and the real derived account in one atomic execution.

A program cannot derive another program's authority, stage a derived-account
debit from a callee frame, write balances, mint or burn value, perform an
unbounded whole-balance sweep, or treat bookkeeping storage as money. EVM
contract balances, Solana PDA-held value and CosmWasm contract balances map to
the same derived-account pattern. Allowances over third-party funds, direct
lamport writes, supply-burning messages and cross-chain transfers remain named
refusals where their source semantics are not representable.

```text
guest program  ->  AbiEffects
TransferCapability::authorize  ->  AtomicTransferSet
KernelTransferPrimitive::apply_and_verify_402lxp_set  ->  KernelTransferEvidence
KernelTransferPrimitive::verify_402lxp_transfer_set_root  ->  verified root
TransferCapability::settle_authorized_set  ->  VerifiedProgramSettlement
```

The kernel applies the transfer set atomically and verifies its commitment root. `VerifiedProgramSettlement` records that root, the leg count and total amount after they match the authorized set. The surrounding activity constructs the signed receipt after this transfer commitment; the settlement value is not itself a signed receipt.

The refusal taxonomy is closed and each variant means one thing:

| Refusal | Cause |
|---|---|
| `UnverifiedAuthority` | The invocation authority was not verified |
| `InvalidProgramAuthority` | The program-account debit authority is invalid |
| `InvalidProgramFunding` | The program funding destination is invalid |
| `InvalidTransfer` / `InvalidTransferSet` | The request or the set is malformed |
| `AmountOverflow` | The set's total exceeded the amount range |
| `InvariantViolation` | A monetary bypass was detected - the guest tried to move value outside the law |
| `CapabilityEscalation` | A child call's transfer exceeded the narrowed authority it was called with |
| `KernelRefused` / `Kernel(i32)` | The kernel refused the set, optionally preserving its result code |
| `ReceiptInvalid` / `ReceiptMismatch` | Settlement evidence is invalid, or does not bind this exact transfer set |
| `StaleStorage` | The prepared storage snapshot is stale |

`CapabilityEscalation` makes composition safe. Principal-scoped transfer authority
can narrow across calls. Program-owned spending stays in the deriving program's
frame: a child returns bounded typed proposals through the ordinary call response,
and the owner validates their exact account, seed, asset, destination and amount
against both the call-edge grant and its original cumulative ceiling before
staging them. The SDK's `ProposalBudget` retains that ceiling across repeated
calls; `invoke_owner_spending_proposals` validates the response and stages each
accepted payment through `pay_from_program_account` after the child returns.
Receiving a proposal or a narrowed grant never gives a child debit authority.

## Determinism

Determinism is enforced at build time and at execution time, not requested in a style guide. The validator rejects modules that reach for non-determinism; the meter bounds fuel, memory and storage against a declared `ResourceBudget` and a `FeeSchedule`; composition is bounded by maximum depth, fan-out, call-graph edges and program visits. A program that exceeds any declared limit is refused - it does not run halfway and leave state behind.

Replay is first-class: a recorded execution can be replayed and any divergence reported as a `ReplayRefusal`. The same inputs produce the same outputs on every node, which is what lets a receipt mean anything.

## Building and deploying

The runtime admits guest ABI versions 1 through 5. The Rust SDK's default `ABI_VERSION` is 4; the CLI's artifact fallback and call default are 2. Declare the ABI required by the artifact and use the deployed program's ABI for calls. `ProgramSpend` and `BalanceView` grants use the ABI-2 capability encoding, which is also used by ABI versions 3 through 5.

Program transactions require a funded EVM sender. Follow the [network quickstart](https://docs.paxeer.app/sdk-quickstarts/) and confirm the funding in committed chain state before spending.

Before an escrow's `OPEN`, register its derived value account using `registerAccount(bytes32,uint8,bytes32,bytes)` on the [Programs precompile](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/precompiles/layerxprograms/LayerXPrograms.sol). Deployment does not register that account automatically. Use the same program, asset and seed as the escrow call.

Run the CLI with a configured node RPC URL. Lifecycle commands require a local, owner-only EVM key file and an independently configured receipt policy:

```sh
layerx program build --manifest-path ./Cargo.toml
layerx --rpc "$PAXEER_RPC_URL" program deploy ./target/program.wasm \
  --program-id <hex32> --upgrade-authority <hex32> \
  --interface ./interface.bin --evm-key-file ./evm-key.json \
  --receipt-policy ./receipt-policy.json
layerx --rpc "$PAXEER_RPC_URL" program upgrade ./target/program.wasm \
  --program-id <hex32> --old-hash <hex32> \
  --evm-key-file ./evm-key.json --receipt-policy ./receipt-policy.json
layerx --rpc "$PAXEER_RPC_URL" program wind-down deprecate \
  --program-id <hex32> --exit-program <hex32> --deadline-batch <batch> \
  --evm-key-file ./evm-key.json --receipt-policy ./receipt-policy.json
layerx --rpc "$PAXEER_RPC_URL" program call <hex32> \
  --evm-key-file ./evm-key.json --abi-version 2 \
  --entrypoint layerx_call --calldata <hex> --fuel <units> --fee-limit <units>
layerx --rpc "$PAXEER_RPC_URL" program simulate <hex32> \
  --from <evm-address> --abi-version 2 --entrypoint layerx_call \
  --calldata <hex> --fuel <units>
layerx --rpc "$PAXEER_RPC_URL" program registry get <hex32>
layerx --rpc "$PAXEER_RPC_URL" program registry list
```

`build` compiles WASM and validates the deterministic runtime policy locally. Deploy, upgrade, wind-down and call submit signed EIP-1559 transactions to `0x000000000000000000000000000000000000101d`. The CLI obtains the sender nonce and gas information through RPC. Simulation uses `eth_call` with the `pending` block tag and does not commit state.

The lifecycle receipt policy is a closed JSON object with `protocol_version: 2`, `network_id: 125`, 32-byte hex `sequencer_id` and `sequencer_key`, and integer `first_batch` and `last_batch`. The first batch must be at least 1 and the last must be at least the first. Obtain these values from an independent trust policy, not from the answer being verified.

Deployment and upgrade read `abi_version` (or `abi`) from the enclosing `LayerX.toml` and `layerx-program.json`, defaulting to ABI 2. Conflicting declarations are refused. Omitting `--upgrade-authority` makes deployment immutable. The caller chooses the 32-byte hex program identifier.

`--interface` takes canonical binary interface encoding, rather than `interface.kvx` source. Its code hash and ABI must match the artifact. Upgrade preserves the existing interface unless given a replacement or `--clear-interface`; those options are mutually exclusive. `--allow-breaking-interface` requires a replacement interface. `--migration-hook` reads the exact hook bytes from a nonempty file.

The wind-down subcommands share `--program-id`, `--evm-key-file`, `--receipt-policy` and optional `--fee-limit`. `route` adds `--account`, `--asset`, `--destination` and optional hex `--seed` (at most 128 bytes, empty by default). `deprecate` adds `--exit-program` and `--deadline-batch`; `exit` adds `--account`; `tombstone` has no additional arguments.

Lifecycle success requires a signed receipt matching the transaction, the Programs target, module 12 operation 2, and the authenticated lifecycle event. Pending and unknown submissions retain those states. A program call's submission acknowledgement alone does not establish terminal execution; consumers must verify the resulting execution evidence.

`registry get` reads `getProgram` through `eth_call` and returns the program identifier, policy, authority, code hash, ABI version, version and active flag. `registry list` scans deployment events with bounded `eth_getLogs` requests. These RPC views do not independently prove consensus commit. `program discover`, `program interface get`, `program interface publish` and `program bindings` provide discovery, interface and binding operations.

Calls accept repeated `--capability` grants using the native capability set:

- `storage-read`, `storage-write`, `shared-storage-read`, `shared-storage-write`, `emit-event`
- `call:<program-hex32>`
- `transfer402:<asset-hex32>:<to-hex32>:<maximum-u128>`
- `receipt-read:<receipt-digest-hex32>`
- `program-spend:<owner-program-hex32>:<seed-hex>:<source-account-hex32>:<asset-hex32>:<to-hex32>:<maximum-u128>`
- `balance-view:<account-hex32>:<asset-hex32>:<receipt-digest-hex32>`

Unscoped legacy `transfer` and `compose` grants are refused. `--access-declaration` accepts canonical declaration hex; omission encodes an explicit absence marker, not an empty access set. Resource ceilings are configurable with `--memory-bytes`, `--storage-read-bytes`, `--storage-write-bytes`, `--output-values`, `--output-bytes`, `--table-elements`, and `--response-capacity`.

## Interpret or compile

The deterministic interpreter is an authoring convenience, not a cheaper or
equivalent execution tier. It is an ordinary ABI-v2 program: the outer Wasm
engine meters its instruction fuel, memory, storage reads, storage writes,
output values, output bytes and persistent occupancy through the same
protocol-owned `ResourceBudget` and `FeeSchedule` used for compiled programs.
The script decoder, bounds checks, register operations and control-flow loop
are therefore real metered work in addition to the host operations the script
requests.

Interpretation adds script decoding and dispatch to the metered guest work. Use it for bounded policies and automation when avoiding a separate compilation step matters; compile repeated or compute-heavy logic when execution cost dominates. Both routes use the same authority and isolation rules.

## The registry

The Programs precompile records deployed code, version, policy, authority and lifecycle state. Deprecation supports a wind-down in which value accounts can exit. The current `getProgram` view does not expose a source digest, so the CLI's source-verification command reports that source verification is unavailable.

## Restart and snapshot restore

The snapshot implementation retains program code as committed state across restore. The snapshot format carries a kernel-blob section holding every module blob the state root commits - module id, key and bytes in canonical order - under declared bounds (`LXP_SNAPSHOT_MAX_BLOBS`, `LXP_SNAPSHOT_MAX_BLOB_BYTES` and `LXP_SNAPSHOT_MAX_BLOB_TOTAL_BYTES` in `include/layerx/lxp_snapshot.h`). `lxp_snapshot_write` serialises the section, accounts for it in the snapshot size and refuses a blob set beyond the bound rather than dropping blobs (`src/state/lxp_snapshot.c`). `lxp_snapshot_load` parses the section structurally, rejects a snapshot whose parsed blobs do not reproduce the committed state root with a typed error, and rejects a legacy snapshot that omits the section while the state carries a registered Programs module (`src/state/lxp_snapshot.c`, `LXP_ERR_SNAPSHOT_BLOBS_MISSING`). Only after the whole snapshot validates does the loader replace the kernel blob store, freeing the previous blobs, so no stale executable bytes survive a restore.

After a restore the artifact resolver locates and hash-checks a program exactly as before, and a later `program call` executes under the recorded runtime version with the same receipt shape as before the restore.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Programs never write balances | `protocol` | Guest code produces typed transfer requests; the kernel owns every balance mutation. |
| Typed program failure with rollback | `protocol` | A refused or faulted execution rolls back program effects; the activity can still consume its sequence and charge metered fees. |
| Deterministic program execution | `protocol` | Validation, metering and composition bounds are enforced at build and at execution. |
| Conserved supply | `protocol` | Conservation is checked by the kernel primitive, not by the program. |
| Atomic settlement | `protocol` | The transfer set applies whole or not at all. |
| Offline receipt verification | `protocol` | Consumers verify the signed activity receipt separately from the kernel transfer commitment. |
