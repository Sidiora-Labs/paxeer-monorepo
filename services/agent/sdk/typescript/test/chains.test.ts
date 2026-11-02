import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
  CHAINS_EVENTS,
  CHAINS_METHODS,
  ChainsClient,
  JsonRpcClient,
  LAYERX_CHAINS_PRECOMPILE,
  abiEventTopic,
  abiSelector,
  chainRpcUrl,
  createChainCall,
  decodeChainsEvent,
  decodeChainsParams,
  decodeGetChain,
  encodeChainsCall,
  precompileWriteCall,
  virtualChainId,
  type AbiArgument,
  type ChainsArguments,
  type PrecompileLog,
} from "../src/index.js";

interface MethodVector {
  readonly name: keyof ChainsArguments;
  readonly signature: string;
  readonly selector: string;
  readonly args: readonly (string | number | boolean)[];
  readonly calldata: string;
}

interface ReturnVector {
  readonly returndata: string;
  readonly decoded: Readonly<Record<string, string | number>>;
}

interface AbiField {
  readonly name: string;
  readonly type: string;
  readonly indexed?: boolean;
}

interface AbiEntry {
  readonly type: string;
  readonly name: string;
  readonly inputs: readonly AbiField[];
  readonly outputs?: readonly AbiField[];
}

function abiTuple(
  types: readonly string[],
  values: readonly AbiArgument[],
): string {
  return `0x${precompileWriteCall(
    LAYERX_CHAINS_PRECOMPILE,
    `tuple(${types.join(",")})`,
    { types, payable: false },
    values,
  ).data.slice(10)}`;
}

function expectedTuple(
  fields: readonly AbiField[],
  vector: ReturnVector,
): Record<string, string | bigint> {
  return Object.fromEntries(
    fields.map((field) => [
      field.name,
      field.type.startsWith("uint")
        ? BigInt(vector.decoded[field.name]!)
        : vector.decoded[field.name],
    ]),
  ) as Record<string, string | bigint>;
}

function replaceWord(bytes: string, index: number, value: bigint): string {
  const start = 2 + index * 64;
  return (
    bytes.slice(0, start) +
    abiTuple(["uint256"], [value]).slice(2) +
    bytes.slice(start + 64)
  );
}

function chains_vectors_match_and_round_trip(): void {
  const vectors = JSON.parse(
    readFileSync(
      new URL(
        "../../../../../../tests/vectors/chains/abi-vectors.json",
        import.meta.url,
      ),
      "utf8",
    ),
  ) as {
    readonly address: string;
    readonly methods: readonly MethodVector[];
    readonly events: readonly {
      readonly name: string;
      readonly signature: string;
      readonly topic0: string;
    }[];
    readonly getChain: ReturnVector;
    readonly params: ReturnVector;
  };
  const abi = JSON.parse(
    readFileSync(
      new URL(
        "../../../../../../precompiles/layerxchains/abi.json",
        import.meta.url,
      ),
      "utf8",
    ),
  ) as readonly AbiEntry[];
  assert.equal(LAYERX_CHAINS_PRECOMPILE, vectors.address);
  assert.deepEqual(
    Object.keys(CHAINS_METHODS).sort(),
    vectors.methods.map((entry) => entry.name).sort(),
  );
  assert.equal(vectors.methods.length, 12);
  for (const vector of vectors.methods) {
    const types = CHAINS_METHODS[vector.name].types;
    assert.equal(`${vector.name}(${types.join(",")})`, vector.signature);
    const args = vector.args.map((value) =>
      typeof value === "number" ? BigInt(value) : value,
    ) as unknown as ChainsArguments[typeof vector.name];
    const encoded = encodeChainsCall(vector.name, args);
    assert.equal(abiSelector(vector.signature), vector.selector, vector.name);
    assert.equal(encoded.slice(0, 10), vector.selector, vector.name);
    assert.equal(encoded, vector.calldata, vector.name);
  }

  for (const [name, vector, decode] of [
    ["getChain", vectors.getChain, decodeGetChain],
    ["params", vectors.params, decodeChainsParams],
  ] as const) {
    const fields = abi.find(
      (entry) => entry.type === "function" && entry.name === name,
    )!.outputs!;
    const expected = expectedTuple(fields, vector);
    const decoded = decode(vector.returndata);
    assert.deepEqual(decoded, expected, name);
    const encoded = abiTuple(
      fields.map((field) => field.type),
      fields.map((field) => expected[field.name]!),
    );
    assert.equal(encoded, vector.returndata, `${name} encoded bytes`);
    assert.deepEqual(decode(encoded), decoded, `${name} round trip`);
    assert.throws(() => decode(vector.returndata.slice(0, -2)), TypeError);
    assert.throws(
      () => decode(vector.returndata + vector.returndata.slice(2, 66)),
      TypeError,
    );
    assert.throws(() => decode(null), TypeError);
  }
  const chain = decodeGetChain(vectors.getChain.returndata);
  assert.equal(virtualChainId(chain.id), chain.chainId);
  assert.equal(virtualChainId(Number(chain.id)), chain.chainId);
  assert.equal(virtualChainId(0xffffffff), 125000000n + 0xffffffffn);
  assert.equal(
    chainRpcUrl("https://rpc.example/base", chain.id),
    `https://rpc.example/base/chain/${chain.id}`,
  );
  assert.equal(
    chainRpcUrl("https://rpc.example/base///", Number(chain.id)),
    `https://rpc.example/base/chain/${chain.id}`,
  );
  for (const invalid of [
    -1,
    1.5,
    Number.NaN,
    Number.MAX_SAFE_INTEGER + 1,
    1n << 32n,
  ]) {
    assert.throws(() => virtualChainId(invalid));
    assert.throws(() => chainRpcUrl("https://rpc.example", invalid));
  }
  for (const invalid of [
    "not a url",
    "ftp://rpc.example",
    "https://rpc.example?key=value",
    "https://rpc.example#hash",
    "https://user:pass@rpc.example",
  ]) {
    assert.throws(() => chainRpcUrl(invalid, chain.id));
  }

  assert.deepEqual(
    CHAINS_EVENTS.map((entry) => entry.name).sort(),
    vectors.events.map((entry) => entry.name).sort(),
  );
  assert.equal(vectors.events.length, 4);
  for (const vector of vectors.events) {
    const spec = CHAINS_EVENTS.find((entry) => entry.name === vector.name)!;
    const eventAbi = abi.find(
      (entry) => entry.type === "event" && entry.name === vector.name,
    )!;
    assert.deepEqual(spec.inputs, eventAbi.inputs);
    assert.equal(
      `${spec.name}(${spec.inputs.map((input) => input.type).join(",")})`,
      vector.signature,
    );
    assert.equal(abiEventTopic(vector.signature), vector.topic0, vector.name);
    const values: Readonly<Record<string, string | bigint>> = {
      id: chain.id,
      name: chain.name,
      operator: chain.operator,
      chainId: chain.chainId,
      status: chain.status,
      from: chain.operator,
      amount: chain.bond,
    };
    const fields = Object.fromEntries(
      spec.inputs.map((input) => [input.name, values[input.name]]),
    );
    const indexed = spec.inputs.filter((input) => input.indexed);
    const data = spec.inputs.filter((input) => !input.indexed);
    const log: PrecompileLog = {
      address: vectors.address,
      topics: [
        vector.topic0,
        ...indexed.map((input) =>
          abiTuple([input.type], [values[input.name]!]),
        ),
      ],
      data: abiTuple(
        data.map((input) => input.type),
        data.map((input) => values[input.name]!),
      ),
    };
    assert.deepEqual(decodeChainsEvent(log), {
      event: vector.name,
      precompile: vectors.address,
      topic0: vector.topic0,
      fields,
    });
    assert.throws(
      () => decodeChainsEvent({ ...log, topics: log.topics.slice(1) }),
      TypeError,
    );
    assert.throws(
      () => decodeChainsEvent({ ...log, address: chain.operator }),
      TypeError,
    );
    assert.throws(
      () =>
        decodeChainsEvent({ ...log, topics: [...log.topics, log.topics[1]!] }),
      TypeError,
    );
    assert.throws(
      () =>
        decodeChainsEvent({
          ...log,
          topics: [
            log.topics[0]!,
            abiTuple(["uint256"], [1n << 32n]),
            ...log.topics.slice(2),
          ],
        }),
      TypeError,
    );
  }

  const createVector = vectors.methods.find(
    (entry) => entry.name === "createChain",
  )!;
  const options = {
    name: chain.name,
    gasAllowance: chain.gasAllowance,
    byteAllowance: chain.byteAllowance,
    feeShareBps: chain.feeShareBps,
    value: chain.bond,
  };
  assert.deepEqual(createChainCall(options), {
    to: vectors.address,
    data: createVector.calldata,
    value: chain.bond,
  });
  const client = new ChainsClient(new JsonRpcClient("https://rpc.example"));
  assert.deepEqual(client.createChain(chain.operator, options), {
    method: "eth_sendTransaction",
    params: [
      {
        from: chain.operator,
        to: vectors.address,
        data: createVector.calldata,
        value: `0x${chain.bond.toString(16)}`,
        type: "0x2",
        chainId: "0x7d",
      },
    ],
  });
  assert.throws(() => createChainCall({ ...options, value: -1n }));
  assert.throws(() => client.createChain("invalid", options));
  assert.throws(() => encodeChainsCall("getChain", [-1n]));
  assert.throws(() => encodeChainsCall("getChain", [1n << 32n]));
  assert.throws(() =>
    encodeChainsCall("createChain", [
      chain.name,
      1n << 64n,
      chain.byteAllowance,
      chain.feeShareBps,
    ]),
  );
  assert.throws(() =>
    encodeChainsCall("createChain", [
      chain.name,
      chain.gasAllowance,
      1n << 64n,
      chain.feeShareBps,
    ]),
  );
  assert.throws(() =>
    encodeChainsCall("createChain", [
      chain.name,
      chain.gasAllowance,
      chain.byteAllowance,
      1n << 16n,
    ]),
  );
  assert.throws(() => encodeChainsCall("setOperator", [chain.id, "invalid"]));
  assert.doesNotThrow(() => encodeChainsCall("getChain", [(1n << 32n) - 1n]));
  assert.doesNotThrow(() =>
    encodeChainsCall("createChain", [
      chain.name,
      (1n << 64n) - 1n,
      (1n << 64n) - 1n,
      (1n << 16n) - 1n,
    ]),
  );
  for (const [index, bits] of [
    [0, 32],
    [3, 8],
    [4, 64],
    [5, 64],
    [6, 64],
    [7, 16],
    [10, 64],
  ] as const) {
    assert.throws(
      () =>
        decodeGetChain(
          replaceWord(vectors.getChain.returndata, index, 1n << BigInt(bits)),
        ),
      TypeError,
    );
  }
  for (const [index, bits] of [
    [0, 160],
    [3, 64],
    [4, 64],
    [5, 64],
    [6, 32],
  ] as const) {
    assert.throws(
      () =>
        decodeChainsParams(
          replaceWord(vectors.params.returndata, index, 1n << BigInt(bits)),
        ),
      TypeError,
    );
  }
  assert.throws(
    () =>
      decodeGetChain(replaceWord(vectors.getChain.returndata, 2, 1n << 160n)),
    TypeError,
  );
  assert.throws(
    () => decodeGetChain(replaceWord(vectors.getChain.returndata, 1, 32n)),
    TypeError,
  );
  assert.throws(
    () =>
      decodeGetChain(replaceWord(vectors.getChain.returndata, 11, 1n << 255n)),
    TypeError,
  );
  const paddingError = vectors.getChain.returndata.slice(0, -1) + "1";
  assert.throws(() => decodeGetChain(paddingError), TypeError);
  const stringStart = 2 + 12 * 64;
  const invalidUtf8 =
    vectors.getChain.returndata.slice(0, stringStart) +
    "ff" +
    vectors.getChain.returndata.slice(stringStart + 2);
  assert.throws(() => decodeGetChain(invalidUtf8), TypeError);
  const chainFields = abi.find((entry) => entry.name === "getChain")!.outputs!;
  for (const name of ["", "école-链", "x".repeat(33)]) {
    const values = { ...chain, name };
    const encoded = abiTuple(
      chainFields.map((field) => field.type),
      chainFields.map((field) => values[field.name as keyof typeof values]),
    );
    assert.deepEqual(decodeGetChain(encoded), values);
  }
}

await test(chains_vectors_match_and_round_trip);
