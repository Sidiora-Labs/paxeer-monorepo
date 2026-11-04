# Human custody and verification

Human supports managed signing and owner self-custody on Paxeer, one network
with EVM chain ID **125** (`0x7d`). The kernel runs inside the network;
committed blocks provide finality. Money does not move between a separate
execution ledger and a settlement chain.

The browser contract is `services/human/schema/human-api/`, version **1.10**.
Implementation paths below are relative to
`services/human/crates/layerx-human-service/src/`. See
[Human journeys](HumanJourneys.md) for operations and
[Hosted internal](HostedInternal.md) for event delivery and webhook signing.
Human's custody provider and the hosted webhook KMS have different contracts;
non-exportable webhook keys do not define the Human owner policy.

---

## Application and protocol identities

The passkey authenticates the application session. The protocol DID and its
managed primary key are separate from that login credential. While custody
holds the key, the browser requests authorized operations rather than receiving
that secret. Human and managed-agent primary keys have distinct key classes
and principal-bound provider references (`custody/mod.rs`,
`custody/provider.rs`).

An EVM wallet signs transactions on the same network. The former
linked-payout-wallet workflow is retired; retained schema declarations are
compatibility history. Managed money movement uses bank sends at `0x1001`;
onboarding's EVM identity binding uses `bindLayerX` at `0x1004` through gas
station sponsorship (`services/human/schema/human-api/movement.kvx`,
`services/human/schema/human-api/identity.kvx`).

## What managed signing authorizes

`custody/signer.rs` binds the authenticated principal, held key, operation,
canonical bytes, structured disclosure, trace and time. The disclosure must
describe the exact bytes to be signed. Rate limits, fresh operation-bound
step-up where required, replay checks and durable signing decisions apply at
this boundary.

EVM signing uses the exact EIP-1559 transaction: chain ID, sender nonce read
at the pending tag, destination, value, calldata, gas limit and fee bounds.
The returned 65-byte recoverable secp256k1 signature must recover the expected
sender before transaction assembly (`custody/evm.rs`, `custody/signer.rs`).
Native consent has a separate typed Ed25519 signing path
(`custody/provider.rs`).

Production uses the authenticated remote provider contract, with opaque key
references, finite frames and deadlines. The file-envelope provider is marked
`DevelopmentOnly`; its presence is not evidence of a hardware KMS deployment
(`custody/provider.rs`, `custody/mod.rs`).

## Owner export and handover

`identity.kvx` declares self-custody as the first offered posture and custodial
holding as explicit opt-in. It also declares these owner operations:

| Operation | Request | Result |
| --- | --- | --- |
| `POST /v1/security/key-export/begin` | `step_up` | `export_id`, service-derived `confirms`, `expires_at` |
| `POST /v1/security/key-export/finish` | `export_id`, fresh `step_up` confirming that digest | `public_key`, `secret` (`TimedSecret`), `self_custodied_at` |

The consequence must be shown before confirmation: after handover the owner
holds the primary key and this service stops signing for that identity. The
client uses the service's operation digest. The legacy `KeyExportCeremony`
checks the principal, export identifier, digest and evidence validity, records
a security audit event, and returns a timed secret with `value`, `remask_at`
and `copyable` (`security.rs`). Repeat plaintext exports and later managed
signing are refused after the self-custody marker.

There are two implementation boundaries to distinguish:

- The HTTP dispatcher in `server/production_components.rs` still calls that
  legacy `KeyExportCeremony`. Its `Keystore::export_primary_once` explicitly
  refuses a `Production` provider (`custody/mod.rs`). The declared timed-secret
  response therefore does not establish a working production plaintext export.
- The provider handover uses `export_primary_envelope` and
  `acknowledge_primary_export` (`custody/export.rs`). It persists the ceremony,
  principal/key binding and recipient-encrypted delivery, recovers the same
  envelope after interruption, and retains it until owner acknowledgment.
  Acknowledgment checks the recipient proof, acknowledges the provider, durably
  marks local self-custody and clears the retained envelope. Interrupted
  acknowledgment can be retried after restart. This protects key delivery;
  it is a separate provider flow from the HTTP timed-secret ceremony.

Only the human-primary key class is eligible for owner handover. Ordinary
reads, receipts, traces, notifications and managed-agent operations do not
return primary key material.

---

## Verification levels on one chain

`journeys.kvx` and `home.kvx` map achieved evidence to these rungs:

| Network rung | Human verification label | Meaning |
| --- | --- | --- |
| `submitted` | `unverified` | Submission is known; execution success is not established. |
| `receipted` | `receipt-verified` | A verified receipt backs the reported result; block commitment is still pending. |
| `committed` | `checkpoint-finalised` | Committed chain evidence backs the result; block commit is finality. |

`checkpoint-finalised` is retained Human API vocabulary for the committed rung,
not a later settlement step. `paxeer-finalised`, `settlement-anchored`, evidence
class `paxeer-finality`, and the optional `settlement_domain` field are retired
compatibility vocabulary. Local journey state and submission acknowledgments
cannot raise verification.

`GET /v1/evidence/{evidence_id}` returns `evidence_id`, `class`,
`verification`, `content_type` and canonical `bytes_base64`, with optional
`rung` and `inclusion_height`. Retain the actual receipt/proof bytes for
independent verification. Account balance and home projections preserve source
evidence and freshness rather than maintaining a second balance
(`server/production_reads.rs`, `server/production_components.rs`).

Native onboarding requires successful **committed** identity evidence and
validates the action, payload and network before accepting it
(`onboarding/native.rs`). Receipt-only evidence does not activate the account.
Recovery registration remains a separately reported stage; local progress
does not establish protocol completion.

## Retired custody journeys

`wallet-binding`, `deposit`, `withdraw` and `exit` are retained journey kinds
for history. Their eleven HTTP operations are retired and return **410
`operation-retired`**, with copy key `error.operation.retired` and retry
classification `final` (`server/http.rs`, `server/backend.rs`). See the
[retired route inventory](HumanJourneys.md#retired-custody-routes) and current
move quote/commit flow for the supported movement contract.

[Human journeys](HumanJourneys.md) · [Hosted internal](HostedInternal.md) ·
[Home](Home.md)
