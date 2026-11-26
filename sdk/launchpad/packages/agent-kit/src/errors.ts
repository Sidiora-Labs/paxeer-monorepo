export class AgentKitError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status?: number,
    readonly details?: unknown,
  ) {
    super(message);
    this.name = 'AgentKitError';
  }
}

export function requireThat(value: unknown, code: string): asserts value {
  if (!value) throw new AgentKitError(code, code);
}

export function record(value: unknown): Record<string, unknown> {
  requireThat(
    value !== null && typeof value === 'object' && !Array.isArray(value),
    'invalid_gateway_response',
  );
  return value as Record<string, unknown>;
}

export function refusal(status: number, payload: unknown): AgentKitError {
  const body = record(payload);
  const error = body.error;
  const envelope = error && typeof error === 'object' ? record(error) : body;
  const code =
    typeof error === 'string'
      ? error
      : typeof envelope.code === 'string'
        ? envelope.code
        : 'gateway_refused';
  return new AgentKitError(
    code,
    typeof envelope.message === 'string' ? envelope.message : code,
    status,
    payload,
  );
}

export type ToolResult =
  | { ok: true; data: unknown }
  | { ok: false; error: { code: string; message: string; status?: number; details?: unknown } };

export async function toolResult(run: () => Promise<unknown>): Promise<ToolResult> {
  try {
    return { ok: true, data: await run() };
  } catch (error) {
    if (error instanceof AgentKitError) {
      return {
        ok: false,
        error: {
          code: error.code,
          message: error.message,
          ...(error.status === undefined ? {} : { status: error.status }),
          ...(error.details === undefined ? {} : { details: error.details }),
        },
      };
    }
    if (error instanceof Error && 'code' in error && typeof error.code === 'string') {
      return { ok: false, error: { code: error.code, message: error.code } };
    }
    return {
      ok: false,
      error: { code: 'operation_failed', message: 'The operation did not complete.' },
    };
  }
}
