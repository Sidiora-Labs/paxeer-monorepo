<!--
order: 7
-->

# Parameters

The transfer module keeps these boolean parameters in its parameter subspace.
The keys and defaults are defined in [types/params.go](../types/params.go);
[keeper/params.go](../keeper/params.go) reads and writes them.

| Parameter key | JSON field | Type | Default |
|---------------|------------|------|---------|
| `SendEnabled` | `send_enabled` | bool | `true` |
| `ReceiveEnabled` | `receive_enabled` | bool | `true` |

The `Params` query returns both under the `params` response field. Where the
Cosmos gRPC gateway is configured, its declared HTTP route is
`GET /ibc/apps/transfer/v1/params`. These are module parameters, not
per-channel switches.

## SendEnabled

`SendEnabled = false` rejects new outbound transfers for every denomination
with `ErrSendDisabled` from the transfer types package (codespace `transfer`,
code `7`).
[sendTransfer](../keeper/relay.go) checks this before changing balances or
sending a packet.

When this parameter is true, the keeper also checks
`bankKeeper.IsSendEnabledCoin(ctx, token)` for the actual local coin denomination.
A bank-level rejection returns a wrapped `ErrSendDisabled`. To block
one denomination from new outbound IBC transfers while allowing others, keep
the module's `SendEnabled` true and disable that denomination in the bank's
send policy. For a voucher, the bank denomination is its local `ibc/{hash}`
representation, not the expanded packet trace. Bank policy and registered bank
send-enabled checkers can impose further restrictions even when the module
parameter is true.

## ReceiveEnabled

`ReceiveEnabled = false` rejects inbound transfer application processing for
all denominations with `ErrReceiveDisabled` from the transfer types package (codespace `transfer`,
code `8`). The [receive keeper](../keeper/relay.go) validates packet data first,
then checks this parameter before minting vouchers or unescrowing coins. The
IBC callback converts a receive error into an error acknowledgement; relaying
that acknowledgement refunds the sender on the sending chain.

There is no per-denomination receive parameter here. The receive path does not
call `IsSendEnabledCoin`, so disabling a bank denomination's `SendEnabled`
policy does not implement an inbound IBC freeze. Other receive checks, such as
address validation and bank transfer restrictions, still apply.

The bank message server checks `IsSendEnabledCoins` for user `MsgSend` and
`MsgMultiSend` messages. IBC receives instead use keeper `SendCoins` to
unescrow, or `MintCoins` and `SendCoinsFromModuleToAccount` to deliver vouchers;
these module flows do not invoke that user-message send-enabled gate.
`TestRepairM_1206_ReceiveGating` proves both branches and the error-acknowledgement
refund with the real bank keeper. Any per-denomination inbound policy requires
a separate domain-6 task with receive and refund proofs.

Neither transfer parameter gates the acknowledgement or timeout refund paths.
Disabling new sends or receives does not itself cancel packets already sent
or prevent their refunds. See [Acknowledgements](01_concepts.md#acknowledgements)
and [Events](05_events.md) for how these outcomes are represented.
