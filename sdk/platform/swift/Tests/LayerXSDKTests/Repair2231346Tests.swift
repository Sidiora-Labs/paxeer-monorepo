import Crypto
import Foundation
import XCTest

@testable import LayerXSDK

#if canImport(FoundationNetworking)
  import FoundationNetworking
#endif
#if os(Linux)
  import Glibc
#else
  import Darwin
#endif

// Task 22.31.346 repair record:
// receipt-committed-rung: testReceiptCommittedRungThroughProgramsActivity
// receipt-payment-denom: testReceiptPaymentDenomIndependentBindings
// portable-receipt-fields: testPortableReceiptRecordedProducerRoundTrips
// committed-chain-trust: testCommittedChainTrustRejectsSelfSignedChains
// Baseline verification was not run: the task's explicit gate order requires implementation first.
// Wiring request: sdk/platform/conformance/fixtures (platform conformance owner) must publish a
// different-payment/fee-denomination revert and its RPC response; the existing fee-token fixture
// records only a successful signed transfer without receiptJson. No such revert is fabricated here.
// Wiring request: SDK applications and CommittedReceiptTests/PortableReceiptTests (platform SDK
// owner) must supply independently authenticated CommittedTrust when requesting commitment.
final class Repair2231346Tests: XCTestCase {
  private typealias Record = [String: JSONValue]
  private var root: URL {
    var url = URL(fileURLWithPath: #filePath)
    for _ in 0..<6 { url.deleteLastPathComponent() }
    return url
  }
  private func read(_ path: String) throws -> Record {
    try JSONDecoder().decode(Record.self, from: Data(contentsOf: root.appendingPathComponent(path)))
  }
  private func fixture(_ name: String) throws -> Record {
    try read("sdk/platform/conformance/fixtures/\(name).json")
  }
  private func object(_ record: Record, _ field: String) throws -> Record {
    try XCTUnwrap(record[field]?.objectValue, field)
  }
  private func text(_ record: Record, _ field: String) throws -> String {
    try XCTUnwrap(record[field]?.stringValue, field)
  }
  private func integer(_ record: Record, _ field: String) throws -> Int64 {
    try XCTUnwrap(record[field]?.integerValue, field)
  }
  private func array(_ record: Record, _ field: String) throws -> [JSONValue] {
    guard case let .array(values) = record[field] else {
      throw NSError(domain: "RecordedReceipt", code: 1)
    }
    return values
  }
  private func hex(_ text: String) throws -> Data {
    try ProgramABI.hex(text)
  }
  private func bytes(_ record: Record, _ field: String) throws -> Data {
    try hex(text(record, field))
  }
  private func base64(_ record: Record, _ field: String) throws -> Data {
    try XCTUnwrap(Data(base64Encoded: text(record, field)))
  }
  private func encode(_ record: Record) throws -> Data {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
    return try encoder.encode(record)
  }
  private func pin(_ vector: Record) throws -> ReceiptKeys {
    let value = try object(vector, "pinnedKey")
    return try ReceiptKeys(
      keyId: bytes(value, "keyId"), publicKey: bytes(value, "publicKey"),
      firstHeight: UInt64(integer(value, "firstHeight")),
      lastHeight: UInt64(integer(value, "lastHeight")))
  }
  private func verify(_ vector: Record, view: Record? = nil) throws -> SequencerReceiptVerification
  {
    try LocalVerifier.verifySequencerReceipt(
      bytes(vector, "signedReceipt"), rawTransaction: bytes(vector, "rawTransaction"),
      keys: [pin(vector)], receiptJSON: view ?? object(vector, "receiptJson"),
      prevSeqHash: bytes(vector, "prevSeqHash"))
  }
  private func recordedCommitted() throws -> (Record, Record) {
    let vector = try object(fixture("vector-v4-committed"), "positive")
    let source = try read("daemon/committed-fixture/vectors/vector-v4-committed.json")
    let view = try object(object(source, "settlement"), "receipt")
    XCTAssertEqual(view["receipt"], vector["signedReceipt"])
    XCTAssertEqual(view["rung"], .string("committed"))
    return (vector, view)
  }
  private func checkpoint() throws -> CommittedTrust {
    // Independently pinned execution block from the recorded local chain, never from bundle input.
    try CommittedTrust(
      chainID: "committed-fixture", height: 10,
      blockHash: hex("0x066b3b86eb6f62437b659c1d075f28eb22246d233fe86b3d5cacbdb5e18216e9"))
  }
  private func export(_ vector: Record, view: Record) throws -> LayerXSDK.PortableReceipt {
    try LayerXSDK.PortableReceipt.export(
      rawTransaction: bytes(vector, "rawTransaction"),
      signedReceipt: bytes(vector, "signedReceipt"),
      receiptJSON: view, receiptKey: pin(vector), prevSeqHash: bytes(vector, "prevSeqHash"))
  }
  private func published(_ level: String) throws -> Record {
    let values = try array(fixture("vector-v5-portable"), "bundles")
    return try XCTUnwrap(
      values.compactMap(\.objectValue).first {
        $0["verificationLevel"] == .string(level)
      })
  }

  func testReceiptCommittedRungThroughProgramsActivity() async throws {
    let (vector, committedView) = try recordedCommitted()
    let pendingView = try object(vector, "receiptJson")
    var digests: [Data] = []
    for view in [pendingView, committedView] {
      let server = try ReceiptReplayServer2231346(
        receipt: view, rawTransaction: text(vector, "rawTransaction"))
      defer { server.stop() }
      let client = try ProgramsClient(
        rpcURL: server.url, sender: bytes(view, "from"), receiptKeys: [pin(vector)],
        maxFeePerGas: 100, maxPriorityFeePerGas: 2, prevSeqHash: bytes(vector, "prevSeqHash"))
      let submission = try await client.activity(transactionHash: bytes(view, "txHash"))
      let verified = try XCTUnwrap(submission.sequencerReceipt)
      XCTAssertEqual(submission.state, "executed")
      XCTAssertEqual(verified.level, "receipted")
      XCTAssertEqual(verified.canonicalBytes, try bytes(vector, "signedReceipt"))
      XCTAssertEqual(verified.receiptDigest, try bytes(vector, "receiptDigest"))
      XCTAssertEqual(submission.transactionHash, try bytes(view, "txHash"))
      XCTAssertEqual(
        server.methods(),
        [
          "eth_getTransactionByHash", "pax_sequencerReceipt", "eth_getRawTransactionByHash",
        ])
      if view["rung"] == .string("committed") {
        XCTAssertEqual(verified.reportedBlockNumber, 10)
        XCTAssertEqual(verified.reportedBlockHash, try checkpoint().blockHash)
      } else {
        XCTAssertNil(verified.reportedBlockNumber)
        XCTAssertNil(verified.reportedBlockHash)
      }
      digests.append(verified.receiptDigest)
    }
    XCTAssertEqual(digests[0], digests[1])
  }

  func testReceiptPaymentDenomIndependentBindings() throws {
    let success = try XCTUnwrap(
      array(fixture("vector-v1-receipted-positive"), "cases").first?.objectValue)
    let revert = try XCTUnwrap(
      array(fixture("vector-v2-receipted-revert"), "cases").first?.objectValue)
    for vector in [success, revert] {
      let view = try object(vector, "receiptJson")
      let verified = try verify(vector)
      XCTAssertEqual(
        verified.receipt.asset, Data(SHA256.hash(data: Data(try text(view, "paymentDenom").utf8))))
      XCTAssertEqual(verified.receiptDigest, try bytes(vector, "receiptDigest"))
      for field in ["paymentDenom", "feeDenom"] {
        for bad in [
          JSONValue.string("usid"), .string(""), .string(String(repeating: "x", count: 65)), .null,
        ] {
          var changed = view
          changed[field] = bad
          XCTAssertThrowsError(try verify(vector, view: changed), field) {
            XCTAssertEqual(($0 as? PlatformSDKError)?.receiptCheck, .fields)
          }
        }
      }
      var missing = view
      missing.removeValue(forKey: "paymentDenom")
      XCTAssertThrowsError(try verify(vector, view: missing))
      if verified.status == 0 {
        XCTAssertEqual(verified.revert?.reason, "LXP_INSUFFICIENT_BALANCE: insufficient balance")
      }
    }
    // The fee-token producer fixture records the same transfer and summary with a distinct asset.
    // Preserve its signed bytes and use the companion recorded transfer's otherwise identical view.
    let feeToken = try XCTUnwrap(
      array(fixture("sequencer-receipt-fee-token-v1"), "cases").first?.objectValue)
    XCTAssertEqual(feeToken["rawTransaction"], success["rawTransaction"])
    var view = try object(success, "receiptJson")
    view["paymentDenom"] = feeToken["paymentDenom"]
    view["receipt"] = feeToken["signedReceipt"]
    let verified = try verify(feeToken, view: view)
    XCTAssertNotEqual(view["paymentDenom"], view["feeDenom"])
    XCTAssertEqual(verified.receipt.asset, try bytes(feeToken, "asset"))
    XCTAssertEqual(verified.receiptDigest, try bytes(feeToken, "receiptDigest"))
    XCTAssertEqual(verified.status, 1)
    let portable = try export(feeToken, view: view)
    XCTAssertEqual(
      try LayerXSDK.PortableReceipt.verify(portable.export(), keys: [pin(feeToken)]).receiptDigest,
      verified.receiptDigest)
  }

  func testPortableReceiptRecordedProducerRoundTrips() throws {
    let (vector, committedView) = try recordedCommitted()
    let pin = try pin(vector)
    for view in [try object(vector, "receiptJson"), committedView] {
      let portable = try export(vector, view: view)
      XCTAssertEqual(portable.verificationLevel, "receipted")
      let imported = try LayerXSDK.PortableReceipt.verify(portable.export(), keys: [pin])
      let document = try JSONDecoder().decode(Record.self, from: imported.export())
      XCTAssertEqual(document["receiptJson"], .object(view))
      XCTAssertEqual(imported.export(), portable.export())
      XCTAssertEqual(imported.receiptDigest, try bytes(vector, "receiptDigest"))
    }
    for level in ["receipted", "committed"] {
      var bundle = try published(level)
      let source: Record
      if level == "committed" {
        source = vector
        bundle["receiptJson"] = .object(committedView)
      } else {
        source = try XCTUnwrap(
          array(fixture("vector-v1-receipted-positive"), "cases").first?.objectValue)
      }
      let raw = try encode(bundle)
      let portable = try LayerXSDK.PortableReceipt.verify(
        raw, keys: [self.pin(source)], trust: checkpoint())
      XCTAssertEqual(portable.verificationLevel, level)
      XCTAssertEqual(portable.export(), raw)
      XCTAssertEqual(portable.receiptDigest, try bytes(source, "receiptDigest"))
      if level == "committed" {
        XCTAssertEqual(portable.committed?.blockHash, try checkpoint().blockHash)
        let e = try evidence(bundle)
        let exported = try LayerXSDK.PortableReceipt.export(
          rawTransaction: bytes(vector, "rawTransaction"),
          signedReceipt: bytes(vector, "signedReceipt"),
          receiptJSON: committedView, receiptKey: pin, blockHeader: e.header,
          receiptInclusion: e.inclusion,
          commit: e.commit, validatorSet: e.validators, trust: checkpoint())
        XCTAssertEqual(exported.verificationLevel, "committed")
        XCTAssertEqual(
          try JSONDecoder().decode(Record.self, from: exported.export())["receiptJson"],
          .object(committedView))
        XCTAssertThrowsError(
          try LayerXSDK.PortableReceipt.export(
            rawTransaction: bytes(vector, "rawTransaction"),
            signedReceipt: bytes(vector, "signedReceipt"),
            receiptJSON: committedView, receiptKey: pin, blockHeader: e.header,
            receiptInclusion: e.inclusion,
            commit: e.commit, validatorSet: e.validators))
      }
    }
  }

  func testReceiptMetadataAndPortableSchemaRejectTampering() throws {
    let (vector, view) = try recordedCommitted()
    var mutations: [Record] = []
    for field in ["blockNumber", "blockHash", "paymentDenom"] {
      var changed = view
      changed.removeValue(forKey: field)
      mutations.append(changed)
    }
    for (field, value) in [
      ("blockNumber", "0x9"), ("blockNumber", "0x0a"), ("blockNumber", "0x0"),
      ("blockHash", "0x00"), ("blockHash", "0x" + String(repeating: "00", count: 32)),
      ("rung", "submitted"), ("rung", "finalized"), ("rung", "receipted"),
    ] {
      var changed = view
      changed[field] = .string(value)
      mutations.append(changed)
    }
    for changed in mutations {
      XCTAssertThrowsError(try verify(vector, view: changed))
      XCTAssertThrowsError(try export(vector, view: changed))
    }
    var unknown = view
    unknown["unversionedField"] = .string("not in v2")
    XCTAssertThrowsError(try export(vector, view: unknown))
    var bundle = try published("committed")
    for (field, value) in [
      ("blockNumber", "0xb"), ("blockHash", "0x" + String(repeating: "11", count: 32)),
    ] {
      var changed = view
      changed[field] = .string(value)
      // The unsigned RPC claim alone never authenticates commitment.
      XCTAssertEqual(try verify(vector, view: changed).level, "receipted")
      bundle["receiptJson"] = .object(changed)
      XCTAssertThrowsError(
        try LayerXSDK.PortableReceipt.verify(
          encode(bundle), keys: [pin(vector)], trust: checkpoint())
      ) {
        XCTAssertEqual($0 as? CommittedError, .receipt)
      }
    }
    bundle["receiptJson"] = .object(view)
    var inclusion = try object(bundle, "receiptInclusion")
    var result = try object(inclusion, "Result")
    result["GasUsed"] = .integer(1)
    inclusion["Result"] = .object(result)
    bundle["receiptInclusion"] = .object(inclusion)
    XCTAssertThrowsError(
      try LayerXSDK.PortableReceipt.verify(encode(bundle), keys: [pin(vector)], trust: checkpoint())
    ) {
      XCTAssertEqual($0 as? CommittedError, .inclusion)
    }
  }

  func testCommittedChainTrustRejectsSelfSignedChains() throws {
    let (vector, view) = try recordedCommitted()
    var bundle = try published("committed")
    bundle["receiptJson"] = .object(view)
    let e = try evidence(bundle)
    let receipt = try verify(vector, view: view)
    let valid = try Committed.verify(
      receipt, blockHeader: e.header, receiptInclusion: e.inclusion,
      commit: e.commit, validatorSet: e.validators, trust: checkpoint())
    XCTAssertEqual(valid.height, 10)
    XCTAssertEqual(valid.blockHash, try checkpoint().blockHash)
    XCTAssertThrowsError(
      try Committed.verify(
        receipt, blockHeader: e.header, receiptInclusion: e.inclusion,
        commit: e.commit, validatorSet: e.validators)
    ) {
      XCTAssertEqual($0 as? CommittedError, .trust)
    }
    XCTAssertThrowsError(try LayerXSDK.PortableReceipt.verify(encode(bundle), keys: [pin(vector)]))
    {
      XCTAssertEqual($0 as? CommittedError, .trust)
    }
    for trust in [
      CommittedTrust(chainID: "another-chain", height: 10, blockHash: try checkpoint().blockHash),
      CommittedTrust(
        chainID: "committed-fixture", height: 11, blockHash: try checkpoint().blockHash),
      CommittedTrust(
        chainID: "committed-fixture", height: 10, blockHash: Data(repeating: 1, count: 32)),
    ] {
      XCTAssertThrowsError(
        try LayerXSDK.PortableReceipt.verify(encode(bundle), keys: [pin(vector)], trust: trust)
      ) {
        XCTAssertEqual($0 as? CommittedError, .trust)
      }
    }
    let attacker = try Curve25519.Signing.PrivateKey(
      rawRepresentation: Data(repeating: 0x77, count: 32))
    let validators = [
      CometValidator(publicKey: attacker.publicKey.rawRepresentation, votingPower: 100)
    ]
    let setHash = CommittedWire.validatorsHash(validators)
    for chainID in ["attacker-chain", "committed-fixture"] {
      var header = e.header
      header.chainID = chainID
      header.validatorsHash = setHash
      header.nextValidatorsHash = setHash
      let commit = try signedCommit(header, key: attacker)
      var inclusion = e.inclusion
      inclusion.nextHeader.chainID = chainID
      inclusion.nextHeader.validatorsHash = setHash
      inclusion.nextHeader.nextValidatorsHash = setHash
      inclusion.nextHeader.lastBlockID = commit.blockID
      inclusion.nextCommit = try signedCommit(inclusion.nextHeader, key: attacker)
      inclusion.nextValidators = validators
      // Demonstrate that both forged consensus commits are internally valid under the attacker key.
      XCTAssertEqual(
        try CommittedWire.verifyCommit(header, commit, validators, error: .commit), 100)
      XCTAssertEqual(
        try CommittedWire.verifyCommit(
          inclusion.nextHeader, inclusion.nextCommit, validators, error: .nextCommit), 100)
      XCTAssertEqual(inclusion.transaction, e.inclusion.transaction)
      XCTAssertEqual(receipt.canonicalBytes, try bytes(vector, "signedReceipt"))
      XCTAssertThrowsError(
        try Committed.verify(
          receipt, blockHeader: header, receiptInclusion: inclusion,
          commit: commit, validatorSet: validators, trust: checkpoint())
      ) {
        XCTAssertEqual($0 as? CommittedError, .trust)
      }
      let uncommittedView = try object(vector, "receiptJson")
      XCTAssertThrowsError(
        try LayerXSDK.PortableReceipt.export(
          rawTransaction: bytes(vector, "rawTransaction"),
          signedReceipt: bytes(vector, "signedReceipt"),
          receiptJSON: uncommittedView, receiptKey: pin(vector), blockHeader: header,
          receiptInclusion: inclusion,
          commit: commit, validatorSet: validators, trust: checkpoint())
      ) {
        XCTAssertEqual($0 as? CommittedError, .trust)
      }
    }
  }

  private struct Evidence {
    let header: CommittedHeader
    let inclusion: ReceiptInclusion
    let commit: CometCommit
    let validators: [CometValidator]
  }
  private func time(_ value: String) throws -> CometTimestamp {
    let formatter = ISO8601DateFormatter()
    let date = try XCTUnwrap(formatter.date(from: String(value.prefix(19)) + "Z"))
    let fraction = value.dropFirst(19).dropLast()
    let digits = fraction.first == "." ? String(fraction.dropFirst()) : ""
    let nanos = try XCTUnwrap(Int32(digits + String(repeating: "0", count: 9 - digits.count)))
    return CometTimestamp(seconds: Int64(date.timeIntervalSince1970), nanos: nanos)
  }
  private func blockID(_ value: Record) throws -> CometBlockID {
    try CometBlockID(
      hash: base64(value, "Hash"), partSetTotal: UInt32(integer(value, "PartSetTotal")),
      partSetHash: base64(value, "PartSetHash"))
  }
  private func header(_ value: Record) throws -> CommittedHeader {
    try CommittedHeader(
      versionBlock: UInt64(integer(value, "VersionBlock")),
      versionApp: UInt64(integer(value, "VersionApp")),
      chainID: text(value, "ChainID"), height: integer(value, "Height"),
      time: time(text(value, "Time")),
      lastBlockID: blockID(object(value, "LastBlockID")),
      lastCommitHash: base64(value, "LastCommitHash"),
      dataHash: base64(value, "DataHash"), validatorsHash: base64(value, "ValidatorsHash"),
      nextValidatorsHash: base64(value, "NextValidatorsHash"),
      consensusHash: base64(value, "ConsensusHash"),
      appHash: base64(value, "AppHash"), lastResultsHash: base64(value, "LastResultsHash"),
      evidenceHash: base64(value, "EvidenceHash"), proposerAddress: base64(value, "ProposerAddress")
    )
  }
  private func validators(_ values: [JSONValue]) throws -> [CometValidator] {
    try values.map {
      let value = try XCTUnwrap($0.objectValue)
      let publicKey = try array(value, "PublicKey").map {
        try XCTUnwrap($0.integerValue).flatMapByte()
      }
      return try CometValidator(
        publicKey: Data(publicKey), votingPower: integer(value, "VotingPower"))
    }
  }
  private func commit(_ value: Record) throws -> CometCommit {
    let signatures = try array(value, "Signatures").map { item -> CometCommitSignature in
      let signature = try XCTUnwrap(item.objectValue)
      return try CometCommitSignature(
        blockIDFlag: XCTUnwrap(
          CometBlockIDFlag(rawValue: UInt8(integer(signature, "BlockIDFlag")))),
        validatorAddress: base64(signature, "ValidatorAddress"),
        timestamp: time(text(signature, "Timestamp")),
        signature: base64(signature, "Signature"))
    }
    return try CometCommit(
      height: integer(value, "Height"), round: Int32(integer(value, "Round")),
      blockID: blockID(object(value, "BlockID")), signatures: signatures)
  }
  private func proof(_ value: Record) throws -> CommittedProof {
    let aunts = try array(value, "Aunts").map { value -> Data in
      guard case let .array(numbers) = value else { throw NSError(domain: "Proof", code: 1) }
      return try Data(numbers.map { try XCTUnwrap($0.integerValue).flatMapByte() })
    }
    return try CommittedProof(
      total: integer(value, "Total"), index: integer(value, "Index"), aunts: aunts)
  }
  private func evidence(_ bundle: Record) throws -> Evidence {
    let i = try object(bundle, "receiptInclusion")
    let r = try object(i, "Result")
    return try Evidence(
      header: header(object(bundle, "blockHeader")),
      inclusion: ReceiptInclusion(
        transaction: base64(i, "Transaction"),
        transactionProof: proof(object(i, "TransactionProof")),
        result: ExecTxResult(
          code: UInt32(integer(r, "Code")), data: base64(r, "Data"),
          gasWanted: integer(r, "GasWanted"), gasUsed: integer(r, "GasUsed")),
        resultProof: proof(object(i, "ResultProof")),
        batchTransaction: base64(i, "BatchTransaction"), batchProof: proof(object(i, "BatchProof")),
        nextHeader: header(object(i, "NextHeader")), nextCommit: commit(object(i, "NextCommit")),
        nextValidators: validators(array(i, "NextValidators"))),
      commit: commit(object(bundle, "commit")),
      validators: validators(array(bundle, "validatorSet")))
  }
  private func signedCommit(_ header: CommittedHeader, key: Curve25519.Signing.PrivateKey) throws
    -> CometCommit
  {
    var commit = CometCommit(
      height: header.height, round: 0,
      blockID: CometBlockID(
        hash: header.hash, partSetTotal: 1, partSetHash: Data(repeating: 1, count: 32)),
      signatures: [])
    var signature = CometCommitSignature(
      blockIDFlag: .block,
      validatorAddress: Data(CommittedWire.sha(key.publicKey.rawRepresentation).prefix(20)),
      timestamp: header.time)
    signature.signature = try key.signature(
      for: CommittedWire.voteBytes(header.chainID, commit, signature))
    commit.signatures = [signature]
    return commit
  }
}

extension Int64 {
  fileprivate func flatMapByte() throws -> UInt8 { try XCTUnwrap(UInt8(exactly: self)) }
}

private final class ReceiptReplayServer2231346: @unchecked Sendable {
  let url: URL
  private let descriptor: Int32
  private let lock = NSLock()
  private var observed: [String] = []
  private var stopped = false

  init(receipt: [String: JSONValue], rawTransaction: String) throws {
    #if os(Linux)
      let fd = socket(AF_INET, Int32(SOCK_STREAM.rawValue), 0)
    #else
      let fd = socket(AF_INET, SOCK_STREAM, 0)
    #endif
    guard fd >= 0 else { throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno)) }
    var address = sockaddr_in()
    address.sin_family = sa_family_t(AF_INET)
    address.sin_addr.s_addr = inet_addr("127.0.0.1")
    let bound = withUnsafePointer(to: &address) {
      $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
        bind(fd, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
      }
    }
    guard bound == 0, listen(fd, 4) == 0 else {
      close(fd)
      throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno))
    }
    var size = socklen_t(MemoryLayout<sockaddr_in>.size)
    let named = withUnsafeMutablePointer(to: &address) {
      $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { getsockname(fd, $0, &size) }
    }
    guard named == 0,
      let url = URL(string: "http://127.0.0.1:\(UInt16(bigEndian: address.sin_port))")
    else {
      close(fd)
      throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno))
    }
    self.descriptor = fd
    self.url = url
    DispatchQueue.global().async { [self] in
      for _ in 0..<3 {
        let client = accept(fd, nil, nil)
        if client < 0 { return }
        defer { close(client) }
        do {
          let request = try Self.readRequest(client)
          let method = try XCTUnwrap(request["method"]?.stringValue)
          lock.lock()
          observed.append(method)
          lock.unlock()
          let result: JSONValue
          switch method {
          case "eth_getTransactionByHash":
            result = .object(["hash": try XCTUnwrap(receipt["txHash"])])
          case "pax_sequencerReceipt": result = .object(receipt)
          case "eth_getRawTransactionByHash": result = .string(rawTransaction)
          default: throw NSError(domain: "UnexpectedRPC", code: 1)
          }
          XCTAssertEqual(request["params"], .array([try XCTUnwrap(receipt["txHash"])]))
          let body = try JSONEncoder().encode(
            JSONValue.object([
              "jsonrpc": .string("2.0"), "id": try XCTUnwrap(request["id"]), "result": result,
            ]))
          let response =
            Data(
              "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: \(body.count)\r\nConnection: close\r\n\r\n"
                .utf8) + body
          var sent = 0
          try response.withUnsafeBytes { pointer in
            while sent < response.count {
              #if os(Linux)
                let count = send(
                  client, pointer.baseAddress!.advanced(by: sent), response.count - sent,
                  Int32(MSG_NOSIGNAL))
              #else
                let count = send(
                  client, pointer.baseAddress!.advanced(by: sent), response.count - sent, 0)
              #endif
              guard count > 0 else { throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno)) }
              sent += count
            }
          }
        } catch {
          XCTFail("Receipt transcript replay failed: \(error)")
          return
        }
      }
    }
  }
  func methods() -> [String] {
    lock.lock()
    defer { lock.unlock() }
    return observed
  }
  func stop() {
    lock.lock()
    defer { lock.unlock() }
    if !stopped {
      stopped = true
      _ = shutdown(descriptor, Int32(SHUT_RDWR))
      close(descriptor)
    }
  }
  private static func readRequest(_ fd: Int32) throws -> [String: JSONValue] {
    var data = Data()
    var buffer = [UInt8](repeating: 0, count: 4096)
    var timeout = timeval(tv_sec: 10, tv_usec: 0)
    _ = setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))
    while data.count <= 1_048_576 {
      let count = recv(fd, &buffer, buffer.count, 0)
      guard count > 0 else { throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno)) }
      data.append(contentsOf: buffer.prefix(count))
      if let end = data.range(of: Data("\r\n\r\n".utf8)) {
        let header = String(decoding: data[..<end.lowerBound], as: UTF8.self)
        let lengthLine = try XCTUnwrap(
          header.components(separatedBy: "\r\n").first {
            $0.lowercased().hasPrefix("content-length:")
          })
        let length = try XCTUnwrap(
          Int(
            lengthLine.split(separator: ":", maxSplits: 1)[1].trimmingCharacters(in: .whitespaces)))
        if data.count - end.upperBound >= length {
          return try JSONDecoder().decode(
            [String: JSONValue].self, from: Data(data[end.upperBound..<(end.upperBound + length)]))
        }
      }
    }
    throw NSError(domain: "ReceiptReplayLimit", code: 1)
  }
}
