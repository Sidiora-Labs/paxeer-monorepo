/**
 * Proves: src/gas-station.ts requestGasQuote/discoverSponsoredNonce/readNativeFeePreference/submitSponsoredGasBatch -> src/index.ts and src/browser.ts over loopback HTTP/JSON-RPC -> sdk/wallet/src/modules/gas-station.ts gasStation and the SDK submission consumer.
 *
 * Repairs wireUint in src/gas-station.ts: an overflowing station quantity is an
 * invalid_response, not invalid_value (caller input). Both consumers assert it.
 * The producer is owned by this test, signs each requested quote and verifies
 * each submitted batch. No configuration, credentials or URLs come from env.
 * The anchor is stateless: repeated reads must reach the producer again.
 * Wallet source is transpiled by a loader, with every import left intact. Only
 * the agent SDK is imported through dist; the declared verify builds it first.
 */
import assert from "node:assert/strict";
import { once } from "node:events";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire, registerHooks } from "node:module";
import { test } from "node:test";
import ts from "typescript";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import * as sdk from "../src/index.js";
import * as browser from "../src/browser.js";
import type { GasQuote, GasResult, SponsoredBatch } from "../src/index.js";
import { gas_nonce_root_route_uses_pending } from "./support/gas-root-route.js";

function value<T>(answer: GasResult<T>): T {
  assert.equal(answer.ok, true, wire(answer));
  if (!answer.ok) throw new Error("expected gas station success");
  return answer.value;
}

function wire(data: unknown): string {
  return JSON.stringify(data, (_key, item: unknown) =>
    typeof item === "bigint" ? item.toString() : item,
  );
}

function object(data: unknown): Record<string, unknown> {
  assert.ok(data !== null && typeof data === "object" && !Array.isArray(data));
  return data as Record<string, unknown>;
}

function evmAddress(key: Uint8Array): string {
  return `0x${Buffer.from(
    keccak_256(secp256k1.getPublicKey(key, false).slice(1)),
  )
    .subarray(12)
    .toString("hex")}`;
}

function sign(digest: string, key: Uint8Array): string {
  const signed = secp256k1.sign(Buffer.from(digest.slice(2), "hex"), key, {
    prehash: false,
    format: "recovered",
  });
  return `0x${Buffer.from(signed.slice(1)).toString("hex")}${(27 + signed[0]!).toString(16)}`;
}

function abiResult(
  types: Parameters<typeof sdk.encodeAbiCall>[1],
  values: Parameters<typeof sdk.encodeAbiCall>[2],
): string {
  return `0x${sdk.encodeAbiCall("result", types, values).slice(10)}`;
}

export async function proof_gas_station_producer_transport_consumer(): Promise<void> {
  for (const name of [
    "requestGasQuote",
    "discoverSponsoredNonce",
    "readNativeFeePreference",
    "submitSponsoredGasBatch",
  ] as const) {
    assert.equal(
      browser[name],
      sdk[name],
      `${name} must cross the public module unchanged`,
    );
  }
  const walletRoot = new URL(
    "../../../../../../sdk/wallet/src/",
    import.meta.url,
  );
  assert.ok(
    existsSync(new URL("modules/gas-station.ts", walletRoot)),
    "real wallet consumer source must exist",
  );
  const walletRequire = createRequire(new URL("../package.json", walletRoot));
  assert.ok(existsSync(walletRequire.resolve("viem")), "wallet workspace viem dependency must be installed");
  const loader = registerHooks({
    resolve(specifier, context, next) {
      if (
        context.parentURL?.startsWith(walletRoot.href) &&
        specifier.startsWith(".") &&
        specifier.endsWith(".js")
      ) {
        const source = new URL(
          specifier.slice(0, -3) + ".ts",
          context.parentURL,
        );
        if (existsSync(source)) return next(source.href, context);
      }
      return next(specifier, context);
    },
    load(url, context, next) {
      if (url.startsWith(walletRoot.href) && url.endsWith(".ts")) {
        return {
          format: "module",
          shortCircuit: true,
          source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
            compilerOptions: {
              module: ts.ModuleKind.ESNext,
              target: ts.ScriptTarget.ES2022,
            },
            fileName: new URL(url).pathname,
          }).outputText,
        };
      }
      return next(url, context);
    },
  });

  const accountKey = keccak_256(
    new TextEncoder().encode("gas station proof account"),
  );
  const sponsorKey = keccak_256(
    new TextEncoder().encode("gas station proof sponsor"),
  );
  const account = evmAddress(accountKey);
  const paymaster = `0x${"44".repeat(20)}`;
  const denom = "factory/producer/usid";
  const blockHash = `0x${"12".repeat(32)}`;
  const calls = [{ to: `0x${"33".repeat(20)}`, value: 0n, data: "0x1234" }];
  const now = 999n;
  const state = {
    nonce: 7n,
    quoteNonce: 31n,
    gasCost: 100n,
    quoteMode: "valid" as "valid" | "wrong-signer" | "overflow",
    malformedNonce: false,
    malformedNative: false,
    malformedSubmission: false,
    nativeEnabled: true,
    delegated: true,
  };
  const counts = new Map<string, number>();
  const producerErrors: unknown[] = [];
  const issued: GasQuote[] = [];
  const submissions: string[] = [];
  const count = (key: string) => counts.set(key, (counts.get(key) ?? 0) + 1);
  const config = {
    chainId: 125n,
    sponsor: evmAddress(sponsorKey),
    token: sdk.SIDIORA_TOKEN,
    decimals: sdk.SIDIORA_DECIMALS,
    paymaster,
    quoteUrl: "",
  };
  const quoteRequest = () => ({
    account,
    did: `did:layerx:${"ab".repeat(32)}`,
    nonce: state.nonce,
    calls,
    maxTokenAmount: 2_100_000n,
    gasCost: state.gasCost,
  });
  const produceQuote = (): GasQuote => ({
    sponsor: config.sponsor,
    token: config.token,
    decimals: config.decimals,
    maxTokenAmount: 2_100_000n,
    tokenAmount: 2_000_000n,
    deadline: 1000n,
    quoteNonce: state.quoteNonce,
    gasCost: state.gasCost,
  });

  const server = createServer(async (incoming, outgoing) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
      const body: unknown = chunks.length
        ? JSON.parse(Buffer.concat(chunks).toString("utf8"))
        : null;
      const path = new URL(incoming.url!, "http://localhost");
      count(path.pathname + path.search);
      outgoing.setHeader("content-type", "application/json");
      const reply = (data: unknown) => outgoing.end(wire(data));
      if (path.pathname === "/gas-station/quote") {
        assert.equal(incoming.method, "POST");
        assert.equal(incoming.headers["content-type"], "application/json");
        assert.deepEqual(
          body,
          JSON.parse(
            wire({
              ...quoteRequest(),
              chainId: config.chainId,
              token: config.token,
              decimals: config.decimals,
            }),
          ),
        );
        const quote = produceQuote();
        issued.push(quote);
        const relayerSignature = sign(
          value(sdk.gasQuoteDigest(config.chainId, account, quote)),
          state.quoteMode === "wrong-signer" ? accountKey : sponsorKey,
        );
        reply({
          quote:
            state.quoteMode === "overflow"
              ? { ...quote, tokenAmount: (1n << 256n).toString() }
              : quote,
          relayerSignature,
          feeDenom: denom,
        });
        return;
      }
      if (path.pathname === "/gas-station/submit") {
        assert.equal(incoming.method, "POST");
        const payload = object(body);
        const batch: SponsoredBatch = {
          chainId: config.chainId,
          account,
          nonce: state.nonce,
          calls,
          quote: issued.at(-1)!,
        };
        const accountSignature = payload.accountSignature;
        const relayerSignature = payload.relayerSignature;
        assert.ok(typeof accountSignature === "string");
        assert.ok(typeof relayerSignature === "string");
        const authorization = object(payload.authorization);
        const authSignature = `${authorization.r}${String(authorization.s).slice(2)}${(Number(authorization.yParity) + 27).toString(16)}`;
        const verifiedAuthorization = value(
          sdk.assembleEip7702Authorization(config, account, 0n, authSignature),
        );
        const call = value(
          sdk.sponsoredBatchCall(
            config,
            batch,
            accountSignature,
            relayerSignature,
            now,
          ),
        );
        assert.deepEqual(
          payload,
          JSON.parse(
            wire({
              batch,
              call,
              authorization: verifiedAuthorization,
              accountSignature,
              relayerSignature,
            }),
          ),
        );
        const transactionHash = `0x${Buffer.from(keccak_256(new TextEncoder().encode(wire(payload)))).toString("hex")}`;
        submissions.push(transactionHash);
        reply({
          transactionHash: state.malformedSubmission
            ? "0x1234"
            : transactionHash,
        });
        return;
      }
      if (path.pathname === "/") {
        assert.equal(incoming.method, "POST");
        const rpc = object(body);
        assert.equal(rpc.jsonrpc, "2.0");
        assert.ok(typeof rpc.method === "string");
        assert.ok(Array.isArray(rpc.params));
        const params = rpc.params;
        count(rpc.method);
        let result: unknown;
        switch (rpc.method) {
          case "eth_chainId":
            assert.deepEqual(params, []);
            result = "0x7d";
            break;
          case "eth_getCode":
            assert.deepEqual(params, [account, "pending"]);
            result = state.delegated
              ? `0xef0100${paymaster.slice(2)}`
              : "0x6000";
            break;
          case "eth_getBlockByNumber":
            assert.ok(params[0] === "finalized" || params[0] === "0x64");
            assert.equal(params[1], false);
            result = { hash: blockHash, number: "0x64" };
            break;
          case "eth_call": {
            const call = object(params[0]);
            if (call.to === account) {
              assert.deepEqual(params, [
                { to: account, data: sdk.abiSelector("nonce()") },
                "pending",
              ]);
              result = state.malformedNonce
                ? "0x01"
                : `0x${state.nonce.toString(16).padStart(64, "0")}`;
            } else if (
              call.data ===
              sdk.encodeAbiCall("getFeeDenom", ["address"], [account])
            ) {
              assert.equal(
                call.to,
                "0x0000000000000000000000000000000000001018",
              );
              assert.equal(params[1], "pending");
              result = state.malformedNative
                ? "0x"
                : abiResult(["string"], ["apax"]);
            } else {
              assert.deepEqual(params[1], {
                blockHash,
                requireCanonical: true,
              });
              if (call.to === "0x0000000000000000000000000000000000001016") {
                assert.equal(
                  call.data,
                  sdk.encodeAbiCall(
                    "getCap",
                    ["uint64", "address"],
                    [0x534f4c414e41n, sdk.SIDIORA_TOKEN],
                  ),
                );
                result = abiResult(
                  ["string", "uint256", "uint256", "uint256"],
                  [denom, 1_000_000n, 0n, 0n],
                );
              } else {
                assert.deepEqual(call, {
                  from: account,
                  to: "0x0000000000000000000000000000000000001018",
                  data: sdk.encodeAbiCall("setFeeDenom", ["string"], [denom]),
                });
                result = "0x";
              }
            }
            break;
          }
          default:
            throw new Error(`unexpected RPC method ${rpc.method}`);
        }
        reply({ jsonrpc: "2.0", id: rpc.id, result });
        return;
      }
      if (path.pathname === "/rpc" || path.pathname === "/rpc/ws") {
        outgoing.writeHead(404);
        outgoing.end();
        return;
      }
      assert.equal(incoming.method, "GET");
      assert.equal(incoming.headers["x-cosmos-block-height"], "100");
      outgoing.setHeader("x-cosmos-block-height", "100");
      if (path.pathname === "/cosmos/upgrade/v1beta1/applied_plan/v6.7") {
        reply({ height: "90" });
      } else if (path.pathname === "/cosmos/params/v1beta1/params") {
        assert.equal(path.searchParams.get("subspace"), "evm");
        const key = path.searchParams.get("key")!;
        const parameters: Record<string, unknown> = {
          KeyFeeTokenEnabled: state.nativeEnabled,
          KeyAllowedFeeDenoms: [
            { denom, rate: "2.5", rate_update_height: "99" },
          ],
          KeyMaxFeeTokenRateAge: "10",
        };
        assert.ok(Object.hasOwn(parameters, key));
        reply({
          param: { subspace: "evm", key, value: wire(parameters[key]) },
        });
      } else {
        assert.equal(
          path.pathname,
          `/cosmos/bank/v1beta1/denoms_metadata/${encodeURIComponent(denom)}`,
        );
        reply({
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
        });
      }
    } catch (error) {
      producerErrors.push(error);
      outgoing.destroy(
        error instanceof Error ? error : new Error(String(error)),
      );
    }
  });

  try {
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const bound = server.address();
    assert.ok(bound && typeof bound !== "string", "producer must be listening");
    const base = `http://127.0.0.1:${bound.port}`;
    for (const suffix of ["/rpc", "/rpc/ws"]) {
      const response = await fetch(base + suffix, { method: "POST", body: "{}" });
      assert.equal(response.status, 404);
    }
    assert.equal(counts.get("eth_call"), undefined);
    config.quoteUrl = `${base}/gas-station/quote`;
    let id = 0;
    const request = async (args: {
      readonly method: string;
      readonly params?: readonly unknown[];
    }): Promise<unknown> => {
      const rpcId = ++id;
      const response = await fetch(base, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: wire({ jsonrpc: "2.0", id: rpcId, ...args }),
        signal: AbortSignal.timeout(5000),
      });
      assert.equal(response.status, 200);
      const payload = object(await response.json());
      assert.equal(payload.id, rpcId);
      assert.equal(payload.jsonrpc, "2.0");
      assert.equal(payload.error, undefined);
      return payload.result;
    };
    const wallet = await import(
      new URL("modules/gas-station.ts", walletRoot).href
    );
    assert.equal(typeof wallet.gasStation, "function");
    const consumer = wallet.gasStation(
      { request },
      {
        chainId: config.chainId,
        sponsor: config.sponsor,
        paymaster,
        quoteUrl: config.quoteUrl,
      },
    );
    for (let iteration = 0; iteration < 2; iteration++) {
      const before = new Map(counts);
      assert.equal(await consumer.batchNonce(account), state.nonce);
      const signed = await consumer.requestQuote(quoteRequest(), { now });
      assert.deepEqual(signed.quote, produceQuote());
      assert.equal(signed.feeDenom, denom);
      assert.equal(
        signed.relayerSignature,
        sign(
          value(sdk.gasQuoteDigest(125n, account, produceQuote())),
          sponsorKey,
        ),
      );
      const native = await consumer.nativeFeePreference(account, base);
      assert.deepEqual(native, {
        kind: "native-fee-preference",
        state: "available",
        chainId: 125n,
        blockHash,
        blockNumber: 100n,
        denom,
        symbol: "SID",
        decimals: 6,
        rate: "2.5",
        rateUpdateHeight: 99n,
        currentDenom: "apax",
        call: {
          to: "0x0000000000000000000000000000000000001018",
          data: sdk.encodeAbiCall("setFeeDenom", ["string"], [denom]),
          value: 0n,
        },
      });
      const batch: SponsoredBatch = {
        chainId: 125n,
        account,
        nonce: state.nonce,
        calls,
        quote: signed.quote,
      };
      const accountSignature = sign(
        value(sdk.sponsoredBatchDigest(batch)),
        accountKey,
      );
      const authorization = value(
        sdk.assembleEip7702Authorization(
          config,
          account,
          0n,
          sign(
            value(
              sdk.eip7702AuthorizationDigest({
                chainId: 125n,
                address: paymaster,
                nonce: 0n,
              }),
            ),
            accountKey,
          ),
        ),
      );
      const submission = value(
        await sdk.submitSponsoredGasBatch(
          config,
          batch,
          accountSignature,
          signed.relayerSignature,
          authorization,
          { now },
        ),
      );
      assert.deepEqual(submission, {
        transactionHash: submissions.at(-1),
        rung: "submitted",
      });
      assert.equal(
        consumer.executeCall(
          batch,
          accountSignature,
          signed.relayerSignature,
          now,
        ).to,
        account,
      );
      for (const route of [
        "/gas-station/quote",
        "/gas-station/submit",
        "eth_getCode",
        "/cosmos/upgrade/v1beta1/applied_plan/v6.7",
        `/cosmos/bank/v1beta1/denoms_metadata/${encodeURIComponent(denom)}`,
      ]) {
        assert.equal(
          counts.get(route),
          (before.get(route) ?? 0) + 1,
          `${route} must be read afresh`,
        );
      }
      if (iteration === 0) {
        // The second observation uses changed producer state to exclude a cached result.
        state.nonce++;
        state.quoteNonce++;
        state.gasCost++;
      }
      assert.deepEqual(producerErrors, []);
    }
    assert.equal(issued.length, 2);
    assert.notEqual(submissions[0], submissions[1]);
    state.quoteMode = "wrong-signer";
    await assert.rejects(
      consumer.requestQuote(quoteRequest(), { now }),
      (error: unknown) => {
        assert.ok(error instanceof wallet.GasStationError);
        assert.equal(object(error).code, "invalid_signature");
        return true;
      },
    );
    state.quoteMode = "overflow";
    assert.deepEqual(
      await sdk.requestGasQuote(config, quoteRequest(), { now }),
      {
        ok: false,
        refusal: { code: "invalid_response", field: "tokenAmount" },
      },
    );
    await assert.rejects(
      consumer.requestQuote(quoteRequest(), { now }),
      (error: unknown) => {
        assert.ok(error instanceof wallet.GasStationError);
        assert.deepEqual(object(error).refusal, {
          code: "invalid_response",
          field: "tokenAmount",
        });
        return true;
      },
    );
    state.quoteMode = "valid";
    state.malformedNonce = true;
    await assert.rejects(consumer.batchNonce(account), (error: unknown) => {
      assert.ok(error instanceof wallet.GasStationError);
      assert.deepEqual(object(error).refusal, {
        code: "replay_state_unavailable",
        field: "nonce",
      });
      return true;
    });
    state.malformedNonce = false;
    state.delegated = false;
    await assert.rejects(consumer.batchNonce(account), (error: unknown) => {
      assert.deepEqual(object(error).refusal, {
        code: "incompatible_delegation",
        field: "delegation",
      });
      return true;
    });
    state.delegated = true;
    state.malformedNative = true;
    assert.deepEqual(await consumer.nativeFeePreference(account, base), {
      kind: "native-fee-preference",
      state: "unavailable",
      reason: "nativeFeeAbi",
    });
    state.malformedNative = false;
    state.nativeEnabled = false;
    assert.deepEqual(await consumer.nativeFeePreference(account, base), {
      kind: "native-fee-preference",
      state: "inactive",
      reason: "fee_token_disabled",
    });
    state.nativeEnabled = true;
    const signed = await consumer.requestQuote(quoteRequest(), { now });
    const batch: SponsoredBatch = {
      chainId: 125n,
      account,
      nonce: state.nonce,
      calls,
      quote: signed.quote,
    };
    const authorization = value(
      sdk.assembleEip7702Authorization(
        config,
        account,
        0n,
        sign(
          value(
            sdk.eip7702AuthorizationDigest({
              chainId: 125n,
              address: paymaster,
              nonce: 0n,
            }),
          ),
          accountKey,
        ),
      ),
    );
    state.malformedSubmission = true;
    assert.deepEqual(
      await sdk.submitSponsoredGasBatch(
        config,
        batch,
        sign(value(sdk.sponsoredBatchDigest(batch)), accountKey),
        signed.relayerSignature,
        authorization,
        { now },
      ),
      {
        ok: false,
        refusal: { code: "invalid_response", field: "transactionHash" },
      },
    );
    assert.deepEqual(producerErrors, []);
  } finally {
    const closed = once(server, "close");
    server.close();
    server.closeAllConnections();
    await closed;
    loader.deregister();
  }
}

test(proof_gas_station_producer_transport_consumer);
test(gas_nonce_root_route_uses_pending);
