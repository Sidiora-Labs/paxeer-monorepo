import { handleKeysRequest } from "../keys-server";

export const runtime = "nodejs";

export function POST(request: Request): Promise<Response> {
  return handleKeysRequest(request, { gatewayOrigin: process.env.DASHBOARD_GATEWAY_ORIGIN });
}

export function DELETE(request: Request): Promise<Response> {
  return handleKeysRequest(request, { gatewayOrigin: process.env.DASHBOARD_GATEWAY_ORIGIN });
}
