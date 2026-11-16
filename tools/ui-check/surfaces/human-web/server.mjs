import { execFileSync } from "node:child_process";
import { createReadStream, existsSync, readFileSync } from "node:fs";
import { createServer } from "node:https";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL(".", import.meta.url));
const fixture = (name) => readFileSync(join(root, "fixtures", name));
const port = Number(process.env.HUMAN_FIXTURE_PORT ?? 3443);
const certRoot = join("/tmp", `layerx-human-ui-${process.pid}`);
const keyFile = `${certRoot}.key`;
const certFile = `${certRoot}.crt`;

if (!existsSync(keyFile) || !existsSync(certFile)) {
  execFileSync("openssl", [
    "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
    "-subj", "/CN=localhost", "-addext", "subjectAltName=IP:127.0.0.1,DNS:localhost",
    "-keyout", keyFile, "-out", certFile,
  ], { stdio: "ignore" });
}

function envelope(name) {
  return fixture(name);
}

function stateFromReferer(request) {
  try {
    const referer = new URL(request.headers.referer ?? "");
    return referer.searchParams.get("state") ?? "filled";
  } catch {
    return "filled";
  }
}

function respond(response, body, status = 200, headers = { "content-type": "application/json; charset=utf-8" }) {
  response.writeHead(status, { ...headers, "cache-control": "no-store" });
  response.end(body);
}

const server = createServer({ key: readFileSync(keyFile), cert: readFileSync(certFile) }, (request, response) => {
  const pathname = new URL(request.url ?? "/", "https://127.0.0.1").pathname;
  if (pathname === "/readyz") {
    respond(response, "ready\n", 200, { "content-type": "text/plain; charset=utf-8" });
    return;
  }
  const state = stateFromReferer(request);
  if (pathname === "/v1/account/balance") return respond(response, envelope("account-balance.json"));
  if (pathname === "/v1/sessions") {
    if (state === "empty") return respond(response, envelope("sessions-empty.json"));
    if (state === "error") return respond(response, envelope("sessions-error.json"));
    return respond(response, envelope("sessions-filled.json"));
  }
  if (pathname === "/v1/notifications") return respond(response, envelope("notifications.json"));
  if (pathname === "/v1/approvals") return respond(response, envelope("approvals.json"));
  if (pathname.startsWith("/v1/program-approvals/page/")) return respond(response, envelope("program-approvals.json"));
  if (pathname === "/v1/stream" && request.method === "POST") return respond(response, envelope("stream-position.json"));
  if (pathname.startsWith("/v1/stream/") && request.method === "GET") {
    response.writeHead(200, {
      "cache-control": "no-store",
      "content-type": "text/event-stream; charset=utf-8",
      "x-layerx-stream-profile": "2",
    });
    request.once("close", () => response.end());
    return;
  }
  respond(response, JSON.stringify({
    ok: false,
    trace: "trc_00000000000000000000000000000009",
    error: { code: "not-found", copy_key: "state.error.body", retry: "final" },
  }));
});

server.listen(port, "127.0.0.1");
process.on("SIGTERM", () => server.close(() => process.exit(0)));
