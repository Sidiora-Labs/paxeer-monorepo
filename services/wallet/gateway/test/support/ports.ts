import { createServer } from 'node:net';

export interface PortReservation {
  readonly port: number;
  release(): Promise<void>;
}

export function reservePort(): Promise<PortReservation> {
  return new Promise((resolve, reject) => {
    const server = createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      if (!address || typeof address === 'string') {
        server.close();
        reject(new Error('port reservation did not bind a TCP port'));
        return;
      }
      resolve({
        port: address.port,
        release: () =>
          new Promise<void>((released, failed) => {
            server.close((error) => (error ? failed(error) : released()));
          }),
      });
    });
  });
}

export async function reservePorts(count: number): Promise<PortReservation[]> {
  if (!Number.isSafeInteger(count) || count < 1) {
    throw new RangeError('port reservation count must be a positive safe integer');
  }
  const results = await Promise.allSettled(Array.from({ length: count }, () => reservePort()));
  const held = results.flatMap((result) =>
    result.status === 'fulfilled' ? [result.value] : []
  );
  const failure = results.find((result) => result.status === 'rejected');
  if (failure) {
    await Promise.all(held.map((reservation) => reservation.release()));
    throw failure.reason;
  }
  return held;
}
