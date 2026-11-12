import { createHumanApiClient, type HumanApiClient, type HumanApiClientOptions } from "./generated/index.ts";
import { csrfTokenFromCookie } from "../auth/session.ts";

export * from "./generated/index.ts";

export function humanApi(options: HumanApiClientOptions = {}): HumanApiClient {
  return createHumanApiClient({
    baseUrl: "/human",
    credentials: "include",
    csrfToken: browserCsrfToken,
    trace: browserTrace,
    ...options,
  });
}

export function abortableHumanApi(
  signal: () => AbortSignal,
  options: HumanApiClientOptions = {},
): HumanApiClient {
  const transport = options.fetch ?? globalThis.fetch;
  return humanApi({
    ...options,
    fetch: (input, init) => transport(input, { ...init, signal: signal() }),
  });
}

function browserCsrfToken(): string | undefined {
  if (typeof document === "undefined") {
    return undefined;
  }
  return csrfTokenFromCookie(document.cookie);
}

function browserTrace(): string | undefined {
  if (typeof crypto === "undefined" || typeof crypto.getRandomValues !== "function") {
    return undefined;
  }
  const entropy = globalThis.crypto.getRandomValues(new Uint8Array(16));
  return `trc_${Array.from(entropy, (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}
