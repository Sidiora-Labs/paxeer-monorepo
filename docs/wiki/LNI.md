# LayerX Node Interface (LNI)

LNI is retired. Its canonical-binary frames, version negotiation, message
tags, and NodeInfo handshake are historical interfaces. Current agents use
native chain JSON-RPC through `NodeRpc` and `SequencerEndpoint`.

Sources: `services/agent/crates/layerx-agentd/src/node_rpc.rs`,
`services/agent/crates/layerx-client/src/sequencer.rs`,
`rpc/sequencer_receipt.go`.

---

## Node connection

`NodeRpc::connect` checks `eth_chainId` against the configured chain id and
bounds requests with a deadline. Configure `LAYERX_AGENT_NODE_RPC_URL` and
`LAYERX_AGENT_NODE_CHAIN_ID`; the latter accepts decimal or hexadecimal
with a `0x` prefix. Paxeer mainnet-beta uses EVM chain id 125 (`0x7d`).

The client implementation is
`services/agent/crates/layerx-agentd/src/node_rpc.rs`.

---

## Current operations

| Operation | JSON-RPC method |
| --- | --- |
| Chain identity | `eth_chainId` |
| Pending nonce | `eth_getTransactionCount` |
| Gas estimation and call preparation | `eth_estimateGas`, `eth_call` |
| Signed transaction submission | `eth_sendRawTransaction` |
| Signed native receipt | `pax_sequencerReceipt` |
| Receipt-key discovery | `px_getReceiptKeys` |
| Blocks and head | `eth_getBlockByNumber`, `eth_blockNumber` |

`SequencerEndpoint` implements these native reads and submissions in
`services/agent/crates/layerx-client/src/sequencer.rs`. The native receipt
producer is `rpc/sequencer_receipt.go`.

A submission hash acknowledges submission. Receipt verification binds the
execution result to that transaction; commitment requires consensus evidence
at the receipt height. The agent verifies committed-state evidence through
`services/agent/crates/layerx-agentd/src/finality.rs` and
`services/agent/crates/layerx-agentd/src/read/committed.rs`.

Transport failures report unavailability. A timed-out submission has an
unknown outcome and must be reconciled by its receipt before another attempt.

---

## Start here

- [Protocol](Protocol.md)
- [Programs](Programs.md)
- [Agent API](AgentApi.md)
- [Agentd](Agentd.md)
- [Fees](Fees.md)
