import { expect, test } from 'vitest';

function vitestEnvOriginsAreLoopback() {
  const names = [
    'NEXT_PUBLIC_DATA_API_URL',
    'NEXT_PUBLIC_GATEWAY_URL',
    'NEXT_PUBLIC_METADATA_API_URL',
    'NEXT_PUBLIC_PNL_API_URL',
  ] as const;
  const ports = new Set<string>();

  for (const name of names) {
    const value = process.env[name];
    if (value === undefined) throw new Error(`${name} is not set`);
    const url = new URL(value);
    expect(url.protocol, name).toBe('http:');
    expect(url.hostname, name).toBe('127.0.0.1');
    expect(url.port, name).not.toBe('');
    ports.add(url.port);
  }

  expect(ports.size).toBe(names.length);
}

test(vitestEnvOriginsAreLoopback);
