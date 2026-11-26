import {
  AGENT_ENV,
  type AgentWorkspaceInterface,
} from "@paxeer-network/agent-kit/interface";

export const AGENT_INTERFACE_PATH = "/agents/interface.json";
export const AGENT_KEY_FILE_PLACEHOLDER = "<path-to-agent-key-file>";

export function agentMcpConfiguration(
  document: AgentWorkspaceInterface,
  did: string,
) {
  if (!/^did:matrix:[A-Za-z0-9_-]{1,128}:[0-9a-f]{16}$/.test(did))
    throw new Error("Select an agent with a valid DID");
  return {
    mcpServers: {
      "paxeer-perps": {
        command: document.entry.command,
        args: document.entry.args,
        cwd: document.entry.cwd,
        env: {
          [AGENT_ENV.walletGateway]: document.connection.walletGatewayUrl,
          [AGENT_ENV.node]: document.connection.nodeUrl,
          [AGENT_ENV.dataGateway]: document.connection.dataGatewayUrl,
          [AGENT_ENV.keyFile]: AGENT_KEY_FILE_PLACEHOLDER,
          [AGENT_ENV.label]: did.split(":")[2],
        },
      },
    },
  };
}
