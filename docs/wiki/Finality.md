# Finality

Finality is CometBFT block finality on Paxeer EVM chain ID `125`.
The kernel runs inside `paxd` and proposes blocks. A receipt is final when its
block is committed; its execution status separately records success or failure.

A signed execution receipt alone does not establish block commitment.
Consumers verify inclusion and the BFT commit against independently trusted
chain and validator information when they require independent finality proof.

See the [Paxeer Network guide](../paxeer-network.md) for execution and finality,
and [Commitment levels](CommitmentLevels.md) for transaction progress.
