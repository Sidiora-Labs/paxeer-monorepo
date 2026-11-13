# Network finality

Paxeer mainnet-beta uses EVM chain ID `125` (`0x7d`). The kernel is part of
`paxd` and participates in block proposal. A receipt is final when its block
is committed by BFT consensus; execution status determines whether the
transaction succeeded.

See the [node operator guide](../platform/hosted-node.md) and
[payments guide](../overview/payments.md) for the current operating and
settlement model.
