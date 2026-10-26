import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { test } from "node:test";
import { keccak_256 } from "@noble/hashes/sha3.js";

import type { Eip1193Requester } from "../src/account-derivation.js";
import {
  LAYERX_EXCHANGE_PRECOMPILE,
  PrecompileAbiError,
  exchangeCancelOrderCall,
  exchangeDepositMarginCall,
  exchangeDepositMarginTokenCall,
  exchangePlaceOrderCall,
  exchangeRequestSettlementCall,
  exchangeWithdrawMarginCall,
  sendExchangeCancelOrder,
  sendExchangeDepositMargin,
  sendExchangeDepositMarginToken,
  sendExchangePlaceOrder,
  sendExchangeRequestSettlement,
  sendExchangeWithdrawMargin,
  type PrecompileCall,
} from "../src/exchange.js";

const marketId = `0x${"11".repeat(32)}`;
const accountId = `0x${"22".repeat(32)}`;
const positionId = `0x${"33".repeat(32)}`;
const assetId = `0x${"44".repeat(32)}`;
const pointer = `0x${"55".repeat(20)}`;
const order = { marketId, accountId, side: 2, price: 18446744073709551618n, quantity: 5000n, timeInForce: 1 };
const selector = (signature: string): string =>
  `0x${Buffer.from(keccak_256(new TextEncoder().encode(signature))).subarray(0, 4).toString("hex")}`;
const word = (value: bigint): string => value.toString(16).padStart(64, "0");

function placeOrder_call_carries_the_account_id(): void {
  const expected = selector("placeOrder(bytes32,bytes32,uint8,uint256,uint256,uint8)") +
    marketId.slice(2) + accountId.slice(2) + word(2n) + word(order.price) + word(order.quantity) + word(1n);
  assert.deepEqual(exchangePlaceOrderCall(order), { to: LAYERX_EXCHANGE_PRECOMPILE, data: expected, value: 0n });
  assert.notEqual(expected.slice(0, 10), selector("placeOrder(bytes32,uint8,uint256,uint256,uint8)"));
  assert.throws(() => exchangePlaceOrderCall({ ...order, accountId: "0x22" }), PrecompileAbiError);
}

async function exchange_wallet_requests_are_type_2_on_chain_125(): Promise<void> {
  const evm = spawn(process.env.ANVIL_BIN ?? "anvil", ["--port", "0", "--accounts", "1", "--chain-id", "125"], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  evm.stderr.resume();
  try {
    const endpoint = await new Promise<string>((resolve, reject) => {
      let output = "";
      const timer = setTimeout(() => reject(new Error("local EVM startup timed out")), 15_000);
      const fail = (error?: Error | number | null) => {
        clearTimeout(timer);
        reject(error instanceof Error ? error : new Error("local EVM exited before readiness"));
      };
      evm.once("error", fail);
      evm.once("exit", fail);
      const ready = (chunk: Buffer) => {
        output = (output + chunk.toString("utf8")).slice(-8192);
        const listening = /Listening on ([^\s]+)/u.exec(output);
        if (listening?.[1] !== undefined) {
          clearTimeout(timer);
          evm.removeListener("error", fail);
          evm.removeListener("exit", fail);
          evm.stdout.removeListener("data", ready);
          evm.stdout.resume();
          resolve(`http://${listening[1]}`);
        }
      };
      evm.stdout.on("data", ready);
    });
    let id = 0;
    const sent: Parameters<Eip1193Requester["request"]>[0][] = [];
    const wallet: Eip1193Requester = {
      async request(request) {
        if (request.method === "eth_sendTransaction") sent.push(request);
        const requestId = ++id;
        const response = await fetch(endpoint, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ jsonrpc: "2.0", id: requestId, ...request }),
          signal: AbortSignal.timeout(15_000),
        });
        assert.equal(response.status, 200);
        const payload = await response.json() as Record<string, unknown>;
        assert.equal(payload.jsonrpc, "2.0");
        assert.equal(payload.id, requestId);
        assert.equal(payload.error, undefined, JSON.stringify(payload.error));
        return payload.result;
      },
    };
    assert.equal(await wallet.request({ method: "eth_chainId", params: [] }), "0x7d");
    const accounts = await wallet.request({ method: "eth_accounts", params: [] });
    assert.ok(Array.isArray(accounts) && typeof accounts[0] === "string");
    const from: string = accounts[0];
    const settlement = exchangeRequestSettlementCall(positionId);
    assert.deepEqual(settlement, {
      to: LAYERX_EXCHANGE_PRECOMPILE,
      data: selector("requestSettlement(bytes32)") + positionId.slice(2),
      value: 0n,
    });
    const sends: readonly (readonly [PrecompileCall, () => Promise<string>])[] = [
      [settlement, () => sendExchangeRequestSettlement(wallet, from, positionId)],
      [exchangePlaceOrderCall(order), () => sendExchangePlaceOrder(wallet, from, order)],
      [exchangeCancelOrderCall(positionId), () => sendExchangeCancelOrder(wallet, from, positionId)],
      [exchangeDepositMarginCall(accountId, 255n), () => sendExchangeDepositMargin(wallet, from, accountId, 255n)],
      [exchangeDepositMarginTokenCall(pointer, 9n, accountId), () => sendExchangeDepositMarginToken(wallet, from, pointer, 9n, accountId)],
      [exchangeWithdrawMarginCall(accountId, assetId, 10n), () => sendExchangeWithdrawMargin(wallet, from, accountId, assetId, 10n)],
    ];
    for (const [call, send] of sends) {
      const before = sent.length;
      const hash = await send();
      assert.match(hash, /^0x[0-9a-f]{64}$/u);
      assert.equal(sent.length, before + 1);
      assert.deepEqual(sent[before], {
        method: "eth_sendTransaction",
        params: [{ from, to: call.to, data: call.data, value: `0x${call.value.toString(16)}`, type: "0x2", chainId: "0x7d" }],
      });
      const transaction = await wallet.request({ method: "eth_getTransactionByHash", params: [hash] });
      assert.ok(typeof transaction === "object" && transaction !== null);
      assert.ok("type" in transaction && "chainId" in transaction && "to" in transaction && "input" in transaction && "value" in transaction);
      assert.equal(transaction.type, "0x2");
      assert.equal(transaction.chainId, "0x7d");
      assert.equal(transaction.to, LAYERX_EXCHANGE_PRECOMPILE);
      assert.equal(transaction.input, call.data);
      assert.equal(transaction.value, `0x${call.value.toString(16)}`);
    }
    assert.equal(sent.length, sends.length);
  } finally {
    if (evm.exitCode === null && evm.signalCode === null && evm.pid !== undefined) {
      const closed = once(evm, "close");
      evm.kill("SIGTERM");
      await closed;
    }
  }
}

test(placeOrder_call_carries_the_account_id);
test(exchange_wallet_requests_are_type_2_on_chain_125);
