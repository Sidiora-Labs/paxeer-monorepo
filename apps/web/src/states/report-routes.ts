export const SUPPORT_REPORT_ROUTES = Object.freeze([
  "/",
  "/app",
  "/app/move",
  "/app/activity",
  "/app/activity/[entryId]",
  "/app/settings",
  "/app/settings/devices",
  "/app/settings/security",
  "/app/settings/wallet",
  "/app/security/devices",
  "/app/security/recovery",
  "/app/support",
  "/app/approvals",
  "/app/approvals/[approvalId]",
  "/app/notifications",
  "/app/agents",
  "/app/agents/new",
  "/app/agents/[agentId]",
  "/explorer",
  "/explorer/checkpoints",
  "/explorer/checkpoints/[checkpointId]",
  "/explorer/batches",
  "/explorer/batches/[batchNumber]",
  "/explorer/receipts/[receiptId]",
  "/explorer/accounts/[accountId]",
  "/explorer/programs/[programId]",
  "/bridge",
  "/exchange",
  "/launchpad",
] as const);

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
