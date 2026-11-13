<!--
order: 3
-->

# State Transitions

The [transfer keeper](../keeper/relay.go) chooses the source/sink branch relative
to the packet's channel. See [Denomination Trace](01_concepts.md#denomination-trace).

## Send Fungible Tokens

If the full denomination does not begin with the sending port/channel prefix,
the sender escrows the local coins in the channel's escrow account. Otherwise,
the sender burns its vouchers. The outgoing packet carries the full denomination
trace and requested amount; the IBC core stores the packet commitment.

## Receive Fungible Tokens

If the packet denomination begins with its source port/channel prefix, the
receiver removes that prefix and unescrows the local coins from its destination
channel's escrow account to the recipient.

Otherwise, the receiver prepends its destination port/channel pair, stores the
trace if absent, mints the corresponding `ibc/{hash}` vouchers and sends them
to the recipient. Successful processing returns an acknowledgement.

## Acknowledgements and Timeouts

A success acknowledgement completes the transfer without a refund. An error
acknowledgement or a proven timeout refunds the original sender: escrowed coins
are released, or burned vouchers are re-minted. The core deletes the sender's
packet commitment after processing. See [Acknowledgements](01_concepts.md#acknowledgements)
and [Events](05_events.md).
