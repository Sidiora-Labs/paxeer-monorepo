export type AgentConnection = {
  status: 'ready';
  entry: { command: string; args: string[] };
  environment: { gatewayUrl: string; dataUrl: string; keyFile: string };
  endpoints: { walletGateway: string; data: string };
} | { status: 'unavailable' };

export const AGENT_INTERFACE_PATH = '/agents/interface.json';

export function agentClientConfiguration(connection: Extract<AgentConnection, { status: 'ready' }>): string {
  return JSON.stringify({
    mcpServers: {
      launchpad: {
        command: connection.entry.command,
        args: connection.entry.args,
        env: {
          [connection.environment.gatewayUrl]: connection.endpoints.walletGateway,
          [connection.environment.dataUrl]: connection.endpoints.data,
          [connection.environment.keyFile]: '<AGENT_KEY_FILE>',
        },
      },
    },
  }, null, 2);
}
