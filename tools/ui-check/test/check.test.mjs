import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { mkdir, mkdtemp, readFile, readdir, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test, { after, before } from "node:test";
import { checkSurface } from "../check.mjs";
import { DEVICES, LIMITS, MODES, REPOSITORY, readSurface, renderMatrix } from "../config.mjs";
import { serveFiles, startSurface } from "../server.mjs";

const fixtures = join(dirname(fileURLToPath(import.meta.url)), "fixtures");
const cli = resolve(dirname(fileURLToPath(import.meta.url)), "../check.mjs");
const widths = [360, 390, 430, 768, 834, 1024, 1280, 1440, 1920, 2560, 844];
let root;
let files;
let server;
let address;
let external;
let externalAddress;
let externalRequests = 0;
let sequence = 0;
const lateRequests = { response: 0, answer: 0 };
const lateClosures = { response: 0, answer: 0 };
const lateResponseMs = LIMITS.navigationMs - 1000;

before(async function serveTestPages() {
  root = await mkdtemp(join(tmpdir(), "ui-check-"));
  server = createServer((request, response) => {
    if (request.url === "/never") return;
    const url = new URL(request.url, "http://localhost");
    if (["/late.html", "/base.css"].includes(url.pathname)) {
      readFile(join(fixtures, url.pathname.slice(1))).then((body) => {
        response.writeHead(200, { "content-type": url.pathname.endsWith(".css") ? "text/css" : "text/html" });
        response.end(body);
      }, () => response.destroy());
      return;
    }
    if (url.pathname === "/network-failure") { response.destroy(); return; }
    if (["/late-response", "/late-answer"].includes(url.pathname)) {
      const kind = url.pathname === "/late-response" ? "response" : "answer";
      lateRequests[kind]++;
      const timer = setTimeout(() => {
        if (kind === "answer") response.writeHead(302, { location: url.searchParams.get("address") });
        else response.writeHead(200, { "content-type": "text/plain" });
        response.end("Late response");
      }, lateResponseMs);
      response.on("close", () => {
        if (!response.writableEnded) lateClosures[kind]++;
        clearTimeout(timer);
      });
      return;
    }
    response.writeHead(200, { "content-type": "text/plain" });
    response.end("Ready");
  });
  await new Promise((resolve) => server.listen(0, "localhost", resolve));
  address = `http://localhost:${server.address().port}`;
  files = await serveFiles(fixtures, "http://localhost:0");
  external = createServer((request, response) => { externalRequests++; response.end("External service"); });
  await new Promise((resolve) => external.listen(0, "localhost", resolve));
  externalAddress = `http://localhost:${external.address().port}`;
});

after(async function closeTestPages() {
  await files?.close();
  for (const instance of [server, external]) {
    if (!instance) continue;
    await new Promise((resolve) => { instance.close(resolve); instance.closeAllConnections(); });
  }
  await rm(root, { recursive: true, force: true });
});

async function surfaceFor(name, options = {}) {
  const surface = {
    name: "Test surface", directory: fixtures,
    start: `${JSON.stringify(process.execPath)} -e "setInterval(()=>{},1000)"`,
    address: files.address, readyPath: "/clean.html",
    views: [{ name, path: `/${name}.html` }],
    answers: [], typeScale: [16, 24], ...options,
  };
  const filename = join(root, `surface-${++sequence}.json`);
  await writeFile(filename, JSON.stringify(surface));
  return filename;
}

async function execute(name, options = {}, renderOptions = {}) {
  const surface = await surfaceFor(name, options);
  const out = join(root, `renders-${sequence}`);
  const lines = [];
  const result = await checkSurface({ surface, out, ...renderOptions }, (line) => lines.push(line));
  assert.deepEqual(JSON.parse(await readFile(join(out, "result.json"), "utf8")), result);
  return { result, out, lines };
}

function assertMatrix(result) {
  assert.equal(result.renders.length, 34);
  assert.deepEqual([...new Set(result.renders.map((render) => render.size.width))], widths);
  assert.equal(result.renders.filter((render) => render.size.width === 844 && render.size.height === 390).length, 2);
  for (const render of result.renders) {
    assert.ok(render.rendered, JSON.stringify(render.findings));
    assert.equal(typeof render.hasContent, "boolean");
    for (const finding of render.findings) {
      for (const key of ["view", "state", "size", "scheme", "mode", "selector", "values"]) assert.ok(key in finding, key);
      assert.equal(finding.view, render.view);
      assert.equal(finding.state, render.state);
      assert.deepEqual(finding.size, render.size);
      assert.equal(finding.scheme, render.scheme);
      assert.equal(finding.mode, render.mode);
      assert.ok(finding.selector);
      assert.ok(Object.keys(finding.values).length);
    }
  }
}

async function cleanPagePassesWholeDeviceList() {
  const { result, out, lines } = await execute("clean");
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.ok(result.renders.every((render) => render.hasContent));
  assert.ok(result.renders.every((render) => render.cancelled === 0));
  assert.deepEqual(result.findings, []);
  assert.equal(lines.length, 2);
  assert.match(lines[0], /^clean: 34 renders, 0 did not render, 0 showed nothing, 0 findings$/);
  assert.match(lines[1], /^Total: 34 renders, 0 showed nothing, 0 findings, 0 cancelled, pass, 0 views and states with fewer than all modes$/);
  assert.equal((await readdir(out)).filter((file) => file.endsWith(".png")).length, 34);
  const png = await readFile(join(out, result.renders[0].screenshot));
  assert.equal(png.readUInt32BE(16), 360);
  assert.ok(png.readUInt32BE(20) > 900, "The screenshot includes content below the viewport");
  for (const render of result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.equal(render.focused.length, 2);
  }
}
test(cleanPagePassesWholeDeviceList);

async function emptyBodyReportsLoadAtEverySize() {
  const { result, lines } = await execute("blank");
  assertMatrix(result);
  assert.equal(result.passed, false);
  for (const render of result.renders) {
    assert.equal(render.hasContent, false);
    assert.deepEqual(render.findings.map((finding) => finding.kind), ["load", "load"]);
    assert.deepEqual(render.findings.map((finding) => finding.values.source), ["empty-document", "single-colour-viewport"]);
  }
  assert.deepEqual(lines, [
    "blank: 34 renders, 0 did not render, 34 showed nothing, 68 findings",
    "Total: 34 renders, 34 showed nothing, 68 findings, 0 cancelled, fail, 0 views and states with fewer than all modes",
  ]);
}
test(emptyBodyReportsLoadAtEverySize);

async function onlyClippedTextReportsLoad() {
  const { result } = await execute("blank", { views: [{ name: "clipped", path: "/blank.html?clipped" }] });
  assertMatrix(result);
  assert.equal(result.passed, false);
  for (const render of result.renders) {
    assert.equal(render.hasContent, false);
    assert.deepEqual(render.findings.map((finding) => finding.kind), ["load", "load"]);
    assert.deepEqual(render.findings.map((finding) => finding.values.source), ["empty-document", "single-colour-viewport"]);
  }
}
test(onlyClippedTextReportsLoad);

async function coveredTextReportsTheViewportMeasurement() {
  const { result, lines, out } = await execute("covered", { views: [{
    name: "covered", path: "/covered.html", states: [{ name: "revealed", steps: [
      { action: "press", selector: "body", key: "Escape" },
    ] }],
  }] });
  assert.equal(result.renders.length, 68);
  assert.equal(result.passed, false);
  assert.equal(result.findings.length, 34);
  for (const state of ["default", "revealed"]) {
    const renders = result.renders.filter((render) => render.state === state);
    assertMatrix({ renders });
    for (const render of renders) {
      assert.equal(render.hasContent, state === "revealed");
      if (state === "default") {
        assert.equal(render.findings.length, 1);
        assert.equal(render.findings[0].kind, "load");
        assert.equal(render.findings[0].values.source, "single-colour-viewport");
      } else assert.deepEqual(render.findings, []);
    }
  }
  const png = await readFile(join(out, result.renders[0].screenshot));
  assert.ok(png.readUInt32BE(20) > 900, "Viewport content is measured separately from the full-page screenshot");
  assert.deepEqual(lines, [
    "covered: 68 renders, 0 did not render, 34 showed nothing, 34 findings",
    "Total: 68 renders, 34 showed nothing, 34 findings, 0 cancelled, fail, 0 views and states with fewer than all modes",
  ]);
}
test(coveredTextReportsTheViewportMeasurement);

async function oneLineOfTextShowsContent() {
  const { result } = await execute("blank", { views: [{ name: "text", path: "/blank.html?text" }] });
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.ok(result.renders.every((render) => render.hasContent));
}
test(oneLineOfTextShowsContent);

async function imageWithAreaShowsContent() {
  const { result } = await execute("blank", { views: [{ name: "image", path: "/blank.html?image" }] });
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.ok(result.renders.every((render) => render.hasContent));
}
test(imageWithAreaShowsContent);

async function contentMeasurementCannotBeOmitted() {
  for (const change of [
    { checkContent: false },
    { views: [{ name: "blank", path: "/blank.html", checkContent: false }] },
    { views: [{ name: "blank", path: "/blank.html", states: [{ name: "ready", checkContent: false, steps: [{ action: "press", selector: "body", key: "Escape" }] }] }] },
    { views: [{ name: "blank", path: "/blank.html", states: [{ name: "ready", steps: [{ action: "press", selector: "body", key: "Escape", checkContent: false }] }] }] },
  ]) {
    const surface = await surfaceFor("blank", change);
    await assert.rejects(readSurface(surface), /unknown field checkContent/);
  }
}
test(contentMeasurementCannotBeOmitted);

async function parallelRendersPreserveContentFindingsAndCounts() {
  const surface = await surfaceFor("content", { views: [
    { name: "blank", path: "/blank.html" },
    { name: "covered", path: "/covered.html", states: [{ name: "revealed", steps: [
      { action: "press", selector: "body", key: "Escape" },
    ] }] },
    { name: "text", path: "/blank.html?text" },
  ] });
  const results = [];
  for (const jobs of [1, 4]) {
    const out = join(root, `content-${sequence}-${jobs}`);
    const lines = [];
    const result = await checkSurface({ surface, out, jobs }, (line) => lines.push(line));
    assert.deepEqual(JSON.parse(await readFile(join(out, "result.json"), "utf8")), result);
    assert.equal(result.passed, false);
    assert.equal(result.renders.length, 136);
    assert.ok(result.renders.every((render) => render.rendered));
    assert.equal(result.renders.filter((render) => !render.hasContent).length, 68);
    assert.equal(result.findings.length, 102);
    assert.deepEqual([...new Set(result.findings.map((finding) => finding.values.source))], ["empty-document", "single-colour-viewport"]);
    assert.deepEqual(lines, [
      "blank: 34 renders, 0 did not render, 34 showed nothing, 68 findings",
      "covered: 68 renders, 0 did not render, 34 showed nothing, 34 findings",
      "text: 34 renders, 0 did not render, 0 showed nothing, 0 findings",
      "Total: 136 renders, 68 showed nothing, 102 findings, 0 cancelled, fail, 0 views and states with fewer than all modes",
    ]);
    results.push({ result, lines });
  }
  assert.deepEqual(results[1], results[0]);
}
test(parallelRendersPreserveContentFindingsAndCounts);

async function eachPageReportsOnlyItsFindingKind() {
  for (const kind of ["load", "sideways", "cut", "contrast", "touch", "focus", "shift", "motion", "scale"]) {
    const { result } = await execute(kind);
    assertMatrix(result);
    assert.equal(result.passed, false, kind);
    assert.deepEqual([...new Set(result.findings.map((finding) => finding.kind))], [kind], JSON.stringify(result.findings));
    assert.ok(result.findings.length > 0, kind);
    if (kind === "focus") {
      assert.ok(result.findings.every((finding) => finding.mode === "keyboard"));
      assert.equal(result.renders.filter((render) => render.findings.length).length, 4);
    } else if (kind === "motion") {
      assert.ok(result.findings.every((finding) => finding.mode === "reduced-motion"));
      assert.equal(result.renders.filter((render) => render.findings.length).length, 4);
    } else if (kind === "touch") {
      assert.ok(result.findings.every((finding) => finding.size.width <= 834));
    } else {
      assert.ok(result.renders.every((render) => render.findings.length), kind);
    }
    if (kind === "load") {
      const sources = new Set(result.findings.map((finding) => finding.values.source));
      for (const source of ["console", "pageerror", "unhandledrejection", "response", "unanswered-request"]) assert.ok(sources.has(source), source);
      for (const resource of ["stylesheet", "script"]) assert.ok(result.findings.some((finding) => finding.values.resource === resource));
    }
  }
}
test(eachPageReportsOnlyItsFindingKind);

async function controlsUseThreeToOneContrast() {
  const { result } = await execute("control-contrast");
  assertMatrix(result);
  assert.equal(result.passed, false);
  assert.deepEqual([...new Set(result.findings.map((finding) => finding.kind))], ["contrast"]);
  assert.ok(result.findings.every((finding) => finding.values.rule === "ui-control-contrast"));
  const surface = await surfaceFor("control-contrast");
  const command = await runCli(["--surface", surface, "--out", join(root, "control-status")]);
  assert.notEqual(command.code, 0);
  assert.match(command.stdout, /findings, 0 cancelled, fail/);
}
test(controlsUseThreeToOneContrast);

async function fluidTypeStepsPass() {
  const { result } = await execute("fluid", { typeScale: [{ min: 16, max: 24 }] });
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
}
test(fluidTypeStepsPass);

async function runCli(args) {
  const child = spawn(process.execPath, [cli, ...args], { stdio: ["ignore", "pipe", "pipe"] });
  let stdout = "";
  let stderr = "";
  child.stdout.on("data", (chunk) => { stdout += chunk; });
  child.stderr.on("data", (chunk) => { stderr += chunk; });
  const code = await new Promise((resolve, reject) => { child.once("error", reject); child.once("close", resolve); });
  return { code, stdout, stderr };
}

async function commandListsDeviceSizes() {
  const result = await runCli(["--list"]);
  assert.equal(result.code, 0, result.stderr);
  assert.deepEqual(result.stdout.trim().split("\n"), DEVICES.map(({ width, height }) => `${width} x ${height}`));
  assert.equal(renderMatrix().length, 34);
}
test(commandListsDeviceSizes);

async function repositoryOutputIsRefused() {
  const surface = await surfaceFor("clean");
  for (const out of [REPOSITORY, join(REPOSITORY, "tools/ui-check/screenshots")]) {
    const result = await runCli(["--surface", surface, "--out", out]);
    assert.notEqual(result.code, 0);
    assert.match(result.stderr, /outside the repository/);
  }
  const link = join(root, "repository");
  await symlink(REPOSITORY, link);
  await assert.rejects(checkSurface({ surface, out: join(link, "screenshots") }), /outside the repository/);
}
test(repositoryOutputIsRefused);

async function limitsAndExemptionsAreRefused() {
  for (const change of [
    { limits: { touch: 1 } },
    { exemptions: ["touch"] },
    { views: [{ name: "clean", path: "/clean.html", exemptions: ["contrast"] }] },
    { views: [{ name: "clean", path: "/clean.html", states: [{ name: "shown", steps: [{ action: "click", selector: "button", exempt: "touch" }] }] }] },
    { typeScale: [{ min: 16, max: 24, tolerance: 100 }] },
  ]) {
    const surface = await surfaceFor("clean", change);
    const result = await runCli(["--surface", surface, "--out", join(root, `refused-${sequence}`)]);
    assert.notEqual(result.code, 0);
    assert.match(result.stderr, /unknown field/);
  }
}
test(limitsAndExemptionsAreRefused);

async function missingViewReturnsNonzero() {
  const surface = await surfaceFor("missing");
  const out = join(root, "missing");
  const result = await runCli(["--surface", surface, "--out", out]);
  assert.notEqual(result.code, 0);
  const report = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
  assert.equal(report.passed, false);
  assert.equal(report.renders.length, 34);
  assert.ok(report.renders.every((render) => !render.rendered));
  assert.ok(report.findings.some((finding) => /View did not answer/.test(finding.values.message)));
}
test(missingViewReturnsNonzero);

async function unavailableSurfaceReturnsNonzero() {
  const surface = await surfaceFor("clean", { start: `${JSON.stringify(process.execPath)} -e "process.exit(2)"`, address, readyPath: "/never" });
  const out = join(root, "unavailable");
  const result = await runCli(["--surface", surface, "--out", out]);
  assert.notEqual(result.code, 0);
  const report = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
  assert.equal(report.passed, false);
  assert.equal(report.renders.length, 0);
  assert.equal(report.findings[0].kind, "load");
  assert.match(report.findings[0].values.message, /before readiness/);
}
test(unavailableSurfaceReturnsNonzero);

async function stateStepsAndViewSelectionRender() {
  const surface = await surfaceFor("states", { views: [
    { name: "states", path: "/states.html", states: [{ name: "filled", steps: [
      { action: "fill", selector: "#entry", value: "Ready" },
      { action: "press", selector: "#entry", key: "Tab" },
      { action: "click", selector: "#show" },
      { action: "wait", text: "Ready" },
    ] }] },
    { name: "other", path: "/missing.html" },
  ] });
  const out = join(root, "states");
  const result = await runCli(["--surface", surface, "--out", out, "--view", "states"]);
  assert.equal(result.code, 0, result.stderr + result.stdout);
  const report = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
  assert.equal(report.passed, true, JSON.stringify(report.findings));
  assert.equal(report.renders.length, 68);
  assert.deepEqual([...new Set(report.renders.map((render) => render.state))], ["default", "filled"]);
  assert.ok(report.renders.every((render) => render.view === "states"));
}
test(stateStepsAndViewSelectionRender);

async function fixedAnswersStayInTheBrowser() {
  const answer = join(root, "answer.json");
  await writeFile(answer, JSON.stringify({ message: "Fixed answer" }));
  const surface = await surfaceFor("answers", {
    views: [{ name: "answers", path: `/answers.html?address=${encodeURIComponent(`${externalAddress}/data`)}`, states: [{ name: "answered", steps: [{ action: "wait", text: "Fixed answer" }] }] }],
    answers: [{ method: "GET", pattern: `${externalAddress}/*`, file: "answer.json", headers: { "content-type": "application/json", "access-control-allow-origin": "*" } }],
  });
  const result = await checkSurface({ surface, out: join(root, "answers") }, () => {});
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.equal(result.renders.length, 68);
  assert.equal(externalRequests, 0);
  const blockedSurface = await surfaceFor("answers", {
    views: [{ name: "answers", path: `/answers.html?address=${encodeURIComponent(`${externalAddress}/unanswered`)}` }],
  });
  const blocked = await checkSurface({ surface: blockedSurface, out: join(root, "blocked") }, () => {});
  assert.equal(blocked.passed, false);
  assert.deepEqual([...new Set(blocked.findings.map((finding) => finding.kind))], ["load"]);
  assert.ok(blocked.findings.some((finding) => finding.values.source === "unanswered-request"));
  assert.equal(externalRequests, 0);
}
test(fixedAnswersStayInTheBrowser);

async function fixedAnswersUseStateViewAndSurfaceOrder() {
  const pattern = `${externalAddress}/ordered`;
  const filesByLevel = {
    surface: join(root, "surface-answer.json"),
    view: join(root, "view-answer.json"),
    state: join(root, "state-answer.json"),
  };
  for (const [level, filename] of Object.entries(filesByLevel)) {
    await writeFile(filename, JSON.stringify({ message: `${level} answer` }));
  }
  const answer = (file) => ({ method: "GET", pattern, file: file.slice(root.length + 1), headers: { "content-type": "application/json" } });
  const page = `/answers.html?address=${encodeURIComponent(pattern)}`;
  const surface = await surfaceFor("ordered", {
    answers: [answer(filesByLevel.surface)],
    views: [
      { name: "surface", path: page, modes: ["default"], states: [{ name: "surface", modes: ["default"], steps: [{ action: "wait", text: "surface answer" }] }] },
      { name: "view", path: page, modes: ["default"], answers: [answer(filesByLevel.view)], states: [{ name: "view", modes: ["default"], steps: [{ action: "wait", text: "view answer" }] }] },
      { name: "state", path: page, modes: ["default"], states: [{ name: "state", modes: ["default"], steps: [{ action: "wait", text: "state answer" }], answers: [answer(filesByLevel.state)] }] },
    ],
  });
  const result = await checkSurface({ surface, out: join(root, "ordered") }, () => {});
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.equal(result.renders.length, 132);
  assert.deepEqual(result.views, [
    { name: "surface", modes: ["default"], answers: 0, states: [{ name: "surface", modes: ["default"], answers: 0 }] },
    { name: "view", modes: ["default"], answers: 1, states: [{ name: "view", modes: ["default"], answers: 0 }] },
    { name: "state", modes: ["default"], answers: 0, states: [{ name: "state", modes: ["default"], answers: 1 }] },
  ]);
  assert.equal(externalRequests, 0);
}
test(fixedAnswersUseStateViewAndSurfaceOrder);

async function fixedAnswersStayWithinTheirStateAndView() {
  const pattern = `${externalAddress}/isolated`;
  const stateFile = join(root, "isolated-state-answer.json");
  const viewFile = join(root, "isolated-view-answer.json");
  await writeFile(stateFile, JSON.stringify({ message: "state only" }));
  await writeFile(viewFile, JSON.stringify({ message: "view only" }));
  const entry = (file) => ({ method: "GET", pattern, file: file.slice(root.length + 1), headers: { "content-type": "application/json" } });
  const path = `/answers.html?address=${encodeURIComponent(pattern)}`;
  const surface = await surfaceFor("isolated", {
    views: [
      { name: "states", path, modes: ["default"], states: [
        { name: "first", modes: ["default"], steps: [{ action: "wait", text: "state only" }], answers: [entry(stateFile)] },
        { name: "second", modes: ["default"], steps: [{ action: "wait", text: "state only" }] },
      ] },
      { name: "scoped", path, modes: ["default"], answers: [entry(viewFile)] },
      { name: "other", path, modes: ["default"], answers: [] },
    ],
  });
  const result = await checkSurface({ surface, out: join(root, "isolated") }, () => {});
  assert.equal(result.renders.filter((render) => render.view === "states" && render.state === "first").every((render) => render.findings.length === 0), true);
  assert.ok(result.renders.filter((render) => render.view === "states" && render.state === "second").every((render) => render.findings.some((finding) => finding.values.source === "unanswered-request")));
  assert.ok(result.renders.filter((render) => render.view === "scoped").every((render) => render.findings.length === 0));
  assert.ok(result.renders.filter((render) => render.view === "other").every((render) => render.findings.some((finding) => finding.values.source === "unanswered-request")));
  assert.equal(result.passed, false);
  assert.equal(externalRequests, 0);
}
test(fixedAnswersStayWithinTheirStateAndView);

async function lateRequestsKeepReportsAndExitStatus() {
  assert.ok(lateResponseMs > LIMITS.settleMs);
  await writeFile(join(root, "late-answer.txt"), "Fixed answer");
  const answered = `${externalAddress}/late-answer`;
  const path = `/late.html?answer=${encodeURIComponent(answered)}`;
  const outsideRequests = externalRequests;
  for (const withFinding of [false, true]) {
    const surface = await surfaceFor("late", {
      address, readyPath: "/late.html",
      views: [
        { name: "late", path, modes: ["default"] },
        { name: "following", path: `${path}${withFinding ? "&finding" : ""}`, modes: ["default"] },
      ],
      answers: [{ method: "GET", pattern: answered, file: "late-answer.txt" }],
    });
    const outcomes = [];
    for (const jobs of [1, 4]) {
      const started = { ...lateRequests };
      const closed = { ...lateClosures };
      const out = join(root, `late-${withFinding}-${jobs}`);
      const command = await runCli(["--surface", surface, "--out", out, "--jobs", String(jobs)]);
      assert.equal(command.stderr, "");
      assert.equal(command.code, withFinding ? 1 : 0, command.stdout);
      const report = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
      assert.equal(report.passed, !withFinding);
      assert.equal(report.renders.length, 44);
      assert.ok(report.renders.every((render) => render.rendered && render.hasContent));
      assert.equal(report.findings.length, withFinding ? 22 : 0);
      for (const finding of report.findings) {
        assert.equal(finding.view, "following");
        assert.equal(finding.kind, "load");
        assert.equal(finding.values.source, "console");
        assert.equal(finding.values.message, "Page finding");
      }
      assert.deepEqual(command.stdout.trim().split("\n"), [
        "late: 22 renders, 0 did not render, 0 showed nothing, 0 findings",
        `following: 22 renders, 0 did not render, 0 showed nothing, ${withFinding ? 22 : 0} findings`,
        `Total: 44 renders, 0 showed nothing, ${withFinding ? 22 : 0} findings, 0 cancelled, ${withFinding ? "fail" : "pass"}, 2 views and states with fewer than all modes`,
      ]);
      for (const kind of ["response", "answer"]) {
        assert.equal(lateRequests[kind] - started[kind], 44);
        assert.equal(lateClosures[kind] - closed[kind], 44);
      }
      outcomes.push({ report, command });
    }
    assert.deepEqual(outcomes[1], outcomes[0]);
  }
  assert.equal(externalRequests, outsideRequests);
}
test(lateRequestsKeepReportsAndExitStatus);

async function activeSurfaceRequestFailuresRemainLoadFindings() {
  const { result } = await execute("late", {
    address, readyPath: "/late.html",
    views: [{ name: "active", path: "/late.html?active", modes: ["default"] }],
  });
  assert.equal(result.passed, false);
  assert.equal(result.renders.length, 22);
  for (const render of result.renders) {
    assert.equal(render.rendered, true);
    assert.equal(render.findings.length, 1, JSON.stringify(render.findings));
    assert.ok(render.findings.every((finding) => finding.kind === "load"));
    assert.ok(render.findings.some((finding) => finding.values.source === "requestfailed"
      && finding.values.address === `${address}/network-failure`
      && finding.values.method === "GET" && finding.values.resource === "fetch"));
  }
}
test(activeSurfaceRequestFailuresRemainLoadFindings);

async function requestSurface(script, { refuse = false } = {}) {
  const directory = await mkdtemp(join(root, "request-surface-"));
  const css = await readFile(join(fixtures, "base.css"), "utf8");
  await writeFile(join(directory, "index.html"), `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Requests</title><link rel="icon" href="data:,"><style>${css}</style><body><p>Reading</p><script>${script}</script></body></html>`);
  const pending = new Map();
  const instance = createServer((request, response) => {
    const url = new URL(request.url, "http://localhost");
    if (url.pathname === "/ready") {
      response.writeHead(200, { connection: "close" });
      response.end("Ready");
      if (refuse) instance.close();
    } else if (url.pathname === "/pending") {
      const id = url.searchParams.get("id");
      pending.set(id, (pending.get(id) ?? 0) + 1);
    } else if (url.pathname === "/started") {
      response.end(String(pending.get(url.searchParams.get("id")) ?? 0));
    } else if (url.pathname === "/network-failure") response.destroy();
    else if (url.pathname === "/redirect") {
      response.writeHead(302, { location: `${externalAddress}/unanswered` });
      response.end();
    } else {
      readFile(join(directory, "index.html")).then((body) => {
        response.writeHead(200, { "content-type": "text/html" });
        response.end(body);
      }, () => response.destroy());
    }
  });
  await new Promise((resolve) => instance.listen(0, "localhost", resolve));
  const origin = `http://localhost:${instance.address().port}`;
  return {
    options: { directory, address: origin, readyPath: "/ready", views: [{ name: "requests", path: "/index.html", modes: ["default"] }] },
    async close() {
      await new Promise((resolve) => { instance.close(resolve); instance.closeAllConnections(); });
    },
  };
}

async function cancelledReads(count, context) {
  const surface = await requestSurface(`
    (async () => {
      const id = crypto.randomUUID();
      const controllers = Array.from({ length: ${count} }, () => new AbortController());
      const reads = controllers.map((controller) => fetch('/pending?id=' + id, { signal: controller.signal }).catch((error) => {
        if (error.name !== 'AbortError') throw error;
      }));
      while (Number(await (await fetch('/started?id=' + id)).text()) !== controllers.length) {
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
      for (const controller of controllers) controller.abort();
      await Promise.all(reads);
      document.querySelector('p').textContent = 'Cancelled ${count} reads';
    })();
  `);
  try {
    const { result, lines } = await execute("requests", surface.options);
    assert.equal(result.passed, true, JSON.stringify(result.findings));
    assert.equal(result.renders.length, 22);
    assert.deepEqual(result.findings, []);
    for (const render of result.renders) {
      assert.equal(render.rendered, true);
      assert.equal(render.cancelled, count);
      assert.deepEqual(render.findings, []);
    }
    assert.equal(lines.at(-1), `Total: 22 renders, 0 showed nothing, 0 findings, ${22 * count} cancelled, pass, 1 views and states with fewer than all modes`);
    context.diagnostic(`${count} page-cancelled reads per render, ${22 * count} cancelled in total, no findings`);
  } finally { await surface.close(); }
}

async function onePageCancelledReadHasNoFinding(context) {
  await cancelledReads(1, context);
}
test(onePageCancelledReadHasNoFinding);

async function threePageCancelledReadsHaveNoFindings(context) {
  await cancelledReads(3, context);
}
test(threePageCancelledReadsHaveNoFindings);

async function unansweredRequestsProduceOneFinding(context) {
  const outsideRequests = externalRequests;
  for (const target of [`${externalAddress}/unanswered`, "/redirect"]) {
    const surface = await requestSurface(`fetch(${JSON.stringify(target)}).catch(() => {});`);
    try {
      const { result } = await execute("requests", surface.options);
      assert.equal(result.passed, false);
      assert.equal(result.renders.length, 22);
      for (const render of result.renders) {
        assert.equal(render.rendered, true);
        assert.equal(render.cancelled, 0);
        assert.equal(render.findings.length, 1, JSON.stringify(render.findings));
        assert.equal(render.findings[0].kind, "load");
        assert.equal(render.findings[0].values.source, "unanswered-request");
        assert.equal(render.findings[0].values.address, `${externalAddress}/unanswered`);
      }
      context.diagnostic(`${target.startsWith("/") ? "Redirected" : "Direct"} unanswered request produces one load finding per render`);
    } finally { await surface.close(); }
  }
  assert.equal(externalRequests, outsideRequests);
}
test(unansweredRequestsProduceOneFinding);

async function unfetchableSurfaceRequestsProduceOneFinding(context) {
  const surface = await requestSurface('fetch("/network-failure").catch(() => {});');
  try {
    const { result } = await execute("requests", surface.options);
    assert.equal(result.passed, false);
    assert.equal(result.renders.length, 22);
    for (const render of result.renders) {
      assert.equal(render.rendered, true);
      assert.equal(render.cancelled, 0);
      assert.equal(render.findings.length, 1, JSON.stringify(render.findings));
      const finding = render.findings[0];
      assert.equal(finding.kind, "load");
      assert.equal(finding.values.source, "requestfailed");
      assert.equal(finding.values.address, `${surface.options.address}/network-failure`);
      assert.match(finding.values.message, /socket hang up|ECONNRESET/);
    }
    context.diagnostic("Unfetchable surface request produces one load finding per render");
  } finally { await surface.close(); }
}
test(unfetchableSurfaceRequestsProduceOneFinding);

async function refusedConnectionsRemainRequestFailures(context) {
  const surface = await requestSurface("", { refuse: true });
  try {
    const { result } = await execute("requests", surface.options);
    assert.equal(result.passed, false);
    assert.equal(result.renders.length, 22);
    for (const render of result.renders) {
      assert.equal(render.rendered, false);
      assert.equal(render.cancelled, 0);
      const failures = render.findings.filter((finding) => finding.values.source === "requestfailed");
      assert.equal(failures.length, 1, JSON.stringify(render.findings));
      assert.equal(failures[0].kind, "load");
      assert.match(failures[0].values.message, /ECONNREFUSED/);
    }
    context.diagnostic("Refused connections remain requestfailed load findings");
  } finally { await surface.close(); }
}
test(refusedConnectionsRemainRequestFailures);

async function cancelledDocumentNavigationRemainsAFinding(context) {
  const surface = await requestSurface(`
    addEventListener('load', async () => {
      const id = crypto.randomUUID();
      location.href = '/pending?id=' + id;
      while (Number(await (await fetch('/started?id=' + id)).text()) !== 1) {
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
      window.stop();
    });
  `);
  try {
    const { result } = await execute("requests", surface.options);
    assert.equal(result.passed, false);
    assert.equal(result.renders.length, 22);
    for (const render of result.renders) {
      assert.equal(render.cancelled, 0);
      const failures = render.findings.filter((finding) => finding.values.source === "requestfailed");
      assert.equal(failures.length, 1, JSON.stringify(render.findings));
      assert.equal(failures[0].kind, "load");
      assert.equal(failures[0].values.resource, "document");
      assert.equal(failures[0].values.message, "net::ERR_ABORTED");
    }
    context.diagnostic("Cancelled document navigation remains a requestfailed load finding with net::ERR_ABORTED");
  } finally { await surface.close(); }
}
test(cancelledDocumentNavigationRemainsAFinding);

async function commandBuildsAndServesFiles() {
  const surface = await surfaceFor("clean", {
    build: `${JSON.stringify(process.execPath)} -e "process.stdout.write('Surface ready')"`,
    start: undefined, address: "http://localhost:0",
  });
  const result = await runCli(["--surface", surface, "--out", join(root, "self-served"), "--jobs", "1"]);
  assert.equal(result.code, 0, result.stderr + result.stdout);
  assert.match(result.stdout, /34 renders, 0 showed nothing, 0 findings, 0 cancelled, pass/);
}
test(commandBuildsAndServesFiles);

function nodeCommand(source) {
  return `${JSON.stringify(process.execPath)} -e '${source.replaceAll("'", "'\\''")}'`;
}

async function temporaryBuildSurface(options = {}) {
  const directory = await mkdtemp(join(root, "build-surface-"));
  await writeFile(join(directory, "index.html"), "Ready");
  const filename = await surfaceFor("built", {
    directory, build: nodeCommand("process.stdout.write('Built')"),
    start: undefined, address: "http://localhost:0", readyPath: "/index.html",
    views: [{ name: "built", path: "/index.html", modes: ["default"] }],
    ...options,
  });
  return { filename, directory };
}

async function buildBoundsRequireWholeMilliseconds(context) {
  for (const buildMs of [null, false, "1000", {}, [], -1, 0, 999, 1000.5, 1800001]) {
    const { filename } = await temporaryBuildSurface({ buildMs });
    await assert.rejects(readSurface(filename), (error) => {
      assert.match(error.message, /Surface Test surface buildMs.*whole number from 1000 to 1800000/);
      context.diagnostic(error.message);
      return true;
    });
  }
  for (const buildMs of [1000, 1800000]) {
    const { filename } = await temporaryBuildSurface({ buildMs });
    assert.equal((await readSurface(filename)).buildMs, buildMs);
  }
}
test(buildBoundsRequireWholeMilliseconds);

async function outputsRequireBuildAndOneToSixteenPaths(context) {
  for (const outputs of [null, false, "dist", {}, [], Array.from({ length: 17 }, (_, index) => `dist-${index}`), [null], [1], [""], [" "]]) {
    const { filename } = await temporaryBuildSurface({ outputs });
    await assert.rejects(readSurface(filename), (error) => {
      assert.match(error.message, /Surface Test surface outputs.*(one to sixteen paths|nonempty string)/);
      context.diagnostic(error.message);
      return true;
    });
  }
  const { filename } = await temporaryBuildSurface({ build: undefined, outputs: ["dist"] });
  await assert.rejects(readSurface(filename), (error) => {
    assert.match(error.message, /Surface Test surface outputs requires a build command/);
    context.diagnostic(error.message);
    return true;
  });
  const outputs = Array.from({ length: 16 }, (_, index) => `dist-${index}/nested`);
  const valid = await temporaryBuildSurface({ outputs });
  assert.deepEqual((await readSurface(valid.filename)).outputs, outputs);
}
test(outputsRequireBuildAndOneToSixteenPaths);

async function outputsStayInsideTheSurfaceAndOutsideDependencies(context) {
  for (const output of [root, "C:\\output", "../outside", "nested/../../outside", ".", "nested/..", "node_modules", "dist/node_modules", "node_modules/pkg/dist", "dist/node_modules/../cache"]) {
    const { filename } = await temporaryBuildSurface({ outputs: [output] });
    await assert.rejects(readSurface(filename), (error) => {
      assert.match(error.message, /Surface Test surface outputs.*(relative paths inside|node_modules)/);
      context.diagnostic(error.message);
      return true;
    });
  }
  for (const target of [root, join(root, "absent-outside")]) {
    for (const output of ["linked", "linked/nested/output"]) {
      const { filename, directory } = await temporaryBuildSurface({ outputs: [output] });
      await symlink(target, join(directory, "linked"));
      await assert.rejects(readSurface(filename), (error) => {
        assert.match(error.message, /Surface Test surface outputs.*inside the surface directory after resolving links/);
        context.diagnostic(error.message);
        return true;
      });
    }
  }
  for (const target of [".", "node_modules/pkg"]) {
    const { filename, directory } = await temporaryBuildSurface({ outputs: ["linked"] });
    await mkdir(join(directory, "node_modules/pkg"), { recursive: true });
    await symlink(target, join(directory, "linked"));
    await assert.rejects(readSurface(filename), (error) => {
      assert.match(error.message, /Surface Test surface outputs.*(directory itself|node_modules)/);
      context.diagnostic(error.message);
      return true;
    });
  }
}
test(outputsStayInsideTheSurfaceAndOutsideDependencies);

async function surfaceBuildCanExceedTheCommonBound(context) {
  const duration = LIMITS.buildMs + 1000;
  const { filename } = await temporaryBuildSurface({
    build: nodeCommand(`setTimeout(() => process.stdout.write('Built'), ${duration})`),
    buildMs: duration + 10000,
  });
  const surface = await readSurface(filename);
  const started = Date.now();
  const service = await startSurface(surface);
  try {
    assert.ok(Date.now() - started >= duration);
    assert.equal(await (await fetch(new URL("/index.html", service.address))).text(), "Ready");
  } finally { await service.close(); }
  context.diagnostic(`Build completed within ${surface.buildMs} ms after exceeding ${LIMITS.buildMs} ms`);
}
test(surfaceBuildCanExceedTheCommonBound);

async function smallerBuildBoundIsReportedWithRemovedOutputs(context) {
  const { filename, directory } = await temporaryBuildSurface({
    build: nodeCommand(`setTimeout(() => process.stdout.write('Built'), ${LIMITS.buildMs + 1000})`), buildMs: 1000,
    outputs: ["old.txt", "absent"],
  });
  await writeFile(join(directory, "old.txt"), "Old output");
  const out = join(root, `build-timeout-${sequence}`);
  const command = await runCli(["--surface", filename, "--out", out]);
  assert.equal(command.code, 1, command.stderr + command.stdout);
  const result = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
  assert.equal(result.passed, false);
  assert.equal(result.renders.length, 0);
  assert.equal(result.buildMs, 1000);
  assert.deepEqual(result.outputs, ["old.txt"]);
  assert.equal(result.findings.length, 1);
  assert.match(result.findings[0].values.message, /Surface Test surface build did not succeed: timed out after 1000 ms/);
  await assert.rejects(readFile(join(directory, "old.txt")), { code: "ENOENT" });
  context.diagnostic(result.findings[0].values.message);
}
test(smallerBuildBoundIsReportedWithRemovedOutputs);

async function removedOutputsAreRebuiltBeforeTheStartServesThem() {
  const outputs = ["dist", "old.txt", "linked.txt", "absent"];
  const html = (await readFile(join(fixtures, "clean.html"), "utf8"))
    .replace('<link rel="stylesheet" href="/base.css">', `<style>${await readFile(join(fixtures, "base.css"), "utf8")}</style>`)
    .replace("<h1>Surface</h1>", "<h1>Fresh build</h1>");
  const reservation = createServer();
  await new Promise((resolve) => reservation.listen(0, "localhost", resolve));
  const origin = `http://localhost:${reservation.address().port}`;
  await new Promise((resolve) => reservation.close(resolve));
  const { filename, directory } = await temporaryBuildSurface({
    buildMs: 3000, outputs,
    build: nodeCommand(`
      const fs = require('node:fs');
      const assert = require('node:assert/strict');
      for (const path of ${JSON.stringify(outputs)}) assert.equal(fs.existsSync(path), false, path);
      assert.equal(fs.readFileSync('keep.txt', 'utf8'), 'Keep beside outputs');
      fs.mkdirSync('dist');
      fs.writeFileSync('dist/index.html', ${JSON.stringify(html)});
    `),
    start: nodeCommand(`
      const fs = require('node:fs');
      const http = require('node:http');
      const body = fs.readFileSync('dist/index.html');
      http.createServer((request, response) => {
        response.writeHead(200, { 'content-type': 'text/html' });
        response.end(body);
      }).listen(${new URL(origin).port}, 'localhost');
    `),
    address: origin,
    views: [{ name: "built", path: "/index.html", modes: ["default"], states: [{
      name: "fresh", steps: [{ action: "wait", text: "Fresh build" }],
    }] }],
  });
  await mkdir(join(directory, "dist/nested"), { recursive: true });
  await writeFile(join(directory, "dist/index.html"), "Stale output");
  await writeFile(join(directory, "dist/nested/stale.txt"), "Stale nested output");
  await writeFile(join(directory, "old.txt"), "Old output");
  await writeFile(join(directory, "keep.txt"), "Keep beside outputs");
  await symlink("keep.txt", join(directory, "linked.txt"));
  const out = join(root, `rebuilt-${sequence}`);
  const result = await checkSurface({ surface: filename, out }, () => {});
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.equal(result.renders.length, 44);
  assert.ok(result.renders.every((render) => render.rendered && render.hasContent));
  assert.equal(result.buildMs, 3000);
  assert.deepEqual(result.outputs, ["dist", "old.txt", "linked.txt"]);
  assert.deepEqual(JSON.parse(await readFile(join(out, "result.json"), "utf8")), result);
  assert.equal(await readFile(join(directory, "dist/index.html"), "utf8"), html);
  assert.equal(await readFile(join(directory, "keep.txt"), "utf8"), "Keep beside outputs");
  for (const path of ["dist/nested", "old.txt", "linked.txt", "absent"]) {
    await assert.rejects(readFile(join(directory, path)), { code: "ENOENT" });
  }
}
test(removedOutputsAreRebuiltBeforeTheStartServesThem);

async function outputLinksAreCheckedAgainBeforeCleanup() {
  const { filename, directory } = await temporaryBuildSurface({ outputs: ["keep.txt", "dist"] });
  await writeFile(join(directory, "keep.txt"), "Keep output");
  const surface = await readSurface(filename);
  await symlink(root, join(directory, "dist"));
  await assert.rejects(startSurface(surface), /Surface Test surface outputs.*inside the surface directory after resolving links/);
  assert.equal(await readFile(join(directory, "index.html"), "utf8"), "Ready");
  assert.equal(await readFile(join(directory, "keep.txt"), "utf8"), "Keep output");
}
test(outputLinksAreCheckedAgainBeforeCleanup);

async function surfacesWithoutBuildKeysKeepTheCommonBoundAndFiles() {
  const { filename, directory } = await temporaryBuildSurface();
  const out = join(root, `unchanged-build-${sequence}`);
  const html = '<!doctype html><html lang="en"><title>Ready</title><link rel="icon" href="data:,"><body style="font:16px sans-serif;color:black;background:white"><p>Ready</p></body></html>';
  await writeFile(join(directory, "index.html"), html);
  await writeFile(join(directory, "keep.txt"), "Keep output");
  const surface = await readSurface(filename);
  assert.equal(surface.buildMs, undefined);
  assert.equal(surface.outputs, undefined);
  const command = await runCli(["--surface", filename, "--out", out]);
  assert.equal(command.code, 0, command.stderr + command.stdout);
  const result = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.equal(result.buildMs, LIMITS.buildMs);
  assert.deepEqual(result.outputs, []);
  assert.equal(await readFile(join(directory, "index.html"), "utf8"), html);
  assert.equal(await readFile(join(directory, "keep.txt"), "utf8"), "Keep output");
}
test(surfacesWithoutBuildKeysKeepTheCommonBoundAndFiles);

async function textClippedToNothingPasses() {
  const { result } = await execute("hidden-text");
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  const { result: visible } = await execute("hidden-text", {
    views: [{ name: "visible-clip", path: "/hidden-text.html?visible" }],
  });
  assertMatrix(visible);
  assert.equal(visible.passed, false);
  assert.deepEqual([...new Set(visible.findings.map((finding) => finding.kind))], ["sideways"]);
  assert.ok(visible.renders.every((render) => render.findings.some((finding) => finding.selector === "#inset")));
}
test(textClippedToNothingPasses);

async function scrollRowsContainSidewaysContent() {
  const { result } = await execute("scroll-row");
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  for (const variant of ["unscrolled", "wide"]) {
    const { result: overflow } = await execute("scroll-row", {
      views: [{ name: variant, path: `/scroll-row.html?${variant}` }],
    });
    assertMatrix(overflow);
    assert.equal(overflow.passed, false);
    assert.deepEqual([...new Set(overflow.findings.map((finding) => finding.kind))], ["sideways"]);
    assert.ok(overflow.renders.every((render) => render.findings.some((finding) => finding.selector === "html" && finding.values.width > finding.values.viewport)));
    if (variant === "wide") assert.ok(overflow.renders.every((render) => render.findings.some((finding) => finding.selector === "#row")));
  }
}
test(scrollRowsContainSidewaysContent);

async function tallFocusKeepsItsStartingEdgeVisible() {
  const { result } = await execute("tall-focus");
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  for (const render of result.renders.filter((render) => render.mode === "keyboard")) assert.deepEqual(render.focused, ["#panel"]);
  for (const variant of ["small", "unmarked"]) {
    const { result: focus } = await execute("tall-focus", {
      views: [{ name: variant, path: `/tall-focus.html?${variant}` }],
    });
    assertMatrix(focus);
    assert.equal(focus.passed, false);
    assert.deepEqual([...new Set(focus.findings.map((finding) => finding.kind))], ["focus"]);
    assert.equal(focus.findings.length, 4);
    assert.ok(focus.findings.every((finding) => finding.mode === "keyboard"));
    if (variant === "small") assert.ok(focus.findings.every((finding) => finding.selector === "#control" && finding.values.outside && finding.values.bottom > finding.values.viewportHeight));
    else assert.ok(focus.findings.every((finding) => finding.selector === "#panel" && !finding.values.visibleChange));
  }
}
test(tallFocusKeepsItsStartingEdgeVisible);

async function modalFocusIncludesTheInitiallyFocusedControl() {
  const { result } = await execute("modal-focus");
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  for (const render of result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.deepEqual([...render.focused].sort(), ["#first", "#second", "#third"]);
  }
}
test(modalFocusIncludesTheInitiallyFocusedControl);

async function modalFocusReportsOnlyTheUnmarkedControl() {
  const { result } = await execute("modal-focus", {
    views: [{ name: "unmarked", path: "/modal-focus.html?unmarked" }],
  });
  assertMatrix(result);
  assert.equal(result.passed, false);
  assert.equal(result.findings.length, 4);
  for (const finding of result.findings) {
    assert.equal(finding.kind, "focus");
    assert.equal(finding.mode, "keyboard");
    assert.equal(finding.selector, "#first");
    assert.equal(finding.values.visibleChange, false);
    assert.equal(finding.values.outside, false);
  }
  for (const render of result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.deepEqual([...render.focused].sort(), ["#first", "#second", "#third"]);
    assert.equal(render.findings.length, 1);
  }
}
test(modalFocusReportsOnlyTheUnmarkedControl);

async function nonmodalFocusStillExpectsBackgroundControls() {
  const { result } = await execute("modal-focus", {
    views: [{ name: "nonmodal", path: "/modal-focus.html?nonmodal" }],
  });
  assertMatrix(result);
  assert.equal(result.passed, false);
  assert.equal(result.findings.length, 4);
  for (const finding of result.findings) {
    assert.equal(finding.kind, "focus");
    assert.equal(finding.mode, "keyboard");
    assert.equal(finding.selector, "#behind");
    assert.equal(finding.values.message, "Element was not reached by Tab");
  }
  for (const render of result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.deepEqual([...render.focused].sort(), ["#background", "#first", "#second", "#third"]);
    assert.equal(render.findings.length, 1);
  }
}
test(nonmodalFocusStillExpectsBackgroundControls);

async function modalFocusTraversesShadowRoots() {
  const { result } = await execute("modal-focus", {
    views: [{ name: "shadow", path: "/modal-focus.html?shadow" }],
  });
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  for (const render of result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.deepEqual([...render.focused].sort(), ["#first", "#second", "#third"]);
  }
}
test(modalFocusTraversesShadowRoots);

async function modalFocusUsesTheTopmostDialog() {
  const { result } = await execute("modal-focus", {
    views: [{ name: "stacked", path: "/modal-focus.html?stacked" }],
  });
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  for (const render of result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.deepEqual([...render.focused].sort(), ["#first", "#second", "#third"]);
  }
}
test(modalFocusUsesTheTopmostDialog);

function refusalView(parameters = "", expects) {
  const refused = `${files.address}/denied?token=a.b`;
  return {
    name: "refusal",
    path: `/expected-answer.html?address=${encodeURIComponent(refused)}${parameters}`,
    ...(expects === undefined ? {} : { expects }),
  };
}

function refusalExpectation() {
  return { method: "GET", pattern: `${files.address}/denied?token=a.b`, status: 404 };
}

async function declaredRefusalResponsesPass() {
  const { result } = await execute("expected-answer", {
    views: [refusalView("&initial", [refusalExpectation()])],
  });
  assertMatrix(result);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
}
test(declaredRefusalResponsesPass);

async function statesDeclareTheirOwnRefusalResponses() {
  const { result } = await execute("expected-answer", {
    views: [{
      ...refusalView(),
      states: [{ name: "refused", expects: [refusalExpectation()], steps: [
        { action: "click", selector: "#request" },
        { action: "wait", text: "Not found" },
      ] }],
    }],
  });
  assert.equal(result.renders.length, 68);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.ok(result.renders.every((render) => render.rendered));
  assert.deepEqual([...new Set(result.renders.map((render) => render.state))], ["default", "refused"]);
}
test(statesDeclareTheirOwnRefusalResponses);

async function missingRefusalResponsesNameTheirExpectation() {
  const entry = refusalExpectation();
  const { result } = await execute("expected-answer", {
    views: [{ ...refusalView("", [entry]), states: [{ name: "ready", steps: [{ action: "wait", text: "Ready" }] }] }],
  });
  assert.equal(result.renders.length, 68);
  assert.equal(result.passed, false);
  assert.ok(result.renders.every((render) => render.rendered));
  assert.equal(result.findings.length, 34);
  for (const finding of result.findings) {
    assert.equal(finding.kind, "load");
    assert.equal(finding.state, "default");
    assert.deepEqual(finding.values, { source: "unmatched-expectation", ...entry });
  }
  assert.ok(result.renders.filter((render) => render.state === "ready").every((render) => render.findings.length === 0));
}
test(missingRefusalResponsesNameTheirExpectation);

async function undeclaredRefusalsAndApplicationErrorsRemainFindings() {
  const second = `${files.address}/another-denial`;
  const { result } = await execute("expected-answer", {
    views: [refusalView(`&initial&second=${encodeURIComponent(second)}&console`, [refusalExpectation()])],
  });
  assertMatrix(result);
  assert.equal(result.passed, false);
  assert.deepEqual([...new Set(result.findings.map((finding) => finding.kind))], ["load"]);
  for (const render of result.renders) {
    assert.ok(render.findings.some((finding) => finding.values.source === "response" && finding.values.address === second && finding.values.status === 404));
    assert.ok(render.findings.some((finding) => finding.values.source === "console" && finding.values.location.url === second));
    assert.ok(render.findings.some((finding) => finding.values.source === "console" && finding.values.message === "Access was refused"));
    assert.ok(!render.findings.some((finding) => finding.values.source === "unmatched-expectation" || finding.values.address === refusalExpectation().pattern || finding.values.location?.url === refusalExpectation().pattern));
  }
}
test(undeclaredRefusalsAndApplicationErrorsRemainFindings);

async function refusalExpectationsRequireMethodPatternAndStatus() {
  for (const entry of [
    { ...refusalExpectation(), method: "POST" },
    { ...refusalExpectation(), pattern: `${files.address}/denied?token=a*b/extra` },
    { ...refusalExpectation(), status: 401 },
  ]) {
    const { result } = await execute("expected-answer", {
      views: [refusalView("&initial", [entry])],
    });
    assertMatrix(result);
    assert.equal(result.passed, false);
    assert.deepEqual([...new Set(result.findings.map((finding) => finding.kind))], ["load"]);
    for (const render of result.renders) {
      assert.ok(render.findings.some((finding) => finding.values.source === "response" && finding.values.status === 404));
      assert.ok(render.findings.some((finding) => finding.values.source === "console" && finding.values.location.url === refusalExpectation().pattern));
      assert.ok(render.findings.some((finding) => finding.values.source === "unmatched-expectation" && finding.values.method === entry.method && finding.values.pattern === entry.pattern && finding.values.status === entry.status));
    }
  }
}
test(refusalExpectationsRequireMethodPatternAndStatus);

async function expectationFieldsAreStrict() {
  for (const expects of [
    {}, [null], [{ pattern: "http://localhost/*", status: 404 }],
    [{ method: "get", pattern: "http://localhost/*", status: 404 }],
    [{ method: "GET", pattern: "/denied", status: 404 }],
    [{ method: "GET", pattern: "http://localhost/*" }],
    ...[399, 600, 404.5, "404"].map((status) => [{ method: "GET", pattern: "http://localhost/*", status }]),
    [{ method: "GET", pattern: "http://localhost/*", status: 404, optional: true }],
  ]) {
    for (const view of [
      { name: "strict", path: "/clean.html", expects },
      { name: "strict", path: "/clean.html", states: [{ name: "ready", steps: [{ action: "wait", text: "Surface" }], expects }] },
    ]) {
      const surface = await surfaceFor("clean", { views: [view] });
      await assert.rejects(readSurface(surface), /must be|unknown field/);
    }
  }
  const surface = await surfaceFor("clean", { views: [{
    name: "strict", path: "/clean.html",
    expects: [400, 599].map((status) => ({ method: "GET", pattern: "http://localhost/a.b?x=*", status })),
  }] });
  const parsed = await readSurface(surface);
  for (const entry of parsed.views[0].expects) {
    assert.equal(entry.matcher.test("http://localhost/a.b?x=1"), true);
    assert.equal(entry.matcher.test("http://localhost/axb?x=1"), false);
    assert.equal(entry.matcher.test("http://localhost/a.b?x=1/extra"), true);
    assert.equal(entry.matcher.test("http://localhost/other/a.b?x=1"), false);
  }
}
test(expectationFieldsAreStrict);

async function viewAndStateAnswersUseTheSurfaceValidationRules() {
  const answer = join(root, "validation-answer.json");
  await writeFile(answer, JSON.stringify({ message: "Answer" }));
  const valid = { method: "GET", pattern: `${externalAddress}/validation`, file: "validation-answer.json" };
  const invalid = [
    { ...valid, unknown: true },
    { ...valid, method: "get" },
    { ...valid, pattern: "/validation" },
    { ...valid, file: "../" },
    ...[199, 600, 404.5, "404"].map((status) => ({ ...valid, status })),
    { ...valid, headers: { "content-type": 4 } },
  ];
  const emptyView = await surfaceFor("clean", { views: [{ name: "view-answers", path: "/clean.html", answers: [] }] });
  assert.deepEqual((await readSurface(emptyView)).views[0].answers, []);
  const emptyState = await surfaceFor("clean", { views: [{ name: "view-answers", path: "/clean.html", states: [{ name: "state-answers", steps: [{ action: "wait", text: "Surface" }], answers: [] }] }] });
  assert.deepEqual((await readSurface(emptyState)).views[0].states[0].answers, []);
  for (const level of ["view", "state"]) {
    for (const entry of invalid) {
      const view = level === "view"
        ? { name: "view-answers", path: "/clean.html", answers: [entry] }
        : { name: "view-answers", path: "/clean.html", states: [{ name: "state-answers", steps: [{ action: "wait", text: "Surface" }], answers: [entry] }] };
      const surface = await surfaceFor("clean", { views: [view] });
      const label = level === "view" ? "View view-answers answers" : "View view-answers state state-answers answers";
      await assert.rejects(readSurface(surface), new RegExp(`${label}.*(?:unknown field|must|Answer files)`));
    }
  }
}
test(viewAndStateAnswersUseTheSurfaceValidationRules);

async function clipboardWritesSucceedWithoutReadPermission() {
  const { result } = await execute("clipboard", { views: [{
    name: "clipboard", path: "/clipboard.html", states: [{ name: "copied", steps: [
      { action: "click", selector: "#copy" },
      { action: "wait", text: "Copied; clipboard read ungranted" },
    ] }],
  }] });
  assert.equal(result.renders.length, 68);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.ok(result.renders.every((render) => render.rendered));
  assert.deepEqual([...new Set(result.renders.map((render) => render.state))], ["default", "copied"]);
}
test(clipboardWritesSucceedWithoutReadPermission);

async function renderDeadlineContinuesWithTheNextView() {
  assert.equal(LIMITS.renderMs, 300000);
  const { result, lines } = await execute("stuck", { views: [
    { name: "stuck", path: "/stuck.html", modes: ["default"] },
    { name: "following", path: "/clean.html", modes: ["default"] },
  ] }, { jobs: 1, renderMs: 3000 });
  assert.equal(result.passed, false);
  assert.equal(result.renders.length, 44);
  const stuck = result.renders.filter((render) => render.view === "stuck");
  assert.equal(stuck.length, 22);
  for (const render of stuck) {
    assert.equal(render.rendered, false);
    assert.equal(render.screenshot, null);
    const deadlines = render.findings.filter((finding) => finding.values.source === "deadline");
    assert.equal(deadlines.length, 1, JSON.stringify(render.findings));
    assert.equal(deadlines[0].kind, "load");
    assert.equal(deadlines[0].values.milliseconds, 3000);
    assert.ok(["navigation", "settle", "measurement", "contrast", "keyboard", "screenshot"].includes(deadlines[0].values.step));
  }
  const following = result.renders.filter((render) => render.view === "following");
  assert.equal(following.length, 22);
  assert.ok(following.every((render) => render.rendered && render.findings.length === 0), JSON.stringify(following));
  assert.equal(following[0].screenshot, "00023.png");
  assert.equal(result.findings.filter((finding) => finding.values.source === "deadline").length, 22);
  assert.match(lines[1], /^following: 22 renders, 0 did not render, 0 showed nothing, 0 findings$/);
}
test(renderDeadlineContinuesWithTheNextView);

async function parallelRendersPreserveResultsAndKeyboardFindings() {
  const options = { views: [
    { name: "clean", path: "/clean.html" },
    { name: "focus", path: "/focus.html" },
    { name: "modal", path: "/modal-focus.html" },
  ] };
  const sequential = await execute("parallel", options, { jobs: 1 });
  const parallel = await execute("parallel", options, { jobs: 4 });
  assert.equal(sequential.result.renders.length, 102);
  assert.ok(sequential.result.renders.every((render) => render.rendered));
  assert.ok(sequential.result.findings.length > 0);
  assert.ok(sequential.result.findings.every((finding) => finding.kind === "focus" && finding.mode === "keyboard"));
  assert.deepEqual(parallel.result.renders, sequential.result.renders);
  assert.deepEqual(parallel.result.findings, sequential.result.findings);
  assert.deepEqual(parallel.result.views, sequential.result.views);
  assert.deepEqual(parallel.lines, sequential.lines);
  assert.deepEqual(sequential.result.renders.map((render) => render.screenshot), Array.from({ length: 102 }, (_, index) => `${String(index + 1).padStart(5, "0")}.png`));
  for (const render of sequential.result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.ok(render.focused.length > 0);
  }
}
test(parallelRendersPreserveResultsAndKeyboardFindings);

async function modesRestrictTheMatrixAndStatesInheritThem() {
  const { result, lines } = await execute("states", { views: [{
    name: "states", path: "/states.html", modes: ["default"],
    states: [{ name: "waiting", steps: [{ action: "wait", text: "Waiting" }] }],
  }] });
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.equal(result.renders.length, 44);
  for (const state of ["default", "waiting"]) {
    const renders = result.renders.filter((render) => render.state === state);
    assert.equal(renders.length, 22);
    assert.ok(renders.every((render) => render.mode === "default" && render.rendered));
    assert.deepEqual(renders.map(({ size, scheme, mode }) => ({ ...size, scheme, mode })), renderMatrix(["default"]));
  }
  assert.deepEqual(result.views, [{ name: "states", modes: ["default"], answers: 0, states: [{ name: "waiting", modes: ["default"], answers: 0 }] }]);
  assert.equal(result.reducedModeCount, 2);
  assert.equal(lines.at(-1), "Total: 44 renders, 0 showed nothing, 0 findings, 0 cancelled, pass, 2 views and states with fewer than all modes");
  const surface = await surfaceFor("states", { views: [{
    name: "states", path: "/states.html", modes: ["default"], states: [
      { name: "all", modes: [...MODES].reverse(), steps: [{ action: "wait", text: "Waiting" }] },
      { name: "keyboard", modes: ["keyboard", "default"], steps: [{ action: "wait", text: "Waiting" }] },
    ],
  }, { name: "all", path: "/clean.html" }] });
  const parsed = await readSurface(surface);
  assert.deepEqual(parsed.views[0].states[0].modes, MODES);
  assert.deepEqual(parsed.views[0].states[1].modes, ["default", "keyboard"]);
  assert.deepEqual(parsed.views[1].modes, MODES);
}
test(modesRestrictTheMatrixAndStatesInheritThem);

async function invalidModesNameTheirView() {
  for (const modes of [[], ["keyboard"], ["default", "default"], ["default", "unknown"], "default", null]) {
    for (const view of [
      { name: "restricted", path: "/clean.html", modes },
      { name: "restricted", path: "/clean.html", states: [{ name: "waiting", modes, steps: [{ action: "wait", text: "Surface" }] }] },
    ]) {
      const surface = await surfaceFor("clean", { views: [view] });
      await assert.rejects(readSurface(surface), /View restricted.*modes/);
    }
  }
}
test(invalidModesNameTheirView);

async function commandValidatesJobsAndKeepsTheDeadlineInternal() {
  for (const jobs of ["0", "-1", "1.5", "no", "Infinity", "9007199254740992", ""]) {
    const result = await runCli(["--list", `--jobs=${jobs}`]);
    assert.notEqual(result.code, 0);
    assert.match(result.stderr, /jobs must be a positive whole number/);
  }
  for (const jobs of ["1", "4"]) {
    const result = await runCli(["--list", "--jobs", jobs]);
    assert.equal(result.code, 0, result.stderr);
    assert.deepEqual(result.stdout.trim().split("\n"), DEVICES.map(({ width, height }) => `${width} x ${height}`));
  }
  const result = await runCli(["--list", "--renderMs", "100"]);
  assert.notEqual(result.code, 0);
  assert.match(result.stderr, /Unknown option/);
  for (const options of [
    { limits: { renderMs: 100 } },
    { renderMs: 100 },
    { views: [{ name: "deadline", path: "/clean.html", renderMs: 100 }] },
    { views: [{ name: "deadline", path: "/clean.html", states: [{ name: "waiting", renderMs: 100, steps: [{ action: "wait", text: "Surface" }] }] }] },
  ]) {
    await assert.rejects(readSurface(await surfaceFor("clean", options)), /unknown field/);
  }
}
test(commandValidatesJobsAndKeepsTheDeadlineInternal);
