# Unified Network Architecture

Paxeer mainnet-beta is one BFT chain, run by `paxd`, with EVM chain ID `125`
(`0x7d`). The kernel is part of the network and proposes blocks. EVM
transactions, native modules and kernel-backed operations participate in the
same chain's execution and commitment. A receipt is final once its block is
committed. Start with the [Quickstart](Quickstart.md) for a transaction
walkthrough.

## Execution inside the network

The chain entry point is `daemon/paxd/`. The Go application in `node/app.go`
wires the Cosmos-SDK modules, EVM and embedded kernel host. Kernel code lives
under `src/` and `include/layerx/`; `modules/lxphost/` connects it to the
transaction's chain state. Native precompiles under `precompiles/` expose
module operations to EVM callers.

`node/sequencer/` and `modules/sequencer/` implement ordered transaction
execution, signed receipts and committed sequence tracking within the
network. The proposer executes transactions and seals a `MsgSequenceBatch`
for validation and commitment by the chain. Early execution receipts and
committed blocks are stages of the same network's transaction lifecycle.

Sub-chains are tenant domains: namespaces in the database, launchable without
new infrastructure. A tenant identifier scopes data and authority; it does
not require a separately deployed consensus network. Services must carry the
domain through reads, writes and authorization.

## Account binding

The `addr` precompile at
`0x0000000000000000000000000000000000001004` links an EVM address to an
Ed25519 DID. Its interface is `precompiles/addr/Addr.sol`; binding messages
and identity names are defined in `modules/evm/types/layerx_binding.go`.

`bindLayerX(didPublicKey, signature)` requires consent from both keys: the DID
key signs this message, and the EVM owner submits the transaction:

```text
"LX:PAXEER-BIND:v1" || chain_id (uint256 big-endian)
                     || evm_address (20 bytes) || nonce (uint64 big-endian)
```

Read `layerXBindNonce(address)` before signing. The DID is rendered as
`did:layerx:<64 lowercase hex characters of the public key>`. An EVM address
has at most one DID and a DID at most one EVM address. `unbindLayerX()`
removes the caller's binding and consumes a nonce. Binding is not callable
through `staticcall` or `delegatecall`.

| Read | Result |
| --- | --- |
| `getPaxAddr(address)` | Associated Pax address. |
| `getEvmAddr(paxAddr)` | EVM address associated with a Pax address. |
| `getLayerXDid(address)` | DID public key and DID string; reverts when unbound. |
| `getEvmAddrByLayerX(didPublicKey)` | Bound EVM address; reverts when unbound. |
| `getUnifiedAccount(address)` | EVM address, Pax address, DID public key and native main-account ID. |

`getUnifiedAccount` does not revert for a missing identity: the Pax address
is empty without an association, and DID key and main-account ID are zero
without a binding. The main-account name is
`agent:did:layerx:<hex>:main`, with its ID derived by `codec.DeriveAccountID`.
That reference is different from the per-asset account IDs used by ledger
payments. Genesis binding entries are checked by
`ValidateLayerXGenesisEntry`.

## Ledger assets and payments

LayerXLedger at `0x000000000000000000000000000000000000101a` exposes
`precompiles/layerxledger/abi.json`; the native keeper lives in
`modules/layerxledger/`. Asset registration, per-asset account opening,
issuance and payments execute through EVM transactions on this chain.

The normal lifecycle is binding, `register`, `openAccount`, `mint`, then
`payExact` or a canonical `send` envelope. Issuance requires the authorized
issuer and asset policy. Read `accountId(owner, assetId)` for the per-asset
ID, `balanceOf(accountId, assetId)` for units and
`nextSequence(accountId, assetId)` for the source sequence. `payExact`
selects that sequence in the keeper and emits `PaymentExact`. The
[Quickstart ledger walkthrough](Quickstart.md#4-use-native-ledger-assets)
gives the call order and units.

A native PAX balance pays gas; a per-asset ledger balance records asset
units. Opening an account does not mint funds.

## Native application precompiles

These interfaces are native operations within the same EVM and commitment
model. Use the current ABI and deployment capability rather than inferring
support from an address alone.

### Perps — `0x0000000000000000000000000000000000001015`

This address serves the native perpetuals precompile. Its exact signatures and overloads are in
`precompiles/perps/abi/abi.json`; implementation is under
`precompiles/perps/` and `modules/perps/`.

| Operations | Methods |
| --- | --- |
| Collateral | `deposit`, `withdraw`, `transferCollateral`, `addCollateral`, `removeCollateral` |
| Positions | `openPosition`, `closePosition`, `partialClose`, `addSize` |
| Orders | `placeOrder`, `placeLimitOrder`, `cancelOrder`, `cancelByCloid` |
| Views | `getAccount`, `getMarket`, `getPosition`, `getOrder`, `getMarkPrice`, `totalMarkets` |

All methods are nonpayable. Use the ABI's collateral arguments rather than
sending native value with the call. These method groups are a starting point;
the ABI includes further margin, funding, order and liquidation operations.
See [Perps](Perps.md) for the product workflow.

### LayerXBridge — `0x0000000000000000000000000000000000001016`

This is an attested bridge to registered external chains, declared in
`precompiles/layerxbridge/LayerXBridge.sol`. It is not an execution-to-settlement
link inside Paxeer. Governance registers external chains and vaults, finality
depths, the attestor set and threshold, and per-asset caps. Genesis registers
no chain, so the bridge is dormant until governance configures one.

`bridgeIn(chain, vault, txHash, logIndex, recipient, asset, amount, signatures)`
mints a bridged denom against the required attestor signatures over an
external vault deposit. `bridgeOut(chain, asset, amount, recipient)` burns
the caller's bridged denom and emits `BridgeOut` with a nonce. Views include
`getChain`, `getAttestors`, `getCap`, `isPaused` and `isNullified`.
`modules/layerxbridge/ATTESTATION.md` defines the byte-exact signing contract.
External-chain evidence and finality requirements are specific to that
bridge; they do not introduce another Paxeer finality authority.

### Launchpad — `0x0000000000000000000000000000000000001017`

Launchpad is a native token-launch AMM on Paxeer. The current interface is
`precompiles/launchpad/Launchpad.sol` and `precompiles/launchpad/abi.json`,
implemented under `precompiles/launchpad/` and `modules/launchpad/`.
It exposes market creation, USDL-quoted pool trading, quotes and creator NFT
fee management. Use that interface for current argument types, permit
variants and fee strategies. A pool trade is a chain transaction, not an
instruction to move funds between execution networks.

## Receipts and commitment

`eth_getTransactionReceipt` reports EVM status and block metadata. The
optional `pax_sequencerReceipt` API in `rpc/sequencer_receipt.go` reports
signed execution progress:

| Rung | Meaning |
| --- | --- |
| `submitted` | A send record exists without a signed execution receipt. |
| `receipted` | A signed execution receipt exists; commitment has not been established. |
| `committed` | A committed block's `MsgSequenceBatch` covers the receipt's sequence. |

A committed transaction can still have failed execution. Check status and
expected effects as well as commitment. A receipt signature binds execution
fields under a pinned receipt key; it does not itself prove BFT commitment.

Independent verification authenticates the receipt key ranges, signed
transaction, execution header, transaction and sequence-batch inclusion
proofs, and validator commit with more than two thirds of voting power.
Execution results from height H are proven against `LastResultsHash` in the
committed successor header at H+1. Authenticate the chain identity and
validator history from a trusted source. This successor proof does not
introduce a second network or settlement stage.

[SDK terminal verification](SdkTerminalVerification.md) describes verifier
entry points and evidence. A node's `committed` label reports its view; an
independent client checks the evidence.

## Public node JSON-RPC and hosted services

Public HTTPS JSON-RPC uses the configured node's root path.
Require `eth_chainId` to return `"0x7d"`. See [Public RPC](PublicRpc.md).
The node namespace inventory is `debug`, `echo`, `eth`, `net`,
`pax`, `px`, `test` and `txpool`; method availability depends on configuration.

The hosted gateway under `services/platform/hosted/gateway/` is an
operator-deployed service over this network. Obtain its endpoint from the
operator. Its `openrpc.json` declares the `px_*` methods and parameter/result
shapes. Gateway operators configure EVM upstreams with
`LAYERX_GATEWAY_PAXEER_RPC_URLS`, a JSON array of two to eight distinct
endpoints, parsed by `src/paxeer.rs` in that service.

`px_resolveAccount` and `px_getAccount` resolve an EVM address or explicit
DID. Identity results carry `evm_address`, `pax_address`, `layerx_did`,
`layerx_account` and `bound`. Keep the original selector: a bare 64-hex
identifier is refused with `-32001` and
`data.code = "native_account_reverse_resolution_unsupported"`; it is not
reinterpreted as a DID.

`px_getBalances` and `px_listAssets` return the declared balance and asset
views; preserve their source and verification metadata. `px_getHistory` and
`px_getUnifiedHistory` provide indexed history with pagination and freshness
information. An index record does not replace a verified committed receipt.

`px_getNetwork` reports network identity, EVM chain ID and latest block. If
consensus status is unavailable it reports `consensus: null` and
`reason: "consensus_unavailable"`; retain that partial availability explicitly.
`px_getCapabilities` probes native precompile support at a recorded RPC
height. Use it when choosing a deployed workflow.

For service-managed payments and approvals, follow
[Human journeys](HumanJourneys.md) and the deployment's authentication and
allowance policy. Hosted services add identity, history and workflow APIs;
they do not constitute another execution network.

[Quickstart](Quickstart.md) ·
[Getting started](Getting-Started-Beta.md) ·
[SDK terminal verification](SdkTerminalVerification.md) ·
[Home](Home.md)
