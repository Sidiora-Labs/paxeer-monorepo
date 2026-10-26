import assert from "node:assert/strict";
import { once } from "node:events";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import { test } from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import { abiSelector, encodeAbiCall } from "../src/exchange.js";
import {
  SIDIORA_TOKEN,
  assembleEip7702Authorization,
  discoverSponsoredNonce,
  eip7702AuthorizationDigest,
  gasQuoteDigest,
  readNativeFeePreference,
  requestGasQuote,
  requestSponsoredGasBatchStatus,
  sponsoredBatchCall,
  sponsoredBatchDigest,
  submitSponsoredGasBatch,
  type GasResult,
  type GasStationConfig,
  type SponsoredBatch,
} from "../src/gas-station.js";

interface RpcRequest {
  readonly method: string;
  readonly params: readonly unknown[];
}

function recordingRequest(responses: readonly unknown[]) {
  const calls: RpcRequest[] = [];
  return {
    calls,
    request: async (args: RpcRequest): Promise<unknown> => {
      assert.ok(calls.length < responses.length, `unexpected ${args.method}`);
      calls.push(args);
      return responses[calls.length - 1];
    },
  };
}

function value<T>(result: GasResult<T>): T {
  assert.equal(
    result.ok,
    true,
    JSON.stringify(result, (_key, item: unknown) =>
      typeof item === "bigint" ? item.toString() : item,
    ),
  );
  if (!result.ok) throw new Error("expected a successful gas station result");
  return result.value;
}

async function listen(
  handler: (request: IncomingMessage, response: ServerResponse) => void,
) {
  const server = createServer(handler);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  return {
    url: `http://127.0.0.1:${address.port}`,
    close: () =>
      new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      }),
  };
}

const account = `0x${"11".repeat(20)}`;
const config: GasStationConfig = {
  chainId: 125n,
  sponsor: `0x${"22".repeat(20)}`,
  token: SIDIORA_TOKEN,
  decimals: 6,
  paymaster: `0x${"44".repeat(20)}`,
};

async function sponsored_nonce_is_read_at_the_pending_tag() {
  const nonce = `0x${"0".repeat(63)}7`;
  const call = { to: account, data: abiSelector("nonce()") };
  for (const delegated of [true, false]) {
    const implementation = "0x60005460005260206000f3";
    const rpc = recordingRequest(
      delegated
        ? ["0x7d", `0xef0100${config.paymaster.slice(2)}`, nonce]
        : ["0x7d", "0x", implementation, nonce],
    );
    assert.equal(
      value(await discoverSponsoredNonce(config, account, rpc.request)),
      7n,
    );
    assert.deepEqual(rpc.calls, [
      { method: "eth_chainId", params: [] },
      { method: "eth_getCode", params: [account, "pending"] },
      ...(!delegated
        ? [{ method: "eth_getCode", params: [config.paymaster, "pending"] }]
        : []),
      {
        method: "eth_call",
        params: delegated
          ? [call, "pending"]
          : [call, "pending", { [account]: { code: implementation } }],
      },
    ]);
  }
  const incompatible = recordingRequest(["0x7d", "0x6000"]);
  assert.deepEqual(
    await discoverSponsoredNonce(config, account, incompatible.request),
    {
      ok: false,
      refusal: { code: "incompatible_delegation", field: "delegation" },
    },
  );
  assert.equal(incompatible.calls.length, 2);
}

async function native_fee_preference_reads_feetoken_getFeeDenom() {
  const denom = "factory/bridge/usid";
  const currentDenom = "apax";
  const blockHash = `0x${"ab".repeat(32)}`;
  const head = { hash: blockHash, number: "0x64" };
  const feetoken = "0x0000000000000000000000000000000000001018";
  const block = { blockHash, requireCanonical: true };
  const rpc = recordingRequest([
    "0x7d",
    `0x${encodeAbiCall("result", ["string"], [currentDenom]).slice(10)}`,
    head,
    `0x${encodeAbiCall("result", ["string", "uint256", "uint256", "uint256"], [denom, 0n, 0n, 0n]).slice(10)}`,
    "0x",
    head,
  ]);
  const reads: { path: string; height: string | string[] | undefined }[] = [];
  const parameters: Readonly<Record<string, unknown>> = {
    KeyFeeTokenEnabled: true,
    KeyAllowedFeeDenoms: [{ denom, rate: "1.25", rate_update_height: "90" }],
    KeyMaxFeeTokenRateAge: "20",
  };
  const http = await listen((request, response) => {
    const url = new URL(request.url!, "http://localhost");
    reads.push({
      path: url.pathname,
      height: request.headers["x-cosmos-block-height"],
    });
    let payload: unknown;
    if (url.pathname === "/cosmos/upgrade/v1beta1/applied_plan/v6.7") {
      payload = { height: "1" };
    } else if (url.pathname === "/cosmos/params/v1beta1/params") {
      const key = url.searchParams.get("key")!;
      assert.ok(Object.hasOwn(parameters, key));
      payload = {
        param: { subspace: "evm", key, value: JSON.stringify(parameters[key]) },
      };
    } else {
      assert.equal(
        url.pathname,
        `/cosmos/bank/v1beta1/denoms_metadata/${encodeURIComponent(denom)}`,
      );
      payload = {
        metadata: {
          base: denom,
          display: "SID",
          symbol: "SID",
          name: "Sidiora",
          denom_units: [
            { denom, exponent: 0 },
            { denom: "SID", exponent: 6 },
          ],
        },
      };
    }
    response.writeHead(200, {
      "content-type": "application/json",
      "x-cosmos-block-height": "100",
    });
    response.end(JSON.stringify(payload));
  });
  try {
    const preference = await readNativeFeePreference(account, {
      restUrl: http.url,
      request: rpc.request,
    });
    assert.deepEqual(preference, {
      kind: "native-fee-preference",
      state: "available",
      chainId: 125n,
      blockHash,
      blockNumber: 100n,
      denom,
      symbol: "SID",
      decimals: 6,
      rate: "1.25",
      rateUpdateHeight: 90n,
      currentDenom,
      call: {
        to: feetoken,
        value: 0n,
        data: encodeAbiCall("setFeeDenom", ["string"], [denom]),
      },
    });
    assert.deepEqual(rpc.calls, [
      { method: "eth_chainId", params: [] },
      {
        method: "eth_call",
        params: [
          {
            to: feetoken,
            data: `0x345ed2a4${account.slice(2).padStart(64, "0")}`,
          },
          "pending",
        ],
      },
      { method: "eth_getBlockByNumber", params: ["finalized", false] },
      {
        method: "eth_call",
        params: [
          {
            to: "0x0000000000000000000000000000000000001016",
            data: encodeAbiCall(
              "getCap",
              ["uint64", "address"],
              [0x534f4c414e41n, SIDIORA_TOKEN],
            ),
          },
          block,
        ],
      },
      {
        method: "eth_call",
        params: [
          {
            from: account,
            to: feetoken,
            data: encodeAbiCall("setFeeDenom", ["string"], [denom]),
          },
          block,
        ],
      },
      { method: "eth_getBlockByNumber", params: ["0x64", false] },
    ]);
    assert.equal(reads.length, 5);
    assert.ok(reads.every((read) => read.height === "100"));
    assert.ok(reads.every((read) => !read.path.includes("preference")));
    const invalid = recordingRequest(["0x7d", "0x"]);
    assert.deepEqual(
      await readNativeFeePreference(account, {
        restUrl: http.url,
        request: invalid.request,
      }),
      {
        kind: "native-fee-preference",
        state: "unavailable",
        reason: "nativeFeeAbi",
      },
    );
    assert.equal(reads.length, 5);
  } finally {
    await http.close();
  }
}

function sign(digest: string, key: Uint8Array): string {
  const signature = secp256k1.sign(Buffer.from(digest.slice(2), "hex"), key, {
    prehash: false,
    format: "recovered",
  });
  return `0x${Buffer.from(signature.slice(1)).toString("hex")}${(27 + signature[0]!).toString(16)}`;
}

function evmAddress(key: Uint8Array): string {
  return `0x${Buffer.from(
    keccak_256(secp256k1.getPublicKey(key, false).slice(1)),
  )
    .subarray(12)
    .toString("hex")}`;
}

async function submitted_sponsored_batch_reports_the_transaction_hash_at_the_submitted_rung() {
  const accountKey = keccak_256(
    new TextEncoder().encode("pending sponsored account"),
  );
  const sponsorKey = keccak_256(
    new TextEncoder().encode("pending sponsored relayer"),
  );
  const batch: SponsoredBatch = {
    chainId: 125n,
    account: evmAddress(accountKey),
    nonce: 7n,
    calls: [{ to: config.paymaster, value: 0n, data: "0x1234" }],
    quote: {
      sponsor: evmAddress(sponsorKey),
      token: SIDIORA_TOKEN,
      decimals: 6,
      maxTokenAmount: 2_000_000n,
      tokenAmount: 1_000_000n,
      deadline: 1000n,
      quoteNonce: 2n,
      gasCost: 100n,
    },
  };
  const transactionHash = `0x${"cd".repeat(32)}`;
  const relayerSignature = sign(
    value(gasQuoteDigest(batch.chainId, batch.account, batch.quote)),
    sponsorKey,
  );
  const requests: {
    method: string | undefined;
    path: string | undefined;
    body: unknown;
  }[] = [];
  const http = await listen((request, response) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => {
      requests.push({
        method: request.method,
        path: request.url,
        body: JSON.parse(Buffer.concat(chunks).toString()),
      });
      assert.ok(
        request.url === "/gas-station/quote" ||
          request.url === "/gas-station/submit",
      );
      response.writeHead(200, { "content-type": "application/json" });
      response.end(
        JSON.stringify(
          request.url.endsWith("/quote")
            ? { quote: batch.quote, relayerSignature, feeDenom: "apax" }
            : { transactionHash },
          (_key, item: unknown) =>
            typeof item === "bigint" ? item.toString() : item,
        ),
      );
    });
  });
  try {
    const stationConfig = {
      ...config,
      sponsor: batch.quote.sponsor,
      quoteUrl: `${http.url}/gas-station/quote`,
    };
    const quoteRequest = {
      account: batch.account,
      did: `did:layerx:${"ab".repeat(32)}`,
      nonce: batch.nonce,
      calls: batch.calls,
      maxTokenAmount: batch.quote.maxTokenAmount,
      gasCost: batch.quote.gasCost,
    };
    const quoted = value(
      await requestGasQuote(stationConfig, quoteRequest, { now: 999n }),
    );
    assert.deepEqual(quoted, {
      quote: batch.quote,
      relayerSignature,
      feeDenom: "apax",
    });
    const accountSignature = sign(
      value(sponsoredBatchDigest(batch)),
      accountKey,
    );
    const authorization = value(
      assembleEip7702Authorization(
        stationConfig,
        batch.account,
        3n,
        sign(
          value(
            eip7702AuthorizationDigest({
              chainId: 125n,
              address: config.paymaster,
              nonce: 3n,
            }),
          ),
          accountKey,
        ),
      ),
    );
    const submission = value(
      await submitSponsoredGasBatch(
        stationConfig,
        batch,
        accountSignature,
        relayerSignature,
        authorization,
        { now: 999n },
      ),
    );
    assert.deepEqual(submission, { transactionHash, rung: "submitted" });
    assert.deepEqual(
      requests.map(({ method, path }) => ({ method, path })),
      [
        { method: "POST", path: "/gas-station/quote" },
        { method: "POST", path: "/gas-station/submit" },
      ],
    );
    assert.deepEqual(
      requests[1]!.body,
      JSON.parse(
        JSON.stringify(
          {
            call: value(
              sponsoredBatchCall(
                stationConfig,
                batch,
                accountSignature,
                relayerSignature,
                999n,
              ),
            ),
            authorization,
            batch,
            accountSignature,
            relayerSignature,
          },
          (_key, item: unknown) =>
            typeof item === "bigint" ? item.toString() : item,
        ),
      ),
    );
    const rpc = recordingRequest([
      null,
      { txHash: transactionHash, rung: "submitted" },
      { txHash: transactionHash, rung: "receipted" },
      { txHash: transactionHash, rung: "committed" },
      { txHash: `0x${"ef".repeat(32)}`, rung: "receipted" },
      { txHash: transactionHash, rung: "included" },
    ]);
    assert.equal(
      value(await requestSponsoredGasBatchStatus(transactionHash, rpc.request)),
      null,
    );
    for (const rung of ["submitted", "receipted", "committed"] as const) {
      assert.deepEqual(
        value(
          await requestSponsoredGasBatchStatus(transactionHash, rpc.request),
        ),
        { transactionHash, rung },
      );
    }
    assert.deepEqual(
      await requestSponsoredGasBatchStatus(transactionHash, rpc.request),
      { ok: false, refusal: { code: "invalid_response", field: "txHash" } },
    );
    assert.deepEqual(
      await requestSponsoredGasBatchStatus(transactionHash, rpc.request),
      { ok: false, refusal: { code: "invalid_response", field: "rung" } },
    );
    assert.deepEqual(
      rpc.calls,
      Array.from({ length: 6 }, () => ({
        method: "pax_sequencerReceipt",
        params: [transactionHash],
      })),
    );
  } finally {
    await http.close();
  }
}

test(sponsored_nonce_is_read_at_the_pending_tag);
test(native_fee_preference_reads_feetoken_getFeeDenom);
test(
  submitted_sponsored_batch_reports_the_transaction_hash_at_the_submitted_rung,
);
