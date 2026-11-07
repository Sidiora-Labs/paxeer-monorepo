import { CURRENT_ROUTES } from "./matrix.ts";

export const SUPPORT_REPORT_ROUTES = CURRENT_ROUTES;

export type SupportReportRoute = (typeof SUPPORT_REPORT_ROUTES)[number];

export function supportReportRoute(value: string): SupportReportRoute {
  const normalized = value.trim();
  const template = SUPPORT_REPORT_ROUTES.find((route) => route === normalized);
  if (template === undefined) {
    throw new Error("Invalid report route");
  }
  return template;
}

export function redactSupportReportRoute(value: string): SupportReportRoute {
  const normalized = value.trim();
  const declared = SUPPORT_REPORT_ROUTES.find((route) => route === normalized);
  if (declared !== undefined) return declared;
  if (normalized.length > 160 || !normalized.startsWith("/")) {
    throw new Error("Invalid report route");
  }
  const segments = normalized.split("/");
  const template = SUPPORT_REPORT_ROUTES.find((route) => {
    const expected = route.split("/");
    return expected.length === segments.length && expected.every((segment, index) => {
      const supplied = segments[index];
      if (supplied === undefined) return false;
      if (!segment.startsWith("[")) return segment === supplied;
      let identifier: string;
      try {
        identifier = decodeURIComponent(supplied);
      } catch {
        return false;
      }
      return /^[A-Za-z0-9_:-]+$/u.test(identifier);
    });
  });
  if (template === undefined) {
    throw new Error("Invalid report route");
  }
  return template;
}
