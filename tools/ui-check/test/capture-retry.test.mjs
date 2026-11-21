import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdir, mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { checkSurface } from "../check.mjs";
import { LIMITS } from "../config.mjs";

const fixtures = join(dirname(fileURLToPath(import.meta.url)), "fixtures");
const cli = fileURLToPath(new URL("../check.mjs", import.meta.url));
const navigationMs = 1000;

async function captureRetrySurface({ capture = "full-page", holds = 1, wide = false, fileError = false } = {}) {
  const root = await mkdtemp(join(tmpdir(), "ui-capture-retry-"));
  const body = await readFile(join(fixtures, "capture-retry.html"));
  const font = await readFile(join(fixtures, "capture-font.woff2"));
  const requests = { document: 0, font: 0, held: 0 };
  const instance = createServer((request, response) => {
    const url = new URL(request.url, "http://localhost");
    if (url.pathname === "/ready") { response.end("Ready"); return; }
    if (url.pathname === "/capture-font.woff2") {
      if (++requests.font <= holds) { requests.held++; return; }
      response.writeHead(200, { "content-type": "font/woff2" });
      response.end(font);
      return;
    }
    if (url.pathname !== "/index.html") { response.writeHead(404); response.end(); return; }
    requests.document++;
    response.writeHead(200, { "content-type": "text/html" });
    response.end(body);
  });
  await new Promise((resolve) => instance.listen(0, "localhost", resolve));
  const surface = join(root, "surface.json");
  const geometryFile = join(root, "geometry.json");
  const out = join(root, "renders");
  const writing = capture === "viewport";
  await writeFile(surface, JSON.stringify({
    name: "Capture readiness", directory: fixtures,
    start: `${JSON.stringify(process.execPath)} -e "setInterval(()=>{},1000)"`,
    address: `http://localhost:${instance.address().port}`, readyPath: "/ready",
    views: [{
      name: "capture", path: `/index.html?capture=${capture}${wide ? "&wide" : ""}`,
      modes: writing || fileError ? ["default"] : ["default", "keyboard"],
      ...(writing ? { parts: [{ name: "panel", selector: "#panel" }] } : {}),
    }],
    ...(writing ? { geometry: { file: "geometry.json", widths: [390] } } : {}),
    answers: [], typeScale: [16],
  }));
  if (fileError) await mkdir(join(out, "00001.png"), { recursive: true });
  return {
    requests, out, geometryFile,
    async run() {
      const lines = [];
      let result;
      if (fileError) {
        const child = spawn(process.execPath, [cli, "--surface", surface, "--out", out, "--jobs", "1"], { stdio: ["ignore", "pipe", "pipe"] });
        let stdout = "";
        let stderr = "";
        child.stdout.on("data", (chunk) => { stdout += chunk; });
        child.stderr.on("data", (chunk) => { stderr += chunk; });
        const code = await new Promise((resolve, reject) => { child.once("error", reject); child.once("close", resolve); });
        assert.equal(code, 1, stderr + stdout);
        assert.equal(stderr, "");
        lines.push(...stdout.trim().split("\n"));
        result = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
      } else result = await checkSurface({ surface, out, writeGeometry: writing, jobs: 1, navigationMs, renderMs: 15000 }, (line) => lines.push(line));
      if (!writing) assert.deepEqual(JSON.parse(await readFile(join(out, "result.json"), "utf8")), result);
      assert.equal(result.limits.navigationMs, LIMITS.navigationMs);
      return { result, lines };
    },
    async close() {
      await new Promise((resolve) => { instance.close(resolve); instance.closeAllConnections(); });
      await rm(root, { recursive: true, force: true });
    },
  };
}

async function captureTimeoutUsesTheSecondRender(capture) {
  const surface = await captureRetrySurface({ capture });
  try {
    const { result, lines } = await surface.run();
    const writing = capture === "viewport";
    const count = writing ? 1 : 26;
    assert.equal(result.passed, true, JSON.stringify(result.findings));
    assert.equal(result.renders.length, count);
    assert.deepEqual(result.findings, []);
    assert.equal(result.retriedCount, 1);
    assert.equal(surface.requests.document, count + 1);
    assert.equal(surface.requests.held, 1);
    const retried = result.renders.filter((render) => render.retried);
    assert.equal(retried.length, 1);
    const [render] = retried;
    assert.equal(render.firstDeadlineStep, null);
    assert.equal(render.firstLoadFailure.cause, "screenshot-timeout");
    assert.equal(render.firstLoadFailure.source, writing ? "render" : "screenshot");
    assert.match(render.firstLoadFailure.message, /page.screenshot: Timeout 1000ms exceeded/);
    assert.ok(result.renders.every((entry) => entry.rendered && entry.hasContent && entry.cancelled === 0));
    assert.ok(result.renders.filter((entry) => entry !== render).every((entry) => !entry.retried && entry.firstLoadFailure === null));
    assert.match(lines[0], /, 0 findings, 1 retried$/);
    assert.match(lines.at(-1), /, pass, .*1 retried$/);
    if (writing) {
      assert.equal(JSON.parse(await readFile(surface.geometryFile, "utf8")).entries.length, 1);
    } else {
      assert.equal(render.firstLoadFailure.height, 900);
      assert.equal(render.firstLoadFailure.milliseconds, navigationMs);
      assert.equal((await readdir(surface.out)).filter((file) => file.endsWith(".png")).length, count);
      assert.deepEqual(result.renders.map((entry) => entry.screenshot), Array.from({ length: count }, (_, index) => `${String(index + 1).padStart(5, "0")}.png`));
    }
  } finally { await surface.close(); }
}

async function viewportCaptureTimeoutUsesTheSecondRender() {
  await captureTimeoutUsesTheSecondRender("viewport");
}
test(viewportCaptureTimeoutUsesTheSecondRender);

async function fullPageCaptureTimeoutUsesTheSecondRender() {
  await captureTimeoutUsesTheSecondRender("full-page");
}
test(fullPageCaptureTimeoutUsesTheSecondRender);

async function captureTimeoutKeepsTheSecondFinding(capture) {
  const surface = await captureRetrySurface({ capture, holds: 2 });
  try {
    const { result, lines } = await surface.run();
    const writing = capture === "viewport";
    const count = writing ? 1 : 26;
    assert.equal(result.passed, false);
    assert.equal(result.renders.length, count);
    assert.equal(result.retriedCount, 1);
    assert.equal(surface.requests.document, count + 1);
    assert.equal(surface.requests.held, 2);
    const retried = result.renders.filter((render) => render.retried);
    assert.equal(retried.length, 1);
    const [render] = retried;
    assert.equal(render.rendered, false);
    assert.equal(render.hasContent, !writing);
    assert.equal(render.screenshot, null);
    assert.equal(render.firstDeadlineStep, null);
    assert.equal(render.firstLoadFailure.cause, "screenshot-timeout");
    assert.equal(render.findings.length, 1, JSON.stringify(render.findings));
    assert.equal(render.findings[0].kind, "load");
    assert.equal(render.findings[0].values.source, writing ? "render" : "screenshot");
    assert.match(render.findings[0].values.message, /page.screenshot: Timeout 1000ms exceeded/);
    assert.deepEqual(result.findings, render.findings);
    assert.ok(result.renders.filter((entry) => entry !== render).every((entry) => entry.rendered && !entry.retried && entry.firstLoadFailure === null));
    assert.match(lines[0], /1 did not render, .*1 findings, 1 retried$/);
    assert.match(lines.at(-1), /, fail, .*1 retried$/);
    if (writing) await assert.rejects(readFile(surface.geometryFile), { code: "ENOENT" });
    else assert.equal((await readdir(surface.out)).filter((file) => file.endsWith(".png")).length, count - 1);
  } finally { await surface.close(); }
}

async function viewportCaptureTimeoutKeepsTheSecondFinding() {
  await captureTimeoutKeepsTheSecondFinding("viewport");
}
test(viewportCaptureTimeoutKeepsTheSecondFinding);

async function fullPageCaptureTimeoutKeepsTheSecondFinding() {
  await captureTimeoutKeepsTheSecondFinding("full-page");
}
test(fullPageCaptureTimeoutKeepsTheSecondFinding);

async function captureTimeoutBesideLayoutFindingsDoesNotRetry() {
  const surface = await captureRetrySurface({ wide: true });
  try {
    const { result, lines } = await surface.run();
    assert.equal(result.passed, false);
    assert.equal(result.renders.length, 26);
    assert.equal(result.retriedCount, 0);
    assert.equal(surface.requests.document, 26);
    assert.equal(surface.requests.held, 1);
    const captures = result.renders.filter((render) => render.findings.some((finding) => finding.values.source === "screenshot"));
    assert.equal(captures.length, 1);
    assert.equal(captures[0].rendered, false);
    assert.ok(captures[0].findings.some((finding) => finding.kind === "sideways"));
    assert.match(captures[0].findings.find((finding) => finding.values.source === "screenshot").values.message, /Timeout 1000ms exceeded/);
    assert.ok(result.renders.every((render) => !render.retried && render.firstLoadFailure === null));
    assert.match(lines.at(-1), /, fail, .*0 retried$/);
  } finally { await surface.close(); }
}
test(captureTimeoutBesideLayoutFindingsDoesNotRetry);

async function captureFileErrorDoesNotRetry() {
  const surface = await captureRetrySurface({ holds: 0, fileError: true });
  try {
    const { result, lines } = await surface.run();
    assert.equal(result.passed, false);
    assert.equal(result.renders.length, 22);
    assert.equal(result.retriedCount, 0);
    assert.equal(surface.requests.document, 22);
    assert.equal(result.findings.length, 1, JSON.stringify(result.findings));
    const [render, ...following] = result.renders;
    assert.equal(render.rendered, false);
    assert.equal(render.hasContent, true);
    assert.equal(render.screenshot, null);
    assert.equal(result.findings[0].kind, "load");
    assert.equal(result.findings[0].values.source, "screenshot");
    assert.match(result.findings[0].values.message, /EISDIR/);
    assert.ok(result.renders.every((entry) => !entry.retried && entry.firstLoadFailure === null));
    assert.ok(following.every((entry) => entry.rendered && entry.findings.length === 0));
    assert.match(lines.at(-1), /, fail, .*0 retried$/);
  } finally { await surface.close(); }
}
test(captureFileErrorDoesNotRetry);
