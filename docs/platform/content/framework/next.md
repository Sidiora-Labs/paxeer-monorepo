# Next.js quickstart

Mount paid App Router handlers and scan browser-facing build artifacts for secret material.

## Before you start

```sh
npm install @sidiora/layerx-next next react react-dom
```

The declared seller configuration includes `LAYERX_RECEIPT_KEYS_JSON`, a JSON array of one to thirty-two pinned receipt keys. Each entry has nonzero 32-byte hex `keyId` and `publicKey` fields and decimal-string `firstHeight` and `lastHeight` bounds (inclusive). Read the configuration from the environment on the first request rather than at module scope. `next build` collects your route modules, so a key read at module scope becomes a build input; reading it on the first request keeps `next build` runnable with no key present.

| Variable | Requirement |
|---|---|
| `LAYERX_X402_SCHEME` | Required: `exact`, `metered` or `subscription`. |
| `LAYERX_PAYMENT_PURPOSE_HASH` | Required for every scheme other than `exact`; optional for `exact`. When supplied, it must be a nonzero 32-byte hex value. |

Missing `LAYERX_PAYMENT_PURPOSE_HASH` for a non-exact scheme refuses construction with an error naming the variable and the scheme. With `mountLayerXOnRequest`, this check runs on the first request.

## The integration

```js sample=paid-route-next
import { SingleProcessWebhookDeliveryStore, mountLayerXOnRequest } from "@sidiora/layerx-next";
export const layerx = mountLayerXOnRequest(() => ({
  environment: process.env,
  resources: { release: async () => ({ contentType: "application/json", body: reportBody }) },
  fulfillments: new FileFulfillmentRepository(fulfillmentDirectory()),
  deliveries: new SingleProcessWebhookDeliveryStore(),
  events: { handle: async (event, deliveryId) => { settlements.push({ deliveryId, event }); } },
}));
```

`mountLayerXOnRequest` returns route objects you export directly. It caches the mount after successful configuration; a configuration refusal leaves it unconfigured so a later request calls the factory again. A request that finds a required variable missing receives `503` and a body naming it, such as `{"error":"missing-declared-key","key":"LAYERX_PRICE"}`. `mountLayerX` reads configuration immediately for a host that configures at startup.

## Wire the routes

The paid route:

```js sample=paid-route-next file=app/paid/route.js region=route
import { layerx } from "../../lib/layerx.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const { GET, POST } = layerx.resource;
```

The webhook route:

```js sample=paid-route-next file=app/layerx/webhooks/route.js region=route
import { layerx } from "../../../lib/layerx.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const { POST } = layerx.webhook;
```

Both must be `runtime = "nodejs"` and `dynamic = "force-dynamic"`. The Edge runtime cannot do the verification the gate performs, and a cached route would serve a paid body to someone who did not pay for it.

## The bundle scanner

The package ships `layerx-scan-bundle`. Run it after `next build`:

```sh
npx layerx-scan-bundle
```

The scanner reads browser-facing artifacts and exits nonzero when it reports a finding. It checks four categories:

| Finding | What it caught |
|---|---|
| `declared-secret-value` | A configured secret value of at least eight characters appears in an artifact |
| `declared-secret-name` | A configured secret variable's name appears in an artifact |
| `private-key-block` | A PEM private key block was bundled |
| `published-key-material` | A `NEXT_PUBLIC_`, `PUBLIC_`, `VITE_`, `REACT_APP_` or `EXPO_PUBLIC_` variable is named like a token, secret, credential or signing key |

Secret-value findings use a redacted locator and an offset. Other categories identify the matched variable name or private-key marker. Run the scanner after `next build` in the same build script to make its exit status part of the build result.

## Fulfilment storage

The sample uses a file-backed fulfilment repository. Its persistence follows the filesystem supplied by the deployment:

```js sample=paid-route-next file=lib/fulfillments.js region=storage
import { mkdir, open, readFile } from "node:fs/promises";
import { join } from "node:path";
import { MiddlewareError } from "@sidiora/layerx-next";

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

The resource callback runs before the file record is created. Use an idempotent callback and shared storage that coordinates concurrent fulfilment across requests and cold starts.

The sample's `SingleProcessWebhookDeliveryStore` keeps delivery claims in memory. Supply a shared durable delivery store when claims must survive restarts or coordinate multiple workers.

## Run it

Supply the required seller and webhook configuration before sending requests.

For this sample, the `dev` script first runs the matching `predev` hook, which invokes
the repository-root `build:dependencies` script for `@sidiora/layerx-doc-paid-route-next`
before starting the Next.js development server.

```sh
cd services/platform/examples/samples/paid-route-next
npm install
npm run dev
```

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | The route verifies the receipt from its own bytes against the pinned receipt keys in `LAYERX_RECEIPT_KEYS_JSON`. |
| Atomic settlement | `protocol` | The payment behind a released body happened whole or not at all. |
| Receipt-gated resource release | `service` | The route handler releases only against a verified receipt. |
| Refusal to publish a secret | `service` | The scanner fails a build whose client bundle carries a declared secret. It is a build-time control, so run it. |
| Exactly-once fulfilment | `service` | Only as durable as your repository. On serverless, that means shared storage. |
| Verified, replay-protected webhooks | `service` | Signature-checked, age-checked and lease-claimed in your process. |
