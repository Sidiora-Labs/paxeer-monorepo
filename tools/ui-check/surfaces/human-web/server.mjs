import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, unlinkSync } from "node:fs";
import { createServer } from "node:https";
import { createRequire } from "node:module";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(
  new URL("../../../../apps/web/package.json", import.meta.url),
);
const { decodeFunctionData, encodeFunctionResult } = require("viem");
const root = fileURLToPath(new URL(".", import.meta.url));
const fixture = (name) =>
  JSON.parse(readFileSync(join(root, "fixtures", `${name}.json`), "utf8"));
const port = Number(process.env.HUMAN_FIXTURE_PORT ?? 3443);
const certRoot = join("/tmp", `layerx-human-ui-${process.pid}`);
const keyFile = `${certRoot}.key`;
const certFile = `${certRoot}.crt`;
if (!existsSync(keyFile) || !existsSync(certFile)) {
  execFileSync(
    "openssl",
    [
      "req",
      "-x509",
      "-newkey",
      "rsa:2048",
      "-nodes",
      "-days",
      "1",
      "-subj",
      "/CN=localhost",
      "-addext",
      "subjectAltName=IP:127.0.0.1,DNS:localhost",
      "-keyout",
      keyFile,
      "-out",
      certFile,
    ],
    { stdio: "ignore" },
  );
}

const routes = new Map([
  ["/v1/home", "home.summary"],
  ["/v1/agents", "agent.list"],
  ["/v1/approvals", "approval.list"],
  ["/v1/activity/query", "activity.query"],
  ["/v1/notifications", "notification.list"],
  ["/v1/notifications/preferences", "notification.preferences.get"],
  ["/v1/profile", "profile.get"],
  ["/v1/security/passkeys", "security.passkey.list"],
  ["/v1/security/authenticators", "authenticator.status"],
  ["/v1/onboarding", "onboarding.resume"],
  ["/v1/onboarding/resume", "onboarding.resume"],
  ["/v1/sessions/fee-policy", "session.fee-policy"],
  ["/v1/support/conversations", "support.list"],
]);
const primary = new Map([
  ["/app", "/v1/home"],
  ["/app/activity", "/v1/activity/query"],
  ["/app/agents", "/v1/agents"],
  ["/app/approvals", "/v1/approvals"],
  ["/app/notifications", "/v1/notifications"],
  ["/app/settings", "/v1/profile"],
  ["/app/settings/security", "/v1/security/passkeys"],
  ["/app/security/recovery", "/v1/security/passkeys"],
  ["/app/support", "/v1/support/conversations"],
  ["/app/settings/devices", "/v1/sessions"],
  ["/app/security/devices", "/v1/sessions"],
]);
const rpcReads = fixture("rpc-reads");
const abis = Object.fromEntries(
  [
    ["0x0000000000000000000000000000000000001015", "perps"],
    ["0x0000000000000000000000000000000000001016", "bridge"],
    ["0x0000000000000000000000000000000000001017", "launchpad"],
    ["0x00000000000000000000000000000000000000a1", "pool"],
    ["0x00000000000000000000000000000000000000b2", "token"],
    ["0x00000000000000000000000000000000000000c3", "nft"],
    ["0x00000000000000000000000000000000000000d4", "token"],
  ].map(([address, name]) => [address, fixture(`${name}-abi`)]),
);

function respond(response, value, status = 200, headers = {}) {
  response.writeHead(status, {
    "content-type": "application/json; charset=utf-8",
    "cache-control": "no-store",
    ...headers,
  });
  response.end(typeof value === "string" ? value : JSON.stringify(value));
}

function fromEnvelope(response, name, state = "filled") {
  const envelope = fixture(name);
  const result = envelope.body.result;
  const now = Date.now();
  if (name === "notification.list") {
    for (const group of result.groups) {
      const elapsed = group.recency === "today" ? 60_000 : 86_400_000;
      for (const notification of group.notifications) {
        notification.created_at = new Date(now - elapsed).toISOString();
      }
    }
  }
  if (name === "approval.get") {
    result.facts.expires_at = new Date(now + 3_600_000).toISOString();
  }
  if (name === "approval.list") {
    for (const approval of result.approvals) {
      approval.expires_at = new Date(now + 3_600_000).toISOString();
    }
  }
  if (state === "error")
    return respond(response, fixture("sessions-error"), 503);
  if (state === "empty") {
    if (name === "profile.get") result.display_name = "";
    for (const key of [
      "agents",
      "approvals",
      "recent_activity",
      "groups",
      "passkeys",
      "conversations",
    ]) {
      if (Array.isArray(result[key])) result[key] = [];
    }
    if ("unread_count" in result) result.unread_count = 0;
  }
  const send = () => respond(response, envelope.body, envelope.status);
  if (state === "loading") {
    const timer = setTimeout(send, 8_000);
    response.once("close", () => clearTimeout(timer));
  } else send();
}

function rpcResult(call) {
  if (call.method === "px_getCapabilities")
    return {
      exchange: true,
      bridge: true,
      launchpad: true,
      probed_at: 0,
      rpc_height: "4660",
    };
  if (call.method === "eth_blockNumber") return "0x1234";
  if (call.method === "eth_getLogs") return [];
  if (call.method !== "eth_call")
    throw new Error(`Undeclared RPC method: ${call.method}`);
  const address = call.params[0].to.toLowerCase();
  const abi = abis[address];
  if (!abi) throw new Error(`Undeclared contract: ${address}`);
  const { functionName, args } = decodeFunctionData({
    abi,
    data: call.params[0].data,
  });
  let result = rpcReads[address]?.[functionName];
  if (result && typeof result === "object" && "byArgument" in result)
    result = result.byArgument[String(args[0])];
  if (result === undefined)
    throw new Error(`Undeclared contract read: ${address}.${functionName}`);
  return encodeFunctionResult({ abi, functionName, result });
}

const server = createServer(
  { key: readFileSync(keyFile), cert: readFileSync(certFile) },
  async (request, response) => {
    try {
      const url = new URL(request.url ?? "/", "https://127.0.0.1");
      const path = url.pathname;
      if (path === "/readyz")
        return respond(response, "ready\n", 200, {
          "content-type": "text/plain; charset=utf-8",
        });
      if (path === "/" && request.method === "POST") {
        let body = "";
        for await (const chunk of request) body += chunk;
        const call = JSON.parse(body);
        try {
          return respond(response, {
            jsonrpc: "2.0",
            id: call.id,
            result: rpcResult(call),
          });
        } catch (error) {
          return respond(response, {
            jsonrpc: "2.0",
            id: call.id,
            error: { code: -32601, message: error.message },
          });
        }
      }
      const account = fixture("explorer-account");
      if (path.startsWith("/api/v2/addresses/") && path.endsWith("/unified")) {
        const identifier = path.split("/")[4];
        if (identifier === account.missingAddress)
          return respond(response, { message: "Not found" }, 404);
        if (identifier === account.unavailableAddress)
          return respond(response, { message: "Unavailable" }, 503);
        if (identifier === account.emptyAddress)
          return respond(response, account.emptyPage);
        if (identifier === account.address)
          return respond(response, account.firstPage);
      }
      const status = fixture("explorer-status");
      if (path === `/api/v2/transactions/${status.transactionHash}`)
        return respond(response, {
          hash: status.transactionHash,
          block_number: 100,
          position: 0,
          status: "ok",
        });
      if (path === `/api/v2/transactions/${status.missingHash}`)
        return respond(response, status.notFound, 404);
      let referer;
      try {
        referer = new URL(request.headers.referer ?? "");
      } catch {
        referer = undefined;
      }
      const state =
        primary.get(referer?.pathname) === path
          ? (referer.searchParams.get("state") ?? "filled")
          : "filled";
      if (path === "/v1/account/balance")
        return respond(response, fixture("account-balance"));
      if (path === "/v1/sessions")
        return respond(
          response,
          fixture(
            `sessions-${state === "empty" ? "empty" : state === "error" ? "error" : "filled"}`,
          ),
          state === "error" ? 503 : 200,
        );
      if (routes.has(path))
        return fromEnvelope(response, routes.get(path), state);
      if (path.startsWith("/v1/evidence/"))
        return fromEnvelope(response, "evidence.get");
      if (/^\/v1\/agents\/[^/]+$/u.test(path))
        return fromEnvelope(response, "agent.get");
      if (/^\/v1\/approvals\/[^/]+$/u.test(path))
        return fromEnvelope(response, "approval.get");
      if (/^\/v1\/activity\/[^/]+$/u.test(path))
        return fromEnvelope(response, "activity.entry");
      if (/^\/v1\/support\/conversations\/[^/]+\/status$/u.test(path))
        return fromEnvelope(response, "support.status");
      if (/^\/v1\/support\/conversations\/[^/]+\/read$/u.test(path))
        return fromEnvelope(response, "support.read");
      if (path.startsWith("/v1/program-approvals/page/"))
        return respond(response, fixture("program-approvals"));
      if (path === "/v1/stream" && request.method === "POST")
        return respond(response, fixture("stream-position"));
      if (path.startsWith("/v1/stream/") && request.method === "GET") {
        const timer = setTimeout(
          () =>
            respond(response, "", 200, {
              "content-type": "text/event-stream; charset=utf-8",
              "x-layerx-stream-profile": "2",
            }),
          1_000,
        );
        response.once("close", () => clearTimeout(timer));
        return;
      }
      respond(
        response,
        {
          ok: false,
          trace: "trc_00000000000000000000000000000009",
          error: {
            code: "not-found",
            copy_key: "state.error.body",
            retry: "final",
          },
        },
        404,
      );
    } catch (error) {
      console.error(error);
      if (!response.headersSent)
        respond(response, { message: "Invalid fixture request" }, 400);
      else response.end();
    }
  },
);
server.listen(port, "127.0.0.1");
process.on("SIGTERM", () => {
  server.close(() => process.exit(0));
  server.closeAllConnections();
  for (const path of [keyFile, certFile]) unlinkSync(path);
});
