import {
  paymentAccount,
  paymentCommitment,
  paymentCurrency,
  paymentPayer,
  paymentPurpose,
  verifyPaymentCommitment,
  type PaymentCommitment,
  type PaymentCommitmentEvidence,
  type PaymentCommitmentResolver,
} from "./commitment.js";
export * from "./commitment.js";
export * from "./ledger.js";
import { decodePaymentExactLog, LAYERX_LEDGER_ADDRESS, type PaymentExactEvent } from "./ledger.js";
import {
  PlatformSdkError,
  keccak256,
  verifySequencerReceipt,
  type SequencerReceiptKey,
  type SequencerReceiptVerification,
} from "@sidiora/layerx-sdk";

export const X402_VERSION = 2 as const;
export const PAYMENT_REQUIRED_HEADER = "PAYMENT-REQUIRED" as const;
export const PAYMENT_SIGNATURE_HEADER = "PAYMENT-SIGNATURE" as const;
export const PAYMENT_RESPONSE_HEADER = "PAYMENT-RESPONSE" as const;

const MAX_U128 = 340282366920938463463374607431768211455n;
const MAX_HEADER_BYTES = 64 * 1024;
const PAYMENT_KEY_DOMAIN = new TextEncoder().encode("LayerX/middleware/x402/idempotency\0");

export type JsonValue = null | boolean | number | string | JsonValue[] | { readonly [key: string]: JsonValue };

export interface ResourceInfo {
  readonly url: string;
  readonly description?: string;
  readonly mimeType?: string;
  readonly serviceName?: string;
  readonly tags?: readonly string[];
  readonly iconUrl?: string;
}

export interface X402Extension {
  readonly info: JsonValue;
  readonly schema: JsonValue;
}

export interface PaymentRequirements {
  readonly scheme: string;
  readonly network: string;
  readonly amount: string;
  readonly asset: string;
  readonly payTo: string;
  readonly maxTimeoutSeconds: number;
  readonly extra?: JsonValue;
}

export interface PaymentRequired {
  readonly x402Version: typeof X402_VERSION;
  readonly error?: string;
  readonly resource: ResourceInfo;
  readonly accepts: readonly PaymentRequirements[];
  readonly extensions?: Readonly<Record<string, X402Extension>>;
}

export interface PaymentPayload {
  readonly x402Version: typeof X402_VERSION;
  readonly resource?: ResourceInfo;
  readonly payload: Readonly<Record<string, JsonValue>>;
  readonly accepted: PaymentRequirements;
  readonly extensions?: Readonly<Record<string, X402Extension>>;
}

export interface PortableReceiptV2 {
  readonly format: "layerx-receipt-proof-v2";
  readonly verificationLevel: PaymentCommitment;
  readonly rawTransaction: string;
  readonly signedReceipt: string;
  readonly receiptDigest: string;
  readonly receiptKey: {
    readonly keyId: string;
    readonly publicKey: string;
    readonly firstHeight: string;
    readonly lastHeight: string;
  };
}

export interface PaymentReceiptVerification extends SequencerReceiptVerification {
  readonly rung: PaymentCommitment;
  readonly txHash: string;
  readonly payment: PaymentExactEvent;
  readonly commitmentEvidence?: PaymentCommitmentEvidence;
}

export interface SettlementResponse {
  readonly success: boolean;
  readonly errorReason?: string;
  readonly payer?: string;
  readonly transaction: string;
  readonly network: string;
  readonly amount?: string;
  readonly extensions?: Readonly<Record<string, JsonValue>>;
}

export type MiddlewareErrorCode =
  | "invalid-payment-required"
  | "invalid-payment-payload"
  | "requirements-mismatch"
  | "unsupported-payment"
  | "payment-pending"
  | "payment-refused"
  | "verification-failure"
  | "fulfillment-conflict"
  | "fulfillment-outcome-unknown"
  | "invalid-webhook"
  | "webhook-replay";

export class MiddlewareError extends Error {
  public constructor(public readonly code: MiddlewareErrorCode) {
    super(code);
    this.name = "MiddlewareError";
  }
}

export interface SellerSettlementRequest {
  readonly principal: string;
  readonly payload: PaymentPayload;
  readonly requirements: PaymentRequirements;
  readonly idempotencyKey: string;
  readonly requestDigest: string;
}

export type SellerSettlementOutcome =
  | { readonly kind: "pending" }
  | { readonly kind: "refused"; readonly reason: string }
  | (PortableReceiptV2 & { readonly kind: "settled" });

export interface SellerPaymentAuthority {
  settle(request: SellerSettlementRequest): Promise<SellerSettlementOutcome>;
}

export interface StoredFulfillment<T> extends PortableReceiptV2 {
  readonly idempotencyKey: string;
  readonly requestDigest: string;
  readonly txHash: string;
  readonly commitmentEvidence?: PaymentCommitmentEvidence;
  readonly resource: T;
}

export interface FulfillmentRepository<T> {
  get(idempotencyKey: string): Promise<StoredFulfillment<T> | undefined>;
  fulfill(
    proposed: Omit<StoredFulfillment<T>, "resource">,
    release: () => Promise<T>,
  ): Promise<StoredFulfillment<T>>;
}

export interface SellerMiddlewareConfig<T> {
  readonly paymentRequired: PaymentRequired;
  readonly receiptKeys: readonly SequencerReceiptKey[];
  readonly authority: SellerPaymentAuthority;
  readonly fulfillments: FulfillmentRepository<T>;
  readonly commitments?: PaymentCommitmentResolver;
}

export type SellerDecision<T> =
  | {
    readonly kind: "payment-required";
    readonly status: 402;
    readonly headers: Readonly<Record<typeof PAYMENT_REQUIRED_HEADER, string>>;
    readonly body: PaymentRequired;
  }
  | { readonly kind: "pending"; readonly status: 202 }
  | {
    readonly kind: "refused";
    readonly status: 402;
    readonly headers: Readonly<Record<typeof PAYMENT_RESPONSE_HEADER, string>>;
    readonly settlement: SettlementResponse;
  }
  | {
    readonly kind: "released";
    readonly status: 200;
    readonly headers: Readonly<Record<typeof PAYMENT_RESPONSE_HEADER, string>>;
    readonly settlement: SettlementResponse;
    readonly verification: PaymentReceiptVerification;
    readonly resource: T;
  };

export class ReceiptPayloadAuthority implements SellerPaymentAuthority {
  readonly #receiptKeys: readonly SequencerReceiptKey[];

  public constructor(receiptKeys: readonly SequencerReceiptKey[]) {
    this.#receiptKeys = copyReceiptKeys(receiptKeys);
  }

  public async settle(request: SellerSettlementRequest): Promise<SellerSettlementOutcome> {
    const evidence = parseLayerXEvidence(request.payload.payload);
    const signedReceipt = parseReceiptHex(evidence.signedReceipt);
    verifyReceiptDigest(evidence.receiptDigest, signedReceipt);
    const keys = matchingReceiptKeys(evidence.receiptKey, this.#receiptKeys);
    try {
      await verifySequencerReceipt(signedReceipt, parseReceiptHex(evidence.rawTransaction), keys);
    } catch (error) {
      if (error instanceof PlatformSdkError) throw new MiddlewareError("verification-failure");
      throw error;
    }
    return { kind: "settled", ...evidence };
  }
}

export class SellerMiddleware<T> {
  readonly #required: PaymentRequired;
  readonly #receiptKeys: readonly SequencerReceiptKey[];
  readonly #authority: SellerPaymentAuthority;
  readonly #fulfillments: FulfillmentRepository<T>;
  readonly #commitments: PaymentCommitmentResolver | undefined;

  public constructor(config: SellerMiddlewareConfig<T>) {
    this.#required = validatePaymentRequired(config.paymentRequired);
    this.#receiptKeys = copyReceiptKeys(config.receiptKeys);
    this.#authority = config.authority;
    this.#fulfillments = config.fulfillments;
    this.#commitments = config.commitments;
  }

  public paymentRequired(): Extract<SellerDecision<T>, { readonly kind: "payment-required" }> {
    return {
      kind: "payment-required",
      status: 402,
      headers: { [PAYMENT_REQUIRED_HEADER]: encodePaymentRequiredHeader(this.#required) },
      body: this.#required,
    };
  }

  public async handle(
    principal: string,
    paymentHeader: string | undefined,
    release: (idempotencyKey: string) => Promise<T>,
  ): Promise<SellerDecision<T>> {
    if (paymentHeader === undefined) {
      return this.paymentRequired();
    }
    if (!boundedText(principal, 512)) {
      throw new MiddlewareError("invalid-payment-payload");
    }
    const payload = decodePaymentPayloadHeader(paymentHeader);
    const requirements = matchRequirements(this.#required, payload);
    const canonical = canonicalJson(payload);
    const requestDigestBytes = await sha256(new TextEncoder().encode(canonical));
    const requestDigest = toHex(requestDigestBytes);
    const idempotencyKey = toHex(await sha256(
      PAYMENT_KEY_DOMAIN,
      new TextEncoder().encode(principal),
      requestDigestBytes,
    ));
    let stored = await this.#fulfillments.get(idempotencyKey);
    if (stored === undefined) {
      const outcome = await this.#authority.settle({
        principal,
        payload,
        requirements,
        idempotencyKey,
        requestDigest,
      });
      if (outcome.kind === "pending") {
        return { kind: "pending", status: 202 };
      }
      if (outcome.kind === "refused") {
        return refusalDecision(requirements, outcome.reason);
      }
      const proof: PortableReceiptV2 = structuredClone({
        format: outcome.format,
        verificationLevel: outcome.verificationLevel,
        rawTransaction: outcome.rawTransaction,
        signedReceipt: outcome.signedReceipt,
        receiptDigest: outcome.receiptDigest,
        receiptKey: outcome.receiptKey,
      });
      const verification = await verifyPaymentReceipt(proof, requirements, this.#receiptKeys, this.#commitments);
      const proposed = {
        ...proof,
        idempotencyKey,
        requestDigest,
        txHash: verification.txHash,
        ...(verification.commitmentEvidence === undefined ? {} : {
          commitmentEvidence: verification.commitmentEvidence,
        }),
      };
      stored = await this.#fulfillments.fulfill(proposed, () => release(idempotencyKey));
      if (stored.receiptDigest !== proposed.receiptDigest || stored.txHash !== proposed.txHash) {
        throw new MiddlewareError("fulfillment-conflict");
      }
    }
    if (stored.idempotencyKey !== idempotencyKey || stored.requestDigest !== requestDigest) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    const commitmentEvidence = stored.commitmentEvidence;
    if (stored.verificationLevel === "committed" && commitmentEvidence === undefined) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    const storedVerification = await verifyPaymentReceipt(stored, requirements, this.#receiptKeys,
      commitmentEvidence === undefined ? undefined : {
        resolve: async () => structuredClone(commitmentEvidence),
      });
    if (stored.txHash !== storedVerification.txHash) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    const evidence = {
      format: stored.format,
      verificationLevel: storedVerification.rung,
      rawTransaction: stored.rawTransaction,
      signedReceipt: stored.signedReceipt,
      receiptDigest: stored.receiptDigest,
      receiptKey: stored.receiptKey,
      ...(storedVerification.commitmentEvidence === undefined ? {} :
        portableCommitmentEvidence(storedVerification.commitmentEvidence)),
    };
    const settlement: SettlementResponse = {
      success: true,
      payer: toHex(storedVerification.receipt.from),
      transaction: stored.txHash,
      network: requirements.network,
      amount: requirements.amount,
      extensions: { layerx: evidence },
    };
    return {
      kind: "released",
      status: 200,
      headers: { [PAYMENT_RESPONSE_HEADER]: encodeSettlementHeader(settlement) },
      settlement,
      verification: storedVerification,
      resource: stored.resource,
    };
  }
}

export function platform_mw_seller(): "receipt-gated-x402-seller" {
  return "receipt-gated-x402-seller";
}

export function encodePaymentRequiredHeader(value: PaymentRequired): string {
  return encodeHeader(validatePaymentRequired(value));
}

export function decodePaymentRequiredHeader(value: string): PaymentRequired {
  return parsePaymentRequired(decodeHeader(value));
}

export function encodePaymentPayloadHeader(value: PaymentPayload): string {
  return encodeHeader(validatePaymentPayload(value));
}

export function decodePaymentPayloadHeader(value: string): PaymentPayload {
  return parsePaymentPayload(decodeHeader(value));
}

export function encodeSettlementHeader(value: SettlementResponse): string {
  return encodeHeader(validateSettlement(value));
}

export function decodeSettlementHeader(value: string): SettlementResponse {
  return parseSettlement(decodeHeader(value));
}

export async function verifyPaymentReceipt(
  evidence: PortableReceiptV2,
  requirements: PaymentRequirements,
  receiptKeys: readonly SequencerReceiptKey[],
  commitments?: PaymentCommitmentResolver,
): Promise<PaymentReceiptVerification> {
  const proof = parseLayerXEvidence({
    format: evidence.format,
    verificationLevel: evidence.verificationLevel,
    rawTransaction: evidence.rawTransaction,
    signedReceipt: evidence.signedReceipt,
    receiptDigest: evidence.receiptDigest,
    receiptKey: evidence.receiptKey,
  });
  const signedReceipt = parseReceiptHex(proof.signedReceipt);
  verifyReceiptDigest(proof.receiptDigest, signedReceipt);
  const keys = matchingReceiptKeys(proof.receiptKey, receiptKeys);
  let verified: SequencerReceiptVerification;
  try {
    verified = await verifySequencerReceipt(signedReceipt, parseReceiptHex(proof.rawTransaction), keys);
  } catch (error) {
    if (error instanceof PlatformSdkError) throw new MiddlewareError("verification-failure");
    throw error;
  }
  const { receipt } = verified;
  if (requirements.scheme !== "exact" || receipt.resultCode !== 0 || receipt.moduleId <= 11) {
    throw new MiddlewareError("verification-failure");
  }
  const summary = receipt.effects[0];
  if (summary === undefined || summary.moduleId !== receipt.moduleId || summary.ordinal !== 0
    || summary.kind !== 1 || summary.monetary || summary.eventType !== 0
    || summary.body.length < 110 || summary.body[108] === 0 || summary.body[108]! > 64
    || summary.body.length !== 109 + summary.body[108]!
    || new DataView(arrayBuffer(summary.body)).getUint32(104, false) !== receipt.effects.length - 1) {
    throw new MiddlewareError("verification-failure");
  }
  let payer: string | undefined;
  let reference: string | undefined;
  try {
    payer = paymentPayer(requirements.extra);
    reference = paymentPurpose(requirements.extra, true);
  } catch (error) {
    if (error instanceof PlatformSdkError) throw new MiddlewareError("verification-failure");
    throw error;
  }
  const amount = BigInt(parseAmount(requirements.amount));
  // Signed log effects retain the data hash, so authenticate the expected ABI
  // words against that hash before supplying them to the event decoder.
  const data = concatenate(parseHex32(requirements.asset), new Uint8Array(16), uint128Bytes(amount));
  const dataDigest = await sha256(data);
  const topic = keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"));
  const payments: PaymentExactEvent[] = [];
  for (const [index, effect] of receipt.effects.slice(1).entries()) {
    const body = effect.body;
    const topicCount = body[20];
    if (effect.moduleId !== receipt.moduleId || effect.ordinal !== index + 1
      || effect.kind !== 3 || effect.monetary || effect.eventType !== 0
      || topicCount === undefined || topicCount > 4 || body.length !== 53 + topicCount * 32) {
      throw new MiddlewareError("verification-failure");
    }
    const address = `0x${toHex(body.subarray(0, 20))}`;
    if (address !== LAYERX_LEDGER_ADDRESS || !equalBytes(body.subarray(21, 53), topic)) continue;
    if (!equalBytes(body.subarray(body.length - 32), dataDigest)) {
      throw new MiddlewareError("verification-failure");
    }
    const payment = decodePaymentExactLog({ address, data,
      topics: Array.from({ length: topicCount }, (_, i) => body.slice(21 + i * 32, 53 + i * 32)),
    });
    payments.push(payment);
  }
  const payment = payments[0];
  if (payments.length !== 1 || payment === undefined || payment.amount !== amount
    || !equalBytes(payment.assetId, parseHex32(requirements.asset))
    || !equalBytes(payment.to, parseHex32(requirements.payTo))
    || reference === undefined || !equalBytes(payment.paymentReference, parseHex32(reference))
    || (payer !== undefined && !equalBytes(payment.from, parseHex32(payer)))) {
    throw new MiddlewareError("verification-failure");
  }
  let commitmentEvidence: PaymentCommitmentEvidence | undefined;
  const rung = proof.verificationLevel;
  try {
    if (paymentCommitment(requirements.extra) === "committed" && rung !== "committed") {
      throw new MiddlewareError("verification-failure");
    }
    await verifyPaymentCommitment(verified, requirements.network, rung,
      commitments === undefined ? undefined : {
        resolve: async (transactionHash, network) => {
          commitmentEvidence = structuredClone(await commitments.resolve(transactionHash, network));
          return commitmentEvidence;
        },
      });
  } catch (error) {
    if (error instanceof PlatformSdkError) throw new MiddlewareError("verification-failure");
    throw error;
  }
  return Object.freeze({ ...verified, rung, txHash: `0x${toHex(receipt.activityId)}`, payment,
    ...(commitmentEvidence === undefined ? {} : { commitmentEvidence }),
  });
}

function portableCommitmentEvidence(evidence: PaymentCommitmentEvidence) {
  const hex = (value: Uint8Array): string => `0x${toHex(value)}`;
  const quantity = (value: bigint): string => `0x${value.toString(16)}`;
  const blockId = (value: PaymentCommitmentEvidence["blockHeader"]["blockId"]) => ({
    hash: hex(value.hash),
    partSetHeader: { total: value.partSetHeader.total, hash: hex(value.partSetHeader.hash) },
  });
  return {
    landingBlockHeight: quantity(evidence.landingBlockHeight),
    blockHeader: { chainId: evidence.blockHeader.chainId, blockId: blockId(evidence.blockHeader.blockId) },
    receiptInclusion: {
      transactionHash: hex(evidence.receiptInclusion.transactionHash),
      receiptDigest: hex(evidence.receiptInclusion.receiptDigest),
      sequence: quantity(evidence.receiptInclusion.sequence),
    },
    commit: {
      height: quantity(evidence.commit.height),
      round: evidence.commit.round,
      blockId: blockId(evidence.commit.blockId),
      signatures: evidence.commit.signatures.map((signature) => ({
        blockIdFlag: signature.blockIdFlag,
        validatorAddress: hex(signature.validatorAddress),
        timestamp: { seconds: quantity(signature.timestamp.seconds), nanos: signature.timestamp.nanos },
        signature: hex(signature.signature),
      })),
    },
    validatorSet: evidence.validatorSet.map((validator) => ({
      address: hex(validator.address),
      publicKey: hex(validator.publicKey),
      votingPower: quantity(validator.votingPower),
    })),
  };
}

export interface WebhookRequestHeaders {
  readonly id: string;
  readonly timestamp: string;
  readonly keyId: string;
  readonly signature: string;
}

export interface WebhookDeliveryClaim {
  readonly deliveryId: string;
  readonly payloadDigest: string;
  readonly leaseUntilMs: number;
}

export type WebhookClaimResult = "claimed" | "processing" | "completed" | "conflict";

export interface WebhookDeliveryStore {
  claim(value: WebhookDeliveryClaim): Promise<WebhookClaimResult>;
  complete(deliveryId: string, payloadDigest: string): Promise<void>;
  release(deliveryId: string, payloadDigest: string): Promise<void>;
}

export type WebhookConsumeResult = "processed" | "duplicate" | "processing";

export interface VerifiedWebhookConsumerConfig {
  readonly publicKeys: Readonly<Record<string, Uint8Array>>;
  readonly deliveries: WebhookDeliveryStore;
  readonly maximumAgeMs?: number;
  readonly leaseMs?: number;
  readonly now?: () => number;
}

export class VerifiedWebhookConsumer {
  readonly #keys: Readonly<Record<string, Uint8Array>>;
  readonly #deliveries: WebhookDeliveryStore;
  readonly #maximumAgeMs: number;
  readonly #leaseMs: number;
  readonly #now: () => number;

  public constructor(config: VerifiedWebhookConsumerConfig) {
    if (Object.keys(config.publicKeys).length === 0) {
      throw new MiddlewareError("invalid-webhook");
    }
    this.#keys = Object.freeze(Object.fromEntries(
      Object.entries(config.publicKeys).map(([keyId, publicKey]) => {
        if (!identifier(keyId, 64) || publicKey.length !== 32) {
          throw new MiddlewareError("invalid-webhook");
        }
        return [keyId, publicKey.slice()];
      }),
    ));
    this.#deliveries = config.deliveries;
    this.#maximumAgeMs = config.maximumAgeMs ?? 5 * 60 * 1000;
    this.#leaseMs = config.leaseMs ?? 60 * 1000;
    this.#now = config.now ?? Date.now;
    if (this.#maximumAgeMs <= 0 || this.#leaseMs <= 0) {
      throw new MiddlewareError("invalid-webhook");
    }
  }

  public async consume(
    rawBody: Uint8Array,
    headers: WebhookRequestHeaders,
    handle: (event: Readonly<Record<string, JsonValue>>, deliveryId: string) => Promise<void>,
  ): Promise<WebhookConsumeResult> {
    const now = this.#now();
    const timestampSeconds = parseCanonicalInteger(headers.timestamp);
    const timestampMs = Number(timestampSeconds) * 1000;
    if (
      !boundedText(headers.id, 255)
      || !identifier(headers.keyId, 64)
      || !Number.isSafeInteger(timestampMs)
      || timestampMs > now + 30_000
      || now - timestampMs > this.#maximumAgeMs
    ) {
      throw new MiddlewareError("invalid-webhook");
    }
    const publicKey = this.#keys[headers.keyId];
    if (publicKey === undefined || publicKey.length !== 32) {
      throw new MiddlewareError("invalid-webhook");
    }
    const signature = parseWebhookSignature(headers.signature);
    const prefix = new TextEncoder().encode(`${headers.id}.${headers.timestamp}.`);
    let verified = false;
    try {
      const key = await globalThis.crypto.subtle.importKey("raw", arrayBuffer(publicKey), "Ed25519", false, ["verify"]);
      verified = await globalThis.crypto.subtle.verify(
        "Ed25519",
        key,
        arrayBuffer(signature),
        arrayBuffer(concatenate(prefix, rawBody)),
      );
    } catch {
      verified = false;
    }
    if (!verified) {
      throw new MiddlewareError("invalid-webhook");
    }
    const payloadDigest = toHex(await sha256(rawBody));
    const claim = await this.#deliveries.claim({
      deliveryId: headers.id,
      payloadDigest,
      leaseUntilMs: now + this.#leaseMs,
    });
    if (claim === "conflict") {
      throw new MiddlewareError("webhook-replay");
    }
    if (claim === "completed") {
      return "duplicate";
    }
    if (claim === "processing") {
      return "processing";
    }
    let event: Readonly<Record<string, JsonValue>>;
    try {
      event = asObject(JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(rawBody)), "invalid-webhook");
      await handle(event, headers.id);
      await this.#deliveries.complete(headers.id, payloadDigest);
    } catch (error) {
      await this.#deliveries.release(headers.id, payloadDigest);
      throw error;
    }
    return "processed";
  }
}

function validatePaymentRequired(value: PaymentRequired): PaymentRequired {
  return parsePaymentRequired(value as unknown);
}

function validatePaymentPayload(value: PaymentPayload): PaymentPayload {
  return parsePaymentPayload(value as unknown);
}

function validateSettlement(value: SettlementResponse): SettlementResponse {
  return parseSettlement(value as unknown);
}

function parsePaymentRequired(value: unknown): PaymentRequired {
  const object = asObject(value, "invalid-payment-required");
  exactKeys(object, ["x402Version", "resource", "accepts"], ["error", "extensions"], "invalid-payment-required");
  if (object["x402Version"] !== X402_VERSION) {
    throw new MiddlewareError("invalid-payment-required");
  }
  const accepts = asArray(object["accepts"], "invalid-payment-required").map(parseRequirements);
  if (accepts.length === 0 || accepts.length > 32) {
    throw new MiddlewareError("invalid-payment-required");
  }
  return {
    x402Version: X402_VERSION,
    resource: parseResource(object["resource"]),
    accepts,
    ...(object["error"] === undefined ? {} : { error: asBoundedString(object["error"], 512, "invalid-payment-required") }),
    ...(object["extensions"] === undefined ? {} : { extensions: parseExtensions(object["extensions"]) }),
  };
}

function parsePaymentPayload(value: unknown): PaymentPayload {
  const object = asObject(value, "invalid-payment-payload");
  exactKeys(object, ["x402Version", "payload", "accepted"], ["resource", "extensions"], "invalid-payment-payload");
  if (object["x402Version"] !== X402_VERSION) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  return {
    x402Version: X402_VERSION,
    payload: asObject(object["payload"], "invalid-payment-payload"),
    accepted: parseRequirements(object["accepted"]),
    ...(object["resource"] === undefined ? {} : { resource: parseResource(object["resource"]) }),
    ...(object["extensions"] === undefined ? {} : { extensions: parseExtensions(object["extensions"]) }),
  };
}

function parseSettlement(value: unknown): SettlementResponse {
  const object = asObject(value, "invalid-payment-payload");
  exactKeys(
    object,
    ["success", "transaction", "network"],
    ["errorReason", "payer", "amount", "extensions"],
    "invalid-payment-payload",
  );
  if (typeof object["success"] !== "boolean") {
    throw new MiddlewareError("invalid-payment-payload");
  }
  const transaction = asString(object["transaction"], "invalid-payment-payload");
  const errorReason = object["errorReason"] === undefined
    ? undefined
    : asBoundedString(object["errorReason"], 512, "invalid-payment-payload");
  if (object["success"] ? transaction.length === 0 || errorReason !== undefined : errorReason === undefined) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  if (!object["success"] && errorReason !== "settlement_pending" && transaction.length !== 0) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  if (!object["success"] && errorReason === "settlement_pending" && transaction.length === 0) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  if (object["success"] && (typeof object["payer"] !== "string"
    || !/^[0-9a-f]{64}$/u.test(object["payer"]) || /^0+$/u.test(object["payer"]))) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  return {
    success: object["success"],
    transaction,
    network: parseNetwork(object["network"]),
    ...(errorReason === undefined ? {} : { errorReason }),
    ...(object["payer"] === undefined ? {} : { payer: asBoundedString(object["payer"], 256, "invalid-payment-payload") }),
    ...(object["amount"] === undefined ? {} : { amount: parseAmount(object["amount"]) }),
    ...(object["extensions"] === undefined ? {} : { extensions: asJsonRecord(object["extensions"], "invalid-payment-payload") }),
  };
}

function parseResource(value: unknown): ResourceInfo {
  const object = asObject(value, "invalid-payment-required");
  exactKeys(object, ["url"], ["description", "mimeType", "serviceName", "tags", "iconUrl"], "invalid-payment-required");
  const url = parseUrl(object["url"]);
  const tags = object["tags"] === undefined
    ? undefined
    : asArray(object["tags"], "invalid-payment-required").map((tag) => asPrintableString(tag, 32));
  if (tags !== undefined && tags.length > 5) {
    throw new MiddlewareError("invalid-payment-required");
  }
  return {
    url,
    ...(object["description"] === undefined ? {} : { description: asBoundedString(object["description"], 512, "invalid-payment-required") }),
    ...(object["mimeType"] === undefined ? {} : { mimeType: asBoundedString(object["mimeType"], 32, "invalid-payment-required") }),
    ...(object["serviceName"] === undefined ? {} : { serviceName: asPrintableString(object["serviceName"], 32) }),
    ...(tags === undefined ? {} : { tags }),
    ...(object["iconUrl"] === undefined ? {} : { iconUrl: parseUrl(object["iconUrl"]) }),
  };
}

function parseRequirements(value: unknown): PaymentRequirements {
  const object = asObject(value, "invalid-payment-required");
  exactKeys(
    object,
    ["scheme", "network", "amount", "asset", "payTo", "maxTimeoutSeconds"],
    ["extra"],
    "invalid-payment-required",
  );
  const scheme = asIdentifier(object["scheme"], 32, "invalid-payment-required");
  if (!["exact", "metered", "subscription"].includes(scheme)) throw new MiddlewareError("unsupported-payment");
  const extra = object["extra"];
  try {
    paymentCommitment(extra);
    paymentPayer(extra, scheme !== "exact");
    paymentPurpose(extra, scheme !== "exact");
    paymentAccount(extra);
    paymentCurrency(extra);
  } catch (error) {
    if (error instanceof PlatformSdkError) throw new MiddlewareError("invalid-payment-required");
    throw error;
  }
  if (scheme !== "exact") {
    const terms = asObject(asObject(extra, "invalid-payment-required")["layerx"], "invalid-payment-required");
    const window = terms["windowSeconds"];
    if (scheme === "subscription" ? typeof window !== "string" || !/^[1-9][0-9]{0,19}$/u.test(window) || BigInt(window) > 0xffff_ffff_ffff_ffffn : window !== undefined) throw new MiddlewareError("invalid-payment-required");
  }
  const maxTimeoutSeconds = object["maxTimeoutSeconds"];
  if (!Number.isSafeInteger(maxTimeoutSeconds) || (maxTimeoutSeconds as number) <= 0 || (maxTimeoutSeconds as number) > 0xffff_ffff) {
    throw new MiddlewareError("invalid-payment-required");
  }
  const asset = asBoundedString(object["asset"], 256, "invalid-payment-required");
  const payTo = asBoundedString(object["payTo"], 256, "invalid-payment-required");
  parseHex32(asset);
  parseHex32(payTo);
  return {
    scheme,
    network: parseNetwork(object["network"]),
    amount: parseAmount(object["amount"]),
    asset,
    payTo,
    maxTimeoutSeconds: maxTimeoutSeconds as number,
    ...(object["extra"] === undefined ? {} : { extra: asJson(object["extra"], "invalid-payment-required") }),
  };
}

function parseExtensions(value: unknown): Readonly<Record<string, X402Extension>> {
  const object = asObject(value, "invalid-payment-required");
  const entries = Object.entries(object);
  if (entries.length > 32) {
    throw new MiddlewareError("invalid-payment-required");
  }
  return Object.fromEntries(entries.map(([name, extension]) => {
    if (!identifier(name, 32)) {
      throw new MiddlewareError("invalid-payment-required");
    }
    const body = asObject(extension, "invalid-payment-required");
    exactKeys(body, ["info", "schema"], [], "invalid-payment-required");
    return [name, { info: asJson(body["info"], "invalid-payment-required"), schema: asJson(body["schema"], "invalid-payment-required") }];
  }));
}

function matchRequirements(required: PaymentRequired, payload: PaymentPayload): PaymentRequirements {
  const match = required.accepts.find((candidate) => canonicalJson(candidate) === canonicalJson(payload.accepted));
  if (match === undefined) {
    throw new MiddlewareError("requirements-mismatch");
  }
  for (const [name, extension] of Object.entries(required.extensions ?? {})) {
    const actual = payload.extensions?.[name];
    if (actual === undefined || canonicalJson(actual) !== canonicalJson(extension)) {
      throw new MiddlewareError("requirements-mismatch");
    }
  }
  return match;
}

function refusalDecision<T>(
  requirements: PaymentRequirements,
  reason: string,
): Extract<SellerDecision<T>, { readonly kind: "refused" }> {
  const safeReason = /^[a-z][a-z0-9_]{0,63}$/u.test(reason) ? reason : "payment_refused";
  const settlement: SettlementResponse = {
    success: false,
    errorReason: safeReason,
    transaction: "",
    network: requirements.network,
  };
  return {
    kind: "refused",
    status: 402,
    headers: { [PAYMENT_RESPONSE_HEADER]: encodeSettlementHeader(settlement) },
    settlement,
  };
}

function parseLayerXEvidence(value: unknown): PortableReceiptV2 {
  const object = asObject(value, "invalid-payment-payload");
  exactKeys(object, ["format", "verificationLevel", "rawTransaction", "signedReceipt", "receiptDigest", "receiptKey"], [], "invalid-payment-payload");
  if (object["format"] !== "layerx-receipt-proof-v2"
    || (object["verificationLevel"] !== "receipted" && object["verificationLevel"] !== "committed")) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  const key = asObject(object["receiptKey"], "invalid-payment-payload");
  exactKeys(key, ["keyId", "publicKey", "firstHeight", "lastHeight"], [], "invalid-payment-payload");
  const hex = (value: unknown, size?: number): string => {
    const text = asString(value, "invalid-payment-payload");
    parseReceiptHex(text, size);
    return text;
  };
  const firstHeight = asString(key["firstHeight"], "invalid-payment-payload");
  const lastHeight = asString(key["lastHeight"], "invalid-payment-payload");
  if (receiptHeight(firstHeight) > receiptHeight(lastHeight)) throw new MiddlewareError("invalid-payment-payload");
  return {
    format: "layerx-receipt-proof-v2",
    verificationLevel: object["verificationLevel"],
    rawTransaction: hex(object["rawTransaction"]),
    signedReceipt: hex(object["signedReceipt"]),
    receiptDigest: hex(object["receiptDigest"], 32),
    receiptKey: { keyId: hex(key["keyId"], 32), publicKey: hex(key["publicKey"], 32), firstHeight, lastHeight },
  };
}

function parseReceiptHex(value: string, size?: number): Uint8Array {
  if (value.length > MAX_HEADER_BYTES * 2 || !/^0x(?:[0-9a-f]{2})+$/u.test(value)
    || (size !== undefined && value.length !== 2 + size * 2)) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  return Uint8Array.from(value.slice(2).match(/../gu)!, (pair) => Number.parseInt(pair, 16));
}

function receiptHeight(value: string): bigint {
  if (!/^0x(?:0|[1-9a-f][0-9a-f]{0,15})$/u.test(value)) throw new MiddlewareError("invalid-payment-payload");
  return BigInt(value);
}

function verifyReceiptDigest(digest: string, signedReceipt: Uint8Array): void {
  if (!constantTimeEqualHex(digest, `0x${toHex(keccak256(signedReceipt))}`)) {
    throw new MiddlewareError("verification-failure");
  }
}

function copyReceiptKeys(keys: readonly SequencerReceiptKey[]): readonly SequencerReceiptKey[] {
  if (!Array.isArray(keys) || keys.length === 0) throw new MiddlewareError("verification-failure");
  return Object.freeze(keys.map((key) => {
    if (!(key.keyId instanceof Uint8Array) || key.keyId.length !== 32
      || !(key.publicKey instanceof Uint8Array) || key.publicKey.length !== 32
      || typeof key.first_height !== "bigint" || typeof key.last_height !== "bigint"
      || key.first_height < 0n || key.last_height > 0xffff_ffff_ffff_ffffn || key.first_height > key.last_height) {
      throw new MiddlewareError("verification-failure");
    }
    return Object.freeze({ ...key, keyId: key.keyId.slice(), publicKey: key.publicKey.slice() });
  }));
}

function matchingReceiptKeys(key: PortableReceiptV2["receiptKey"], keys: readonly SequencerReceiptKey[]): readonly SequencerReceiptKey[] {
  const pinned = copyReceiptKeys(keys).filter((pin) => equalBytes(pin.keyId, parseReceiptHex(key.keyId, 32))
    && equalBytes(pin.publicKey, parseReceiptHex(key.publicKey, 32))
    && pin.first_height === receiptHeight(key.firstHeight) && pin.last_height === receiptHeight(key.lastHeight));
  if (pinned.length === 0) throw new MiddlewareError("verification-failure");
  return pinned;
}

function uint128Bytes(value: bigint): Uint8Array {
  const bytes = new Uint8Array(16);
  for (let index = 15; index >= 0; index -= 1) {
    bytes[index] = Number(value & 255n);
    value >>= 8n;
  }
  return bytes;
}

function encodeHeader(value: unknown): string {
  const bytes = new TextEncoder().encode(JSON.stringify(value));
  if (bytes.length > MAX_HEADER_BYTES) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  return encodeBase64(bytes);
}

function decodeHeader(value: string): unknown {
  const bytes = decodeBase64(value);
  if (bytes.length > MAX_HEADER_BYTES) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  try {
    return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
  } catch {
    throw new MiddlewareError("invalid-payment-payload");
  }
}

function encodeBase64(value: Uint8Array): string {
  let binary = "";
  for (let offset = 0; offset < value.length; offset += 0x8000) {
    binary += String.fromCharCode(...value.subarray(offset, Math.min(offset + 0x8000, value.length)));
  }
  return btoa(binary);
}

function decodeBase64(value: string): Uint8Array {
  if (value.length === 0 || value.length > MAX_HEADER_BYTES * 2 || !/^[A-Za-z0-9+/]*={0,2}$/u.test(value)) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  try {
    const binary = atob(value);
    return Uint8Array.from(binary, (character) => character.charCodeAt(0));
  } catch {
    throw new MiddlewareError("invalid-payment-payload");
  }
}

function parseWebhookSignature(value: string): Uint8Array {
  const encoded = value.startsWith("v1=") ? value.slice(3) : "";
  const signature = decodeBase64(encoded);
  if (signature.length !== 64) {
    throw new MiddlewareError("invalid-webhook");
  }
  return signature;
}

function canonicalJson(value: unknown): string {
  if (value === null || typeof value === "boolean" || typeof value === "string") {
    return JSON.stringify(value);
  }
  if (typeof value === "number") {
    if (!Number.isFinite(value)) {
      throw new MiddlewareError("invalid-payment-payload");
    }
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map(canonicalJson).join(",")}]`;
  }
  if (typeof value === "object" && value !== null) {
    return `{${Object.entries(value as Record<string, unknown>)
      .filter(([, item]) => item !== undefined)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, item]) => `${JSON.stringify(key)}:${canonicalJson(item)}`)
      .join(",")}}`;
  }
  throw new MiddlewareError("invalid-payment-payload");
}

function asJson(value: unknown, code: MiddlewareErrorCode): JsonValue {
  canonicalJson(value);
  return value as JsonValue;
}

function asJsonRecord(value: unknown, code: MiddlewareErrorCode): Readonly<Record<string, JsonValue>> {
  const object = asObject(value, code);
  for (const item of Object.values(object)) {
    asJson(item, code);
  }
  return object;
}

function asObject(value: unknown, code: MiddlewareErrorCode): Readonly<Record<string, JsonValue>> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new MiddlewareError(code);
  }
  return value as Readonly<Record<string, JsonValue>>;
}

function asArray(value: unknown, code: MiddlewareErrorCode): readonly unknown[] {
  if (!Array.isArray(value)) {
    throw new MiddlewareError(code);
  }
  return value;
}

function exactKeys(
  value: Readonly<Record<string, JsonValue>>,
  required: readonly string[],
  optional: readonly string[],
  code: MiddlewareErrorCode,
): void {
  const allowed = new Set([...required, ...optional]);
  if (required.some((key) => value[key] === undefined) || Object.keys(value).some((key) => !allowed.has(key))) {
    throw new MiddlewareError(code);
  }
}

function asString(value: unknown, code: MiddlewareErrorCode): string {
  if (typeof value !== "string") {
    throw new MiddlewareError(code);
  }
  return value;
}

function asBoundedString(value: unknown, limit: number, code: MiddlewareErrorCode): string {
  const text = asString(value, code);
  if (!boundedText(text, limit)) {
    throw new MiddlewareError(code);
  }
  return text;
}

function asPrintableString(value: unknown, limit: number): string {
  const text = asBoundedString(value, limit, "invalid-payment-required");
  if (!/^[\x20-\x7e]+$/u.test(text)) {
    throw new MiddlewareError("invalid-payment-required");
  }
  return text;
}

function asIdentifier(value: unknown, limit: number, code: MiddlewareErrorCode): string {
  const text = asString(value, code);
  if (!identifier(text, limit)) {
    throw new MiddlewareError(code);
  }
  return text;
}

function parseNetwork(value: unknown): string {
  const text = asString(value, "invalid-payment-required");
  const chainId = /^eip155:([1-9][0-9]{0,9})$/u.exec(text)?.[1];
  if (chainId === undefined || Number(chainId) > 0xffff_ffff) {
    throw new MiddlewareError("unsupported-payment");
  }
  return text;
}

function parseUrl(value: unknown): string {
  const text = asBoundedString(value, 2048, "invalid-payment-required");
  let url: URL;
  try {
    url = new URL(text);
  } catch {
    throw new MiddlewareError("invalid-payment-required");
  }
  if ((url.protocol !== "https:" && url.protocol !== "http:") || /[\r\n\0]/u.test(text)) {
    throw new MiddlewareError("invalid-payment-required");
  }
  return text;
}

function parseAmount(value: unknown): string {
  const text = asString(value, "invalid-payment-required");
  if (!/^(0|[1-9][0-9]*)$/u.test(text)) {
    throw new MiddlewareError("invalid-payment-required");
  }
  const amount = BigInt(text);
  if (amount <= 0n || amount > MAX_U128) {
    throw new MiddlewareError("invalid-payment-required");
  }
  return text;
}

function parseCanonicalInteger(value: string): bigint {
  if (!/^(0|[1-9][0-9]*)$/u.test(value)) {
    throw new MiddlewareError("invalid-webhook");
  }
  return BigInt(value);
}

function boundedText(value: string, limit: number): boolean {
  return value.length > 0 && value.length <= limit && !value.includes("\0");
}

function identifier(value: string, limit: number): boolean {
  return boundedText(value, limit) && /^[A-Za-z0-9._-]+$/u.test(value);
}

function parseHex32(value: string): Uint8Array {
  const digits = value.startsWith("0x") ? value.slice(2) : value;
  if (!/^[0-9a-fA-F]{64}$/u.test(digits)) {
    throw new MiddlewareError("invalid-payment-required");
  }
  return Uint8Array.from({ length: 32 }, (_, index) => Number.parseInt(digits.slice(index * 2, index * 2 + 2), 16));
}

function toHex(value: Uint8Array): string {
  return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

function constantTimeEqualHex(left: string, right: string): boolean {
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

function arrayBuffer(value: Uint8Array): ArrayBuffer {
  const result = new ArrayBuffer(value.length);
  new Uint8Array(result).set(value);
  return result;
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

async function sha256(...values: readonly Uint8Array[]): Promise<Uint8Array> {
  return new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", arrayBuffer(concatenate(...values))));
}


export * from "./rpc.js";
export * from "./grant.js";
export * from "./checkpoint.js";
