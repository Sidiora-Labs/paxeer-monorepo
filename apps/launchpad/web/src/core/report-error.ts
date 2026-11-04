import { CORE_DEPLOYMENT } from './network/deployment';

type ErrorContext = Record<string, unknown> | string;

const AREAS = [
  'metadata-client',
  'market-stream',
  'auth-callback',
  'widget',
  'global-search',
  'platform-metrics',
  'market-widget',
] as const;
const ACTIONS = ['consumer', 'subscribe'] as const;
const CODES = ['chunk_load', 'unauthorized', 'unavailable', 'render_failed'] as const;
const SENTRY_DSN = process.env.NEXT_PUBLIC_SENTRY_DSN;

/** Only reviewed diagnostic categories and public correlation identifiers leave the page. */
export function redactedErrorPayload(error: unknown, context?: ErrorContext) {
  const input = typeof context === 'string' ? { area: context } : (context ?? {});
  const area = AREAS.find((value) => value === input.area) ?? 'widget';
  const action = ACTIONS.find((value) => value === input.action);
  const code = CODES.find((value) => value === input.code) ?? 'render_failed';
  const correlationId =
    typeof input.correlationId === 'string' &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(input.correlationId)
      ? input.correlationId
      : crypto.randomUUID();
  // Free-form messages, stacks and custom Error properties can all contain credentials.
  const name =
    error instanceof Error &&
    ['Error', 'TypeError', 'RangeError', 'AbortError', 'TimeoutError'].includes(error.name)
      ? error.name
      : 'Error';
  let url: string | undefined;
  if (typeof window !== 'undefined') {
    const location = new URL(window.location.href);
    url = `${location.origin}${location.pathname}`;
  }
  return {
    message: `${name}: ${code}`,
    context: {
      area,
      code,
      correlationId,
      deploymentVersion: CORE_DEPLOYMENT.manifestSha256,
      ...(action ? { action } : {}),
    },
    url,
    timestamp: Date.now(),
  };
}

export function reportError(error: unknown, context?: ErrorContext): void {
  const payload = redactedErrorPayload(error, context);
  const safeError = new Error(payload.message);
  safeError.stack = undefined;

  if (process.env.NODE_ENV !== 'production') console.warn('[Sidiora]', payload.context, safeError);

  if (SENTRY_DSN) {
    import('@sentry/nextjs')
      .then((module) => {
        const Sentry = ('default' in module ? module.default : module) as typeof module;
        Sentry.withScope((scope) => {
          scope.clear();
          // Browser integrations may add the current auth URL or breadcrumbs.
          scope.addEventProcessor((event) => ({
            event_id: event.event_id,
            timestamp: event.timestamp,
            level: 'error',
            message: payload.message,
            extra: payload.context,
          }));
          Sentry.captureException(safeError);
        });
      })
      .catch(() => {
        sendBeacon(payload);
      });
    return;
  }
  sendBeacon(payload);
}

function sendBeacon(payload: ReturnType<typeof redactedErrorPayload>): void {
  if (process.env.NODE_ENV !== 'production' || typeof window === 'undefined') return;
  try {
    navigator.sendBeacon('/api/errors', JSON.stringify(payload));
  } catch {
    /* best-effort */
  }
}
