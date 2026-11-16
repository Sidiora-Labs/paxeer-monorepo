#!/usr/bin/env node
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { availableParallelism } from "node:os";
import { createRequire } from "node:module";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parseArgs } from "node:util";
import { chromium } from "playwright";
import { PNG } from "pngjs";
import { DEVICES, LIMITS, MODES, outputDirectory, readSurface, renderMatrix } from "./config.mjs";
import { startSurface } from "./server.mjs";
import { doubleText, focusedElement, focusSnapshot, measureContrast, measurePage, observePage } from "./browser.mjs";

const require = createRequire(import.meta.url);
const axeSource = await readFile(require.resolve("axe-core/axe.min.js"), "utf8");

function screenshotHasContent(screenshot) {
  const { data } = PNG.sync.read(screenshot);
  const colour = data.readUInt32BE(0);
  for (let offset = 4; offset < data.length; offset += 4) {
    if (data.readUInt32BE(offset) !== colour) return true;
  }
  return false;
}

async function applySteps(page, steps) {
  for (const step of steps) {
    if (step.action === "click") await page.locator(step.selector).click();
    if (step.action === "fill") await page.locator(step.selector).fill(step.value);
    if (step.action === "press") await page.locator(step.selector).press(step.key);
    if (step.action === "wait") await page.getByText(step.text, { exact: false }).first().waitFor({ state: "visible" });
  }
}

async function discardIntentionalShifts(page, start) {
  await page.evaluate(async (shiftStart) => {
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    window.__uiCheck.shifts.splice(shiftStart);
  }, start);
}

async function keyboardFindings(page) {
  const expected = await page.evaluate(focusSnapshot);
  const visited = new Set();
  const findings = [];
  let empty = 0;
  for (let index = 0; index < 10000; index++) {
    await page.keyboard.press("Tab");
    await page.waitForTimeout(120);
    const focus = await page.evaluate(focusedElement);
    if (!focus) {
      if (++empty > 1) break;
      continue;
    }
    empty = 0;
    if (visited.has(focus.selector)) break;
    visited.add(focus.selector);
    let pixelsChanged = false;
    if (focus.changed && !focus.outside) {
      const focused = await page.screenshot({ fullPage: false, animations: "allow" });
      await page.evaluate(() => window.__uiCheck.lastFocused.blur());
      const unfocused = await page.screenshot({ fullPage: false, animations: "allow" });
      pixelsChanged = !focused.equals(unfocused);
      await page.evaluate(() => window.__uiCheck.lastFocused.focus({ preventScroll: true }));
    }
    if (!focus.changed || !pixelsChanged || focus.outside) findings.push({ kind: "focus", selector: focus.selector, values: { ...focus.values, visibleChange: focus.changed && pixelsChanged, outside: focus.outside } });
    if (index === 9999) findings.push({ kind: "focus", selector: "document", values: { message: "Keyboard traversal did not finish", visited: visited.size } });
  }
  for (const selector of expected) {
    if (!visited.has(selector)) findings.push({ kind: "focus", selector, values: { message: "Element was not reached by Tab", visited: visited.size } });
  }
  return { findings, visited: [...visited] };
}

async function settlePage(page) {
  await page.evaluate(async (timeout) => {
    await Promise.race([
      document.fonts.ready,
      new Promise((_, reject) => setTimeout(() => reject(new Error("Fonts did not become ready")), timeout)),
    ]);
  }, LIMITS.navigationMs);
  // Visit the full document so lazy content participates in the measurements.
  let bottom = false;
  for (let index = 0; index < 200; index++) {
    bottom = await page.evaluate(() => {
      const end = Math.max(document.documentElement.scrollHeight, document.body.scrollHeight) - innerHeight;
      if (scrollY >= end) return true;
      scrollBy(0, innerHeight);
      return false;
    });
    if (bottom) break;
    await page.waitForTimeout(30);
  }
  if (!bottom) throw new Error("Document did not reach its end during rendering");
  await page.evaluate(() => scrollTo(0, 0));
  await page.waitForTimeout(LIMITS.settleMs);
}

async function withinTime(operation, milliseconds) {
  let timer;
  try {
    return await Promise.race([
      operation.then(() => true),
      new Promise((resolve) => { timer = setTimeout(() => resolve(false), milliseconds); }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

async function closeBrowser(worker, force = false) {
  if (!worker.server) return;
  const server = worker.server;
  worker.server = null;
  worker.browser = null;
  if (force || !await withinTime(server.close(), LIMITS.navigationMs)) await server.kill();
}

async function openBrowser(worker) {
  if (worker.browser) return;
  worker.server = await chromium.launchServer();
  worker.browser = await chromium.connect(worker.server.wsEndpoint());
}

async function renderView(worker, surface, view, state, configuration, out, index, renderMs) {
  const { width, height, scheme, mode } = configuration;
  let context;
  let active = true;
  let step = "navigation";
  let timer;
  const deadline = new Error("Render deadline exceeded");
  const startStep = (name) => {
    if (!active) throw deadline;
    step = name;
  };
  const findings = [];
  const addLoad = (values) => { if (active) findings.push({ kind: "load", selector: "document", values }); };
  const expects = state.expects ?? [];
  const matchedExpectations = new Set();
  const expectedResponses = new WeakSet();
  const failedResponses = [];
  const consoleErrors = [];
  const render = { view: view.name, state: state.name, size: { width, height }, scheme, mode, rendered: false, hasContent: false, screenshot: `${String(index + 1).padStart(5, "0")}.png`, findings: [] };
  let page;
  const work = async () => {
    try {
      context = await worker.browser.newContext({
        viewport: { width, height }, colorScheme: scheme,
        reducedMotion: mode === "reduced-motion" ? "reduce" : "no-preference",
        serviceWorkers: "block", deviceScaleFactor: 1,
        hasTouch: width <= LIMITS.touchWidth,
      });
      if (!active) { await context.close(); return; }
      context.on("close", () => { active = false; });
      await context.grantPermissions(["clipboard-write"], { origin: surface.address });
      await context.addInitScript(observePage);
      await context.route("**/*", async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        const requestFailed = (error) => addLoad({ source: "requestfailed", method: request.method(), address: url.href, resource: request.resourceType(), message: error.message });
        if (!active) return;
        try {
          if (!["http:", "https:"].includes(url.protocol)) { await route.continue(); return; }
          if (url.origin === surface.address) {
            let response;
            try {
              response = await route.fetch({ maxRedirects: 0, timeout: LIMITS.navigationMs });
            } catch (error) {
              requestFailed(error);
              if (active) await route.abort("failed");
              return;
            }
            const location = response.headers()["location"];
            if (response.status() >= 300 && response.status() < 400 && location) {
              const redirected = new URL(location, url);
              if (redirected.origin !== surface.address) {
                const method = [307, 308].includes(response.status()) ? request.method() : "GET";
                const answer = surface.answers.find((entry) => entry.method === method && entry.pattern.test(redirected.href));
                if (answer) { await route.fulfill({ status: answer.status ?? 200, headers: answer.headers, body: answer.body }); return; }
                addLoad({ source: "unanswered-request", method, address: redirected.href, resource: request.resourceType() });
                await route.abort("blockedbyclient");
                return;
              }
            }
            await route.fulfill({ response });
            return;
          }
          const answer = surface.answers.find((entry) => entry.method === request.method() && entry.pattern.test(url.href));
          if (answer) { await route.fulfill({ status: answer.status ?? 200, headers: answer.headers, body: answer.body }); return; }
          addLoad({ source: "unanswered-request", method: request.method(), address: url.href, resource: request.resourceType() });
          await route.abort("blockedbyclient");
        } catch (error) {
          requestFailed(error);
        }
      });
      await context.routeWebSocket("**/*", async (socket) => {
        const url = new URL(socket.url());
        const origin = `${url.protocol === "wss:" ? "https:" : "http:"}//${url.host}`;
        if (!active) return;
        try {
          if (origin === surface.address) socket.connectToServer();
          else {
            addLoad({ source: "unanswered-request", method: "GET", address: url.href, resource: "websocket" });
            await socket.close({ code: 1008, reason: "No fixed answer" });
          }
        } catch (error) {
          addLoad({ source: "requestfailed", method: "GET", address: url.href, resource: "websocket", message: error.message });
        }
      });
      page = await context.newPage();
      page.setDefaultTimeout(LIMITS.navigationMs);
      page.setDefaultNavigationTimeout(LIMITS.navigationMs);
      page.on("console", (message) => {
        if (active && message.type() === "error") consoleErrors.push({ source: "console", message: message.text(), location: message.location() });
      });
      page.on("pageerror", (error) => addLoad({ source: "pageerror", message: error.message }));
      page.on("requestfailed", (request) => addLoad({ source: "requestfailed", method: request.method(), address: request.url(), resource: request.resourceType(), message: request.failure()?.errorText }));
      page.on("response", (response) => {
        if (!active || response.status() < 400) return;
        const matches = expects.filter((entry) => entry.method === response.request().method() && entry.matcher.test(response.url()) && entry.status === response.status());
        for (const entry of matches) matchedExpectations.add(entry);
        if (matches.length) expectedResponses.add(response);
        else addLoad({ source: "response", method: response.request().method(), address: response.url(), status: response.status(), resource: response.request().resourceType() });
        failedResponses.push({ address: response.url(), status: response.status(), expected: matches.length > 0 });
      });
      const response = await page.goto(new URL(view.path, surface.address).href, { waitUntil: "load" });
      if (!response || (!response.ok() && !expectedResponses.has(response))) throw new Error(`View did not answer successfully: ${response?.status() ?? "no response"}`);
      if (new URL(page.url()).origin !== surface.address) throw new Error("View navigated outside the surface");
      if (state.steps.length) {
        const shiftStart = await page.evaluate(() => window.__uiCheck.shifts.length);
        await applySteps(page, state.steps);
        await discardIntentionalShifts(page, shiftStart);
      }
      if (mode === "double-text") {
        const shiftStart = await page.evaluate(() => window.__uiCheck.shifts.length);
        await page.evaluate(doubleText);
        await discardIntentionalShifts(page, shiftStart);
      }
      startStep("settle");
      await settlePage(page);
      startStep("measurement");
      const measurements = await page.evaluate(measurePage, { limits: LIMITS, typeScale: surface.typeScale, doubled: mode === "double-text", reduced: mode === "reduced-motion" });
      if (!active) return;
      findings.push(...measurements);
      startStep("screenshot");
      const viewportHasContent = screenshotHasContent(await page.screenshot({ fullPage: false, timeout: LIMITS.navigationMs, animations: "allow" }));
      if (!active) return;
      if (!viewportHasContent) addLoad({ source: "single-colour-viewport", message: "Viewport screenshot contains a single colour" });
      render.hasContent = viewportHasContent && !measurements.some((finding) => finding.kind === "load" && finding.values.source === "empty-document");
      startStep("contrast");
      await page.evaluate(axeSource);
      const contrast = await page.evaluate(measureContrast, LIMITS);
      if (!active) return;
      findings.push(...contrast);
      if (mode === "keyboard") {
        startStep("keyboard");
        await page.bringToFront();
        const keyboard = await keyboardFindings(page);
        if (!active) return;
        findings.push(...keyboard.findings);
        render.focused = keyboard.visited;
      }
      render.rendered = true;
    } catch (error) {
      if (!active) return;
      addLoad({ source: "render", message: error.message });
      render.rendered = false;
    }
    if (page && active) {
      startStep("screenshot");
      try {
        await page.screenshot({ path: join(out, render.screenshot), fullPage: true, timeout: LIMITS.navigationMs, animations: "allow" });
      } catch (error) {
        if (!active) return;
        addLoad({ source: "screenshot", message: error.message });
        render.screenshot = null;
        render.rendered = false;
      }
    }
  };
  try {
    await Promise.race([
      work(),
      new Promise((_, reject) => { timer = setTimeout(() => reject(deadline), renderMs); }),
    ]);
  } catch (error) {
    if (error !== deadline) throw error;
    addLoad({ source: "deadline", step, milliseconds: renderMs });
    render.rendered = false;
    render.screenshot = null;
  } finally {
    clearTimeout(timer);
    active = false;
    if (context && !await withinTime(context.close(), LIMITS.navigationMs)) await closeBrowser(worker, true);
    for (const error of consoleErrors) {
      const status = error.message.match(/^Failed to load resource: the server responded with a status of (\d{3})(?: \(.*\))?$/);
      const responses = status ? failedResponses.filter((response) => response.address === error.location.url && response.status === Number(status[1])) : [];
      if (!responses.length || !responses.every((response) => response.expected) || error.location.lineNumber !== 0 || error.location.columnNumber !== 0) findings.push({ kind: "load", selector: "document", values: error });
    }
    for (const entry of expects) {
      if (!matchedExpectations.has(entry)) findings.push({ kind: "load", selector: "document", values: { source: "unmatched-expectation", method: entry.method, pattern: entry.pattern, status: entry.status } });
    }
  }
  const completed = { ...render };
  completed.findings = findings.map((finding) => ({ view: view.name, state: state.name, size: render.size, scheme, mode, ...finding }));
  return completed;
}

function jobCount(value = Math.min(availableParallelism(), 4)) {
  if (!["number", "string"].includes(typeof value) || (typeof value === "string" && !/^[1-9]\d*$/.test(value)) || !Number.isSafeInteger(Number(value)) || Number(value) < 1) {
    throw new Error("jobs must be a positive whole number");
  }
  return Number(value);
}

export async function checkSurface({ surface: filename, out: destination, view: selected, jobs, renderMs = LIMITS.renderMs }, print = console.log) {
  const count = jobCount(jobs);
  if (!Number.isSafeInteger(renderMs) || renderMs <= 0 || renderMs > LIMITS.renderMs) throw new Error("renderMs must be a positive whole number within the render limit");
  const out = await outputDirectory(destination);
  const surface = await readSurface(filename);
  const views = selected ? surface.views.filter((view) => view.name === selected) : surface.views;
  if (!views.length) throw new Error(`Unknown view: ${selected}`);
  await mkdir(out, { recursive: true });
  const summaries = views.map((view) => ({ name: view.name, modes: view.modes, states: (view.states ?? []).map((state) => ({ name: state.name, modes: state.modes })) }));
  const reducedModeCount = summaries.reduce((total, view) => total + Number(view.modes.length < MODES.length) + view.states.filter((state) => state.modes.length < MODES.length).length, 0);
  const result = { surface: surface.name, buildMs: surface.buildMs ?? LIMITS.buildMs, outputs: [], devices: DEVICES, limits: LIMITS, views: summaries, reducedModeCount, renders: [], findings: [], passed: false };
  const tasks = views.flatMap((view) => [{ name: "default", steps: [], expects: view.expects, modes: view.modes }, ...(view.states ?? [])].flatMap((state) => renderMatrix(state.modes).map((configuration) => ({ view, state, configuration }))));
  const workers = Array.from({ length: Math.min(count, tasks.length) }, () => ({}));
  let next = 0;
  let service;
  try {
    service = await startSurface(surface, result.outputs);
    surface.address = service.address;
    const outcomes = await Promise.allSettled(workers.map(async (worker) => {
      try {
        while (next < tasks.length) {
          const index = next++;
          const { view, state, configuration } = tasks[index];
          await openBrowser(worker);
          result.renders[index] = await renderView(worker, surface, view, state, configuration, out, index, renderMs);
        }
      } finally {
        await closeBrowser(worker);
      }
    }));
    const failed = outcomes.find((outcome) => outcome.status === "rejected");
    if (failed) throw failed.reason;
    for (const view of views) {
      const renders = result.renders.filter((render) => render.view === view.name);
      print(`${view.name}: ${renders.length} renders, ${renders.filter((render) => !render.rendered).length} did not render, ${renders.filter((render) => !render.hasContent).length} showed nothing, ${renders.reduce((total, render) => total + render.findings.length, 0)} findings`);
    }
  } catch (error) {
    result.findings.push({ kind: "load", view: null, state: null, size: null, scheme: null, mode: null, selector: "document", values: { source: "surface", message: error.message } });
  } finally {
    await service?.close();
  }
  result.renders = result.renders.filter(Boolean);
  result.findings.push(...result.renders.flatMap((render) => render.findings));
  result.passed = result.renders.length === tasks.length && result.renders.every((render) => render.rendered) && result.findings.length === 0;
  await writeFile(join(out, "result.json"), `${JSON.stringify(result, null, 2)}\n`);
  print(`Total: ${result.renders.length} renders, ${result.renders.filter((render) => !render.hasContent).length} showed nothing, ${result.findings.length} findings, ${result.passed ? "pass" : "fail"}, ${reducedModeCount} views and states with fewer than all modes`);
  return result;
}

export async function main(args = process.argv.slice(2)) {
  const { values } = parseArgs({ args, options: { surface: { type: "string" }, out: { type: "string" }, view: { type: "string" }, list: { type: "boolean" }, jobs: { type: "string" } }, allowPositionals: false });
  if (values.jobs !== undefined) jobCount(values.jobs);
  if (values.list) {
    for (const device of DEVICES) console.log(`${device.width} x ${device.height}`);
    return 0;
  }
  if (!values.surface || !values.out) throw new Error("Usage: node tools/ui-check/check.mjs --surface FILE --out DIRECTORY [--view NAME] [--jobs COUNT] | --list");
  const result = await checkSurface(values);
  return result.passed ? 0 : 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().then((code) => { process.exitCode = code; }, (error) => { console.error(error.message); process.exitCode = 1; });
}
