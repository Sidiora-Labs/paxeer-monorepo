import assert from 'node:assert/strict';
import { spawn, type ChildProcess } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { fileURLToPath } from 'node:url';
import { startAttestorNetwork, startIdentity } from '../e2e/attestors.js';
import { assertProcessRunning, reservePort, stopPortProcess } from './ports.js';
import { startPostgresEngine } from './postgres.js';

/** Forwards the node's responses verbatim and records submitted RPC methods. */
export async function startRpcForwarder(upstream: string) {
  const methods: string[] = [];
  const broadcasts: Array<{ method: string; raw: string }> = [];
  const receiptRungs: Array<{ hash: string; rung: string }> = [];
  let activeReads = 0;
  let maxConcurrentReads = 0;
  const server = createServer(async (req, res) => {
    let reads = 0;
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks);
      const payload: unknown = JSON.parse(body.toString('utf8'));
      for (const call of Array.isArray(payload) ? payload : [payload]) {
        assert.equal(typeof call.method, 'string');
        methods.push(call.method);
        if (call.method === 'eth_call') reads++;
        if (call.method === 'eth_sendRawTransaction' || call.method === 'px_sendRawTransaction') {
          broadcasts.push({ method: call.method, raw: call.params[0] });
        }
      }
      activeReads += reads;
      maxConcurrentReads = Math.max(maxConcurrentReads, activeReads);
      const response = await fetch(upstream, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
        signal: AbortSignal.timeout(120_000),
      });
      res.writeHead(response.status, {
        'content-type': response.headers.get('content-type') ?? 'application/json',
      });
      const responseBody = Buffer.from(await response.arrayBuffer());
      const answers: unknown = JSON.parse(responseBody.toString('utf8'));
      const requests = Array.isArray(payload) ? payload : [payload];
      for (const answer of Array.isArray(answers) ? answers : [answers]) {
        const request = requests.find((call) => call.id === answer.id);
        if (
          request?.method === 'pax_sequencerReceipt' &&
          typeof answer.result?.txHash === 'string' &&
          typeof answer.result?.rung === 'string'
        )
          receiptRungs.push({ hash: answer.result.txHash, rung: answer.result.rung });
      }
      res.end(responseBody);
    } catch (error) {
      res.writeHead(502, { 'content-type': 'text/plain' });
      res.end(String(error));
    } finally {
      activeReads -= reads;
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  return {
    url: `http://127.0.0.1:${(server.address() as AddressInfo).port}`,
    methods,
    broadcasts,
    receiptRungs,
    get maxConcurrentReads() {
      return maxConcurrentReads;
    },
    beginReadObservation() {
      assert.equal(activeReads, 0);
      maxConcurrentReads = 0;
    },
    async stop() {
      server.closeIdleConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
  };
}

/** Runs the HTTP gateway with persistent database and threshold wallet custody. */
export async function startLocalWalletGateway(
  rpcUrl: string,
  custodyPolicy: Required<
    Pick<Parameters<typeof startAttestorNetwork>[0], 'tokenCaps' | 'selectors' | 'trustedSpenders'>
  >,
) {
  const rpc = await startRpcForwarder(rpcUrl);
  let identityHandle: Awaited<ReturnType<typeof startIdentity>> | undefined;
  let databaseHandle: Awaited<ReturnType<typeof startPostgresEngine>> | undefined;
  let network: Awaited<ReturnType<typeof startAttestorNetwork>> | undefined;
  let child: ChildProcess | undefined;
  const logs: string[] = [];
  const stop = async () => {
    await stopPortProcess(child);
    await network?.stop();
    await databaseHandle?.stop();
    await identityHandle?.close();
    await rpc.stop();
  };
  try {
    const identity = (identityHandle = await startIdentity());
    const database = (databaseHandle = await startPostgresEngine());
    network = await startAttestorNetwork({
      identity,
      rpcUrl: rpc.url,
      readyTimeoutMs: 120_000,
      ...custodyPolicy,
    });
    const reservation = await reservePort('::');
    const url = `http://127.0.0.1:${reservation.port}`;
    const adminToken = randomBytes(32).toString('hex');
    const env: NodeJS.ProcessEnv = {
      ...process.env,
      NODE_ENV: 'test',
      API_WORKERS: '1',
      LOG_LEVEL: 'info',
      PORT: String(reservation.port),
      DATABASE_URL: database.url,
      SUPABASE_URL: identity.url,
      WALLET_MASTER_KEY: randomBytes(32).toString('base64'),
      AGENT_JWT_SECRET: randomBytes(32).toString('hex'),
      GATEWAY_ADMIN_TOKEN: adminToken,
      CORS_ORIGINS: 'http://localhost:3000',
      HYPERPAXEER_CHAIN_ID: '125',
      HYPERPAXEER_RPC_URL: rpc.url,
      RPC_URLS: rpc.url,
      SEQUENCER_RPC_URL: rpc.url,
      WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE: network.authority.producerKeyFile,
      WALLET_IDENTITY_BINDING_TENANT: network.authority.tenant,
      WALLET_CUSTODY_INVENTORY_FILE: network.authority.inventoryFile,
      WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE: network.authority.inventoryPublicKeyFile,
      ATTESTOR_ENDPOINTS: network.nodes.map((node) => node.apiUrl).join(','),
      ATTESTOR_CLIENT_CERT_FILE: network.pki.clientCert,
      ATTESTOR_CLIENT_KEY_FILE: network.pki.clientKey,
      ATTESTOR_CA_FILE: network.pki.caFile,
      ATTESTOR_QUORUM: '3',
      ATTESTOR_TIMEOUT_MS: '120000',
      ACTION_WORKER_INTERVAL_MS: '100',
      ACTION_CONFIRMATIONS: '1',
    };
    delete env.WALLET_GAS_STATION_URL;
    delete env.LAYER_X_DB_URI;
    delete env.LAYERX_SYNC_INTERVAL_MS;
    const launch = async () => {
      child = spawn(process.execPath, ['--import', 'tsx', 'src/index.ts'], {
        cwd: fileURLToPath(new URL('../..', import.meta.url)),
        env,
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      child.stdout!.on('data', (chunk: Buffer) => logs.push(chunk.toString('utf8')));
      child.stderr!.on('data', (chunk: Buffer) => logs.push(chunk.toString('utf8')));
      child.on('error', (error) => logs.push(error.message));
      const deadline = Date.now() + 60_000;
      for (;;) {
        assertProcessRunning(child, logs);
        const ready = await fetch(`${url}/readyz`, { signal: AbortSignal.timeout(2_000) }).catch(
          () => null,
        );
        if (ready?.status === 200) {
          assert.equal(ready.headers.get('x-served-by'), 'paxeer-wallet-gateway');
          assert.equal((await ready.json()).ready, true);
          return;
        }
        if (Date.now() >= deadline) throw new Error(`gateway readiness: ${logs.join('')}`);
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    };
    await reservation.release();
    await launch();
    return {
      url,
      adminToken,
      network,
      identity,
      rpc,
      logs,
      stop,
      async restart() {
        await stopPortProcess(child);
        await launch();
      },
    };
  } catch (error) {
    await stop();
    throw error;
  }
}
