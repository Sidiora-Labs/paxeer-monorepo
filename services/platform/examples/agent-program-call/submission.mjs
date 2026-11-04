import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { once } from "node:events";
import { lstat, mkdir, open, readFile, rename } from "node:fs/promises";
import { createServer } from "node:http";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { decodeRawEip1559, keccak256, recoverEip1559Sender } from "@sidiora/layerx-sdk";

const HASH = /^0x[0-9a-f]{64}$/u;
const ID = /^[0-9a-f]{64}$/u;
const READ_METHODS = new Set([
  "eth_chainId", "eth_getTransactionCount", "eth_gasPrice",
  "eth_maxPriorityFeePerGas", "eth_estimateGas",
]);
const ACTION = "Query receipt get with this transaction hash; do not resubmit the call";

function rpcEndpoint(value) {
  const url = new URL(value);
  if (url.username || url.password || url.search || url.hash || !url.pathname.endsWith("/rpc")
    || (url.protocol !== "https:" && !(url.protocol === "http:"
      && ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)))) {
    throw new Error("invalid_submission_rpc_endpoint");
  }
  return url.href;
}

async function syncDirectory(path) {
  const directory = await open(dirname(path), "r");
  try { await directory.sync(); } finally { await directory.close(); }
}

async function writeRecord(path, record, exclusive = false) {
  const target = exclusive ? path : `${path}.${randomBytes(16).toString("hex")}.tmp`;
  const file = await open(target, "wx", 0o600);
  try {
    await file.writeFile(`${JSON.stringify(record)}\n`);
    await file.sync();
  } finally {
    await file.close();
  }
  if (!exclusive) await rename(target, path);
  await syncDirectory(path);
}

export async function readSubmissionRecord(path, rpcUrl) {
  const stat = await lstat(path);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size > 16384) {
    throw new Error("invalid_submission_record_file");
  }
  const record = JSON.parse(await readFile(path, "utf8"));
  if (record === null || typeof record !== "object" || Array.isArray(record)
    || record.version !== 1 || typeof record.program_id !== "string" || !ID.test(record.program_id)
    || typeof record.interface_digest !== "string"
    || !(record.interface_digest === "absent" || ID.test(record.interface_digest))
    || record.rpc_url !== rpcEndpoint(rpcUrl)
    || !["preparing", "refused", "attempted", "acknowledged"].includes(record.state)
    || (["attempted", "acknowledged"].includes(record.state)
      ? !(typeof record.tx_hash === "string" && HASH.test(record.tx_hash)) : record.tx_hash !== null)) {
    throw new Error("invalid_submission_record");
  }
  return record;
}

export function submissionResult(record) {
  const result = {
    program_id: record.program_id,
    interface_digest: record.interface_digest,
    submission_state: record.state,
    status: record.state === "refused" ? "local-or-deterministic-refusal" : "unknown-fate",
    action: record.state === "refused"
      ? "Submission was not attempted; inspect the CLI failure before starting a new call"
      : record.tx_hash === null
        ? "Preserve the submission journal and investigate interrupted preparation; do not resubmit the call"
        : ACTION,
  };
  if (record.state === "refused") result.submitted = false;
  if (record.tx_hash !== null) {
    result.tx_hash = record.tx_hash;
    result.rung = "submitted";
  }
  return result;
}

async function boundedBody(stream) {
  const chunks = [];
  let size = 0;
  for await (const chunk of stream) {
    size += chunk.length;
    if (size > 2097152) throw new Error("submission_body_too_large");
    chunks.push(Buffer.from(chunk));
  }
  return Buffer.concat(chunks).toString("utf8");
}

export async function createSubmissionRelay(path, rpcUrl, programId, interfaceDigest) {
  const endpoint = rpcEndpoint(rpcUrl);
  if (!ID.test(programId) || !(interfaceDigest === "absent" || ID.test(interfaceDigest))) {
    throw new Error("invalid_submission_identity");
  }
  await mkdir(dirname(path), { recursive: true, mode: 0o700 });
  let record = {
    version: 1, program_id: programId, interface_digest: interfaceDigest,
    rpc_url: endpoint, state: "preparing", tx_hash: null,
  };
  // Exclusive creation is the submission claim. Existing records are recovery-only.
  await writeRecord(path, record, true);
  let attempted = false;
  let finished = false;
  let closing;
  const pending = new Set();
  const shutdown = new AbortController();
  const route = `/${randomBytes(32).toString("hex")}/rpc`;
  async function handle(request, response) {
    let id = null;
    try {
      if (request.method !== "POST" || request.url !== route || finished) {
        throw new Error("submission_route_refused");
      }
      const body = await boundedBody(request);
      if (finished) throw new Error("submission_relay_stopped");
      const message = JSON.parse(body);
      if (message === null || typeof message !== "object" || Array.isArray(message)
        || message.jsonrpc !== "2.0" || !Array.isArray(message.params)
        || !(typeof message.id === "string" || Number.isSafeInteger(message.id))) {
        throw new Error("invalid_submission_request");
      }
      id = message.id;
      const send = message.method === "eth_sendRawTransaction";
      if (send) {
        const raw = message.params[0];
        if (attempted || message.params.length !== 1 || typeof raw !== "string"
          || !/^0x02(?:[0-9a-fA-F]{2})+$/u.test(raw)) {
          throw new Error("submission_already_attempted_or_invalid");
        }
        const bytes = Buffer.from(raw.slice(2), "hex");
        decodeRawEip1559(bytes);
        recoverEip1559Sender(bytes);
        // Claim synchronously so concurrent requests cannot sign/send a second call.
        attempted = true;
        const txHash = `0x${Buffer.from(keccak256(bytes)).toString("hex")}`;
        record = { ...record, state: "attempted", tx_hash: txHash };
        await writeRecord(path, record);
        process.stderr.write(`${JSON.stringify({ tx_hash: txHash, status: "unknown-fate", action: ACTION })}\n`);
        if (finished) throw new Error("submission_relay_stopped");
      } else if (!READ_METHODS.has(message.method)) {
        throw new Error("submission_method_refused");
      }
      const headers = { "content-type": "application/json", accept: "application/json" };
      if (request.headers.authorization) headers.authorization = request.headers.authorization;
      const upstream = await fetch(endpoint, {
        method: "POST", headers, body, redirect: "error",
        signal: AbortSignal.any([shutdown.signal, AbortSignal.timeout(25000)]),
      });
      const reply = await boundedBody(upstream.body);
      if (send) {
        const acknowledgement = JSON.parse(reply);
        if (!upstream.ok || acknowledgement?.jsonrpc !== "2.0" || acknowledgement.id !== id
          || acknowledgement.error !== undefined || typeof acknowledgement.result !== "string"
          || acknowledgement.result.toLowerCase() !== record.tx_hash) {
          throw new Error("submission_acknowledgement_unknown");
        }
        record = { ...record, state: "acknowledged" };
        await writeRecord(path, record);
      }
      response.writeHead(upstream.status, { "content-type": "application/json" });
      response.end(reply);
    } catch {
      if (!response.destroyed) {
        response.writeHead(502, { "content-type": "application/json" });
        response.end(JSON.stringify({ jsonrpc: "2.0", id,
          error: { code: -32000, message: "Submission relay failed; recover the recorded hash; do not resubmit" } }));
      }
    }
  }
  const server = createServer((request, response) => {
    const task = handle(request, response);
    pending.add(task);
    void task.finally(() => pending.delete(task));
  });
  server.requestTimeout = 30000;
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  function close(proveRefusal) {
    closing ??= (async () => {
      finished = true;
      shutdown.abort();
      server.closeAllConnections();
      await new Promise((accept, reject) => server.close((error) => error ? reject(error) : accept()));
      await Promise.all([...pending]);
      if (proveRefusal && !attempted) {
        record = { ...record, state: "refused" };
        await writeRecord(path, record);
      }
      return readSubmissionRecord(path, endpoint);
    })();
    return closing;
  }
  return {
    rpcUrl: `http://127.0.0.1:${server.address().port}${route}`,
    finish() { return close(true); },
    stop() { return close(false); },
  };
}

export async function submitProgramCall(path, rpcUrl, programId, interfaceDigest, keyFile) {
  let relay;
  try {
    relay = await createSubmissionRelay(path, rpcUrl, programId, interfaceDigest);
  } catch (error) {
    if (error.code === "EEXIST") return readSubmissionRecord(path, rpcUrl);
    throw error;
  }
  try {
    const child = spawn("layerx", ["--json", "--rpc", relay.rpcUrl, "program", "call", programId,
      "--fuel", "1000000", "--fee-limit", "50000000", "--evm-key-file", keyFile],
    { stdio: ["ignore", "ignore", "inherit"] });
    // A CLI exit is never evidence of refusal after the relay has attempted the send.
    await new Promise((accept) => {
      child.once("error", accept);
      child.once("exit", accept);
    });
  } finally {
    await relay.finish();
  }
  return readSubmissionRecord(path, rpcUrl);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const [mode, path, rpcUrl, programId, interfaceDigest, keyFile] = process.argv.slice(2);
    let record;
    if (mode === "recover" && process.argv.length === 5) {
      record = await readSubmissionRecord(path, rpcUrl);
    } else if (mode === "submit" && process.argv.length === 8) {
      record = await submitProgramCall(path, rpcUrl, programId, interfaceDigest, keyFile);
    } else {
      throw new Error("invalid_submission_arguments");
    }
    console.log(JSON.stringify(submissionResult(record)));
  } catch {
    console.log(JSON.stringify({ status: "unknown-fate", action: ACTION,
      error: "Submission record unavailable; restore the record before taking further action" }));
    process.exitCode = 75;
  }
}
