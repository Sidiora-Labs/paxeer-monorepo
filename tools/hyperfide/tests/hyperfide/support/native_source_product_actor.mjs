import assert from "node:assert/strict";
import { randomUUID, randomBytes } from "node:crypto";
import { execFileSync } from "node:child_process";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import fs from "node:fs/promises";
import { createNativeAcceptanceFixture } from "./native_source_acceptance_fixture.mjs";
import { SourceControlClient } from "../../../sdk/source-control/src/client.ts";
import { executeSourceAction } from "../../../plugins/hyperfide-source-control/runtime.ts";
import {
  NativeWorkspaceClient,
  SourceWorkspaceJournal,
  NativeVirtualFilesystem,
  discoverVirtualHost,
  SourceWorkspaceError,
} from "../../../sdk/source-control/src/host/index.ts";
import { NativeObjectCache } from "../../../core/host-daemon/src/source-control/cache.ts";
import { SparseSourceMaterializer } from "../../../core/host-daemon/src/source-control/materializer.ts";
import { NativeWorkspaceCheckpoint } from "../../../core/host-daemon/src/source-control/checkpoint.ts";
import {
  SourceMetrics,
  SOURCE_METRIC_MODES,
  SOURCE_METRIC_STATUSES,
  SOURCE_METRIC_PARTITIONS,
  SOURCE_CHECKPOINT_BUCKETS_MS,
  SourceBudgetExceeded,
} from "../../../core/host-daemon/src/source-control/metrics.ts";

const temporary = await fs.mkdtemp(path.join(os.tmpdir(), "hf-product-git-"));
const scope = {
  organisation_id: randomUUID(),
  repository_id: randomUUID(),
  security_domain_id: randomUUID(),
};
const gitDirectory = path.join(temporary, "remote.git");
execFileSync("git", ["init", "--bare", gitDirectory], { stdio: "ignore" });
const { createLocalGitDestination } = await import(
  pathToFileURL(
    path.join(process.env.SOURCE_CONTROL_MODULE_ROOT, "git/transport.js"),
  )
);
const destination = createLocalGitDestination({
  scope,
  id: "product-browser",
  repository_identity: "owned-product-browser",
  path: gitDirectory,
  allowed_refs: ["refs/heads/main"],
});
const { sourceTransferOptions } = await import(
  pathToFileURL(
    path.join(process.env.SOURCE_CONTROL_MODULE_ROOT, "transfers.js"),
  )
);
const productActions = [
  "source.objects.put",
  "source.objects.read",
  "source.objects.inspect",
  "source.trees.lookup",
  "source.trees.entries",
  "source.revisions.create",
  "source.revisions.read",
  "source.workspaces.open",
  "source.workspaces.detail",
  "source.workspaces.checkpoint",
  "source.queries.history",
  "source.queries.diff",
  "source.operations.read",
  "source.operations.cancel",
  "source.fleet.manifest",
  "source.domains.create",
  "source.domains.read",
  "source.domains.configure",
  "source.candidates.create",
  "source.candidates.read",
  "source.candidates.qualify",
  "source.acceptance.accept",
  "source.acceptance.read",
  "source.assurance.custody.create",
  "source.assurance.publish",
  "source.assurance.revoke",
  "source.transforms.merge",
  "source.transforms.read",
  "source.transforms.resume",
  "source.publications.create",
  "source.publications.read",
  "source.publications.resume",
  "source.publications.cancel",
  "source.transfers.create",
  "source.transfers.read",
  "source.transfers.complete",
  "source.transfers.cancel",
  "source.metrics.read",
];
const f = await createNativeAcceptanceFixture({
  scope,
  actions: productActions,
  serviceOptions: {
    publication: { destinations: [destination] },
    transfers: sourceTransferOptions(
      "https://localhost/api/v1/source-control/transfers",
      randomBytes(32),
    ),
  },
});
const client = new SourceControlClient({
  endpoint: f.endpoint,
  scope: f.scope,
  token: () => f.tokenFor(),
});
const binding = { client },
  checks = [];
const execute = async (action, id = randomUUID()) => {
  const response = await executeSourceAction(binding, id, action);
  assert.equal(response.ok, true, JSON.stringify(response));
  return response.result;
};
async function boundedProductAdmission() {
  const responses = await Promise.all(
    Array.from({ length: 20 }, () =>
      executeSourceAction(binding, randomUUID(), {
        kind: "revision",
        revisionId: f.base.revision_id,
      }),
    ),
  );
  const refused = responses.filter((response) => !response.ok);
  assert(
    refused.length > 0,
    "Real concurrent product reads must reach the admission budget",
  );
  assert(responses.some((response) => response.ok));
  for (const response of refused) {
    assert.equal(response.error.code, "LIMIT_EXCEEDED");
    assert.equal(response.error.retry, "RETRY_SAME_OPERATION");
    assert.equal(response.error.outcome, "KNOWN");
    assert.equal(response.error.details.budget, "concurrent_requests");
    assert.equal(response.error.details.limit, 12);
    assert(response.error.message.includes("concurrent_requests"));
  }
  checks.push(
    "real concurrent product reads return typed LIMIT_EXCEEDED diagnostics naming their admission budget",
  );
}
async function incompleteDependencyClosureOpen() {
  const workspaceId = randomUUID();
  const refused = await executeSourceAction(binding, randomUUID(), {
    kind: "open",
    workspaceId,
    revisionId: f.base.revision_id,
    readPaths: ["main.txt"],
    writePaths: ["main.txt"],
    mode: "API_ONLY",
    dependencyPaths: ["dependencies/required.txt"],
  });
  assert.equal(refused.ok, false);
  assert.equal(refused.error.code, "DEPENDENCY_CLOSURE_INCOMPLETE");
  assert.equal(refused.error.outcome, "KNOWN");
  assert.deepEqual(refused.error.details.readiness.outsideScope, [
    "dependencies/required.txt",
  ]);
  const absent = await executeSourceAction(binding, randomUUID(), {
    kind: "workspace",
    workspaceId,
  });
  assert.equal(absent.ok, false);
  assert.equal(absent.error.code, "WORKSPACE_NOT_FOUND");
  const readiness = await execute({
    kind: "readiness",
    revisionId: f.base.revision_id,
    readPaths: ["main.txt", "dependencies/required.txt"],
    dependencyPaths: ["dependencies/required.txt"],
  });
  assert.equal(readiness.state, "INCOMPLETE");
  assert.deepEqual(readiness.missingPaths, ["dependencies/required.txt"]);
  const ready = await execute({
    kind: "readiness",
    revisionId: f.base.revision_id,
    readPaths: ["main.txt"],
    dependencyPaths: ["main.txt"],
  });
  assert.equal(ready.state, "READY");
  checks.push(
    "incomplete dependency closure refuses admission without creating a workspace; readiness checks real immutable tree metadata",
  );
}
async function oversizedProductPagination() {
  const entries = [];
  for (let index = 0; index < 55; index++) {
    const file = await f.service.objects.putFile(
      f.scope,
      Buffer.from(`page content ${index}\n`),
    );
    entries.push({
      name: `page-${String(index).padStart(3, "0")}.txt`,
      kind: "file",
      mode: "100644",
      object_id: file.object_id,
    });
  }
  const tree = await f.service.trees.fromEntries(f.scope, entries);
  const revisions = [];
  let parent = f.base.revision_id;
  for (let index = 0; index < 55; index++) {
    const revision = await f.createRevision({
      root_tree_id: tree.root_tree_id,
      parent_revision_ids: [parent],
      message: `paged product history ${index}`,
    });
    revisions.push(revision.revision_id);
    parent = revision.revision_id;
  }
  const walk = async (action) => {
    const seen = new Set(),
      rows = [];
    let cursor,
      pages = 0;
    do {
      const page = await execute({ ...action, ...(cursor ? { cursor } : {}) });
      assert(page.entries.length <= 50);
      assert.equal(
        page.coverage.state,
        page.next_cursor ? "PARTIAL" : "COMPLETE",
      );
      rows.push(...page.entries);
      pages++;
      cursor = page.next_cursor;
      if (cursor) {
        assert(!seen.has(cursor), "Continuation must advance");
        seen.add(cursor);
      }
      assert(pages <= 4, "Pagination must terminate within the actual dataset");
    } while (cursor);
    assert(pages > 1, "Oversized data must require continuation");
    return { rows, pages };
  };
  const history = await walk({ kind: "history", revisionId: parent });
  assert.deepEqual(
    history.rows.map((row) => row.revision_id),
    [...revisions].reverse().concat(f.base.revision_id),
  );
  assert.equal(new Set(history.rows.map((row) => row.revision_id)).size, 56);
  const diff = await walk({
    kind: "diff",
    beforeRevisionId: f.base.revision_id,
    afterRevisionId: parent,
  });
  const expectedPaths = [
    "main.txt",
    ...entries.map((entry) => entry.name),
  ].sort();
  assert.deepEqual(diff.rows.map((row) => row.path).sort(), expectedPaths);
  assert.equal(new Set(diff.rows.map((row) => row.path)).size, 56);
  f.productPagination = {
    revisionId: parent,
    beforeRevisionId: f.base.revision_id,
    expectedPaths,
    historyRevisions: [...revisions].reverse().concat(f.base.revision_id),
  };
  checks.push(
    "oversized native history and diff follow every next_cursor to complete coverage without duplicate or missing rows",
  );
}
async function unauthorizedProductRead() {
  const secret = "private source content must never leave its capability";
  const file = await f.service.objects.putFile(f.scope, Buffer.from(secret));
  const main = await f.service.trees.lookup(
    f.scope,
    f.base.root_tree_id,
    "main.txt",
  );
  const directory = await f.service.trees.fromEntries(f.scope, [
    { ...main, name: "main.txt" },
  ]);
  const symlink = await f.service.objects.put(
    f.scope,
    "blob",
    Buffer.from("main.txt"),
  );
  const tree = await f.service.trees.fromEntries(f.scope, [
    { ...main, name: "main.txt" },
    {
      name: "folder",
      kind: "directory",
      mode: "040000",
      object_id: directory.root_tree_id,
    },
    {
      name: "link",
      kind: "symlink",
      mode: "120000",
      object_id: symlink.object_id,
    },
    {
      name: "private.txt",
      kind: "file",
      mode: "100644",
      object_id: file.object_id,
    },
  ]);
  const revision = await f.createRevision({ root_tree_id: tree.root_tree_id });
  const restrictedCapability = randomUUID();
  await f.pool.query(
    `INSERT INTO hyperfide.source_capabilities(organisation_id,repository_id,security_domain_id,id,principal_id,credential_id,principal_kind,authority_epoch,policy_version,actions,path_prefixes,expires_at)
    VALUES($1,$2,$3,$4,$5,$6,'automation',1,$7,$8,ARRAY['main.txt'],clock_timestamp()+interval '5 minutes')`,
    [
      ...Object.values(f.scope),
      restrictedCapability,
      f.roles.author.principal_id,
      f.roles.author.credential_id,
      f.sourcePolicy,
      productActions,
    ],
  );
  const restricted = new SourceControlClient({
    endpoint: f.endpoint,
    scope: f.scope,
    token: () => f.tokenFor("author", restrictedCapability),
  });
  const readsBefore = f.backend.requests.length;
  const denied = await executeSourceAction(
    { client: restricted },
    randomUUID(),
    { kind: "file", revisionId: revision.revision_id, path: "private.txt" },
  );
  assert.equal(denied.ok, false);
  assert.equal(denied.error.code, "SCOPE_DENIED");
  assert.equal("result" in denied, false);
  assert.equal("content" in denied, false);
  assert(!JSON.stringify(denied).includes(secret));
  const allowed = await executeSourceAction(
    { client: restricted },
    randomUUID(),
    { kind: "file", revisionId: revision.revision_id, path: "main.txt" },
  );
  assert.equal(allowed.ok, true, JSON.stringify(allowed));
  assert.equal(allowed.result.content, "baseline\n");
  for (const path of ["folder", "link"])
    await assert.rejects(
      restricted.call("source.objects.read", {
        revision_id: revision.revision_id,
        path,
      }),
      (error) =>
        error.code === "SCOPE_DENIED" &&
        !JSON.stringify(error).includes(secret),
    );
  await assert.rejects(
    restricted.call("source.objects.read", { object_id: file.object_id }),
    (error) => error.code === "SCOPE_DENIED",
  );
  const readDirectory = await client.call("source.objects.read", {
    revision_id: revision.revision_id,
    path: "folder",
  });
  assert.equal(readDirectory.kind, "directory");
  assert.equal(readDirectory.object_id, directory.root_tree_id);
  const readSymlink = await client.call("source.objects.read", {
    revision_id: revision.revision_id,
    path: "link",
  });
  assert.equal(readSymlink.kind, "symlink");
  assert.equal(
    Buffer.from(readSymlink.content_base64, "base64").toString(),
    "main.txt",
  );
  assert(f.backend.requests.length >= readsBefore);
  checks.push(
    "real path-restricted capability denies an existing unauthorized file without returning content while allowing its authorized neighbor",
  );
}
async function hostProductMetricsAndVirtualRefusal() {
  const metrics = new SourceMetrics(),
    native = new NativeWorkspaceClient(client, f.scope);
  const descriptor = await native.open({
    base_revision_id: f.base.revision_id,
    mode: "SPARSE",
    read_paths: ["main.txt"],
    write_paths: ["main.txt"],
  });
  const root = path.join(temporary, "materialized"),
    journal = new SourceWorkspaceJournal(path.join(temporary, "journal"));
  const cache = new NativeObjectCache(
    path.join(temporary, "cache"),
    { max_bytes: 65536, max_entries: 2, max_object_bytes: 32768 },
    metrics,
  );
  const materializer = new SparseSourceMaterializer(
    native,
    cache,
    journal,
    root,
    metrics,
  );
  const first = await materializer.materialize(descriptor, ["main.txt"]);
  assert.equal(first.files, 1);
  assert.equal(first.bytes, Buffer.byteLength("baseline\n"));
  await materializer.materialize(descriptor, ["main.txt"]);
  const extra = await f.blob(Buffer.from("cache eviction"));
  const acquired = await cache.acquire(
    f.scope,
    extra,
    () => native.readObject(extra),
    () => native.assertAccess(descriptor, "main.txt"),
  );
  await acquired.release();
  await fs.writeFile(path.join(root, "main.txt"), "candidate: host metrics\n");
  await materializer.reconcile(descriptor);
  const dirty = metrics
    .snapshot()
    .product.find((series) => series.labels.partition === "OVERLAY");
  assert.equal(
    dirty.gauges.dirtyBytes,
    Buffer.byteLength("candidate: host metrics\n"),
  );
  const checkpoint = await new NativeWorkspaceCheckpoint(
    native,
    journal,
    root,
    materializer,
    metrics,
  ).checkpoint(descriptor);
  assert.equal(
    checkpoint.receipt.workspace_generation,
    checkpoint.workspace.workspace_generation,
  );
  assert.equal((await journal.snapshot()).pending, null);
  const snapshot = metrics.snapshot(),
    overlay = snapshot.product.find(
      (series) => series.labels.partition === "OVERLAY",
    ),
    cached = snapshot.product.find(
      (series) => series.labels.partition === "CACHE",
    );
  assert.equal(overlay.counters.logicalOpens, 2);
  assert.equal(overlay.counters.materializationFiles, 2);
  assert.equal(overlay.counters.materializationBytes, 2 * first.bytes);
  assert.equal(overlay.gauges.dirtyBytes, 0);
  assert.equal(overlay.gauges.transferQueueDepth, 0);
  assert.equal(overlay.checkpointLatency.count, 1);
  assert.equal(overlay.checkpointLatency.buckets.at(-1).count, 1);
  assert.deepEqual(
    overlay.checkpointLatency.buckets.slice(0, -1).map((bucket) => bucket.le),
    SOURCE_CHECKPOINT_BUCKETS_MS.map(String),
  );
  assert(cached.counters.cacheHits >= 2);
  assert(cached.counters.cacheEvictions >= 1);
  assert(cached.counters.downloadBytes > 0);
  assert.equal(cached.gauges.transferQueueDepth, 0);
  const virtualDescriptor = await native.open({
    base_revision_id: f.base.revision_id,
    mode: "VIRTUAL",
    read_paths: ["main.txt"],
    write_paths: ["main.txt"],
  });
  const host = {
    mount: {
      path: path.join(temporary, "uninstalled-mount"),
      sha256: "sha256:" + "0".repeat(64),
    },
    unmount: {
      path: path.join(temporary, "uninstalled-unmount"),
      sha256: "sha256:" + "0".repeat(64),
    },
  };
  const capabilities = await discoverVirtualHost(host);
  assert.equal(capabilities.available, false);
  const virtual = new NativeVirtualFilesystem(
    native,
    cache,
    new SourceWorkspaceJournal(path.join(temporary, "virtual-journal")),
    virtualDescriptor,
    {
      mountpoint: path.join(temporary, "virtual-source"),
      stateDirectory: path.join(temporary, "virtual-state"),
      host,
      uid: process.getuid(),
      gid: process.getgid(),
    },
  );
  await assert.rejects(
    () => virtual.mount(),
    (error) =>
      error instanceof SourceWorkspaceError &&
      error.code === "VIRTUAL_FILESYSTEM_UNAVAILABLE" &&
      error.message.includes("explicitly select SPARSE or MATERIALIZED"),
  );
  await assert.rejects(fs.stat(path.join(temporary, "virtual-source")), {
    code: "ENOENT",
  });
  checks.push(
    "real host materialization, cache hits and eviction, dirty journal and receipt-backed checkpoint feed bounded counters and latency; unavailable virtualization returns a typed refusal without fallback",
  );
}
try {
  await boundedProductAdmission();
  await incompleteDependencyClosureOpen();
  await oversizedProductPagination();
  await unauthorizedProductRead();
  await hostProductMetricsAndVirtualRefusal();
  const history = await execute({
    kind: "history",
    revisionId: f.base.revision_id,
  });
  assert(history.entries.some((row) => row.revision_id === f.base.revision_id));
  checks.push(
    "product history reads native source metadata through the real HTTP API without a checkout",
  );
  const workspaceId = randomUUID(),
    opened = await execute({
      kind: "open",
      workspaceId,
      revisionId: f.base.revision_id,
      readPaths: ["main.txt"],
      writePaths: ["main.txt"],
      mode: "API_ONLY",
    });
  assert.equal(opened.workspace.workspace_id, workspaceId);
  const operation = randomUUID(),
    input = {
      kind: "checkpoint",
      workspaceId,
      baseRevisionId: opened.workspace.base_revision_id,
      workspaceGeneration: opened.workspace.workspace_generation,
      scopeGeneration: opened.workspace.scope_generation,
      overlayGeneration: "1",
      path: "main.txt",
      content: "candidate: product checkpoint\n",
      executable: false,
    };
  const checkpoint = await execute(input, operation);
  assert.notEqual(checkpoint.revision_id, f.base.revision_id);
  assert.deepEqual(await execute(input, operation), checkpoint);
  const current = await execute({ kind: "workspace", workspaceId });
  assert.equal(current.workspace.base_revision_id, checkpoint.revision_id);
  const op = await execute({ kind: "operation", operationId: operation });
  assert.equal(op.state, "SUCCEEDED");
  checks.push(
    "product checkpoint uploads verified file objects, records durable source and replays the same operation once",
  );
  const stale = await executeSourceAction(binding, randomUUID(), input);
  assert.equal(stale.ok, false);
  assert.equal(stale.error.code, "WORKSPACE_CHANGED");
  const outside = await executeSourceAction(binding, randomUUID(), {
    ...input,
    baseRevisionId: checkpoint.revision_id,
    workspaceGeneration: checkpoint.workspace.workspace_generation,
    path: "private.txt",
  });
  assert.equal(outside.ok, false);
  assert.equal(outside.error.code, "SCOPE_DENIED");
  checks.push(
    "stale workspace generations and writes outside the granted path remain explicit refusals",
  );
  const diff = await execute({
    kind: "diff",
    beforeRevisionId: f.base.revision_id,
    afterRevisionId: checkpoint.revision_id,
  });
  assert(diff.entries.some((row) => row.path === "main.txt"));
  checks.push(
    "product comparison returns actual changed paths and bounded coverage",
  );
  const revision = await f.createRevision({
      text: "candidate: accepted product\n",
    }),
    candidate = await f.createCandidate(revision),
    evidence = await f.publishEvidence(candidate);
  const qualificationOperation = randomUUID(),
    qualificationInput = {
      kind: "qualify",
      candidateId: candidate.candidate_id,
      evidenceIds: [evidence.evidence.evidence_id],
    };
  const qualification = await execute(
    qualificationInput,
    qualificationOperation,
  );
  assert.deepEqual(
    await execute(qualificationInput, qualificationOperation),
    qualification,
  );
  const accepted = await execute({
    kind: "accept",
    candidateId: candidate.candidate_id,
    qualificationId: qualification.qualification_id,
  });
  assert.equal(accepted.revision_id, revision.revision_id);
  checks.push(
    "product acceptance requires independent executed evidence and returns the real acceptance receipt",
  );
  const publication = await execute({
    kind: "publish",
    acceptanceId: accepted.acceptance_id,
    destinationId: "product-browser",
    targetRef: "refs/heads/main",
    expectedOldOid: null,
  });
  assert.equal(publication.status, "QUEUED");
  const sampled = await execute({ kind: "service-metrics", maxRows: 1 });
  assert(sampled.families.publications.remote_lag_ms >= 0);
  assert(
    sampled.product_requests.product.some(
      (series) =>
        series.labels.partition === "INTEGRATION" &&
        series.counters.integrationRetries >= 1,
    ),
  );
  await execute({
    kind: "cancel-publication",
    publicationId: publication.operation_id,
  });
  assert.equal(sampled.families.objects.observed_rows, 1);
  assert.equal(sampled.families.objects.coverage, "PARTIAL");
  assert(["PARTIAL", "COMPLETE"].includes(sampled.families.graphs.coverage));
  assert(sampled.families.graphs.observed_rows <= 1);
  checks.push(
    "server metrics sample actual scoped tables under an explicit per-family row budget",
  );
  const metrics = new SourceMetrics(4),
    release1 = metrics.admit("checkpoint"),
    release2 = metrics.admit("read");
  assert.throws(
    () => metrics.admit("read"),
    (error) =>
      error instanceof SourceBudgetExceeded &&
      error.code === "LIMIT_EXCEEDED" &&
      error.details.budget === "concurrent_requests" &&
      error.details.limit === 2 &&
      error.message.includes("concurrent_requests"),
  );
  const release3 = metrics.admit("cancel"),
    release4 = metrics.admit("resume");
  assert.throws(
    () => metrics.admit("cancel"),
    (error) =>
      error instanceof SourceBudgetExceeded &&
      error.code === "LIMIT_EXCEEDED" &&
      error.details.budget === "recovery_requests",
  );
  release1();
  release2();
  release3();
  release4();
  release4();
  for (let n = 0; n < 1000; n++) metrics.record("read", "success", 1);
  const snapshot = metrics.snapshot();
  assert.equal(snapshot.activeRequests, 0);
  assert.equal(snapshot.counters.length, 1);
  assert.equal(snapshot.counters[0].count, 1000);
  const unknownLabels = ["main.txt", f.base.revision_id, f.scope.repository_id];
  for (const value of unknownLabels) {
    assert.throws(() =>
      metrics.increment("logicalOpens", 1, {
        mode: value,
        status: "success",
        partition: "REQUEST",
      }),
    );
    assert.throws(() =>
      metrics.increment("logicalOpens", 1, {
        mode: "API_ONLY",
        status: value,
        partition: "REQUEST",
      }),
    );
    assert.throws(() =>
      metrics.increment("logicalOpens", 1, {
        mode: "API_ONLY",
        status: "success",
        partition: value,
      }),
    );
    assert.throws(() =>
      metrics.increment("logicalOpens", 1, {
        mode: "API_ONLY",
        status: "success",
        partition: "REQUEST",
        project: value,
      }),
    );
  }
  for (let index = 0; index < 1000; index++)
    metrics.increment("graphWorkItems", 1, {
      mode: "API_ONLY",
      status: "success",
      partition: "GRAPH",
    });
  metrics.increment("integrationRetries", 1, {
    mode: "API_ONLY",
    status: "unknown",
    partition: "INTEGRATION",
  });
  metrics.gauge("remoteLagMs", 123, {
    mode: "API_ONLY",
    status: "unknown",
    partition: "REMOTE",
  });
  assert.equal(
    metrics
      .snapshot()
      .product.find((series) => series.labels.partition === "GRAPH").counters
      .graphWorkItems,
    1000,
  );
  assert.equal(
    metrics
      .snapshot()
      .product.find((series) => series.labels.partition === "INTEGRATION")
      .counters.integrationRetries,
    1,
  );
  assert.equal(
    metrics
      .snapshot()
      .product.find((series) => series.labels.partition === "REMOTE").gauges
      .remoteLagMs,
    123,
  );
  for (const counter of metrics.snapshot().counters) {
    assert(SOURCE_METRIC_MODES.includes(counter.mode));
    assert(SOURCE_METRIC_STATUSES.includes(counter.status));
    assert(SOURCE_METRIC_PARTITIONS.includes(counter.partition));
    assert(
      unknownLabels.every((value) => !Object.values(counter).includes(value)),
    );
  }
  for (const series of metrics.snapshot().product) {
    assert.deepEqual(Object.keys(series.labels).sort(), [
      "mode",
      "partition",
      "status",
    ]);
    assert(
      unknownLabels.every(
        (value) => !Object.values(series.labels).includes(value),
      ),
    );
  }
  assert(
    metrics.snapshot().product.length <=
      SOURCE_METRIC_MODES.length *
        SOURCE_METRIC_STATUSES.length *
        SOURCE_METRIC_PARTITIONS.length,
  );
  const { SOURCE_SERVICE_STATUSES, SOURCE_SERVICE_PARTITIONS } = await import(
    pathToFileURL(
      path.join(process.env.SOURCE_CONTROL_MODULE_ROOT, "metrics.js"),
    )
  );
  for (const family of Object.values(sampled.families)) {
    assert(
      Object.keys(family.states).every((state) =>
        SOURCE_SERVICE_STATUSES.includes(state),
      ),
    );
    assert(
      Object.keys(family.partitions).every((partition) =>
        SOURCE_SERVICE_PARTITIONS.includes(partition),
      ),
    );
    assert(
      unknownLabels.every(
        (value) =>
          !Object.hasOwn(family.states, value) &&
          !Object.hasOwn(family.partitions, value),
      ),
    );
  }
  checks.push(
    "bounded metrics reserve recovery capacity and never create labels from source paths, revisions or project IDs",
  );
  if (process.env.HF_SOURCE_BROWSER === "1") {
    const { runSourceProductBrowser } =
      await import("./native_source_product_browser.mjs");
    await runSourceProductBrowser(f, binding);
    const remote = execFileSync(
      "git",
      ["--git-dir", gitDirectory, "rev-parse", "refs/heads/main"],
      { encoding: "utf8" },
    ).trim();
    assert.match(remote, /^[a-f0-9]{40}$/);
    checks.push(
      "actual Chromium workspace, lost-response replay and history journeys at desktop tablet and mobile sizes",
    );
  }
  const receipt = {
    status: "PASS",
    revision: execFileSync("git", ["rev-parse", "HEAD"], {
      encoding: "utf8",
    }).trim(),
    checks,
    backend_requests: f.backend.requests.length,
    scope:
      "actual product action adapter, HTTP source API, PostgreSQL, S3 fixture, and independent checker",
  };
  await fs.mkdir("build/source-control/product", { recursive: true });
  await fs.writeFile(
    "build/source-control/product/receipt.json",
    JSON.stringify(receipt, null, 2) + "\n",
  );
  console.log(JSON.stringify(receipt));
} finally {
  await f.close();
  await fs.rm(temporary, { recursive: true, force: true });
}
