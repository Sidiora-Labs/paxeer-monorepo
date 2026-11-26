import { randomBytes } from 'node:crypto';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { createInterface } from 'node:readline';
import { afterEach, expect, test } from 'vitest';
import { agentInterfaceDescription, TOOL_NAMES } from '@kindle/agent-kit';
import { launchpadAgentInterfaceDocument, agentConnectionConfiguration } from '@/core/agent-interface';
import { backendOrigins, PAXPORT_WALLET_API, walletGatewayApiBase } from '@/core/backend-origins';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { AGENT_INTERFACE_PATH } from '@/widgets/agents/agent-connection';
import { GET } from './interface.json/route';

type InterfaceDocument = ReturnType<typeof launchpadAgentInterfaceDocument>;
const savedEnvironment = new Map<string, string | undefined>();
function environment(name: string, value: string) {
  if (!savedEnvironment.has(name)) savedEnvironment.set(name, process.env[name]);
  process.env[name] = value;
}
afterEach(() => {
  for (const [name, value] of savedEnvironment) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  savedEnvironment.clear();
});

async function agentInterfaceDocumentMatchesTheKit() {
  const configuration = process.env.JOINED_CONFIG_FILE;
  if (!configuration) throw new Error('Joined services are required');
  const joined = JSON.parse(await readFile(configuration, 'utf8')) as { browser_origin: string; api_origin: string };
  const response = await fetch(`${joined.browser_origin}${AGENT_INTERFACE_PATH}`);
  expect(response.status).toBe(200);
  expect(response.headers.get('Content-Type')).toContain('application/json');
  expect(response.headers.get('Cache-Control')).toBe('public, max-age=300');
  expect(response.headers.get('Set-Cookie')).toBeNull();
  expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
  const document: InterfaceDocument = await response.json();
  const description = agentInterfaceDescription();
  expect(document.package).toEqual(description.package);
  expect(document.entry).toEqual(description.entry);
  expect(document.environment).toEqual({ gatewayUrl: 'KINDLE_GATEWAY_URL', dataUrl: 'KINDLE_DATA_URL', keyFile: 'KINDLE_AGENT_KEY_FILE' });
  expect(document.tools).toEqual(description.tools);
  expect(document.tools.map(tool => tool.name)).toEqual(TOOL_NAMES);
  expect(JSON.stringify(description)).not.toMatch(/https?:\/\/|0x[0-9a-fA-F]{40}/);
  expect(document.endpoints).toEqual({ walletGateway: walletGatewayApiBase(), data: joined.api_origin });
  expect(document.chain).toEqual({ id: CORE_DEPLOYMENT.chainId, name: 'Paxeer', nativeCurrency: { symbol: CORE_DEPLOYMENT.nativeSymbol, decimals: CORE_DEPLOYMENT.nativeDecimals } });
  expect(document.contracts).toEqual({ router: CORE_DEPLOYMENT.router, quoter: CORE_DEPLOYMENT.quoter, feesRouter: CORE_DEPLOYMENT.feesRouter, opticalRegistry: CORE_DEPLOYMENT.opticalRegistry, usdl: CORE_DEPLOYMENT.usdl, protocolConfig: CORE_DEPLOYMENT.protocolConfig, launchpad: CORE_DEPLOYMENT.launchpadPrecompile });

  const repository = resolve(process.cwd(), '../../..');
  const manifest = JSON.parse(await readFile(join(repository, 'sdk/launchpad/packages/agent-kit/package.json'), 'utf8'));
  expect(document.package).toEqual({ name: manifest.name, version: manifest.version });
  expect(document.entry.args).toEqual([`sdk/launchpad/packages/agent-kit/${manifest.bin['kindle-agent-kit'].replace(/^\.\//, '')}`]);

  const directory = await mkdtemp(join(tmpdir(), 'agent-interface-'));
  const keyFile = join(directory, 'agent.json');
  const seed = randomBytes(32).toString('hex');
  await writeFile(keyFile, JSON.stringify({ seedHex: seed }), { mode: 0o600 });
  const child = spawn(document.entry.command, document.entry.args, { cwd: repository, env: {
    ...process.env,
    [document.environment.gatewayUrl]: document.endpoints.walletGateway,
    [document.environment.dataUrl]: new URL(document.endpoints.data).protocol === 'https:' ? document.endpoints.data : 'https://launchpad-data.invalid',
    [document.environment.keyFile]: keyFile,
  }, stdio: ['pipe', 'pipe', 'pipe'] });
  const lines = createInterface({ input: child.stdout });
  const pending = new Map<number, { resolve: (value: unknown) => void; reject: (cause: Error) => void }>();
  let id = 0;
  let stderr = '';
  let processError: Error | undefined;
  function rejectPending(cause: Error) {
    processError = cause;
    for (const request of pending.values()) request.reject(cause);
    pending.clear();
  }
  child.stderr.on('data', value => { stderr += value; });
  lines.on('line', line => {
    const message = JSON.parse(line) as { id?: number; result?: unknown; error?: { message: string } };
    if (message.id === undefined) return;
    const request = pending.get(message.id);
    if (!request) return;
    pending.delete(message.id);
    if (message.error) request.reject(new Error(message.error.message));
    else request.resolve(message.result);
  });
  child.on('error', rejectPending);
  child.on('close', (code, signal) => {
    rejectPending(new Error(`Agent kit exited with code ${code}, signal ${signal}: ${stderr}`));
  });
  function request(method: string, params: object = {}): Promise<unknown> {
    if (processError) return Promise.reject(processError);
    const requestId = ++id;
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => { pending.delete(requestId); reject(new Error('MCP response required')); }, 10000);
      pending.set(requestId, {
        resolve(value) { clearTimeout(timeout); resolve(value); },
        reject(cause) { clearTimeout(timeout); reject(cause); },
      });
      child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', id: requestId, method, params })}\n`);
    });
  }
  try {
    const initialized = await request('initialize', { protocolVersion: '2025-03-26', capabilities: {}, clientInfo: { name: 'launchpad-interface', version: '1.0.0' } });
    expect(initialized).toMatchObject({ serverInfo: document.package });
    child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' })}\n`);
    const listed = await request('tools/list') as { tools: Array<{ name: string; description: string; inputSchema: unknown }> };
    expect(listed.tools.map(({ name, description, inputSchema }) => ({ name, description, inputSchema }))).toEqual(document.tools);
    const called = await request('tools/call', { name: 'preset', arguments: {
      ownerWallet: CORE_DEPLOYMENT.router, maxTxValueWei: '1', maxDailyValueWei: '2', maxApproveWei: '1', budgetCapWei: '1', expiresInSeconds: 60,
    } }) as { isError: boolean; content: Array<{ type: string; text: string }> };
    expect(called.isError).toBe(false);
    expect(JSON.parse(called.content[0].text)).toMatchObject({ ok: true, data: { policy: { mode: 'trade_only' } } });
    expect(JSON.stringify({ document, listed, called, stderr })).not.toContain(seed);
  } finally {
    lines.close();
    if (child.exitCode === null) {
      const closed = once(child, 'close');
      child.kill('SIGTERM');
      await closed;
    }
    await rm(directory, { recursive: true, force: true });
  }
}

async function agentInterfaceContainsOnlyPublicConfiguration() {
  const sensitive = randomBytes(32).toString('hex');
  environment('NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY', sensitive);
  environment('KINDLE_AGENT_KEY_FILE', sensitive);
  environment('NEXT_PUBLIC_PAXEER_WALLET_API', 'https://wallet.example.com');
  const response = GET();
  const document = await response.json();
  expect(document.endpoints).toEqual({ walletGateway: 'https://wallet.example.com', data: backendOrigins().data });
  expect(Object.keys(document).sort()).toEqual(['chain', 'contracts', 'endpoints', 'entry', 'environment', 'package', 'tools']);
  expect(JSON.stringify(document)).not.toContain(sensitive);
  expect(response.headers.get('Set-Cookie')).toBeNull();
  expect(response.headers.get('Vary')).toBeNull();
  environment('NEXT_PUBLIC_PAXEER_WALLET_API', '');
  expect((await GET().json()).endpoints.walletGateway).toBe(PAXPORT_WALLET_API);
}

async function agentInterfaceRejectsInvalidConfigurationWithoutDisclosingIt() {
  for (const value of ['https://user:password@wallet.example.com', 'https://wallet.example.com?key=value']) {
    environment('NEXT_PUBLIC_PAXEER_WALLET_API', value);
    const response = GET();
    expect(response.status).toBe(503);
    expect(response.headers.get('Cache-Control')).toBe('no-store');
    expect(await response.json()).toEqual({ error: 'Agent interface unavailable' });
    expect(agentConnectionConfiguration()).toEqual({ status: 'unavailable' });
  }
  environment('NEXT_PUBLIC_PAXEER_WALLET_API', PAXPORT_WALLET_API);
  environment('NEXT_PUBLIC_DATA_API_URL', '');
  expect(GET().status).toBe(503);
}

async function agentInterfaceHttpMethodsAndCookiesStayPublic() {
  const configuration = process.env.JOINED_CONFIG_FILE;
  if (!configuration) throw new Error('Joined services are required');
  const joined = JSON.parse(await readFile(configuration, 'utf8')) as { browser_origin: string };
  const url = `${joined.browser_origin}${AGENT_INTERFACE_PATH}`;
  const anonymous = await fetch(url);
  const withCookie = await fetch(url, { headers: { Cookie: 'preference=light' } });
  expect(withCookie.status).toBe(200);
  expect(await withCookie.json()).toEqual(await anonymous.json());
  expect(withCookie.headers.get('Set-Cookie')).toBeNull();
  expect(withCookie.headers.get('Vary') ?? '').not.toMatch(/cookie/i);
  const head = await fetch(url, { method: 'HEAD' });
  expect(head.status).toBe(200);
  expect(head.headers.get('Cache-Control')).toBe('public, max-age=300');
  expect(await head.text()).toBe('');
  const post = await fetch(url, { method: 'POST', body: '{}' });
  expect(post.status).toBe(405);
}

test(agentInterfaceDocumentMatchesTheKit.name, agentInterfaceDocumentMatchesTheKit, 60000);
test(agentInterfaceContainsOnlyPublicConfiguration.name, agentInterfaceContainsOnlyPublicConfiguration);
test(agentInterfaceRejectsInvalidConfigurationWithoutDisclosingIt.name, agentInterfaceRejectsInvalidConfigurationWithoutDisclosingIt);
test(agentInterfaceHttpMethodsAndCookiesStayPublic.name, agentInterfaceHttpMethodsAndCookiesStayPublic, 60000);
