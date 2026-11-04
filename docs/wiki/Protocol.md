# Protocol

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID
`125` (`0x7d`). LayerX is merged into that network. The kernel is part of the
network and its block proposal path; execution, accounting and consensus
operate within the same chain. A receipt is final once its block is committed.

The pre-merge split between LayerX execution and Paxeer settlement is retired.
Checkpoint attestations and an external finality ladder are no longer the
payment completion procedure. The developer payment path is documented in
[Payments quickstart](PaymentsQuickstart.md).

Normative requirements live in `spec/`. The implementation paths below make
clear which wire format and authority each part of the network uses.

## One network, integrated execution

| Component | Role in the merged network | Implementation |
| --- | --- | --- |
| Node and consensus | Transaction admission, ordering, block proposal, execution and BFT commit | `daemon/`, `node/`, `consensus/`, `modules/sequencer/` |
| Kernel host | Deterministic native activity execution against chain state | `modules/lxphost/`, `src/`, `include/layerx/` |
| Ledger | Asset registration, accounts, transfers and supply accounting | `modules/layerxledger/`, `precompiles/layerxledger/` |
| Payments | Budgets, escrow, streams and service operations | `modules/layerxpay/`, `precompiles/layerxpay/` |
| Identity and authority | EVM-to-DID binding and authorization for native effects | `modules/evm/keeper/layerx_binding.go`, `modules/layerxauth/` |
| Programs | Deterministic guest execution and validated monetary effects | `programs/`, `precompiles/layerxprograms/` |
| Receipt API | Signed execution results and committed block association | `rpc/sequencer_receipt.go` |

These are components of the same network, not independent settlement systems.
Sub-chains are tenant domains represented by namespaces in the database;
a tenant does not require a separately deployed chain infrastructure.

## Public access

The public JSON-RPC listener is at the root of
`https://api-mainnet-beta.paxeer.network`. The numbered node hosts run from
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, also at the root. An `eth_chainId` read
returns `0x7d`. Public node namespaces include `debug`, `echo`, `eth`, `net`,
`pax`, `test`, and `txpool`; method availability also depends on node
configuration. See [Public JSON-RPC](PublicRpc.md).

There is no separate LayerX interface, router, or unified `/rpc` endpoint.
The retired `layerxd` binary is not a running network component.
`node.hyperpaxeer.com` is only the hpx library mirror used by builds, not a
public transaction endpoint.

Hosted gateway account and capability helpers have their own
[OpenRPC contract](../../services/platform/hosted/gateway/openrpc.json).
Obtain a hosted deployment's endpoint and authentication from its operator;
its `px_*` catalogue is not a promise that every node exposes those helpers.
In particular, hosted capability checks can refuse a disabled precompile
surface with `-32003 surface_unavailable`.

## The activity (LXC/1)

The native kernel retains the canonical LXC/1 activity envelope defined by
[`lxp_activity`](../../include/layerx/lxp_activity.h). Integers have canonical
binary encodings; payload hashes, signatures and complete decoding bind the
operation's bytes. Activity type high bits select the module and low bits
select its ordinal through `lxp_activity_module_id` and
`lxp_activity_type_ordinal`.

| Field | Role |
| --- | --- |
| `protocol_version` | Selects the native protocol version |
| `network_id` | Binds execution to the intended network |
| `activity_type` | Module ID and operation ordinal |
| `actor_did` | Acting identity |
| `authority` | Authority material interpreted by the kernel |
| `account_sequence` | Identity sequence, separate from EVM and debit-account sequences |
| `timestamp_bound` | `not_before` and `not_after` checked against execution time |
| `idempotency_key` | Binds recovery and duplicate handling to the original operation |
| `fee_limit` | Native activity fee bound |
| `payload` / `payload_hash` | Module body and its byte binding |
| `signature` | Native signature material for the applicable authorization path |

[`lxp_protocol.h`](../../include/layerx/lxp_protocol.h) declares legacy version
`1`, occupancy version `2`, and state-commitment version `3`; its generic
`LXP_PROTOCOL_VERSION` default remains `2`. The merged ledger adapter in
[`execute.go`](../../modules/layerxledger/keeper/execute.go) constructs
protocol-3 activities with the identity sequence read from chain state and
time bounds set to the current block time.

This internal envelope is distinct from the public EVM transaction. A client
signs an EIP-1559 transaction to a precompile; the adapter resolves its bound
caller and constructs the native operation. It does not require the client
to submit a separately signed LXC/1 envelope to another public service.
Envelope checks run in version, network, payload-binding order in
`lxp_activity_check_envelope`. The host and module apply authorization,
sequence, timestamp, payload and execution checks on their real paths.

## Three design rules

1. **Canonical state and history.** Deterministic execution and ordered
   receipts bind native effects to chain state. Kernel state roots describe
   native execution; consensus commit supplies network finality.
2. **Validated monetary effects.** Native modules and Programs guests express
   monetary effects through validated transfer sets. The kernel's `402LXP`
   machinery applies native balance changes; the ledger adapter mirrors the
   corresponding bank state. A guest's local bookkeeping cannot create funds.
3. **Reproducible results.** Consensus execution uses deterministic inputs,
   including block time and committed configuration. A local clock, external
   HTTP response, or an untrusted receipt label cannot establish a state
   transition or payment finality.

## Identity and authority

The address precompile at `0x0000000000000000000000000000000000001004` binds
an EVM address to an Ed25519 DID. `bindLayerX` requires the DID key's signature
over the chain ID, EVM address and binding nonce, while the transaction itself
supplies the EVM address's consent. The binding is one-to-one and nonce-bound.
See [`LayerXBindMessage`](../../modules/evm/types/layerx_binding.go) for exact
bytes and [Payments quickstart](PaymentsQuickstart.md#2-create-or-import-a-signing-key)
for the sequence of calls.

The ledger resolves a state-changing caller through
[`Actor`](../../modules/layerxledger/keeper/authority.go); an unbound caller
is refused with `LXP_UNAUTHORIZED_DEBIT`. Source-account ownership and
operation authority are still checked after binding. The authority precompile
at `0x000000000000000000000000000000000000101c` is a separate operation
surface within the same network, not a replacement for address binding.

Historical authority encodings do not imply every old operation remains live.
The ledger's payer-grant issue, revoke, read and draw routes are retired and
refuse with `LXP_NO_PAYER_GRANT`, subject to earlier authorization and pause
checks. Use the current keeper and served ABI when choosing a payment path.

## Programs: module `0x09`

The kernel identifies Programs as module `9` (`LXP_MODULE_PROGRAMS` in
[`lxp_module.h`](../../include/layerx/lxp_module.h)). Module IDs and EVM
addresses are different identifiers: the Programs precompile is
`0x000000000000000000000000000000000000101d`. The same header also declares
Asset, escrow, budget, stream, service, perps, governance, bridge, spot and
web module IDs; it is the source for the current enumeration.

Programs execute untrusted guests in the deterministic WASM runtime. They
emit validated transfers under the invoking authority rather than writing
balances directly. Program-derived accounts must be registered for the
selected Asset before receiving funds; derivation alone does not create
spending authority. Spending capabilities bind the account, Asset,
destination and ceiling. Storage occupancy is accounted for by the native
execution and fee machinery.

The ABI-v2 LXT20 and merchant examples preserve this boundary: token storage
and backing transfers commit or roll back together. See
[Programs](Programs.md) and the
[example walkthrough](PaymentsQuickstart.md#6-build-and-deploy-the-lxt20-example).
A registry entry, uploaded bytecode, or locally built guest is not evidence
that deployment executed successfully.

## From transaction to committed receipt

1. The client prepares calldata, estimates gas, chooses fee caps and signs a
   chain-125 EVM transaction locally. `eth_sendRawTransaction` submits those
   exact signed bytes and returns the transaction hash.
2. Native precompiles resolve the caller's bound DID, validate the operation,
   and execute through their keepers and kernel host. For example, ledger
   `payExact` constructs a native SEND using the source's current sequence,
   expiry and payment reference, then mirrors the resulting monetary effects.
3. Execution yields a receipt binding the transaction and its result. A
   successful submission is distinct from successful execution: inspect the
   status, revert information and expected effects. A reverted transaction
   can still incur EVM gas charges.
4. The network commits the block through BFT consensus. The committed receipt
   associates the execution result with its landing block. There is no later
   checkpoint settlement step needed to make that block final.

[`pax_sequencerReceipt`](../../rpc/sequencer_receipt.go) reports these rungs:

| Rung | Evidence | Meaning |
| --- | --- | --- |
| `submitted` | Transaction hash and nonce | Accepted for tracking, without an execution receipt |
| `receipted` | Signed receipt and execution fields | Execution evidence before committed block association |
| `committed` | Receipt plus `blockNumber` and `blockHash` | Execution included in a committed block |

The method name is retained by the node API. It does not describe an
independently deployed execution network. Receipt signatures authenticate
execution under a trusted signing key; a signer public key in the response
must be checked against independently trusted key IDs and height ranges.
For independent committed verification, check consensus header, validator
commit and transaction inclusion as well as the receipt's signature and
transaction binding. The verifier implementations are in
[`sdk/platform/go/committed.go`](../../sdk/platform/go/committed.go) and
[`layerx-committed-proof`](../../services/interop/crates/layerx-committed-proof/src/lib.rs).

The EVM read `eth_getTransactionReceipt` supplies transaction receipt data;
`pax_sequencerReceipt` additionally exposes the signed execution and commitment
view. A JSON label alone does not prove consensus finality. See
[Portable receipt verifier](PortableVerifier.md), [Finality](Finality.md),
and [Commitment levels](CommitmentLevels.md).

HTTP 402 adapters verify these network results without creating their own
balances. Current payment evidence uses `layerx-receipt-proof-v2` and the
`receipted` / `committed` policy. A successful payment response identifies
the EVM transaction hash. The payment implementation and account preparation
are covered in [Payments quickstart](PaymentsQuickstart.md#7-pay-an-http-402-endpoint).

## Start here

- [Home](Home.md)
- [Getting started](Getting-Started-Beta.md)
- [Payments quickstart](PaymentsQuickstart.md)
- [Modules](Modules.md)
- [Assets](Assets.md)
- [Fees](Fees.md)
- [Programs](Programs.md)
- [Public JSON-RPC](PublicRpc.md)
