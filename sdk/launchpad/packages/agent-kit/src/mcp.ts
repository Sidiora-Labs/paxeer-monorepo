#!/usr/bin/env node
import { readFile, stat } from 'node:fs/promises';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { LaunchpadAgentKit } from './kit.js';
import { requireThat } from './errors.js';
import { TOOL_NAMES, TOOL_SCHEMAS, TOOL_DESCRIPTIONS, invokeTool } from './tools.js';

async function main(): Promise<void> {
  const gatewayUrl = process.env.KINDLE_GATEWAY_URL;
  const dataUrl = process.env.KINDLE_DATA_URL;
  const keyFile = process.env.KINDLE_AGENT_KEY_FILE;
  requireThat(gatewayUrl && dataUrl && keyFile, 'missing_configuration');
  const info = await stat(keyFile);
  requireThat(
    info.isFile() && info.size <= 4096 && (info.mode & 0o077) === 0,
    'invalid_key_file_permissions',
  );
  const key: unknown = JSON.parse(await readFile(keyFile, 'utf8'));
  requireThat(key !== null && typeof key === 'object' && !Array.isArray(key), 'invalid_key_file');
  const config = key as Record<string, unknown>;
  requireThat(
    Object.keys(config).every((name) => ['seedHex', 'label'].includes(name)) &&
      typeof config.seedHex === 'string' &&
      /^[0-9a-fA-F]{64}$/.test(config.seedHex),
    'invalid_key_file',
  );
  requireThat(config.label === undefined || typeof config.label === 'string', 'invalid_key_file');
  const kit = new LaunchpadAgentKit({
    gatewayUrl,
    dataUrl,
    privateKey: Buffer.from(config.seedHex, 'hex'),
    ...(typeof config.label === 'string' ? { label: config.label } : {}),
  });
  const server = new McpServer({ name: 'kindle-agent-kit', version: '0.1.0' });
  for (const name of TOOL_NAMES) {
    server.registerTool(
      name,
      {
        description: TOOL_DESCRIPTIONS[name],
        inputSchema: TOOL_SCHEMAS[name],
        annotations: {
          readOnlyHint: !['buy', 'sell', 'launch', 'provision_agent'].includes(name),
          destructiveHint: ['buy', 'sell', 'launch'].includes(name),
          idempotentHint: !['buy', 'sell', 'launch', 'provision_agent'].includes(name),
          openWorldHint: true,
        },
      },
      async (args: unknown) => {
        const result = await invokeTool(kit, name, args);
        return {
          content: [{ type: 'text' as const, text: JSON.stringify(result) }],
          isError: !result.ok,
        };
      },
    );
  }
  await server.connect(new StdioServerTransport());
}

main().catch(() => {
  process.stderr.write(
    'Unable to start the launchpad agent kit. Check the environment and the private key file.\n',
  );
  process.exitCode = 1;
});
