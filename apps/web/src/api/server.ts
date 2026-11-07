import "server-only";

import { createHumanApiClient, type HumanApiClient } from "./generated/index.ts";
import { humanServiceOrigin } from "./human-origin.ts";

export interface ServerHumanApiContext {
  readonly cookie: string;
  readonly csrfToken?: string;
  readonly trace?: string;
}

export function humanApiForServer(context: ServerHumanApiContext): HumanApiClient {
  const baseUrl = humanServiceOrigin();
  const origin = process.env.LAYERX_HUMAN_WEB_ORIGIN;
  if (origin === undefined || new URL(origin).origin !== origin || !origin.startsWith("https://")) {
    throw new Error("LAYERX_HUMAN_WEB_ORIGIN must name the HTTPS web application");
  }
  return createHumanApiClient({
    baseUrl,
    credentials: "include",
    headers: { Cookie: context.cookie, Origin: origin },
    csrfToken: () => context.csrfToken,
    trace: () => context.trace,
  });
}
