import { HttpError, type V2ContractTypes } from '@kindle/sdk';
import { PoolNotFoundError, PoolUnavailableError } from '@/core/market/pool-resolver';

export type CoinSnapshotFailure =
  | { kind: 'not-found' }
  | { kind: 'unavailable'; operation: string; status: number | null }
  | { kind: 'http'; status: number; envelope?: V2ContractTypes['Error']; code: string }
  | { kind: 'error'; name: string; message: string };

export function snapshotFailure(error: unknown): CoinSnapshotFailure {
  return error instanceof PoolNotFoundError ? { kind: 'not-found' }
    : error instanceof PoolUnavailableError ? { kind: 'unavailable', operation: error.operation, status: error.status }
      : error instanceof HttpError ? { kind: 'http', status: error.status, envelope: error.envelope, code: error.code }
        : { kind: 'error', name: error instanceof Error ? error.name : 'Error', message: error instanceof Error ? error.message : String(error) };
}

export function snapshotError(failure: CoinSnapshotFailure, poolAddress: string): Error {
  return failure.kind === 'not-found' ? new PoolNotFoundError(poolAddress)
    : failure.kind === 'unavailable' ? new PoolUnavailableError(failure.operation, failure.status)
      : failure.kind === 'http' ? new HttpError(failure.status, failure.envelope, failure.code)
        : Object.assign(new Error(failure.message), { name: failure.name });
}
