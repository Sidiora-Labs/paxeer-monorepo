import type { ErrorEvent, EventHint } from "@sentry/nextjs";

const REDACTED = "[redacted]";
const SENSITIVE_KEY = /address|wallet|privatekey|secret|seed|mnemonic|passphrase|password|signature|authorization|cookie|credential|token|session|email|userid|ipaddress/i;

function redactString(input: string): string {
  // Request URLs and breadcrumb text may contain percent-encoded secrets.
  let value = input;
  try {
    value = decodeURIComponent(value);
  } catch {
    // Malformed URL escapes do not prevent ordinary text redaction.
  }
  return value
    .replace(/\b(?:0x)?[0-9a-f]{64}\b/gi, REDACTED)
    .replace(/\b0x[0-9a-f]{40}\b/gi, REDACTED)
    .replace(/\bpax1[02-9ac-hj-np-z]{20,}\b/gi, REDACTED)
    .replace(/\b(?:Bearer|Basic)\s+[a-z0-9._~+\/-]+=*/gi, REDACTED)
    .replace(
      /(\b(?:private[_ -]?key|secret|seed|mnemonic|passphrase|password|signature|authorization|cookie|credential|token|session)\b["']?\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^&\s,;}]+)/gi,
      `$1${REDACTED}`,
    )
    .replace(/\b(?:[a-z]{1,8}\s+){11,23}[a-z]{1,8}\b/gi, REDACTED);
}

function redactStructured(input: unknown, ancestors: Set<object>): unknown {
  if (typeof input === "string") return redactString(input);
  if (input === null || typeof input !== "object") return input;
  if (ancestors.has(input)) return "[redacted-circular]";
  ancestors.add(input);
  try {
    if (Array.isArray(input)) {
      return input.map((value) => redactStructured(value, ancestors));
    }
    return Object.fromEntries(
      Object.entries(input).map(([key, value]) => [
        redactString(key),
        SENSITIVE_KEY.test(key.replace(/[^a-z0-9]/gi, ""))
          ? REDACTED
          : redactStructured(value, ancestors),
      ]),
    );
  } finally {
    ancestors.delete(input);
  }
}

/** Final privacy boundary shared by browser and edge error reporting. */
export function sanitizeSentryEvent(
  event: ErrorEvent,
  hint: EventHint,
): ErrorEvent | null {
  const original = hint.originalException;
  if (
    original !== null &&
    typeof original === "object" &&
    "name" in original &&
    original.name === "AbortError"
  ) return null;

  return redactStructured(event, new Set()) as ErrorEvent;
}
