import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { test, type TestContext } from "node:test";
import type { ComponentType, ReactNode } from "react";
import { Window } from "happy-dom";
import { APPLICATION_NAME } from "../apps/web/src/PageMetadata";
import "./consent-privacy-native-venue-terms.test";

const webRequire = createRequire(
  new URL("../apps/web/package.json", import.meta.url),
);
const React: typeof import("react") = webRequire("react");
const { createElement: h, act } = React;
const { renderToStaticMarkup }: typeof import("react-dom/server") =
  webRequire("react-dom/server");
const {
  HeadManagerContext,
}: typeof import("next/dist/shared/lib/head-manager-context.shared-runtime") =
  webRequire("next/dist/shared/lib/head-manager-context.shared-runtime");

function installGlobals(t: TestContext, values: Record<string, unknown>) {
  const previous = new Map(
    Object.keys(values).map((key) => [
      key,
      Object.getOwnPropertyDescriptor(globalThis, key),
    ]),
  );
  for (const [key, value] of Object.entries(values))
    Object.defineProperty(globalThis, key, { value, configurable: true });
  t.after(() => {
    for (const [key, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  });
}

function renderPage(t: TestContext, Page: ComponentType) {
  let head: ReactNode[] = [];
  const html = renderToStaticMarkup(
    h(
      HeadManagerContext.Provider,
      {
        value: {
          mountedInstances: new Set(),
          updateHead(elements: ReactNode[]) {
            head = elements;
          },
        },
      },
      h(Page),
    ),
  );
  const dom = new Window();
  t.after(() => dom.happyDOM.close());
  dom.document.head.innerHTML = renderToStaticMarkup(
    h(React.Fragment, null, head),
  );
  dom.document.body.innerHTML = html;
  return dom.document;
}

async function legalPagesCarryATitleAndTheIcon(t: TestContext) {
  installGlobals(t, { React });
  const routes = [
    [await import("../apps/web/src/pages/terms"), "Terms of Service"],
    [await import("../apps/web/src/pages/privacy"), "Privacy Policy"],
    [await import("../apps/web/src/pages/cookies"), "Cookie Policy"],
  ] as const;
  for (const [page, title] of routes) {
    const document = renderPage(t, page.default);
    assert.equal(document.title, `${title} | ${APPLICATION_NAME}`);
    assert.equal(document.querySelector("h1")?.textContent, title);
    assert.equal(
      document.querySelector('link[rel="icon"]')?.getAttribute("href"),
      "/brand/paxeer-x.png",
    );
    assert.equal(
      document.querySelector('link[rel="icon"]')?.getAttribute("type"),
      "image/png",
    );
    assert.equal(
      document
        .querySelector('link[rel="apple-touch-icon"]')
        ?.getAttribute("href"),
      "/brand/paxeer-x.png",
    );
  }
}

async function viewsCarryADescriptionAndAPreviewTitle(t: TestContext) {
  installGlobals(t, { React });
  const routes = [
    [await import("../apps/web/src/pages/[[...route]]"), undefined],
    [await import("../apps/web/src/pages/terms"), "Terms of Service"],
    [await import("../apps/web/src/pages/privacy"), "Privacy Policy"],
    [await import("../apps/web/src/pages/cookies"), "Cookie Policy"],
  ] as const;
  assert.equal(APPLICATION_NAME, "Paxeer Xtrades");
  for (const [page, documentTitle] of routes) {
    const document = renderPage(t, page.default);
    const title = documentTitle
      ? `${documentTitle} | ${APPLICATION_NAME}`
      : APPLICATION_NAME;
    const description = documentTitle
      ? `Read the ${documentTitle} for ${APPLICATION_NAME}.`
      : `${APPLICATION_NAME} provides a perpetual futures trading workspace on the Paxeer Network.`;
    const content = (selector: string) =>
      document.querySelector(selector)?.getAttribute("content");
    assert.equal(document.title, title);
    assert.equal(content('meta[name="application-name"]'), APPLICATION_NAME);
    assert.equal(content('meta[name="description"]'), description);
    assert.equal(content('meta[property="og:site_name"]'), APPLICATION_NAME);
    assert.equal(content('meta[property="og:title"]'), title);
    assert.equal(content('meta[property="og:description"]'), description);
    assert.equal(content('meta[name="twitter:title"]'), title);
    assert.equal(content('meta[name="twitter:description"]'), description);
    assert.equal(content('meta[name="twitter:card"]'), "summary");
    assert.equal(document.querySelectorAll("title").length, 1);
    if (!documentTitle) {
      assert.equal(document.querySelectorAll('link[rel="preload"]').length, 3);
      assert.equal(
        document.querySelector('link[rel="icon"]')?.getAttribute("href"),
        "/brand/paxeer-x.png",
      );
    }
  }
}

async function cookieNoticeLeavesTheBottomNavigationFree(t: TestContext) {
  const dom = new Window({ width: 390, height: 844 });
  installGlobals(t, {
    React,
    window: dom,
    self: dom,
    document: dom.document,
    navigator: dom.navigator,
    HTMLElement: dom.HTMLElement,
    localStorage: dom.localStorage,
    IS_REACT_ACT_ENVIRONMENT: true,
  });
  const style = dom.document.createElement("style");
  const stylesheet = await readFile(
    new URL("../apps/web/src/styles/globals.css", import.meta.url),
    "utf8",
  );
  style.textContent = stylesheet;
  dom.document.head.append(style);
  dom.document.documentElement.style.fontSize = "16px";

  const { CookieBanner } = await import("../apps/web/src/legal/CookieBanner");
  const { cookieConsentStore, COOKIE_CONSENT_STORAGE_KEY } =
    await import("../apps/web/src/legal/useCookieConsent");
  const { BottomNav } =
    await import("../packages/ui-scaffold/src/components/bottomNav/bottomNav.ui.mobile");
  const previousConsent = cookieConsentStore.getState();
  cookieConsentStore.setState({ value: null, loading: false });
  const { createRoot }: typeof import("react-dom/client") =
    webRequire("react-dom/client");
  const container = dom.document.createElement("div");
  dom.document.body.append(container);
  const root = createRoot(container as unknown as HTMLElement);
  try {
    await act(async () => {
      root.render(h(CookieBanner));
    });
    const banner = container.querySelector('[role="dialog"]');
    assert.ok(banner);
    assert.equal(banner.getAttribute("aria-modal"), "false");
    const nav = dom.document.createElement("footer");
    nav.innerHTML = renderToStaticMarkup(
      h(BottomNav, {
        current: "/trade",
        mainMenus: [
          {
            name: "Trade",
            href: "/trade",
            activeIcon: null,
            inactiveIcon: null,
          },
        ],
      }),
    );
    const navHeight = nav.firstElementChild?.getAttribute("style");
    assert.match(navHeight ?? "", /--oui-height:64px/);

    for (const width of [390, 1023]) {
      dom.happyDOM.setWindowSize({ width });
      style.textContent = stylesheet;
      const mobile = dom.getComputedStyle(banner);
      assert.equal(mobile.position, "sticky");
      assert.equal(
        mobile.bottom,
        "calc(64px + env(safe-area-inset-bottom, 0px) + 12px)",
      );
      assert.equal(mobile.marginBottom, mobile.bottom);
      assert.equal(mobile.transform, "none");
      assert.equal(mobile.overflowY, "auto");
      assert.equal(
        mobile.maxHeight,
        "calc(100dvh - 64px - env(safe-area-inset-bottom, 0px) - 24px)",
      );
    }
    for (const width of [1024, 1440]) {
      dom.happyDOM.setWindowSize({ width });
      style.textContent = stylesheet;
      const desktop = dom.getComputedStyle(banner);
      assert.equal(desktop.position, "fixed");
      const bottom = desktop.bottom;
      assert.match(bottom, /^([0-9.]+)(px|rem)$/);
      assert.equal(parseFloat(bottom) * (bottom.endsWith("rem") ? 16 : 1), 24);
      assert.equal(desktop.left, "50%");
      assert.equal(desktop.transform, "translateX(-50%)");
    }
    const close = banner.querySelector(
      'button[aria-label="Dismiss cookie banner"]',
    );
    assert.ok(close);
    assert.ok(
      close.querySelector("svg")?.classList.contains("oui-text-base-contrast"),
    );
    assert.equal(
      close.querySelector("path")?.getAttribute("fill-opacity"),
      "1",
    );
    assert.match(banner.textContent, /HyperPax uses essential cookies/);
    assert.ok(banner.querySelector('a[href="/cookies"]'));
    assert.ok(banner.querySelector('a[href="/privacy"]'));
    await act(async () => close.click());
    assert.equal(container.querySelector('[role="dialog"]'), null);
    assert.equal(COOKIE_CONSENT_STORAGE_KEY, "hyperpax_cookie_consent");
    assert.equal(
      dom.localStorage.getItem(COOKIE_CONSENT_STORAGE_KEY),
      "declined",
    );
  } finally {
    await act(async () => root.unmount());
    cookieConsentStore.setState(previousConsent);
    await dom.happyDOM.close();
  }
}

test(legalPagesCarryATitleAndTheIcon);
test(viewsCarryADescriptionAndAPreviewTitle);
test(cookieNoticeLeavesTheBottomNavigationFree);
