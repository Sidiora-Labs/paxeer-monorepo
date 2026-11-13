import {
  decodeEventLog,
  decodeFunctionResult,
  encodeFunctionData,
  encodeFunctionResult,
  type Hex,
} from 'viem';
import { publicClient } from './chainReads.js';

/**
 * HyperPaxeer network-native precompiles, surfaced to agents through the
 * wallet API so they don't have to hand-encode calldata.
 *
 * Addresses + ABIs verified against:
 *   knowledge/HyperPax-OS/precompiles/{scheduler,streams,eip712,teeattestor}/abi.json
 *   tools/paxeer/lib/config.mjs (PRECOMPILES map)
 *
 * Write builders return `{ to, data, value? }` ready to be threaded through the
 * agent policy engine + signer. Read helpers eth_call the precompile and decode
 * the typed result.
 */

export const PRECOMPILE_ADDRESSES = Object.freeze({
  orob: '0x0000000000000000000000000000000000000901',
  clearing: '0x0000000000000000000000000000000000000902',
  oracle: '0x0000000000000000000000000000000000000903',
  pofq: '0x0000000000000000000000000000000000000904',
  scheduler: '0x0000000000000000000000000000000000000905',
  streams: '0x0000000000000000000000000000000000000906',
  teeAttestor: '0x0000000000000000000000000000000000000907',
  eip712: '0x0000000000000000000000000000000000000908',
  staking: '0x0000000000000000000000000000000000001005',
  bech32: '0x0000000000000000000000000000000000001004',
  p256: '0x0000000000000000000000000000000000001011',
} as const satisfies Record<string, `0x${string}`>);

/** Route keys without a registered merge constructor under precompiles/. */
export const UNSERVED_PRECOMPILES: ReadonlySet<keyof typeof PRECOMPILE_ADDRESSES> = new Set();

export function unservedPrecompileAt(
  to: string | undefined
): keyof typeof PRECOMPILE_ADDRESSES | null {
  if (to === undefined) return null;
  const address = to.toLowerCase();
  for (const key of UNSERVED_PRECOMPILES) {
    if (PRECOMPILE_ADDRESSES[key].toLowerCase() === address) return key;
  }
  return null;
}

export class PrecompileNotServedError extends Error {
  constructor(public readonly precompile: keyof typeof PRECOMPILE_ADDRESSES) {
    super(
      `precompile ${precompile} at ${PRECOMPILE_ADDRESSES[precompile]} is not served by the node`
    );
  }
}

export function servedAddress(key: keyof typeof PRECOMPILE_ADDRESSES): `0x${string}` {
  if (!Object.hasOwn(PRECOMPILE_ADDRESSES, key)) throw new Error(`unknown precompile ${key}`);
  if (UNSERVED_PRECOMPILES.has(key)) throw new PrecompileNotServedError(key);
  return PRECOMPILE_ADDRESSES[key];
}

const servedPrecompiles = new Set<keyof typeof PRECOMPILE_ADDRESSES>();

export async function ensureServed(key: keyof typeof PRECOMPILE_ADDRESSES): Promise<void> {
  const address = servedAddress(key);
  if (servedPrecompiles.has(key)) return;
  try {
    const { data } = await publicClient().call({ to: address, data: '0xffffffff' });
    if (!data || data === '0x') throw new PrecompileNotServedError(key);
  } catch (error) {
    if (!(error instanceof Error) || !error.message.includes('LXP_PRECOMPILE_UNKNOWN_METHOD')) {
      throw error;
    }
  }
  servedPrecompiles.add(key);
}

/** TEE attestation families — order matches x/attestor/types/keys.go + ITEEAttestor.sol. */
export const TEE_FAMILIES = {
  intel_tdx: 0,
  amd_sev_snp: 1,
  nvidia_h100: 2,
  intel_sgx: 3,
} as const;
export type TeeFamilyName = keyof typeof TEE_FAMILIES;

export function precompileAddressBook() {
  const keys = Object.keys(PRECOMPILE_ADDRESSES) as (keyof typeof PRECOMPILE_ADDRESSES)[];
  const served = Object.fromEntries(
    keys.map((key) => [key, !UNSERVED_PRECOMPILES.has(key)])
  ) as Record<keyof typeof PRECOMPILE_ADDRESSES, boolean>;
  return { addresses: PRECOMPILE_ADDRESSES, served, tee_families: TEE_FAMILIES };
}

export interface BuiltCall {
  to: `0x${string}`;
  data: Hex;
  value?: string; // decimal wei string when payable
}

// -----------------------------------------------------------------------------
// ABIs
// -----------------------------------------------------------------------------

export const SCHEDULER_ABI = [
  {
    type: 'function',
    name: 'schedule',
    stateMutability: 'payable',
    inputs: [
      { name: 'target', type: 'address' },
      { name: 'callData', type: 'bytes' },
      { name: 'executeAtBlock', type: 'uint64' },
      { name: 'gasLimit', type: 'uint64' },
    ],
    outputs: [{ name: 'jobId', type: 'uint256' }],
  },
  {
    type: 'function',
    name: 'cancel',
    stateMutability: 'nonpayable',
    inputs: [{ name: 'jobId', type: 'uint256' }],
    outputs: [],
  },
  {
    type: 'function',
    name: 'reschedule',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'jobId', type: 'uint256' },
      { name: 'newBlock', type: 'uint64' },
    ],
    outputs: [],
  },
  {
    type: 'function',
    name: 'getJob',
    stateMutability: 'view',
    inputs: [{ name: 'jobId', type: 'uint256' }],
    outputs: [
      { name: 'id', type: 'uint256' },
      { name: 'creator', type: 'address' },
      { name: 'target', type: 'address' },
      { name: 'callData', type: 'bytes' },
      { name: 'executeAtBlock', type: 'uint64' },
      { name: 'gasLimit', type: 'uint64' },
      { name: 'deposit', type: 'uint256' },
      { name: 'active', type: 'bool' },
    ],
  },
  {
    type: 'function',
    name: 'pending',
    stateMutability: 'view',
    inputs: [{ name: 'creator', type: 'address' }],
    outputs: [{ name: 'jobIds', type: 'uint256[]' }],
  },
  {
    type: 'event',
    name: 'JobScheduled',
    anonymous: false,
    inputs: [
      {
        name: 'jobId',
        type: 'uint256',
        indexed: true,
      },
      {
        name: 'creator',
        type: 'address',
        indexed: true,
      },
      {
        name: 'target',
        type: 'address',
        indexed: false,
      },
      {
        name: 'executeAtBlock',
        type: 'uint64',
        indexed: false,
      },
      {
        name: 'gasLimit',
        type: 'uint64',
        indexed: false,
      },
      {
        name: 'deposit',
        type: 'uint256',
        indexed: false,
      },
    ],
  },
  {
    type: 'event',
    name: 'JobCancelled',
    anonymous: false,
    inputs: [
      {
        name: 'jobId',
        type: 'uint256',
        indexed: true,
      },
      {
        name: 'creator',
        type: 'address',
        indexed: true,
      },
      {
        name: 'refund',
        type: 'uint256',
        indexed: false,
      },
    ],
  },
  {
    type: 'event',
    name: 'JobRescheduled',
    anonymous: false,
    inputs: [
      {
        name: 'jobId',
        type: 'uint256',
        indexed: true,
      },
      {
        name: 'newBlock',
        type: 'uint64',
        indexed: false,
      },
    ],
  },
] as const;

export const STREAMS_ABI = [
  {
    type: 'function',
    name: 'open',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'payee', type: 'address' },
      { name: 'token', type: 'address' },
      { name: 'ratePerSecond', type: 'uint256' },
      { name: 'startTime', type: 'uint64' },
      { name: 'stopTime', type: 'uint64' },
      { name: 'cap', type: 'uint256' },
    ],
    outputs: [{ name: 'streamId', type: 'uint256' }],
  },
  {
    type: 'function',
    name: 'settle',
    stateMutability: 'nonpayable',
    inputs: [{ name: 'streamId', type: 'uint256' }],
    outputs: [{ name: 'paid', type: 'uint256' }],
  },
  {
    type: 'function',
    name: 'close',
    stateMutability: 'nonpayable',
    inputs: [{ name: 'streamId', type: 'uint256' }],
    outputs: [{ name: 'finalPaid', type: 'uint256' }],
  },
  {
    type: 'function',
    name: 'updateRate',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'streamId', type: 'uint256' },
      { name: 'newRate', type: 'uint256' },
    ],
    outputs: [],
  },
  {
    type: 'function',
    name: 'accrued',
    stateMutability: 'view',
    inputs: [{ name: 'streamId', type: 'uint256' }],
    outputs: [{ name: 'amount', type: 'uint256' }],
  },
  {
    type: 'function',
    name: 'getStream',
    stateMutability: 'view',
    inputs: [{ name: 'streamId', type: 'uint256' }],
    outputs: [
      {
        name: 'stream',
        type: 'tuple',
        components: [
          { name: 'id', type: 'uint256' },
          { name: 'payer', type: 'address' },
          { name: 'payee', type: 'address' },
          { name: 'token', type: 'address' },
          { name: 'ratePerSecond', type: 'uint256' },
          { name: 'cap', type: 'uint256' },
          { name: 'startTime', type: 'uint64' },
          { name: 'stopTime', type: 'uint64' },
          { name: 'settled', type: 'uint256' },
          { name: 'active', type: 'bool' },
        ],
      },
    ],
  },
  {
    type: 'event',
    name: 'StreamOpened',
    anonymous: false,
    inputs: [
      { name: 'streamId', type: 'uint256', indexed: true },
      { name: 'payer', type: 'address', indexed: true },
      { name: 'payee', type: 'address', indexed: true },
      { name: 'token', type: 'address', indexed: false },
      { name: 'ratePerSecond', type: 'uint256', indexed: false },
      { name: 'cap', type: 'uint256', indexed: false },
    ],
  },
  {
    type: 'event',
    name: 'StreamSettled',
    anonymous: false,
    inputs: [
      { name: 'streamId', type: 'uint256', indexed: true },
      { name: 'payee', type: 'address', indexed: true },
      { name: 'paid', type: 'uint256', indexed: false },
      { name: 'committed', type: 'uint256', indexed: false },
    ],
  },
  {
    type: 'event',
    name: 'StreamClosed',
    anonymous: false,
    inputs: [
      { name: 'streamId', type: 'uint256', indexed: true },
      { name: 'finalPaid', type: 'uint256', indexed: false },
      { name: 'refund', type: 'uint256', indexed: false },
    ],
  },
  {
    type: 'event',
    name: 'StreamRateUpdated',
    anonymous: false,
    inputs: [
      { name: 'streamId', type: 'uint256', indexed: true },
      { name: 'oldRate', type: 'uint256', indexed: false },
      { name: 'newRate', type: 'uint256', indexed: false },
    ],
  },
] as const;

/** Decode the native streams logs with the same ABI used for wallet calls. */
export function decodeStreamEvent(log: { data: Hex; topics: [Hex, ...Hex[]] }) {
  return decodeEventLog({ abi: STREAMS_ABI, data: log.data, topics: log.topics, strict: true });
}

export const EIP712_ABI = [
  {
    type: 'function',
    name: 'hashTypedData',
    stateMutability: 'pure',
    inputs: [
      { name: 'domainSeparator', type: 'bytes32' },
      { name: 'structHash', type: 'bytes32' },
    ],
    outputs: [{ name: 'digest', type: 'bytes32' }],
  },
  {
    type: 'function',
    name: 'domainSeparator',
    stateMutability: 'pure',
    inputs: [
      { name: 'name', type: 'string' },
      { name: 'version', type: 'string' },
      { name: 'chainId', type: 'uint256' },
      { name: 'verifyingContract', type: 'address' },
    ],
    outputs: [{ name: 'separator', type: 'bytes32' }],
  },
  {
    type: 'function',
    name: 'recoverTypedSigner',
    stateMutability: 'view',
    inputs: [
      { name: 'domainSeparator', type: 'bytes32' },
      { name: 'structHash', type: 'bytes32' },
      { name: 'signature', type: 'bytes' },
    ],
    outputs: [{ name: 'signer', type: 'address' }],
  },
] as const;

/** Frozen event metadata; the native view precompile does not emit this event. */
export const AttestationVerified = {
  type: 'event',
  name: 'AttestationVerified',
  anonymous: false,
  inputs: [
    { name: 'family', type: 'uint8', indexed: true },
    { name: 'mrtd', type: 'bytes32', indexed: true },
    { name: 'reportData', type: 'bytes32', indexed: false },
    { name: 'timestamp', type: 'uint64', indexed: false },
    { name: 'debug', type: 'bool', indexed: false },
  ],
} as const;

export const TEE_ATTESTOR_ABI = [
  {
    type: 'function',
    name: 'bindPeer',
    stateMutability: 'nonpayable',
    inputs: [
      {
        name: 'family',
        type: 'uint8',
      },
      {
        name: 'envelope',
        type: 'bytes',
      },
      {
        name: 'peerKey',
        type: 'bytes32',
      },
      {
        name: 'modelDigest',
        type: 'bytes32',
      },
      {
        name: 'runtimeDigest',
        type: 'bytes32',
      },
    ],
    outputs: [
      {
        name: 'bound',
        type: 'bool',
      },
    ],
  },
  {
    type: 'function',
    name: 'peerBinding',
    stateMutability: 'view',
    inputs: [
      {
        name: 'peerKey',
        type: 'bytes32',
      },
    ],
    outputs: [
      {
        name: 'family',
        type: 'uint8',
      },
      {
        name: 'modelDigest',
        type: 'bytes32',
      },
      {
        name: 'runtimeDigest',
        type: 'bytes32',
      },
      {
        name: 'blockHeight',
        type: 'uint64',
      },
    ],
  },
  {
    type: 'event',
    name: 'PeerAttestationBound',
    anonymous: false,
    inputs: [
      {
        name: 'peerKey',
        type: 'bytes32',
        indexed: true,
      },
      {
        name: 'family',
        type: 'uint8',
        indexed: false,
      },
      {
        name: 'modelDigest',
        type: 'bytes32',
        indexed: false,
      },
      {
        name: 'runtimeDigest',
        type: 'bytes32',
        indexed: false,
      },
    ],
  },
  {
    type: 'function',
    name: 'verify',
    stateMutability: 'view',
    inputs: [
      { name: 'family', type: 'uint8' },
      { name: 'quote', type: 'bytes' },
    ],
    outputs: [
      {
        name: 'att',
        type: 'tuple',
        components: [
          { name: 'family', type: 'uint8' },
          { name: 'mrtd', type: 'bytes32' },
          { name: 'reportData', type: 'bytes32' },
          { name: 'timestamp', type: 'uint64' },
          { name: 'debug', type: 'bool' },
        ],
      },
    ],
  },
  {
    type: 'function',
    name: 'verifyAndExpect',
    stateMutability: 'view',
    inputs: [
      { name: 'family', type: 'uint8' },
      { name: 'quote', type: 'bytes' },
      { name: 'expectedReportData', type: 'bytes32' },
    ],
    outputs: [
      {
        name: 'att',
        type: 'tuple',
        components: [
          { name: 'family', type: 'uint8' },
          { name: 'mrtd', type: 'bytes32' },
          { name: 'reportData', type: 'bytes32' },
          { name: 'timestamp', type: 'uint64' },
          { name: 'debug', type: 'bool' },
        ],
      },
    ],
  },
  {
    type: 'function',
    name: 'rootOf',
    stateMutability: 'view',
    inputs: [
      { name: 'family', type: 'uint8' },
      { name: 'index', type: 'uint256' },
    ],
    outputs: [{ name: 'root', type: 'bytes' }],
  },
  {
    type: 'function',
    name: 'rootCount',
    stateMutability: 'view',
    inputs: [{ name: 'family', type: 'uint8' }],
    outputs: [{ name: 'count', type: 'uint256' }],
  },
  AttestationVerified,
] as const;

// -----------------------------------------------------------------------------
// Scheduler (0x0905)
// -----------------------------------------------------------------------------

export const scheduler = {
  schedule(args: {
    target: `0x${string}`;
    callData?: Hex;
    executeAtBlock: bigint;
    gasLimit: bigint;
    depositWei?: bigint;
  }): BuiltCall {
    const data = encodeFunctionData({
      abi: SCHEDULER_ABI,
      functionName: 'schedule',
      args: [args.target, args.callData ?? '0x', args.executeAtBlock, args.gasLimit],
    });
    return {
      to: servedAddress('scheduler'),
      data,
      value: args.depositWei !== undefined ? args.depositWei.toString() : undefined,
    };
  },
  cancel(jobId: bigint): BuiltCall {
    return {
      to: servedAddress('scheduler'),
      data: encodeFunctionData({ abi: SCHEDULER_ABI, functionName: 'cancel', args: [jobId] }),
    };
  },
  reschedule(jobId: bigint, newBlock: bigint): BuiltCall {
    return {
      to: servedAddress('scheduler'),
      data: encodeFunctionData({
        abi: SCHEDULER_ABI,
        functionName: 'reschedule',
        args: [jobId, newBlock],
      }),
    };
  },
  async getJob(jobId: bigint): Promise<{
    id: string;
    creator: string;
    target: string;
    callData: string;
    executeAtBlock: string;
    gasLimit: string;
    deposit: string;
    active: boolean;
  } | null> {
    await ensureServed('scheduler');
    const [id, creator, target, callData, executeAtBlock, gasLimit, deposit, active] =
      await publicClient().readContract({
        address: servedAddress('scheduler'),
        abi: SCHEDULER_ABI,
        functionName: 'getJob',
        args: [jobId],
      });
    if (id === 0n && active === false) return null;
    return {
      id: id.toString(),
      creator,
      target,
      callData,
      executeAtBlock: executeAtBlock.toString(),
      gasLimit: gasLimit.toString(),
      deposit: deposit.toString(),
      active,
    };
  },
  async pending(creator: `0x${string}`): Promise<string[]> {
    await ensureServed('scheduler');
    const r = (await publicClient().readContract({
      address: servedAddress('scheduler'),
      abi: SCHEDULER_ABI,
      functionName: 'pending',
      args: [creator],
    })) as readonly bigint[];
    return r.map((x) => x.toString());
  },
};

// -----------------------------------------------------------------------------
// PaymentStreams (0x0906)
// -----------------------------------------------------------------------------

export const streams = {
  open(args: {
    payee: `0x${string}`;
    token: `0x${string}`;
    ratePerSecond: bigint;
    startTime?: bigint;
    stopTime?: bigint;
    cap: bigint;
  }): BuiltCall {
    const data = encodeFunctionData({
      abi: STREAMS_ABI,
      functionName: 'open',
      args: [
        args.payee,
        args.token,
        args.ratePerSecond,
        args.startTime ?? 0n,
        args.stopTime ?? 0n,
        args.cap,
      ],
    });
    return { to: servedAddress('streams'), data };
  },
  settle(streamId: bigint): BuiltCall {
    return {
      to: servedAddress('streams'),
      data: encodeFunctionData({ abi: STREAMS_ABI, functionName: 'settle', args: [streamId] }),
    };
  },
  close(streamId: bigint): BuiltCall {
    return {
      to: servedAddress('streams'),
      data: encodeFunctionData({ abi: STREAMS_ABI, functionName: 'close', args: [streamId] }),
    };
  },
  updateRate(streamId: bigint, newRate: bigint): BuiltCall {
    return {
      to: servedAddress('streams'),
      data: encodeFunctionData({
        abi: STREAMS_ABI,
        functionName: 'updateRate',
        args: [streamId, newRate],
      }),
    };
  },
  async accrued(streamId: bigint): Promise<string> {
    await ensureServed('streams');
    const r = (await publicClient().readContract({
      address: servedAddress('streams'),
      abi: STREAMS_ABI,
      functionName: 'accrued',
      args: [streamId],
    })) as bigint;
    return r.toString();
  },
  async getStream(streamId: bigint): Promise<Record<string, string | boolean> | null> {
    await ensureServed('streams');
    const r = (await publicClient().readContract({
      address: servedAddress('streams'),
      abi: STREAMS_ABI,
      functionName: 'getStream',
      args: [streamId],
    })) as {
      id: bigint;
      payer: string;
      payee: string;
      token: string;
      ratePerSecond: bigint;
      cap: bigint;
      startTime: bigint;
      stopTime: bigint;
      settled: bigint;
      active: boolean;
    };
    if (r.id === 0n && !r.active) return null;
    return {
      id: r.id.toString(),
      payer: r.payer,
      payee: r.payee,
      token: r.token,
      ratePerSecond: r.ratePerSecond.toString(),
      cap: r.cap.toString(),
      startTime: r.startTime.toString(),
      stopTime: r.stopTime.toString(),
      settled: r.settled.toString(),
      active: r.active,
    };
  },
};

// -----------------------------------------------------------------------------
// EIP-712 helper (0x0908) — all pure/view
// -----------------------------------------------------------------------------

type Eip712ReadMethod = 'hashTypedData' | 'domainSeparator' | 'recoverTypedSigner';

/** Reject incomplete, trailing, or noncanonical return words before ABI decoding. */
export function decodeEip712Result(method: Eip712ReadMethod, data: Hex | undefined): Hex {
  if (!data || !/^0x[0-9a-fA-F]{64}$/.test(data)) {
    throw new Error(`Malformed EIP-712 ${method} result: expected one ABI word`);
  }
  if (method === 'recoverTypedSigner' && !/^0x0{24}/.test(data)) {
    throw new Error('Malformed EIP-712 recoverTypedSigner result: nonzero address padding');
  }
  return decodeFunctionResult({ abi: EIP712_ABI, functionName: method, data });
}

async function hashTypedData(domainSeparator: Hex, structHash: Hex): Promise<Hex> {
  await ensureServed('eip712');
  const { data } = await publicClient().call({
    to: servedAddress('eip712'),
    data: encodeFunctionData({
      abi: EIP712_ABI,
      functionName: 'hashTypedData',
      args: [domainSeparator, structHash],
    }),
  });
  return decodeEip712Result('hashTypedData', data);
}

async function domainSeparator(args: {
  name: string;
  version: string;
  chainId: bigint;
  verifyingContract: `0x${string}`;
}): Promise<Hex> {
  await ensureServed('eip712');
  const { data } = await publicClient().call({
    to: servedAddress('eip712'),
    data: encodeFunctionData({
      abi: EIP712_ABI,
      functionName: 'domainSeparator',
      args: [args.name, args.version, args.chainId, args.verifyingContract],
    }),
  });
  return decodeEip712Result('domainSeparator', data);
}

async function recoverTypedSigner(
  domainSeparator: Hex,
  structHash: Hex,
  signature: Hex
): Promise<Hex> {
  await ensureServed('eip712');
  const { data } = await publicClient().call({
    to: servedAddress('eip712'),
    data: encodeFunctionData({
      abi: EIP712_ABI,
      functionName: 'recoverTypedSigner',
      args: [domainSeparator, structHash, signature],
    }),
  });
  return decodeEip712Result('recoverTypedSigner', data);
}

export const eip712 = { hashTypedData, domainSeparator, recoverTypedSigner };

// -----------------------------------------------------------------------------
// TEE Attestor (0x0907) — all view
// -----------------------------------------------------------------------------

export interface Attestation {
  family: number;
  family_name: TeeFamilyName | 'unknown';
  mrtd: Hex;
  report_data: Hex;
  timestamp: string;
  debug: boolean;
}

function familyName(f: number): TeeFamilyName | 'unknown' {
  const hit = (Object.entries(TEE_FAMILIES) as [TeeFamilyName, number][]).find(([, v]) => v === f);
  return hit ? hit[0] : 'unknown';
}

function shapeAttestation(r: {
  family: number;
  mrtd: Hex;
  reportData: Hex;
  timestamp: bigint;
  debug: boolean;
}): Attestation {
  return {
    family: r.family,
    family_name: familyName(r.family),
    mrtd: r.mrtd,
    report_data: r.reportData,
    timestamp: r.timestamp.toString(),
    debug: r.debug,
  };
}

/** Decode only canonical, non-debug results for the requested attestation family. */
export function decodeTeeAttestationResult(data: Hex, expectedFamily: number): Attestation {
  const result = decodeFunctionResult({ abi: TEE_ATTESTOR_ABI, functionName: 'verify', data });
  const canonical = encodeFunctionResult({
    abi: TEE_ATTESTOR_ABI,
    functionName: 'verify',
    result,
  });
  if (
    canonical.toLowerCase() !== data.toLowerCase() ||
    result.family !== expectedFamily ||
    familyName(result.family) === 'unknown' ||
    result.debug
  ) {
    throw new Error('invalid attestation result');
  }
  return shapeAttestation(result);
}

export async function verify(family: number, quote: Hex): Promise<Attestation> {
  await ensureServed('teeAttestor');
  const { data } = await publicClient().call({
    to: servedAddress('teeAttestor'),
    data: encodeFunctionData({
      abi: TEE_ATTESTOR_ABI,
      functionName: 'verify',
      args: [family, quote],
    }),
  });
  if (!data) throw new Error('missing attestation result');
  return decodeTeeAttestationResult(data, family);
}

export async function verifyAndExpect(
  family: number,
  quote: Hex,
  expectedReportData: Hex
): Promise<Attestation> {
  await ensureServed('teeAttestor');
  const { data } = await publicClient().call({
    to: servedAddress('teeAttestor'),
    data: encodeFunctionData({
      abi: TEE_ATTESTOR_ABI,
      functionName: 'verifyAndExpect',
      args: [family, quote, expectedReportData],
    }),
  });
  if (!data) throw new Error('missing attestation result');
  return decodeTeeAttestationResult(data, family);
}

export async function rootOf(family: number, index: bigint): Promise<Hex> {
  await ensureServed('teeAttestor');
  return publicClient().readContract({
    address: servedAddress('teeAttestor'),
    abi: TEE_ATTESTOR_ABI,
    functionName: 'rootOf',
    args: [family, index],
  });
}

export async function rootCount(family: number): Promise<string> {
  await ensureServed('teeAttestor');
  const count = await publicClient().readContract({
    address: servedAddress('teeAttestor'),
    abi: TEE_ATTESTOR_ABI,
    functionName: 'rootCount',
    args: [family],
  });
  return count.toString();
}

export const teeAttestor = { verify, verifyAndExpect, rootOf, rootCount };
