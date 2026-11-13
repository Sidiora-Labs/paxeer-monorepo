# Finality

Paxeer mainnet-beta uses CometBFT block finality on EVM chain ID `125`
(`0x7d`). The node's integrated kernel participates in proposal and execution.
A receipt is final when its block is committed.

A submitted transaction hash or an execution receipt alone does not prove
commitment. Verify the transaction's successful committed inclusion using the
[receipt evidence](commitment-levels.md) required by the application.

See the [Paxeer Network guide](../overview/unified-network.md) for the execution
and finality model.
