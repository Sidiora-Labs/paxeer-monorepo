#!/usr/bin/env node
import { lstat, mkdir, readFile, readlink, realpath, stat, writeFile } from "node:fs/promises";
import { availableParallelism } from "node:os";
import { createRequire } from "node:module";
import { dirname, isAbsolute, join, resolve, sep, win32 } from "node:path";
import { pathToFileURL } from "node:url";
import { parseArgs } from "node:util";
import { chromium } from "playwright";
import { PNG } from "pngjs";
import { addressPattern, DEVICES, FINDING_KINDS, GEOMETRY_WIDTHS, LIMITS, MODES, outputDirectory, renderMatrix, surfaceOutput, within } from "./config.mjs";
import { startSurface } from "./server.mjs";
import { beginIntentionalShifts, doubleText, endIntentionalShifts, focusedElement, focusSnapshot, measureContrast, measurePage, observePage } from "./browser.mjs";
import { geometryFindings, measureParts, readGeometry, writeGeometry } from "./geometry.mjs";

const require = createRequire(import.meta.url);
const axeSource = await readFile(require.resolve("axe-core/axe.min.js"), "utf8");

function object(value, keys, label) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  for (const key of Object.keys(value)) {
    if (!keys.includes(key)) throw new Error(`${label}: unknown field ${key}`);
  }
}
function text(value, label) {
  if (typeof value !== "string" || !value.trim()) throw new Error(`${label} must be a nonempty string`);
}
function array(value, label, nonempty = false) {
  if (!Array.isArray(value) || (nonempty && !value.length)) throw new Error(`${label} must be an array${nonempty ? " with entries" : ""}`);
}
function localPath(value, label) {
  text(value, label);
  if (!value.startsWith("/") || value.startsWith("//") || value.includes("\\")) {
    throw new Error(`${label} must be an origin-relative path`);
  }
}
function uniqueNames(values, label) {
  const names = values.map((value) => value.name);
  if (new Set(names).size !== names.length) throw new Error(`${label} names must be unique`);
}
async function canonicalDestination(path) {
  try {
    return await realpath(path);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    try {
      if ((await lstat(path)).isSymbolicLink()) {
        return canonicalDestination(resolve(dirname(path), await readlink(path)));
      }
    } catch (entryError) {
      if (entryError.code !== "ENOENT") throw entryError;
    }
    const parent = dirname(path);
    if (parent === path) throw error;
    return join(await canonicalDestination(parent), path.slice(parent.length + (parent.endsWith(sep) ? 0 : 1)));
  }
}

function readEmbeddedMessages(value) {
  if (value === undefined) return [];
  array(value, "surface.embeddedMessages");
  return value.map((entry, index) => {
    const label = `surface.embeddedMessages[${index}]`;
    object(entry, ["script", "message"], label);
    localPath(entry.script, `${label}.script`);
    const path = new URL(entry.script, "http://localhost");
    if (path.pathname !== entry.script || !/^\/[^/]+\//.test(entry.script) || entry.script.split("/").slice(1, -1).some((part) => !part || [".", ".."].includes(part))) {
      throw new Error(`${label}.script must be a path with at least one directory`);
    }
    text(entry.message, `${label}.message`);
    return { script: entry.script, message: entry.message };
  });
}

function expectations(value, label) {
  if (value === undefined) return [];
  array(value, label);
  return value.map((entry) => {
    object(entry, ["method", "pattern", "status"], `${label} entry`);
    text(entry.method, `${label}.method`);
    if (!/^[A-Z]+$/.test(entry.method)) throw new Error(`${label}.method must be uppercase`);
    const matcher = addressPattern(entry.pattern);
    if (!Number.isInteger(entry.status) || entry.status < 400 || entry.status > 599) {
      throw new Error(`${label}.status must be an HTTP status from 400 to 599`);
    }
    return { ...entry, matcher };
  });
}

function readModes(value, inherited, label) {
  if (value === undefined) return [...inherited];
  array(value, label, true);
  if (!value.includes("default") || new Set(value).size !== value.length || value.some((mode) => !MODES.includes(mode))) {
    throw new Error(`${label} must contain default and unique names from ${MODES.join(", ")}`);
  }
  return MODES.filter((mode) => value.includes(mode));
}

async function readAnswers(value, label, base, required = false) {
  if (value === undefined) {
    if (required) array(value, label);
    return [];
  }
  array(value, label);
  const answers = [];
  for (const answer of value) {
    const fieldLabel = label === "surface.answers" ? "answer" : `${label} entry`;
    object(answer, ["method", "pattern", "file", "status", "headers"], fieldLabel);
    text(answer.method, `${fieldLabel}.method`);
    if (!/^[A-Z]+$/.test(answer.method)) throw new Error(`${fieldLabel}.method must be uppercase`);
    const pattern = addressPattern(answer.pattern, `${fieldLabel}.pattern`);
    text(answer.file, `${fieldLabel}.file`);
    const path = await realpath(resolve(base, answer.file));
    if (!within(base, path)) {
      const message = "Answer files must be beside the surface file or in its subdirectories";
      throw new Error(label === "surface.answers" ? message : `${label}: ${message}`);
    }
    if (answer.status !== undefined && (!Number.isInteger(answer.status) || answer.status < 200 || answer.status > 599)) {
      throw new Error(`${fieldLabel}.status must be an HTTP status from 200 to 599`);
    }
    if (answer.headers !== undefined) {
      if (!answer.headers || typeof answer.headers !== "object" || Array.isArray(answer.headers) || Object.values(answer.headers).some((v) => typeof v !== "string")) {
        throw new Error(`${fieldLabel}.headers must contain string values`);
      }
    }
    answers.push({ ...answer, pattern, body: await readFile(path) });
  }
  return answers;
}

export async function readSurface(filename) {
  const file = await realpath(resolve(filename));
  const base = dirname(file);
  const value = JSON.parse(await readFile(file, "utf8"));
  object(value, ["name", "directory", "build", "buildMs", "outputs", "start", "address", "readyPath", "views", "answers", "typeScale", "geometry", "reportedKinds", "embeddedMessages"], "surface");
  const embeddedMessages = readEmbeddedMessages(value.embeddedMessages);
  text(value.name, "surface.name");
  if (value.reportedKinds !== undefined) {
    array(value.reportedKinds, "surface.reportedKinds");
    if (new Set(value.reportedKinds).size !== value.reportedKinds.length || value.reportedKinds.some((kind) => !FINDING_KINDS.includes(kind) || ["load", "geometry"].includes(kind))) {
      throw new Error("surface.reportedKinds must contain unique known kinds other than load and geometry");
    }
  }
  if (value.geometry !== undefined) {
    object(value.geometry, ["file", "widths"], "surface.geometry");
    text(value.geometry.file, "surface.geometry.file");
    if (isAbsolute(value.geometry.file) || win32.isAbsolute(value.geometry.file)) {
      throw new Error("surface.geometry.file must be relative to the surface file");
    }
    const destination = await canonicalDestination(resolve(base, value.geometry.file));
    if (destination === file) throw new Error("surface.geometry.file must differ from the surface file");
    const widths = value.geometry.widths === undefined ? [...GEOMETRY_WIDTHS] : value.geometry.widths;
    array(widths, "surface.geometry.widths", true);
    if (new Set(widths).size !== widths.length || widths.some((width) => !Number.isSafeInteger(width) || width <= 0)) {
      throw new Error("surface.geometry.widths must contain unique positive whole pixel widths");
    }
    value.geometry = { file: destination, widths: [...widths].sort((a, b) => a - b) };
  }
  text(value.directory, "surface.directory");
  const directory = await realpath(resolve(base, value.directory));
  if (!(await stat(directory)).isDirectory()) throw new Error("surface.directory must be a directory");
  for (const key of ["build", "start"]) {
    if (value[key] !== undefined) text(value[key], `surface.${key}`);
  }
  if (value.buildMs !== undefined && (!Number.isInteger(value.buildMs) || value.buildMs < 1000 || value.buildMs > 1800000)) {
    throw new Error(`Surface ${value.name} buildMs must be a whole number from 1000 to 1800000`);
  }
  if (value.outputs !== undefined) {
    const label = `Surface ${value.name} outputs`;
    if (!value.build) throw new Error(`${label} requires a build command`);
    if (!Array.isArray(value.outputs) || value.outputs.length < 1 || value.outputs.length > 16) {
      throw new Error(`${label} must be a list of one to sixteen paths`);
    }
    for (const path of value.outputs) await surfaceOutput(directory, path, value.name);
  }
  text(value.address, "surface.address");
  const address = new URL(value.address);
  if (!["http:", "https:"].includes(address.protocol) || address.username || address.password || address.pathname !== "/" || address.search || address.hash) {
    throw new Error("surface.address must be an HTTP origin");
  }
  if (!value.start && (address.protocol !== "http:" || !["localhost", "127.0.0.1", "[::1]"].includes(address.hostname))) {
    throw new Error("The file server requires a loopback HTTP address");
  }
  localPath(value.readyPath, "surface.readyPath");
  array(value.views, "surface.views", true);
  for (const view of value.views) {
    object(view, ["name", "path", "states", "expects", "answers", "modes", "parts"], "view");
    text(view.name, "view.name");
    if (view.parts !== undefined) {
      array(view.parts, `View ${view.name} parts`);
      for (const part of view.parts) {
        object(part, ["name", "selector"], `View ${view.name} part`);
        text(part.name, `View ${view.name} part.name`);
        text(part.selector, `View ${view.name} part.selector`);
      }
      uniqueNames(view.parts, `View ${view.name} part`);
    }
    localPath(view.path, "view.path");
    view.modes = readModes(view.modes, MODES, `View ${view.name} modes`);
    view.expects = expectations(view.expects, "view.expects");
    view.answers = await readAnswers(view.answers, `View ${view.name} answers`, base);
    if (view.states !== undefined) {
      array(view.states, "view.states");
      for (const state of view.states) {
        object(state, ["name", "steps", "expects", "answers", "modes"], "state");
        text(state.name, "state.name");
        state.modes = readModes(state.modes, view.modes, `View ${view.name} state ${state.name} modes`);
        if (state.name === "default") throw new Error("State name default is reserved");
        state.expects = expectations(state.expects, "state.expects");
        state.answers = await readAnswers(state.answers, `View ${view.name} state ${state.name} answers`, base);
        array(state.steps, "state.steps", true);
        for (const step of state.steps) {
          const keys = {
            click: ["action", "selector"], fill: ["action", "selector", "value"],
            press: ["action", "selector", "key"], wait: ["action", "text"],
          }[step?.action];
          if (!keys) throw new Error("State action must be click, fill, press or wait");
          object(step, keys, "step");
          for (const key of keys.filter((key) => key !== "action")) {
            if (key === "value" && step[key] === "") continue;
            text(step[key], `step.${key}`);
          }
        }
      }
      uniqueNames(view.states, "State");
    }
  }
  uniqueNames(value.views, "View");
  array(value.typeScale, "surface.typeScale", true);
  for (const step of value.typeScale) {
    if (typeof step === "number" && Number.isFinite(step) && step > 0) continue;
    object(step, ["min", "max"], "typeScale step");
    if (!Number.isFinite(step.min) || !Number.isFinite(step.max) || step.min <= 0 || step.max < step.min) {
      throw new Error("A fluid type step requires positive min and max CSS pixels");
    }
  }
  const answers = await readAnswers(value.answers, "surface.answers", base, true);
  return { ...value, directory, address: address.origin, answers, embeddedMessages };
}

function screenshotHasContent(screenshot) {
  const { data } = PNG.sync.read(screenshot);
  const colour = data.readUInt32BE(0);
  for (let offset = 4; offset < data.length; offset += 4) {
    if (data.readUInt32BE(offset) !== colour) return true;
  }
  return false;
}

export const FULL_PAGE_CAPTURE_MAX_MS = 60000;

export async function captureFullPage(page, render, out, { navigationMs = LIMITS.navigationMs, maxMs = FULL_PAGE_CAPTURE_MAX_MS } = {}, isActive = () => !page.isClosed()) {
  let height = null;
  let milliseconds = navigationMs;
  try {
    const dimensions = await page.waitForFunction(() => JSON.stringify({
      height: Math.max(document.documentElement.scrollHeight, document.body?.scrollHeight ?? 0, innerHeight),
      viewportHeight: innerHeight,
    }), undefined, { timeout: navigationMs });
    let document;
    try {
      document = JSON.parse(await dimensions.jsonValue());
    } finally {
      await dimensions.dispose();
    }
    height = document.height;
    milliseconds = Math.max(navigationMs, Math.min(maxMs, navigationMs * Math.ceil(height / document.viewportHeight)));
    await page.screenshot({ path: join(out, render.screenshot), fullPage: true, timeout: milliseconds, animations: "allow" });
  } catch (error) {
    if (!isActive()) return;
    render.findings.push({ kind: "load", selector: "document", values: { source: "screenshot", height, milliseconds, message: error.message } });
    render.screenshot = null;
    render.rendered = false;
  }
}

export async function applySteps(page, steps) {
  for (const step of steps) {
    if (step.action === "click") await page.locator(step.selector).click();
    if (step.action === "fill") await page.locator(step.selector).fill(step.value);
    if (step.action === "press") await page.locator(step.selector).press(step.key);
    if (step.action === "wait") await page.getByText(step.text, { exact: false }).first().waitFor({ state: "visible" });
  }
}

export const SHIFT_QUIET_FRAMES = 3;
export const SHIFT_SETTLE_MS = 500;

export async function withIntentionalShifts(page, action) {
  await page.evaluate(beginIntentionalShifts);
  await action();
  return page.evaluate(endIntentionalShifts, { quietFrames: SHIFT_QUIET_FRAMES, timeoutMs: SHIFT_SETTLE_MS });
}

export const FRAME_TAB_PRESSES = 32;
export const FRAME_TRAVERSAL_MS = 2000;

async function passFrame(page, selector) {
  let active = true;
  let timer;
  let presses = 0;
  const work = async () => {
    while (active && presses < FRAME_TAB_PRESSES) {
      presses++;
      await page.keyboard.press("Tab");
      if (!active) return;
      let focus = await page.evaluate(focusedElement);
      if (!active) return;
      if (!focus?.frame || focus.selector !== selector) {
        await page.waitForTimeout(120);
        if (!active) return;
        focus = await page.evaluate(focusedElement);
        return { left: !focus?.frame || focus.selector !== selector, focus, presses };
      }
    }
    return { left: false, presses };
  };
  try {
    return await Promise.race([
      work(),
      new Promise((resolve) => { timer = setTimeout(() => resolve({ left: false, presses }), FRAME_TRAVERSAL_MS); }),
    ]);
  } finally {
    active = false;
    clearTimeout(timer);
  }
}

export async function keyboardFindings(page) {
  const expected = await page.evaluate(focusSnapshot);
  const visited = new Set();
  const findings = [];
  let empty = 0;
  traversal: for (let index = 0; index < 10000; index++) {
    await page.keyboard.press("Tab");
    await page.waitForTimeout(120);
    let focus = await page.evaluate(focusedElement);
    while (focus?.frame) {
      if (visited.has(focus.selector)) break traversal;
      visited.add(focus.selector);
      const frame = await passFrame(page, focus.selector);
      if (!frame.left) {
        findings.push({ kind: "focus", selector: focus.selector, values: { message: "Keyboard traversal did not leave the frame", presses: frame.presses, maximumPresses: FRAME_TAB_PRESSES, maximumMilliseconds: FRAME_TRAVERSAL_MS } });
        return { findings, visited: [...visited] };
      }
      focus = frame.focus;
    }
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

async function renderView(worker, surface, view, state, configuration, out, index, renderMs, geometry) {
  const { width, height, scheme, mode } = configuration;
  let context;
  let active = true;
  let step = "navigation";
  let deadlineStep = null;
  let timer;
  const deadline = new Error("Render deadline exceeded");
  const startStep = (name) => {
    if (!active) throw deadline;
    step = name;
  };
  const findings = [];
  const retryCauses = new WeakMap();
  const addLoad = (values, cause = null) => {
    if (!active) return;
    const finding = { kind: "load", selector: "document", values };
    findings.push(finding);
    if (cause) retryCauses.set(finding, cause);
  };
  let navigationPending = false;
  let navigationAnswered = false;
  let navigationFailure = null;
  const expects = state.expects ?? [];
  const answers = [...(state.answers ?? []), ...(view.answers ?? []), ...surface.answers];
  const matchedExpectations = new Set();
  const expectedResponses = new WeakSet();
  const endedRequests = new WeakSet();
  const endedFailures = [];
  const failedResponses = [];
  const consoleErrors = [];
  const render = { view: view.name, state: state.name, size: { width, height }, scheme, mode, rendered: false, hasContent: false, cancelled: 0, retried: false, firstDeadlineStep: null, firstLoadFailure: null, embeddedMessages: [], screenshot: out ? `${String(index + 1).padStart(5, "0")}.png` : null, findings };
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
      await context.grantPermissions(["clipboard-write", "local-network-access"], { origin: surface.address });
      await context.addInitScript(observePage);
      await context.route("**/*", async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        const requestFailed = (error, beforeResponse = false) => {
          const navigation = navigationPending && !navigationAnswered && request.isNavigationRequest() && request.frame() === page.mainFrame();
          let cause = null;
          if (beforeResponse && url.origin === surface.address) {
            if (/\b(?:ECONNRESET|ERR_CONNECTION_CLOSED|ERR_CONNECTION_RESET|ERR_EMPTY_RESPONSE)\b|socket hang up/.test(error.message)) cause = "connection-closed";
            else if (navigation && (error.name === "TimeoutError" || /\bTimeout \d+ms exceeded\b/.test(error.message))) cause = "navigation-timeout";
          }
          if (navigation && cause) navigationFailure = cause;
          addLoad({ source: "requestfailed", method: request.method(), address: url.href, resource: request.resourceType(), message: error.message }, cause);
        };
        if (!active) return;
        try {
          if (!["http:", "https:"].includes(url.protocol)) { await route.continue(); return; }
          if (url.origin === surface.address) {
            let response;
            try {
              response = await route.fetch({ maxRedirects: 0, timeout: LIMITS.navigationMs });
            } catch (error) {
              requestFailed(error, true);
              if (active) {
                endedRequests.add(request);
                await route.abort("failed");
              }
              return;
            }
            const location = response.headers()["location"];
            if (response.status() >= 300 && response.status() < 400 && location) {
              const redirected = new URL(location, url);
              if (redirected.origin !== surface.address) {
                const method = [307, 308].includes(response.status()) ? request.method() : "GET";
                const answer = answers.find((entry) => entry.method === method && entry.pattern.test(redirected.href));
                if (answer) { await route.fulfill({ status: answer.status ?? 200, headers: answer.headers, body: answer.body }); return; }
                addLoad({ source: "unanswered-request", method, address: redirected.href, resource: request.resourceType() });
                endedRequests.add(request);
                await route.abort("blockedbyclient");
                return;
              }
            }
            await route.fulfill({ response });
            return;
          }
          const answer = answers.find((entry) => entry.method === request.method() && entry.pattern.test(url.href));
          if (answer) { await route.fulfill({ status: answer.status ?? 200, headers: answer.headers, body: answer.body }); return; }
          addLoad({ source: "unanswered-request", method: request.method(), address: url.href, resource: request.resourceType() });
          endedRequests.add(request);
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
      page.on("requestfailed", (request) => {
        if (!active) return;
        const message = request.failure()?.errorText;
        if (endedRequests.has(request)) {
          endedFailures.push({ address: request.url(), message });
          return;
        }
        if (message === "net::ERR_ABORTED" && !request.isNavigationRequest()) {
          render.cancelled++;
          return;
        }
        addLoad({ source: "requestfailed", method: request.method(), address: request.url(), resource: request.resourceType(), message });
      });
      page.on("response", (response) => {
        if (navigationPending && response.request().isNavigationRequest() && response.request().frame() === page.mainFrame()) navigationAnswered = true;
        if (!active || response.status() < 400) return;
        const matches = expects.filter((entry) => entry.method === response.request().method() && entry.matcher.test(response.url()) && entry.status === response.status());
        for (const entry of matches) matchedExpectations.add(entry);
        if (matches.length) expectedResponses.add(response);
        else addLoad({ source: "response", method: response.request().method(), address: response.url(), status: response.status(), resource: response.request().resourceType() });
        failedResponses.push({ address: response.url(), status: response.status(), expected: matches.length > 0 });
      });
      navigationPending = true;
      const response = await page.goto(new URL(view.path, surface.address).href, { waitUntil: "load" });
      navigationPending = false;
      if (!response || (!response.ok() && !expectedResponses.has(response))) throw new Error(`View did not answer successfully: ${response?.status() ?? "no response"}`);
      if (new URL(page.url()).origin !== surface.address) throw new Error("View navigated outside the surface");
      if (state.steps.length) {
        await withIntentionalShifts(page, () => applySteps(page, state.steps));
      }
      if (mode === "double-text") {
        await withIntentionalShifts(page, () => page.evaluate(doubleText));
      }
      startStep("settle");
      await settlePage(page);
      startStep("measurement");
      const measurements = await page.evaluate(measurePage, { limits: LIMITS, typeScale: surface.typeScale, doubled: mode === "double-text", reduced: mode === "reduced-motion" });
      if (!active) return;
      findings.push(...measurements.filter((finding) => !geometry?.writing || finding.kind === "load"));
      if (geometry) {
        startStep("geometry");
        render.geometry = await measureParts(page, geometry.parts);
        if (!active) return;
        findings.push(...geometryFindings(render.geometry, geometry.stored, geometry.writing));
      }
      startStep("screenshot");
      const viewportHasContent = screenshotHasContent(await page.screenshot({ fullPage: false, timeout: LIMITS.navigationMs, animations: "allow" }));
      if (!active) return;
      if (!viewportHasContent) addLoad({ source: "single-colour-viewport", message: "Viewport screenshot contains a single colour" });
      render.hasContent = viewportHasContent && !measurements.some((finding) => finding.kind === "load" && finding.values.source === "empty-document");
      if (!geometry?.writing) {
        startStep("contrast");
        await page.evaluate(axeSource);
        const contrast = await page.evaluate(measureContrast, LIMITS);
        if (!active) return;
        findings.push(...contrast);
      }
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
      const cause = navigationPending && !navigationAnswered
        ? (error.name === "TimeoutError" ? "navigation-timeout" : navigationFailure) : null;
      addLoad({ source: "render", message: error.message }, cause);
      render.rendered = false;
    }
    if (page && active && out) {
      startStep("screenshot");
      await captureFullPage(page, render, out, undefined, () => active);
    }
  };
  try {
    await Promise.race([
      work(),
      new Promise((_, reject) => { timer = setTimeout(() => reject(deadline), renderMs); }),
    ]);
  } catch (error) {
    if (error !== deadline) throw error;
    deadlineStep = step;
    addLoad({ source: "deadline", step, milliseconds: renderMs });
    render.rendered = false;
    render.screenshot = null;
  } finally {
    clearTimeout(timer);
    active = false;
    if (deadlineStep) await closeBrowser(worker, true);
    else if (context && !await withinTime(context.close(), LIMITS.navigationMs)) await closeBrowser(worker, true);
    for (const error of consoleErrors) {
      let location;
      try { location = new URL(error.location.url); } catch { /* Console locations may be absent. */ }
      const declared = surface.embeddedMessages.find((entry) => location?.origin === surface.address && location.pathname.startsWith(entry.script) && error.message === entry.message);
      if (declared) {
        const recorded = render.embeddedMessages.find((entry) => entry.script === declared.script && entry.message === declared.message);
        if (recorded) recorded.count++;
        else render.embeddedMessages.push({ ...declared, count: 1 });
        continue;
      }
      if (error.location.lineNumber === 0 && error.location.columnNumber === 0 && endedFailures.some((failure) => failure.address === error.location.url && error.message === `Failed to load resource: ${failure.message}`)) continue;
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
  const loadRetryCause = findings.length && findings.every((finding) => finding.kind === "load" && retryCauses.has(finding))
    ? { cause: retryCauses.get(findings[0]), ...findings[0].values } : null;
  return { render: completed, loadRetryCause };
}

function jobCount(value = Math.min(availableParallelism(), 4)) {
  if (!["number", "string"].includes(typeof value) || (typeof value === "string" && !/^[1-9]\d*$/.test(value)) || !Number.isSafeInteger(Number(value)) || Number(value) < 1) {
    throw new Error("jobs must be a positive whole number");
  }
  return Number(value);
}

export async function checkSurface({ surface: filename, out: destination, view: selected, jobs, renderMs = LIMITS.renderMs, writeGeometry: writing = false }, print = console.log) {
  const count = jobCount(jobs);
  if (!Number.isSafeInteger(renderMs) || renderMs <= 0 || renderMs > LIMITS.renderMs) throw new Error("renderMs must be a positive whole number within the render limit");
  const surface = await readSurface(filename);
  if (writing && !surface.geometry) throw new Error("--write-geometry requires surface.geometry");
  const geometry = surface.geometry ? await readGeometry(surface.geometry.file, writing) : null;
  const out = writing ? null : await outputDirectory(destination);
  const views = selected ? surface.views.filter((view) => view.name === selected) : surface.views;
  if (!views.length) throw new Error(`Unknown view: ${selected}`);
  if (writing && !views.some((view) => view.parts?.length)) throw new Error("--write-geometry requires a selected view with parts");
  if (out) await mkdir(out, { recursive: true });
  const summaries = views.map((view) => ({
    name: view.name,
    modes: view.modes,
    answers: view.answers.length,
    states: (view.states ?? []).map((state) => ({ name: state.name, modes: state.modes, answers: state.answers.length })),
  }));
  const reducedModeCount = summaries.reduce((total, view) => total + Number(view.modes.length < MODES.length) + view.states.filter((state) => state.modes.length < MODES.length).length, 0);
  const result = { surface: surface.name, buildMs: surface.buildMs ?? LIMITS.buildMs, outputs: [], devices: DEVICES, limits: LIMITS, views: summaries, reducedModeCount, retriedCount: 0, renders: [], findings: [], passed: false };
  const tasks = views.flatMap((view) => {
    const initial = { name: "default", steps: [], expects: view.expects, modes: view.modes };
    const entries = geometry?.entries.filter((entry) => entry.view === view.name) ?? [];
    const tracksGeometry = surface.geometry && (view.parts?.length || entries.length);
    const configurations = tracksGeometry ? surface.geometry.widths.map((width) => ({ width, height: 900, scheme: "dark", mode: "default" })) : [];
    if (writing && !view.parts?.length) return [];
    return (writing ? [initial] : [initial, ...(view.states ?? [])]).flatMap((state) => {
      const matrix = writing ? configurations : renderMatrix(state.modes);
      if (!writing && state === initial) {
        for (const configuration of configurations) {
          if (!matrix.some((entry) => entry.width === configuration.width && entry.height === configuration.height && entry.scheme === configuration.scheme && entry.mode === configuration.mode)) matrix.push(configuration);
        }
      }
      return matrix.map((configuration) => {
        let plan;
        if (state === initial && configuration.mode === "default" && configuration.scheme === "dark" && configuration.height === 900 && configurations.some((entry) => entry.width === configuration.width)) {
          const stored = entries.filter((entry) => entry.width === configuration.width);
          const parts = new Map((writing ? [] : stored).map((entry) => [entry.part, { name: entry.part, selector: entry.selector }]));
          for (const part of view.parts ?? []) parts.set(part.name, part);
          plan = { stored, parts: [...parts.values()], writing };
        }
        return { view, state, configuration, geometry: plan };
      });
    });
  });
  const reported = (findings) => findings.filter((finding) => surface.reportedKinds?.includes(finding.kind)).length;
  const counts = (findings) => surface.reportedKinds === undefined ? "" : `, ${reported(findings)} reported, ${findings.length - reported(findings)} failing`;
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
          const { view, state, configuration, geometry: plan } = tasks[index];
          await openBrowser(worker);
          const first = await renderView(worker, surface, view, state, configuration, out, index, renderMs, plan);
          let render = first.render;
          const deadline = render.findings.find((finding) => finding.values.source === "deadline");
          if (deadline || first.loadRetryCause) {
            await closeBrowser(worker, true);
            await openBrowser(worker);
            ({ render } = await renderView(worker, surface, view, state, configuration, out, index, renderMs, plan));
            render.retried = true;
            render.firstDeadlineStep = deadline?.values.step ?? null;
            render.firstLoadFailure = first.loadRetryCause;
          }
          result.renders[index] = render;
        }
      } finally {
        await closeBrowser(worker);
      }
    }));
    const failed = outcomes.find((outcome) => outcome.status === "rejected");
    if (failed) throw failed.reason;
    for (const view of views) {
      const renders = result.renders.filter((render) => render.view === view.name);
      const findings = renders.flatMap((render) => render.findings);
      const embeddedCount = renders.reduce((total, render) => total + render.embeddedMessages.reduce((sum, entry) => sum + entry.count, 0), 0);
      const embeddedSummary = embeddedCount ? `, ${embeddedCount} declared messages` : "";
      print(`${view.name}: ${renders.length} renders, ${renders.filter((render) => !render.rendered).length} did not render, ${renders.filter((render) => !render.hasContent).length} showed nothing, ${findings.length} findings${counts(findings)}, ${renders.filter((render) => render.retried).length} retried${embeddedSummary}`);
    }
  } catch (error) {
    result.findings.push({ kind: "load", view: null, state: null, size: null, scheme: null, mode: null, selector: "document", values: { source: "surface", message: error.message } });
  } finally {
    await service?.close();
  }
  result.renders = result.renders.filter(Boolean);
  result.retriedCount = result.renders.filter((render) => render.retried).length;
  result.findings.push(...result.renders.flatMap((render) => render.findings));
  result.passed = result.renders.length === tasks.length && result.renders.every((render) => render.rendered) && result.findings.length === reported(result.findings);
  if (surface.reportedKinds !== undefined) {
    result.reportedKinds = surface.reportedKinds;
    result.reportedCount = reported(result.findings);
    result.failingCount = result.findings.length - result.reportedCount;
  }
  if (writing && result.passed) await writeGeometry(surface.geometry.file, result.renders, geometry, selected);
  if (out) await writeFile(join(out, "result.json"), `${JSON.stringify(result, null, 2)}\n`);
  print(`Total: ${result.renders.length} renders, ${result.renders.filter((render) => !render.hasContent).length} showed nothing, ${result.findings.length} findings${counts(result.findings)}, ${result.renders.reduce((total, render) => total + render.cancelled, 0)} cancelled, ${result.passed ? "pass" : "fail"}, ${reducedModeCount} views and states with fewer than all modes, ${result.retriedCount} retried`);
  return result;
}

export async function main(args = process.argv.slice(2)) {
  const { values } = parseArgs({ args, options: { surface: { type: "string" }, out: { type: "string" }, view: { type: "string" }, list: { type: "boolean" }, jobs: { type: "string" }, "write-geometry": { type: "boolean" } }, allowPositionals: false });
  if (values.jobs !== undefined) jobCount(values.jobs);
  if (values.list) {
    for (const device of DEVICES) console.log(`${device.width} x ${device.height}`);
    return 0;
  }
  if (!values.surface || (!values.out && !values["write-geometry"])) throw new Error("Usage: node tools/ui-check/check.mjs --surface FILE [--out DIRECTORY | --write-geometry] [--view NAME] [--jobs COUNT] | --list");
  const result = await checkSurface({ ...values, writeGeometry: values["write-geometry"] });
  return result.passed ? 0 : 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().then((code) => { process.exitCode = code; }, (error) => { console.error(error.message); process.exitCode = 1; });
}
