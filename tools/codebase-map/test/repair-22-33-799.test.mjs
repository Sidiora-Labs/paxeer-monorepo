import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { generateSnapshot, parseStatus, parseTrackedFiles, readActiveSpec } from "../generate.mjs";
import { DeliveryMap, deliveryGraph, loadSnapshot, TASK_PAGE_SIZE, WAVE_PAGE_SIZE } from "../delivery.mjs";
import { createMapServer } from "../serve.mjs";

const toolRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = resolve(toolRoot, "../..");
const fixtures = join(toolRoot, "test/fixtures/repair-22-33-799");
const fixtureRoot = join(fixtures, "repository");
const transcript = JSON.parse(readFileSync(join(fixtures, "git.json"), "utf8"));
const colors = { task: "#8695a8", in_progress: "#efb45e", pending: "#5e6b7b" };

function produceSnapshot(platform = readActiveSpec(fixtureRoot)) {
  const database = new DatabaseSync(":memory:");
  try {
    database.exec(readFileSync(join(fixtures, "graph.sql"), "utf8"));
    return generateSnapshot({
      database, root: repositoryRoot, platform,
      tracked: parseTrackedFiles(Buffer.from(transcript.tracked)),
      statusByPath: parseStatus(Buffer.from(transcript.status)),
      commit: transcript.commit, branch: transcript.branch,
      graphUpdatedAt: transcript.graphUpdatedAt,
    });
  } finally {
    database.close();
  }
}

async function serveSnapshot(t, snapshot) {
  const root = mkdtempSync(join(tmpdir(), "codebase-map-repair-"));
  mkdirSync(join(root, "data"));
  writeFileSync(join(root, "data/codebase-map.json"), JSON.stringify(snapshot));
  const server = createMapServer(root);
  const requests = [];
  server.on("request", (request) => requests.push({ method: request.method, url: request.url, headers: request.headers }));
  t.after(async () => {
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
    rmSync(root, { recursive: true });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const url = `http://127.0.0.1:${server.address().port}/data/codebase-map.json`;
  const response = await fetch(url);
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("content-type"), "application/json; charset=utf-8");
  assert.equal(response.headers.get("cache-control"), "no-store");
  assert.equal(response.headers.get("x-content-type-options"), "nosniff");
  assert.deepEqual(await response.json(), snapshot);
  const received = await loadSnapshot(url);
  assert.deepEqual(received, snapshot);
  assert.equal(requests.length, 2);
  for (const request of requests) {
    assert.equal(request.method, "GET");
    assert.equal(request.url, "/data/codebase-map.json");
    assert.equal(request.headers.host, new URL(url).host);
  }
  assert.equal(requests[1].headers["cache-control"], "no-cache");
  return { snapshot: received, delivery: new DeliveryMap(received.tasks), url, requests };
}

async function activeFeatureSnapshotWalkthrough(t) {
  const active = readActiveSpec(repositoryRoot);
  assert.equal(active.feature, "layerx-merge");
  assert.equal(active.specPath, "spec/layerx-merge/spec.kvx");
  assert.ok(active.tasks.some((task) => task.id === "22.33.799"));
  const produced = produceSnapshot(active);
  const { snapshot, delivery } = await serveSnapshot(t, produced);
  assert.equal(snapshot.schemaVersion, 1);
  assert.equal(snapshot.meta.commit, transcript.commit);
  assert.deepEqual(snapshot.meta.sources, [
    "git ls-files --stage", "git status --porcelain=v1", ".codegraph/graph.db schema 14", "spec/layerx-merge/spec.kvx",
  ]);
  assert.equal(snapshot.coverage.trackedFiles, 3);
  assert.equal(snapshot.coverage.unionFiles, 3);
  assert.equal(snapshot.coverage.graphFiles, 0);
  assert.equal(snapshot.coverage.tasks, active.tasks.filter((task) => task.deliverable).length);
  const found = delivery.search("22.33.799").find((result) => result.id === "22.33.799");
  assert.ok(found, "the active task is searchable after transport");
  const selected = delivery.view(delivery.scopeForTask(found.id));
  assert.ok(selected.items.some((task) => task.id === found.id));
  assert.deepEqual(delivery.tasks.get(found.id).touches, ["tools/codebase-map"]);
}

async function sectionBoundariesSurviveTransport(t) {
  const { snapshot, delivery } = await serveSnapshot(t, produceSnapshot());
  assert.deepEqual(snapshot.requirements, [
    { id: "22998", title: "Repair audited paths" },
    { id: "final", title: "Final requirement" },
  ]);
  const original = delivery.tasks.get("repair.1201");
  assert.deepEqual(original, {
    id: "repair.1201", title: "Deliver repair 1201", status: "in_progress", wave: 0,
    section: null, requires: ["repair.1200"], reqs: ["22998.1"],
    symbols: ["parsePlatformSpec"], touches: ["tools/codebase-map/generate.mjs"],
    verifyCommand: "node --test tools/codebase-map/test/repair-22-33-799.test.mjs", deliverable: true,
  });
  assert.equal(snapshot.tasks.length, 2487);
  assert.equal(snapshot.coverage.tasks, 2486);
  assert.equal(snapshot.coverage.parentTaskGroups, 1);
  assert.equal(delivery.tasks.get("group").title, "Parent group");
  assert.equal(delivery.tasks.get("repair.2402").status, "in_progress");
  assert.ok(!delivery.byWave.has(999));
  assert.equal(delivery.search("Decision must not overwrite task").length, 0);
  assert.ok(snapshot.taskLinks.some((link) => link.task === original.id && link.touch === original.touches[0]));
}

async function deliveryWaveExpansionAndSearch(t) {
  const { delivery } = await serveSnapshot(t, produceSnapshot());
  const overview = delivery.view();
  assert.equal(overview.expanded, false);
  assert.equal(overview.total, 85);
  assert.equal(overview.items.length, WAVE_PAGE_SIZE);
  assert.deepEqual(overview.items[0], { wave: 0, count: 1202 });
  const aggregate = deliveryGraph(delivery, {}, colors);
  assert.equal(aggregate.layout, "fixed");
  assert.deepEqual(aggregate.edges.find((edge) => edge.source === "w:0" && edge.target === "w:4"), {
    source: "w:0", target: "w:4", total: 1, kind: "requires",
  });
  const zeroWave = delivery.view({ wave: 0 });
  assert.equal(zeroWave.expanded, true, "wave zero is an explicit expansion");
  assert.equal(zeroWave.total, 1202);
  assert.equal(zeroWave.items.length, TASK_PAGE_SIZE);
  assert.deepEqual(zeroWave.edges[0], { source: "t:repair.0", target: "t:repair.1", total: 1, kind: "requires" });
  const search = delivery.search("repair.1201");
  const result = search.find((item) => item.id === "repair.1201");
  assert.ok(result, "search includes tasks outside the first page");
  assert.deepEqual(delivery.scopeForTask(result.id), { wave: 0, page: 7 });
  const expanded = deliveryGraph(delivery, delivery.scopeForTask(result.id), colors);
  assert.ok(expanded.nodes.some((node) => node.entityId === result.id && node.secondary === "Deliver repair 1201"));
  assert.equal(expanded.layout, "fixed");
  assert.ok(delivery.search("Parent group").some((item) => item.id === "group"));
  assert.ok(delivery.view({ page: 1 }).items.some((item) => item.wave === 87));
  assert.deepEqual(delivery.scopeForTask("wave.87"), { wave: 87, page: 0 });
  assert.ok(delivery.view(delivery.scopeForTask("wave.87")).items.some((item) => item.id === "wave.87"));
}

async function deliveryScaleRemainsBounded(t) {
  const { snapshot, delivery } = await serveSnapshot(t, produceSnapshot());
  const reached = new Set();
  for (let page = 0; page < delivery.view().pageCount; page += 1) {
    const waves = delivery.view({ page });
    assert.ok(waves.items.length <= WAVE_PAGE_SIZE);
    for (const { wave } of waves.items) {
      for (let taskPage = 0; taskPage < delivery.view({ wave }).pageCount; taskPage += 1) {
        const view = delivery.view({ wave, page: taskPage });
        const graph = deliveryGraph(delivery, { wave, page: taskPage }, colors);
        assert.ok(graph.nodes.length <= TASK_PAGE_SIZE);
        assert.equal(graph.nodes.length, view.items.length);
        assert.equal(graph.layout, "fixed");
        const points = new Set();
        const ids = new Set(graph.nodes.map((node) => node.id));
        for (const node of graph.nodes) {
          assert.ok(Number.isFinite(node.initialX) && Number.isFinite(node.initialY));
          points.add(`${node.initialX},${node.initialY}`);
          reached.add(node.entityId);
          assert.deepEqual(delivery.scopeForTask(node.entityId), { wave, page: taskPage });
        }
        assert.equal(points.size, graph.nodes.length, "fixed layout gives each task a distinct position");
        for (const edge of graph.edges) assert.ok(ids.has(edge.source) && ids.has(edge.target));
      }
    }
  }
  assert.equal(reached.size, snapshot.coverage.tasks, "explicit expansion and paging reach every deliverable");
  for (const task of snapshot.tasks.filter((task) => task.deliverable)) assert.ok(reached.has(task.id));
  assert.equal(delivery.byWave.get(0).length, 1202, "paging retains the complete grouped inventory");
  assert.equal(delivery.view({ wave: 0, page: -1 }).page, 0);
  assert.equal(delivery.view({ wave: 0, page: 999 }).page, 7);
}

function authoritativeSourceIsRequired(t) {
  const root = mkdtempSync(join(tmpdir(), "codebase-map-config-"));
  t.after(() => rmSync(root, { recursive: true }));
  assert.throws(() => readActiveSpec(root), /Missing spec configuration spec\/workflow\.kvx/);
  mkdirSync(join(root, "spec"));
  writeFileSync(join(root, "spec/workflow.kvx"), '[meta]\nactive_feature = "chosen"\n');
  assert.throws(() => readActiveSpec(root), /Missing authoritative spec\/chosen\/spec\.kvx/);
  mkdirSync(join(root, "spec/chosen"));
  const source = readFileSync(join(fixtureRoot, "spec/layerx-merge/spec.kvx"), "utf8");
  writeFileSync(join(root, "spec/chosen/spec.kvx"), source);
  assert.equal(readActiveSpec(root).specPath, "spec/chosen/spec.kvx", "configuration selects the source without a hardcoded feature");
  writeFileSync(join(root, "spec/workflow.kvx"), '[meta]\nactive_feature = "../outside"\n');
  assert.throws(() => readActiveSpec(root), /Missing or invalid \[meta\] active_feature/);
}

async function readmeWalkthroughMatchesGenerator(t) {
  const readme = readFileSync(join(toolRoot, "README.md"), "utf8");
  assert.match(readme, /spec\/layerx-merge\/spec\.kvx/);
  assert.match(readme, /active_feature/);
  assert.match(readme, /schema version \*\*14\*\*/);
  for (const command of ["cg init", "cg sync", "npm --prefix tools/codebase-map run generate", "npm --prefix tools/codebase-map run serve"]) {
    assert.ok(readme.includes(command), `walkthrough documents ${command}`);
  }
  assert.doesNotMatch(readme, /layerx-platform|spec\/paxeer-x|schema 10|generator migration/);
  const { snapshot, delivery, url } = await serveSnapshot(t, produceSnapshot(readActiveSpec(repositoryRoot)));
  assert.ok(snapshot.meta.sources.includes(readActiveSpec(repositoryRoot).specPath));
  assert.equal(snapshot.meta.codifySchemaVersion, "14");
  assert.equal(deliveryGraph(delivery, {}, colors).layout, "fixed");
  const missing = url.replace("codebase-map.json", "missing.json");
  await assert.rejects(() => loadSnapshot(missing), /Map data returned HTTP 404/);
}

test(activeFeatureSnapshotWalkthrough);
test(sectionBoundariesSurviveTransport);
test(deliveryWaveExpansionAndSearch);
test(deliveryScaleRemainsBounded);
test(authoritativeSourceIsRequired);
test(readmeWalkthroughMatchesGenerator);
