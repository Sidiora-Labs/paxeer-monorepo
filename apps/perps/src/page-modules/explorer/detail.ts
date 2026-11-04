import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";
import { buildUrl, paxscanFetch } from "@/core/paxscan/api";
import type { Block } from "@/types/paxscan/api/block";
import type { Transaction } from "@/types/paxscan/api/transaction";

export type ExplorerSelection =
  | { kind: "none" }
  | { kind: "refused"; reason: string }
  | { kind: "block" | "tx"; value: string };

const hash = z.string().regex(/^0x[0-9a-fA-F]{64}$/);
const address = z.object({ hash: z.string().regex(/^0x[0-9a-fA-F]{40}$/) });
const timestamp = z.string().refine((value) => Number.isFinite(Date.parse(value)));
const height = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const quantity = z.string().regex(/^\d+$/);

type BlockDetail = Pick<Block, "hash" | "height" | "timestamp" | "transactions_count"> & {
  miner: Pick<Block["miner"], "hash">;
};
type TransactionDetail = Pick<Transaction, "hash" | "block_number" | "timestamp" | "status" | "value"> & {
  from: Pick<Transaction["from"], "hash">;
  to: Pick<NonNullable<Transaction["to"]>, "hash"> | null;
};
const blockSchema: z.ZodType<BlockDetail> = z.object({
  hash,
  height,
  timestamp,
  transactions_count: z.number().int().nonnegative(),
  miner: address,
});
const transactionSchema: z.ZodType<TransactionDetail> = z.object({
  hash,
  block_number: height.nullable(),
  timestamp: timestamp.nullable(),
  status: z.enum(["ok", "error"]).nullable(),
  value: quantity,
  from: address,
  to: address.nullable(),
});

export type ExplorerDetail =
  | { kind: "block"; block: BlockDetail }
  | { kind: "tx"; transaction: TransactionDetail };

export function selectExplorerDetail(params: Pick<URLSearchParams, "getAll">): ExplorerSelection {
  const blocks = params.getAll("block");
  const txs = params.getAll("tx");
  if (blocks.length === 0 && txs.length === 0) return { kind: "none" };
  if (blocks.length + txs.length !== 1) {
    return { kind: "refused", reason: "Choose exactly one block or transaction; repeated selectors are ambiguous." };
  }
  const kind = blocks.length ? "block" : "tx";
  const value = (blocks.length ? blocks : txs)[0];
  if (hash.safeParse(value).success) return { kind, value };
  if (kind === "block" && /^(0|[1-9]\d*)$/.test(value) && BigInt(value) <= BigInt(Number.MAX_SAFE_INTEGER)) {
    return { kind, value };
  }
  return { kind: "refused", reason: kind === "block"
    ? "A block selector must be a non-negative height or a 32-byte 0x hash."
    : "A transaction selector must be a 32-byte 0x hash." };
}

export function explorerDetailQuery(selection: ExplorerSelection) {
  return queryOptions({
    queryKey: ["explorer-detail", selection.kind, "value" in selection ? selection.value : null],
    enabled: selection.kind === "block" || selection.kind === "tx",
    retry: false,
    retryOnMount: false,
    staleTime: 0,
    queryFn: async ({ signal }): Promise<ExplorerDetail> => {
      if (selection.kind === "block") {
        const raw = await paxscanFetch<unknown>(buildUrl("general:block", { height_or_hash: selection.value }), { signal });
        const parsed = blockSchema.safeParse(raw);
        if (!parsed.success) throw new Error("The explorer returned an invalid block response.");
        const block = parsed.data;
        const matches = /^\d+$/.test(selection.value)
          ? block.height.toString() === selection.value
          : block.hash.toLowerCase() === selection.value.toLowerCase();
        if (!matches) throw new Error("The explorer returned a different block than requested.");
        return { kind: "block", block };
      }
      if (selection.kind === "tx") {
        const raw = await paxscanFetch<unknown>(buildUrl("general:transaction", { hash: selection.value }), { signal });
        const parsed = transactionSchema.safeParse(raw);
        if (!parsed.success) throw new Error("The explorer returned an invalid transaction response.");
        const transaction = parsed.data;
        if (transaction.hash.toLowerCase() !== selection.value.toLowerCase()) {
          throw new Error("The explorer returned a different transaction than requested.");
        }
        return { kind: "tx", transaction };
      }
      throw new Error("Choose a valid block or transaction.");
    },
  });
}
