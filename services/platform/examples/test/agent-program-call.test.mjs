import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, mkdir, rm, symlink, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { delimiter, join } from "node:path";
import test, { before, after } from "node:test";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { keccak_256 } from "@noble/hashes/sha3.js";

const execute = promisify(execFile);
const root = fileURLToPath(new URL("../../../../", import.meta.url));
const walkthrough = fileURLToPath(new URL("../agent-program-call/walkthrough.sh", import.meta.url));
const programsAddress = "0x000000000000000000000000000000000000101d";
const programId = "11".repeat(32);
const codeHash = "33".repeat(32);
const deployedTopic = "0x2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709";
const sender = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf";
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
let scratch;
let cli;

before(async () => {
  scratch = await mkdtemp(join(tmpdir(), "agent-program-call-"));
  await execute("npm", ["run", "build", "--workspace", "@sidiora/layerx-sdk"], {
    cwd: root, timeout: 600000, maxBuffer: 8 * 1024 * 1024,
  });
  const target = join(scratch, "target");
  const result = await execute("cargo", [
    "build", "--manifest-path", join(root, "tools/platform/Cargo.toml"),
    "--package", "layerx-platform-cli", "--bin", "layerx", "--target-dir", target,
  ], { cwd: root, timeout: 600000, maxBuffer: 8 * 1024 * 1024 });
  assert.equal(result.stdout.includes("error:"), false, result.stdout);
  cli = join(target, "debug/layerx");
});

after(async () => {
  if (scratch) await rm(scratch, { recursive: true, force: true });
});

function word(value) {
  return BigInt.asUintN(256, BigInt(value)).toString(16).padStart(64, "0");
}

function abiOutcome(result, kind, response = "") {
  return `0x${word(result)}${word(kind)}${word(96)}${word(response.length / 2)}${response.padEnd(Math.ceil(response.length / 64) * 64, "0")}`;
}

function decodeRlp(encoded) {
  function item(offset) {
    const tag = encoded[offset];
    if (tag < 0x80) return [encoded.subarray(offset, offset + 1), offset + 1];
    const list = tag >= 0xc0;
    const base = list ? 0xc0 : 0x80;
    let length = tag - base;
    let start = offset + 1;
    if (length > 55) {
      const width = length - 55;
      length = Number(BigInt(hex(encoded.subarray(start, start + width))));
      start += width;
    }
    const end = start + length;
    assert.ok(end <= encoded.length);
    if (!list) return [encoded.subarray(start, end), end];
    const values = [];
    while (start < end) {
      const [value, next] = item(start);
      values.push(value);
      start = next;
    }
    assert.equal(start, end);
    return [values, end];
  }
  const [value, end] = item(0);
  assert.equal(end, encoded.length);
  return value;
}

function signedReceipt(raw, returnData, pair) {
  const uint = (value, width) => Buffer.from(BigInt(value).toString(16).padStart(width * 2, "0"), "hex");
  const bounded = (value) => Buffer.concat([uint(value.length, 4), value]);
  const sha256 = (value) => createHash("sha256").update(value).digest();
  const txHash = hex(keccak_256(raw));
  const publicKey = hex(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
  const keyId = Buffer.alloc(32, 0x55);
  const zero = Buffer.alloc(32);
  const prefix = Buffer.concat([
    uint(2, 2), uint(0x5201, 2), uint(2, 2), bounded(bytes(txHash)), uint(1, 8),
    bounded(zero), bounded(sha256(raw)), bounded(sha256(bytes(returnData))), uint(0, 4), uint(0, 4),
    uint(0, 16), bounded(uint(7, 32)), uint(12, 2), uint(1, 4), uint(0, 4), Buffer.of(2),
    bounded(zero), uint(0, 16), bounded(zero), uint(0, 16), uint(0, 16), uint(0, 8),
    bounded(zero), uint(0, 16), uint(0, 16), bounded(zero), bounded(zero), bounded(keyId), uint(1, 8),
  ]);
  const digest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), prefix, Buffer.of(0)]));
  const receipt = hex(Buffer.concat([prefix, Buffer.of(1), bounded(sign(null, digest, pair.privateKey))]));
  return { txHash, rung: "receipted", status: "0x1", returnData, vmError: "", receipt, publicKey };
}

async function runWalkthrough(t, refused, receiptState = "receipted", unpinned = false) {
  const signer = generateKeyPairSync("ed25519");
  const pinnedKey = unpinned ? generateKeyPairSync("ed25519").publicKey : signer.publicKey;
  const directory = await mkdtemp(join(scratch, "case-"));
  const bin = join(directory, "bin");
  await mkdir(bin);
  await symlink(cli, join(bin, "layerx"));
  const keyFile = join(directory, "evm-key.json");
  await writeFile(keyFile, JSON.stringify({
    index: 0,
    evm_address: sender, evm_private_key: "1".padStart(64, "0"),
    did: "did:layerx:8a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c",
    layerx_seed: "01".repeat(32),
  }), { mode: 0o600 });
  const requests = [];
  const errors = [];
  let transaction;
  let receipt;
  let estimate;
  const programInterface = Buffer.concat([
    Buffer.from("LayerX/program-interface/v1\0"), Buffer.from(codeHash, "hex"),
    Buffer.from("00020001000463616c6c010203040202000000000000", "hex"),
  ]);
  const interfaceDigest = createHash("sha256").update(programInterface).digest("hex");
  const simulation = abiOutcome(refused ? -1 : 0, refused ? 2 : 1, "aabb");
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = Buffer.concat(chunks).toString("utf8");
      response.setHeader("Content-Type", "application/json");
      if (request.method === "GET") {
        requests.push({ method: "GET", path: request.url });
        if (request.url === "/v1/programs/registry") {
          response.end(JSON.stringify({ value: {
            program_ids: [programId], observed_sequence: 1, state_root: "22".repeat(32),
          } }));
        } else {
          assert.equal(request.url, `/v1/programs/registry/${programId}`);
          assert.equal(JSON.parse(body).program_id, programId);
          response.end(JSON.stringify({
            request_id: "walkthrough-discovery", verification_status: { state: "Unverified", reason: "local transport fixture" },
            value: { program_id: programId, lifecycle: "active", interface_status: "absent",
              observed_sequence: 1, state_root: "22".repeat(32) },
          }));
        }
        return;
      }
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/rpc");
      const message = JSON.parse(body);
      requests.push(message);
      assert.equal(message.jsonrpc, "2.0");
      let result;
      switch (message.method) {
        case "eth_getLogs":
          assert.deepEqual(message.params, [{
            address: programsAddress, fromBlock: "earliest", toBlock: "latest", topics: [deployedTopic],
          }]);
          result = [{
            address: programsAddress, removed: false, topics: [deployedTopic, `0x${programId}`],
            data: `0x${codeHash}`,
          }];
          break;
        case "eth_call":
          assert.equal(message.params[0].to, programsAddress);
          assert.equal(message.params[1], "pending");
          if (message.params[0].data.startsWith("0xb1ffb3d4")) {
            assert.equal(message.params[0].data, `0xb1ffb3d4${programId}`);
            result = `0x${word(0)}${"22".repeat(32)}${codeHash}${word(2)}${word(1)}${word(1)}`;
            break;
          }
          if (message.params[0].data.startsWith("0x8efc7240")) {
            assert.equal(message.params[0].data, `0x8efc7240${programId}`);
            const encoded = programInterface.toString("hex");
            result = `0x${word(1)}${interfaceDigest}${word(96)}${word(programInterface.length)}${encoded.padEnd(Math.ceil(encoded.length / 64) * 64, "0")}`;
            break;
          }
          assert.match(message.params[0].data, /^0x3044bd56/u);
          assert.equal(message.params[0].data.slice(10, 74), programId);
          assert.equal(BigInt(`0x${message.params[0].data.slice(10 + 7 * 64, 10 + 8 * 64)}`), 1000000n);
          assert.equal(BigInt(`0x${message.params[0].data.slice(10 + 14 * 64, 10 + 15 * 64)}`), 50000000n);
          result = simulation;
          break;
        case "eth_chainId": result = "0x7d"; break;
        case "eth_getTransactionCount":
          assert.deepEqual(message.params, [sender, "pending"]);
          result = "0x7";
          break;
        case "eth_gasPrice": result = "0x64"; break;
        case "eth_maxPriorityFeePerGas": result = "0x2"; break;
        case "eth_estimateGas":
          assert.equal(message.params[0].to, programsAddress);
          assert.equal(message.params[0].from, sender);
          assert.equal(message.params[1], "pending");
          estimate = message.params[0].data;
          assert.match(estimate, /^0x54bd2b48/u);
          assert.equal(estimate.slice(10), requests.find((entry) => entry.method === "eth_call"
            && entry.params[0].data.startsWith("0x3044bd56")).params[0].data.slice(10));
          result = "0x493e0";
          break;
        case "eth_sendRawTransaction": {
          assert.equal(refused, false);
          assert.equal(transaction, undefined, "submit exactly once");
          assert.equal(message.params.length, 1);
          transaction = bytes(message.params[0]);
          assert.equal(transaction[0], 2);
          const fields = decodeRlp(transaction.subarray(1));
          assert.equal(fields.length, 12);
          assert.equal(hex(fields[0]), "0x7d");
          assert.equal(hex(fields[1]), "0x07");
          assert.equal(hex(fields[5]), programsAddress);
          assert.equal(hex(fields[7]), estimate);
          assert.ok(fields[10].length > 0 && fields[11].length > 0);
          receipt = signedReceipt(transaction, abiOutcome(0, 1, "ccdd"), signer);
          result = receipt.txHash;
          break;
        }
        case "eth_getRawTransactionByHash":
          assert.ok(transaction);
          assert.deepEqual(message.params, [receipt.txHash]);
          result = hex(transaction);
          break;
        case "pax_sequencerReceipt":
          assert.ok(transaction);
          assert.deepEqual(message.params, [receipt.txHash]);
          result = receiptState === "receipted" ? receipt : { txHash: receipt.txHash, rung: "submitted" };
          break;
        default: assert.fail(`unexpected RPC method ${message.method}`);
      }
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) {
      errors.push(error);
      response.statusCode = 500;
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  t.after(() => new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve())));
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const configuration = join(directory, "config.json");
  await writeFile(configuration, JSON.stringify({
    version: 1, current_environment: "emulator", default_key: null, keys: {},
    environments: { emulator: { endpoint: `http://127.0.0.1:${server.address().port}`, network_id: 125 } },
  }));
  const env = Object.fromEntries(Object.entries(process.env).filter(([name]) => !name.startsWith("LAYERX_")));
  const output = await execute("bash", [walkthrough], {
    cwd: root, timeout: 30000, maxBuffer: 1024 * 1024,
    env: { ...env, PATH: `${bin}${delimiter}${env.PATH}`, LAYERX_CONFIG: configuration,
      LAYERX_CREDENTIAL_STORE: "file", LAYERX_CREDENTIAL_PASSPHRASE: "walkthrough local integration test",
      LAYERX_EVM_KEY_FILE: keyFile,
      LAYERX_RPC_URL: `http://127.0.0.1:${server.address().port}/rpc`,
      LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([{
        keyId: `0x${"55".repeat(32)}`,
        publicKey: hex(pinnedKey.export({ type: "spki", format: "der" }).subarray(-32)),
        firstHeight: "0", lastHeight: "10",
      }]) },
  });
  assert.deepEqual(errors, []);
  assert.equal(output.stderr, "");
  return { output: JSON.parse(output.stdout), requests, receipt };
}

async function walkthroughSimulatesThroughProgramsPrecompile(t) {
  const { output, requests } = await runWalkthrough(t, true);
  assert.equal(output.status, "typed-refusal");
  assert.equal(output.program_id, programId);
  assert.deepEqual(output.refusal, { terminal_result: -1, terminal_kind: 2, response: "0xaabb" });
  assert.deepEqual(requests.filter((request) => request.jsonrpc).map((request) => request.method), [
    "eth_getLogs", "eth_call", "eth_call", "eth_call",
  ]);
  assert.equal(requests.some((request) => request.path?.includes("account")), false);
}

async function walkthroughReportsTransactionHashAndRung(t) {
  const { output, requests, receipt } = await runWalkthrough(t, false);
  assert.equal(output.status, "called", output.refusal ?? JSON.stringify(output));
  assert.match(output.tx_hash, /^0x[0-9a-f]{64}$/u);
  assert.equal(output.tx_hash, receipt.txHash);
  assert.equal(output.rung, "receipted");
  assert.deepEqual(output.outcome, { status: "0x1", returnData: abiOutcome(0, 1, "ccdd"), vmError: "" });
  assert.equal(Object.hasOwn(output, "activity_id"), false);
  assert.equal(Object.hasOwn(output, "receipt_digest"), false);
  assert.equal(requests.filter((request) => request.method === "eth_sendRawTransaction").length, 1);
  assert.equal(requests.filter((request) => request.method === "pax_sequencerReceipt").length, 2);
  assert.deepEqual(requests.filter((request) => request.jsonrpc).map((request) => request.method), [
    "eth_getLogs", "eth_call", "eth_call",
    "eth_call", "eth_chainId", "eth_getTransactionCount", "eth_gasPrice",
    "eth_maxPriorityFeePerGas", "eth_estimateGas", "eth_sendRawTransaction", "pax_sequencerReceipt",
    "eth_getRawTransactionByHash", "pax_sequencerReceipt",
  ]);
  const pending = await runWalkthrough(t, false, "submitted");
  assert.equal(pending.output.status, "unknown-fate");
  assert.equal(pending.output.tx_hash, pending.receipt.txHash);
  assert.equal(pending.output.rung, "submitted");
  assert.equal(Object.hasOwn(pending.output, "activity_id"), false);
  assert.equal(pending.requests.filter((request) => request.method === "eth_sendRawTransaction").length, 1);
}

async function walkthroughRefusesUnpinnedReceipt(t) {
  const { output, requests, receipt } = await runWalkthrough(t, false, "receipted", true);
  assert.equal(output.status, "unknown-fate");
  assert.equal(output.rung, "submitted");
  assert.equal(output.tx_hash, receipt.txHash);
  assert.equal(Object.hasOwn(output, "outcome"), false);
  assert.equal(output.action, "Query receipt get with this transaction hash; do not resubmit the call");
  assert.equal(requests.filter((request) => request.method === "eth_sendRawTransaction").length, 1);
  assert.equal(requests.filter((request) => request.method === "eth_getRawTransactionByHash").length, 1);
  assert.equal(requests.filter((request) => request.method === "pax_sequencerReceipt").length, 2);
}

test(walkthroughSimulatesThroughProgramsPrecompile);
test(walkthroughReportsTransactionHashAndRung);
test(walkthroughRefusesUnpinnedReceipt);
