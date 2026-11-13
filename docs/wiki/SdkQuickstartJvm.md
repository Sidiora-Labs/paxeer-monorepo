# JVM SDK quickstart

Maven coordinates: `com.sidiora.layerx:layerx-sdk` at
`sdk/platform/jvm`. There is no public JSON-RPC client in this
package.

The native transaction examples below use HTTP directly. For hosted
human payments, a separate quote/commit example lives at
`services/platform/examples/samples/first-payment-jvm`.

---

## Register

No `px_register` wrapper. Catalog constant
`GeneratedSchema.AgentOperations.AGENT_REGISTER` (`"agent.register"`)
can be sent with `ProductionClient.agent` when
`HttpProductionTransport` is constructed with an agent RPC URL.
That operation registers an agent through the agent plane. Hosted
self-service principal registration uses `px_register`. Use
[Public JSON-RPC](PublicRpc.md) or the Rust SDK for self-service principal creation.

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

```java
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.LocalVerifier.ReceiptKeys;
import com.sidiora.layerx.sdk.verify.LocalVerifier.SequencerReceiptVerification;

public final class Verify {
    public static SequencerReceiptVerification verify(
            byte[] canonicalReceipt, byte[] rawTransaction, ReceiptKeys keys) {
        return LocalVerifier.verifySequencerReceipt(canonicalReceipt, rawTransaction, keys);
    }

    private Verify() {}
}
```

`ReceiptKeys` holds a list of `ReceiptKey` values with `keyId`, `publicKey`,
`firstHeight` and `lastHeight`; height bounds use `BigInteger` and are inclusive.
Authenticate the keys independently of the receipt response. Pass the exact
signed EVM transaction as `rawTransaction`.

The overload taking a `JsonNode` view and `previousSequenceHash` additionally
checks the RPC view and authenticated predecessor. Call `requireSuccess()` on
the result before treating execution as successful. Receipt authentication
alone does not prove block commitment; use the evidence and trust interfaces in
[`Committed.java`](../../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/Committed.java)
for that step.

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
