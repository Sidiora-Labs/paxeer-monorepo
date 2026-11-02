import { readFile } from "node:fs/promises";
import { parseArgs } from "node:util";
import {
  decodePxResolvedIdentities,
  decodeRawEip1559,
  keccak256,
} from "@sidiora/layerx-sdk";
import {
  PaymentRpc,
  PaymentRpcError,
  decodePaymentRequiredHeader,
  verifyRpcPayment as verifySellerRpcPayment,
  rpcHex,
  rpcObject,
} from "../seller/dist/index.js";

class PublicRpc extends PaymentRpc {
  #id = 0;
  #endpoint;
  #headers;

  constructor(endpoint, headers) {
    super(endpoint, headers);
    this.#endpoint = new URL(endpoint);
    this.#headers = headers;
  }

  async call(method, params) {
    const id = ++this.#id;
    const response = await fetch(this.#endpoint, {
      method: "POST",
      redirect: "error",
      signal: AbortSignal.timeout(30_000),
      headers: { ...this.#headers, "content-type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
    });
    const reader = response.body?.getReader();
    if (!reader) throw new Error("missing-rpc-body");
    const chunks = [];
    let size = 0;
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      size += part.value.length;
      if (size > 8_388_608) {
        await reader.cancel();
        throw new Error("rpc-body-too-large");
      }
      chunks.push(part.value);
    }
    const bytes = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) {
      bytes.set(chunk, offset);
      offset += chunk.length;
    }
    let body;
    try {
      body = rpcObject(
        JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)),
      );
    } catch (error) {
      if (!response.ok) throw new Error(`rpc-http-${response.status}`);
      throw error;
    }
    if (
      body.jsonrpc !== "2.0" ||
      body.id !== id ||
      "error" in body === "result" in body
    )
      throw new Error("invalid-rpc-envelope");
    if ("error" in body) {
      const failure = rpcObject(body.error);
      if (
        !Number.isInteger(failure.code) ||
        typeof failure.message !== "string"
      )
        throw new Error("invalid-rpc-error");
      const error = new PaymentRpcError(failure.code, failure.data);
      error.message = `${method}: ${error.message}`;
      throw error;
    }
    if (!response.ok) throw new Error(`rpc-http-${response.status}`);
    if (method === "eth_getTransactionCount") {
      if (
        typeof body.result !== "string" ||
        !/^0x(?:0|[1-9a-f][0-9a-f]{0,15})$/u.test(body.result)
      )
        throw new Error("invalid-rpc-nonce");
      return body.result;
    }
    if (method === "eth_sendRawTransaction") return body.result;
    if (method === "pax_sequencerReceipt" && body.result === null) return null;
    return rpcObject(body.result);
  }
}

async function verifyRpcPayment(
  rpc,
  rawTransaction,
  txHash,
  payer,
  offer,
  receiptKeys,
) {
  const submittedHash = await rpc.send(
    rawTransaction,
    offer.extra?.layerx?.commitment ?? "receipted",
  );
  if (submittedHash !== txHash) throw new Error("transaction-hash-mismatch");
  const receipt = await rpc.receipt(txHash);
  if (receipt === null) return { kind: "pending" };
  return verifySellerRpcPayment(
    receipt,
    rawTransaction,
    payer,
    offer,
    receiptKeys,
  );
}

function receiptHeight(value) {
  if (
    !(typeof value === "number" && Number.isSafeInteger(value) && value >= 0) &&
    !(
      typeof value === "string" &&
      /^(?:0|[1-9][0-9]*|0x(?:0|[1-9a-f][0-9a-f]*))$/u.test(value)
    )
  )
    throw new Error("invalid-receipt-key-height");
  const height = BigInt(value);
  if (height > 0xffff_ffff_ffff_ffffn)
    throw new Error("invalid-receipt-key-height");
  return height;
}

const { values } = parseArgs({
  options: {
    help: { type: "boolean" },
    rpc: { type: "string" },
    did: { type: "string" },
    faucet: { type: "string" },
    "public-key": { type: "string" },
    "claim-key": { type: "string" },
    activity: { type: "string" },
    offer: { type: "string" },
    "receipt-keys": { type: "string" },
    payer: { type: "string" },
  },
});
values.rpc ??= process.env.LAYERX_RPC_URL;
values.faucet ??= process.env.LAYERX_FAUCET_URL;
values.did ??= process.env.LAYERX_DID;
if (values.help) {
  process.stdout.write(
    "Usage: node public-rpc.mjs --rpc https://HOST/rpc --did DID [--faucet https://HOST/v1/faucet/claims --public-key HEX32 --claim-key KEY] [--activity RAW_EIP1559_HEX_FILE --offer PAYMENT_REQUIRED_HEADER_FILE --receipt-keys RECEIPT_KEYS_JSON --payer ACCOUNT_HEX32]\nURLs may come from LAYERX_RPC_URL and LAYERX_FAUCET_URL; DID from LAYERX_DID. Use LAYERX_RPC_TOKEN for submission and LAYERX_FAUCET_TOKEN for faucet authentication. The payment example verifies the receipted or committed rung selected by the offer. Receipt keys are a JSON array with keyId, publicKey (32-byte hex), firstHeight and lastHeight for each entry.\n",
  );
} else {
  if (!values.rpc || !values.did) throw new Error("rpc-and-did-required");
  if (
    new URL(values.rpc).port === "18545" ||
    (values.faucet && new URL(values.faucet).port === "18545")
  )
    throw new Error("persistent-host-chain-forbidden");
  const rpc = new PublicRpc(
    values.rpc,
    process.env.LAYERX_RPC_TOKEN
      ? { authorization: `Bearer ${process.env.LAYERX_RPC_TOKEN}` }
      : {},
  );
  if (values.faucet) {
    const url = new URL(values.faucet);
    if (
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      url.pathname !== "/v1/faucet/claims" ||
      (url.protocol !== "https:" &&
        !(
          url.protocol === "http:" &&
          ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)
        ))
    )
      throw new Error("invalid-faucet-url");
    rpcHex(values["public-key"], 32);
    if (!values["claim-key"] || !process.env.LAYERX_FAUCET_TOKEN)
      throw new Error("faucet-claim-key-and-token-required");
    const response = await fetch(url, {
      method: "POST",
      redirect: "error",
      signal: AbortSignal.timeout(30_000),
      headers: {
        "content-type": "application/json",
        authorization: `Bearer ${process.env.LAYERX_FAUCET_TOKEN}`,
        "idempotency-key": values["claim-key"],
      },
      body: JSON.stringify({
        did: values.did,
        public_key: values["public-key"],
      }),
    });
    await response.body?.cancel();
    process.stdout.write(
      `Faucet HTTP status: ${response.status}; confirm the account balance before spending.\n`,
    );
    if (!response.ok) process.exitCode = 1;
  }
  const account = decodePxResolvedIdentities(
    await rpc.call("px_resolveAccount", [values.did]),
  );
  if (
    !account.bound ||
    account.layerxDid !== values.did ||
    account.evmAddress === null
  )
    throw new Error("did-evm-binding-required");
  const sequence = await rpc.call("eth_getTransactionCount", [
    account.evmAddress,
    "pending",
  ]);
  process.stdout.write(JSON.stringify({ sequence }) + "\n");
  if (values.activity) {
    if (!values.offer || !values["receipt-keys"] || !values.payer)
      throw new Error("offer-receipt-keys-and-payer-required");
    const required = decodePaymentRequiredHeader(
      (await readFile(values.offer, "utf8")).trim(),
    );
    const offer = required.accepts.find(
      (v) =>
        ["exact", "metered", "subscription"].includes(v.scheme) &&
        ["receipted", "committed"].includes(
          v.extra?.layerx?.commitment ?? "receipted",
        ),
    );
    if (!offer) throw new Error("receipted-or-committed-offer-required");
    const commitment = offer.extra?.layerx?.commitment ?? "receipted";
    const rawTransaction = (await readFile(values.activity, "utf8")).trim();
    if (!/^0x02(?:[0-9a-f]{2})+$/u.test(rawTransaction))
      throw new Error("raw-eip1559-transaction-required");
    const raw = rpcHex(rawTransaction);
    decodeRawEip1559(raw);
    const txHash = `0x${Buffer.from(keccak256(raw)).toString("hex")}`;
    const configured = JSON.parse(
      await readFile(values["receipt-keys"], "utf8"),
    );
    if (!Array.isArray(configured) || configured.length === 0)
      throw new Error("invalid-receipt-keys");
    const receiptKeys = configured.map((entry) => {
      const key = rpcObject(entry);
      if (
        !["keyId", "firstHeight", "lastHeight", "publicKey"].every((name) =>
          Object.hasOwn(key, name),
        )
      )
        throw new Error("receipt-key-id-range-and-public-key-required");
      const first_height = receiptHeight(key.firstHeight);
      const last_height = receiptHeight(key.lastHeight);
      if (first_height > last_height)
        throw new Error("invalid-receipt-key-range");
      return {
        keyId: rpcHex(key.keyId, 32),
        publicKey: rpcHex(key.publicKey, 32),
        first_height,
        last_height,
      };
    });
    rpcHex(values.payer, 32);
    const payment = await verifyRpcPayment(
      rpc,
      rawTransaction,
      txHash,
      values.payer,
      offer,
      receiptKeys,
    );
    if (payment.kind === "pending") {
      process.stdout.write(
        JSON.stringify({ state: "pending", transaction: txHash }) + "\n",
      );
      process.exitCode = 2;
    } else {
      process.stdout.write(
        JSON.stringify({
          transaction: payment.verification.txHash,
          commitment,
        }) + "\n",
      );
    }
  }
}
