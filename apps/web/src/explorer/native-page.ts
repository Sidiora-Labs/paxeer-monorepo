import {
  ExplorerHttpError,
  ExplorerUnavailableError,
  NativeCursorRefusedError,
  nativeUnifiedAccount,
} from "./client.ts";
import { nativeAddress, nativeCursor, type NativeUnifiedAccount, type NativeCursorRefusalReason } from "./native-model.ts";

export type NativeAccountPageModel =
  | Readonly<{ state: "ready"; account: NativeUnifiedAccount; refreshHref: string; firstHref: string; nextHref?: string }>
  | Readonly<{ state: "not-found" }>
  | Readonly<{ state: "cursor-refused"; reason: NativeCursorRefusalReason; resetHref: string }>
  | Readonly<{ state: "unavailable"; status?: number; responseBody?: unknown }>;

export function nativeAccountPagePath(identifier: string, cursor?: string): string {
  const path = `/explorer/accounts/${encodeURIComponent(nativeAddress(identifier).toLowerCase())}`;
  return cursor === undefined ? path : `${path}?${new URLSearchParams({ cursor: nativeCursor(cursor) })}`;
}

export async function loadNativeAccountPage(identifier: string, cursor?: string): Promise<NativeAccountPageModel> {
  const resetHref = nativeAccountPagePath(identifier);
  try {
    const account = await nativeUnifiedAccount(identifier, cursor);
    if (account === undefined) return Object.freeze({ state: "not-found" });
    return Object.freeze({
      state: "ready",
      account,
      refreshHref: nativeAccountPagePath(identifier, account.page_cursor),
      firstHref: nativeAccountPagePath(identifier, account.first_page_cursor),
      ...(account.next_page_params === null ? {} : { nextHref: nativeAccountPagePath(identifier, account.next_page_params.cursor) }),
    });
  } catch (error) {
    if (error instanceof NativeCursorRefusedError) {
      return Object.freeze({ state: "cursor-refused", reason: error.reason, resetHref });
    }
    if (error instanceof ExplorerHttpError) {
      return Object.freeze({ state: "unavailable", status: error.status, responseBody: error.responseBody });
    }
    if (error instanceof ExplorerUnavailableError) return Object.freeze({ state: "unavailable" });
    throw error;
  }
}
