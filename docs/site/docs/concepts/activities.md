# Activities

State changes on Paxeer mainnet-beta enter the merged network as signed
transactions. The node is `paxd`, the EVM chain id is `125` (`0x7d`), and
the kernel runs inside the network and participates in block proposal.
Execution and settlement share the chain's consensus boundary; a successful
payment is final when its block is committed.

## Admission and execution

For an EVM write, construct and sign an EIP-1559 (type 2) transaction with
the chain id, sender nonce, destination, ABI-encoded calldata, gas limit,
and fee caps. Submit its exact signed bytes with `eth_sendRawTransaction`
to a node's root JSON-RPC endpoint. The returned transaction hash identifies
the submission; it does not prove successful execution.

The precompile resolves the caller's authority and invokes the relevant
module through the embedded host. For example, `precompiles/layerxledger/`
delegates to `modules/layerxledger/keeper/`, while `modules/lxphost/` owns
the kernel integration. Module authorization, account sequences, expiry,
idempotency, and monetary checks still apply inside that path. An EVM nonce
and a module's account sequence are distinct values.

Execution can revert. A committed transaction with `status: "0x0"` is a
committed failure, not a successful transfer. Check the outcome and payment
facts as well as inclusion. Gas accounting and the nonce must be handled
according to the transaction result; do not infer them from an admission
acknowledgement.

## Activity type

Kernel activity types remain internal dispatch identifiers: the high 16 bits
name the module and the low 16 bits name its operation
(`include/layerx/lxp_module.h`). They are not JSON-RPC method names or EVM
addresses. Programs use module `9`.

The current EVM entry points include:

| Surface | Address | Contract source |
| --- | --- | --- |
| Ledger and exact payments | `0x000000000000000000000000000000000000101A` | `precompiles/layerxledger/abi.json` |
| Authority | `0x000000000000000000000000000000000000101C` | `precompiles/layerxauth/abi.json` |
| Programs | `0x000000000000000000000000000000000000101D` | `precompiles/layerxprograms/LayerXPrograms.sol` |

Use the published ABI for argument encoding. Some compatibility selectors
remain in an ABI while their implementation refuses the operation; payer
grants and draws, for example, are retired. The
[payments path](../overview/payments.md) describes supported exact payments
and the Programs token example.

## Public submission

The public node endpoints are `https://api-mainnet-beta.paxeer.network`
and `https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, at the root path. See
[Public JSON-RPC](../reference/public-rpc.md) for the node list.

The pre-merge `lx_sendActivity` production route is retired; hosted gateway
dispatch refuses every `lx_` method with JSON-RPC `-32601` (`Method not found`).
Hosted `px_` methods are declared separately in
`services/platform/hosted/gateway/openrpc.json`; see
[Gateway RPC](../platform/gateway-rpc.md). A hosted method's presence does
not imply that every public node serves it.

## Status

`rpc/sequencer_receipt.go` defines the response of
`pax_sequencerReceipt([txHash])`:

| Rung | Evidence returned | Meaning |
| --- | --- | --- |
| `submitted` | `rung`, `txHash`, `nonce` | The transaction is known, with no execution receipt yet. |
| `receipted` | Canonical signed `receipt` and its decoded execution fields | Execution has a signed result, without committed block inclusion. |
| `committed` | The same receipt plus `blockNumber` and `blockHash` | A committed block's sequence batch covers the receipt. |

An unknown hash returns `null`. A node without a receipt source or forwarding
configuration can refuse the method; that is unavailable evidence, not
success. `eth_getTransactionReceipt` reads the committed EVM receipt and does
not expose an early execution receipt as a mined transaction.

Authenticate receipt keys independently, including their authorized height
ranges. Verify the canonical signature, transaction hash and nonce, decoded
fields, return-data and log digests; a response's `signingKey` alone is not
a trust anchor. For commitment, also check matching block and transaction
inclusion. Notifications are unverified wake-ups: reconcile with verified
reads after a notification or stream closure. See
[Commitment levels](../protocol/commitment-levels.md) and
[Hosted internal](../operators/hosted-internal.md) for the event-delivery boundary.

## Legacy envelope compatibility

The canonical activity envelopes, LXC/1 encoding, module ordinals, and
older protocol-version rules remain useful when reading fixtures and kernel
compatibility code. Their reference is [Protocol](../protocol/index.md)
and the declarations in `include/layerx/lxp_activity.h` and
`include/layerx/lxp_module.h`. These archival envelope descriptions
are not instructions for public transaction submission. In particular, the
pre-merge execution/batch/checkpoint ladder is retired as a payment finality
recipe; use the receipt rungs above and committed chain inclusion.
