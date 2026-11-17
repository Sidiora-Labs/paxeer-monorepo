import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdtemp, readFile, readdir, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { chromium } from "playwright";
import { PNG } from "pngjs";
import { captureFullPage, FULL_PAGE_CAPTURE_MAX_MS, keyboardFindings } from "../check.mjs";
import { measureContrast, measurePage, observePage } from "../browser.mjs";
import { LIMITS } from "../config.mjs";
import { measureParts } from "../geometry.mjs";

const fixtures = join(dirname(fileURLToPath(import.meta.url)), "fixtures");
const require = createRequire(import.meta.url);
const axeSource = await readFile(require.resolve("axe-core/axe.min.js"), "utf8");

async function captureFixture({ screens, delayMs, limits, wide = false }) {
  const body = await readFile(join(fixtures, "capture.html"));
  const font = await readFile(join(fixtures, "capture-font.woff2"));
  const out = await mkdtemp(join(tmpdir(), "ui-capture-"));
  let fontResponse;
  let timer;
  let responded = false;
  let requested;
  const fontRequested = new Promise((resolve) => { requested = resolve; });
  const server = createServer((request, response) => {
    if (request.url === "/capture-font.woff2") {
      fontResponse = response;
      requested();
      return;
    }
    response.writeHead(200, { "content-type": "text/html" });
    response.end(body);
  });
  await new Promise((resolve) => server.listen(0, "localhost", resolve));
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 390, height: 900 } });
  const consoleErrors = [];
  page.on("console", (message) => { if (message.type() === "error") consoleErrors.push(message.text()); });
  const close = async () => {
    clearTimeout(timer);
    await browser.close();
    await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
    await rm(out, { recursive: true, force: true });
  };
  try {
    await page.addInitScript(observePage);
    await page.goto(`http://localhost:${server.address().port}/capture.html?screens=${screens}${wide ? "&wide" : ""}`);
    const findings = await page.evaluate(measurePage, { limits: LIMITS, typeScale: [16], doubled: false, reduced: false });
    await page.evaluate(axeSource);
    findings.push(...await page.evaluate(measureContrast, LIMITS));
    const geometry = await measureParts(page, [{ name: "panel", selector: "#panel" }]);
    const keyboard = await keyboardFindings(page);
    findings.push(...keyboard.findings);
    const viewport = PNG.sync.read(await page.screenshot({ fullPage: false, timeout: LIMITS.navigationMs, animations: "allow" }));
    const render = {
      rendered: true, hasContent: new Set(new Uint32Array(viewport.data.buffer, viewport.data.byteOffset, viewport.data.byteLength / 4)).size > 1,
      focused: keyboard.visited, geometry, findings, screenshot: "capture.png",
    };
    const measured = structuredClone(render);
    await page.evaluate(() => beginCapture());
    await fontRequested;
    assert.equal(await page.evaluate(() => document.fonts.status), "loading");
    const capture = captureFullPage(page, render, out, limits);
    timer = setTimeout(() => {
      responded = true;
      fontResponse.writeHead(200, { "content-type": "font/woff2" });
      fontResponse.end(font);
    }, delayMs);
    await capture;
    return { render, measured, out, page, consoleErrors, responded, close };
  } catch (error) {
    await close();
    throw error;
  }
}

async function tallPageCaptureOutlastsNavigationAndKeepsItsImage() {
  const limits = { navigationMs: 1000, maxMs: 12000 };
  const delayMs = 3000;
  const screens = 8;
  assert.ok(delayMs > limits.navigationMs && delayMs < limits.navigationMs * screens);
  const capture = await captureFixture({ screens, delayMs, limits });
  try {
    assert.deepEqual(capture.render, capture.measured);
    assert.equal(capture.render.rendered, true);
    assert.equal(capture.render.hasContent, true);
    assert.deepEqual(capture.render.findings, []);
    assert.equal(capture.responded, true);
    assert.equal(await capture.page.evaluate(() => window.captureFont.status), "loaded");
    assert.deepEqual(capture.consoleErrors, []);
    const image = PNG.sync.read(await readFile(join(capture.out, capture.render.screenshot)));
    assert.equal(image.width, 390);
    assert.equal(image.height, screens * 900);
  } finally { await capture.close(); }
}
test(tallPageCaptureOutlastsNavigationAndKeepsItsImage);

async function oneScreenCaptureUsesTheNavigationLimit() {
  const limits = { navigationMs: 1000, maxMs: 6000 };
  const capture = await captureFixture({ screens: 1, delayMs: limits.maxMs * 2, limits });
  try {
    assert.equal(capture.render.rendered, false);
    assert.equal(capture.render.screenshot, null);
    assert.equal(capture.responded, false);
    assert.equal(capture.render.findings.length, 1);
    const [finding] = capture.render.findings;
    assert.equal(finding.kind, "load");
    assert.equal(finding.values.source, "screenshot");
    assert.equal(finding.values.height, 900);
    assert.equal(finding.values.milliseconds, limits.navigationMs);
    assert.match(finding.values.message, /Timeout 1000ms exceeded/);
    assert.deepEqual(await readdir(capture.out), []);
  } finally { await capture.close(); }
}
test(oneScreenCaptureUsesTheNavigationLimit);

async function partialViewportHeightRoundsUpToTheNextNavigationLimit() {
  const limits = { navigationMs: 1000, maxMs: 6000 };
  const capture = await captureFixture({ screens: 1.5, delayMs: limits.maxMs * 2, limits });
  try {
    assert.equal(capture.render.rendered, false);
    assert.equal(capture.render.screenshot, null);
    assert.equal(capture.render.findings.length, 1);
    const [finding] = capture.render.findings;
    assert.equal(finding.values.source, "screenshot");
    assert.equal(finding.values.height, 1350);
    assert.equal(finding.values.milliseconds, limits.navigationMs * 2);
    assert.match(finding.values.message, /Timeout 2000ms exceeded/);
  } finally { await capture.close(); }
}
test(partialViewportHeightRoundsUpToTheNextNavigationLimit);

async function captureAtTheUpperBoundRetainsMeasuredFindingsAndGeometry() {
  assert.equal(FULL_PAGE_CAPTURE_MAX_MS, 60000);
  assert.equal(LIMITS.navigationMs, 10000);
  assert.equal(LIMITS.renderMs, 300000);
  const limits = { navigationMs: 1000, maxMs: 3000 };
  const screens = 20;
  const capture = await captureFixture({ screens, delayMs: limits.maxMs * 3, limits, wide: true });
  try {
    const { render, measured } = capture;
    assert.ok(measured.findings.some((finding) => finding.kind === "sideways"));
    assert.deepEqual(render.findings.slice(0, -1), measured.findings);
    assert.equal(render.hasContent, true);
    assert.equal(render.hasContent, measured.hasContent);
    assert.deepEqual(render.focused, ["#control"]);
    assert.deepEqual(render.focused, measured.focused);
    assert.equal(render.geometry.length, 1);
    assert.deepEqual(render.geometry, measured.geometry);
    assert.equal(render.rendered, false);
    assert.equal(render.screenshot, null);
    assert.equal(capture.responded, false);
    const finding = render.findings.at(-1);
    assert.equal(finding.kind, "load");
    assert.equal(finding.selector, "document");
    assert.equal(finding.values.source, "screenshot");
    assert.equal(finding.values.height, screens * 900);
    assert.equal(finding.values.milliseconds, limits.maxMs);
    assert.match(finding.values.message, /Timeout 3000ms exceeded/);
    assert.deepEqual(await readdir(capture.out), []);
  } finally { await capture.close(); }
}
test(captureAtTheUpperBoundRetainsMeasuredFindingsAndGeometry);
