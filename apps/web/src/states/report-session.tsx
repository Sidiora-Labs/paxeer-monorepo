"use client";

import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react";
import type { VerifiedSessionIdentity } from "../auth/session.ts";
import { BrowserSupportReportOutbox, browserSupportReportOutbox, clearPrivateSupportReports } from "./report.ts";

const SupportReportContext = createContext(browserSupportReportOutbox);

export function SupportReportSessionBoundary({ session, children }: Readonly<{
  session: VerifiedSessionIdentity | undefined;
  children: ReactNode;
}>) {
  const outbox = useMemo(() => new BrowserSupportReportOutbox(session),
    [session?.principalScope, session?.sessionId, session?.accountId]);
  useEffect(() => {
    if (session === undefined) clearPrivateSupportReports(window.localStorage);
    else {
      const disconnect = outbox.connect();
      const flush = () => { void Promise.resolve().then(() => outbox.flushPending()).catch(() => undefined); };
      flush();
      window.addEventListener("online", flush);
      return () => { window.removeEventListener("online", flush); disconnect(); };
    }
    return () => { outbox.close(); };
  }, [outbox]);
  return <SupportReportContext.Provider value={outbox}>{children}</SupportReportContext.Provider>;
}

export function useSupportReportOutbox(): BrowserSupportReportOutbox {
  return useContext(SupportReportContext);
}
