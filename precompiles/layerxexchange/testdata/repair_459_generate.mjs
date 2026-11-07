import { createHash, createPrivateKey, createPublicKey } from "node:crypto";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { execFileSync } from "node:child_process";

const source = "services/agent/sdk/typescript/src/exchange.ts";
const temporary = await mkdtemp(join(tmpdir(), "repair459-sdk-"));
await writeFile(join(temporary, "package.json"), '{"type":"module"}\n');
execFileSync("npm", ["install", "--prefix", temporary, "--ignore-scripts", "--no-audit", "--no-fund", "@noble/hashes@2.4.0"], { stdio: "inherit" });
const sdk = await readFile(source);
await writeFile(join(temporary, "exchange.ts"), sdk);
const { exchangePlaceOrderCall } = await import(pathToFileURL(join(temporary, "exchange.ts")));
const hash = (...parts) => createHash("sha256").update(Buffer.concat(parts)).digest();
const seed = Buffer.alloc(32);
seed[0] = 1;
const privateKey = createPrivateKey({ key: Buffer.concat([Buffer.from("302e020100300506032b657004220420", "hex"), seed]), format: "der", type: "pkcs8" });
const key = createPublicKey(privateKey).export({ format: "der", type: "spki" }).subarray(-32);
const did = Buffer.from(`did:layerx:${key.toString("hex")}`);
const didLength = Buffer.alloc(2);
didLength.writeUInt16BE(did.length);
const owner = hash(Buffer.from("LXP/v1/did-id\0"), didLength, did);
const salt = Buffer.alloc(32);
salt[0] = 0x31;
const asset = hash(Buffer.from("LX:ASSET:v1"), owner, salt);
const name = Buffer.from(`agent:${did.toString()}:asset:${asset.toString("hex")}`);
const nameLength = Buffer.alloc(4);
nameLength.writeUInt32BE(name.length);
const accountId = `0x${hash(Buffer.from("LX:ACCOUNT:v1"), nameLength, name).toString("hex")}`;
const marketId = `0x6d01${"00".repeat(30)}`;
const call = exchangePlaceOrderCall({ marketId, accountId, side: 1, price: 30000n, quantity: 2n, timeInForce: 0 });
await writeFile("precompiles/layerxexchange/testdata/repair_459_typescript.json", `${JSON.stringify({
  producer: `${source}#exchangePlaceOrderCall`,
  sourceSha256: createHash("sha256").update(sdk).digest("hex"),
  generate: "node precompiles/layerxexchange/testdata/repair_459_generate.mjs",
  signature: "placeOrder(bytes32,bytes32,uint8,uint256,uint256,uint8)",
  pythonCaller: "services/agent/sdk/python/layerx_sdk/exchange.py#exchange_place_order_call",
  marketId, accountId, side: 1, price: "30000", quantity: "2", timeInForce: 0,
  to: call.to, data: call.data, value: call.value.toString(),
}, null, 2)}\n`);
