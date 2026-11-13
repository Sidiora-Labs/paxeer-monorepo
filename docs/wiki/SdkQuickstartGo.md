# Go SDK quickstart

Module: `github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go`
at `sdk/platform/go`. There is no public JSON-RPC client in this
module.

---

## Register

No `px_register` wrapper. The agent catalog constant
`AgentOperationAgentRegister` (`"agent.register"`) can be sent
with `Client.Agent` **if** you supply a `Transport` that implements
the agent plane. `NewAgentEnvelopeHTTPTransport` in
[`http.go`](../../sdk/platform/go/http.go) supplies the hosted agent-envelope
transport with a gateway authorizer and optional agent session credential.
`NewHumanHTTPTransport` serves human operations. Agent registration does not
invoke the hosted self-service `px_register` method.

Use [Public JSON-RPC](PublicRpc.md) or the Rust SDK.

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

```go
package main

import layerx "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"

func verify(receiptJSON, rawTransaction []byte, keys layerx.ReceiptKeys) (layerx.SequencerReceiptVerification, error) {
	return layerx.VerifySequencerReceipt(receiptJSON, rawTransaction, keys)
}
```

`receiptJSON` is the JSON result object from `pax_sequencerReceipt`, including
its canonical signed `receipt` field. `rawTransaction` is the exact signed EVM
transaction. `ReceiptKeys` contains `Pins` and `PreviousSequenceHash`;
each `ReceiptKey` carries `KeyID`, `PublicKey`, `FirstHeight` and `LastHeight`.
Authenticate the pins and predecessor independently of the response.

The verifier checks the signature, transaction and JSON bindings, logs and
sequence continuity. Inspect `Status` and `Revert` for execution success.
`VerifyCommittedReceipt` additionally verifies block inclusion and consensus
commitment using caller-supplied evidence and trust. See
[`verify.go`](../../sdk/platform/go/verify.go) and
[`committed.go`](../../sdk/platform/go/committed.go).

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
