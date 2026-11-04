import type { SequencerReceipt } from "./verifier.js";

/** Canonical ProtocolReceipt v2 encoding, including its unsigned signing form. */
export function encodeSequencerReceipt(receipt: SequencerReceipt, signed = true): Uint8Array {
  const parts: Uint8Array[] = [];
  function integer(value: bigint | number, width: number): void {
    let n = BigInt(value);
    if (n < 0n || n >= 1n << BigInt(width * 8)) throw new TypeError("receipt integer out of range");
    const bytes = new Uint8Array(width);
    for (let i = width - 1; i >= 0; i--) { bytes[i] = Number(n & 255n); n >>= 8n; }
    parts.push(bytes);
  }
  function bytes(value: Uint8Array, exact?: number, maximum = 1048576): void {
    if (exact !== undefined && value.length !== exact || value.length > maximum) throw new TypeError("receipt bytes out of range");
    integer(value.length, 4); parts.push(value);
  }
  if (receipt.protocolVersion !== 2 || receipt.effects.length > 512) throw new TypeError("unsupported receipt");
  integer(2, 2); integer(receipt.totalUnits === undefined ? 0x5201 : 0x5202, 2); integer(receipt.protocolVersion, 2);
  bytes(receipt.activityId, 32); integer(receipt.globalSequence, 8);
  bytes(receipt.previousStateRoot, 32); bytes(receipt.resultingStateRoot, 32); bytes(receipt.activityRoot, 32);
  if (!Number.isInteger(receipt.resultCode) || receipt.resultCode < -2147483648 || receipt.resultCode > 2147483647) throw new TypeError("invalid result code");
  integer(BigInt.asUintN(32, BigInt(receipt.resultCode)), 4); integer(receipt.effects.length, 4);
  for (const effect of receipt.effects) {
    if (![1, 2, 3].includes(effect.kind) || effect.monetary && effect.kind !== 2) throw new TypeError("invalid receipt effect");
    integer(effect.moduleId, 2); integer(effect.ordinal, 2); integer(effect.eventType, 2);
    integer(effect.kind, 1); integer(effect.monetary ? 1 : 0, 1);
    bytes(effect.transferSetRoot, 32); bytes(effect.body, undefined, 256);
  }
  integer(receipt.feeCharged, 16); bytes(receipt.batchId, 32); integer(receipt.moduleId, 2);
  integer(receipt.moduleVersion, 4); integer(receipt.parameterVersion, 4); integer(receipt.operation, 1);
  bytes(receipt.asset, 32); integer(receipt.amount, 16); bytes(receipt.from, 32);
  integer(receipt.fromBalanceBefore, 16); integer(receipt.fromBalanceAfter, 16); integer(receipt.fromSequence, 8);
  bytes(receipt.to, 32); integer(receipt.toBalanceBefore, 16); integer(receipt.toBalanceAfter, 16);
  bytes(receipt.transferSetRoot, 32); bytes(receipt.authorizationHash, 32); bytes(receipt.contextHash, 32); integer(receipt.timestamp, 8);
  if (receipt.totalUnits !== undefined) { integer(receipt.totalUnits[0], 16); integer(receipt.totalUnits[1], 16); }
  integer(signed ? 1 : 0, 1);
  if (signed) bytes(receipt.sequencerSignature, 64);
  const output = new Uint8Array(parts.reduce((length, part) => length + part.length, 0));
  if (output.length > 1048576) throw new TypeError("receipt too large");
  let offset = 0; for (const part of parts) { output.set(part, offset); offset += part.length; }
  return output;
}
