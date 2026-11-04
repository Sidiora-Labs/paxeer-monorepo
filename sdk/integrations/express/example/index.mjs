import { mkdir, open, readFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import express from "express";
import {
  decodePaymentCommitmentEvidence,
  encodePaymentCommitmentEvidence,
} from "@sidiora/layerx-seller-middleware";
import {
  MiddlewareError,
  SingleProcessWebhookDeliveryStore,
  mountLayerX,
} from "@sidiora/layerx-express";

export class FileFulfillmentRepository {
  constructor(directory) {
    this.directory = directory;
  }

  async get(idempotencyKey) {
    try {
      return await this.read(join(this.directory, `${idempotencyKey}.json`));
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
      return undefined;
    }
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
    const record = { ...proposed, resource };
    let file;
    try {
      file = await open(path, "wx", 0o600);
      await file.writeFile(JSON.stringify({
        ...record,
        ...(record.commitmentEvidence === undefined ? {} : {
          commitmentEvidence: encodePaymentCommitmentEvidence(record.commitmentEvidence),
        }),
      }), "utf8");
      await file.sync();
      return record;
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      return await this.read(path, proposed);
    } finally {
      await file?.close();
    }
  }

  async read(path, proposed) {
    const stored = JSON.parse(await readFile(path, "utf8"));
    if (proposed !== undefined && stored.requestDigest !== proposed.requestDigest) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    return {
      ...stored,
      ...(stored.commitmentEvidence === undefined ? {} : {
        commitmentEvidence: decodePaymentCommitmentEvidence(stored.commitmentEvidence),
      }),
    };
  }
}

export async function createExampleApplication(environment = process.env) {
  const resourceBody = await readFile(environment.LAYERX_RESOURCE_FILE ?? "./resource.json", "utf8");
  const settlements = [];

  const app = express();
  const mount = mountLayerX(app, {
    environment,
    resources: {
      async release() {
        return { contentType: "application/json", body: resourceBody };
      },
    },
    fulfillments: new FileFulfillmentRepository(environment.LAYERX_FULFILLMENT_DIR ?? "./fulfillments"),
    deliveries: new SingleProcessWebhookDeliveryStore(),
    events: {
      async handle(event, deliveryId) {
        settlements.push({ deliveryId, event });
      },
    },
  });

  app.get("/layerx/settlements", (request, response) => {
    response.status(200).set("content-type", "application/json").send(JSON.stringify({ settlements }));
  });

  return { app, mount };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const { app, mount } = await createExampleApplication();
  const port = Number(process.env.PORT ?? "8080");
  if (!Number.isSafeInteger(port) || port <= 0 || port > 65535) throw new Error("invalid_port");

  const server = app.listen(port, "127.0.0.1", () => {
    process.stdout.write(JSON.stringify({
      listening: port,
      resource: mount.config.protectedPath,
      webhook: mount.config.webhook.path,
      buyer: mount.buyer !== undefined,
    }) + "\n");
  });

  for (const signal of ["SIGINT", "SIGTERM"]) {
    process.on(signal, () => {
      mount.destroy();
      server.close(() => process.exit(0));
    });
  }
}
