# Commitment levels (retired)

The former `executed`, `batched` and `finalised` submission contract is retired.
`lx_sendActivity`, `lx_getActivityStatus`, `lx_getReceipt` and the old `lx_`
proof/status methods are retired; the node and gateway return `-32601`
(Method not found) for them.
The separate sequencer service and guarantor finality model are retired.

For current node receipt evidence, payment commitment requirements and recovery,
see [Commitment levels](../site/docs/protocol/commitment-levels.md).
See [Public JSON-RPC](PublicRpc.md) for node and hosted API ownership.

[Home](Home.md)
