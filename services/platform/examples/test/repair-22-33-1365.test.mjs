import assert from "node:assert/strict";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { setTimeout as delay } from "node:timers/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { command } from "../build-reference-apps.mjs";

const fixture = fileURLToPath(new URL("./fixtures/reference-build-process.mjs", import.meta.url));
const cwd = fileURLToPath(new URL("../", import.meta.url));
const limits = { timeoutMs: 2000, terminationGraceMs: 150, maxOutputBytes: 2048, diagnosticBytes: 64 };

async function peer(t) {
  const exchanges = [];
  const failures = [];
  const server = createServer(async (req, res) => {
    try {
      assert.equal(req.method, "POST");
      assert.equal(req.url, "/command");
      assert.equal(req.headers["content-type"], "application/json");
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks));
      assert.ok(["ready", "term"].includes(body.event));
      assert.equal(typeof body.mode, "string");
      assert.ok(Number.isSafeInteger(body.pid) && body.pid > 0);
      const response = { ok: true, data: { bytes: 3072, code_hash: "recorded-reference-artifact" } };
      exchanges.push({ request: body, response });
      res.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify(response));
    } catch (error) {
      failures.push(error);
      res.writeHead(500).end();
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    // Emergency teardown only: assertions below establish cleanup before this.
    for (const { request } of exchanges) {
      if (request.event !== "ready" || request.mode === "descendant") continue;
      try { process.kill(-request.pid, "SIGKILL"); }
      catch (error) { if (error.code !== "ESRCH") throw error; }
    }
    server.closeAllConnections();
    await new Promise((accept, reject) => server.close((error) => error ? reject(error) : accept()));
    assert.deepEqual(failures, []);
  });
  return { url: `http://127.0.0.1:${server.address().port}/command`, exchanges };
}

async function assertStopped(pid, reaped = false) {
  // Orphan descendants may briefly remain as zombies awaiting the sandbox's
  // init; they cannot execute. Our own command leader must already be reaped.
  for (let attempt = 0; attempt < 100; attempt++) {
    try {
      const status = await readFile(`/proc/${pid}/status`, "utf8");
      if (!reaped && /^State:\s+Z\b/m.test(status)) return;
    } catch (error) {
      if (error.code === "ENOENT") return;
      throw error;
    }
    await delay(10);
  }
  assert.fail(`process ${pid} survived command settlement`);
}

function run(peer_, mode, options = limits) {
  return command(process.execPath, [fixture, peer_.url, mode, "argument with spaces", "--literal"], cwd, options);
}

async function referenceBuildCommandPreservesRealResponse(t) {
  const server = await peer(t);
  const result = JSON.parse(await run(server, "success"));
  assert.equal(server.exchanges.length, 1);
  const { request, response } = server.exchanges[0];
  assert.deepEqual(request, {
    event: "ready", mode: "success", pid: request.pid,
    arguments: ["argument with spaces", "--literal"], cwd: cwd.replace(/\/$/, ""), jobs: "4", descendant: null,
  });
  assert.deepEqual(result, response);
  await assertStopped(request.pid, true);
}

async function referenceBuildCommandReportsBoundedFailure(t) {
  const server = await peer(t);
  await assert.rejects(run(server, "fails"), (error) => {
    assert.equal(error.code, `reference_build_failed_${process.execPath}_7`);
    assert.equal(error.exitCode, 7);
    assert.equal(error.signal, null);
    assert.equal(Buffer.byteLength(error.stdout), limits.diagnosticBytes);
    assert.equal(Buffer.byteLength(error.stderr), limits.diagnosticBytes);
    assert.ok(error.stdout.endsWith("stdout-tail"));
    assert.ok(error.stderr.endsWith("stderr-tail"));
    return true;
  });
  assert.equal(server.exchanges[0].request.mode, "fails");
  await assertStopped(server.exchanges[0].request.pid, true);
}

async function referenceBuildDeadlineKillsTermResistantGroup(t) {
  for (const mode of ["tree", "leader-exits", "closes-pipes"]) {
    await t.test(mode, async (t) => {
      const server = await peer(t);
      const started = performance.now();
      await assert.rejects(run(server, mode), { code: "reference_build_deadline" });
      const elapsed = performance.now() - started;
      assert.ok(elapsed >= limits.timeoutMs + limits.terminationGraceMs - 20, `cleanup was not awaited: ${elapsed}`);
      assert.ok(elapsed < 5000, `deadline did not bound settlement: ${elapsed}`);
      const leader = server.exchanges.find(({ request }) => request.mode === mode && request.event === "ready")?.request;
      assert.ok(leader, "real child reached HTTP peer before deadline");
      await assertStopped(leader.pid, true);
      if (mode !== "closes-pipes") {
        const descendant = server.exchanges.find(({ request }) => request.mode === "descendant" && request.event === "ready")?.request;
        assert.equal(leader.descendant, descendant.pid);
        assert.equal(descendant.parent, leader.pid);
        assert.ok(server.exchanges.some(({ request }) => request.pid === descendant.pid && request.event === "term"), "descendant received TERM before KILL");
        await assertStopped(descendant.pid);
      }
    });
  }
}

async function referenceBuildOverflowKillsAndBoundsBothStreams(t) {
  for (const mode of ["stdout-overflow", "stderr-overflow", "combined-overflow"]) {
    await t.test(mode, async (t) => {
      const server = await peer(t);
      const started = performance.now();
      await assert.rejects(run(server, mode), (error) => {
        assert.equal(error.code, "reference_build_output_bound");
        assert.equal(error.signal, "SIGKILL");
        assert.ok(Buffer.byteLength(error.stdout) <= limits.diagnosticBytes);
        assert.ok(Buffer.byteLength(error.stderr) <= limits.diagnosticBytes);
        if (mode !== "stderr-overflow") assert.equal(error.stdout, "X".repeat(limits.diagnosticBytes));
        if (mode !== "stdout-overflow") assert.equal(error.stderr, (mode === "combined-overflow" ? "Y" : "X").repeat(limits.diagnosticBytes));
        return true;
      });
      assert.ok(performance.now() - started < limits.timeoutMs, "overflow must trigger cleanup before the deadline");
      const leader = server.exchanges.find(({ request }) => request.event === "ready")?.request;
      assert.equal(leader.mode, mode);
      assert.ok(server.exchanges.some(({ request }) => request.event === "term" && request.pid === leader.pid));
      await assertStopped(leader.pid, true);
    });
  }
}

async function referenceBuildFailedLeaderCleansDescendants(t) {
  const server = await peer(t);
  await assert.rejects(run(server, "tree-fails"), (error) => {
    assert.equal(error.exitCode, 7);
    assert.equal(error.code, `reference_build_failed_${process.execPath}_7`);
    assert.ok(error.stderr.endsWith("stderr-tail"));
    return true;
  });
  const leader = server.exchanges.find(({ request }) => request.mode === "tree-fails")?.request;
  const descendant = server.exchanges.find(({ request }) => request.mode === "descendant")?.request;
  assert.ok(leader && descendant);
  assert.equal(leader.descendant, descendant.pid);
  assert.ok(server.exchanges.some(({ request }) => request.event === "term" && request.pid === descendant.pid));
  await assertStopped(leader.pid, true);
  await assertStopped(descendant.pid);
}

async function referenceBuildSpawnFailureAndLimits(t) {
  await assert.rejects(command(process.execPath, [fixture], "/missing-reference-build-directory", limits), (error) => {
    assert.ok(error instanceof Error);
    assert.equal(error.code, "reference_build_spawn_failed");
    assert.equal(error.message, "reference_build_spawn_failed");
    assert.ok(error.cause instanceof Error);
    assert.equal(error.cause.message, `spawn ${process.execPath} ENOENT`);
    assert.deepEqual({ ...error.cause }, {
      errno: -2, code: "ENOENT", syscall: `spawn ${process.execPath}`,
      path: process.execPath, spawnargs: [fixture],
    });
    return true;
  });
  for (const invalid of [{ timeoutMs: 0 }, { terminationGraceMs: -1 }, { maxOutputBytes: Infinity }, { diagnosticBytes: 16385 }]) {
    await assert.rejects(command(process.execPath, [fixture], cwd, invalid), { message: "invalid_reference_build_command_limits" });
  }
}

test(referenceBuildCommandPreservesRealResponse);
test(referenceBuildCommandReportsBoundedFailure);
test(referenceBuildDeadlineKillsTermResistantGroup);
test(referenceBuildOverflowKillsAndBoundsBothStreams);
test(referenceBuildFailedLeaderCleansDescendants);
test(referenceBuildSpawnFailureAndLimits);
