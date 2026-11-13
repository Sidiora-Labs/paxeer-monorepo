# TypeScript quickstart

Add a payment to a Node.js application. Eight lines, no protocol vocabulary, no key handling.

## Before you start

Use Node.js 22 or newer. Configure an authenticated human-payment service that supports
`/v1/moves/quote`, `/v1/moves` and journey reads. The local emulator retires
these payment-write routes. For native node transactions, follow the
[reference applications](../guide/reference-applications.md).

```sh
npm install @sidiora/layerx-sdk @sidiora/layerx-buyer-middleware
```

Set two values in your environment. Both are given to you; neither is signing material.

| Variable | What it is |
|---|---|
| `LAYERX_API_URL` | The base URL of your human-payment service |
| `LAYERX_API_TOKEN` | A bearer token identifying your account |

## The integration

```js sample=first-payment-typescript
import { LayerXPaymentHttpTransport } from "@sidiora/layerx-buyer-middleware";
import { ProductionClient, SecretBytes, idempotencyKey } from "@sidiora/layerx-sdk";
export const openLayerX = (apiUrl, apiToken) => new ProductionClient(
  new LayerXPaymentHttpTransport({ baseUrl: apiUrl, bearerToken: new SecretBytes(new TextEncoder().encode(apiToken)) }));
export const pay = async (layerx, source, destination, money, paymentKey) => {
  const quote = await layerx.human("move.quote", { source, destination, money });
  return layerx.human("move.commit", { quote_id: quote.quote_id }, { idempotencyKey: idempotencyKey(paymentKey) });
};
```

That is all of it. `openLayerX` builds a client over an HTTPS transport that holds your token in a `SecretBytes` container - redacted in logs, zeroed when destroyed. `pay` quotes the move, then commits the quote under an idempotency key you choose.

Loopback HTTP URLs are allowed for a local human-payment service. Any other `http://` URL is refused before a request is made, so a misconfigured environment cannot leak a bearer token in clear text.

## Run the whole sample

Set `LAYERX_API_URL` and `LAYERX_API_TOKEN` for that service, plus
`LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY`
and `LAYERX_PAYMENT_KEY` for the payment. Amounts use integer base units;
reuse the same payment key when reconciling the same operation.

The sample directory adds the parts that are yours rather than LayerX's: reading the environment, polling until the journey settles, and printing a report.

```sh
cd services/platform/examples/samples/first-payment-typescript
npm install
node index.mjs
```

It prints the journey identifier, the final state, the receipt evidence references, and - if the payment was refused - who refused it and whether any money left the account. It exits non-zero unless the journey reached `done` or `done-finalised`.

## Choosing an idempotency key

Derive it from the thing being paid for, not from the attempt. `order-2f9c1b7e4a10` is right; a UUID generated at the call site is the classic mistake, because the retry generates a new one and pays twice. Calling `move.commit` without a key fails inside the SDK with `idempotency-required` rather than reaching the network.

## Handling refusals

Every failure arrives as a `PlatformSdkError` carrying a machine code and a retry class. Switch on `error.code`; do not parse messages.

| Code | What to do |
|---|---|
| `idempotency-required` | You omitted the key. Fix the call |
| `idempotency-conflict` | Same key, different body. Reuse the original result |
| `rate-limit` | Wait for `error.retryAfterMs` |
| `unknown-outcome` | Do not retry. Resolve by looking up the receipt under your key |
| `budget-refusal` | The payment exceeded a funded budget. This is a protocol refusal |

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The committed move applies completely or not at all. |
| Replay refusal | `protocol` | A retried submission cannot apply twice, whatever your key handling does. |
| Idempotent money moves | `service` | Repeating the commit with the same key returns the original journey. |
| Quote then commit | `service` | The fee ceiling and arrival expectation you saw are the ones you committed to. |
| Done means verified | `service` | The `done` state your poll observes is backed by receipt evidence. |
