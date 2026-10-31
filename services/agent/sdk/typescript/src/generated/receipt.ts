// Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.

export const RECEIPT_VERSION_V2 = 2;
export const RECEIPT_VERSION_V3 = 3;
export const RECEIPT_DOMAIN = "LXP/v1/receipt";
export const SEQUENCE_DOMAIN = "PAXEER/v1/sequence";

export enum ReceiptFailureCode {
  Decode = "decode",
  CanonicalEncoding = "canonical-encoding",
  ReceiptVersion = "receipt-version",
  ActivityId = "activity-id",
  KeyId = "key-id",
  KeyHeight = "key-height",
  RawTransaction = "raw-transaction",
  Fields = "fields",
  ReturnData = "return-data",
  Logs = "logs",
  SequenceHash = "sequence-hash",
  Signature = "signature",
}

export const PROGRAM_ABI_V1 = 1;
export const PROGRAM_ABI_V2 = 2;
export const PROGRAM_ABI_V3 = 3;
export const PROGRAM_ABI_V4 = 4;
export const PROGRAM_ABI_V5 = 5;
export function supportsProgramGuestAbi(version: number): boolean {
  switch (version) {
    case PROGRAM_ABI_V1: case PROGRAM_ABI_V2: case PROGRAM_ABI_V3: case PROGRAM_ABI_V4: case PROGRAM_ABI_V5: return true;
    default: return false;
  }
}
