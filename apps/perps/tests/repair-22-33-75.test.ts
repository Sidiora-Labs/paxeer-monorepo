import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import { readFileSync } from "node:fs";
import {
  createServer,
  type IncomingMessage,
  type Server,
  type ServerResponse,
} from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import {
  encodeErrorResult,
  recoverTypedDataAddress,
  type Abi,
  type Hex,
  type TypedDataDefinition,
} from "viem";
import type { PaxeerEIP1193Provider } from "../src/core/wallet/embedded/eip1193";

function fixture(name: string) {
  return JSON.parse(
    readFileSync(
      new URL(
        `../../../sdk/wallet/test/fixtures/gateway/${name}.json`,
        import.meta.url,
      ),
      "utf8",
    ),
  );
}
const me = fixture("wallet-me");
const typedResponse = fixture("wallet-sign-typed-data");
const vectors = fixture("constructions");
const sentResponse = fixture("wallet-send");
const user = {
  id: "00000000-0000-4000-8000-000000000075",
  aud: "authenticated",
  role: "authenticated",
  app_metadata: {},
  user_metadata: {},
  created_at: "1970-01-01T00:00:00Z",
};
function token() {
  const part = (v: unknown) =>
    Buffer.from(JSON.stringify(v)).toString("base64url");
  const data = `${part({ alg: "HS256", typ: "JWT" })}.${part({ sub: user.id, aud: user.aud, exp: Math.floor(Date.now() / 1000) + 3600 })}`;
  return `${data}.${createHmac("sha256", "embedded-wallet-loopback-fixture").update(data).digest("base64url")}`;
}
async function readBody(req: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(Buffer.from(chunk));
  const body = Buffer.concat(chunks).toString("utf8");
  return body ? JSON.parse(body) : undefined;
}
function json(res: ServerResponse, status: number, value: unknown) {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(value));
}
async function listen(server: Server) {
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}
async function close(server: Server) {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
}
type Exchange = { path: string; body: unknown; authorization?: string };
type RpcRequest = {
  jsonrpc: string;
  id: number;
  method: string;
  params: unknown[];
};
async function harness() {
  const exchanges: Exchange[] = [];
  let rpc: (request: RpcRequest, res: ServerResponse) => void = (
    request,
    res,
  ) => json(res, 200, { jsonrpc: "2.0", id: request.id, result: null });
  let typedStatus = 200;
  let typedBody = typedResponse.body;
  let origin: string;
  const { SignTxBody, SendTxBody } = await import(
    "../src/core/wallet/gateway"
  );
  const server = createServer((req, res) => {
    void (async () => {
      const body = await readBody(req);
      const path = req.url ?? "/";
      exchanges.push({ path, body, authorization: req.headers.authorization });
      if (path === "/auth/v1/user") return json(res, 200, user);
      if (path === "/v1/wallet/me") return json(res, me.status, me.body);
      if (path === "/rpc") return rpc(body as RpcRequest, res);
      if (path === "/v1/wallet/sign-typed-data")
        return json(res, typedStatus, typedBody);
      if (path === "/v1/wallet/send" || path === "/v1/wallet/sign") {
        const parsed = (
          path.endsWith("/send") ? SendTxBody : SignTxBody
        ).safeParse(body);
        if (!parsed.success)
          return json(res, 400, {
            error: "invalid_body",
            issues: parsed.error.issues,
          });
        if (path.endsWith("/send"))
          return json(res, sentResponse.status, sentResponse.body);
        // Only the real gateway validation boundary is needed for sign-only quantity cases.
        return json(res, 403, {
          error: "WALLET_DISABLED",
          message: "Fixture wallet signing is disabled",
        });
      }
      json(res, 404, { error: "not_found" });
    })().catch((error) => {
      json(res, 500, { error: String(error) });
    });
  });
  origin = await listen(server);
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  const { PaxeerWallet } = await import("../src/core/wallet/embedded/sdk");
  const { createPaxeerEIP1193Provider } = await import(
    "../src/core/wallet/embedded/eip1193"
  );
  const wallet = new PaxeerWallet({
    apiUrl: origin,
    supabaseUrl: origin,
    supabaseAnonKey: "loopback-publishable-key",
  });
  const accessToken = token();
  const { error } = await wallet.supabase.auth.setSession({
    access_token: accessToken,
    refresh_token: "loopback-refresh",
  });
  assert.equal(error, null);
  const providers: PaxeerEIP1193Provider[] = [];
  async function provider(timeout = 2000) {
    const value = createPaxeerEIP1193Provider({
      wallet,
      chainId: 125,
      rpcUrl: `${origin}/rpc`,
      rpcTimeoutMs: timeout,
    });
    providers.push(value);
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error("Wallet fixture did not connect")),
        2000,
      );
      const connected = () => {
        clearTimeout(timer);
        value.removeListener("connect", connected);
        resolve();
      };
      value.on("connect", connected);
    });
    assert.deepEqual(await value.request({ method: "eth_accounts" }), [
      me.body.wallet.address,
    ]);
    return value;
  }
  return {
    wallet,
    origin,
    accessToken,
    exchanges,
    provider,
    setRpc(value: typeof rpc) {
      rpc = value;
    },
    setTyped(status: number, body: typeof typedBody) {
      typedStatus = status;
      typedBody = body;
    },
    async close() {
      for (const provider of providers) provider.destroy();
      await wallet.supabase.auth.stopAutoRefresh();
      await close(server);
    },
  };
}
function rpcError(code: number, field?: string) {
  return (error: unknown) => {
    assert.ok(error instanceof Error);
    assert.equal((error as Error & { code: number }).code, code);
    if (field)
      assert.equal(
        (error as Error & { data: { field: string } }).data.field,
        field,
      );
    return true;
  };
}

async function transactionQuantitiesReachSerializerAndGatewayValidation() {
  const h = await harness();
  try {
    const p = await h.provider();
    const fields = [
      "value",
      "gas",
      "gasLimit",
      "maxFeePerGas",
      "maxPriorityFeePerGas",
      "nonce",
    ];
    for (const method of ["eth_sendTransaction", "eth_signTransaction"]) {
      for (const field of fields) {
        for (const value of [
          null,
          "",
          " ",
          "garbage",
          "0x",
          "0xzz",
          "1.5",
          "-1",
          "+1",
          "0b10",
          "1e3",
          true,
          {},
          [],
          -1,
          -1n,
          0.5,
          Number.MAX_SAFE_INTEGER + 1,
          1n << 256n,
        ]) {
          const before = h.exchanges.filter(
            (e) => e.path.endsWith("/send") || e.path.endsWith("/sign"),
          ).length;
          await assert.rejects(
            p.request({ method, params: [{ [field]: value }] }),
            rpcError(-32602, field),
          );
          assert.equal(
            h.exchanges.filter(
              (e) => e.path.endsWith("/send") || e.path.endsWith("/sign"),
            ).length,
            before,
          );
        }
      }
      for (const nonce of [
        BigInt(Number.MAX_SAFE_INTEGER) + 1n,
        "9007199254740992",
        "0x20000000000000",
      ]) {
        await assert.rejects(
          p.request({ method, params: [{ nonce }] }),
          rpcError(-32602, "nonce"),
        );
      }
      await assert.rejects(
        p.request({ method, params: [{ gas: null, gasLimit: "0x5208" }] }),
        rpcError(-32602, "gas"),
      );
      await assert.rejects(
        p.request({ method, params: [{ gas: "0x5208", gasLimit: "bad" }] }),
        rpcError(-32602, "gasLimit"),
      );
    }
    for (const value of [
      0,
      0n,
      "0",
      "0x0",
      Number.MAX_SAFE_INTEGER,
      BigInt(Number.MAX_SAFE_INTEGER) + 1n,
      (1n << 256n) - 1n,
    ]) {
      for (const field of fields.filter((field) => field !== "nonce")) {
        assert.equal(
          await p.request({
            method: "eth_sendTransaction",
            params: [{ [field]: value, nonce: "0x1fffffffffffff" }],
          }),
          sentResponse.body.tx_hash,
        );
        const exchange = h.exchanges.at(-1)!;
        assert.equal(exchange.path, "/v1/wallet/send");
        assert.equal(exchange.authorization, `Bearer ${h.accessToken}`);
        assert.deepEqual(exchange.body, {
          tx: {
            [field === "gasLimit" ? "gas" : field]: BigInt(value).toString(),
            nonce: Number.MAX_SAFE_INTEGER,
            chainId: 125,
          },
        });
      }
    }
    assert.equal(
      await p.request({ method: "eth_sendTransaction", params: [{}] }),
      sentResponse.body.tx_hash,
    );
    assert.deepEqual(h.exchanges.at(-1)!.body, { tx: { chainId: 125 } });
    assert.equal(
      await p.request({
        method: "eth_sendTransaction",
        params: [{ nonce: 0, value: "0x0" }],
      }),
      sentResponse.body.tx_hash,
    );
    assert.deepEqual(h.exchanges.at(-1)!.body, {
      tx: { value: "0", nonce: 0, chainId: 125 },
    });
    await assert.rejects(
      p.request({
        method: "eth_signTransaction",
        params: [
          { value: "0x0", gasLimit: "0x5208", nonce: Number.MAX_SAFE_INTEGER },
        ],
      }),
      (error: unknown) => {
        assert.equal((error as { code: string }).code, "WALLET_DISABLED");
        return true;
      },
    );
    assert.deepEqual(h.exchanges.at(-1)!.body, {
      tx: {
        value: "0",
        gas: "21000",
        nonce: Number.MAX_SAFE_INTEGER,
        chainId: 125,
      },
    });
    // Drive the same real gateway validator directly to prove malformed wire values cannot pass it.
    for (const tx of [
      { value: "-1" },
      { value: "0x1" },
      { nonce: Number.MAX_SAFE_INTEGER + 1 },
      { gas: "1.5" },
    ]) {
      const response = await fetch(`${h.origin}/v1/wallet/send`, {
        method: "POST",
        body: JSON.stringify({ tx }),
        headers: { "Content-Type": "application/json" },
      });
      assert.equal(response.status, 400);
      assert.equal((await response.json()).error, "invalid_body");
    }
  } finally {
    await h.close();
  }
}

async function embeddedRpcBoundsAndValidatesRealReadTransport() {
  const h = await harness();
  try {
    const p = await h.provider();
    for (const result of [null, false, 0, "0x0"]) {
      h.setRpc((req, res) =>
        json(res, 200, { jsonrpc: "2.0", id: req.id, result }),
      );
      assert.equal(await p.request({ method: "eth_blockNumber" }), result);
      const wire = h.exchanges.at(-1)!.body as RpcRequest;
      assert.equal(wire.method, "eth_blockNumber");
      assert.deepEqual(wire.params, []);
      assert.equal(wire.jsonrpc, "2.0");
    }
    const malformed = [
      (id: number) => ({ jsonrpc: "2.0", id: id + 1, result: "0x1" }),
      (id: number) => ({ jsonrpc: "2.0", id: String(id), result: "0x1" }),
      (id: number) => ({ jsonrpc: "1.0", id, result: "0x1" }),
      (id: number) => ({ jsonrpc: "2.0", id }),
      (id: number) => ({
        jsonrpc: "2.0",
        id,
        result: null,
        error: { code: -32000, message: "refused" },
      }),
      (id: number) => ({ jsonrpc: "2.0", id, error: null }),
      (id: number) => ({
        jsonrpc: "2.0",
        id,
        error: { code: "-32000", message: "refused" },
      }),
      (id: number) => ({
        jsonrpc: "2.0",
        id,
        error: { code: 1.5, message: "refused" },
      }),
      (id: number) => ({
        jsonrpc: "2.0",
        id,
        error: { code: -32000, message: {} },
      }),
      () => [],
      () => null,
    ];
    for (const envelope of malformed) {
      h.setRpc((req, res) => json(res, 200, envelope(req.id)));
      await assert.rejects(
        p.request({
          method: "eth_getBalance",
          params: [me.body.wallet.address, "latest"],
        }),
        rpcError(-32603),
      );
    }
    h.setRpc((_req, res) => {
      res.writeHead(200);
      res.end("{broken");
    });
    await assert.rejects(
      p.request({ method: "eth_blockNumber" }),
      rpcError(-32603),
    );
    for (const status of [400, 429, 503]) {
      h.setRpc((req, res) =>
        json(res, status, { jsonrpc: "2.0", id: req.id, result: "0x1" }),
      );
      await assert.rejects(
        p.request({ method: "eth_blockNumber" }),
        (error: unknown) => {
          rpcError(4900)(error);
          assert.equal((error as { status: number }).status, status);
          return true;
        },
      );
    }
    // A real stalled socket must be cancelled, including a response whose body never ends.
    for (const sendHeaders of [false, true]) {
      let socketClosed!: () => void;
      const closed = new Promise<void>((resolve) => {
        socketClosed = resolve;
      });
      h.setRpc((_req, res) => {
        res.once("close", socketClosed);
        if (sendHeaders) {
          res.writeHead(200, { "Content-Type": "application/json" });
          res.write('{"jsonrpc":"2.0",');
        }
      });
      const started = performance.now();
      await assert.rejects(
        (await h.provider(100)).request({ method: "eth_blockNumber" }),
        (error: unknown) => {
          rpcError(4900)(error);
          assert.match((error as Error).message, /timed out/);
          return true;
        },
      );
      assert.ok(performance.now() - started < 1500);
      await Promise.race([
        closed,
        new Promise<never>((_, reject) => {
          const timer = setTimeout(
            () => reject(new Error("RPC socket was not cancelled")),
            1000,
          );
          timer.unref();
        }),
      ]);
    }
    let entered!: () => void;
    const active = new Promise<void>((resolve) => {
      entered = resolve;
    });
    h.setRpc((_req, _res) => {
      entered();
    });
    const doomed = await h.provider();
    const pending = doomed.request({ method: "eth_blockNumber" });
    const rejected = assert.rejects(pending, rpcError(4900));
    await active;
    doomed.destroy();
    await rejected;
    h.setRpc((req, res) =>
      json(res, 200, { jsonrpc: "2.0", id: req.id, result: "0x75" }),
    );
    assert.equal(
      await (await h.provider()).request({ method: "eth_blockNumber" }),
      "0x75",
    );
  } finally {
    await h.close();
  }
}

async function encodedNodeRefusalsReachClassifierAndUserFacingConsumer() {
  const h = await harness();
  try {
    const { classifyError } = await import("../src/core/errors/classify");
    const { ERROR_CATALOG } = await import("../src/core/errors/catalog");
    const { alertErrorText } = await import(
      "../src/widgets/alerts/AlertsPanel"
    );
    // Encode with the node's served ABI, independently of the app's decoder ABI.
    const perps = JSON.parse(
      readFileSync(
        new URL("../../../precompiles/perps/abi/abi.json", import.meta.url),
        "utf8",
      ),
    ) as Abi;
    const spot = JSON.parse(
      readFileSync(
        new URL("../../../precompiles/spot/abi/pecor.json", import.meta.url),
        "utf8",
      ),
    ) as Abi;
    const cases = [
      { abi: perps, name: "PerpsTickSize", code: "VALIDATION_PRECISION" },
      {
        abi: perps,
        name: "PerpsInsufficientMargin",
        code: "TRADE_POSITION_LIQUIDATABLE",
      },
      { abi: perps, name: "PerpsMarketDisabled", code: "TRADE_MARKET_HALTED" },
      { abi: perps, name: "PerpsPaused", code: "RPC_REVERT" },
      {
        abi: spot,
        name: "InsufficientOutput",
        code: "TRADE_SLIPPAGE_EXCEEDED",
      },
      { abi: spot, name: "TokenPaused", code: "TRADE_MARKET_HALTED" },
    ] as const;
    const p = await h.provider();
    for (const c of cases) {
      const data = encodeErrorResult({ abi: c.abi, errorName: c.name });
      for (const method of ["eth_call", "eth_estimateGas"]) {
        for (const errorData of [data, { data }]) {
          h.setRpc((req, res) =>
            json(res, 200, {
              jsonrpc: "2.0",
              id: req.id,
              error: {
                code: 3,
                message: "execution reverted",
                data: errorData,
              },
            }),
          );
          await assert.rejects(
            p.request({
              method,
              params: [
                {
                  to:
                    c.abi === perps
                      ? "0x0000000000000000000000000000000000001015"
                      : "0x0000000000000000000000000000000000001021",
                  data: "0x",
                },
                "latest",
              ],
            }),
            (error: unknown) => {
              rpcError(3)(error);
              assert.deepEqual((error as { data: unknown }).data, errorData);
              const classified = classifyError(error, "trade/simulate");
              const expected = ERROR_CATALOG[c.code];
              assert.equal(classified.code, c.code);
              assert.equal(classified.isRetryable, expected.isRetryable);
              assert.equal(classified.suggestedFix, expected.suggestedFix);
              assert.ok(classified.fingerprint?.includes(c.name));
              const expectedMessage =
                c.name === "PerpsPaused"
                  ? "The contract rejected this transaction with PerpsPaused."
                  : expected.message;
              assert.equal(
                alertErrorText(classified),
                expected.suggestedFix
                  ? `${expectedMessage} ${expected.suggestedFix}`
                  : expectedMessage,
              );
              return true;
            },
          );
        }
      }
    }
  } finally {
    await h.close();
  }
}

async function typedDataUsesValidatedCanonicalProviderAndDeploymentRefusals() {
  const h = await harness();
  try {
    const p = await h.provider();
    const address = me.body.wallet.address as Hex;
    const typedData = vectors.typedData as TypedDataDefinition;
    const params = [address, JSON.stringify(typedData)];
    const signature = await p.request({
      method: "eth_signTypedData_v4",
      params,
    });
    assert.equal(signature, typedResponse.body.signature);
    assert.equal(
      (
        await recoverTypedDataAddress({
          ...typedData,
          signature: signature as Hex,
        })
      ).toLowerCase(),
      address.toLowerCase(),
    );
    const posted = h.exchanges.filter(
      (e) => e.path === "/v1/wallet/sign-typed-data",
    );
    assert.equal(posted.length, 1);
    assert.equal(posted[0].authorization, `Bearer ${h.accessToken}`);
    assert.deepEqual(posted[0].body, { typedData });
    for (const [badParams, code] of [
      [["0x4444444444444444444444444444444444444444", params[1]], 4100],
      [[address, "{broken"], -32602],
      [[address, {}], -32602],
      [
        [
          address,
          { ...typedData, domain: { ...typedData.domain, chainId: 1 } },
        ],
        4901,
      ],
    ] as const) {
      const count = h.exchanges.filter(
        (e) => e.path === "/v1/wallet/sign-typed-data",
      ).length;
      await assert.rejects(
        p.request({ method: "eth_signTypedData_v4", params: [...badParams] }),
        rpcError(code),
      );
      assert.equal(
        h.exchanges.filter((e) => e.path === "/v1/wallet/sign-typed-data")
          .length,
        count,
      );
    }
    h.setTyped(200, {
      ...typedResponse.body,
      address: "0x4444444444444444444444444444444444444444",
    });
    await assert.rejects(
      p.request({ method: "eth_signTypedData_v4", params }),
      rpcError(4100),
    );
    h.setTyped(200, typedResponse.body);
    await assert.rejects(
      p.request({
        method: "eth_signTypedData_v4",
        params: [
          address,
          { ...typedData, message: { ...typedData.message, nonce: "10" } },
        ],
      }),
      rpcError(4100),
    );
    for (const status of [404, 405, 501]) {
      h.setTyped(status, { error: "not_found" });
      await assert.rejects(
        p.request({ method: "eth_signTypedData_v4", params }),
        (error: unknown) => {
          rpcError(4200)(error);
          assert.match(
            (error as Error).message,
            /this wallet gateway deployment/,
          );
          assert.match((error as Error).message, /canonical EIP-712 v4/);
          return true;
        },
      );
    }
    h.setTyped(403, {
      error: "WALLET_DISABLED",
      message: "Signing refused by policy",
    });
    await assert.rejects(
      p.request({ method: "eth_signTypedData_v4", params }),
      (error: unknown) => {
        rpcError(4100)(error);
        assert.equal((error as { reason: string }).reason, "WALLET_DISABLED");
        return true;
      },
    );
    for (const method of [
      "eth_signTypedData",
      "eth_signTypedData_v1",
      "eth_signTypedData_v3",
    ]) {
      const before = h.exchanges.length;
      await assert.rejects(p.request({ method, params }), (error: unknown) => {
        rpcError(4200)(error);
        assert.match((error as Error).message, /Use eth_signTypedData_v4/);
        return true;
      });
      assert.equal(h.exchanges.length, before);
    }
    h.setTyped(200, typedResponse.body);
    assert.equal(
      await p.request({
        method: "eth_signTypedData_v4",
        params: [address, typedData],
      }),
      signature,
    );
  } finally {
    await h.close();
  }
}

test(transactionQuantitiesReachSerializerAndGatewayValidation);
test(embeddedRpcBoundsAndValidatesRealReadTransport);
test(encodedNodeRefusalsReachClassifierAndUserFacingConsumer);
test(typedDataUsesValidatedCanonicalProviderAndDeploymentRefusals);
