import DashboardClient from "./dashboard-client";
import { gatewayOriginConfigured, readGatewayTenant } from "./runtime-config";

export const dynamic = "force-dynamic";

export default function DashboardPage() {
  return (
    <DashboardClient
      gatewayTenant={readGatewayTenant(process.env.DASHBOARD_GATEWAY_TENANT)}
      gatewayOriginConfigured={gatewayOriginConfigured(
        process.env.DASHBOARD_GATEWAY_ORIGIN,
      )}
    />
  );
}
