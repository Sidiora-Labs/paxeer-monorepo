import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { Socket, type AddressInfo } from "node:net";
import type { ParsedUrlQuery } from "node:querystring";
import { test, type TestContext } from "node:test";
import type { GetServerSidePropsContext } from "next";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  QueryClient,
  QueryClientProvider,
  QueryObserver,
} from "@tanstack/react-query";
import type { Transaction } from "../src/types/paxscan/api/transaction";

async function txDeepLinkRealTransport(t: TestContext) {
  const committedHash = `0x${"ab".repeat(32)}`;
  const missingHash = `0x${"22".repeat(32)}`;
  const wrongHash = `0x${"33".repeat(32)}`;
  const malformedHash = `0x${"44".repeat(32)}`;
  const unavailableHash = `0x${"55".repeat(32)}`;
  const pendingHash = `0x${"77".repeat(32)}`;
  const from = {
    hash: `0x${"12".repeat(20)}`,
    implementations: null,
    name: null,
    is_contract: false,
    is_verified: null,
    ens_domain_name: null,
    private_tags: null,
    watchlist_names: null,
    public_tags: null,
  };
  const transaction: Transaction = {
    hash: committedHash,
    block_number: 12345,
    timestamp: "2026-10-01T12:00:00.000000Z",
    status: "ok",
    result: "success",
    confirmations: 12,
    confirmation_duration: [1000, 1000],
    from,
    to: { ...from, hash: `0x${"34".repeat(20)}` },
    created_contract: null,
    value: "123456789012345678901234567890",
    fee: { type: "actual", value: "1407000000000" },
    gas_price: "67000000",
    type: 2,
    gas_used: "21000",
    gas_limit: "21000",
    max_fee_per_gas: "67000000",
    max_priority_fee_per_gas: "0",
    priority_fee: "0",
    base_fee_per_gas: "67000000",
    transaction_burnt_fee: "1407000000000",
    nonce: 5,
    position: 0,
    revert_reason: null,
    raw_input: "0x",
    decoded_input: null,
    token_transfers: [],
    token_transfers_overflow: false,
    exchange_rate: null,
    historic_exchange_rate: null,
    method: null,
    transaction_types: ["coin_transfer"],
    transaction_tag: null,
    actions: [],
    has_error_in_internal_transactions: false,
  };
  const responses = new Map<string, { status: number; body: unknown }>([
    [committedHash, { status: 200, body: transaction }],
    [missingHash, { status: 404, body: { message: "Not found" } }],
    [wrongHash, { status: 200, body: transaction }],
    [malformedHash, {
      status: 200,
      body: { ...transaction, hash: malformedHash, gas_limit: null },
    }],
    [unavailableHash, { status: 503, body: { message: "Indexer unavailable" } }],
    [pendingHash, { status: 200, body: {
      ...transaction,
      hash: pendingHash,
      block_number: null,
      timestamp: null,
      status: null,
      result: "pending",
      gas_used: null,
    } }],
  ]);
  const requests: string[] = [];
  const server = createServer((req, res) => {
    requests.push(req.url!);
    const hash = req.url?.match(
      /^\/api\/v2\/transactions\/(0x[0-9a-f]{64})$/i,
    )?.[1];
    const response = hash ? responses.get(hash.toLowerCase()) : undefined;
    res.writeHead(response?.status ?? 400, {
      "content-type": "application/json",
    });
    res.end(JSON.stringify(
      response?.body ?? { message: "Unexpected fixture request" },
    ));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const environment = {
    NEXT_PUBLIC_PERPS_GATEWAY_URL: origin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: origin,
    NEXT_PUBLIC_PAXSCAN_API_URL: `${origin}/api/v2`,
    NEXT_PUBLIC_PAXSCAN_WS_URL: origin.replace("http:", "ws:"),
  };
  const previous = new Map<string, string | undefined>();
  for (const [key, value] of Object.entries(environment)) {
    previous.set(key, process.env[key]);
    process.env[key] = value;
  }

  const clients: QueryClient[] = [];
  try {
    const {
      getServerSideProps,
      TransactionQueryDetail,
      selectTransaction,
      transactionDetailOptions,
    } = await import("../src/page-modules/tx-detail");
    const page = await import("../src/pages/tx/[hash]");
    assert.equal(page.getServerSideProps, getServerSideProps);

    function client() {
      const instance = new QueryClient({
        defaultOptions: { queries: { gcTime: Infinity } },
      });
      clients.push(instance);
      return instance;
    }

    function render(instance: QueryClient, query: ParsedUrlQuery) {
      return renderToStaticMarkup(createElement(
        QueryClientProvider,
        { client: instance },
        createElement(TransactionQueryDetail, { query }),
      ));
    }

    async function redirectQuery(
      raw: string | string[] | undefined,
      query: ParsedUrlQuery = {},
    ): Promise<ParsedUrlQuery> {
      const socket = new Socket();
      const req = Object.assign(new IncomingMessage(socket), { cookies: {} });
      const context: GetServerSidePropsContext = {
        req,
        res: new ServerResponse(req),
        params: raw === undefined ? {} : { hash: raw },
        query,
        resolvedUrl: `/tx/${Array.isArray(raw) ? raw[0] : raw ?? ""}`,
      };
      try {
        const result = await page.getServerSideProps(context);
        assert.ok("redirect" in result);
        assert.ok("permanent" in result.redirect);
        assert.equal(result.redirect.permanent, false);
        const destination = new URL(result.redirect.destination, origin);
        assert.equal(destination.origin, origin);
        assert.equal(destination.pathname, "/explorer");
        return Object.fromEntries(destination.searchParams);
      } finally {
        socket.destroy();
      }
    }

    async function resolve(instance: QueryClient, hash: string) {
      const observer = new QueryObserver(instance, transactionDetailOptions(hash));
      let unsubscribe = () => {};
      try {
        await new Promise<void>((done) => {
          unsubscribe = observer.subscribe((state) => {
            if (!state.isPending) done();
          });
          if (!observer.getCurrentResult().isPending) done();
        });
        return observer.getCurrentResult();
      } finally {
        unsubscribe();
        observer.destroy();
      }
    }

    async function txDeepLinkCommittedTransactionSurvivesRefresh() {
      const requestedHash = `0x${"AB".repeat(32)}`;
      const query = await redirectQuery(requestedHash, { tx: missingHash });
      assert.deepEqual(query, { tx: requestedHash });
      const before = requests.length;
      for (let refresh = 0; refresh < 2; refresh++) {
        const instance = client();
        const reopenedQuery = Object.fromEntries(
          new URLSearchParams(query as Record<string, string>),
        );
        const loading = render(instance, reopenedQuery);
        assert.match(loading, /Loading transaction/);
        assert.match(loading, new RegExp(requestedHash));
        const state = await resolve(instance, requestedHash);
        assert.equal(state.status, "success");
        assert.equal(state.data?.kind, "found");
        const html = render(instance, reopenedQuery);
        assert.match(html, /aria-label="Transaction details"/);
        assert.match(html, new RegExp(`data-transaction-hash="${requestedHash}"`));
        assert.match(html, /#12345/);
        assert.match(html, /success/);
        assert.match(html, new RegExp(from.hash));
        assert.match(html, new RegExp(transaction.to!.hash));
        assert.match(html, new RegExp(transaction.value));
        assert.match(html, /21000/);
        assert.doesNotMatch(html, /Loading transaction|not found|Unable to load/);
        instance.clear();
      }
      assert.deepEqual(requests.slice(before), [
        `/api/v2/transactions/${requestedHash}`,
        `/api/v2/transactions/${requestedHash}`,
      ]);
    }

    async function txDeepLinkMissingTransactionPreservesIdentity() {
      const query = await redirectQuery(missingHash);
      for (let refresh = 0; refresh < 2; refresh++) {
        const instance = client();
        const state = await resolve(instance, String(query.tx));
        assert.equal(state.status, "success");
        assert.deepEqual(state.data, { kind: "missing" });
        const html = render(instance, query);
        assert.match(html, new RegExp(missingHash));
        assert.match(html, /Transaction not found/);
        assert.doesNotMatch(html, /#12345|success|Loading transaction|Unable to load/);
        instance.clear();
      }
      assert.deepEqual(requests.slice(-2), [
        `/api/v2/transactions/${missingHash}`,
        `/api/v2/transactions/${missingHash}`,
      ]);
    }

    async function txDeepLinkRejectsAmbiguousSelectors() {
      const before = requests.length;
      const instance = client();
      for (const query of [
        { tx: [committedHash, missingHash] },
        { tx: committedHash, block: "12345" },
        { tx: committedHash, block: "" },
        { tx: "" },
        { tx: "not-a-hash" },
        { tx: `${committedHash}/?tx=${missingHash}` },
      ]) {
        assert.equal(selectTransaction(query).kind, "invalid");
        assert.match(render(instance, query), /role="alert"/);
        assert.doesNotMatch(render(instance, query), /Loading transaction|#12345/);
      }
      assert.deepEqual(selectTransaction({}), { kind: "none" });
      assert.equal(render(instance, {}), "");
      assert.equal(render(instance, { block: "12345" }), "");
      assert.deepEqual(
        await redirectQuery(undefined, { tx: committedHash }),
        {},
      );
      assert.deepEqual(await redirectQuery(""), {});
      assert.deepEqual(await redirectQuery([committedHash, missingHash]), {
        tx: committedHash,
      });
      const delimiters = "a/b?c=d&x=+ #%雪";
      assert.deepEqual(await redirectQuery(delimiters), { tx: delimiters });
      assert.equal(requests.length, before);
    }

    async function txDeepLinkRejectsWrongIdentityAndInvalidResponses() {
      for (const hash of [wrongHash, malformedHash]) {
        const instance = client();
        const query = await redirectQuery(hash);
        const state = await resolve(instance, hash);
        assert.equal(state.status, "error");
        assert.match(
          state.error!.message,
          hash === wrongHash
            ? /different transaction/
            : /invalid transaction details/,
        );
        assert.equal(state.data, undefined);
        const html = render(instance, query);
        assert.match(html, new RegExp(hash));
        assert.match(html, /role="alert"/);
        assert.match(html, /Unable to load this transaction/);
        assert.doesNotMatch(html, /Transaction not found|#12345|success/);
      }
    }

    async function txDeepLinkDistinguishesUnavailableFromMissing() {
      const instance = client();
      const query = await redirectQuery(unavailableHash);
      const state = await resolve(instance, unavailableHash);
      assert.equal(state.status, "error");
      assert.equal(state.data, undefined);
      const html = render(instance, query);
      assert.match(html, new RegExp(unavailableHash));
      assert.match(html, /Unable to load this transaction/);
      assert.doesNotMatch(html, /Transaction not found|#12345|success/);
    }

    async function txDeepLinkQuerySwitchDoesNotReusePreviousDetails() {
      const instance = client();
      await resolve(instance, committedHash);
      const nextQuery = await redirectQuery(missingHash);
      const loading = render(instance, nextQuery);
      assert.match(loading, /Loading transaction/);
      assert.match(loading, new RegExp(missingHash));
      assert.doesNotMatch(
        loading,
        new RegExp(`${committedHash}|${from.hash}|#12345`),
      );
      await resolve(instance, missingHash);
      assert.match(render(instance, nextQuery), /Transaction not found/);
      const pendingQuery = await redirectQuery(pendingHash);
      await resolve(instance, pendingHash);
      const pending = render(instance, pendingQuery);
      assert.match(pending, new RegExp(pendingHash));
      assert.match(pending, /Pending/);
      assert.doesNotMatch(pending, /#12345|Transaction not found|Unable to load/);
    }

    await t.test(txDeepLinkCommittedTransactionSurvivesRefresh);
    await t.test(txDeepLinkMissingTransactionPreservesIdentity);
    await t.test(txDeepLinkRejectsAmbiguousSelectors);
    await t.test(txDeepLinkRejectsWrongIdentityAndInvalidResponses);
    await t.test(txDeepLinkDistinguishesUnavailableFromMissing);
    await t.test(txDeepLinkQuerySwitchDoesNotReusePreviousDetails);
  } finally {
    for (const instance of clients) instance.clear();
    for (const [key, value] of previous) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    server.closeAllConnections();
    await new Promise<void>((done, reject) => {
      server.close((error) => error ? reject(error) : done());
    });
  }
}

test({ timeout: 15_000 }, txDeepLinkRealTransport);
