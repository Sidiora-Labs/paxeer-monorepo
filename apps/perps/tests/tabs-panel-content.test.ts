import assert from "node:assert/strict";
import { test, type TestContext } from "node:test";
import {
  isMainThread,
  parentPort,
  Worker,
  workerData,
} from "node:worker_threads";

async function exerciseTabs(scenario: string) {
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
    HTMLInputElement: dom.HTMLInputElement,
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
  const React = await import("react");
  Object.assign(globalThis, { React });
  const { createElement: h, useState, useLayoutEffect } = React;
  const { flushSync } = await import("react-dom");
  const { act, render, fireEvent, cleanup } =
    await import("@testing-library/react");
  const { Tabs, TabPanel } = await import("../packages/ui/src/tabs/tabs");
  const { PaxeerNetworkThemeProvider } =
    await import("../packages/ui/src/provider/paxeerNetworkThemeProvider");

  async function mount(Component: React.ComponentType) {
    let renderer!: ReturnType<typeof render>;
    await act(async () => {
      renderer = render(h(PaxeerNetworkThemeProvider, null, h(Component)));
    });
    return renderer;
  }

  function input(container: HTMLElement) {
    const element = container.querySelector<HTMLInputElement>("input");
    assert.ok(element);
    return element;
  }

  async function sameCommit() {
    let setText!: React.Dispatch<React.SetStateAction<string>>;
    const committedText: string[] = [];
    function Parent() {
      const [text, update] = useState("before");
      setText = update;
      useLayoutEffect(() => {
        committedText.push(document.querySelector("input")!.value);
      }, [text]);
      return h(
        Tabs,
        { defaultValue: "first" },
        h(
          TabPanel,
          { value: "first", title: "First" },
          h("input", {
            value: text,
            onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
              update(event.target.value),
          }),
        ),
      );
    }
    const renderer = await mount(Parent);
    flushSync(() => setText("after"));
    assert.equal(input(renderer.container).value, "after");
    assert.deepEqual(committedText, ["before", "after"]);
  }

  async function consecutiveChanges() {
    function Parent() {
      const [text, setText] = useState("");
      return h(
        Tabs,
        { defaultValue: "first" },
        h(
          TabPanel,
          { value: "first", title: "First" },
          h("input", {
            value: text,
            onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
              setText(event.target.value),
          }),
        ),
      );
    }
    const renderer = await mount(Parent);
    const element = input(renderer.container);
    flushSync(() =>
      fireEvent.change(element, { target: { value: element.value + "a" } }),
    );
    assert.equal(element.value, "a");
    flushSync(() =>
      fireEvent.change(element, { target: { value: element.value + "b" } }),
    );
    assert.equal(element.value, "ab");
  }

  async function oneRender() {
    let setText!: React.Dispatch<React.SetStateAction<string>>;
    let renders = 0;
    function Content({ text }: { text: string }) {
      renders++;
      return h("input", { value: text, readOnly: true });
    }
    function Parent() {
      const [text, update] = useState("before");
      setText = update;
      return h(
        Tabs,
        { defaultValue: "first" },
        h(
          TabPanel,
          {
            value: "first",
            title: h("span", null, text),
            classNames: { trigger: "panel-trigger" },
          },
          h(Content, { text }),
        ),
      );
    }
    const renderer = await mount(Parent);
    const before = renders;
    flushSync(() => setText("after"));
    assert.equal(input(renderer.container).value, "after");
    assert.equal(renders, before + 1);
    await act(async () => {});
    assert.equal(renders, before + 1);
    assert.equal(renderer.getByRole("tab").textContent, "after");
  }

  async function wrappedPanel() {
    let setText!: React.Dispatch<React.SetStateAction<string>>;
    function Panel() {
      const [text, update] = useState("before");
      setText = update;
      return h(
        TabPanel,
        { value: "wrapped", title: "Wrapped" },
        h("input", { value: text, readOnly: true }),
      );
    }
    function Parent() {
      return h(Tabs, { defaultValue: "wrapped" }, h(Panel));
    }
    const renderer = await mount(Parent);
    assert.equal(renderer.getByRole("tab").textContent, "Wrapped");
    flushSync(() => setText("after"));
    assert.equal(input(renderer.container).value, "after");
    assert.equal(
      input(renderer.container).parentElement,
      renderer.getByRole("tabpanel"),
    );
  }

  async function dynamicPanels() {
    let setAdded!: React.Dispatch<React.SetStateAction<boolean>>;
    let setTitle!: React.Dispatch<React.SetStateAction<string>>;
    function Parent() {
      const [added, updateAdded] = useState(false);
      const [title, updateTitle] = useState("First");
      setAdded = updateAdded;
      setTitle = updateTitle;
      return h(
        Tabs,
        { defaultValue: "first" },
        h(TabPanel, { value: "first", title, key: "first" }, "first content"),
        h(
          TabPanel,
          { value: "second", title: "Second", key: "second" },
          "second content",
        ),
        added &&
          h(
            TabPanel,
            { value: "third", title: "Third", key: "third" },
            "third content",
          ),
      );
    }
    const renderer = await mount(Parent);
    const titles = () =>
      renderer.getAllByRole("tab").map((tab) => tab.textContent);
    const panels = () =>
      Array.from(
        renderer.container.querySelectorAll<HTMLElement>('[role="tabpanel"]'),
      );
    assert.deepEqual(titles(), ["First", "Second"]);
    await act(async () => {
      setAdded(true);
    });
    assert.deepEqual(titles(), ["First", "Second", "Third"]);
    await act(async () => {
      setTitle("Renamed");
    });
    assert.deepEqual(titles(), ["Renamed", "Second", "Third"]);
    assert.deepEqual(
      panels().map((panel) => panel.getAttribute("aria-labelledby")),
      renderer.getAllByRole("tab").map((tab) => tab.id),
    );
    await act(async () => {
      setAdded(false);
    });
    assert.deepEqual(titles(), ["Renamed", "Second"]);
    assert.equal(panels().length, 2);
    assert.equal(
      renderer.container.textContent?.includes("third content"),
      false,
    );
  }

  async function selectionAndMarkup() {
    let setSelected!: React.Dispatch<React.SetStateAction<string>>;
    const selections: string[] = [];
    function panels() {
      return [
        h(
          TabPanel,
          {
            key: "first",
            value: "first",
            title: "First",
            testid: "first-trigger",
            className: "panel-content",
            classNames: { trigger: "panel-trigger" },
            style: { opacity: 0.9 },
          },
          h("input", { defaultValue: "first" }),
        ),
        h(
          TabPanel,
          { key: "second", value: "second", title: "Second" },
          h("input", { defaultValue: "second" }),
        ),
      ];
    }
    function Parent() {
      const [selected, update] = useState("first");
      setSelected = update;
      return h(
        Tabs,
        {
          id: "workspace-tabs",
          value: selected,
          dir: "rtl",
          variant: "text",
          size: "sm",
          showScrollIndicator: true,
          leading: h("span", { "data-testid": "leading" }, "Leading"),
          trailing: h("span", { "data-testid": "trailing" }, "Trailing"),
          classNames: {
            tabsContent: "shared-content",
            trigger: "shared-trigger",
            tabsList: "custom-list",
            tabsListContainer: "custom-container",
            scrollIndicator: "custom-scroll",
          },
          onValueChange: (value) => {
            selections.push(value);
            update(value);
          },
        },
        ...panels(),
      );
    }
    const renderer = await mount(Parent);
    const root = renderer.container.querySelector("#workspace-tabs")!;
    const shell = root.firstElementChild!;
    assert.equal(root.getAttribute("dir"), "rtl");
    assert.ok(shell.classList.contains("custom-container"));
    assert.equal(shell.firstElementChild, renderer.getByTestId("leading"));
    assert.equal(shell.lastElementChild, renderer.getByTestId("trailing"));
    assert.ok(shell.querySelector(".oui-scroll-indicator.custom-scroll"));
    assert.ok(renderer.getByRole("tablist").classList.contains("custom-list"));
    const first = renderer.getByTestId("first-trigger");
    assert.ok(first.classList.contains("shared-trigger"));
    assert.ok(first.classList.contains("panel-trigger"));
    const contents = Array.from(
      root.querySelectorAll<HTMLElement>('[role="tabpanel"]'),
    );
    assert.equal(root.children.length, 3);
    assert.equal(root.children[1], contents[0]);
    assert.equal(root.children[2], contents[1]);
    assert.ok(contents[0].classList.contains("oui-tabs-content"));
    assert.ok(contents[0].classList.contains("shared-content"));
    assert.ok(contents[0].classList.contains("panel-content"));
    assert.equal(contents[0].style.opacity, "0.9");
    assert.equal(contents[0].getAttribute("tabindex"), "0");
    assert.equal(first.getAttribute("aria-controls"), contents[0].id);
    assert.equal(contents[0].getAttribute("aria-labelledby"), first.id);
    assert.equal(contents[1].hidden, true);
    assert.equal(contents[1].childNodes.length, 0);
    assert.equal(input(renderer.container).value, "first");
    flushSync(() => setSelected("second"));
    assert.equal(input(renderer.container).value, "second");
    assert.equal(contents[0].hidden, true);
    assert.equal(contents[0].childNodes.length, 0);
    assert.equal(contents[1].hidden, false);
    await act(async () => {
      first.focus();
    });
    assert.deepEqual(selections, ["first"]);
    assert.equal(input(renderer.container).value, "first");
    renderer.unmount();
    function Uncontrolled() {
      return h(Tabs, { defaultValue: "first" }, ...panels());
    }
    const uncontrolled = await mount(Uncontrolled);
    await act(async () => {
      fireEvent.mouseDown(uncontrolled.getByRole("tab", { name: "Second" }), {
        button: 0,
        ctrlKey: false,
      });
    });
    assert.equal(input(uncontrolled.container).value, "second");
    assert.equal(
      uncontrolled
        .getByRole("tab", { name: "Second" })
        .getAttribute("aria-selected"),
      "true",
    );
  }

  async function omittedContent() {
    let setVisible!: React.Dispatch<React.SetStateAction<boolean>>;
    let renders = 0;
    function Content() {
      renders++;
      return h("input", { defaultValue: "content" });
    }
    function Parent() {
      const [visible, update] = useState(false);
      setVisible = update;
      return h(
        Tabs,
        { defaultValue: "first", contentVisible: visible },
        h(TabPanel, { value: "first", title: "First" }, h(Content)),
      );
    }
    const renderer = await mount(Parent);
    const trigger = renderer.getByRole("tab");
    assert.equal(renderer.container.querySelector('[role="tabpanel"]'), null);
    assert.equal(renders, 0);
    flushSync(() => setVisible(true));
    assert.equal(input(renderer.container).value, "content");
    assert.equal(renders, 1);
    assert.equal(renderer.getByRole("tab"), trigger);
    flushSync(() => setVisible(false));
    assert.equal(renderer.container.querySelector('[role="tabpanel"]'), null);
    assert.equal(renderer.container.querySelector("input"), null);
    assert.equal(renderer.getByRole("tab"), trigger);
  }

  async function inputIdentity() {
    let setRevision!: React.Dispatch<React.SetStateAction<number>>;
    function Content() {
      const [text, setText] = useState("initial");
      return h("input", {
        value: text,
        onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
          setText(event.target.value),
      });
    }
    function Parent() {
      const [revision, update] = useState(0);
      setRevision = update;
      return h(
        Tabs,
        {
          defaultValue: "first",
          classNames: { tabsContent: "shared-content" },
        },
        h(
          TabPanel,
          {
            value: "first",
            title: h("span", null, `First ${revision}`),
            style: { opacity: 1 },
            classNames: { trigger: "panel-trigger" },
          },
          h(Content),
        ),
      );
    }
    const renderer = await mount(Parent);
    const element = input(renderer.container);
    element.focus();
    flushSync(() =>
      fireEvent.change(element, { target: { value: "typed text" } }),
    );
    element.setSelectionRange(1, 6, "backward");
    for (let revision = 1; revision <= 10; revision++) {
      flushSync(() => setRevision(revision));
      assert.equal(input(renderer.container), element);
      assert.equal(document.activeElement, element);
      assert.equal(element.value, "typed text");
      assert.equal(element.selectionStart, 1);
      assert.equal(element.selectionEnd, 6);
      assert.equal(element.selectionDirection, "backward");
    }
    await act(async () => {});
    assert.equal(input(renderer.container), element);
    assert.equal(document.activeElement, element);
    assert.equal(renderer.getByRole("tab").textContent, "First 10");
  }

  const scenarios = {
    sameCommit,
    consecutiveChanges,
    oneRender,
    wrappedPanel,
    dynamicPanels,
    selectionAndMarkup,
    omittedContent,
    inputIdentity,
  };
  assert.ok(scenario in scenarios);
  try {
    await scenarios[scenario as keyof typeof scenarios]();
  } finally {
    cleanup();
    await dom.happyDOM.close();
  }
}

async function runScenario(t: TestContext, scenario: string) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
    workerData: scenario,
  });
  t.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(
        new Error(`Tab panel assertions exited before completion: ${code}`),
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

async function tabsPanelCommitsParentValue(t: TestContext) {
  await runScenario(t, "sameCommit");
}
async function tabsPanelKeepsConsecutiveChanges(t: TestContext) {
  await runScenario(t, "consecutiveChanges");
}
async function tabsPanelRendersOncePerParentChange(t: TestContext) {
  await runScenario(t, "oneRender");
}
async function tabsPanelSupportsComponentWrappers(t: TestContext) {
  await runScenario(t, "wrappedPanel");
}
async function tabsPanelSupportsDynamicPanels(t: TestContext) {
  await runScenario(t, "dynamicPanels");
}
async function tabsPanelPreservesSelectionAndMarkup(t: TestContext) {
  await runScenario(t, "selectionAndMarkup");
}
async function tabsPanelCanOmitContent(t: TestContext) {
  await runScenario(t, "omittedContent");
}
async function tabsPanelPreservesInputIdentity(t: TestContext) {
  await runScenario(t, "inputIdentity");
}

if (isMainThread) {
  test(tabsPanelCommitsParentValue);
  test(tabsPanelKeepsConsecutiveChanges);
  test(tabsPanelRendersOncePerParentChange);
  test(tabsPanelSupportsComponentWrappers);
  test(tabsPanelSupportsDynamicPanels);
  test(tabsPanelPreservesSelectionAndMarkup);
  test(tabsPanelCanOmitContent);
  test(tabsPanelPreservesInputIdentity);
} else {
  void exerciseTabs(workerData).then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
}
