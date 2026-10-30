// Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.

public let receiptVersionV2: UInt16 = 2
public let receiptVersionV3: UInt16 = 3
public let receiptDomain = "LXP/v1/receipt"
public let sequenceDomain = "PAXEER/v1/sequence"

public enum ReceiptCheck: String, Sendable, CaseIterable {
  case decode = "decode"
  case canonicalEncoding = "canonical-encoding"
  case receiptVersion = "receipt-version"
  case activityId = "activity-id"
  case keyId = "key-id"
  case keyHeight = "key-height"
  case rawTransaction = "raw-transaction"
  case fields = "fields"
  case returnData = "return-data"
  case logs = "logs"
  case sequenceHash = "sequence-hash"
  case signature = "signature"
}
let programAbiV1: UInt16 = 1
let programAbiV2: UInt16 = 2
let programAbiV3: UInt16 = 3
let programAbiV4: UInt16 = 4
let programAbiV5: UInt16 = 5
func supportsProgramGuestAbi(_ version: UInt16) -> Bool {
  switch version {
  case programAbiV1, programAbiV2, programAbiV3, programAbiV4, programAbiV5: return true
  default: return false
  }
}
