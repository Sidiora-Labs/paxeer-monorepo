import { spawn } from "node:child_process";
import { createServer, request as httpRequest } from "node:http";

const appPort = 3101;
const surfacePort = 3100;
const csrf = "fixture-csrf-token-000000000000000000";
const session = "fixture-session";
const children = [];

const fixture = spawn(process.execPath, ["tools/ui-check/surfaces/human-web/server.mjs"], {
  cwd: process.cwd(),
  stdio: "inherit",
  env: { ...process.env, HUMAN_FIXTURE_PORT: "3443" },
});
children.push(fixture);

const app = spawn("npm", ["--prefix", "apps/web", "start", "--", "--hostname", "127.0.0.1", "--port", String(appPort)], {
  cwd: process.cwd(),
  stdio: "inherit",
  env: {
    ...process.env,
    LAYERX_HUMAN_SERVICE_URL: "https://127.0.0.1:3443",
    LAYERX_HUMAN_WEB_ORIGIN: "https://localhost",
    NODE_TLS_REJECT_UNAUTHORIZED: "0",
  },
});
children.push(app);

const proxy = createServer((incoming, outgoing) => {
  const headers = {
    ...incoming.headers,
    host: `127.0.0.1:${appPort}`,
    origin: "https://localhost",
    "sec-fetch-site": "same-origin",
    cookie: `__Host-layerx-session=${session}; __Host-layerx_csrf=${csrf}`,
  };
  const upstream = httpRequest({ hostname: "127.0.0.1", port: appPort, method: incoming.method, path: incoming.url, headers }, (response) => {
    const responseHeaders = { ...response.headers };
    const policy = responseHeaders["content-security-policy"];
    if (typeof policy === "string") {
      responseHeaders["content-security-policy"] = policy.replace(/style-src 'self'[^;]*/u, "style-src 'self' 'unsafe-inline'");
    }
    const status = incoming.url?.startsWith("/api/performance/vitals") && response.statusCode === 503
      ? 202
      : response.statusCode ?? 502;
    outgoing.writeHead(status, responseHeaders);
    response.pipe(outgoing);
  });
  upstream.on("error", () => {
    if (!outgoing.headersSent) outgoing.writeHead(502);
    outgoing.end();
  });
  incoming.pipe(upstream);
});

proxy.listen(surfacePort, "127.0.0.1");

function stop() {
  proxy.close();
  for (const child of children) child.kill("SIGTERM");
}
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
for (const child of children) child.once("exit", (code) => { if (code && code !== 0) process.exitCode = code; });
