import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { lstat, readFile, realpath, rm, stat } from "node:fs/promises";
import { extname, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { LIMITS, surfaceOutput, within } from "./config.mjs";

const CONTENT_TYPES = Object.freeze({
  ".html": "text/html; charset=utf-8", ".css": "text/css", ".js": "text/javascript",
  ".mjs": "text/javascript", ".json": "application/json", ".svg": "image/svg+xml",
  ".png": "image/png", ".jpg": "image/jpeg", ".woff": "font/woff", ".woff2": "font/woff2",
});

export async function serveFiles(directory, address) {
  const root = await realpath(directory);
  const url = new URL(address);
  const server = createServer(async (request, response) => {
    try {
      const path = decodeURIComponent(new URL(request.url, address).pathname);
      let file = await realpath(resolve(root, `.${path}`));
      if (!within(root, file)) throw new Error("Path is outside the served directory");
      if ((await stat(file)).isDirectory()) file = await realpath(resolve(file, "index.html"));
      if (!within(root, file)) throw new Error("Path is outside the served directory");
      const body = await readFile(file);
      response.writeHead(200, { "content-type": CONTENT_TYPES[extname(file)] ?? "application/octet-stream", "cache-control": "no-store" });
      response.end(request.method === "HEAD" ? undefined : body);
    } catch {
      response.writeHead(404, { "content-type": "text/plain" });
      response.end("Not found");
    }
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(Number(url.port || 80), url.hostname.replace(/^\[|\]$/g, ""), resolve);
  });
  const actual = new URL(address);
  actual.port = String(server.address().port);
  return {
    address: actual.origin,
    close: () => new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

function commandProcess(command, cwd) {
  const child = spawn(command, { cwd, shell: true, detached: true, stdio: ["ignore", "pipe", "pipe"] });
  let output = "";
  let failure;
  for (const stream of [child.stdout, child.stderr]) {
    stream.on("data", (chunk) => { output = (output + chunk.toString()).slice(-16000); });
  }
  const ended = new Promise((resolve) => {
    child.once("error", (error) => { failure = error; resolve({ code: null, signal: null }); });
    child.once("exit", (code, signal) => resolve({ code, signal }));
  });
  return {
    child, ended,
    description: () => failure?.message ?? output,
    async close() {
      if (!child.pid) return;
      try { process.kill(-child.pid, "SIGTERM"); } catch (error) { if (error.code !== "ESRCH") throw error; }
      await Promise.race([ended, delay(250)]);
      try { process.kill(-child.pid, "SIGKILL"); } catch (error) { if (error.code !== "ESRCH") throw error; }
      await ended;
    },
  };
}

export async function startSurface(surface, removedOutputs = []) {
  if (surface.build) {
    const outputs = await Promise.all((surface.outputs ?? []).map(async (path) => ({
      path, destination: await surfaceOutput(surface.directory, path, surface.name),
    })));
    for (const { path, destination } of outputs) {
      try {
        await lstat(destination);
      } catch (error) {
        if (error.code === "ENOENT") continue;
        throw error;
      }
      await rm(destination, { recursive: true, force: true });
      removedOutputs.push(path);
    }
    const buildMs = surface.buildMs ?? LIMITS.buildMs;
    const build = commandProcess(surface.build, surface.directory);
    try {
      const result = await Promise.race([build.ended, delay(buildMs, undefined, { ref: false }).then(() => ({ timeout: true }))]);
      if (result.timeout || result.code !== 0) throw new Error(`Surface ${surface.name} build did not succeed: ${result.timeout ? `timed out after ${buildMs} ms` : result.code}\n${build.description()}`);
    } finally { await build.close(); }
  }
  let service;
  if (surface.start) {
    const command = commandProcess(surface.start, surface.directory);
    service = { address: surface.address, close: () => command.close(), command };
  } else {
    service = await serveFiles(surface.directory, surface.address);
  }
  const deadline = Date.now() + LIMITS.readyMs;
  try {
    while (Date.now() < deadline) {
      if (service.command && (service.command.child.exitCode !== null || service.command.child.signalCode !== null)) {
        throw new Error(`Surface stopped before readiness: ${service.command.description()}`);
      }
      try {
        const response = await fetch(new URL(surface.readyPath, service.address), { signal: AbortSignal.timeout(500), redirect: "error" });
        await response.body?.cancel();
        if (response.ok) return service;
      } catch { /* Readiness is polled until the fixed deadline. */ }
      await delay(100);
    }
    throw new Error(`Surface did not become ready at ${surface.readyPath}`);
  } catch (error) {
    await service.close();
    throw error;
  }
}
