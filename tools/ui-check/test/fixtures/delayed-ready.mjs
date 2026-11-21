import { readFile } from "node:fs/promises";
import { createServer } from "node:http";

const started = Date.now();
const pages = new Map(await Promise.all(["clean.html", "base.css"].map(async (name) => [
  `/${name}`, await readFile(new URL(name, import.meta.url)),
])));

createServer((request, response) => {
  if (request.url === "/ready") {
    const ready = Date.now() - started >= 2000;
    response.writeHead(ready ? 200 : 503, { "content-type": "text/plain" });
    response.end(ready ? "Ready" : "Starting");
    return;
  }
  const body = pages.get(request.url);
  response.writeHead(body ? 200 : 404, {
    "content-type": request.url.endsWith(".css") ? "text/css" : "text/html",
  });
  response.end(body ?? "Not found");
}).listen(Number(process.argv[2]), "localhost");
