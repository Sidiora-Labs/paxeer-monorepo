import { z } from 'zod';
import { TOOL_NAMES, TOOL_DESCRIPTIONS, TOOL_SCHEMAS } from './tools.js';

export const AGENT_KIT_PACKAGE = Object.freeze({ name: '@kindle/agent-kit', version: '0.1.0' });
export const AGENT_KIT_ENTRY = Object.freeze({
  command: 'node',
  args: ['sdk/launchpad/packages/agent-kit/dist/mcp.js'],
});
export const AGENT_KIT_ENVIRONMENT = Object.freeze({
  gatewayUrl: 'KINDLE_GATEWAY_URL',
  dataUrl: 'KINDLE_DATA_URL',
  keyFile: 'KINDLE_AGENT_KEY_FILE',
});

export function agentInterfaceDescription() {
  return {
    package: { ...AGENT_KIT_PACKAGE },
    entry: { command: AGENT_KIT_ENTRY.command, args: [...AGENT_KIT_ENTRY.args] },
    environment: { ...AGENT_KIT_ENVIRONMENT },
    tools: TOOL_NAMES.map(name => {
      const inputSchema = z.toJSONSchema(TOOL_SCHEMAS[name], { target: 'draft-7', io: 'input' });
      delete inputSchema.$schema;
      return {
        name,
        description: TOOL_DESCRIPTIONS[name],
        inputSchema: { ...inputSchema, type: 'object' as const },
      };
    }),
  };
}
