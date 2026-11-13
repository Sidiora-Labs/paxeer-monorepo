<!-- Source: https://docs.paxeer.app/payments-quickstart/ -->

# Your first payment

Quote, commit, and verify a Paxeer X payment using the application SDK or a native Asset transfer.

## Choose your integration

Use the application SDK when you want account-level payment journeys and bearer-token authentication. Use the native CLI when you control a signing identity and need explicit canonical activities, receipt policy, fee limits, and commitment selection. The LayerX package and command names are the integration interfaces of Paxeer X.

## 1. Install the application packages

```
npm install @sidiora/layerx-sdk @sidiora/layerx-buyer-middleware
```

Configure `LAYERX_API_URL` with your environment's API base URL and `LAYERX_API_TOKEN` with its account bearer token. Use HTTPS outside a loopback emulator. These credentials identify your API account; do not embed them in browser bundles or logs.

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

The CLI uses your local key, a stored gateway credential, and a receipt policy supplied independently of the RPC response you verify. A policy binds protocol version, network, sequencer identity and key, permitted batches, and checkpoint context. Keep private seeds out of shell history.

```
cargo build --manifest-path tools/platform/cli/Cargo.toml
export PATH="$PWD/tools/platform/target/debug:$PATH"
layerx key create alice
layerx key default alice
layerx wallet list

export RPC_URL=https://api-mainnet-beta.paxeer.network/
export RECEIPT_POLICY=/absolute/path/to/receipt-policy.json
export EVM_KEY_FILE=/absolute/path/to/evm-key-export
export FEE_LIMIT='<maximum fee in base units>'
export ASSET_ID='<64-hex Asset id>'
export RECIPIENT_DID='<recipient DID>'
```

Public JSON-RPC endpoints are HTTPS URLs at the root path. The old `/rpc` and `/rpc/ws` routes are retired. Note the current checkout still has CLI transport gaps: setting `--rpc` to a correct public root URL does not by itself make wallet commands work against a public node. Treat the native commands below as the command surface and verify connectivity in your environment before relying on them.

The OS keyring stores keys by default; headless environments use the configured encrypted file store. The payer must already hold a spendable balance on the merged network before sending. Do not fund through the former custody precompile path: the custody module is retired, its keeper refuses deposits, and the precompile reverts. Confirm the account balance before spending.

## Open the recipient account and send

Select the recipient wallet to open its account for the chosen Asset, then select the funded payer wallet to transfer. Identity sequence and source-account sequence are obtained separately. The signing flow discloses the debit authorization and outer activity, including recipient, amount, Asset, fee ceiling, expiry, network, and idempotency key.

```
# Run with the recipient wallet selected.
# open-account requires --evm-key-file, --receipt-policy, and --fee-limit.
layerx --rpc "$RPC_URL" --gateway-credential beta wallet open-account \
  --asset "$ASSET_ID" --evm-key-file "$EVM_KEY_FILE" \
  --receipt-policy "$RECEIPT_POLICY" --fee-limit "$FEE_LIMIT"

# Select the funded payer before sending.
layerx key default alice
layerx --rpc "$RPC_URL" --gateway-credential beta wallet send \
  --to "$RECIPIENT_DID" --asset "$ASSET_ID" --amount 10 \
  --wait executed --receipt-policy "$RECEIPT_POLICY" \
  --fee-limit "$FEE_LIMIT"
```

Choose `executed` for a verified execution receipt, `batched` for authenticated batch inclusion, or `finalised` for matching finalised checkpoint evidence. The exact wire spelling is `finalised`.

## Recover a pending native payment

Each new CLI write creates a new idempotency key. If the first attempt times out, save its transaction hash and recover the original receipt instead of repeating `wallet send`. `wallet receipt` takes a transaction hash, with `--wait` and `--receipt-policy` as its options.

```
layerx --rpc "$RPC_URL" --gateway-credential beta wallet receipt \
  "$TX_HASH" --receipt-policy "$RECEIPT_POLICY" --wait finalised
```

`--timeout-seconds` (1 to 300, default 60) belongs to `wallet watch` and to write commands such as `wallet send`, not to `wallet receipt`. Stream notifications are wake-ups: reconcile them with verified reads or a receipt verification.

## Public JSON-RPC surface

The hosted gateway publishes `px_*` methods — including `px_register`, `px_getAccount`, `px_getBalances`, `px_listAssets`, and `px_requestFunds` — plus `pax_sequencerReceipt`. Every legacy `lx_*` method name is refused wholesale with JSON-RPC `-32601` (`Method not found`). Do not build submission or recovery flows on `lx_sendActivity`, `lx_getActivityStatus`, `lx_getReceipt`, or `lx_getProof`; no such public transport exists in this checkout. Use the SDK, the CLI, or the published `px_*` and node namespaces instead, and verify recovered evidence before treating a payment as settled.

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
