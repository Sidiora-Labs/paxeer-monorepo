import { DatabaseSync } from "node:sqlite";
import { decodeRawEip1559 } from "../evm-transaction.js";
import { verifySequencerReceipt, type SequencerReceiptKey, type SequencerReceiptVerification } from "../verifier.js";
import { validateGrantDraw, type GrantOffer } from "./grant.js";
import { bindDrawTransaction, type ExpectedDrawTransaction } from "./activity.js";

export interface GrantDrawRpc {
  sendRawTransaction(raw: string): Promise<string>;
  sequencerReceipt(transactionHash: string): Promise<Record<string, unknown> | null>;
}

type GrantDrawResult = { readonly kind: "pending" }
  | { readonly kind: "settled"; readonly canonicalReceipt: Uint8Array; readonly verifiedReceipt: SequencerReceiptVerification };

export class PreparedGrantDraws {
  readonly #db: DatabaseSync;
  readonly #receiptKeys: readonly SequencerReceiptKey[];
  readonly #inFlight = new Map<string, Promise<GrantDrawResult>>();

  public constructor(path: string, private readonly actor: string, private readonly chainId: bigint,
    private readonly rpc: GrantDrawRpc, receiptKeys: readonly SequencerReceiptKey[]) {
    if (!/^[0-9a-f]{64}$/u.test(actor) || /^0+$/u.test(actor) || chainId <= 0n) throw new Error("invalid-draw-registration");
    this.#receiptKeys = receiptKeys.map((key) => ({ ...key, keyId: key.keyId.slice(), publicKey: key.publicKey.slice() }));
    this.#db = new DatabaseSync(path);
    this.#db.exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=30000; BEGIN IMMEDIATE");
    try {
      this.#db.exec("CREATE TABLE IF NOT EXISTS grant_draw_transactions (idempotency_key TEXT PRIMARY KEY, principal TEXT NOT NULL, request_digest TEXT NOT NULL, raw_transaction BLOB NOT NULL, calldata BLOB NOT NULL, transaction_hash TEXT NOT NULL UNIQUE, period_key TEXT UNIQUE, attempted INTEGER NOT NULL DEFAULT 0)");
      const columns = this.#db.prepare("PRAGMA table_info(grant_draw_transactions)").all();
      if (!columns.some((column) => column["name"] === "admission_state")) {
        this.#db.exec("ALTER TABLE grant_draw_transactions ADD COLUMN admission_state TEXT NOT NULL DEFAULT 'prepared' CHECK (admission_state IN ('prepared', 'unknown', 'admitted')); UPDATE grant_draw_transactions SET admission_state = 'unknown' WHERE attempted <> 0");
      }
      this.#db.exec("COMMIT");
    } catch (error) {
      this.#db.exec("ROLLBACK");
      this.#db.close();
      throw error;
    }
  }

  public close(): void { this.#db.close(); }

  public register(principal: string, requestDigest: string, raw: Uint8Array, expected: ExpectedDrawTransaction, periodKey?: string): void {
    if (!principal || !/^[0-9a-f]{64}$/u.test(requestDigest) || (periodKey !== undefined && !periodKey)
      || expected.chainId !== this.chainId || expected.recipient !== this.actor) throw new Error("invalid-draw-registration");
    const transactionHash = bindDrawTransaction(raw, expected);
    const calldata = decodeRawEip1559(raw).data;
    const existing = this.#db.prepare("SELECT * FROM grant_draw_transactions WHERE idempotency_key = ?").get(expected.idempotencyKey);
    if (existing) {
      if (existing["principal"] !== principal || existing["request_digest"] !== requestDigest
        || existing["transaction_hash"] !== transactionHash || existing["period_key"] !== (periodKey ?? null)) throw new Error("draw-registration-conflict");
      return;
    }
    this.#db.prepare("INSERT INTO grant_draw_transactions (idempotency_key, principal, request_digest, raw_transaction, calldata, transaction_hash, period_key) VALUES (?, ?, ?, ?, ?, ?, ?)")
      .run(expected.idempotencyKey, principal, requestDigest, raw, calldata, transactionHash, periodKey ?? null);
  }

  public async execute(request: { readonly principal: string; readonly requestDigest: string; readonly draw: string; readonly idempotencyKey: string; readonly requirements: GrantOffer }): Promise<GrantDrawResult> {
    const row = this.#db.prepare("SELECT * FROM grant_draw_transactions WHERE idempotency_key = ?").get(request.idempotencyKey);
    if (!row || row["principal"] !== request.principal || row["request_digest"] !== request.requestDigest
      || Buffer.from(row["calldata"] as Uint8Array).toString("hex") !== request.draw) throw new Error("unregistered-grant-draw");
    const draw = validateGrantDraw(row["calldata"] as Uint8Array, request.requirements, request.idempotencyKey);
    if (draw.recipient !== this.actor) throw new Error("draw-recipient-mismatch");
    if (request.requirements.scheme === "subscription" && row["period_key"] === null) throw new Error("subscription-period-required");
    const inFlight = this.#inFlight.get(request.idempotencyKey);
    if (inFlight) return inFlight;
    const admissionState = row["admission_state"];
    if (admissionState !== "prepared" && admissionState !== "unknown" && admissionState !== "admitted") throw new Error("invalid-draw-admission-state");
    const execution = this.#execute(request.idempotencyKey, row["transaction_hash"] as string,
      row["raw_transaction"] as Uint8Array, admissionState);
    this.#inFlight.set(request.idempotencyKey, execution);
    try {
      return await execution;
    } finally {
      this.#inFlight.delete(request.idempotencyKey);
    }
  }

  async #execute(idempotencyKey: string, transactionHash: string, raw: Uint8Array,
    admissionState: "prepared" | "unknown" | "admitted"): Promise<GrantDrawResult> {
    let result: Record<string, unknown> | null = null;
    let reconciliationFailure: { readonly error: unknown } | undefined;
    try {
      if (admissionState !== "prepared") {
        try {
          result = await this.rpc.sequencerReceipt(transactionHash);
        } catch (error) {
          if (admissionState === "admitted") throw error;
          if (!isProtocolPending(error)) reconciliationFailure = { error };
        }
      }
      if (result === null && admissionState !== "admitted") {
        const claimed = this.#db.prepare("UPDATE grant_draw_transactions SET attempted = 1, admission_state = 'unknown' WHERE idempotency_key = ? AND admission_state <> 'admitted'").run(idempotencyKey);
        if (claimed.changes === 1) {
          try {
            const submittedHash = await this.rpc.sendRawTransaction(`0x${Buffer.from(raw).toString("hex")}`);
            if (submittedHash !== transactionHash) throw new Error("draw-transaction-mismatch");
          } catch (error) {
            if (reconciliationFailure) throw new AggregateError([reconciliationFailure.error, error], "draw-admission-unresolved");
            throw error;
          }
          this.#db.prepare("UPDATE grant_draw_transactions SET admission_state = 'admitted' WHERE idempotency_key = ?").run(idempotencyKey);
        }
        if (reconciliationFailure) throw reconciliationFailure.error;
        result = await this.rpc.sequencerReceipt(transactionHash);
      }
    } catch (error) {
      if (isProtocolPending(error)) return { kind: "pending" };
      throw error;
    }
    if (result === null) return { kind: "pending" };
    if (result["txHash"] !== transactionHash) throw new Error("draw-transaction-mismatch");
    if (result["rung"] === "submitted") {
      this.#db.prepare("UPDATE grant_draw_transactions SET admission_state = 'admitted' WHERE idempotency_key = ?").run(idempotencyKey);
      return { kind: "pending" };
    }
    if ((result["rung"] !== "receipted" && result["rung"] !== "committed")
      || typeof result["receipt"] !== "string" || result["receipt"].length > 2097154
      || !/^0x(?:[0-9a-f]{2})+$/u.test(result["receipt"])) throw new Error("draw-receipt-unavailable");
    const canonicalReceipt = Uint8Array.from(Buffer.from(result["receipt"].slice(2), "hex"));
    const verifiedReceipt = await verifySequencerReceipt(canonicalReceipt, raw, this.#receiptKeys);
    if (verifiedReceipt.receipt.resultCode !== 0) throw new Error("draw-transaction-failed");
    this.#db.prepare("UPDATE grant_draw_transactions SET admission_state = 'admitted' WHERE idempotency_key = ?").run(idempotencyKey);
    return { kind: "settled", canonicalReceipt, verifiedReceipt };
  }
}

function isProtocolPending(error: unknown): boolean {
  if (!(error instanceof Error) || !("code" in error) || !("data" in error)
    || (error as { readonly code?: unknown }).code !== -32001) return false;
  const data = (error as { readonly data?: unknown }).data;
  return data !== null && typeof data === "object" && !Array.isArray(data)
    && (data as Record<string, unknown>)["state"] === "pending";
}
