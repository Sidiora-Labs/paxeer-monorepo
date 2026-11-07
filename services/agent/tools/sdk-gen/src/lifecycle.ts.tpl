
export const PROGRAM_LIFECYCLE_METADATA_JSON = '{{LIFECYCLE_METADATA}}';
export const PROGRAM_LIFECYCLE_TRANSPORT = JSON.parse(PROGRAM_LIFECYCLE_METADATA_JSON) as {
  method: "eth_sendRawTransaction";
  transaction_type: "EIP-1559";
  chain_id: 125;
  operations: Record<ProgramLifecycleOperation, { precompile: string; selectors: readonly string[] }>;
  rungs: { submitted: string; receipted: string; committed: string };
  unknown: string;
};
export type ProgramLifecycleOperation = "program.deploy" | "program.upgrade" | "program.wind-down";
export const PROGRAM_LIFECYCLE_REQUEST_JSON = '{"jsonrpc":"2.0","id":{{request_id}},"method":"eth_sendRawTransaction","params":["{{signed_activity}}"]}';

// The caller binds and signs the exact ABI calldata before constructing this envelope.
// A successful RPC response establishes submission only; higher rungs require verification.
export function programLifecycleRequest(operation: ProgramLifecycleOperation, signedActivity: string, requestId: number): string {
  if (!Object.prototype.hasOwnProperty.call(PROGRAM_LIFECYCLE_TRANSPORT.operations, operation)) throw new TypeError("invalid lifecycle operation");
  if (!Number.isSafeInteger(requestId) || requestId < 1) throw new RangeError("invalid request id");
  if (!/^0x02(?:[0-9a-f]{2})+$/.test(signedActivity) || signedActivity.length > 2097154) throw new TypeError("invalid signed EIP-1559 transaction");
  return PROGRAM_LIFECYCLE_REQUEST_JSON.replace("{{request_id}}", String(requestId)).replace("{{signed_activity}}", signedActivity);
}
