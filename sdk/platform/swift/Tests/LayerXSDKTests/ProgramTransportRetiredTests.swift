import Foundation
import XCTest

@testable import LayerXSDK

#if canImport(FoundationNetworking)
  import FoundationNetworking
#endif

final class ProgramTransportRetiredTests: XCTestCase {
  private struct Fixture: Decodable {
    let payload_hex: String
    let signed_activity_hex: String
    let idempotency_key_hex: String
  }

  private func fixture(_ name: String) throws -> Fixture {
    let root = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
    return try JSONDecoder().decode(
      Fixture.self,
      from: Data(contentsOf: root.appendingPathComponent("conformance/fixtures/\(name).json")))
  }

  func testProgramOperationsNeverCallProgramsRestIngress() async throws {
    let configuration = URLSessionConfiguration.ephemeral
    configuration.protocolClasses = [RecordingProgramHTTP.self]
    let session = URLSession(configuration: configuration)
    defer { session.invalidateAndCancel() }
    let endpoint = try XCTUnwrap(URL(string: "http://127.0.0.1:8080/rpc"))
    let programs = try ProgramsClient(
      rpcURL: endpoint, sender: Data(repeating: 0x55, count: 20),
      actor: Data(repeating: 0x66, count: 32), receiptKeys: [],
      maxFeePerGas: 100, maxPriorityFeePerGas: 2, session: session)
    let vector = try fixture("native-program-call-v4")
    let call = try ProgramCall(
      nativeCall: NativeProgramCall.decode(nativeTransportHex(vector.payload_hex)),
      feeLimit: ProtocolAmount("1000"))
    let signedCall = try ProgramCall(
      nativeCall: XCTUnwrap(call.nativeCall), feeLimit: ProtocolAmount("1000"),
      signedActivity: nativeTransportHex(vector.signed_activity_hex))
    let key = try IdempotencyKey(vector.idempotency_key_hex)
    let programID = call.programID
    let transactionHash = Data(repeating: 0x77, count: 32)
    RecordingProgramHTTP.requests.reset()

    func record(
      _ operation: String, method: String, selector: String? = nil,
      _ invoke: () async throws -> Void
    ) async throws {
      let before = RecordingProgramHTTP.requests.snapshot().count
      do {
        try await invoke()
        XCTFail("\(operation) did not reach the recording transport")
      } catch let error as URLError {
        XCTAssertEqual(error.code, .networkConnectionLost, operation)
      }
      let requests = RecordingProgramHTTP.requests.snapshot()
      XCTAssertEqual(requests.count, before + 1, operation)
      let request = try XCTUnwrap(requests.last)
      XCTAssertEqual(request.url, endpoint, operation)
      XCTAssertEqual(request.httpMethod, "POST", operation)
      XCTAssertFalse(try XCTUnwrap(request.url).path.hasPrefix("/v1/programs"), operation)
      let body = try JSONDecoder().decode(JSONValue.self, from: XCTUnwrap(request.httpBody))
      XCTAssertEqual(body.objectValue?["jsonrpc"], .string("2.0"), operation)
      XCTAssertEqual(body.objectValue?["method"], .string(method), operation)
      if let selector {
        guard case let .array(params)? = body.objectValue?["params"] else {
          return XCTFail("\(operation) has no RPC parameters")
        }
        let transaction = try XCTUnwrap(params.first?.objectValue)
        XCTAssertTrue(
          try XCTUnwrap(transaction["data"]?.stringValue).hasPrefix("0x" + selector), operation)
        XCTAssertEqual(params.last, .string("pending"), operation)
        XCTAssertEqual(
          transaction["to"],
          .string(selector == "2eda9f2b" ? Programs.authAddress : Programs.address),
          operation)
      }
    }

    try await record("call", method: "eth_getTransactionCount") {
      _ = try await programs.call(call)
    }
    try await record("submit", method: "eth_getTransactionCount") {
      _ = try await programs.submit(call, idempotencyKey: key)
    }
    try await record("signed submit", method: "eth_getTransactionCount") {
      _ = try await programs.submit(signedCall, idempotencyKey: key)
    }
    try await record("simulate", method: "eth_call", selector: "3044bd56") {
      _ = try await programs.simulate(call)
    }
    try await record("discover", method: "eth_call", selector: "b1ffb3d4") {
      _ = try await programs.discover(programID: programID)
    }
    try await record("interface", method: "eth_call", selector: "8efc7240") {
      _ = try await programs.interface(programID: programID)
    }
    try await record("receipt by actor", method: "eth_call", selector: "2eda9f2b") {
      _ = try await programs.receipt(actor: Data(repeating: 0x66, count: 32), idempotencyKey: key)
    }
    try await record("receipt", method: "eth_call", selector: "2eda9f2b") {
      _ = try await programs.receipt(
        idempotencyKey: key, expectedActivityID: transactionHash, verificationLevel: "pending")
    }
    try await record("activity", method: "eth_getTransactionByHash") {
      _ = try await programs.activity(activityID: transactionHash, verificationLevel: "pending")
    }
    try await record("activity by transaction", method: "eth_getTransactionByHash") {
      _ = try await programs.activity(transactionHash: transactionHash)
    }

    let deployVector = try fixture("native-program-deploy-v4")
    let deploy = try NativeProgramDeploy.decode(nativeTransportHex(deployVector.payload_hex))
    let signedDeploy = try nativeTransportHex(deployVector.signed_activity_hex)
    let lifecycle = try NativeProgramLifecycleRequest(
      operation: deploy, signedActivity: signedDeploy)
    try await record("deploy", method: "eth_getTransactionCount") {
      _ = try await programs.deploy(deploy)
    }
    try await record("signed deploy", method: "eth_getTransactionCount") {
      _ = try await programs.deploy(
        deploy, signedActivity: signedDeploy,
        idempotencyKey: IdempotencyKey(deployVector.idempotency_key_hex))
    }
    try await record("lifecycle receipt", method: "eth_call", selector: "2eda9f2b") {
      _ = try await programs.lifecycleReceipt(lifecycle)
    }
    let upgradeVector = try fixture("native-program-upgrade-v4")
    let upgrade = try NativeProgramUpgrade.decode(nativeTransportHex(upgradeVector.payload_hex))
    try await record("upgrade", method: "eth_getTransactionCount") {
      _ = try await programs.upgrade(upgrade, feeLimit: ProtocolAmount("1000"))
    }
    try await record("signed upgrade", method: "eth_getTransactionCount") {
      _ = try await programs.upgrade(
        upgrade, signedActivity: nativeTransportHex(upgradeVector.signed_activity_hex),
        idempotencyKey: IdempotencyKey(upgradeVector.idempotency_key_hex))
    }
    let windDownVector = try fixture("native-program-wind-down-tombstone-v4")
    let windDown = try NativeProgramWindDown.decode(nativeTransportHex(windDownVector.payload_hex))
    try await record("wind down", method: "eth_getTransactionCount") {
      _ = try await programs.windDown(windDown)
    }
    try await record("signed wind down", method: "eth_getTransactionCount") {
      _ = try await programs.windDown(
        windDown, signedActivity: nativeTransportHex(windDownVector.signed_activity_hex),
        idempotencyKey: IdempotencyKey(windDownVector.idempotency_key_hex))
    }
    XCTAssertEqual(RecordingProgramHTTP.requests.snapshot().count, 17)

    let token = try AccessToken(Data("recording-transport".utf8))
    defer { token.destroy() }
    let retired = try AgentHTTPTransport(baseURL: endpoint, session: session, accessToken: token)
    let operations = PlatformOperation.allCases.filter { $0.descriptor.name.hasPrefix("program.") }
    XCTAssertEqual(
      Set(operations.map { $0.descriptor.name }),
      [
        "program.call", "program.simulate", "program.discover", "program.interface",
        "program.receipt", "program.activity", "program.deploy", "program.upgrade",
        "program.wind-down",
      ])
    for operation in operations {
      do {
        _ = try await retired.send(
          .init(
            operation: operation, request: .emptyObject, pathParameters: [:],
            idempotencyKey: operation.descriptor.requiresIdempotency ? key : nil))
        XCTFail("retired REST transport accepted \(operation.descriptor.name)")
      } catch let error as PlatformSDKError {
        XCTAssertEqual(error.code, .unavailableCapability)
        XCTAssertEqual(error.retry, .never)
      }
    }
    XCTAssertEqual(RecordingProgramHTTP.requests.snapshot().count, 17)
  }
}

private final class RecordingProgramRequests: @unchecked Sendable {
  private let lock = NSLock()
  private var values: [URLRequest] = []

  func append(_ request: URLRequest) {
    lock.lock()
    defer { lock.unlock() }
    values.append(request)
  }

  func reset() {
    lock.lock()
    defer { lock.unlock() }
    values.removeAll()
  }

  func snapshot() -> [URLRequest] {
    lock.lock()
    defer { lock.unlock() }
    return values
  }
}

private final class RecordingProgramHTTP: URLProtocol, @unchecked Sendable {
  static let requests = RecordingProgramRequests()

  override class func canInit(with request: URLRequest) -> Bool { true }
  override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

  override func startLoading() {
    Self.requests.append(request)
    client?.urlProtocol(self, didFailWithError: URLError(.networkConnectionLost))
  }

  override func stopLoading() {}
}
