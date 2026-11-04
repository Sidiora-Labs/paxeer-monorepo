"use client";

import { submitWebVital } from "../api/performance-client";
import { classifyPerformanceRoute, isWebVitalName, normalizeWebVital } from "./budgets";

export type ReportedWebVital = Readonly<{ name: string; value: number }>;

export function collectWebVital(pathname: string, metric: ReportedWebVital): void {
  const route = classifyPerformanceRoute(pathname);
  if (route !== undefined && isWebVitalName(metric.name)) {
    const observed = normalizeWebVital(metric.name, metric.value);
    if (observed !== undefined) {
      submitWebVital(route, metric.name, observed);
    }
  }
}
