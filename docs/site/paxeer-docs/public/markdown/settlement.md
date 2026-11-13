<!-- Source: https://docs.paxeer.app/settlement/ -->

# Checkpoint settlement & evidence

Follow LayerX activities from ordered receipts to bonded replay, finalized Paxeer checkpoints, and verifiable custody claims.

## Execution and settlement form one history

Paxeer X settles LayerX activity on EVM chain ID `125`. The sequencer orders activities; guarantors independently replay sealed batches; Paxeer records the resulting checkpoints and enforces custody releases. Payments, program calls, program-owned transfers, and storage occupancy charges participate in the same deterministic history.

| Level | Evidence | Meaning |
| --- | --- | --- |
| L0 · Accepted | Sequencer receipt | The activity is ordered in the current batch and bound to a state transition. |
| L1 · Sealed | Fixed batch history | Ordering is fixed and the batch is ready for distribution. |
| L2 · Distributed | Batch and availability bundle | Guarantors receive the material needed for independent replay. |
| L3 · Attested | Bonded guarantor certificate | Guarantors attest byte-identical replay results. |
| L4 · Settled | Paxeer checkpoint | Settlement custody enforces finality, challenge windows, withdrawal limits, and exits. |

A guarantor attestation is bonded re-execution evidence. Its security combines bonds, challenges, availability checks, and release limits. Applications retain the receipt for the fast path and follow the same activity into settlement evidence before presenting settled funds.

## Native checkpoint anchor

The `layerxanchor` precompile at `0x0000000000000000000000000000000000001014` records checkpoints, finality, availability attestations, and guarantor bonds. Anyone can submit a self-verifying checkpoint certificate. Finalization requires the configured threshold of active bonded guarantors, continuity with the finalized chain, no open challenge, and completion of the challenge window.

| Method | Purpose |
| --- | --- |
| `submitCheckpoint(header, headerSignature, certificate)` | Admit a canonical 354-byte header, 64-byte sequencer signature, and guarantor certificate. |
| `submitAvailabilityAttestation(attestation)` | Record a 274-byte attestation for a known checkpoint. |
| `finalize(batchNumber)` | Finalize a checkpoint after its challenge conditions clear. |
| `latestFinalized()`, `checkpoint(batchNumber)` | Read finalized progress and full checkpoint metadata. |
| `finalizedStateRoot(batchNumber)`, `finalizedReceiptRoot(batchNumber)` | Retrieve roots together with their finality flags. |
| `openChallenge(batchNumber, kind, evidenceHash)` | Open a bonded fraud or data-availability challenge. |
| `submitEquivocation(evidenceA, evidenceB)` | Present conflicting guarantor attestations for slashing. |

Anchor status is `0` unknown, `1` submitted, or `2` final. Read the finality flag with a root; the existence of a root alone does not establish payout eligibility. The exact interface is `precompiles/layerxanchor/LayerXAnchor.sol`.

## Deposit-root registration

The custody precompile (`ILayerXCustody`) records the deposit root of a finalized checkpoint with `registerDepositRoot(registration, signature, leafOrdering)`. Only the account that submitted the checkpoint to the anchor may call it. No settlement publication contract exposing an `EVIDENCE_VERSION` or withdrawal/balance witness vectors exists in the current contracts; do not build against one.

| Item | Contents | Commitment |
| --- | --- | --- |
| Deposit root registration | Registration bytes, deposit-root authority signature, and positional leaf ordering. The ordering is committed to, not verified by the precompile. | `SHA256(abi.encode(uint16(2), registration, signature, leafOrdering))`, read back with `depositRegistrationDigest(checkpointId)` |

Registration records evidence; payout still requires the custody checks on the claim path. Use `depositRootRegistered(checkpointId)` and `depositRootAuthority()` to inspect registration state.

### Independent signing roles

A separately configured Ed25519 deposit-root authority signs the registration. Guarantors attest replay. A guarantor key cannot substitute for the deposit-root authority.

```
Deposit-root registration:
"LX:PAXEER:DEPOSIT:ROOT:v1" || checkpoint_id32
|| checkpoint_state_root32 || deposit_root32 || custody_reference32
|| network_id:u32 (4 bytes) || protocol_version:u16 (2 bytes)
```

The `signature` argument is the deposit-root authority's Ed25519 signature over those registration bytes. An Ed25519 public key is not converted into an EVM address.

## Producer and consumer workflow

1. Replay native history independently and construct the deposit evidence for the checkpoint.
2. Obtain the configured deposit-root authority's signature over the registration bytes.
3. Submit and finalize the checkpoint at the anchor, then call `registerDepositRoot` from the checkpoint submitter account.
4. Verify `depositRootRegistered` and `depositRegistrationDigest` before treating the deposit root as recorded.
5. Apply claim-specific custody checks and verify the final payout on the custody path.

## Verify the transport and the money

Settlement clients verify endpoint TLS trust and chain identity before consuming evidence. A healthy process or matching chain ID establishes connectivity, while finalized checkpoint evidence establishes the settlement claim. Do not elevate one finality level merely because an earlier stage succeeded.

Continue with [deposits, withdrawals, and emergency exits](https://docs.paxeer.app/custody), [Paxeer consensus](https://docs.paxeer.app/consensus), and [native EVM interfaces](https://docs.paxeer.app/precompiles).
