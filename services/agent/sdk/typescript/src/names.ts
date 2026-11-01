import { keccak_256 } from "@noble/hashes/sha3.js";
import {
  bytesToHex,
  concatBytes,
  hexToBytes,
  utf8ToBytes,
} from "@noble/hashes/utils.js";
import { encodeAbiCall } from "./exchange.js";
import type { JsonRpcClient } from "./rpc.js";

export interface NameResolution {
  readonly node: string;
  readonly owner: string;
  readonly address: string | null;
  readonly did: string | null;
  readonly contenthash: { readonly digest: string } | null;
}

const ZERO_ADDRESS = `0x${"00".repeat(20)}`;

export function labelhash(label: string): `0x${string}` {
  return `0x${bytesToHex(keccak_256(utf8ToBytes(label)))}`;
}

export function namehash(name: string): `0x${string}` {
  const normalized = name.toLowerCase().normalize("NFC");
  let node: Uint8Array = new Uint8Array(32);
  if (normalized !== "") {
    const labels = normalized.split(".");
    for (const label of labels) {
      if (!/^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/u.test(label)) {
        throw new TypeError(`Invalid name label: ${label}`);
      }
    }
    for (const label of labels.reverse()) {
      node = keccak_256(
        concatBytes(node, hexToBytes(labelhash(label).slice(2))),
      );
    }
  }
  return `0x${bytesToHex(node)}`;
}

function contractAddress(value: string): string {
  if (!/^0x[0-9a-fA-F]{40}$/u.test(value)) {
    throw new TypeError("Invalid name contract address");
  }
  return value.toLowerCase();
}

function decodeAddress(value: unknown): string {
  if (typeof value !== "string" || !/^0x0{24}[0-9a-fA-F]{40}$/u.test(value)) {
    throw new TypeError("Invalid name address response");
  }
  return `0x${value.slice(26).toLowerCase()}`;
}

function decodeBytes(value: unknown): Uint8Array {
  if (
    typeof value !== "string" ||
    !/^0x(?:[0-9a-fA-F]{64}){2,}$/u.test(value)
  ) {
    throw new TypeError("Invalid name bytes response");
  }
  const body = value.slice(2);
  const offset = BigInt(`0x${body.slice(0, 64)}`);
  const length = BigInt(`0x${body.slice(64, 128)}`);
  const available = BigInt(body.length / 2 - 64);
  if (offset !== 32n || ((length + 31n) / 32n) * 32n !== available) {
    throw new TypeError("Invalid name bytes response");
  }
  const end = 128 + Number(length) * 2;
  if (!/^0*$/u.test(body.slice(end))) {
    throw new TypeError("Invalid name bytes padding");
  }
  return hexToBytes(body.slice(128, end));
}

export async function resolveName(
  client: JsonRpcClient,
  contracts: { registry: string; resolver: string },
  name: string,
): Promise<NameResolution> {
  const node = namehash(name);
  const registry = contractAddress(contracts.registry);
  const resolver = contractAddress(contracts.resolver);
  const owner = await client["call"](
    "eth_call",
    [
      { to: registry, data: encodeAbiCall("owner", ["bytes32"], [node]) },
      "latest",
    ],
    decodeAddress,
  );
  if (owner === ZERO_ADDRESS) {
    return { node, owner, address: null, did: null, contenthash: null };
  }
  const address = await client["call"](
    "eth_call",
    [
      { to: resolver, data: encodeAbiCall("addr", ["bytes32"], [node]) },
      "latest",
    ],
    decodeAddress,
  );
  const did = await client["call"](
    "eth_call",
    [
      {
        to: resolver,
        data: encodeAbiCall(
          "text",
          ["bytes32", "string"],
          [node, "layerx.did"],
        ),
      },
      "latest",
    ],
    (value) =>
      new TextDecoder("utf-8", { fatal: true }).decode(decodeBytes(value)),
  );
  const content = await client["call"](
    "eth_call",
    [
      { to: resolver, data: encodeAbiCall("contenthash", ["bytes32"], [node]) },
      "latest",
    ],
    decodeBytes,
  );
  const contentHex = bytesToHex(content);
  const contenthash =
    content.length === 36 && contentHex.startsWith("70617831")
      ? { digest: `0x${contentHex.slice(8)}` }
      : null;
  return {
    node,
    owner,
    address: address === ZERO_ADDRESS ? null : address,
    did: did === "" ? null : did,
    contenthash,
  };
}
