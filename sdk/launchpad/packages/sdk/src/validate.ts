import type { ContractTypes } from './types.gen.js';
import type { ErrorObject } from 'ajv/dist/2020.js';
import { contractValidators } from './validators-v1.gen.js';
import type { ChainsResponse, ChainDetailResponse, ChainRevenueResponse } from './chains.js';

// Unknown is the untrusted-input boundary, never a generated DTO fallback.
// Validators are compiled ahead of time so no code generation happens under a browser content security policy.
export function errorsText(errors: ReadonlyArray<ErrorObject> | null | undefined): string {
  return (errors ?? []).map(error => `data${error.instancePath} ${error.message ?? error.keyword}`).join(', ');
}
export class ContractValidationError extends Error {
  constructor(readonly contract: string, readonly violations: ReadonlyArray<ErrorObject>) {
    super(`Invalid contract ${contract}: ${violations.map(e => `${e.instancePath} ${e.message ?? e.keyword}`).join('; ')}`);
    this.name = 'ContractValidationError';
  }
}
export function validateContract(name: string, value: unknown, schemaMajor = 1): boolean {
  if (schemaMajor !== 1) throw new Error(`Unsupported schema major ${schemaMajor}`);
  const validator = Object.hasOwn(contractValidators, name) ? contractValidators[name] : undefined;
  if (!validator) throw new Error(`Unknown contract ${name}`);
  return validator(value);
}
export function decodeContract<N extends keyof ContractTypes>(name: N, value: unknown, schemaMajor = 1): ContractTypes[N] {
  if (!validateContract(name, value, schemaMajor)) throw new ContractValidationError(name, [...(contractValidators[name]?.errors ?? [])]);
  // Caller can only assert a generated named type; runtime validation is authoritative.
  return value as ContractTypes[N];
}

function object(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new TypeError('Invalid chain response');
  return value as Record<string, unknown>;
}

function decimal(value: unknown, positive = false): boolean {
  return typeof value === 'string' && /^(0|[1-9][0-9]*)$/.test(value) && (!positive || value !== '0');
}

function address(value: unknown): boolean {
  return typeof value === 'string' && /^0x[0-9a-f]{40}$/.test(value);
}

function chainRow(value: unknown): void {
  const row = object(value);
  if (!decimal(row.chain, true) || !decimal(row.chainId, true) || !decimal(row.nftId) || !decimal(row.blockNumber) ||
      !address(row.pool) || !address(row.token) || !address(row.creator) ||
      (row.status !== 'active' && row.status !== 'retired')) throw new TypeError('Invalid chain row');
}

function envelope(value: unknown): Record<string, unknown> {
  const response = object(value);
  if (typeof response.requestId !== 'string' || !response.requestId ||
      typeof response.generatedAt !== 'string' || !Number.isFinite(Date.parse(response.generatedAt)) ||
      !['complete', 'partial', 'stale', 'quarantined'].includes(String(response.quality)) ||
      (response.nextCursor !== null && typeof response.nextCursor !== 'string') ||
      (response.revision !== null && !decimal(response.revision, true)) ||
      response.watermark !== null) throw new TypeError('Invalid chain response envelope');
  return object(response.data);
}

export function validateChainsResponse(value: unknown): ChainsResponse {
  const data = envelope(value);
  if (!Array.isArray(data.items)) throw new TypeError('Invalid chains page');
  data.items.forEach(chainRow);
  return value as ChainsResponse;
}

export function validateChainDetailResponse(value: unknown): ChainDetailResponse {
  const data = envelope(value);
  chainRow(data.chain);
  return value as ChainDetailResponse;
}

export function validateChainRevenueResponse(value: unknown): ChainRevenueResponse {
  const data = envelope(value);
  if (!decimal(data.chain, true) || !Array.isArray(data.epochs)) throw new TypeError('Invalid chain revenue');
  for (const value of data.epochs) {
    const epoch = object(value);
    if (!decimal(epoch.epoch, true) || !decimal(epoch.epochAmount) || !decimal(epoch.snapshotBlock) || !decimal(epoch.eligibleSupply)) throw new TypeError('Invalid chain revenue epoch');
  }
  if (data.holder !== undefined || data.claims !== undefined) {
    if (!address(data.holder) || !Array.isArray(data.claims)) throw new TypeError('Invalid chain revenue claims');
    for (const value of data.claims) {
      const claim = object(value);
      if (!decimal(claim.epoch, true) || claim.holder !== data.holder || !decimal(claim.amount) || !decimal(claim.blockNumber)) throw new TypeError('Invalid chain revenue claim');
    }
  }
  return value as ChainRevenueResponse;
}
