"use client";

import { usePathname } from "next/navigation";
import { useReportWebVitals } from "next/web-vitals";
import { useCallback, useRef } from "react";

import { collectWebVital, type ReportedWebVital } from "./collect-web-vital";

export function WebVitalsRum() {
  const pathname = usePathname();
  const currentPathname = useRef(pathname);
  currentPathname.current = pathname;
  const report = useCallback((metric: ReportedWebVital) => {
    collectWebVital(currentPathname.current, metric);
  }, []);
  useReportWebVitals(report);
  return null;
}
