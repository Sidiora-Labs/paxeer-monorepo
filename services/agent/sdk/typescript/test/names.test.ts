import assert from "node:assert/strict";
import { once } from "node:events";
import * as http from "node:http";
import { test } from "node:test";
import { keccak_256 } from "@noble/hashes/sha3.js";
import {
  JsonRpcClient,
  JsonRpcError,
  labelhash,
  namehash,
  resolveName as resolvePaxName,
} from "../src/index.js";

const REGISTRY = `0x${"11".repeat(20)}`;
const RESOLVER = `0x${"22".repeat(20)}`;
const OWNER = `0x${"ab".repeat(20)}`;
const ADDRESS = `0x${"cd".repeat(20)}`;
const ZERO = `0x${"00".repeat(20)}`;
const DIGEST = `0x${"ef".repeat(32)}`;
const DID = `did:layerx:${"12".repeat(32)}`;
const contracts = { registry: REGISTRY, resolver: RESOLVER };

function word(value: bigint): string {
  return value.toString(16).padStart(64, "0");
}

function abiBytes(bytes: Uint8Array): string {
  return `0x${word(32n)}${word(BigInt(bytes.length))}${Buffer.from(bytes)
    .toString("hex")
    .padEnd(Math.ceil(bytes.length / 32) * 64, "0")}`;
}

function addressWord(address: string): string {
  return `0x${address.slice(2).padStart(64, "0")}`;
}

export async function resolveName(): Promise<void> {
  assert.equal(namehash(""), `0x${"00".repeat(32)}`);
  assert.equal(
    namehash("eth"),
    "0x93cdeb708b7545dc668eb9280176169d1c33cfd8ed6f04690a0bcc88a93fc4ae",
  );
  assert.equal(
    namehash("foo.eth"),
    "0xde9b09fd7c5f901e23a3f19fecc54828e9c848539801e86591bd9801b019f84f",
  );
  assert.equal(
    labelhash("eth"),
    "0x4f5b812789fc606be1b3b16908db13fc7a9adf7ca72641f84d75b47069d3d7f0",
  );
  const hash = (bytes: Uint8Array): Buffer => Buffer.from(keccak_256(bytes));
  const zone = hash(
    Buffer.concat([Buffer.alloc(32), hash(Buffer.from("pax"))]),
  );
  const node = `0x${hash(Buffer.concat([zone, hash(Buffer.from("example"))])).toString("hex")}`;
  assert.equal(namehash("example.pax"), node);
  assert.equal(namehash("EXAMPLE.PAX"), node);
  assert.equal(namehash(`${"a".repeat(63)}.pax`).length, 66);
  assert.equal(namehash("abc-def.pax").length, 66);
  for (const invalid of [
    "ab.pax",
    "-abc.pax",
    "abc-.pax",
    "ex_ample.pax",
    "abc..pax",
    ".pax",
    "abc.pax.",
    "abc pax",
    "école.pax",
    "e\u0301cole.pax",
    `${"a".repeat(64)}.pax`,
  ]) {
    assert.throws(() => namehash(invalid), TypeError);
  }

  const requests: { to: string; data: string }[] = [];
  let currentNode = node;
  let owner = OWNER;
  let addressResult = addressWord(ADDRESS);
  let textResult = abiBytes(Buffer.from(DID));
  let contentResult = abiBytes(
    Buffer.from(`70617831${DIGEST.slice(2)}`, "hex"),
  );
  let rpcError = false;
  const gateway = http.createServer((request, response) => {
    if (request.url !== "/") {
      request.resume();
      response.writeHead(404);
      response.end();
      return;
    }
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(Buffer.from(chunk)));
    request.on("end", () => {
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as {
        jsonrpc: string;
        id: string;
        method: string;
        params: [{ to: string; data: string }, string];
      };
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      assert.equal(request.headers["content-type"], "application/json");
      assert.equal(body.jsonrpc, "2.0");
      assert.equal(body.method, "eth_call");
      assert.equal(body.params.length, 2);
      assert.equal(body.params[1], "latest");
      const call = body.params[0];
      requests.push(call);
      const selector = call.data.slice(0, 10);
      const expectedData =
        selector === "0x59d1d43c"
          ? `${selector}${currentNode.slice(2)}${word(64n)}${word(10n)}${Buffer.from("layerx.did").toString("hex").padEnd(64, "0")}`
          : `${selector}${currentNode.slice(2)}`;
      assert.equal(call.data, expectedData);
      assert.equal(call.to, selector === "0x02571be3" ? REGISTRY : RESOLVER);
      const answers: Record<string, string> = {
        "0x02571be3": addressWord(owner),
        "0x3b3b57de": addressResult,
        "0x59d1d43c": textResult,
        "0xbc1c58d1": contentResult,
      };
      assert(
        Object.hasOwn(answers, selector),
        `Unexpected selector ${selector}`,
      );
      const answer = rpcError
        ? { error: { code: -32000, message: "Name read unavailable" } }
        : { result: answers[selector] };
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, ...answer }));
    });
  });
  gateway.listen(0, "127.0.0.1");
  await once(gateway, "listening");
  const listener = gateway.address();
  assert(listener !== null && typeof listener === "object");
  try {
    const endpoint = `http://127.0.0.1:${listener.port}`;
    for (const suffix of ["/rpc", "/rpc/ws"]) {
      const response = await fetch(endpoint + suffix, { method: "POST", body: "{}" });
      assert.equal(response.status, 404);
    }
    assert.equal(requests.length, 0);
    const client = new JsonRpcClient(endpoint);
    assert.deepEqual(await resolvePaxName(client, contracts, "ExAmPlE.PAX"), {
      node,
      owner: OWNER,
      address: ADDRESS,
      did: DID,
      contenthash: { digest: DIGEST },
    });
    assert.deepEqual(
      requests.map((call) => call.data.slice(0, 10)),
      ["0x02571be3", "0x3b3b57de", "0x59d1d43c", "0xbc1c58d1"],
    );

    owner = ZERO;
    currentNode = namehash("unowned.pax");
    const beforeUnowned = requests.length;
    assert.deepEqual(await resolvePaxName(client, contracts, "unowned.pax"), {
      node: currentNode,
      owner: ZERO,
      address: null,
      did: null,
      contenthash: null,
    });
    assert.equal(requests.length, beforeUnowned + 1);
    assert.equal(requests.at(-1)!.to, REGISTRY);

    owner = OWNER;
    currentNode = node;
    addressResult = addressWord(ZERO);
    textResult = abiBytes(Buffer.alloc(0));
    for (const bytes of [
      "",
      `70617832${DIGEST.slice(2)}`,
      `70617831${DIGEST.slice(2, -2)}`,
      `70617831${DIGEST.slice(2)}00`,
    ]) {
      contentResult = abiBytes(Buffer.from(bytes, "hex"));
      assert.deepEqual(await resolvePaxName(client, contracts, "example.pax"), {
        node,
        owner: OWNER,
        address: null,
        did: null,
        contenthash: null,
      });
    }

    for (const malformed of [
      "0x",
      addressWord(ADDRESS).slice(0, -2),
      `0x1${"0".repeat(63)}`,
    ]) {
      addressResult = malformed;
      await assert.rejects(resolvePaxName(client, contracts, "example.pax"), {
        message: "Invalid name address response",
      });
    }
    addressResult = addressWord(ADDRESS);
    for (const malformed of [
      "0x",
      `0x${word(64n)}${word(0n)}`,
      `0x${word(32n)}${word(33n)}${"00".repeat(32)}`,
      `0x${word(32n)}${word(1n)}ff${"00".repeat(30)}01`,
    ]) {
      textResult = malformed;
      await assert.rejects(
        resolvePaxName(client, contracts, "example.pax"),
        TypeError,
      );
    }
    textResult = abiBytes(new Uint8Array([0xff]));
    await assert.rejects(
      resolvePaxName(client, contracts, "example.pax"),
      TypeError,
    );
    textResult = abiBytes(Buffer.from(DID));
    contentResult = `0x${word(32n)}${word(36n)}70617831`;
    await assert.rejects(
      resolvePaxName(client, contracts, "example.pax"),
      TypeError,
    );

    rpcError = true;
    await assert.rejects(
      resolvePaxName(client, contracts, "example.pax"),
      (error: unknown) =>
        error instanceof JsonRpcError &&
        error.code === -32000 &&
        error.message === "Name read unavailable",
    );
    const beforeInvalid = requests.length;
    await assert.rejects(
      resolvePaxName(client, contracts, "abc..pax"),
      TypeError,
    );
    await assert.rejects(
      resolvePaxName(
        client,
        { ...contracts, registry: "invalid" },
        "example.pax",
      ),
      TypeError,
    );
    await assert.rejects(
      resolvePaxName(
        client,
        { ...contracts, resolver: "invalid" },
        "example.pax",
      ),
      TypeError,
    );
    assert.equal(requests.length, beforeInvalid);
  } finally {
    gateway.close();
    await once(gateway, "close");
  }
}

await test(resolveName);
