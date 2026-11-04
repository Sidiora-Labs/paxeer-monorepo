import { expect, test as base, type BrowserContext, type Page } from "@playwright/test";

import { ACTIVE_ACCOUNT_STORAGE_KEY } from "../src/auth/session.ts";
import { establishPublicSession, type PublicSession } from "./public-session.ts";
import { JourneyFixtureStore } from "./journey-fixtures.ts";

export async function installJourneyPeer(context: BrowserContext, store: JourneyFixtureStore): Promise<void> {
  await context.route("**/human/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const answer = store.answer({
      method: request.method(), path: `${url.pathname}${url.search}`,
      body: request.postData() ?? undefined, idempotencyKey: request.headers()["idempotency-key"]
    });
    if (answer === undefined) { await route.continue(); return; }
    await route.fulfill({ status: answer.status, contentType: "application/json", body: answer.body });
  });
}

export async function installPasskey(context: BrowserContext, session: PublicSession, page: Page): Promise<void> {
  const device = await context.newCDPSession(page);
  await device.send("WebAuthn.enable");
  const { authenticatorId } = await device.send("WebAuthn.addVirtualAuthenticator", {
    options: {
      protocol: "ctap2", transport: "internal", hasResidentKey: true,
      hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true
    },
  });
  await device.send("WebAuthn.addCredential", { authenticatorId, credential: session.authenticator.browserCredential() });
}

export const test = base.extend<{ publicSession: PublicSession; journeyPeer: JourneyFixtureStore; }>({
  publicSession: async ({ context, page, baseURL }, use) => {
    expect(baseURL).toBeDefined();
    const session = await establishPublicSession(context, baseURL!);
    await installPasskey(context, session, page);
    await context.addInitScript(({ key, account }) => localStorage.setItem(key, account),
      { key: ACTIVE_ACCOUNT_STORAGE_KEY, account: session.accountId });
    await use(session);
  },
  journeyPeer: async ({ context, publicSession, baseURL }, use) => {
    const store = new JourneyFixtureStore(new URL(baseURL!).origin, publicSession.authenticator.publicKey());
    await installJourneyPeer(context, store);
    await use(store);
  },
});

export async function requirePost(page: Page, path: string, work: () => Promise<void>): Promise<void> {
  const observed = page.waitForResponse((response) => response.request().method() === "POST"
    && new URL(response.url()).pathname === path);
  await work();
  const response = await observed;
  expect(response.request().headers()["content-type"]).toContain("application/json");
  const envelope: unknown = await response.json();
  expect(envelope).toHaveProperty("ok");
  expect(envelope).toHaveProperty("trace");
}

export async function requireDecision(store: JourneyFixtureStore, approvalId: string, state: "approved" | "rejected"): Promise<void> {
  await expect.poll(() => store.decisions.get(approvalId)?.state).toBe(state);
  expect(store.decisions.size).toBe(1);
  expect(store.decisions.get(approvalId)?.approval_id).toBe(approvalId);
  expect(store.decisions.get(approvalId)?.money_moved).toBe(false);
  expect(store.releases.size).toBe(state === "approved" ? 1 : 0);
}
