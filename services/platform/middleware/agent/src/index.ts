import { validateDaemonSpend, decodeDaemonPrepared, decodeDaemonSubmission, daemonApprovalHold, signedActivityId, type DaemonPreparation } from "./daemon.js";
export { decodeDaemonSubmission, decodeDaemonPrepared, signedActivityId } from "./daemon.js";
export type { DaemonPreparation, DaemonPrepareRequest } from "./daemon.js";
import { paymentCommitment, verifyPaymentReceipt, ReceiptPayloadAuthority, LAYERX_LEDGER_ADDRESS, type PaymentReceiptVerification, type GrantDrawExecution, type SellerSettlementOutcome } from "@sidiora/layerx-seller-middleware";
import { verifyPaymentCommitment, type PaymentCommitment, type PaymentCommitmentResolver } from "@sidiora/layerx-seller-middleware";
import {
  PlatformSdkError,
  ProductionClient,
  SDK_ERROR_CODES,
  idempotencyKey,
  protocolAmount,
  decodeRawEip1559,
  keccak256,
  verifySequencerReceipt,
  type SequencerReceiptKey,
  type ReceiptVerification,
  type RetryClass,
  type SdkErrorCode,
} from "@sidiora/layerx-sdk";

const POST_SUBMIT_UNCERTAIN_CODES: ReadonlySet<SdkErrorCode> = new Set([
  "transport-failure",
  "deadline",
  "decode-failure",
  "internal-fault",
]);

export interface AgentSpendRequest {
  readonly commitment?: { readonly network: string; readonly level: PaymentCommitment };
  readonly tenant: string;
  readonly preparation: DaemonPreparation;
  readonly networkId: string;
  readonly authorityHex: string;
  readonly signerPublicKey: string;
  readonly submitIdempotencyKey: string;
  readonly approvalCurrentSequence: string;
  readonly approvalReleaseRef?: string;
  readonly walletApprovalId?: string;
  readonly asset: string;
  readonly amount: string;
  readonly recipient: string;
}

interface BudgetReservationBase {
  readonly reservationId: string;
  readonly requestDigest: string;
  readonly amount: string;
  readonly asset: string;
}

export type ReservedBudgetReservation = BudgetReservationBase & {
  readonly state: "reserved";
};

export type HeldBudgetReservation = BudgetReservationBase & {
  readonly state: "held";
  readonly approvalId: string;
  readonly canonicalBytesDigest: string;
};

export type CommittedBudgetReservation = BudgetReservationBase & {
  readonly state: "committed";
  readonly receiptDigest: string;
};

export type ReleasedBudgetReservation = BudgetReservationBase & {
  readonly state: "released";
  readonly refusal: AgentRefusal;
};

export type BudgetReservation =
  | ReservedBudgetReservation
  | HeldBudgetReservation
  | CommittedBudgetReservation
  | ReleasedBudgetReservation;

export type BudgetReserveResult =
  | { readonly kind: "reserved"; readonly reservation: BudgetReservation }
  | { readonly kind: "exhausted"; readonly available: string }
  | { readonly kind: "conflict" };

export interface BudgetTransition {
  readonly reservationId: string;
  readonly requestDigest: string;
  readonly amount: string;
  readonly asset: string;
}

export interface BudgetHoldTransition extends BudgetTransition {
  readonly approvalId: string;
  readonly canonicalBytesDigest: string;
}

export interface BudgetCommitTransition extends BudgetTransition {
  readonly receiptDigest: string;
}

export interface BudgetReleaseTransition extends BudgetTransition {
  readonly refusal: AgentRefusal;
}

export interface AgentBudgetLedger {
  reserve(request: {
    readonly tenant: string;
    readonly idempotencyKey: string;
    readonly requestDigest: string;
    readonly amount: string;
    readonly asset: string;
  }): Promise<BudgetReserveResult>;
  hold(transition: BudgetHoldTransition): Promise<HeldBudgetReservation>;
  commit(transition: BudgetCommitTransition): Promise<CommittedBudgetReservation>;
  release(transition: BudgetReleaseTransition): Promise<ReleasedBudgetReservation>;
}

export interface PreparedActivity {
  readonly preparation_ref: string;
  readonly chain_id: string;
  readonly nonce: string;
  readonly max_priority_fee_per_gas: string;
  readonly max_fee_per_gas: string;
  readonly gas_limit: string;
  readonly to: string;
  readonly data: string;
  readonly signing_hash: string;
  readonly sender: string;
  readonly disclosure: Readonly<Record<string, unknown>>;
  readonly expiry: string;
  readonly approval?: ApprovalHold;
  readonly wallet_approval_id?: string;
  readonly signer_public_key?: string;
}

export interface AgentSigner {
  readonly walletApprovalRequired?: true;
  verifyWalletApproval?(prepared: PreparedActivity): Promise<void>;
  sign(prepared: PreparedActivity): Promise<string>;
}

export interface Submission {
  readonly submission_ref: string;
  readonly state: string | Readonly<Record<string, unknown>>;
  readonly evidence?: readonly unknown[];
  readonly verification_level?: "Submitted" | "Receipted" | "Committed";
  readonly tx_hash: string;
  readonly receipt_ref?: string;
  readonly receiptEvidence?: AgentReceiptEvidence;
  readonly portableReceipt?: PortableReceiptV2;
  readonly transitions?: readonly unknown[];
}

export interface PortableReceiptV2 {
  readonly format: "layerx-receipt-proof-v2";
  readonly verificationLevel: "receipted" | "committed";
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

export interface AgentReceiptEvidence extends PortableReceiptV2 {}

export interface AgentReceiptContext {
  readonly idempotencyKey: string;
  readonly activityId: string;
}

export interface AgentReceiptRecording extends AgentReceiptContext {
  readonly receiptDigest: string;
  readonly receiptRef?: string;
}

export interface AgentReceiptResolver {
  resolve(receiptRef: string): Promise<AgentReceiptEvidence>;
  resolveFor?(receiptRef: string, context: AgentReceiptContext): Promise<AgentReceiptEvidence>;
  retainVerified?(record: AgentReceiptRecording): Promise<void>;
}

export interface ApprovalHold {
  readonly approvalId: string;
  readonly state: "Held";
  readonly canonicalBytesDigest: string;
  readonly enforcement: "daemon_enforced";
}

export type AgentRefusalRetry = Exclude<RetryClass, "unknown-outcome">;

export interface AgentRefusal {
  readonly code: SdkErrorCode;
  readonly retry: AgentRefusalRetry;
  readonly retryAfterMs?: number;
  readonly protocolResultCode?: number;
  readonly submissionState?: "Failed" | "Expired";
}

export interface OwnerBudgetSpendResult {
  readonly kind: "owner-budget";
  readonly preparationId: string;
  readonly admissionObserved: boolean;
  readonly prepared?: PreparedActivity;
  readonly state: "admission-unknown" | "wallet-consent" | "approval" | "pending" | "unknown" | "owner-rejected" | "owner-expired" | "settled";
  readonly approval?: { readonly approvalId: string; readonly heldDigest: string; readonly state: string };
  readonly submission?: Submission;
  readonly verification?: ReceiptVerification;
}

export interface AgentPreparationBudget {
  spendPrepared(request: AgentSpendRequest, requestDigest: string, services: {
    readonly signer: AgentSigner;
    readonly receipts: AgentReceiptResolver;
    readonly receiptKeys: readonly SequencerReceiptKey[];
    readonly commitments?: PaymentCommitmentResolver;
  }): Promise<OwnerBudgetSpendResult>;
}

export interface AgentMiddlewareConfig {
  readonly commitments?: PaymentCommitmentResolver;
  readonly client: ProductionClient;
  readonly receiptKeys: readonly SequencerReceiptKey[];
  readonly budgets?: AgentBudgetLedger;
  readonly preparationBudgets?: AgentPreparationBudget;
  readonly signer: AgentSigner;
  readonly receipts: AgentReceiptResolver;
  readonly maximumTrackPolls?: number;
  readonly wait?: (milliseconds: number) => Promise<void>;
}

export type AgentSpendResult =
  | OwnerBudgetSpendResult
  | {
    readonly kind: "verified";
    readonly submission?: Submission;
    readonly verification: ReceiptVerification;
    readonly reservation: CommittedBudgetReservation;
  }
  | { readonly kind: "approval-hold"; readonly approval: ApprovalHold; readonly reservation: HeldBudgetReservation }
  | { readonly kind: "pending"; readonly submission: Submission; readonly reservation: BudgetReservation }
  | { readonly kind: "unknown"; readonly reservation: BudgetReservation; readonly submission?: Submission }
  | ({ readonly kind: "refused"; readonly reservation: BudgetReservation } & AgentRefusal)
  | { readonly kind: "budget-refused"; readonly code: "budget-refusal"; readonly retry: "never"; readonly available: string };

export class AgentMiddleware {
  readonly #commitments: PaymentCommitmentResolver | undefined;
  readonly #client: ProductionClient;
  readonly #receiptKeys: readonly SequencerReceiptKey[];
  readonly #budgets: AgentBudgetLedger | undefined;
  readonly #preparationBudgets: AgentPreparationBudget | undefined;
  readonly #signer: AgentSigner;
  readonly #receipts: AgentReceiptResolver;
  readonly #maximumTrackPolls: number;
  readonly #wait: (milliseconds: number) => Promise<void>;

  public constructor(config: AgentMiddlewareConfig) {
    this.#commitments = config.commitments;
    this.#client = config.client;
    this.#receiptKeys = copyReceiptKeys(config.receiptKeys);
    this.#budgets = config.budgets;
    this.#preparationBudgets = config.preparationBudgets;
    if ((this.#budgets === undefined) === (this.#preparationBudgets === undefined)) throw new AgentMiddlewareError("invalid-request");
    this.#signer = config.signer;
    this.#receipts = config.receipts;
    this.#maximumTrackPolls = config.maximumTrackPolls ?? 20;
    this.#wait = config.wait ?? ((milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds)));
    if (!Number.isSafeInteger(this.#maximumTrackPolls) || this.#maximumTrackPolls < 0 || this.#maximumTrackPolls > 1_000) {
      throw new AgentMiddlewareError("invalid-request");
    }
  }

  public async spend(input: AgentSpendRequest): Promise<AgentSpendResult> {
    let request: AgentSpendRequest;
    try { request = freezeRequest(JSON.parse(JSON.stringify(input))) as AgentSpendRequest; }
    catch { throw new AgentMiddlewareError("invalid-request"); }
    validateSpend(request);
    const walletMode = this.#signer.walletApprovalRequired === true;
    if (request.walletApprovalId !== undefined && !walletMode
      || walletMode && request.walletApprovalId === undefined
        && !("variant" in request.preparation && this.#preparationBudgets !== undefined)) {
      throw new PlatformSdkError({ code: "policy-refusal", retry: "never" });
    }
    if (request.commitment?.level === "committed" && this.#commitments === undefined) throw new AgentMiddlewareError("invalid-request");
    const amount = protocolAmount(request.amount).toString();
    const mutationKey = idempotencyKey(request.preparation.idempotency_key);
    const requestDigest = await digestSpend(request);
    if ("variant" in request.preparation && this.#preparationBudgets !== undefined) {
      const { approvalReleaseRef: _release, walletApprovalId: _walletConsent, ...admissionRequest } = request;
      return this.#preparationBudgets.spendPrepared(request, await digestSpend(admissionRequest), {
        signer: this.#signer, receipts: this.#receipts, receiptKeys: this.#receiptKeys,
        ...(this.#commitments === undefined ? {} : { commitments: this.#commitments }),
      });
    }
    if (this.#budgets === undefined) throw new PlatformSdkError({ code: "unavailable-capability", retry: "never" });
    const reserved = await this.#budgets.reserve({
      tenant: request.tenant,
      idempotencyKey: request.preparation.idempotency_key,
      requestDigest,
      amount,
      asset: request.asset,
    });
    if (reserved.kind === "exhausted") {
      protocolAmount(reserved.available);
      return { kind: "budget-refused", code: "budget-refusal", retry: "never", available: reserved.available };
    }
    if (reserved.kind === "conflict") {
      throw new AgentMiddlewareError("idempotency-conflict");
    }
    const budget = { requestDigest, amount, asset: request.asset };
    const reservation = validateBudgetReservation(reserved.reservation, budget);
    if (reservation.state === "held") {
      return {
        kind: "approval-hold",
        approval: {
          approvalId: reservation.approvalId,
          state: "Held",
          canonicalBytesDigest: reservation.canonicalBytesDigest,
          enforcement: "daemon_enforced",
        },
        reservation,
      };
    }
    if (reservation.state === "released") {
      return { kind: "refused", reservation, ...reservation.refusal };
    }
    if (reservation.state === "committed") {
      try {
        const evidence = await this.#receipts.resolve(reservation.receiptDigest);
        return await verifyCommittedAgentPayment(evidence, reservation, request, this.#receiptKeys, this.#commitments);
      } catch {
        return { kind: "unknown", reservation };
      }
    }
    let prepared: PreparedActivity;
    try {
      prepared = decodeDaemonPrepared(await this.#client.agent("prepare", request.preparation,
        { idempotencyKey: mutationKey }), request, 0);
    } catch (error) {
      return this.#sdkFailure(error, reservation, budget, false);
    }
    if (prepared.approval !== undefined) {
      return this.#hold(prepared.approval, reservation, budget);
    }
    let signature: string;
    try {
      signature = await this.#signer.sign(prepared);
    } catch (error) {
      return this.#sdkFailure(error, reservation, budget, false);
    }
    let expectedActivityId: string;
    try {
      expectedActivityId = signedActivityId(prepared, signature, request.signerPublicKey);
    } catch (error) {
      return this.#sdkFailure(error, reservation, budget, false);
    }
    let submission: Submission;
    try {
      submission = decodeDaemonSubmission(await this.#client.agent("submit", {
        preparation_ref: prepared.preparation_ref,
        signature,
        signer_public_key: request.signerPublicKey,
        approval_release_ref: request.approvalReleaseRef ?? null,
      }, { idempotencyKey: idempotencyKey(request.submitIdempotencyKey) }), expectedActivityId);
    } catch (error) {
      if (error instanceof PlatformSdkError && (error.code === "policy-refusal" || error.code === "budget-refusal")) {
        let approval: ApprovalHold | undefined;
        try {
          approval = await daemonApprovalHold(this.#client, request, prepared);
        } catch {
          return { kind: "unknown", reservation };
        }
        if (approval !== undefined) {
          return this.#hold(approval, reservation, budget);
        }
      }
      return this.#sdkFailure(error, reservation, budget, true);
    }
    let state: ReturnType<typeof submissionState>;
    try {
      state = submissionState(submission);
    } catch {
      return { kind: "unknown", reservation, submission };
    }
    for (let poll = 0; poll < this.#maximumTrackPolls && state === "Pending"; poll += 1) {
      await this.#wait(Math.min((poll + 1) * 250, 2_500));
      try {
        submission = decodeDaemonSubmission(await this.#client.agent("track", {
          submission_ref: submission.submission_ref,
        }), expectedActivityId, submission.submission_ref);
        state = submissionState(submission);
      } catch (error) {
        if (error instanceof PlatformSdkError && error.retry === "unknown-outcome") {
          return { kind: "unknown", reservation, submission };
        }
        if (error instanceof PlatformSdkError) {
          return { kind: "pending", submission, reservation };
        }
        return { kind: "unknown", reservation, submission };
      }
    }
    if (state === "Unknown") {
      return { kind: "unknown", reservation, submission };
    }
    if (state === "Pending") {
      return { kind: "pending", submission, reservation };
    }
    if (state === "Failed" || state === "Expired") {
      const refusal: AgentRefusal = {
        code: "core-rejection",
        retry: "never",
        submissionState: state,
      };
      try {
        const released = await this.#release(reservation, budget, refusal);
        return { kind: "refused", reservation: released, ...refusal };
      } catch {
        return { kind: "unknown", reservation, submission };
      }
    }
    const receiptRef = executedReceiptRef(submission);
    if (receiptRef === undefined) {
      return { kind: "pending", submission, reservation };
    }
    let evidence: AgentReceiptEvidence;
    try {
      evidence = submission.receiptEvidence ?? (this.#receipts.resolveFor === undefined
        ? await this.#receipts.resolve(receiptRef)
        : await this.#receipts.resolveFor(receiptRef, {
          idempotencyKey: request.preparation.idempotency_key, activityId: expectedActivityId,
        }));
    } catch {
      return { kind: "pending", submission, reservation };
    }
    let verification: PaymentReceiptVerification;
    try {
      verification = await verifyAgentPayment(evidence, request, this.#receiptKeys, this.#commitments);
    } catch {
      return { kind: "unknown", reservation, submission };
    }
    if (
      !constantTimeHex(verification.receipt.activityId, expectedActivityId)
      || verification.payment.amount !== BigInt(amount)
      || !constantTimeHex(verification.payment.assetId, request.asset)
      || !constantTimeHex(verification.payment.to, request.recipient)
    ) {
      return { kind: "unknown", reservation, submission };
    }
    const receiptDigest = toHex(verification.receiptDigest);
    let committed: BudgetReservation;
    try {
      await this.#receipts.retainVerified?.({
        idempotencyKey: request.preparation.idempotency_key, activityId: expectedActivityId,
        receiptDigest, receiptRef,
      });
      committed = validateBudgetReservation(
        await this.#budgets.commit({
          reservationId: reservation.reservationId,
          requestDigest,
          amount,
          asset: request.asset,
          receiptDigest,
        }),
        budget,
        reservation.reservationId,
      );
    } catch {
      return { kind: "unknown", reservation, submission };
    }
    if (committed.state !== "committed" || committed.receiptDigest !== receiptDigest) {
      throw new AgentMiddlewareError("budget-conflict");
    }
    return { kind: "verified", submission, verification, reservation: committed };
  }

  async #sdkFailure(
    error: unknown,
    reservation: BudgetReservation,
    budget: BudgetFacts,
    mayHaveExecuted: boolean,
  ): Promise<AgentSpendResult> {
    if (!(error instanceof PlatformSdkError)) {
      return { kind: "unknown", reservation };
    }
    if (
      error.retry === "unknown-outcome"
      || error.code === "unknown-outcome"
      || (mayHaveExecuted && POST_SUBMIT_UNCERTAIN_CODES.has(error.code))
    ) {
      return { kind: "unknown", reservation };
    }
    let refusal: AgentRefusal;
    try {
      refusal = sdkRefusal(error);
    } catch {
      return { kind: "unknown", reservation };
    }
    if (refusal.retry === "safe" || refusal.retry === "after") {
      return { kind: "refused", reservation, ...refusal };
    }
    try {
      const released = await this.#release(reservation, budget, refusal);
      return { kind: "refused", reservation: released, ...refusal };
    } catch {
      return { kind: "unknown", reservation };
    }
  }

  async #release(
    reservation: BudgetReservation,
    budget: BudgetFacts,
    refusal: AgentRefusal,
  ): Promise<ReleasedBudgetReservation> {
    if (reservation.state === "released") {
      if (!sameRefusal(reservation.refusal, refusal)) {
        throw new AgentMiddlewareError("budget-conflict");
      }
      return reservation;
    }
    if (reservation.state === "committed") {
      throw new AgentMiddlewareError("budget-conflict");
    }
    if (this.#budgets === undefined) throw new AgentMiddlewareError("budget-conflict");
    const released = validateBudgetReservation(
      await this.#budgets.release({
        reservationId: reservation.reservationId,
        requestDigest: budget.requestDigest,
        amount: budget.amount,
        asset: budget.asset,
        refusal,
      }),
      budget,
      reservation.reservationId,
    );
    if (released.state !== "released" || !sameRefusal(released.refusal, refusal)) {
      throw new AgentMiddlewareError("budget-conflict");
    }
    return released;
  }

  async #hold(approval: ApprovalHold, reservation: BudgetReservation, budget: BudgetFacts): Promise<AgentSpendResult> {
    let held: BudgetReservation;
    if (this.#budgets === undefined) throw new AgentMiddlewareError("budget-conflict");
    try {
      held = validateBudgetReservation(await this.#budgets.hold({
        reservationId: reservation.reservationId, ...budget,
        approvalId: approval.approvalId, canonicalBytesDigest: approval.canonicalBytesDigest,
      }), budget, reservation.reservationId);
    } catch {
      return { kind: "unknown", reservation };
    }
    if (held.state !== "held" || held.approvalId !== approval.approvalId
      || held.canonicalBytesDigest !== approval.canonicalBytesDigest) throw new AgentMiddlewareError("budget-conflict");
    return { kind: "approval-hold", approval, reservation: held };
  }

}

export type AgentMiddlewareErrorCode =
  | "invalid-request"
  | "invalid-signature"
  | "idempotency-conflict"
  | "budget-conflict"
  | "verification-failure"
  | "decode-failure";

export class AgentMiddlewareError extends Error {
  public constructor(public readonly code: AgentMiddlewareErrorCode) {
    super(code);
    this.name = "AgentMiddlewareError";
  }
}

export function platform_mw_agent(): "budget-aware-receipt-verified-agent" {
  return "budget-aware-receipt-verified-agent";
}

interface BudgetFacts {
  readonly requestDigest: string;
  readonly amount: string;
  readonly asset: string;
}

function validateBudgetReservation(
  reservation: BudgetReservation,
  expected: BudgetFacts,
  reservationId?: string,
): BudgetReservation {
  if (
    reservation === null
    || typeof reservation !== "object"
    || Array.isArray(reservation)
    || typeof reservation.reservationId !== "string"
    || typeof reservation.requestDigest !== "string"
    || typeof reservation.amount !== "string"
    || typeof reservation.asset !== "string"
    || !(new Set(["reserved", "held", "committed", "released"])).has((reservation as { readonly state: string }).state)
  ) {
    throw new AgentMiddlewareError("budget-conflict");
  }
  if (
    reservation.reservationId.length === 0
    || reservation.reservationId.length > 512
    || reservation.reservationId.includes("\0")
    || (reservationId !== undefined && reservation.reservationId !== reservationId)
    || reservation.requestDigest !== expected.requestDigest
    || reservation.amount !== expected.amount
    || reservation.asset !== expected.asset
    || !/^[0-9a-f]{64}$/u.test(reservation.requestDigest)
    || !/^[0-9a-f]{64}$/u.test(reservation.asset)
  ) {
    throw new AgentMiddlewareError("budget-conflict");
  }
  try {
    protocolAmount(reservation.amount);
  } catch {
    throw new AgentMiddlewareError("budget-conflict");
  }
  if (reservation.state === "held") {
    if (
      typeof reservation.approvalId !== "string"
      || typeof reservation.canonicalBytesDigest !== "string"
      || reservation.approvalId.length === 0
      || reservation.approvalId.length > 512
      || reservation.approvalId.includes("\0")
      || !/^[0-9a-f]{64}$/u.test(reservation.canonicalBytesDigest)
    ) {
      throw new AgentMiddlewareError("budget-conflict");
    }
  } else if (reservation.state === "committed") {
    if (typeof reservation.receiptDigest !== "string" || !/^[0-9a-f]{64}$/u.test(reservation.receiptDigest)) {
      throw new AgentMiddlewareError("budget-conflict");
    }
  } else if (reservation.state === "released") {
    validateRefusal(reservation.refusal);
  }
  return reservation;
}

function sdkRefusal(error: PlatformSdkError): AgentRefusal {
  if (error.retry === "unknown-outcome" || error.code === "unknown-outcome") {
    throw new AgentMiddlewareError("budget-conflict");
  }
  const refusal: AgentRefusal = {
    code: error.code,
    retry: error.retry,
    ...(error.retryAfterMs === undefined ? {} : { retryAfterMs: error.retryAfterMs }),
    ...(error.protocolResultCode === undefined ? {} : { protocolResultCode: error.protocolResultCode }),
  };
  validateRefusal(refusal);
  return refusal;
}

function validateRefusal(refusal: AgentRefusal): void {
  if (
    refusal === null
    || typeof refusal !== "object"
    || Array.isArray(refusal)
    || !(SDK_ERROR_CODES as readonly string[]).includes(refusal.code)
    || refusal.code === "unknown-outcome"
    || !(new Set(["never", "safe", "after"])).has((refusal as { readonly retry: string }).retry)
  ) {
    throw new AgentMiddlewareError("budget-conflict");
  }
  if (
    (refusal.retry === "after" && refusal.retryAfterMs === undefined)
    || (refusal.retry !== "after" && refusal.retryAfterMs !== undefined)
    || (refusal.retryAfterMs !== undefined
      && (!Number.isSafeInteger(refusal.retryAfterMs) || refusal.retryAfterMs < 0))
  ) {
    throw new AgentMiddlewareError("budget-conflict");
  }
  if (
    refusal.protocolResultCode !== undefined
    && (!Number.isSafeInteger(refusal.protocolResultCode)
      || refusal.protocolResultCode < -2_147_483_648
      || refusal.protocolResultCode > 2_147_483_647)
  ) {
    throw new AgentMiddlewareError("budget-conflict");
  }
  if (
    refusal.submissionState !== undefined
    && refusal.submissionState !== "Failed"
    && refusal.submissionState !== "Expired"
  ) {
    throw new AgentMiddlewareError("budget-conflict");
  }
}

function sameRefusal(left: AgentRefusal, right: AgentRefusal): boolean {
  return left.code === right.code
    && left.retry === right.retry
    && left.retryAfterMs === right.retryAfterMs
    && left.protocolResultCode === right.protocolResultCode
    && left.submissionState === right.submissionState;
}

function validateSpend(request: AgentSpendRequest): void {
  try { validateDaemonSpend(request); } catch { throw new AgentMiddlewareError("invalid-request"); }
  if (typeof request.tenant !== "string" || request.tenant.length === 0 || request.tenant.length > 255 || request.tenant.includes("\0")) throw new AgentMiddlewareError("invalid-request");
  if (request.commitment !== undefined && (!/^eip155:[0-9]+$/u.test(request.commitment.network)
    || !["receipted", "committed"].includes(request.commitment.level))) throw new AgentMiddlewareError("invalid-request");
  protocolAmount(request.amount);
  if (!/^[0-9a-f]{64}$/u.test(request.asset) || !/^[0-9a-f]{64}$/u.test(request.recipient)) throw new AgentMiddlewareError("invalid-request");
}

function submissionState(submission: Submission): "Pending" | "Unknown" | "Executed" | "Failed" | "Expired" {
  const state = submission.state;
  if (typeof state === "string") {
    if (["Prepared", "Signed", "Queued", "Submitted", "Acknowledged"].includes(state)) return "Pending";
    if (state === "Unknown" || state === "Executed" || state === "Failed" || state === "Expired") return state;
  } else {
    for (const candidate of ["Unknown", "Executed", "Failed", "Expired", "Pending"] as const) {
      if (candidate in state) return candidate;
      if (state["kind"] === candidate) return candidate;
    }
  }
  throw new AgentMiddlewareError("decode-failure");
}

function executedReceiptRef(submission: Submission): string | undefined {
  if (submission.state === "Executed" && submission.receipt_ref !== undefined) return submission.receipt_ref;
  if (typeof submission.state === "object") {
    const executed = submission.state["Executed"];
    if (executed !== null && typeof executed === "object" && !Array.isArray(executed)) {
      const receipt = (executed as Record<string, unknown>)["receiptRef"];
      if (typeof receipt === "string") return receipt;
    }
    if (submission.state["kind"] === "Executed" && typeof submission.state["receiptRef"] === "string") {
      return submission.state["receiptRef"];
    }
  }
  for (const evidence of submission.evidence ?? []) {
    if (evidence !== null && typeof evidence === "object" && !Array.isArray(evidence)) {
      const object = evidence as Record<string, unknown>;
      if (object["class"] === "layerx-receipt" && typeof object["reference"] === "string") {
        return object["reference"];
      }
    }
  }
  return undefined;
}

function disclosureDigest(prepared: PreparedActivity): string {
  const digest = prepared.disclosure["canonical_digest"] ?? prepared.disclosure["canonicalDigest"];
  const normalized = normalizeDigest(digest);
  if (normalized === undefined) {
    throw new AgentMiddlewareError("decode-failure");
  }
  return normalized;
}

async function digestSpend(request: AgentSpendRequest): Promise<string> {
  const canonical = JSON.stringify(canonicalObject(request));
  return toHex(new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", new TextEncoder().encode(canonical))));
}
function freezeRequest(value: unknown): unknown {
  if (value !== null && typeof value === "object") { for (const child of Object.values(value)) freezeRequest(child); Object.freeze(value); }
  return value;
}
function canonicalObject(value: unknown): unknown {
  if (value === null || typeof value !== "object") return value;
  if (Array.isArray(value)) return value.map(canonicalObject);
  return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonicalObject((value as Record<string, unknown>)[key])]));
}

function constantTimeHex(actual: Uint8Array, expected: string): boolean {
  if (!/^[0-9a-f]{64}$/u.test(expected) || actual.length !== 32) return false;
  let difference = 0;
  for (let index = 0; index < actual.length; index += 1) {
    const expectedByte = Number.parseInt(expected.slice(index * 2, index * 2 + 2), 16);
    difference |= (actual[index] ?? 0) ^ expectedByte;
  }
  return difference === 0;
}

function record(value: unknown): Readonly<Record<string, unknown>> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new AgentMiddlewareError("decode-failure");
  }
  return value as Readonly<Record<string, unknown>>;
}

function normalizeDigest(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const digest = value.startsWith("sha256:") ? value.slice(7) : value;
  return /^[0-9a-f]{64}$/u.test(digest) ? digest : undefined;
}

function isCanonicalBase64(value: string): boolean {
  if (value.length % 4 !== 0 || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/u.test(value)) {
    return false;
  }
  return true;
}

function text(value: unknown, maximum: number): string {
  if (typeof value !== "string" || value.length === 0 || value.length > maximum || value.includes("\0")) {
    throw new AgentMiddlewareError("decode-failure");
  }
  return value;
}

function toHex(bytes: Uint8Array): string {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

function copyReceiptKeys(keys: readonly SequencerReceiptKey[]): readonly SequencerReceiptKey[] {
  new ReceiptPayloadAuthority(keys);
  return Object.freeze(keys.map((key) => Object.freeze({
    ...key, keyId: key.keyId.slice(), publicKey: key.publicKey.slice(),
  })));
}

function receiptBytes(value: string): Uint8Array {
  if (typeof value !== "string" || value.length > 131072 || !/^0x(?:[0-9a-f]{2})+$/u.test(value)) {
    throw new AgentMiddlewareError("verification-failure");
  }
  return Uint8Array.from(value.slice(2).match(/../gu)!, (byte) => Number.parseInt(byte, 16));
}

type AgentPaymentTerms = Pick<AgentSpendRequest, "amount" | "asset" | "recipient" | "commitment">;

async function verifyAgentReceiptPayment(
  evidence: AgentReceiptEvidence,
  request: AgentPaymentTerms,
  receiptKeys: readonly SequencerReceiptKey[],
  commitments?: PaymentCommitmentResolver,
): Promise<PaymentReceiptVerification> {
  const proof = structuredClone(evidence);
  const rawTransaction = receiptBytes(proof.rawTransaction);
  const signedReceipt = receiptBytes(proof.signedReceipt);
  const keys = copyReceiptKeys(receiptKeys);
  const signed = await verifySequencerReceipt(signedReceipt, rawTransaction, keys);
  const network = `eip155:${decodeRawEip1559(rawTransaction).chainId}`;
  if (request.commitment !== undefined && request.commitment.network !== network) {
    throw new AgentMiddlewareError("verification-failure");
  }
  const topic = toHex(keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")));
  const log = signed.receipt.effects.find((effect) => effect.kind === 3
    && toHex(effect.body.subarray(0, 20)) === LAYERX_LEDGER_ADDRESS.slice(2)
    && effect.body[20] === 4 && toHex(effect.body.subarray(21, 53)) === topic);
  if (log === undefined) throw new AgentMiddlewareError("verification-failure");
  const verified = await verifyPaymentReceipt(proof, {
    scheme: "exact", network, amount: request.amount, asset: `0x${request.asset}`,
    payTo: `0x${request.recipient}`, maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", purposeHash: toHex(log.body.subarray(53, 85)) } },
  }, keys);
  const verification = Object.freeze({ ...verified, receiptDigest: keccak256(signedReceipt) });
  if (verification.payment.amount !== protocolAmount(request.amount)
    || !constantTimeHex(verification.payment.assetId, request.asset)
    || !constantTimeHex(verification.payment.to, request.recipient)) {
    throw new AgentMiddlewareError("verification-failure");
  }
  await verifyPaymentCommitment(verification, network, request.commitment?.level ?? "receipted", commitments);
  return verification;
}

export async function verifyCommittedAgentPayment(
  evidence: AgentReceiptEvidence,
  reservation: CommittedBudgetReservation,
  request: AgentPaymentTerms,
  receiptKeys: readonly SequencerReceiptKey[],
  commitments?: PaymentCommitmentResolver,
): Promise<Extract<AgentSpendResult, { readonly kind: "verified" }>> {
  if (!constantTimeHex(keccak256(receiptBytes(evidence.signedReceipt)), reservation.receiptDigest)
    || reservation.amount !== request.amount || reservation.asset !== request.asset) {
    throw new AgentMiddlewareError("budget-conflict");
  }
  const verification = await verifyAgentReceiptPayment(evidence, request, receiptKeys, commitments);
  return { kind: "verified", verification, reservation };
}

export async function verifyAgentPayment(
  evidence: AgentReceiptEvidence,
  request: AgentSpendRequest,
  receiptKeys: readonly SequencerReceiptKey[],
  commitments?: PaymentCommitmentResolver,
): Promise<PaymentReceiptVerification> {
  validateSpend(request);
  if (decodeRawEip1559(receiptBytes(evidence.rawTransaction)).chainId !== BigInt(request.networkId)) {
    throw new AgentMiddlewareError("verification-failure");
  }
  return verifyAgentReceiptPayment(evidence, request, receiptKeys, commitments);
}

export class AgentGrantMiddleware implements GrantDrawExecution {
  readonly #receiptKeys: readonly SequencerReceiptKey[];

  public constructor(private readonly config: {
    readonly tenant: string;
    readonly receiptKeys: readonly SequencerReceiptKey[];
    readonly budgets: AgentBudgetLedger;
    readonly draws: GrantDrawExecution;
    readonly receipts: AgentReceiptResolver;
    readonly commitments?: PaymentCommitmentResolver;
  }) {
    if (!config.tenant || config.tenant.length > 512 || config.tenant.includes("\0")) throw new AgentMiddlewareError("invalid-request");
    this.#receiptKeys = copyReceiptKeys(config.receiptKeys);
  }

  public async execute(request: Parameters<GrantDrawExecution["execute"]>[0]): Promise<SellerSettlementOutcome> {
    const requirements = request.requirements;
    if (requirements.scheme !== "metered" && requirements.scheme !== "subscription") throw new AgentMiddlewareError("invalid-request");
    const commitment = paymentCommitment(requirements.extra);
    if (commitment !== "receipted" && this.config.commitments === undefined) throw new AgentMiddlewareError("invalid-request");
    const amount = protocolAmount(requirements.amount).toString();
    if (!/^[0-9a-f]{64}$/u.test(request.requestDigest) || !/^[0-9a-f]{64}$/u.test(request.idempotencyKey)) throw new AgentMiddlewareError("invalid-request");
    const facts = { amount, asset: requirements.asset, requestDigest: request.requestDigest };
    const result = await this.config.budgets.reserve({ tenant: this.config.tenant, idempotencyKey: request.idempotencyKey, ...facts });
    if (result.kind === "exhausted") return { kind: "refused", reason: "budget_refused" };
    if (result.kind === "conflict") throw new AgentMiddlewareError("budget-conflict");
    const reservation = validateBudgetReservation(result.reservation, facts);
    if (reservation.state === "held") return { kind: "pending" };
    if (reservation.state === "released") return { kind: "refused", reason: "budget_released" };
    if (reservation.state === "committed") {
      const evidence = await this.config.receipts.resolve(reservation.receiptDigest);
      const outcome: SellerSettlementOutcome = { kind: "settled", ...evidence };
      const verification = await verifyPaymentReceipt(outcome, requirements, this.#receiptKeys, this.config.commitments);
      if (toHex(keccak256(receiptBytes(evidence.signedReceipt))) !== reservation.receiptDigest) throw new AgentMiddlewareError("budget-conflict");
      return outcome;
    }
    const outcome = await this.config.draws.execute(request);
    if (outcome.kind !== "settled") return outcome;
    const verification = await verifyPaymentReceipt(outcome, requirements, this.#receiptKeys, this.config.commitments);
    const receiptDigest = toHex(keccak256(receiptBytes(outcome.signedReceipt)));
    await this.config.receipts.retainVerified?.({
      idempotencyKey: request.idempotencyKey, activityId: toHex(verification.receipt.activityId), receiptDigest,
    });
    const committed = validateBudgetReservation(await this.config.budgets.commit({ reservationId: reservation.reservationId, ...facts, receiptDigest }), facts, reservation.reservationId);
    if (committed.state !== "committed" || committed.receiptDigest !== receiptDigest) throw new AgentMiddlewareError("budget-conflict");
    return outcome;
  }
}
