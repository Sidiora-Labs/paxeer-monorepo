# Commitment levels

The node reports `submitted`, `receipted` and `committed` through
`pax_sequencerReceipt` (`rpc/sequencer_receipt.go`).

| Rung | Meaning |
| --- | --- |
| `submitted` | The transaction is known; execution evidence is pending. |
| `receipted` | A signed execution receipt is available for verification under independently pinned receipt keys. |
| `committed` | The receipt identifies the committed block containing the transaction. |

Execution success and commitment are separate facts. Status `0x0` records
failed execution; status `0x1` records success. Authenticate the receipt and
block evidence before relying on a reported rung.

For payment evidence requirements and recovery, see
[Commitment levels](../site/docs/protocol/commitment-levels.md).
See [Public JSON-RPC](PublicRpc.md) for node and hosted API ownership.

[Home](Home.md)
