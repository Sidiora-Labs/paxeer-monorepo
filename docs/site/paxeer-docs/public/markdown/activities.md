<!-- Source: https://docs.paxeer.app/activities/ -->

# Activities

The signed, canonical unit of agent action across the Paxeer X execution system.

The kernel represents native operations as activities with an actor, authority, payload, sequence, and receipt. Public clients submit signed EVM transactions to node JSON-RPC; native precompiles invoke the kernel within those transactions. The kernel is part of the Paxeer mainnet-beta BFT node and proposes blocks. Kernel state changes commit with the node's blocks, and a receipt is final when its block is committed.

## The exact envelope

The `lxp_activity` structure carries exactly twelve fields. Omitting, repeating, or introducing an undeclared field makes an envelope malformed. LXC/1 uses canonical binary encoding, fixed-width big-endian integers, and no floating-point values or unordered maps. Trailing bytes are refused.

| Field | Meaning |
| --- | --- |
| `protocol_version` | The execution rules. Version 1 is the base format, 2 includes occupancy, and 3 includes state commitment. |
| `network_id` | The exact execution network; prevents cross-network replay. |
| `activity_type` | A 32-bit module and operation discriminator. |
| `actor_did` | The identity whose authority and sequence authorize the action. |
| `authority` | Authority bytes resolved by the execution path; the standalone Ed25519 verifier requires a 32-byte public key. |
| `account_sequence` | The actor identity’s exact next expected counter. |
| `timestamp_bound` | `not_before` and `not_after` bounds checked against batch time. |
| `idempotency_key` | A 32-byte retry key carried by the operation; admission refuses a key marked as already present in its context. |
| `fee_limit` | The maximum authorized deterministic fee, represented as `lxp_u128`. |
| `payload_hash` | A 32-byte domain-separated commitment to the canonical payload. |
| `payload` | The module’s canonical operation body. |
| `signature` | The standalone verifier checks a 64-byte Ed25519 signature over the canonical signing digest, excluding this field. EVM calls authenticate through their transaction authority. |

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
2. Check the batch-time validity window.
3. Require a valid signature under the resolved authority.
4. Enforce the exact account sequence, check the admission context for a reused idempotency key, and require a spendable fee limit.
5. Compute the fee and validate and execute the module operation within the state journal.
6. Commit successful effects or roll back failed module effects; produce the applicable receipt and bookkeeping.

| Outcome | Application consequence |
| --- | --- |
| Malformed or wrong-network envelope | No admission, sequence consumption, or execution fee. |
| Payload binding mismatch | Refused before authority evaluation. |
| Sequence too low or too high | Replay or gap refusal. Recover the actor’s current identity counter before preparing new work. |
| Fee exceeds the limit | The requested effects are not applied. |
| Admitted execution failure | Module effects roll back; the sequence, global position, and applicable fee bookkeeping persist. |
| Reused idempotency key | Admission refuses a key marked as already present. Reconcile the original transaction and receipt before preparing another action. |

## Submitting and recovering

Submit caller-signed EIP-1559 bytes with `eth_sendRawTransaction` at the node root endpoint. Its EVM chain ID is `125` (`0x7d`). The hosted gateway API is the `px_` family defined in `services/platform/hosted/gateway/openrpc.json`, including account resolution and receipt-key discovery. Public nodes expose `pax_sequencerReceipt` for transaction receipt progress.

1. Prepare the target precompile call and read the sender's pending EVM nonce.
2. Sign the EIP-1559 transaction with currently valid EVM authority.
3. Submit the signed bytes through eth_sendRawTransaction and retain the transaction hash.
4. Verify the receipt against independently trusted receipt keys and check its result before reporting success.
5. Track finality with `pax_sequencerReceipt`: `submitted`, then `receipted`, then `committed`. A receipt is final only at `committed`. If the outcome is unknown, recover the original result before preparing another economic action.

### Keep the two sequence systems separate

The native envelope’s account sequence belongs to the actor DID; it is separate from the EVM sender nonce. Transfer sets also carry an actor sequence for their designated sequence account, usually the debited asset record. A per-asset account can start at zero while its owner identity already has a long activity history. Read the correct counter for each protocol structure.

## Native codec interfaces

`lxp_activity_encode` and `lxp_activity_decode` handle canonical bytes; `lxp_activity_id` identifies those bytes; `lxp_activity_signing_preimage` produces the signing commitment. Envelope, payload-hash, and signature verification are exposed through `lxp_activity_check_envelope`, `lxp_activity_verify_payload_hash`, and `lxp_activity_verify_signature`.

## Continue reading

- [Accounts and sequence ownership](https://docs.paxeer.app/accounts)

- [Identity and delegated authority](https://docs.paxeer.app/identity)

- [Receipts and result verification](https://docs.paxeer.app/receipts)

- [Kernel execution](https://docs.paxeer.app/layerx-kernel)

- [Module operations](https://docs.paxeer.app/modules)
