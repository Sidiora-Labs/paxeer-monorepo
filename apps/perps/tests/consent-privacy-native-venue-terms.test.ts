import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { after, test, type TestContext } from "node:test";
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import type { ComponentType } from "react";

const webRequire = createRequire(
  new URL("../apps/web/package.json", import.meta.url),
);
const backendRequire = createRequire(
  new URL("../packages/paxeer-backend/package.json", import.meta.url),
);
const React: typeof import("react") = webRequire("react");
const { createElement } = React;
const { renderToStaticMarkup }: typeof import("react-dom/server") =
  webRequire("react-dom/server");

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

function installReact(t: TestContext): void {
  const previous = Object.getOwnPropertyDescriptor(globalThis, "React");
  Object.defineProperty(globalThis, "React", {
    value: React,
    configurable: true,
  });
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

async function cookieErrorConsentMatchesOperationalCapture(
  t: TestContext,
): Promise<void> {
  installReact(t);
  const storage = localStorage;
  storage.clear();

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
  const priorDSNs = [
    process.env.SENTRY_DSN,
    process.env.NEXT_PUBLIC_SENTRY_DSN,
  ];
  process.env.SENTRY_DSN = dsn;
  process.env.NEXT_PUBLIC_SENTRY_DSN = dsn;
  t.after(() => {
    for (const [index, key] of [
      "SENTRY_DSN",
      "NEXT_PUBLIC_SENTRY_DSN",
    ].entries()) {
      const previous = priorDSNs[index];
      if (previous === undefined) delete process.env[key];
      else process.env[key] = previous;
    }
  });

  const { cookieConsentStore } =
    await import("../apps/web/src/legal/useCookieConsent");
  const { default: CookiePolicy } =
    await import("../apps/web/src/legal/CookiePolicy");
  const Sentry: typeof import("../packages/paxeer-backend/node_modules/@sentry/nextjs") =
    backendRequire("@sentry/nextjs");
  const { initializeErrorReporting } =
    await import("../apps/web/src/legal/useCookieConsent");
  t.after(async () => {
    await Sentry.close(5_000);
  });
  const initialConsent = cookieConsentStore.getState();
  t.after(() => cookieConsentStore.setState(initialConsent));

  for (const runtime of ["browser", "server"] as const) {
    await initializeErrorReporting(runtime);
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
    for (const state of [
      "unset",
      "declined",
      "accepted",
      "withdrawn",
    ] as const) {
      if (state === "accepted") cookieConsentStore.getState().accept();
      if (state === "declined" || state === "withdrawn")
        cookieConsentStore.getState().decline();
      const expected =
        state === "unset"
          ? null
          : state === "accepted"
            ? "accepted"
            : "declined";
      assert.equal(cookieConsentStore.getState().value, expected);
      assert.equal(storage.getItem("hyperpax_cookie_consent"), expected);

      const { html, text } = pageText(CookiePolicy);
      assert.match(html, /<h1[^>]*>Cookie Policy<\/h1>/);
      assert.match(
        text,
        /browser error reporting starts only after 'Accept all cookies' and stops when you select 'Essential only'/,
      );
      assert.match(
        text,
        /Withdrawing optional consent stops browser error reporting but does not stop server error reporting or erase reports already sent/,
      );
      assert.match(
        text,
        /before you choose a preference, after 'Accept all cookies', after 'Essential only', and after you withdraw optional consent/,
      );
      assert.match(
        text,
        /Error-event transmission is separate from placing cookies/,
      );
      assert.match(
        text,
        /error messages, stack traces, page or request URLs, browser and device information, breadcrumbs, environment and release identifiers/,
      );
      assert.match(text, /not guaranteed to be anonymous or aggregated/);
      assert.match(
        text,
        /server configuration does not apply that same redaction/,
      );
      assert.match(
        text,
        /retained according to the deployed Sentry project's retention settings/,
      );
      assert.match(
        text,
        /does not configure a fixed retention period or automatically delete reports/,
      );
      assert.doesNotMatch(
        text,
        /These cookies are only placed if|Captures anonymised information|Retention: up to 24 months/,
      );

      const message = `consent-${runtime}-${state}`;
      Sentry.captureException(new Error(message));
      assert.equal(await Sentry.flush(5_000), true);
      assert.ok(
        envelopes.some((envelope) => envelope.includes(message)),
        `${message} must reach the real local Sentry transport`,
      );
    }
    await Sentry.close(5_000);
    // close disables the client but Next.js init still sees it as initialized.
    // Detach it so each production configuration initializes its own client.
    Sentry.getCurrentScope().setClient(undefined);
    assert.equal(Sentry.getClient(), undefined);
  }
}

async function privacyOffchainRecordsDescribeFieldsRetentionAndDeletion(
  t: TestContext,
): Promise<void> {
  installReact(t);
  const { default: PrivacyPolicy } =
    await import("../apps/web/src/legal/PrivacyPolicy");
  const { html, text } = pageText(PrivacyPolicy);
  assert.match(html, /<h1[^>]*>Privacy Policy<\/h1>/);
  assert.match(
    text,
    /browser exceptions are transmitted only after optional consent and stop when consent is withdrawn; server exceptions may be transmitted regardless of cookie preference/,
  );
  assert.match(
    text,
    /wallet address, challenge nonce, sign-in message, issue and expiry timestamps, and a used-at timestamp/,
  );
  assert.match(
    text,
    /valid for five minutes; a bearer token is valid for one hour, with refresh limited to 24 hours/,
  );
  assert.match(
    text,
    /referral code, the referring wallet address, the time a referral was applied and total rewards earned/,
  );
  assert.match(text, /competition identifier, wallet address and join time/);
  assert.match(
    text,
    /rankings may display your wallet address and score publicly/,
  );
  assert.match(
    text,
    /alert identifiers, alert types, market identifiers, thresholds, enabled state, creation and trigger times/,
  );
  assert.match(
    text,
    /does not implement automatic expiry-based deletion.*or a self-service deletion endpoint/,
  );
  assert.match(
    text,
    /expiry prevents authentication use; it does not erase a database row/,
  );
  assert.match(
    text,
    /Disconnecting a wallet, clearing browser storage or withdrawing optional cookie consent does not delete these records/,
  );
  assert.match(
    text,
    /Alert records remain until deleted using the authenticated alert controls/,
  );
  assert.match(text, /No fixed maximum retention period.*is enforced/);
  assert.match(text, /Requests require verification of wallet ownership/);
  assert.match(html, /href="mailto:privacy@paxeer.network"/);
  assert.match(
    text,
    /Deleting an off-chain record cannot erase immutable public-chain history/,
  );
  assert.doesNotMatch(
    text,
    /We do not retain wallet addresses or transaction data beyond|Anonymised analytics data is retained for up to 24 months|does not require account registration/,
  );
}

async function termsNativeVenueDisclosesExecutionGovernanceAndCustody(
  t: TestContext,
): Promise<void> {
  installReact(t);
  const { default: TermsOfService } =
    await import("../apps/web/src/legal/TermsOfService");
  const { html, text } = pageText(TermsOfService);
  assert.match(html, /<h1[^>]*>Terms of Service<\/h1>/);
  assert.match(text, /native Go chain software in modules\/perps/);
  assert.match(text, /0x0000000000000000000000000000000000001015/);
  assert.match(
    text,
    /Perps.sol file defines the calling interface; it is not the venue's execution implementation/,
  );
  assert.match(text, /USDL is a separate ERC20 collateral token/);
  assert.match(
    text,
    /governance can change venue parameters, including fees, margin requirements, market configuration and pause controls/,
  );
  assert.match(text, /software upgrades can change native execution rules/);
  assert.match(
    text,
    /Depositing USDL transfers ERC20 collateral to the perps module account's EVM address/,
  );
  assert.match(
    text,
    /Withdrawals depend on available margin, venue rules and successful chain execution/,
  );
  assert.match(
    text,
    /make no claim that any particular release or component has been independently audited/,
  );
  assert.match(
    text,
    /audit report.*must identify its exact release or code revision and component scope/,
  );
  assert.doesNotMatch(
    text,
    /consists of open-source, audited smart contracts|Despite audits|We have no ability to.*freeze funds|not operate, control, or have custody over the Protocol/,
  );
}

async function applicationErrorReportingWaitsForConsentAndStopsOnWithdrawal(
  t: TestContext,
): Promise<void> {
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
  receiver.listen(0, "127.0.0.1");
  await once(receiver, "listening");
  const oldDSN = process.env.NEXT_PUBLIC_SENTRY_DSN;
  process.env.NEXT_PUBLIC_SENTRY_DSN = `http://fixture@127.0.0.1:${(receiver.address() as AddressInfo).port}/1`;
  const Sentry: typeof import("../packages/paxeer-backend/node_modules/@sentry/nextjs") =
    backendRequire("@sentry/nextjs");
  const { cookieConsentStore, startConsentErrorReporting } =
    await import("../apps/web/src/legal/useCookieConsent");
  const initialState = cookieConsentStore.getState();
  localStorage.clear();
  cookieConsentStore.setState({ value: null, loading: true });
  Sentry.getCurrentScope().setClient(undefined);
  const stop = startConsentErrorReporting();
  t.after(async () => {
    stop();
    await Sentry.close(5_000);
    Sentry.getCurrentScope().setClient(undefined);
    cookieConsentStore.setState(initialState);
    if (oldDSN === undefined) delete process.env.NEXT_PUBLIC_SENTRY_DSN;
    else process.env.NEXT_PUBLIC_SENTRY_DSN = oldDSN;
    receiver.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      receiver.close((error) => (error ? reject(error) : resolve())),
    );
  });

  assert.equal(Sentry.getClient(), undefined);
  cookieConsentStore.getState().hydrate();
  assert.equal(Sentry.getClient(), undefined);
  assert.equal(localStorage.length, 0);
  Sentry.captureException(new Error("application-unset"));
  assert.equal(await Sentry.flush(5_000), false);
  cookieConsentStore.getState().decline();
  assert.equal(Sentry.getClient(), undefined);
  Sentry.captureException(new Error("application-declined"));
  assert.equal(await Sentry.flush(5_000), false);
  assert.equal(envelopes.length, 0);

  cookieConsentStore.getState().accept();
  const deadline = Date.now() + 5_000;
  while (!Sentry.getClient() && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.ok(Sentry.getClient(), "accepting enables the configured reporter");
  assert.equal(typeof Sentry.getClient()!.getOptions().beforeSend, "function");
  Sentry.captureException(new Error("application-accepted"));
  assert.equal(await Sentry.flush(5_000), true);
  assert.ok(
    envelopes.some((envelope) => envelope.includes("application-accepted")),
  );

  cookieConsentStore.getState().decline();
  assert.equal(
    await Sentry.getClient()!.getOptions().beforeSend!(
      { message: "application-withdrawn" },
      {},
    ),
    null,
  );
  Sentry.captureException(new Error("application-withdrawn"));
  assert.equal(await Sentry.flush(5_000), true);
  assert.equal(
    envelopes.some((envelope) => envelope.includes("application-withdrawn")),
    false,
  );
  assert.equal(localStorage.getItem("hyperpax_cookie_consent"), "declined");
  assert.equal(localStorage.length, 1);

  cookieConsentStore.getState().accept();
  Sentry.captureException(new Error("application-accepted-again"));
  assert.equal(await Sentry.flush(5_000), true);
  assert.ok(
    envelopes.some((envelope) =>
      envelope.includes("application-accepted-again"),
    ),
  );
  stop();
  Sentry.captureException(new Error("application-stopped"));
  assert.equal(await Sentry.flush(5_000), true);
  assert.equal(
    envelopes.some((envelope) => envelope.includes("application-stopped")),
    false,
  );
}

async function legalRoutesRenderTheirOwnPoliciesAndNavigation(
  t: TestContext,
): Promise<void> {
  installReact(t);
  const routes = [
    ["terms", "Terms of Service"],
    ["privacy", "Privacy Policy"],
    ["cookies", "Cookie Policy"],
  ] as const;
  for (const [route, title] of routes) {
    const { default: Page } = await import(
      `../apps/web/src/pages/${route}.tsx`
    );
    const { html } = pageText(Page);
    assert.match(html, new RegExp(`<h1[^>]*>${title}</h1>`));
    for (const [destination] of routes)
      assert.ok(html.includes(`href="/${destination}"`));
    assert.ok(html.includes('href="/"'));
  }
  const { CookieBanner } = await import("../apps/web/src/legal/CookieBanner");
  assert.equal(renderToStaticMarkup(createElement(CookieBanner)), "");
}

async function consentPrivacyAndNativeVenueTerms(): Promise<void> {
  const directory = mkdtempSync(join(tmpdir(), "perps-consent-"));
  const { NODE_TEST_CONTEXT, ...childEnvironment } = process.env;
  try {
    const output = execFileSync(
      process.execPath,
      [
        "--experimental-webstorage",
        `--localstorage-file=${join(directory, "preferences")}`,
        "--test",
        "--test-reporter=tap",
        "--import",
        "tsx",
        fileURLToPath(import.meta.url),
      ],
      {
        stdio: "pipe",
        env: { ...childEnvironment, PAXEER_CONSENT_STORAGE_TEST: "1" },
      },
    );
    process.stdout.write(output);
    assert.match(output.toString(), /# tests 5\b/);
  } catch (error) {
    if (
      error &&
      typeof error === "object" &&
      "stdout" in error &&
      "stderr" in error
    ) {
      process.stdout.write(String(error.stdout));
      process.stderr.write(String(error.stderr));
    }
    throw error;
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

if (process.env.PAXEER_CONSENT_STORAGE_TEST === "1") {
  test(cookieErrorConsentMatchesOperationalCapture);
  test(privacyOffchainRecordsDescribeFieldsRetentionAndDeletion);
  test(termsNativeVenueDisclosesExecutionGovernanceAndCustody);
  test(applicationErrorReportingWaitsForConsentAndStopsOnWithdrawal);
  test(legalRoutesRenderTheirOwnPoliciesAndNavigation);
} else {
  test(consentPrivacyAndNativeVenueTerms);
}
