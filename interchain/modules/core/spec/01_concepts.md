<!--
order: 1
-->

# Concepts

IBC core provides clients, connections, channels and packet proof verification
for communication with counterparty chains. In the merged Paxeer network these
keepers execute within `paxd`, alongside the EVM and the kernel that participates
in block proposal. Paxeer mainnet-beta has EVM chain id `125` (`0x7d`); a receipt
is final when its block is committed. IBC packet completion additionally depends
on counterparty state proofs and their relay.

Sub-chains are tenant domains in the database. A tenant namespace alone does not
establish an IBC client, connection or channel to another chain.

## Client Misbehaviour

Each IBC client implementation defines the evidence it accepts and how it
freezes. The local Tendermint implementation accepts verified conflicting
headers at the same height with different block hashes, or headers at different
heights that violate monotonic block time. Both headers must pass the light
client's verification against their trusted consensus states; an allegation
alone does not freeze a client.

The [Tendermint misbehaviour handler](../../light-clients/07-tendermint/types/misbehaviour_handle.go)
sets `FrozenHeight` to the non-zero sentinel height `{0, 1}`. This implementation
treats it as a frozen flag rather than a boundary allowing proof verification
below the offending height. Core proof verification requires an active client.
Validator evidence handled by consensus is separate from IBC client
misbehaviour processing.

Governance recovery uses a `ClientUpdateProposal` with `SubjectClientId` and
`SubstituteClientId`, as implemented in
[the client proposal keeper](../02-client/keeper/proposal.go):

- Both clients must exist and neither may be the localhost client.
- The subject must not be active (for Tendermint, frozen or expired); the
  substitute must be active.
- The substitute's latest height must be strictly greater than the subject's.
- The subject's `CheckSubstituteAndUpdateState` implementation must accept the
  substitute and update the required state successfully.

For [Tendermint clients](../../light-clients/07-tendermint/types/proposal_handle.go),
compatibility requires matching client types and parameters, except frozen
height, latest height, trusting period, chain ID and the deprecated
`AllowUpdateAfterExpiry`/`AllowUpdateAfterMisbehaviour` flags. Recovery clears a
frozen subject's frozen height, copies the substitute's latest consensus state
and its processed-height/time metadata, and adopts the substitute's latest
height, chain ID and trusting period. It retains the subject client identifier,
so existing connections continue to refer to that client. Recovery does not
consist of overriding a header and does not reopen closed channels.

Submit the proposal using the current [client-update proposal CLI](../02-client/client/cli/tx.go),
registered under governance's `submit-proposal` command:

```sh
paxd tx gov submit-proposal update-client 07-tendermint-0 07-tendermint-1 \
  --title "Recover IBC client" \
  --description "Recover the subject from a compatible active substitute" \
  --deposit "$DEPOSIT" --from "$PROPOSER" --chain-id "$CHAIN_ID"
```

The first ID is the existing subject to recover; the second is the substitute
whose state will be copied. Set the deposit, proposer and chain ID for the chain
hosting these clients. Submitting a proposal does not itself recover the client:
the checks above run when governance executes the approved proposal through the
[client proposal handler](../02-client/proposal_handler.go). The deprecated
allow-update flags do not bypass these checks. See
[`TestRepairM_1225_GovernanceRecovery`](../repair_m1225_test.go) for CLI-generated
proposal encoding, rejected prerequisites, and frozen/expired subject recovery
through that handler with consensus-state metadata preserved.

## Connection Handshake

The four-step connection handshake follows
[ICS 03](https://github.com/cosmos/ibc/blob/master/spec/core/ics-003-connection-semantics).
Each connection end refers to a local light client of the counterparty chain.
Connection identifiers are generated locally as `connection-{N}`, beginning
with `connection-0`; identifiers at the two ends need not match.

[The connection keeper](../03-connection/keeper/handshake.go) implements:

1. `ConnOpenInit`: creates the local connection in `INIT`. The counterparty
   connection ID is empty so the other chain can allocate its own. An optional
   version must be supported; otherwise the compatible versions are stored.
   This step requires outbound connections to be enabled.
2. `ConnOpenTry`: verifies proof of the counterparty's `INIT` connection, the
   client state it holds for this chain and the corresponding consensus state.
   It validates the self-client parameters and negotiates a compatible version.
   If `previousConnectionID` is supplied for crossing handshakes, that local
   connection must exist in `INIT` with matching client IDs, counterparty prefix
   and delay period and no counterparty connection ID. Otherwise a new ID is
   generated. Success stores `TRYOPEN`. This step requires inbound connections
   to be enabled.
3. `ConnOpenAck`: requires the local end in `INIT` or `TRYOPEN`, verifies the
   counterparty's `TRYOPEN` connection and its client and consensus states, and
   validates the selected version. It stores that version and the counterparty
   connection ID and moves the local end to `OPEN`.
4. `ConnOpenConfirm`: requires the local end in `TRYOPEN` and verifies a proof
   that the counterparty end is `OPEN`, then moves the local end to `OPEN`.

These proofs are verified against client consensus state, not inferred from a
relayer's report that the other chain accepted a transaction.

## Connection Version Negotiation

A connection version has this shape (see
[version.go](../03-connection/types/version.go)):

```go
type Version struct {
    Identifier string
    Features   []string
}
```

Identifiers and individual feature strings must not be blank. Basic version
validation permits an empty feature list, but negotiation for the currently
supported identifier `"1"` requires a non-empty feature intersection. Its
supported features are `ORDER_ORDERED` and `ORDER_UNORDERED`.

`ConnOpenInit` stores the compatible versions, or a single explicitly requested
supported version. `ConnOpenTry` calls `PickVersion`, iterating the local
preference list and choosing the first identifier also supported by the
counterparty whose feature intersection is allowed. It uses the previous
connection's versions when continuing an initialized handshake. It returns an
error when no compatible choice exists.

`ConnOpenAck` checks that the selected version is supported when the local end
is `INIT`. If the local end is already `TRYOPEN`, it must exactly match that
end's single negotiated version. After acknowledgement both sides have agreed
on the version and feature set.

::: warning
Keep version identifiers unique within each offered list. Repeating an
identifier with different feature sets makes first-match selection ambiguous.
:::

## Channel Version Negotiation

Channel version strings belong to applications. Core verifies the channel
handshake proofs and ordering support, while the application callbacks decide
which version strings they accept. The
[core message server](../keeper/msg_server.go) calls these callbacks before
writing the resulting channel state.

`ChanOpenInit` proposes and stores the application's initial version.
`ChanOpenTry` verifies the counterparty's `INIT` channel using
`CounterpartyVersion`, calls `OnChanOpenTry`, and stores the version returned by
that callback. If continuing an existing `INIT` channel, its parameters,
including its version, must match the expected handshake values.

During `ChanOpenAck`, the application validates the `CounterpartyVersion`
proposed by the other end. Core then stores that version and opens the local
channel. `ChanOpenConfirm` verifies the counterparty is open and completes the
other end.

Core does not impose a shared application version format. Applications may use
structured version strings, semantic versions or exact string matching; an
empty string is usable only if the application's callbacks accept it. The
[transfer application](../../apps/transfer/ibc_module.go) requires `UNORDERED`
channels on its bound port (normally `transfer`) and exactly `ics20-1` during
init, try and acknowledgement. It does not accept an empty version.

## Sending, Receiving, Acknowledging Packets

The [packet keeper](../04-channel/keeper/packet.go) manages:

- **Packet commitment**: a hash commitment to a sent packet, stored on the
  sending chain under its source port, source channel and sequence.
- **Packet receipt**: a marker stored on the receiving chain for an unordered
  channel. Ordered channels instead track the next receive sequence.
- **Acknowledgement**: application result bytes whose hash is committed on the
  receiving chain. Relayers supply the full bytes for verification on the sender.

For unordered channels, a typical successful packet lifecycle is:

| Stage | Sending chain | Receiving chain |
|-------|---------------|-----------------|
| Not sent | No packet commitment | No receipt or acknowledgement |
| Sent | Packet commitment exists | No receipt or acknowledgement |
| Received, asynchronous acknowledgement pending | Commitment remains | Receipt exists, no acknowledgement yet |
| Acknowledgement written | Commitment remains until relay | Receipt and acknowledgement commitment exist |
| Acknowledgement processed on sender | Packet commitment deleted | Receipt and acknowledgement commitment remain |

An application initiates sending through `ChannelKeeper.SendPacket` or its ICS4
wrapper. Core checks the channel, capability, sequence, packet fields and
counterparty timeout bounds against the client's known state before storing
the commitment and advancing the send sequence. Relayers reconstruct packet
data from the sending transaction's events and relay it with commitment proofs.

The receiver verifies the sender's packet commitment through its light client,
using a consensus state at the proof height. A proof cannot verify against a
state that predates the commitment. Core also checks the receiving chain's own
height and timestamp against the packet deadlines. On an unordered channel it
writes a receipt; on an ordered channel it checks and advances the receive
sequence. Duplicate receives do not execute the application again.

Applications may acknowledge synchronously in `OnRecvPacket`, or return no
acknowledgement and write it asynchronously later. The
[transfer module](../../apps/transfer/spec/01_concepts.md#acknowledgements)
decodes ICS20 packet data as JSON and returns a synchronous JSON acknowledgement.
Other applications define their own data encodings and acknowledgement rules.

When an acknowledgement is relayed, the sender verifies its hash against the
counterparty commitment and deletes the packet commitment. The application
callback interprets the result. For ICS20, an error acknowledgement refunds the
original sender by unescrowing tokens or re-minting burned vouchers; a success
acknowledgement leaves the completed transfer intact. An application error
acknowledgement still represents a received packet, so it is not a timeout.

### Timeouts and channel closure

An ordinary timeout requires proof that the packet was not received and that
**at least one enabled deadline has been reached** on the receiving chain.
[TimeoutPacket](../04-channel/keeper/timeout.go) accepts either:

- A non-zero timeout height with `proofHeight >= timeoutHeight`; or
- A non-zero timeout timestamp with `proofTimestamp >= timeoutTimestamp`, where
  `proofTimestamp` is the counterparty consensus timestamp at `proofHeight`
  (timestamps are in nanoseconds).

A zero height or timestamp disables that individual deadline. Both cannot be
zero for a valid packet. A deadline is reached at equality; both enabled
bounds do not have to be reached. Local sender wall-clock time alone is not a
counterparty timeout proof.

The receiver's [RecvPacket checks](../04-channel/keeper/packet.go) likewise reject
a packet when either enabled local deadline is reached. The
[packet validation](../04-channel/types/packet.go) rejects packets with both
deadlines disabled. Representative keeper tests are
[`TestRepairM_1225_TimeoutDeadlines`](../repair_m1225_test.go) and
[`TestTimeoutPacket`](../04-channel/keeper/timeout_test.go): they cover independent
deadlines, disabled fields, equality and non-receipt proofs.

For unordered channels, non-receipt is proven by absence of the packet receipt.
For ordered channels, the next receive sequence must prove that the packet's
sequence has not been received. After successful application timeout handling,
`TimeoutExecuted` deletes the sender's packet commitment and closes the local
channel if it is ordered. The transfer callback refunds the sender.

`TimeoutOnClose` instead verifies that the counterparty channel is `CLOSED`
and that the individual packet was not received. It does not require a packet
deadline to have elapsed. A closed channel does not automatically refund all
tokens or erase outstanding obligations; relayers must submit the applicable
proofs and execute the packet callbacks. The
[`TimeoutOnClose` keeper checks](../04-channel/keeper/timeout.go) and
[`TestRepairM_1225_TimeoutOnClose`](../repair_m1225_test.go) demonstrate closure
and non-receipt verification before either deadline. See the transfer application's
[locked-funds discussion](../../apps/transfer/spec/01_concepts.md#locked-funds)
for the effect of channel closure on voucher redemption.
