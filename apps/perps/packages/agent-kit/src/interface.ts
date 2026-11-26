import { z } from "zod";
import packageInfo from "../package.json";
import { AGENT_TOOLS } from "./catalog";

export const AGENT_ENV = {
  walletGateway: "PAXEER_AGENT_GATEWAY_URL",
  node: "PAXEER_AGENT_NODE_URL",
  dataGateway: "NEXT_PUBLIC_PERPS_GATEWAY_URL",
  keyFile: "PAXEER_AGENT_KEY_FILE",
  label: "PAXEER_AGENT_LABEL",
} as const;

export function describeAgentInterface() {
  return {
    name: packageInfo.name,
    version: packageInfo.version,
    entry: {
      command: "node",
      args: ["--import", "tsx", "packages/agent-kit/src/mcp-cli.ts"],
      cwd: "apps/perps",
      transport: "stdio",
    },
    environment: [
      { name: AGENT_ENV.walletGateway, required: true },
      { name: AGENT_ENV.node, required: true },
      { name: AGENT_ENV.dataGateway, required: true },
      { name: AGENT_ENV.keyFile, required: true },
      { name: AGENT_ENV.label, required: false, default: "perps" },
    ],
    tools: Object.values(AGENT_TOOLS).map((tool) => ({
      name: tool.name,
      description: tool.description,
      inputSchema: z.toJSONSchema(tool.inputSchema, {
        target: "draft-7",
        io: "input",
      }),
      readOnly: tool.readOnly,
    })),
  };
}

export type AgentInterface = ReturnType<typeof describeAgentInterface>;

export interface AgentWorkspaceInterface extends AgentInterface {
  connection: {
    walletGatewayUrl: string;
    nodeUrl: string;
    dataGatewayUrl: string;
    chain: { id: number; name: string };
    venue: { contract: string };
  };
}
