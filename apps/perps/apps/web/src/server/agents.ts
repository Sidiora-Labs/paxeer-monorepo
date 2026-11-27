import {
  describeAgentInterface,
  type AgentWorkspaceInterface,
} from "@paxeer-network/agent-kit/interface";
import { paxeerChain } from "@paxeer-network/paxeer-backend/core/chain";
import { getPaxeerRpcUrls } from "@paxeer-network/paxeer-backend/core/rpc";
import { gatewayUrls } from "@paxeer-network/paxeer-backend/core/network/gateway";
import { PRECOMPILES } from "@paxeer-network/paxeer-backend/core/network/precompiles";

function publicEndpoint(raw: string | undefined): string {
  if (!raw) throw new Error("Agent connection is not configured");
  const url = new URL(raw.trim());
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  )
    throw new Error("Agent connection requires public HTTP endpoints");
  return url.toString().replace(/\/$/, "");
}

export function agentInterfaceDocument(): AgentWorkspaceInterface {
  return {
    ...describeAgentInterface(),
    connection: {
      walletGatewayUrl: publicEndpoint(
        process.env.NEXT_PUBLIC_PAXEER_WALLET_API,
      ),
      nodeUrl: publicEndpoint(getPaxeerRpcUrls()[0]),
      dataGatewayUrl: gatewayUrls().origin,
      chain: { id: paxeerChain.id, name: paxeerChain.name },
      venue: { contract: PRECOMPILES.perps },
    },
  };
}
