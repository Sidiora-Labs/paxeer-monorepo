import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { TOOL_NAMES } from '../dist/index.js';
import { seed, wallet } from './support.mjs';

const entry = fileURLToPath(new URL('../dist/mcp.js', import.meta.url));
const expectedTools = [
  'list_coins',
  'search_coins',
  'coin',
  'trades',
  'candles',
  'holders',
  'balances',
  'positions',
  'quote',
  'buy',
  'sell',
  'launch',
  'action',
  'preset',
  'agent_identity',
  'provision_agent',
  'claim_agent',
].sort();

async function mcpChildListsEveryToolAndCallsPreset() {
  const directory = await mkdtemp(join(tmpdir(), 'kindle-agent-kit-'));
  const keyFile = join(directory, 'agent.json');
  await writeFile(keyFile, JSON.stringify({ seedHex: seed.toString('hex') }), { mode: 0o600 });
  const transport = new StdioClientTransport({
    command: process.execPath,
    args: [entry],
    stderr: 'pipe',
    env: {
      PATH: process.env.PATH ?? '',
      KINDLE_GATEWAY_URL: 'https://gateway.example.com',
      KINDLE_DATA_URL: 'https://data.example.com',
      KINDLE_AGENT_KEY_FILE: keyFile,
    },
  });
  const client = new Client({ name: 'kindle-agent-kit-client', version: '0.1.0' });
  let stderr = '';
  try {
    await client.connect(transport);
    transport.stderr?.on('data', (value) => {
      stderr += value;
    });
    const response = await client.listTools();
    assert.deepEqual(response.tools.map((tool) => tool.name).sort(), expectedTools);
    assert.deepEqual([...TOOL_NAMES].sort(), expectedTools);
    for (const tool of response.tools) {
      assert.equal(tool.inputSchema.type, 'object');
      assert.equal(tool.inputSchema.additionalProperties, false);
      assert.ok(tool.description.length >= 50, tool.name);
      assert.equal(
        tool.annotations.readOnlyHint,
        !['buy', 'sell', 'launch', 'provision_agent'].includes(tool.name),
      );
    }
    for (const side of ['buy', 'sell']) {
      const tool = response.tools.find((tool) => tool.name === side);
      for (const field of ['pool', 'quoteId', 'amountInRaw', 'maxSlippageBps', 'idempotencyKey'])
        assert.ok(tool.inputSchema.required.includes(field));
      assert.equal(tool.inputSchema.properties.maxSlippageBps.maximum, 5000);
    }
    const preset = await client.callTool({
      name: 'preset',
      arguments: {
        ownerWallet: wallet,
        maxTxValueWei: '100',
        maxDailyValueWei: '1000',
        maxApproveWei: '1000000',
        budgetCapWei: '1000000',
        expiresInSeconds: 3600,
      },
    });
    assert.equal(preset.isError, false);
    assert.equal(JSON.parse(preset.content[0].text).data.policy.mode, 'trade_only');
    assert.equal(
      JSON.stringify({ response, preset, stderr }).includes(seed.toString('hex')),
      false,
    );
  } finally {
    await client.close();
    await transport.close();
    await rm(directory, { recursive: true, force: true });
  }
}

async function mcpStartupErrorsNeverPrintPrivateFileContents() {
  const directory = await mkdtemp(join(tmpdir(), 'kindle-agent-kit-'));
  try {
    for (const [value, mode] of [
      [seed.toString('hex'), 0o600],
      [JSON.stringify({ seedHex: seed.toString('hex') }), 0o644],
    ]) {
      const keyFile = join(directory, `agent-${mode}.json`);
      await writeFile(keyFile, value, { mode });
      const child = spawn(process.execPath, [entry], {
        env: {
          PATH: process.env.PATH ?? '',
          KINDLE_GATEWAY_URL: 'https://gateway.example.com',
          KINDLE_DATA_URL: 'https://data.example.com',
          KINDLE_AGENT_KEY_FILE: keyFile,
        },
        stdio: ['pipe', 'pipe', 'pipe'],
      });
      let output = '';
      child.stdout.on('data', (value) => {
        output += value;
      });
      child.stderr.on('data', (value) => {
        output += value;
      });
      const [code] = await once(child, 'close');
      assert.equal(code, 1);
      assert.match(output, /Unable to start the launchpad agent kit/);
      assert.equal(output.includes(seed.toString('hex')), false);
      child.stdin.end();
    }
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

test(
  mcpChildListsEveryToolAndCallsPreset.name,
  { timeout: 30000 },
  mcpChildListsEveryToolAndCallsPreset,
);
test(
  mcpStartupErrorsNeverPrintPrivateFileContents.name,
  { timeout: 30000 },
  mcpStartupErrorsNeverPrintPrivateFileContents,
);
