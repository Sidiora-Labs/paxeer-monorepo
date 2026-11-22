import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { createServer, request as httpRequest } from "node:http";
import { get as httpsGet } from "node:https";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";

const appPort = 3101;
const surfacePort = 3100;
const service = "https://127.0.0.1:3443";
const csrf = "fixture-csrf-token-000000000000000000";
const session = "fixture-session";
const rumDirectory = mkdtempSync(join(tmpdir(), "layerx-human-rum-"));
const children = [];
const require = createRequire(
  new URL("../../../../apps/web/package.json", import.meta.url),
);
const fixture = spawn(
  process.execPath,
  ["tools/ui-check/surfaces/human-web/server.mjs"],
  {
    cwd: process.cwd(),
    stdio: "inherit",
    env: { ...process.env, HUMAN_FIXTURE_PORT: "3443" },
  },
);
children.push(fixture);

await new Promise((resolve, reject) => {
  const deadline = Date.now() + 10_000;
  function ready() {
    const request = httpsGet(
      `${service}/readyz`,
      { rejectUnauthorized: false },
      (response) => {
        response.resume();
        if (response.statusCode === 200) resolve();
        else
          reject(
            new Error(`Fixture readiness answered ${response.statusCode}`),
          );
      },
    );
    request.on("error", (error) => {
      if (Date.now() >= deadline) reject(error);
      else setTimeout(ready, 100);
    });
  }
  ready();
});

const app = spawn(
  process.execPath,
  [
    require.resolve("next/dist/bin/next"),
    "start",
    "apps/web",
    "--hostname",
    "127.0.0.1",
    "--port",
    String(appPort),
  ],
  {
    cwd: process.cwd(),
    stdio: "inherit",
    env: {
      ...process.env,
      LAYERX_HUMAN_SERVICE_URL: service,
      LAYERX_HUMAN_WEB_ORIGIN: "https://localhost",
      LAYERX_GATEWAY_URL: service,
      LAYERX_NODE_ENDPOINT: service,
      PAXEER_X_EXPLORER_BASE_URL: service,
      LAYERX_RUM_STORAGE_DIRECTORY: rumDirectory,
      NODE_TLS_REJECT_UNAUTHORIZED: "0",
    },
  },
);
children.push(app);

const proxy = createServer((incoming, outgoing) => {
  const url = new URL(incoming.url ?? "/", "http://127.0.0.1");
  const signedOut = url.pathname === "/" && !url.searchParams.has("signed-in");
  const headers = {
    ...incoming.headers,
    host: `127.0.0.1:${appPort}`,
    "x-forwarded-host": "localhost",
    "x-forwarded-proto": "https",
    origin: "https://localhost",
    "sec-fetch-site": "same-origin",
    cookie: signedOut
      ? ""
      : `${incoming.headers.cookie ?? ""}; __Host-layerx-session=${session}; __Host-layerx_csrf=${csrf}`,
  };
  const upstream = httpRequest(
    {
      hostname: "127.0.0.1",
      port: appPort,
      method: incoming.method,
      path: incoming.url?.replace(/^\/human-ui\/_next\//u, "/_next/"),
      headers,
    },
    (response) => {
      const headers = { ...response.headers };
      delete headers["content-security-policy"];
      delete headers["content-security-policy-report-only"];
      outgoing.writeHead(response.statusCode ?? 502, headers);
      response.pipe(outgoing);
    },
  );
  upstream.on("error", () => {
    if (!outgoing.headersSent) outgoing.writeHead(502);
    outgoing.end();
  });
  incoming.pipe(upstream);
});
proxy.listen(surfacePort, "127.0.0.1");

let stopping = false;
function stop() {
  if (stopping) return;
  stopping = true;
  proxy.close();
  proxy.closeAllConnections();
  for (const child of children) child.kill("SIGTERM");
  rmSync(rumDirectory, { recursive: true, force: true });
}
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
for (const child of children) {
  child.once("error", (error) => {
    console.error(error);
    process.exitCode = 1;
    stop();
  });
  child.once("exit", (code) => {
    if (stopping) return;
    process.exitCode = code || 1;
    stop();
  });
}
