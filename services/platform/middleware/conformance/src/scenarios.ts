import { ProductionClient, SecretBytes, keccak256 } from "@sidiora/layerx-sdk";
import {
  MiddlewareError,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  VerifiedWebhookConsumer,
  encodePaymentPayloadHeader,
  encodePaymentRequiredHeader,
  encodeSettlementHeader,
  verifyPaymentReceipt,
  type FulfillmentRepository,
  type PaymentCommitmentResolver,
  type PaymentPayload,
  type PaymentReceiptVerification,
  type PaymentRequirements,
  type SellerDecision,
} from "@sidiora/layerx-seller-middleware";
import type {
  BuyerMiddleware,
  CapturedSettlement,
  Journey,
  MoveQuote,
  ParsedOffer,
  PreparedPayment,
} from "@sidiora/layerx-buyer-middleware";
import {
  MerchantError,
  MerchantMiddleware,
  MerchantSettlementWebhooks,
  type CheckoutOpenRequest,
  type MerchantOrder,
  type MerchantOrderStore,
  type MerchantReceiptEvidence,
  type MerchantReceiptResolver,
  type MerchantSettlementWebhooksConfig,
  type SettlementWebhookEvent,
} from "@sidiora/layerx-merchant-middleware";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  buyerPayload,
  encodeBase64,
  fixedBytes,
  offerFixture,
  toHex,
  type OfferFixture,
  type ReceiptFacts,
  type SignedReceipt,
} from "./receipts.js";
import {
  FixedReceiptKeys,
  InMemoryDeliveryStore,
  InMemoryFulfillmentRepository,
  Suite,
  assert,
  expectThrows,
} from "./support.js";

const isMiddlewareError = (code: string) => (error: unknown): boolean =>
  error instanceof MiddlewareError && error.code === code;

export class CommittedRungRequiresExactPaymentError extends Error {
  public constructor() {
    super("the committed rung requires an exact payment");
    this.name = "CommittedRungRequiresExactPaymentError";
  }
}

export function committedRungOffer(
  offer: OfferFixture,
  payment: PaymentReceiptVerification["payment"],
): OfferFixture {
  if (!("paymentReference" in payment)) {
    throw new CommittedRungRequiresExactPaymentError();
  }
  const requirements: PaymentRequirements = {
    ...offer.requirements,
    extra: {
      layerx: {
        commitment: "committed",
        purposeHash: toHex(payment.paymentReference),
      },
    },
  };
  return {
    requirements,
    paymentRequired: { ...offer.paymentRequired, accepts: [requirements] },
  };
}

/** A session issuing and retaining signed payments from the conformance producer. */
export class ConformancePaymentRpcSession {
  readonly #receipts = new Map<string, Readonly<Record<string, unknown>>>();
  private readonly sequencer: ConformanceSequencer;
  private readonly facts: ReceiptFacts;
  private readonly requirements: PaymentRequirements;

  public constructor(
    sequencer: ConformanceSequencer,
    facts: ReceiptFacts,
    requirements: PaymentRequirements,
  ) {
    this.sequencer = sequencer;
    this.facts = facts;
    this.requirements = requirements;
  }

  public get issuedCount(): number {
    return this.#receipts.size;
  }

  public async call(method: string, params: readonly unknown[]): Promise<unknown> {
    if (params.length !== 1 || typeof params[0] !== "string") {
      throw new Error("invalid-conformance-rpc-params");
    }
    if (method === "pax_sequencerReceipt") {
      const receipt = this.#receipts.get(params[0]);
      return receipt === undefined ? null : structuredClone(receipt);
    }
    if (method !== "eth_sendRawTransaction") throw new Error("unknown-conformance-rpc-method");
    const raw = params[0];
    if (!/^0x(?:[0-9a-f]{2})+$/u.test(raw)) throw new Error("invalid-conformance-rpc-transaction");
    const bytes = Uint8Array.from(raw.slice(2).match(/../gu)!, (pair) => Number.parseInt(pair, 16));
    const txHash = `0x${toHex(keccak256(bytes))}`;
    if (this.#receipts.has(txHash)) return txHash;
    const receipt = await buildSignedReceipt(this.sequencer, this.facts);
    if (receipt.rawTransaction !== raw) throw new Error("conformance-payment-mismatch");
    const keys = await new FixedReceiptKeys([this.sequencer.receiptKey()]).resolve();
    const verified = await verifyPaymentReceipt(receipt, this.requirements, keys);
    this.#receipts.set(verified.txHash, structuredClone(receipt.receiptJson));
    return txHash;
  }
}

async function buildBuyer(offer: OfferFixture, resolver: FixedReceiptKeys): Promise<BuyerMiddleware> {
  const { BuyerMiddleware, LayerXPaymentHttpTransport } = await import("@sidiora/layerx-buyer-middleware");
  const receiptKeys = await resolver.resolve();
  const transport = new LayerXPaymentHttpTransport({
    baseUrl: "http://127.0.0.1:1/",
    bearerToken: new SecretBytes(new TextEncoder().encode("unused-conformance-token")),
  });
  return new BuyerMiddleware({
    client: new ProductionClient(transport),
    source: "acct:conformance-buyer",
    supported: [{ scheme: offer.requirements.scheme, network: offer.requirements.network }],
    receiptKeys,
  });
}

function preparedPayment(
  offer: ParsedOffer,
  receipt: SignedReceipt,
  verification: PaymentReceiptVerification,
): PreparedPayment {
  const quote: MoveQuote = {
    quote_id: "quote-conformance",
    description_copy_key: "move.description",
    mechanism: "transfer",
    money: { amount: offer.accepted.amount, currency: offer.accepted.asset },
    fee_estimate: { amount: "0", currency: offer.accepted.asset },
    fee_ceiling: { amount: "0", currency: offer.accepted.asset },
    arrival_estimate: "2026-01-01T00:00:00.000Z",
    expires_at: "2999-01-01T00:00:00.000Z",
  };
  const journey: Journey = {
    journey_id: "journey-conformance",
    kind: "move",
    state: "done",
    state_copy_key: "journey.done",
    evidence: [],
    stages: [],
    started_at: "2026-01-01T00:00:00.000Z",
    updated_at: "2026-01-01T00:00:00.000Z",
  };
  const payload = buyerPayload({ requirements: offer.accepted, paymentRequired: offer.required }, receipt, "buyer-key");
  return {
    offer,
    quote,
    journey,
    payload,
    paymentHeader: encodePaymentPayloadHeader(payload),
    idempotencyKey: "buyer-key",
    ...receipt,
    verification,
    receiptDigest: receipt.receiptDigest,
  };
}

class ConformanceMerchantOrders implements MerchantOrderStore {
  readonly #orders = new Map<string, MerchantOrder>();

  public async open(request: CheckoutOpenRequest): Promise<MerchantOrder> {
    const existing = this.#orders.get(request.checkoutKey);
    if (existing !== undefined) {
      if (existing.requestDigest !== request.requestDigest) throw new Error("order-conflict");
      return existing;
    }
    const order: MerchantOrder = {
      orderId: request.checkoutKey,
      checkoutKey: request.checkoutKey,
      requestDigest: request.requestDigest,
      state: "awaiting-payment",
      quote: request.quote,
    };
    this.#orders.set(order.orderId, order);
    return order;
  }

  public async releaseResource(orderId: string): Promise<MerchantOrder> {
    return this.required(orderId);
  }

  public async markPaid(
    orderId: string,
    requestDigest: string,
    receiptDigest: string,
    transaction: string,
  ): Promise<MerchantOrder> {
    const current = this.required(orderId);
    if (current.requestDigest !== requestDigest) throw new Error("order-conflict");
    if (current.state === "paid-verified") {
      if (current.receiptDigest !== receiptDigest || current.transaction !== transaction) {
        throw new Error("order-conflict");
      }
      return current;
    }
    const paid: MerchantOrder = { ...current, state: "paid-verified", receiptDigest, transaction };
    this.#orders.set(orderId, paid);
    return paid;
  }

  public async markRefused(orderId: string, requestDigest: string): Promise<MerchantOrder> {
    const current = this.required(orderId);
    if (current.requestDigest !== requestDigest || current.state === "paid-verified") throw new Error("order-conflict");
    const refused: MerchantOrder = { ...current, state: "refused" };
    this.#orders.set(orderId, refused);
    return refused;
  }

  public async get(orderId: string): Promise<MerchantOrder | undefined> {
    return this.#orders.get(orderId);
  }

  private required(orderId: string): MerchantOrder {
    const order = this.#orders.get(orderId);
    if (order === undefined) throw new Error("order-missing");
    return order;
  }
}

export async function releasedDecision(
  offer: OfferFixture,
  payload: PaymentPayload,
  resolver: FixedReceiptKeys,
  repository: FulfillmentRepository<string>,
  commitments?: PaymentCommitmentResolver,
): Promise<SellerDecision<string>> {
  const receiptKeys = await resolver.resolve();
  const seller = new SellerMiddleware<string>({
    paymentRequired: offer.paymentRequired,
    receiptKeys,
    authority: new ReceiptPayloadAuthority(receiptKeys),
    fulfillments: repository,
    ...(commitments === undefined ? {} : { commitments }),
  });
  return seller.handle("acct:conformance-seller", encodePaymentPayloadHeader(payload), async () => "the-paid-resource");
}

export async function assertConcurrentReceiptFulfillment(
  offer: OfferFixture,
  receipt: SignedReceipt,
  resolver: FixedReceiptKeys,
): Promise<void> {
  const repository = new InMemoryFulfillmentRepository<string>();
  const receiptKeys = await resolver.resolve();
  const seller = new SellerMiddleware<string>({
    paymentRequired: offer.paymentRequired,
    receiptKeys,
    authority: new ReceiptPayloadAuthority(receiptKeys),
    fulfillments: repository,
  });
  const header = encodePaymentPayloadHeader(buyerPayload(offer, receipt, "seller-concurrent"));
  let entered!: () => void;
  const started = new Promise<void>((resolve) => { entered = resolve; });
  let finish!: () => void;
  const barrier = new Promise<void>((resolve) => { finish = resolve; });
  let releases = 0;
  const first = seller.handle("acct:conformance-seller", header, async () => {
    releases += 1;
    entered();
    await barrier;
    return "the-paid-resource";
  });
  await started;
  const replay = seller.handle("acct:conformance-seller", header, async () => {
    releases += 1;
    return "duplicate-resource";
  });
  try {
    for (const [principal, competingHeader] of [
      ["acct:other-seller", header],
      ["acct:conformance-seller", encodePaymentPayloadHeader(buyerPayload(offer, receipt, "competing-request"))],
    ] as const) {
      const conflict = await seller.handle(principal, competingHeader, async () => {
        releases += 1;
        return "conflicting-resource";
      });
      assert(conflict.kind === "refused" && conflict.status === 409, "an in-flight receipt reuse must be refused");
      assert(conflict.kind === "refused" && !conflict.settlement.success && conflict.settlement.errorReason === "receipt_reuse", "the refusal must declare receipt reuse");
    }
  } finally {
    finish();
    await Promise.all([first, replay]);
  }
  const [original, repeated] = await Promise.all([first, replay]);
  assert(original.kind === "released" && repeated.kind === "released", "matching concurrent requests must release");
  if (original.kind === "released" && repeated.kind === "released") {
    assert(original.resource === "the-paid-resource" && repeated.resource === original.resource, "replays must return the claimed resource");
    assert(original.settlement.transaction === receipt.txHash && repeated.settlement.transaction === receipt.txHash, "both responses must bind the producer transaction");
  }
  assert(releases === 1 && repository.releaseCount === 1, "concurrent receipt claims must invoke release once");
}

export async function runScenarios(): Promise<Suite> {
  const suite = new Suite();

  const sequencer = await ConformanceSequencer.generate();
  const foreignSequencer = await ConformanceSequencer.generate();
  const payTo = fixedBytes(0xaa);
  const asset = fixedBytes(0xbb);
  const amount = 250_000n;
  const offer = offerFixture(payTo, asset, amount);
  const facts: ReceiptFacts = {
    asset,
    payTo,
    amount,
    from: fixedBytes(0xcc),
    batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2),
    resultingStateRoot: fixedBytes(0xd3),
  };
  const receipt = await buildSignedReceipt(sequencer, facts);
  const resolver = new FixedReceiptKeys([sequencer.receiptKey()]);
  const receiptKeys = await resolver.resolve();

  await suite.check("sdk: in-flight receipt verification snapshots evidence and authority", async () => {
    const mutableReceipt = { ...receipt, receiptKey: { ...receipt.receiptKey } };
    const mutableAuthority = await resolver.resolve();
    const verification = verifyPaymentReceipt(mutableReceipt, offer.requirements, mutableAuthority);
    mutableReceipt.signedReceipt = "0x00";
    mutableReceipt.rawTransaction = "0x00";
    mutableReceipt.receiptKey.keyId = "0x" + "00".repeat(32);
    mutableReceipt.receiptKey.publicKey = "0x" + "00".repeat(32);
    mutableReceipt.receiptKey.firstHeight = "0x0";
    mutableReceipt.receiptKey.lastHeight = "0x0";
    for (const key of mutableAuthority) {
      key.keyId.fill(0);
      key.publicKey.fill(0);
    }
    const result = await verification;
    assert(result.rung === "receipted", "caller mutation changed in-flight verification");
    assert(result.txHash === receipt.txHash, "caller mutation changed the verified transaction hash");
    assert(result.canonicalBytes.some((byte: number) => byte !== 0), "verified evidence aliased caller bytes");
  });

  await suite.check("seller: no payment header yields a 402 offer and never releases", async () => {
    const repository = new InMemoryFulfillmentRepository<string>();
    const seller = new SellerMiddleware<string>({
      paymentRequired: offer.paymentRequired,
      receiptKeys,
      authority: new ReceiptPayloadAuthority(receiptKeys),
      fulfillments: repository,
    });
    const decision = await seller.handle("acct:conformance-seller", undefined, async () => "the-paid-resource");
    assert(decision.kind === "payment-required" && decision.status === 402, "expected a 402 payment-required decision");
    assert(repository.releaseCount === 0, "the resource must not be released without payment");
  });

  await suite.check("seller: a valid receipt releases the resource under receipted evidence", async () => {
    const repository = new InMemoryFulfillmentRepository<string>();
    const payload = buyerPayload(offer, receipt, "seller-happy");
    const decision = await releasedDecision(offer, payload, resolver, repository);
    assert(decision.kind === "released" && decision.status === 200, "expected a released decision");
    if (decision.kind !== "released") {
      return;
    }
    assert(repository.releaseCount === 1, "the resource must be released exactly once");
    assert(decision.resource === "the-paid-resource", "the released resource must be returned");
    assert(decision.settlement.success, "the settlement response must report success");
    assert(decision.settlement.payer === toHex(decision.verification.receipt.from), "the settlement must bind the verified receipt payer");
    assert(/^0x[0-9a-f]{64}$/u.test(decision.settlement.transaction), "the settlement must carry a 0x transaction hash");
    assert(decision.settlement.transaction === receipt.txHash, "the settlement must reference the verified transaction hash");
    assert(decision.verification.rung === "receipted", "the verification rung must be recorded");
  });

  await suite.check("seller: a tampered receipt cannot be presented as success", async () => {
    const repository = new InMemoryFulfillmentRepository<string>();
    const tampered = Uint8Array.from(receipt.signedReceipt.slice(2).match(/../gu)!, (byte: string) => Number.parseInt(byte, 16));
    const last = tampered.length - 1;
    tampered[last] = (tampered[last] ?? 0) ^ 0x01;
    const payload = buyerPayload(offer, { ...receipt, signedReceipt: `0x${toHex(tampered)}` }, "seller-tamper");
    await expectThrows(
      () => releasedDecision(offer, payload, resolver, repository),
      isMiddlewareError("verification-failure"),
      "tampered receipt",
    );
    assert(repository.releaseCount === 0, "a tampered receipt must never release the resource");
  });

  await suite.check("seller: a receipt signed by an unauthorised sequencer is rejected", async () => {
    const repository = new InMemoryFulfillmentRepository<string>();
    const forged = await buildSignedReceipt(foreignSequencer, facts);
    const forgedKeys = new FixedReceiptKeys([sequencer.receiptKey()]);
    const payload = buyerPayload(offer, forged, "seller-forged");
    await expectThrows(
      () => releasedDecision(offer, payload, forgedKeys, repository),
      isMiddlewareError("verification-failure"),
      "forged sequencer signature",
    );
    assert(repository.releaseCount === 0, "a forged signature must never release the resource");
  });

  await suite.check("seller: a receipt whose amount disagrees with the offer is rejected", async () => {
    const repository = new InMemoryFulfillmentRepository<string>();
    const cheapReceipt = await buildSignedReceipt(sequencer, { ...facts, amount: 1n });
    const cheapResolver = new FixedReceiptKeys([sequencer.receiptKey()]);
    const payload = buyerPayload(offer, cheapReceipt, "seller-underpaid");
    await expectThrows(
      () => releasedDecision(offer, payload, cheapResolver, repository),
      isMiddlewareError("verification-failure"),
      "underpaid receipt",
    );
    assert(repository.releaseCount === 0, "an underpaid receipt must never release the resource");
  });

  await suite.check("seller: fulfilment is idempotent under a replayed payment", async () => {
    const repository = new InMemoryFulfillmentRepository<string>();
    const payload = buyerPayload(offer, receipt, "seller-idempotent");
    const first = await releasedDecision(offer, payload, resolver, repository);
    const second = await releasedDecision(offer, payload, resolver, repository);
    assert(first.kind === "released" && second.kind === "released", "both replays must release");
    assert(repository.releaseCount === 1, "the resource must be released only once across replays");
  });

  await suite.check("seller: concurrent receipt claims release once and refuse competing requests", async () => {
    await assertConcurrentReceiptFulfillment(offer, receipt, resolver);
  });

  await suite.check("buyer: parseOffer selects only a supported requirement", async () => {
    const buyer = await buildBuyer(offer, resolver);
    const parsed = buyer.parseOffer(encodePaymentRequiredHeader(offer.paymentRequired));
    assert(parsed.accepted.scheme === offer.requirements.scheme, "the supported scheme must be selected");
    const unsupported = offerFixture(payTo, asset, amount);
    const other = {
      ...unsupported.paymentRequired,
      accepts: [{ ...unsupported.requirements, network: "eip155:1" }],
    };
    expectThrowsSync(
      () => buyer.parseOffer(encodePaymentRequiredHeader(other)),
      isMiddlewareError("unsupported-payment"),
      "unsupported network",
    );
  });

  await suite.check("buyer: captureSettlement verifies the seller's settlement evidence", async () => {
    const repository = new InMemoryFulfillmentRepository<string>();
    const payload = buyerPayload(offer, receipt, "buyer-capture");
    const decision = await releasedDecision(offer, payload, resolver, repository);
    assert(decision.kind === "released", "seller must release for the capture scenario");
    if (decision.kind !== "released") {
      return;
    }
    const { BuyerMiddleware, LayerXPaymentHttpTransport } = await import("@sidiora/layerx-buyer-middleware");
    const buyer = new BuyerMiddleware({
      client: new ProductionClient(new LayerXPaymentHttpTransport({
        baseUrl: "http://127.0.0.1:1/",
        bearerToken: new SecretBytes(new TextEncoder().encode("unused-conformance-token")),
      })),
      source: "acct:conformance-buyer",
      supported: [{ scheme: offer.requirements.scheme, network: offer.requirements.network }],
      receiptKeys,
    });
    const parsed = buyer.parseOffer(encodePaymentRequiredHeader(offer.paymentRequired));
    const prepared = preparedPayment(parsed, receipt, decision.verification);
    const captured = await buyer.captureSettlement(
      decision.headers["PAYMENT-RESPONSE"],
      prepared,
    );
    assert(captured.verification.rung === "receipted", "the buyer must verify the captured receipt rung");
    assert(captured.response.transaction === receipt.txHash, "the buyer must capture the verified transaction hash");
    await expectThrows(
      () => buyer.captureSettlement(encodeSettlementHeader({ ...decision.settlement, payer: "01".repeat(32) }), prepared),
      isMiddlewareError("verification-failure"),
      "settlement payer mismatch",
    );
  });

  await suite.check("x402: verificationLevel equals the rung of the evidence", async () => {
    const verification = await verifyPaymentReceipt(receipt, offer.requirements, receiptKeys);
    assert(receipt.verificationLevel === verification.rung, "verificationLevel must equal the verified evidence rung");
    assert(verification.rung === "receipted", "a sequencer receipt must carry the receipted rung");
    await expectThrows(
      () => verifyPaymentReceipt({ ...receipt, verificationLevel: "committed" }, offer.requirements, receiptKeys),
      isMiddlewareError("verification-failure"),
      "a committed claim without commitment evidence",
    );
    for (const level of ["executed", "batched", "finalised", "finalized"]) {
      const payload = buyerPayload(offer, receipt, "verification-level");
      const invalid: PaymentPayload = {
        ...payload,
        payload: { ...payload.payload, verificationLevel: level },
      };
      await expectThrows(
        () => releasedDecision(offer, invalid, resolver, new InMemoryFulfillmentRepository<string>()),
        isMiddlewareError("invalid-payment-payload"),
        `verificationLevel ${level}`,
      );
    }
  });

  await suite.check("x402: a committed requirement refuses receipted-only evidence", async () => {
    const verification = await verifyPaymentReceipt(receipt, offer.requirements, receiptKeys);
    assert(verification.rung === "receipted", "the evidence must first verify at the receipted rung");
    const committedOffer = committedRungOffer(offer, verification.payment);
    await expectThrows(
      () => verifyPaymentReceipt(receipt, committedOffer.requirements, receiptKeys),
      isMiddlewareError("verification-failure"),
      "committed requirement with receipted-only evidence",
    );
    const repository = new InMemoryFulfillmentRepository<string>();
    await expectThrows(
      () => releasedDecision(
        committedOffer,
        buyerPayload(committedOffer, receipt, "committed-no-downgrade"),
        resolver,
        repository,
      ),
      isMiddlewareError("verification-failure"),
      "seller must not downgrade a committed requirement",
    );
    assert(repository.releaseCount === 0, "receipted-only evidence must not release a committed resource");
  });

  await suite.check("buyer: a failed settlement is never reported as paid", async () => {
    const buyer = await buildBuyer(offer, resolver);
    const parsed = buyer.parseOffer(encodePaymentRequiredHeader(offer.paymentRequired));
    const prepared = preparedPayment(parsed, receipt, await sequencerVerification(receipt, resolver));
    const failure = encodeSettlementHeader({
      success: false,
      errorReason: "insufficient_funds",
      transaction: "",
      network: offer.requirements.network,
    });
    await expectThrows(
      () => captureConformanceSettlement(buyer, failure, prepared),
      isMiddlewareError("payment-refused"),
      "failed settlement",
    );
  });

  await suite.check("webhook: a valid signature is processed exactly once and replays are duplicates", async () => {
    const deliveries = new InMemoryDeliveryStore();
    const consumer = new VerifiedWebhookConsumer({
      publicKeys: { "seq-1": sequencer.publicKey },
      deliveries,
    });
    const body = new TextEncoder().encode(JSON.stringify({ event: "settlement", ok: true }));
    const id = "delivery-1";
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = await signWebhook(sequencer, id, timestamp, body);
    let handled = 0;
    const first = await consumer.consume(body, { id, timestamp, keyId: "seq-1", signature }, async () => {
      handled += 1;
    });
    assert(first === "processed", "a valid webhook must be processed");
    assert(handled === 1, "the handler must run exactly once");
    const second = await consumer.consume(body, { id, timestamp, keyId: "seq-1", signature }, async () => {
      handled += 1;
    });
    assert(second === "duplicate", "a replay must be reported as a duplicate");
    assert(handled === 1, "a replay must not re-run the handler");
  });

  await suite.check("webhook: caller mutation cannot replace configured verification authority", async () => {
    const deliveries = new InMemoryDeliveryStore();
    const configuredKey = sequencer.publicKey.slice();
    const consumer = new VerifiedWebhookConsumer({
      publicKeys: { "seq-stable": configuredKey },
      deliveries,
    });
    configuredKey.fill(0);
    const body = new TextEncoder().encode(JSON.stringify({ event: "settlement", authority: "stable" }));
    const id = "delivery-stable-authority";
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = await signWebhook(sequencer, id, timestamp, body);
    const result = await consumer.consume(
      body,
      { id, timestamp, keyId: "seq-stable", signature },
      async () => undefined,
    );
    assert(result === "processed", "configured webhook authority changed through caller alias");
  });

  await suite.check("webhook: a tampered body cannot be presented as verified", async () => {
    const deliveries = new InMemoryDeliveryStore();
    const consumer = new VerifiedWebhookConsumer({
      publicKeys: { "seq-1": sequencer.publicKey },
      deliveries,
    });
    const body = new TextEncoder().encode(JSON.stringify({ event: "settlement", ok: true }));
    const id = "delivery-2";
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = await signWebhook(sequencer, id, timestamp, body);
    const tampered = new TextEncoder().encode(JSON.stringify({ event: "settlement", ok: false }));
    let handled = 0;
    await expectThrows(
      () => consumer.consume(tampered, { id, timestamp, keyId: "seq-1", signature }, async () => {
        handled += 1;
      }),
      isMiddlewareError("invalid-webhook"),
      "tampered webhook body",
    );
    assert(handled === 0, "a tampered webhook must never run the handler");
  });

  await suite.check("webhook: an unknown key id is rejected", async () => {
    const deliveries = new InMemoryDeliveryStore();
    const consumer = new VerifiedWebhookConsumer({
      publicKeys: { "seq-1": sequencer.publicKey },
      deliveries,
    });
    const body = new TextEncoder().encode(JSON.stringify({ event: "settlement" }));
    const id = "delivery-3";
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = await signWebhook(sequencer, id, timestamp, body);
    await expectThrows(
      () => consumer.consume(body, { id, timestamp, keyId: "unknown", signature }, async () => undefined),
      isMiddlewareError("invalid-webhook"),
      "unknown webhook key",
    );
  });

  await runMerchantScenarios(suite);

  return suite;
}

async function sequencerVerification(receipt: SignedReceipt, resolver: FixedReceiptKeys): Promise<PaymentReceiptVerification> {
  const receiptKeys = await resolver.resolve();
  return verifyPaymentReceipt(
    receipt,
    offerFixture(fixedBytes(0xaa), fixedBytes(0xbb), 250_000n).requirements,
    receiptKeys,
  );
}

async function signWebhook(
  sequencer: ConformanceSequencer,
  id: string,
  timestamp: string,
  body: Uint8Array,
): Promise<string> {
  const prefix = new TextEncoder().encode(`${id}.${timestamp}.`);
  const message = new Uint8Array(prefix.length + body.length);
  message.set(prefix);
  message.set(body, prefix.length);
  const signature = await sequencer.sign(message);
  return `v1=${encodeBase64(signature)}`;
}

function expectThrowsSync(body: () => unknown, predicate: (error: unknown) => boolean, description: string): void {
  try {
    body();
  } catch (error) {
    if (!predicate(error)) {
      throw new Error(`${description}: threw the wrong error`);
    }
    return;
  }
  throw new Error(`${description}: expected a throw but the call returned`);
}

export async function runMerchantScenarios(suite = new Suite()): Promise<Suite> {
  const sequencer = await ConformanceSequencer.generate();
  const payTo = fixedBytes(0xaa);
  const asset = fixedBytes(0xbb);
  const amount = 250_000n;
  const offer = offerFixture(payTo, asset, amount);
  const receipt = await buildSignedReceipt(sequencer, {
    asset, payTo, amount, from: fixedBytes(0xcc), batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2), resultingStateRoot: fixedBytes(0xd3),
  });
  const receiptKeys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  await suite.check("merchant: a signed settlement webhook verifies the receipt and pays exactly once", async () => {
    const orders = new ConformanceMerchantOrders();
    const merchant = new MerchantMiddleware({
      catalog: { get: async () => ({
        sku: "merchant-item",
        title: "Merchant conformance item",
        unitAmount: amount.toString(),
        asset: toHex(asset),
        payTo: toHex(payTo),
        scheme: "exact",
        network: "eip155:125",
        maxTimeoutSeconds: 120,
        extra: offer.requirements.extra!,
      }) },
      orders,
      sellers: {
        create: (paymentRequired) => new SellerMiddleware<MerchantOrder>({
          paymentRequired,
          receiptKeys,
          authority: new ReceiptPayloadAuthority(receiptKeys),
          fulfillments: new InMemoryFulfillmentRepository<MerchantOrder>(),
        }),
      },
      resourceUrl: (checkoutKey) => `https://merchant.example/checkout/${checkoutKey}`,
    });
    const checkout = await merchant.checkout(
      "acct:merchant-conformance",
      "merchant-webhook-order",
      [{ sku: "merchant-item", quantity: 1 }],
    );
    assert(checkout.kind === "payment-required", "checkout must start payment-required");
    const deliveries = new InMemoryDeliveryStore();
    const consumer = new VerifiedWebhookConsumer({
      publicKeys: { "seq-merchant": sequencer.publicKey },
      deliveries,
    });
    const receipts = new ConformanceMerchantReceipts();
    receipts.add(receipt.txHash, receipt);
    const config: MerchantSettlementWebhooksConfig = {
      verifier: consumer,
      orders,
      receipts,
      receiptKeys,
    };
    const webhooks = new MerchantSettlementWebhooks(config);
    const { event, body, headers } = await buildMerchantSettlementDelivery(
      sequencer, checkout.order, receipt, "merchant-delivery",
    );
    const { timestamp, signature } = headers;
    const first = await webhooks.consume(body, {
      id: "merchant-delivery",
      timestamp,
      keyId: "seq-merchant",
      signature,
    });
    const paid = await orders.get(checkout.order.orderId);
    assert(first === "processed" && paid?.state === "paid-verified", "verified webhook must pay the order");
    assert(paid?.receiptDigest === receipt.receiptDigest.slice(2), "paid order must bind the verified receipt digest");
    assert(paid?.transaction === receipt.txHash, "paid order must bind the verified transaction hash");
    const second = await webhooks.consume(body, {
      id: "merchant-delivery",
      timestamp,
      keyId: "seq-merchant",
      signature,
    });
    assert(second === "duplicate", "settlement redelivery must be idempotent");
    assert(receipts.resolutions === 1, "only identical delivery IDs may skip receipt resolution");
    const nextId = "merchant-delivery-fresh";
    const nextSignature = await signWebhook(sequencer, nextId, timestamp, body);
    assert(await webhooks.consume(body, {
      id: nextId, timestamp, keyId: "seq-merchant", signature: nextSignature,
    }) === "processed", "a new delivery must verify the receipt again");
    assert(Number(receipts.resolutions) === 2, "a fresh delivery must not use cached receipt evidence");
    assert((await orders.get(checkout.order.orderId))?.transaction === receipt.txHash, "repeat verification must preserve payment");
    const malformed = new TextEncoder().encode(JSON.stringify({ ...event, receipt_ref: "" }));
    const malformedId = "merchant-malformed";
    const malformedSignature = await signWebhook(sequencer, malformedId, timestamp, malformed);
    await expectThrows(() => webhooks.consume(malformed, {
      id: malformedId, timestamp, keyId: "seq-merchant", signature: malformedSignature,
    }), (error) => error instanceof MerchantError && error.code === "invalid-webhook", "malformed receipt reference");
    assert(Number(receipts.resolutions) === 2, "malformed events must fail before resolving evidence");
  });

  await suite.check("merchant: a signed claim with mismatched receipt evidence cannot pay an order", async () => {
    const orders = new ConformanceMerchantOrders();
    const merchant = new MerchantMiddleware({
      catalog: { get: async () => ({
        sku: "merchant-item",
        title: "Merchant conformance item",
        unitAmount: amount.toString(),
        asset: toHex(asset),
        payTo: toHex(payTo),
        scheme: "exact",
        network: "eip155:125",
        maxTimeoutSeconds: 120,
        extra: offer.requirements.extra!,
      }) },
      orders,
      sellers: {
        create: (paymentRequired) => new SellerMiddleware<MerchantOrder>({
          paymentRequired,
          receiptKeys,
          authority: new ReceiptPayloadAuthority(receiptKeys),
          fulfillments: new InMemoryFulfillmentRepository<MerchantOrder>(),
        }),
      },
      resourceUrl: (checkoutKey) => `https://merchant.example/checkout/${checkoutKey}`,
    });
    const checkout = await merchant.checkout(
      "acct:merchant-conformance",
      "merchant-mismatch",
      [{ sku: "merchant-item", quantity: 1 }],
    );
    assert(checkout.kind === "payment-required", "mismatch checkout must start payment-required");
    const order = checkout.order;
    const receipts = new ConformanceMerchantReceipts();
    receipts.add("receipt:merchant-conformance", receipt);
    const webhooks = new MerchantSettlementWebhooks({
      verifier: new VerifiedWebhookConsumer({
        publicKeys: { "seq-merchant": sequencer.publicKey },
        deliveries: new InMemoryDeliveryStore(),
      }),
      orders,
      receipts,
      receiptKeys,
    });
    const body = new TextEncoder().encode(JSON.stringify({
      order_id: order.orderId,
      request_digest: order.requestDigest,
      receipt_digest: toHex(fixedBytes(0xff)),
      receipt_ref: "receipt:merchant-conformance",
      transaction: receipt.txHash,
      verification: "receipted",
    }));
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = await signWebhook(sequencer, "merchant-mismatch-delivery", timestamp, body);
    await expectThrows(
      () => webhooks.consume(body, {
        id: "merchant-mismatch-delivery",
        timestamp,
        keyId: "seq-merchant",
        signature,
      }),
      (error) => error instanceof MerchantError && error.code === "invalid-webhook",
      "mismatched merchant receipt digest",
    );
    assert((await orders.get(order.orderId))?.state === "awaiting-payment", "mismatched evidence must not alter order state");
  });

  return suite;
}

class ConformanceMerchantReceipts implements MerchantReceiptResolver {
  readonly #receipts = new Map<string, MerchantReceiptEvidence>();
  public resolutions = 0;

  public add(reference: string, receipt: MerchantReceiptEvidence): void {
    this.#receipts.set(reference, structuredClone(receipt));
  }

  public async resolve(reference: string): Promise<MerchantReceiptEvidence> {
    this.resolutions += 1;
    const receipt = this.#receipts.get(reference);
    if (receipt === undefined) throw new MerchantError("invalid-webhook");
    return structuredClone(receipt);
  }
}

export async function buildMerchantSettlementDelivery(
  sequencer: ConformanceSequencer,
  order: MerchantOrder,
  receipt: SignedReceipt,
  id: string,
): Promise<{
  event: SettlementWebhookEvent;
  body: Uint8Array;
  headers: { id: string; timestamp: string; keyId: string; signature: string };
}> {
  const event: SettlementWebhookEvent = {
    order_id: order.orderId,
    request_digest: order.requestDigest,
    receipt_digest: receipt.receiptDigest.slice(2),
    receipt_ref: receipt.txHash,
    transaction: receipt.txHash,
    verification: "receipted",
  };
  const body = new TextEncoder().encode(JSON.stringify(event));
  const timestamp = Math.floor(Date.now() / 1000).toString();
  return {
    event,
    body,
    headers: {
      id, timestamp, keyId: "seq-merchant",
      signature: await signWebhook(sequencer, id, timestamp, body),
    },
  };
}

export async function captureConformanceSettlement(
  buyer: BuyerMiddleware,
  header: string,
  payment: PreparedPayment,
): Promise<CapturedSettlement> {
  const captured = await buyer.captureSettlement(header, payment);
  assert(captured.receiptDigest === payment.receiptDigest, "settlement must preserve the prepared receipt digest");
  return captured;
}
