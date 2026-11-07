"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import { ErrorBoundary } from "./error";
import { OfflineBanner } from "./offline";

export function ApplicationStateBoundary({ children }: Readonly<{ children: ReactNode }>) {
  const pathname = usePathname();
  return (
    <>
      <OfflineBanner />
      <ErrorBoundary key={pathname} route={pathname}>{children}</ErrorBoundary>
    </>
  );
}
