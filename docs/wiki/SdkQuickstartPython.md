# Python SDK quickstart

Package: `layerx-sdk` at `services/agent/sdk/python`. `PaymentRpc` in
`layerx_sdk.x402_rpc` submits native signed transactions and reads their
sequencer and committed receipts. Its public `call` accepts `eth_` and `pax_`
JSON-RPC methods. `PxClient` in `layerx_sdk.px` provides typed `px_` read helpers.

---

## Register

Use `HostedGatewayRpc` from `layerx_sdk.x402_rpc` for `px_register` on
the configured hosted gateway:

```python
from layerx_sdk.x402_rpc import HostedGatewayRpc


def register(rpc: HostedGatewayRpc, signer_public_key: str, registration_signature: str) -> dict:
    result = rpc.call("px_register", [signer_public_key, registration_signature])
    if not isinstance(result, dict):
        raise ValueError("invalid-registration-response")
    return result
```

Both arguments are lowercase hex as required by
[Public JSON-RPC](PublicRpc.md). Binding-digest construction is not
in this package; the Rust crate's `register::binding` is the typed
helper for that.

---

## Fund from faucet

Call the hosted `px_requestFunds` operation using `HostedGatewayRpc.call` with
an identity session in the `Authorization` header:

```python
from layerx_sdk.x402_rpc import HostedGatewayRpc


def request_funds(
    hosted_endpoint: str, session_token: str, did: str, signer_public_key: str
) -> dict:
    rpc = HostedGatewayRpc(
        hosted_endpoint,
        {"Authorization": f"Bearer {session_token}"},
    )
    result = rpc.call("px_requestFunds", [did, signer_public_key])
    if not isinstance(result, dict):
        raise ValueError("invalid-faucet-response")
    return result
```

Use the configured hosted gateway endpoint, a DID, and a 32-byte signer
public key encoded as lowercase hex. This authenticated faucet request is
separate from native transaction submission at the node root.
Both RPC clients require HTTPS except for loopback. `HostedGatewayRpc`
requires `/rpc` on the configured hosted service; this is not a public node
endpoint. `PaymentRpc` requires the node root.
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
`HostedGatewayRpc` instance for the hosted faucet's Bearer session.

---

## Verify a receipt

Install the package's `derivation` extra for its `cryptography` dependency.

```python
from layerx_sdk import ReceiptKey, SequencerReceiptVerification, verify_sequencer_receipt


def verify(
    canonical_receipt: bytes,
    raw_transaction: bytes,
    trusted_keys: list[ReceiptKey],
) -> SequencerReceiptVerification:
    return verify_sequencer_receipt(canonical_receipt, raw_transaction, trusted_keys)
```

Each `ReceiptKey` carries `key_id`, `public_key`, `first_height` and
`last_height`. Authenticate the pins independently of the response. The verifier
checks the canonical receipt signature against a pin active at its height and
binds its activity ID to the exact signed transaction's Keccak-256 hash.

The returned level is `sequencer-signed`. Check `receipt.result_code` for
execution success; a signature does not prove block commitment. The
[`CommitTrust`](../../services/agent/sdk/python/layerx_sdk/committed.py)
interface verifies committed transaction inclusion using independently pinned
consensus trust and caller-supplied evidence.

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
