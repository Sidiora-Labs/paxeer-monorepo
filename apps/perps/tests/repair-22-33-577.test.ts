import assert from "node:assert/strict";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SearchParamsContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import type { Block } from "../src/types/paxscan/api/block";
import type { Transaction } from "../src/types/paxscan/api/transaction";
import type { AddressParam } from "../src/types/paxscan/api/addressParams";

const blockHash = `0x${"ab".repeat(32)}`;
const txHash = `0x${"cd".repeat(32)}`;
const missingHash = `0x${"ef".repeat(32)}`;
const account: AddressParam = {
  hash: `0x${"12".repeat(20)}`,
  implementations: [], name: null, is_contract: false, is_verified: null,
  ens_domain_name: null, private_tags: [], watchlist_names: [], public_tags: [],
};
// Committed Paxscan HTTP fixture bodies use the catalog's production types.
const block: Block = {
  hash: blockHash, height: 42, timestamp: "2026-05-12T16:11:09.515Z",
  transactions_count: 3, internal_transactions_count: 0, miner: account,
  parent_hash: `0x${"00".repeat(32)}`, gas_used: "63000", gas_limit: "30000000",
  nonce: "0x0000000000000000", burnt_fees: null, priority_fee: null,
  extra_data: null, state_root: null, gas_target_percentage: null,
  gas_used_percentage: null, burnt_fees_percentage: null, type: "block",
  transaction_fees: null, uncles_hashes: [],
};
const transaction: Transaction = {
  hash: txHash, block_number: 42, timestamp: block.timestamp, from: account,
  to: null, created_contract: { ...account, is_contract: true },
  result: "success", confirmations: 8, status: "ok", confirmation_duration: [1000],
  value: "900719925474099312345", fee: { type: "actual", value: "21000" },
  gas_price: "1", type: 0, gas_used: "21000", gas_limit: "21000",
  max_fee_per_gas: null, max_priority_fee_per_gas: null, priority_fee: null,
  base_fee_per_gas: null, transaction_burnt_fee: null, nonce: 1, position: 0,
  revert_reason: null, raw_input: "0x", decoded_input: null, token_transfers: [],
  token_transfers_overflow: false, exchange_rate: null, historic_exchange_rate: null,
  method: null, transaction_types: ["contract_creation"], transaction_tag: null,
  actions: [], has_error_in_internal_transactions: false,
};

async function explorerFixture() {
  const server = createServer();
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  const { default: ExplorerPage } = await import("../src/page-modules/explorer");
  const { explorerDetailQuery, selectExplorerDetail } = await import("../src/page-modules/explorer/detail");
  const { getServerSideProps: blockRedirect } = await import("../src/page-modules/block-detail");
  const { getServerSideProps: txRedirect } = await import("../src/page-modules/tx-detail");
  const apiRequests: string[] = [];
  let status = 200;
  let body: unknown = block;
  server.on("request", (req, res) => {
    void (async () => {
      const url = new URL(req.url!, origin);
      if (url.pathname.startsWith("/block/") || url.pathname.startsWith("/tx/")) {
        const isBlock = url.pathname.startsWith("/block/");
        const value = decodeURIComponent(url.pathname.split("/")[2]);
        const result = await (isBlock ? blockRedirect : txRedirect)({
          params: isBlock ? { height: value } : { hash: value },
          req: Object.assign(req, { cookies: {} }), res, query: {}, resolvedUrl: url.pathname,
        });
        assert.ok("redirect" in result);
        res.writeHead(307, { Location: result.redirect.destination }).end();
      } else if (url.pathname.startsWith("/api/v2/")) {
        apiRequests.push(url.pathname);
        res.writeHead(status, { "Content-Type": "application/json" }).end(JSON.stringify(body));
      } else {
        res.writeHead(404).end();
      }
    })().catch((error: unknown) => res.destroy(error instanceof Error ? error : new Error(String(error))));
  });
  const client = new QueryClient({ defaultOptions: { queries: { gcTime: Infinity } } });
  function render(params: URLSearchParams | null) {
    return renderToStaticMarkup(React.createElement(QueryClientProvider, { client },
      React.createElement(SearchParamsContext.Provider, { value: params }, React.createElement(ExplorerPage))));
  }
  function assertLive(html: string) {
    assert.match(html, /Latest Blocks/);
    assert.match(html, /Latest Transactions/);
  }
  return {
    client, apiRequests, render, assertLive, selectExplorerDetail, explorerDetailQuery,
    reply(value: unknown, code = 200) { body = value; status = code; },
    async redirect(path: string) {
      const response = await fetch(`${origin}${path}`, { redirect: "manual" });
      assert.equal(response.status, 307);
      const destination = response.headers.get("location");
      assert.ok(destination);
      const url = new URL(destination, origin);
      assert.equal(url.pathname, "/explorer");
      return url.searchParams;
    },
    async load(params: URLSearchParams) {
      return client.fetchQuery(explorerDetailQuery(selectExplorerDetail(params)));
    },
    async close() {
      client.clear();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    },
  };
}

async function explorerQueryConsumerResolvesLegacyDetailsAndRefusesInvalidSelections() {
  const h = await explorerFixture();
  try {
    for (const selector of ["42", blockHash]) {
      h.client.clear();
      h.reply(block);
      const params = await h.redirect(`/block/${selector}`);
      assert.equal(params.get("block"), selector);
      const loading = h.render(params);
      assert.match(loading, /role="status"/);
      assert.match(loading, /Loading block/);
      h.assertLive(loading);
      await h.load(params);
      assert.equal(h.apiRequests.at(-1), `/api/v2/blocks/${selector}`);
      const html = h.render(params);
      assert.match(html, /Block #42/);
      assert.ok(html.includes(blockHash));
      assert.ok(html.includes(block.timestamp));
      assert.ok(html.includes(account.hash));
      assert.match(html, /<dt>Transactions<\/dt><dd>3<\/dd>/);
      assert.match(html, /href="\/explorer"/);
      h.assertLive(html);
      // A fresh query client, as on refresh, resolves the same URL again.
      h.client.clear();
      await h.load(params);
      assert.match(h.render(params), /Block #42/);
    }
    const params = await h.redirect(`/tx/${txHash}`);
    assert.equal(params.get("tx"), txHash);
    assert.match(h.render(params), /Loading transaction/);
    h.reply(transaction);
    await h.load(params);
    assert.equal(h.apiRequests.at(-1), `/api/v2/transactions/${txHash}`);
    let html = h.render(params);
    assert.match(html, /Transaction details/);
    assert.ok(html.includes(txHash));
    assert.ok(html.includes(transaction.value));
    assert.match(html, /Success/);
    assert.match(html, /Contract creation/);
    assert.match(html, /href="\/explorer\?block=42"/);
    h.assertLive(html);
    h.client.clear();
    h.reply({ ...transaction, block_number: null, timestamp: null, status: null });
    await h.load(params);
    html = h.render(params);
    assert.match(html, /Pending/);
    assert.doesNotMatch(html, /#null|Invalid Date|Success/);
    h.assertLive(html);

    const beforeInvalid = h.apiRequests.length;
    for (const query of [
      `block=42&tx=${txHash}`, "block=42&block=43", `tx=${txHash}&tx=${txHash}`,
      "block=", "tx=", "block=-1", "block=1.5", "block=9007199254740992",
      "block=latest", "block=0x1234", "tx=not-a-hash", "tx=42", "block=%2F..%2Ftx",
    ]) {
      const invalid = new URLSearchParams(query);
      const selection = h.selectExplorerDetail(invalid);
      assert.equal(selection.kind, "refused", query);
      assert.equal(h.explorerDetailQuery(selection).enabled, false);
      html = h.render(invalid);
      assert.match(html, /role="alert"/, query);
      assert.doesNotMatch(html, /Loading block|Loading transaction|Transaction details|Block #42/);
      h.assertLive(html);
    }
    assert.equal(h.apiRequests.length, beforeInvalid);
    for (const empty of [null, new URLSearchParams(), new URLSearchParams("other=42")]) {
      html = h.render(empty);
      assert.doesNotMatch(html, /Selected explorer detail|role="alert"/);
      h.assertLive(html);
    }
    assert.deepEqual(h.selectExplorerDetail(new URLSearchParams("block=0")), { kind: "block", value: "0" });

    for (const [query, reply, code, expected] of [
      [`tx=${missingHash}`, { message: "Not found" }, 404, /not found/],
      ["block=42", null, 200, /invalid block response/],
      ["block=42", { ...block, height: 43 }, 200, /different block/],
      [`block=${blockHash}`, { ...block, hash: missingHash }, 200, /different block/],
      [`tx=${txHash}`, { ...transaction, hash: missingHash }, 200, /different transaction/],
      [`tx=${txHash}`, { ...transaction, from: account.hash }, 200, /invalid transaction response/],
      [`tx=${txHash}`, { ...transaction, block_number: "0x2a" }, 200, /invalid transaction response/],
      ["block=42", { ...block, timestamp: "bad" }, 200, /invalid block response/],
    ] as const) {
      h.client.clear();
      h.reply(reply, code);
      const refused = new URLSearchParams(query);
      await assert.rejects(h.load(refused));
      html = h.render(refused);
      assert.match(html, /role="alert"/);
      assert.match(html, expected);
      assert.match(html, /Retry detail/);
      h.assertLive(html);
    }
    // A later valid selector does not retain the previous detail or refusal.
    h.reply(block);
    await h.load(new URLSearchParams("block=42"));
    html = h.render(new URLSearchParams("block=42"));
    assert.match(html, /Block #42/);
    assert.doesNotMatch(html, /role="alert"|Transaction details/);
    h.assertLive(html);
  } finally {
    await h.close();
  }
}

test(explorerQueryConsumerResolvesLegacyDetailsAndRefusesInvalidSelections);
