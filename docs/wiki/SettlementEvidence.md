# Settlement evidence

Paxeer mainnet-beta is one BFT network, with EVM chain ID `125` (`0x7d`).
The kernel runs inside `paxd` through `modules/lxphost` and participates in
block production. A transaction receipt is final once its block commits.
Execution, receipt authentication, block inclusion and consensus commitment
are distinct facts that a consumer must check.

## Authority and publication

The node's signed execution receipt authenticates an execution result under
a governed receipt key. The BFT validator commit establishes consensus
finality. A valid receipt signature alone establishes neither block inclusion
nor commitment; a committed transaction can also contain a failed execution.

`rpc/sequencer_receipt.go` implements `pax_sequencerReceipt` with these states:

| `rung` | Evidence |
| --- | --- |
| `submitted` | The node holds the transaction, but has no signed receipt for it. |
| `receipted` | The signed execution receipt is available, before its covering batch commits. |
| `committed` | A committed block's `MsgSequenceBatch` covers the receipt's sequence; the response includes `blockNumber` and `blockHash`. |

An unknown transaction returns `null`. The node uses committed sequence state
to find the covering block. A node with neither a local receipt source nor a
configured forwarding endpoint returns JSON-RPC error `-32000` with the
message `pax_sequencerReceipt served by sequencer`. This describes a role
inside the node.

`SequencerReceiptResult.Verify` checks the signed receipt against its JSON
view, including the return-data, log-data and VM-error digests. Supply trusted
receipt key pins to authenticate the signer. Without pins, this method checks
consistency and the signature under the supplied key, but does not establish
that key's authority. The response's `signingKey` cannot authorize itself.
The node's governed key reader, declared as `px_getReceiptKeys`, carries
`keyId`, `publicKey`, `firstHeight` and `lastHeight`; key height bounds are JSON
numbers. Production trust requires authenticated chain state or an
independently trusted chain configuration for these pins.

The node receipt's `PreviousStateRoot` and `ResultingStateRoot` fields carry
sequence-chain hashes (`node/sequencer/receipt.go`). They must not be treated
as native composite state roots or as the Cosmos application hash.

## Consumer sequence

1. Use the configured node's HTTPS root for JSON-RPC and require `eth_chainId`
   to return `0x7d`. Chain ID selects a network; it does not authenticate a server
   or validator set. See [Public RPC](PublicRpc.md).
2. Fetch `pax_sequencerReceipt` with the transaction hash. Preserve the raw
   signed transaction, canonical receipt bytes, JSON view and key
   authorization. Verify the pinned key identifier and its height range,
   receipt signature, transaction hash, sender, nonce and sequence bindings.
3. Verify execution status, return data, logs and the operation-specific
   fields the application relies on. A committed revert is final evidence
   of failure, not proof of a successful payment or state change.
4. Require the `committed` rung before treating the execution as final. Bind
   the returned block identity and the transaction's position to the actual
   committed sequence batch. For independent verification, authenticate the
   block header and validator commit under a trusted validator history or
   light-client trust anchor. An RPC response alone supplies no independent
   consensus proof.
5. When proving execution results, verify their inclusion as well. The
   recorded V4 example proves the sequence batch and wrapped EVM transaction
   against the execution header's `data_hash`, then proves the execution
   result against the successor header's `last_results_hash`. It verifies
   both blocks' commits and their continuity, and requires more than two
   thirds of validator voting power. Waiting for that successor proof is an
   evidence-availability step; the execution block is final at its commit.

[SDK terminal verification](SdkTerminalVerification.md) describes these
bindings and the reference evidence at
`daemon/committed-fixture/vectors/vector-v4-committed.json`.
`TestVectorV4CommittedVerifies` in `daemon/committed-fixture/vector_v4_test.go`
checks its chain evidence separately from its signed inclusion envelope.
The recorded fixture's validator set is test material; production consumers
need their own chain trust anchor.

## Native state witness version 2

Native state witnesses prove a key/value leaf under a supplied composite
kernel state root. Their codec version is independent of receipt protocol
versions. A witness does not by itself authenticate the supplied root, identify a committed block, or
authorize a transfer. Establish the root's provenance and state height
separately before relying on the proven value.

The C representation and entry points are declared in
`include/layerx/lxp_state_proof.h` and implemented in
`src/state/lxp_state_proof.c`. All integers in the wire format are unsigned
big-endian; each sibling is 32 bytes:

```text
version:u16=2 || module_id:u16 || key_len:u32 || key || value_len:u32 || value
|| [account_index:u32 || account_count:u32 || account_depth:u8 || account_siblings[32]*]
|| leaf_index_a:u32 || leaf_count_a:u32 || depth_a:u8 || siblings_a[32]*
|| leaf_count_b:u32 || depth_b:u8 || siblings_b[32]*
```

The bracketed account segment is present exactly when `module_id == 0`,
`key_len == 33` and the first key byte is `0x04`. Layer B's index is the
module ID and is implicit on the wire.

The leaf hash is:

```text
SHA256("LXP/v1/state-leaf\0" || key_len:u32 || value_len:u32 || key || value)
```

Both lengths precede the key in the hash preimage, although each length
precedes its own bytes on the wire. Internal nodes hash:

```text
SHA256("LXP/v1/state-node\0" || left[32] || right[32])
```

The `\0` notation denotes one zero byte. Paths use positional left/right
ordering. At an odd width, the last node duplicates itself and the proof must
carry that exact sibling. Counts, indices, depths and trailing bytes are
checked strictly.

Layer A proves the leaf in its module subtree. Layer B proves a wrapper leaf
whose key is `module_id:u16` and whose value is the 32-byte module subtree
root, using the same state-leaf hash. Empty module subtrees still occupy
their positions in the composite tree.

### Module-tree width

`lxp_state_module_root_count` in `src/state/lxp_state_root.c` starts with
module IDs 0 through 8 and extends through the highest registered module ID.
The width is not fixed at 9 or 10 leaves. The current IDs in
`include/layerx/lxp_module.h` include service `5`, programs `9`, spot `10`
and the retained web ID `11`; `LXP_MODULE_RESERVED_COUNT` is `11`. Registering
spot therefore requires 11 module positions, and a root extending through
ID 11 has 12 positions.

The C witness verifier accepts module-tree counts from 9 through
`LXP_MODULE_RESERVED_COUNT + 1`, with the module index inside the stated
count. The retained Go codec in `layerxproof/codec/state.go` mirrors this
bound with `MaxStateModuleID == 11`. Use the producer's actual count and root;
accepting a permitted count alone does not verify the proof.

### Account registry path

For a module-zero key `0x04 || account_id32`,
`lx_account_state_leaf_material` supplies the canonical account leaf. The
account path folds that leaf into the account registry root. That root is
then the value of the 12-byte key `account-tree`; layer A proves this binding
in module zero, and layer B proves its module wrapper in the composite tree.
All other keys omit the account segment.

`layerxproof/verify/state.go` retains two Go consumers:

- `StateProof(witnessBytes, stateRoot)` decodes the witness and requires its
  reconstructed root to equal the supplied root.
- `AccountProof(witnessBytes, stateRoot, expectedAccount, expectedAsset)`
  additionally requires an account path, binds the keyed account identity,
  decodes the canonical account value and, when supplied, checks asset
  identity.

`lxp_state_proof_build` constructs the native paths and verifies them against
the computed root before returning. `lxp_state_proof_encode`,
`lxp_state_proof_decode` and `lxp_state_proof_verify` share the representation.
Allocate `lxp_state_witness` on the heap: its value buffer can hold one MiB.
Successful decoding establishes a canonical witness; successful verification
also requires the expected root to match.

## Native verification boundary

The native and Go proof codecs verify witness bytes against an independently
authenticated root. The `layerxVerify` precompile refuses calls with
`LXP_MODULE_DISABLED: module disabled; layerxVerify retired` and does not serve
these proof operations. Current transfers use native signed transactions and
their operation-specific authority checks. A balance witness alone grants no
permission to debit an account.

[SDK terminal verification](SdkTerminalVerification.md) ·
[Public RPC](PublicRpc.md) · [Home](Home.md)
