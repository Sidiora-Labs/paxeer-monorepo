# Express quickstart

Mount a payment gate and a verified webhook endpoint on an Express application.

## Before you start

```sh
npm install @sidiora/layerx-express express
```

The integration reads its declared configuration from the environment. The declared keys are `LAYERX_PRINCIPAL`, `LAYERX_PROTECTED_PATH`, the `LAYERX_RESOURCE_*` description fields, `LAYERX_PROTOCOL_VERSION` (the selected receipt compatibility version), `LAYERX_X402_SCHEME`, `LAYERX_X402_NETWORK`, `LAYERX_PRICE`, `LAYERX_ASSET`, `LAYERX_PAY_TO`, `LAYERX_PAYMENT_TIMEOUT_SECONDS`, `LAYERX_RECEIPT_KEYS_JSON`, and the four `LAYERX_WEBHOOK_*` values. Optional buyer settings use `LAYERX_HUMAN_URL`, `LAYERX_SOURCE` and `LAYERX_TOKEN`. `LAYERX_TOKEN` is the only declared secret.

## The integration

```js sample=paid-endpoint-express
import { SingleProcessWebhookDeliveryStore, mountLayerX } from "@sidiora/layerx-express";
const layerx = mountLayerX(app, {
  environment: process.env,
  resources: { release: async () => ({ contentType: "application/json", body: reportBody }) },
  fulfillments: new FileFulfillmentRepository(fulfillmentDirectory),
  deliveries: new SingleProcessWebhookDeliveryStore(),
  events: { handle: async (event, deliveryId) => { settlements.push({ deliveryId, event }); } },
});
```

`mountLayerX` installs the payment gate at `LAYERX_PROTECTED_PATH` and the webhook route at `LAYERX_WEBHOOK_PATH`. You supply four things:

| Option | What it is |
|---|---|
| `resources` | How to produce the thing you are selling, once payment is verified |
| `fulfillments` | Where a completed fulfilment is stored so a repeat returns the same bytes |
| `deliveries` | Where webhook delivery claims live |
| `events` | What to do with a verified settlement event |

## What happens on a request

1. An unpaid request gets `402` with a `PAYMENT-REQUIRED` header describing exactly what is owed.
2. The client pays and retries with a `PAYMENT-SIGNATURE` header.
3. The middleware verifies the signed receipt and transaction against the pinned receipt keys and matches the payment facts. Only then does it call your `resources.release`.
4. The response carries a `PAYMENT-RESPONSE` header stating what settled.

A payment that is still settling returns `202`, not a guess. A payment that was refused returns the refusal, not a `500`.

## Fulfilment storage is yours

The sample stores receipt bytes and response content in files:

```js sample=paid-endpoint-express file=fulfillments.mjs region=storage
import { mkdir, open, readFile } from "node:fs/promises";
import { join } from "node:path";
import { MiddlewareError } from "@sidiora/layerx-express";

export class FileFulfillmentRepository {
  constructor(directory) {
    this.directory = directory;
  }

  async fulfill(proposed, release) {
    await mkdir(this.directory, { recursive: true, mode: 0o700 });
    const path = join(this.directory, `${proposed.idempotencyKey}.json`);
    try {
      return await this.read(path, proposed);
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    const resource = await release();
    const record = JSON.stringify({
      requestDigest: proposed.requestDigest,
      receipt: Buffer.from(proposed.canonicalReceipt).toString("base64"),
      resource,
    });
    let file;
    try {
      file = await open(path, "wx", 0o600);
      await file.writeFile(record, "utf8");
      await file.sync();
      return { ...proposed, resource };
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      return await this.read(path, proposed);
    } finally {
      await file?.close();
    }
  }

  async read(path, proposed) {
    const stored = JSON.parse(await readFile(path, "utf8"));
    if (stored.requestDigest !== proposed.requestDigest) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    return {
      ...proposed,
      canonicalReceipt: Uint8Array.from(Buffer.from(stored.receipt, "base64")),
      resource: stored.resource,
    };
  }
}
```

It creates one record per key with `wx`, syncs it before returning, and raises `fulfillment-conflict` for a different request digest. The resource callback runs before record creation; use an idempotent callback and storage that coordinates concurrent requests and restarts when fulfilment has side effects.

The sample's `SingleProcessWebhookDeliveryStore` keeps delivery claims in memory. Supply a shared durable delivery store when claims must survive restarts or coordinate multiple workers.

## Run it

Supply the required seller and webhook configuration before starting. The sample listens on loopback port `8080` unless `PORT` selects another port.

```sh
cd services/platform/examples/samples/paid-endpoint-express
npm install
node index.mjs
```

It prints the port, the paid path and the webhook path, and exposes `/settlements` so you can see what arrived.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | The gate verifies signed receipt bytes against independently pinned receipt keys. |
| Atomic settlement | `protocol` | The payment behind a released resource happened whole or not at all. |
| Receipt-gated resource release | `service` | Your middleware releases only against a verified receipt. It binds requests arriving through the middleware. |
| Exactly-once fulfilment | `service` | Fulfilment persistence and concurrency follow the repository supplied by the application. |
| Verified, replay-protected webhooks | `service` | Signature-checked, age-checked and lease-claimed in your process. |
