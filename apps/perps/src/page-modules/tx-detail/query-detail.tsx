import type { ParsedUrlQuery } from "node:querystring";
import React from "react";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { FetchPolicyError } from "@/core/net/fetch";
import { buildUrl, paxscanFetch, PaxscanError } from "@/core/paxscan/api";

const hashSchema = z.string().regex(/^0x[0-9a-f]{64}$/i);
const addressSchema = z.object({
  hash: z.string().regex(/^0x[0-9a-f]{40}$/i),
});
const quantitySchema = z.string().regex(/^\d+$/);
const transactionSchema = z.object({
  hash: hashSchema,
  block_number: z.number().int().nonnegative().nullable(),
  timestamp: z.string().nullable(),
  status: z.enum(["ok", "error"]).nullish(),
  result: z.string(),
  from: addressSchema,
  to: addressSchema.nullable(),
  created_contract: addressSchema.nullable(),
  value: quantitySchema,
  gas_used: quantitySchema.nullable(),
  gas_limit: quantitySchema,
});

type TransactionSelection =
  | { kind: "none" }
  | { kind: "invalid"; message: string }
  | { kind: "selected"; hash: string };

type TransactionDetailResult =
  | { kind: "found"; transaction: z.infer<typeof transactionSchema> }
  | { kind: "missing" };

export function selectTransaction(query: ParsedUrlQuery): TransactionSelection {
  if (query.tx === undefined) return { kind: "none" };
  if (Array.isArray(query.tx) || query.block !== undefined) {
    return {
      kind: "invalid",
      message: "Choose exactly one transaction and no block selector.",
    };
  }
  if (!hashSchema.safeParse(query.tx).success) {
    return { kind: "invalid", message: "Invalid transaction hash." };
  }
  return { kind: "selected", hash: query.tx };
}

export function transactionDetailOptions(hash: string | null) {
  return queryOptions({
    queryKey: ["transaction-detail", hash] as const,
    enabled: hash !== null,
    retry: false,
    retryOnMount: false,
    queryFn: async ({ signal }): Promise<TransactionDetailResult> => {
      if (hash === null || !hashSchema.safeParse(hash).success) {
        throw new Error("Invalid transaction hash.");
      }
      let response: unknown;
      try {
        response = await paxscanFetch<unknown>(
          buildUrl("general:transaction", { hash }),
          { signal },
        );
      } catch (error) {
        if (
          (error instanceof FetchPolicyError || error instanceof PaxscanError) &&
          error.status === 404
        ) {
          return { kind: "missing" };
        }
        throw error;
      }
      const parsed = transactionSchema.safeParse(response);
      if (!parsed.success) {
        throw new Error("The explorer returned invalid transaction details.");
      }
      if (parsed.data.hash.toLowerCase() !== hash.toLowerCase()) {
        throw new Error("The explorer returned a different transaction.");
      }
      return { kind: "found", transaction: parsed.data };
    },
  });
}

export function TransactionQueryDetail({ query }: { query: ParsedUrlQuery }) {
  const selection = selectTransaction(query);
  const result = useQuery(
    transactionDetailOptions(
      selection.kind === "selected" ? selection.hash : null,
    ),
  );

  if (selection.kind === "none") return null;
  if (selection.kind === "invalid") {
    return <p role="alert">{selection.message}</p>;
  }

  const transaction =
    result.data?.kind === "found" ? result.data.transaction : null;
  return (
    <section
      aria-label="Transaction details"
      data-transaction-hash={selection.hash}
      className="p-4 border-b border-border space-y-2 text-sm"
    >
      <h2 className="font-semibold">Transaction details</h2>
      <p className="font-mono break-all">{selection.hash}</p>
      {result.isPending && <p role="status">Loading transaction…</p>}
      {result.isError && (
        <p role="alert">Unable to load this transaction. Refresh to retry.</p>
      )}
      {!result.isError && result.data?.kind === "missing" && (
        <p role="status">Transaction not found.</p>
      )}
      {!result.isError && transaction && (
        <dl className="space-y-1 break-all">
          <dt>Block</dt>
          <dd>
            {transaction.block_number === null
              ? "Pending"
              : `#${transaction.block_number}`}
          </dd>
          <dt>Result</dt>
          <dd>{transaction.result}</dd>
          <dt>From</dt>
          <dd>{transaction.from.hash}</dd>
          <dt>To</dt>
          <dd>
            {transaction.to?.hash ??
              transaction.created_contract?.hash ??
              "Contract creation"}
          </dd>
          <dt>Value</dt>
          <dd>{transaction.value}</dd>
          <dt>Gas limit</dt>
          <dd>{transaction.gas_limit}</dd>
          <dt>Gas used</dt>
          <dd>{transaction.gas_used ?? "Pending"}</dd>
        </dl>
      )}
    </section>
  );
}
