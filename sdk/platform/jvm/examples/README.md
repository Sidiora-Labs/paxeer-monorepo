# Runnable JVM examples

These sources use the APIs and configuration boundaries described in the
[JVM README](../README.md#network-and-api-configuration). Java examples are
standalone sources; the Kotlin journey uses the SDK package and its Kotlin
extensions.

## Origins and credentials

`LAYERX_NODE_RPC` is the root EVM JSON-RPC origin from the
[public RPC reference](../../../../docs/site/docs/reference/public-rpc.md).
The payment checks `eth_chainId` equals `0x7d` before signing. It sends no service bearer or owner session to the
node. Origins must have no path beyond `/`, credentials, query or fragment; plaintext HTTP
is accepted only for local loopback connections.

`LAYERX_HUMAN_ORIGIN` and `LAYERX_AGENT_ORIGIN` are independently configured
authenticated service origins obtained from your deployment. A public node
origin does not establish either service. The SDK selects `/v1/agent/rpc` on the
Agent origin and encodes an operation envelope, rather than a node JSON-RPC
method. `LAYERX_API_KEY` is the deployment bearer token. Obtain an authorised
owner session from that deployment and configure `LAYERX_TENANT`,
`LAYERX_SESSION_ID`, `LAYERX_SESSION_TOKEN_ID` and
`LAYERX_SESSION_GENERATION`. Session and token IDs are 64 lowercase hex
characters; generation is a canonical unsigned decimal string. Never log them.

Agent operations use the v2 service envelope selected by
`HttpProductionTransport`; configure the Agent service for that contract.

## FirstPaymentExample

This example sends **native PAX**, in integer base units, directly through EVM
RPC. Configure:

| Variable | Value |
| --- | --- |
| `LAYERX_NODE_RPC` | Root node origin |
| `LAYERX_EVM_KEY_FILE` | Private local file containing 64 hex characters, without `0x`, for the sender's secp256k1 key |
| `LAYERX_RECIPIENT` | Nonzero lowercase `0x` EVM address |
| `LAYERX_AMOUNT` | Positive integer PAX base units |
| `LAYERX_MAX_FEE_PER_GAS` | Nonnegative integer fee cap in base units |
| `LAYERX_PRIORITY_FEE_PER_GAS` | Nonnegative integer priority fee, at most the fee cap |
| `LAYERX_RECEIPT_PINS` | Independently provisioned pin file described below |
| `LAYERX_PAYMENT_JOURNAL` | Private persistent directory for signed transactions |

The typed preparation obtains the pending nonce and gas estimate, then builds
and signs an EIP-1559 native transfer with chain ID 125 and an empty access list.
A cryptographically random 32-byte key, printed as 64 lowercase hex characters,
names its recovery file. EVM transactions have no Agent idempotency-key field;
the transaction hash and nonce provide the on-chain identity. The example writes
and flushes the exact signed bytes to `<key>.raw` with mode `0600` before broadcast.
Keep the directory on durable storage with a single writer per account. This
short example does not implement concurrent nonce allocation or a payment queue.

After `eth_sendRawTransaction`, it asks for `pax_sequencerReceipt`, verifies the
signed receipt and RPC view against the retained transaction and independent
key pins, requires success, and matches the signed native asset, sender,
recipient and amount. A `receipted` signature is not a proof of block commitment.

Pending receipts and mismatched broadcast hashes raise `unknown-outcome`.
Transport failures also leave the journal intact. Reconcile the retained hash
with the node and verify its receipt before retrying. Never generate a new key
or transaction automatically after an uncertain outcome. The offline example
can verify a subsequently obtained portable bundle against the same `.raw` file.

## OfflineVerificationExample

The command-line arguments are:

```text
bundle.json transaction.raw independent-pins.json [independent-consensus.json [transitions.json]]
```

`bundle.json` is the unmodified `layerx-receipt-proof-v2` producer document;
`transaction.raw` is the binary signed EIP-1559 transaction retained at submission.
There are no zero-filled sample bytes or implicit trust keys. The pin file is a
JSON array provisioned independently of the bundle:

```text
[{"keyId":"0x<64 lowercase hex>","publicKey":"0x<64 lowercase hex>",
  "firstHeight":"<unsigned decimal>","lastHeight":"<unsigned decimal>"}]
```

Use actual nonzero keys and the authorised inclusive height range. Do not copy
trust from `receiptKey` in an untrusted bundle. For committed evidence supply
an independently pinned consensus document with `chainId` (the consensus chain
string), `height` (positive decimal string), and `validatorsHash` (`0x` plus
64 hex characters). Optional transitions are a JSON array of objects with
`header`, `commit`, and `validatorSet` in the production `Committed` wire shapes.
Supply every step from the trusted height to the execution height. A bundle at
that height needs an empty list. The verifier also authenticates the successor
header proving execution results. Missing trust, altered signatures, changed
transactions, reverts and mismatched evidence are errors.

## ServiceLifecycleExample (Kotlin)

In addition to the service origins, bearer and owner session above, configure
`LAYERX_AGENT_DID`, `LAYERX_ASSET`, `LAYERX_BUDGET_LIMIT` (integer base units),
and `LAYERX_BUDGET_EXPIRY_SECONDS` (absolute protocol expiry in decimal seconds).
This journey calls the typed Java `BUDGET_CREATE` overload from Kotlin and
generates a canonical random idempotency key. It retains credentials until the
asynchronous call finishes and then clears their owned secret bytes. Retain the
printed key for reconciliation; do not retry an unknown mutation with a new key.

It creates a service-side `DaemonLimit` through the Agent API. Service refusals
propagate as typed SDK exceptions.
