<!-- Source: https://docs.paxeer.app/payments-quickstart/ -->

# Your first payment

Quote, commit, and verify a Paxeer X payment using the application SDK or a signed ledger transaction.

## Choose your integration

Use the application SDK when you want account-level payment journeys and bearer-token authentication. Use the native CLI when you control the wallet identity and its EVM key and need explicit transaction signing, receipt policy, fee limits, and commitment selection. The CLI submits ledger calls as signed EIP-1559 transactions on chain ID 125.

## 1. Install the application packages

```
npm install @sidiora/layerx-sdk@0.1.0 @sidiora/layerx-buyer-middleware@0.1.0
```

The packages require Node.js 22 or newer. Their sources are `services/agent/sdk/typescript` and `services/platform/middleware/buyer`. In the sample under `services/platform/examples/samples/first-payment-typescript`, configure `LAYERX_API_URL` with your environment's API base URL and `LAYERX_API_TOKEN` with its account bearer token. Use HTTPS outside a loopback emulator. These credentials identify your API account; do not embed them in browser bundles or logs.

## 2. Quote and commit the payment

This SDK integration accepts your typed source, destination, and money values. Choose `paymentKey` from a persistent order identifier or request digest and retain it before committing.

```
import { LayerXPaymentHttpTransport } from "@sidiora/layerx-buyer-middleware";
import { ProductionClient, SecretBytes, idempotencyKey } from "@sidiora/layerx-sdk";

export const openLayerX = (apiUrl, apiToken) => new ProductionClient(
  new LayerXPaymentHttpTransport({
    baseUrl: apiUrl,
    bearerToken: new SecretBytes(new TextEncoder().encode(apiToken))
  })
);

export const pay = async (layerx, source, destination, money, paymentKey) => {
  const quote = await layerx.human("move.quote", { source, destination, money });
  return layerx.human(
    "move.commit",
    { quote_id: quote.quote_id },
    { idempotencyKey: idempotencyKey(paymentKey) }
  );
};
```

The quote establishes the payment terms, mechanism, fee ceiling, arrival expectation, and irreversible legs. Present these terms for approval before committing. An expired quote must be refreshed and approved again. Monetary quantities use integer base units rather than floating-point arithmetic.

## 3. Follow the original journey

Store the returned journey identifier alongside the purchase key. Read its progress through `journey.get` and resolve evidence references through `evidence.get`. A `layerx-receipt` reference identifies canonical receipt evidence. Finish only when the journey reaches `done` or `done-finalised` with verified evidence.

| Observed outcome | Next action |
| --- | --- |
| `waiting-for-you` | Obtain the requested decision before expiry. |
| `still-checking` | Track the existing journey; lock controls that could create a duplicate payment. |
| `refused` | Read refusal authority and `money_left`. |
| `done` / `done-finalised` | Verify evidence and record the settlement before fulfilment. |

## Native path: prepare a funded identity

The CLI uses a selected local identity, its matching owner-only EVM key export, and an independently supplied receipt policy. The policy requires protocol version 2, network 125, a receipt signer identity and key, and a bounded height range. Prepare the matching identity and EVM key with the wallet derivation and import workflow in the CLI guide. Receipt verification also relies on the configured node connection.

```
cargo build --locked --manifest-path tools/platform/cli/Cargo.toml
export PATH="$PWD/tools/platform/target/debug:$PATH"
layerx wallet list
layerx key default payer
```

In the examples below, `RPC_URL` is the node JSON-RPC root URL, and `GATEWAY_RPC_URL` is the hosted gateway root URL supplied by its operator. `GATEWAY_CREDENTIAL` names a locally stored gateway credential. These shell variables are sample inputs. Configure `RECEIPT_POLICY`, `EVM_KEY_FILE`, `FEE_LIMIT`, `ASSET_ID`, and `RECIPIENT_DID` for the selected wallet, fee ceiling and payment.

The selected wallet must hold a spendable balance of the chosen Asset, and its EVM sender must hold enough native currency for gas. `EVM_KEY_FILE` must be a private-key export from `wallet derive --export-private-keys` for the same identity. Confirm the source account and recipient account through the hosted gateway before sending.

## Open the recipient account and send

Select the recipient wallet to open its account for the chosen Asset, then select the funded payer wallet to transfer. The CLI reads the EVM sender nonce from the node and payment account state from the hosted gateway. The signing flow discloses the transaction and debit authorization, including recipient, amount, Asset, fee ceiling, expiry, network, and idempotency key.

```
# Run with the recipient wallet and its matching EVM_KEY_FILE selected.
layerx --rpc "$RPC_URL" --gateway-credential "$GATEWAY_CREDENTIAL" wallet open-account \
  --gateway-rpc "$GATEWAY_RPC_URL" --asset "$ASSET_ID" \
  --evm-key-file "$EVM_KEY_FILE" --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT" --wait committed

# Select the funded payer and set EVM_KEY_FILE to its matching export.
layerx key default payer
layerx --rpc "$RPC_URL" --gateway-credential "$GATEWAY_CREDENTIAL" wallet send \
  --gateway-rpc "$GATEWAY_RPC_URL" --evm-key-file "$EVM_KEY_FILE" \
  --to "$RECIPIENT_DID" --asset "$ASSET_ID" --amount 10 \
  --wait committed --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT"
```

Choose `submitted` for an accepted transaction hash, `receipted` for a verified signed execution receipt, or `committed` for receipt and committed-block evidence. Only `committed` establishes finality. The amount is an integer in the Asset's base units.

## Recover a pending native payment

Each new send creates a new payment idempotency key. The CLI journals the signed transaction before submission. Retain its transaction hash and use the journal-backed recovery flow when the response is uncertain. `wallet receipt` takes a transaction hash, with `--wait` and `--receipt-policy` as its options.

```
layerx --rpc "$RPC_URL" --gateway-credential "$GATEWAY_CREDENTIAL" wallet receipt \
  "$TX_HASH" --receipt-policy "$RECEIPT_POLICY" --wait committed
```

`--timeout-seconds` (1 to 300, default 60) belongs to `wallet watch` and to write commands such as `wallet send`, not to `wallet receipt`. Use the recovered receipt and block evidence to establish the payment outcome.

## Node and hosted JSON-RPC

The hosted gateway publishes `px_*` methods — including `px_register`, `px_getAccount`, `px_getBalances`, `px_listAssets`, and `px_requestFunds` — plus `pax_sequencerReceipt`. The node accepts signed transactions through `eth_sendRawTransaction` and reports receipt progress through `pax_sequencerReceipt`. The hosted gateway supplies account and asset reads at its separately configured origin. Use each endpoint for its declared methods and verify committed evidence before treating a payment as final.

## Handle common failures

| Code or condition | Response |
| --- | --- |
| `idempotency-required` | Supply the persistent purchase key before committing. |
| `idempotency-conflict` | Resolve the original request; do not reuse its key for a different body. |
| `rate-limit` | Respect the carried retry timing. |
| `unknown-outcome` | Recover the original payment by key and evidence. |
| `budget-refusal` | Inspect the funded budget and approved spending limits. |
| Invalid signature, stale sequence, missing scope, or evidence mismatch | Resolve the specific refusal before preparing any new payment. |

## Next steps

- [Payment lifecycle and merchant workflows](https://docs.paxeer.app/payments)

- [Create an Asset and manage accounts](https://docs.paxeer.app/assets)

- [Funding and money movement](https://docs.paxeer.app/funding)

- [Charge for HTTP resources](https://docs.paxeer.app/paid-apis)

- [Estimate fees and set limits](https://docs.paxeer.app/fees)

- [Public payment API reference](https://docs.paxeer.app/platform-api)
