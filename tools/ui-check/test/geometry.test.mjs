import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test, { after, before } from "node:test";
import { checkSurface } from "../check.mjs";
import { GEOMETRY_WIDTHS, readSurface } from "../config.mjs";

const fixtures = join(dirname(fileURLToPath(import.meta.url)), "fixtures");
const cli = resolve(dirname(fileURLToPath(import.meta.url)), "../check.mjs");
let root;
let sequence = 0;

before(async function createGeometryDirectory() {
  root = await mkdtemp(join(tmpdir(), "ui-geometry-"));
});
after(async function closeGeometryDirectory() {
  await rm(root, { recursive: true, force: true });
});

async function geometrySurface(options = {}) {
  const directory = await mkdtemp(join(root, "surface-"));
  const filename = join(directory, "surface.json");
  const value = {
    name: "Named parts", directory: fixtures, address: "http://localhost:0", readyPath: "/geometry.html",
    views: [{ name: "parts", path: "/geometry.html", modes: ["default"], parts: [{ name: "part", selector: "#part" }] }],
    answers: [], typeScale: [16, 18, 24], geometry: { file: "geometry.json", widths: [390] }, ...options,
  };
  await writeFile(filename, JSON.stringify(value));
  return { filename, value, file: join(directory, value.geometry?.file ?? "geometry.json"), directory };
}

async function command(args) {
  const child = spawn(process.execPath, [cli, ...args], { stdio: ["ignore", "pipe", "pipe"] });
  let stdout = "";
  let stderr = "";
  child.stdout.on("data", (chunk) => { stdout += chunk; });
  child.stderr.on("data", (chunk) => { stderr += chunk; });
  const code = await new Promise((resolve, reject) => { child.once("error", reject); child.once("close", resolve); });
  return { code, stdout, stderr };
}

async function capture(surface, args = []) {
  const result = await command(["--surface", surface.filename, "--write-geometry", ...args]);
  assert.equal(result.code, 0, result.stderr + result.stdout);
  return result;
}

async function inspect(surface, options = {}) {
  const lines = [];
  const out = join(root, `renders-${++sequence}`);
  const result = await checkSurface({ surface: surface.filename, out, ...options }, (line) => lines.push(line));
  assert.deepEqual(JSON.parse(await readFile(join(out, "result.json"), "utf8")), result);
  return { result, lines };
}

async function geometryWritesOnlyTheStableBaselineAndReadsItBack(context) {
  const surface = await geometrySurface({ geometry: { file: "saved/geometry.json" }, views: [
    { name: "zeta", path: "/geometry.html", modes: ["default"], parts: [{ name: "z", selector: "#other" }, { name: "a", selector: "#part" }] },
    { name: "alpha", path: "/geometry.html", modes: ["default"], parts: [{ name: "a", selector: "#part" }], states: [{ name: "absent", steps: [{ action: "click", selector: "#absent" }] }] },
    { name: "unmeasured", path: "/missing.html", modes: ["default"] },
  ] });
  const out = join(surface.directory, "unused-output");
  const captured = await capture(surface, ["--out", out]);
  assert.match(captured.stdout, /Total: 10 renders, 0 showed nothing, 0 findings/);
  assert.deepEqual((await readdir(surface.directory)).sort(), ["saved", "surface.json"]);
  assert.deepEqual(await readdir(dirname(surface.file)), ["geometry.json"]);
  const first = await readFile(surface.file, "utf8");
  const stored = JSON.parse(first);
  assert.equal(stored.version, 1);
  assert.equal(stored.entries.length, 15);
  assert.deepEqual(stored.entries.map(({ view, width, part }) => [view, width, part]), [
    ...GEOMETRY_WIDTHS.map((width) => ["alpha", width, "a"]),
    ...GEOMETRY_WIDTHS.flatMap((width) => [["zeta", width, "a"], ["zeta", width, "z"]]),
  ]);
  for (const entry of stored.entries.filter((entry) => entry.part === "a")) {
    assert.deepEqual(entry.values, { left: 32, top: 64, width: 120, height: 48, fontSize: 16, lineHeight: 24 });
  }
  await capture(surface, ["--jobs", "1"]);
  assert.equal(await readFile(surface.file, "utf8"), first);
  surface.value.views = surface.value.views.slice(0, 1);
  await writeFile(surface.filename, JSON.stringify(surface.value));
  const { result } = await inspect(surface);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.deepEqual(result.findings, []);
  const measured = result.renders.filter((render) => render.geometry);
  assert.deepEqual(measured.map((render) => render.size.width), GEOMETRY_WIDTHS);
  assert.ok(measured.every((render) => render.state === "default" && render.mode === "default" && render.scheme === "dark"));
  assert.equal(await readFile(surface.file, "utf8"), first);
  context.diagnostic(captured.stdout.trim());
}
test(geometryWritesOnlyTheStableBaselineAndReadsItBack);

async function geometryReportsOneFindingForEachChangedPart(context) {
  const surface = await geometrySurface();
  await capture(surface);
  const original = await readFile(surface.file, "utf8");
  const stored = JSON.parse(original).entries[0].values;
  for (const change of ["left=2", "top=2", "width=2", "height=2", "fontSize=2", "lineHeight=2", "missing"]) {
    surface.value.views[0].path = `/geometry.html?${change}`;
    await writeFile(surface.filename, JSON.stringify(surface.value));
    const { result, lines } = await inspect(surface);
    assert.equal(result.passed, false, change);
    assert.equal(result.findings.length, 1, JSON.stringify(result.findings));
    const [finding] = result.findings;
    assert.equal(finding.kind, "geometry");
    assert.equal(finding.view, "parts");
    assert.equal(finding.state, "default");
    assert.equal(finding.size.width, 390);
    assert.equal(finding.scheme, "dark");
    assert.equal(finding.mode, "default");
    assert.equal(finding.part, "part");
    assert.equal(finding.selector, "#part");
    assert.deepEqual(finding.values.stored, stored);
    if (change === "missing") assert.equal(finding.values.measured, null);
    else {
      const key = change.split("=")[0];
      assert.equal(finding.values.measured[key], stored[key] + 2);
    }
    assert.equal(await readFile(surface.file, "utf8"), original);
    context.diagnostic(`${change}: ${finding.values.message}; ${lines.at(-1)}`);
  }
}
test(geometryReportsOneFindingForEachChangedPart);

async function geometryAllowsExactlyOnePixelAndNormalLineHeight() {
  const surface = await geometrySurface();
  await capture(surface);
  surface.value.views[0].path = "/geometry.html?left=1&top=1&width=1&height=1&fontSize=1&lineHeight=1";
  surface.value.typeScale.push(17);
  await writeFile(surface.filename, JSON.stringify(surface.value));
  const { result } = await inspect(surface);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.deepEqual(result.findings, []);
  surface.value.views[0].path = "/geometry.html?normal";
  await writeFile(surface.filename, JSON.stringify(surface.value));
  await capture(surface);
  assert.equal(JSON.parse(await readFile(surface.file, "utf8")).entries[0].values.lineHeight, "normal");
  const normal = await inspect(surface);
  assert.equal(normal.result.passed, true, JSON.stringify(normal.result.findings));
  assert.deepEqual(normal.result.findings, []);
}
test(geometryAllowsExactlyOnePixelAndNormalLineHeight);

async function listedAndStoredPartsRemainAccountedFor(context) {
  const surface = await geometrySurface();
  await capture(surface);
  surface.value.views[0].parts.push({ name: "other", selector: "#other" });
  await writeFile(surface.filename, JSON.stringify(surface.value));
  const added = await inspect(surface);
  assert.equal(added.result.passed, false);
  assert.equal(added.result.findings.length, 1);
  assert.equal(added.result.findings[0].kind, "geometry");
  assert.equal(added.result.findings[0].part, "other");
  assert.equal(added.result.findings[0].values.stored, null);
  assert.deepEqual(added.result.findings[0].values.measured, { left: 32, top: 200, width: 120, height: 48, fontSize: 16, lineHeight: 24 });
  context.diagnostic(added.result.findings[0].values.message);
  await capture(surface);
  surface.value.views[0].parts.pop();
  surface.value.views[0].path = "/geometry.html?missingOther";
  await writeFile(surface.filename, JSON.stringify(surface.value));
  const removed = await inspect(surface);
  assert.equal(removed.result.passed, false);
  assert.equal(removed.result.findings.length, 1);
  assert.equal(removed.result.findings[0].kind, "geometry");
  assert.equal(removed.result.findings[0].part, "other");
  assert.equal(removed.result.findings[0].values.measured, null);
  context.diagnostic(removed.result.findings[0].values.message);
}
test(listedAndStoredPartsRemainAccountedFor);

async function selectedViewWritesOnlyItsEntries() {
  const surface = await geometrySurface({ views: [
    { name: "first", path: "/geometry.html", modes: ["default"], parts: [{ name: "part", selector: "#part" }] },
    { name: "second", path: "/geometry.html", modes: ["default"], parts: [{ name: "part", selector: "#part" }] },
  ] });
  await capture(surface);
  const before = JSON.parse(await readFile(surface.file, "utf8"));
  surface.value.views[0].path = "/geometry.html?left=2";
  surface.value.views[1].path = "/missing.html";
  await writeFile(surface.filename, JSON.stringify(surface.value));
  const captured = await capture(surface, ["--view", "first"]);
  assert.match(captured.stdout, /Total: 1 renders/);
  const after = JSON.parse(await readFile(surface.file, "utf8"));
  assert.deepEqual(after.entries.filter((entry) => entry.view === "second"), before.entries.filter((entry) => entry.view === "second"));
  assert.equal(after.entries[0].values.left, before.entries[0].values.left + 2);
  const { result } = await inspect(surface, { view: "first" });
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.deepEqual(result.findings, []);
}
test(selectedViewWritesOnlyItsEntries);

async function customGeometryWidthsAreMeasuredOnEveryRun() {
  const surface = await geometrySurface({ geometry: { file: "geometry.json", widths: [1440, 401] } });
  const captured = await capture(surface);
  assert.match(captured.stdout, /Total: 2 renders/);
  assert.deepEqual(JSON.parse(await readFile(surface.file, "utf8")).entries.map((entry) => entry.width), [401, 1440]);
  const { result } = await inspect(surface);
  assert.equal(result.passed, true, JSON.stringify(result.findings));
  assert.equal(result.renders.length, 23);
  assert.deepEqual(result.renders.filter((render) => render.geometry).map((render) => render.size.width).sort((a, b) => a - b), [401, 1440]);
}
test(customGeometryWidthsAreMeasuredOnEveryRun);

async function unsuccessfulGeometryCaptureKeepsTheStoredFile(context) {
  const surface = await geometrySurface();
  await capture(surface);
  const stored = await readFile(surface.file, "utf8");
  for (const path of ["/geometry.html?missing", "/missing.html", "/geometry.html?loadError"]) {
    surface.value.views[0].path = path;
    await writeFile(surface.filename, JSON.stringify(surface.value));
    const result = await command(["--surface", surface.filename, "--write-geometry"]);
    assert.equal(result.code, 1, result.stderr + result.stdout);
    assert.match(result.stdout, /fail/);
    assert.equal(await readFile(surface.file, "utf8"), stored);
    assert.deepEqual((await readdir(surface.directory)).sort(), ["geometry.json", "surface.json"]);
    context.diagnostic(result.stdout.trim());
  }
}
test(unsuccessfulGeometryCaptureKeepsTheStoredFile);

async function reportedKindsKeepTheirContextAndDetermineTheExitStatus(context) {
  const surface = await geometrySurface({ geometry: undefined, reportedKinds: ["scale"], views: [{ name: "scale", path: "/scale.html", modes: ["default"] }] });
  let reported;
  for (const reporting of [true, false]) {
    if (!reporting) delete surface.value.reportedKinds;
    await writeFile(surface.filename, JSON.stringify(surface.value));
    const out = join(root, `reported-${++sequence}`);
    const result = await command(["--surface", surface.filename, "--out", out]);
    assert.equal(result.code, reporting ? 0 : 1, result.stderr + result.stdout);
    const report = JSON.parse(await readFile(join(out, "result.json"), "utf8"));
    assert.equal(report.passed, reporting);
    assert.equal(report.findings.length, 22);
    assert.ok(report.renders.every((render) => render.rendered && render.findings.length === 1));
    for (const finding of report.findings) {
      assert.equal(finding.kind, "scale");
      assert.equal(finding.view, "scale");
      assert.equal(finding.state, "default");
      assert.equal(finding.mode, "default");
      assert.ok(["light", "dark"].includes(finding.scheme));
      assert.equal(typeof finding.size.width, "number");
      assert.ok(finding.selector);
      assert.equal(finding.values.fontSize, 17);
      assert.deepEqual(finding.values.typeScale, [16, 18, 24]);
    }
    if (reporting) {
      reported = report;
      assert.equal(report.reportedCount, 22);
      assert.equal(report.failingCount, 0);
      assert.match(result.stdout, /^scale: 22 renders, 0 did not render, 0 showed nothing, 22 findings, 22 reported, 0 failing, 0 retried\n/);
      assert.match(result.stdout, /Total: 22 renders, 0 showed nothing, 22 findings, 22 reported, 0 failing, 0 cancelled, pass/);
    } else assert.deepEqual(report.findings, reported.findings);
    context.diagnostic(result.stdout.trim());
  }
}
test(reportedKindsKeepTheirContextAndDetermineTheExitStatus);

async function reportedKindsLeaveGeometryAndLoadingFailuresFatal() {
  const surface = await geometrySurface({ reportedKinds: ["scale"] });
  await capture(surface);
  surface.value.views[0].path = "/geometry.html?fontSize=3&loadError";
  await writeFile(surface.filename, JSON.stringify(surface.value));
  const { result, lines } = await inspect(surface);
  assert.equal(result.passed, false);
  assert.equal(result.reportedCount, 22);
  assert.equal(result.failingCount, 23);
  assert.equal(result.findings.filter((finding) => finding.kind === "geometry").length, 1);
  assert.equal(result.findings.filter((finding) => finding.kind === "load").length, 22);
  assert.match(lines.at(-1), /45 findings, 22 reported, 23 failing, 0 cancelled, fail/);
}
test(reportedKindsLeaveGeometryAndLoadingFailuresFatal);

async function invalidReportedKindsAreRejectedBeforeRendering(context) {
  for (const reportedKinds of [["geometry"], ["load"], ["surface"], ["render"], ["unknown"], ["scale", "scale"], [null], "scale", null]) {
    const surface = await geometrySurface({ reportedKinds, build: "exit 9" });
    const out = join(surface.directory, "no-output");
    const result = await command(["--surface", surface.filename, "--out", out]);
    assert.equal(result.code, 1);
    assert.match(result.stderr, /surface.reportedKinds/);
    assert.equal(result.stdout, "");
    assert.deepEqual(await readdir(surface.directory), ["surface.json"]);
    context.diagnostic(result.stderr.trim());
  }
  const surface = await geometrySurface({ reportedKinds: ["sideways", "cut", "contrast", "touch", "focus", "shift", "motion", "scale"] });
  assert.deepEqual((await readSurface(surface.filename)).reportedKinds, surface.value.reportedKinds);
}
test(invalidReportedKindsAreRejectedBeforeRendering);

async function geometryConfigurationAndFileAreStrict(context) {
  for (const geometry of [null, "geometry.json", {}, { file: "" }, { file: "/geometry.json" }, { file: "C:\\geometry.json" }, { file: "surface.json" }, { file: "geometry.json", widths: null }, { file: "geometry.json", widths: [] }, { file: "geometry.json", widths: [0] }, { file: "geometry.json", widths: [390.5] }, { file: "geometry.json", widths: ["390"] }, { file: "geometry.json", widths: [390, 390] }, { file: "geometry.json", optional: true }]) {
    const surface = await geometrySurface({ geometry });
    await assert.rejects(readSurface(surface.filename), /surface.geometry/);
  }
  for (const parts of [null, "part", [{}], [{ name: "part" }], [{ name: "part", selector: "#part", optional: true }], [{ name: "part", selector: "#part" }, { name: "part", selector: "#other" }]]) {
    const surface = await geometrySurface({ views: [{ name: "parts", path: "/geometry.html", parts }] });
    await assert.rejects(readSurface(surface.filename), /View parts part/);
  }
  const surface = await geometrySurface();
  await assert.rejects(inspect(surface), { code: "ENOENT" });
  await capture(surface);
  const valid = JSON.parse(await readFile(surface.file, "utf8"));
  for (const value of [
    { ...valid, version: 2 }, { ...valid, optional: true }, { ...valid, entries: [...valid.entries, ...valid.entries] },
    { ...valid, entries: [{ ...valid.entries[0], values: { ...valid.entries[0].values, left: "32" } }] },
  ]) {
    await writeFile(surface.file, JSON.stringify(value));
    await assert.rejects(inspect(surface), /Geometry/);
  }
  const without = await geometrySurface({ geometry: undefined });
  const result = await command(["--surface", without.filename, "--write-geometry"]);
  assert.equal(result.code, 1);
  assert.match(result.stderr, /requires surface.geometry/);
  context.diagnostic(result.stderr.trim());
}
test(geometryConfigurationAndFileAreStrict);
