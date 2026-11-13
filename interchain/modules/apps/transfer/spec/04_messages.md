<!--
order: 4
-->

# Messages

## MsgTransfer

A fungible token cross chain transfer is achieved by using the `MsgTransfer`:

```go
type MsgTransfer struct {
  SourcePort        string
  SourceChannel     string
  Token             sdk.Coin
  Sender            string
  Receiver          string
  TimeoutHeight     clienttypes.Height
  TimeoutTimestamp  uint64
  Memo              string
}
```

`clienttypes.Height` is defined in
[`interchain/modules/core/02-client/types`](../../../core/02-client/types/height.go),
and `sdk.Coin` comes from this repository's SDK.

[`ValidateBasic`](../types/msgs.go) rejects the message if:

- `SourcePort` is invalid (see 24-host naming requirements)
- `SourceChannel` is invalid (see 24-host naming requirements)
- `Token` is invalid (denom is invalid or amount is negative)
- `Token.Amount` is not positive
- `Sender` is not a valid SDK Bech32 account address
- `Receiver` is blank after trimming whitespace
- `Memo` exceeds `MaximumMemoLength` (32768 bytes)
- `Token.Denom` is not a valid IBC denomination as per [ADR 001 - Coin Source Tracing](https://github.com/cosmos/ibc-go/blob/main/docs/architecture/adr-001-coin-source-tracing.md).

This message will send a fungible token to the counterparty chain represented
by the counterparty Channel End connected to the Channel End with the identifiers
`SourcePort` and `SourceChannel`.

`TimeoutHeight` is a concrete revision-number/revision-height pair.
`TimeoutTimestamp` is an absolute Unix timestamp in nanoseconds. Zero disables
an individual timeout; the core packet checks reject a packet with both
deadlines disabled. See [core packet timeouts](../../../core/spec/01_concepts.md#timeouts-and-channel-closure).

The denomination is its local bank denomination. The keeper expands an `ibc/`
hash to its trace before constructing the packet; the receiving chain adjusts
the path according to the channel-relative source/sink rules.
