import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { setTimeout as delay } from "node:timers/promises";

const upstream = "http://127.0.0.1:4174";
const child = spawn(
  "pnpm",
  ["exec", "next", "start", "--hostname", "127.0.0.1", "--port", "4174"],
  {
    stdio: "inherit",
  },
);
const identity = await readFile(
  new URL("./identity.json", import.meta.url),
  "utf8",
);
const browser = await readFile(
  new URL("./browser.js", import.meta.url),
  "utf8",
);
const bootstrap = `const walletIdentity = ${identity};\n${browser}`;

for (let index = 0; index < 120; index++) {
  if (child.exitCode !== null)
    throw new Error(`Demo stopped: ${child.exitCode}`);
  try {
    const response = await fetch(upstream, {
      signal: AbortSignal.timeout(300),
    });
    await response.body?.cancel();
    if (response.ok) break;
  } catch {
    // The page becomes available after the application starts.
  }
  if (index === 119) throw new Error("Demo did not become available");
  await delay(100);
}

const server = createServer(async (request, response) => {
  try {
    if (request.url === "/__wallet_fixtures__/bootstrap.js") {
      response.writeHead(200, { "content-type": "text/javascript" });
      response.end(bootstrap);
      return;
    }
    const answer = await fetch(new URL(request.url, upstream), {
      method: request.method,
      headers: { ...request.headers, host: new URL(upstream).host },
      redirect: "manual",
    });
    const headers = Object.fromEntries(answer.headers);
    delete headers["content-length"];
    delete headers["content-encoding"];
    delete headers["transfer-encoding"];
    const body = Buffer.from(await answer.arrayBuffer());
    response.writeHead(answer.status, headers);
    response.end(
      headers["content-type"]?.includes("text/html")
        ? body
            .toString()
            .replace(
              "<head>",
              '<head><script src="/__wallet_fixtures__/bootstrap.js"></script>',
            )
        : body,
    );
  } catch (error) {
    response.writeHead(502, { "content-type": "text/plain" });
    response.end(error.message);
  }
});
server.listen(4173, "127.0.0.1");

function close() {
  server.close();
  server.closeAllConnections();
  child.kill("SIGTERM");
}
process.on("SIGTERM", close);
process.on("SIGINT", close);
child.on("exit", (code) => {
  server.close();
  process.exitCode = code ?? 1;
});
