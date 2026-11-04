import type { Log } from 'viem';

import type { AllowanceType } from 'types/client/revoke';

export interface ApprovalScanIssue {
  kind: 'permission' | 'metadata' | 'balance' | 'timestamp' | 'scan';
  address?: `0x${ string }`;
  spender?: `0x${ string }`;
  cause: unknown;
}

export interface ApprovalScanResult {
  allowances: Array<AllowanceType>;
  issues: Array<ApprovalScanIssue>;
}

export class ApprovalScanError extends Error {
  readonly retryable = true;
  readonly coverage = 'incomplete';

  constructor(readonly partialAllowances: Array<AllowanceType>, readonly issues: Array<ApprovalScanIssue>) {
    super([
      'Approval scan is incomplete. Retry to check all permissions.',
      ...issues.map((issue) => issue.cause instanceof Error ? issue.cause.message : String(issue.cause)),
    ].join('\n'));
    this.name = 'ApprovalScanError';
  }
}

export function checkScanCancellation(signal?: AbortSignal, error?: unknown) {
  if (signal?.aborted) {
    throw new DOMException('Aborted', 'AbortError');
  }
  if (error instanceof Error && error.name === 'AbortError') {
    throw error;
  }
}

export function latestPermissionEvents(events: Array<Log>, permissionKey: (event: Log) => string): Array<Log> {
  const latest = new Map<string, Log>();
  for (const event of events) {
    if (event.removed) continue;
    const key = `${ event.address.toLowerCase() }:${ permissionKey(event).toLowerCase() }`;
    const previous = latest.get(key);
    if (!previous || (event.blockNumber ?? BigInt(0)) > (previous.blockNumber ?? BigInt(0)) ||
      (event.blockNumber === previous.blockNumber && (event.logIndex ?? -1) > (previous.logIndex ?? -1))) {
      latest.set(key, event);
    }
  }
  return Array.from(latest.values());
}
