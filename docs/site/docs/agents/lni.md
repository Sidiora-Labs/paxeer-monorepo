# LayerX Node Interface (LNI)

LNI is retired. The canonical-binary node socket protocol and its `NodeInfo`,
batch-header and checkpoint handshake are historical interfaces. Current
agents use native chain JSON-RPC and committed-state evidence through
`NodeRpc` and `SequencerEndpoint`; do not configure an LNI socket or construct
a separate node-interface client from the old beta design.

Paxeer mainnet-beta is one BFT network with EVM chain id **125** (`0x7d`).
The kernel is part of the network and proposes blocks. A receipt is final
when its block is committed. Tenant domains are database namespaces, not
additional chains requiring new infrastructure.

## Node connection

Public JSON-RPC uses HTTPS at the root of
`https://api-mainnet-beta.paxeer.network/` or the numbered nodes
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`. The ordinary node namespaces are
`debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`.
The old router and /rpc, /rpc/ws and /rpc/schema paths are retired.
`node.hyperpaxeer.com` is only the library mirror, not an RPC endpoint.

`NodeRpc::connect` checks `eth_chainId` against configuration and applies
bounded request deadlines. Its environment configuration requires
`LAYERX_AGENT_NODE_RPC_URL` and `LAYERX_AGENT_NODE_CHAIN_ID`; the latter
accepts decimal or `0x` hexadecimal. These variables configure the node
client, not a hosted gateway deployment or every full-mode agent reader.
See [Agentd configuration](agentd.md#config-keys) for that distinction.

Sources: `services/agent/crates/layerx-agentd/src/node_rpc.rs`,
`services/agent/crates/layerx-client/src/sequencer.rs`.

## Native operations and evidence

| Operation | Current interface |
| --- | --- |
| Chain identity | `eth_chainId`, checked against the configured id |
| Pending nonce and call preparation | `eth_getTransactionCount`, `eth_estimateGas`, `eth_call` |
| Submit signed EIP-1559 bytes | `eth_sendRawTransaction` |
| Standard transaction receipt | `eth_getTransactionReceipt` |
| Signed native receipt and progress | `pax_sequencerReceipt` |
| Receipt-key discovery | `px_getReceiptKeys` from node state; the hosted gateway separately exports its operator-pinned key list loaded at startup |
| Blocks and head | `eth_getBlockByNumber`, `eth_blockNumber` |

The node registers native `px_` reads, including `px_getReceiptKeys`,
`px_getCommit`, `px_getValidators` and `px_getSequenceBatch`. Agent readiness
consumes node-produced key discovery. The hosted gateway has a separate
contract and authentication policy; registration and funding require its
identity and faucet services.

A submission hash establishes `submitted`, a verified signed receipt
establishes `receipted`, and a verified CometBFT commit at the receipt height
establishes `committed`. Admission is not execution. Published signing-key
intervals must be checked under the caller's trust policy; a key supplied
beside a receipt is not independent authority. Consensus chain identity and
validators are separate trust inputs. Storage witnesses and precompile
results must bind to the authenticated committed state.

Sources: `rpc/sequencer_receipt.go`,
`services/agent/crates/layerx-agentd/src/{boot,finality}.rs`,
`services/agent/crates/layerx-agentd/src/read/{checkpoint,committed}.rs`.
The [Agentd evidence guide](agentd.md#protocol-evidence) describes readiness,
verification refusals and recovery in more detail.

## Hosted path

The hosted gateway is an optional application boundary in
`services/platform/hosted/gateway`, with its method contract in
`services/platform/hosted/gateway/openrpc.json`. It declares authenticated
activity submission, scoped subscriptions and native reads, plus selected
EVM and native-receipt forwarding. It is not an LNI transport and does not
create another network or another finality event.

Check the gateway's capabilities and method contract before relying on a
surface. In particular, its declared EVM forwarding refuses transactions to
some native precompiles with `-32003 surface_unavailable`; a direct node call
and a hosted call are not interchangeable. Missing backends or evidence must
produce an explicit refusal rather than fabricated balances or finality.

MCP uses an authenticated agent session, not the retired LNI boundary.
See [MCP](mcp.md), [Agentd](agentd.md) and the
[Hosted agent boundary](../platform/agent-boundary.md) for the respective
client, daemon and hosted surfaces.

[Home](../index.md)
