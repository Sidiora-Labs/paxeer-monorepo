import type { JsonValue } from "../../api/index.ts";

export type AgentRecoveryStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export interface PendingAgentOperation {
  readonly version: 1;
  readonly principal: string;
  readonly target: string;
  readonly scope: string;
  readonly key: string;
  readonly request?: JsonValue;
  rotationRequest?: JsonValue;
  fundingAccess?: string;
  journeyId?: string;
}

export function agentRecoveryStorage(storage?: AgentRecoveryStorage): AgentRecoveryStorage {
  if (storage !== undefined) return storage;
  if (typeof window === "undefined") throw new Error("Private agent recovery storage is unavailable");
  return window.localStorage;
}

export async function agentRecoveryKey(principal: string, target: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(JSON.stringify([principal, target])));
  return `layerx.agents.pending.v1.${Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

export function readAgentOperation(
  storage: AgentRecoveryStorage,
  key: string,
  principal: string,
  target: string,
): PendingAgentOperation | undefined {
  const encoded = storage.getItem(key);
  if (encoded === null) return undefined;
  const value: unknown = JSON.parse(encoded);
  if (typeof value !== "object" || value === null || !("version" in value) || value.version !== 1
    || !("principal" in value) || value.principal !== principal
    || !("target" in value) || value.target !== target
    || !("scope" in value) || typeof value.scope !== "string"
    || !("key" in value) || typeof value.key !== "string" || value.key.length === 0
    || ("fundingAccess" in value && typeof value.fundingAccess !== "string")
    || ("journeyId" in value && typeof value.journeyId !== "string")) {
    throw new Error("Invalid private agent recovery record");
  }
  agentOperationParts(value.scope);
  return value as PendingAgentOperation;
}

export function agentOperationParts(scope: string): [string, string, ...string[]] {
  const parts: unknown = JSON.parse(scope);
  if (!Array.isArray(parts) || parts.length < 2 || !parts.every((part: unknown) => typeof part === "string")) {
    throw new Error("Invalid agent operation identity");
  }
  const [operation, target, ...rest] = parts;
  if (operation === undefined || target === undefined) throw new Error("Invalid agent operation identity");
  return [operation, target, ...rest];
}

export function writeAgentOperation(
  storage: AgentRecoveryStorage,
  storageKey: string,
  operation: PendingAgentOperation,
): void {
  const current = readAgentOperation(storage, storageKey, operation.principal, operation.target);
  if (current !== undefined && current.key !== operation.key) {
    throw new Error("Another agent operation is awaiting reconciliation");
  }
  storage.setItem(storageKey, JSON.stringify(operation));
}

export function clearAgentOperation(
  storage: AgentRecoveryStorage,
  storageKey: string,
  operation: PendingAgentOperation,
): void {
  const current = readAgentOperation(storage, storageKey, operation.principal, operation.target);
  if (current?.key === operation.key) storage.removeItem(storageKey);
}
