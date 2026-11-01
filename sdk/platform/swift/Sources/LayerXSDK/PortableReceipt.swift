import Foundation

public struct PortableReceipt: Sendable {
  public static let format = "layerx-receipt-proof-v2"
  public let receiptVerification: SequencerReceiptVerification
  public let committed: Committed?
  private let bytes: Data

  public var verificationLevel: String { committed?.level ?? receiptVerification.level }
  public var status: UInt8 { receiptVerification.status }
  public var receiptDigest: Data { receiptVerification.receiptDigest }

  public func export() -> Data { bytes }

  public static func export(
    rawTransaction: Data, signedReceipt: Data, receiptJSON: [String: JSONValue],
    receiptKey: ReceiptKeys, prevSeqHash: Data? = nil,
    blockHeader: CommittedHeader? = nil, receiptInclusion: ReceiptInclusion? = nil,
    commit: CometCommit? = nil, validatorSet: [CometValidator]? = nil
  ) throws -> PortableReceipt {
    let supplied = [
      blockHeader != nil, receiptInclusion != nil, commit != nil, validatorSet != nil,
    ]
    guard supplied.allSatisfy({ $0 }) || supplied.allSatisfy({ !$0 }) else {
      throw portableFailure()
    }
    let receipt = try verifyReceipt(
      signedReceipt, rawTransaction: rawTransaction, receiptJSON: receiptJSON,
      keys: [receiptKey], prevSeqHash: prevSeqHash)
    var document = PortableDocument(
      format: format, verificationLevel: supplied[0] ? "committed" : "receipted",
      rawTransaction: portableHex(rawTransaction), signedReceipt: portableHex(signedReceipt),
      receiptDigest: portableHex(receipt.receiptDigest), receiptKey: PortableKey(receiptKey),
      receiptJson: receiptJSON)
    if let blockHeader, let receiptInclusion, let commit, let validatorSet {
      document.blockHeader = PortableHeader(blockHeader)
      document.receiptInclusion = PortableInclusion(receiptInclusion)
      document.commit = PortableCommit(commit)
      document.validatorSet = validatorSet.map(PortableValidator.init)
    }
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
    return try verify(encoder.encode(document), keys: [receiptKey], prevSeqHash: prevSeqHash)
  }

  public static func verify(
    _ bundle: Data, keys: [ReceiptKeys], prevSeqHash: Data? = nil
  ) throws -> PortableReceipt {
    guard !bundle.isEmpty, bundle.count <= 16 * 1_048_576 else { throw portableFailure() }
    var scanner = PortableDocumentScanner(bundle)
    try scanner.validate()
    guard let root = try JSONSerialization.jsonObject(with: bundle) as? [String: Any],
      let level = root["verificationLevel"] as? String,
      level == "receipted" || level == "committed"
    else { throw portableFailure() }
    var fields: Set<String> = [
      "format", "verificationLevel", "rawTransaction", "signedReceipt", "receiptDigest",
      "receiptKey", "receiptJson",
    ]
    if level == "committed" {
      fields.formUnion(["blockHeader", "receiptInclusion", "commit", "validatorSet"])
    }
    guard Set(root.keys) == fields,
      let key = root["receiptKey"] as? [String: Any],
      Set(key.keys) == ["keyId", "publicKey", "firstHeight", "lastHeight"]
    else { throw portableFailure() }
    let decoder = JSONDecoder()
    decoder.dataDecodingStrategy = .custom { decoder in
      let container = try decoder.singleValueContainer()
      if container.decodeNil() { return Data() }
      let text = try container.decode(String.self)
      guard let bytes = Data(base64Encoded: text), bytes.count <= 1_048_576,
        bytes.base64EncodedString() == text
      else { throw portableFailure() }
      return bytes
    }
    let document = try decoder.decode(PortableDocument.self, from: bundle)
    guard document.format == format else { throw portableFailure() }
    let carried = try document.receiptKey.decoded()
    let pinned = keys.filter {
      $0.keyId == carried.keyId && $0.publicKey == carried.publicKey
        && $0.firstHeight == carried.firstHeight && $0.lastHeight == carried.lastHeight
    }
    guard !pinned.isEmpty else { throw portableFailure() }
    let raw = try portableBytes(document.rawTransaction)
    let signed = try portableBytes(document.signedReceipt)
    let receipt = try verifyReceipt(
      signed, rawTransaction: raw, receiptJSON: document.receiptJson,
      keys: pinned, prevSeqHash: prevSeqHash)
    guard try portableBytes(document.receiptDigest, count: 32) == receipt.receiptDigest else {
      throw portableFailure()
    }
    let committed: Committed?
    if level == "committed" {
      guard let header = document.blockHeader, let inclusion = document.receiptInclusion,
        let commit = document.commit, let validators = document.validatorSet, !validators.isEmpty,
        validators.count <= 10_000
      else { throw portableFailure() }
      let proof = try inclusion.decoded()
      guard try CommittedWire.evmTransaction(proof.transaction) == raw else {
        throw portableFailure()
      }
      committed = try Committed.verify(
        receipt, blockHeader: header.decoded(), receiptInclusion: proof,
        commit: commit.decoded(), validatorSet: validators.map { try $0.decoded() })
    } else {
      committed = nil
    }
    return PortableReceipt(receiptVerification: receipt, committed: committed, bytes: bundle)
  }

  private static func verifyReceipt(
    _ signed: Data, rawTransaction: Data, receiptJSON: [String: JSONValue],
    keys: [ReceiptKeys], prevSeqHash: Data?
  ) throws -> SequencerReceiptVerification {
    let fields: Set<String> = [
      "rung", "txHash", "nonce", "seq", "height", "index", "timestamp", "status", "returnData",
      "logs", "gasUsed", "cumulativeGasUsed", "effectiveGasPrice", "feeDenom", "feeAmount",
      "sponsor", "from", "to", "contractAddress", "vmError", "keyId", "receipt",
    ]
    var actual = Set(receiptJSON.keys)
    actual.remove("signingKey")
    guard actual == fields, signed.count >= 86,
      case let .array(logs) = receiptJSON["logs"]
    else { throw portableFailure() }
    for log in logs {
      guard let object = log.objectValue,
        Set(object.keys) == ["address", "topics", "data", "logIndex"]
      else { throw portableFailure() }
    }
    if let signingKey = receiptJSON["signingKey"] {
      guard let text = signingKey.stringValue,
        keys.contains(where: { portableHex($0.publicKey) == text })
      else { throw portableFailure() }
    }
    return try LocalVerifier.verifySequencerReceipt(
      signed, rawTransaction: rawTransaction, keys: keys, receiptJSON: receiptJSON,
      prevSeqHash: prevSeqHash ?? Data(signed.dropFirst(54).prefix(32)))
  }
}

private func portableFailure() -> PlatformSDKError {
  PlatformSDKError(code: .verificationFailure, retry: .never)
}

private func portableHex(_ value: Data) -> String {
  "0x" + value.map { String(format: "%02x", $0) }.joined()
}

private func portableBytes(_ text: String, count: Int? = nil) throws -> Data {
  let bytes = Array(text.utf8)
  guard bytes.count >= 4, bytes.count <= 2 + 2 * 1_048_576, bytes.count % 2 == 0,
    bytes.prefix(2) == [48, 120],
    bytes.dropFirst(2).allSatisfy({ (48...57).contains($0) || (97...102).contains($0) })
  else { throw portableFailure() }
  var result = Data()
  for index in stride(from: 2, to: bytes.count, by: 2) {
    guard let byte = UInt8(String(decoding: bytes[index...index + 1], as: UTF8.self), radix: 16)
    else { throw portableFailure() }
    result.append(byte)
  }
  guard count == nil || result.count == count else { throw portableFailure() }
  return result
}

private struct PortableKey: Codable {
  let keyId: String
  let publicKey: String
  let firstHeight: String
  let lastHeight: String

  init(_ key: ReceiptKeys) {
    keyId = portableHex(key.keyId)
    publicKey = portableHex(key.publicKey)
    firstHeight = "0x" + String(key.firstHeight, radix: 16)
    lastHeight = "0x" + String(key.lastHeight, radix: 16)
  }

  func decoded() throws -> ReceiptKeys {
    func height(_ text: String) throws -> UInt64 {
      guard text.hasPrefix("0x"), let value = UInt64(text.dropFirst(2), radix: 16),
        text == "0x" + String(value, radix: 16)
      else { throw portableFailure() }
      return value
    }
    let first = try height(firstHeight)
    let last = try height(lastHeight)
    guard first <= last else { throw portableFailure() }
    return ReceiptKeys(
      keyId: try portableBytes(keyId, count: 32),
      publicKey: try portableBytes(publicKey, count: 32),
      firstHeight: first, lastHeight: last)
  }
}

private struct PortableDocument: Codable {
  let format: String
  let verificationLevel: String
  let rawTransaction: String
  let signedReceipt: String
  let receiptDigest: String
  let receiptKey: PortableKey
  let receiptJson: [String: JSONValue]
  var blockHeader: PortableHeader? = nil
  var receiptInclusion: PortableInclusion? = nil
  var commit: PortableCommit? = nil
  var validatorSet: [PortableValidator]? = nil
}

private struct PortableDocumentScanner {
  private let bytes: [UInt8]
  private var offset = 0

  init(_ data: Data) { bytes = Array(data) }

  mutating func validate() throws {
    try value(depth: 0)
    whitespace()
    guard offset == bytes.count else { throw portableFailure() }
  }

  private mutating func whitespace() {
    while offset < bytes.count, [9, 10, 13, 32].contains(bytes[offset]) { offset += 1 }
  }

  private mutating func consume(_ byte: UInt8) -> Bool {
    whitespace()
    guard offset < bytes.count, bytes[offset] == byte else { return false }
    offset += 1
    return true
  }

  private mutating func string() throws -> String {
    whitespace()
    let start = offset
    guard consume(34) else { throw portableFailure() }
    while offset < bytes.count {
      let byte = bytes[offset]
      offset += 1
      if byte == 34 {
        return try JSONDecoder().decode(String.self, from: Data(bytes[start..<offset]))
      }
      if byte == 92 {
        guard offset < bytes.count else { throw portableFailure() }
        offset += 1
      }
    }
    throw portableFailure()
  }

  private mutating func value(depth: Int) throws {
    guard depth <= 64 else { throw portableFailure() }
    whitespace()
    if consume(123) {
      var names = Set<String>()
      if consume(125) { return }
      repeat {
        let name = try string().lowercased()
        guard names.insert(name).inserted,
          !["batchid", "asset", "previousstateroot", "resultingstateroot"].contains(name),
          consume(58)
        else { throw portableFailure() }
        try value(depth: depth + 1)
        if consume(125) { return }
      } while consume(44)
      throw portableFailure()
    }
    if consume(91) {
      if consume(93) { return }
      repeat {
        try value(depth: depth + 1)
        if consume(93) { return }
      } while consume(44)
      throw portableFailure()
    }
    guard offset < bytes.count else { throw portableFailure() }
    if bytes[offset] == 34 {
      _ = try string()
      return
    }
    let start = offset
    while offset < bytes.count, ![9, 10, 13, 32, 44, 93, 125].contains(bytes[offset]) {
      offset += 1
    }
    guard offset > start else { throw portableFailure() }
  }
}

private struct PortableCodingKey: CodingKey {
  let stringValue: String
  var intValue: Int? { nil }
  init?(stringValue: String) { self.stringValue = stringValue }
  init?(intValue: Int) { return nil }
}

private struct PortableTime: Codable {
  let value: CometTimestamp
  init(_ value: CometTimestamp) { self.value = value }

  init(from decoder: Decoder) throws {
    let text = try decoder.singleValueContainer().decode(String.self)
    let pattern = #"^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$"#
    let regex = try NSRegularExpression(pattern: pattern)
    let range = NSRange(text.startIndex..., in: text)
    guard let match = regex.firstMatch(in: text, range: range), match.range == range,
      let baseRange = Range(match.range(at: 1), in: text),
      let zoneRange = Range(match.range(at: 3), in: text)
    else { throw portableFailure() }
    let base = String(text[baseRange]) + String(text[zoneRange])
    let formatter = ISO8601DateFormatter()
    guard let date = formatter.date(from: base) else { throw portableFailure() }
    var nanos: Int32 = 0
    if let fraction = Range(match.range(at: 2), in: text) {
      let digits = String(text[fraction])
      guard let parsed = Int32(digits + String(repeating: "0", count: 9 - digits.count))
      else { throw portableFailure() }
      nanos = parsed
    }
    value = CometTimestamp(seconds: Int64(date.timeIntervalSince1970), nanos: nanos)
    guard value.valid else { throw portableFailure() }
  }

  func encode(to encoder: Encoder) throws {
    guard value.valid else { throw portableFailure() }
    let formatter = ISO8601DateFormatter()
    let base = formatter.string(from: Date(timeIntervalSince1970: Double(value.seconds)))
    guard base.hasSuffix("Z") else { throw portableFailure() }
    var fraction = value.nanos == 0 ? "" : String(format: "%09d", value.nanos)
    while fraction.last == "0" { fraction.removeLast() }
    var container = encoder.singleValueContainer()
    try container.encode(String(base.dropLast()) + (fraction.isEmpty ? "" : "." + fraction) + "Z")
  }
}

private struct PortableBlockID: Codable {
  let Hash: Data
  let PartSetTotal: UInt32
  let PartSetHash: Data

  private enum CodingKeys: String, CodingKey {
    case Hash
    case PartSetTotal
    case PartSetHash
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard Set(shape.allKeys.map(\.stringValue)) == ["Hash", "PartSetTotal", "PartSetHash"]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    Hash = try container.decode(Foundation.Data.self, forKey: .Hash)
    PartSetTotal = try container.decode(UInt32.self, forKey: .PartSetTotal)
    PartSetHash = try container.decode(Foundation.Data.self, forKey: .PartSetHash)
  }

  init(_ value: CometBlockID) {
    Hash = value.hash
    PartSetTotal = value.partSetTotal
    PartSetHash = value.partSetHash
  }

  func decoded() throws -> CometBlockID {
    return CometBlockID(
      hash: Hash,
      partSetTotal: PartSetTotal,
      partSetHash: PartSetHash)
  }
}

private struct PortableHeader: Codable {
  let VersionBlock: UInt64
  let VersionApp: UInt64
  let ChainID: String
  let Height: Int64
  let Time: PortableTime
  let LastBlockID: PortableBlockID
  let LastCommitHash: Data
  let DataHash: Data
  let ValidatorsHash: Data
  let NextValidatorsHash: Data
  let ConsensusHash: Data
  let AppHash: Data
  let LastResultsHash: Data
  let EvidenceHash: Data
  let ProposerAddress: Data

  private enum CodingKeys: String, CodingKey {
    case VersionBlock
    case VersionApp
    case ChainID
    case Height
    case Time
    case LastBlockID
    case LastCommitHash
    case DataHash
    case ValidatorsHash
    case NextValidatorsHash
    case ConsensusHash
    case AppHash
    case LastResultsHash
    case EvidenceHash
    case ProposerAddress
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard
      Set(shape.allKeys.map(\.stringValue)) == [
        "VersionBlock", "VersionApp", "ChainID", "Height", "Time", "LastBlockID", "LastCommitHash",
        "DataHash", "ValidatorsHash", "NextValidatorsHash", "ConsensusHash", "AppHash",
        "LastResultsHash", "EvidenceHash", "ProposerAddress",
      ]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    VersionBlock = try container.decode(UInt64.self, forKey: .VersionBlock)
    VersionApp = try container.decode(UInt64.self, forKey: .VersionApp)
    ChainID = try container.decode(String.self, forKey: .ChainID)
    Height = try container.decode(Int64.self, forKey: .Height)
    Time = try container.decode(PortableTime.self, forKey: .Time)
    LastBlockID = try container.decode(PortableBlockID.self, forKey: .LastBlockID)
    LastCommitHash = try container.decode(Foundation.Data.self, forKey: .LastCommitHash)
    DataHash = try container.decode(Foundation.Data.self, forKey: .DataHash)
    ValidatorsHash = try container.decode(Foundation.Data.self, forKey: .ValidatorsHash)
    NextValidatorsHash = try container.decode(Foundation.Data.self, forKey: .NextValidatorsHash)
    ConsensusHash = try container.decode(Foundation.Data.self, forKey: .ConsensusHash)
    AppHash = try container.decode(Foundation.Data.self, forKey: .AppHash)
    LastResultsHash = try container.decode(Foundation.Data.self, forKey: .LastResultsHash)
    EvidenceHash = try container.decode(Foundation.Data.self, forKey: .EvidenceHash)
    ProposerAddress = try container.decode(Foundation.Data.self, forKey: .ProposerAddress)
  }

  init(_ value: CommittedHeader) {
    VersionBlock = value.versionBlock
    VersionApp = value.versionApp
    ChainID = value.chainID
    Height = value.height
    Time = PortableTime(value.time)
    LastBlockID = PortableBlockID(value.lastBlockID)
    LastCommitHash = value.lastCommitHash
    DataHash = value.dataHash
    ValidatorsHash = value.validatorsHash
    NextValidatorsHash = value.nextValidatorsHash
    ConsensusHash = value.consensusHash
    AppHash = value.appHash
    LastResultsHash = value.lastResultsHash
    EvidenceHash = value.evidenceHash
    ProposerAddress = value.proposerAddress
  }

  func decoded() throws -> CommittedHeader {
    return try CommittedHeader(
      versionBlock: VersionBlock,
      versionApp: VersionApp,
      chainID: ChainID,
      height: Height,
      time: Time.value,
      lastBlockID: LastBlockID.decoded(),
      lastCommitHash: LastCommitHash,
      dataHash: DataHash,
      validatorsHash: ValidatorsHash,
      nextValidatorsHash: NextValidatorsHash,
      consensusHash: ConsensusHash,
      appHash: AppHash,
      lastResultsHash: LastResultsHash,
      evidenceHash: EvidenceHash,
      proposerAddress: ProposerAddress)
  }
}

private struct PortableValidator: Codable {
  let PublicKey: [UInt8]
  let VotingPower: Int64

  private enum CodingKeys: String, CodingKey {
    case PublicKey
    case VotingPower
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard Set(shape.allKeys.map(\.stringValue)) == ["PublicKey", "VotingPower"]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    PublicKey = try container.decode([UInt8].self, forKey: .PublicKey)
    VotingPower = try container.decode(Int64.self, forKey: .VotingPower)
  }

  init(_ value: CometValidator) {
    PublicKey = Array(value.publicKey)
    VotingPower = value.votingPower
  }

  func decoded() throws -> CometValidator {
    guard PublicKey.count == 32 else { throw portableFailure() }
    return CometValidator(
      publicKey: Data(PublicKey),
      votingPower: VotingPower)
  }
}

private struct PortableSignature: Codable {
  let BlockIDFlag: UInt8
  let ValidatorAddress: Data
  let Timestamp: PortableTime
  let Signature: Data

  private enum CodingKeys: String, CodingKey {
    case BlockIDFlag
    case ValidatorAddress
    case Timestamp
    case Signature
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard
      Set(shape.allKeys.map(\.stringValue)) == [
        "BlockIDFlag", "ValidatorAddress", "Timestamp", "Signature",
      ]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    BlockIDFlag = try container.decode(UInt8.self, forKey: .BlockIDFlag)
    ValidatorAddress = try container.decode(Foundation.Data.self, forKey: .ValidatorAddress)
    Timestamp = try container.decode(PortableTime.self, forKey: .Timestamp)
    Signature = try container.decode(Foundation.Data.self, forKey: .Signature)
  }

  init(_ value: CometCommitSignature) {
    BlockIDFlag = value.blockIDFlag.rawValue
    ValidatorAddress = value.validatorAddress
    Timestamp = PortableTime(
      value.blockIDFlag == .absent && value.timestamp == .init()
        ? CometTimestamp(seconds: -62_135_596_800) : value.timestamp)
    Signature = value.signature
  }

  func decoded() throws -> CometCommitSignature {
    guard let flag = CometBlockIDFlag(rawValue: BlockIDFlag) else { throw portableFailure() }
    return CometCommitSignature(
      blockIDFlag: flag,
      validatorAddress: ValidatorAddress,
      timestamp: flag == .absent && Timestamp.value == CometTimestamp(seconds: -62_135_596_800)
        ? .init() : Timestamp.value,
      signature: Signature)
  }
}

private struct PortableCommit: Codable {
  let Height: Int64
  let Round: Int32
  let BlockID: PortableBlockID
  let Signatures: [PortableSignature]

  private enum CodingKeys: String, CodingKey {
    case Height
    case Round
    case BlockID
    case Signatures
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard Set(shape.allKeys.map(\.stringValue)) == ["Height", "Round", "BlockID", "Signatures"]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    Height = try container.decode(Int64.self, forKey: .Height)
    Round = try container.decode(Int32.self, forKey: .Round)
    BlockID = try container.decode(PortableBlockID.self, forKey: .BlockID)
    Signatures = try container.decode([PortableSignature].self, forKey: .Signatures)
  }

  init(_ value: CometCommit) {
    Height = value.height
    Round = value.round
    BlockID = PortableBlockID(value.blockID)
    Signatures = value.signatures.map(PortableSignature.init)
  }

  func decoded() throws -> CometCommit {
    guard Signatures.count <= 10_000 else { throw portableFailure() }
    return try CometCommit(
      height: Height,
      round: Round,
      blockID: BlockID.decoded(),
      signatures: Signatures.map { try $0.decoded() })
  }
}

private struct PortableProof: Codable {
  let Total: Int64
  let Index: Int64
  let Aunts: [[UInt8]]?

  private enum CodingKeys: String, CodingKey {
    case Total
    case Index
    case Aunts
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard Set(shape.allKeys.map(\.stringValue)) == ["Total", "Index", "Aunts"]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    Total = try container.decode(Int64.self, forKey: .Total)
    Index = try container.decode(Int64.self, forKey: .Index)
    Aunts = try container.decodeIfPresent([[UInt8]].self, forKey: .Aunts)
  }

  init(_ value: CommittedProof) {
    Total = value.total
    Index = value.index
    Aunts = value.aunts.map { Array($0) }
  }

  func decoded() throws -> CommittedProof {
    guard (Aunts ?? []).count <= 63, (Aunts ?? []).allSatisfy({ $0.count == 32 })
    else { throw portableFailure() }
    return CommittedProof(
      total: Total,
      index: Index,
      aunts: (Aunts ?? []).map { Data($0) })
  }
}

private struct PortableResult: Codable {
  let Code: UInt32
  let Data: Data
  let GasWanted: Int64
  let GasUsed: Int64

  private enum CodingKeys: String, CodingKey {
    case Code
    case Data
    case GasWanted
    case GasUsed
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard Set(shape.allKeys.map(\.stringValue)) == ["Code", "Data", "GasWanted", "GasUsed"]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    Code = try container.decode(UInt32.self, forKey: .Code)
    Data = try container.decode(Foundation.Data.self, forKey: .Data)
    GasWanted = try container.decode(Int64.self, forKey: .GasWanted)
    GasUsed = try container.decode(Int64.self, forKey: .GasUsed)
  }

  init(_ value: ExecTxResult) {
    Code = value.code
    Data = value.data
    GasWanted = value.gasWanted
    GasUsed = value.gasUsed
  }

  func decoded() throws -> ExecTxResult {
    return ExecTxResult(
      code: Code,
      data: Data,
      gasWanted: GasWanted,
      gasUsed: GasUsed)
  }
}

private struct PortableInclusion: Codable {
  let Transaction: Data
  let TransactionProof: PortableProof
  let Result: PortableResult
  let ResultProof: PortableProof
  let BatchTransaction: Data
  let BatchProof: PortableProof
  let NextHeader: PortableHeader
  let NextCommit: PortableCommit
  let NextValidators: [PortableValidator]

  private enum CodingKeys: String, CodingKey {
    case Transaction
    case TransactionProof
    case Result
    case ResultProof
    case BatchTransaction
    case BatchProof
    case NextHeader
    case NextCommit
    case NextValidators
  }

  init(from decoder: Decoder) throws {
    let shape = try decoder.container(keyedBy: PortableCodingKey.self)
    guard
      Set(shape.allKeys.map(\.stringValue)) == [
        "Transaction", "TransactionProof", "Result", "ResultProof", "BatchTransaction",
        "BatchProof", "NextHeader", "NextCommit", "NextValidators",
      ]
    else { throw portableFailure() }
    let container = try decoder.container(keyedBy: CodingKeys.self)
    Transaction = try container.decode(Foundation.Data.self, forKey: .Transaction)
    TransactionProof = try container.decode(PortableProof.self, forKey: .TransactionProof)
    Result = try container.decode(PortableResult.self, forKey: .Result)
    ResultProof = try container.decode(PortableProof.self, forKey: .ResultProof)
    BatchTransaction = try container.decode(Foundation.Data.self, forKey: .BatchTransaction)
    BatchProof = try container.decode(PortableProof.self, forKey: .BatchProof)
    NextHeader = try container.decode(PortableHeader.self, forKey: .NextHeader)
    NextCommit = try container.decode(PortableCommit.self, forKey: .NextCommit)
    NextValidators = try container.decode([PortableValidator].self, forKey: .NextValidators)
  }

  init(_ value: ReceiptInclusion) {
    Transaction = value.transaction
    TransactionProof = PortableProof(value.transactionProof)
    Result = PortableResult(value.result)
    ResultProof = PortableProof(value.resultProof)
    BatchTransaction = value.batchTransaction
    BatchProof = PortableProof(value.batchProof)
    NextHeader = PortableHeader(value.nextHeader)
    NextCommit = PortableCommit(value.nextCommit)
    NextValidators = value.nextValidators.map(PortableValidator.init)
  }

  func decoded() throws -> ReceiptInclusion {
    guard NextValidators.count <= 10_000 else { throw portableFailure() }
    return try ReceiptInclusion(
      transaction: Transaction,
      transactionProof: TransactionProof.decoded(),
      result: Result.decoded(),
      resultProof: ResultProof.decoded(),
      batchTransaction: BatchTransaction,
      batchProof: BatchProof.decoded(),
      nextHeader: NextHeader.decoded(),
      nextCommit: NextCommit.decoded(),
      nextValidators: NextValidators.map { try $0.decoded() })
  }
}
