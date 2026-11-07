import { createHash } from 'node:crypto';
import { existsSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { Client } from 'pg';
import { expect, test } from 'vitest';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../../src/db/testDatabase.js';

async function gatewayOwnsTestDatabaseDependencies(): Promise<void> {
  const manifest = JSON.parse(await readFile(new URL('../../package.json', import.meta.url), 'utf8'));
  expect(manifest.devDependencies['@electric-sql/pglite']).toBe('0.5.15');
  expect(manifest.devDependencies['@electric-sql/pglite-socket']).toBe('0.2.18');
  expect(existsSync(new URL('../../src/db/package.json', import.meta.url))).toBe(false);
  expect(existsSync(new URL('../../src/db/pnpm-workspace.yaml', import.meta.url))).toBe(false);
}

async function gatewayTestDatabaseLoadsPgcryptoOverPostgresSocket(): Promise<void> {
  const db = await PGlite.create({ extensions: { pgcrypto } });
  try {
    const server = new PGLiteSocketServer({ db, host: '127.0.0.1', port: 0 });
    try {
      await server.start();
      const client = new Client({
        connectionString: `postgres://postgres:postgres@${server.getServerConn()}/postgres`,
      });
      try {
        await client.connect();
        await client.query('CREATE EXTENSION pgcrypto');
        await client.query('CREATE TABLE dependency_probe (payload text NOT NULL)');
        const payload = 'wallet-gateway-22.31.447';
        await client.query('INSERT INTO dependency_probe (payload) VALUES ($1)', [payload]);
        const result = await client.query<{ payload: string; digest: string }>(
          "SELECT payload, encode(digest(payload, 'sha256'), 'hex') AS digest FROM dependency_probe",
        );
        expect(result.rows).toEqual([
          {
            payload,
            digest: createHash('sha256').update(payload).digest('hex'),
          },
        ]);
        expect(
          (await db.query<{ payload: string }>('SELECT payload FROM dependency_probe')).rows,
        ).toEqual([{ payload }]);
      } finally {
        await client.end();
      }
    } finally {
      await server.stop();
    }
  } finally {
    await db.close();
  }
}

test(gatewayOwnsTestDatabaseDependencies.name, gatewayOwnsTestDatabaseDependencies);
test(
  gatewayTestDatabaseLoadsPgcryptoOverPostgresSocket.name,
  gatewayTestDatabaseLoadsPgcryptoOverPostgresSocket,
  30_000,
);
