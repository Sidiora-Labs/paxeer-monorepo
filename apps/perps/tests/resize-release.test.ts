import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

test(
  "ResizeHandle flushes pending mouse and touch positions with bounds and cleanup",
  { timeout: 30_000 },
  async (t) => {
    const directory = await mkdtemp(join(tmpdir(), "resize-release-"));
    t.after(() => rm(directory, { recursive: true, force: true }));
    const bundle =
      process.env.RESIZE_RELEASE_TEST_BUNDLE ?? join(directory, "browser.js");
    if (!process.env.RESIZE_RELEASE_TEST_BUNDLE) {
      const require = createRequire(import.meta.url);
      const { build } = createRequire(require.resolve("tsx"))(
        "esbuild",
      ) as typeof import("esbuild");
      await build({
        entryPoints: [
          fileURLToPath(
            new URL("./resize-release.browser.ts", import.meta.url),
          ),
        ],
        bundle: true,
        platform: "browser",
        format: "iife",
        jsx: "automatic",
        define: { "process.env.NODE_ENV": '"development"' },
        outfile: bundle,
      });
    }

    const script = await readFile(bundle);
    const server = createServer((request, response) => {
      if (request.url === "/browser.js") {
        response.setHeader("Content-Type", "text/javascript");
        response.end(script);
      } else if (request.url === "/") {
        response.setHeader("Content-Type", "text/html");
        response.end(
          '<!doctype html><html><body><script src="/browser.js"></script></body></html>',
        );
      } else {
        response.writeHead(404).end();
      }
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    t.after(
      () =>
        new Promise<void>((resolve, reject) =>
          server.close((error) => (error ? reject(error) : resolve())),
        ),
    );
    const { port } = server.address() as AddressInfo;
    const { stdout, stderr } = await promisify(execFile)(
      process.env.CHROMIUM_BIN ?? "chromium",
      [
        "--headless",
        "--disable-gpu",
        "--no-first-run",
        `--user-data-dir=${join(directory, "profile")}`,
        "--virtual-time-budget=10000",
        "--dump-dom",
        `http://127.0.0.1:${port}/`,
      ],
      { timeout: 20_000, maxBuffer: 4 * 1024 * 1024 },
    );
    assert.match(
      stdout,
      /<pre id="resize-release-result">PASS \d+<\/pre>/,
      `${stdout}\n${stderr}`,
    );
    t.diagnostic(stdout.match(/PASS \d+/)![0]);
  },
);
