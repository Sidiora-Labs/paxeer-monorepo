import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import React from "react";
import { Window } from "happy-dom";
import { act, create, type ReactTestRenderer } from "react-test-renderer";
import type { API } from "../packages/types/src";

const previousGateway = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://app.hyperpax.xyz";
const require = createRequire(import.meta.url);
const documentRuntime = new Window();
const previousStorage = Object.getOwnPropertyDescriptor(
  globalThis,
  "localStorage",
);
Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: documentRuntime.localStorage,
});
const previousReact = Object.getOwnPropertyDescriptor(globalThis, "React");
Object.defineProperty(globalThis, "React", {
  configurable: true,
  value: React,
});
const {
  Account,
  DefaultConfigStore,
  LocalStorageStore,
  SimpleDI,
}: typeof import("../packages/core/src") = require("../packages/core/src");
const {
  PaxeerNetworkProvider,
  swr,
  unstable_serialize,
}: typeof import("../packages/hooks/src") = require("../packages/hooks/src");
const {
  AccountStatusEnum,
}: typeof import("../packages/types/src") = require("../packages/types/src");
const {
  useAccountValue,
}: typeof import("../packages/ui-scaffold/src/components/subAccount/useAccountValue") = require("../packages/ui-scaffold/src/components/subAccount/useAccountValue");
const {
  DefaultEVMWalletAdapter,
}: typeof import("../packages/default-evm-adapter/src") = require("../packages/default-evm-adapter/src");
const {
  EthersProvider,
}: typeof import("../packages/web3-provider-ethers/src") = require("../packages/web3-provider-ethers/src");

const QUERY = "/v1/client/aggregate/positions";
const MAIN_ACCOUNT = "main-account";
const OTHER_ACCOUNT = "other-account";
const REST_FLUSHES = 8;

function position(account_id: string, unsettled_pnl: number): API.PositionExt {
  return {
    account_id,
    unsettled_pnl,
    symbol: "PERP_ETH_USDC",
    position_qty: 1,
    cost_position: 100,
    last_sum_unitary_funding: 0,
    pending_long_qty: 0,
    pending_short_qty: 0,
    settle_price: 100,
    average_open_price: 100,
    unrealized_pnl: unsettled_pnl,
    unrealized_pnl_ROI: 0,
    unsettled_pnl_ROI: 0,
    mark_price: 100,
    est_liq_price: null,
    timestamp: 0,
    mmr: 0.01,
    imr: 0.1,
    IMR_withdraw_orders: 0.1,
    MMR_with_orders: 0.01,
    pnl_24_h: 0,
    fee_24_h: 0,
    leverage: 10,
    notional: 100,
    mm: 1,
  };
}

async function mountAccountValue(
  data: API.PositionExt[] | undefined,
  readOnMount = false,
) {
  const configStore = new DefaultConfigStore({ networkId: "mainnet" });
  const keyStore = new LocalStorageStore("mainnet");
  const walletAdapters = [new DefaultEVMWalletAdapter(new EthersProvider())];
  const account = new Account(configStore, keyStore, walletAdapters);
  Object.assign(account.stateValue, {
    status: AccountStatusEnum.EnableTrading,
    accountId: MAIN_ACCOUNT,
    mainAccountId: MAIN_ACCOUNT,
  });
  const previousAccount = SimpleDI.get<Account>("account");
  SimpleDI.registerByName("account", account);
  const key = unstable_serialize([QUERY, MAIN_ACCOUNT]);
  const cache = new Map<
    string,
    {
      data?: API.PositionExt[];
      error?: Error;
      isLoading?: boolean;
      isValidating?: boolean;
    }
  >();
  cache.set(key, { data, isLoading: false, isValidating: false });
  const config = {
    provider: () => cache,
    revalidateOnMount: readOnMount,
  };
  const context = {
    configStore,
    keyStore,
    walletAdapters,
    networkId: "mainnet" as const,
  };
  const values: Record<string, number>[] = [];
  const rendersPerFlush: number[] = [];
  let rendersThisFlush = 0;
  let root: ReactTestRenderer;
  let publish: ReturnType<typeof swr.useSWRConfig>["mutate"];

  function AccountValueProbe({ version }: { version: number }) {
    void version;
    rendersThisFlush += 1;
    assert.ok(rendersThisFlush <= 8, "a flush has a bounded number of renders");
    publish = swr.useSWRConfig().mutate;
    values.push(useAccountValue(MAIN_ACCOUNT).accountValue);
    return null;
  }

  function tree(version: number) {
    return React.createElement(
      swr.SWRConfig,
      { value: config },
      React.createElement(
        PaxeerNetworkProvider,
        { value: context },
        React.createElement(AccountValueProbe, { version }),
      ),
    );
  }

  async function flush(action?: () => void | Promise<unknown>) {
    rendersThisFlush = 0;
    await act(async () => {
      await action?.();
    });
    rendersPerFlush.push(rendersThisFlush);
    return rendersThisFlush;
  }

  try {
    await flush(() => {
      root = create(tree(0));
    });
    return {
      values,
      cache,
      key,
      rendersPerFlush,
      flush,
      rerender: () => flush(() => root.update(tree(1))),
      update: (next: API.PositionExt[] | undefined) =>
        flush(() => publish(key, next, { revalidate: false })),
      close: async () => {
        await flush(() => root.unmount());
        SimpleDI.registerByName("account", previousAccount);
      },
    };
  } catch (error) {
    await act(async () => {
      root?.unmount();
    });
    SimpleDI.registerByName("account", previousAccount);
    throw error;
  }
}

async function assertAtRest(
  mounted: Awaited<ReturnType<typeof mountAccountValue>>,
) {
  const start = mounted.values.length;
  for (let flush = 0; flush < REST_FLUSHES; flush += 1) {
    assert.equal(await mounted.flush(), 0, `rest flush ${flush + 1}`);
  }
  assert.equal(mounted.values.length, start);
  assert.deepEqual(
    mounted.rendersPerFlush.slice(-REST_FLUSHES),
    Array(REST_FLUSHES).fill(0),
  );
  const value = mounted.values.at(-1);
  assert.equal(await mounted.rerender(), 1);
  assert.strictEqual(mounted.values.at(-1), value);
}

async function accountValueWithoutDataSettles() {
  const mounted = await mountAccountValue(undefined);
  try {
    assert.deepEqual(mounted.values.at(-1), {});
    await assertAtRest(mounted);
  } finally {
    await mounted.close();
  }
}

async function accountValueAfterFailedReadSettles() {
  const mounted = await mountAccountValue(undefined, true);
  try {
    for (let flush = 0; flush < REST_FLUSHES; flush += 1) await mounted.flush();
    const state = mounted.cache.get(mounted.key);
    assert.equal(state?.isValidating, false);
    assert.equal(state?.isLoading, false);
    assert.equal(state?.error?.message, "paxeerNetworkKeyPair is not defined");
    assert.equal(state?.data, undefined);
    assert.deepEqual(mounted.values.at(-1), {});
    await assertAtRest(mounted);
    assert.strictEqual(mounted.cache.get(mounted.key)?.error, state.error);
  } finally {
    await mounted.close();
  }
}

async function accountValueSumsPositionsPerAccountAndSettles() {
  const mounted = await mountAccountValue([
    position(MAIN_ACCOUNT, 0.1),
    position(MAIN_ACCOUNT, 0.2),
    position(OTHER_ACCOUNT, 8),
    position(OTHER_ACCOUNT, -3),
  ]);
  try {
    assert.deepEqual(mounted.values.at(-1), {
      [MAIN_ACCOUNT]: 0.3,
      [OTHER_ACCOUNT]: 5,
    });
    await assertAtRest(mounted);
    const previous = mounted.values.at(-1);
    await mounted.update([
      position(MAIN_ACCOUNT, 2),
      position(OTHER_ACCOUNT, 4),
    ]);
    assert.deepEqual(mounted.values.at(-1), {
      [MAIN_ACCOUNT]: 2,
      [OTHER_ACCOUNT]: 4,
    });
    assert.notStrictEqual(mounted.values.at(-1), previous);
    await assertAtRest(mounted);
  } finally {
    await mounted.close();
  }
}

async function accountValueEmptyReadsShareOneValue() {
  const mounted = await mountAccountValue(undefined);
  try {
    const empty = mounted.values.at(-1);
    await mounted.update([]);
    assert.strictEqual(mounted.values.at(-1), empty);
    await assertAtRest(mounted);
    await mounted.update([position(MAIN_ACCOUNT, 1)]);
    assert.deepEqual(mounted.values.at(-1), { [MAIN_ACCOUNT]: 1 });
    await mounted.update([]);
    assert.strictEqual(mounted.values.at(-1), empty);
    await assertAtRest(mounted);
  } finally {
    await mounted.close();
  }
}

test(accountValueWithoutDataSettles);
test(accountValueAfterFailedReadSettles);
test(accountValueSumsPositionsPerAccountAndSettles);
test(accountValueEmptyReadsShareOneValue);
test.after(async () => {
  if (previousGateway === undefined)
    delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  else process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = previousGateway;
  await documentRuntime.happyDOM.close();
  if (previousStorage)
    Object.defineProperty(globalThis, "localStorage", previousStorage);
  else Reflect.deleteProperty(globalThis, "localStorage");
  if (previousReact) Object.defineProperty(globalThis, "React", previousReact);
  else Reflect.deleteProperty(globalThis, "React");
});
