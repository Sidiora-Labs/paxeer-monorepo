import Crypto
import Foundation

private let merkleLeafDomain = Data("LXP/v1/merkle-leaf\0".utf8)
private let merkleInternalDomain = Data("LXP/v1/merkle-internal\0".utf8)
private let wireReceiptDomain = Data("LXP/v1/receipt\0".utf8)
private let maximumMessageBytes = 1_048_576
private let maximumEffects: UInt32 = 512
private let maximumEffectBody: UInt32 = 256
public struct UInt128Value: Hashable, Sendable {
  public let high: UInt64
  public let low: UInt64

  public init(high: UInt64, low: UInt64) {
    self.high = high
    self.low = low
  }

  fileprivate func subtracting(_ other: Self) -> Self? {
    let (low, lowUnderflow) = low.subtractingReportingOverflow(other.low)
    let (highAfterValue, highUnderflow) = high.subtractingReportingOverflow(other.high)
    let (resultHigh, borrowUnderflow) = highAfterValue.subtractingReportingOverflow(
      lowUnderflow ? 1 : 0)
    guard !highUnderflow, !borrowUnderflow else { return nil }
    return Self(high: resultHigh, low: low)
  }

  fileprivate func adding(_ other: Self) -> Self? {
    let (low, lowOverflow) = low.addingReportingOverflow(other.low)
    let (highAfterValue, highOverflow) = high.addingReportingOverflow(other.high)
    let (resultHigh, carryOverflow) = highAfterValue.addingReportingOverflow(lowOverflow ? 1 : 0)
    guard !highOverflow, !carryOverflow else { return nil }
    return Self(high: resultHigh, low: low)
  }
}

public struct MerkleProof: Sendable {
  public let leafIndex: UInt32
  public let leafCount: UInt32
  public let siblings: [Data]

  public init(leafIndex: UInt32, leafCount: UInt32, siblings: [Data]) {
    self.leafIndex = leafIndex
    self.leafCount = leafCount
    self.siblings = siblings
  }
}

public enum VerificationLevel: String, CaseIterable, Sendable {
  case receipted
  case committed
}

public struct ReceiptEffect: Sendable {
  public let moduleID: UInt16
  public let ordinal: UInt16
  public let eventType: UInt16
  public let kind: UInt8
  public let monetary: Bool
  public let transferSetRoot: Data
  public let body: Data
}

public struct ProgramReceiptOutcome: Sendable {
  public let appliedLegsDigest: Data
  public let encodingVersion: UInt8
  public let terminalKind: UInt8
  public let resultCode: Int32
  public let runtimeVersion: UInt16
  public let abiVersion: UInt16
  public let feeScheduleVersion: UInt32
  public let meteringScheduleVersion: UInt32
  public let cpuFuel: UInt64
  public let memoryBytes: UInt64
  public let storageReadBytes: UInt64
  public let storageWriteBytes: UInt64
  public let outputValues: UInt32
  public let outputBytes: UInt64
  public let occupancyByteBatches: UInt128Value
  public let occupancyFeeUnits: UInt128Value
  public let feeSchedulePrices: [UInt64]
  public let occupancyAssetID: Data
  public let occupancyEvidenceDigest: Data
  public let occupancyTransferRoot: Data
  public let feeUnits: UInt128Value
  public let callGraphRoot: Data
  public let terminalPayloadRoot: Data
  public let transferRoot: Data
}

public struct ProtocolReceipt: Sendable {
  public let protocolVersion: UInt16
  public let activityID: Data
  public let globalSequence: UInt64
  public let previousStateRoot: Data
  public let resultingStateRoot: Data
  public let activityRoot: Data
  public let resultCode: Int32
  public let effects: [ReceiptEffect]
  public let feeCharged: UInt128Value
  public let batchID: Data
  public let moduleID: UInt16
  public let moduleVersion: UInt32
  public let parameterVersion: UInt32
  public let operation: UInt8
  public let asset: Data
  public let amount: UInt128Value
  public let from: Data
  public let fromBalanceBefore: UInt128Value
  public let fromBalanceAfter: UInt128Value
  public let fromSequence: UInt64
  public let to: Data
  public let toBalanceBefore: UInt128Value
  public let toBalanceAfter: UInt128Value
  public let transferSetRoot: Data
  public let authorizationHash: Data
  public let contextHash: Data
  public let timestamp: UInt64
  public let programOutcome: ProgramReceiptOutcome?
  public let sequencerSignature: Data
}

public struct ReceiptVerification: Sendable {
  public let level: String
  public let receipt: ProtocolReceipt
  public let canonicalBytes: Data
  public let receiptDigest: Data
}

public enum LocalVerifier {
  public static func verifySequencerReceipt(
    _ receiptBytes: Data, rawTransaction: Data, keys: [ReceiptKeys],
    receiptJSON: [String: JSONValue], prevSeqHash: Data
  ) throws -> SequencerReceiptVerification {
    let decoded = try decodeProtocolReceipt(receiptBytes, sequencerOnly: true)
    let r = decoded.receipt
    guard r.protocolVersion == receiptVersionV2 || r.protocolVersion == receiptVersionV3 else {
      throw receiptFailure(.receiptVersion)
    }
    guard r.programOutcome == nil,
      encodeSequencerReceipt(r, signed: true) == receiptBytes
    else {
      throw receiptFailure(.canonicalEncoding)
    }
    guard !allZero(r.activityID) else { throw receiptFailure(.activityId) }
    let matching = keys.filter { $0.keyId.count == 32 && $0.keyId == r.contextHash }
    guard !matching.isEmpty else { throw receiptFailure(.keyId) }
    guard allZero(Data(r.batchID.prefix(24))) else { throw receiptFailure(.keyHeight) }
    let height = r.batchID.suffix(8).reduce(UInt64(0)) { ($0 << 8) | UInt64($1) }
    let active = matching.filter { $0.firstHeight <= height && height <= $0.lastHeight }
    guard !active.isEmpty else { throw receiptFailure(.keyHeight) }
    let receiptDigest = digest(wireReceiptDomain, encodeSequencerReceipt(r, signed: false))
    guard
      active.contains(where: {
        verifyEd25519(
          publicKey: $0.publicKey, signature: r.sequencerSignature, message: receiptDigest)
      })
    else {
      throw receiptFailure(.signature)
    }
    guard !rawTransaction.isEmpty, sequencerKeccak256(rawTransaction) == r.activityID else {
      throw receiptFailure(.rawTransaction)
    }
    let returnData: Data
    do {
      returnData = try verifySequencerJSON(
        receiptJSON, receipt: r, bytes: receiptBytes, height: height)
    } catch let error as PlatformSDKError where error.receiptCheck != nil {
      throw error
    } catch {
      throw receiptFailure(.fields)
    }
    guard prevSeqHash.count == 32, r.previousStateRoot == prevSeqHash,
      r.resultingStateRoot
        == digest(
          Data((sequenceDomain + "\0").utf8), prevSeqHash,
          encodeUInt64(r.globalSequence), r.activityID)
    else {
      throw receiptFailure(.sequenceHash)
    }
    return SequencerReceiptVerification(
      level: VerificationLevel.receipted.rawValue, status: r.resultCode == 0 ? 1 : 0,
      revert: r.resultCode == 0 ? nil : decodeSequencerRevert(returnData), receipt: r,
      canonicalBytes: receiptBytes, receiptDigest: receiptDigest)
  }

  public static func decodeProgramReceiptOutcome(
    _ canonicalOutcome: Data,
    protocolVersion: UInt16
  ) throws -> ProgramReceiptOutcome {
    guard !canonicalOutcome.isEmpty, canonicalOutcome.count <= maximumMessageBytes else {
      throw receiptFailure(.receiptShape)
    }
    do {
      var decoder = WireDecoder(canonicalOutcome)
      let outcome = try decodeProgramReceiptOutcomeFrom(&decoder, protocolVersion: protocolVersion)
      try decoder.finish()
      return outcome
    } catch let error as PlatformSDKError where error.receiptCheck != nil {
      throw error
    } catch {
      throw receiptFailure(.programOutcome)
    }
  }

  public static func verifyMerkleInclusion(
    canonicalLeaf: Data, proof: MerkleProof, expectedRoot: Data
  ) throws {
    guard proof.leafCount > 0, proof.leafIndex < proof.leafCount,
      proof.siblings.count <= 32, proof.siblings.count == proofDepth(proof.leafCount),
      expectedRoot.count == 32
    else { throw verificationFailure() }
    var current = digest(merkleLeafDomain, canonicalLeaf)
    var index = proof.leafIndex
    var count = proof.leafCount
    for untrustedSibling in proof.siblings {
      let sibling = try exact(untrustedSibling, 32)
      if (index ^ 1) >= count, sibling != current { throw verificationFailure() }
      current =
        index % 2 == 0
        ? digest(merkleInternalDomain, current, sibling)
        : digest(merkleInternalDomain, sibling, current)
      index /= 2
      count = count / 2 + count % 2
    }
    guard current == expectedRoot else { throw verificationFailure() }
  }

  public static func verifyProgramLifecycleReceipt(
    _ canonical: Data, expectedActivity: Data, sequencer: Data
  ) throws -> ReceiptVerification {
    let decoded = try decodeProtocolReceipt(canonical, sequencerOnly: false)
    let receipt = decoded.receipt
    guard receipt.protocolVersion == 3, receipt.moduleID == 9, receipt.moduleVersion == 4,
      receipt.operation == 0,
      receipt.programOutcome == nil, expectedActivity.count == 32, !allZero(expectedActivity),
      receipt.activityID == expectedActivity,
      sequencer.count == 32, !allZero(sequencer)
    else { throw receiptFailure(.receiptShape) }
    let receiptDigest = digest(wireReceiptDomain, decoded.unsignedBytes)
    guard
      verifyEd25519(
        publicKey: sequencer, signature: receipt.sequencerSignature, message: receiptDigest)
    else { throw receiptFailure(.sequencerSignature) }
    return ReceiptVerification(
      level: "sequencer-signed", receipt: receipt, canonicalBytes: canonical,
      receiptDigest: receiptDigest)
  }

}

private struct DecodedReceipt {
  let receipt: ProtocolReceipt
  let unsignedBytes: Data
}

private func decodeProtocolReceipt(_ canonicalReceipt: Data, sequencerOnly: Bool) throws
  -> DecodedReceipt
{
  do {
    return try decodeProtocolReceiptInner(canonicalReceipt, sequencerOnly: sequencerOnly)
  } catch let error as PlatformSDKError where error.receiptCheck != nil {
    throw error
  } catch {
    throw receiptFailure(.decode)
  }
}

private func decodeProtocolReceiptInner(_ canonicalReceipt: Data, sequencerOnly: Bool) throws
  -> DecodedReceipt
{
  guard !canonicalReceipt.isEmpty, canonicalReceipt.count <= maximumMessageBytes else {
    throw receiptFailure(.receiptShape)
  }
  var decoder = WireDecoder(canonicalReceipt)
  let envelopeVersion = try decoder.u16()
  guard envelopeVersion == 1 || envelopeVersion == 2 || envelopeVersion == 3,
    try decoder.u16() == 0x5201
  else {
    throw receiptFailure(.decode)
  }
  let protocolVersion = try decoder.u16()
  guard protocolVersion == envelopeVersion else { throw receiptFailure(.protocolVersion) }
  let activityID = try decoder.array32()
  let globalSequence = try decoder.u64()
  let previousStateRoot = try decoder.array32()
  let resultingStateRoot = try decoder.array32()
  let activityRoot = try decoder.array32()
  let resultCode = try decoder.i32()
  let effectCount = try decoder.u32()
  guard effectCount <= maximumEffects else { throw verificationFailure() }
  var effects: [ReceiptEffect] = []
  effects.reserveCapacity(Int(effectCount))
  for _ in 0..<effectCount {
    let moduleID = try decoder.u16()
    let ordinal = try decoder.u16()
    let eventType = try decoder.u16()
    let kind = try decoder.u8()
    let monetaryValue = try decoder.u8()
    guard (1...3).contains(kind), monetaryValue <= 1, monetaryValue == 0 || kind == 2 else {
      throw verificationFailure()
    }
    effects.append(
      ReceiptEffect(
        moduleID: moduleID, ordinal: ordinal, eventType: eventType, kind: kind,
        monetary: monetaryValue == 1, transferSetRoot: try decoder.array32(),
        body: try decoder.bounded(maximum: maximumEffectBody)))
  }
  let feeCharged = try decoder.u128()
  let batchID = try decoder.array32()
  let moduleID = try decoder.u16()
  guard !sequencerOnly || moduleID > 11 else { throw receiptFailure(.decode) }
  let moduleVersion = try decoder.u32()
  let parameterVersion = try decoder.u32()
  let operation = try decoder.u8()
  let asset = try decoder.array32()
  let amount = try decoder.u128()
  let from = try decoder.array32()
  let fromBalanceBefore = try decoder.u128()
  let fromBalanceAfter = try decoder.u128()
  let fromSequence = try decoder.u64()
  let to = try decoder.array32()
  let toBalanceBefore = try decoder.u128()
  let toBalanceAfter = try decoder.u128()
  let transferSetRoot = try decoder.array32()
  let authorizationHash = try decoder.array32()
  let contextHash = try decoder.array32()
  let timestamp = try decoder.u64()
  guard globalSequence != 0 else { throw receiptFailure(.globalSequence) }
  guard moduleID != 0 else { throw receiptFailure(.moduleId) }
  guard moduleVersion != 0 else { throw receiptFailure(.moduleVersion) }
  guard timestamp != 0 else { throw receiptFailure(.timestamp) }
  guard !allZero(activityID) else { throw receiptFailure(.activityId) }
  guard !allZero(resultingStateRoot) else { throw receiptFailure(.resultingStateRoot) }
  let programOutcome: ProgramReceiptOutcome?
  do {
    programOutcome =
      decoder.remaining > 69
      ? try decodeProgramReceiptOutcomeFrom(&decoder, protocolVersion: protocolVersion) : nil
  } catch {
    throw receiptFailure(.programOutcome)
  }
  if let outcome = programOutcome {
    guard moduleID == programsModuleID, outcome.resultCode == resultCode,
      outcome.terminalKind == 1 ? outcome.transferRoot == transferSetRoot : allZero(transferSetRoot)
    else { throw receiptFailure(.programOutcome) }
  }
  let signatureFlagOffset = decoder.position
  guard try decoder.u8() == 1 else { throw receiptFailure(.missingSignature) }
  let sequencerSignature = try decoder.bounded(exactly: 64)
  try decoder.finish()
  let receipt = ProtocolReceipt(
    protocolVersion: protocolVersion, activityID: activityID, globalSequence: globalSequence,
    previousStateRoot: previousStateRoot, resultingStateRoot: resultingStateRoot,
    activityRoot: activityRoot, resultCode: resultCode, effects: effects, feeCharged: feeCharged,
    batchID: batchID, moduleID: moduleID, moduleVersion: moduleVersion,
    parameterVersion: parameterVersion, operation: operation, asset: asset, amount: amount,
    from: from, fromBalanceBefore: fromBalanceBefore, fromBalanceAfter: fromBalanceAfter,
    fromSequence: fromSequence, to: to, toBalanceBefore: toBalanceBefore,
    toBalanceAfter: toBalanceAfter, transferSetRoot: transferSetRoot,
    authorizationHash: authorizationHash, contextHash: contextHash, timestamp: timestamp,
    programOutcome: programOutcome, sequencerSignature: sequencerSignature)
  var unsigned = canonicalReceipt.prefix(signatureFlagOffset)
  unsigned.append(0)
  return DecodedReceipt(receipt: receipt, unsignedBytes: Data(unsigned))
}

private func decodeProgramReceiptOutcomeFrom(
  _ decoder: inout WireDecoder,
  protocolVersion: UInt16
) throws -> ProgramReceiptOutcome {
  let tag = try decoder.u32()
  let encodingVersion: UInt8
  switch tag {
  case programOutcomeV1: encodingVersion = 1
  case programOutcomeV2: encodingVersion = 2
  case programOutcomeV3: encodingVersion = 3
  case programOutcomeV4: encodingVersion = 4
  default: throw verificationFailure()
  }
  let terminalKind = try decoder.u8()
  let resultCode = try decoder.i32()
  let runtimeVersion = try decoder.u16()
  let abiVersion = try decoder.u16()
  let feeScheduleVersion = try decoder.u32()
  let meteringScheduleVersion: UInt32 = encodingVersion >= 3 ? (try decoder.u32()) : 1
  let cpuFuel = try decoder.u64()
  let memoryBytes = try decoder.u64()
  let storageReadBytes = try decoder.u64()
  let storageWriteBytes = try decoder.u64()
  let outputValues = try decoder.u32()
  let outputBytes = try decoder.u64()
  let zero128 = UInt128Value(high: 0, low: 0)
  let occupancyByteBatches = encodingVersion >= 2 ? (try decoder.u128()) : zero128
  let occupancyFeeUnits = encodingVersion >= 2 ? (try decoder.u128()) : zero128
  var feeSchedulePrices = Array(repeating: UInt64(0), count: 7)
  if encodingVersion >= 2 {
    for index in feeSchedulePrices.indices { feeSchedulePrices[index] = try decoder.u64() }
  }
  let occupancyAssetID =
    encodingVersion >= 2 ? (try decoder.array32()) : Data(repeating: 0, count: 32)
  let occupancyEvidenceDigest =
    encodingVersion >= 2 ? (try decoder.array32()) : Data(repeating: 0, count: 32)
  let occupancyTransferRoot =
    encodingVersion >= 2 ? (try decoder.array32()) : Data(repeating: 0, count: 32)
  let feeUnits = try decoder.u128()
  let callGraphRoot = try decoder.array32()
  let terminalPayloadRoot = try decoder.array32()
  let transferRoot = try decoder.array32()
  let appliedLegsDigest =
    encodingVersion == 4 ? (try decoder.array32()) : Data(repeating: 0, count: 32)
  let occupancyZero =
    occupancyByteBatches == zero128 && occupancyFeeUnits == zero128
    && allZero(occupancyAssetID) && allZero(occupancyEvidenceDigest)
    && allZero(occupancyTransferRoot)
  let validVersion =
    protocolVersion == 1 && (encodingVersion == 1 || encodingVersion == 3)
    || (protocolVersion == 2 || protocolVersion == 3)
      && (encodingVersion == 2 || encodingVersion == 3)
    || protocolVersion == 3 && encodingVersion == 4
  guard (encodingVersion == 4) != allZero(appliedLegsDigest), (1...3).contains(terminalKind),
    runtimeVersion != 0, abiVersion != 0,
    feeScheduleVersion != 0, meteringScheduleVersion == 1, !allZero(terminalPayloadRoot),
    terminalKind != 1 || resultCode == 0,
    terminalKind == 1 || (resultCode != 0 && resultCode > -1000),
    terminalKind == 1 || allZero(transferRoot), validVersion,
    encodingVersion != 1 || occupancyZero,
    encodingVersion < 2 || terminalKind == 1 || occupancyZero,
    encodingVersion != 2 || terminalKind != 1
      || (!allZero(occupancyAssetID) && !allZero(occupancyEvidenceDigest)),
    encodingVersion < 3 || allZero(occupancyAssetID) == allZero(occupancyEvidenceDigest),
    protocolVersion != 1 || encodingVersion < 3 || occupancyZero,
    (protocolVersion != 2 && protocolVersion != 3) || encodingVersion < 3 || terminalKind != 1
      || (!allZero(occupancyAssetID) && !allZero(occupancyEvidenceDigest))
  else { throw verificationFailure() }
  return ProgramReceiptOutcome(
    appliedLegsDigest: appliedLegsDigest, encodingVersion: encodingVersion,
    terminalKind: terminalKind,
    resultCode: resultCode, runtimeVersion: runtimeVersion, abiVersion: abiVersion,
    feeScheduleVersion: feeScheduleVersion, meteringScheduleVersion: meteringScheduleVersion,
    cpuFuel: cpuFuel, memoryBytes: memoryBytes, storageReadBytes: storageReadBytes,
    storageWriteBytes: storageWriteBytes, outputValues: outputValues, outputBytes: outputBytes,
    occupancyByteBatches: occupancyByteBatches, occupancyFeeUnits: occupancyFeeUnits,
    feeSchedulePrices: feeSchedulePrices, occupancyAssetID: occupancyAssetID,
    occupancyEvidenceDigest: occupancyEvidenceDigest, occupancyTransferRoot: occupancyTransferRoot,
    feeUnits: feeUnits, callGraphRoot: callGraphRoot, terminalPayloadRoot: terminalPayloadRoot,
    transferRoot: transferRoot)
}

private struct WireDecoder {
  private let bytes: [UInt8]
  private(set) var position = 0
  var remaining: Int { bytes.count - position }

  init(_ data: Data) { bytes = Array(data) }

  mutating func fixed(_ count: Int) throws -> Data {
    guard count >= 0, position <= bytes.count - count else { throw verificationFailure() }
    let value = Data(bytes[position..<(position + count)])
    position += count
    return value
  }

  mutating func u8() throws -> UInt8 { try fixed(1)[0] }
  mutating func u16() throws -> UInt16 { try fixed(2).reduce(0) { ($0 << 8) | UInt16($1) } }
  mutating func u32() throws -> UInt32 { try fixed(4).reduce(0) { ($0 << 8) | UInt32($1) } }
  mutating func i32() throws -> Int32 { Int32(bitPattern: try u32()) }
  mutating func u64() throws -> UInt64 { try fixed(8).reduce(0) { ($0 << 8) | UInt64($1) } }
  mutating func u128() throws -> UInt128Value { UInt128Value(high: try u64(), low: try u64()) }

  mutating func bounded(maximum: UInt32) throws -> Data {
    let length = try u32()
    guard length <= maximum, UInt64(length) <= UInt64(Int.max) else { throw verificationFailure() }
    return try fixed(Int(length))
  }

  mutating func bounded(exactly length: UInt32) throws -> Data {
    let value = try bounded(maximum: length)
    guard value.count == Int(length) else { throw verificationFailure() }
    return value
  }

  mutating func array32() throws -> Data { try bounded(exactly: 32) }
  func finish() throws { guard position == bytes.count else { throw verificationFailure() } }
}

private func verificationFailure() -> PlatformSDKError {
  PlatformSDKError(code: .verificationFailure, retry: .never)
}

private func receiptFailure(_ check: ReceiptCheck) -> PlatformSDKError {
  PlatformSDKError(code: .verificationFailure, retry: .never, receiptCheck: check)
}

private func exact(_ value: Data, _ length: Int) throws -> Data {
  guard value.count == length else { throw verificationFailure() }
  return value
}

private func allZero(_ value: Data) -> Bool {
  value.reduce(UInt8(0)) { $0 | $1 } == 0
}

private func concatenate(_ values: Data...) throws -> Data {
  var result = Data()
  for value in values { result.append(value) }
  return result
}

private func digest(_ values: Data...) -> Data {
  var hasher = SHA256()
  for value in values { hasher.update(data: value) }
  return Data(hasher.finalize())
}

private func verifyEd25519(publicKey: Data, signature: Data, message: Data) -> Bool {
  guard publicKey.count == 32, signature.count == 64, message.count == 32,
    let key = try? Curve25519.Signing.PublicKey(rawRepresentation: publicKey)
  else { return false }
  return key.isValidSignature(signature, for: message)
}

private func proofDepth(_ count: UInt32) -> Int {
  var count = count
  var depth = 0
  while count > 1 {
    count = count / 2 + count % 2
    depth += 1
  }
  return depth
}

private func encodeUInt32(_ value: UInt32) -> Data {
  Data([
    UInt8(truncatingIfNeeded: value >> 24), UInt8(truncatingIfNeeded: value >> 16),
    UInt8(truncatingIfNeeded: value >> 8), UInt8(truncatingIfNeeded: value),
  ])
}

private func encodeUInt16(_ value: UInt16) -> Data {
  Data([UInt8(truncatingIfNeeded: value >> 8), UInt8(truncatingIfNeeded: value)])
}

private func encodeUInt64(_ value: UInt64) -> Data {
  Data((0..<8).map { UInt8(truncatingIfNeeded: value >> UInt64(56 - 8 * $0)) })
}

// Compatibility names for the kernel receipt entry points retained during migration.
extension ReceiptCheck {
  public static let receiptShape: ReceiptCheck = .fields
  public static let missingSignature: ReceiptCheck = .canonicalEncoding
  public static let protocolVersion: ReceiptCheck = .receiptVersion
  public static let resultCode: ReceiptCheck = .fields
  public static let operation: ReceiptCheck = .fields
  public static let globalSequence: ReceiptCheck = .fields
  public static let moduleId: ReceiptCheck = .fields
  public static let moduleVersion: ReceiptCheck = .fields
  public static let timestamp: ReceiptCheck = .fields
  public static let batchId: ReceiptCheck = .fields
  public static let asset: ReceiptCheck = .fields
  public static let previousStateRoot: ReceiptCheck = .fields
  public static let resultingStateRoot: ReceiptCheck = .fields
  public static let debitBalance: ReceiptCheck = .fields
  public static let creditBalance: ReceiptCheck = .fields
  public static let programOutcome: ReceiptCheck = .fields
  public static let sequencerSignature: ReceiptCheck = .signature
}

private let programsModuleID: UInt16 = 9
private let programOutcomeV1: UInt32 = 0x5052_4731
private let programOutcomeV2: UInt32 = 0x5052_4732
private let programOutcomeV3: UInt32 = 0x5052_4733
private let programOutcomeV4: UInt32 = 0x5052_4734

public struct ReceiptKeys: Sendable {
  public let keyId: Data
  public let publicKey: Data
  public let firstHeight: UInt64
  public let lastHeight: UInt64

  public init(keyId: Data, publicKey: Data, firstHeight: UInt64, lastHeight: UInt64) {
    self.keyId = keyId
    self.publicKey = publicKey
    self.firstHeight = firstHeight
    self.lastHeight = lastHeight
  }
}

public struct SequencerRevert: Error, Sendable, Equatable {
  public let reason: String
}

public struct SequencerReceiptVerification: Sendable {
  public let level: String
  public let status: UInt8
  public let revert: SequencerRevert?
  public let receipt: ProtocolReceipt
  public let canonicalBytes: Data
  public let receiptDigest: Data
}

private func encodeSequencerReceipt(_ r: ProtocolReceipt, signed: Bool) -> Data {
  var b = Data()
  func u16(_ v: UInt16) { b.append(encodeUInt16(v)) }
  func u32(_ v: UInt32) { b.append(encodeUInt32(v)) }
  func u64(_ v: UInt64) { b.append(encodeUInt64(v)) }
  func u128(_ v: UInt128Value) {
    u64(v.high)
    u64(v.low)
  }
  func blob(_ v: Data) {
    u32(UInt32(v.count))
    b.append(v)
  }
  u16(r.protocolVersion)
  u16(0x5201)
  u16(r.protocolVersion)
  blob(r.activityID)
  u64(r.globalSequence)
  blob(r.previousStateRoot)
  blob(r.resultingStateRoot)
  blob(r.activityRoot)
  u32(UInt32(bitPattern: r.resultCode))
  u32(UInt32(r.effects.count))
  for e in r.effects {
    u16(e.moduleID)
    u16(e.ordinal)
    u16(e.eventType)
    b.append(e.kind)
    b.append(e.monetary ? 1 : 0)
    blob(e.transferSetRoot)
    blob(e.body)
  }
  u128(r.feeCharged)
  blob(r.batchID)
  u16(r.moduleID)
  u32(r.moduleVersion)
  u32(r.parameterVersion)
  b.append(r.operation)
  blob(r.asset)
  u128(r.amount)
  blob(r.from)
  u128(r.fromBalanceBefore)
  u128(r.fromBalanceAfter)
  u64(r.fromSequence)
  blob(r.to)
  u128(r.toBalanceBefore)
  u128(r.toBalanceAfter)
  blob(r.transferSetRoot)
  blob(r.authorizationHash)
  blob(r.contextHash)
  u64(r.timestamp)
  b.append(signed ? 1 : 0)
  if signed { blob(r.sequencerSignature) }
  return b
}

private func sequencerHex(_ value: JSONValue?, check: ReceiptCheck, length: Int? = nil) throws
  -> Data
{
  guard let s = value?.stringValue, s.hasPrefix("0x") else {
    throw receiptFailure(check)
  }
  let chars = Array(s.dropFirst(2).utf8)
  guard chars.count % 2 == 0 else { throw receiptFailure(check) }
  var data = Data()
  func nibble(_ c: UInt8) -> UInt8? {
    if c >= 48 && c <= 57 { return c - 48 }
    if c >= 97 && c <= 102 { return c - 87 }
    return nil
  }
  for i in stride(from: 0, to: chars.count, by: 2) {
    guard let a = nibble(chars[i]), let b = nibble(chars[i + 1]) else {
      throw receiptFailure(check)
    }
    data.append(a * 16 + b)
  }
  guard length == nil || data.count == length else { throw receiptFailure(check) }
  return data
}

private func sequencerQuantity(_ value: JSONValue?, check: ReceiptCheck = .fields) throws -> UInt64
{
  guard let s = value?.stringValue, s.hasPrefix("0x"),
    s == "0x0" || (s.count > 2 && s.dropFirst(2).first != "0"),
    s.dropFirst(2).utf8.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) }),
    let n = UInt64(s.dropFirst(2), radix: 16)
  else { throw receiptFailure(check) }
  return n
}

private func verifySequencerJSON(
  _ j: [String: JSONValue], receipt r: ProtocolReceipt,
  bytes: Data, height: UInt64
) throws -> Data {
  guard r.moduleID > 11, r.moduleVersion == 1, r.parameterVersion == 0,
    (0...1).contains(r.resultCode), !r.effects.isEmpty, allZero(r.transferSetRoot),
    j["rung"] == .string("receipted"),
    try sequencerHex(j["receipt"], check: .fields) == bytes,
    try sequencerHex(j["txHash"], check: .fields, length: 32) == r.activityID,
    try sequencerHex(j["keyId"], check: .fields, length: 32) == r.contextHash,
    try sequencerQuantity(j["seq"]) == r.globalSequence,
    try sequencerQuantity(j["nonce"]) == r.fromSequence,
    try sequencerQuantity(j["height"]) == height,
    try sequencerQuantity(j["timestamp"]) == r.timestamp,
    try sequencerQuantity(j["status"]) == (r.resultCode == 0 ? 1 : 0),
    allZero(Data(r.from.prefix(12))), allZero(Data(r.to.prefix(12))),
    try sequencerHex(j["from"], check: .fields, length: 20) == Data(r.from.suffix(20)),
    try sequencerHex(j["to"], check: .fields, length: 20) == Data(r.to.suffix(20))
  else {
    throw receiptFailure(.fields)
  }
  for (i, e) in r.effects.enumerated() {
    guard e.moduleID == r.moduleID, Int(e.ordinal) == i, e.eventType == 0,
      e.kind == (i == 0 ? 1 : 3), !e.monetary, allZero(e.transferSetRoot)
    else {
      throw receiptFailure(.fields)
    }
  }
  var summary = WireDecoder(r.effects[0].body)
  guard try summary.u64() == sequencerQuantity(j["gasUsed"]),
    try summary.u64() == sequencerQuantity(j["cumulativeGasUsed"]),
    try summary.u64() == sequencerQuantity(j["effectiveGasPrice"]),
    try UInt64(summary.u32()) == sequencerQuantity(j["index"]),
    try summary.fixed(20) == sequencerHex(j["contractAddress"], check: .fields, length: 20),
    try summary.fixed(20) == sequencerHex(j["sponsor"], check: .fields, length: 20),
    let vmError = j["vmError"]?.stringValue,
    try summary.fixed(32) == digest(Data(vmError.utf8))
  else { throw receiptFailure(.fields) }
  let returnLength = try summary.u32()
  let logCount = try summary.u32()
  let denomLength = try summary.u8()
  guard (1...64).contains(denomLength), let denom = j["feeDenom"]?.stringValue,
    try summary.fixed(Int(denomLength)) == Data(denom.utf8),
    digest(Data(denom.utf8)) == r.asset
  else { throw receiptFailure(.fields) }
  try summary.finish()
  guard let fee = j["feeAmount"]?.stringValue, fee.hasPrefix("0x"), fee.count > 2, fee.count <= 34
  else {
    throw receiptFailure(.fields)
  }
  let digits = String(fee.dropFirst(2))
  guard fee == "0x0" || digits.first != "0",
    digits.utf8.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) })
  else { throw receiptFailure(.fields) }
  let lowDigits = String(digits.suffix(16))
  let highDigits = String(digits.dropLast(min(16, digits.count)))
  guard let low = UInt64(lowDigits, radix: 16),
    let high = UInt64(highDigits.isEmpty ? "0" : highDigits, radix: 16),
    r.feeCharged == UInt128Value(high: high, low: low)
  else { throw receiptFailure(.fields) }
  let returnData = try sequencerHex(j["returnData"], check: .returnData)
  guard returnData.count == Int(returnLength), digest(returnData) == r.activityRoot else {
    throw receiptFailure(.returnData)
  }
  guard case let .array(logs) = j["logs"], logs.count == Int(logCount),
    r.effects.count == logs.count + 1
  else { throw receiptFailure(.logs) }
  for (i, value) in logs.enumerated() {
    guard let log = value.objectValue, case let .array(topics) = log["topics"], topics.count <= 4
    else {
      throw receiptFailure(.logs)
    }
    var body = try sequencerHex(log["address"], check: .logs, length: 20)
    body.append(UInt8(topics.count))
    for topic in topics { body.append(try sequencerHex(topic, check: .logs, length: 32)) }
    body.append(digest(try sequencerHex(log["data"], check: .logs)))
    guard body == r.effects[i + 1].body,
      try sequencerQuantity(log["logIndex"], check: .logs) == UInt64(i)
    else { throw receiptFailure(.logs) }
  }
  return returnData
}

private func decodeSequencerRevert(_ data: Data) -> SequencerRevert? {
  let b = [UInt8](data)
  guard b.count >= 68, Array(b.prefix(4)) == [8, 195, 121, 160],
    b[4..<35].allSatisfy({ $0 == 0 }), b[35] == 32,
    b[36..<60].allSatisfy({ $0 == 0 })
  else { return nil }
  let length = b[60..<68].reduce(UInt64(0)) { ($0 << 8) | UInt64($1) }
  guard length <= UInt64(b.count - 68),
    let reason = String(bytes: b[68..<(68 + Int(length))], encoding: .utf8),
    reason.hasPrefix("LXP_")
  else { return nil }
  return SequencerRevert(reason: reason)
}

private func sequencerKeccak256(_ data: Data) -> Data {
  let rc: [UInt64] = [
    0x1, 0x8082, 0x8000_0000_0000_808a, 0x8000_0000_8000_8000, 0x808b, 0x8000_0001,
    0x8000_0000_8000_8081, 0x8000_0000_0000_8009, 0x8a, 0x88, 0x8000_8009, 0x8000_000a, 0x8000_808b,
    0x8000_0000_0000_008b, 0x8000_0000_0000_8089, 0x8000_0000_0000_8003, 0x8000_0000_0000_8002,
    0x8000_0000_0000_0080, 0x800a, 0x8000_0000_8000_000a, 0x8000_0000_8000_8081,
    0x8000_0000_0000_8080,
    0x8000_0001, 0x8000_0000_8000_8008,
  ]
  let shifts = [
    0, 1, 62, 28, 27, 36, 44, 6, 55, 20, 3, 10, 43, 25, 39, 41, 45, 15, 21, 8, 18, 2, 61, 56, 14,
  ]
  func rotate(_ x: UInt64, _ n: Int) -> UInt64 { n == 0 ? x : (x << n) | (x >> (64 - n)) }
  var state = [UInt64](repeating: 0, count: 25)
  var bytes = [UInt8](data)
  let padding = 136 - bytes.count % 136
  bytes.append(contentsOf: repeatElement(0, count: padding))
  bytes[data.count] = 1
  bytes[bytes.count - 1] |= 128
  for block in stride(from: 0, to: bytes.count, by: 136) {
    for i in 0..<136 { state[i / 8] ^= UInt64(bytes[block + i]) << ((i % 8) * 8) }
    for round in rc {
      var c = [UInt64](repeating: 0, count: 5)
      for x in 0..<5 { for y in 0..<5 { c[x] ^= state[x + 5 * y] } }
      for x in 0..<5 {
        let d = c[(x + 4) % 5] ^ rotate(c[(x + 1) % 5], 1)
        for y in 0..<5 { state[x + 5 * y] ^= d }
      }
      var b = [UInt64](repeating: 0, count: 25)
      for x in 0..<5 {
        for y in 0..<5 {
          b[y + 5 * ((2 * x + 3 * y) % 5)] = rotate(state[x + 5 * y], shifts[x + 5 * y])
        }
      }
      for x in 0..<5 {
        for y in 0..<5 {
          state[x + 5 * y] = b[x + 5 * y] ^ ((~b[(x + 1) % 5 + 5 * y]) & b[(x + 2) % 5 + 5 * y])
        }
      }
      state[0] ^= round
    }
  }
  return Data((0..<32).map { UInt8(truncatingIfNeeded: state[$0 / 8] >> (($0 % 8) * 8)) })
}
