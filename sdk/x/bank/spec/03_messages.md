<!--
order: 3
-->

# Messages

Paxeer's bank message types retain the `cosmos.bank.v1beta1` protobuf namespace.
Definitions are in [tx.proto](../../../proto/cosmos/bank/v1beta1/tx.proto), basic
validation in [types/msgs.go](../types/msgs.go), and processing in
[keeper/msg_server.go](../keeper/msg_server.go). These messages transfer ordinary
coin units; fractional wei is handled by the [keeper APIs](02_keepers.md).

## MsgSend

`/cosmos.bank.v1beta1.MsgSend` transfers `amount` from `from_address` to
`to_address`. The sender signs. Basic validation requires valid Bech32 addresses
and a valid, positive coin set. On success, a missing recipient account is created.

The handler checks the following before transferring:

1. Every denomination must be send-enabled under the parameters and registered
   send-enabled checkers; otherwise it returns `types.ErrSendDisabled`.
2. The sender must pass the denomination allowlists for the transferred coins;
   otherwise it returns `sdkerrors.ErrUnauthorized` with
   `<from_address> is not allowed to send funds`.
3. The recipient must not be blocklisted and must pass those allowlists;
   otherwise it returns `sdkerrors.ErrUnauthorized` with
   `<to_address> is not allowed to receive funds`.

The transfer then requires enough unlocked coins. Insufficient spendable funds,
including funds locked by vesting, return `sdkerrors.ErrInsufficientFunds`.
A registered recipient checker can reject the credit with
`sdkerrors.ErrInvalidRecipient`; balance-change hooks can also return errors.
These errors are propagated rather than treated as successful transfers.

## MsgMultiSend

`/cosmos.bank.v1beta1.MsgMultiSend` supplies `inputs` and `outputs`, each containing
an address and coins. Every input address is a signer. Basic validation requires
at least one input and output, valid addresses and positive coin sets, and equal
total inputs and outputs for each denomination.

The handler checks send-enabled policy and denomination allowlists for **every
input**, then the blocklist and denomination allowlists for **every output**.
An unlisted input produces the same unauthorized-sender error as `MsgSend`; an
unlisted or blocked output produces the same unauthorized-recipient error. Checks
use each input's or output's own coin set, with an allowlist cache shared for that
message.

`InputOutputCoins` debits unlocked input funds and credits outputs, creating
missing recipient accounts. Insufficient spendable funds, recipient-checker
rejection, or a balance-change-hook error aborts processing. Input and output
totals that disagree produce `types.ErrInputOutputMismatch`. In normal SDK
transaction execution, a failure does not commit partial message balance writes.

## Denomination allowlists

`IsInDenomAllowList` applies to denomination strings beginning with `factory`.
A missing or empty list allows all addresses; a non-empty list requires each
participating sender and recipient to appear in `types.AllowList.Addresses`.
Other denominations bypass this allowlist check, but still face send-enabled,
blocklist, balance, and applicable keeper-hook checks.

For example, if `factory/<creator>/restricted` has a non-empty list containing
only Alice and Bob:

- Carol sending that token to Bob fails with
  `<Carol's address> is not allowed to send funds`.
- Alice sending it to Carol fails with
  `<Carol's address> is not allowed to receive funds`.
- Alice sending it to Bob passes the allowlist checks, but can still fail if
  sends are disabled, Bob is blocked, or Alice lacks spendable funds.

An [authz send authorization](../../authz/spec/01_concepts.md#sendauthorization)
does not bypass these rules: the inner `MsgSend` still checks the granter as sender
and the actual recipient. See [State](01_state.md) for allowlist storage and
recovery, and [Keepers](02_keepers.md) for lower-level policy obligations.

## Error identities

The relevant sentinel errors have these codespaces and codes; descriptions above
may be wrapped with context:

| Error | Codespace | Code |
| --- | --- | --- |
| `types.ErrNoInputs` | `bank` | 2 |
| `types.ErrNoOutputs` | `bank` | 3 |
| `types.ErrInputOutputMismatch` | `bank` | 4 |
| `types.ErrSendDisabled` | `bank` | 5 |
| `sdkerrors.ErrUnauthorized` | `sdk` | 4 |
| `sdkerrors.ErrInsufficientFunds` | `sdk` | 5 |
| `sdkerrors.ErrInvalidRecipient` | `sdk` | 46 |

Definitions: [bank errors](../types/errors.go) and
[SDK errors](../../../types/errors/errors.go). In particular, allowlist failures
and blocklisted recipients share `ErrUnauthorized`; they are distinct from a
send-disabled error in the `bank` codespace.
