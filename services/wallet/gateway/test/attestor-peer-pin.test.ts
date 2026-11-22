import { afterAll, beforeAll, describe, expect, test } from 'vitest';
import { createHash, generateKeyPairSync, randomBytes, sign, X509Certificate } from 'node:crypto';
import { once } from 'node:events';
import { readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer, type Agent, type Server } from 'node:https';
import type { AddressInfo } from 'node:net';
import { join } from 'node:path';
import type { TLSSocket } from 'node:tls';
import type { AttestorClient } from '../src/attestor/client.js';
import { makePki, type Pki } from './e2e/attestors.js';

const ids = ['node-1', 'node-2', 'node-3', 'node-4', 'node-5'];
let pki: Pki;
let Client: typeof AttestorClient;
const inventorySigner = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
let sequence = 0;
const savedEnv = new Map<string, string | undefined>();

function approveMembers(members = ids): void {
  const header = Buffer.from(
    JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' }),
  ).toString('base64url');
  const now = Math.floor(Date.now() / 1000);
  const payload = Buffer.from(
    JSON.stringify({
      version: 1,
      iss: `${process.env.SUPABASE_URL!.replace(/\/$/, '')}/auth/v1`,
      aud: 'wallet-custody-inventory',
      tenant: 'peer-pin',
      sequence: String(++sequence),
      iat: now - 1,
      exp: now + 3600,
      protocol: 'wallet',
      threshold: 3,
      members: members.map((id) => ({ id, spki_sha256: pki.node(id).pin })),
      keys: [],
    }),
  ).toString('base64url');
  const input = `${header}.${payload}`;
  const signature = sign('sha256', Buffer.from(input), {
    key: inventorySigner.privateKey,
    dsaEncoding: 'ieee-p1363',
  });
  writeFileSync(join(pki.dir, 'inventory.jwt'), `${input}.${signature.toString('base64url')}`);
}

beforeAll(async () => {
  pki = makePki([...ids, 'outside']);
  const publicFile = join(pki.dir, 'inventory.pub');
  const privateFile = join(pki.dir, 'identity.key');
  writeFileSync(publicFile, inventorySigner.publicKey.export({ format: 'pem', type: 'spki' }));
  writeFileSync(privateFile, inventorySigner.privateKey.export({ format: 'pem', type: 'pkcs8' }), {
    mode: 0o600,
  });
  for (const [name, value] of Object.entries({
    NODE_ENV: 'test',
    SUPABASE_URL: 'https://supabase.test.invalid',
    DATABASE_URL: 'postgres://localhost:5432/test',
    WALLET_MASTER_KEY: randomBytes(32).toString('base64'),
    CORS_ORIGINS: 'http://localhost:3000',
    HYPERPAXEER_RPC_URL: 'http://localhost:0/rpc',
    ATTESTOR_ENDPOINTS: '',
    WALLET_CUSTODY_INVENTORY_FILE: join(pki.dir, 'inventory.jwt'),
    WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE: publicFile,
    WALLET_IDENTITY_BINDING_TENANT: 'peer-pin',
    WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE: privateFile,
  })) {
    savedEnv.set(name, process.env[name]);
    process.env[name] = value;
  }
  approveMembers();
  Client = (await import('../src/attestor/client.js')).AttestorClient;
}, 30_000);

afterAll(() => {
  for (const [name, value] of savedEnv) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  if (pki) rmSync(pki.dir, { recursive: true, force: true });
});

interface Connection {
  socket: TLSSocket;
  resumed: boolean;
  certificate: Buffer | undefined;
  session: Promise<void>;
}

async function startPeer(id = 'node-1', closeAfterResponse = false) {
  const leaf = pki.node(id);
  const requests: TLSSocket[] = [];
  const serverConnections: boolean[] = [];
  let heldResponse: (() => void) | undefined;
  let announceRequest: (() => void) | undefined;
  const server: Server = createServer(
    {
      cert: readFileSync(leaf.cert),
      key: readFileSync(leaf.key),
      ca: readFileSync(pki.caFile),
      requestCert: true,
      rejectUnauthorized: true,
      minVersion: 'TLSv1.3',
      maxVersion: 'TLSv1.3',
    },
    (req, res) => {
      requests.push(req.socket as TLSSocket);
      const answer = () => {
        res.writeHead(200, {
          'content-type': 'application/json',
          ...(closeAfterResponse ? { connection: 'close' } : {}),
        });
        res.end(JSON.stringify({ node_id: id, request: requests.length }));
      };
      if (announceRequest) {
        heldResponse = answer;
        const announce = announceRequest;
        announceRequest = undefined;
        announce();
      } else answer();
    },
  );
  server.keepAliveTimeout = 0;
  server.on('secureConnection', (socket) => serverConnections.push(socket.isSessionReused()));
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const origin = `https://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const client = new Client({
    endpoints: Array.from({ length: 5 }, () => origin),
    quorum: 3,
    tls: {
      cert: readFileSync(pki.clientCert),
      key: readFileSync(pki.clientKey),
      ca: readFileSync(pki.caFile),
    },
    healthIntervalMs: 60_000,
    timeoutMs: 10_000,
  });
  const agent: Agent = Reflect.get(client, 'agent');
  const pins: Map<string, string> = Reflect.get(client, 'peerPins');
  const connections: Connection[] = [];
  const observed = new Set<TLSSocket>();
  const sessionCertificates = new Map<string, Buffer>();
  agent.on('keylog', (_line: Buffer, socket: TLSSocket) => {
    if (observed.has(socket)) return;
    observed.add(socket);
    const offeredSession = socket.getSession();
    const resumedCertificate = offeredSession
      ? sessionCertificates.get(createHash('sha256').update(offeredSession).digest('hex'))
      : undefined;
    const session = once(socket, 'session').then(() => undefined);
    void session.catch(() => undefined);
    socket.once('secureConnect', () => {
      const certificate = socket.isSessionReused()
        ? resumedCertificate
        : socket.getPeerCertificate()?.raw;
      connections.push({
        socket,
        resumed: socket.isSessionReused(),
        certificate,
        session,
      });
      if (certificate) {
        const publicKey = new X509Certificate(certificate).publicKey.export({
          type: 'spki',
          format: 'der',
        });
        expect(createHash('sha256').update(publicKey).digest('hex')).toBe(leaf.pin);
        socket.on('session', (ticket: Buffer) => {
          sessionCertificates.set(createHash('sha256').update(ticket).digest('hex'), certificate);
        });
      }
    });
  });
  const http: (
    method: 'GET',
    url: string,
    body: null,
    headers: Record<string, string>,
  ) => Promise<{ status: number; body: unknown; peerPin: string }> = Reflect.get(
    client,
    'http',
  ).bind(client);
  return {
    connections,
    serverConnections,
    requests,
    pins,
    origin,
    async request() {
      return http('GET', `${origin}/health`, null, {});
    },
    holdNextResponse() {
      return new Promise<void>((resolve) => {
        announceRequest = resolve;
      });
    },
    releaseResponse() {
      if (!heldResponse) throw new Error('no response is held');
      heldResponse();
      heldResponse = undefined;
    },
    async close() {
      client.stop();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
      });
    },
  };
}

async function keptConnectionRecordsEachResponsePin(): Promise<void> {
  const peer = await startPeer();
  try {
    for (let i = 1; i <= 32; i++) {
      peer.pins.clear();
      expect(await peer.request()).toEqual({
        status: 200,
        body: { node_id: 'node-1', request: i },
        peerPin: pki.node('node-1').pin,
      });
      expect(peer.pins.get(peer.origin)).toBe(pki.node('node-1').pin);
    }
    expect(peer.connections).toHaveLength(1);
    expect(peer.connections[0]!.resumed).toBe(false);
    expect(peer.connections[0]!.certificate).toBeInstanceOf(Buffer);
    expect(new Set(peer.requests).size).toBe(1);
  } finally {
    await peer.close();
  }
}

async function resumedSessionsRecordEachConnectionPin(): Promise<void> {
  const peer = await startPeer();
  try {
    for (let i = 1; i <= 16; i++) {
      peer.pins.clear();
      expect(await peer.request()).toEqual({
        status: 200,
        body: { node_id: 'node-1', request: i },
        peerPin: pki.node('node-1').pin,
      });
      expect(peer.pins.get(peer.origin)).toBe(pki.node('node-1').pin);
      expect(peer.connections).toHaveLength(i);
      const connection = peer.connections[i - 1]!;
      expect(connection.resumed).toBe(i > 1);
      expect(connection.certificate).toBeInstanceOf(Buffer);
      if (!connection.resumed) await connection.session;
      const closed = once(connection.socket, 'close');
      connection.socket.destroy();
      await closed;
      expect(connection.socket.getPeerCertificate()?.raw).toBeUndefined();
    }
    expect(peer.serverConnections).toEqual([false, ...Array<boolean>(15).fill(true)]);
    expect(new Set(peer.requests).size).toBe(16);
  } finally {
    await peer.close();
  }
}

async function peerClosedConnectionsAreReplaced(): Promise<void> {
  const peer = await startPeer('node-1', true);
  try {
    for (let i = 1; i <= 16; i++) {
      peer.pins.clear();
      expect(await peer.request()).toEqual({
        status: 200,
        body: { node_id: 'node-1', request: i },
        peerPin: pki.node('node-1').pin,
      });
      expect(peer.pins.get(peer.origin)).toBe(pki.node('node-1').pin);
      expect(peer.connections).toHaveLength(i);
      const connection = peer.connections[i - 1]!;
      if (!connection.socket.closed) await once(connection.socket, 'close');
      expect(connection.socket.getPeerCertificate()?.raw).toBeUndefined();
    }
    expect(new Set(peer.requests).size).toBe(16);
    expect(peer.connections.slice(1).every((connection) => connection.resumed)).toBe(true);
  } finally {
    await peer.close();
  }
}

async function unapprovedPeerIsRefusedAtChannelEstablishment(): Promise<void> {
  const peer = await startPeer('outside');
  try {
    await expect(peer.request()).rejects.toThrow(
      'participant TLS authority is outside approved inventory',
    );
    expect(peer.pins.has(peer.origin)).toBe(false);
  } finally {
    await peer.close();
  }
}

async function responsePinIsCheckedAgainstCurrentInventory(): Promise<void> {
  const peer = await startPeer();
  try {
    await peer.request();
    const received = peer.holdNextResponse();
    const response = peer.request();
    await received;
    approveMembers(['outside', ...ids.slice(1)]);
    peer.pins.clear();
    const refused = expect(response).rejects.toThrow(
      'participant TLS authority is outside approved inventory',
    );
    peer.releaseResponse();
    await refused;
    expect(peer.pins.has(peer.origin)).toBe(false);
    expect(peer.connections).toHaveLength(1);
  } finally {
    approveMembers();
    await peer.close();
  }
}

describe('attestor connection peer pins', () => {
  test(keptConnectionRecordsEachResponsePin, keptConnectionRecordsEachResponsePin);
  test(resumedSessionsRecordEachConnectionPin, resumedSessionsRecordEachConnectionPin);
  test(peerClosedConnectionsAreReplaced, peerClosedConnectionsAreReplaced);
  test(
    unapprovedPeerIsRefusedAtChannelEstablishment,
    unapprovedPeerIsRefusedAtChannelEstablishment,
  );
  test(responsePinIsCheckedAgainstCurrentInventory, responsePinIsCheckedAgainstCurrentInventory);
});
