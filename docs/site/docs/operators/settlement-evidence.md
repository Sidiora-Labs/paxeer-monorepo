# Settlement evidence

Current settlement evidence belongs to Paxeer's committed BFT chain, EVM chain
id `125` (`0x7d`). The kernel runs inside the network. A successful transaction
settles when its block is committed; a later custody checkpoint or guarantor
certificate is not required. See [Paxeer settlement](../concepts/paxeer-settlement.md)
for the receipt lifecycle and the merchant verifier.

This page separates current block/state evidence from historical native witness
and publication formats. Archive decoding does not enable a retired payout path.

## Collect evidence at a committed height

Use a node endpoint at its root path, for example
`https://api1.mainnet-beta.paxeer.network/`, and check `eth_chainId` is `0x7d`.
The public node set is `api1` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`. `node.hyperpaxeer.com` is a library mirror,
not an RPC endpoint. Hosted `px_` methods are gateway methods and are not
implied by a public node's method catalogue.

For a native bank or ledger movement:

1. Retain the signed transaction and its transaction hash, expected asset or
   denom, base-unit amount, sender, recipient and application request binding.
2. Fetch `pax_sequencerReceipt`. Verify its canonical receipt with the pinned
   receipt key valid at its signed height. A `submitted` or `receipted` rung
   is not committed evidence. Keep unavailable evidence pending.
3. Once the rung is `committed`, fetch `eth_getTransactionReceipt`. Match its
   transaction hash, block number and hash, execution status and logs to the
   signed execution. A committed failure is not a successful payment.
4. For bank's context-hash send overload, verify the `LayerXReceipt` event from
   bank `0x0000000000000000000000000000000000001001`, including sender,
   recipient, denom, amount and context hash. For ledger operations, use the
   served ledger ABI and its actual events. Do not infer transfers from labels.
5. Read balances and metadata at an explicit block height. Hosted
   `px_getBalances` and `px_listAssets` return a height and pin their chain reads
   to it; their JSON alone does not prove membership in a committed state root.

The bank/ledger mapping and distinction between bank supply and ledger issued
units are in [Assets and tokens](../concepts/assets.md). A pointer view is a
lookup of a token interface, not custody or balance evidence.

## Authenticate a block independently

The hosted gateway's `src/chain_reads.rs` implements `px_getCommit`,
`px_getValidators`, `px_getSequenceBatch` and `px_getReceiptKeys`; its
`openrpc.json` declares their request and result shapes. The first three take
a positional height; `px_getReceiptKeys` takes an empty parameter array and
returns `{keys:[{keyId,publicKey,firstHeight,lastHeight}]}`. The receipt key's
inclusive height interval is part of its authorization.

The verifier in `services/interop/crates/layerx-committed-proof/src/lib.rs`
provides two distinct steps:

- `verify_receipted` binds raw transaction bytes, canonical receipt, the JSON
  execution view and the previous sequence hash to an authorized Ed25519 key.
- `committed::verify_committed` authenticates sequence-batch inclusion under a
  BFT commit. It checks the validator-set hash and more than two-thirds voting
  power, the expected consensus chain id, block links and transaction inclusion.

The committed proof includes an execution header at height H and a signed
successor header at H+1. The latter authenticates the execution block through
`LastBlockID` and the validator-set transition. It then proves the sequence
batch at transaction index zero, the activity list, and the receipt's position
in that batch. Preserve the exact transaction bytes, aunts and ordered activity
ids. Do not substitute an EVM transaction hash for the consensus transaction
leaf or use an unauthenticated validator list as a trust anchor.

EVM chain id `125` and the consensus header's textual chain id are different
fields. Pin the expected network and authenticate the validator set under the
consumer's trust policy. Fetching a commit and validator set from one server
without establishing trust in that set is not independent network authentication.

## Prove committed state with ICS23

SDK state uses the chain application hash. A native kernel composite root and
the application hash are different commitments. To verify a value in a module
store, retain the exact store name, key bytes, value bytes, query height and
both proof operations:

1. Verify the IAVL membership proof for the key/value against the store root
   using `ics23.IavlSpec`.
2. Verify the multistore proof for `(store_name, store_root)` against the
   committed application hash using `ics23.TendermintSpec`.
3. Bind that application hash to an authenticated consensus header. Account for
   the consensus header's state-height relationship; do not assume a query
   height and a block's post-execution state are interchangeable.

`modules/sequencer/keeper/ics23_vector_test.go` produces receipt-key proofs
using an ABCI query with `Path: "/sequencer/key"`, the exact `ReceiptKeysKey`,
`Height: commit.Version` and `Prove: true`.
`modules/sequencer/keeper/ics23_module_catalogue_vector_test.go` does the same
for program metadata and blobs under `Path: "/lxphost/key"`. Both verify the
inner and outer proof. These are ABCI store query paths, not public HTTP routes.

The shared vectors are `programs/tests/vectors/ics23-receipt-keys-app-hash.json`
and `programs/tests/vectors/ics23-module-catalogue-app-hash.json`. Decode exact
producer bytes and reject substituted keys, values, roots or heights. A present
value's membership proof cannot establish an unrelated value's absence.

## Receipts, ownership, and recovery

A valid receipt proves only the execution it binds. Verify the expected asset,
amount, owner or payer authority, recipient and application context before
releasing goods or recording a payment. A signature over a different recipient
or network cannot be reused. The merchant verifier additionally compares the
committed receipt with the EVM transaction receipt; see
[Paxeer settlement](../concepts/paxeer-settlement.md#merchant-verification).

Retain canonical evidence, verification outcomes and durable request/transaction
bindings so a restart can resume lookup instead of issuing a second payment.
Missing evidence is pending or unavailable, never a zero balance or fabricated
success. The [Agent boundary](../platform/agent-boundary.md) serves validated
receipt bytes but does not independently establish a BFT commit.

## Native state witness version 2

The retained native witness codec proves a kernel key/value to a kernel
composite root. It is useful for replay and archive verification. It is not an
ICS23 application-hash proof, a receipt signature or a payout authorization.
Its producer and verifier are `src/state/lxp_state_proof.c` and
`include/layerx/lxp_state_proof.h`; the Go codec is `layerxproof/codec/state.go`.
All integers are unsigned big-endian:

```text
version:u16=2 || module_id:u16 || key_len:u32 || key || value_len:u32 || value
|| [account_index:u32 || account_count:u32 || account_depth:u8 || account_siblings[32]*]
|| leaf_index_a:u32 || leaf_count_a:u32 || depth_a:u8 || siblings_a[32]*
|| leaf_count_b:u32 || depth_b:u8 || siblings_b[32]*
```

The leaf preimage is
`"LXP/v1/state-leaf\0" || key_len:u32 || value_len:u32 || key || value`.
Both lengths precede the key in the hash preimage; on the wire, each length
precedes its own bytes. Internal nodes hash
`"LXP/v1/state-node\0" || left32 || right32` with SHA-256. At odd widths the
last node duplicates itself and the witness must supply that exact sibling.
Counts, indices, depths and trailing bytes are validated strictly.

The optional account segment appears exactly for module `0`, a 33-byte key and
first key byte `0x04`. It proves the canonical account leaf into the account
registry. That root becomes the value of `account-tree` (12 bytes); layer A
proves this binding in module zero. Other leaves omit the segment.

Layer B uses index `module_id` and the module wrapper leaf
`SHA256("LXP/v1/state-leaf\0" || u32(2) || u32(32) || module_id:u16 || subtree_root32)`.
The codec accepts 9 through 12 module leaves, bounded by
`LXP_MODULE_RESERVED_COUNT == 11`; the actual tree extends through its highest
registered module. Do not hard-code the former 9/10-leaf assumption.

`lxp_state_proof_build` verifies the constructed proof before returning it;
`lxp_state_proof_encode`, `decode` and `verify` share the representation. Heap
allocate `lxp_state_witness`, which includes up to one MiB of value storage.
The canonical account value still binds account identity, asset, balance and
authority; a caller-supplied recipient cannot override those bindings.

## Historical publication formats

The pre-merge checkpoint/custody publication lifecycle is retired. The following
information is only for interpreting retained version-2 artifacts, not for
provisioning a producer, deploying contracts, registering roots or making claims.

Historical vectors used big-endian framing
`tag || count:u32 || (item_length:u32 || item)*`:

| Tag | Archived item |
| --- | --- |
| `LXP/Paxeer/withdrawal-witnesses/v2\0` | `withdrawal_id32`, semantic withdrawal leaf and checkpoint proof |
| `LXP/Paxeer/balance-witnesses/v2\0` | `account32`, `asset32`, `amount:u128`, `recipient20` and checkpoint proof |

A checkpoint publication digest bound
`SHA256(abi.encode(checkpointHash,uint16(2),withdrawals,balances))`; a deposit
publication digest bound
`SHA256(abi.encode(uint16(2),registration,signature,leafOrdering))`. The
publication envelope version, native witness version and checkpoint ordering
version were separate fields, even where each happened to equal 2.

The archived owner authorization preimage was:

```text
"LX:SETTLE:RECIPIENT:v1" || 0x00 || network_id:u32 || account32 || asset32
|| recipient20 || request_anchor32
```

It required the account owner's Ed25519 signature. The archived deposit-root
signature used a separately provisioned Ed25519 authority and this preimage,
without a terminating zero after its domain:

```text
"LX:PAXEER:DEPOSIT:ROOT:v1" || checkpoint_id32 || checkpoint_state_root32
|| deposit_root32 || custody_reference32 || network_id:u32 || protocol_version:u16
```

Replay cannot manufacture either authority's signature. Preserve both the
request anchor and inclusion checkpoint when decoding: one bound the request
and nullifier, while the other selected the proving root. Archived ancestry,
recipient, canonical encoding and replay/nullifier checks remain necessary to
interpret an old claim. Confirmation-depth and publication-contract procedures
from those artifacts do not define current BFT settlement.

[Home](../index.md)
