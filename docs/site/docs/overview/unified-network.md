# Unified Network Architecture

Paxeer mainnet-beta is one BFT chain, run by `paxd`, with EVM chain ID `125`
(`0x7d`). The kernel is part of the network and proposes blocks. EVM
transactions, native modules and kernel-backed ledger operations participate
in that chain's execution and commitment. A receipt is final once its block
is committed.

## Execution inside the network

The chain entry point is `daemon/paxd/`. The Go application in `node/app.go`
wires the Cosmos-SDK modules, EVM and embedded kernel host. The kernel code
lives under `src/` and `include/layerx/`; `modules/lxphost/` connects it to
chain state. Native precompiles under `precompiles/` expose module operations
to EVM callers. `node/sequencer/` and `modules/sequencer/` implement ordered
transaction execution, receipt production and committed sequence tracking
inside this network.

The pre-merge router, separate `layerxd` daemon and execution-to-settlement
bridge are retired. The historical LayerXVerify, LayerXCustody and LayerXAnchor
surfaces at `0x1012`, `0x1013` and `0x1014` are retired integration guidance;
they do not define current funding or finality. Current applications submit
signed EVM transactions and verify committed-chain evidence.

Sub-chains are tenant domains: namespaces in the database, launchable without
new infrastructure. A tenant identifier scopes data and authority; it does
not imply a separately deployed consensus network. Services must retain that
domain when reading or writing tenant state.

## Accounts and native operations

An EVM address may be bound to an Ed25519 DID through the `addr` precompile at
`0x0000000000000000000000000000000000001004`. The interface is
`precompiles/addr/Addr.sol`; binding messages and identity names are defined in
`modules/evm/types/layerx_binding.go`.

The DID is `did:layerx:<64 lowercase hex characters of the public key>`.
`bindLayerX(didPublicKey, signature)` requires both the EVM caller's transaction
and the DID key's signature over:

```text
"LX:PAXEER-BIND:v1" || chain_id (uint256 big-endian)
                     || evm_address (20 bytes) || nonce (uint64 big-endian)
```

Read `layerXBindNonce(address)` before signing. An EVM address has at most one
DID and a DID at most one EVM address. `unbindLayerX()` removes the caller's
binding and consumes a nonce. `getUnifiedAccount(address)` reports the EVM
address, Pax address, DID public key and main account ID without reverting
for a missing identity; absent fields use the interface's empty or zero
values. `getLayerXDid` and the reverse `getEvmAddrByLayerX` read revert when
unbound. [One account](../concepts/one-account.md) describes derivation,
consent, recovery and chain-domain compatibility.

The LayerXLedger precompile at
`0x000000000000000000000000000000000000101a` exposes the ABI in
`precompiles/layerxledger/abi.json`. Asset registration, per-asset account
opening, issuance and payments execute through the native keeper in
`modules/layerxledger/`. Read balances through `balanceOf(accountId, assetId)`
and submit mutations as EVM transactions. Native PAX for gas is funded with
chain transfers; binding and opening an asset account do not create funds.
The [getting-started lifecycle](getting-started.md#ledger-assets-and-payments)
shows the register/open/mint/payment order.

## Receipts and commitment

`eth_getTransactionReceipt` reports EVM execution status and block metadata.
The optional `pax_sequencerReceipt` API in `rpc/sequencer_receipt.go` also
reports signed receipt progress:

| Rung | Meaning |
| --- | --- |
| `submitted` | The node has a send record but no signed execution receipt. |
| `receipted` | A signed execution receipt is available; commitment has not yet been established. |
| `committed` | A committed block's `MsgSequenceBatch` covers the receipt's sequence. |

A committed transaction can still have failed execution. Check status and
return data as well as commitment. The receipt signature binds execution
fields under a pinned receipt key; it does not itself prove a BFT commit.

Independent verification needs authenticated receipt key ranges, the signed
transaction, the execution header, transaction and sequence-batch inclusion
proofs, and a validator commit with more than two thirds of voting power.
Execution results from height H are proven against `LastResultsHash` in the
committed successor header at H+1. Verifying that successor authenticates the
result proof; it does not introduce a second network or settlement authority.
Authenticate the chain identity and validator history from a trusted source.

[SDK terminal verification](../agents/sdk-verification.md) documents current
verifier entry points, the recorded evidence and refusal cases. An RPC
`committed` label is the node's report; an independent client establishes the
same conclusion by checking that evidence.

## Public node JSON-RPC and the hosted gateway

Public HTTPS JSON-RPC is at the root of
`https://api-mainnet-beta.paxeer.network` and
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. Require `eth_chainId` to return
`"0x7d"`. These nodes serve the merged network. The public node namespace
inventory is `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`;
method availability still depends on the node's configuration.

The old `/rpc`, `/rpc/ws` and `/rpc/schema` public paths are retired.
`node.hyperpaxeer.com` is the hpx library mirror used by build scripts, not an RPC endpoint.

The hosted gateway under `services/platform/hosted/gateway/` is an
operator-deployed service. Use the endpoint supplied by its operator. Its
`openrpc.json` declares the `px_*` operations and parameter/result shapes;
these methods are not a separate execution network. Gateway operators
configure EVM upstreams with `LAYERX_GATEWAY_PAXEER_RPC_URLS`, a JSON array of
two to eight distinct endpoints, as parsed by `src/paxeer.rs` in that service.
Consumers of the shared Human service gateway client configure
`LAYERX_NETWORK_GATEWAY_ENDPOINT`.

| Gateway method | Current contract |
| --- | --- |
| `px_resolveAccount`, `px_getAccount` | Resolve an EVM address or explicit DID through the addr precompile and report the binding. |
| `px_getBalances`, `px_listAssets` | Return the gateway's declared balance and asset views; retain source and verification metadata. |
| `px_getNetwork` | Report `network_id`, EVM `paxeer.chain_id` and `paxeer.latest_block`, and consensus head information when available. |
| `px_getHistory`, `px_getUnifiedHistory` | Return indexed history with the declared pagination and freshness fields. |
| `px_getCapabilities` | Probe the declared native precompile capabilities at a recorded RPC height. |

Identity results carry `evm_address`, `pax_address`, `layerx_did`,
`layerx_account` and `bound`. Keep the original EVM or DID selector for later
lookups: `layerx_account` is a separate native reference. A bare 64-hex
identifier is refused with `-32001` and
`data.code = "native_account_reverse_resolution_unsupported"`; it is not
reinterpreted as a DID. Conflicting binding joins also refuse.

`px_getNetwork` reports `consensus: null` and
`reason: "consensus_unavailable"` when it cannot read consensus status; the
EVM chain ID and latest block can still answer. Keep that partial availability
explicit. A reported balance, index record or head is not a substitute for a
verified committed receipt.

## Unified intent planning

The Human service plans wallet and ledger operations from observed state and
user-authorized allowances. Its implementation is under
`services/human/crates/layerx-human-service/src/journeys/`.
`ObservedStateBuilder` assembles the required observations; a missing source
is an explicit refusal rather than an assumed zero balance.

`POST /v1/intents/plan` returns the proposed legs, fees, plan digest and signing
requirements without executing a journey. `POST /v1/intents/submit` re-plans
against current observations and refuses when the recomputed digest differs
from the signed plan. Progress is available through
`GET /v1/journeys/{journey_id}`. These are service operations over the same
network and require the deployment's authentication and allowance policy.

[Getting started](getting-started.md) ·
[One account](../concepts/one-account.md) ·
[SDK terminal verification](../agents/sdk-verification.md) ·
[Home](../index.md)
