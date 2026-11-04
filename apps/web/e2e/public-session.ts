import { randomUUID } from "node:crypto";

import type { BrowserContext } from "@playwright/test";

import { createHumanApiClient, type FetchLike, type HumanApiClient, type Session } from "../src/api/generated/index.ts";
import { singleCurrentSessionId } from "../src/auth/session.ts";
import { SoftwareAuthenticator } from "./software-authenticator.ts";

const SESSION_COOKIES = ["__Host-layerx_access", "__Host-layerx_refresh", "__Host-layerx_csrf"] as const;

export async function publicSessionClient(context: BrowserContext, baseUrl: string): Promise<HumanApiClient> {
  const endpoint = new URL(baseUrl);
  if (endpoint.protocol !== "https:" || endpoint.port !== "" || endpoint.pathname !== "/"
    || endpoint.search !== "" || endpoint.hash !== "" || endpoint.username !== ""
    || endpoint.password !== "") throw new Error("Public session requires the HTTPS application origin");
  const origin = endpoint.origin;
  if ((await context.cookies(origin)).some((cookie) => SESSION_COOKIES.some((name) => name === cookie.name))) {
    throw new Error("Public session setup requires a fresh browser context");
  }
  let csrf: string | undefined;
  const transport: FetchLike = async (input, init) => {
    if (new URL(input).origin !== origin || (init.body !== undefined && typeof init.body !== "string")) {
      throw new Error("Public API request left the application origin or encoding");
    }
    const headers = new Headers(init.headers);
    headers.set("Origin", origin);
    const response = await context.request.fetch(input, {
      method: init.method ?? "GET",
      headers: Object.fromEntries(headers),
      ...(typeof init.body === "string" ? { data: init.body } : {}),
      maxRedirects: 0,
      timeout: 30_000,
    });
    try {
      if (response.status() >= 300 && response.status() < 400) {
        throw new Error("Public API redirected the authenticated request");
      }
      const bytes = await response.body();
      if (bytes.length > 1_048_576) throw new Error("Public API response exceeds its bound");
      csrf = (await context.cookies(origin)).find((cookie) => cookie.name === "__Host-layerx_csrf")?.value;
      return new Response(new Uint8Array(bytes), { status: response.status(), headers: response.headers() });
    } finally {
      await response.dispose();
    }
  };
  return createHumanApiClient({ baseUrl: `${origin}/human`, fetch: transport, csrfToken: () => csrf });
}

export interface PublicSession {
  readonly accountId: string;
  readonly session: Session;
  readonly client: HumanApiClient;
  readonly authenticator: SoftwareAuthenticator;
  readonly email: string;
  readonly passkeyId: string;
}

export async function establishPublicSession(context: BrowserContext, baseUrl: string): Promise<PublicSession> {
  const origin = new URL(baseUrl).origin;
  const client = await publicSessionClient(context, baseUrl);
  const authenticator = new SoftwareAuthenticator(origin);
  const email = `browser-${randomUUID()}@paxeer.network`;
  const account = await client.accountCreate({ email, display_name: "Browser qualification" }, randomUUID());
  const registration = await client.passkeyRegisterBegin({ account_id: account.account_id });
  const passkey = await client.passkeyRegisterFinish(registration.registration_id, {
    credential: authenticator.register(registration.ceremony),
  });
  const session = await openPublicSession(client, email, authenticator, passkey.passkey_id);
  const endpoint = new URL(baseUrl);
  const cookies = await context.cookies(origin);
  for (const name of SESSION_COOKIES) {
    const matches = cookies.filter((cookie) => cookie.name === name);
    const cookie = matches.at(0);
    if (matches.length !== 1 || cookie === undefined || cookie.value.length === 0
      || !cookie.secure || cookie.path !== "/" || cookie.domain !== endpoint.hostname
      || cookie.sameSite !== "Strict" || (name !== "__Host-layerx_csrf" && !cookie.httpOnly)) {
      throw new Error("Public session cookie is missing or has invalid security attributes");
    }
  }
  const [balance, sessions] = await Promise.all([client.accountBalance(), client.sessionList()]);
  if (balance.account_id !== account.account_id || singleCurrentSessionId(sessions.sessions) !== session.session_id) {
    throw new Error("Authenticated reads do not identify the newly opened account and session");
  }
  if (passkey.passkey_id.length === 0) throw new Error("Registration returned no passkey identity");
  return Object.freeze({ accountId: account.account_id, session, client, authenticator, email, passkeyId: passkey.passkey_id });
}

export async function openPublicSession(client: HumanApiClient, email: string, authenticator: SoftwareAuthenticator, passkeyId: string): Promise<Session> {
  const challenge = await client.passkeyAssertBegin({ email });
  const assertion = await client.passkeyAssertFinish(challenge.assertion_id, {
    credential: authenticator.assert(challenge.ceremony),
  });
  if (assertion.assertion_id !== challenge.assertion_id || assertion.passkey_id !== passkeyId) {
    throw new Error("Public assertion is not bound to a registered passkey");
  }
  const session = await client.sessionOpen({
    assertion_id: assertion.assertion_id,
    device: { label: "Browser qualification", platform: "Playwright" },
  }, randomUUID());
  if (!session.current) throw new Error("Public API did not open the current session");
  return session;
}

export async function signInPublicSession(context: BrowserContext, baseUrl: string, first: PublicSession): Promise<PublicSession> {
  const client = await publicSessionClient(context, baseUrl);
  const session = await openPublicSession(client, first.email, first.authenticator, first.passkeyId);
  const [balance, sessions] = await Promise.all([client.accountBalance(), client.sessionList()]);
  if (balance.account_id !== first.accountId || singleCurrentSessionId(sessions.sessions) !== session.session_id
    || session.session_id === first.session.session_id || session.device.device_id === first.session.device.device_id) {
    throw new Error("Second device did not establish an isolated session for the same account");
  }
  return Object.freeze({ ...first, client, session });
}
