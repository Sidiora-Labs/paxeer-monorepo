<!--
order: 3
-->

# State Transitions

These transitions describe the vendored solo-machine v2 client used by Paxeer's
IBC client module. See [Concepts](01_concepts.md) for validation requirements and
proof encoding. `Sequence` tracks the current sequence; `IsFrozen` is a separate
boolean freeze flag.

## Client State Verification Functions

Each successful [state verification](../types/client_state.go) performs these changes:

- increments `Sequence` by one;
- sets the embedded `ConsensusState.Timestamp` to the proof's signature timestamp;
- writes the updated client state to the client store.

The public key, diversifier, `IsFrozen`, and `AllowUpdateAfterProposal` are unchanged.
Proof generation itself causes no on-chain state transition.

## Update By Header

A successful [header update](../types/update.go) performs these changes:

- replaces the embedded consensus state with the header's `NewPublicKey`,
  `NewDiversifier`, and `Timestamp`;
- increments `Sequence` by one.

The handler returns the updated client and consensus states for the
[IBC client keeper](../../../core/02-client/keeper/client.go) to persist.
`IsFrozen` and `AllowUpdateAfterProposal` are unchanged. The keeper requires an
active client, so this operation cannot recover a frozen client.

## Update By Governance Proposal

A successful [substitute update](../types/proposal_handle.go) performs these changes
to the subject client:

- sets `Sequence` to the substitute's current sequence;
- replaces `ConsensusState` with the substitute's consensus state, including its
  public key, diversifier, and timestamp;
- sets `IsFrozen = false`, unfreezing the subject if necessary.

The subject retains its own `AllowUpdateAfterProposal` flag, which must be `true`
for this update to succeed. Core requires a frozen subject and an active
substitute with a higher sequence. The sequence is copied from the substitute
rather than incremented. See [Updates By Proposal](01_concepts.md#updates-by-proposal)
for the other requirements.

## Upgrade

Client upgrades are unsupported. `VerifyUpgradeAndUpdateState` returns
`ErrInvalidUpgradeClient`; no successful upgrade transition occurs. Public-key
changes use [header updates](01_concepts.md#updates-by-header).

## Misbehaviour

Successful [misbehaviour processing](../types/misbehaviour_handle.go) sets
`IsFrozen = true`. It preserves `Sequence`, the embedded consensus state, and
`AllowUpdateAfterProposal`. The evidence's sequence is not stored as a freeze
marker in `ClientState`.

The keeper persists the returned client state. Its status is then `Frozen`, so
ordinary client updates are refused. Proposal recovery, when enabled, clears the
boolean flag as described above. See [Misbehaviour](01_concepts.md#misbehaviour)
for signature checks and the effect of processing order.
