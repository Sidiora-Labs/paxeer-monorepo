import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdir, open, readFile, stat, writeFile } from "node:fs/promises";
import { isAbsolute, resolve } from "node:path";
import { StringDecoder } from "node:string_decoder";
import { pathToFileURL } from "node:url";
import { referenceArtifacts } from "./support/artifacts.mjs";

const root = resolve(import.meta.dirname, "../../..");
async function main() {
  const environment = process.env.PAXEER_X_REFERENCE_APPS_ENVIRONMENT ?? "emulator";
  assert.ok(["emulator", "beta"].includes(environment), "unknown reference environment");
  const evidence = process.env.PAXEER_X_EVIDENCE_DIR;
  assert.ok(typeof evidence === "string" && isAbsolute(evidence) && resolve(evidence) === evidence
    && !evidence.startsWith(`${root}/`), "private evidence directory required");
  const info = await stat(evidence);
  assert.ok(info.isDirectory() && info.uid === process.getuid() && (info.mode & 0o077) === 0, "unsafe evidence directory");
  const artifacts = await referenceArtifacts(process.env.PAXEER_X_REFERENCE_APP_ARTIFACTS, root);
  const directory = resolve(evidence, `reference-apps-${Date.now()}-${process.pid}`);
  await mkdir(directory, { mode: 0o700 });
  const log = await open(resolve(directory, "actual-processes.log"), "wx", 0o600);
  const rows = [];
  let pending = "";
  let total = 0;
  let code;
  let processLog = Promise.resolve();
  const decoder = new StringDecoder("utf8");
  try {
    code = await new Promise((accept, reject) => {
      const child = spawn(process.execPath, ["services/platform/examples/run-reference-apps.mjs", "--scenario", environment], {
        cwd: root, env: { ...process.env, LAYERX_BIN: artifacts.cli.path,
          LAYERX_EXAMPLE_STATE_ROOT: resolve(directory, "state") }, stdio: ["ignore", "pipe", "pipe"], detached: true,
      });
      let failure;
      let killing;
      const terminate = () => {
        try { process.kill(-child.pid, "SIGTERM"); } catch (error) { if (error.code !== "ESRCH") failure ??= error; }
        killing ??= setTimeout(() => {
          try { process.kill(-child.pid, "SIGKILL"); } catch (error) { if (error.code !== "ESRCH") failure ??= error; }
        }, 10_000);
      };
      const fail = (error) => { failure ??= error; terminate(); };
      process.once("SIGTERM", terminate);
      process.once("SIGINT", terminate);
      const timeout = setTimeout(() => { fail(new Error("reference_runtime_deadline")); }, 20 * 60 * 1000);
      const write = async (chunk) => {
        total += chunk.length;
        if (total > 16 * 1024 * 1024) { fail(new Error("reference_output_bound")); return; }
        processLog = processLog.then(() => log.write(chunk));
        await processLog;
      };
      child.stderr.on("data", (chunk) => { void write(chunk).catch(fail); });
      child.stdout.on("data", (chunk) => {
        void write(chunk).catch(fail);
        pending += decoder.write(chunk);
        for (;;) {
          const newline = pending.indexOf("\n");
          if (newline < 0) break;
          const line = pending.slice(0, newline); pending = pending.slice(newline + 1);
          if (line.startsWith("{")) {
            try { rows.push(JSON.parse(line)); } catch { fail(new Error("invalid_reference_process_json")); }
          }
        }
      });
      child.once("error", fail);
      child.once("close", (result, signal) => {
        clearTimeout(timeout); clearTimeout(killing);
        process.removeListener("SIGTERM", terminate); process.removeListener("SIGINT", terminate);
        if (failure !== undefined) reject(failure);
        else if (signal !== null) reject(new Error("reference_process_interrupted"));
        else accept(result);
      });
    });
    assert.equal(code, 0, "actual reference application failed");
    const outcomesFile = resolve(directory, "recorded-outcomes.json");
    await writeFile(outcomesFile, `${JSON.stringify(rows)}\n`, { flag: "wx", mode: 0o600 });
    const { buyer, merchant, marketplace } = await qualifyReferenceAppOutcomes(outcomesFile, environment);
    const record = { source_revision: artifacts.source_revision, environment, tests: 6, skipped: 0,
      buyer_receipt: buyer.receiptDigest, buyer_transaction: buyer.transactionHash, buyer_rung: buyer.rung,
      merchant_receipt: merchant.receiptDigest, merchant_transaction: merchant.transactionHash, merchant_rung: merchant.rung,
      marketplace: marketplace.map((row) => ({ action: row.action, receipt: row.receiptDigest, transactionHash: row.transactionHash,
        rung: row.rung, resultCode: row.resultCode })) };
    const output = await open(resolve(directory, "outcomes.json"), "wx", 0o600);
    try { await output.writeFile(`${JSON.stringify(record, null, 2)}\n`); await output.sync(); } finally { await output.close(); }
    process.stdout.write(`${JSON.stringify({ source_revision: artifacts.source_revision, environment, evidence: directory, tests: 6, skipped: 0 })}\n`);
    process.stdout.write("PAXEER_X_GATE tests=6 skipped=0\n");
  } finally {
    await processLog;
    await log.sync();
    await log.close();
  }
}

export async function qualifyReferenceAppOutcomes(outcomesFile, environment) {
  assert.ok(["emulator", "beta"].includes(environment), "unknown reference environment");
  const rows = JSON.parse(await readFile(outcomesFile, "utf8"));
  assert.ok(Array.isArray(rows), "recorded reference outcomes must be an array");
  const only = (predicate, name) => {
    const found = rows.filter(predicate);
    assert.equal(found.length, 1, `missing or duplicate real outcome: ${name}`);
    return found[0];
  };
  const seller = only((row) => row.application === "paid-api" && row.environment === environment && row.path === "/paid", "paid API process");
  assert.ok(Number.isSafeInteger(seller.listening) && seller.listening > 0);
  const buyer = only((row) => row.application === "buyer-agent" && row.environment === environment && row.state === "paid", "buyer payment");
  assert.equal(buyer.status, 200);
  assert.ok(["receipted", "committed"].includes(buyer.rung), "buyer payment rung must be receipted or committed");
  assert.match(buyer.receiptDigest, /^[0-9a-f]{64}$/u);
  assert.match(buyer.transactionHash, /^0x[0-9a-fA-F]{64}$/u, "invalid buyer transaction hash");
  const declaredResource = JSON.parse(await readFile(resolve(root, "services/platform/examples/paid-api/resource.json"), "utf8"));
  assert.deepEqual(JSON.parse(buyer.body), declaredResource, "seller did not release actual declared resource after verified payment");
  const merchant = only((row) => row.application === "merchant-shop" && row.environment === environment && row.state === "paid", "verified merchant checkout");
  assert.ok(["receipted", "committed"].includes(merchant.rung), "merchant checkout rung must be receipted or committed");
  assert.equal(merchant.resultCode, 0, "merchant checkout result code must indicate success");
  assert.match(merchant.receiptDigest, /^[0-9a-f]{64}$/u);
  assert.match(merchant.transactionHash, /^0x[0-9a-fA-F]{64}$/u, "invalid merchant transaction hash");
  assert.ok(typeof merchant.orderId === "string" && merchant.orderId.length > 0);
  const marketplace = [];
  for (const action of ["deploy", "list", "buy"]) {
    const outcome = only((row) => row.application === "marketplace" && row.environment === environment && row.action === action, `marketplace ${action}`);
    assert.equal(outcome.state, "completed");
    assert.ok(["receipted", "committed"].includes(outcome.rung), `marketplace ${action} rung must be receipted or committed`);
    assert.equal(outcome.resultCode, 0, `marketplace ${action} result code must indicate success`);
    assert.match(outcome.receiptDigest, /^[0-9a-f]{64}$/u);
    assert.match(outcome.transactionHash, /^0x[0-9a-fA-F]{64}$/u, `invalid marketplace ${action} transaction hash`);
    marketplace.push(outcome);
  }
  assert.equal(new Set([buyer.receiptDigest, merchant.receiptDigest, ...marketplace.map((row) => row.receiptDigest)]).size, 5,
    "distinct real economic operations reused a receipt");
  assert.equal(new Set([buyer, merchant, ...marketplace].map((row) => row.transactionHash.toLowerCase())).size, 5,
    "distinct real economic operations reused a transaction");
  const completed = only((row) => row.environment === environment && row.state === "completed" && Array.isArray(row.applications), "full reference suite");
  assert.deepEqual(completed.applications, ["buyer-agent", "paid-api", "merchant-shop", "marketplace"]);
  return { buyer, merchant, marketplace };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  await main();
}
