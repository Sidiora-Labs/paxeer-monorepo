import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import {
  isMainThread,
  parentPort,
  Worker,
  workerData,
} from "node:worker_threads";

let sumPositionMaintenanceMargin: typeof import("../packages/hooks/src/paxeer-network/useMarginRatio").sumPositionMaintenanceMargin;

async function runCaseInWorker(
  caseFunction: (context?: TestContext) => Promise<void>,
  context?: TestContext,
) {
  assert.ok(context);
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
    workerData: caseFunction.name,
  });
  context.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(new Error(`Margin assertions exited before completion: ${code}`)),
    );
    worker.once("message", (message) => {
      try {
        assert.equal(message, "assertions-passed");
        resolve();
      } catch (error) {
        reject(error);
      }
    });
  });
}

async function aRowWithoutTheFieldIsPassedOver(context?: TestContext) {
  if (isMainThread) {
    return runCaseInWorker(aRowWithoutTheFieldIsPassedOver, context);
  }
  const rows = [{}, { mm: 2.5 }];
  let sum: number | null = null;
  assert.doesNotThrow(() => {
    sum = sumPositionMaintenanceMargin(rows);
  });
  assert.equal(sum, 2.5);
}

async function aValueThatIsNoFiniteNumberIsPassedOver(context?: TestContext) {
  if (isMainThread) {
    return runCaseInWorker(aValueThatIsNoFiniteNumberIsPassedOver, context);
  }
  assert.equal(
    sumPositionMaintenanceMargin([
      { mm: null },
      { mm: undefined },
      { mm: NaN },
      { mm: Infinity },
      { mm: -Infinity },
      { mm: "2.5" },
      { mm: 4 },
    ]),
    4,
  );
}

async function whenNoRowGivesAValueTheSumIsNull(context?: TestContext) {
  if (isMainThread) {
    return runCaseInWorker(whenNoRowGivesAValueTheSumIsNull, context);
  }
  assert.equal(sumPositionMaintenanceMargin(null), null);
  assert.equal(sumPositionMaintenanceMargin(), null);
  assert.equal(sumPositionMaintenanceMargin([]), null);
  assert.equal(
    sumPositionMaintenanceMargin([
      {},
      { mm: null },
      { mm: undefined },
      { mm: NaN },
      { mm: Infinity },
      { mm: -Infinity },
      { mm: "2.5" },
    ]),
    null,
  );
}

async function valuesAddUpAsDecimals(context?: TestContext) {
  if (isMainThread) {
    return runCaseInWorker(valuesAddUpAsDecimals, context);
  }
  assert.equal(sumPositionMaintenanceMargin([{ mm: 0.1 }, { mm: 0.2 }]), 0.3);
  assert.equal(sumPositionMaintenanceMargin([{ mm: 1.25 }, { mm: 2.5 }]), 3.75);
  assert.equal(sumPositionMaintenanceMargin([{ mm: 0 }]), 0);
}

const cases = {
  aRowWithoutTheFieldIsPassedOver,
  aValueThatIsNoFiniteNumberIsPassedOver,
  whenNoRowGivesAValueTheSumIsNull,
  valuesAddUpAsDecimals,
};

if (isMainThread) {
  test(aRowWithoutTheFieldIsPassedOver);
  test(aValueThatIsNoFiniteNumberIsPassedOver);
  test(whenNoRowGivesAValueTheSumIsNull);
  test(valuesAddUpAsDecimals);
} else {
  void (async () => {
    const chainOrigin = "https://api1.mainnet-beta.paxeer.network";
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = chainOrigin;
    process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = chainOrigin;
    ({ sumPositionMaintenanceMargin } =
      await import("../packages/hooks/src/paxeer-network/useMarginRatio"));
    await cases[workerData as keyof typeof cases]();
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  })();
}
