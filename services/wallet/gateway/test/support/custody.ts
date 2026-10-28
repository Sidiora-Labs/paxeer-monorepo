import { execFileSync, spawn, type ChildProcess } from 'node:child_process';
import { createHash, generateKeyPairSync, randomBytes, sign, X509Certificate } from 'node:crypto';
import { closeSync, mkdtempSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { request as httpsRequest } from 'node:https';
import { createServer, type Server } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { buildAttestor, CHAIN_ID, KERNEL_POLICY } from '../e2e/attestors.js';
import { ADDR, BIND_SELECTOR } from './chain.js';
import type { IdentityProvider } from './identity.js';

async function reservePorts(count: number): Promise<number[]> {
  const servers: Server[] = [];
  try {
    return await Promise.all(Array.from({ length: count }, () => new Promise<number>((resolve, reject) => {
      const server = createServer();
      servers.push(server);
      server.once('error', reject);
      server.listen(0, '127.0.0.1', () => {
        const address = server.address();
        if (!address || typeof address === 'string') return reject(new Error('no port'));
        resolve(address.port);
      });
    })));
  } finally {
    await Promise.all(servers.map((server) => new Promise<void>((resolve) => server.close(() => resolve()))));
  }
}

function ossl(args: string[]): void {
  execFileSync('openssl', args, { stdio: 'pipe' });
}

function issueCertificates(dir: string, names: string[]): void {
  const p = (n: string): string => join(dir, n);
  const ec = ['-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:prime256v1', '-nodes'];
  ossl(['req', '-x509', ...ec, '-keyout', p('ca.key'), '-out', p('ca.crt'), '-days', '1', '-subj', '/CN=provisioning-test-ca']);
  writeFileSync(p('leaf.ext'), 'subjectAltName=IP:127.0.0.1\nextendedKeyUsage=serverAuth,clientAuth\n');
  for (const name of names) {
    ossl(['req', ...ec, '-keyout', p(`${name}.key`), '-out', p(`${name}.csr`), '-subj', `/CN=${name}`]);
    ossl(['x509', '-req', '-in', p(`${name}.csr`), '-CA', p('ca.crt'), '-CAkey', p('ca.key'), '-CAcreateserial', '-out', p(`${name}.crt`), '-days', '1', '-extfile', p('leaf.ext')]);
  }
}

function spkiPin(certPath: string): string {
  const cert = new X509Certificate(readFileSync(certPath));
  return createHash('sha256').update(cert.publicKey.export({ type: 'spki', format: 'der' })).digest('hex');
}

export async function startCustodyAuthority(idp: IdentityProvider, rpcUrl: string) {
  const workDir = mkdtempSync(join(tmpdir(), 'gateway-custody-'));
  const daemons: ChildProcess[] = [];
  const endpoints: string[] = [];
  let startupError: Error | undefined;

  async function stop(): Promise<void> {
    await Promise.all(daemons.map((child) => new Promise<void>((resolve) => {
      if (!child.pid || child.exitCode !== null || child.signalCode !== null) return resolve();
      child.once('exit', () => resolve());
      child.kill('SIGTERM');
    })));
    rmSync(workDir, { recursive: true, force: true });
  }

  try {
    const bin = buildAttestor(workDir);
    const authority = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
    const authorityPrivateKeyFile = join(workDir, 'authority-private.pem');
    const authorityPublicKeyFile = join(workDir, 'authority-public.pem');
    writeFileSync(authorityPrivateKeyFile, authority.privateKey.export({ type: 'pkcs8', format: 'pem' }), { mode: 0o600 });
    writeFileSync(authorityPublicKeyFile, authority.publicKey.export({ type: 'spki', format: 'pem' }));

    const nodes = ['node-1', 'node-2', 'node-3', 'node-4', 'node-5'];
    issueCertificates(workDir, [...nodes, 'gateway']);
    const clientTls = {
      cert: readFileSync(join(workDir, 'gateway.crt')),
      key: readFileSync(join(workDir, 'gateway.key')),
      ca: readFileSync(join(workDir, 'ca.crt')),
    };
    const policyFile = join(workDir, 'policy.json');
    writeFileSync(
      policyFile,
      JSON.stringify({
        version: 1,
        defaults: {
          chain_id: CHAIN_ID,
          kinds: ['evm_tx', 'lx_bind'],
          rate_per_minute: 1000,
          caps: { native: { per_transaction: '1000000000000000000', daily: '10000000000000000000' } },
          selectors: { [ADDR]: [BIND_SELECTOR] },
        },
      }),
    );
    const kernelPolicyFile = join(workDir, 'kernel-policy.json');
    writeFileSync(kernelPolicyFile, JSON.stringify(KERNEL_POLICY));
    const ports = await reservePorts(nodes.length * 2);
    const apiPorts = ports.slice(0, nodes.length);
    const peerPorts = ports.slice(nodes.length);
    const pins = nodes.map((n) => spkiPin(join(workDir, `${n}.crt`)));
    const inventoryFile = join(workDir, 'inventory.jwt');
    const issuedAt = Math.floor(Date.now() / 1000);
    const inventoryHeader = Buffer.from(JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })).toString('base64url');
    const inventoryPayload = Buffer.from(JSON.stringify({
      version: 1,
      iss: `${idp.url}/auth/v1`,
      aud: 'wallet-custody-inventory',
      tenant: 'provisioning-test',
      sequence: '1',
      iat: issuedAt,
      exp: issuedAt + 3600,
      protocol: 'wallet',
      threshold: 3,
      members: nodes.map((id, i) => ({ id, spki_sha256: pins[i]! })),
      keys: [],
    })).toString('base64url');
    const inventoryInput = `${inventoryHeader}.${inventoryPayload}`;
    const inventorySignature = sign('sha256', Buffer.from(inventoryInput), { key: authority.privateKey, dsaEncoding: 'ieee-p1363' });
    writeFileSync(inventoryFile, `${inventoryInput}.${inventorySignature.toString('base64url')}`);
    nodes.forEach((node, i) => {
      const others = nodes.map((n, j) => ({ n, j })).filter((x) => x.j !== i);
      const keyFile = join(workDir, `${node}.store`);
      writeFileSync(keyFile, randomBytes(32).toString('hex'));
      const log = openSync(join(workDir, `${node}.log`), 'w');
      const child = spawn(bin, [], {
        env: {
          PATH: process.env.PATH ?? '',
          ATTESTOR_NODE_ID: node,
          ATTESTOR_REGION: `region-${i + 1}`,
          ATTESTOR_LISTEN_ADDR: `127.0.0.1:${apiPorts[i]}`,
          ATTESTOR_PEER_LISTEN_ADDR: `127.0.0.1:${peerPorts[i]}`,
          ATTESTOR_PEERS: others.map((x) => `${x.n}=127.0.0.1:${peerPorts[x.j]}`).join(','),
          ATTESTOR_PEER_PINS: others.map((x) => `${x.n}=${pins[x.j]}`).join(','),
          ATTESTOR_NODE_KEY_FILE: keyFile,
          ATTESTOR_DATA_DIR: join(workDir, `${node}-data`),
          ATTESTOR_CHAIN_ID: String(CHAIN_ID),
          ATTESTOR_JWKS_URL: `${idp.url}/auth/v1/.well-known/jwks.json`,
          ATTESTOR_JWT_ISSUER: `${idp.url}/auth/v1`,
          ATTESTOR_JWT_AUDIENCE: 'authenticated',
          ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE: authorityPublicKeyFile,
          ATTESTOR_AUTHORITY_ISSUER: `${idp.url}/auth/v1`,
          ATTESTOR_AUTHORITY_TENANT: 'provisioning-test',
          ATTESTOR_INVENTORY_PUBLIC_KEY_FILE: authorityPublicKeyFile,
          ATTESTOR_INVENTORY_FILE: inventoryFile,
          ATTESTOR_POLICY_FILE: policyFile,
          ATTESTOR_TLS_CERT_FILE: join(workDir, `${node}.crt`),
          ATTESTOR_TLS_KEY_FILE: join(workDir, `${node}.key`),
          ATTESTOR_TLS_CA_FILE: join(workDir, 'ca.crt'),
          ATTESTOR_OPERATOR_CA_FILE: join(workDir, 'ca.crt'),
          ATTESTOR_KERNEL_POLICY_FILE: kernelPolicyFile,
          ATTESTOR_RPC_URL: rpcUrl,
          ATTESTOR_ACTIVITY_TYPES: '0x10005',
        },
        stdio: ['ignore', log, log],
      });
      child.once('error', (error) => { startupError = error; });
      daemons.push(child);
      closeSync(log);
      endpoints.push(`https://127.0.0.1:${apiPorts[i]}`);
    });
    function health(url: string): Promise<{ ready: boolean }> {
      return new Promise((resolveHealth) => {
        const req = httpsRequest(new URL('/health', url), { method: 'GET', ...clientTls, timeout: 5_000 }, (res) => {
          const chunks: Buffer[] = [];
          res.on('data', (c: Buffer) => chunks.push(c));
          res.on('end', () => {
            try {
              resolveHealth(JSON.parse(Buffer.concat(chunks).toString('utf8')) as { ready: boolean });
            } catch {
              resolveHealth({ ready: false });
            }
          });
        });
        req.on('error', () => resolveHealth({ ready: false }));
        req.on('timeout', () => req.destroy());
        req.end();
      });
    }

    const deadline = Date.now() + 120_000;
    for (;;) {
      if (startupError || daemons.some((child) => child.exitCode !== null || child.signalCode !== null)) {
        const logs = nodes.map((n) => readFileSync(join(workDir, `${n}.log`), 'utf8')).join('\n');
        throw new Error(`attestor exited before readiness: ${startupError ?? logs}`);
      }
      const states = await Promise.all(endpoints.map((u) => health(u)));
      if (states.every((s) => s.ready)) break;
      if (Date.now() > deadline) {
        const logs = nodes.map((n) => readFileSync(join(workDir, `${n}.log`), 'utf8')).join('\n');
        throw new Error(`attestors did not become ready: ${logs}`);
      }
      await new Promise((r) => setTimeout(r, 300));
    }

    return {
      env: {
        ATTESTOR_ENDPOINTS: endpoints.join(','),
        ATTESTOR_CLIENT_CERT_FILE: join(workDir, 'gateway.crt'),
        ATTESTOR_CLIENT_KEY_FILE: join(workDir, 'gateway.key'),
        ATTESTOR_CA_FILE: join(workDir, 'ca.crt'),
        ATTESTOR_QUORUM: '3',
        WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE: authorityPrivateKeyFile,
        WALLET_IDENTITY_BINDING_TENANT: 'provisioning-test',
        WALLET_CUSTODY_INVENTORY_FILE: inventoryFile,
        WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE: authorityPublicKeyFile,
      },
      stop,
    };
  } catch (error) {
    await stop();
    throw error;
  }
}
