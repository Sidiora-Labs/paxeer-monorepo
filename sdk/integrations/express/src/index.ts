import { createHash } from "node:crypto";
import {
  BuyerMiddleware,
  LayerXPaymentHttpTransport,
  type BuyerSupportedKind,
} from "@sidiora/layerx-buyer-middleware";
import {
  ProductionClient,
  SecretBytes,
  isSelectableProtocolVersion,
  type SequencerReceiptKey,
  type SelectableProtocolVersion,
} from "@sidiora/layerx-sdk";
import {
  MiddlewareError,
  PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  VerifiedWebhookConsumer,
  X402_VERSION,
  type FulfillmentRepository,
  type JsonValue,
  type MiddlewareErrorCode,
  type PaymentRequired,
  type PaymentRequirements,
  type SellerDecision,
  type SellerPaymentAuthority,
  type WebhookClaimResult,
  type WebhookDeliveryClaim,
  type WebhookDeliveryStore,
  type WebhookRequestHeaders,
} from "@sidiora/layerx-seller-middleware";

export {
  MiddlewareError,
  PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  VerifiedWebhookConsumer,
  X402_VERSION,
  verifyPaymentReceipt,
  verifyPaymentCommitmentEvidence,
  verifyRpcPayment,
  PaymentRpc,
} from "@sidiora/layerx-seller-middleware";
export type {
  PaymentCommitmentEvidence,
  PaymentCommitmentResolver,
  FulfillmentRepository,
  JsonValue,
  MiddlewareErrorCode,
  PaymentPayload,
  PaymentRequired,
  PaymentRequirements,
  SellerDecision,
  SellerPaymentAuthority,
  SellerSettlementOutcome,
  SellerSettlementRequest,
  SettlementResponse,
  StoredFulfillment,
  WebhookClaimResult,
  WebhookConsumeResult,
  WebhookDeliveryClaim,
  WebhookDeliveryStore,
  WebhookRequestHeaders,
} from "@sidiora/layerx-seller-middleware";
export { BuyerMiddleware, LayerXPaymentHttpTransport, grantPaymentHeader } from "@sidiora/layerx-buyer-middleware";
export type {
  BuyerMiddlewareConfig,
  BuyerSupportedKind,
  CapturedSettlement,
  EvidenceMaterial,
  EvidenceReference,
  Journey,
  PaidFetchResult,
  ParsedOffer,
  PreparedPayment,
} from "@sidiora/layerx-buyer-middleware";
export { ProductionClient, SecretBytes } from "@sidiora/layerx-sdk";
export type { SequencerReceiptKey, ReceiptVerification } from "@sidiora/layerx-sdk";

export const WEBHOOK_ID_HEADER = "layerx-webhook-id" as const;
export const WEBHOOK_TIMESTAMP_HEADER = "layerx-webhook-timestamp" as const;
export const WEBHOOK_KEY_HEADER = "layerx-webhook-key-id" as const;
export const WEBHOOK_SIGNATURE_HEADER = "layerx-webhook-signature" as const;

export type ReceiptKeys = readonly SequencerReceiptKey[];

export const LAYERX_RECEIPT_KEYS_JSON = "LAYERX_RECEIPT_KEYS_JSON" as const;

export const DECLARED_KEYS = [
  "LAYERX_PRINCIPAL",
  "LAYERX_PROTECTED_PATH",
  "LAYERX_RESOURCE_URL",
  "LAYERX_RESOURCE_DESCRIPTION",
  "LAYERX_RESOURCE_MIME_TYPE",
  "LAYERX_RESOURCE_SERVICE_NAME",
  "LAYERX_PROTOCOL_VERSION",
  "LAYERX_X402_SCHEME",
  "LAYERX_X402_NETWORK",
  "LAYERX_PRICE",
  "LAYERX_ASSET",
  "LAYERX_PAY_TO",
  "LAYERX_PAYMENT_TIMEOUT_SECONDS",
  LAYERX_RECEIPT_KEYS_JSON,
  "LAYERX_WEBHOOK_PATH",
  "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON",
  "LAYERX_WEBHOOK_MAX_AGE_MS",
  "LAYERX_WEBHOOK_LEASE_MS",
  "LAYERX_HUMAN_URL",
  "LAYERX_SOURCE",
  "LAYERX_TOKEN",
] as const;

export type DeclaredKey = (typeof DECLARED_KEYS)[number];

const SECRET_KEYS: readonly DeclaredKey[] = ["LAYERX_TOKEN"];
const MAXIMUM_WEBHOOK_BYTES = 1_048_576;

export type LayerXIntegrationErrorCode =
  | "missing-declared-key"
  | "invalid-declared-key"
  | "client-runtime-refused"
  | "duplicate-header"
  | "unverifiable-body"
  | "receipt-not-backed";

export class LayerXIntegrationError extends Error {
  public constructor(public readonly code: LayerXIntegrationErrorCode) {
    super(code);
    this.name = "LayerXIntegrationError";
  }
}

export type Environment = Readonly<Record<string, string | undefined>>;

export interface ExpressRequest {
  readonly method: string;
  readonly headers: Readonly<Record<string, string | readonly string[] | undefined>>;
  readonly body?: unknown;
}

export interface ExpressStreamedRequest extends ExpressRequest {
  [Symbol.asyncIterator](): AsyncIterator<Uint8Array>;
}

export interface ExpressResponse {
  status(code: number): ExpressResponse;
  set(field: string, value: string): ExpressResponse;
  send(body?: string): unknown;
  end(): unknown;
  readonly locals: Record<string, unknown>;
}

export type ExpressNext = (error?: unknown) => void;

export type ExpressHandler = (
  request: ExpressRequest,
  response: ExpressResponse,
  next: ExpressNext,
) => void;

export type ExpressWebhookHandler = (
  request: ExpressStreamedRequest,
  response: ExpressResponse,
  next: ExpressNext,
) => void;

export interface ExpressRouter {
  use(path: string, handler: ExpressHandler): unknown;
  post(path: string, handler: ExpressWebhookHandler): unknown;
}

export interface LayerXWebhookSettings {
  readonly path: string;
  readonly publicKeys: Readonly<Record<string, Uint8Array>>;
  readonly maximumAgeMs: number;
  readonly leaseMs: number;
}

export interface LayerXBuyerSettings {
  readonly humanUrl: string;
  readonly source: string;
  readonly supported: readonly BuyerSupportedKind[];
}

export interface LayerXDeclaredConfig {
  readonly principal: string;
  readonly protectedPath: string;
  readonly protocolVersion: SelectableProtocolVersion;
  readonly paymentRequired: PaymentRequired;
  readonly requirements: PaymentRequirements;
  readonly receiptKeys: ReceiptKeys;
  readonly webhook: LayerXWebhookSettings;
  readonly buyer?: LayerXBuyerSettings;
}

export interface LayerXResource {
  readonly contentType: string;
  readonly body: string;
}

export interface LayerXResourceHandler {
  release(request: ExpressRequest, idempotencyKey: string): Promise<LayerXResource>;
}

export interface LayerXWebhookHandlerConsumer {
  handle(event: Readonly<Record<string, JsonValue>>, deliveryId: string): Promise<void>;
}

export interface LayerXMountOptions {
  readonly environment: Environment;
  readonly resources: LayerXResourceHandler;
  readonly fulfillments: FulfillmentRepository<LayerXResource>;
  readonly deliveries: WebhookDeliveryStore;
  readonly events: LayerXWebhookHandlerConsumer;
  readonly authority?: SellerPaymentAuthority;
  readonly now?: () => number;
  readonly fetch?: typeof globalThis.fetch;
}

export interface LayerXExpressMount {
  readonly config: LayerXDeclaredConfig;
  readonly seller: SellerMiddleware<LayerXResource>;
  readonly webhooks: VerifiedWebhookConsumer;
  readonly paymentGate: ExpressHandler;
  readonly webhookEndpoint: ExpressWebhookHandler;
  readonly buyer?: BuyerMiddleware;
  destroy(): void;
}

export function platform_int_express(): "receipt-gated-x402-express" {
  return "receipt-gated-x402-express";
}

export function assertServerRuntime(): void {
  const scope = globalThis as { readonly window?: unknown; readonly document?: unknown };
  if (scope.window !== undefined || scope.document !== undefined) {
    throw new LayerXIntegrationError("client-runtime-refused");
  }
}

export function assertNoPublishedSecrets(environment: Environment): void {
  for (const [name, value] of Object.entries(environment)) {
    if (value === undefined || value.length === 0) {
      continue;
    }
    if (!isPublishedName(name)) {
      continue;
    }
    for (const secret of SECRET_KEYS) {
      const declared = environment[secret];
      if (declared !== undefined && declared.length > 0 && declared === value) {
        throw new LayerXIntegrationError("client-runtime-refused");
      }
    }
    if (looksLikeKeyMaterial(name)) {
      throw new LayerXIntegrationError("client-runtime-refused");
    }
  }
}

export function readDeclaredConfig(environment: Environment): LayerXDeclaredConfig {
  assertServerRuntime();
  assertNoPublishedSecrets(environment);
  const scheme = required(environment, "LAYERX_X402_SCHEME");
  const network = required(environment, "LAYERX_X402_NETWORK");
  const resourceUrl = required(environment, "LAYERX_RESOURCE_URL");
  const requirements: PaymentRequirements = {
    scheme,
    network,
    amount: required(environment, "LAYERX_PRICE"),
    asset: required(environment, "LAYERX_ASSET"),
    payTo: required(environment, "LAYERX_PAY_TO"),
    maxTimeoutSeconds: positiveInteger(required(environment, "LAYERX_PAYMENT_TIMEOUT_SECONDS")),
    extra: { layerx: { commitment: "receipted", purposeHash: offerPurposeHash(resourceUrl) } },
  };
  const description = optional(environment, "LAYERX_RESOURCE_DESCRIPTION");
  const mimeType = optional(environment, "LAYERX_RESOURCE_MIME_TYPE");
  const serviceName = optional(environment, "LAYERX_RESOURCE_SERVICE_NAME");
  const paymentRequired: PaymentRequired = {
    x402Version: X402_VERSION,
    resource: {
      url: resourceUrl,
      ...(description === undefined ? {} : { description }),
      ...(mimeType === undefined ? {} : { mimeType }),
      ...(serviceName === undefined ? {} : { serviceName }),
    },
    accepts: [requirements],
  };
  const humanUrl = optional(environment, "LAYERX_HUMAN_URL");
  const source = optional(environment, "LAYERX_SOURCE");
  const buyer: LayerXBuyerSettings | undefined = humanUrl === undefined || source === undefined
    ? undefined
    : { humanUrl, source, supported: [{ scheme, network }] };
  return {
    principal: required(environment, "LAYERX_PRINCIPAL"),
    protectedPath: mountPath(required(environment, "LAYERX_PROTECTED_PATH")),
    protocolVersion: declaredProtocolVersion(required(environment, PROTOCOL_VERSION_VARIABLE)),
    paymentRequired,
    requirements,
    receiptKeys: parseReceiptKeys(required(environment, LAYERX_RECEIPT_KEYS_JSON)),
    webhook: {
      path: mountPath(required(environment, "LAYERX_WEBHOOK_PATH")),
      publicKeys: parseWebhookKeys(required(environment, "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON")),
      maximumAgeMs: positiveInteger(optional(environment, "LAYERX_WEBHOOK_MAX_AGE_MS") ?? "300000"),
      leaseMs: positiveInteger(optional(environment, "LAYERX_WEBHOOK_LEASE_MS") ?? "60000"),
    },
    ...(buyer === undefined ? {} : { buyer }),
  };
}

export function offerPurposeHash(resourceUrl: string): string {
  return createHash("sha256").update(`layerx-express-purpose:${resourceUrl}`).digest("hex");
}

export class SingleProcessWebhookDeliveryStore implements WebhookDeliveryStore {
  readonly #entries = new Map<string, { payloadDigest: string; leaseUntilMs: number; completed: boolean }>();
  readonly #now: () => number;

  public constructor(now?: () => number) {
    this.#now = now ?? Date.now;
  }

  public claim(value: WebhookDeliveryClaim): Promise<WebhookClaimResult> {
    const existing = this.#entries.get(value.deliveryId);
    if (existing === undefined) {
      this.#entries.set(value.deliveryId, {
        payloadDigest: value.payloadDigest,
        leaseUntilMs: value.leaseUntilMs,
        completed: false,
      });
      return Promise.resolve("claimed");
    }
    if (existing.payloadDigest !== value.payloadDigest) {
      return Promise.resolve("conflict");
    }
    if (existing.completed) {
      return Promise.resolve("completed");
    }
    if (existing.leaseUntilMs > this.#now()) {
      return Promise.resolve("processing");
    }
    existing.leaseUntilMs = value.leaseUntilMs;
    return Promise.resolve("claimed");
  }

  public complete(deliveryId: string, payloadDigest: string): Promise<void> {
    const existing = this.#entries.get(deliveryId);
    if (existing === undefined || existing.payloadDigest !== payloadDigest) {
      throw new MiddlewareError("webhook-replay");
    }
    existing.completed = true;
    existing.leaseUntilMs = 0;
    return Promise.resolve();
  }

  public release(deliveryId: string, payloadDigest: string): Promise<void> {
    const existing = this.#entries.get(deliveryId);
    if (existing !== undefined && existing.payloadDigest === payloadDigest && !existing.completed) {
      this.#entries.delete(deliveryId);
    }
    return Promise.resolve();
  }
}

export function createLayerXMount(options: LayerXMountOptions): LayerXExpressMount {
  const config = readDeclaredConfig(options.environment);
  const seller = new SellerMiddleware<LayerXResource>({
    paymentRequired: config.paymentRequired,
    receiptKeys: config.receiptKeys,
    authority: options.authority ?? new ReceiptPayloadAuthority(config.receiptKeys),
    fulfillments: options.fulfillments,
  });
  const webhooks = new VerifiedWebhookConsumer({
    publicKeys: config.webhook.publicKeys,
    deliveries: options.deliveries,
    maximumAgeMs: config.webhook.maximumAgeMs,
    leaseMs: config.webhook.leaseMs,
    ...(options.now === undefined ? {} : { now: options.now }),
  });
  const token = optional(options.environment, "LAYERX_TOKEN");
  const secret = config.buyer === undefined || token === undefined
    ? undefined
    : new SecretBytes(new TextEncoder().encode(token));
  const buyer = config.buyer === undefined || secret === undefined
    ? undefined
    : new BuyerMiddleware({
      client: new ProductionClient(new LayerXPaymentHttpTransport({
        baseUrl: config.buyer.humanUrl,
        bearerToken: secret,
        ...(options.fetch === undefined ? {} : { fetch: options.fetch }),
      })),
      source: config.buyer.source,
      supported: config.buyer.supported,
      receiptKeys: config.receiptKeys,
      ...(options.now === undefined ? {} : { now: options.now }),
      ...(options.fetch === undefined ? {} : { fetch: options.fetch }),
    });
  return {
    config,
    seller,
    webhooks,
    paymentGate: layerXPaymentGate({
      seller,
      requirements: config.requirements,
      principal: config.principal,
      resources: options.resources,
    }),
    webhookEndpoint: layerXWebhookEndpoint(webhooks, options.events),
    ...(buyer === undefined ? {} : { buyer }),
    destroy: () => {
      secret?.destroy();
    },
  };
}

export function mountLayerX(router: ExpressRouter, options: LayerXMountOptions): LayerXExpressMount {
  const mount = createLayerXMount(options);
  router.use(mount.config.protectedPath, mount.paymentGate);
  router.post(mount.config.webhook.path, mount.webhookEndpoint);
  return mount;
}

export interface LayerXSellerRuntime {
  readonly seller: SellerMiddleware<LayerXResource>;
  readonly requirements: PaymentRequirements;
  readonly principal: string;
  readonly resources: LayerXResourceHandler;
}

export function layerXPaymentGate(runtime: LayerXSellerRuntime): ExpressHandler {
  return (request, response, next) => {
    releaseGuarded(runtime, request, response).catch((error: unknown) => {
      next(error);
    });
  };
}

export function layerXWebhookEndpoint(
  consumer: VerifiedWebhookConsumer,
  events: LayerXWebhookHandlerConsumer,
): ExpressWebhookHandler {
  return (request, response, next) => {
    consumeGuarded(consumer, events, request, response).catch((error: unknown) => {
      next(error);
    });
  };
}

export async function assertReceiptBacked(
  decision: Extract<SellerDecision<LayerXResource>, { readonly kind: "released" }>,
  requirements: PaymentRequirements,
): Promise<void> {
  const evidenceDigest = layerXEvidenceDigest(decision.settlement.extensions);
  const receiptDigest = `0x${toHex(decision.verification.receiptDigest)}`;
  if (
    decision.verification.level !== "sequencer-signed"
    || decision.verification.receipt.resultCode !== 0
    || !decision.settlement.success
    || decision.settlement.network !== requirements.network
    || decision.settlement.amount !== requirements.amount
    || decision.settlement.transaction !== decision.verification.txHash
    || decision.verification.txHash !== `0x${toHex(decision.verification.receipt.activityId)}`
    || !constantTimeEqualText(evidenceDigest, receiptDigest)
    || decision.verification.payment.amount !== BigInt(requirements.amount)
    || !equalBytes(decision.verification.payment.assetId, parseHex32(requirements.asset))
    || !equalBytes(decision.verification.payment.to, parseHex32(requirements.payTo))
  ) {
    throw new LayerXIntegrationError("receipt-not-backed");
  }
}

async function releaseGuarded(
  runtime: LayerXSellerRuntime,
  request: ExpressRequest,
  response: ExpressResponse,
): Promise<void> {
  let decision: SellerDecision<LayerXResource>;
  try {
    decision = await runtime.seller.handle(
      runtime.principal,
      singleHeader(request.headers, PAYMENT_SIGNATURE_HEADER.toLowerCase()),
      (idempotencyKey) => runtime.resources.release(request, idempotencyKey),
    );
  } catch (error) {
    if (error instanceof MiddlewareError) {
      response
        .status(paymentErrorStatus(error.code))
        .set("content-type", "application/json")
        .send(JSON.stringify({ error: error.code }));
      return;
    }
    throw error;
  }
  if (decision.kind === "payment-required") {
    response
      .status(decision.status)
      .set(PAYMENT_REQUIRED_HEADER, decision.headers[PAYMENT_REQUIRED_HEADER])
      .set("content-type", "application/json")
      .send(JSON.stringify(decision.body));
    return;
  }
  if (decision.kind === "pending") {
    response.status(decision.status).set("retry-after", "1").end();
    return;
  }
  if (decision.kind === "refused") {
    response
      .status(decision.status)
      .set(PAYMENT_RESPONSE_HEADER, decision.headers[PAYMENT_RESPONSE_HEADER])
      .end();
    return;
  }
  await assertReceiptBacked(decision, runtime.requirements);
  response.locals["layerx"] = {
    receiptDigest: toHex(decision.verification.receiptDigest),
    transaction: decision.settlement.transaction,
    level: decision.verification.level,
  };
  response
    .status(decision.status)
    .set(PAYMENT_RESPONSE_HEADER, decision.headers[PAYMENT_RESPONSE_HEADER])
    .set("content-type", decision.resource.contentType)
    .send(decision.resource.body);
}

async function consumeGuarded(
  consumer: VerifiedWebhookConsumer,
  events: LayerXWebhookHandlerConsumer,
  request: ExpressStreamedRequest,
  response: ExpressResponse,
): Promise<void> {
  const headers = webhookHeaders(request.headers);
  const rawBody = await readRawBody(request);
  try {
    const outcome = await consumer.consume(rawBody, headers, (event, deliveryId) => events.handle(event, deliveryId));
    if (outcome === "processed") {
      response.status(204).end();
      return;
    }
    if (outcome === "duplicate") {
      response.status(200).set("content-type", "application/json").send(JSON.stringify({ outcome }));
      return;
    }
    response
      .status(409)
      .set("retry-after", "1")
      .set("content-type", "application/json")
      .send(JSON.stringify({ outcome }));
  } catch (error) {
    if (error instanceof MiddlewareError && error.code === "invalid-webhook") {
      response.status(401).set("content-type", "application/json").send(JSON.stringify({ error: error.code }));
      return;
    }
    if (error instanceof MiddlewareError && error.code === "webhook-replay") {
      response.status(409).set("content-type", "application/json").send(JSON.stringify({ error: error.code }));
      return;
    }
    throw error;
  }
}

function paymentErrorStatus(code: MiddlewareErrorCode): number {
  if (code === "payment-pending") {
    return 202;
  }
  if (code === "fulfillment-conflict") {
    return 409;
  }
  return 402;
}

function webhookHeaders(
  headers: Readonly<Record<string, string | readonly string[] | undefined>>,
): WebhookRequestHeaders {
  const id = singleHeader(headers, WEBHOOK_ID_HEADER);
  const timestamp = singleHeader(headers, WEBHOOK_TIMESTAMP_HEADER);
  const keyId = singleHeader(headers, WEBHOOK_KEY_HEADER);
  const signature = singleHeader(headers, WEBHOOK_SIGNATURE_HEADER);
  if (id === undefined || timestamp === undefined || keyId === undefined || signature === undefined) {
    throw new MiddlewareError("invalid-webhook");
  }
  return { id, timestamp, keyId, signature };
}

async function readRawBody(request: ExpressStreamedRequest): Promise<Uint8Array> {
  const body = request.body;
  if (body instanceof Uint8Array) {
    return body;
  }
  if (typeof body === "string") {
    return new TextEncoder().encode(body);
  }
  if (body !== undefined && body !== null) {
    throw new LayerXIntegrationError("unverifiable-body");
  }
  const chunks: Uint8Array[] = [];
  let total = 0;
  const iterator = request[Symbol.asyncIterator]();
  for (let step = await iterator.next(); step.done !== true; step = await iterator.next()) {
    const chunk = step.value;
    total += chunk.length;
    if (total > MAXIMUM_WEBHOOK_BYTES) {
      throw new MiddlewareError("invalid-webhook");
    }
    chunks.push(chunk);
  }
  return concatenate(...chunks);
}

function singleHeader(
  headers: Readonly<Record<string, string | readonly string[] | undefined>>,
  name: string,
): string | undefined {
  const value = headers[name];
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== "string") {
    throw new LayerXIntegrationError("duplicate-header");
  }
  return value;
}

function layerXEvidenceDigest(extensions: Readonly<Record<string, JsonValue>> | undefined): string {
  const layerx = extensions?.["layerx"];
  if (typeof layerx !== "object" || layerx === null || Array.isArray(layerx)) {
    throw new LayerXIntegrationError("receipt-not-backed");
  }
  const digest = (layerx as Readonly<Record<string, JsonValue>>)["receiptDigest"];
  if (typeof digest !== "string" || !/^0x[0-9a-f]{64}$/u.test(digest)) {
    throw new LayerXIntegrationError("receipt-not-backed");
  }
  return digest;
}

function required(environment: Environment, key: DeclaredKey): string {
  const value = environment[key];
  if (value === undefined || value.length === 0) {
    throw new LayerXIntegrationError("missing-declared-key");
  }
  return value;
}

function optional(environment: Environment, key: DeclaredKey): string | undefined {
  const value = environment[key];
  return value === undefined || value.length === 0 ? undefined : value;
}

export const PROTOCOL_VERSION_VARIABLE = "LAYERX_PROTOCOL_VERSION" as const;

function declaredProtocolVersion(value: string): SelectableProtocolVersion {
  const parsed = positiveInteger(value);
  if (!isSelectableProtocolVersion(parsed)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return parsed;
}

function positiveInteger(value: string): number {
  if (!/^[1-9][0-9]*$/u.test(value)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return parsed;
}

function mountPath(value: string): string {
  if (!value.startsWith("/") || value.length > 512 || /[\s?#]/u.test(value)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return value;
}

function parseReceiptKeys(value: string): ReceiptKeys {
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  if (!Array.isArray(parsed) || parsed.length === 0 || parsed.length > 32) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return Object.freeze(parsed.map((entry: unknown) => {
    if (typeof entry !== "object" || entry === null || Array.isArray(entry)) {
      throw new LayerXIntegrationError("invalid-declared-key");
    }
    const key = entry as Readonly<Record<string, JsonValue>>;
    const fields = ["keyId", "publicKey", "firstHeight", "lastHeight"];
    if (Object.keys(key).length !== fields.length || fields.some((field) => !Object.hasOwn(key, field))) {
      throw new LayerXIntegrationError("invalid-declared-key");
    }
    const firstHeight = receiptKeyHeight(requiredText(key["firstHeight"]));
    const lastHeight = receiptKeyHeight(requiredText(key["lastHeight"]));
    if (firstHeight > lastHeight) {
      throw new LayerXIntegrationError("invalid-declared-key");
    }
    return Object.freeze({
      keyId: parseHex32(requiredText(key["keyId"])),
      publicKey: parseHex32(requiredText(key["publicKey"])),
      first_height: firstHeight,
      last_height: lastHeight,
    });
  }));
}

function receiptKeyHeight(value: string): bigint {
  if (!/^(0|[1-9][0-9]{0,19})$/u.test(value)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  const height = BigInt(value);
  if (height > 0xffff_ffff_ffff_ffffn) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return height;
}

function parseWebhookKeys(value: string): Readonly<Record<string, Uint8Array>> {
  const parsed = parseJsonObject(value);
  const entries = Object.entries(parsed);
  if (entries.length === 0 || entries.length > 32) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return Object.fromEntries(entries.map(([keyId, encoded]) => {
    if (!/^[A-Za-z0-9._-]{1,64}$/u.test(keyId)) {
      throw new LayerXIntegrationError("invalid-declared-key");
    }
    const key = decodeBase64(requiredText(encoded));
    if (key.length !== 32) {
      throw new LayerXIntegrationError("invalid-declared-key");
    }
    return [keyId, key];
  }));
}

function parseJsonObject(value: string): Readonly<Record<string, JsonValue>> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return parsed as Readonly<Record<string, JsonValue>>;
}

function requiredText(value: JsonValue | undefined): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return value;
}

function isPublishedName(name: string): boolean {
  return name.startsWith("NEXT_PUBLIC_")
    || name.startsWith("PUBLIC_")
    || name.startsWith("VITE_")
    || name.startsWith("REACT_APP_")
    || name.startsWith("EXPO_PUBLIC_");
}

function looksLikeKeyMaterial(name: string): boolean {
  return /(^|_)(TOKEN|SECRET|PRIVATE|CREDENTIAL|PASSWORD|SIGNING_KEY|API_KEY)(_|$)/u.test(name);
}

function parseHex32(value: string): Uint8Array {
  const digits = value.startsWith("0x") ? value.slice(2) : value;
  if (!/^[0-9a-fA-F]{64}$/u.test(digits)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  return Uint8Array.from({ length: 32 }, (_, index) => Number.parseInt(digits.slice(index * 2, index * 2 + 2), 16));
}

function decodeBase64(value: string): Uint8Array {
  if (value.length === 0 || !/^[A-Za-z0-9+/]*={0,2}$/u.test(value)) {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
  try {
    return Uint8Array.from(atob(value), (character) => character.charCodeAt(0));
  } catch {
    throw new LayerXIntegrationError("invalid-declared-key");
  }
}

function toHex(value: Uint8Array): string {
  return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

function constantTimeEqualText(left: string, right: string): boolean {
  if (left.length !== right.length) {
    return false;
  }
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) {
    difference |= left.charCodeAt(index) ^ right.charCodeAt(index);
  }
  return difference === 0;
}

function equalBytes(left: Uint8Array, right: Uint8Array): boolean {
  if (left.length !== right.length) {
    return false;
  }
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) {
    difference |= (left[index] ?? 0) ^ (right[index] ?? 0);
  }
  return difference === 0;
}

function concatenate(...values: readonly Uint8Array[]): Uint8Array {
  const result = new Uint8Array(values.reduce((total, value) => total + value.length, 0));
  let offset = 0;
  for (const value of values) {
    result.set(value, offset);
    offset += value.length;
  }
  return result;
}
