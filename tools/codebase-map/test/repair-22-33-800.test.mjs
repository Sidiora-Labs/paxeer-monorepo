import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync, rmSync, unlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { parseStatus, parseTrackedFiles, readActiveSpec } from "../generate.mjs";
import { loadSnapshot } from "../delivery.mjs";
import { readGeneratedSnapshot, registerMapTests, setupSnapshot } from "./map.test.mjs";

const toolRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = resolve(toolRoot, "../..");
const fixtures = join(toolRoot, "test/fixtures/repair-22-33-800");
const priorFixtures = join(toolRoot, "test/fixtures/repair-22-33-799");
const graph = JSON.parse(readFileSync(join(fixtures, "graph.json"), "utf8"));
const git = JSON.parse(readFileSync(join(fixtures, "git.json"), "utf8"));

function insertRecordedRow(database, table, row) {
  const columns = Object.keys(row);
  database.prepare(`INSERT INTO ${table} (${columns.join(", ")}) VALUES (${columns.map(() => "?").join(", ")})`)
    .run(...columns.map((column) => row[column]));
}

function recordedSources() {
  const database = new DatabaseSync(":memory:");
  try {
    database.exec(readFileSync(join(priorFixtures, "graph.sql"), "utf8"));
    for (const table of ["files", "symbols", "refs", "imports"]) {
      for (const row of graph[table]) insertRecordedRow(database, table, row);
    }
    return {
      root: repositoryRoot, database,
      tracked: parseTrackedFiles(Buffer.from(git.tracked)),
      statusByPath: parseStatus(Buffer.from(git.status)),
      commit: git.commit, branch: git.branch, graphUpdatedAt: git.graphUpdatedAt,
    };
  } catch (error) {
    database.close();
    throw error;
  }
}

registerMapTests(recordedSources);

async function missingGeneratedSnapshotFailsQualification(t) {
  const sources = recordedSources();
  t.after(() => sources.database.close());
  const setup = await setupSnapshot(sources);
  t.after(() => setup.close());
  const snapshot = readGeneratedSnapshot(setup.dataPath);
  assert.equal(snapshot.coverage.graphFiles, 3);
  assert.equal(snapshot.coverage.symbols, 2);
  assert.equal(snapshot.coverage.references, 1);
  assert.equal(snapshot.coverage.imports, 1);
  assert.equal(snapshot.symbolEdges.length, 1);
  assert.ok(snapshot.fileEdges.length > 0);
  assert.ok(snapshot.tasks.some((task) => task.id === "22.33.800"));
  const url = new URL("data/codebase-map.json", setup.baseUrl);
  assert.deepEqual(await loadSnapshot(url), snapshot);
  unlinkSync(setup.dataPath);
  assert.throws(() => readGeneratedSnapshot(setup.dataPath), { code: "ENOENT" });
  const response = await fetch(url);
  assert.equal(response.status, 404);
  assert.equal(response.headers.get("content-type"), "text/plain; charset=utf-8");
  assert.equal(await response.text(), "Not found\n");
  await assert.rejects(() => loadSnapshot(url), /Map data returned HTTP 404/);
  assert.equal(setup.requests.length, 3);
  for (const request of setup.requests) {
    assert.equal(request.method, "GET");
    assert.equal(request.url, "/data/codebase-map.json");
    assert.equal(request.host, url.host);
  }
}

async function setupRegeneratesFromCurrentGraphAndActiveSpec(t) {
  const sources = recordedSources();
  t.after(() => sources.database.close());
  const first = await setupSnapshot(sources);
  t.after(() => first.close());
  const original = await loadSnapshot(new URL("data/codebase-map.json", first.baseUrl));
  assert.deepEqual(original.tasks, readActiveSpec(repositoryRoot).tasks);
  assert.equal(original.coverage.symbols, 2);
  insertRecordedRow(sources.database, "symbols", graph.additionalSymbol);
  const selectedRoot = join(priorFixtures, "repository");
  const second = await setupSnapshot({ ...sources, root: selectedRoot });
  t.after(() => second.close());
  assert.notEqual(first.dataPath, second.dataPath);
  const current = await loadSnapshot(new URL("data/codebase-map.json", second.baseUrl));
  assert.equal(current.coverage.symbols, 3);
  assert.ok(current.symbols.some((symbol) => symbol.name === graph.additionalSymbol.name));
  const active = readActiveSpec(selectedRoot);
  assert.deepEqual(current.tasks, active.tasks);
  assert.deepEqual(current.requirements, active.requirements);
  assert.ok(current.meta.sources.includes(active.specPath));
  assert.equal(current.coverage.tasks, 2486);
  assert.notDeepEqual(current.tasks, original.tasks);
  assert.deepEqual(readGeneratedSnapshot(first.dataPath), original, "a prior output remains separate from the fresh source snapshot");
}

async function setupRejectsMissingAuthorityAndUnsupportedGraph(t) {
  const sources = recordedSources();
  t.after(() => sources.database.close());
  const setup = await setupSnapshot(sources);
  t.after(() => setup.close());
  const emptyRoot = mkdtempSync(join(tmpdir(), "codebase-map-no-authority-"));
  t.after(() => rmSync(emptyRoot, { recursive: true }));
  await assert.rejects(() => setupSnapshot({ ...sources, root: emptyRoot }), /Missing spec configuration spec\/workflow\.kvx/);
  sources.database.prepare("UPDATE meta SET value = ? WHERE key = 'schema_version'").run("unsupported");
  await assert.rejects(() => setupSnapshot(sources), /Unsupported Codify schema unsupported; expected 14/);
  assert.ok(existsSync(setup.dataPath), "existing output cannot substitute for invalid current sources");
}

test(missingGeneratedSnapshotFailsQualification);
test(setupRegeneratesFromCurrentGraphAndActiveSpec);
test(setupRejectsMissingAuthorityAndUnsupportedGraph);
