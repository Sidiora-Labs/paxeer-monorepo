<!--
order: 1
-->

# Concepts

This guide describes the vendored ICS06 solo-machine v2 implementation used by
Paxeer's IBC client module. Its wire types are defined in the
[v2 protobuf schema](../../../../proto/ibc/lightclients/solomachine/v2/solomachine.proto).
See [State Transitions](03_state_transitions.md) for the effects of each operation.

## Client State

The `ClientState` stores four fields:

- `Sequence`: the current solo-machine sequence, which must be nonzero.
- `IsFrozen`: a boolean indicating whether the client is frozen.
- `ConsensusState`: the current public key, diversifier, and timestamp.
- `AllowUpdateAfterProposal`: whether governance may update this client using a substitute.

[`ClientState.Status`](../types/client_state.go) returns `Frozen` when `IsFrozen`
is `true` and `Active` otherwise. New clients start with `IsFrozen = false`.
The freeze flag is independent of `Sequence`; freezing does not store an offending
sequence in the client state. The IBC height representation uses revision number
zero and the solo-machine sequence as the revision height.

## Consensus State

The [`ConsensusState`](../types/consensus_state.go) stores the public key,
diversifier, and timestamp. The latest consensus state is embedded in `ClientState`.
The timestamp must be nonzero.

The diversifier separates signatures when the same key is reused across clients
or chains. Choose a distinct diversifier for each such context. Empty diversifiers
are accepted, but a nonempty diversifier containing only whitespace is invalid.

## Public Key

The public key can be a single public key or a multi-signature public key. It must
implement the SDK's `cryptotypes.PubKey` interface, defined in
[`sdk/crypto/types`](../../../../../sdk/crypto/types/types.go), and its concrete type
must be registered with the application codec so that it can be unpacked from a
protobuf `Any`. The consensus state stores that `Any`, rather than raw key bytes.

[`VerifySignature`](../types/proof.go) requires `SingleSignatureData` for a single
key and `MultiSignatureData` for a multisig key. For multisig keys, the key's
`VerifyMultisignature` implementation checks the constituent signatures.

## Counterparty Verification

The solo-machine light client verifies signed counterparty client states,
consensus states, connection states, channel states, packet commitments, packet
acknowledgements, packet receipt absence, and the next receive sequence.

The [verification functions](../types/client_state.go) check the proof's sequence,
timestamp, commitment prefix, and signature using the current consensus public
key and diversifier. The signature timestamp must be at least the stored timestamp.
Each successful verification increments `Sequence` by one, updates the embedded
consensus timestamp to the proof timestamp, and persists the client state.

Proof heights use revision number zero. For connection-handshake ordering,
`VerifyClientState` adds one to the supplied revision height before comparing it
with the current sequence; `VerifyClientConsensusState` adds two. Other state
verification functions compare the supplied revision height directly.

## Proofs

A proof attests that the current solo-machine key signed a particular state at a
particular sequence, timestamp, and diversifier. Use the same timestamp throughout
the following encoding steps:

1. Marshal the operation's data message, including the prefixed commitment path.
   The `...DataBytes` helpers in [proof.go](../types/proof.go) construct these messages.
2. Marshal `SignBytes` containing the sequence, timestamp, current diversifier,
   operation-specific `DataType`, and marshaled data. The `...SignBytes` helpers
   combine these first two steps. `DataType` distinguishes the signed message types.
3. Sign those bytes. Put the signature in `SingleSignatureData`, or assemble
   `MultiSignatureData` for a multisig key. Convert it with
   `signing.SignatureDataToProto` and marshal the resulting `SignatureDescriptor_Data`.
4. Put those serialized signature descriptor bytes in
   `TimestampedSignatureData.SignatureData`, set its timestamp, and marshal the
   wrapper. Pass the wrapper bytes as the verification function's `proof` argument.

For example, this helper produces a single-key client-state proof. The caller
supplies the codec with the required interfaces registered, the current private
key, the already-prefixed client-state path, and the state to attest to:

```go
import (
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	solomachinetypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/06-solomachine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
)

func clientStateProof(
	cdc codec.BinaryCodec,
	key cryptotypes.PrivKey,
	path commitmenttypes.MerklePath,
	clientState exported.ClientState,
	sequence, timestamp uint64,
	diversifier string,
) ([]byte, error) {
	signBz, err := solomachinetypes.ClientStateSignBytes(
		cdc, sequence, timestamp, diversifier, path, clientState,
	)
	if err != nil {
		return nil, err
	}

	sig, err := key.Sign(signBz)
	if err != nil {
		return nil, err
	}
	sigData := &signing.SingleSignatureData{Signature: sig}
	protoSigData := signing.SignatureDataToProto(sigData)
	bz, err := cdc.Marshal(protoSigData)
	if err != nil {
		return nil, err
	}

	return cdc.Marshal(&solomachinetypes.TimestampedSignatureData{
		SignatureData: bz,
		Timestamp:     timestamp,
	})
}
```

`SignatureData` is a byte slice containing the encoded signature descriptor, not
the `SingleSignatureData` object or just the raw signature. The verifier decodes
it through [`UnmarshalSignatureData`](../types/codec.go).

The executable example in
[`TestRepairM_1234_DocumentedProofRoundTrip`](../repair_m1234_test.go) compiles
this helper and passes its wrapper bytes to `ClientState.VerifyClientState`,
then checks the sequence and timestamp persisted by the verifier. The companion
rejection test covers a changed wrapper timestamp and raw signature bytes in
place of the encoded signature descriptor.

Coordinate proof generation with consumption: each successful verification
advances the stored sequence, so the next distinct proof uses that next sequence.
Generating proof bytes alone does not update the on-chain client state.

## Updates By Header

The [IBC client keeper](../../../core/02-client/keeper/client.go) requires an active
client. After basic header validation, the [solo-machine update handler](../types/update.go)
requires:

- a solo-machine `Header` whose sequence equals the current client sequence;
- a timestamp greater than or equal to the current consensus timestamp;
- a valid signature from the currently registered key over `HeaderSignBytes`.

`HeaderSignBytes` uses `HEADER` as its data type and the header's **new** diversifier
in its signing envelope. Its payload contains the new public key and diversifier.
`Header.Signature` contains the serialized signature descriptor bytes directly,
without a `TimestampedSignatureData` wrapper.

A successful update replaces the embedded consensus state with the header's new
public key, diversifier, and timestamp, and increments `Sequence` by one. It does
not clear `IsFrozen`; ordinary header updates cannot recover a frozen client.

## Updates By Proposal

The [substitute update handler](../types/proposal_handle.go) requires:

- the subject client's `AllowUpdateAfterProposal` flag to be `true`;
- a substitute of type `*ClientState` for a solo machine;
- decodable subject and substitute public keys that are different.

A successful update copies the substitute's `Sequence` and `ConsensusState` into
the subject and sets the subject's `IsFrozen` to `false`. The subject keeps its
own `AllowUpdateAfterProposal` setting; the handler does not copy every field of
the substitute client.

## Misbehaviour

The IBC client keeper requires an active client and calls
[`Misbehaviour.ValidateBasic`](../types/misbehaviour.go). Evidence must contain two
different signed data messages and different signatures at the same nonzero
sequence, with valid identifiers, data types, and nonzero timestamps.

The [misbehaviour handler](../types/misbehaviour_handle.go) verifies both signatures
using the currently registered public key and diversifier. It permits past
misbehaviour: the evidence sequence need not equal the latest client sequence,
and its timestamps are not compared with the current consensus timestamp.

If both signatures verify, the handler sets `IsFrozen = true`. It leaves the
client's `Sequence`, consensus state, and proposal-update setting unchanged.

Misbehaviour processing depends on transaction order. Evidence against an old key
may no longer verify if a key update is processed first; the handler does not
track previous public keys for this purpose.

## Upgrades

`VerifyUpgradeAndUpdateState` returns `ErrInvalidUpgradeClient` because solo-machine
client upgrades are unsupported. Normal header updates can replace the public
key, including its type, without using the upgrade mechanism.
