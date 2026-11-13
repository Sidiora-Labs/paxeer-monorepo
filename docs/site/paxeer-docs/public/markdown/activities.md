<!-- Source: https://docs.paxeer.app/activities/ -->

# Activities

The signed, canonical unit of agent action across the Paxeer X execution system.

A payment, escrow capture, budget spend, stream draw, service delivery, trade, governance action, bridge claim, or program call enters LayerX as one signed activity. The kernel verifies the envelope and authority, orders admitted work globally, executes the operation, and produces protocol-computed receipt evidence. An EVM transaction and an activity are distinct operation types on the same network. Kernel state changes commit with the node's blocks.

## The exact envelope

The `lxp_activity` structure carries exactly twelve fields. Omitting, repeating, or introducing an undeclared field makes an envelope malformed. LXC/1 uses canonical binary encoding, fixed-width big-endian integers, and no floating-point values or unordered maps. Trailing bytes are refused.

| Field | Meaning |
| --- | --- |
| `protocol_version` | The enabled execution rules. Version 1 is legacy, 2 includes occupancy, and 3 includes state commitment. |
| `network_id` | The exact execution network; prevents cross-network replay. |
| `activity_type` | A 32-bit module and operation discriminator. |
| `actor_did` | The identity whose authority and sequence authorize the action. |
| `authority` | A primary-key, session-key, or scoped-grant authorization reference. |
| `account_sequence` | The actor identity’s exact next expected counter. |
| `timestamp_bound` | `not_before` and `not_after` bounds checked against batch time. |
| `idempotency_key` | A 32-byte actor-scoped key binding retries to one economic result. |
| `fee_limit` | The maximum authorized deterministic fee, represented as `lxp_u128`. |
| `payload_hash` | A 32-byte domain-separated commitment to the canonical payload. |
| `payload` | The module’s canonical operation body. |
| `signature` | Ed25519 authorization over the canonical envelope signing preimage, excluding this field. |

### Module and ordinal

```
activity_type = (module_id << 16) | operation_ordinal
module_id = lxp_activity_module_id(activity_type)
operation_ordinal = lxp_activity_type_ordinal(activity_type)

Programs module: 0x09
Governance migration budget: 0x0007000a
Governance programs fee:     0x0007000b
```

The ordinal is module-specific. Use the operation encoding for the target module rather than treating every payload as a transfer. The governance header defines only the two activity types above for module `0x0007`. The compile-time `LXP_PROTOCOL_VERSION` default is 2; applications explicitly select the protocol version enabled by their execution environment.

## Admission and execution

1. Check protocol version, network, and payload binding in that order.
2. Resolve the actor’s current authority and verify its signature.
3. Enforce the exact account sequence and the batch-time validity window.
4. Resolve idempotency and compute fees against the active schedule and fee limit.
5. Validate and execute the module operation within the state journal.
6. Commit successful effects or roll back failed module effects; produce the applicable receipt and bookkeeping.

| Outcome | Application consequence |
| --- | --- |
| Malformed or wrong-network envelope | No admission, sequence consumption, or execution fee. |
| Payload binding mismatch | Refused before authority evaluation. |
| Sequence too low or too high | Replay or gap refusal. Recover the actor’s current identity counter before preparing new work. |
| Fee exceeds the limit | The requested effects are not applied. |
| Admitted execution failure | Module effects roll back; the sequence, global position, and applicable fee bookkeeping persist. |
| Idempotent retry | The original receipt returns without a second economic effect. |

## Submitting and recovering

The hosted gateway does not serve `lx_*` methods. It answers `-32601 Method not found` for every `lx_*` method. Do not build submission or recovery flows on `lx_sendActivity`, `lx_getActivityStatus`, or `lx_getReceipt`. The live hosted gateway API is the `px_` family defined by its OpenRPC contract, including `px_register`, `px_resolveAccount`, `px_getReceiptKeys`, and `px_getSequenceBatch`. Obtain its deployment URL and credentials from the operator. Public nodes expose `pax_sequencerReceipt` for receipt progress.

1. Prepare the module payload and exact envelope; retain the idempotency key.
2. Sign the canonical preimage with currently valid authority.
3. Submit the same canonical bytes through the submission path your operator provides, and retain the activity identifier.
4. Check the verified receipt’s result code before reporting success.
5. Track finality with `pax_sequencerReceipt`: `submitted`, then `receipted`, then `committed`. A receipt is final only at `committed`. If the outcome is unknown, recover the original result instead of creating a replacement economic action.

### Keep the two sequence systems separate

The envelope’s account sequence belongs to the actor DID. Transfer sets also carry an actor sequence for their designated sequence account, usually the debited asset record. A per-asset account can start at zero while its owner identity already has a long activity history. Read the correct counter for each protocol structure.

## Native codec interfaces

`lxp_activity_encode` and `lxp_activity_decode` handle canonical bytes; `lxp_activity_id` identifies those bytes; `lxp_activity_signing_preimage` produces the signing commitment. Envelope, payload-hash, and signature verification are exposed through `lxp_activity_check_envelope`, `lxp_activity_verify_payload_hash`, and `lxp_activity_verify_signature`.

## Continue reading

- [Accounts and sequence ownership](https://docs.paxeer.app/accounts)

- [Identity and delegated authority](https://docs.paxeer.app/identity)

- [Receipts and result verification](https://docs.paxeer.app/receipts)

- [Kernel execution](https://docs.paxeer.app/layerx-kernel)

- [Module operations](https://docs.paxeer.app/modules)
