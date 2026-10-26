import { strict as assert } from "node:assert";
import test from "node:test";
import {
  type Abi,
  ContractFunctionExecutionError,
  ContractFunctionRevertedError,
  encodeErrorResult,
} from "viem";

import {
  classifyError,
  PRECOMPILE_ERROR_CODES,
  type PrecompileErrorName,
} from "../src/core/errors/classify";
import { ERROR_CATALOG } from "../src/core/errors/catalog";
import type { ErrorCode } from "../src/core/errors/types";
import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { pecorAbi } from "../src/core/network/spot_abi/PECOR";

function assertRevertMapping(abi: Abi, errorName: PrecompileErrorName, expectedCode: ErrorCode) {
  assert.equal(PRECOMPILE_ERROR_CODES[errorName].code, expectedCode, errorName);
  const data = encodeErrorResult({ abi, errorName });
  const context = "trade/classify-limits";
  const errors = [
    { code: 3, message: "execution reverted", data },
    new ContractFunctionExecutionError(
      new ContractFunctionRevertedError({ abi, data, functionName: "call" }),
      {
        abi,
        functionName: "call",
        args: [],
        contractAddress: "0x0000000000000000000000000000000000001015",
      },
    ),
  ];

  for (const raw of errors) {
    const result = classifyError(raw, context);
    assert.equal(result.code, expectedCode, errorName);
    assert.equal(result.title, ERROR_CATALOG[expectedCode].title, errorName);
    assert.equal(result.message, ERROR_CATALOG[expectedCode].message, errorName);
    assert.equal(result.context, context, errorName);
    assert.deepEqual(result.fingerprint, [expectedCode, context, errorName]);
  }
}

function limitRevertsMapped() {
  const cases = [
    ["PerpsTickSize", "VALIDATION_PRECISION"],
    ["PerpsLotSize", "VALIDATION_PRECISION"],
    ["PerpsLeverageTooHigh", "VALIDATION_ABOVE_MAX"],
    ["PerpsMinOrderValue", "VALIDATION_BELOW_MIN"],
    ["PerpsMaxOrderValue", "VALIDATION_ABOVE_MAX"],
  ] as const;

  for (const [errorName, expectedCode] of cases) {
    assertRevertMapping(PerpsPrecompileABI, errorName, expectedCode);
  }

  const unknown = classifyError({ code: 3, message: "execution reverted", data: "0x12345678" });
  assert.equal(unknown.code, "RPC_REVERT");
  assert.equal(unknown.message, ERROR_CATALOG.RPC_REVERT.message);
}

function haltedRevertsMapped() {
  assertRevertMapping(PerpsPrecompileABI, "PerpsMarketDisabled", "TRADE_MARKET_HALTED");
  assertRevertMapping(pecorAbi, "TokenPaused", "TRADE_MARKET_HALTED");

  assert.equal(PRECOMPILE_ERROR_CODES.PerpsPaused.code, "RPC_REVERT");
  const paused = classifyError({
    code: 3,
    message: "execution reverted",
    data: encodeErrorResult({ abi: PerpsPrecompileABI, errorName: "PerpsPaused" }),
  });
  assert.equal(paused.code, "RPC_REVERT");
  assert.equal(paused.message, "The contract rejected this transaction with PerpsPaused.");
}

test(limitRevertsMapped);
test(haltedRevertsMapped);
