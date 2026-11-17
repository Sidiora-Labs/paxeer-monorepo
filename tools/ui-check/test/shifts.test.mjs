import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test, { after, before } from "node:test";
import { chromium } from "playwright";
import { applySteps, checkSurface, SHIFT_QUIET_FRAMES, SHIFT_SETTLE_MS, withIntentionalShifts } from "../check.mjs";
import { doubleText, measurePage, observePage } from "../browser.mjs";
import { LIMITS } from "../config.mjs";
import { serveFiles } from "../server.mjs";

const fixtures = join(dirname(fileURLToPath(import.meta.url)), "fixtures");
const afterRestMs = 80;
const beforeStepsMs = 40;
const panelSteps = [
  { action: "click", selector: "#open" },
  { action: "wait", text: "Panel opened" },
];
let browser;
let files;
let root;

before(async function openShiftPages() {
  root = await mkdtemp(join(tmpdir(), "ui-shifts-"));
  files = await serveFiles(fixtures, "http://localhost:0");
  browser = await chromium.launch();
});
after(async function closeShiftPages() {
  await browser?.close();
  await files?.close();
  if (root) await rm(root, { recursive: true, force: true });
});

async function shiftPage(path, action) {
  const context = await browser.newContext({ viewport: { width: 390, height: 900 } });
  try {
    const delivery = await readFile(join(fixtures, "shift-delivery.js"), "utf8");
    await context.addInitScript({ content: `${delivery}\n(${observePage.toString()})();` });
    const page = await context.newPage();
    await page.goto(`${files.address}/${path}`);
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await action(page);
  } finally { await context.close(); }
}

async function measureShifts(page, doubled = false) {
  return page.evaluate(measurePage, { limits: LIMITS, typeScale: [16, 24], doubled, reduced: false });
}

async function delayedEnlargementEntriesStayExcludedAtEveryRead() {
  await shiftPage("intentional-shifts.html?hold", async (page) => {
    const interval = await withIntentionalShifts(page, () => page.evaluate(doubleText));
    assert.ok(interval.end >= interval.start);
    await page.waitForFunction(() => window.shiftDelivery.pending.length > 0);
    assert.deepEqual(await measureShifts(page, true), []);
    const delivered = await page.evaluate(() => {
      const releasedAt = performance.now();
      window.shiftDelivery.release();
      return { releasedAt, entries: window.shiftDelivery.delivered, raw: window.__uiCheck.shifts };
    });
    assert.ok(delivered.releasedAt > interval.end);
    assert.ok(delivered.entries.reduce((sum, entry) => sum + entry.value, 0) > LIMITS.shift);
    assert.ok(delivered.entries.every((entry) => entry.time >= interval.start && entry.time <= interval.end));
    assert.ok(delivered.raw.length > 0);
    assert.deepEqual(await measureShifts(page, true), []);
    assert.deepEqual(await measureShifts(page, true), []);
  });
}
test(delayedEnlargementEntriesStayExcludedAtEveryRead);

async function movementAfterEnlargementRestRemainsAFinding() {
  await shiftPage(`intentional-shifts.html?movement=after&delay=${afterRestMs}`, async (page) => {
    assert.equal(await page.evaluate(() => window.shiftFixture.delayMs), afterRestMs);
    const interval = await withIntentionalShifts(page, () => page.evaluate(doubleText));
    await page.waitForFunction((end) => window.__uiCheck.shifts.some((entry) => entry.time > end), interval.end);
    const findings = await measureShifts(page, true);
    assert.equal(findings.length, 1, JSON.stringify(findings));
    assert.equal(findings[0].kind, "shift");
    assert.ok(findings[0].values.score > LIMITS.shift);
    assert.ok(findings[0].values.entries.every((entry) => entry.time > interval.end));
    assert.equal(await page.evaluate(() => window.shiftFixture.restEnd), interval.end);
  });
}
test(movementAfterEnlargementRestRemainsAFinding);

async function delayedMovementBeforeStateStepsRemainsAFinding() {
  await shiftPage(`intentional-shifts.html?hold&movement=before&delay=${beforeStepsMs}`, async (page) => {
    assert.equal(await page.evaluate(() => window.shiftFixture.delayMs), beforeStepsMs);
    await page.getByText("Ready for steps").waitFor();
    await page.waitForFunction(() => window.shiftDelivery.pending.length > 0);
    const interval = await withIntentionalShifts(page, () => applySteps(page, [{ action: "wait", text: "Ready for steps" }]));
    await page.evaluate(() => window.shiftDelivery.release());
    const findings = await measureShifts(page);
    assert.equal(findings.length, 1, JSON.stringify(findings));
    assert.equal(findings[0].kind, "shift");
    assert.ok(findings[0].values.score > LIMITS.shift);
    assert.ok(findings[0].values.entries.every((entry) => entry.time < interval.start));
  });
}
test(delayedMovementBeforeStateStepsRemainsAFinding);

async function panelStateStepsExcludeTheirMovement() {
  await shiftPage("shift-panel.html", async (page) => {
    const interval = await withIntentionalShifts(page, () => applySteps(page, panelSteps));
    const entries = await page.evaluate(() => window.__uiCheck.shifts);
    assert.ok(entries.reduce((sum, entry) => sum + entry.value, 0) > LIMITS.shift);
    assert.ok(entries.every((entry) => entry.time >= interval.start && entry.time <= interval.end));
    assert.deepEqual(await measureShifts(page), []);
  });
}
test(panelStateStepsExcludeTheirMovement);

async function quietPageWaitsOnlyForTheNamedFrameCount() {
  assert.equal(SHIFT_QUIET_FRAMES, 3);
  assert.equal(SHIFT_SETTLE_MS, 500);
  await shiftPage("shift-quiet.html", async (page) => {
    assert.deepEqual(await measureShifts(page), []);
    const interval = await withIntentionalShifts(page, () => applySteps(page, [{ action: "wait", text: "Content at rest" }]));
    assert.equal(interval.frames, SHIFT_QUIET_FRAMES);
    assert.equal(interval.quietFrames, SHIFT_QUIET_FRAMES);
    assert.deepEqual(await measureShifts(page), []);
  });
}
test(quietPageWaitsOnlyForTheNamedFrameCount);

async function continuingMovementEndsTheWaitAndKeepsLaterShifts() {
  await shiftPage("intentional-shifts.html?hold", async (page) => {
    const interval = await withIntentionalShifts(page, () => page.evaluate(() => window.shiftFixture.startMotion()));
    assert.ok(interval.quietFrames < SHIFT_QUIET_FRAMES);
    assert.ok(interval.end - interval.start >= SHIFT_SETTLE_MS);
    await page.waitForFunction((end) => window.shiftDelivery.pending.some(({ list }) => list.getEntries().some((entry) => entry.startTime > end)), interval.end);
    await page.evaluate(() => {
      window.shiftFixture.moving = false;
      window.shiftDelivery.release();
    });
    const findings = await measureShifts(page);
    assert.equal(findings.length, 1, JSON.stringify(findings));
    assert.equal(findings[0].kind, "shift");
    assert.ok(findings[0].values.entries.every((entry) => entry.time > interval.end));
  });
}
test(continuingMovementEndsTheWaitAndKeepsLaterShifts);

async function renderReportsPreserveIndependentShiftsAndCleanIntentionalChanges() {
  const surface = join(root, "surface.json");
  await writeFile(surface, JSON.stringify({
    name: "Layout windows", directory: fixtures, address: "http://localhost:0", readyPath: "/shift-quiet.html",
    views: [
      { name: "quiet", path: "/shift-quiet.html", modes: ["default", "double-text"] },
      { name: "enlargement", path: "/intentional-shifts.html", modes: ["default", "double-text"] },
      { name: "after", path: `/intentional-shifts.html?movement=after&delay=${afterRestMs}`, modes: ["default", "double-text"] },
      { name: "panel", path: "/shift-panel.html", modes: ["default"], states: [{ name: "open", steps: panelSteps }] },
    ],
    answers: [], typeScale: [16, 24],
  }));
  const result = await checkSurface({ surface, out: join(root, "renders"), jobs: 4 }, () => {});
  assert.equal(result.passed, false);
  assert.equal(result.renders.length, 122);
  assert.ok(result.renders.every((render) => render.rendered && render.hasContent));
  assert.equal(result.findings.length, 4, JSON.stringify(result.findings));
  for (const render of result.renders) {
    if (render.view === "after" && render.mode === "double-text") {
      assert.equal(render.findings.length, 1);
      assert.equal(render.findings[0].kind, "shift");
      assert.ok(render.findings[0].values.score > LIMITS.shift);
    } else assert.deepEqual(render.findings, []);
  }
  assert.equal(result.renders.filter((render) => render.view === "enlargement" && render.mode === "double-text").length, 4);
  assert.equal(result.renders.filter((render) => render.view === "panel" && render.state === "open").length, 22);
  assert.deepEqual(JSON.parse(await readFile(join(root, "renders/result.json"), "utf8")), result);
}
test(renderReportsPreserveIndependentShiftsAndCleanIntentionalChanges);
