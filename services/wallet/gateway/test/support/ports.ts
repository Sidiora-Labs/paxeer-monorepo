import type { ChildProcess } from 'node:child_process';
import { createServer } from 'node:net';

export const PORT_START_ATTEMPTS = 3;

export interface PortReservation {
  readonly port: number;
  release(): Promise<void>;
}

export function reservePort(host = '127.0.0.1'): Promise<PortReservation> {
  return new Promise((resolve, reject) => {
    const server = createServer();
    server.once('error', reject);
    server.listen(0, host, () => {
      const address = server.address();
      if (!address || typeof address === 'string') {
        server.close();
        reject(new Error('port reservation did not bind a TCP port'));
        return;
      }
      let released: Promise<void> | undefined;
      resolve({
        port: address.port,
        release: () =>
          (released ??= new Promise<void>((done, failed) => {
            server.close((error) => (error ? failed(error) : done()));
          })),
      });
    });
  });
}

export async function reservePorts(count: number, host = '127.0.0.1'): Promise<PortReservation[]> {
  if (!Number.isSafeInteger(count) || count < 1) {
    throw new RangeError('port reservation count must be a positive safe integer');
  }
  const results = await Promise.allSettled(Array.from({ length: count }, () => reservePort(host)));
  const held = results.flatMap((result) => (result.status === 'fulfilled' ? [result.value] : []));
  const failure = results.find((result) => result.status === 'rejected');
  if (failure) {
    await Promise.all(held.map((reservation) => reservation.release()));
    throw failure.reason;
  }
  return held;
}

export function isAddressInUse(error: unknown): boolean {
  return error instanceof Error && /\bEADDRINUSE\b|\baddress already in use\b/i.test(error.message);
}

export async function startOnReservedPorts<T>(
  count: number,
  start: (reservations: PortReservation[]) => Promise<T>,
  stop: () => Promise<void>,
  host = '127.0.0.1'
): Promise<T> {
  if (!Number.isSafeInteger(count) || count < 1) {
    throw new RangeError('port reservation count must be a positive safe integer');
  }
  const tried = new Set<number>();
  for (let attempt = 1; attempt <= PORT_START_ATTEMPTS; attempt++) {
    const reservations: PortReservation[] = [];
    const reused: PortReservation[] = [];
    try {
      while (reservations.length < count) {
        const held = await reservePorts(count - reservations.length, host);
        for (const reservation of held) {
          if (tried.has(reservation.port)) reused.push(reservation);
          else reservations.push(reservation);
        }
      }
      await Promise.all(reused.map((reservation) => reservation.release()));
      for (const reservation of reservations) tried.add(reservation.port);
      try {
        return await start(reservations);
      } catch (error) {
        await stop();
        if (!isAddressInUse(error) || attempt === PORT_START_ATTEMPTS) throw error;
      }
    } finally {
      await Promise.all([...reservations, ...reused].map((reservation) => reservation.release()));
    }
  }
  throw new Error('port start attempts exhausted');
}

export function assertProcessRunning(child: ChildProcess, logs: string[]): void {
  const output = logs.join('');
  if (isAddressInUse(new Error(output))) throw new Error(output);
  if (!child.pid || child.exitCode !== null || child.signalCode !== null) {
    throw new Error(`process exited before readiness: ${output}`);
  }
}

export async function stopPortProcess(child: ChildProcess | undefined): Promise<void> {
  if (!child?.pid || child.exitCode !== null || child.signalCode !== null) return;
  const closed = new Promise<void>((resolve) => child.once('close', () => resolve()));
  child.kill('SIGTERM');
  const timer = setTimeout(() => child.kill('SIGKILL'), 5_000);
  try {
    await closed;
  } finally {
    clearTimeout(timer);
  }
}
