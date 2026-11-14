# Finality

Finality is CometBFT block finality on Paxeer EVM chain ID `125`.
The LayerX kernel inside `paxd` executes and orders each transaction and builds
each block; validators check and commit it. A receipt is final when the
validators commit its block; its execution status separately records success or
failure.

A signed execution receipt alone does not establish block commitment.
Consumers verify inclusion and the BFT commit against independently trusted
chain and validator information when they require independent finality proof.

See the [Paxeer Network guide](../paxeer-network.md) for execution and finality,
and [Commitment levels](CommitmentLevels.md) for transaction progress.
