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

// Task 22.33.1397 repair record:
// deploy-payload-limit: testDeployPayloadLimitIncludesEnvelopeAndInterface
// program-rpc-response-bound: testRPCBoundsActualHTTPBodyAndCancelsOverflow
// programs-sequence-recovery: testSequenceRecoveryReopensAuthenticatedTransactionHistory
// programs-inert-constructor: testRetiredConstructorRefusesAtConstruction; testLifecycleTransactionUsesCurrentReceipts
// stream-memory-and-pacing: testSlowConsumerOverflowResumesWithoutLoss; testReplayWindowAndEmptyPagePacing; testHumanStreamRecordedHTTPShape
// legacy-lifecycle-verification: testLegacyLifecycleVerifierRefusesRecordedKernelReceipt; testLifecycleTransactionUsesCurrentReceipts
// Attempt 1 verification failed in testRPCBoundsActualHTTPBodyAndCancelsOverflow
// and the lifecycle receipt fixture; its lifecycle-view and cancellation corrections
// are retained here for the continuation's build and verification on one revision.
// Wiring request: sdk/platform/swift/Tests/LayerXSDKTests/ProgramsContractTests.swift and
// ProgramTransportRetiredTests.swift (platform SDK owner) must migrate the retired constructor
// to rpcURL, sender and receiptKeys; it now explicitly refuses construction, with a migration diagnostic.
// The lifecycle transcript below retains the signed EVM deploy and signed sequencer receipt
// already committed in ProgramsEvmTests.testProgramLifecycleReceiptResolvesBySignedTransactionHash.
// No sequencer receipt is generated, re-encoded or signed by this test.
// The current RPC view includes paymentDenom=uhpx, authenticated by the retained
// receipt's asset commitment. Both this transcript and the retained ProgramsEvmTests
// deploy RPC view include that denomination.
final class Repair22331397Tests: XCTestCase {
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
  private func text(_ record: Record, _ key: String) throws -> String {
    try XCTUnwrap(record[key]?.stringValue, key)
  }
  private func object(_ record: Record, _ key: String) throws -> Record {
    try XCTUnwrap(record[key]?.objectValue, key)
  }
  private func bytes(_ record: Record, _ key: String) throws -> Data {
    try ProgramABI.hex(text(record, key))
  }
  private func vectors() throws -> [Record] {
    guard case .array(let values)? = try fixture("sequencer-receipt-v1")["cases"] else {
      throw NativeProgramCallError.invalid
    }
    return try values.prefix(6).map { try XCTUnwrap($0.objectValue) }
  }
  private func pin(_ record: Record) throws -> ReceiptKeys {
    let key = try object(record, "pinnedKey")
    return try ReceiptKeys(
      keyId: bytes(key, "keyId"), publicKey: bytes(key, "publicKey"),
      firstHeight: UInt64(XCTUnwrap(key["firstHeight"]?.integerValue)),
      lastHeight: UInt64(XCTUnwrap(key["lastHeight"]?.integerValue)))
  }
  private func client(
    _ server: RepairHTTPServer22331397, receiptKeys: [ReceiptKeys] = [],
    prevSeqHash: Data = Data(repeating: 0, count: 32),
    recovery: ProgramSequenceRecovery? = nil
  ) throws -> ProgramsClient {
    try ProgramsClient(
      rpcURL: server.url, sender: Data(repeating: 0x55, count: 20), receiptKeys: receiptKeys,
      maxFeePerGas: 100, maxPriorityFeePerGas: 2,
      prevSeqHash: prevSeqHash, sequenceRecovery: recovery)
  }

  func testDeployPayloadLimitIncludesEnvelopeAndInterface() throws {
    let record = try fixture("native-program-deploy-v4")
    let recorded = try nativeTransportHex(text(record, "payload_hex"))
    XCTAssertEqual(try NativeProgramDeploy.decode(recorded).encode(), recorded)
    let program = Data(recorded.prefix(32))
    let authority = recorded.subdata(in: 36..<68)
    for interfaceLength in [0, 1, 952] {
      let interface: Data? =
        interfaceLength == 0 ? nil : Data(repeating: 0x42, count: interfaceLength)
      let overhead = 104 + (interface == nil ? 0 : 4 + interfaceLength)
      func deploy(_ length: Int, abi: UInt16 = 2, corruptHash: Bool = false) throws
        -> NativeProgramDeploy
      {
        var wasm = Data([0, 97, 115, 109, 1, 0, 0, 0])
        wasm.append(Data(repeating: 0, count: length - wasm.count))
        var hash = Data(SHA256.hash(data: wasm))
        if corruptHash { hash[0] ^= 1 }
        return try NativeProgramDeploy(
          programID: program, guestABI: abi, policy: 1, authority: authority,
          newHash: hash, programInterface: interface, wasm: wasm)
      }
      let boundary = try deploy(524_288 - overhead)
      XCTAssertEqual(boundary.encode().count, 524_288)
      XCTAssertEqual(try NativeProgramDeploy.decode(boundary.encode()).encode(), boundary.encode())
      XCTAssertNoThrow(try NativeProgramLifecycleRequest.calldata(operation: boundary))
      XCTAssertThrowsError(try deploy(524_289 - overhead))
      XCTAssertThrowsError(try deploy(524_288 - overhead, abi: UInt16.max))
      XCTAssertThrowsError(try deploy(524_288 - overhead, corruptHash: true))
      var firstRejected = boundary.encode()
      firstRejected.append(0)
      XCTAssertThrowsError(try NativeProgramDeploy.decode(firstRejected))
    }
  }

  func testRPCBoundsActualHTTPBodyAndCancelsOverflow() async throws {
    let limit = 8 * 1024 * 1024
    let value = try JSONEncoder().encode(
      JSONValue.object([
        "jsonrpc": .string("2.0"), "id": .integer(1), "result": .string("0x7"),
      ]))
    let atLimit = value + Data(repeating: 32, count: limit - value.count)
    let exact = try RepairHTTPServer22331397 { _ in .body(atLimit) }
    defer { exact.stop() }
    let accepted = try await ProgramRPC(url: exact.url, session: .shared).quantity(
      "eth_blockNumber", [])
    XCTAssertEqual(accepted, 7)
    for declared in [true, false] {
      let cancelled = expectation(description: "oversized HTTP connection cancelled \(declared)")
      let server = try RepairHTTPServer22331397 { _ in
        .oversized(declaredLength: declared, limit: limit, cancelled: cancelled)
      }
      defer { server.stop() }
      do {
        _ = try await ProgramRPC(url: server.url, session: .shared).call("pax_sequencerReceipt", [])
        XCTFail("Accepted an oversized actual HTTP response")
      } catch let error as PlatformSDKError {
        XCTAssertEqual(error.code, .transportFailure)
        XCTAssertEqual(error.retry, .safe)
      }
      await fulfillment(of: [cancelled], timeout: 10)
    }
    let malformed = try RepairHTTPServer22331397 { _ in .body(Data("{".utf8)) }
    defer { malformed.stop() }
    do {
      _ = try await ProgramRPC(url: malformed.url, session: .shared).call("eth_blockNumber", [])
      XCTFail("Accepted malformed JSON")
    } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .decodeFailure) }
  }

  func testSequenceRecoveryReopensAuthenticatedTransactionHistory() async throws {
    let records = try vectors()
    XCTAssertEqual(records.count, 6)
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let path = directory.appendingPathComponent("authenticated-history.json")
    var history: [String: ProgramSequenceContext] = [:]
    var byHash: [String: Record] = [:]
    for (index, vector) in records.enumerated() {
      let view = try object(vector, "receiptJson")
      let hash = try text(view, "txHash")
      history[hash] = try ProgramSequenceContext(
        transactionHash: ProgramABI.hex(hash), sequence: UInt64(index + 1),
        previousSequenceHash: bytes(vector, "prevSeqHash"))
      byHash[hash] = vector
      if index > 0 {
        let predecessor = try LocalVerifier.verifySequencerReceipt(
          bytes(records[index - 1], "signedReceipt"),
          rawTransaction: bytes(records[index - 1], "rawTransaction"),
          keys: [pin(records[index - 1])], receiptJSON: object(records[index - 1], "receiptJson"),
          prevSeqHash: bytes(records[index - 1], "prevSeqHash"))
        XCTAssertEqual(predecessor.receipt.resultingStateRoot, history[hash]?.previousSequenceHash)
      }
    }
    try JSONEncoder().encode(history).write(to: path, options: .atomic)
    let transcript = byHash
    let server = try RepairHTTPServer22331397 { request in
      guard let method = request.json?["method"]?.stringValue,
        case .array(let params)? = request.json?["params"],
        let hash = params.first?.stringValue, let vector = transcript[hash]
      else { throw NativeProgramCallError.invalid }
      let result: JSONValue
      switch method {
      case "eth_getTransactionByHash": result = .object(["hash": .string(hash)])
      case "pax_sequencerReceipt": result = .object(try self.object(vector, "receiptJson"))
      case "eth_getRawTransactionByHash": result = try XCTUnwrap(vector["rawTransaction"])
      default: throw NativeProgramCallError.invalid
      }
      return try .rpc(result)
    }
    defer { server.stop() }
    let keys = try records.map(pin)
    func reopen() throws -> ProgramsClient {
      // Reload independently authenticated history, not a predecessor read from RPC.
      let recovered = try JSONDecoder().decode(
        [String: ProgramSequenceContext].self, from: Data(contentsOf: path))
      return try client(
        server, receiptKeys: keys,
        recovery: { hash in
          let id = "0x" + hash.map { String(format: "%02x", $0) }.joined()
          guard let context = recovered[id] else { throw NativeProgramCallError.invalid }
          return context
        })
    }
    var first: ProgramsClient? = try reopen()
    for index in [0, 1, 4] {
      let hash = try bytes(object(records[index], "receiptJson"), "txHash")
      let submission = try await XCTUnwrap(first).activity(transactionHash: hash)
      XCTAssertEqual(submission.sequencerReceipt?.receipt.globalSequence, UInt64(index + 1))
      XCTAssertEqual(submission.state, "executed")
    }
    first = nil
    let restarted = try reopen()
    for index in [2, 5, 3, 1] {
      let hash = try bytes(object(records[index], "receiptJson"), "txHash")
      let submission = try await restarted.activity(transactionHash: hash)
      XCTAssertEqual(submission.sequencerReceipt?.receipt.globalSequence, UInt64(index + 1))
      XCTAssertEqual(submission.state, index == 5 ? "refused" : "executed")
    }
    let vector = records[2]
    let hash = try bytes(object(vector, "receiptJson"), "txHash")
    for context in [
      try ProgramSequenceContext(
        transactionHash: hash, sequence: 3, previousSequenceHash: Data(repeating: 0, count: 32)),
      try ProgramSequenceContext(
        transactionHash: hash, sequence: 4, previousSequenceHash: bytes(vector, "prevSeqHash")),
      try ProgramSequenceContext(
        transactionHash: Data(repeating: 1, count: 32), sequence: 3,
        previousSequenceHash: bytes(vector, "prevSeqHash")),
    ] {
      let invalid = try client(server, receiptKeys: keys, recovery: { _ in context })
      do {
        _ = try await invalid.activity(transactionHash: hash)
        XCTFail("Accepted unauthenticated transaction sequence context")
      } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .verificationFailure) }
    }
  }

  func testRetiredConstructorRefusesAtConstruction() throws {
    let server = try RepairHTTPServer22331397 { _ in throw NativeProgramCallError.invalid }
    defer { server.stop() }
    let platform = PlatformClient(transport: try HumanHTTPTransport(baseURL: server.url))
    XCTAssertThrowsError(
      try ProgramsClient(client: platform, sequencerPublicKey: Data(repeating: 1, count: 32))
    ) {
      XCTAssertEqual(($0 as? PlatformSDKError)?.code, .protocolIncompatibility)
      XCTAssertEqual(($0 as? PlatformSDKError)?.retry, .never)
    }
    XCTAssertTrue(server.requests().isEmpty)
  }

  func testLegacyLifecycleVerifierRefusesRecordedKernelReceipt() throws {
    let record = try read(
      "services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json")
    let canonical = try nativeTransportHex(text(record, "receipt_hex"))
    let key = try nativeTransportHex(text(record, "sequencer_public_key_hex"))
    let activityID = canonical.subdata(in: 10..<42)
    let digest = Data(
      SHA256.hash(data: Data("LXP/v1/receipt\0".utf8) + canonical.dropLast(69) + Data([0])))
    XCTAssertTrue(
      try Curve25519.Signing.PublicKey(rawRepresentation: key).isValidSignature(
        canonical.suffix(64), for: digest))
    XCTAssertThrowsError(
      try LocalVerifier.verifyProgramLifecycleReceipt(
        canonical, expectedActivity: activityID, sequencer: key)
    ) {
      XCTAssertEqual(($0 as? PlatformSDKError)?.code, .protocolIncompatibility)
      XCTAssertEqual(($0 as? PlatformSDKError)?.retry, .never)
    }
    XCTAssertThrowsError(
      try LocalVerifier.verifySequencerReceipt(
        canonical, rawTransaction: Data(), keys: [], receiptJSON: [:],
        prevSeqHash: Data(repeating: 0, count: 32))
    ) {
      XCTAssertEqual(($0 as? PlatformSDKError)?.receiptCheck, .decode)
    }
  }
  func testLifecycleTransactionUsesCurrentReceipts() async throws {
    let vector = try fixture("native-program-deploy-v4")
    let operation = try NativeProgramDeploy.decode(nativeTransportHex(text(vector, "payload_hex")))
    let lifecycle = try NativeProgramLifecycleRequest(
      operation: operation, signedActivity: nativeTransportHex(text(vector, "signed_activity_hex")))
    let buildServer = try RepairHTTPServer22331397()
    defer { buildServer.stop() }
    let transaction = try await client(buildServer).deploy(
      operation, signedActivity: lifecycle.signedActivity,
      idempotencyKey: IdempotencyKey(text(vector, "idempotency_key_hex")))
    XCTAssertEqual(
      transaction.calldata, try ProgramABI.hex(text(object(vector, "evm"), "calldata")))
    XCTAssertEqual(transaction.chainID, 125)
    XCTAssertEqual(transaction.nonce, 7)
    XCTAssertEqual(transaction.gas, 100_000)
    XCTAssertEqual(buildServer.requests().count, 2)

    // EIP-1559 signature of this fixture deploy, using the conformance sender key.
    let signature = try ProgramABI.hex(
      "0x00c86d1e2d4442198de61fff30e6aacaa6dfc62825a3f55e5a56a757281ce214b779c855bfdb3362162ca58e1c7363a3"
        + "784488e131af02502fe0ca0c5df549e207")
    let fields = try decodeRLPList(Data(transaction.signingPayload().dropFirst()))
    var signedBody = fields.prefix(8).reduce(into: Data()) { $0.append(ProgramABI.rlp($1)) }
    signedBody.append(0xc0)
    signedBody.append(ProgramABI.rlp(Data(signature.prefix(1).drop(while: { $0 == 0 }))))
    signedBody.append(ProgramABI.rlp(Data(signature[1..<33]).drop(while: { $0 == 0 })))
    signedBody.append(ProgramABI.rlp(Data(signature[33..<65]).drop(while: { $0 == 0 })))
    let signedTransaction = Data([2]) + ProgramABI.rlp(signedBody, list: true)
    let hash = CommittedWire.keccak256(signedTransaction)
    let hashText = "0x" + hash.map { String(format: "%02x", $0) }.joined()
    XCTAssertEqual(hashText, "0xe827f22f1857d2d443a5b88b414201cb2bfab1e17974072c306d82412ffce6e6")
    XCTAssertNotEqual(hash, lifecycle.activityID)

    // Canonical sequencer receipt signed by the conformance receipt key for this transaction.
    let canonicalReceipt = try ProgramABI.hex(
      "0x00025201000200000020e827f22f1857d2d443a5b88b414201cb2bfab1e17974072c306d82412ffce6e6000000000000"
        + "0001000000200000000000000000000000000000000000000000000000000000000000000000000000204d0cafa6bd21"
        + "5de579b82477db5522e45e9fcb4f9b83d82d0d095c912e1500db00000020e3b0c44298fc1c149afbf4c8996fb92427ae"
        + "41e4649b934ca495991b7852b8550000000000000001000c000000000100000000200000000000000000000000000000"
        + "0000000000000000000000000000000000000000007100000000000186a000000000000186a000000000000000640000"
        + "000100000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc"
        + "1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000000475687078000000000000000000"
        + "000000009896800000002000000000000000000000000000000000000000000000000000000000000003e8000c000000"
        + "010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73000000000000"
        + "0000000000000000000000000020000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd730000"
        + "0000000000056bc75e2d6310000000000000000000056bc75e2d62776980000000000000000700000020000000000000"
        + "000000000000000000000000000000000000000000000000101d00000000000000000000000000000000000000000000"
        + "000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000"
        + "00200000000000000000000000000000000000000000000000000000000000000000000000206b6b6b6b6b6b6b6b6b6b"
        + "6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b000001977420dc00010000004080b0f0e6f3d6877cc1b3508ca8"
        + "3f06434ddc33b69f68d4f4548b7ef21bb4b3fa198e5bc57efd522a0b5e52d6143f5c1f1ff203befc5ba1c09245e3fa02"
        + "685700")
    let keyID = Data(repeating: 0x6b, count: 32)
    let publicKey = try ProgramABI.hex(
      "0x8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e")
    let receiptKey = ReceiptKeys(
      keyId: keyID, publicKey: publicKey, firstHeight: 1000, lastHeight: 1999)
    var receiptJSON: [String: JSONValue] = [
      "rung": .string("receipted"),
      "nonce": .string("0x7"),
      "seq": .string("0x1"),
      "height": .string("0x3e8"),
      "index": .string("0x1"),
      "timestamp": .string("0x1977420dc00"),
      "status": .string("0x1"),
      "returnData": .string("0x"),
      "logs": .array([]),
      "gasUsed": .string("0x186a0"),
      "cumulativeGasUsed": .string("0x186a0"),
      "effectiveGasPrice": .string("0x64"),
      "feeDenom": .string("uhpx"),
      "paymentDenom": .string("uhpx"),
      "feeAmount": .string("0x989680"),
      "sponsor": .string("0x0000000000000000000000000000000000000000"),
      "from": .string("0xfe3b557e8fb62b89f4916b721be55ceb828dbd73"),
      "to": .string("0x000000000000000000000000000000000000101d"),
      "contractAddress": .string("0x0000000000000000000000000000000000000000"),
      "vmError": .string(""),
    ]
    receiptJSON["txHash"] = .string(hashText)
    receiptJSON["keyId"] = .string("0x" + keyID.map { String(format: "%02x", $0) }.joined())
    receiptJSON["signingKey"] = .string(
      "0x8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e")
    receiptJSON["receipt"] = .string(
      "0x" + canonicalReceipt.map { String(format: "%02x", $0) }.joined())
    let server = try RepairHTTPServer22331397(results: [
      "eth_getTransactionByHash": .object(["hash": .string(hashText)]),
      "pax_sequencerReceipt": .object(receiptJSON),
      "eth_getRawTransactionByHash": .string(
        "0x" + signedTransaction.map { String(format: "%02x", $0) }.joined()),
    ])
    defer { server.stop() }
    let programs = try client(server, receiptKeys: [receiptKey])
    for invalidHash in [
      lifecycle.activityID, Data(), Data(repeating: 1, count: 31),
      Data(repeating: 1, count: 33),
    ] {
      do {
        _ = try await programs.lifecycleReceipt(lifecycle, transactionHash: invalidHash)
        XCTFail("Accepted an activity ID or malformed transaction hash")
      } catch let error as PlatformSDKError {
        XCTAssertEqual(error.code, .invalidArgument)
        XCTAssertEqual(error.retry, .never)
      }
      XCTAssertTrue(try server.requests().isEmpty)
    }
    let receipt = try await programs.lifecycleReceipt(lifecycle, transactionHash: hash)
    XCTAssertEqual(receipt, canonicalReceipt)
    let verified = try LocalVerifier.verifySequencerReceipt(
      receipt, rawTransaction: signedTransaction, keys: [receiptKey],
      receiptJSON: receiptJSON, prevSeqHash: Data(repeating: 0, count: 32))
    XCTAssertEqual(verified.level, "receipted")
    XCTAssertEqual(verified.status, 1)
    XCTAssertEqual(verified.receipt.activityID, hash)
    let requests = try server.requests()
    XCTAssertEqual(
      requests.compactMap { $0["method"]?.stringValue },
      ["eth_getTransactionByHash", "pax_sequencerReceipt", "eth_getRawTransactionByHash"])
    for request in requests {
      XCTAssertEqual(request["params"], .array([.string(hashText)]))
    }
  }

  private func decodeRLPList(_ bytes: Data) throws -> [Data] {
    let bytes = [UInt8](bytes)
    var index = 0
    func next() throws -> (Data, Bool) {
      let prefix = try XCTUnwrap(bytes.indices.contains(index) ? bytes[index] : nil)
      index += 1
      if prefix < 128 { return (Data([prefix]), false) }
      let list = prefix >= 192
      let short = list ? 192 : 128
      let long = list ? 247 : 183
      let length: Int
      if Int(prefix) <= long {
        length = Int(prefix) - short
      } else {
        let width = Int(prefix) - long
        guard width <= 8, index + width <= bytes.count else { throw NativeProgramCallError.invalid }
        length = bytes[index..<(index + width)].reduce(0) { ($0 << 8) | Int($1) }
        index += width
      }
      guard length <= bytes.count - index else { throw NativeProgramCallError.invalid }
      let payload = Data(bytes[index..<(index + length)])
      index += length
      return (payload, list)
    }
    let outer = try next()
    XCTAssertTrue(outer.1)
    XCTAssertEqual(index, bytes.count)
    let inner = [UInt8](outer.0)
    var cursor = 0
    var fields: [Data] = []
    while cursor < inner.count {
      let prefix = Int(inner[cursor])
      cursor += 1
      if prefix < 128 {
        fields.append(Data([UInt8(prefix)]))
        continue
      }
      if prefix == 192 {
        fields.append(Data())
        continue
      }
      guard prefix < 192 else { throw NativeProgramCallError.invalid }
      var length = prefix - 128
      if prefix > 183 {
        let width = prefix - 183
        guard cursor + width <= inner.count else { throw NativeProgramCallError.invalid }
        length = inner[cursor..<(cursor + width)].reduce(0) { ($0 << 8) | Int($1) }
        cursor += width
      }
      guard length <= inner.count - cursor else { throw NativeProgramCallError.invalid }
      fields.append(Data(inner[cursor..<(cursor + length)]))
      cursor += length
    }
    return fields
  }
  private func streamValues() throws -> [JSONValue] {
    let golden = try read("services/human/schema/human-api/golden/stream.next.response.json")
    let result = try object(object(golden, "body"), "result")
    guard case .array(let events)? = result["events"] else { throw NativeProgramCallError.invalid }
    return events
  }

  func testHumanStreamRecordedHTTPShape() async throws {
    let open = try object(
      read("services/human/schema/human-api/golden/stream.open.response.json"), "body")
    let next = try object(
      read("services/human/schema/human-api/golden/stream.next.response.json"), "body")
    let server = try RepairHTTPServer22331397 { request in
      if request.path == "/v1/stream" { return .body(try JSONEncoder().encode(open)) }
      XCTAssertEqual(request.path, "/v1/stream/cur-000000000000000042")
      return .body(try JSONEncoder().encode(next))
    }
    defer { server.stop() }
    let source = HumanStreamSource(
      client: PlatformClient(transport: try HumanHTTPTransport(baseURL: server.url)))
    let cursor = try await source.open()
    let stream = ResumableStream<JSONValue>(cursor: cursor)
    let events = try await stream.next(from: source.source())
    XCTAssertEqual(events.map(\.value), try streamValues())
    XCTAssertEqual(events.map(\.eventID), ["cur-000000000000000043", "cur-000000000000000044"])
    let position = await stream.cursor
    XCTAssertEqual(position.rawValue, "cur-000000000000000044")
  }

  func testSlowConsumerOverflowResumesWithoutLoss() async throws {
    let values = try streamValues()
    let server = try RepairHTTPServer22331397 { request in
      let cursor = try XCTUnwrap(Int(request.path.split(separator: "/").last ?? ""))
      let end = min(200, cursor + StreamLimits.pageEvents)
      let records: [JSONValue] = try (cursor..<end).map { index in
        var value = try XCTUnwrap(values[index % values.count].objectValue)
        value["cursor"] = .string(String(index + 1))
        return .object(value)
      }
      return .body(
        try JSONEncoder().encode(
          JSONValue.object([
            "ok": .boolean(true), "trace": .string("recorded-journal"),
            "result": .object(["events": .array(records), "next_cursor": .string(String(end))]),
          ])))
    }
    defer { server.stop() }
    let source = HumanStreamSource(
      client: PlatformClient(transport: try HumanHTTPTransport(baseURL: server.url)))
    let stream = ResumableStream<JSONValue>(cursor: try StreamCursor("0"))
    let events = stream.events(from: source.source())
    // Leave the consumer idle while the real producer fills its bounded queue.
    try await Task.sleep(nanoseconds: 200_000_000)
    var delivered: [StreamEvent<JSONValue>] = []
    var overflow: StreamOverflow?
    do {
      for try await event in events { delivered.append(event) }
      XCTFail("An idle consumer did not receive an explicit overflow")
    } catch let error as StreamOverflow { overflow = error }
    XCTAssertEqual(delivered.count, StreamLimits.bufferedEvents)
    let resume = try XCTUnwrap(overflow).resumeCursor
    XCTAssertEqual(resume, delivered.last?.cursor)
    let retained = await stream.retainedReplayCount
    XCTAssertLessThanOrEqual(retained, StreamLimits.replayEvents)
    let reopened = ResumableStream<JSONValue>(cursor: resume)
    while delivered.count < 200 {
      delivered += try await reopened.next(from: source.source())
    }
    XCTAssertEqual(delivered.map(\.eventID), (1...200).map(String.init))
    XCTAssertEqual(Set(delivered.map(\.eventID)).count, 200)
    for event in delivered {
      XCTAssertEqual(event.previousCursor.rawValue, String(try XCTUnwrap(Int(event.eventID)) - 1))
    }
  }

  func testReplayWindowAndEmptyPagePacing() async throws {
    let journal = RecordedStreamJournal22331397(values: try streamValues(), count: 1536)
    let stream = ResumableStream<JSONValue>(cursor: try StreamCursor("0"))
    for _ in 0..<12 {
      let page = try await stream.next { try await journal.page($0) }
      XCTAssertEqual(page.count, StreamLimits.pageEvents)
      let retained = await stream.retainedReplayCount
      XCTAssertLessThanOrEqual(retained, StreamLimits.replayEvents)
    }
    let retained = await stream.retainedReplayCount
    XCTAssertEqual(retained, StreamLimits.replayEvents)
    let cursor = await stream.cursor
    let duplicate = StreamEvent(
      eventID: "1536", previousCursor: cursor, cursor: try StreamCursor("1537"),
      value: JSONValue.null)
    do {
      _ = try await stream.accept(
        StreamPage(requestedCursor: cursor, events: [duplicate], nextCursor: duplicate.cursor))
      XCTFail("Recent replay accepted")
    } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .decodeFailure) }
    let tooLarge = try await RecordedStreamJournal22331397(values: streamValues(), count: 129).page(
      try StreamCursor("0"), size: 129)
    let fresh = ResumableStream<JSONValue>(cursor: try StreamCursor("0"))
    do {
      _ = try await fresh.accept(tooLarge)
      XCTFail("Oversized page accepted")
    } catch let error as StreamOverflow { XCTAssertEqual(error.resumeCursor.rawValue, "0") }
    let empty = RecordedEmptyPages22331397()
    let polling = fresh.events { await empty.page($0) }
    let consumer = Task {
      var iterator = polling.makeAsyncIterator()
      _ = try await iterator.next()
    }
    try await Task.sleep(nanoseconds: 350_000_000)
    consumer.cancel()
    _ = try await consumer.value
    let times = await empty.times
    XCTAssertGreaterThanOrEqual(times.count, 2)
    for (first, second) in zip(times, times.dropFirst()) {
      XCTAssertGreaterThanOrEqual(second - first, StreamLimits.emptyPageDelayNanoseconds)
    }
  }
}

// A finite, in-memory event journal retains the recorded human event payloads.
// Positions are assigned by the journal; pages are read through the production stream.
private actor RecordedStreamJournal22331397 {
  let values: [JSONValue]
  let count: Int
  init(values: [JSONValue], count: Int) {
    self.values = values
    self.count = count
  }
  func page(_ cursor: StreamCursor, size: Int = StreamLimits.pageEvents) throws -> StreamPage<
    JSONValue
  > {
    guard let start = Int(cursor.rawValue), start >= 0, start <= count else {
      throw NativeProgramCallError.invalid
    }
    let end = min(count, start + size)
    var events: [StreamEvent<JSONValue>] = []
    for index in start..<end {
      let previous = try StreamCursor(String(index))
      let next = try StreamCursor(String(index + 1))
      let value = values[index % values.count]
      events.append(
        StreamEvent<JSONValue>(
          eventID: String(index + 1), previousCursor: previous, cursor: next, value: value))
    }
    return try StreamPage(
      requestedCursor: cursor, events: events, nextCursor: StreamCursor(String(end)))
  }
}

private actor RecordedEmptyPages22331397 {
  var times: [UInt64] = []
  func page(_ cursor: StreamCursor) -> StreamPage<JSONValue> {
    times.append(DispatchTime.now().uptimeNanoseconds)
    return StreamPage(requestedCursor: cursor, events: [], nextCursor: cursor)
  }
}

private final class RepairHTTPServer22331397: @unchecked Sendable {
  struct Request {
    let path: String
    let json: [String: JSONValue]?
  }
  enum Reply {
    case body(Data)
    case oversized(declaredLength: Bool, limit: Int, cancelled: XCTestExpectation)
    static func rpc(_ value: JSONValue) throws -> Reply {
      .body(
        try JSONEncoder().encode(
          JSONValue.object([
            "jsonrpc": .string("2.0"), "id": .integer(1), "result": value,
          ])))
    }
  }
  let url: URL
  private let descriptor: Int32
  private let lock = NSLock()
  private var observed: [[String: JSONValue]] = []
  private var stopped = false
  private let handler: (Request) throws -> Reply

  convenience init(
    results: [String: JSONValue] = [
      "eth_getTransactionCount": .string("0x7"), "eth_estimateGas": .string("0x186a0"),
    ]
  ) throws {
    try self.init { request in
      let method = try XCTUnwrap(request.json?["method"]?.stringValue)
      return try .rpc(XCTUnwrap(results[method], "Unexpected RPC: \(method)"))
    }
  }

  init(handler: @escaping (Request) throws -> Reply) throws {
    self.handler = handler
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
    guard bound == 0, listen(fd, 16) == 0 else {
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
    self.url = url
    descriptor = fd
    DispatchQueue.global().async { [self] in
      while true {
        let connection = accept(fd, nil, nil)
        if connection < 0 { return }
        defer { close(connection) }
        do {
          var timeout = timeval(tv_sec: 10, tv_usec: 0)
          _ = setsockopt(
            connection, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))
          let request = try Self.readRequest(connection)
          lock.lock()
          observed.append(request.json ?? ["path": .string(request.path)])
          lock.unlock()
          switch try handler(request) {
          case .body(let body):
            try Self.sendAll(
              connection,
              Data(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: \(body.count)\r\nConnection: close\r\n\r\n"
                  .utf8))
            try Self.sendAll(connection, body)
          case .oversized(let declared, let limit, let cancelled):
            let length = declared ? "Content-Length: \(limit + 1)\r\n" : ""
            try Self.sendAll(
              connection,
              Data(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\(length)Connection: close\r\n\r\n"
                  .utf8))
            if !declared {
              let slab = Data(repeating: 32, count: 16_384)
              for _ in 0..<(limit / slab.count) { try Self.sendAll(connection, slab) }
              try Self.sendAll(connection, Data([32]))
            }
            // Keep the response open. Completion must come from client cancellation,
            // not EOF or the server's declared response length being exhausted.
            var byte: UInt8 = 0
            let closed = recv(connection, &byte, 1, 0)
            XCTAssertEqual(closed, 0, "The RPC did not cancel the oversized response")
            cancelled.fulfill()
          }
        } catch {
          XCTFail("HTTP transcript replay failed: \(error)")
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
  func stop() {
    lock.lock()
    defer { lock.unlock() }
    if !stopped {
      stopped = true
      _ = shutdown(descriptor, Int32(SHUT_RDWR))
      close(descriptor)
    }
  }
  private static func sendAll(_ fd: Int32, _ data: Data) throws {
    try data.withUnsafeBytes { pointer in
      var sent = 0
      while sent < data.count {
        #if os(Linux)
          let count = send(
            fd, pointer.baseAddress!.advanced(by: sent), data.count - sent, Int32(MSG_NOSIGNAL))
        #else
          let count = send(fd, pointer.baseAddress!.advanced(by: sent), data.count - sent, 0)
        #endif
        guard count > 0 else { throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno)) }
        sent += count
      }
    }
  }
  private static func readRequest(_ fd: Int32) throws -> Request {
    var data = Data()
    var buffer = [UInt8](repeating: 0, count: 4096)
    while data.count <= 2 * 1024 * 1024 {
      let count = recv(fd, &buffer, buffer.count, 0)
      guard count > 0 else { throw NSError(domain: NSPOSIXErrorDomain, code: Int(errno)) }
      data.append(contentsOf: buffer.prefix(count))
      if let end = data.range(of: Data("\r\n\r\n".utf8)) {
        let header = String(decoding: data[..<end.lowerBound], as: UTF8.self)
        let lines = header.components(separatedBy: "\r\n")
        let requestLine = try XCTUnwrap(lines.first).split(separator: " ")
        guard requestLine.count == 3 else { throw NativeProgramCallError.invalid }
        let lengthLine = lines.first { $0.lowercased().hasPrefix("content-length:") }
        let length: Int
        if let lengthLine {
          length = try XCTUnwrap(
            Int(
              lengthLine.split(separator: ":", maxSplits: 1)[1].trimmingCharacters(in: .whitespaces)
            ))
        } else {
          length = 0
        }
        if data.count - end.upperBound >= length {
          let body = data.subdata(in: end.upperBound..<(end.upperBound + length))
          let json =
            body.isEmpty ? nil : try JSONDecoder().decode([String: JSONValue].self, from: body)
          return Request(path: String(requestLine[1]), json: json)
        }
      }
    }
    throw NativeProgramCallError.invalid
  }
}
