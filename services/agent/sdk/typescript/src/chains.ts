import type { WalletRequest } from "./account-derivation.js";
import {
  abiEventTopic,
  precompileTransactionRequest,
  precompileWriteCall,
  type PrecompileCall,
  type PrecompileLog,
} from "./exchange.js";
import type { JsonRpcClient } from "./rpc.js";

export const LAYERX_CHAINS_PRECOMPILE =
  "0x000000000000000000000000000000000000101e";

export const CHAINS_METHODS = {
  createChain: {
    types: ["string", "uint64", "uint64", "uint16"],
    payable: true,
  },
  fundChain: { types: ["uint32"], payable: true },
  pauseChain: { types: ["uint32"], payable: false },
  resumeChain: { types: ["uint32"], payable: false },
  retireChain: { types: ["uint32"], payable: false },
  governancePause: { types: ["uint32", "bool"], payable: false },
  setOperator: { types: ["uint32", "address"], payable: false },
  getChain: { types: ["uint32"], payable: false },
  chainByName: { types: ["string"], payable: false },
  chainCount: { types: [], payable: false },
  chainIdOf: { types: ["uint32"], payable: false },
  params: { types: [], payable: false },
} as const;

export interface ChainsArguments {
  readonly createChain: readonly [string, bigint, bigint, bigint];
  readonly fundChain: readonly [bigint];
  readonly pauseChain: readonly [bigint];
  readonly resumeChain: readonly [bigint];
  readonly retireChain: readonly [bigint];
  readonly governancePause: readonly [bigint, boolean];
  readonly setOperator: readonly [bigint, string];
  readonly getChain: readonly [bigint];
  readonly chainByName: readonly [string];
  readonly chainCount: readonly [];
  readonly chainIdOf: readonly [bigint];
  readonly params: readonly [];
}

export function encodeChainsCall<M extends keyof ChainsArguments>(
  method: M,
  args: ChainsArguments[M],
): string {
  const spec = CHAINS_METHODS[method];
  if (spec === undefined) throw new TypeError("Unknown chains method");
  return precompileWriteCall(
    LAYERX_CHAINS_PRECOMPILE,
    `${method}(${spec.types.join(",")})`,
    spec,
    args,
  ).data;
}

export interface ChainInfo {
  readonly id: bigint;
  readonly name: string;
  readonly operator: string;
  readonly status: bigint;
  readonly chainId: bigint;
  readonly gasAllowance: bigint;
  readonly byteAllowance: bigint;
  readonly feeShareBps: bigint;
  readonly bond: bigint;
  readonly baseFeePerGas: bigint;
  readonly createdHeight: bigint;
}

export interface ChainsParams {
  readonly authority: string;
  readonly minBond: bigint;
  readonly creationFee: bigint;
  readonly minGasAllowance: bigint;
  readonly maxGasAllowance: bigint;
  readonly totalGasAllowanceLeft: bigint;
  readonly maxChains: bigint;
}

const CHAIN_FIELDS = [
  "id",
  "name",
  "operator",
  "status",
  "chainId",
  "gasAllowance",
  "byteAllowance",
  "feeShareBps",
  "bond",
  "baseFeePerGas",
  "createdHeight",
] as const;
const CHAIN_TYPES = [
  "uint32",
  "string",
  "address",
  "uint8",
  "uint64",
  "uint64",
  "uint64",
  "uint16",
  "uint256",
  "uint256",
  "uint64",
] as const;
const PARAM_FIELDS = [
  "authority",
  "minBond",
  "creationFee",
  "minGasAllowance",
  "maxGasAllowance",
  "totalGasAllowanceLeft",
  "maxChains",
] as const;
const PARAM_TYPES = [
  "address",
  "uint256",
  "uint256",
  "uint64",
  "uint64",
  "uint64",
  "uint32",
] as const;

type ChainsValue = string | bigint;

function decodeTuple(types: readonly string[], value: unknown): ChainsValue[] {
  if (typeof value !== "string" || !/^0x(?:[0-9a-fA-F]{64})*$/u.test(value)) {
    throw new TypeError("Invalid chains ABI bytes");
  }
  const body = value.slice(2);
  const word = (offset: number): string => {
    if (
      !Number.isSafeInteger(offset) ||
      offset < 0 ||
      offset + 64 > body.length
    ) {
      throw new TypeError("Truncated chains ABI tuple");
    }
    return body.slice(offset, offset + 64);
  };
  let end = types.length * 64;
  const values = types.map((type, index): ChainsValue => {
    const raw = word(index * 64);
    const integer = BigInt(`0x${raw}`);
    if (type === "string") {
      if (integer !== BigInt(end / 2))
        throw new TypeError("Invalid chains string offset");
      const length = BigInt(`0x${word(end)}`);
      const available = BigInt((body.length - end - 64) / 2);
      if (length > available) throw new TypeError("Truncated chains string");
      const start = end + 64;
      const stop = start + Number(length) * 2;
      end = start + Math.ceil(Number(length) / 32) * 64;
      if (end > body.length || !/^0*$/u.test(body.slice(stop, end))) {
        throw new TypeError("Invalid chains string padding");
      }
      return new TextDecoder("utf-8", { fatal: true }).decode(
        Uint8Array.from(body.slice(start, stop).match(/../gu) ?? [], (byte) =>
          Number.parseInt(byte, 16),
        ),
      );
    }
    if (type === "address") {
      if (!/^0{24}/u.test(raw))
        throw new TypeError("Invalid chains address word");
      return `0x${raw.slice(24).toLowerCase()}`;
    }
    if (
      !/^uint(?:8|16|32|64|256)$/u.test(type) ||
      integer >= 1n << BigInt(type.slice(4))
    ) {
      throw new TypeError("Invalid chains integer word");
    }
    return integer;
  });
  if (body.length !== end) throw new TypeError("Invalid chains tuple length");
  return values;
}

export function decodeGetChain(value: unknown): ChainInfo {
  const values = decodeTuple(CHAIN_TYPES, value);
  return Object.fromEntries(
    CHAIN_FIELDS.map((field, index) => [field, values[index]]),
  ) as unknown as ChainInfo;
}

export function decodeChainsParams(value: unknown): ChainsParams {
  const values = decodeTuple(PARAM_TYPES, value);
  return Object.fromEntries(
    PARAM_FIELDS.map((field, index) => [field, values[index]]),
  ) as unknown as ChainsParams;
}

export const CHAINS_EVENTS = [
  {
    name: "ChainCreated",
    inputs: [
      { name: "id", type: "uint32", indexed: true },
      { name: "name", type: "string", indexed: false },
      { name: "operator", type: "address", indexed: true },
      { name: "chainId", type: "uint64", indexed: false },
    ],
  },
  {
    name: "ChainStatusChanged",
    inputs: [
      { name: "id", type: "uint32", indexed: true },
      { name: "status", type: "uint8", indexed: false },
    ],
  },
  {
    name: "ChainFunded",
    inputs: [
      { name: "id", type: "uint32", indexed: true },
      { name: "from", type: "address", indexed: true },
      { name: "amount", type: "uint256", indexed: false },
    ],
  },
  {
    name: "ChainOperatorChanged",
    inputs: [
      { name: "id", type: "uint32", indexed: true },
      { name: "operator", type: "address", indexed: true },
    ],
  },
] as const;

export interface DecodedChainsEvent {
  readonly event: (typeof CHAINS_EVENTS)[number]["name"];
  readonly precompile: string;
  readonly topic0: string;
  readonly fields: Readonly<Record<string, ChainsValue>>;
}

export function decodeChainsEvent(log: PrecompileLog): DecodedChainsEvent {
  const spec = CHAINS_EVENTS.find(
    (event) =>
      abiEventTopic(
        `${event.name}(${event.inputs.map((input) => input.type).join(",")})`,
      ) === log.topics[0]?.toLowerCase(),
  );
  if (
    log.address.toLowerCase() !== LAYERX_CHAINS_PRECOMPILE ||
    spec === undefined
  ) {
    throw new TypeError("Unknown chains event");
  }
  if (
    log.topics.length !==
    1 + spec.inputs.filter((input) => input.indexed).length
  ) {
    throw new TypeError("Invalid chains event topics");
  }
  const values = decodeTuple(
    spec.inputs.filter((input) => !input.indexed).map((input) => input.type),
    log.data,
  );
  const fields: Record<string, ChainsValue> = {};
  let topic = 1;
  let data = 0;
  for (const input of spec.inputs) {
    fields[input.name] = input.indexed
      ? decodeTuple([input.type], log.topics[topic++])[0]!
      : values[data++]!;
  }
  return {
    event: spec.name,
    precompile: LAYERX_CHAINS_PRECOMPILE,
    topic0: log.topics[0]!.toLowerCase(),
    fields,
  };
}

function chainIndex(id: number | bigint): bigint {
  if (typeof id === "number" && !Number.isSafeInteger(id))
    throw new TypeError("Invalid chain index");
  if (typeof id !== "number" && typeof id !== "bigint")
    throw new TypeError("Invalid chain index");
  const value = BigInt(id);
  if (value < 0n || value >= 1n << 32n)
    throw new RangeError("Chain index must be a uint32");
  return value;
}

export function virtualChainId(id: number | bigint): bigint {
  return 125000000n + chainIndex(id);
}

export function chainRpcUrl(base: string, id: number | bigint): string {
  const url = new URL(base);
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.search ||
    url.hash ||
    url.username ||
    url.password
  ) {
    throw new TypeError("Invalid chain RPC base URL");
  }
  return `${base.replace(/\/+$/u, "")}/chain/${chainIndex(id)}`;
}

export interface CreateChainOptions {
  readonly name: string;
  readonly gasAllowance: bigint;
  readonly byteAllowance: bigint;
  readonly feeShareBps: bigint;
  readonly value: bigint;
}

export function createChainCall(options: CreateChainOptions): PrecompileCall {
  const spec = CHAINS_METHODS.createChain;
  return precompileWriteCall(
    LAYERX_CHAINS_PRECOMPILE,
    `createChain(${spec.types.join(",")})`,
    spec,
    [
      options.name,
      options.gasAllowance,
      options.byteAllowance,
      options.feeShareBps,
    ],
    options.value,
  );
}

export class ChainsClient {
  public constructor(private readonly rpc: JsonRpcClient) {}

  public getChain(id: number | bigint): Promise<ChainInfo> {
    return this.read("getChain", [chainIndex(id)], decodeGetChain);
  }

  public chainByName(name: string): Promise<bigint> {
    return this.read("chainByName", [name], decodeChainIndex);
  }

  public chainCount(): Promise<bigint> {
    return this.read("chainCount", [], decodeChainIndex);
  }

  public params(): Promise<ChainsParams> {
    return this.read("params", [], decodeChainsParams);
  }

  public createChain(from: string, options: CreateChainOptions): WalletRequest {
    return precompileTransactionRequest(from, createChainCall(options));
  }

  private read<M extends keyof ChainsArguments, T>(
    method: M,
    args: ChainsArguments[M],
    decode: (value: unknown) => T,
  ): Promise<T> {
    return this.rpc["call"](
      "eth_call",
      [
        { to: LAYERX_CHAINS_PRECOMPILE, data: encodeChainsCall(method, args) },
        "latest",
      ],
      decode,
    );
  }
}

function decodeChainIndex(value: unknown): bigint {
  return decodeTuple(["uint32"], value)[0] as bigint;
}
