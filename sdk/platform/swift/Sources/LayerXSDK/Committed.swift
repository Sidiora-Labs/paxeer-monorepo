import Crypto
import Foundation

public struct CometTimestamp: Sendable, Equatable {
  public var seconds: Int64
  public var nanos: Int32
  public init(seconds: Int64 = 0, nanos: Int32 = 0) {
    self.seconds = seconds
    self.nanos = nanos
  }
  var valid: Bool {
    (-62_135_596_800...253_402_300_799).contains(seconds) && (0..<1_000_000_000).contains(nanos)
  }
}

public struct CometBlockID: Sendable, Equatable {
  public var hash: Data
  public var partSetTotal: UInt32
  public var partSetHash: Data
  public init(hash: Data = Data(), partSetTotal: UInt32 = 0, partSetHash: Data = Data()) {
    self.hash = hash
    self.partSetTotal = partSetTotal
    self.partSetHash = partSetHash
  }
  var encoded: Data {
    let parts =
      CommittedWire.number(1, UInt64(partSetTotal))
      + (partSetHash.isEmpty ? Data() : CommittedWire.bytes(2, partSetHash))
    return (hash.isEmpty ? Data() : CommittedWire.bytes(1, hash)) + CommittedWire.bytes(2, parts)
  }
}

public struct CommittedHeader: Sendable {
  public var versionBlock: UInt64
  public var versionApp: UInt64
  public var chainID: String
  public var height: Int64
  public var time: CometTimestamp
  public var lastBlockID: CometBlockID
  public var lastCommitHash: Data
  public var dataHash: Data
  public var validatorsHash: Data
  public var nextValidatorsHash: Data
  public var consensusHash: Data
  public var appHash: Data
  public var lastResultsHash: Data
  public var evidenceHash: Data
  public var proposerAddress: Data

  public init(
    versionBlock: UInt64 = 11, versionApp: UInt64 = 0, chainID: String,
    height: Int64, time: CometTimestamp, lastBlockID: CometBlockID = .init(),
    lastCommitHash: Data = Data(), dataHash: Data, validatorsHash: Data,
    nextValidatorsHash: Data, consensusHash: Data = Data(), appHash: Data = Data(),
    lastResultsHash: Data = Data(), evidenceHash: Data = Data(), proposerAddress: Data = Data()
  ) {
    self.versionBlock = versionBlock
    self.versionApp = versionApp
    self.chainID = chainID
    self.height = height
    self.time = time
    self.lastBlockID = lastBlockID
    self.lastCommitHash = lastCommitHash
    self.dataHash = dataHash
    self.validatorsHash = validatorsHash
    self.nextValidatorsHash = nextValidatorsHash
    self.consensusHash = consensusHash
    self.appHash = appHash
    self.lastResultsHash = lastResultsHash
    self.evidenceHash = evidenceHash
    self.proposerAddress = proposerAddress
  }
  public var hash: Data {
    CommittedWire.root([
      CommittedWire.number(1, versionBlock) + CommittedWire.number(2, versionApp),
      CommittedWire.wrapped(Data(chainID.utf8)),
      CommittedWire.number(1, UInt64(bitPattern: height)),
      CommittedWire.timestamp(time), lastBlockID.encoded, CommittedWire.wrapped(lastCommitHash),
      CommittedWire.wrapped(dataHash), CommittedWire.wrapped(validatorsHash),
      CommittedWire.wrapped(nextValidatorsHash), CommittedWire.wrapped(consensusHash),
      CommittedWire.wrapped(appHash), CommittedWire.wrapped(lastResultsHash),
      CommittedWire.wrapped(evidenceHash), CommittedWire.wrapped(proposerAddress),
    ])
  }
}

public struct CometValidator: Sendable {
  public var publicKey: Data
  public var votingPower: Int64
  public init(publicKey: Data, votingPower: Int64) {
    self.publicKey = publicKey
    self.votingPower = votingPower
  }
}

public enum CometBlockIDFlag: UInt8, Sendable {
  case absent = 1
  case block = 2
  case `nil` = 3
}

public struct CometCommitSignature: Sendable {
  public var blockIDFlag: CometBlockIDFlag
  public var validatorAddress: Data
  public var timestamp: CometTimestamp
  public var signature: Data
  public init(
    blockIDFlag: CometBlockIDFlag, validatorAddress: Data = Data(),
    timestamp: CometTimestamp = .init(), signature: Data = Data()
  ) {
    self.blockIDFlag = blockIDFlag
    self.validatorAddress = validatorAddress
    self.timestamp = timestamp
    self.signature = signature
  }
}

public struct CometCommit: Sendable {
  public var height: Int64
  public var round: Int32
  public var blockID: CometBlockID
  public var signatures: [CometCommitSignature]
  public init(
    height: Int64, round: Int32, blockID: CometBlockID,
    signatures: [CometCommitSignature]
  ) {
    self.height = height
    self.round = round
    self.blockID = blockID
    self.signatures = signatures
  }
}

public struct CommittedProof: Sendable {
  public var total: Int64
  public var index: Int64
  public var aunts: [Data]
  public init(total: Int64, index: Int64, aunts: [Data]) {
    self.total = total
    self.index = index
    self.aunts = aunts
  }
}

public struct ExecTxResult: Sendable {
  public var code: UInt32
  public var data: Data
  public var gasWanted: Int64
  public var gasUsed: Int64
  public init(code: UInt32 = 0, data: Data = Data(), gasWanted: Int64 = 0, gasUsed: Int64 = 0) {
    self.code = code
    self.data = data
    self.gasWanted = gasWanted
    self.gasUsed = gasUsed
  }
  var encoded: Data {
    CommittedWire.number(1, UInt64(code)) + (data.isEmpty ? Data() : CommittedWire.bytes(2, data))
      + CommittedWire.number(5, UInt64(bitPattern: gasWanted))
      + CommittedWire.number(6, UInt64(bitPattern: gasUsed))
  }
}

public struct ReceiptInclusion: Sendable {
  public var transaction: Data
  public var transactionProof: CommittedProof
  public var result: ExecTxResult
  public var resultProof: CommittedProof
  public var batchTransaction: Data
  public var batchProof: CommittedProof
  public var nextHeader: CommittedHeader
  public var nextCommit: CometCommit
  public var nextValidators: [CometValidator]
  public init(
    transaction: Data, transactionProof: CommittedProof, result: ExecTxResult,
    resultProof: CommittedProof, batchTransaction: Data, batchProof: CommittedProof,
    nextHeader: CommittedHeader, nextCommit: CometCommit, nextValidators: [CometValidator]
  ) {
    self.transaction = transaction
    self.transactionProof = transactionProof
    self.result = result
    self.resultProof = resultProof
    self.batchTransaction = batchTransaction
    self.batchProof = batchProof
    self.nextHeader = nextHeader
    self.nextCommit = nextCommit
    self.nextValidators = nextValidators
  }
}

public enum CommittedError: Error, Sendable, Equatable {
  case receipt, height, commit, nextHeader, nextCommit, inclusion, result, batch
}

public struct Committed: Sendable {
  public let receiptVerification: SequencerReceiptVerification
  public let height: UInt64
  public let blockHash: Data
  public let votingPower: Int64
  public var level: String { "committed" }
  public var rung: String { level }
  public var status: UInt8 { receiptVerification.status }
  public var receiptDigest: Data { receiptVerification.receiptDigest }

  public static func verify(
    _ receiptVerification: SequencerReceiptVerification,
    blockHeader: CommittedHeader, receiptInclusion: ReceiptInclusion,
    commit: CometCommit, validatorSet: [CometValidator]
  ) throws -> Committed {
    let r = receiptVerification.receipt
    let i = receiptInclusion
    guard receiptVerification.level == "receipted", (0...1).contains(r.resultCode),
      receiptVerification.status == UInt8(1 - r.resultCode), r.batchID.count == 32,
      r.batchID.prefix(24).allSatisfy({ $0 == 0 }),
      let effect = r.effects.first, effect.body.count >= 28
    else { throw CommittedError.receipt }
    let batchHeight = CommittedWire.bigEndian(r.batchID.suffix(8))
    guard batchHeight > 0, blockHeader.height > 0, blockHeader.height < Int64.max,
      UInt64(blockHeader.height) >= batchHeight,
      i.nextHeader.height == blockHeader.height + 1,
      i.nextHeader.chainID == blockHeader.chainID,
      i.nextHeader.validatorsHash == blockHeader.nextValidatorsHash
    else { throw CommittedError.height }
    let power = try CommittedWire.verifyCommit(blockHeader, commit, validatorSet, error: .commit)
    guard i.nextHeader.lastBlockID == commit.blockID else { throw CommittedError.nextHeader }
    _ = try CommittedWire.verifyCommit(
      i.nextHeader, i.nextCommit, i.nextValidators, error: .nextCommit)
    let raw: Data
    do { raw = try CommittedWire.evmTransaction(i.transaction) } catch {
      throw CommittedError.inclusion
    }
    guard CommittedWire.keccak256(raw) == r.activityID,
      i.transactionProof.index > 0, i.transactionProof.index == i.resultProof.index,
      i.transactionProof.total == i.resultProof.total,
      i.batchProof.total == i.transactionProof.total, i.batchProof.index == 0,
      UInt64(i.transactionProof.index)
        == CommittedWire.bigEndian(effect.body.dropFirst(24).prefix(4)),
      CommittedWire.verifyProof(
        CommittedWire.sha(i.transaction), i.transactionProof, blockHeader.dataHash),
      CommittedWire.verifyProof(i.result.encoded, i.resultProof, i.nextHeader.lastResultsHash),
      CommittedWire.verifyProof(
        CommittedWire.sha(i.batchTransaction), i.batchProof, blockHeader.dataHash)
    else { throw CommittedError.inclusion }
    let resultStatus: Int32
    switch i.result.code {
    case 0: resultStatus = 0
    case 45: resultStatus = 1
    default: throw CommittedError.result
    }
    guard resultStatus == r.resultCode, i.result.gasUsed >= 0,
      i.result.gasWanted >= i.result.gasUsed,
      UInt64(i.result.gasUsed) == CommittedWire.bigEndian(effect.body.prefix(8))
    else { throw CommittedError.result }
    let domain = Data("PAXEER/v1/sequence-batch\0".utf8)
    guard i.batchTransaction.count == domain.count + 24 + 128 + 64,
      i.batchTransaction.starts(with: domain)
    else { throw CommittedError.batch }
    let batch = Data(i.batchTransaction.dropFirst(domain.count))
    let from = CommittedWire.bigEndian(batch.dropFirst(8).prefix(8))
    let to = CommittedWire.bigEndian(batch.dropFirst(16).prefix(8))
    guard CommittedWire.bigEndian(batch.prefix(8)) == UInt64(blockHeader.height),
      from > 0, from <= r.globalSequence, r.globalSequence <= to
    else { throw CommittedError.batch }
    return Committed(
      receiptVerification: receiptVerification, height: UInt64(blockHeader.height),
      blockHash: blockHeader.hash, votingPower: power)
  }
}

enum CommittedWire {
  static func sha(_ data: Data) -> Data { Data(SHA256.hash(data: data)) }
  static func bigEndian(_ data: Data) -> UInt64 {
    data.reduce(UInt64(0)) { ($0 << 8) | UInt64($1) }
  }
  static func varint(_ value: UInt64) -> Data {
    var n = value
    var out = Data()
    while n >= 128 {
      out.append(UInt8(n & 127) | 128)
      n >>= 7
    }
    out.append(UInt8(n))
    return out
  }
  static func number(_ field: UInt64, _ value: UInt64) -> Data {
    value == 0 ? Data() : varint(field << 3) + varint(value)
  }
  static func bytes(_ field: UInt64, _ value: Data) -> Data {
    varint(field << 3 | 2) + varint(UInt64(value.count)) + value
  }
  static func wrapped(_ value: Data) -> Data { value.isEmpty ? Data() : bytes(1, value) }
  static func timestamp(_ time: CometTimestamp) -> Data {
    number(1, UInt64(bitPattern: time.seconds)) + number(2, UInt64(bitPattern: Int64(time.nanos)))
  }
  static func leaf(_ data: Data) -> Data { sha(Data([0]) + data) }
  static func inner(_ left: Data, _ right: Data) -> Data { sha(Data([1]) + left + right) }
  static func split(_ total: Int64) -> Int64 {
    Int64(1) << (63 - (UInt64(total - 1)).leadingZeroBitCount)
  }
  static func root(_ leaves: [Data]) -> Data {
    if leaves.isEmpty { return sha(Data()) }
    if leaves.count == 1 { return leaf(leaves[0]) }
    let k = Int(split(Int64(leaves.count)))
    return inner(root(Array(leaves[..<k])), root(Array(leaves[k...])))
  }
  static func validatorsHash(_ set: [CometValidator]) -> Data {
    root(
      set.map { bytes(1, bytes(1, $0.publicKey)) + number(2, UInt64(bitPattern: $0.votingPower)) })
  }
  static func verifyProof(_ value: Data, _ proof: CommittedProof, _ expected: Data) -> Bool {
    guard expected.count == 32, proof.total > 0, proof.index >= 0, proof.index < proof.total,
      proof.aunts.count <= 63, proof.aunts.allSatisfy({ $0.count == 32 })
    else { return false }
    func reconstruct(_ index: Int64, _ total: Int64, _ depth: Int) -> Data? {
      if total == 1 { return depth == 0 ? leaf(value) : nil }
      guard depth > 0 else { return nil }
      let k = split(total)
      let aunt = proof.aunts[depth - 1]
      if index < k {
        guard let child = reconstruct(index, k, depth - 1) else { return nil }
        return inner(child, aunt)
      }
      guard let child = reconstruct(index - k, total - k, depth - 1) else { return nil }
      return inner(aunt, child)
    }
    return reconstruct(proof.index, proof.total, proof.aunts.count) == expected
  }
  static func voteBytes(_ chainID: String, _ commit: CometCommit, _ sig: CometCommitSignature)
    -> Data
  {
    func fixed(_ value: UInt64) -> Data {
      Data((0..<8).map { UInt8(truncatingIfNeeded: value >> ($0 * 8)) })
    }
    var out = Data([8, 2, 0x11]) + fixed(UInt64(bitPattern: commit.height))
    if commit.round != 0 { out += Data([0x19]) + fixed(UInt64(bitPattern: Int64(commit.round))) }
    if sig.blockIDFlag == .block { out += bytes(4, commit.blockID.encoded) }
    out += bytes(5, timestamp(sig.timestamp)) + bytes(6, Data(chainID.utf8))
    return varint(UInt64(out.count)) + out
  }
  static func verifyCommit(
    _ header: CommittedHeader, _ commit: CometCommit,
    _ set: [CometValidator], error: CommittedError
  ) throws -> Int64 {
    guard header.height > 0, !header.chainID.isEmpty, header.chainID.utf8.count <= 50,
      header.time.valid, commit.height == header.height, commit.round >= 0,
      !set.isEmpty, commit.signatures.count == set.count,
      commit.blockID.hash.count == 32, commit.blockID.partSetTotal > 0,
      commit.blockID.partSetHash.count == 32,
      header.validatorsHash == validatorsHash(set), commit.blockID.hash == header.hash
    else { throw error }
    var seen = Set<Data>()
    var total: Int64 = 0
    var signed: Int64 = 0
    for v in set {
      guard v.publicKey.count == 32, v.votingPower > 0,
        v.votingPower <= Int64.max / 8 - total, seen.insert(v.publicKey).inserted
      else { throw error }
      total += v.votingPower
    }
    for (v, sig) in zip(set, commit.signatures) {
      if sig.blockIDFlag == .absent {
        guard sig.signature.isEmpty, sig.validatorAddress.isEmpty, sig.timestamp == .init()
        else { throw error }
        continue
      }
      guard sig.timestamp.valid, sig.signature.count == 64,
        sig.validatorAddress == sha(v.publicKey).prefix(20),
        let key = try? Curve25519.Signing.PublicKey(rawRepresentation: v.publicKey),
        key.isValidSignature(sig.signature, for: voteBytes(header.chainID, commit, sig))
      else { throw error }
      if sig.blockIDFlag == .block { signed += v.votingPower }
    }
    guard signed * 3 > total * 2 else { throw error }
    return signed
  }

  struct ProtoValue {
    var wire: UInt64
    var data = Data()
    var number: UInt64 = 0
  }
  struct Proto {
    var fields: [UInt64: [ProtoValue]] = [:]
    init(_ input: Data) throws {
      let data = [UInt8](input)
      var offset = 0
      func read() throws -> UInt64 {
        var result: UInt64 = 0
        for i in 0..<10 {
          guard offset < data.count else { throw CommittedError.inclusion }
          let byte = data[offset]
          offset += 1
          guard i < 9 || byte <= 1 else { throw CommittedError.inclusion }
          result |= UInt64(byte & 127) << (7 * i)
          if byte < 128 { return result }
        }
        throw CommittedError.inclusion
      }
      while offset < data.count {
        let tag = try read()
        guard tag >> 3 > 0, tag >> 3 < (1 << 29) else { throw CommittedError.inclusion }
        var value = ProtoValue(wire: tag & 7)
        switch value.wire {
        case 0: value.number = try read()
        case 1, 2, 5:
          let count = value.wire == 2 ? try read() : (value.wire == 1 ? 8 : 4)
          guard count <= UInt64(data.count - offset) else { throw CommittedError.inclusion }
          value.data = Data(data[offset..<(offset + Int(count))])
          offset += Int(count)
        default: throw CommittedError.inclusion
        }
        fields[tag >> 3, default: []].append(value)
      }
    }
    func single(_ field: UInt64, _ wire: UInt64) throws -> ProtoValue {
      let values = fields[field, default: []]
      if values.isEmpty { return ProtoValue(wire: wire) }
      guard values.count == 1, values[0].wire == wire else { throw CommittedError.inclusion }
      return values[0]
    }
    func message(_ field: UInt64) throws -> Proto {
      let value = try single(field, 2)
      guard !value.data.isEmpty else { throw CommittedError.inclusion }
      return try Proto(value.data)
    }
  }
  static func evmTransaction(_ transaction: Data) throws -> Data {
    guard !transaction.isEmpty, transaction.count <= 1_048_576 else {
      throw CommittedError.inclusion
    }
    let body = try Proto(transaction).message(1)
    guard let first = body.fields[1]?.first, first.wire == 2 else { throw CommittedError.inclusion }
    let message = try Proto(first.data)
    let type = try message.single(1, 2).data
    guard
      ["/paxprotocol.paxchain.evm.MsgEVMTransaction", "/ethermint.evm.v1.MsgEthereumTx"]
        .contains(String(decoding: type, as: UTF8.self))
    else { throw CommittedError.inclusion }
    let raw = try message.message(2).single(1, 2).data
    guard let prefix = raw.first else { throw CommittedError.inclusion }
    if prefix >= 0xc0 || (1...4).contains(prefix) { return raw }
    let any = try Proto(raw)
    return try encodeEVM(String(decoding: any.single(1, 2).data, as: UTF8.self), any.message(2))
  }
  static func rlp(_ data: Data, list: Bool = false) -> Data {
    if !list, data.count == 1, data.first! < 128 { return data }
    let prefix: UInt8 = list ? 0xc0 : 0x80
    if data.count <= 55 { return Data([prefix + UInt8(data.count)]) + data }
    let size = integer(UInt64(data.count))
    return Data([prefix + 55 + UInt8(size.count)]) + size + data
  }
  static func integer(_ n: UInt64) -> Data {
    Data(
      (0..<8).reversed().map { UInt8(truncatingIfNeeded: n >> ($0 * 8)) }.drop(while: { $0 == 0 }))
  }
  static func hex(_ data: Data, length: Int) throws -> Data {
    let chars = Array(data)
    guard chars.count == 2 + length * 2, chars.prefix(2) == [48, 120] else {
      throw CommittedError.inclusion
    }
    var out = Data()
    for i in stride(from: 2, to: chars.count, by: 2) {
      guard let n = UInt8(String(decoding: chars[i...i + 1], as: UTF8.self), radix: 16)
      else { throw CommittedError.inclusion }
      out.append(n)
    }
    return out
  }
  static func decimal(_ data: Data) throws -> Data {
    guard data.count <= 78 else { throw CommittedError.inclusion }
    var value = [UInt8](repeating: 0, count: 32)
    for digit in data {
      guard (48...57).contains(digit) else { throw CommittedError.inclusion }
      var carry = Int(digit - 48)
      for i in (0..<32).reversed() {
        carry += Int(value[i]) * 10
        value[i] = UInt8(carry & 255)
        carry >>= 8
      }
      guard carry == 0 else { throw CommittedError.inclusion }
    }
    return Data(value.drop(while: { $0 == 0 }))
  }
  static func encodeEVM(_ kind: String, _ fields: Proto) throws -> Data {
    let layouts: [String: (UInt8, String)] = [
      "/paxprotocol.paxchain.eth.LegacyTx": (0, "uduhdrbbb"),
      "/paxprotocol.paxchain.eth.AccessListTx": (1, "duduhdrabbb"),
      "/paxprotocol.paxchain.eth.DynamicFeeTx": (2, "dudduhdrabbb"),
      "/paxprotocol.paxchain.eth.BlobTx": (3, "dudduhdradx_bbb"),
      "/paxprotocol.paxchain.eth.SetCodeTx": (4, "dudduhdraobbb"),
    ]
    guard let (type, layout) = layouts[kind] else { throw CommittedError.inclusion }
    var payload = Data()
    for (index, format) in layout.enumerated() {
      let field = UInt64(index + 1)
      if format == "_" { continue }
      if format == "a" || format == "x" || format == "o" {
        var entries = Data()
        for entry in fields.fields[field, default: []] {
          guard entry.wire == 2 else { throw CommittedError.inclusion }
          if format == "x" {
            guard entry.data.count == 32 else { throw CommittedError.inclusion }
            entries += rlp(entry.data)
          } else {
            let tuple = try Proto(entry.data)
            var encoded = Data()
            if format == "o" {
              for (i, f) in Array("dhubbb").enumerated() {
                encoded += try evmField(tuple.single(UInt64(i + 1), f == "u" ? 0 : 2), f)
              }
            } else {
              encoded += try rlp(hex(tuple.single(1, 2).data, length: 20))
              var keys = Data()
              for key in tuple.fields[2, default: []] {
                guard key.wire == 2 else { throw CommittedError.inclusion }
                keys += try rlp(hex(key.data, length: 32))
              }
              encoded += rlp(keys, list: true)
            }
            entries += rlp(encoded, list: true)
          }
        }
        payload += rlp(entries, list: true)
      } else {
        payload += try evmField(fields.single(field, format == "u" ? 0 : 2), format)
      }
    }
    return (type == 0 ? Data() : Data([type])) + rlp(payload, list: true)
  }
  static func evmField(_ value: ProtoValue, _ format: Character) throws -> Data {
    var data = value.data
    switch format {
    case "u": data = integer(value.number)
    case "d": data = try decimal(data)
    case "h": if !data.isEmpty { data = try hex(data, length: 20) }
    case "b":
      guard data.count <= 32 else { throw CommittedError.inclusion }
      data = Data(data.drop(while: { $0 == 0 }))
    default: break
    }
    return rlp(data)
  }
  static func keccak256(_ data: Data) -> Data {
    let rc: [UInt64] = [
      0x1, 0x8082, 0x8000_0000_0000_808a, 0x8000_0000_8000_8000, 0x808b, 0x8000_0001,
      0x8000_0000_8000_8081, 0x8000_0000_0000_8009, 0x8a, 0x88, 0x8000_8009, 0x8000_000a,
      0x8000_808b,
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

}
