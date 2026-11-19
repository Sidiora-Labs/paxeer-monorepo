export function failureStatus(error: unknown): number | undefined {
  return error &&
    typeof error === 'object' &&
    'status' in error &&
    typeof error.status === 'number'
    ? error.status
    : undefined;
}

export function networkFailure(error: unknown): boolean {
  const status = failureStatus(error);
  if (status !== undefined)
    return status === 0 || [502, 503, 504].includes(status);
  return (
    (error instanceof TypeError &&
      /(?:network request failed|failed to fetch|fetch failed|networkerror)/i.test(
        error.message,
      )) ||
    (error instanceof Error &&
      ['TimeoutError', 'AbortError'].includes(error.name)) ||
    (!!error &&
      typeof error === 'object' &&
      'code' in error &&
      error.code === 'NETWORK_INTERRUPTED')
  );
}

export function actionFailure(error: unknown): string {
  const status = failureStatus(error);
  if (status === 401)
    return 'Authorization expired. Sign out and sign in again.';
  if (status === 403)
    return 'This account cannot perform this action. Refresh authorization.';
  if (status === 404)
    return 'This item is no longer available. Go back or refresh.';
  if (status === 409 || status === 412)
    return 'The current revision changed. Reload current data before trying again.';
  if (status === 400 || status === 410)
    return 'This page expired or is invalid. Return to the first page and refresh.';
  if (status === 422) return 'Check the entered values and try again.';
  if (status === 429)
    return 'Too many requests. Wait before trying again.';
  if (status === 501)
    return 'This service is unavailable in this deployment. Go back.';
  if (networkFailure(error))
    return 'Service unavailable. Retry when connected.';
  const message = error instanceof Error ? error.message : '';
  return message &&
    message.length <= 256 &&
    !/(?:0x[0-9a-f]{40,}|https?:|jwt|token=|signature|payload|\r|\n)/i.test(
      message,
    )
    ? message
    : 'Operation failed. Refresh current authorization and retry explicitly.';
}

export function walletRejected(error: unknown): boolean {
  return (
    !!error &&
    typeof error === 'object' &&
    'code' in error &&
    (error.code === 4001 || error.code === 5000)
  );
}

export function pageResourceKey(key: string, cursor?: string): string {
  return cursor ? `${key}:page:${cursor}` : key;
}

export class PageHistory {
  private cursors: string[] = [];
  get cursor(): string | undefined {
    return this.cursors.at(-1);
  }
  next(cursor: string) {
    if (!cursor || cursor.length > 2048 || this.cursors.includes(cursor))
      throw new Error(
        'Page cursor did not advance. Return to the first page.',
      );
    this.cursors.push(cursor);
  }
  previous() {
    this.cursors.pop();
  }
  reset() {
    this.cursors = [];
  }
}

export async function withinDeadline<T>(
  signal: AbortSignal,
  action: (bounded: AbortSignal) => Promise<T>,
  milliseconds = 30000,
): Promise<T> {
  signal.throwIfAborted();
  const controller = new AbortController();
  const abort = () => {
    controller.abort(signal.reason);
    rejectDeadline(
      signal.reason instanceof Error
        ? signal.reason
        : new Error('Request cancelled'),
    );
  };
  signal.addEventListener('abort', abort, { once: true });
  let rejectDeadline: (reason: Error) => void = () => {};
  const deadline = new Promise<never>((_, reject) => {
    rejectDeadline = reject;
  });
  const timer = setTimeout(() => {
    const error = new Error(
      'Service request timed out. Retry when connected.',
    );
    error.name = 'TimeoutError';
    controller.abort(error);
    rejectDeadline(error);
  }, milliseconds);
  try {
    return await Promise.race([action(controller.signal), deadline]);
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', abort);
  }
}

export function notificationLinkPath(path: string): string {
  return /^\/notifications\/[0-9a-f]{64}$/.test(path)
    ? '/notifications'
    : path;
}

export function approvalChainId(chainId: unknown): number {
  if (chainId === undefined) return 125;
  if (
    typeof chainId !== 'number' ||
    !Number.isSafeInteger(chainId) ||
    !(chainId === 125 || (chainId >= 125000001 && chainId <= 4419967295))
  )
    throw new Error('Invalid approval chain');
  return chainId;
}

export function requireRootQuoteChain(chainId: number) {
  if (chainId !== 125)
    throw new Error(
      'Home-chain quotes are unavailable in the current quote contract. Open this market in the web app.',
    );
}
