import assert from "node:assert/strict";
import { test, type TestContext } from "node:test";
import { isMainThread, parentPort, Worker } from "node:worker_threads";

async function exerciseDepositQuantityKeys() {
  const chainOrigin = "https://api1.mainnet-beta.paxeer.network";
  Object.assign(process.env, {
    NEXT_PUBLIC_PAXEER_RPC_URLS: chainOrigin,
    NEXT_PUBLIC_PERPS_GATEWAY_URL: chainOrigin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: chainOrigin,
    NEXT_PUBLIC_PAXSCAN_API_URL: `${chainOrigin}/api/v2`,
    NEXT_PUBLIC_PAXSCAN_WS_URL: chainOrigin.replace("https:", "wss:"),
  });
  const { Window } = await import("happy-dom");
  const dom = new Window();
  for (const [name, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    localStorage: dom.localStorage,
    Event: dom.Event,
    CustomEvent: dom.CustomEvent,
    HTMLElement: dom.HTMLElement,
    HTMLFormElement: dom.HTMLFormElement,
    HTMLInputElement: dom.HTMLInputElement,
    HTMLSelectElement: dom.HTMLSelectElement,
    Element: dom.Element,
    Node: dom.Node,
    MutationObserver: dom.MutationObserver,
    ResizeObserver: dom.ResizeObserver,
    getComputedStyle: dom.getComputedStyle.bind(dom),
    requestAnimationFrame: dom.requestAnimationFrame.bind(dom),
    cancelAnimationFrame: dom.cancelAnimationFrame.bind(dom),
  }))
    Object.defineProperty(globalThis, name, { value, configurable: true });
  for (const name of Object.getOwnPropertyNames(dom)) {
    if (/^(HTML|SVG|Document|Node|Element|DOM)|Event$|Observer$/.test(name)) {
      Object.defineProperty(globalThis, name, {
        value: (dom as unknown as Record<string, unknown>)[name],
        configurable: true,
        writable: true,
      });
    }
  }
  await import("fake-indexeddb/auto");
  const { indexedDB, IDBKeyRange } = await import("fake-indexeddb");
  Object.assign(globalThis, { indexedDB, IDBKeyRange });
  const React = await import("react");
  Object.assign(globalThis, { React });
  const { createElement, useCallback, useState } = React;
  const { act, render, fireEvent } = await import("@testing-library/react");
  const { QuantityInput } =
    await import("../packages/ui-transfer/src/components/quantityInput/index");
  const { AvailableQuantity } =
    await import("../packages/ui-transfer/src/components/availableQuantity/index");
  const { useDepositQuantityInput } =
    await import("../packages/ui-transfer/src/components/depositForm/hooks/useDepositQuantityInput");
  const { useMaxDepositAmount } =
    await import("../packages/ui-transfer/src/components/depositForm/hooks/useMaxDepositAmount");
  const token = { symbol: "USDL", precision: 2, user_max_qty: -1 };
  const answers: Array<() => void> = [];
  const changes: string[] = [];
  let confirmedQuantity = "";
  let latest!: ReturnType<typeof useDepositQuantityInput>;
  let maxDepositAmount = "";

  function Form() {
    const [confirmed, setConfirmed] = useState("");
    confirmedQuantity = confirmed;
    const publish = useCallback((value: string) => {
      changes.push(value);
      answers.push(() => setConfirmed(value));
    }, []);
    latest = useDepositQuantityInput(publish);
    maxDepositAmount = useMaxDepositAmount(token, "1234");
    return createElement(
      React.Fragment,
      null,
      createElement(QuantityInput, {
        token,
        value: latest.quantity,
        onValueChange: latest.setQuantity,
      }),
      createElement(AvailableQuantity, {
        token,
        quantity: latest.quantity,
        maxQuantity: maxDepositAmount,
        onClick: () => latest.setQuantity(maxDepositAmount),
      }),
    );
  }

  let renderer!: ReturnType<typeof render>;
  await act(async () => {
    renderer = render(createElement(Form));
  });
  const input = () => {
    const element = renderer.container.querySelector<HTMLInputElement>("input");
    assert.ok(element);
    return element;
  };
  const text = () => input().value;
  function change(value: string) {
    fireEvent.change(input(), {
      target: { value, selectionStart: value.length },
    });
  }
  async function key(value: string) {
    await act(async () => {
      change(text() + value);
    });
  }
  async function answer(index: number) {
    assert.ok(answers[index]);
    await act(async () => {
      answers[index]();
    });
  }

  try {
    await key("1");
    assert.equal(text(), "1");
    await key("2");
    assert.equal(confirmedQuantity, "");
    assert.equal(text(), "12");
    assert.deepEqual(changes, ["1", "12"]);

    await answer(0);
    assert.equal(confirmedQuantity, "1");
    assert.equal(text(), "12");
    await key("3");
    assert.equal(text(), "123");
    assert.equal(latest.quantity, "123");

    await answer(2);
    assert.equal(confirmedQuantity, "123");
    await answer(1);
    assert.equal(confirmedQuantity, "12");
    assert.equal(text(), "123");
    await key("4");
    assert.equal(text(), "1,234");
    assert.equal(changes.at(-1), "1234");

    await act(async () => {
      fireEvent.click(renderer.getByText("Max"));
    });
    assert.equal(maxDepositAmount, "1234");
    assert.equal(text(), "1,234");
    await key("5");
    assert.equal(text(), "12,345");
    assert.equal(changes.at(-1), "12345");
    assert.ok(Number(latest.quantity) > Number(maxDepositAmount));

    await act(async () => {
      latest.setQuantity("12.34");
    });
    await key("5");
    assert.equal(text(), "12.34");
    assert.equal(changes.at(-1), "12.34");

    await act(async () => {
      latest.setQuantity("");
    });
    assert.equal(text(), "");
    await key(".");
    assert.equal(text(), "0.");
    await key("1");
    await key("0");
    assert.equal(text(), "0.10");
    assert.equal(changes.at(-1), "0.10");
    await answer(3);
    assert.equal(confirmedQuantity, "1234");
    assert.equal(text(), "0.10");

    await act(async () => {
      change("");
    });
    assert.equal(text(), "");
    assert.equal(changes.at(-1), "");
  } finally {
    await act(async () => {
      renderer.unmount();
    });
    await dom.happyDOM.close();
  }
}

async function depositQuantityKeepsKeysAcrossStateAnswers(t: TestContext) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
  });
  t.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(
        new Error(
          `Deposit quantity assertions exited before completion: ${code}`,
        ),
      ),
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

if (isMainThread) test(depositQuantityKeepsKeysAcrossStateAnswers);
else
  void exerciseDepositQuantityKeys().then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
