<!--
order: 5
-->

# Events

The following tables describe attributes emitted directly by the transfer
[message server](../keeper/msg_server.go),
[IBC callbacks](../ibc_module.go) and [receive keeper](../keeper/relay.go).
Core IBC and bank operations can emit additional events. Packet `denom` and
`amount` attributes are the packet's full denomination path and decimal amount
string; they need not be the receiver's bank denomination.

## MsgTransfer

After a successful send, the transfer message server emits:

| Type | Attribute Key | Attribute Value |
|------|---------------|-----------------|
| ibc_transfer | sender | `msg.Sender` |
| ibc_transfer | receiver | `msg.Receiver` |
| message | module | `transfer` |

For normal protobuf message-service execution, BaseApp also emits
`message.action = /ibc.applications.transfer.v1.MsgTransfer`. Its legacy message
execution path uses the message's `Type()`, `transfer`, instead. The transfer
keeper itself does not emit an `action` attribute.

## OnRecvPacket callback

The callback emits this event for both successful and unsuccessful receives,
including packet decoding failures. Fields reflect the decoded packet data;
fields unavailable after a decoding failure may be empty.

| Type | Attribute Key | Attribute Value |
|------|---------------|-----------------|
| fungible_token_packet | module | `transfer` |
| fungible_token_packet | sender | `data.Sender` |
| fungible_token_packet | receiver | `data.Receiver` |
| fungible_token_packet | denom | `data.Denom` |
| fungible_token_packet | amount | `data.Amount` |
| fungible_token_packet | memo | `data.Memo` |
| fungible_token_packet | success | `true` or `false`, from `ack.Success()` |

On the voucher-minting receive branch, the keeper also emits:

| Type | Attribute Key | Attribute Value |
|------|---------------|-----------------|
| denomination_trace | trace_hash | Uppercase hexadecimal `denomTrace.Hash()` |
| denomination_trace | denom | Receiver's voucher denomination, `ibc/{trace_hash}` |

The trace event is emitted on that branch even if the trace already exists.
It is not emitted on the unescrow branch. Use the receive `success` attribute
to determine the application outcome.

## OnAcknowledgePacket callback

After decoding and validating the acknowledgement and completing any required
refund, the callback emits:

| Type | Attribute Key | Attribute Value |
|------|---------------|-----------------|
| fungible_token_packet | module | `transfer` |
| fungible_token_packet | sender | `data.Sender` |
| fungible_token_packet | receiver | `data.Receiver` |
| fungible_token_packet | denom | `data.Denom` |
| fungible_token_packet | amount | `data.Amount` |
| fungible_token_packet | memo | `data.Memo` |
| fungible_token_packet | acknowledgement | `ack.String()` |

It then emits a second `fungible_token_packet` event with exactly one attribute:

| Acknowledgement variant | Attribute Key | Attribute Value |
|-------------------------|---------------|-----------------|
| Result | success | `string(resp.Result)` |
| Error | error | `resp.Error` |

The result attribute is the raw result bytes converted to a string. For an
ICS20 success this is the single byte `0x01`, not the text `true` used by the
receive callback. `ack.String()` is the generated protobuf string
representation, not the JSON acknowledgement bytes. An error acknowledgement
refunds the original sender; the `receiver` attribute still names the packet's
intended recipient. A decoding, canonical-encoding or refund failure returns an
error before these callback events are emitted.

## OnTimeoutPacket callback

After decoding the packet and successfully refunding its sender, the callback
emits the application event `timeout`:

| Type | Attribute Key | Attribute Value |
|------|---------------|-----------------|
| timeout | module | `transfer` |
| timeout | refund_receiver | `data.Sender` |
| timeout | refund_denom | `data.Denom` |
| timeout | refund_amount | `data.Amount` |
| timeout | memo | `data.Memo` |

This is distinct from the core IBC `timeout_packet` event. The refund recipient
is the sender, and `refund_denom` is the full packet denomination path. The bank
refund uses the corresponding native or `ibc/{hash}` denomination. See
[Acknowledgements](01_concepts.md#acknowledgements) for refund semantics and
[Metrics](06_metrics.md) for the separate telemetry observations.
