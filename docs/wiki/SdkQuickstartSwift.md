# Swift SDK quickstart

SwiftPM product: `LayerXSDK` at `sdk/platform/swift`. There is no
public JSON-RPC client in this package.

---

## Register

No `px_register` wrapper. Generated
`PlatformClient.agentAgentRegister` / `mutate(.agentAgentRegister, …)`
selects the catalog entry `"agent:agent.register"`, whose wire operation
name is `"agent.register"`. `AgentHTTPTransport` refuses calls;
`AgentEnvelopeTransport` implements hosted agent-envelope calls. Use [Public JSON-RPC](PublicRpc.md) or the Rust SDK
for self-service principal creation.

---

## Fund from faucet

Send `px_requestFunds` to the configured hosted gateway with an identity
session. Set `HOSTED_GATEWAY_URL`, `SESSION_TOKEN`, `DID`, and
`SIGNER_PUBLIC_KEY` (the 32-byte public key as lowercase hex):

```sh
curl --fail-with-body "$HOSTED_GATEWAY_URL" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"px_requestFunds\",\"params\":[\"$DID\",\"$SIGNER_PUBLIC_KEY\"]}"
```

This is the hosted faucet operation; it is separate from node transaction
submission. See [Hosted faucet](HostedFaucet.md).

---

## Send

Build and sign a native EVM transaction with your wallet, using the target
network's chain ID, nonce, gas limit and fees. Set `RAW_TRANSACTION` to its
`0x`-prefixed signed bytes and `NODE_RPC_URL` to the node root (`/`). Submit
with `eth_sendRawTransaction`:

```sh
curl --fail-with-body "$NODE_RPC_URL" \
  -H 'Content-Type: application/json' \
  --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_sendRawTransaction\",\"params\":[\"$RAW_TRANSACTION\"]}"
```

The result is a transaction hash, not a verified receipt. Set `TX_HASH` to
that hash and read its committed transaction receipt at the same node root:

```sh
curl --fail-with-body "$NODE_RPC_URL" \
  -H 'Content-Type: application/json' \
  --data "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$TX_HASH\"]}"
```

A null receipt is pending. Read account balances with `px_getBalances`
using `DID`, and network state with `px_getNetwork`, at that same root:

```sh
curl --fail-with-body "$NODE_RPC_URL" \
  -H 'Content-Type: application/json' \
  --data "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"px_getBalances\",\"params\":[\"$DID\"]}"
curl --fail-with-body "$NODE_RPC_URL" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":4,"method":"px_getNetwork","params":[]}'
```

These wire methods are used by `services/agent/crates/layerx-sdk/src/rpc.rs`.
This package has no public JSON-RPC wrapper; use an HTTP client as above.

---

## Verify a receipt

```swift
import Foundation
import LayerXSDK

func verify(
    canonicalReceipt: Data,
    rawTransaction: Data,
    keys: [ReceiptKeys],
    receiptJSON: [String: JSONValue],
    previousSequenceHash: Data
) throws -> SequencerReceiptVerification {
    try LocalVerifier.verifySequencerReceipt(
        canonicalReceipt, rawTransaction: rawTransaction, keys: keys,
        receiptJSON: receiptJSON, prevSeqHash: previousSequenceHash)
}
```

Each `ReceiptKeys` value has `keyId`, `publicKey`, `firstHeight` and
`lastHeight`. Authenticate these inclusive height-bounded pins and the
previous sequence hash independently of the response. Retain the exact signed
EVM transaction and the decoded `pax_sequencerReceipt` result object.

The verifier checks the signature, transaction, JSON fields, logs and sequence
continuity. Inspect `status` and `revert` for execution success. The returned
level is `receipted`; reported block coordinates alone do not prove consensus
commitment. See [`Committed.swift`](../../sdk/platform/swift/Sources/LayerXSDK/Committed.swift)
for the committed-evidence and trust interfaces.

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
