import Crypto
import Foundation
import LayerXMobile
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

// Task 22.31.419, finding 032: testFinding032CompletionAfterReconnect proves
// journey.get -> authenticated evidence.get -> configured gate after reopening.
// testFinding032RejectsUntrustedAndDifferentPayments proves exact payment binding.
// testFinding032EvidenceSelectionAndStates proves normative states and evidence selection.
// Baseline not run: the task's bounded gate order requires implementation first.
// Wiring request: sdk/integrations/ios/Sources/LayerXMobileSampleKit/WalletModel.swift
// (mobile samples domain) should construct its gate with mobile.gate().
// The runner's hf-done evidence records the qualified revision; no spec file is edited here.
final class Repair2231419Tests: XCTestCase {
  private let journeyID = "jrn_2231419"

  private func decode(_ text: String) throws -> JSONValue {
    try JSONDecoder().decode(JSONValue.self, from: Data(text.utf8))
  }

  private func configuration(_ server: ReceiptTranscriptServer) throws -> PublishableConfiguration {
    let pin = try XCTUnwrap(try decode(recordedPayment).objectValue?["receiptKey"]?.objectValue)
    let key: JSONValue = .object([
      "keyId": try XCTUnwrap(pin["keyId"]), "publicKey": try XCTUnwrap(pin["publicKey"]),
      "firstHeight": .integer(1000), "lastHeight": .integer(1999),
    ])
    return try PublishableConfiguration(declaredKeys: [
      PublishableConfiguration.serviceURLKey: server.baseURL.absoluteString,
      PublishableConfiguration.sessionBrokerURLKey: server.baseURL.appendingPathComponent("broker")
        .absoluteString,
      PublishableConfiguration.eventPublicKeyPrefix + "test": String(repeating: "01", count: 32),
      PublishableConfiguration.receiptKeysKey: String(
        decoding: try JSONEncoder().encode([key]), as: UTF8.self),
    ])
  }

  private func session() -> URLSession {
    let config = URLSessionConfiguration.ephemeral
    config.timeoutIntervalForRequest = 5
    config.timeoutIntervalForResource = 10
    config.connectionProxyDictionary = [:]
    return URLSession(configuration: config)
  }

  private func expected(amount: String = "1000000000000000000") throws
    -> MobileSettlementExpectation
  {
    try MobileSettlementExpectation(
      asset: bytes("0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73"),
      recipient: bytes("0000000000000000000000008ba1f109551bd432803012645ac136ddd64dba72"),
      amount: amount)
  }

  private func bytes(_ hex: String) throws -> Data {
    let digits = Array(hex)
    return try Data(
      stride(from: 0, to: digits.count, by: 2).map {
        try XCTUnwrap(UInt8(String(digits[$0...($0 + 1)]), radix: 16))
      })
  }

  private func material(_ proof: String = recordedPayment) throws -> JSONValue {
    let document = try XCTUnwrap(try decode(proof).objectValue)
    let signed = try bytes(
      String(try XCTUnwrap(document["signedReceipt"]?.stringValue).dropFirst(2)))
    return .object([
      "evidence_id": .string("evd_" + hexadecimal(Data(SHA256.hash(data: signed)))),
      "class": .string("layerx-receipt"), "verification": .string("receipt-verified"),
      "content_type": .string("application/vnd.layerx.receipt-proof-v2+json"),
      "bytes_base64": .string(Data(proof.utf8).base64EncodedString()),
      "rung": .string("receipted"), "inclusion_height": .integer(1000),
    ])
  }

  private func reference(_ material: JSONValue) throws -> JSONValue {
    let fields = try XCTUnwrap(material.objectValue)
    return .object([
      "evidence_id": try XCTUnwrap(fields["evidence_id"]),
      "class": try XCTUnwrap(fields["class"]),
      "verification": try XCTUnwrap(fields["verification"]),
    ])
  }

  private func journey(state: String, references: [JSONValue], stagesOnly: Bool = false)
    -> JSONValue
  {
    // Shape and projection from production_reads::native_journey_json over the
    // committed sequencer recording below. No receipt is created or re-signed.
    .object([
      "journey_id": .string(journeyID), "kind": .string("move"), "state": .string(state),
      "state_copy_key": .string("journey.state." + state),
      "evidence": .array(stagesOnly ? [] : references),
      "stages": .array([
        .object([
          "stage_id": .string("stg_" + journeyID + "_0"),
          "copy_key": .string("journey.stage.receipt-verified"), "state": .string(state),
          "evidence": .array(references),
        ])
      ]),
      "started_at": .string("2026-08-18T09:32:00Z"), "updated_at": .string("2026-08-18T09:32:41Z"),
    ])
  }

  private func routes(journey: JSONValue, materials: [JSONValue]) throws -> [String: JSONValue] {
    var routes = ["/v1/journeys/" + journeyID: journey]
    for material in materials {
      let id = try XCTUnwrap(material.objectValue?["evidence_id"]?.stringValue)
      routes["/v1/evidence/" + id] = material
    }
    return routes
  }

  func testFinding032CompletionAfterReconnect() async throws {
    let evidence = try material()
    let id = try XCTUnwrap(evidence.objectValue?["evidence_id"]?.stringValue)
    let pendingServer = try ReceiptTranscriptServer(
      routes: routes(
        journey: journey(state: "processing", references: []), materials: []))
    let firstSession = session()
    let firstMobile = try LayerXMobile(
      configuration: configuration(pendingServer), session: firstSession,
      deliveries: InMemoryEventDeliveryStore())
    let pending = try await firstMobile.gate().project(
      journey: firstMobile.client.journey(id: journeyID), expecting: expected())
    XCTAssertEqual(pending, .pending(reference: journeyID))
    XCTAssertEqual(pendingServer.paths, ["/broker", "/v1/journeys/" + journeyID])
    firstSession.invalidateAndCancel()
    pendingServer.stop()

    for state in ["done", "done-finalised"] {
      var fields = try XCTUnwrap(evidence.objectValue)
      if state == "done-finalised" {
        fields["verification"] = .string("checkpoint-finalised")
        fields["rung"] = .string("committed")
      }
      let completionEvidence = JSONValue.object(fields)
      let server = try ReceiptTranscriptServer(
        routes: routes(
          journey: journey(state: state, references: [reference(completionEvidence)]),
          materials: [completionEvidence]))
      defer { server.stop() }
      let reopenedSession = session()
      defer { reopenedSession.invalidateAndCancel() }
      let mobile = try LayerXMobile(
        configuration: configuration(server), session: reopenedSession,
        deliveries: InMemoryEventDeliveryStore())
      let restoredJourney = try await mobile.client.journey(id: journeyID)
      let settlement = try await mobile.gate().project(
        journey: restoredJourney, expecting: expected())
      let digest = try XCTUnwrap(
        try decode(recordedPayment).objectValue?["receiptDigest"]?.stringValue)
      XCTAssertEqual(
        settlement, .verified(level: "receipted", receiptDigest: String(digest.dropFirst(2))))
      XCTAssertEqual(server.paths, ["/broker", "/v1/journeys/" + journeyID, "/v1/evidence/" + id])
      XCTAssertEqual(server.authorizedPaths, ["/v1/journeys/" + journeyID, "/v1/evidence/" + id])
    }
  }

  func testFinding032RejectsUntrustedAndDifferentPayments() async throws {
    let original = try material()
    let id = try XCTUnwrap(original.objectValue?["evidence_id"]?.stringValue)
    let originalFields = try XCTUnwrap(original.objectValue)
    var invalid: [JSONValue] = []
    for (key, value): (String, JSONValue) in [
      ("evidence_id", .string("evd_wrong")), ("class", .string("checkpoint-proof")),
      ("verification", .string("unverified")), ("bytes_base64", .string("invalid")),
      ("rung", .string("committed")), ("inclusion_height", .integer(999)),
    ] {
      var fields = originalFields
      fields[key] = value
      invalid.append(.object(fields))
    }
    let proof = try XCTUnwrap(try decode(recordedPayment).objectValue)
    var damagedSignature = try XCTUnwrap(proof["signedReceipt"]?.stringValue)
    damagedSignature =
      String(damagedSignature.dropLast()) + (damagedSignature.last == "0" ? "1" : "0")
    for (key, value): (String, JSONValue) in [
      ("signedReceipt", .string(damagedSignature)),
      ("receiptDigest", .string("0x" + String(repeating: "00", count: 32))),
      ("inclusionHeight", .string("0x3e7")), ("rawTransaction", .string("0x01")),
    ] {
      var document = proof
      document[key] = value
      var fields = originalFields
      fields["bytes_base64"] = .string(
        try JSONEncoder().encode(JSONValue.object(document)).base64EncodedString())
      invalid.append(.object(fields))
    }
    var unpinned = proof
    var key = try XCTUnwrap(proof["receiptKey"]?.objectValue)
    key["publicKey"] = .string("0x" + String(repeating: "ff", count: 32))
    unpinned["receiptKey"] = .object(key)
    var unpinnedFields = originalFields
    unpinnedFields["bytes_base64"] = .string(
      try JSONEncoder().encode(JSONValue.object(unpinned)).base64EncodedString())
    invalid.append(.object(unpinnedFields))
    for (index, bad) in invalid.enumerated() {
      let server = try ReceiptTranscriptServer(routes: [
        "/v1/journeys/" + journeyID: journey(state: "done", references: [reference(original)]),
        "/v1/evidence/" + id: bad,
      ])
      defer { server.stop() }
      let transportSession = session()
      defer { transportSession.invalidateAndCancel() }
      let mobile = try LayerXMobile(
        configuration: configuration(server), session: transportSession,
        deliveries: InMemoryEventDeliveryStore())
      do {
        let response = try await mobile.client.journey(id: journeyID)
        _ = try await mobile.gate().project(journey: response, expecting: expected())
        XCTFail("Must refuse tampered material \(index)")
      } catch {
        XCTAssertEqual(
          error as? MobileIntegrationError, MobileIntegrationError(.verificationFailure),
          "case \(index)")
      }
      XCTAssertTrue(server.paths.contains("/v1/evidence/" + id))
    }
    let server = try ReceiptTranscriptServer(
      routes: routes(
        journey: journey(state: "done", references: [reference(original)]), materials: [original]))
    defer { server.stop() }
    let transportSession = session()
    defer { transportSession.invalidateAndCancel() }
    let mobile = try LayerXMobile(
      configuration: configuration(server), session: transportSession,
      deliveries: InMemoryEventDeliveryStore())
    let expected = try expected()
    let mismatches = [
      try MobileSettlementExpectation(
        asset: Data(repeating: 1, count: 32), recipient: expected.recipient, amount: expected.amount
      ),
      try MobileSettlementExpectation(
        asset: expected.asset, recipient: Data(repeating: 1, count: 32), amount: expected.amount),
      try self.expected(amount: "1"),
    ]
    for mismatch in mismatches {
      do {
        let response = try await mobile.client.journey(id: journeyID)
        _ = try await mobile.gate().project(journey: response, expecting: mismatch)
        XCTFail("A valid receipt for a different payment must not complete")
      } catch {
        XCTAssertEqual(
          error as? MobileIntegrationError, MobileIntegrationError(.verificationFailure))
      }
    }
  }

  func testFinding032SignatureAndOptionalTransaction() async throws {
    let proof = try XCTUnwrap(try decode(recordedPayment).objectValue)
    for tampered in [false, true] {
      var document = proof
      document.removeValue(forKey: "rawTransaction")
      if tampered {
        let signature = try XCTUnwrap(document["signedReceipt"]?.stringValue)
        document["signedReceipt"] = .string(
          String(signature.dropLast()) + (signature.last == "0" ? "1" : "0"))
      }
      // Recompute the content identifier as the producer does: the invalid case
      // reaches signature verification even though its content hash agrees.
      let text = String(
        decoding: try JSONEncoder().encode(JSONValue.object(document)), as: UTF8.self)
      let evidence = try material(text)
      let server = try ReceiptTranscriptServer(
        routes: routes(
          journey: journey(state: "done", references: [reference(evidence)]), materials: [evidence])
      )
      defer { server.stop() }
      let transportSession = session()
      defer { transportSession.invalidateAndCancel() }
      let mobile = try LayerXMobile(
        configuration: configuration(server), session: transportSession,
        deliveries: InMemoryEventDeliveryStore())
      do {
        let response = try await mobile.client.journey(id: journeyID)
        let result = try await mobile.gate().project(journey: response, expecting: expected())
        XCTAssertFalse(tampered, "An invalid signature cannot establish completion")
        let digest = try XCTUnwrap(proof["receiptDigest"]?.stringValue)
        XCTAssertEqual(
          result, .verified(level: "receipted", receiptDigest: String(digest.dropFirst(2))))
      } catch {
        XCTAssertTrue(tampered, "Human managed receipts may omit rawTransaction")
        XCTAssertEqual(
          error as? MobileIntegrationError, MobileIntegrationError(.verificationFailure))
      }
      XCTAssertEqual(server.paths.count, 3)
    }
  }

  func testFinding032EvidenceSelectionAndStates() async throws {
    let evidence = try material()
    let reference = try reference(evidence)
    let other = try material(recordedOtherPayment)
    let otherReference = try self.reference(other)
    let untrusted: JSONValue = .object([
      "evidence_id": .string("evd_local"), "class": .string("local-journey-state"),
      "verification": .string("unverified"),
    ])
    let server = try ReceiptTranscriptServer(
      routes: routes(
        journey: journey(
          state: "done", references: [untrusted, otherReference, reference], stagesOnly: true),
        materials: [evidence, other]))
    defer { server.stop() }
    let transportSession = session()
    defer { transportSession.invalidateAndCancel() }
    let mobile = try LayerXMobile(
      configuration: configuration(server), session: transportSession,
      deliveries: InMemoryEventDeliveryStore())
    let gate = try mobile.gate()
    let value = try await mobile.client.journey(id: journeyID)
    let settled = try await gate.project(journey: value, expecting: expected())
    guard case .verified(level: "receipted", receiptDigest: _) = settled else {
      return XCTFail("Stage evidence must resolve the exact payment after another valid receipt")
    }
    XCTAssertEqual(server.paths.count, 4)
    for state in ["getting-ready", "sending", "processing", "still-checking", "waiting-for-you"] {
      let state = try await gate.project(
        journey: journey(state: state, references: [reference]), expecting: expected())
      XCTAssertEqual(state, .pending(reference: journeyID))
    }
    let refused = try await gate.project(
      journey: journey(state: "refused", references: [reference]), expecting: expected())
    XCTAssertEqual(refused, .refused(code: "refused"))
    for state in ["settled", "completed", "unknown"] {
      do {
        _ = try await gate.project(
          journey: journey(state: state, references: [reference]), expecting: expected())
        XCTFail("Unknown or retired states must be refused")
      } catch {
        XCTAssertEqual(error as? MobileIntegrationError, MobileIntegrationError(.decodeFailure))
      }
    }
    for references in [[], [untrusted]] {
      do {
        _ = try await gate.project(
          journey: journey(state: "done", references: references), expecting: expected())
        XCTFail("A done label alone cannot prove a payment")
      } catch {
        XCTAssertEqual(
          error as? MobileIntegrationError, MobileIntegrationError(.verificationFailure))
      }
    }
    XCTAssertEqual(
      server.paths.count, 4, "Pending, refusal and missing evidence must not fetch material")
  }
}

// In-process HTTP transcript peer: real URLSession, HumanHTTPTransport and broker.
// No URLProtocol interception, production-component substitutes or subprocesses.
private final class ReceiptTranscriptServer: @unchecked Sendable {
  let baseURL: URL
  private let listener: Int32
  private let lock = NSLock()
  private var recordedPaths: [String] = []
  private var authenticatedPaths: [String] = []
  private var stopped = false
  var paths: [String] {
    lock.lock()
    defer { lock.unlock() }
    return recordedPaths
  }
  var authorizedPaths: [String] {
    lock.lock()
    defer { lock.unlock() }
    return authenticatedPaths
  }

  init(routes: [String: JSONValue]) throws {
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
    guard bound == 0, listen(listener, 32) == 0 else {
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
    DispatchQueue(label: "receipt-transcript").async { [self] in serve(routes) }
  }

  func stop() {
    lock.lock()
    defer { lock.unlock() }
    guard !stopped else { return }
    stopped = true
    _ = shutdown(listener, Int32(SHUT_RDWR))
    close(listener)
  }

  private func serve(_ routes: [String: JSONValue]) {
    while true {
      let connection = accept(listener, nil, nil)
      if connection < 0 { return }
      defer { close(connection) }
      var request = Data()
      var buffer = [UInt8](repeating: 0, count: 4096)
      while request.range(of: Data("\r\n\r\n".utf8)) == nil && request.count < 65_536 {
        let count = recv(connection, &buffer, buffer.count, 0)
        if count <= 0 { break }
        request.append(contentsOf: buffer.prefix(count))
      }
      let text = String(decoding: request, as: UTF8.self)
      let contentLength =
        text.components(separatedBy: "\r\n").first(where: {
          $0.lowercased().hasPrefix("content-length:")
        }).flatMap {
          Int(
            $0.split(separator: ":", maxSplits: 1).last?.trimmingCharacters(in: .whitespaces) ?? "")
        } ?? 0
      if let header = request.range(of: Data("\r\n\r\n".utf8)) {
        while request.count < header.upperBound + contentLength {
          let count = recv(connection, &buffer, buffer.count, 0)
          if count <= 0 { break }
          request.append(contentsOf: buffer.prefix(count))
        }
      }
      let path = text.components(separatedBy: " ").dropFirst().first ?? ""
      let authorized = text.lowercased().contains("authorization: bearer recorded-mobile-session")
      lock.lock()
      recordedPaths.append(path)
      if authorized { authenticatedPaths.append(path) }
      lock.unlock()
      let body: JSONValue
      let status: String
      if path == "/broker", text.hasPrefix("POST ") {
        let now = Int64(Date().timeIntervalSince1970 * 1000)
        body = .object([
          "session_token": .string("recorded-mobile-session"),
          "issued_at_ms": .integer(now), "expires_at_ms": .integer(now + 300_000),
        ])
        status = "200 OK"
      } else if authorized, text.hasPrefix("GET "), let result = routes[path] {
        body = .object(["ok": .boolean(true), "result": result, "trace": .string("trc_2231419")])
        status = "200 OK"
      } else {
        body = .object(["unexpected_request": .string(path)])
        status = "400 Bad Request"
      }
      do {
        let data = try JSONEncoder().encode(body)
        var response = Data(
          "HTTP/1.1 \(status)\r\nContent-Type: application/json\r\nContent-Length: \(data.count)\r\nConnection: close\r\n\r\n"
            .utf8)
        response.append(data)
        response.withUnsafeBytes { bytes in
          var sent = 0
          while sent < bytes.count {
            #if os(Linux)
              let count = send(
                connection, bytes.baseAddress!.advanced(by: sent), bytes.count - sent,
                Int32(MSG_NOSIGNAL))
            #else
              let count = send(
                connection, bytes.baseAddress!.advanced(by: sent), bytes.count - sent, 0)
            #endif
            if count <= 0 { break }
            sent += count
          }
        }
      } catch { return }
    }
  }
}

// Signed node recordings from vector-v1-receipted-positive.json, exported in
// production_reads::managed_receipt_material / PortableReceiptV2 field layout.
private let recordedPayment = #"""
  {
    "format": "layerx-receipt-proof-v2",
    "verificationLevel": "receipted",
    "signedReceipt": "0x00025201000200000020a52f2455a87ccb3b84a90ae4e3c9153e7122b66cd69cb080ae9ecd9732619277000000000000000100000020000000000000000000000000000000000000000000000000000000000000000000000020a48065818165fb64611dff9b6de4056284bde354442b57d953ecf798a046330d00000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000001000c000000000100000000200000000000000000000000000000000000000000000000000000000000000000000000710000000000005208000000000000520800000000773594000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000000475687078000000000000000000002632e314a0000000002000000000000000000000000000000000000000000000000000000000000003e8000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f7300000000000000000de0b6b3a764000000000020000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd7300000000000000056bc75e2d6310000000000000000000055de68146d89760000000000000000000000000200000000000000000000000008ba1f109551bd432803012645ac136ddd64dba720000000000000000000000000000000000000000000000000de0b6b3a7640000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000206b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b000001977426f6800100000040bfff5ed541997e8ce3eb29f7492d71d29689b76f85b41b422184810697d08244dcb3c8db9cfdbabd7b8fd3fd22e707f834fac3ebcb52d09be6435487f7cd190c",
    "receiptDigest": "0xa80ea169a5acd85faa9f4b4eb6016a77f2c1d96db66a1b0cc8ac217171e81560",
    "receiptKey": {
      "keyId": "0x6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b",
      "publicKey": "0x8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
      "firstHeight": "0x3e8",
      "lastHeight": "0x7cf"
    },
    "inclusionHeight": "0x3e8",
    "rawTransaction": "0x02f8727d80843b9aca0084b2d05e00825208948ba1f109551bd432803012645ac136ddd64dba72880de0b6b3a764000080c080a0c10d1d3ded8a2ac5bf5652235628abf4963e5618e1f0b374794de957bc2401cfa079c223a5baf08e5d5b17ab51b244e2d1d528df9d692adc690653eaa3ddfb0981"
  }
  """#
private let recordedOtherPayment = #"""
  {
    "format": "layerx-receipt-proof-v2",
    "verificationLevel": "receipted",
    "signedReceipt": "0x00025201000200000020f36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db000000000000000200000020a48065818165fb64611dff9b6de4056284bde354442b57d953ecf798a046330d00000020495f54c9be4c9531a0e3b2e3dc3c5919cadad7edeb4a35ec9bce39f98d1307ff00000020ec4916dd28fc4c10d78e287ca5d9cc51ee1ae73cbfde08c6b37324cbfaac8bc50000000000000002000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000b48c000000000001069400000000773594000000000200000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000020000000010475687078000c000100000300000000200000000000000000000000000000000000000000000000000000000000000000000000955fbdb2315678afecb367f032d93f642f64180aa3038c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd7300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c876913dc7284abac0b1db00384ca95defe320bf25718f15dc53e9a5d0da13de86000000000000000000005412dd5cf0000000002000000000000000000000000000000000000000000000000000000000000003e8000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f730000000000000000000000000000000000000020000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd7300000000000000055de68146d897600000000000000000055de62d33fb3a70000000000000000001000000200000000000000000000000005fbdb2315678afecb367f032d93f642f64180aa30000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000206b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b000001977426f680010000004091124eade3afbc904482fa048347e5abefbc4d8e92cb81a9af10f411b526f6b5e729760b360e4f726beace6b23cc5fda1e11a19610139989d83268980b90b902",
    "receiptDigest": "0x9790b240b3421bba4f8aead49b361487308f6679143429593dbb35751de639c0",
    "receiptKey": {
      "keyId": "0x6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b",
      "publicKey": "0x8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
      "firstHeight": "0x3e8",
      "lastHeight": "0x7cf"
    },
    "inclusionHeight": "0x3e8",
    "rawTransaction": "0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"
  }
  """#
