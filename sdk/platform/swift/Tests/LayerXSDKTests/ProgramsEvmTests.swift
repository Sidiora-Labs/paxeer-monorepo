import Crypto
import Foundation
import XCTest

@testable import LayerXSDK

#if canImport(FoundationNetworking)
  import FoundationNetworking
#endif

final class ProgramsEvmTests: XCTestCase {
  private struct Fixture: Decodable {
    let payload_hex: String
    let signed_activity_hex: String
    let idempotency_key_hex: String
    let evm: EVM
    struct EVM: Decodable {
      let to: String
      let chainId: UInt64
      let calldata: String
    }
  }

  private func fixture(_ name: String) throws -> Fixture {
    let root = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
    return try JSONDecoder().decode(
      Fixture.self,
      from: Data(
        contentsOf:
          root.appendingPathComponent("conformance/fixtures/\(name).json")))
  }

  private struct SequencerFixture: Decodable {
    let cases: [Vector]
    struct Vector: Decodable {
      let rawTransaction: String
      let receiptJson: [String: JSONValue]
      let prevSeqHash: String
      let pinnedKey: Key
    }
    struct Key: Decodable {
      let keyId: String
      let publicKey: String
      let firstHeight: UInt64
      let lastHeight: UInt64
    }
  }

  private func client(
    _ server: RPCFixtureServer, receiptKeys: [ReceiptKeys] = [],
    prevSeqHash: Data = Data(repeating: 0, count: 32)
  ) throws -> ProgramsClient {
    try ProgramsClient(
      rpcURL: server.url, sender: Data(repeating: 0x55, count: 20),
      actor: Data(repeating: 0x66, count: 32), receiptKeys: receiptKeys,
      maxFeePerGas: 100, maxPriorityFeePerGas: 2, prevSeqHash: prevSeqHash)
  }

  private func nativeCall(_ fixture: Fixture) throws -> ProgramCall {
    try ProgramCall(
      nativeCall: NativeProgramCall.decode(nativeTransportHex(fixture.payload_hex)),
      feeLimit: ProtocolAmount("1000"))
  }

  private func assertTransaction(
    _ tx: UnsignedProgramTransaction, fixture: Fixture,
    requests: [[String: JSONValue]], file: StaticString = #filePath,
    line: UInt = #line
  ) throws {
    XCTAssertEqual(tx.type, 2, file: file, line: line)
    XCTAssertEqual(tx.chainID, fixture.evm.chainId, file: file, line: line)
    XCTAssertEqual(
      try ProgramABI.hex(tx.to), try ProgramABI.hex(normalize(fixture.evm.to)), file: file,
      line: line)
    XCTAssertEqual(tx.calldata, try ProgramABI.hex(fixture.evm.calldata), file: file, line: line)
    XCTAssertEqual(tx.nonce, 7, file: file, line: line)
    XCTAssertEqual(tx.gas, 100_000, file: file, line: line)
    XCTAssertEqual(tx.maxFeePerGas, 100, file: file, line: line)
    XCTAssertEqual(tx.maxPriorityFeePerGas, 2, file: file, line: line)
    XCTAssertEqual(tx.value, 0, file: file, line: line)
    XCTAssertEqual(tx.signingPayload().first, 2, file: file, line: line)
    let signingFields = try decodeRLPList(Data(tx.signingPayload().dropFirst()))
    XCTAssertEqual(signingFields.count, 9, file: file, line: line)
    XCTAssertEqual(signingFields[0], Data([125]), file: file, line: line)
    XCTAssertEqual(signingFields[1], Data([7]), file: file, line: line)
    XCTAssertEqual(signingFields[2], Data([2]), file: file, line: line)
    XCTAssertEqual(signingFields[3], Data([100]), file: file, line: line)
    XCTAssertEqual(signingFields[4], Data([1, 0x86, 0xa0]), file: file, line: line)
    XCTAssertEqual(signingFields[5], try ProgramABI.hex(tx.to), file: file, line: line)
    XCTAssertEqual(signingFields[6], Data(), file: file, line: line)
    XCTAssertEqual(signingFields[7], tx.calldata, file: file, line: line)
    XCTAssertEqual(signingFields[8], Data(), file: file, line: line)
    XCTAssertEqual(requests.count, 2, file: file, line: line)
    XCTAssertEqual(
      requests[0]["method"], .string("eth_getTransactionCount"), file: file, line: line)
    XCTAssertEqual(
      requests[0]["params"],
      .array([
        .string("0x" + String(repeating: "55", count: 20)), .string("pending"),
      ]), file: file, line: line)
    XCTAssertEqual(requests[1]["method"], .string("eth_estimateGas"), file: file, line: line)
    XCTAssertEqual(
      requests[1]["params"], .array([tx.rpcValue(includeGas: false), .string("pending")]),
      file: file, line: line)
    XCTAssertNil(tx.rpcValue().objectValue?["r"], file: file, line: line)
    XCTAssertNil(tx.rpcValue().objectValue?["s"], file: file, line: line)
  }

  private func normalize(_ address: String) -> String {
    let digits = String(address.dropFirst(2)).lowercased()
    return "0x" + String(repeating: "0", count: 40 - digits.count) + digits
  }

  func testProgramCallBuildsLayerxprogramsCallTransaction() async throws {
    let vector = try fixture("native-program-call-v4")
    let server = try RPCFixtureServer()
    defer { server.stop() }
    let programs = try client(server)
    let call = try nativeCall(vector)
    let transaction = try await programs.submit(
      call, idempotencyKey: IdempotencyKey(vector.idempotency_key_hex))
    try assertTransaction(transaction, fixture: vector, requests: server.requests())
    XCTAssertNil(call.transactionHash)
    let maximum = try ProgramCall(
      nativeCall: XCTUnwrap(call.nativeCall),
      feeLimit: ProtocolAmount("340282366920938463463374607431768211455"))
    XCTAssertEqual(
      try maximum.evmCalldata().subdata(in: 452..<484),
      Data(repeating: 0, count: 16) + Data(repeating: 255, count: 16))
    let signed = try ProgramCall(
      nativeCall: XCTUnwrap(call.nativeCall), feeLimit: ProtocolAmount("1000"),
      signedActivity: nativeTransportHex(vector.signed_activity_hex))
    XCTAssertEqual(try signed.evmCalldata(), transaction.calldata)
  }

  func testProgramSimulateUsesPendingEthCall() async throws {
    let vector = try fixture("native-program-call-v4")
    let server = try RPCFixtureServer()
    defer { server.stop() }
    let simulation = try await client(server).simulate(nativeCall(vector))
    XCTAssertEqual(simulation.terminalResult, 0)
    XCTAssertEqual(simulation.terminalKind, 1)
    XCTAssertEqual(simulation.response, Data([0, 0x61, 0xff, 0x10]))
    let requests = try server.requests()
    XCTAssertEqual(requests.count, 1)
    XCTAssertEqual(requests[0]["method"], .string("eth_call"))
    let calldata = "0x3044bd56" + vector.evm.calldata.dropFirst(10)
    XCTAssertEqual(
      requests[0]["params"],
      .array([
        .object([
          "from": .string("0x" + String(repeating: "55", count: 20)),
          "to": .string(Programs.address), "data": .string(String(calldata)),
        ]), .string("pending"),
      ]))
    XCTAssertEqual(requests[0]["http_path"], .string("/"))
    XCTAssertFalse(
      requests.contains { $0["http_path"]?.stringValue?.hasPrefix("/v1/programs") == true })
  }

  func testProgramDeployBuildsLayerxprogramsDeployTransaction() async throws {
    let vector = try fixture("native-program-deploy-v4")
    let operation = try NativeProgramDeploy.decode(nativeTransportHex(vector.payload_hex))
    let request = try NativeProgramLifecycleRequest(
      operation: operation,
      signedActivity: nativeTransportHex(vector.signed_activity_hex))
    XCTAssertEqual(try request.calldata(), try ProgramABI.hex(vector.evm.calldata))
    let server = try RPCFixtureServer()
    defer { server.stop() }
    let transaction = try await client(server).deploy(
      operation,
      signedActivity: nativeTransportHex(vector.signed_activity_hex),
      idempotencyKey: IdempotencyKey(vector.idempotency_key_hex))
    try assertTransaction(transaction, fixture: vector, requests: server.requests())
    XCTAssertEqual(
      try NativeProgramLifecycleRequest.calldata(operation: operation), transaction.calldata)
    var invalid = try nativeTransportHex(vector.payload_hex)
    invalid[invalid.count - 1] ^= 1
    XCTAssertThrowsError(try NativeProgramDeploy.decode(invalid))
  }

  func testProgramReceiptResolvesByTransaction() async throws {
    let root = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
    let key = try IdempotencyKey(String(repeating: "77", count: 32))
    for (name, state) in [
      ("vector-v1-receipted-positive", "executed"),
      ("vector-v2-receipted-revert", "refused"),
    ] {
      let fixture = try JSONDecoder().decode(
        SequencerFixture.self,
        from: Data(contentsOf: root.appendingPathComponent("conformance/fixtures/\(name).json")))
      let vector = try XCTUnwrap(fixture.cases.first)
      let hashText = try XCTUnwrap(vector.receiptJson["txHash"]?.stringValue)
      let hash = try ProgramABI.hex(hashText)
      let receiptKey = ReceiptKeys(
        keyId: try ProgramABI.hex(vector.pinnedKey.keyId),
        publicKey: try ProgramABI.hex(vector.pinnedKey.publicKey),
        firstHeight: vector.pinnedKey.firstHeight, lastHeight: vector.pinnedKey.lastHeight)
      let server = try RPCFixtureServer(results: [
        "eth_getTransactionByHash": .object(["hash": .string(hashText)]),
        "pax_sequencerReceipt": .object(vector.receiptJson),
        "eth_getRawTransactionByHash": .string(vector.rawTransaction),
      ])
      defer { server.stop() }
      let submission = try await client(
        server, receiptKeys: [receiptKey], prevSeqHash: ProgramABI.hex(vector.prevSeqHash)
      ).receipt(idempotencyKey: key, expectedActivityID: hash, verificationLevel: "pending")
      XCTAssertEqual(submission.state, state)
      XCTAssertEqual(submission.transactionHash, hash)
      XCTAssertEqual(submission.idempotencyKey, key.rawValue)
      XCTAssertEqual(submission.retainedSignedActivity, try ProgramABI.hex(vector.rawTransaction))
      let verified = try XCTUnwrap(submission.sequencerReceipt)
      XCTAssertEqual(verified.receipt.activityID, hash)
      XCTAssertEqual(
        verified.canonicalBytes, try ProgramABI.hex(vector.receiptJson["receipt"]?.stringValue))
      XCTAssertEqual(verified.status, state == "executed" ? 1 : 0)
      let requests = try server.requests()
      XCTAssertEqual(
        requests.compactMap { $0["method"]?.stringValue },
        ["eth_getTransactionByHash", "pax_sequencerReceipt", "eth_getRawTransactionByHash"])
      for request in requests {
        XCTAssertEqual(request["params"], .array([.string(hashText)]))
        if case .array(let params)? = request["params"] {
          for parameter in params {
            XCTAssertNotEqual(parameter.objectValue?["to"], .string(Programs.authAddress))
            XCTAssertFalse(
              parameter.objectValue?["data"]?.stringValue?.hasPrefix("0x2eda9f2b") == true)
          }
        }
      }
    }

    let hash = Data(repeating: 0x88, count: 32)
    let unknownServer = try RPCFixtureServer(results: ["eth_getTransactionByHash": .null])
    defer { unknownServer.stop() }
    let unknown = try await client(unknownServer).receipt(
      idempotencyKey: key, expectedActivityID: hash, verificationLevel: "pending")
    XCTAssertEqual(unknown.state, "unknown")
    XCTAssertEqual(unknown.transactionHash, hash)
    XCTAssertEqual(unknown.idempotencyKey, key.rawValue)
    XCTAssertNil(unknown.sequencerReceipt)
    XCTAssertEqual(
      try unknownServer.requests().compactMap { $0["method"]?.stringValue },
      ["eth_getTransactionByHash"])

    let deploy = try fixture("native-program-deploy-v4")
    let lifecycle = try NativeProgramLifecycleRequest(
      operation: NativeProgramDeploy.decode(nativeTransportHex(deploy.payload_hex)),
      signedActivity: nativeTransportHex(deploy.signed_activity_hex))
    XCTAssertNotEqual(hash, lifecycle.activityID)
    do {
      _ = try await client(unknownServer).lifecycleReceipt(lifecycle, transactionHash: hash)
      XCTFail("Accepted a lifecycle transaction without a sequencer receipt")
    } catch let error as PlatformSDKError {
      XCTAssertEqual(error.code, .unknownOutcome)
      XCTAssertEqual(error.retry, .unknownOutcome)
    }
    let unknownRequests = try unknownServer.requests()
    XCTAssertEqual(unknownRequests.count, 2)
    let lifecycleRequest = try XCTUnwrap(unknownRequests.last)
    XCTAssertEqual(lifecycleRequest["method"], .string("eth_getTransactionByHash"))
    XCTAssertEqual(
      lifecycleRequest["params"],
      .array([.string("0x" + hash.map { String(format: "%02x", $0) }.joined())]))
  }

  func testProgramLifecycleReceiptResolvesBySignedTransactionHash() async throws {
    let vector = try fixture("native-program-deploy-v4")
    let operation = try NativeProgramDeploy.decode(nativeTransportHex(vector.payload_hex))
    let lifecycle = try NativeProgramLifecycleRequest(
      operation: operation, signedActivity: nativeTransportHex(vector.signed_activity_hex))
    let buildServer = try RPCFixtureServer()
    defer { buildServer.stop() }
    let transaction = try await client(buildServer).deploy(
      operation, signedActivity: lifecycle.signedActivity,
      idempotencyKey: IdempotencyKey(vector.idempotency_key_hex))
    try assertTransaction(transaction, fixture: vector, requests: buildServer.requests())

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
      "paymentDenom": .string("uhpx"),
      "feeDenom": .string("uhpx"),
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
    let server = try RPCFixtureServer(results: [
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
    let requests = try server.requests()
    XCTAssertEqual(
      requests.compactMap { $0["method"]?.stringValue },
      ["eth_getTransactionByHash", "pax_sequencerReceipt", "eth_getRawTransactionByHash"])
    for request in requests {
      XCTAssertEqual(request["params"], .array([.string(hashText)]))
    }
  }

  func testProgramReceiptCarriesIdempotencyResult() async throws {
    let key = try IdempotencyKey(String(repeating: "77", count: 32))
    let hash = Data(repeating: 0x88, count: 32)
    let server = try RPCFixtureServer(results: ["eth_getTransactionByHash": .null])
    defer { server.stop() }
    let submission = try await client(server).receipt(
      idempotencyKey: key, expectedActivityID: hash, verificationLevel: "pending")
    XCTAssertEqual(submission.state, "unknown")
    XCTAssertEqual(submission.idempotencyKey, key.rawValue)
    XCTAssertEqual(submission.transactionHash, hash)
    XCTAssertNil(submission.sequencerReceipt)
    let requests = try server.requests()
    XCTAssertEqual(requests.count, 1)
    XCTAssertEqual(requests.first?["method"], .string("eth_getTransactionByHash"))
    XCTAssertEqual(
      requests.first?["params"],
      .array([.string("0x" + hash.map { String(format: "%02x", $0) }.joined())]))
    XCTAssertFalse(
      requests.contains { request in
        guard case .array(let params)? = request["params"] else { return false }
        return params.contains {
          $0.objectValue?["data"]?.stringValue?.hasPrefix("0x2eda9f2b") == true
        }
      })
  }

  func testProgramActivityVerifiesSequencerReceipt() async throws {
    let root = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
    func vector(_ name: String) throws -> SequencerFixture.Vector {
      let fixture = try JSONDecoder().decode(
        SequencerFixture.self,
        from: Data(contentsOf: root.appendingPathComponent("conformance/fixtures/\(name).json")))
      return try XCTUnwrap(fixture.cases.first)
    }
    let positive = try vector("vector-v1-receipted-positive")
    let refused = try vector("vector-v2-receipted-revert")
    for (vector, state) in [(positive, "executed"), (refused, "refused")] {
      let hashText = try XCTUnwrap(vector.receiptJson["txHash"]?.stringValue)
      let hash = try ProgramABI.hex(hashText)
      let key = ReceiptKeys(
        keyId: try ProgramABI.hex(vector.pinnedKey.keyId),
        publicKey: try ProgramABI.hex(vector.pinnedKey.publicKey),
        firstHeight: vector.pinnedKey.firstHeight, lastHeight: vector.pinnedKey.lastHeight)
      let server = try RPCFixtureServer(results: [
        "eth_getTransactionByHash": .object(["hash": .string(hashText)]),
        "pax_sequencerReceipt": .object(vector.receiptJson),
        "eth_getRawTransactionByHash": .string(vector.rawTransaction),
      ])
      defer { server.stop() }
      let submission = try await client(
        server, receiptKeys: [key], prevSeqHash: ProgramABI.hex(vector.prevSeqHash)
      ).activity(transactionHash: hash)
      XCTAssertEqual(submission.state, state)
      XCTAssertEqual(submission.transactionHash, hash)
      let verified = try XCTUnwrap(submission.sequencerReceipt)
      XCTAssertEqual(verified.receipt.activityID, hash)
      XCTAssertEqual(
        verified.canonicalBytes, try ProgramABI.hex(vector.receiptJson["receipt"]?.stringValue))
      XCTAssertEqual(verified.status, state == "executed" ? 1 : 0)
      let requests = try server.requests()
      XCTAssertEqual(
        requests.compactMap { $0["method"]?.stringValue },
        [
          "eth_getTransactionByHash", "pax_sequencerReceipt", "eth_getRawTransactionByHash",
        ])
      for request in requests {
        XCTAssertEqual(request["params"], .array([.string(hashText)]))
      }
    }

    let hashText = try XCTUnwrap(positive.receiptJson["txHash"]?.stringValue)
    let hash = try ProgramABI.hex(hashText)
    let key = ReceiptKeys(
      keyId: try ProgramABI.hex(positive.pinnedKey.keyId),
      publicKey: try ProgramABI.hex(positive.pinnedKey.publicKey),
      firstHeight: positive.pinnedKey.firstHeight, lastHeight: positive.pinnedKey.lastHeight)
    let prevSeqHash = try ProgramABI.hex(positive.prevSeqHash)
    let pendingServer = try RPCFixtureServer(results: [
      "eth_getTransactionByHash": .object(["hash": .string(hashText)]),
      "pax_sequencerReceipt": .object([
        "rung": .string("submitted"), "txHash": .string(hashText), "nonce": .string("0x0"),
      ]),
    ])
    defer { pendingServer.stop() }
    let pending = try await client(pendingServer).activity(transactionHash: hash)
    XCTAssertEqual(pending.state, "pending")
    XCTAssertEqual(pending.transactionHash, hash)
    XCTAssertNil(pending.sequencerReceipt)
    XCTAssertEqual(
      try pendingServer.requests().compactMap { $0["method"]?.stringValue },
      [
        "eth_getTransactionByHash", "pax_sequencerReceipt",
      ])

    let unknownServer = try RPCFixtureServer(results: ["eth_getTransactionByHash": .null])
    defer { unknownServer.stop() }
    let unknown = try await client(unknownServer).activity(transactionHash: hash)
    XCTAssertEqual(unknown.state, "unknown")
    XCTAssertEqual(unknown.transactionHash, hash)
    XCTAssertNil(unknown.sequencerReceipt)
    XCTAssertEqual(try unknownServer.requests().count, 1)

    let failures: [(String, [ReceiptKeys], String, Data)] = [
      ("unpinned key", [], positive.rawTransaction, prevSeqHash),
      ("wrong raw transaction", [key], refused.rawTransaction, prevSeqHash),
      ("wrong predecessor hash", [key], positive.rawTransaction, Data(repeating: 1, count: 32)),
    ]
    for (name, keys, rawTransaction, predecessor) in failures {
      let server = try RPCFixtureServer(results: [
        "eth_getTransactionByHash": .object(["hash": .string(hashText)]),
        "pax_sequencerReceipt": .object(positive.receiptJson),
        "eth_getRawTransactionByHash": .string(rawTransaction),
      ])
      defer { server.stop() }
      do {
        _ = try await client(server, receiptKeys: keys, prevSeqHash: predecessor)
          .activity(transactionHash: hash)
        XCTFail("Accepted \(name)")
      } catch {
        XCTAssertEqual((error as? PlatformSDKError)?.code, .verificationFailure, name)
      }
    }
    for count in [31, 33] {
      XCTAssertThrowsError(
        try client(pendingServer, receiptKeys: [key], prevSeqHash: Data(repeating: 0, count: count))
      ) { error in
        XCTAssertEqual((error as? PlatformSDKError)?.code, .invalidArgument)
      }
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
}

private final class RPCFixtureServer {
  let url: URL
  private let process: Process
  private let directory: URL
  private let transcript: URL

  init(results: [String: JSONValue] = [:]) throws {
    directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    transcript = directory.appendingPathComponent("rpc.jsonl")
    try Data().write(to: transcript)
    let configuredResults = directory.appendingPathComponent("results.json")
    try JSONEncoder().encode(results).write(to: configuredResults)
    process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    process.arguments = [
      "python3", "-u", "-c",
      #"""
      import http.server, json, signal, sys
      signal.pthread_sigmask(signal.SIG_UNBLOCK, {signal.SIGTERM})
      class Handler(http.server.BaseHTTPRequestHandler):
          def log_message(self, *args): pass
          def do_POST(self):
              request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
              with open(sys.argv[1], 'a') as output:
                  output.write(json.dumps(dict(request, http_path=self.path)) + '\n')
              method = request['method']
              with open(sys.argv[2]) as configured:
                  results = json.load(configured)
              if method in results: result = results[method]
              elif method == 'eth_getTransactionCount': result = '0x7'
              elif method == 'eth_estimateGas': result = '0x186a0'
              elif method == 'eth_call':
                  result = '0x' + ''.join(format(v, '064x') for v in [0, 1, 96, 4]) + '0061ff10' + '00' * 28
              else:
                  self.send_error(400)
                  return
              body = json.dumps({'jsonrpc': '2.0', 'id': request['id'], 'result': result}).encode()
              self.send_response(200)
              self.send_header('Content-Type', 'application/json')
              self.send_header('Content-Length', str(len(body)))
              self.end_headers()
              self.wfile.write(body)
      server = http.server.HTTPServer(('127.0.0.1', 0), Handler)
      print(server.server_port, flush=True)
      server.serve_forever()
      """#, transcript.path, configuredResults.path,
    ]
    let stdout = Pipe()
    process.standardOutput = stdout
    process.standardError = FileHandle.standardError
    try process.run()
    var port = Data()
    while true {
      let byte = stdout.fileHandleForReading.readData(ofLength: 1)
      guard !byte.isEmpty else { throw NativeProgramCallError.invalid }
      if byte == Data([10]) { break }
      port.append(byte)
    }
    guard let text = String(data: port, encoding: .utf8), let number = UInt16(text),
      let address = URL(string: "http://127.0.0.1:\(number)")
    else { throw NativeProgramCallError.invalid }
    url = address
  }
  func requests() throws -> [[String: JSONValue]] {
    let bytes = try String(contentsOf: transcript, encoding: .utf8)
    return try bytes.split(separator: "\n").map {
      try XCTUnwrap(JSONDecoder().decode(JSONValue.self, from: Data($0.utf8)).objectValue)
    }
  }
  func stop() {
    if process.isRunning {
      process.terminate()
      process.waitUntilExit()
    }
    try? FileManager.default.removeItem(at: directory)
  }
  deinit { stop() }
}

extension ProgramsEvmTests {
  func testRecordedNodeReceiptsAcrossCommitAndDifferentDenominations() async throws {
    try await Self.proveRecordedEVMProducer1398()
  }

  static func proveRecordedEVMProducer1398() async throws {
    typealias E = RecordedProgramEvidence1398
    let (committed, committedView) = try E.recordedCommitted()
    let (feeToken, feeTokenView) = try E.feeToken()
    let reverted = try E.firstVector("vector-v2-receipted-revert")
    let sources: [(E.Record, E.Record, String)] = [
      (committed, try E.object(committed, "receiptJson"), "executed"),
      (committed, committedView, "executed"),
      (feeToken, feeTokenView, "executed"),
      (reverted, try E.object(reverted, "receiptJson"), "refused"),
    ]
    var preCommitDigest: Data?
    for (vector, view, state) in sources {
      let server = try ProgramReplayServer1398(E.rpcSteps(vector, view: view))
      defer { server.stop() }
      let hash = try E.bytes(view, "txHash")
      let submission = try await E.client(server, vector: vector).activity(transactionHash: hash)
      XCTAssertEqual(submission.state, state)
      XCTAssertEqual(submission.transactionHash, hash)
      XCTAssertEqual(submission.retainedSignedActivity, try E.bytes(vector, "rawTransaction"))
      let receipt = try XCTUnwrap(submission.sequencerReceipt)
      XCTAssertEqual(receipt.canonicalBytes, try E.bytes(vector, "signedReceipt"))
      XCTAssertEqual(receipt.receiptDigest, try E.bytes(vector, "receiptDigest"))
      XCTAssertEqual(receipt.receipt.activityID, hash)
      XCTAssertEqual(receipt.status, state == "executed" ? 1 : 0)
      XCTAssertEqual(
        receipt.receipt.asset,
        Data(SHA256.hash(data: Data(try E.text(view, "paymentDenom").utf8))))
      // An RPC's committed label does not replace independently verified consensus.
      XCTAssertEqual(receipt.level, "receipted")
      if vector["id"] == committed["id"] {
        if view["rung"] == .string("committed") {
          XCTAssertEqual(receipt.reportedBlockHash, try E.checkpoint().blockHash)
          XCTAssertEqual(receipt.reportedBlockNumber, 10)
          XCTAssertEqual(receipt.receiptDigest, try XCTUnwrap(preCommitDigest))
        } else {
          XCTAssertNil(receipt.reportedBlockHash)
          preCommitDigest = receipt.receiptDigest
        }
      }
      if vector["id"] == feeToken["id"] {
        XCTAssertNotEqual(view["paymentDenom"], view["feeDenom"])
        XCTAssertEqual(view["paymentDenom"], .string("usid"))
        XCTAssertEqual(view["feeDenom"], .string("uhpx"))
        XCTAssertEqual(receipt.receipt.asset, try E.bytes(feeToken, "asset"))
      }
      server.assertConsumed(3)
      XCTAssertEqual(
        server.requests().compactMap { $0["method"]?.stringValue },
        ["eth_getTransactionByHash", "pax_sequencerReceipt", "eth_getRawTransactionByHash"])
    }

    // Refusals traverse HTTP and the production transaction and signature checks.
    for (vector, view) in [(committed, committedView), (feeToken, feeTokenView)] {
      let hash = try E.bytes(view, "txHash")
      let unpinned = try ProgramReplayServer1398(E.rpcSteps(vector, view: view))
      defer { unpinned.stop() }
      do {
        _ = try await E.client(unpinned, vector: vector, keys: []).activity(transactionHash: hash)
        XCTFail("Accepted an unpinned receipt key")
      } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .verificationFailure) }
      unpinned.assertConsumed(3)
      let other = vector["id"] == committed["id"] ? feeToken : committed
      let wrongTransaction = try ProgramReplayServer1398(
        E.rpcSteps(vector, view: view, raw: E.text(other, "rawTransaction")))
      defer { wrongTransaction.stop() }
      do {
        _ = try await E.client(wrongTransaction, vector: vector).activity(transactionHash: hash)
        XCTFail("Accepted another signed transaction")
      } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .verificationFailure) }
      wrongTransaction.assertConsumed(3)
      for field in ["paymentDenom", "feeDenom"] {
        var changed = view
        changed[field] = .string("unregistered")
        let tampered = try ProgramReplayServer1398(E.rpcSteps(vector, view: changed))
        defer { tampered.stop() }
        do {
          _ = try await E.client(tampered, vector: vector).activity(transactionHash: hash)
          XCTFail("Accepted a changed \(field)")
        } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .verificationFailure) }
        tampered.assertConsumed(3)
      }
    }
  }
}
