import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";

const upstream = "http://127.0.0.1:3100";
const transport = await readFile(new URL("./transport.js", import.meta.url));
const child = spawn(
  "pnpm",
  ["exec", "next", "start", "-H", "127.0.0.1", "-p", "3100"],
  { stdio: "inherit" },
);
const server = createServer(async (request, response) => {
  try {
    if (request.url === "/wallet/__answers__/transport.js") {
      response.writeHead(200, { "content-type": "text/javascript" });
      response.end(transport);
      return;
    }
    if (request.url.startsWith("/wallet/api/")) {
      response.writeHead(307, {
        location: `http://127.0.0.1:4174/reads/${request.url.slice("/wallet/api/".length)}`,
      });
      response.end();
      return;
    }
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = Buffer.concat(chunks);
    const answer = await fetch(new URL(request.url, upstream), {
      method: request.method,
      headers: { ...request.headers, host: new URL(upstream).host },
      ...(body.length ? { body } : {}),
      redirect: "manual",
    });
    const headers = Object.fromEntries(answer.headers);
    delete headers["content-length"];
    delete headers["content-encoding"];
    delete headers["transfer-encoding"];
    const content = Buffer.from(await answer.arrayBuffer());
    response.writeHead(answer.status, headers);
    response.end(
      headers["content-type"]?.includes("text/html")
        ? content
            .toString()
            .replace(
              "<head>",
              `<head><script nonce="${headers["content-security-policy"]?.match(/'nonce-([^']+)'/)?.[1] ?? ""}" src="/wallet/__answers__/transport.js"></script>`,
            )
        : content,
    );
  } catch (error) {
    response.writeHead(502, { "content-type": "text/plain" });
    response.end(error.message);
  }
});
server.listen(3099, "127.0.0.1");
function close() {
  server.close();
  server.closeAllConnections();
  child.kill("SIGTERM");
}
process.on("SIGTERM", close);
process.on("SIGINT", close);
child.on("exit", (code) => {
  server.close();
  server.closeAllConnections();
  process.exitCode = code ?? 1;
});
