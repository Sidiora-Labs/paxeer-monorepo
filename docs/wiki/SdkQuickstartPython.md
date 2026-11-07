# Python SDK quickstart

Package: `layerx-sdk` at `services/agent/sdk/python`. `PaymentRpc` in
`layerx_sdk.x402_rpc` submits native signed transactions and reads their
sequencer and committed receipts. Its public `call` sends generic JSON-RPC
requests. `PxClient` in `layerx_sdk.px` provides typed `px_` read helpers.

---

## Register

No `register()` helper. `PaymentRpc.call` can send `px_register`:

```python
from layerx_sdk import PaymentRpc

def register(rpc: PaymentRpc, signer_public_key: str, registration_signature: str) -> dict:
    return rpc.call("px_register", [signer_public_key, registration_signature])
```

Both arguments are lowercase hex as required by
[Public JSON-RPC](PublicRpc.md). Binding-digest construction is not
in this package; the Rust crate's `register::binding` is the typed
helper for that.

---

## Fund from faucet

Call the hosted `px_requestFunds` operation using `PaymentRpc.call` with
an identity session in the `Authorization` header:

```python
from layerx_sdk import PaymentRpc


def request_funds(
    hosted_endpoint: str, session_token: str, did: str, signer_public_key: str
) -> dict:
    rpc = PaymentRpc(
        hosted_endpoint,
        {"Authorization": f"Bearer {session_token}"},
        hosted_gateway_endpoint=hosted_endpoint,
    )
    result = rpc.call("px_requestFunds", [did, signer_public_key])
    if not isinstance(result, dict):
        raise ValueError("invalid-faucet-response")
    return result
```

Use the configured hosted gateway endpoint, a DID, and a 32-byte signer
public key encoded as lowercase hex. This authenticated faucet request is
separate from native transaction submission at the node root.
`PaymentRpc` requires HTTPS except for loopback; the explicit
`hosted_gateway_endpoint` permits the configured hosted path.
See [Hosted faucet](HostedFaucet.md).

---

## Send

Build and sign a native EVM transaction with your wallet, using the target
network's chain ID, nonce, gas limit and fees. Pass its `0x`-prefixed signed
bytes to `PaymentRpc.send`, which calls `eth_sendRawTransaction` at the
node root (`/`):

```python
from layerx_sdk import PaymentRpc, PxClient


def send(node_root: str, signed_transaction: str) -> str:
    return PaymentRpc(node_root).send(signed_transaction)


def read_state(node_root: str, account: str):
    px = PxClient(node_root)
    return px.get_account(account), px.get_balances(account), px.get_network()
```

`send` returns a transaction hash, not a receipt, and takes no commitment
argument. `PaymentRpc.receipt(transaction_hash)` reads
`pax_sequencerReceipt`; `PaymentRpc.status(transaction_hash)` reads
`eth_getTransactionReceipt`. A null receipt is pending. Sequencer rungs
are submitted, receipted and committed; receipt evidence must still be
verified before it is trusted.

The `PxClient` calls above use `px_getAccount`, `px_getBalances` and
`px_getNetwork` from `layerx_sdk/px.py` at the same node root. These are
state reads, not submission or receipt verification. Use a separate
`PaymentRpc` instance for the hosted faucet's Bearer session.

---

## Verify a receipt

```python
from layerx_sdk import (
    AuthorizedReceiptBatch,
    LocalSignatureVerifier,
    ReceiptVerification,
    verify_receipt,
)

def verify(
    canonical_receipt: bytes,
    authorized_batch: AuthorizedReceiptBatch,
    signatures: LocalSignatureVerifier,
) -> ReceiptVerification:
    return verify_receipt(canonical_receipt, authorized_batch, signatures)
```

`AuthorizedReceiptBatch` fields: `batch_id`, `asset`,
`previous_state_root`, `resulting_state_root`,
`sequencer_public_key`. `LocalSignatureVerifier` is a `Protocol`
with `verify_ed25519` and `verify_recoverable_secp256k1`. This
package does not ship a default implementation of that protocol;
the caller supplies one. Those batch facts must come from a source
you already trust.

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
