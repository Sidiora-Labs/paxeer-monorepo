import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer, type Server } from "node:http";
import { after, before, test } from "node:test";
import { Worker } from "node:worker_threads";

const DECLARED_RPCS = [
  ...Array.from(
    { length: 14 },
    (_, i) => `https://api${i + 1}.mainnet-beta.paxeer.network`,
  ),
  "https://api-mainnet-beta.paxeer.network",
];
const ENV_KEYS = [
  "NEXT_PUBLIC_PERPS_GATEWAY_URL",
  "NEXT_PUBLIC_PAXSCAN_BASE_URL",
  "NEXT_PUBLIC_PAXSCAN_API_URL",
  "NEXT_PUBLIC_PAXSCAN_WS_URL",
  "NEXT_PUBLIC_PAXEER_RPC_URLS",
  "NEXT_PUBLIC_PAXEER_RPC_URL",
] as const;
const savedEnv = ENV_KEYS.map((key) => [key, process.env[key]] as const);
const servers: Server[] = [];
const requests: { peer: string; path: string; method: string }[] = [];
let primaryOrigin: string;
let secondaryOrigin: string;

async function startRpcPeer(
  peer: string,
  unavailable: boolean,
): Promise<string> {
  const server = createServer(async (req, res) => {
    let body = "";
    for await (const chunk of req) body += chunk;
    const rpc = JSON.parse(body) as { id: number; method: string };
    requests.push({ peer, path: req.url ?? "", method: rpc.method });
    res.setHeader("content-type", "application/json");
    if (unavailable) {
      res.writeHead(503).end(JSON.stringify({ error: "node unavailable" }));
      return;
    }
    const results: Record<string, boolean | string> = {
      net_listening: true,
      eth_chainId: "0x7d",
      eth_blockNumber: "0x2a",
    };
    if (!(rpc.method in results)) {
      res.end(
        JSON.stringify({
          jsonrpc: "2.0",
          id: rpc.id,
          error: { code: -32601, message: "Method not found" },
        }),
      );
      return;
    }
    res.end(
      JSON.stringify({
        jsonrpc: "2.0",
        id: rpc.id,
        result: results[rpc.method],
      }),
    );
  });
  servers.push(server);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  return `http://127.0.0.1:${address.port}`;
}

before(async () => {
  primaryOrigin = await startRpcPeer("primary", true);
  secondaryOrigin = await startRpcPeer("secondary", false);
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = secondaryOrigin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = secondaryOrigin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${secondaryOrigin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = secondaryOrigin.replace(
    "http:",
    "ws:",
  );
  delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
  delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
});

after(async () => {
  for (const [key, value] of savedEnv) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  await Promise.all(
    servers.map(
      (server) =>
        new Promise<void>((resolve, reject) => {
          server.close((error) => (error ? reject(error) : resolve()));
          server.closeAllConnections();
        }),
    ),
  );
});

async function defaultRpcNodeSetFeedsChainConfiguration() {
  const { getPaxeerRpcUrls } = await import("../src/core/rpc");
  const { paxeerChain } = await import("../src/core/chain");
  assert.deepEqual(getPaxeerRpcUrls(), DECLARED_RPCS);
  assert.equal(paxeerChain.id, 125);
  assert.deepEqual(paxeerChain.rpcUrls.default.http, DECLARED_RPCS);
  for (const endpoint of getPaxeerRpcUrls()) {
    const url = new URL(endpoint);
    assert.equal(url.protocol, "https:");
    assert.equal(url.pathname, "/");
    assert.equal(url.search, "");
    assert.equal(url.hash, "");
  }
  const copy = getPaxeerRpcUrls();
  copy.pop();
  assert.deepEqual(getPaxeerRpcUrls(), DECLARED_RPCS);
  assert.deepEqual(requests, []);
}

async function explicitRpcOverridesPreserveDeclaredDefaults() {
  const { getPaxeerRpcUrls } = await import("../src/core/rpc");
  try {
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = ` ${primaryOrigin},${secondaryOrigin},${primaryOrigin}, `;
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL =
      "https://api-mainnet-beta.paxeer.network";
    assert.deepEqual(getPaxeerRpcUrls(), [primaryOrigin, secondaryOrigin]);
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = " , ";
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = primaryOrigin;
    assert.deepEqual(getPaxeerRpcUrls(), [primaryOrigin, ...DECLARED_RPCS]);
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = DECLARED_RPCS[0];
    assert.deepEqual(getPaxeerRpcUrls(), DECLARED_RPCS);
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS =
      "https://rpcpaxeer.online,https://api.hyperpax.xyz,https://stats.paxscan.io";
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
    assert.deepEqual(getPaxeerRpcUrls(), DECLARED_RPCS);
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = "https://rpcpaxeer.online";
    assert.deepEqual(getPaxeerRpcUrls(), DECLARED_RPCS);
  } finally {
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
  }
}

async function sharedPublicClientsUseRealRpcTransport() {
  // Ranking runs for the client's lifetime. A worker owns the real clients
  // so terminating it also stops their rank timers after the HTTP round trip.
  const worker = new Worker(
    `
    const { parentPort, workerData } = require("node:worker_threads");
    require(workerData.tsx);
    (async () => {
      const { getPublicClient } = require(workerData.clients);
      const { paxeerChain } = require(workerData.chain);
      const { wagmiConfig } = require(workerData.wagmi);
      const client = getPublicClient();
      const walletReadClient = wagmiConfig.getClient({ chainId: 125 });
      const height = await client.getBlockNumber({ cacheTime: 0 });
      const walletHeight = await walletReadClient.request({ method: "eth_blockNumber" });
      parentPort.postMessage({
        height, walletHeight,
        singleton: client === getPublicClient(),
        chainId: client.chain.id,
        chainUrls: paxeerChain.rpcUrls.default.http,
        urls: client.transport.transports.map((transport) => transport.value.url),
        walletUrls: walletReadClient.transport.transports.map((transport) => transport.value.url),
      });
    })().catch((error) => { throw error; });
  `,
    {
      eval: true,
      env: {
        ...process.env,
        NEXT_PUBLIC_PAXEER_RPC_URLS: `${primaryOrigin},${secondaryOrigin}`,
      },
      workerData: {
        tsx: require.resolve("tsx/cjs"),
        clients: require.resolve("../src/core/viem/clients"),
        chain: require.resolve("../src/core/chain"),
        wagmi: require.resolve("../src/core/wagmi"),
      },
    },
  );
  try {
    const [result] = await once(worker, "message", {
      signal: AbortSignal.timeout(15_000),
    });
    assert.deepEqual(result, {
      height: 42n,
      walletHeight: "0x2a",
      singleton: true,
      chainId: 125,
      chainUrls: [primaryOrigin, secondaryOrigin],
      urls: [primaryOrigin, secondaryOrigin],
      walletUrls: [primaryOrigin, secondaryOrigin],
    });
    assert.ok(requests.some((request) => request.peer === "primary"));
    assert.equal(
      requests.filter(
        (request) =>
          request.peer === "secondary" && request.method === "eth_blockNumber",
      ).length,
      2,
    );
    assert.ok(requests.every((request) => request.path === "/"));
  } finally {
    await worker.terminate();
  }
}

test(defaultRpcNodeSetFeedsChainConfiguration);
test(explicitRpcOverridesPreserveDeclaredDefaults);
test(sharedPublicClientsUseRealRpcTransport);
