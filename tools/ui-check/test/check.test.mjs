import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { mkdtemp, readFile, readdir, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test, { after, before } from "node:test";
import { checkSurface } from "../check.mjs";
import { DEVICES, REPOSITORY, renderMatrix } from "../config.mjs";
import { serveFiles } from "../server.mjs";

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

before(async function serveTestPages() {
  root = await mkdtemp(join(tmpdir(), "ui-check-"));
  server = createServer((request, response) => {
    if (request.url === "/never") return;
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

async function execute(name, options = {}) {
  const surface = await surfaceFor(name, options);
  const out = join(root, `renders-${sequence}`);
  const lines = [];
  const result = await checkSurface({ surface, out }, (line) => lines.push(line));
  assert.deepEqual(JSON.parse(await readFile(join(out, "result.json"), "utf8")), result);
  return { result, out, lines };
}

function assertMatrix(result) {
  assert.equal(result.renders.length, 34);
  assert.deepEqual([...new Set(result.renders.map((render) => render.size.width))], widths);
  assert.equal(result.renders.filter((render) => render.size.width === 844 && render.size.height === 390).length, 2);
  for (const render of result.renders) {
    assert.ok(render.rendered, JSON.stringify(render.findings));
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
  assert.deepEqual(result.findings, []);
  assert.equal(lines.length, 2);
  assert.match(lines[0], /^clean: 34 renders, 0 did not render, 0 findings$/);
  assert.match(lines[1], /^Total: 34 renders, 0 findings, pass$/);
  assert.equal((await readdir(out)).filter((file) => file.endsWith(".png")).length, 34);
  const png = await readFile(join(out, result.renders[0].screenshot));
  assert.equal(png.readUInt32BE(16), 360);
  assert.ok(png.readUInt32BE(20) > 900, "The screenshot includes content below the viewport");
  for (const render of result.renders.filter((entry) => entry.mode === "keyboard")) {
    assert.equal(render.focused.length, 2);
  }
}
test(cleanPagePassesWholeDeviceList);

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
      for (const source of ["console", "pageerror", "unhandledrejection", "response", "requestfailed", "unanswered-request"]) assert.ok(sources.has(source), source);
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
  assert.match(command.stdout, /findings, fail/);
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

async function commandBuildsAndServesFiles() {
  const surface = await surfaceFor("clean", {
    build: `${JSON.stringify(process.execPath)} -e "process.stdout.write('Surface ready')"`,
    start: undefined, address: "http://localhost:0",
  });
  const result = await runCli(["--surface", surface, "--out", join(root, "self-served")]);
  assert.equal(result.code, 0, result.stderr + result.stdout);
  assert.match(result.stdout, /34 renders, 0 findings, pass/);
}
test(commandBuildsAndServesFiles);
