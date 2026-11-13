import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { once } from "node:events";
import { createRequire } from "node:module";
import { test, after } from "node:test";
import { Worker, isMainThread, parentPort } from "node:worker_threads";
import { fileURLToPath } from "node:url";
import React, {
  createElement as h,
  type ReactNode,
  type ReactElement,
} from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiContext } from "wagmi";
import { injected } from "wagmi/connectors";
import { connect, disconnect, getAccount } from "wagmi/actions";
import { privateKeyToAccount } from "viem/accounts";
import type { AuthChangeEvent, Session } from "@supabase/supabase-js";

// In-memory host for the real React reconciler used by the installed renderer.
// Components, effects, context, external stores and hooks are production code.
const localRequire = createRequire(import.meta.url);
const demoRequire = createRequire(
  new URL("../../../services/wallet/demo/package.json", import.meta.url),
);
// Initialize the animation dependency in its real Node/server realm, before
// attaching browser-storage peers for provider and preference lifecycle tests.
localRequire("framer-motion");
const rendererRequire = createRequire(
  localRequire.resolve("@react-three/fiber"),
);
interface Host {
  type: string;
  props: Record<string, unknown>;
  children: Host[];
  text?: string;
}
interface Renderer {
  createContainer(
    container: Host,
    tag: number,
    hydration: null,
    strict: boolean,
    concurrent: null,
    prefix: string,
    onError: (error: Error) => void,
    transition: null,
  ): unknown;
  updateContainer(
    node: ReactNode,
    root: unknown,
    parent: null,
    callback: (() => void) | null,
  ): void;
  flushSync<T>(callback: () => T): T;
  flushPassiveEffects(): boolean;
}
const makeRenderer: (config: Record<string, unknown>) => Renderer =
  rendererRequire("react-reconciler");
const emptyContext = {};
const append = (parent: Host, child: Host) => {
  parent.children = parent.children.filter((entry) => entry !== child);
  parent.children.push(child);
};
const remove = (parent: Host, child: Host) => {
  parent.children = parent.children.filter((entry) => entry !== child);
};
const insert = (parent: Host, child: Host, before: Host) => {
  remove(parent, child);
  parent.children.splice(parent.children.indexOf(before), 0, child);
};
const renderer = makeRenderer({
  now: Date.now,
  isPrimaryRenderer: true,
  supportsMutation: true,
  supportsPersistence: false,
  supportsHydration: false,
  getRootHostContext: () => emptyContext,
  getChildHostContext: () => emptyContext,
  getPublicInstance: () => null,
  prepareForCommit: () => null,
  resetAfterCommit: () => {},
  createInstance: (type: string, props: Record<string, unknown>): Host => ({
    type,
    props,
    children: [],
  }),
  createTextInstance: (text: string): Host => ({
    type: "#text",
    props: {},
    children: [],
    text,
  }),
  appendInitialChild: append,
  appendChild: append,
  appendChildToContainer: append,
  removeChild: remove,
  removeChildFromContainer: remove,
  insertBefore: insert,
  insertInContainerBefore: insert,
  finalizeInitialChildren: () => false,
  shouldSetTextContent: () => false,
  prepareUpdate: () => true,
  commitUpdate: (
    instance: Host,
    _payload: unknown,
    _type: string,
    _old: unknown,
    props: Record<string, unknown>,
  ) => {
    instance.props = props;
  },
  commitTextUpdate: (instance: Host, _old: string, text: string) => {
    instance.text = text;
  },
  resetTextContent: (instance: Host) => {
    instance.children = [];
  },
  clearContainer: (container: Host) => {
    container.children = [];
  },
  scheduleTimeout: setTimeout,
  cancelTimeout: clearTimeout,
  noTimeout: -1,
  getCurrentEventPriority: () => 16,
  detachDeletedInstance: () => {},
});

class MemoryStorage implements Storage {
  private values = new Map<string, string>();
  failWrites = false;
  failReads = false;
  get length() {
    return this.values.size;
  }
  getItem(key: string) {
    if (this.failReads) throw new Error("Browser denied storage access");
    return this.values.get(key) ?? null;
  }
  setItem(key: string, value: string) {
    if (this.failWrites) throw new Error("Storage quota exceeded");
    this.values.set(key, value);
  }
  removeItem(key: string) {
    this.values.delete(key);
  }
  clear() {
    this.values.clear();
  }
  key(index: number) {
    return [...this.values.keys()][index] ?? null;
  }
}
const storage = new MemoryStorage();
const browserEvents = new EventTarget();
const classes = new Set<string>();
const alice = privateKeyToAccount(`0x${"11".repeat(32)}`);
const bob = privateKeyToAccount(`0x${"22".repeat(32)}`);
const user = {
  id: "fixture-user",
  aud: "authenticated",
  role: "authenticated",
  email: "fixture@example.test",
};
const requests: string[] = [];
let origin = "";
const server = createServer((request, response) => {
  void (async () => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    response.setHeader("content-type", "application/json");
    const path = (request.url ?? "/").split("?")[0];
    if (path.endsWith("/auth/v1/user")) {
      response.end(JSON.stringify(user));
    } else if (path.endsWith("/auth/v1/logout")) {
      response.end("{}");
    } else if (path.endsWith("/v1/wallet/me")) {
      response.end(
        JSON.stringify({
          wallet: {
            id: "fixture-wallet",
            address: path.startsWith("/bob/") ? bob.address : alice.address,
            chain_id: 125,
            created_at: "",
            last_used_at: null,
          },
          chain: { id: 125, rpc_url: origin, explorer_url: origin },
        }),
      );
    } else {
      const body = JSON.parse(Buffer.concat(chunks).toString()) as {
        id: number;
        method: string;
      };
      requests.push(body.method);
      const transcript: Record<string, string> = {
        eth_chainId: "0x7d",
        eth_blockNumber: "0x10",
        eth_getBalance: "0xde0b6b3a7640000",
      };
      assert.ok(
        Object.hasOwn(transcript, body.method),
        `Recorded RPC ${body.method}`,
      );
      response.end(
        JSON.stringify({
          jsonrpc: "2.0",
          id: body.id,
          result: transcript[body.method],
        }),
      );
    }
  })().catch((error: Error) => response.destroy(error));
});
let settings: typeof import("../src/app/modals/SettingsModal");
let collateral: typeof import("../src/app/modals/DepositWithdrawModal");
let panel: typeof import("../src/app/modals/WalletControlPanel");
let preferences: typeof import("../src/app/modals/dialogPreferences");
let focusModule: typeof import("../src/app/modals/useDialogFocus");
let recoveryModule: typeof import("../src/app/modals/useCollateralRecovery");
let embedded: typeof import("../src/core/wallet/embedded");
let config: typeof import("../src/core/wagmi").wagmiConfig;
let Dialog: typeof import("@radix-ui/react-dialog");
let Popover: typeof import("@radix-ui/react-popover");
let themes: typeof import("next-themes");
const queries = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});
let setupDone = false;
let mobile = false;
let domWindow: Window & typeof globalThis;
async function setup() {
  if (setupDone) return;
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address === "object");
  origin = `http://127.0.0.1:${address.port}`;
  // Every application import follows fixture configuration, including all Paxscan URLs.
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  process.env.NEXT_PUBLIC_PAXEER_WALLET_API = origin;
  process.env.NEXT_PUBLIC_SUPABASE_URL = origin;
  process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY = "fixture-public-key";
  // Server-rendered motion inspects DOM element constructors even though this
  // reconciler does not attach refs or portals to browser nodes. Use jsdom's
  // real constructors before loading the components, as in the wallet tests.
  const { JSDOM } = demoRequire("jsdom");
  domWindow = new JSDOM("<!doctype html><html><body></body></html>", {
    url: origin,
  }).window;
  Object.assign(globalThis, {
    SVGElement: domWindow.SVGElement,
    HTMLElement: domWindow.HTMLElement,
    Element: domWindow.Element,
    Node: domWindow.Node,
  });
  Object.assign(globalThis, { React, localStorage: storage });
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      localStorage: storage,
      sessionStorage: new MemoryStorage(),
      location: {
        origin,
        href: origin,
        pathname: "/perps",
        search: "",
        hash: "",
      },
      matchMedia(query: string) {
        const events = new EventTarget();
        return {
          matches: query === "(max-width: 767px)" ? mobile : false,
          addEventListener: events.addEventListener.bind(events),
          removeEventListener: events.removeEventListener.bind(events),
          addListener(listener: EventListener) {
            events.addEventListener("change", listener);
          },
          removeListener(listener: EventListener) {
            events.removeEventListener("change", listener);
          },
        };
      },
      addEventListener: browserEvents.addEventListener.bind(browserEvents),
      removeEventListener:
        browserEvents.removeEventListener.bind(browserEvents),
      dispatchEvent: browserEvents.dispatchEvent.bind(browserEvents),
    },
  });
  // This reconciler has no DOM host. The theme provider's class transport is an
  // in-memory store; Radix portals remain unmounted. Content semantics are also
  // exercised with the real server renderer below, without replacing primitives.
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: {
      documentElement: {
        classList: {
          add: (...names: string[]) =>
            names.forEach((name) => classes.add(name)),
          remove: (...names: string[]) =>
            names.forEach((name) => classes.delete(name)),
        },
        style: {},
      },
    },
  });
  Dialog = localRequire("@radix-ui/react-dialog");
  Popover = localRequire("@radix-ui/react-popover");
  themes = localRequire("next-themes");
  preferences = localRequire("../src/app/modals/dialogPreferences");
  settings = localRequire("../src/app/modals/SettingsModal");
  collateral = localRequire("../src/app/modals/DepositWithdrawModal");
  panel = localRequire("../src/app/modals/WalletControlPanel");
  focusModule = localRequire("../src/app/modals/useDialogFocus");
  recoveryModule = localRequire("../src/app/modals/useCollateralRecovery");
  embedded = localRequire("../src/core/wallet/embedded");
  ({ wagmiConfig: config } = localRequire("../src/core/wagmi"));
  setupDone = true;
}
function wrap(node: ReactNode) {
  return h(
    WagmiContext.Provider,
    { value: config },
    h(
      QueryClientProvider,
      { client: queries },
      h(
        embedded.PaxeerProvider,
        null,
        h(
          themes.ThemeProvider,
          {
            attribute: "class",
            defaultTheme: "dark",
            themes: ["light", "dark", "greenify", "hotline", "oled"],
          },
          node,
        ),
      ),
    ),
  );
}
interface Fiber {
  elementType?: React.ElementType;
  child: Fiber | null;
  sibling: Fiber | null;
  memoizedProps: Record<string, unknown> | null;
}
function mount(node: ReactNode) {
  const container: Host = { type: "root", props: {}, children: [] };
  const errors: Error[] = [];
  const root = renderer.createContainer(
    container,
    0,
    null,
    false,
    null,
    "",
    (error) => errors.push(error),
    null,
  );
  const update = (value: ReactNode) => {
    renderer.flushSync(() =>
      renderer.updateContainer(wrap(value), root, null, null),
    );
    renderer.flushPassiveEffects();
    assert.deepEqual(errors, []);
  };
  update(node);
  return {
    container,
    root: root as { current: Fiber },
    update,
    unmount: () => update(null),
  };
}
function nodes(host: Host): Host[] {
  return [host, ...host.children.flatMap(nodes)];
}
function text(host: Host): string {
  return host.text ?? host.children.map(text).join("");
}
function elements(mounted: ReturnType<typeof mount>): ReactElement[] {
  const result: ReactElement[] = [];
  const seen = new Set<unknown>();
  const children = (value: unknown) => {
    if (seen.has(value)) return;
    if (Array.isArray(value)) {
      seen.add(value);
      value.forEach(children);
    } else if (React.isValidElement<{ children?: ReactNode }>(value)) {
      seen.add(value);
      result.push(value);
      children(value.props.children);
    }
  };
  const visit = (fiber: Fiber | null) => {
    if (!fiber) return;
    if (fiber.elementType && fiber.memoizedProps)
      result.push(h(fiber.elementType, fiber.memoizedProps));
    children(fiber.memoizedProps?.children);
    visit(fiber.child);
    visit(fiber.sibling);
  };
  visit(mounted.root.current);
  return result;
}
function element(mounted: ReturnType<typeof mount>, type: unknown) {
  const found = elements(mounted).find((entry) => entry.type === type);
  assert.ok(found, "production element exists");
  return found as ReactElement<Record<string, unknown>>;
}
function click(host: Host) {
  assert.ok(!host.props.disabled);
  renderer.flushSync(() =>
    (host.props.onClick as (event: unknown) => void)({
      defaultPrevented: false,
      isPropagationStopped: () => false,
      stopPropagation() {},
    }),
  );
  renderer.flushPassiveEffects();
}
async function until(condition: () => boolean, message: string) {
  const deadline = Date.now() + 10_000;
  while (!condition() && Date.now() < deadline) {
    renderer.flushPassiveEffects();
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  renderer.flushPassiveEffects();
  assert.ok(condition(), message);
}
function assertDialogSemantics(
  mounted: ReturnType<typeof mount>,
  title: string,
  close: string,
) {
  const root = element(mounted, Dialog.Root);
  assert.equal(root.props.open, true);
  assert.notEqual(root.props.modal, false);
  const content = element(mounted, Dialog.Content);
  assert.equal(typeof content.props.onOpenAutoFocus, "function");
  assert.equal(typeof content.props.onCloseAutoFocus, "function");
  assert.equal(
    content.props.onEscapeKeyDown,
    undefined,
    "Escape remains handled by Radix",
  );
  assert.equal(
    content.props.onFocusOutside,
    undefined,
    "Radix focus containment remains active",
  );
  const markup = renderToStaticMarkup(h(Dialog.Root, { open: true }, content));
  assert.match(markup, /role="dialog"/);
  const labelledBy = markup.match(/aria-labelledby="([^"]+)"/)?.[1];
  assert.ok(labelledBy);
  assert.ok(markup.includes(`id="${labelledBy}"`));
  assert.ok(markup.includes(title));
  assert.ok(markup.includes(`aria-label="${close}"`));
  // Drive the very Close element returned by the production component through
  // Radix's real controlled root, so the primitive's dismissal reaches its caller.
  const closeElement = element(mounted, Dialog.Close);
  let dismissed = false;
  const closeMount = mount(
    h(
      Dialog.Root,
      {
        open: true,
        onOpenChange(open) {
          dismissed = !open;
        },
      },
      closeElement,
    ),
  );
  const button = nodes(closeMount.container).find(
    (node) => node.type === "button",
  );
  assert.ok(button);
  click(button);
  assert.equal(dismissed, true);
  closeMount.unmount();
}

async function collateralDialogAccess() {
  if (isMainThread) return runWorker("collateralDialogAccess");
  await setup();
  let closed = 0;
  const modal = (open: boolean) =>
    h(collateral.DepositWithdrawModal, {
      isOpen: open,
      onClose() {
        closed++;
      },
    });
  const mounted = mount(modal(true));
  await until(
    () => Boolean(element(mounted, Dialog.Content)),
    "collateral dialog mounted",
  );
  assertDialogSemantics(
    mounted,
    "Deposit Collateral",
    "Close collateral dialog",
  );
  (element(mounted, Dialog.Root).props.onOpenChange as (open: boolean) => void)(
    false,
  );
  assert.equal(closed, 1);
  let recovery:
    | ReturnType<typeof recoveryModule.useCollateralRecovery>
    | undefined;
  const onSettled = () => {};
  function RecoveryConsumer() {
    recovery = recoveryModule.useCollateralRecovery(onSettled);
    return null;
  }
  const owner = mount(h(RecoveryConsumer));
  await until(
    () => Boolean(recovery && !recovery.busy),
    "real recovery store ready",
  );
  renderer.flushSync(() =>
    assert.equal(
      recovery!.begin({ address: alice.address, chainId: 125 }),
      true,
    ),
  );
  mounted.update(modal(false));
  mounted.update(modal(true));
  assert.equal(
    recovery!.busy,
    true,
    "dismissal does not clear a financial operation",
  );
  assert.equal(recovery!.active?.address, alice.address);
  assert.equal(recovery!.begin({ address: bob.address, chainId: 125 }), false);
  renderer.flushSync(() => recovery!.end());
  owner.unmount();
  mounted.unmount();
}
async function settingsDialogAccess() {
  if (isMainThread) return runWorker("settingsDialogAccess");
  await setup();
  let closed = 0;
  const mounted = mount(
    h(settings.SettingsModal, {
      isOpen: true,
      onClose() {
        closed++;
      },
    }),
  );
  assertDialogSemantics(mounted, "Settings", "Close settings");
  (element(mounted, Dialog.Root).props.onOpenChange as (open: boolean) => void)(
    false,
  );
  assert.equal(closed, 1);
  mounted.unmount();
  // The production controller restores an external launcher (there is no
  // Dialog.Trigger), captures a fresh launcher on reopening and ignores removals.
  let focused = "launcher-a";
  let target = {
    isConnected: true,
    focus() {
      focused = "launcher-a";
    },
  };
  const lifecycle = focusModule.createDialogFocusLifecycle(() => target);
  lifecycle.onOpenAutoFocus();
  focused = "drawer-control";
  const close = new Event("closeAutoFocus", { cancelable: true });
  lifecycle.onCloseAutoFocus(close);
  assert.equal(focused, "launcher-a");
  assert.equal(close.defaultPrevented, true);
  target = {
    isConnected: true,
    focus() {
      focused = "launcher-b";
    },
  };
  lifecycle.onOpenAutoFocus();
  lifecycle.onCloseAutoFocus(new Event("closeAutoFocus", { cancelable: true }));
  assert.equal(focused, "launcher-b");
  lifecycle.onOpenAutoFocus();
  target.isConnected = false;
  focused = "other";
  lifecycle.onCloseAutoFocus(new Event("closeAutoFocus", { cancelable: true }));
  assert.equal(focused, "other");
}
async function settingsWithoutConsumers() {
  if (isMainThread) return runWorker("settingsWithoutConsumers");
  await setup();
  const settingsMount = mount(
    h(settings.SettingsModal, { isOpen: true, onClose() {} }),
  );
  const controls = mount(h(settings.DialogMotionSettings));
  const wallet = mount(
    h(panel.WalletControlPanel, {
      open: true,
      onOpenChange() {},
      trigger: h("button", null, "Wallet"),
    }),
  );
  const before = element(settingsMount, Dialog.Content).props
    .children as ReactElement<{ initial: unknown; transition: unknown }>;
  assert.deepEqual(before.props.initial, { x: "100%" });
  assert.match(
    String(element(wallet, Popover.Content).props.className),
    /animate-in/,
  );
  const motionSwitch = nodes(controls.container).find(
    (node) => node.props.role === "switch",
  );
  assert.ok(motionSwitch);
  assert.equal(motionSwitch.props["aria-checked"], false);
  click(motionSwitch);
  await until(() => {
    const drawer = element(settingsMount, Dialog.Content).props
      .children as typeof before;
    return drawer.props.initial === false;
  }, "real shared hook updates the drawer's motion consumer");
  const after = element(settingsMount, Dialog.Content).props
    .children as typeof before;
  assert.deepEqual(after.props.transition, { duration: 0 });
  assert.doesNotMatch(
    String(element(wallet, Popover.Content).props.className),
    /animate-in|zoom-in/,
  );
  assert.equal(
    nodes(controls.container).find((node) => node.props.role === "switch")
      ?.props["aria-checked"],
    true,
  );
  assert.match(text(controls.container), /saved on this device/);
  const unsupported =
    /Hide PnL|Hide Size|Show All Warnings|Sound for Fills|Low Power Mode|Persist Trading Connection|Skip Order Confirmations/;
  const content = element(settingsMount, Dialog.Content);
  const markup = renderToStaticMarkup(h(Dialog.Root, { open: true }, content));
  assert.doesNotMatch(markup, unsupported);
  // Theme choices still use the real next-themes producer and shared class consumer.
  const themeButtons = elements(settingsMount).filter(
    (entry) => entry.type === "button" && "aria-pressed" in entry.props,
  );
  assert.equal(themeButtons.length, 4);
  renderer.flushSync(() => (themeButtons[2].props.onClick as () => void)());
  renderer.flushPassiveEffects();
  assert.equal(storage.getItem("theme"), "hotline");
  assert.equal(classes.has("hotline"), true);
  controls.unmount();
  settingsMount.unmount();
  wallet.unmount();
}
async function settingsSavePromise() {
  if (isMainThread) return runWorker("settingsSavePromise");
  await setup();
  let control = mount(h(settings.DialogMotionSettings));
  // Save through the actual accessible switch, not by hand-building a storage record.
  const current = nodes(control.container).find(
    (node) => node.props.role === "switch",
  );
  assert.ok(current);
  if (current.props["aria-checked"] === true) click(current);
  click(nodes(control.container).find((node) => node.props.role === "switch")!);
  const durable = storage.getItem(preferences.DIALOG_PREFERENCES_KEY);
  assert.ok(durable);
  assert.deepEqual(preferences.decodeDialogPreferences(durable), {
    version: 1,
    reduceMotion: true,
  });
  assert.match(text(control.container), /saved on this device/);
  control.unmount();
  // Stop both the component and the module runtime, retaining only browser storage.
  for (const path of [
    "../src/app/modals/SettingsModal",
    "../src/app/modals/WalletControlPanel",
    "../src/app/modals/dialogPreferences",
  ]) {
    delete localRequire.cache[localRequire.resolve(path)];
  }
  preferences = localRequire("../src/app/modals/dialogPreferences");
  settings = localRequire("../src/app/modals/SettingsModal");
  panel = localRequire("../src/app/modals/WalletControlPanel");
  const fresh = preferences.createDialogPreferenceStore(() => storage);
  assert.equal(fresh.getSnapshot().status, "loading");
  fresh.hydrate();
  assert.equal(fresh.getSnapshot().reduceMotion, true);
  assert.equal(fresh.getSnapshot().status, "saved");
  control = mount(h(settings.DialogMotionSettings));
  assert.equal(
    nodes(control.container).find((node) => node.props.role === "switch")
      ?.props["aria-checked"],
    true,
  );
  const drawer = mount(
    h(settings.SettingsModal, { isOpen: true, onClose() {} }),
  );
  assert.equal(
    (
      element(drawer, Dialog.Content).props.children as ReactElement<{
        initial: unknown;
      }>
    ).props.initial,
    false,
  );
  storage.failWrites = true;
  click(nodes(control.container).find((node) => node.props.role === "switch")!);
  assert.equal(storage.getItem(preferences.DIALOG_PREFERENCES_KEY), durable);
  assert.equal(
    nodes(control.container).find((node) => node.props.role === "switch")
      ?.props["aria-checked"],
    true,
  );
  assert.match(text(control.container), /was not saved/);
  assert.ok(
    nodes(control.container).some((node) => node.props.role === "alert"),
  );
  storage.failWrites = false;
  click(nodes(control.container).find((node) => node.props.role === "switch")!);
  assert.match(text(control.container), /saved on this device/);
  assert.equal(
    nodes(control.container).find((node) => node.props.role === "switch")
      ?.props["aria-checked"],
    false,
  );
  control.unmount();
  drawer.unmount();
  // Malformed/version-incompatible data is reported, never treated as a saved default.
  assert.throws(
    () =>
      preferences.decodeDialogPreferences('{"version":2,"reduceMotion":true}'),
    /Unsupported/,
  );
  assert.throws(
    () =>
      preferences.decodeDialogPreferences(
        '{"version":1,"reduceMotion":"true"}',
      ),
    /Unsupported/,
  );
  storage.failReads = true;
  const blocked = preferences.createDialogPreferenceStore(() => storage);
  blocked.hydrate();
  assert.equal(blocked.getSnapshot().status, "error");
  assert.match(blocked.getSnapshot().error!, /could not be restored/);
  storage.failReads = false;
}
async function authorize(
  wallet: NonNullable<ReturnType<typeof embedded.usePaxeer>["wallet"]>,
) {
  const payload = Buffer.from(
    JSON.stringify({
      exp: Math.floor(Date.now() / 1000) + 3600,
      sub: user.id,
      aud: user.aud,
    }),
  ).toString("base64url");
  const token = `${Buffer.from('{"alg":"HS256"}').toString("base64url")}.${payload}.fixture`;
  assert.equal(
    (
      await wallet.supabase.auth.setSession({
        access_token: token,
        refresh_token: "fixture-refresh",
      })
    ).error,
    null,
  );
}
async function awaitAdapterAccount(
  connector: NonNullable<ReturnType<typeof getAccount>["connector"]>,
  address: `0x${string}`,
) {
  const deadline = Date.now() + 10_000;
  let accounts = await connector.getAccounts();
  while (!accounts.includes(address) && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 10));
    accounts = await connector.getAccounts();
  }
  assert.deepEqual(
    accounts,
    [address],
    "real adapter completes initial auth observation before connecting",
  );
}
async function walletPanelIdentityLabel() {
  if (isMainThread) return runWorker("walletPanelIdentityLabel");
  await setup();
  const { getPaxeerWallet } = localRequire(
    "../src/core/wallet/embedded/client",
  ) as typeof import("../src/core/wallet/embedded/client");
  const primary = getPaxeerWallet();
  assert.ok(primary);
  await authorize(primary);
  const session = await primary.getSession();
  assert.ok(session, "loopback auth producer established the fixture session");
  const { PaxeerWallet } = localRequire(
    "../src/core/wallet/embedded/sdk",
  ) as typeof import("../src/core/wallet/embedded/sdk");
  // Inject the authenticated session at the connector's SDK boundary. The
  // production connector, EIP-1193 bridge, SDK getWallet and HTTP transport
  // still run; each peer owns its session instead of sharing Supabase's
  // hostname-derived browser storage key with the panel's primary identity.
  class SessionWallet extends PaxeerWallet {
    private session: Session | null = session;
    private listeners = new Set<
      (event: AuthChangeEvent, value: Session | null) => void
    >();
    override async getSession() {
      return this.session;
    }
    override onAuthStateChange(
      listener: (event: AuthChangeEvent, value: Session | null) => void,
    ) {
      this.listeners.add(listener);
      listener("INITIAL_SESSION", this.session);
      return () => {
        this.listeners.delete(listener);
      };
    }
    override async signOut() {
      this.session = null;
      for (const listener of this.listeners) listener("SIGNED_OUT", null);
    }
  }
  const matching = new SessionWallet({
    apiUrl: origin,
    supabaseUrl: origin,
    supabaseAnonKey: "fixture-matching-key",
  });
  const originalEmbedded = config.connectors.find(
    (entry) => entry.id === embedded.PAXEER_CONNECTOR_ID,
  );
  assert.ok(originalEmbedded);
  const embeddedConnector = config._internal.connectors.setup(
    embedded.paxeerEmbeddedConnector({
      wallet: matching,
      chainId: 125,
      rpcUrl: origin,
    }),
  );
  config._internal.connectors.setState([
    ...config.connectors.filter((entry) => entry !== originalEmbedded),
    embeddedConnector,
  ]);
  await awaitAdapterAccount(embeddedConnector, alice.address);
  await connect(config, { connector: embeddedConnector });
  const mounted = mount(h(panel.PanelBody, { close() {} }));
  await until(
    () => text(mounted.container).includes(user.email),
    "matching connected embedded signer receives its email",
  );
  assert.match(text(mounted.container), new RegExp(alice.address));
  const events = new Map<string, Set<(...args: unknown[]) => void>>();
  const provider = {
    async request({ method }: { method: string }) {
      if (method === "eth_requestAccounts" || method === "eth_accounts")
        return [bob.address];
      if (method === "eth_chainId") return "0x7d";
      if (method === "wallet_requestPermissions")
        return [{ parentCapability: "eth_accounts" }];
      if (method === "wallet_getCapabilities") return {};
      throw new Error(`Unrecorded browser wallet request ${method}`);
    },
    on(name: string, listener: (...args: unknown[]) => void) {
      const listeners = events.get(name) ?? new Set();
      listeners.add(listener);
      events.set(name, listeners);
    },
    removeListener(name: string, listener: (...args: unknown[]) => void) {
      events.get(name)?.delete(listener);
    },
  };
  const injectedConnector = config._internal.connectors.setup(
    injected({
      target: {
        id: "fixture-browser",
        name: "Browser Wallet",
        provider: provider as import("viem").EIP1193Provider,
      },
      shimDisconnect: false,
    }),
  );
  config._internal.connectors.setState([
    ...config.connectors,
    injectedConnector,
  ]);
  await connect(config, { connector: injectedConnector });
  await until(
    () =>
      text(mounted.container).includes("Browser Wallet") &&
      text(mounted.container).includes(bob.address),
    "connected browser signer reaches actual wallet header",
  );
  assert.doesNotMatch(text(mounted.container), /fixture@example\.test/);
  // The same injected session reaches a different provisioned signer through
  // the real SDK and loopback gateway, while the panel keeps Alice's identity.
  const other = new SessionWallet({
    apiUrl: `${origin}/bob`,
    supabaseUrl: `${origin}/bob`,
    supabaseAnonKey: "fixture-other-key",
  });
  const otherConnector = config._internal.connectors.setup(
    embedded.paxeerEmbeddedConnector({
      wallet: other,
      chainId: 125,
      rpcUrl: origin,
    }),
  );
  config._internal.connectors.setState([...config.connectors, otherConnector]);
  await awaitAdapterAccount(otherConnector, bob.address);
  await connect(config, { connector: otherConnector });
  await until(
    () =>
      getAccount(config).address === bob.address &&
      text(mounted.container).includes("Paxeer Wallet"),
    "mismatched embedded signer remains identified by its wallet",
  );
  assert.doesNotMatch(text(mounted.container), /fixture@example\.test/);
  assert.match(text(mounted.container), new RegExp(bob.address));
  await disconnect(config, { connector: otherConnector });
  other.supabase.auth.stopAutoRefresh();
  await disconnect(config, { connector: embeddedConnector });
  matching.supabase.auth.stopAutoRefresh();
  mounted.unmount();
  assert.ok(
    requests.includes("eth_getBalance"),
    "real balance consumer reached fixture RPC",
  );
}
async function settingsGuideDrift() {
  if (isMainThread) return runWorker("settingsGuideDrift");
  await setup();
  const guide = readFileSync(
    new URL("../src/app/modals/SettingsModal.md", import.meta.url),
    "utf8",
  );
  const record = readFileSync(
    new URL("../src/app/modals/repair-22-33-11.kvx", import.meta.url),
    "utf8",
  );
  const controls = mount(h(settings.DialogMotionSettings));
  assert.match(text(controls.container), /Reduce dialog motion/);
  for (const required of [
    "Reduce dialog motion",
    "ui/atoms",
    "hooks/ui/useMobile",
    "next-themes",
    preferences.DIALOG_PREFERENCES_KEY,
    "version",
    "Escape",
    "focus",
    "WalletControlPanel",
  ])
    assert.ok(guide.includes(required), `guide documents ${required}`);
  assert.doesNotMatch(
    guide,
    /components\/ui|hooks\/use-mobile|seven local toggle|Settings are automatically saved/,
  );
  for (const finding of [
    "collateral-dialog-access",
    "settings-guide-drift",
    "settings-without-consumers",
    "settings-dialog-access",
    "settings-save-promise",
    "wallet-panel-identity-label",
  ])
    assert.ok(
      record.includes(`[finding.${finding}]`),
      `closure record names ${finding}`,
    );
  for (const name of Object.keys(cases))
    assert.ok(record.includes(name), `closure record names ${name}`);
  assert.match(record, /Domain 3.*src\/page-modules\/portfolio/);
  assert.match(record, /Domain 3.*src\/widgets\/trading/);
  assert.match(record, /Domain 3.*src\/pages\/_app\.tsx/);
  controls.unmount();
}
const cases = {
  collateralDialogAccess,
  settingsDialogAccess,
  settingsWithoutConsumers,
  settingsSavePromise,
  walletPanelIdentityLabel,
  settingsGuideDrift,
};
let worker: Worker | undefined;
function runWorker(name: keyof typeof cases) {
  if (!worker)
    worker = new Worker(
      `const { workerData } = require("node:worker_threads"); require(workerData.loader); require(workerData.file)`,
      {
        eval: true,
        workerData: {
          loader: localRequire.resolve("tsx/cjs"),
          file: fileURLToPath(import.meta.url),
        },
      },
    );
  return new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(
      () => reject(new Error(`Proof ${name} exceeded 30 seconds`)),
      30_000,
    );
    const onError = (error: Error) => {
      clearTimeout(timeout);
      reject(error);
    };
    worker!.once("error", onError);
    worker!.once("message", (message: { name: string; error?: string }) => {
      clearTimeout(timeout);
      worker!.removeListener("error", onError);
      if (message.name !== name)
        reject(new Error(`Unexpected proof ${message.name}`));
      else if (message.error) reject(new Error(message.error));
      else resolve();
    });
    worker!.postMessage(name);
  });
}
if (isMainThread) {
  test(collateralDialogAccess);
  test(settingsDialogAccess);
  test(settingsWithoutConsumers);
  test(settingsSavePromise);
  test(walletPanelIdentityLabel);
  test(settingsGuideDrift);
  after(async () => {
    if (!worker) return;
    try {
      await Promise.race([
        new Promise<void>((resolve, reject) => {
          worker!.once("message", (message: { error?: string }) =>
            message.error ? reject(new Error(message.error)) : resolve(),
          );
          worker!.postMessage("cleanup");
        }),
        new Promise<never>((_, reject) =>
          setTimeout(
            () => reject(new Error("Cleanup timed out")),
            10_000,
          ).unref(),
        ),
      ]);
    } finally {
      await worker.terminate();
    }
  });
} else {
  parentPort!.on("message", (name: keyof typeof cases | "cleanup") => {
    void (name === "cleanup" ? cleanup() : cases[name]()).then(
      () => parentPort!.postMessage({ name }),
      (error: unknown) =>
        parentPort!.postMessage({
          name,
          error: error instanceof Error ? error.stack : String(error),
        }),
    );
  });
}
async function cleanup() {
  if (!setupDone) return;
  queries.clear();
  for (const connection of config.state.connections.values())
    await disconnect(config, { connector: connection.connector });
  const { getPaxeerWallet } = localRequire(
    "../src/core/wallet/embedded/client",
  ) as typeof import("../src/core/wallet/embedded/client");
  getPaxeerWallet()?.supabase.auth.stopAutoRefresh();
  server.closeAllConnections();
  await new Promise<void>((resolve) => server.close(() => resolve()));
  domWindow.close();
}
