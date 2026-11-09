import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { createHash, randomUUID } from "node:crypto";
import { spawn, execFileSync } from "node:child_process";
import { pathToFileURL } from "node:url";
import { createNativeAcceptanceFixture } from "./native_source_acceptance_fixture.mjs";

const owned = process.env.SOURCE_CONTROL_SPARSE_OWNED === "1";
function redactLiveInputs(text) {
  const values = Object.entries(process.env)
    .filter(
      ([name, value]) =>
        value && /^(S3_|HF_SOURCE_S3_|HF_ARTIFACT_S3_)/.test(name),
    )
    .flatMap(([, value]) => [
      value,
      encodeURIComponent(value),
      JSON.stringify(value).slice(1, -1),
    ])
    .sort((a, b) => b.length - a.length);
  for (const value of values) text = text.replaceAll(value, "[REDACTED]");
  return text;
}
let serviceOptions = {};
if (!owned) {
  try {
    process.loadEnvFile(path.resolve(".env"));
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  const { S3SourceObjectBackend, sourceS3Configuration } = await import(
    pathToFileURL(path.join(process.env.SOURCE_CONTROL_MODULE_ROOT, "s3.js"))
  );
  serviceOptions = {
    backend: new S3SourceObjectBackend(sourceS3Configuration(process.env)),
  };
}
const f = await createNativeAcceptanceFixture({ serviceOptions });
const output = path.resolve(
  process.env.HYPERFIDE_SOURCE_EVIDENCE_DIR ??
    (owned ? "build/source-control/2.2" : "build/source-control/sparse-live"),
);
await fs.mkdir(output, { recursive: true, mode: 0o700 });
const profile = path.join(
  process.env.SOURCE_CONTROL_MODULE_ROOT,
  "sparse-profile.json",
);
try {
  await fs.writeFile(
    profile,
    JSON.stringify({
      endpoint: f.endpoint,
      scope: f.scope,
      token: f.tokenFor(),
      revocation_token: f.tokenFor("checker"),
    }),
    { mode: 0o600 },
  );
  const program =
    process.env.SOURCE_CONTROL_WORKSPACE_PROGRAM ??
    execFileSync(
      "python3",
      [
        "-c",
        "import runpy; print(runpy.run_path('tests/hyperfide/cases/source_control/2_2.py')['PROGRAM'])",
      ],
      { encoding: "utf8" },
    );
  const child = spawn(
    process.execPath,
    [
      "--conditions=source",
      "--import",
      "tsx",
      "--input-type=module",
      "-e",
      program,
      path.resolve("."),
      JSON.stringify({ profile_path: profile }),
    ],
    { env: process.env, stdio: ["ignore", "pipe", "pipe", "ipc"] },
  );
  child.on("message", async (message) => {
    try {
      assert.equal(message.action, "revoke-checker");
      const result = await f.pool.query(
        "UPDATE hyperfide.source_capabilities SET revoked_at=clock_timestamp() WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND id=$4 AND revoked_at IS NULL",
        [...Object.values(f.scope), f.roles.checker.capability_id],
      );
      assert.equal(result.rowCount, 1);
      child.send({ request_id: message.request_id, revoked: true });
    } catch (error) {
      child.send({ request_id: message.request_id, error: String(error) });
    }
  });
  let stdout = "",
    stderr = "";
  child.stdout.on("data", (chunk) => (stdout += chunk));
  child.stderr.on("data", (chunk) => (stderr += chunk));
  const code = await new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("close", resolve);
  });
  await fs.writeFile(
    path.join(output, "host.log"),
    redactLiveInputs(stdout + stderr),
    {
      mode: 0o600,
    },
  );
  assert.equal(code, 0, stdout + stderr);
  const report = JSON.parse(stdout.trim().split("\n").at(-1));
  assert.equal(
    report.qualification_level,
    "REAL_SOURCE_SERVICE_AND_HOST_FILESYSTEM",
  );
  const rows = (
    await f.pool.query(
      "SELECT checkpoint_revision_id FROM hyperfide.source_workspaces WHERE checkpoint_revision_id IS NOT NULL",
    )
  ).rows;
  assert.equal(rows.length, 1);
  const revision = await f.client.revision(rows[0].checkpoint_revision_id);
  const hidden = await f.service.trees.lookup(
    f.scope,
    revision.root_tree_id,
    "omitted/hidden.txt",
  );
  assert(hidden);
  assert.equal(
    (await f.service.objects.readFile(f.scope, hidden.object_id)).toString(),
    "must survive sparse checkpoint\n",
  );
  const edited = await f.service.trees.lookup(
    f.scope,
    revision.root_tree_id,
    "src/a.txt",
  );
  assert(edited);
  assert.equal(
    (await f.service.objects.readFile(f.scope, edited.object_id)).toString(),
    "captured checkpoint\n",
  );
  const removed = await f.service.trees.lookup(
    f.scope,
    revision.root_tree_id,
    "src/new.txt",
  );
  assert.equal(removed, null);
  const checks = [
    "independent native revision and object readback preserve omitted bytes, committed edits and explicit deletion",
  ];
  const ready = report.readiness_receipts[0];
  assert(ready);
  const input = { workspace_id: ready.workspace_id };
  const beforeRestart = await f.client.call("source.workspaces.detail", input);
  assert.equal(
    BigInt(beforeRestart.workspace.scope_generation),
    BigInt(ready.scope_generation) + 1n,
  );
  await f.restart();
  const operation_id = randomUUID();
  let restartReadAttempts = 0;
  async function readRestartedWorkspace() {
    for (;;) {
      restartReadAttempts++;
      try {
        return await f.client.call("source.workspaces.detail", input, {
          operation_id,
        });
      } catch (error) {
        if (
          error.code !== "OUTCOME_UNKNOWN" ||
          error.retry !== "RETRY_SAME_OPERATION" ||
          restartReadAttempts >= 5
        )
          throw error;
      }
    }
  }
  const detail = await readRestartedWorkspace();
  assert.deepEqual(detail, beforeRestart);
  assert.equal(
    detail.workspace.dependency_closure_manifest.state,
    "INCOMPLETE",
  );
  const closure = detail.workspace.dependency_closure_manifest;
  assert.equal(
    ready.closure_manifest_digest,
    "sha256:" +
      createHash("sha256")
        .update(
          JSON.stringify([
            closure.schema_version,
            closure.state,
            closure.dependencies,
            closure.reason,
          ]),
        )
        .digest("hex"),
  );
  const persisted = (
    await f.pool.query(
      "SELECT dependency_closure_manifest FROM hyperfide.source_workspaces WHERE workspace_id=$1",
      [ready.workspace_id],
    )
  ).rows[0];
  assert.deepEqual(
    JSON.parse(JSON.stringify(detail.workspace.dependency_closure_manifest)),
    persisted.dependency_closure_manifest,
  );
  const receipt = {
    ...report,
    checks,
    backend: owned ? "SIGV4_COMPONENT" : "REAL_S3",
    scope: f.scope,
    checkpoint_revision_id: revision.revision_id,
    host_platform: process.platform,
    exit_code: code,
    restart_read_attempts: restartReadAttempts,
    revision: execFileSync("git", ["rev-parse", "HEAD"], {
      encoding: "utf8",
    }).trim(),
  };
  const bytes = Buffer.from(
    redactLiveInputs(JSON.stringify(receipt, null, 2)) + "\n",
  );
  const digest = "sha256:" + createHash("sha256").update(bytes).digest("hex");
  await fs.writeFile(path.join(output, "receipt.json"), bytes, { mode: 0o600 });
  await fs.writeFile(path.join(output, "receipt.sha256"), digest + "\n", {
    mode: 0o600,
  });
  console.log(
    JSON.stringify({
      status: "PASS",
      exit_code: code,
      receipt_digest: digest,
      receipt: path.join(output, "receipt.json"),
      assertions: report.assertions,
      checks,
      backend: receipt.backend,
    }),
  );
} finally {
  try {
    const placements = (
      await f.pool.query(
        "SELECT object_id,incarnation_id,storage_key,state FROM hyperfide.source_object_placements WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3",
        Object.values(f.scope),
      )
    ).rows;
    await fs.writeFile(
      path.join(output, "retained-placements.json"),
      redactLiveInputs(
        JSON.stringify({ scope: f.scope, placements }, null, 2),
      ) + "\n",
      { mode: 0o600 },
    );
  } finally {
    await f.close();
  }
}
