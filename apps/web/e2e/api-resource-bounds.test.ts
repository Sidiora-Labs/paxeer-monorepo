import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { chmod, link, lstat, mkdtemp, readFile, readdir, rm, symlink, utimes, writeFile } from "node:fs/promises";
import { createServer, type ServerResponse } from "node:http";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import { threadId } from "node:worker_threads";
import ts from "typescript";
import { encodeAbiParameters, encodeEventTopics, encodeFunctionResult, type Abi, type Hex } from "viem";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)(?:\?|$)/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const gateway = await import("../src/api/gateway.ts");
const views = await import("../src/api/precompiles.ts");
const codec = await import("../src/api/abi-read.ts");
const sdk = await import("../src/api/sdk.ts");
const sink = await import("../src/api/performance-sink.ts");
const bridgeAbi: Abi = JSON.parse(readFileSync(new URL("../../../precompiles/layerxbridge/abi.json", import.meta.url), "utf8"));
const ACCOUNT = "0x1111111111111111111111111111111111111111";
const OTHER = "0x2222222222222222222222222222222222222222";
const BUDGET = 9 * 1024 * 1024;

interface RpcRequest { id: string; method: string; params: readonly unknown[] }
type Reply = (request: RpcRequest, response: ServerResponse) => void | Promise<void>;

async function peer(context: TestContext, reply: Reply): Promise<RpcRequest[]> {
  const requests: RpcRequest[] = [];
  const errors: unknown[] = [];
  const server = createServer(async (request, response) => {
    try {
      let body = "";
      for await (const chunk of request) body += String(chunk);
      const document: RpcRequest = JSON.parse(body);
      requests.push(document);
      response.setHeader("content-type", "application/json");
      await reply(document, response);
    } catch (error) {
      errors.push(error);
      response.writeHead(500).end();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  const previous = process.env.LAYERX_GATEWAY_URL;
  process.env.LAYERX_GATEWAY_URL = `http://127.0.0.1:${address.port}`;
  context.after(async () => {
    if (previous === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previous;
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
    assert.deepEqual(errors, [], "loopback peer must accept the real request contract");
  });
  return requests;
}

function answer(request: RpcRequest, response: ServerResponse, result: unknown): void {
  response.end(JSON.stringify({ jsonrpc: "2.0", id: request.id, result }));
}

function replaceWord(data: Hex, byteOffset: number, value: bigint): Hex {
  const start = 2 + byteOffset * 2;
  return `${data.slice(0, start)}${value.toString(16).padStart(64, "0")}${data.slice(start + 64)}` as Hex;
}

async function abi_array_allocation_real_view(context: TestContext): Promise<void> {
  let data = encodeFunctionResult({ abi: bridgeAbi, functionName: "getAttestors", result: [[ACCOUNT], [7n], 1] });
  const requests = await peer(context, (request, response) => {
    assert.equal(request.method, "eth_call");
    assert.deepEqual(request.params, [{ to: sdk.LAYERX_BRIDGE_PRECOMPILE, data: sdk.encodeAbiCall("getAttestors", [], []) }, "latest"]);
    answer(request, response, data);
  });
  const valid = data;
  assert.deepEqual(await views.bridgeAttestors(), { signers: [ACCOUNT], threshold: 1n });
  data = replaceWord(valid, 96, 160n);
  await assert.rejects(views.bridgeAttestors(), /data_length: ABI head extent/u);
  for (const badOffset of [0n, 97n, 128n]) {
    data = replaceWord(valid, 0, badOffset);
    await assert.rejects(views.bridgeAttestors(), /non_canonical_word: ABI offset/u);
  }
  data = encodeFunctionResult({
    abi: bridgeAbi, functionName: "getAttestors",
    result: [Array.from({ length: codec.ABI_DECODE_WORK_LIMIT }, () => ACCOUNT), [], 1],
  });
  await assert.rejects(views.bridgeAttestors(), /data_length: ABI decoding work bound/u);
  assert.equal(requests.length, 6);
}

function abi_array_canonical_nested_extent(): void {
  const parameters = [{ name: "rows", type: "string[][]" }] as const;
  const encoded = encodeAbiParameters(parameters, [[["one", "é"], [], ["three"]]]);
  const fields = [{ name: "rows", type: { array: { array: "string" as const } } }];
  assert.deepEqual(codec.decodeAbiResult(encoded, fields), { rows: [["one", "é"], [], ["three"]] });
  assert.throws(() => codec.decodeAbiResult(replaceWord(encoded, 64, 0n), fields), /non_canonical_word: ABI offset/u);
  const huge = replaceWord(encodeAbiParameters([{ type: "uint256[]" }], [[1n]]), 32, (1n << 256n) - 1n);
  assert.throws(() => codec.decodeAbiResult(huge, [{ name: "items", type: { array: "uint256" } }]), sdk.PrecompileAbiError);
  assert.throws(() => codec.decodeAbiResult(`0x${"00".repeat(32)}`, [
    { name: "items", type: { fixed: "uint256", length: Number.MAX_SAFE_INTEGER } },
  ]), /ABI head extent/u);
  const padded = encodeAbiParameters([{ type: "bytes" }], ["0x01"]);
  assert.throws(() => codec.decodeAbiResult(`${padded.slice(0, -2)}01`, [{ name: "value", type: "bytes" }]), /ABI padding/u);
}

async function rpc_response_memory_bound_stream_cancellation(context: TestContext): Promise<void> {
  let closed = false;
  let ended = false;
  let sent = 0;
  await peer(context, (request, response) => {
    response.on("close", () => { closed = true; });
    response.write(`{"jsonrpc":"2.0","id":"${request.id}","padding":"`);
    const chunk = Buffer.from("€".repeat(16_384));
    function pump(): void {
      if (response.destroyed) return;
      // Deliberately leave the body open beyond the byte budget: the reader must cancel it.
      while (sent <= BUDGET + chunk.length) {
        sent += chunk.length;
        if (!response.write(chunk)) { response.once("drain", pump); return; }
      }
      ended = response.writableEnded;
    }
    pump();
  });
  const start = Date.now();
  await assert.rejects(gateway.ethBlockNumber(), (error: unknown) => {
    assert.ok(error instanceof gateway.GatewayUnavailableError);
    assert.match(error.message, /answer too large/u);
    return true;
  });
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(ended, false);
  assert.ok(Date.now() - start < 8_000, "size rejection precedes the network deadline");
  // Closing the connection is observable at the producer, before it sends an EOF.
  for (let attempt = 0; !closed && attempt < 50; attempt += 1) {
    await new Promise<void>((resolve) => setTimeout(resolve, 10));
  }
  assert.equal(closed, true);
}

async function rpc_response_byte_boundary_utf8_and_envelope(context: TestContext): Promise<void> {
  let mode = "boundary";
  await peer(context, (request, response) => {
    const prefix = `{"jsonrpc":"2.0","id":"${request.id}","result":"0x1","padding":"`;
    const suffix = '"}';
    if (mode === "boundary") {
      const overhead = Buffer.byteLength(prefix + suffix);
      response.write(prefix);
      response.write("x".repeat(BUDGET - overhead));
      response.end(suffix);
    } else if (mode === "utf8") {
      response.write(prefix);
      response.write(Buffer.from([0xc3, 0x28]));
      response.end(suffix);
    } else if (mode === "split") {
      const bytes = Buffer.from(`${prefix}é${suffix}`);
      const index = bytes.indexOf(0xc3);
      response.write(bytes.subarray(0, index + 1));
      response.end(bytes.subarray(index + 1));
    } else if (mode === "id") {
      response.end(JSON.stringify({ jsonrpc: "2.0", id: "wrong", result: "0x1" }));
    } else {
      response.end(JSON.stringify({ jsonrpc: "2.0", id: request.id, error: { code: -32003, message: "unavailable" } }));
    }
  });
  assert.equal(await gateway.ethBlockNumber(), 1n);
  mode = "split";
  assert.equal(await gateway.ethBlockNumber(), 1n);
  mode = "utf8";
  await assert.rejects(gateway.ethBlockNumber(), /answered malformed UTF-8/u);
  mode = "id";
  await assert.rejects(gateway.ethBlockNumber(), /malformed envelope/u);
  mode = "error";
  await assert.rejects(gateway.ethBlockNumber(), gateway.GatewayRpcError);
}

async function rpc_response_body_retains_deadline(context: TestContext): Promise<void> {
  await peer(context, (request, response) => {
    assert.equal(request.method, "eth_blockNumber");
    response.write('{"jsonrpc":"2.0",');
  });
  const start = Date.now();
  await assert.rejects(gateway.ethBlockNumber(), (error: unknown) => {
    assert.ok(error instanceof gateway.GatewayUnavailableError);
    assert.match(error.message, /body did not answer/u);
    return true;
  });
  assert.ok(Date.now() - start >= 7_500);
  assert.ok(Date.now() - start < 12_000);
}

async function privateSink(context: TestContext): Promise<string> {
  const directory = await mkdtemp(path.join(tmpdir(), "repair-rum-"));
  await chmod(directory, 0o700);
  const previous = process.env.LAYERX_RUM_STORAGE_DIRECTORY;
  process.env.LAYERX_RUM_STORAGE_DIRECTORY = directory;
  context.after(async () => {
    if (previous === undefined) delete process.env.LAYERX_RUM_STORAGE_DIRECTORY;
    else process.env.LAYERX_RUM_STORAGE_DIRECTORY = previous;
    await rm(directory, { recursive: true, force: true });
  });
  return directory;
}

async function rum_orphaned_temporary_stop_reopen(context: TestContext): Promise<void> {
  const directory = await privateSink(context);
  const observation = { version: 1, route: "bridge", metric: "LCP", observed: 100 } as const;
  assert.equal((await sink.persistPerformanceObservation(observation)).durable, true);
  const prepared = await sink.DurablePerformanceWrite.begin(directory, { ...observation, observed: 200 });
  await prepared.write();
  const incomplete = await sink.DurablePerformanceWrite.begin(directory, { ...observation, observed: 300 });
  const activeSnapshot = await sink.performanceSinkSnapshot();
  assert.equal(activeSnapshot.aggregates[0]?.samples, 1, "active prepared writes must not be published by recovery");
  assert.equal((await readdir(directory)).filter((name) => name.endsWith(".tmp")).length, 2);
  // Stop both real writers in the crash window: one before data, one after fsync but before rename.
  await prepared.close();
  await incomplete.close();
  const reopened: typeof sink = await import(`../src/api/performance-sink.ts?reopen=${Date.now()}`);
  const recovered = await reopened.performanceSinkSnapshot();
  assert.equal(recovered.health.delivery, "healthy");
  assert.equal(recovered.aggregates[0]?.samples, 2);
  assert.equal(recovered.aggregates[0]?.percentile75, 200);
  assert.ok((await readdir(directory)).every((name) => name.endsWith(".json")));
  assert.deepEqual((await reopened.performanceSinkSnapshot()).aggregates, recovered.aggregates, "recovery is idempotent");
  const afterRestart = await reopened.DurablePerformanceWrite.begin(directory, { ...observation, observed: 400 });
  await afterRestart.write();
  await afterRestart.commit();
  assert.equal((await sink.performanceSinkSnapshot()).aggregates[0]?.samples, 3);
  const interruptedRecovery = await sink.DurablePerformanceWrite.begin(directory, { ...observation, observed: 500 });
  await interruptedRecovery.write();
  await interruptedRecovery.close();
  const temporary = (await readdir(directory)).find((name) => name.endsWith(".tmp"));
  assert.ok(temporary);
  const destination = temporary.slice(1).replace(/\.[0-9]+\.[0-9]+\.tmp$/u, "");
  // Reproduce a filesystem stop after recovery's exclusive link, before its unlink.
  await link(path.join(directory, temporary), path.join(directory, destination));
  const recoveredAgain = await reopened.performanceSinkSnapshot();
  assert.equal(recoveredAgain.health.delivery, "healthy");
  assert.equal(recoveredAgain.aggregates[0]?.samples, 4);
  assert.ok((await readdir(directory)).every((name) => name.endsWith(".json")));
}

async function rum_recovery_ownership_and_legacy_temporaries(context: TestContext): Promise<void> {
  const directory = await privateSink(context);
  const observation = { version: 1, route: "root", metric: "INP", observed: 100 } as const;
  assert.equal((await sink.persistPerformanceObservation(observation)).accepted, true);
  const legacy = ".v1-root-inp-1000000000000-11111111-1111-1111-1111-111111111111.json.tmp";
  await writeFile(path.join(directory, legacy), "{", { mode: 0o600 });
  await utimes(path.join(directory, legacy), new Date(1), new Date(1));
  assert.equal((await sink.performanceSinkSnapshot()).aggregates[0]?.samples, 1, "old unleased temporaries cannot suppress committed records");
  assert.equal(await readFile(path.join(directory, legacy), "utf8"), "{", "unproven ownership must not authorize deletion");
  const unsafe = `.v1-root-inp-1000000000000-22222222-2222-2222-2222-222222222222.json.${process.pid}.${threadId}.tmp`;
  const target = `${directory}-outside`;
  context.after(() => rm(target, { force: true }));
  await writeFile(target, "untouched", { mode: 0o600 });
  await symlink(target, path.join(directory, unsafe));
  assert.equal((await sink.performanceSinkSnapshot()).health.delivery, "degraded");
  assert.equal(await readFile(target, "utf8"), "untouched");
  assert.equal((await lstat(path.join(directory, unsafe))).isSymbolicLink(), true);
}

interface WireLog { address: string; topics: readonly Hex[]; data: Hex; blockNumber: Hex; transactionHash: Hex; logIndex: Hex }
function bridgeOutLog(block: bigint, index: bigint, recipient = ACCOUNT): WireLog {
  return {
    address: sdk.LAYERX_BRIDGE_PRECOMPILE,
    topics: encodeEventTopics({ abi: bridgeAbi, eventName: "BridgeOut", args: { chain: 1n, asset: OTHER, nonce: index } }).map((topic) => {
      assert.ok(typeof topic === "string", "the producer emits concrete event topics");
      return topic;
    }),
    data: encodeAbiParameters([{ type: "uint256" }, { type: "address" }], [10n, recipient as Hex]),
    blockNumber: `0x${block.toString(16)}`, transactionHash: `0x${block.toString(16).padStart(64, "0")}`,
    logIndex: `0x${index.toString(16)}`,
  };
}

async function bridge_wide_log_scan_bounded_cursor_real_rpc(context: TestContext): Promise<void> {
  let head = 250n;
  let failed = false;
  const busy = Array.from({ length: 251 }, (_, block) => [
    bridgeOutLog(BigInt(block), 0n), bridgeOutLog(BigInt(block), 1n), bridgeOutLog(BigInt(block), 2n, OTHER),
  ]).flat();
  const ranges: { from: bigint; to: bigint; topics: readonly (string | null)[] }[] = [];
  await peer(context, (request, response) => {
    if (request.method === "eth_blockNumber") { answer(request, response, `0x${head.toString(16)}`); return; }
    assert.equal(request.method, "eth_getLogs");
    const filter = request.params[0] as { address: string; fromBlock: string; toBlock: string; topics: readonly (string | null)[] };
    assert.equal(filter.address, sdk.LAYERX_BRIDGE_PRECOMPILE);
    assert.notEqual(filter.toBlock, "latest");
    const from = BigInt(filter.fromBlock);
    const to = BigInt(filter.toBlock);
    assert.ok(to >= from && to - from + 1n <= 100n, "busy node accepts at most one hundred explicit blocks per query");
    ranges.push({ from, to, topics: filter.topics });
    if (filter.topics[0] === sdk.abiEventTopic(sdk.precompileEventSignature(sdk.BRIDGE_EVENTS[0]!))) {
      assert.equal(filter.topics[3], `0x${"00".repeat(12)}${ACCOUNT.slice(2)}`);
      answer(request, response, []);
    } else if (failed) {
      response.end(JSON.stringify({ jsonrpc: "2.0", id: request.id, error: { code: -32000, message: "busy range" } }));
    } else {
      const logs = busy.filter((log) => BigInt(log.blockNumber) >= from && BigInt(log.blockNumber) <= to).reverse();
      answer(request, response, [...logs, ...logs.slice(0, 10)]);
    }
  });
  const first = await views.bridgeActivityPage(ACCOUNT);
  assert.equal(first.status, "partial");
  assert.equal(first.items.length, 200);
  assert.ok(first.nextCursor);
  assert.equal(first.items[0]?.blockNumber, 250n);
  assert.equal(first.items[0]?.logIndex, 1n);
  assert.equal(first.items.at(-1)?.blockNumber, 151n);
  head = 999n;
  const second = await views.bridgeActivityPage(ACCOUNT, first.nextCursor);
  assert.equal(second.head, 250n, "continuation pins the original chain head");
  assert.equal(second.items[0]?.blockNumber, 150n);
  assert.ok(second.nextCursor);
  const last = await views.bridgeActivityPage(ACCOUNT, second.nextCursor);
  assert.equal(last.status, "complete");
  assert.equal(last.nextCursor, null);
  const all = [...first.items, ...second.items, ...last.items];
  assert.equal(all.length, 502);
  assert.equal(new Set(all.map((entry) => `${entry.transactionHash}:${entry.logIndex}`)).size, all.length);
  assert.ok(all.every((entry, index) => index === 0 || all[index - 1]!.blockNumber >= entry.blockNumber));
  assert.deepEqual(ranges.slice(0, 6).map(({ from, to }) => [from, to]), [[151n, 250n], [151n, 250n], [51n, 150n], [51n, 150n], [0n, 50n], [0n, 50n]]);
  await assert.rejects(views.bridgeActivityPage(OTHER, first.nextCursor), /invalid bridge history cursor/u);
  failed = true;
  const unavailable = await views.bridgeActivityPage(ACCOUNT, first.nextCursor);
  assert.equal(unavailable.status, "unavailable");
  assert.equal(unavailable.nextCursor, first.nextCursor, "failed coverage must remain resumable at the same block");
  assert.match(unavailable.detail!, /busy range/u);
  failed = false;
  await assert.rejects(views.bridgeActivity(ACCOUNT), (error: unknown) => {
    assert.ok(error instanceof views.BridgeActivityIncompleteError);
    assert.equal(error.page.status, "partial");
    assert.ok(error.page.nextCursor);
    return true;
  });
}

test(abi_array_allocation_real_view);
test(abi_array_canonical_nested_extent);
test(rpc_response_memory_bound_stream_cancellation);
test(rpc_response_byte_boundary_utf8_and_envelope);
test(rpc_response_body_retains_deadline);
test(rum_orphaned_temporary_stop_reopen);
test(rum_recovery_ownership_and_legacy_temporaries);
test(bridge_wide_log_scan_bounded_cursor_real_rpc);
