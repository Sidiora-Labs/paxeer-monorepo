import { Buffer } from 'buffer';
import { sha256, stringToHex } from 'viem';
import type { DeviceProofIssuer } from './push.ts';

export type DeviceProofInput = Parameters<DeviceProofIssuer>[0];

export function validateDeviceProof(
  value: unknown,
  input: DeviceProofInput,
): string {
  if (typeof value !== 'string' || !/^[A-Za-z0-9_-]{228}$/.test(value))
    throw new Error('Invalid compactv2 installation proof');
  const wire = Buffer.from(
    value.replace(/-/g, '+').replace(/_/g, '/'),
    'base64',
  );
  const uuid = (offset: number) =>
    wire.subarray(offset, offset + 16).toString('hex');
  const expiry = Number(wire.readBigUInt64BE(97));
  const now = Math.floor(Date.now() / 1000);
  if (
    wire.length !== 171 ||
    wire[0] !== 2 ||
    uuid(1) !== input.accountId.replace(/-/g, '').toLowerCase() ||
    uuid(17) !== input.sessionId.replace(/-/g, '').toLowerCase() ||
    uuid(33) !== input.installationId.replace(/-/g, '').toLowerCase() ||
    wire.subarray(49, 81).toString('hex') !==
      sha256(stringToHex(input.token)).slice(2) ||
    !wire.subarray(81, 97).some((byte) => byte !== 0) ||
    expiry <= now ||
    expiry > now + 300 ||
    wire[105] !== (input.platform === 'ios' ? 2 : 3) ||
    wire[106] !== (input.provider === 'apns' ? 2 : 3)
  )
    throw new Error('Installation proof binding or expiry changed');
  input.signal.throwIfAborted();
  return value;
}
