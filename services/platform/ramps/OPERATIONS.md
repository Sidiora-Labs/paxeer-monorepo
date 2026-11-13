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

The Kubernetes deployment uses **`layerx-testnet`** for the ramp, identity,
merged-chain TLS relay and private owner boundaries. Identity, compliance,
provider egress, KMS and custody Services listen on 9443 and select their named
`app` pods on 9443. The merged node endpoint uses the root path of the gateway Service: port 443
forwards to gateway pod 9443. The gateway relays root JSON-RPC to the embedded-kernel
`paxd` Service `layerx-native-chain` on 8545; it is not another chain.
The ramp policy pairs an explicit `namespaceSelector` with each `podSelector`
and permits TCP 9443 to these six peers plus cluster DNS. The upstream ingress
policy admits only the ramp namespace/app pair on 9443. There is no public
provider egress; the owner-operated provider-egress boundary owns that hop.
The four owner boundary Services in the template require matching owner pods
and TLS listeners; obtain and qualify those deployments before activation.

Ingress admits the ingress controller namespace and same-namespace
`app=layerx-ramp-worker` pods on 8443. API authentication must still protect
internal routes. Configure the gateway-to-node relay and the owner boundary Services
for the routes being enabled.

Set `layerx.rpc_endpoint` to that pinned merged root JSON-RPC endpoint and
`chain_id` to 125. Project the DER CA for its TLS certificate as
`paxeer-rpc-ca.der` in Secret `layerx-reference-ramp-layerx`; the init container
copies it to the private `/run/secrets/paxeer-rpc-ca.der` mount named by
`rpc_trust_anchor_der`. The certificate must cover the gateway service DNS
name. Signer and other service calls use the outbound TLS identity and CA.
The Railway environment example is a separate deployment profile: its
`RAMP_SEQUENCER_RPC_ENDPOINT` compatibility variable must name an owner-supplied
private merged root TLS relay and `RAMP_RPC_TRUST_ANCHOR_DER` supplies the same
DER pin file. Kubernetes service DNS does not resolve automatically on Railway.

The transaction signer receives the **EIP-1559 signing hash**, 32 bytes encoded
as standard padded base64, at `POST /v1/signatures`. Its request includes the
configured `key_handle`, `algorithm: secp256k1` and `message`. It must return
`signature` as base64 compact 64-byte `(r,s)` and `recovery_id` as 0 or 1. The
client recovers the EVM address and checks `layerx.operator_address`. Obtain
that address through an independently authenticated KMS public-key ceremony;
never copy an address claimed only by a signer response. This is
distinct from Ed25519 signatures used for receipts, compliance and callbacks.
Pin `layerx.receipt_keys` with `keyId`, `publicKey`, `firstHeight` and `lastHeight`
and cover the relevant transaction heights. During receipt-key rotation, verify
the new key and activation height independently, install its pin before use,
and retain old pins for historical recovery; do not extend a retired key
beyond its authorised height interval. Do not derive trust from an
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
3. For an on-ramp `submit_layerx`, **omit `account_sequence` or send JSON `null`**. The client reads
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

Current request bodies (replace the digest with the bound order's 32-byte array):

```json
{"order_digest":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"action":"submit_layerx","account_sequence":null}
```

```json
{"order_digest":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"action":"resolve_layerx"}
```

For off-ramp order creation use `customer_budget_id` and
`customer_budget_purpose` as shown in the README. The customer must sign a
chain-id-125 `budgetSpend` transaction bound to that budget, purpose, asset,
amount and operator recipient. Call `RampEngine::submit_customer_payment` with
the exact signed bytes through an authenticated customer integration. There is
currently no reference HTTP request that submits these bytes: keep off-ramp
enablement closed until that integration is implemented and qualified. A
legacy grant identifier or receiver sequence cannot supply this authority.

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
