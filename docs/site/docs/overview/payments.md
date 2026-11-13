# Payments developer path

Payments execute on the merged Paxeer network, EVM chain id `125` (`0x7d`).
The kernel is part of the network and proposes blocks. A signed execution
receipt can arrive before commit; release paid resources only after the
required committed evidence and payment facts verify. See
[Activities](../concepts/activities.md) for admission and receipt rungs and
[Monorepo layout](monorepo.md) for the implementation boundaries.

## 1. Configure trusted inputs

Set `RPC_URL` to the operator-provided HTTPS node root:

```sh
: "${RPC_URL:?}"
export RPC_URL
curl --fail-with-body --silent --show-error "$RPC_URL" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected chain id is `0x7d`; see
[Public JSON-RPC](../reference/public-rpc.md). A successful HTTP response
can still contain a JSON-RPC error: inspect `error` and the request `id`.

Obtain these inputs independently of the response you intend to verify:

- the sender's signing account and its on-chain DID binding;
- the payment Asset, source account, recipient account, integer base-unit
  amount, expiry, and unique payment reference;
- a gas budget and fee ceiling, plus enough balance to pay fees;
- authorized receipt key ids, public keys and height ranges;
- trusted consensus configuration when independently proving block commitment.

The node serves the `px` namespace at the root. `px_register` and
`px_requestFunds` are hosted operations declared in
`services/platform/hosted/gateway/openrpc.json`; obtain the gateway's
operator-provided connection and credential settings for them. The legacy `lx_` activity
submission methods are retired and gateway dispatch returns `-32601`.

## 2. Create or import a signing key

Use a wallet that signs EIP-1559 transactions for chain `125`. A standalone
Ed25519 seed does not sign an EVM transaction. The CLI's
`layerx wallet derive` derives an EVM address and LayerX DID from a BIP-39
phrase read from standard input or `--mnemonic-file`; `--bind` with `--rpc`
submits the on-chain binding. Its implementation is
`tools/platform/cli/src/wallet_derive.rs` and the typed binding client is
`services/agent/crates/layerx-client/src/account_binding.rs`.

Local Ed25519 key management remains available:

```sh
layerx key create payment-key
layerx key default payment-key
layerx wallet list
```

These commands require the built CLI on `PATH`; its manifest is
`tools/platform/cli/Cargo.toml`. The OS keyring is the default credential
store. Headless hosts must configure the encrypted file store described in
`tools/platform/cli/README.md`. `wallet import` reads a 32-byte hexadecimal
Ed25519 seed from standard input; it does not register or fund an account.
`wallet create` registers only in the emulator and refuses public registration
with `wallet_registration_unavailable`.

For hosted signing, the internal KMS supports secp256k1 digest signing as
well as Ed25519. Its handles do not export private keys. See
[Hosted internal](../operators/hosted-internal.md#kms-request-and-response)
for the public-key, signature and recovery-id formats.

## 3. Fund the account through the ledger

Gas balance and a per-Asset ledger balance are separate checks. Read the
sender's EVM balance with `eth_getBalance`; for an Asset account use
`balanceOf(bytes32,bytes32)` on the ledger precompile. Have the recipient
open its per-Asset account before sending or minting to it.

Funding availability is deployment-specific. The tree includes a hosted
faucet and `px_requestFunds`; neither establishes a universally available
mainnet faucet. The faucet requires a configured
`LAYERX_TESTNET_FUNDING_URL`, `LAYERX_TESTNET_ADMIN_TOKEN_FILE`, identity
introspection, and Redis credentials
(`services/platform/hosted/faucet/src/main.rs`). Use the operator's current
funding instructions for the selected network. A funding request or pending
response is not a balance credit; reconcile the resulting committed state.
The pre-merge local custody-credit procedure is retired.

## 4. Create and use an Asset

The ledger entry point is
`0x000000000000000000000000000000000000101A`. Its canonical signatures are
in `precompiles/layerxledger/abi.json`:

| Operation | ABI method | Required preparation |
| --- | --- | --- |
| Register | `register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)` | Correct derived Asset id, salt, metadata, supply cap and issuance/custody fields |
| Open account | `openAccount(bytes32)` | Select the recipient's bound wallet to open its account |
| Mint | `mint(bytes32,bytes32,uint128)` | Issuer authority and an existing destination account |
| Send | `send(bytes)` | Canonical debit payload, including source sequence and authorization |
| Exact payment | `payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)` | Authorized source, recipient, Asset, amount, Unix expiry in milliseconds, payment reference |
| Burn | `burn(bytes32,bytes32,uint128)` | Source account and authority |

Each write is a signed transaction to that precompile with zero EVM `value`.
The account identifiers are 32-byte ledger ids, not 20-byte EVM addresses.
`accountId(bytes32,bytes32)` and `accountOwner(bytes32)` are ledger views.
An Asset id is derived as
`SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)`; do not substitute a
symbol or EVM contract address.

For an exact payment, set `FROM_ACCOUNT`, `TO_ACCOUNT`, `ASSET_ID`, and
`PAYMENT_REFERENCE` to `0x`-prefixed bytes32 values; set `AMOUNT` to a positive
integer in base units and `EXPIRES_AT_MS` to the agreed future Unix expiry.
With Foundry's `cast` installed, encode the actual precompile call:

```sh
PAYMENT_CALLDATA=$(cast calldata \
  'payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)' \
  "$FROM_ACCOUNT" "$TO_ACCOUNT" "$ASSET_ID" "$AMOUNT" \
  "$EXPIRES_AT_MS" "$PAYMENT_REFERENCE")
```

`precompiles/layerxledger/payments.go` passes that call to
`modules/layerxledger/keeper/payexact.go`. The keeper checks expiry against
block time, requires bound source authority, and obtains the source account's
next sequence. On success the precompile emits `PaymentExact`. Confirm its
emitter and decoded reference, source, recipient, Asset and amount; the event
name alone does not identify the payment.

Use the sender's `eth_getTransactionCount` at `pending`, `eth_gasPrice`, and
`eth_estimateGas` for the transaction. The unsigned type-2 transaction must
bind `chainId: "0x7d"`, that nonce, destination `0x...101A`, the encoded
`data`, `value: "0x0"`, `gas`, `maxFeePerGas`, `maxPriorityFeePerGas`, and an
access list. Review all fields before signing. Reject a quote that exceeds
the fee ceiling; an estimate neither reserves fees nor guarantees execution.

After your wallet signs, set `SIGNED_TX_HEX` to the complete `0x02`-prefixed
raw transaction. Submit those bytes, retaining the locally computed hash:

```sh
jq -n --arg raw "$SIGNED_TX_HEX" \
  '{jsonrpc:"2.0",id:2,method:"eth_sendRawTransaction",params:[$raw]}' |
  curl --fail-with-body --silent --show-error "$RPC_URL" \
    -H 'Content-Type: application/json' --data-binary @-
```

The returned hash must match the signed transaction's hash. Retain the
payment reference, nonce, raw bytes and hash before retrying. A timeout does
not prove refusal: recover the original transaction rather than signing a
replacement debit with a new reference.

## 5. Inspect Assets, balances, fees and receipts

Use ledger `getAsset`, `balanceOf` and `totalUnits` views through `eth_call`
for native Asset state. For the hosted gateway, `px_getBalances` returns an
`account`, snapshot `height`, and `balances` containing `denom` and decimal
`amount`. `px_listAssets` returns `height`, `assets`, `joined_limit`, `next_cursor`
and `order: "registration"`. It accepts an optional decimal registration-row
cursor and a limit from 1 to 16, defaulting to 16.
Those are the shapes implemented by
`services/platform/hosted/gateway/src/paxeer.rs`; neither a snapshot label
nor an Asset listing establishes payment commitment.

Set `TX_HASH` to the retained `0x`-prefixed transaction hash and query the
configured node receipt source:

```sh
jq -n --arg hash "$TX_HASH" \
  '{jsonrpc:"2.0",id:3,method:"pax_sequencerReceipt",params:[$hash]}' |
  curl --fail-with-body --silent --show-error "$RPC_URL" \
    -H 'Content-Type: application/json' --data-binary @-

jq -n --arg hash "$TX_HASH" \
  '{jsonrpc:"2.0",id:4,method:"eth_getTransactionReceipt",params:[$hash]}' |
  curl --fail-with-body --silent --show-error "$RPC_URL" \
    -H 'Content-Type: application/json' --data-binary @-
```

The possible receipt rungs are `submitted`, `receipted`, and `committed`;
[Activities](../concepts/activities.md#status) defines their actual response
fields. Null results and method-unavailable errors provide no settlement
proof. Verify the signed canonical receipt under independently authorized
keys, including the transaction hash, nonce, result and log digests. Then
bind the committed block number/hash and EVM transaction receipt to the same
execution. `status: "0x0"` is failure even in a committed block.

Live notifications and webhook deliveries are unverified wake-ups. Reconcile
with verified reads after each wake-up or stream closure; never create a new
payment solely because delivery was interrupted.

## 6. Build and deploy the LXT20 example

The token example is `programs/sdk/rust/examples/token-lxt20`, a Programs
ABI-v2 token backed by one native Asset. It does not create a second ledger
or mint native units. Build the token itself:

```sh
(cd programs/sdk/rust/examples/token-lxt20 && \
  cargo build --locked --target wasm32-unknown-unknown --release)
```

Its release manifest uses `opt-level = "z"`, LTO and aborting panics. The
merchant example is a different package at
`programs/sdk/rust/examples/payments-merchant`.

Deploy and call through Programs at
`0x000000000000000000000000000000000000101D`, using
`precompiles/layerxprograms/LayerXPrograms.sol` and its payload encoders.
A registry publication or source upload alone is not a deployment receipt.
Register the derived program account for the selected Asset before funding
it; the guest SDK exposes `PreparedProgramAccount::registration_payload`.
Include the matching ABI-v2 interface on deployment and verify the committed
transaction and program outcome before using guest results.

The reference exports `initialize`, `transfer`, `approve`, `transfer_from`,
`balance_of`, `allowance`, `total_supply`, and `metadata`. Recipients call
`approve`, including approval of zero, to register their derived storage
before receiving. Initialization stages the fixed supply from the issuer to
the registered program account atomically. There is no LXT20 mint, burn,
permit, or nested-call surface. See [Programs](../programs/index.md) for
account and call encoding details.

## 7. Pay an HTTP 402 endpoint

The x402 v2 HTTP exchange uses these headers:

1. The seller returns HTTP 402 with `PAYMENT-REQUIRED`.
2. The buyer chooses an offered requirement without modifying it.
3. The buyer returns the corresponding evidence in `PAYMENT-SIGNATURE`.
4. The seller verifies the evidence and settlement before releasing the
   resource, and returns `PAYMENT-RESPONSE`.

For the supported exact-payment path, bind the offer to the `payExact`
transaction above and use that payment reference as the scheme payload
`idempotencyKey`. The current x402 seller checks the ledger payment log
against the offered network, payer/recipient, Asset, amount and payment
reference (`services/interop/crates/layerx-x402/src/seller.rs`). Pin receipt
keys; committed proof verification additionally needs independently trusted
consensus information. Do not adopt a key or validator set merely because it
arrived in the payment response.

The hosted merchant `POST /v1/settle` consumes the middleware's settlement
request and verifies its canonical receipt and idempotency binding. It queries
`pax_sequencerReceipt` and `eth_getTransactionReceipt`, requires matching
committed evidence and a successful result, and returns HTTP 202 while
inclusion is pending. Receipt or inclusion mismatches are refusals, not
permission to release the resource. This path is implemented in
`services/platform/hosted/gateway/src/main.rs` and
`services/platform/hosted/gateway/src/settlement.rs`.

The hosted middleware retains the `sequencer-signed` verification label for
execution evidence; that label alone is not a committed settlement claim. The
Rust x402 seller returns the `0x`-prefixed transaction hash in `transaction`,
with receipt bytes, digest and rung in `extensions.layerx`; a committed result
adds block details and its proof. Preserve the request digest, idempotency
binding and original transaction when recovering a pending payment.

The old payer-grant, metered draw and subscription recipes are retired.
Although some ABIs and transport schemas retain those names, ledger
`IssueGrant`, `RevokeGrant`, and `Draw` refuse with `ErrNoPayerGrant`.
Do not construct a new payment integration from their old ordinal encodings.
See [x402 transport](../interop/x402.md) for header schemas.

## CLI and MCP boundaries

The CLI declares account-open, Asset info/list, burn, fee estimate, receipt
and watch commands in `tools/platform/cli/src/wallet.rs`. Node transactions
use root-path RPC; hosted source-account queries take `--gateway-rpc`.
Writes require an EVM key export, fee limit and independently pinned receipt
policy. `--wait committed` checks signed receipt evidence and the matching
successful EVM receipt under the authenticated node trust boundary.
`executed`, `batched` and `finalised` remain aliases for `receipted`,
`committed` and `committed`, respectively. See [CLI](../platform/cli.md).

The MCP base catalogue has 22 entries in
`services/agent/crates/layerx-mcp/src/server.rs`, including `tenant.readiness`,
`activity.wait`, and `faucet.request`. Actual tools are filtered by the bound
session's scopes and deployment mode. The presence of `grant.issue` or
`grant.draw` in that catalogue does not restore the retired ledger routes.
Tool acknowledgement or a `submitted` result still requires receipt recovery
and commitment verification before treating a payment as settled.

[Home](../index.md)
