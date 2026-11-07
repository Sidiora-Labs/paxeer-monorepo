import assert from "node:assert/strict";
import { exchangeOrderAbi } from "./exchange-abi.js";

import {
  LAYERX_EXCHANGE_PRECOMPILE,
  abiSelector,
  exchangeRequestSettlementCall,
  precompileTransactionRequest,
  type exchangePlaceOrderCall,
  type PrecompileCall,
  type WalletRequest,
} from "../../src/index.js";

export function consumeExchangeCall(
  call: PrecompileCall,
  signature: string,
  body: string,
  sender: string,
): WalletRequest {
  const request = precompileTransactionRequest(sender, call);
  assert.deepEqual(call, {
    to: LAYERX_EXCHANGE_PRECOMPILE,
    data: abiSelector(signature) + body,
    value: 0n,
  });
  assert.deepEqual(request, {
    method: "eth_sendTransaction",
    params: [{ from: sender, to: call.to, data: call.data, value: "0x0", type: "0x2", chainId: "0x7d" }],
  });
  return request;
}

/** Consumes the emitted settlement request and returns it for inspection. */
export function consumeExchangeSettlementRequest(
  request: Readonly<{ method: string; params?: readonly unknown[] }>,
  sender: string,
  positionId: string,
): Readonly<{ method: string; params?: readonly unknown[] }> {
  const call = exchangeRequestSettlementCall(positionId);
  assert.deepEqual(request, consumeExchangeCall(call, "requestSettlement(bytes32)", positionId.slice(2), sender));
  return request;
}

export function consumeExchangePlaceOrderCall(
  actual: PrecompileCall,
  order: Parameters<typeof exchangePlaceOrderCall>[0],
): void {
  const word = (value: bigint | number): string => BigInt(value).toString(16).padStart(64, "0");
  const selector = abiSelector(exchangeOrderAbi("accountId" in order).signature);
  assert.deepEqual(actual, {
    to: "0x0000000000000000000000000000000000001015",
    data: selector + order.marketId.slice(2).toLowerCase() + (order.accountId === undefined ? "" : order.accountId.slice(2).toLowerCase())
      + word(order.side) + word(order.price) + word(order.quantity) + word(order.timeInForce),
    value: 0n,
  });
}

