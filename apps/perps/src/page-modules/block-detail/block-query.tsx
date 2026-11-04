import React from "react";
import type { ParsedUrlQuery } from "node:querystring";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { buildUrl, paxscanFetch, PaxscanError } from "@/core/paxscan/api";
import { FetchPolicyError } from "@/core/net/fetch";
import type { Block } from "@/types/paxscan/api/block";

export function blockQueryIdentifier(query: ParsedUrlQuery): string | null {
  const raw = query.block;
  return (Array.isArray(raw) ? raw[0] : raw) ?? null;
}

function validBlockIdentifier(identifier: string): boolean {
  return /^\d+$/.test(identifier) || /^0x[\da-f]{64}$/i.test(identifier);
}

function blockNotFound(error: Error): boolean {
  return (
    (error instanceof PaxscanError || error instanceof FetchPolicyError) &&
    error.status === 404
  );
}

export function blockDetailQueryOptions(identifier: string | null) {
  return queryOptions({
    queryKey: ["paxscan", "general:block", { height_or_hash: identifier }, null],
    queryFn: ({ signal }) => {
      if (identifier === null || !validBlockIdentifier(identifier)) {
        throw new Error("Invalid block identifier.");
      }
      return paxscanFetch<Block>(
        buildUrl("general:block", { height_or_hash: identifier }),
        { signal },
      );
    },
    enabled: identifier !== null && validBlockIdentifier(identifier),
    retry: (failures, error) => !blockNotFound(error) && failures < 2,
    retryOnMount: false,
  });
}

export function BlockQueryDetail({ query }: { query: ParsedUrlQuery }) {
  const identifier = blockQueryIdentifier(query);
  const { data: block, error, isPending } = useQuery(
    blockDetailQueryOptions(identifier),
  );

  if (identifier === null) return null;
  if (!validBlockIdentifier(identifier)) {
    return <p role="alert">Invalid block identifier: {identifier}</p>;
  }
  if (error) {
    return (
      <section aria-label="Block details">
        <h2>Block {identifier}</h2>
        <p role="alert">
          {blockNotFound(error)
            ? "Block not found."
            : "Unable to load block details. Please try again."}
        </p>
      </section>
    );
  }
  if (isPending) return <p role="status">Loading block {identifier}…</p>;

  return (
    <section aria-label="Block details" className="p-4 overflow-auto">
      <h2 className="text-base font-semibold">Block #{block.height}</h2>
      <dl className="text-xs font-mono break-all space-y-2">
        <dt>Hash</dt>
        <dd>{block.hash}</dd>
        <dt>Parent hash</dt>
        <dd>{block.parent_hash}</dd>
        <dt>Timestamp</dt>
        <dd>{block.timestamp}</dd>
        <dt>Transactions</dt>
        <dd>{block.transactions_count}</dd>
        <dt>Miner</dt>
        <dd>{block.miner.hash}</dd>
        <dt>Gas used</dt>
        <dd>{block.gas_used ?? "Unavailable"}</dd>
        <dt>Gas limit</dt>
        <dd>{block.gas_limit}</dd>
      </dl>
    </section>
  );
}
