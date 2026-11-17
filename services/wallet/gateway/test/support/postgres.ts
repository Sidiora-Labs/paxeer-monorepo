import { execFileSync, spawnSync } from 'node:child_process';
import { chmodSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { startOnReservedPorts } from './ports.js';

export interface EphemeralPostgres {
  url: string;
  stop(): Promise<void>;
}

export interface RestartablePostgres extends EphemeralPostgres {
  restart(): Promise<void>;
}

function runAsOwner(bin: string, args: string[]): void {
  const isRoot = typeof process.getuid === 'function' && process.getuid() === 0;
  const [cmd, argv] = isRoot ? ['runuser', ['-u', 'postgres', '--', bin, ...args]] : [bin, args];
  const res = spawnSync(cmd, argv, { encoding: 'utf8' });
  if (res.status !== 0) {
    throw new Error(
      `${bin} ${args.join(' ')} failed with ${res.status}:\nstderr: ${res.stderr ?? ''}\nstdout: ${res.stdout ?? ''}`
    );
  }
}

export async function startPostgresEngine(): Promise<RestartablePostgres> {
  const binDir = process.env.PG_BIN_DIR ?? '/usr/lib/postgresql/16/bin';
  const initdb = join(binDir, 'initdb');
  const pgCtl = join(binDir, 'pg_ctl');
  if (!existsSync(initdb) || !existsSync(pgCtl)) {
    throw new Error(`postgres binaries not found under ${binDir}; set PG_BIN_DIR`);
  }
  const isRoot = typeof process.getuid === 'function' && process.getuid() === 0;
  const dir = mkdtempSync(join(tmpdir(), 'gateway-pg-'));
  const dataDir = join(dir, 'data');
  const logFile = join(dir, 'postgres.log');
  if (isRoot) {
    chmodSync(dir, 0o777);
    execFileSync('chown', ['postgres', dir]);
  }
  runAsOwner(initdb, ['-D', dataDir, '-U', 'postgres', '-A', 'trust', '--no-sync', '-E', 'UTF8']);
  let url: string;
  const stopEngine = async (): Promise<void> => {
    if (existsSync(join(dataDir, 'postmaster.pid'))) {
      runAsOwner(pgCtl, ['-D', dataDir, '-m', 'immediate', '-w', 'stop']);
    }
  };
  const startEngine = async (): Promise<void> => {
    await startOnReservedPorts(
      1,
      async ([reservation]) => {
        const listenOptions = `-p ${reservation!.port} -c listen_addresses=127.0.0.1 -c unix_socket_directories='' -c fsync=off -c max_connections=200`;
        if (existsSync(logFile)) writeFileSync(logFile, '');
        await reservation!.release();
        try {
          runAsOwner(pgCtl, ['-D', dataDir, '-l', logFile, '-w', '-o', listenOptions, 'start']);
        } catch (error) {
          const serverLog = existsSync(logFile) ? readFileSync(logFile, 'utf8') : '';
          throw new Error(`${(error as Error).message}\n${serverLog}`);
        }
        url = `postgres://postgres@127.0.0.1:${reservation!.port}/postgres`;
      },
      stopEngine
    );
  };
  try {
    await startEngine();
  } catch (error) {
    rmSync(dir, { recursive: true, force: true });
    throw error;
  }

  const stop = async (): Promise<void> => {
    try {
      await stopEngine();
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  };
  const restart = async (): Promise<void> => {
    await stopEngine();
    await startEngine();
  };
  return {
    get url() {
      return url;
    },
    stop,
    restart,
  };
}

export async function startPostgres(): Promise<EphemeralPostgres> {
  const database = await startPostgresEngine();
  const { url, stop } = database;
  try {
    process.env.DATABASE_URL = url;
    const { env } = await import('../../src/env.js');
    if (env.DATABASE_URL !== url) {
      throw new Error('startPostgres must run before any module loads src/env.ts');
    }
    const { runMigrations } = await import('../../src/db/migrate.js');
    await runMigrations();
  } catch (err) {
    await stop();
    throw err;
  }
  return database;
}
