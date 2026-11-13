import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { after, test, type TestContext } from "node:test";
import React, { createElement, type ComponentType } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const fixtureEnvironment = {
  NEXT_PUBLIC_PERPS_GATEWAY_URL: "https://gateway.fixture.test",
  NEXT_PUBLIC_PAXSCAN_BASE_URL: "https://explorer.fixture.test",
  NEXT_PUBLIC_PAXSCAN_API_URL: "https://explorer.fixture.test/api/v2",
  NEXT_PUBLIC_PAXSCAN_WS_URL: "wss://explorer.fixture.test/socket",
};
const savedEnvironment = new Map(
  Object.keys(fixtureEnvironment).map((key) => [key, process.env[key]]),
);
Object.assign(process.env, fixtureEnvironment);
after(() => {
  for (const [key, value] of savedEnvironment) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

class ConsentStorage implements Storage {
  private readonly entries = new Map<string, string>();
  get length(): number {
    return this.entries.size;
  }
  clear(): void {
    this.entries.clear();
  }
  getItem(key: string): string | null {
    return this.entries.get(key) ?? null;
  }
  key(index: number): string | null {
    return [...this.entries.keys()][index] ?? null;
  }
  removeItem(key: string): void {
    this.entries.delete(key);
  }
  setItem(key: string, value: string): void {
    this.entries.set(key, value);
  }
}

function installReact(t: TestContext): void {
  const previous = Object.getOwnPropertyDescriptor(globalThis, "React");
  Object.defineProperty(globalThis, "React", { value: React, configurable: true });
  t.after(() => {
    if (previous) Object.defineProperty(globalThis, "React", previous);
    else Reflect.deleteProperty(globalThis, "React");
  });
}

function pageText(page: ComponentType): { html: string; text: string } {
  const html = renderToStaticMarkup(createElement(page));
  return {
    html,
    text: html
      .replace(/<[^>]*>/g, " ")
      .replace(/&quot;|&#x27;/g, "'")
      .replace(/\s+/g, " "),
  };
}

async function cookieErrorConsentMatchesOperationalCapture(t: TestContext): Promise<void> {
  installReact(t);
  const storage = new ConsentStorage();
  const oldStorage = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  Object.defineProperty(globalThis, "localStorage", { value: storage, configurable: true });
  t.after(() => {
    if (oldStorage) Object.defineProperty(globalThis, "localStorage", oldStorage);
    else Reflect.deleteProperty(globalThis, "localStorage");
  });

  const envelopes: string[] = [];
  const receiver = createServer((request, response) => {
    let body = "";
    request.setEncoding("utf8");
    request.on("data", (chunk: string) => {
      body += chunk;
    });
    request.on("end", () => {
      envelopes.push(body);
      response.writeHead(200, { "content-type": "application/json" });
      response.end("{}");
    });
  });
  t.after(async () => {
    receiver.closeAllConnections();
    await new Promise<void>((resolve, reject) => {
      receiver.close((error) => (error ? reject(error) : resolve()));
    });
  });
  receiver.listen(0, "127.0.0.1");
  await once(receiver, "listening");
  const dsn = `http://fixture@127.0.0.1:${(receiver.address() as AddressInfo).port}/1`;
  const priorDSNs = [process.env.SENTRY_DSN, process.env.NEXT_PUBLIC_SENTRY_DSN];
  process.env.SENTRY_DSN = dsn;
  process.env.NEXT_PUBLIC_SENTRY_DSN = dsn;
  t.after(() => {
    for (const [index, key] of ["SENTRY_DSN", "NEXT_PUBLIC_SENTRY_DSN"].entries()) {
      const previous = priorDSNs[index];
      if (previous === undefined) delete process.env[key];
      else process.env[key] = previous;
    }
  });

  const { cookieConsentStore } = await import("../src/hooks/ui/useCookieConsent");
  const { default: CookiePolicy } = await import("../src/page-modules/legal/CookiePolicy");
  const Sentry = await import("@sentry/nextjs");
  t.after(async () => {
    await Sentry.close(5_000);
  });
  const initialConsent = cookieConsentStore.getState();
  t.after(() => cookieConsentStore.setState(initialConsent));

  for (const runtime of ["browser", "server"] as const) {
    if (runtime === "browser") await import("../sentry.client.config");
    else await import("../sentry.server.config");
    const options = Sentry.getClient()!.getOptions();
    assert.equal(options.sendDefaultPii, false);
    assert.equal(options.tracesSampleRate, 0);
    if (runtime === "browser") {
      assert.equal(typeof options.beforeSend, "function");
      assert.ok("replaysSessionSampleRate" in options);
      assert.ok("replaysOnErrorSampleRate" in options);
      assert.equal(options.replaysSessionSampleRate, 0);
      assert.equal(options.replaysOnErrorSampleRate, 0);
    } else {
      assert.equal(options.beforeSend, undefined);
    }

    storage.clear();
    cookieConsentStore.setState({ value: null, loading: true });
    cookieConsentStore.getState().hydrate();
    for (const state of ["unset", "declined", "accepted", "withdrawn"] as const) {
      if (state === "accepted") cookieConsentStore.getState().accept();
      if (state === "declined" || state === "withdrawn") cookieConsentStore.getState().decline();
      const expected = state === "unset" ? null : state === "accepted" ? "accepted" : "declined";
      assert.equal(cookieConsentStore.getState().value, expected);
      assert.equal(storage.getItem("hyperpax_cookie_consent"), expected);

      const { html, text } = pageText(CookiePolicy);
      assert.match(html, /<h1[^>]*>Cookie Policy<\/h1>/);
      assert.match(text, /before you choose a preference, after 'Accept all cookies', after 'Essential only', and after you withdraw optional consent/);
      assert.match(text, /Error-event transmission is separate from placing cookies/);
      assert.match(text, /error messages, stack traces, page or request URLs, browser and device information, breadcrumbs, environment and release identifiers/);
      assert.match(text, /not guaranteed to be anonymous or aggregated/);
      assert.match(text, /server configuration does not apply that same redaction/);
      assert.match(text, /retained according to the deployed Sentry project's retention settings/);
      assert.match(text, /does not configure a fixed retention period or automatically delete reports/);
      assert.doesNotMatch(text, /These cookies are only placed if|Captures anonymised information|Retention: up to 24 months/);

      const message = `consent-${runtime}-${state}`;
      Sentry.captureException(new Error(message));
      assert.equal(await Sentry.flush(5_000), true);
      assert.ok(envelopes.some((envelope) => envelope.includes(message)), `${message} must reach the real local Sentry transport`);
    }
    await Sentry.close(5_000);
    // close disables the client but Next.js init still sees it as initialized.
    // Detach it so each production configuration initializes its own client.
    Sentry.getCurrentScope().setClient(undefined);
    assert.equal(Sentry.getClient(), undefined);
  }
}

async function privacyOffchainRecordsDescribeFieldsRetentionAndDeletion(t: TestContext): Promise<void> {
  installReact(t);
  const { default: PrivacyPolicy } = await import("../src/page-modules/legal/PrivacyPolicy");
  const { html, text } = pageText(PrivacyPolicy);
  assert.match(html, /<h1[^>]*>Privacy Policy<\/h1>/);
  assert.match(text, /wallet address, challenge nonce, sign-in message, issue and expiry timestamps, and a used-at timestamp/);
  assert.match(text, /valid for five minutes; a bearer token is valid for one hour, with refresh limited to 24 hours/);
  assert.match(text, /referral code, the referring wallet address, the time a referral was applied and total rewards earned/);
  assert.match(text, /competition identifier, wallet address and join time/);
  assert.match(text, /rankings may display your wallet address and score publicly/);
  assert.match(text, /alert identifiers, alert types, market identifiers, thresholds, enabled state, creation and trigger times/);
  assert.match(text, /does not implement automatic expiry-based deletion.*or a self-service deletion endpoint/);
  assert.match(text, /expiry prevents authentication use; it does not erase a database row/);
  assert.match(text, /Disconnecting a wallet, clearing browser storage or withdrawing optional cookie consent does not delete these records/);
  assert.match(text, /Alert records remain until deleted using the authenticated alert controls/);
  assert.match(text, /No fixed maximum retention period.*is enforced/);
  assert.match(text, /Requests require verification of wallet ownership/);
  assert.match(html, /href="mailto:privacy@paxeer.network"/);
  assert.match(text, /Deleting an off-chain record cannot erase immutable public-chain history/);
  assert.doesNotMatch(text, /We do not retain wallet addresses or transaction data beyond|Anonymised analytics data is retained for up to 24 months|does not require account registration/);
}

async function termsNativeVenueDisclosesExecutionGovernanceAndCustody(t: TestContext): Promise<void> {
  installReact(t);
  const { default: TermsOfService } = await import("../src/page-modules/legal/TermsOfService");
  const { html, text } = pageText(TermsOfService);
  assert.match(html, /<h1[^>]*>Terms of Service<\/h1>/);
  assert.match(text, /native Go chain software in modules\/perps/);
  assert.match(text, /0x0000000000000000000000000000000000001015/);
  assert.match(text, /Perps.sol file defines the calling interface; it is not the venue's execution implementation/);
  assert.match(text, /USDL is a separate ERC20 collateral token/);
  assert.match(text, /governance can change venue parameters, including fees, margin requirements, market configuration and pause controls/);
  assert.match(text, /software upgrades can change native execution rules/);
  assert.match(text, /Depositing USDL transfers ERC20 collateral to the perps module account's EVM address/);
  assert.match(text, /Withdrawals depend on available margin, venue rules and successful chain execution/);
  assert.match(text, /make no claim that any particular release or component has been independently audited/);
  assert.match(text, /audit report.*must identify its exact release or code revision and component scope/);
  assert.doesNotMatch(text, /consists of open-source, audited smart contracts|Despite audits|We have no ability to.*freeze funds|not operate, control, or have custody over the Protocol/);
}

test(cookieErrorConsentMatchesOperationalCapture);
test(privacyOffchainRecordsDescribeFieldsRetentionAndDeletion);
test(termsNativeVenueDisclosesExecutionGovernanceAndCustody);
