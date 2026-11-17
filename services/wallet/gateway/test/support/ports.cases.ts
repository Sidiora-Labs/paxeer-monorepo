import { spawn, type ChildProcess } from 'node:child_process';
import { createServer, type Server } from 'node:net';
import { expect, test } from 'vitest';
import { PORT_START_ATTEMPTS, startOnReservedPorts, stopPortProcess } from './ports.js';

const listener = `
  const server = require('node:net').createServer();
  server.listen(Number(process.argv[1]), process.argv[2], () => process.send('listening'));
`;

function startListener(port: number, host: string) {
  const child = spawn(process.execPath, ['-e', listener, '--', String(port), host], {
    stdio: ['ignore', 'pipe', 'pipe', 'ipc'],
  });
  const output: string[] = [];
  child.stderr!.on('data', (chunk: Buffer) => output.push(chunk.toString()));
  const ready = new Promise<void>((resolve, reject) => {
    child.once('message', () => resolve());
    child.once('error', reject);
    child.once('close', () => reject(new Error(output.join(''))));
  });
  return { child, ready };
}

async function occupy(port: number): Promise<Server> {
  const server = createServer();
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', resolve);
  });
  return server;
}

async function close(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
}

async function occupiedStartUsesFreshPort() {
  const ports: number[] = [];
  const blockers: Server[] = [];
  let child: ChildProcess | undefined;
  let stopped = 0;
  try {
    const port = await startOnReservedPorts(
      1,
      async ([reservation]) => {
        ports.push(reservation!.port);
        await reservation!.release();
        if (ports.length === 1) blockers.push(await occupy(reservation!.port));
        const started = startListener(reservation!.port, '::');
        child = started.child;
        await started.ready;
        return reservation!.port;
      },
      async () => {
        await stopPortProcess(child);
        stopped++;
      },
      '::'
    );
    expect(ports).toHaveLength(2);
    expect(ports[0]).not.toBe(port);
    expect(port).toBe(ports[1]);
    expect(stopped).toBe(1);
    expect(child!.exitCode).toBeNull();
  } finally {
    await stopPortProcess(child);
    await Promise.all(blockers.map(close));
  }
}

async function occupiedStartsKeepLastProcessError() {
  const ports: number[] = [];
  const blockers: Server[] = [];
  const errors: unknown[] = [];
  let child: ChildProcess | undefined;
  let stopped = 0;
  try {
    const result = await startOnReservedPorts(
      1,
      async ([reservation]) => {
        ports.push(reservation!.port);
        await reservation!.release();
        blockers.push(await occupy(reservation!.port));
        const started = startListener(reservation!.port, '127.0.0.1');
        child = started.child;
        try {
          await started.ready;
        } catch (error) {
          errors.push(error);
          throw error;
        }
      },
      async () => {
        await stopPortProcess(child);
        stopped++;
      }
    ).catch((error: unknown) => error);
    expect(PORT_START_ATTEMPTS).toBeGreaterThanOrEqual(3);
    expect(ports).toHaveLength(PORT_START_ATTEMPTS);
    expect(new Set(ports).size).toBe(PORT_START_ATTEMPTS);
    expect(stopped).toBe(PORT_START_ATTEMPTS);
    expect(result).toBe(errors.at(-1));
    expect(result).toBeInstanceOf(Error);
    expect((result as Error).message).toContain('EADDRINUSE');
  } finally {
    await stopPortProcess(child);
    await Promise.all(blockers.map(close));
  }
}

async function otherStartErrorsAreImmediate() {
  let starts = 0;
  let stopped = 0;
  let child: ChildProcess | undefined;
  let processError: unknown;
  try {
    const result = await startOnReservedPorts(
      1,
      async ([reservation]) => {
        starts++;
        await reservation!.release();
        const started = startListener(-1, '127.0.0.1');
        child = started.child;
        try {
          await started.ready;
        } catch (error) {
          processError = error;
          throw error;
        }
      },
      async () => {
        await stopPortProcess(child);
        stopped++;
      }
    ).catch((error: unknown) => error);
    expect(starts).toBe(1);
    expect(stopped).toBe(1);
    expect(result).toBe(processError);
    expect(result).toBeInstanceOf(Error);
    expect((result as Error).message).toContain('ERR_SOCKET_BAD_PORT');
  } finally {
    await stopPortProcess(child);
  }
}

export function registerPortCases(): void {
  test(occupiedStartUsesFreshPort, occupiedStartUsesFreshPort);
  test(occupiedStartsKeepLastProcessError, occupiedStartsKeepLastProcessError);
  test(otherStartErrorsAreImmediate, otherStartErrorsAreImmediate);
}
