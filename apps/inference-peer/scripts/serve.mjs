import { createReadStream } from "node:fs";
import { realpath, stat } from "node:fs/promises";
import { createServer } from "node:http";
import {
  dirname,
  extname,
  isAbsolute,
  relative,
  resolve,
  sep,
} from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { pipeline } from "node:stream/promises";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const types = new Map([
  [".html", "text/html; charset=utf-8"],
  [".js", "text/javascript; charset=utf-8"],
  [".mjs", "text/javascript; charset=utf-8"],
  [".json", "application/json; charset=utf-8"],
  [".wasm", "application/wasm"],
]);

export function createPeerServer() {
  return createServer(async (request, response) => {
    response.setHeader("cache-control", "no-store");
    response.setHeader("x-content-type-options", "nosniff");
    response.setHeader("cross-origin-opener-policy", "same-origin");
    response.setHeader("cross-origin-embedder-policy", "require-corp");
    if (!["GET", "HEAD"].includes(request.method)) {
      response.writeHead(405, { allow: "GET, HEAD" });
      response.end();
      return;
    }
    try {
      const pathname = decodeURIComponent(
        new URL(request.url, "http://localhost").pathname,
      );
      if (pathname.includes("\0") || pathname.includes("\\")) {
        response.writeHead(400);
        response.end();
        return;
      }
      if (pathname.split("/").some((part) => part === ".." || part === ".")) {
        response.writeHead(403);
        response.end();
        return;
      }
      if (pathname === "/probe") {
        response.writeHead(200, {
          "content-type": "application/octet-stream",
          "content-length": 65536,
        });
        response.end(
          request.method === "HEAD" ? undefined : Buffer.alloc(65536),
        );
        return;
      }
      if (
        pathname !== "/" &&
        pathname !== "/index.html" &&
        !pathname.startsWith("/src/") &&
        !pathname.startsWith("/engine/")
      ) {
        response.writeHead(404);
        response.end();
        return;
      }
      const path = await realpath(
        resolve(root, `.${pathname === "/" ? "/index.html" : pathname}`),
      );
      const local = relative(root, path);
      if (
        local === ".." ||
        local.startsWith(`..${sep}`) ||
        isAbsolute(local) ||
        (local !== "index.html" &&
          !local.startsWith(`src${sep}`) &&
          !local.startsWith(`engine${sep}`))
      ) {
        response.writeHead(403);
        response.end();
        return;
      }
      const metadata = await stat(path);
      if (!metadata.isFile() || !types.has(extname(path))) {
        response.writeHead(404);
        response.end();
        return;
      }
      response.writeHead(200, {
        "content-type": types.get(extname(path)),
        "content-length": metadata.size,
      });
      if (request.method === "HEAD") response.end();
      else await pipeline(createReadStream(path), response);
    } catch (error) {
      if (response.headersSent) response.destroy(error);
      else {
        response.writeHead(
          error instanceof URIError
            ? 400
            : error.code === "ENOENT" || error.code === "ENOTDIR"
              ? 404
              : 500,
        );
        response.end();
      }
    }
  });
}

if (
  process.argv[1] &&
  pathToFileURL(resolve(process.argv[1])).href === import.meta.url
) {
  const port = Number(process.env.PORT ?? 4173);
  if (!Number.isInteger(port) || port < 1 || port > 65535)
    throw new RangeError("PORT must be an integer from 1 to 65535");
  const server = createPeerServer();
  server.on("error", (error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
  server.listen(port, "127.0.0.1", () =>
    console.log(`Inference peer: http://127.0.0.1:${port}`),
  );
  for (const signal of ["SIGINT", "SIGTERM"])
    process.once(signal, () => server.close());
}
