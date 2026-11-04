import { RpcError, RpcRequestError, createPublicClient, http } from 'viem';
import type { PublicClient, GetLogsParameters, Log } from 'viem';
import { getLogs as getLogsAction } from 'viem/actions';

const MAX_LOG_REQUESTS = 1024;
const SCAN_TIMEOUT_MS = 60_000;

export class IncompleteLogScanError extends Error {
  readonly cause: unknown;

  constructor(readonly fromBlock: bigint, readonly toBlock: bigint, cause: unknown) {
    super(`Incomplete log scan for blocks ${ fromBlock } to ${ toBlock }`);
    this.name = 'IncompleteLogScanError';
    this.cause = cause;
  }
}

function isRangeLimitError(error: unknown): boolean {
  const seen = new Set<unknown>();
  while (error instanceof Error && !seen.has(error)) {
    seen.add(error);
    const message = error instanceof RpcRequestError || error instanceof RpcError ? error.details || error.shortMessage : '';
    const rangeLimits = [
      /block range.{0,60}(too (wide|large)|exceed|limit)/i,
      /(?:exceed|limit).{0,60}block range/i,
      /eth_getLogs.{0,60}limit.{0,60}blocks? range/i,
      /query returned more than|too many (?:log )?results/i,
      /log response size exceeded|response size.{0,30}exceed/i,
    ];
    if (rangeLimits.some((pattern) => pattern.test(message))) {
      return true;
    }
    error = 'cause' in error ? error.cause : undefined;
  }
  return false;
}

export default async function getLogs(
  publicClient: PublicClient,
  baseFilter: GetLogsParameters,
  fromBlock: bigint,
  toBlock: bigint,
  signal?: AbortSignal,
): Promise<Array<Log>> {
  if (signal?.aborted) {
    throw new DOMException('Aborted', 'AbortError');
  }
  if (fromBlock < BigInt(0) || toBlock < fromBlock || baseFilter.blockHash !== undefined) {
    throw new RangeError('Log scan requires a nonnegative, ordered block range and no blockHash');
  }

  const controller = new AbortController();
  const abort = () => controller.abort(new DOMException('Aborted', 'AbortError'));
  const transportSignal: AbortSignal | undefined = publicClient.transport.type === 'http' ? publicClient.transport.fetchOptions?.signal : undefined;
  signal?.addEventListener('abort', abort, { once: true });
  transportSignal?.addEventListener('abort', abort, { once: true });
  if (transportSignal?.aborted) abort();
  const timeout = setTimeout(() => controller.abort(new Error('Log scan timed out')), SCAN_TIMEOUT_MS);
  let rejectAbort: () => void = () => {};
  const cancelled = new Promise<never>((_, reject) => {
    rejectAbort = () => reject(controller.signal.reason);
    controller.signal.addEventListener('abort', rejectAbort, { once: true });
  });

  try {
    const scanClient = publicClient.transport.type === 'http' ? createPublicClient({
      chain: publicClient.chain,
      transport: http(publicClient.transport.url, {
        fetchOptions: { ...publicClient.transport.fetchOptions, signal: controller.signal },
        retryCount: 0,
        timeout: 10_000,
        batch: false,
      }),
    }) : {
      ...publicClient,
      request: publicClient.transport.request,
    };
    const pending: Array<readonly [bigint, bigint]> = [ [ fromBlock, toBlock ] ];
    const logs: Array<Log> = [];
    let requests = 0;
    while (pending.length) {
      if (controller.signal.aborted) throw controller.signal.reason;
      if (requests >= MAX_LOG_REQUESTS) throw new Error('Log scan request budget exhausted');
      const [ start, end ] = pending.pop()!;
      const filter = { ...baseFilter, fromBlock: start, toBlock: end } as GetLogsParameters;
      requests++;
      try {
        // One chunk at a time bounds both active RPCs and queued split work.
        const chunk = await Promise.race([ getLogsAction(scanClient, filter), cancelled ]);
        if (controller.signal.aborted) throw controller.signal.reason;
        for (const log of chunk) logs.push(log);
      } catch (error) {
        if (controller.signal.aborted || start === end || !isRangeLimitError(error)) throw error;
        const middle = start + (end - start) / BigInt(2);
        pending.push([ middle + BigInt(1), end ], [ start, middle ]);
      }
    }
    return logs;
  } catch (error) {
    if (controller.signal.aborted && controller.signal.reason instanceof DOMException && controller.signal.reason.name === 'AbortError') {
      throw controller.signal.reason;
    }
    throw new IncompleteLogScanError(fromBlock, toBlock, error);
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener('abort', abort);
    transportSignal?.removeEventListener('abort', abort);
    controller.signal.removeEventListener('abort', rejectAbort);
    controller.abort();
  }
}
