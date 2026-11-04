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

// Task 22.33.1398 repair record (audit 22.30.78, acceptance 22998.1):
// committed-test-trust -> testCommittedTestTrust
// portable-test-producer-shape -> testPortableTestProducerShape
// program-pending-test -> testProgramPending
// program-old-status-test -> testProgramCurrentStatusAndDiscovery
// program-abi-five-test -> testProgramABIFive
// program-evm-test-producer -> testProgramEVMProducer
// No baseline gate ran: the explicit gate order requires the complete implementation first.
// The exact qualified revision, command, exit and private log are recorded by hf-done.
// First qualification: 25cb2220d1104fa756d614b52c35e35e41ea4072, hf-done exit 1,
// /out/logs/verify-1.log: ABI5 and registry tests incorrectly framed multiple ABI tails.
// Gotcha: ProgramABI.dynamic validates one final tail; multi-tail test callers must
// frame each ordered tail exactly and assert total consumption, never relax the decoder.
// Wiring request: sdk/platform/conformance/fixtures (platform conformance domain 8) must
// publish an ABI-5 deployment/upgrade and registry RPC recording from the registered v5 host
// exercised in tests/programs/test_programs_abi5_admission.c (programs runtime domain 13).
// This task proves the Swift registration transaction carrying an interface, not host execution.
// Wiring request: programs/sdk/rust/src/bindgen.rs and programs/crates/layerx-programs-registry/src/interface.rs
// (programs runtime domain 13) must supply the ABI-5 interface admission/recording; the
// committed interface policy still admits only ABI 1...4. SDK interface bytes are opaque.
// Wiring request: rpc/sequencer_receipt_vectors_test.go (node RPC domain 5) must record the
// fee-token vector's complete receiptJson; its signed receipt and denominations are retained
// here with the companion transfer's unchanged summary, as in Repair2231346Tests.
final class Repair22331398Tests: XCTestCase {
  func testCommittedTestTrust() throws {
    try CommittedReceiptTests.proveRecordedCommitTrust1398()
  }
  func testPortableTestProducerShape() throws {
    try PortableReceiptTests.proveRecordedPortableShapes1398()
    // Published bytes and independent receipt-key refusals remain part of the focused gate.
    try PortableReceiptTests.provePublishedPortableBytes1398()
  }
  func testProgramPending() async throws {
    try await ProgramsContractTests.proveRetainedPending1398()
  }
  func testProgramCurrentStatusAndDiscovery() async throws {
    try ProgramsContractTests.proveCurrentDaemonStatuses1398()
    try await ProgramsContractTests.proveCurrentRegistry1398()
  }
  func testProgramABIFive() async throws {
    try await ProgramsContractTests.proveABIFiveRegistration1398()
  }
  func testProgramEVMProducer() async throws {
    try await ProgramsEvmTests.proveRecordedEVMProducer1398()
  }
}

// These helpers decode committed producer fixtures without replacing or signing their bytes.
enum RecordedProgramEvidence1398 {
  typealias Record = [String: JSONValue]
  static var root: URL {
    var url = URL(fileURLWithPath: #filePath)
    for _ in 0..<6 { url.deleteLastPathComponent() }
    return url
  }
  static func read(_ path: String) throws -> Record {
    try JSONDecoder().decode(Record.self, from: Data(contentsOf: root.appendingPathComponent(path)))
  }
  static func fixture(_ name: String) throws -> Record {
    try read("sdk/platform/conformance/fixtures/\(name).json")
  }
  static func object(_ record: Record, _ field: String) throws -> Record {
    try XCTUnwrap(record[field]?.objectValue, field)
  }
  static func text(_ record: Record, _ field: String) throws -> String {
    try XCTUnwrap(record[field]?.stringValue, field)
  }
  static func integer(_ record: Record, _ field: String) throws -> Int64 {
    try XCTUnwrap(record[field]?.integerValue, field)
  }
  static func array(_ record: Record, _ field: String) throws -> [JSONValue] {
    guard case let .array(values) = record[field] else {
      throw NSError(domain: "RecordedReceipt", code: 1)
    }
    return values
  }
  static func hex(_ text: String) throws -> Data {
    try ProgramABI.hex(text)
  }
  static func bytes(_ record: Record, _ field: String) throws -> Data {
    try hex(text(record, field))
  }
  static func base64(_ record: Record, _ field: String) throws -> Data {
    try XCTUnwrap(Data(base64Encoded: text(record, field)))
  }
  static func encode(_ record: Record) throws -> Data {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
    return try encoder.encode(record)
  }
  static func pin(_ vector: Record) throws -> ReceiptKeys {
    let value = try object(vector, "pinnedKey")
    return try ReceiptKeys(
      keyId: bytes(value, "keyId"), publicKey: bytes(value, "publicKey"),
      firstHeight: UInt64(integer(value, "firstHeight")),
      lastHeight: UInt64(integer(value, "lastHeight")))
  }
  static func verify(_ vector: Record, view: Record? = nil) throws -> SequencerReceiptVerification {
    try LocalVerifier.verifySequencerReceipt(
      bytes(vector, "signedReceipt"), rawTransaction: bytes(vector, "rawTransaction"),
      keys: [pin(vector)], receiptJSON: view ?? object(vector, "receiptJson"),
      prevSeqHash: bytes(vector, "prevSeqHash"))
  }
  static func recordedCommitted() throws -> (Record, Record) {
    let vector = try object(fixture("vector-v4-committed"), "positive")
    let source = try read("daemon/committed-fixture/vectors/vector-v4-committed.json")
    let view = try object(object(source, "settlement"), "receipt")
    XCTAssertEqual(view["receipt"], vector["signedReceipt"])
    XCTAssertEqual(view["rung"], .string("committed"))
    return (vector, view)
  }
  static func checkpoint() throws -> CommittedTrust {
    // Independently pinned execution block from the recorded local chain, never from bundle input.
    try CommittedTrust(
      chainID: "committed-fixture", height: 10,
      blockHash: hex("0x066b3b86eb6f62437b659c1d075f28eb22246d233fe86b3d5cacbdb5e18216e9"))
  }
  static func published(_ level: String) throws -> Record {
    let values = try array(fixture("vector-v5-portable"), "bundles")
    return try XCTUnwrap(
      values.compactMap(\.objectValue).first {
        $0["verificationLevel"] == .string(level)
      })
  }

  struct Evidence {
    let header: CommittedHeader
    let inclusion: ReceiptInclusion
    let commit: CometCommit
    let validators: [CometValidator]
  }
  static func time(_ value: String) throws -> CometTimestamp {
    let formatter = ISO8601DateFormatter()
    let date = try XCTUnwrap(formatter.date(from: String(value.prefix(19)) + "Z"))
    let fraction = value.dropFirst(19).dropLast()
    let digits = fraction.first == "." ? String(fraction.dropFirst()) : ""
    let nanos = try XCTUnwrap(Int32(digits + String(repeating: "0", count: 9 - digits.count)))
    return CometTimestamp(seconds: Int64(date.timeIntervalSince1970), nanos: nanos)
  }
  static func blockID(_ value: Record) throws -> CometBlockID {
    try CometBlockID(
      hash: base64(value, "Hash"), partSetTotal: UInt32(integer(value, "PartSetTotal")),
      partSetHash: base64(value, "PartSetHash"))
  }
  static func header(_ value: Record) throws -> CommittedHeader {
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
  static func validators(_ values: [JSONValue]) throws -> [CometValidator] {
    try values.map {
      let value = try XCTUnwrap($0.objectValue)
      let publicKey = try array(value, "PublicKey").map {
        try XCTUnwrap(UInt8(exactly: XCTUnwrap($0.integerValue)))
      }
      return try CometValidator(
        publicKey: Data(publicKey), votingPower: integer(value, "VotingPower"))
    }
  }
  static func commit(_ value: Record) throws -> CometCommit {
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
  static func proof(_ value: Record) throws -> CommittedProof {
    let aunts = try array(value, "Aunts").map { value -> Data in
      guard case let .array(numbers) = value else { throw NSError(domain: "Proof", code: 1) }
      return try Data(numbers.map { try XCTUnwrap(UInt8(exactly: XCTUnwrap($0.integerValue))) })
    }
    return try CommittedProof(
      total: integer(value, "Total"), index: integer(value, "Index"), aunts: aunts)
  }
  static func evidence(_ bundle: Record) throws -> Evidence {
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
  static func signedCommit(_ header: CommittedHeader, key: Curve25519.Signing.PrivateKey) throws
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

  static func firstVector(_ name: String) throws -> Record {
    try XCTUnwrap(array(fixture(name), "cases").first?.objectValue)
  }
  static func client(
    _ server: ProgramReplayServer1398, vector: Record? = nil,
    keys: [ReceiptKeys]? = nil
  ) throws -> ProgramsClient {
    try ProgramsClient(
      rpcURL: server.url, sender: Data(repeating: 0x55, count: 20),
      receiptKeys: keys ?? (try vector.map { try [pin($0)] } ?? []),
      maxFeePerGas: 100, maxPriorityFeePerGas: 2,
      prevSeqHash: try vector.map { try bytes($0, "prevSeqHash") } ?? Data(repeating: 0, count: 32))
  }
  static func rpcSteps(_ vector: Record, view: Record, raw: String? = nil)
    throws -> [ProgramReplayServer1398.Exchange]
  {
    let hash = try text(view, "txHash")
    return [
      .init("eth_getTransactionByHash", [.string(hash)], .object(["hash": .string(hash)])),
      .init("pax_sequencerReceipt", [.string(hash)], .object(view)),
      .init(
        "eth_getRawTransactionByHash", [.string(hash)],
        .string(try raw ?? text(vector, "rawTransaction"))),
    ]
  }
  static func feeToken() throws -> (Record, Record) {
    let vector = try firstVector("sequencer-receipt-fee-token-v1")
    let companion = try firstVector("vector-v1-receipted-positive")
    XCTAssertEqual(vector["rawTransaction"], companion["rawTransaction"])
    var view = try object(companion, "receiptJson")
    view["paymentDenom"] = vector["paymentDenom"]
    view["receipt"] = vector["signedReceipt"]
    return (vector, view)
  }
  static func wireHex(_ data: Data) -> String {
    "0x" + data.map { String(format: "%02x", $0) }.joined()
  }
}

// A bounded, in-process HTTP transcript peer. Production URLSession, ProgramRPC,
// ProgramsClient and the verifiers remain on the exercised path.
final class ProgramReplayServer1398: @unchecked Sendable {
  struct Exchange: Sendable {
    let method: String
    let params: [JSONValue]
    let result: JSONValue
    init(_ method: String, _ params: [JSONValue], _ result: JSONValue) {
      self.method = method
      self.params = params
      self.result = result
    }
  }
  let url: URL
  private let descriptor: Int32
  private let lock = NSLock()
  private let worker = DispatchGroup()
  private var observed: [[String: JSONValue]] = []
  private var failures: [String] = []
  private var stopped = false

  init(_ exchanges: [Exchange]) throws {
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
    guard bound == 0, listen(fd, 8) == 0 else {
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
    worker.enter()
    DispatchQueue.global().async { [self] in
      defer { worker.leave() }
      var index = 0
      while true {
        let connection = accept(fd, nil, nil)
        if connection < 0 { return }
        defer { close(connection) }
        do {
          let request = try Self.readRequest(connection)
          lock.lock()
          observed.append(request)
          lock.unlock()
          guard exchanges.indices.contains(index) else {
            throw NSError(domain: "UnexpectedRPCExchange", code: index)
          }
          let exchange = exchanges[index]
          index += 1
          guard request["method"] == .string(exchange.method),
            request["params"] == .array(exchange.params)
          else {
            throw NSError(domain: "RPCTranscriptMismatch", code: index)
          }
          let body = try JSONEncoder().encode(
            JSONValue.object([
              "jsonrpc": .string("2.0"), "id": try XCTUnwrap(request["id"]),
              "result": exchange.result,
            ]))
          let response =
            Data(
              "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: \(body.count)\r\nConnection: close\r\n\r\n"
                .utf8) + body
          try response.withUnsafeBytes { pointer in
            var sent = 0
            while sent < response.count {
              #if os(Linux)
                let count = send(
                  connection, pointer.baseAddress!.advanced(by: sent),
                  response.count - sent, Int32(MSG_NOSIGNAL))
              #else
                let count = send(
                  connection, pointer.baseAddress!.advanced(by: sent),
                  response.count - sent, 0)
              #endif
              guard count > 0 else { throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno)) }
              sent += count
            }
          }
        } catch {
          lock.lock()
          failures.append(String(describing: error))
          lock.unlock()
          return
        }
      }
    }
  }
  func requests() -> [[String: JSONValue]] {
    lock.lock()
    defer { lock.unlock() }
    return observed
  }
  func assertConsumed(_ count: Int, file: StaticString = #filePath, line: UInt = #line) {
    lock.lock()
    defer { lock.unlock() }
    XCTAssertEqual(observed.count, count, file: file, line: line)
    XCTAssertTrue(failures.isEmpty, failures.joined(separator: "; "), file: file, line: line)
  }
  func stop() {
    lock.lock()
    let shouldStop = !stopped
    stopped = true
    lock.unlock()
    if shouldStop {
      _ = shutdown(descriptor, Int32(SHUT_RDWR))
      close(descriptor)
      XCTAssertEqual(worker.wait(timeout: .now() + 15), .success)
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
            lengthLine.split(separator: ":", maxSplits: 1)[1]
              .trimmingCharacters(in: .whitespaces)))
        guard length >= 0, length <= 1_048_576 else { throw NativeProgramCallError.invalid }
        if data.count - end.upperBound >= length {
          return try JSONDecoder().decode(
            [String: JSONValue].self,
            from: Data(data[end.upperBound..<(end.upperBound + length)]))
        }
      }
    }
    throw NSError(domain: "RPCReplayLimit", code: 1)
  }
}
