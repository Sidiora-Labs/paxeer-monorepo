import {
  PlatformSdkError,
  ProductionClient,
  SecretBytes,
  idempotencyKey,
  decodeRawEip1559,
  recoverEip1559Sender,
  keccak256,
  verifySequencerReceipt,
  type ProductionTransport,
  type SequencerReceiptKey,
  type TransportCall,
} from "@sidiora/layerx-sdk";
import {
  MiddlewareError,
  PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  X402_VERSION,
  decodePaymentRequiredHeader,
  decodePaymentPayloadHeader,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
  verifyPaymentReceipt,
  paymentAccount,
  paymentCommitment,
  paymentCurrency,
  paymentPayer,
  paymentPurpose,
  decodePaymentDrawnLog,
  LAYERX_LEDGER_ADDRESS,
  LEDGER_DRAW_SELECTOR,
  verifyPaymentCommitment,
  type PaymentCommitmentEvidence,
  type PaymentDrawnEvent,
  type PaymentCommitmentResolver,
  type JsonValue,
  type PaymentReceiptVerification,
  type PortableReceiptV2,
  type PaymentPayload,
  type PaymentRequired,
  type PaymentRequirements,
  type ResourceInfo,
  type SettlementResponse,
} from "@sidiora/layerx-seller-middleware";

const MAX_U128 = 340282366920938463463374607431768211455n;
const LEGACY_ACCOUNT_ID_DOMAIN = new TextEncoder().encode("LXP/v1/account-id\0");
const NATIVE_ACCOUNT_ID_DOMAIN = new TextEncoder().encode("LX:ACCOUNT:v1");
const MAXIMUM_HTTP_RESPONSE_BYTES = 8 * 1024 * 1024;
const HTTP_REQUEST_TIMEOUT_MS = 30_000;

export interface BuyerSupportedKind {
  readonly scheme: string;
  readonly network: string;
}

export interface BuyerRetryPolicy {
  readonly maximumQuoteAttempts: number;
  readonly maximumCommitAttempts: number;
  readonly maximumJourneyPolls: number;
  readonly baseDelayMs: number;
  readonly wait: (delayMs: number) => Promise<void>;
}

export interface BuyerMiddlewareConfig {
  readonly client: ProductionClient;
  readonly source: string;
  readonly supported: readonly BuyerSupportedKind[];
  readonly receiptKeys: readonly SequencerReceiptKey[];
  readonly commitments?: PaymentCommitmentResolver;
  readonly retry?: Partial<BuyerRetryPolicy>;
  readonly now?: () => number;
  readonly fetch?: typeof globalThis.fetch;
}

export interface ParsedOffer {
  readonly required: PaymentRequired;
  readonly accepted: PaymentRequirements;
}

export interface MoveQuoteRequest {
  readonly source: string;
  readonly destination: string;
  readonly money: { readonly amount: string; readonly currency: string };
}

export interface PaymentQuoteTerms {
  readonly account: string;
  readonly currency: string;
}

export interface MoveQuote {
  readonly quote_id: string;
  readonly description_copy_key: string;
  readonly mechanism: "fund" | "allocate" | "return" | "transfer";
  readonly money: { readonly amount: string; readonly currency: string };
  readonly fee_estimate: { readonly amount: string; readonly currency: string };
  readonly fee_ceiling: { readonly amount: string; readonly currency: string };
  readonly arrival_estimate: string;
  readonly expires_at: string;
  readonly irreversibility_copy_key?: string;
}

export type JourneyState =
  | "getting-ready"
  | "sending"
  | "processing"
  | "done"
  | "done-finalised"
  | "still-checking"
  | "refused"
  | "waiting-for-you";

export interface EvidenceReference {
  readonly evidence_id: string;
  readonly class: string;
  readonly verification: string;
}

export interface Journey {
  readonly journey_id: string;
  readonly kind: string;
  readonly state: JourneyState;
  readonly state_copy_key: string;
  readonly evidence: readonly EvidenceReference[];
  readonly stages: readonly {
    readonly stage_id: string;
    readonly copy_key: string;
    readonly state: JourneyState;
    readonly evidence: readonly EvidenceReference[];
  }[];
  readonly started_at: string;
  readonly updated_at: string;
  readonly refusal?: {
    readonly refused_by: string;
    readonly copy_key: string;
    readonly money_left: boolean;
  };
}

export interface EvidenceMaterial {
  readonly evidence_id: string;
  readonly class: string;
  readonly verification: string;
  readonly content_type: string;
  readonly bytes_base64: string;
}

export interface PreparedPayment extends PortableReceiptV2 {
  readonly offer: ParsedOffer;
  readonly quote: MoveQuote;
  readonly journey: Journey;
  readonly payload: PaymentPayload;
  readonly paymentHeader: string;
  readonly idempotencyKey: string;
  readonly verification: PaymentReceiptVerification;
  readonly txHash: string;
}

export type PaymentPreparation =
  | { readonly kind: "ready"; readonly payment: PreparedPayment }
  | { readonly kind: "pending"; readonly journey: Journey }
  | { readonly kind: "unknown"; readonly idempotencyKey: string; readonly journey?: Journey }
  | { readonly kind: "refused"; readonly journey: Journey };

export interface CapturedSettlement extends PortableReceiptV2 {
  readonly response: SettlementResponse;
  readonly verification: PaymentReceiptVerification;
  readonly txHash: string;
}

export interface CapturedGrantSettlement extends PortableReceiptV2 {
  readonly response: SettlementResponse;
  readonly verification: Omit<PaymentReceiptVerification, "payment"> & { readonly payment: PaymentDrawnEvent };
  readonly txHash: string;
}

export type PaidFetchResult =
  | { readonly kind: "not-payment-required"; readonly response: Response }
  | { readonly kind: "pending"; readonly journey: Journey }
  | { readonly kind: "unknown"; readonly idempotencyKey: string; readonly journey?: Journey }
  | { readonly kind: "refused"; readonly journey: Journey }
  | {
    readonly kind: "paid";
    readonly response: Response;
    readonly payment: PreparedPayment;
    readonly settlement: CapturedSettlement;
  };

export class BuyerMiddleware {
  readonly #client: ProductionClient;
  readonly #source: string;
  readonly #supported: readonly BuyerSupportedKind[];
  readonly #receiptKeys: readonly SequencerReceiptKey[];
  readonly #commitments: PaymentCommitmentResolver | undefined;
  readonly #retry: BuyerRetryPolicy;
  readonly #now: () => number;
  readonly #fetch: typeof globalThis.fetch;

  public constructor(config: BuyerMiddlewareConfig) {
    if (config.source.length === 0 || config.source.length > 512 || config.supported.length === 0) {
      throw new MiddlewareError("unsupported-payment");
    }
    const seen = new Set<string>();
    for (const kind of config.supported) {
      const key = `${kind.scheme}\0${kind.network}`;
      const chainId = /^eip155:([1-9][0-9]{0,9})$/u.exec(kind.network)?.[1];
      if (kind.scheme.length === 0 || chainId === undefined || Number(chainId) > 0xffff_ffff || seen.has(key)) {
        throw new MiddlewareError("unsupported-payment");
      }
      seen.add(key);
    }
    this.#client = config.client;
    this.#source = config.source;
    this.#supported = structuredClone(config.supported);
    this.#receiptKeys = structuredClone(config.receiptKeys);
    this.#commitments = config.commitments;
    this.#retry = retryPolicy(config.retry);
    this.#now = config.now ?? Date.now;
    this.#fetch = config.fetch ?? globalThis.fetch;
  }

  public parseOffer(paymentRequiredHeader: string): ParsedOffer {
    const required = decodePaymentRequiredHeader(paymentRequiredHeader);
    const accepted = required.accepts.find((candidate) => this.#supported.some(
      (kind) => kind.scheme === candidate.scheme && kind.network === candidate.network,
    ));
    if (accepted === undefined) {
      throw new MiddlewareError("unsupported-payment");
    }
    if (paymentCommitment(accepted.extra) !== "receipted" && this.#commitments === undefined) {
      throw new MiddlewareError("unsupported-payment");
    }
    return { required, accepted };
  }

  public async prepare(paymentRequiredHeader: string, callerIdempotencyKey: string): Promise<PaymentPreparation> {
    const offer = this.parseOffer(paymentRequiredHeader);
    if (offer.accepted.scheme !== "exact") throw new MiddlewareError("unsupported-payment");
    const terms = paymentQuoteTerms(offer.accepted);
    if (!(await accountNamesPayTo(terms.account, offer.accepted.payTo, offer.accepted.asset))) {
      throw new MiddlewareError("requirements-mismatch");
    }
    const mutationKey = idempotencyKey(callerIdempotencyKey);
    const quote = await this.#retrySdk(
      () => this.#client.human<MoveQuoteRequest, unknown>("move.quote", {
        source: this.#source,
        destination: terms.account,
        money: { amount: offer.accepted.amount, currency: terms.currency },
      }),
      this.#retry.maximumQuoteAttempts,
      false,
    );
    const parsedQuote = parseMoveQuote(quote);
    if (
      parsedQuote.money.amount !== offer.accepted.amount
      || !constantTimeEqualText(parsedQuote.money.currency, terms.currency)
      || Date.parse(parsedQuote.expires_at) <= this.#now()
    ) {
      throw new MiddlewareError("requirements-mismatch");
    }
    let rawJourney: unknown;
    try {
      rawJourney = await this.#retrySdk(
        () => this.#client.human("move.commit", { quote_id: parsedQuote.quote_id }, { idempotencyKey: mutationKey }),
        this.#retry.maximumCommitAttempts,
        true,
      );
    } catch (error) {
      if (error instanceof PlatformSdkError && error.retry === "unknown-outcome") {
        return { kind: "unknown", idempotencyKey: callerIdempotencyKey };
      }
      throw error;
    }
    let journey = parseJourney(rawJourney);
    for (let poll = 0; poll < this.#retry.maximumJourneyPolls && isProgress(journey.state); poll += 1) {
      await this.#retry.wait(this.#retry.baseDelayMs * Math.min(poll + 1, 10));
      journey = parseJourney(await this.#retrySdk(
        () => this.#client.human("journey.get", { journey_id: journey.journey_id }),
        this.#retry.maximumQuoteAttempts,
        false,
      ));
    }
    if (journey.state === "still-checking") {
      return { kind: "unknown", idempotencyKey: callerIdempotencyKey, journey };
    }
    if (journey.state === "refused") {
      return { kind: "refused", journey };
    }
    if (journey.state !== "done" && journey.state !== "done-finalised") {
      return { kind: "pending", journey };
    }
    const receipt = await this.#findMatchingReceipt(journey, offer.accepted);
    if (receipt === undefined) return { kind: "pending", journey };
    const payload: PaymentPayload = {
      x402Version: X402_VERSION,
      resource: offer.required.resource,
      payload: {
        format: receipt.format,
        rawTransaction: receipt.rawTransaction,
        signedReceipt: receipt.signedReceipt,
        receiptDigest: receipt.receiptDigest,
        receiptKey: { ...receipt.receiptKey },
        verificationLevel: receipt.verificationLevel,
      },
      accepted: offer.accepted,
      extensions: offer.required.extensions ?? {},
    };
    return {
      kind: "ready",
      payment: {
        offer,
        quote: parsedQuote,
        journey,
        payload,
        paymentHeader: encodePaymentPayloadHeader(payload),
        idempotencyKey: callerIdempotencyKey,
        ...receipt,
      },
    };
  }

  public grantHeader(paymentRequiredHeader: string, rawTransaction: string): string {
    const offer = this.parseOffer(paymentRequiredHeader);
    return grantPaymentHeader(offer.required, offer.accepted, rawTransaction);
  }

  public async fetchGrant(input: RequestInfo | URL, init: RequestInit, rawTransaction: string, expectedTransactionHash: string): Promise<{ readonly response: Response; readonly settlement?: CapturedGrantSettlement }> {
    if (!/^0x[0-9a-f]{64}$/u.test(expectedTransactionHash)) throw new MiddlewareError("invalid-payment-payload");
    const initial = await this.#fetch(input, init);
    if (initial.status !== 402) return { response: initial };
    const required = initial.headers.get(PAYMENT_REQUIRED_HEADER);
    await initial.body?.cancel();
    if (required === null) throw new MiddlewareError("invalid-payment-required");
    const paymentHeader = this.grantHeader(required, rawTransaction);
    if (`0x${toHex(keccak256(grantTransactionBytes(rawTransaction)))}` !== expectedTransactionHash) {
      throw new MiddlewareError("invalid-payment-payload");
    }
    const headers = new Headers(init.headers);
    headers.set(PAYMENT_SIGNATURE_HEADER, paymentHeader);
    const response = await this.#fetch(input, { ...init, headers });
    if (response.status === 202) return { response };
    const responseHeader = response.headers.get(PAYMENT_RESPONSE_HEADER);
    if (responseHeader === null) { await response.body?.cancel(); throw new MiddlewareError("verification-failure"); }
    const settlement = await this.captureGrantSettlement(responseHeader, paymentHeader, expectedTransactionHash);
    return { response, settlement };
  }

  public async captureGrantSettlement(responseHeader: string, paymentHeader: string, expectedTransactionHash: string): Promise<CapturedGrantSettlement> {
    const payload = decodePaymentPayloadHeader(paymentHeader);
    const rawTransaction = payload.payload["rawTransaction"];
    if (Object.keys(payload.payload).length !== 2 || typeof rawTransaction !== "string"
      || !/^0x[0-9a-f]{64}$/u.test(expectedTransactionHash)) throw new MiddlewareError("invalid-payment-payload");
    const { raw, data, asset } = grantDrawTransaction(payload.accepted, rawTransaction);
    if (payload.payload["idempotencyKey"] !== toHex(data.subarray(100, 132))) {
      throw new MiddlewareError("invalid-payment-payload");
    }
    const txHash = `0x${toHex(keccak256(raw))}`;
    const response = decodeSettlementHeader(responseHeader);
    const payer = paymentPayer(payload.accepted.extra, true)!;
    if (!response.success || response.network !== payload.accepted.network || response.amount !== payload.accepted.amount
      || response.payer !== payer || response.transaction !== txHash || expectedTransactionHash !== txHash) {
      throw new MiddlewareError("verification-failure");
    }
    const evidence = parseReceiptEvidence(response.extensions?.["layerx"]);
    try {
      const signedReceipt = grantTransactionBytes(evidence.signedReceipt);
      const keys = this.#receiptKeys.filter((key) => `0x${toHex(key.keyId)}` === evidence.receiptKey.keyId
        && `0x${toHex(key.publicKey)}` === evidence.receiptKey.publicKey
        && key.first_height === BigInt(evidence.receiptKey.firstHeight)
        && key.last_height === BigInt(evidence.receiptKey.lastHeight));
      if (keys.length === 0 || evidence.rawTransaction !== rawTransaction
        || (paymentCommitment(payload.accepted.extra) === "committed" && evidence.verificationLevel !== "committed")) {
        throw new MiddlewareError("verification-failure");
      }
      const verified = await verifySequencerReceipt(signedReceipt, raw, keys);
      if (!constantTimeEqualText(evidence.receiptDigest, `0x${toHex(verified.receiptDigest)}`)) {
        throw new MiddlewareError("verification-failure");
      }
      const { receipt } = verified;
      const transaction = decodeRawEip1559(raw);
      const sender = recoverEip1559Sender(raw).slice(2).toLowerCase().padStart(64, "0");
      if (toHex(receipt.from) !== sender
        || !equalBytes(receipt.to, concatenate(new Uint8Array(12), transaction.to))) {
        throw new MiddlewareError("verification-failure");
      }
      const summary = receipt.effects[0];
      if (receipt.resultCode !== 0 || receipt.moduleId <= 11 || summary === undefined
        || summary.moduleId !== receipt.moduleId || summary.ordinal !== 0 || summary.kind !== 1
        || summary.monetary || summary.eventType !== 0 || summary.body.length < 110
        || summary.body[108] === 0 || summary.body[108]! > 64 || summary.body.length !== 109 + summary.body[108]!
        || new DataView(arrayBuffer(summary.body)).getUint32(104, false) !== receipt.effects.length - 1) {
        throw new MiddlewareError("verification-failure");
      }
      const eventData = concatenate(grantTransactionBytes(asset), data.subarray(68, 132), data.subarray(196, 228));
      const digest = await sha256(eventData);
      const topic = keccak256(new TextEncoder().encode("PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)"));
      const payments: PaymentDrawnEvent[] = [];
      for (const [index, effect] of receipt.effects.slice(1).entries()) {
        const body = effect.body;
        const count = body[20];
        if (effect.moduleId !== receipt.moduleId || effect.ordinal !== index + 1 || effect.kind !== 3
          || effect.monetary || effect.eventType !== 0 || count === undefined || count > 4
          || body.length !== 53 + count * 32) throw new MiddlewareError("verification-failure");
        const address = `0x${toHex(body.subarray(0, 20))}`;
        if (address !== LAYERX_LEDGER_ADDRESS || !equalBytes(body.subarray(21, 53), topic)) continue;
        if (!equalBytes(body.subarray(body.length - 32), digest)) throw new MiddlewareError("verification-failure");
        const payment = decodePaymentDrawnLog({ address, data: eventData,
          topics: Array.from({ length: count }, (_, i) => body.slice(21 + i * 32, 53 + i * 32)) });
        if (!equalBytes(payment.grantId, data.subarray(4, 36))
          || toHex(payment.from) !== payer
          || !equalBytes(payment.to, data.subarray(36, 68))
          || `0x${toHex(payment.assetId)}` !== payload.accepted.asset
          || payment.amount !== BigInt(payload.accepted.amount)
          || toHex(payment.idempotencyKey) !== payload.payload["idempotencyKey"]
          || !equalBytes(payment.contextHash, data.subarray(196, 228))) {
          throw new MiddlewareError("verification-failure");
        }
        payments.push(payment);
      }
      const payment = payments[0];
      if (payments.length !== 1 || payment === undefined) throw new MiddlewareError("verification-failure");
      let commitmentEvidence: PaymentCommitmentEvidence | undefined;
      await verifyPaymentCommitment(verified, payload.accepted.network, evidence.verificationLevel,
        this.#commitments === undefined ? undefined : {
          resolveValidators: (network, height) => this.#commitments!.resolveValidators(network, height),
          resolve: async (transactionHash, network) => {
            commitmentEvidence = structuredClone(await this.#commitments!.resolve(transactionHash, network));
            return commitmentEvidence;
          },
        });
      const verification = Object.freeze({ ...verified, rung: evidence.verificationLevel, txHash, payment,
        ...(commitmentEvidence === undefined ? {} : { commitmentEvidence }),
      });
      return { response, verification, ...evidence, txHash };
    } catch {
      throw new MiddlewareError("verification-failure");
    }
  }

  public async captureSettlement(
    paymentResponseHeader: string,
    payment: PreparedPayment,
  ): Promise<CapturedSettlement> {
    const response = decodeSettlementHeader(paymentResponseHeader);
    if (!response.success) {
      throw new MiddlewareError(response.errorReason === "settlement_pending" ? "payment-pending" : "payment-refused");
    }
    if (
      response.network !== payment.offer.accepted.network
      || response.amount !== payment.offer.accepted.amount
      || response.transaction !== payment.txHash
    ) {
      throw new MiddlewareError("verification-failure");
    }
    const evidence = parseReceiptEvidence(response.extensions?.["layerx"]);
    if (!constantTimeEqualText(evidence.receiptDigest, payment.receiptDigest)
      || !constantTimeEqualText(evidence.rawTransaction, payment.rawTransaction)
      || !constantTimeEqualText(evidence.signedReceipt, payment.signedReceipt)) {
      throw new MiddlewareError("verification-failure");
    }
    const verification = await verifyPaymentReceipt(
      evidence, payment.offer.accepted, this.#receiptKeys, this.#commitments,
    );
    if (response.transaction !== verification.txHash
      || response.payer !== toHex(verification.receipt.from)) throw new MiddlewareError("verification-failure");
    return { response, verification, ...evidence, txHash: verification.txHash };
  }

  public async fetch(
    input: RequestInfo | URL,
    init: RequestInit,
    callerIdempotencyKey: string,
  ): Promise<PaidFetchResult> {
    const initial = await this.#fetch(input, init);
    if (initial.status !== 402) {
      return { kind: "not-payment-required", response: initial };
    }
    const required = initial.headers.get(PAYMENT_REQUIRED_HEADER);
    if (required === null) {
      throw new MiddlewareError("invalid-payment-required");
    }
    await initial.body?.cancel();
    const preparation = await this.prepare(required, callerIdempotencyKey);
    if (preparation.kind !== "ready") {
      return preparation;
    }
    const headers = new Headers(init.headers);
    headers.set(PAYMENT_SIGNATURE_HEADER, preparation.payment.paymentHeader);
    const paid = await this.#fetch(input, { ...init, headers });
    const responseHeader = paid.headers.get(PAYMENT_RESPONSE_HEADER);
    if (responseHeader === null) {
      await paid.body?.cancel();
      throw new MiddlewareError("verification-failure");
    }
    try {
      const settlement = await this.captureSettlement(responseHeader, preparation.payment);
      return { kind: "paid", response: paid, payment: preparation.payment, settlement };
    } catch (error) {
      await paid.body?.cancel();
      throw error;
    }
  }

  async #findMatchingReceipt(
    journey: Journey,
    requirements: PaymentRequirements,
  ): Promise<(PortableReceiptV2 & {
    readonly verification: PaymentReceiptVerification;
    readonly txHash: string;
  }) | undefined> {
    const references = uniqueEvidence(journey).filter((reference) =>
      reference.class === "layerx-receipt"
      && evidenceRung(reference.verification) !== undefined
    );
    let matched: (PortableReceiptV2 & {
      readonly verification: PaymentReceiptVerification;
      readonly txHash: string;
    }) | undefined;
    let hasPortableEvidence = false;
    for (const reference of references) {
      const material = parseEvidenceMaterial(await this.#retrySdk(
        () => this.#client.human("evidence.get", { evidence_id: reference.evidence_id }),
        this.#retry.maximumQuoteAttempts,
        false,
      ));
      if (material.evidence_id !== reference.evidence_id || material.class !== "layerx-receipt") {
        throw new MiddlewareError("verification-failure");
      }
      const rung = evidenceRung(material.verification);
      if (material.content_type !== "application/vnd.layerx.receipt-proof-v2+json"
        || rung === undefined) continue;
      hasPortableEvidence = true;
      let decoded: unknown;
      try {
        decoded = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(decodeBase64(material.bytes_base64)));
      } catch {
        throw new MiddlewareError("verification-failure");
      }
      const evidence = parseReceiptEvidence(decoded);
      if (evidence.verificationLevel !== rung) throw new MiddlewareError("verification-failure");
      let verification: PaymentReceiptVerification;
      try {
        verification = await verifyPaymentReceipt(evidence, requirements, this.#receiptKeys, this.#commitments);
      } catch (error) {
        if (!(error instanceof MiddlewareError) || error.code !== "verification-failure") throw error;
        continue;
      }
      if (matched !== undefined) throw new MiddlewareError("verification-failure");
      matched = { ...evidence, receiptDigest: `0x${toHex(verification.receiptDigest)}`, verification, txHash: verification.txHash };
    }
    if (hasPortableEvidence && matched === undefined) throw new MiddlewareError("verification-failure");
    return matched;
  }

  async #retrySdk<T>(operation: () => Promise<T>, maximumAttempts: number, mutation: boolean): Promise<T> {
    for (let attempt = 1; ; attempt += 1) {
      try {
        return await operation();
      } catch (error) {
        if (!(error instanceof PlatformSdkError) || attempt >= maximumAttempts) {
          throw error;
        }
        if (error.retry === "never" || error.retry === "unknown-outcome") {
          throw error;
        }
        if (mutation && error.retry !== "safe" && error.retry !== "after") {
          throw error;
        }
        const delay = error.retryAfterMs ?? this.#retry.baseDelayMs * attempt;
        await this.#retry.wait(delay);
      }
    }
  }
}

export interface LayerXHttpTransportConfig {
  readonly baseUrl: string;
  readonly bearerToken: SecretBytes;
  readonly fetch?: typeof globalThis.fetch;
}

export class LayerXPaymentHttpTransport implements ProductionTransport {
  readonly #baseUrl: URL;
  readonly #token: SecretBytes;
  readonly #fetch: typeof globalThis.fetch;

  public constructor(config: LayerXHttpTransportConfig) {
    this.#baseUrl = new URL(config.baseUrl.endsWith("/") ? config.baseUrl : `${config.baseUrl}/`);
    const loopback = this.#baseUrl.hostname === "127.0.0.1"
      || this.#baseUrl.hostname === "localhost"
      || this.#baseUrl.hostname === "[::1]";
    if (this.#baseUrl.username.length > 0 || this.#baseUrl.password.length > 0
      || this.#baseUrl.hash.length > 0
      || (this.#baseUrl.protocol !== "https:" && !(this.#baseUrl.protocol === "http:" && loopback))) {
      throw new MiddlewareError("unsupported-payment");
    }
    this.#token = config.bearerToken;
    this.#fetch = config.fetch ?? globalThis.fetch;
  }

  public async call<TRequest, TResponse>(call: TransportCall<TRequest>): Promise<TResponse> {
    if (call.plane !== "human") {
      throw new PlatformSdkError({ code: "unavailable-capability", retry: "never" });
    }
    const route = paymentRoute(call.operation, call.request);
    const headers = new Headers({ accept: "application/json" });
    this.#token.withBytes((bytes) => {
      headers.set("authorization", `Bearer ${new TextDecoder("utf-8", { fatal: true }).decode(bytes)}`);
    });
    if (call.idempotencyKey !== undefined) {
      headers.set("idempotency-key", call.idempotencyKey);
    }
    if (route.body !== undefined) {
      headers.set("content-type", "application/json");
    }
    let response: Response;
    try {
      response = await this.#fetch(new URL(route.path, this.#baseUrl), {
        method: route.method,
        headers,
        ...(route.body === undefined ? {} : { body: JSON.stringify(route.body) }),
        signal: AbortSignal.timeout(HTTP_REQUEST_TIMEOUT_MS),
      });
    } catch {
      throw new PlatformSdkError({ code: "transport-failure", retry: "safe" });
    }
    const body = await readBoundedJson(response);
    if (!response.ok) {
      throw mapHttpError(response);
    }
    const envelope = asObject(body);
    if (envelope["ok"] !== true || envelope["result"] === undefined) {
      throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
    }
    return envelope["result"] as TResponse;
  }
}

async function readBoundedJson(response: Response): Promise<unknown> {
  if (response.body === null) return undefined;
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (let step = await reader.read(); step.done !== true; step = await reader.read()) {
    total += step.value.length;
    if (total > MAXIMUM_HTTP_RESPONSE_BYTES) {
      await reader.cancel();
      throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
    }
    chunks.push(step.value);
  }
  const encoded = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    encoded.set(chunk, offset);
    offset += chunk.length;
  }
  try {
    return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(encoded));
  } catch {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
}

export function platform_mw_buyer(): "sdk-quoted-receipt-verified-x402-buyer" {
  return "sdk-quoted-receipt-verified-x402-buyer";
}

function retryPolicy(value: Partial<BuyerRetryPolicy> | undefined): BuyerRetryPolicy {
  const policy: BuyerRetryPolicy = {
    maximumQuoteAttempts: value?.maximumQuoteAttempts ?? 3,
    maximumCommitAttempts: value?.maximumCommitAttempts ?? 3,
    maximumJourneyPolls: value?.maximumJourneyPolls ?? 20,
    baseDelayMs: value?.baseDelayMs ?? 250,
    wait: value?.wait ?? ((delayMs) => new Promise((resolve) => setTimeout(resolve, delayMs))),
  };
  if (
    !Number.isSafeInteger(policy.maximumQuoteAttempts) || policy.maximumQuoteAttempts <= 0
    || !Number.isSafeInteger(policy.maximumCommitAttempts) || policy.maximumCommitAttempts <= 0
    || !Number.isSafeInteger(policy.maximumJourneyPolls) || policy.maximumJourneyPolls < 0
    || !Number.isSafeInteger(policy.baseDelayMs) || policy.baseDelayMs < 0
  ) {
    throw new MiddlewareError("unsupported-payment");
  }
  return policy;
}

function parseMoveQuote(value: unknown): MoveQuote {
  const object = asObject(value);
  const mechanism = object["mechanism"];
  if (mechanism !== "fund" && mechanism !== "allocate" && mechanism !== "return" && mechanism !== "transfer") {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return {
    quote_id: requiredString(object["quote_id"]),
    description_copy_key: requiredString(object["description_copy_key"]),
    mechanism,
    money: parseMoney(object["money"]),
    fee_estimate: parseMoney(object["fee_estimate"]),
    fee_ceiling: parseMoney(object["fee_ceiling"]),
    arrival_estimate: requiredTimestamp(object["arrival_estimate"]),
    expires_at: requiredTimestamp(object["expires_at"]),
    ...(object["irreversibility_copy_key"] === undefined
      ? {}
      : { irreversibility_copy_key: requiredString(object["irreversibility_copy_key"]) }),
  };
}

function parseJourney(value: unknown): Journey {
  const object = asObject(value);
  const state = parseJourneyState(object["state"]);
  const evidence = parseEvidenceReferences(object["evidence"]);
  if (!Array.isArray(object["stages"])) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  const stages = object["stages"].map((value) => {
    const stage = asObject(value);
    return {
      stage_id: requiredString(stage["stage_id"]),
      copy_key: requiredString(stage["copy_key"]),
      state: parseJourneyState(stage["state"]),
      evidence: parseEvidenceReferences(stage["evidence"]),
    };
  });
  const refusalValue = object["refusal"];
  const refusal = refusalValue === undefined ? undefined : asObject(refusalValue);
  if (state === "refused" && refusal === undefined) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return {
    journey_id: requiredString(object["journey_id"]),
    kind: requiredString(object["kind"]),
    state,
    state_copy_key: requiredString(object["state_copy_key"]),
    evidence,
    stages,
    started_at: requiredTimestamp(object["started_at"]),
    updated_at: requiredTimestamp(object["updated_at"]),
    ...(refusal === undefined ? {} : {
      refusal: {
        refused_by: requiredString(refusal["refused_by"]),
        copy_key: requiredString(refusal["copy_key"]),
        money_left: requiredBoolean(refusal["money_left"]),
      },
    }),
  };
}

function parseEvidenceMaterial(value: unknown): EvidenceMaterial {
  const object = asObject(value);
  return {
    evidence_id: requiredString(object["evidence_id"]),
    class: requiredString(object["class"]),
    verification: requiredString(object["verification"]),
    content_type: requiredString(object["content_type"]),
    bytes_base64: requiredString(object["bytes_base64"]),
  };
}

function parseEvidenceReferences(value: unknown): readonly EvidenceReference[] {
  if (!Array.isArray(value)) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return value.map((item) => {
    const object = asObject(item);
    return {
      evidence_id: requiredString(object["evidence_id"]),
      class: requiredString(object["class"]),
      verification: requiredString(object["verification"]),
    };
  });
}

export function evidenceRung(label: string): "receipted" | "committed" | undefined {
  if (label === "receipted" || label === "receipt-verified") return "receipted";
  if (label === "committed" || label === "checkpoint-finalised") return "committed";
  return undefined;
}

function uniqueEvidence(journey: Journey): readonly EvidenceReference[] {
  const unique = new Map<string, EvidenceReference>();
  for (const reference of [...journey.evidence, ...journey.stages.flatMap((stage) => stage.evidence)]) {
    const previous = unique.get(reference.evidence_id);
    if (previous !== undefined && (
      previous.class !== reference.class || previous.verification !== reference.verification
    )) {
      throw new MiddlewareError("verification-failure");
    }
    unique.set(reference.evidence_id, reference);
  }
  return [...unique.values()];
}

function parseMoney(value: unknown): { readonly amount: string; readonly currency: string } {
  const object = asObject(value);
  return { amount: requiredAmount(object["amount"]), currency: requiredString(object["currency"]) };
}

function parseJourneyState(value: unknown): JourneyState {
  if (
    value !== "getting-ready" && value !== "sending" && value !== "processing"
    && value !== "done" && value !== "done-finalised" && value !== "still-checking"
    && value !== "refused" && value !== "waiting-for-you"
  ) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return value;
}

function isProgress(state: JourneyState): boolean {
  return state === "getting-ready" || state === "sending" || state === "processing";
}

function parseReceiptEvidence(value: unknown): PortableReceiptV2 {
  const failure = (): never => { throw new MiddlewareError("verification-failure"); };
  const object = (input: unknown): Readonly<Record<string, unknown>> => {
    if (input === null || typeof input !== "object" || Array.isArray(input)) return failure();
    return input as Readonly<Record<string, unknown>>;
  };
  const hex = (input: unknown, size?: number): string => {
    if (typeof input !== "string" || input.length > 128 * 1024
      || !/^0x(?:[0-9a-f]{2})+$/u.test(input)
      || (size !== undefined && input.length !== 2 + size * 2)) return failure();
    return input;
  };
  const height = (input: unknown): string => {
    if (typeof input !== "string" || !/^0x(?:0|[1-9a-f][0-9a-f]{0,15})$/u.test(input)) return failure();
    return input;
  };
  const proof = object(value);
  if (proof["format"] !== "layerx-receipt-proof-v2"
    || (proof["verificationLevel"] !== "receipted" && proof["verificationLevel"] !== "committed")) return failure();
  const key = object(proof["receiptKey"]);
  if (Object.keys(key).length !== 4) return failure();
  const firstHeight = height(key["firstHeight"]);
  const lastHeight = height(key["lastHeight"]);
  if (BigInt(firstHeight) > BigInt(lastHeight)) return failure();
  return {
    format: "layerx-receipt-proof-v2",
    verificationLevel: proof["verificationLevel"],
    rawTransaction: hex(proof["rawTransaction"]),
    signedReceipt: hex(proof["signedReceipt"]),
    receiptDigest: hex(proof["receiptDigest"], 32),
    receiptKey: {
      keyId: hex(key["keyId"], 32), publicKey: hex(key["publicKey"], 32), firstHeight, lastHeight,
    },
  };
}

function paymentRoute(operation: string, request: unknown): { readonly method: string; readonly path: string; readonly body?: unknown } {
  switch (operation) {
    case "move.quote":
      return { method: "POST", path: "v1/moves/quote", body: request };
    case "move.commit":
      return { method: "POST", path: "v1/moves", body: request };
    case "journey.get":
      return { method: "GET", path: `v1/journeys/${encodeURIComponent(requiredString(asObject(request)["journey_id"]))}` };
    case "evidence.get":
      return { method: "GET", path: `v1/evidence/${encodeURIComponent(requiredString(asObject(request)["evidence_id"]))}` };
    default:
      throw new PlatformSdkError({ code: "unavailable-capability", retry: "never" });
  }
}

function mapHttpError(response: Response): PlatformSdkError {
  if (response.status === 429) {
    const seconds = response.headers.get("retry-after");
    const retryAfterMs = seconds !== null && /^[0-9]+$/u.test(seconds) ? Number(seconds) * 1000 : undefined;
    return new PlatformSdkError({
      code: "rate-limit",
      retry: "after",
      ...(retryAfterMs === undefined || !Number.isSafeInteger(retryAfterMs) ? {} : { retryAfterMs }),
    });
  }
  if (response.status === 409) {
    return new PlatformSdkError({ code: "idempotency-conflict", retry: "never" });
  }
  if (response.status === 400 || response.status === 422) {
    return new PlatformSdkError({ code: "invalid-argument", retry: "never" });
  }
  if (response.status === 401 || response.status === 403) {
    return new PlatformSdkError({ code: "capability-refusal", retry: "never" });
  }
  if (response.status >= 500) {
    return new PlatformSdkError({ code: "transport-failure", retry: "safe" });
  }
  return new PlatformSdkError({ code: "internal-fault", retry: "never" });
}

function asObject(value: unknown): Readonly<Record<string, JsonValue>> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return value as Readonly<Record<string, JsonValue>>;
}

function requiredString(value: unknown): string {
  if (typeof value !== "string" || value.length === 0 || value.includes("\0")) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return value;
}

function requiredBoolean(value: unknown): boolean {
  if (typeof value !== "boolean") {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return value;
}

function requiredAmount(value: unknown): string {
  const text = requiredString(value);
  if (!/^(0|[1-9][0-9]*)$/u.test(text) || BigInt(text) > MAX_U128) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return text;
}

function requiredTimestamp(value: unknown): string {
  const text = requiredString(value);
  if (!Number.isFinite(Date.parse(text))) {
    throw new PlatformSdkError({ code: "decode-failure", retry: "never" });
  }
  return text;
}

function decodeBase64(value: string): Uint8Array {
  if (value.length === 0 || !/^[A-Za-z0-9+/]*={0,2}$/u.test(value)) {
    throw new MiddlewareError("verification-failure");
  }
  try {
    return Uint8Array.from(atob(value), (character) => character.charCodeAt(0));
  } catch {
    throw new MiddlewareError("verification-failure");
  }
}

export function paymentQuoteTerms(requirements: PaymentRequirements): PaymentQuoteTerms {
  try {
    return {
      account: paymentAccount(requirements.extra, true)!,
      currency: paymentCurrency(requirements.extra, true)!,
    };
  } catch (error) {
    if (error instanceof PlatformSdkError) throw new MiddlewareError("invalid-payment-required");
    throw error;
  }
}

export async function accountIdentifiers(account: string): Promise<readonly Uint8Array[]> {
  const name = new TextEncoder().encode(account);
  const length = new Uint8Array(4);
  new DataView(length.buffer).setUint32(0, name.length);
  return Promise.all([
    sha256(concatenate(LEGACY_ACCOUNT_ID_DOMAIN, name)),
    sha256(concatenate(NATIVE_ACCOUNT_ID_DOMAIN, length, name)),
  ]);
}

async function accountNamesPayTo(account: string, payTo: string, asset: string): Promise<boolean> {
  const digits = payTo.startsWith("0x") ? payTo.slice(2) : payTo;
  if (!/^[0-9a-fA-F]{64}$/u.test(digits)) return false;
  const expected = Uint8Array.from({ length: 32 }, (_, index) => Number.parseInt(digits.slice(index * 2, index * 2 + 2), 16));
  let matched = false;
  for (const candidate of await accountIdentifiers(account)) {
    matched = equalBytes(candidate, expected) || matched;
  }
  const did = /^agent:([a-z0-9._:-]+):main$/u.exec(account)?.[1];
  const assetHex = asset.replace(/^0x/u, "").toLowerCase();
  if (did !== undefined && /^[0-9a-f]{64}$/u.test(assetHex)) {
    const [, nativeAssetAccount] = await accountIdentifiers(`agent:${did}:asset:${assetHex}`);
    matched = equalBytes(nativeAssetAccount!, expected) || matched;
  }
  return matched;
}

async function sha256(value: Uint8Array): Promise<Uint8Array> {
  return new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", arrayBuffer(value)));
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

function toHex(value: Uint8Array): string {
  return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

export function grantPaymentHeader(required: PaymentRequired, accepted: PaymentRequirements, rawTransaction: string): string {
  const { data } = grantDrawTransaction(accepted, rawTransaction);
  if (!required.accepts.some(value => JSON.stringify(value) === JSON.stringify(accepted))) {
    throw new MiddlewareError("requirements-mismatch");
  }
  return encodePaymentPayloadHeader({ x402Version: X402_VERSION, resource: required.resource, accepted,
    extensions: required.extensions ?? {}, payload: { rawTransaction, idempotencyKey: toHex(data.subarray(100, 132)) } });
}

function grantTransactionBytes(value: string): Uint8Array {
  if (typeof value !== "string" || value.length > 128 * 1024 || !/^0x(?:[0-9a-f]{2})+$/u.test(value)) {
    throw new MiddlewareError("invalid-payment-payload");
  }
  return Uint8Array.from(value.slice(2).match(/../gu)!, (pair) => Number.parseInt(pair, 16));
}

function grantDrawTransaction(accepted: PaymentRequirements, rawTransaction: string): { readonly raw: Uint8Array; readonly data: Uint8Array; readonly asset: string } {
  try {
    const raw = grantTransactionBytes(rawTransaction);
    const transaction = decodeRawEip1559(raw);
    const data = transaction.data;
    const layerx = asObject(asObject(accepted.extra)["layerx"]);
    const grant = asObject(layerx["grant"]);
    const grantId = requiredString(grant["grantId"]);
    const asset = requiredString(grant["asset"]);
    const contextHash = requiredString(layerx["contextHash"]);
    paymentPayer(accepted.extra, true);
    if ((accepted.scheme !== "metered" && accepted.scheme !== "subscription")
      || `0x${toHex(transaction.to)}` !== LAYERX_LEDGER_ADDRESS || transaction.value !== 0n
      || accepted.network !== `eip155:${transaction.chainId}`
      || data.length !== 4 + 7 * 32 || !equalBytes(data.subarray(0, 4), LEDGER_DRAW_SELECTOR)
      || !/^[0-9a-f]{64}$/u.test(grantId) || /^0+$/u.test(grantId)
      || toHex(data.subarray(4, 36)) !== grantId
      || `0x${toHex(data.subarray(36, 68))}` !== accepted.payTo
      || !/^0x[0-9a-f]{64}$/u.test(asset) || asset !== accepted.asset
      || !/^(0|[1-9][0-9]{0,38})$/u.test(accepted.amount)
      || BigInt(`0x${toHex(data.subarray(68, 100))}`) > MAX_U128
      || BigInt(`0x${toHex(data.subarray(68, 100))}`) !== BigInt(accepted.amount)
      || BigInt(`0x${toHex(data.subarray(132, 164))}`) >= 1n << 64n
      || toHex(data.subarray(164, 196)) !== paymentPurpose(accepted.extra, true)
      || !/^[0-9a-f]{64}$/u.test(contextHash)
      || toHex(data.subarray(196, 228)) !== contextHash) {
      throw new MiddlewareError("requirements-mismatch");
    }
    return { raw, data, asset };
  } catch {
    throw new MiddlewareError("requirements-mismatch");
  }
}
