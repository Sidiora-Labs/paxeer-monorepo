import { handleSessionRequest } from "../../session-server";

export const runtime = "nodejs";

export function POST(request: Request): Promise<Response> {
  return handleSessionRequest(request, {
    apiOrigin: process.env.DASHBOARD_API_ORIGIN!,
    gatewayOrigin: process.env.DASHBOARD_GATEWAY_ORIGIN,
  });
}
