import { proxyHumanRequest } from "../../../../api/human-proxy.ts";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

async function humanRoute(
  request: Request,
  context: { params: Promise<{ path: string[] }> },
): Promise<Response> {
  const { path } = await context.params;
  return proxyHumanRequest(request, path);
}

export {
  humanRoute as GET,
  humanRoute as HEAD,
  humanRoute as POST,
  humanRoute as PUT,
  humanRoute as PATCH,
  humanRoute as DELETE,
  humanRoute as OPTIONS,
};
