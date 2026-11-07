import assert from "node:assert/strict";
import { createHash, randomUUID } from "node:crypto";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { runSourceProductBrowser } from "./native_source_product_browser.mjs";
const execute = promisify(execFile);
const sha = (bytes) => createHash("sha256").update(bytes).digest("hex");
export async function runFullSourceJourney({
  f,
  root,
  inputs,
  accepted,
  qualified,
  submission,
  taskQualification,
  baseline,
  checkpointOne,
  checkpointTwo,
  replacement,
  selectedSubmission,
  replacementJourney,
}) {
  const assertions = [],
    steps = [];
  let stepRevision = baseline.revision_id,
    stepAssignment = submission.assignment_id;
  const providerFor = async (assignmentId) => {
    const result = (
      await f.pool.query(
        `SELECT p.provider,p.id AS pool_id,p.profile_id,p.account_key,p.environment_key,p.region,
    a.sandbox_id,a.sandbox_boot_generation::text AS boot_generation
    FROM hyperfide.source_fleet_assignments a
    JOIN hyperfide.sandboxes s ON s.organisation_id=a.organisation_id AND s.id=a.sandbox_id
    JOIN hyperfide.provider_pools p ON p.organisation_id=s.organisation_id AND p.id=s.pool_id
    WHERE a.organisation_id=$1 AND a.repository_id=$2 AND a.security_domain_id=$3 AND a.assignment_id=$4`,
        [...Object.values(f.scope), assignmentId],
      )
    ).rows[0];
    assert(
      result,
      "Every journey step must identify its actual assigned provider",
    );
    return result;
  };
  const proved = async (id, actual) => {
    assert.equal(actual, true, id);
    const revision = await f.client.revision(stepRevision),
      provider = await providerFor(stepAssignment);
    assertions.push({ id: "SCM-023." + id, expected: true, actual: true });
    steps.push({
      id: "SCM-023." + id,
      source: {
        scope: f.scope,
        revision_id: revision.revision_id,
        root_tree_id: revision.root_tree_id,
      },
      provider,
      assignment_id: stepAssignment,
    });
  };
  const scope = Object.values(f.scope),
    query = (sql, args = []) => f.pool.query(sql, [...scope, ...args]);
  const original = (
    await query(
      "SELECT * FROM hyperfide.source_fleet_assignments WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND assignment_id=$4",
      [submission.assignment_id],
    )
  ).rows[0];
  await proved(
    "assignment-workspace-binding",
    Boolean(
      original &&
      original.base_revision_id === baseline.revision_id &&
      original.workspace_id &&
      original.attempt_id,
    ),
  );
  const first = await f.client.revision(checkpointOne.revision_id),
    second = await f.client.revision(checkpointTwo.revision_id);
  stepRevision = second.revision_id;
  await proved(
    "durable-checkpoint-lineage",
    first.parent_revision_ids.includes(baseline.revision_id) &&
      second.parent_revision_ids.includes(first.revision_id),
  );
  const checkpoints = (
    await query(
      "SELECT result FROM hyperfide.source_workspace_checkpoints WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND workspace_id=$4",
      [original.workspace_id],
    )
  ).rows;
  await proved(
    "checkpoint-and-submission-distinct",
    checkpoints.length === 2 &&
      submission.revision_id === second.revision_id &&
      accepted.revision_id === second.revision_id,
  );
  const submissions = (
    await query(
      "SELECT * FROM hyperfide.source_fleet_submissions WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND assignment_id=$4",
      [original.assignment_id],
    )
  ).rows;
  await proved(
    "unknown-submission-reconciles-once",
    submissions.length === 1 &&
      submissions[0].revision_id === second.revision_id,
  );
  const replacements = (
    await query(
      "SELECT attempt_id,attempt_generation::text,workspace_id FROM hyperfide.source_fleet_assignments WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND task_id=$4 ORDER BY attempt_generation",
      [replacement.assignment.tasks[0].task_id],
    )
  ).rows;
  await proved(
    "replacement-keeps-prior-history",
    replacements.length === 2 &&
      replacements[0].attempt_id !== replacements[1].attempt_id &&
      replacements[0].workspace_id !== replacements[1].workspace_id &&
      BigInt(replacements[1].attempt_generation) >
        BigInt(replacements[0].attempt_generation),
  );
  const proof = (
    await query(
      "SELECT receipt FROM hyperfide.source_fleet_qualifications WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND qualification_id=$4",
      [taskQualification.qualification_id],
    )
  ).rows[0];
  await proved(
    "independent-qualification-before-task-release",
    Boolean(proof) &&
      accepted.qualification_id === qualified.qualification.qualification_id &&
      taskQualification.state === "QUALIFIED",
  );
  stepRevision = selectedSubmission.revision_id;
  stepAssignment = selectedSubmission.assignment_id;
  await proved(
    "dependent-consumes-accepted-source",
    selectedSubmission.revision_id !== accepted.revision_id &&
      (
        await f.client.revision(selectedSubmission.revision_id)
      ).parent_revision_ids.includes(accepted.revision_id),
  );
  stepRevision = replacementJourney.accepted.revision_id;
  stepAssignment = replacementJourney.submission.assignment_id;
  await proved(
    "forged-qualified-dependency-refused",
    replacementJourney.forged_dependency_refused === true,
  );
  await proved(
    "plain-sql-qualification-snapshot",
    replacementJourney.plain_sql_qualification_read === true,
  );
  await proved(
    "queue-preserves-qualified-dependencies",
    replacementJourney.queue_preserved_dependencies === true,
  );
  await proved(
    "replacement-reaches-qualified-acceptance",
    replacementJourney.taskQualification.state === "QUALIFIED" &&
      replacementJourney.accepted.revision_id ===
        replacementJourney.checkpoint.revision_id &&
      replacementJourney.submission.revision_id ===
        replacementJourney.accepted.revision_id &&
      replacementJourney.workspace_id === replacements[1].workspace_id,
  );
  const publicationAccepted = replacementJourney.accepted;
  // Each newly exported ancestor needs its own retained native acceptance.
  const pending = [publicationAccepted.revision_id],
    seen = new Set();
  while (pending.length) {
    const id = pending.pop();
    if (seen.has(id)) continue;
    seen.add(id);
    const revision = await f.client.revision(id);
    pending.push(...revision.parent_revision_ids);
    const retained = (
      await query(
        "SELECT acceptance_id FROM hyperfide.source_native_acceptances WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND revision_id=$4",
        [id],
      )
    ).rows[0];
    if (!retained) {
      const change = (
        await query(
          "SELECT change_id FROM hyperfide.source_change_revisions WHERE organisation_id=$1 AND repository_id=$2 AND security_domain_id=$3 AND revision_id=$4",
          [id],
        )
      ).rows[0];
      await f.accept(
        await f.createCandidate(
          { ...revision, change_id: change.change_id },
          { domain: await f.createDomain(revision) },
        ),
      );
    }
  }
  const config = path.join(root, "cli-source.json");
  await fs.writeFile(
    config,
    JSON.stringify({
      schema_version: 1,
      endpoint: inputs.origin + "/api/v1/source-control",
      scope: f.scope,
      credential_id: inputs.ids.operator,
      ca_file: inputs.ca,
      key_file: inputs.keys.operator,
    }),
    { mode: 0o600 },
  );
  const cli = async (args) => {
    const result = await execute(
      process.execPath,
      [
        path.join(f.ROOT, "apps/cli/dist/index.js"),
        "source",
        "--config",
        config,
        ...args,
      ],
      {
        cwd: root,
        env: process.env,
        timeout: 30000,
        maxBuffer: 4 * 1024 * 1024,
      },
    );
    return JSON.parse(result.stdout.trim().split("\n").at(-1));
  };
  // The sparse workspace is an independent branch of the exact accepted source.
  const {
    NativeWorkspaceClient,
    NativeObjectCache,
    SourceWorkspaceJournal,
    SparseSourceMaterializer,
    NativeWorkspaceCheckpoint,
  } = await import(
    pathToFileURL(path.join(f.ROOT, "sdk/source-control/src/host/index.ts"))
  );
  const workspaceClient = new NativeWorkspaceClient(f.client, f.scope);
  let sparse = await workspaceClient.open({
    base_revision_id: publicationAccepted.revision_id,
    mode: "SPARSE",
    read_paths: ["main.txt"],
    write_paths: ["main.txt"],
  });
  const sparseRoot = path.join(root, "journey-sparse"),
    journal = new SourceWorkspaceJournal(path.join(root, "journey-journal"));
  const cache = new NativeObjectCache(path.join(root, "journey-cache"), {
    max_bytes: sparse.budgets.cache_bytes,
    max_entries: sparse.budgets.max_cache_entries,
    max_object_bytes: sparse.budgets.max_file_bytes,
  });
  const materializer = new SparseSourceMaterializer(
    workspaceClient,
    cache,
    journal,
    sparseRoot,
  );
  let sparseCheckpoint, sourceDiff;
  try {
    const ready = await materializer.materialize(sparse, ["main.txt"]);
    assert.equal(ready.state, "READY");
    assert.equal(ready.files, 1);
    assert.equal(ready.physically_materialized, true);
    assert.deepEqual(await fs.readdir(sparseRoot), ["main.txt"]);
    const priorFile = await f.client.lookup(
      publicationAccepted.root_tree_id,
      "main.txt",
    );
    const omittedBefore = await f.client.lookup(
      publicationAccepted.root_tree_id,
      "private.txt",
    );
    assert(omittedBefore.entry);
    const priorBytes = await f.service.objects.readFile(
      f.scope,
      priorFile.entry.object_id,
    );
    assert.deepEqual(
      await fs.readFile(path.join(sparseRoot, "main.txt")),
      priorBytes,
    );
    const edited = Buffer.from(
      priorBytes.toString("utf8") + "candidate: scoped sparse journey edit\n",
    );
    await fs.writeFile(path.join(sparseRoot, "main.txt"), edited);
    await journal.mark("main.txt");
    sparseCheckpoint = await new NativeWorkspaceCheckpoint(
      workspaceClient,
      journal,
      sparseRoot,
      materializer,
    ).checkpoint(sparse);
    sparse = sparseCheckpoint.workspace;
    stepRevision = sparseCheckpoint.revision_id;
    const sparseRevision = await f.client.revision(stepRevision),
      editedFile = await f.client.lookup(
        sparseRevision.root_tree_id,
        "main.txt",
      );
    assert.deepEqual(
      await f.service.objects.readFile(f.scope, editedFile.entry.object_id),
      edited,
    );
    assert.deepEqual(
      (await f.client.lookup(sparseRevision.root_tree_id, "private.txt")).entry,
      omittedBefore.entry,
    );
    assert.deepEqual(await fs.readdir(sparseRoot), ["main.txt"]);
    await proved(
      "sparse-edit-materializes-only-scope",
      sparseRevision.parent_revision_ids.includes(
        publicationAccepted.revision_id,
      ) && sparseRevision.root_tree_id !== publicationAccepted.root_tree_id,
    );
    const apiDiff = await f.client.call("source.queries.diff", {
      before_revision_id: publicationAccepted.revision_id,
      after_revision_id: stepRevision,
      limit: 50,
    });
    const cliDiff = await cli([
      "diff",
      publicationAccepted.revision_id,
      stepRevision,
    ]);
    assert.equal(cliDiff.ok, true);
    assert.deepEqual(cliDiff.result.entries, apiDiff.entries);
    for (const result of [apiDiff, cliDiff.result]) {
      assert.equal(result.before_revision_id, publicationAccepted.revision_id);
      assert.equal(result.after_revision_id, stepRevision);
      assert.equal(result.coverage.state, "COMPLETE");
      assert.equal(result.next_cursor, null);
      assert.deepEqual(result.entries, [
        {
          path: "main.txt",
          kind: "MODIFIED",
          before: priorFile.entry,
          after: editedFile.entry,
        },
      ]);
    }
    sourceDiff = {
      before: {
        revision_id: publicationAccepted.revision_id,
        root_tree_id: publicationAccepted.root_tree_id,
      },
      after: {
        revision_id: sparseRevision.revision_id,
        root_tree_id: sparseRevision.root_tree_id,
      },
      workspace_id: sparse.workspace_id,
      api: apiDiff,
      cli: cliDiff.result,
      edited_sha256: sha(edited),
    };
    await proved("source-diff-matches-checkpoint", true);
  } finally {
    await workspaceClient.close(sparse);
  }
  stepRevision = publicationAccepted.revision_id;
  const revisionRead = await cli(["revision", publicationAccepted.revision_id]);
  await proved(
    "signed-cli-revision-read",
    revisionRead.ok === true &&
      revisionRead.result.revision_id === publicationAccepted.revision_id &&
      revisionRead.result.root_tree_id === publicationAccepted.root_tree_id,
  );
  const history = await cli(["history", publicationAccepted.revision_id]);
  await proved(
    "signed-cli-history-without-checkout",
    history.ok === true &&
      history.result.entries.some(
        (item) => item.revision_id === publicationAccepted.revision_id,
      ) &&
      !(await fs.readdir(root)).includes(".git"),
  );
  const browser = await runSourceProductBrowser(
    f,
    { client: f.client },
    {
      acceptanceId: publicationAccepted.acceptance_id,
      revisionId: publicationAccepted.revision_id,
      rootTreeId: publicationAccepted.root_tree_id,
      destinationId: "full-case",
      outputDirectory: "build/source-control/full-case/SCM-023/browser",
    },
  );
  await proved(
    "ui-shows-exact-fleet-accepted-source",
    browser.acceptance_id === publicationAccepted.acceptance_id &&
      browser.revision_id === publicationAccepted.revision_id &&
      browser.root_tree_id === publicationAccepted.root_tree_id,
  );
  await proved(
    "ui-conflict-resolution-preserves-source",
    browser.conflict.status === "PASS" &&
      browser.conflict.base_revision_id === publicationAccepted.revision_id,
  );
  await proved(
    "ui-desktop-tablet-mobile",
    browser.screenshots.length === 12 &&
      (
        await Promise.all(browser.screenshots.map((file) => fs.stat(file)))
      ).every((stat) => stat.size > 0),
  );
  const publication = await f.client.call("source.publications.read", {
    operation_id: browser.publication.operation_id,
  });
  await proved(
    "ui-publication-has-explicit-durable-status",
    publication.status === "PUBLISHED" &&
      publication.acceptance_id === publicationAccepted.acceptance_id &&
      publication.revision_id === publicationAccepted.revision_id,
  );
  const git = async (args) =>
    (
      await execute(
        "git",
        ["--git-dir=" + path.join(root, "publication.git"), ...args],
        { timeout: 30000, maxBuffer: 4 * 1024 * 1024 },
      )
    ).stdout;
  const actualOid = (await git(["rev-parse", "refs/heads/main"])).trim(),
    nativeFile = await f.service.trees.lookup(
      f.scope,
      publicationAccepted.root_tree_id,
      "main.txt",
    ),
    nativeBytes = await f.service.objects.readFile(
      f.scope,
      nativeFile.object_id,
    );
  const publishedBytes = await git(["show", "refs/heads/main:main.txt"]),
    nativeRevision = await f.client.revision(publicationAccepted.revision_id);
  const author = {
    name: (await git(["show", "-s", "--format=%an", "refs/heads/main"])).trim(),
    email: (
      await git(["show", "-s", "--format=%ae", "refs/heads/main"])
    ).trim(),
  };
  assert.equal(author.name, nativeRevision.descriptor.provenance.author.name);
  assert.equal(author.email, nativeRevision.descriptor.provenance.author.email);
  await git(["fsck", "--full", "--strict"]);
  await proved(
    "independent-git-readback-matches-native",
    publication.git_oid === "git:sha1:" + actualOid &&
      Buffer.from(publishedBytes).equals(nativeBytes),
  );
  const output = path.join(f.ROOT, "build/source-control/full-case/SCM-023");
  await fs.mkdir(output, { recursive: true });
  const { captureNativeSource } =
    await import("../source-control/full_case/native_capture.mjs");
  const nativeCapture = await captureNativeSource({
    client: f.client,
    context: f.context,
    revisionId: publicationAccepted.revision_id,
    workspaceRoot: f.ROOT,
    outputDirectory: path.join(output, "native"),
    assertionIds: [
      ...assertions.map((row) => row.id),
      "SCM-023.genuine-native-source-capture",
    ],
    toolchain: [process.execPath],
  });
  await proved(
    "genuine-native-source-capture",
    nativeCapture.revision_id === publicationAccepted.revision_id &&
      nativeCapture.root_tree_id === publicationAccepted.root_tree_id,
  );
  const snapshot = (revision) => ({
    revision_id: revision.revision_id,
    root_tree_id: revision.root_tree_id,
  });
  const snapshots = {
    baseline: snapshot(baseline),
    checkpoint_one: snapshot(first),
    checkpoint_two: snapshot(second),
    selected_submission: snapshot(
      await f.client.revision(selectedSubmission.revision_id),
    ),
    accepted: snapshot(publicationAccepted),
    sparse_checkpoint: sourceDiff.after,
  };
  const evidence = {
    schema_version: 1,
    status: "PASS",
    case_id: "SCM-023",
    variant: "owned-api-cli-ui",
    execution_profile:
      "owned-postgresql-sigv4-local-signed-worker-and-local-git",
    owned_temporary_root: root,
    owned_endpoints: [
      inputs.origin,
      f.endpoint,
      f.backend.config.endpoint,
      inputs.source_backend_endpoint,
    ],
    source: {
      scope: f.scope,
      revision_id: publicationAccepted.revision_id,
      root_tree_id: publicationAccepted.root_tree_id,
    },
    assertions,
    steps,
    snapshots,
    selected_assignment: selectedSubmission.assignment_id,
    source_diff: sourceDiff,
    sparse_checkpoint: sparseCheckpoint,
    assignment: original.assignment_id,
    submission: submission.submission_id,
    qualification: replacementJourney.taskQualification.qualification_id,
    original_qualification: taskQualification.qualification_id,
    replacement: replacementJourney,
    acceptance: publicationAccepted.acceptance_id,
    publication,
    native_capture: nativeCapture,
    git_readback: {
      oid: actualOid,
      author,
      bytes: nativeBytes.length,
      sha256: sha(nativeBytes),
    },
    browser,
  };
  await fs.writeFile(
    path.join(output, "journey.json"),
    JSON.stringify(evidence, null, 2) + "\n",
  );
  return evidence;
}
