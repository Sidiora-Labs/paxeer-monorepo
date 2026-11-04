import Crypto
import Foundation
import LayerXMobile
import LayerXMobileSampleKit
import LayerXSDK
import XCTest

#if canImport(FoundationNetworking)
  import FoundationNetworking
#endif
#if os(Linux)
  import Glibc
#else
  import Darwin
#endif

// Task 22.33.630 repair record, audit 22.30.163 finding 061:
// testFinding061CurrentHumanCompletionThroughConfiguredWallet proves Human JSON ->
// HTTP/session -> WalletModel -> mobile-configured gate -> receipt -> visible completion.
// testFinding061WalletRefusesUntrustedTamperedOrDifferentPayment proves refusal on that path.
// testFinding061WalletRequiresConfiguredReceiptAuthority proves sample pin wiring is required.
// ReceiptGateTests retains its direct signature and tamper assertions and adds the completion case.
// Baseline tests: UNRUN; the explicit gate order forbids testing before implementation.
// The exact qualified revision and private gate log are recorded by hf-done and the final evidence.
// Wiring request: sdk/integrations/ios/Sources/LayerXMobileSampleKit/WalletModel.swift,
// owner domain 15 (mobile samples): use mobile.gate() for authenticated Human evidence.get
// instead of requiring the sample relay; this task proves the existing configured relay path.
final class Repair2233630Tests: XCTestCase {
  func testFinding061CurrentHumanCompletionThroughConfiguredWallet() async throws {
    try await RecordedHumanWallet2233630.assertCompletion()
  }

  func testFinding061WalletRefusesUntrustedTamperedOrDifferentPayment() async throws {
    try await RecordedHumanWallet2233630.assertRefusals()
  }

  func testFinding061WalletRequiresConfiguredReceiptAuthority() throws {
    try RecordedHumanWallet2233630.assertMissingAuthority()
  }
}

// Replay the committed Human golden envelopes and signed sequencer recording.
// Only evidence identifiers are bound to the recording's SHA-256 identity, as in
// production_reads::native_journey_json; the payment is never fabricated or re-signed.
enum RecordedHumanWallet2233630 {
  private struct Fixture: Decodable {
    let cases: [Vector]
  }

  private struct Vector: Decodable {
    struct Key: Codable {
      let keyId: String
      let publicKey: String
      let firstHeight: UInt64
      let lastHeight: UInt64
    }

    let id: String
    let rawTransaction: String
    let signedReceipt: String
    let receiptJson: [String: JSONValue]
    let receiptDigest: String
    let prevSeqHash: String
    let pinnedKey: Key

    var relayResponse: JSONValue {
      .object([
        "raw_transaction": .string(rawTransaction), "signed_receipt": .string(signedReceipt),
        "receipt_key": .object([
          "key_id": .string(String(pinnedKey.keyId.dropFirst(2))),
          "public_key": .string(String(pinnedKey.publicKey.dropFirst(2))),
          "first_height": .integer(Int64(pinnedKey.firstHeight)),
          "last_height": .integer(Int64(pinnedKey.lastHeight)),
        ]),
        "receipt_json": .object(receiptJson),
        "prev_seq_hash": .string(String(prevSeqHash.dropFirst(2))),
      ])
    }

    func bindingEvidence(in envelope: JSONValue) throws -> JSONValue {
      let reference = try receiptReference()
      func bind(_ value: JSONValue) -> JSONValue {
        switch value {
        case let .object(fields):
          return .object(
            fields.mapValues(bind).merging(
              fields["evidence_id"] == nil ? [:] : ["evidence_id": .string(reference)],
              uniquingKeysWith: { _, recorded in recorded }))
        case let .array(values):
          return .array(values.map(bind))
        default:
          return value
        }
      }
      return bind(envelope)
    }

    func receiptReference() throws -> String {
      XCTAssertTrue(signedReceipt.hasPrefix("0x"))
      let characters = Array(signedReceipt.dropFirst(2))
      XCTAssertFalse(characters.isEmpty)
      XCTAssertEqual(characters.count % 2, 0)
      let bytes = try Data(
        stride(from: 0, to: characters.count, by: 2).map {
          try XCTUnwrap(UInt8(String(characters[$0...($0 + 1)]), radix: 16))
        })
      return "evd_" + hexadecimal(Data(SHA256.hash(data: bytes)))
    }
  }

  private static var vectorURL: URL {
    URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent()
      .appendingPathComponent("platform/conformance/fixtures/vector-v1-receipted-positive.json")
  }

  private static var goldenURL: URL {
    vectorURL.deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("services/human/schema/human-api/golden")
  }

  private static func vector() throws -> Vector {
    let fixture = try JSONDecoder().decode(Fixture.self, from: Data(contentsOf: vectorURL))
    return try XCTUnwrap(fixture.cases.first { $0.id == "V1-pax-transfer" })
  }

  private static func configuration(baseURL: URL, key: Vector.Key?) throws
    -> PublishableConfiguration
  {
    var environment = [
      "LAYERX_SERVICE_URL": baseURL.absoluteString,
      "LAYERX_SESSION_BROKER_URL": baseURL.appendingPathComponent("session").absoluteString,
      "LAYERX_EVENT_KEY_ID": "events",
      "LAYERX_EVENT_PUBLIC_KEY": String(repeating: "01", count: 32),
    ]
    if let key {
      environment["LAYERX_RECEIPT_KEYS_JSON"] = String(
        decoding: try JSONEncoder().encode([key]), as: UTF8.self)
    }
    return try SampleEnvironment.configuration(from: environment)
  }

  private static func expectation() throws -> MobileSettlementExpectation {
    try MobileSettlementExpectation(
      asset: RelayReceiptResolver.hex32(
        "0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73"),
      recipient: RelayReceiptResolver.hex32(
        "0000000000000000000000008ba1f109551bd432803012645ac136ddd64dba72"),
      amount: "1000000000000000000"
    )
  }

  static func assertCompletion() async throws {
    let vector = try vector()
    let snapshot = try await payment(key: vector.pinnedKey, pollCompletion: true)
    XCTAssertNil(snapshot.refusal)
    XCTAssertEqual(
      snapshot.settlement,
      .verified(level: "receipted", receiptDigest: String(vector.receiptDigest.dropFirst(2))))
  }

  static func assertRefusals() async throws {
    let vector = try vector()
    let key = vector.pinnedKey
    let other = try Curve25519.Signing.PrivateKey(
      rawRepresentation: Data(repeating: 0x99, count: 32))
    let wrongKey = Vector.Key(
      keyId: key.keyId, publicKey: "0x" + hexadecimal(other.publicKey.rawRepresentation),
      firstHeight: key.firstHeight, lastHeight: key.lastHeight)
    XCTAssertNotEqual(wrongKey.publicKey, key.publicKey)
    let expected = try expectation()
    let differentPayments = [
      try MobileSettlementExpectation(
        asset: Data(repeating: 0xff, count: 32), recipient: expected.recipient,
        amount: expected.amount),
      try MobileSettlementExpectation(
        asset: expected.asset, recipient: Data(repeating: 0xff, count: 32), amount: expected.amount),
      try MobileSettlementExpectation(
        asset: expected.asset, recipient: expected.recipient, amount: "1"),
    ]
    var snapshots = [
      try await payment(key: wrongKey),
      try await payment(key: key, damageSignature: true),
    ]
    for mismatch in differentPayments {
      snapshots.append(try await payment(key: key, expecting: mismatch))
    }
    for (index, snapshot) in snapshots.enumerated() {
      XCTAssertNil(snapshot.settlement, "Invalid payment case \(index) must not display completion")
      XCTAssertEqual(
        snapshot.refusal, MobileErrorCode.verificationFailure.rawValue, "case \(index)")
    }
  }

  static func assertMissingAuthority() throws {
    let session = SampleEnvironment.session(timeoutSeconds: 1)
    defer { session.invalidateAndCancel() }
    let baseURL = try XCTUnwrap(URL(string: "http://127.0.0.1:1"))
    let mobile = try LayerXMobile(
      configuration: configuration(baseURL: baseURL, key: nil), session: session,
      deliveries: InMemoryEventDeliveryStore())
    let receipts = try RelayReceiptResolver(
      relayURL: baseURL.appendingPathComponent("receipts"), session: session)
    XCTAssertTrue(mobile.configuration.receiptKeys.isEmpty)
    XCTAssertThrowsError(try WalletModel(mobile: mobile, receipts: receipts)) {
      XCTAssertEqual($0 as? MobileIntegrationError, MobileIntegrationError(.invalidConfiguration))
    }
  }

  private static func payment(
    key: Vector.Key,
    damageSignature: Bool = false,
    expecting expected: MobileSettlementExpectation? = nil,
    pollCompletion: Bool = false
  ) async throws -> WalletSnapshot {
    let replay = try ReceiptReplay(
      fixtureURL: vectorURL, goldenURL: goldenURL, damageSignature: damageSignature)
    defer { replay.stop() }
    let session = SampleEnvironment.session(timeoutSeconds: 5)
    defer { session.invalidateAndCancel() }
    let mobile = try LayerXMobile(
      configuration: configuration(baseURL: replay.baseURL, key: key), session: session,
      deliveries: InMemoryEventDeliveryStore())
    XCTAssertEqual(mobile.configuration.receiptKeys.count, 1)
    let receipts = try RelayReceiptResolver(
      relayURL: replay.baseURL.appendingPathComponent("receipts"), session: session)
    let model = try WalletModel(mobile: mobile, receipts: receipts)
    let initial = await model.current()
    XCTAssertNil(initial.settlement)
    XCTAssertNil(initial.refusal)
    func recorded(_ name: String) throws -> JSONValue {
      try JSONDecoder().decode(
        JSONValue.self, from: Data(contentsOf: goldenURL.appendingPathComponent(name)))
    }
    let request = try recorded("move.quote.request.json")
    let commit = try recorded("move.commit.request.json")
    let completion = try XCTUnwrap(
      try recorded("move.commit.response.json").objectValue?["body"]?.objectValue?["result"])
    XCTAssertEqual(completion.objectValue?["state"]?.stringValue, "done")
    guard case let .array(references)? = completion.objectValue?["evidence"] else {
      return try XCTUnwrap(nil as WalletSnapshot?, "Recorded Human completion must carry evidence")
    }
    XCTAssertFalse(references.isEmpty)
    let journeyID = try XCTUnwrap(completion.objectValue?["journey_id"]?.stringValue)
    let idempotency = try XCTUnwrap(
      commit.objectValue?["headers"]?.objectValue?["Idempotency-Key"]?.stringValue)
    let expected = try expected ?? expectation()
    let snapshot = await model.pay(
      quote: try XCTUnwrap(request.objectValue?["body"]), expecting: expected,
      idempotencyKey: try XCTUnwrap(IdempotencyKey(rawValue: idempotency)))
    let current = await model.current()
    XCTAssertEqual(current, snapshot)
    let receiptPath = "GET /receipts/" + (try vector().receiptReference())
    var expectedRequests = [
      "POST /session", "POST /v1/moves/quote", "POST /v1/moves", receiptPath,
    ]
    if pollCompletion {
      let polled = await model.awaitSettlement(
        journeyID: journeyID, expecting: expected, attempts: 1,
        wait: { _ in XCTFail("A recorded completed Journey must not wait or remain pending") })
      XCTAssertEqual(polled, snapshot)
      let current = await model.current()
      XCTAssertEqual(current, polled)
      expectedRequests += ["GET /v1/journeys/" + journeyID, receiptPath]
    }
    let (data, response) = try await session.data(
      from: replay.baseURL.appendingPathComponent("observations"))
    XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
    XCTAssertEqual(try JSONDecoder().decode([String].self, from: data), expectedRequests)
    return snapshot
  }

  private final class ReceiptReplay: @unchecked Sendable {
    let baseURL: URL
    private let listener: Int32
    private let lock = NSLock()
    private var observations: [String] = []
    private var stopped = false
    private let finished = DispatchSemaphore(value: 0)
    private let quoteRequest: JSONValue
    private let quoteResponse: JSONValue
    private let commitRequest: JSONValue
    private let commitResponse: JSONValue
    private let receiptReference: String
    private let receipt: JSONValue

    init(fixtureURL: URL, goldenURL: URL, damageSignature: Bool) throws {
      func recorded(_ name: String) throws -> JSONValue {
        try JSONDecoder().decode(
          JSONValue.self, from: Data(contentsOf: goldenURL.appendingPathComponent(name)))
      }
      quoteRequest = try recorded("move.quote.request.json")
      quoteResponse = try recorded("move.quote.response.json")
      commitRequest = try recorded("move.commit.request.json")
      let fixture = try JSONDecoder().decode(
        Fixture.self, from: Data(contentsOf: fixtureURL))
      let vector = try XCTUnwrap(fixture.cases.first { $0.id == "V1-pax-transfer" })
      receiptReference = try vector.receiptReference()
      commitResponse = try vector.bindingEvidence(in: recorded("move.commit.response.json"))
      var material = try XCTUnwrap(vector.relayResponse.objectValue)
      if damageSignature {
        var signed = vector.signedReceipt
        signed = String(signed.dropLast()) + (signed.last == "0" ? "1" : "0")
        material["signed_receipt"] = .string(signed)
      }
      receipt = .object(material)
      #if os(Linux)
        let listener = socket(AF_INET, Int32(SOCK_STREAM.rawValue), 0)
      #else
        let listener = socket(AF_INET, SOCK_STREAM, 0)
      #endif
      guard listener >= 0 else { throw POSIXError(.EIO) }
      var address = sockaddr_in()
      address.sin_family = sa_family_t(AF_INET)
      address.sin_port = 0
      address.sin_addr.s_addr = inet_addr("127.0.0.1")
      let bound = withUnsafePointer(to: &address) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
          bind(listener, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
        }
      }
      guard bound == 0, listen(listener, 16) == 0 else {
        close(listener)
        throw POSIXError(.EIO)
      }
      var length = socklen_t(MemoryLayout<sockaddr_in>.size)
      let named = withUnsafeMutablePointer(to: &address) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { getsockname(listener, $0, &length) }
      }
      guard named == 0,
        let url = URL(string: "http://127.0.0.1:\(UInt16(bigEndian: address.sin_port))")
      else {
        close(listener)
        throw POSIXError(.EIO)
      }
      self.listener = listener
      baseURL = url
      DispatchQueue(label: "repair-2233630-human-replay").async { [self] in
        defer { finished.signal() }
        serve()
      }
    }

    func stop() {
      lock.lock()
      guard !stopped else {
        lock.unlock()
        return
      }
      stopped = true
      lock.unlock()
      _ = shutdown(listener, Int32(SHUT_RDWR))
      close(listener)
      _ = finished.wait(timeout: .now() + 10)
    }

    private func serve() {
      while true {
        let connection = accept(listener, nil, nil)
        if connection < 0 { return }
        defer { close(connection) }
        var timeout = timeval(tv_sec: 5, tv_usec: 0)
        _ = setsockopt(
          connection, SOL_SOCKET, SO_RCVTIMEO, &timeout,
          socklen_t(MemoryLayout<timeval>.size))
        var request = Data()
        var buffer = [UInt8](repeating: 0, count: 4096)
        let separator = Data("\r\n\r\n".utf8)
        while request.range(of: separator) == nil && request.count < 65_536 {
          let count = recv(connection, &buffer, buffer.count, 0)
          if count <= 0 { break }
          request.append(contentsOf: buffer.prefix(count))
        }
        guard let header = request.range(of: separator) else { continue }
        let lines = String(decoding: request[..<header.lowerBound], as: UTF8.self)
          .components(separatedBy: "\r\n")
        let fields = lines[0].split(separator: " ")
        guard fields.count == 3 else { continue }
        let method = String(fields[0])
        let path = String(fields[1])
        var headers: [String: String] = [:]
        for line in lines.dropFirst() {
          let parts = line.split(separator: ":", maxSplits: 1)
          if parts.count == 2 {
            headers[parts[0].lowercased()] = parts[1].trimmingCharacters(in: .whitespaces)
          }
        }
        let contentLength = Int(headers["content-length"] ?? "0") ?? 0
        guard contentLength >= 0, contentLength <= 65_536 else { continue }
        while request.count < header.upperBound + contentLength {
          let count = recv(connection, &buffer, buffer.count, 0)
          if count <= 0 { break }
          request.append(contentsOf: buffer.prefix(count))
        }
        guard request.count == header.upperBound + contentLength else { continue }
        let requestBody = try? JSONDecoder().decode(
          JSONValue.self, from: Data(request[header.upperBound...]))
        let response: JSONValue
        let status: Int
        if method == "GET", path == "/observations" {
          response = .array(observations.map(JSONValue.string))
          status = 200
        } else {
          observations.append(method + " " + path)
          if method == "POST", path == "/session",
            requestBody == .object(["audience": .string("layerx-human-api")])
          {
            let now = Int64(Date().timeIntervalSince1970 * 1000)
            response = .object([
              "session_token": .string("receipt-replay-session"),
              "issued_at_ms": .integer(now), "expires_at_ms": .integer(now + 600_000),
            ])
            status = 200
          } else if method == "GET", path == "/receipts/" + receiptReference {
            response = receipt
            status = 200
          } else if headers["authorization"] == "Bearer receipt-replay-session", method == "GET",
            let journey = commitResponse.objectValue?["body"]?.objectValue?["result"],
            let journeyID = journey.objectValue?["journey_id"]?.stringValue,
            path == "/v1/journeys/" + journeyID,
            let body = commitResponse.objectValue?["body"]
          {
            response = body
            status = 200
          } else if headers["authorization"] == "Bearer receipt-replay-session", method == "POST",
            path == quoteRequest.objectValue?["path"]?.stringValue,
            requestBody == quoteRequest.objectValue?["body"],
            let body = quoteResponse.objectValue?["body"]
          {
            response = body
            status = 200
          } else if headers["authorization"] == "Bearer receipt-replay-session", method == "POST",
            path == commitRequest.objectValue?["path"]?.stringValue,
            requestBody == commitRequest.objectValue?["body"],
            headers["idempotency-key"]
              == commitRequest.objectValue?["headers"]?
              .objectValue?["Idempotency-Key"]?.stringValue,
            let body = commitResponse.objectValue?["body"]
          {
            response = body
            status = 200
          } else {
            response = .object(["error": .string("unexpected request")])
            status = 400
          }
        }
        guard let data = try? JSONEncoder().encode(response) else { return }
        var output = Data(
          "HTTP/1.1 \(status) \(status == 200 ? "OK" : "Bad Request")\r\nContent-Type: application/json\r\nContent-Length: \(data.count)\r\nConnection: close\r\n\r\n"
            .utf8)
        output.append(data)
        output.withUnsafeBytes { bytes in
          var sent = 0
          while sent < bytes.count {
            #if os(Linux)
              let count = send(
                connection, bytes.baseAddress!.advanced(by: sent),
                bytes.count - sent, Int32(MSG_NOSIGNAL))
            #else
              let count = send(
                connection, bytes.baseAddress!.advanced(by: sent),
                bytes.count - sent, 0)
            #endif
            if count <= 0 { break }
            sent += count
          }
        }
      }
    }
  }
}
