# Market-maker ramp toolkit

This directory contains two members of the [platform Cargo workspace](../Cargo.toml):
`layerx-ramp-toolkit` ([toolkit/](toolkit/)), including `ramp-verify-receipt`,
and `layerx-reference-ramp` ([reference/](reference/)), the reference HTTP service.
They support an independent market maker using ordinary accounts on Paxeer.
They confer no minting power, protocol role or authority over network finality.
Every customer surface must display:

> External custody: this independent market maker controls the off-platform funds and payout.

Paxeer is one BFT network, with the kernel inside `paxd` executing transactions
and proposing blocks. EVM chain ID is **125** (`0x7d`). Block commit is finality;
there is no separate execution/settlement chain pair. External provider custody
and the operator's inventory rebalancing remain distinct business workflows.
A customer order cannot choose the operator's inventory account, wallet, vault
or signing handle. Tenant domains are database namespaces within the network.

## Order and money binding

[`RampOrder::bind`](toolkit/src/lib.rs) combines an authenticated identity-plane
customer, an operator-owned quote and the configured operator identity.
`POST /v1/orders` accepts a strict `CreateOrder` object:

| Field | Meaning |
| --- | --- |
| `order_id` | Customer's order identifier |
| `quote_id` | Identifier of an operator-provided quote |
| `customer_budget_id` | Off-ramp only: nonzero 32-byte customer budget ID |
| `customer_budget_purpose` | Off-ramp only: nonzero 32-byte purpose binding |

The two budget fields are JSON arrays of 32 byte values. Legacy input names
`payer_grant` and `payer_grant_purpose` are accepted aliases. An on-ramp rejects
both fields; an off-ramp requires both. Customer account identity comes from
remote token introspection, not the order JSON.

A domain-separated `OrderDigest` binds direction, quote and order IDs, customer
and operator accounts/principals, non-exporting signer handle, asset and exact
integer amount, external currency/minor units, rational rate, fees, slippage,
provider and payout tokens, expiry, budget ID/purpose and application context.
The on-ramp payment's reference is this digest. Preserve it throughout provider
idempotency and journal reconciliation. Quote expiry is Unix seconds; native
payment expiry uses checked milliseconds.

## Native payment path

The implementation is in [toolkit/src/clients.rs](toolkit/src/clients.rs),
[toolkit/src/evm.rs](toolkit/src/evm.rs) and [toolkit/src/engine.rs](toolkit/src/engine.rs).
Payments are signed EIP-1559 transactions, submitted by `eth_sendRawTransaction`
and reconciled by their exact transaction hash through `pax_sequencerReceipt`.
The legacy signed-activity envelopes and independent `AuthorizedBatch` fetch
are retired payment-path requirements.

- On-ramp: `payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)` at the
  ledger precompile `0x000000000000000000000000000000000000101a` transfers from
  the operator to the customer, binding asset, amount, expiry and order digest.
  The client reads the operator's pending EVM nonce, estimates gas, enforces fee
  and gas caps, and requests a secp256k1 signature from the configured KMS.
- Off-ramp: the maintained call is `budgetSpend(bytes32,bytes32,uint128)` at the
  pay precompile `0x000000000000000000000000000000000000101b`. The customer signs
  the exact transaction. `prepare_customer_payment` validates its sender, nonce,
  calldata, fee bounds and current budget state; the budget binds the customer,
  asset and purpose. The operator never signs a customer debit.

The toolkit's `submit_customer_payment` supports that off-ramp path. The current
reference HTTP worker only exposes `submit_layerx`, which calls operator
preparation and refuses off-ramp orders with `PayerGrantRequired`. Its
`account_sequence` field does not provide customer authorisation. A deployment
needs an authenticated integration to the toolkit's customer-signed submission
path before it can offer a complete off-ramp journey.

Signed execution receipts are checked with independently pinned Ed25519 receipt
keys and height ranges, bound to the exact transaction and authenticated logs.
On-ramp verification requires the matching ledger `PaymentExact` event;
off-ramp verification requires the matching pay `BudgetSpent` event and budget
state. These receipt keys are distinct from transaction-signing secp256k1 keys.
`submitted` is acknowledgement, `receipted` is signed execution, and `committed`
is block inclusion. The engine keeps receipted payments pending until commitment.

## Durable workflow

The append-only, hash-chained journal uses owner-only files, an exclusive writer
lock released by the OS, and fsync before changing in-memory state. Startup
validates the entire chain. Financial effects follow a durable
`*_submission_planned` transition. The journal retains worker leases, order and
provider idempotency bindings, callback IDs/sequences, exact raw transactions,
transaction hashes, receipt digests/heights/rungs and inventory operation IDs.
Reconcile planned and unknown operations under the persisted identifiers;
do not create another submission to resolve uncertainty.

Public status is `pending`, `unknown`, `refused`, `manual_review`, `reversed` or
`done`. For the current transaction path, `done` requires committed verified
payment evidence and the required external settlement. An off-ramp also needs
a settled payout. A later provider reversal removes `done`; no provider or
network failure is converted into success. Versioned source-settlement and
reversal records have additional evidence requirements in
[toolkit/src/migration.rs](toolkit/src/migration.rs).

## Production boundaries

Configure [config.example.json](config.example.json) for the actual deployment;
it contains replacement values and legacy service names, not proven production
coordinates. The native client uses `layerx.rpc_endpoint`,
`rpc_trust_anchor_der`, `chain_id`, `operator_address`, `receipt_keys`,
`max_fee_per_gas`, `max_priority_fee_per_gas`, `gas_limit_cap`, `signer_endpoint`,
`signer_token_file` and `actor_did`.

Service boundaries use pinned TLS and mTLS where configured; secret credentials
are read from protected files. Only non-exporting KMS/HSM handles and public
verification material belong in durable application state. Contracts include:

- `layerx-ramp-compliance-v1`: signed, expiring decisions bound to the order,
  customer and operator.
- `layerx-ramp-provider-v1`: idempotent settlement/status and signed callbacks
  bound to the customer, beneficiary, amount and currency.
- KMS `POST /v1/signatures`: `key_handle`, `algorithm: secp256k1`, and the standard
  padded base64 EIP-1559 signing hash in `message`. The response contains a
  base64 compact 64-byte `signature` and `recovery_id` (0 or 1); the client
  recovers the signer and checks `operator_address`.
- `layerx-ramp-paxeer-v1`: operator-only inventory broadcast/status with the
  configured account, wallet, vault and signer handle.

See [OPERATIONS.md](OPERATIONS.md) for enablement, worker inputs and recovery,
and the [gateway](../hosted/gateway/README.md) for root-path public RPC names.
The [receipt authority](../hosted/authority/README.md) describes a separate
hosted verifier; it is not a required batch lookup for the native ramp client.

## Run

Prepare the protected configuration and secret mounts, then from repository root:

```sh
cargo run --locked --manifest-path services/platform/Cargo.toml -p layerx-reference-ramp -- /run/layerx-ramp/config.json
```

The service's TLS listener exposes `POST /v1/orders`,
`GET /v1/orders/{order_digest}`, `POST /v1/provider-callbacks`, `/livez` and
`/readyz`. Internal bearer-protected routes include `POST /internal/v1/work`,
`POST /internal/v1/rebalances`, `GET /internal/v1/rebalances/{idempotency_key}`,
`GET /internal/v1/journal`, `POST /internal/v1/journal/recover` and
`POST /internal/v2/source-settlements`. Keep internal routes off public ingress.
The supplied [deployment.yaml](deployment.yaml) and [runbook](OPERATIONS.md)
define the network and filesystem boundaries to review before activation.
