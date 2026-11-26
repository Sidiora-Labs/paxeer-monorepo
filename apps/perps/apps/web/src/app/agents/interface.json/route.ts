import { agentInterfaceDocument } from "../../../server/agents";

export function GET(): Response {
  try {
    return Response.json(agentInterfaceDocument(), {
      headers: { "Cache-Control": "no-store" },
    });
  } catch {
    return Response.json(
      {
        error:
          "Agent connection is unavailable. Check the workspace configuration.",
      },
      { status: 503, headers: { "Cache-Control": "no-store" } },
    );
  }
}
