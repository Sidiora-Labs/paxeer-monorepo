# Rust SDK quickstart

Package: `layerx-sdk` at `services/agent/crates/layerx-sdk`.

This crate ships typed public JSON-RPC wrappers for `px_register`,
`px_requestFunds`, native `eth_sendRawTransaction` submission and
`px_getBalances` / `px_getNetwork` reads
(`services/agent/crates/layerx-sdk/src/rpc.rs`,
`services/agent/crates/layerx-sdk/src/register.rs`).

---

## Register

Connect to the configured hosted gateway with `RpcClient::connect_hosted`;
registration is a hosted operation. `RpcClient::register` takes a 32-byte
Ed25519 public key and its 64-byte signature over `layerx_sdk::register::binding`. The gateway
derives the subject; the caller cannot name another principal.

```rust
use layerx_sdk::register::{binding, TENANT};
use layerx_sdk::rpc::RpcClient;

fn register(rpc: &RpcClient, public_key: [u8; 32], signature: &[u8; 64])
    -> Result<layerx_sdk::register::Registration, layerx_sdk::rpc::RpcError>
{
    let _digest = binding(TENANT, &public_key);
    rpc.register(public_key, signature)
}
```

`TENANT` is the literal `"beta"`. `binding` / `subject` are the
same SHA-256 tagged constructions the gateway uses.

---

## Fund from faucet

`RpcClient::request_funds` calls `px_requestFunds` and accepts only a grant
confirmed as `funded`. A successful `FaucetGrant` exposes `funding_id`,
`transaction_id`, `amount`, and `network`.

The hosted operation requires `Authorization: Bearer <identity session token>`.
`RpcClient::connect` accepts `Option<LayerXKeyCredential>`, which attaches
`LayerX-Key`; it has no Bearer session constructor. Use HTTP for the hosted
claim with the required session header:

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
network's chain ID, nonce, gas limit and fees. Connect to the node root
(`/`), and submit the signed transaction bytes with
`RpcClient::send_raw_transaction` (`eth_sendRawTransaction`):

```rust
use layerx_sdk::rpc::{RpcClient, RpcError};

fn send(node_root: &str, signed_transaction: &[u8]) -> Result<[u8; 32], RpcError> {
    let rpc = RpcClient::connect(node_root, None)?;
    rpc.send_raw_transaction(signed_transaction)
}

fn read_state(node_root: &str, did: &str) -> Result<(), RpcError> {
    let rpc = RpcClient::connect(node_root, None)?;
    let _balances = rpc.get_balances(did)?;
    let _network = rpc.get_node_info()?;
    Ok(())
}
```

Signed bytes must be non-empty and at most 524_288 bytes. The returned
32-byte hash identifies the submission; it does not prove execution or
commit. `RpcClient::sequencer_receipt` reads `pax_sequencerReceipt` for that
hash. Its submitted, receipted and committed rungs describe progress; the
returned evidence still needs verification.

`get_balances` calls `px_getBalances`; `get_node_info` calls `px_getNetwork`.
Both read from the same node root. Read responses alone are not verified
receipts.

---

## Verify a receipt

```rust
use layerx_proof::receipt::AuthorizedBatch;
use layerx_sdk::production::verify_receipt;

fn verify(
    receipt: &[u8],
    batch_id: [u8; 32],
    asset: [u8; 32],
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    sequencer_public_key: [u8; 32],
) -> Result<layerx_proof::receipt::VerifiedReceipt, layerx_proof::receipt::VerificationFailure> {
    let authorised = AuthorizedBatch::new(
        batch_id,
        asset,
        previous_state_root,
        resulting_state_root,
        sequencer_public_key,
    );
    verify_receipt(receipt, &authorised)
}
```

This helper verifies kernel receipt bytes under independently authenticated
batch facts. It does not verify the node RPC receipt view or BFT commitment.
For native signed-transaction receipts, use `RpcClient::wait_for_with_commit`
with caller-pinned `ReceiptCommitEvidence`; its implementation is in
[`rpc_verification.rs`](../../services/agent/crates/layerx-sdk/src/rpc_verification.rs).

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
