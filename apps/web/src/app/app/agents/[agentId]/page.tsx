import { cookies } from "next/headers";

import { humanApiForServer } from "../../../../api/server";
import { AgentDetailScreen } from "../../../../journeys/agents";
import type { Agent } from "../../../../api";

export const dynamic = "force-dynamic";

export default async function AgentDetailPage({
  params,
}: Readonly<{ params: Promise<{ agentId: string }> }>) {
  const { agentId } = await params;
  let initialAgent: Agent | undefined;
  try {
    const requestCookies = await cookies();
    initialAgent = await humanApiForServer({ cookie: requestCookies.toString() }).agentGet(agentId);
  } catch {
    initialAgent = undefined;
  }
  return (
    <AgentDetailScreen
      agentId={agentId}
      {...(initialAgent === undefined ? {} : { initialAgent })}
    />
  );
}
