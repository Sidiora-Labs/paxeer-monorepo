# Payments developer path

This guide covers a wallet, account funding, an Asset payment, token issuance,
a Programs guest, and HTTP 402 payment evidence on the merged Paxeer network.
The network runs `paxd`, including its kernel and block proposal path, with
EVM chain ID `125` (`0x7d`). A successful payment is final when its block is
committed. See [Protocol](Protocol.md) for execution and receipt semantics.

Start with [Getting started](Getting-Started-Beta.md). Related references are
[Assets](Assets.md), [Public JSON-RPC](PublicRpc.md),
[Programs](Programs.md), and [SDK quickstarts](SdkQuickstarts.md).

## 1. Configure trusted inputs

Use the public node JSON-RPC at the root path:

```sh
export RPC_URL=https://api-mainnet-beta.paxeer.network

curl --fail-with-body --silent --show-error "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected result is `"0x7d"`. Other public node names are
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, also at the root path. Check chain ID
before signing. Use integer base units for amounts and fee limits.

The hosted gateway is an operator deployment with its own contract in
[`openrpc.json`](../../services/platform/hosted/gateway/openrpc.json).
Its account helpers and capability reads are distinct from the public node
API. A hosted client must obtain the endpoint and credentials from its
operator and consult `px_getCapabilities` before using a gated surface;
unsupported precompile submissions return `-32003 surface_unavailable`.
This guide submits signed EVM transactions directly to a node.

For receipt verification, configure trusted signing keys with their valid
height ranges independently of the receipt being checked. For committed
proofs, also configure the trusted consensus chain and validator information.
A key returned beside an untrusted receipt does not establish its authority.

## 2. Create or import a signing key

Use a local EVM signer for type-2 (EIP-1559) transactions. Keep the private
key in the wallet or signer. Ledger operations also require the EVM address
to be bound to an Ed25519 DID key through the address precompile at
`0x0000000000000000000000000000000000001004`.

The binding sequence is:

1. Read `layerXBindNonce(address)` from the address precompile with `eth_call`.
2. Sign the following exact bytes with the Ed25519 DID key:
   `"LX:PAXEER-BIND:v1" || chainId:uint256be || evmAddress:20 || nonce:uint64be`.
3. Submit `bindLayerX(bytes32 didPublicKey,bytes signature)` from that EVM
   address, using the 64-byte Ed25519 signature. The EVM transaction supplies
   the address owner's consent; the inner signature supplies the DID's consent.
4. After commit, read `getLayerXDid(address)` and `getUnifiedAccount(address)`
   to confirm the binding and main account identifier.

The DID is `did:layerx:<64 lowercase hex public-key characters>`. Each address
and DID can have only one binding. The binding nonce is independent of the
EVM transaction nonce. Exact message construction is in
[`LayerXBindMessage`](../../modules/evm/types/layerx_binding.go), and the
served ABI is [`precompiles/addr/abi.json`](../../precompiles/addr/abi.json).

## 3. Fund the account

Arrange a transfer of the native gas currency to the EVM address from a funded
account. Read `eth_getBalance(address,"latest")` to confirm the gas balance.
For a registered Asset, open the recipient's per-Asset account and fund it
with an authorized ledger transfer, such as `payExact` below, or an issuer's
`mint` operation. Confirm the Asset balance through `balanceOf` after commit.
A gas balance and a per-Asset ledger balance are different reads.

The old custody-deposit-to-credit funding flow is retired. Do not use the
old `0x1013` deposit, custody-credit attestation script, or credit signer as
account funding instructions. The current ledger precompile is
`0x000000000000000000000000000000000000101a`; its
[ABI](../../precompiles/layerxledger/abi.json) is the source for the calls below.

## 4. Create and use an Asset

All methods in this section target the ledger precompile. Encode calldata
with its ABI, submit writes as locally signed type-2 transactions for chain
`125`, and send zero EVM `value`: these methods are nonpayable.

| Operation | ABI method | Required preparation |
| --- | --- | --- |
| Register an Asset | `register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)` | Issuer binding, derived Asset ID, salt, metadata, supply cap, issuer kind and custody reference |
| Open an account | `openAccount(bytes32 assetId)` | Run from the bound owner of that account |
| Mint units | `mint(bytes32 assetId,bytes32 accountId,uint128 amount)` | Authorized issuer and opened recipient account |
| Pay | `payExact(bytes32 from,bytes32 to,bytes32 assetId,uint128 amount,uint64 expiresAtMs,bytes32 paymentReference)` | Funded source owned by the bound caller, opened destination, future expiry and payment reference |
| Burn units | `burn(bytes32 assetId,bytes32 accountId,uint128 amount)` | Authority required by the ledger's burn rules |

The Asset identifier is
`SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)`.
A DID id32 is a derived identity identifier, not the raw public key. Derive
account identifiers with `accountId(bytes32 owner,bytes32 assetId)` and read
`accountOwner(bytes32 accountId)` to check ownership. Derivation alone does
not open or fund an account. Use the `AccountOpened` event and a subsequent
read to confirm registration.

For a payment, `payExact` checks the binding and source ownership, reads the
source sequence, constructs the native SEND payload and applies its ledger
effects through the kernel. `paymentReference` is the native idempotency key;
retain it with the signed transaction and reuse the recorded outcome when
recovering the same payment. `expiresAtMs` must exceed the executing block's
time in milliseconds. The ledger emits `PaymentExact` and `Transfer` logs.

To submit any prepared call:

1. Read `eth_getTransactionCount(sender,"pending")` for the EVM nonce.
2. Set `to` to the ledger address, ABI-encode the method and arguments as
   `data`, and set `value` to zero. Estimate gas with `eth_estimateGas` using
   the actual `from`, `to`, `value`, and `data`.
3. Choose EIP-1559 fee caps within the wallet's approved budget and sign
   locally with type `2`, chain ID `125`, nonce, gas limit and those fee caps.
4. Retain the raw signed bytes and their transaction hash before submitting.

With `RAW_TX` set to the `0x`-prefixed signed transaction bytes:

```sh
jq -cn --arg raw "$RAW_TX" \
  '{jsonrpc:"2.0",id:2,method:"eth_sendRawTransaction",params:[$raw]}' |
  curl --fail-with-body --silent --show-error "$RPC_URL" \
    --header 'Content-Type: application/json' --data-binary @-
```

Submission returns a transaction hash, not a payment success. After a timeout,
recover that hash and receipt before authorizing another payment. Resubmitting
the retained signed bytes preserves the transaction identity; creating a new
nonce and payment reference creates a new payment attempt.

## 5. Inspect Assets, balances, fees, and receipts

Use `eth_call` with ledger ABI calldata for `getAsset(assetId)`,
`balanceOf(accountId,assetId)`, `totalUnits(assetId)`, and
`nextSequence(accountId,assetId)`. These are ledger reads; an ERC-20
`balanceOf(address)` call has a different signature. Read EVM gas funds with
`eth_getBalance`. The ledger ABI has no Asset enumeration method; a hosted
`px_listAssets` read has the separate gateway contract described above.

`eth_estimateGas` estimates execution gas; `eth_gasPrice` is a price read.
Neither reserves funds or guarantees successful execution. An included
transaction can still revert and pay gas. Ledger refusals include
`LXP_UNAUTHORIZED_DEBIT`, `LXP_EXPIRED`, `LXP_INSUFFICIENT_BALANCE`,
`LXP_ASSET_PAUSED`, and `LXP_SEQUENCE_MISMATCH`; decode the revert rather
than accepting a transaction hash as proof of transfer.

With `TX_HASH` set to the submitted hash:

```sh
jq -cn --arg hash "$TX_HASH" \
  '{jsonrpc:"2.0",id:3,method:"pax_sequencerReceipt",params:[$hash]}' |
  curl --fail-with-body --silent --show-error "$RPC_URL" \
    --header 'Content-Type: application/json' --data-binary @-
```

The node receipt reports `submitted`, `receipted`, or `committed`.
`receipted` supplies a signed execution result; `committed` also supplies
`blockNumber` and `blockHash`. Require successful execution (`status` is
`0x1`), the expected payment log, and committed block evidence before treating
the payment as final. `eth_getTransactionReceipt` is the EVM receipt read;
a null response is not a confirmed payment. Notifications are wake-ups for
receipt reconciliation, not payment evidence.

See [Protocol](Protocol.md#from-transaction-to-committed-receipt) for the
verification boundary and [Portable receipt verifier](PortableVerifier.md)
for proof tooling. The current receipt implementation is
[`rpc/sequencer_receipt.go`](../../rpc/sequencer_receipt.go).

## 6. Build and deploy the LXT20 example

The LXT20 example is a Programs ABI-v2 guest backed by a registered native
Asset. It moves real backing units between program-derived accounts and
maintains token storage atomically with those transfers. Its eight exports
are `initialize`, `transfer`, `approve`, `transfer_from`, `balance_of`,
`allowance`, `total_supply`, and `metadata`.

Build from each example directory so `programs/.cargo/config.toml` applies:

```sh
(cd programs/sdk/rust/examples/token-lxt20 && \
  cargo build --locked --release --target wasm32-unknown-unknown)
(cd programs/sdk/rust/examples/payments-merchant && \
  cargo build --locked --release --target wasm32-unknown-unknown)
```

These are build instructions for the examples, not deployment commands.
Configure the reference issuer, backing Asset and fixed supply before making
a deployment, and regenerate the matching interface. Include the ABI-v2
interface with the guest in the deployment. The merged Programs precompile
is `0x000000000000000000000000000000000000101d`.

Register each Asset-bound program account using
`PreparedProgramAccount::registration_payload` before funding it. LXT20
recipients must also call `approve`, including approval of zero, to register
their derived account in the guest's storage. Initialization transfers the
fixed supply from the issuer into its registered program account; it does
not mint backing units. Calls require the exact funding or ProgramSpend
authority. There is no LXT20 mint, burn, permit, or nested-call surface.

The merchant guest stages one principal-funded deposit and two payouts as
one atomic transfer set. See the
[LXT20 example](../../programs/sdk/rust/examples/token-lxt20/README.md),
[merchant example](../../programs/sdk/rust/examples/payments-merchant/README.md),
and [Programs](Programs.md) for calldata, account registration, capabilities,
and deployment details. Native examples and fixtures are development
references; they do not establish a second live transaction endpoint.

## 7. Pay an HTTP 402 endpoint

The x402 v2 transport uses this exchange:

1. The seller returns HTTP 402 with `PAYMENT-REQUIRED`.
2. The buyer selects an offered requirement unchanged, including network
   `eip155:125`, Asset, amount, destination and required commitment.
3. For an `exact` offer, the buyer makes the ledger payment and returns
   `PAYMENT-SIGNATURE` containing its receipt evidence and accepted requirement.
4. The seller verifies the payment before releasing the resource and returns
   `PAYMENT-RESPONSE`.

The seller middleware's `layerx-receipt-proof-v2` evidence has
`verificationLevel` (`receipted` or `committed`), `rawTransaction`,
`signedReceipt`, `receiptDigest`, and `receiptKey` with `keyId`, `publicKey`,
`firstHeight`, and `lastHeight`. It binds the transaction hash, signer,
network, successful receipt and `PaymentExact` log to the offered payment.
The receipt key must match locally trusted keys. A `committed` offer also
requires consensus and transaction-inclusion evidence checked against trusted
validator information; changing the verification label is insufficient.

The successful response's `transaction` is the EVM transaction hash
(`0x` plus 64 hex characters). The old Merkle-leaf-derived `lxp:` transaction
identifier and protocol-v1 payment proof recipe are retired. Pending payment
returns HTTP 202 without releasing the resource; retain the original payment
identity for recovery.

The old payer-grant issue, revoke, read, and draw paths are retired in the
ledger keeper and return `LXP_NO_PAYER_GRANT` (other validation or pause
checks can refuse earlier). Do not use the former ordinal-7 grant and
ordinal-6 receive recipe to fund metered or subscription payments. Middleware
codecs retaining those schemes do not establish a live ledger grant route.
See [x402 transport](X402Transport.md) and the
[seller implementation](../../services/platform/middleware/seller/src/index.ts)
for the transport contract.

## CLI and MCP boundaries

CLI, SDK, and MCP integrations are clients of network authority. A tool
catalogue or emulator command does not prove a public deployment supports
that operation. Check the served ABI and, for hosted calls, the gateway
capabilities before choosing a signing or submission path. Keep approval
bound to the destination, Asset, amount, expiry, payment reference and fee
budget; recover uncertain outcomes from retained transaction evidence.

[Protocol](Protocol.md) · [Home](Home.md)
