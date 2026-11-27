import assert from 'node:assert/strict';
import { spawn, type ChildProcess } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
  startIdentity,
  startAttestorNetwork,
  type Identity,
  type AttestorNetwork,
} from '../e2e/attestors.js';
import { startPostgresEngine, type RestartablePostgres } from './postgres.js';
import { reservePort, stopPortProcess, assertProcessRunning } from './ports.js';
import type { WalletInventoryKey } from '../../src/agent/authority.js';

export async function startJourneyGateway(rpcUrl: string, quoteToken: string) {
  let identity: Identity | undefined;
  let network: AttestorNetwork | undefined;
  let database: RestartablePostgres | undefined;
  let gateway: ChildProcess | undefined;
  const logs: string[] = [];
  const inventory: WalletInventoryKey[] = [];
  let url: string;
  let gatewayEnv: NodeJS.ProcessEnv;
  const adminToken = randomBytes(32).toString('hex');
  const ownerId = randomUUID();
  const request = async (method: string, path: string, body?: unknown, token?: string) => {
    const response = await fetch(url + path, {
      method,
      headers: {
        ...(body === undefined ? {} : { 'content-type': 'application/json' }),
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      signal: AbortSignal.timeout(120_000),
    });
    return { status: response.status, body: (await response.json()) as Record<string, any> };
  };
  const stop = async () => {
    if (gateway) await stopPortProcess(gateway);
    await database?.stop();
    await network?.stop();
    await identity?.close();
  };
  let port: number | undefined;
  const start = async () => {
    const reservation = port === undefined ? await reservePort('::') : undefined;
    try {
      port ??= reservation!.port;
      gatewayEnv.PORT = String(port);
      await reservation?.release();
      gateway = spawn(process.execPath, ['--import', 'tsx', 'src/index.ts'], {
        cwd: fileURLToPath(new URL('../../', import.meta.url)),
        env: gatewayEnv,
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      gateway.stdout!.on('data', (chunk) => logs.push(chunk.toString()));
      gateway.stderr!.on('data', (chunk) => logs.push(chunk.toString()));
      gateway.on('error', (error) => logs.push(error.message));
      url = `http://127.0.0.1:${port}`;
      const until = Date.now() + 60_000;
      for (;;) {
        assertProcessRunning(gateway, logs);
        const response = await fetch(url + '/readyz', { signal: AbortSignal.timeout(5000) }).catch(
          () => null
        );
        if (response?.status === 200) {
          assert.equal((await response.json()).ready, true);
          assert.equal(response.headers.get('x-served-by'), 'paxeer-wallet-gateway');
          break;
        }
        if (Date.now() >= until) throw new Error(`Gateway readiness timed out.\n${logs.join('')}`);
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    } finally {
      await reservation?.release();
    }
  };
  try {
    identity = await startIdentity();
    network = await startAttestorNetwork({
      identity,
      rpcUrl,
      assetCaps: {
        [quoteToken.toLowerCase()]: { per_transaction: '50000000', daily: '100000000' },
      },
    });
    database = await startPostgresEngine();
    gatewayEnv = {
      ...process.env,
      NODE_ENV: 'test',
      API_WORKERS: '1',
      LOG_LEVEL: 'info',
      DATABASE_URL: database.url,
      SUPABASE_URL: identity.url,
      WALLET_MASTER_KEY: randomBytes(32).toString('base64'),
      AGENT_JWT_SECRET: randomBytes(32).toString('hex'),
      GATEWAY_ADMIN_TOKEN: adminToken,
      CORS_ORIGINS: 'http://localhost:3000',
      HYPERPAXEER_CHAIN_ID: '125',
      HYPERPAXEER_RPC_URL: rpcUrl,
      RPC_URLS: rpcUrl,
      SEQUENCER_RPC_URL: rpcUrl,
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
      CHAIN_RECEIPT_SOURCE: 'evm',
      RATE_LIMIT_CLIENT_PER_MINUTE: '10000',
      RATE_LIMIT_ACCOUNT_PER_MINUTE: '10000',
    };
    delete gatewayEnv.WALLET_GAS_STATION_URL;
    delete gatewayEnv.LAYER_X_DB_URI;
    await start();
    const ownerToken = await identity.mint(ownerId);
    const publish = async () => {
      network!.authority.approveInventory(inventory);
      const response = await request(
        'POST',
        '/v1/admin/custody-inventory',
        {
          token: readFileSync(network!.authority.inventoryFile, 'utf8'),
        },
        adminToken
      );
      assert.equal(response.status, 200, JSON.stringify(response.body));
    };
    return {
      get url() {
        return url;
      },
      ownerId,
      ownerToken,
      request,
      logs,
      async admitKey(keyId: string, owner: string) {
        inventory.push({
          key_id: keyId,
          owner,
          curve: 'secp256k1',
          epoch: 0,
          public_key: '',
          operations: ['generate'],
        });
        await publish();
      },
      async admitSigning(keyId: string) {
        const entry = inventory.find((key) => key.key_id === keyId);
        assert.ok(entry);
        entry.public_key = await network!.publicKey(keyId);
        entry.operations = ['generate', 'sign'];
        await publish();
      },
      async admitAsset(asset: string, perTransaction: string, daily: string) {
        await network!.admitAsset(asset, { per_transaction: perTransaction, daily });
        await publish();
      },
      async restart() {
        await stopPortProcess(gateway!);
        await start();
      },
      stop,
    };
  } catch (error) {
    await stop();
    throw error;
  }
}
