import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { Socket, type AddressInfo } from "node:net";
import { after, before, test } from "node:test";
import type { GetServerSidePropsContext } from "next";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Block } from "../src/types/paxscan/api/block";

const HASH = `0x${"ab".repeat(32)}`;
const PARENT_HASH = `0x${"cd".repeat(32)}`;
const MINER = `0x${"ef".repeat(20)}`;
const block: Block = {
  height: 42,
  timestamp: "2026-06-01T12:00:00.000Z",
  transactions_count: 7,
  internal_transactions_count: 0,
  miner: {
    hash: MINER,
    implementations: null,
    name: null,
    is_contract: false,
    is_verified: null,
    ens_domain_name: null,
    private_tags: null,
    watchlist_names: null,
    public_tags: null,
  },
  hash: HASH,
  parent_hash: PARENT_HASH,
  gas_used: "21000",
  gas_limit: "30000000",
  nonce: "0x0000000000000000",
  burnt_fees: null,
  priority_fee: null,
  extra_data: null,
  state_root: null,
  gas_target_percentage: null,
  gas_used_percentage: null,
  burnt_fees_percentage: null,
  type: "block",
  transaction_fees: null,
  uncles_hashes: [],
};

const requests: string[] = [];
const server = createServer((req, res) => {
  requests.push(`${req.method} ${req.url}`);
  res.setHeader("Content-Type", "application/json");
  if (req.method !== "GET") {
    res.writeHead(405).end(JSON.stringify({ message: "Method not allowed" }));
  } else if (req.url === "/api/v2/blocks/404") {
    res.writeHead(404).end(JSON.stringify({ message: "Not found" }));
  } else if (req.url === "/api/v2/blocks/403") {
    res.writeHead(403).end(JSON.stringify({ message: "Forbidden" }));
  } else if (req.url === `/api/v2/blocks/${HASH}`) {
    res.end(JSON.stringify(block));
  } else if (req.url === "/api/v2/blocks/0") {
    res.end(JSON.stringify({ ...block, height: 0 } satisfies Block));
  } else if (req.url === "/api/v2/blocks/9007199254740993") {
    res.writeHead(404).end(JSON.stringify({ message: "Not found" }));
  } else if (req.url === "/api/v2/blocks/42") {
    res.end(JSON.stringify(block));
  } else {
    res.writeHead(400).end(JSON.stringify({ message: "Unexpected fixture request" }));
  }
});

let origin: string;
let detail: typeof import("../src/page-modules/block-detail");
let route: typeof import("../src/pages/block/[height]");
const savedEnv = new Map<string, string | undefined>();

before(async () => {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const fixtureEnv = {
    NEXT_PUBLIC_PERPS_GATEWAY_URL: origin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: origin,
    NEXT_PUBLIC_PAXSCAN_API_URL: origin,
    NEXT_PUBLIC_PAXSCAN_WS_URL: `${origin.replace(/^http/, "ws")}/socket`,
  };
  for (const [key, value] of Object.entries(fixtureEnv)) {
    savedEnv.set(key, process.env[key]);
    process.env[key] = value;
  }
  detail = await import("../src/page-modules/block-detail");
  route = await import("../src/pages/block/[height]");
});

after(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
  });
  for (const [key, value] of savedEnv) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

async function redirectQuery(height?: string | string[]) {
  const socket = new Socket();
  const req = Object.assign(new IncomingMessage(socket), { cookies: {} });
  const context: GetServerSidePropsContext = {
    req,
    res: new ServerResponse(req),
    params: height === undefined ? {} : { height },
    query: { block: "untrusted-override" },
    resolvedUrl: `/block/${String(height ?? "")}`,
  };
  try {
    const result = await route.getServerSideProps(context);
    assert.ok("redirect" in result);
    assert.ok("permanent" in result.redirect);
    assert.equal(result.redirect.permanent, false);
    const destination = new URL(result.redirect.destination, origin);
    assert.equal(destination.pathname, "/explorer");
    return Object.fromEntries(destination.searchParams);
  } finally {
    socket.destroy();
  }
}

function render(query: Record<string, string | string[]>, client: QueryClient) {
  return renderToStaticMarkup(
    React.createElement(
      QueryClientProvider,
      { client },
      React.createElement(detail.BlockQueryDetail, { query }),
    ),
  );
}

async function resolveRedirect(height: string | string[]) {
  const query = await redirectQuery(height);
  const identifier = detail.blockQueryIdentifier(query);
  const client = new QueryClient({
    defaultOptions: { queries: { gcTime: Infinity } },
  });
  const requestStart = requests.length;
  try {
    await client.fetchQuery(detail.blockDetailQueryOptions(identifier));
    return {
      html: render(query, client),
      requests: requests.slice(requestStart),
      identifier,
    };
  } finally {
    client.clear();
  }
}

async function blockDeepLinkNumericHeight() {
  for (const height of ["42", "0"]) {
    const result = await resolveRedirect(height);
    assert.equal(result.identifier, height);
    assert.deepEqual(result.requests, [`GET /api/v2/blocks/${height}`]);
    assert.match(result.html, new RegExp(`Block #${height}</h2>`));
    assert.ok(result.html.includes(HASH));
    assert.ok(result.html.includes(PARENT_HASH));
    assert.ok(result.html.includes(MINER));
    assert.match(result.html, /<dt>Transactions<\/dt><dd>7<\/dd>/);
    assert.match(result.html, /<dt>Gas used<\/dt><dd>21000<\/dd>/);
    assert.doesNotMatch(result.html, /Loading|not found|Unable/);
  }
  const first = await resolveRedirect(["42", "404"]);
  assert.equal(first.identifier, "42");
  assert.deepEqual(first.requests, ["GET /api/v2/blocks/42"]);
}

async function blockDeepLinkHash() {
  const result = await resolveRedirect(HASH);
  assert.equal(result.identifier, HASH);
  assert.deepEqual(result.requests, [`GET /api/v2/blocks/${HASH}`]);
  assert.match(result.html, /Block #42<\/h2>/);
  assert.ok(result.html.includes(HASH));
  assert.ok(result.html.includes(block.timestamp));
}

async function blockDeepLinkMissingBlock() {
  for (const height of ["404", "9007199254740993"]) {
    const query = await redirectQuery(height);
    const client = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity } },
    });
    const requestStart = requests.length;
    try {
      assert.equal(detail.blockQueryIdentifier(query), height);
      await assert.rejects(
        client.fetchQuery(detail.blockDetailQueryOptions(height)),
        { name: "FetchPolicyError", status: 404 },
      );
      const html = render(query, client);
      assert.deepEqual(requests.slice(requestStart), [
        `GET /api/v2/blocks/${height}`,
      ]);
      assert.ok(html.includes(`Block ${height}</h2>`));
      assert.match(html, /role="alert">Block not found\./);
      assert.doesNotMatch(html, /Loading|Unable|<dl/);
    } finally {
      client.clear();
    }
  }
}

async function blockDeepLinkNoSelection() {
  const query = await redirectQuery();
  assert.deepEqual(query, {});
  const client = new QueryClient();
  const requestStart = requests.length;
  try {
    assert.equal(detail.blockQueryIdentifier(query), null);
    assert.equal(render(query, client), "");
    assert.deepEqual(requests.slice(requestStart), []);
  } finally {
    client.clear();
  }
}

async function blockDeepLinkInvalidIdentifier() {
  const client = new QueryClient();
  const requestStart = requests.length;
  try {
    for (const query of [
      await redirectQuery("a/b?c=d&x=+ #%雪"),
      { block: "" },
      { block: [] },
      { block: "-1" },
      { block: "0x1234" },
    ]) {
      const html = render(query, client);
      // An empty repeated query has no selection; every other malformed value is refused.
      if (Array.isArray(query.block) && query.block.length === 0) {
        assert.equal(html, "");
      } else {
        assert.match(html, /role="alert">Invalid block identifier:/);
      }
    }
    assert.deepEqual(requests.slice(requestStart), []);
  } finally {
    client.clear();
  }
}

async function blockDeepLinkUnavailableIsNotMissing() {
  const query = await redirectQuery("403");
  const client = new QueryClient({
    defaultOptions: { queries: { gcTime: Infinity } },
  });
  try {
    await assert.rejects(
      client.fetchQuery(detail.blockDetailQueryOptions("403")),
      { name: "FetchPolicyError", status: 403 },
    );
    const html = render(query, client);
    assert.match(
      html,
      /role="alert">Unable to load block details\. Please try again\./,
    );
    assert.doesNotMatch(html, /Block not found|Loading|<dl/);
  } finally {
    client.clear();
  }
}

async function blockDeepLinkLoading() {
  const query = await redirectQuery("42");
  const client = new QueryClient();
  try {
    const html = render(query, client);
    assert.match(html, /role="status">Loading block 42…/);
    assert.doesNotMatch(html, /Block not found|Unable|<dl/);
  } finally {
    client.clear();
  }
}

test(blockDeepLinkNumericHeight);
test(blockDeepLinkHash);
test(blockDeepLinkMissingBlock);
test(blockDeepLinkNoSelection);
test(blockDeepLinkInvalidIdentifier);
test(blockDeepLinkUnavailableIsNotMissing);
test(blockDeepLinkLoading);
