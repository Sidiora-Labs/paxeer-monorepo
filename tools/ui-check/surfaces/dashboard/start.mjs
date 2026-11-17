import { spawn } from "node:child_process";
import { createServer, request as forward } from "node:http";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const app = fileURLToPath(
  new URL(
    "../../../../services/platform/hosted/dashboard/web/",
    import.meta.url,
  ),
);
const environment = await readFile(
  new URL("browser-environment.js", import.meta.url),
  "utf8",
);
const processes = [true, false].map((configured, index) => {
  const child = spawn(
    process.execPath,
    [
      `${app}node_modules/next/dist/bin/next`,
      "start",
      "--hostname",
      "127.0.0.1",
      "--port",
      String(4176 + index),
    ],
    {
      cwd: app,
      env: {
        ...process.env,
        DASHBOARD_API_ORIGIN: "http://127.0.0.1:9445",
        DASHBOARD_GATEWAY_TENANT: configured ? "beta" : "",
        DASHBOARD_GATEWAY_ORIGIN: configured ? "http://localhost:4175" : "",
      },
      stdio: ["ignore", "inherit", "inherit"],
    },
  );
  child.on("error", (error) => {
    console.error(error);
    void close(1);
  });
  child.on("exit", (code) => {
    if (!closing) void close(code || 1);
  });
  return child;
});

function answer(response, name) {
  response.writeHead(307, {
    location: `https://dashboard.example/answers/${name}`,
    "cache-control": "no-store",
  });
  response.end();
}

const surface = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, "http://localhost:4175");
    const document = request.headers.referer
      ? new URL(request.headers.referer)
      : url;
    const scenario =
      document.searchParams.get("fixture") ??
      /(?:^|; )dashboard-fixture=([a-z-]+)/.exec(
        request.headers.cookie ?? "",
      )?.[1] ??
      "filled";
    const cookies = request.headers.cookie ?? "";
    const authenticated = cookies.includes(
      "__Host-layerx-session=session_fixture_opaque",
    );
    if (url.pathname === "/v1/keys" && request.method === "POST") {
      return answer(response, "key-issued.json");
    }
    if (url.pathname === "/keys" && request.method === "POST") {
      return answer(response, "key-issued.json");
    }
    if (url.pathname.startsWith("/v1/dashboard/")) {
      if (scenario === "loading") return;
      if (scenario.startsWith("sign-in") && !authenticated) {
        return answer(response, "session-required.json");
      }
      if (scenario === "unavailable")
        return answer(response, "unavailable.json");
      if (url.pathname.startsWith("/v1/dashboard/receipts/")) {
        if (scenario === "receipt-loading") return;
        return answer(
          response,
          scenario === "receipt-unavailable"
            ? "unavailable.json"
            : "receipt.json",
        );
      }
      if (url.pathname === "/v1/dashboard/overview") {
        return answer(
          response,
          scenario === "empty"
            ? "empty.json"
            : scenario === "payments-long"
              ? "payments-long.json"
              : "filled.json",
        );
      }
      if (url.pathname === "/v1/dashboard/requests") {
        return answer(
          response,
          scenario === "empty"
            ? "empty-list.json"
            : scenario === "requests-long"
              ? "requests-long.json"
              : "requests.json",
        );
      }
      if (url.pathname === "/v1/dashboard/webhook-deliveries") {
        return answer(
          response,
          scenario === "empty" ? "empty-list.json" : "deliveries.json",
        );
      }
    }
    if (url.pathname === "/session/registration" && request.method === "POST") {
      if (scenario === "sign-in-key-busy") return;
      if (
        scenario === "sign-in-held-busy" &&
        cookies.includes("dashboard-key-held=true")
      )
        return;
      return answer(response, "registration-refused.json");
    }
    if (url.pathname === "/session") {
      if (request.method === "DELETE") {
        if (scenario === "sign-out-busy") return;
        return answer(
          response,
          scenario === "sign-out-refused"
            ? "unavailable.json"
            : "signed-out.txt",
        );
      }
      if (request.method === "POST") {
        if (scenario === "sign-in-token-busy") return;
        const chunks = [];
        for await (const chunk of request) chunks.push(chunk);
        const body = JSON.parse(Buffer.concat(chunks).toString());
        return answer(
          response,
          body.token === "session_fixture_opaque"
            ? "signed-in.txt"
            : "token-refused.json",
        );
      }
    }
    const target =
      url.pathname === "/" &&
      url.searchParams.get("fixture") === "sign-in-unconfigured"
        ? 4177
        : 4176;
    const upstream = forward(
      {
        hostname: "127.0.0.1",
        port: target,
        method: request.method,
        path: url.pathname === "/" ? "/" : `${url.pathname}${url.search}`,
        headers: { ...request.headers, "accept-encoding": "identity" },
      },
      (result) => {
        const headers = { ...result.headers };
        headers["access-control-allow-origin"] = "null";
        headers["access-control-allow-credentials"] = "true";
        if (url.searchParams.get("fixture") === "sign-in-unsupported") {
          headers["content-security-policy"] = "sandbox allow-scripts";
          headers["referrer-policy"] = "unsafe-url";
          headers["set-cookie"] =
            "dashboard-fixture=sign-in-unsupported; Secure; HttpOnly; SameSite=None; Path=/";
        }
        if (url.searchParams.get("fixture") === "sign-in-copy-refused") {
          headers["permissions-policy"] = "clipboard-write=()";
        }
        delete headers["content-length"];
        delete headers["transfer-encoding"];
        response.writeHead(result.statusCode, headers);
        if (url.pathname !== "/") {
          result.pipe(response);
          return;
        }
        const chunks = [];
        result.on("data", (chunk) => chunks.push(chunk));
        result.on("end", () => {
          const html = Buffer.concat(chunks).toString();
          response.end(
            html.replace("<head>", `<head><script>${environment}</script>`),
          );
        });
      },
    );
    upstream.on("error", (error) => {
      if (!response.headersSent)
        response.writeHead(502, { "content-type": "text/plain" });
      response.end(error.message);
    });
    request.pipe(upstream);
  } catch (error) {
    console.error(error);
    if (!response.headersSent) response.writeHead(500);
    response.end();
  }
});

let closing = false;
async function close(code = 0) {
  if (closing) return;
  closing = true;
  surface.closeAllConnections();
  surface.close();
  for (const child of processes) child.kill("SIGTERM");
  process.exitCode = code;
}
process.on("SIGTERM", () => void close());
process.on("SIGINT", () => void close());
surface.listen(4175, "127.0.0.1");
