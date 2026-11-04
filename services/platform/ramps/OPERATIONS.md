# Ramp operator and recovery runbook

Use this with the [toolkit overview](README.md),
[configuration example](config.example.json) and
[deployment template](deployment.yaml). The reference service has production
integration prerequisites; a valid configuration does not establish a working
provider or a complete off-ramp submission path.

## Enablement

Obtain actual identity, compliance, provider, KMS, native RPC and inventory
boundary coordinates from their operators. Provision dedicated least-privilege
identities and mount tokens and PKCS#12 passwords in separate read-only,
owner-only files (mode `0600`). Keep signing keys in non-exporting KMS/HSM
handles; do not put private keys in configuration, journals, API messages or
browser state. Display the external-custody disclosure in [README.md](README.md)
on every customer surface.

The deployment is in **`layerx-testnet`**. Its NetworkPolicy allows cluster DNS
and egress to same-namespace pods selected by `layerx-plane=trusted-boundary`,
`app=layerx-gateway`, or `app=paxeer` on the listed ports. A pod selector without
a namespace selector selects only that namespace. Names in the example under
`layerx-platform` or `layerx-ramps` therefore require deployment-specific
alignment; the shipped policy does not permit those namespaces automatically.
Terminate external provider traffic at a reachable mTLS boundary. Ingress admits
the ingress controller namespace and same-namespace `app=layerx-ramp-worker`
pods on port 8443; API authentication must still protect internal routes.

Set `layerx.rpc_endpoint` to the actual pinned native JSON-RPC endpoint at its
root, and set `chain_id` to 125. Paxeer is one BFT network; its kernel executes
inside the node and proposes blocks. The example's legacy service hostname is
a replacement coordinate, not another network component. Public node names are
documented in the [gateway guide](../hosted/gateway/README.md). Native RPC trust
uses `rpc_trust_anchor_der`; the signer and other service calls use the
configured outbound TLS identity and CA.

The transaction signer receives the **EIP-1559 signing hash**, 32 bytes encoded
as standard padded base64, at `POST /v1/signatures`. Its request includes the
configured `key_handle`, `algorithm: secp256k1` and `message`. It must return
`signature` as base64 compact 64-byte `(r,s)` and `recovery_id` as 0 or 1. The
client recovers the EVM address and checks `layerx.operator_address`. This is
distinct from Ed25519 signatures used for receipts, compliance and callbacks.
Pin `layerx.receipt_keys` with `keyId`, `publicKey`, `firstHeight` and `lastHeight`
and cover the relevant transaction heights. Do not derive trust from an
unverified receipt response.

Keep the quote catalogue operator-owned and immutable for its validity
interval. Amounts use integer minor units and rates use numerator/denominator.
Replace an expired quote rather than changing one already bound to an order.
For off-ramp, bind both `customer_budget_id` and `customer_budget_purpose` before
creating the order; the old input names are compatibility aliases.

## Worker procedure

The reconciler performs status and receipt reads for pending and unknown states;
it does not submit again. An authenticated worker sends a strict object to
`POST /internal/v1/work`: `order_digest` (32-byte array), `action`, and optional
`account_sequence`.

1. Use `compliance` to obtain a signed order-bound decision. A manual-review
   result is non-terminal.
2. Use `submit_provider` only when the durable workflow admits the external leg;
   `reconcile_provider` reads the existing provider operation.
3. For an on-ramp `submit_layerx`, **omit `account_sequence`**. The client reads
   the operator's pending EVM nonce, estimates gas and enforces configured gas
   and fee caps before signing the exact native ledger payment.
4. The HTTP handler requires `account_sequence` for off-ramp `submit_layerx`,
   but operator preparation refuses that path with `PayerGrantRequired`.
   Supplying a sequence cannot authorise a customer debit. The toolkit supports
   customer-signed `budgetSpend` through `submit_customer_payment`; the reference
   HTTP service does not expose that submission operation. Integrate and qualify
   it before enabling off-ramp orders.
5. Use `resolve_layerx` for the persisted transaction hash. Leave transport loss
   unknown and let reconciliation establish its outcome.

`account_sequence` on other work actions is rejected with 400 `work_invalid`;
on-ramp submission with it is also rejected. The operator bearer is distinct
from customer credentials. NetworkPolicy limits callers by pod/port and does
not distinguish public and internal paths on the same TLS listener.

## Refusals, unknowns and reversals

Never create fresh identifiers to retry `provider_submission_planned`,
`provider_submitted_unknown`, `layerx_submission_planned` or
`layerx_submitted_unknown`. Read provider status with the persisted operation
and order idempotency binding; read native evidence with
`pax_sequencerReceipt` using the stored transaction hash. Retain the exact signed
bytes. An explicit provider, compliance or native refusal is distinct from a
lost acknowledgement and requires a separately authorised new workflow to retry.

Callback IDs and sequence/fact bindings are persisted. Conflicting reuse is
rejected; out-of-order callbacks cannot move settled or reversed state backwards.
A payout reversal after payment changes status to `reversed`. Follow the
operator's disclosed reversal agreement. Any compensating transfer requires
new authorisation and must preserve the original journal and receipt.

The engine authenticates receipt signatures and logs against pinned keys and the
order. It keeps a `receipted` leg pending; only a `committed` leg can satisfy the
native payment condition for `done`, together with the required external
settlement. Block commit is network finality. No separate guarantor decision
or confirmation-depth threshold supplies it.

## Paxeer inventory

Inventory rebalancing is independent of customer orders. Use
`POST /internal/v1/rebalances` with the configured operator account, wallet,
vault and signing handle; poll/reconcile the same persisted operation and
transaction hash. The journal records intent before broadcast and retains
broadcast ambiguity.

Current inventory evidence records `submitted`, `receipted` and `committed`
rungs. A committed observation requires its block hash and height, bound to
the exact transaction. Recovery rejects rung regression, operation/transaction
mismatch, or a changed committed block identity. Keep inventory unavailable on
such a contradiction; do not credit from stale inclusion or bypass the journal
by treating a different block as equivalent. Legacy final-stage records have
explicit compatibility handling in `verify_custody_recovery`.

## Recovery

On restart, the service acquires a non-blocking OS-exclusive lock on the
owner-only writer-lock file and verifies journal sequence, predecessor hashes
and record digests before accepting work. The OS releases the advisory lock on
exit or process death. The file remains; never delete it to bypass a live writer.
Corruption, ambiguous truncation, insecure permissions, concurrent writers and
conflicting idempotency bindings prevent safe operation. Restore only from an
authenticated durable volume and preserve the original evidence.

Inspect `GET /internal/v1/journal` with the operator bearer. The protected
`POST /internal/v1/journal/recover` route coordinates recovery; it is not
permission to edit journal lines. Reconcile every planned, pending or unknown
operation before admitting new financial effects. Versioned source-settlement
records use `POST /internal/v2/source-settlements` and retain their additional
proof requirements; they do not turn the network into two settlement domains.

`/livez` reports process liveness. `/readyz` reports local configuration and
journal health; it does not attest to provider settlement, committed network
receipts, inventory finality or a qualified customer journey. Activation needs
real owner-provided sandbox contracts and recovery evidence for the routes
being enabled.
