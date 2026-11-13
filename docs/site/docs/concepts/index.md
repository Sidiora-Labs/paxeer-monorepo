# Concepts

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).
The LayerX kernel runs inside `paxd` and participates in execution and block
proposal. A receipt is final when its block is committed; a committed failure
does not establish a successful payment.

Native ledger operations, bank transfers and EVM execution apply monetary
effects through their respective chain state paths. Kernel execution uses
canonical encodings and deterministic arithmetic. See
[Protocol](../protocol/index.md) for the kernel's encoding and execution rules.

| Topic | Page |
| --- | --- |
| Identity, keys, grants | [Identity](identity.md) |
| Named accounts and sequences | [Accounts](accounts.md) |
| One secret for the EVM account and the LayerX identity | [One account](one-account.md) |
| Assets, issuance, fees | [Assets](assets.md) |
| Transactions and kernel activities | [Activities](activities.md) |
| Receipts and verification | [Receipts](receipts.md) |
| Committed block evidence | [Checkpoints](checkpoints.md) |
| Native payment settlement | [Paxeer settlement](paxeer-settlement.md) |
| Node interfaces and trust | [Paxeer boundary](paxeer-boundary.md) |
