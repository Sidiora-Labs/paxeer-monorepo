import { z } from "zod";

export class AgentKitError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly details?: unknown,
  ) {
    super(message);
    this.name = "AgentKitError";
  }
}

export type ToolResult =
  | { ok: true; data: unknown }
  | { ok: false; error: { code: string; message: string; details?: unknown } };

export function errorResult(error: unknown): ToolResult {
  if (error instanceof AgentKitError) {
    return {
      ok: false,
      error: {
        code: error.code,
        message: error.message,
        details: error.details,
      },
    };
  }
  if (error instanceof z.ZodError) {
    return {
      ok: false,
      error: {
        code: "INVALID_ARGUMENTS",
        message: "Tool arguments do not match the schema",
        details: error.issues,
      },
    };
  }
  if (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    "message" in error &&
    typeof error.code === "string" &&
    typeof error.message === "string"
  ) {
    return { ok: false, error: { code: error.code, message: error.message } };
  }
  return {
    ok: false,
    error: {
      code: "REQUEST_FAILED",
      message: "The trading request could not be completed",
    },
  };
}

export function jsonText(value: unknown): string {
  return JSON.stringify(value, (_, v: unknown) =>
    typeof v === "bigint" ? v.toString() : v,
  );
}
