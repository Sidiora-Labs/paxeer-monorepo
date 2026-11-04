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

final class Repair2233618Tests: XCTestCase {
  private static let timestamp: Int64 = 1_750_000_000_000

  private final class Clock: @unchecked Sendable {
    private let lock = NSLock()
    private var timestamp = Repair2233618Tests.timestamp
    func read() -> Int64 {
      lock.lock()
      defer { lock.unlock() }
      return timestamp
    }
    func advance(_ milliseconds: Int64) {
      lock.lock()
      defer { lock.unlock() }
      timestamp += milliseconds
    }
  }

  private actor Handler {
    private var identifiers: [String] = []
    private var resume: CheckedContinuation<Void, Never>?
    func record(_ identifier: String) { identifiers.append(identifier) }
    func hold(_ identifier: String, entered: XCTestExpectation) async {
      identifiers.append(identifier)
      await withCheckedContinuation { continuation in
        resume = continuation
        entered.fulfill()
      }
    }
    func finish() {
      resume?.resume()
      resume = nil
    }
    func values() -> [String] { identifiers }
  }

  private struct Delivery: Sendable {
    let body: Data
    let event: JSONValue
    let headers: EventEnvelopeHeaders
    let publicKey: Data
    var digest: String { Data(SHA256.hash(data: body)).map { String(format: "%02x", $0) }.joined() }
    func consumer(_ store: EventDeliveryStore, clock: Clock) throws -> VerifiedEventConsumer {
      try VerifiedEventConsumer(
        publicKeys: [headers.keyID: publicKey], deliveries: store,
        leaseMilliseconds: 60_000, now: { clock.read() })
    }
  }

  // The receipt is emitted by the conformance producer named in this committed
  // vector's provenance. Only the outer webhook signature belongs to the peer.
  private func delivery(_ identifier: String) async throws -> Delivery {
    let vectorURL = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent()
      .appendingPathComponent("platform/conformance/fixtures/vector-v1-receipted-positive.json")
    let vector = try JSONDecoder().decode(JSONValue.self, from: Data(contentsOf: vectorURL))
    guard case let .array(cases) = vector.objectValue?["cases"] else {
      throw MobileIntegrationError(.decodeFailure)
    }
    let recorded = try XCTUnwrap(cases.first?.objectValue?["receiptJson"])
    XCTAssertEqual(recorded.objectValue?["rung"]?.stringValue, "receipted")
    XCTAssertEqual(recorded.objectValue?["receipt"], cases.first?.objectValue?["signedReceipt"])
    let body = try JSONEncoder().encode(recorded)
    let key = try Curve25519.Signing.PrivateKey(rawRepresentation: Data(repeating: 0x61, count: 32))
    let timestamp = String(Self.timestamp / 1_000)
    var message = Data("\(identifier).\(timestamp).".utf8)
    message.append(body)
    let fields = [
      EventEnvelopeHeaders.idHeader: identifier,
      EventEnvelopeHeaders.timestampHeader: timestamp,
      EventEnvelopeHeaders.keyIDHeader: "whk_repair2233618",
      EventEnvelopeHeaders.signatureHeader: "v1="
        + (try key.signature(for: message)).base64EncodedString(),
    ]
    let replay = try ReceiptReplay(body: body, fields: fields)
    defer { replay.stop() }
    let session = URLSession(configuration: .ephemeral)
    defer { session.invalidateAndCancel() }
    let (received, response) = try await session.data(from: replay.url)
    let http = try XCTUnwrap(response as? HTTPURLResponse)
    XCTAssertEqual(http.statusCode, 200)
    XCTAssertEqual(received, body)
    var receivedFields: [String: String] = [:]
    for name in fields.keys {
      receivedFields[name] = try XCTUnwrap(http.value(forHTTPHeaderField: name))
    }
    return Delivery(
      body: received, event: recorded, headers: try EventEnvelopeHeaders(fields: receivedFields),
      publicKey: key.publicKey.rawRepresentation)
  }

  private func ledgerURL() throws -> URL {
    let url = try FileEventDeliveryStore.applicationSupportURL(
      applicationIdentifier: "repair2233618." + UUID().uuidString)
    addTeardownBlock {
      if FileManager.default.fileExists(atPath: url.deletingLastPathComponent().path) {
        try FileManager.default.removeItem(at: url.deletingLastPathComponent())
      }
    }
    return url
  }

  func test021PackagedJSONAndPlistCredentialsAreRefusedWithoutValuesInDiagnostics() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }
    let jsonSecret = "sk-proj-" + String(repeating: "testCredential", count: 3)
    let plistSecret = "ghp_" + String(repeating: "packagedCredential", count: 2)
    let json = try JSONSerialization.data(withJSONObject: ["resource": ["credential": jsonSecret]])
    let plist = try PropertyListSerialization.data(
      fromPropertyList: ["resource": ["credential": plistSecret]], format: .xml, options: 0)
    let attribute = Data("<resource credential=\"\(plistSecret)\"/>".utf8)
    let artifacts = [
      ("config.json", json, jsonSecret, "openai-key"),
      ("Info.plist", plist, plistSecret, "github-token"),
      ("attribute.plist", attribute, plistSecret, "github-token"),
    ]
    for (name, bytes, _, _) in artifacts {
      try bytes.write(to: root.appendingPathComponent(name))
    }
    let findings = try EmbeddedSecretDetector.scan(directoryAt: root, exempt: [])
    XCTAssertFalse(findings.isEmpty, "Packaged artifacts containing credentials must be refused")
    for (name, bytes, secret, rule) in artifacts {
      let finding = try XCTUnwrap(findings.first { $0.path == name && $0.rule == rule })
      XCTAssertEqual(
        Data(bytes.dropFirst(finding.offset).prefix(finding.length)), Data(secret.utf8))
    }
    let diagnostics = findings.map(\.description).joined(separator: "\n")
    XCTAssertFalse(diagnostics.contains(jsonSecret))
    XCTAssertFalse(diagnostics.contains(plistSecret))
    XCTAssertTrue(diagnostics.contains("rule=openai-key"))
    XCTAssertTrue(diagnostics.contains("rule=github-token"))
    let clean = Data("{\"label\":\"ordinary-resource\",\"version\":1}".utf8)
    XCTAssertTrue(
      EmbeddedSecretDetector.scan(contentsOf: clean, path: "clean.json", textual: true, exempt: [])
        .isEmpty)
  }

  func test021TokensAcrossPrintableRunBoundsAndDeclaredPublicExemptions() throws {
    let credential = "lxs_" + String(repeating: "testCredential", count: 4)
    let prefix = "{\"padding\":\"" + String(repeating: "a", count: 8_160) + "\",\"value\":\""
    let data = Data((prefix + credential + "\"}").utf8)
    let findings = EmbeddedSecretDetector.scan(
      contentsOf: data, path: "large.json", textual: true, exempt: [])
    let finding = try XCTUnwrap(findings.first { $0.rule == "layerx-service-secret" })
    XCTAssertEqual(finding.offset, prefix.utf8.count)
    XCTAssertEqual(finding.length, credential.utf8.count)
    let publicValues = [
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/",
      Data((0..<32).map { UInt8($0) }).base64EncodedString(),
    ]
    XCTAssertTrue(publicValues[1].hasSuffix("="))
    for publicValue in publicValues {
      let wrapped = Data("{\"public\":\"\(publicValue)\",\"credential\":\"\(credential)\"}".utf8)
      let exempted = EmbeddedSecretDetector.scan(
        contentsOf: wrapped, path: "keys.json", textual: true, exempt: [publicValue])
      XCTAssertFalse(exempted.contains { $0.rule == "high-entropy-material" })
      XCTAssertTrue(exempted.contains { $0.rule == "layerx-service-secret" })
    }
  }

  func test042CapacityOneOverlappingSignedDeliveriesRunHandlerOnceAndRetainCompletion() async throws
  {
    let first = try await delivery("evt_capacity_first")
    let second = try await delivery("evt_capacity_second")
    let clock = Clock()
    let store = InMemoryEventDeliveryStore(capacity: 1, now: { clock.read() })
    let consumer = try first.consumer(store, clock: clock)
    let handler = Handler()
    let entered = expectation(description: "first signed delivery entered the handler")
    let running = Task {
      try await consumer.consume(rawBody: first.body, headers: first.headers) { event, id in
        XCTAssertEqual(event, first.event)
        await handler.hold(id, entered: entered)
      }
    }
    await fulfillment(of: [entered], timeout: 5)
    do {
      _ = try await consumer.consume(rawBody: second.body, headers: second.headers) { _, id in
        await handler.record(id)
      }
      XCTFail("A full store must apply backpressure")
    } catch let error as MobileIntegrationError {
      XCTAssertEqual(error.code, .deliveryStoreFailure)
    }
    let overlap = try await consumer.consume(rawBody: first.body, headers: first.headers) { _, id in
      await handler.record(id)
    }
    XCTAssertEqual(overlap, .processing)
    await handler.finish()
    let result = try await running.value
    XCTAssertEqual(result, .processed)
    clock.advance(120_000)
    let duplicate = try await consumer.consume(rawBody: first.body, headers: first.headers) {
      _, id in
      await handler.record(id)
    }
    XCTAssertEqual(duplicate, .duplicate)
    do {
      _ = try await consumer.consume(rawBody: second.body, headers: second.headers) { _, id in
        await handler.record(id)
      }
      XCTFail("Completed-claim retention must also apply backpressure")
    } catch let error as MobileIntegrationError {
      XCTAssertEqual(error.code, .deliveryStoreFailure)
    }
    let handled = await handler.values()
    XCTAssertEqual(handled, [first.headers.id])
  }

  func test042ExpiredAndReleasedIncompleteClaimsFreeCapacity() async throws {
    let first = try await delivery("evt_expired_first")
    let second = try await delivery("evt_expired_second")
    let clock = Clock()
    let store = InMemoryEventDeliveryStore(capacity: 1, now: { clock.read() })
    let claimed = try await store.claim(
      deliveryID: first.headers.id, payloadDigest: first.digest,
      leaseUntilMilliseconds: clock.read() + 10)
    XCTAssertEqual(claimed, .claimed)
    clock.advance(10)
    let next = try await store.claim(
      deliveryID: second.headers.id, payloadDigest: second.digest,
      leaseUntilMilliseconds: clock.read() + 60_000)
    XCTAssertEqual(next, .claimed)
    try await store.release(deliveryID: second.headers.id, payloadDigest: second.digest)
    let handler = Handler()
    let result = try await first.consumer(store, clock: clock).consume(
      rawBody: first.body, headers: first.headers
    ) { event, id in
      XCTAssertEqual(event, first.event)
      await handler.record(id)
    }
    XCTAssertEqual(result, .processed)
    let handled = await handler.values()
    XCTAssertEqual(handled, [first.headers.id])
  }

  func test020TwoOpenInstancesPreserveClaimsAndCompletedDeliveriesOnReopen() async throws {
    let first = try await delivery("evt_shared_first")
    let second = try await delivery("evt_shared_second")
    let clock = Clock()
    let url = try ledgerURL()
    let left = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    let right = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    let leftConsumer = try first.consumer(left, clock: clock)
    let rightConsumer = try first.consumer(right, clock: clock)
    let handler = Handler()
    let entered = expectation(description: "first app holds a durable claim")
    let running = Task {
      try await leftConsumer.consume(rawBody: first.body, headers: first.headers) { event, id in
        XCTAssertEqual(event, first.event)
        await handler.hold(id, entered: entered)
      }
    }
    await fulfillment(of: [entered], timeout: 5)
    let overlap = try await rightConsumer.consume(rawBody: first.body, headers: first.headers) {
      _, id in
      await handler.record(id)
    }
    XCTAssertEqual(overlap, .processing)
    let independent = try await rightConsumer.consume(rawBody: second.body, headers: second.headers)
    { event, id in
      XCTAssertEqual(event, second.event)
      await handler.record(id)
    }
    XCTAssertEqual(independent, .processed)
    await handler.finish()
    let completed = try await running.value
    XCTAssertEqual(completed, .processed)
    let reopened = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    for store in [left, right, reopened] {
      for capture in [first, second] {
        let duplicate = try await capture.consumer(store, clock: clock).consume(
          rawBody: capture.body, headers: capture.headers
        ) { _, id in
          await handler.record(id)
        }
        XCTAssertEqual(duplicate, .duplicate)
      }
    }
    let handled = await handler.values()
    XCTAssertEqual(handled, [first.headers.id, second.headers.id])
    for path in [url.path, url.path + ".lock"] {
      let attributes = try FileManager.default.attributesOfItem(atPath: path)
      XCTAssertEqual((attributes[.posixPermissions] as? NSNumber)?.intValue, 0o600)
    }
  }

  func test020ConcurrentInstancesAdmitOnlyOneSignedDelivery() async throws {
    let capture = try await delivery("evt_concurrent_claim")
    let clock = Clock()
    let url = try ledgerURL()
    let stores = try (0..<16).map { _ in
      try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    }
    let handler = Handler()
    let outcomes = try await withThrowingTaskGroup(of: EventConsumeOutcome.self) { group in
      for store in stores {
        group.addTask {
          try await capture.consumer(store, clock: clock).consume(
            rawBody: capture.body, headers: capture.headers
          ) { event, id in
            XCTAssertEqual(event, capture.event)
            await handler.record(id)
          }
        }
      }
      var values: [EventConsumeOutcome] = []
      for try await value in group { values.append(value) }
      return values
    }
    XCTAssertEqual(outcomes.count, 16)
    XCTAssertEqual(outcomes.filter { $0 == .processed }.count, 1)
    XCTAssertTrue(outcomes.allSatisfy { [.processed, .processing, .duplicate].contains($0) })
    let handled = await handler.values()
    XCTAssertEqual(handled, [capture.headers.id])
  }

  private func abandonClaim(_ capture: Delivery, url: URL, clock: Clock) async throws {
    let store = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    let claim = try await store.claim(
      deliveryID: capture.headers.id, payloadDigest: capture.digest,
      leaseUntilMilliseconds: clock.read() + 60_000)
    XCTAssertEqual(claim, .claimed)
    // Dropping this instance without complete/release models termination
    // after durable admission and before applying the handler's effect.
  }

  private func completeRecovered(_ capture: Delivery, url: URL, clock: Clock, handler: Handler)
    async throws
  {
    let store = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    let result = try await capture.consumer(store, clock: clock).consume(
      rawBody: capture.body, headers: capture.headers
    ) { event, id in
      XCTAssertEqual(event, capture.event)
      await handler.record(id)
    }
    XCTAssertEqual(result, .processed)
  }

  func test020AbandonedClaimAndCompletedRecordSurviveCrashReopen() async throws {
    let capture = try await delivery("evt_crash_reopen")
    let clock = Clock()
    let url = try ledgerURL()
    let handler = Handler()
    try await abandonClaim(capture, url: url, clock: clock)
    do {
      let reopened = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
      let pending = try await capture.consumer(reopened, clock: clock).consume(
        rawBody: capture.body, headers: capture.headers
      ) { _, id in
        await handler.record(id)
      }
      XCTAssertEqual(pending, .processing)
      let conflict = try await reopened.claim(
        deliveryID: capture.headers.id, payloadDigest: String(repeating: "0", count: 64),
        leaseUntilMilliseconds: clock.read() + 60_000)
      XCTAssertEqual(conflict, .conflict)
    }
    // A crash before rename can leave a partial staging file. It is never
    // authoritative and must not prevent reopening the committed ledger.
    try Data("{\"version\":1,\"entries\":".utf8).write(
      to: url.deletingLastPathComponent().appendingPathComponent(".layerx-event-interrupted"))
    clock.advance(60_000)
    try await completeRecovered(capture, url: url, clock: clock, handler: handler)
    let reopened = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    let duplicate = try await capture.consumer(reopened, clock: clock).consume(
      rawBody: capture.body, headers: capture.headers
    ) { _, id in
      await handler.record(id)
    }
    XCTAssertEqual(duplicate, .duplicate)
    try await reopened.release(deliveryID: capture.headers.id, payloadDigest: capture.digest)
    let retained = try await reopened.claim(
      deliveryID: capture.headers.id, payloadDigest: capture.digest,
      leaseUntilMilliseconds: clock.read() + 60_000)
    XCTAssertEqual(retained, .completed)
    let handled = await handler.values()
    XCTAssertEqual(handled, [capture.headers.id])
  }

  func test020UnreadableLedgerDoesNotCreatePhantomClaimAndCorruptionFailsClosed() async throws {
    let capture = try await delivery("evt_failed_write")
    let clock = Clock()
    let url = try ledgerURL()
    let store = try FileEventDeliveryStore(fileURL: url, now: { clock.read() })
    try FileManager.default.createDirectory(at: url, withIntermediateDirectories: false)
    do {
      _ = try await store.claim(
        deliveryID: capture.headers.id, payloadDigest: capture.digest,
        leaseUntilMilliseconds: clock.read() + 60_000)
      XCTFail("An unreadable ledger must not admit a claim")
    } catch let error as MobileIntegrationError {
      XCTAssertEqual(error.code, .deliveryStoreFailure)
    }
    try FileManager.default.removeItem(at: url)
    let accepted = try await store.claim(
      deliveryID: capture.headers.id, payloadDigest: capture.digest,
      leaseUntilMilliseconds: clock.read() + 60_000)
    XCTAssertEqual(accepted, .claimed)
    try Data("{truncated".utf8).write(to: url)
    XCTAssertThrowsError(try FileEventDeliveryStore(fileURL: url, now: { clock.read() })) {
      XCTAssertEqual($0 as? MobileIntegrationError, MobileIntegrationError(.deliveryStoreFailure))
    }
    do {
      try await store.complete(deliveryID: capture.headers.id, payloadDigest: capture.digest)
      XCTFail("An existing instance must reload and refuse a corrupt ledger")
    } catch let error as MobileIntegrationError {
      XCTAssertEqual(error.code, .deliveryStoreFailure)
    }
  }

  private final class ReceiptReplay: @unchecked Sendable {
    let url: URL
    private let listener: Int32
    private let finished = DispatchSemaphore(value: 0)

    init(body: Data, fields: [String: String]) throws {
      #if os(Linux)
        let descriptor = socket(AF_INET, Int32(SOCK_STREAM.rawValue), 0)
      #else
        let descriptor = socket(AF_INET, SOCK_STREAM, 0)
      #endif
      guard descriptor >= 0 else { throw POSIXError(.EIO) }
      var address = sockaddr_in()
      address.sin_family = sa_family_t(AF_INET)
      address.sin_addr.s_addr = inet_addr("127.0.0.1")
      let bound = withUnsafePointer(to: &address) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
          bind(descriptor, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
        }
      }
      guard bound == 0, listen(descriptor, 1) == 0 else {
        close(descriptor)
        throw POSIXError(.EIO)
      }
      var length = socklen_t(MemoryLayout<sockaddr_in>.size)
      let named = withUnsafeMutablePointer(to: &address) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
          getsockname(descriptor, $0, &length)
        }
      }
      guard named == 0,
        let url = URL(
          string: "http://127.0.0.1:\(UInt16(bigEndian: address.sin_port))/recorded-receipt")
      else {
        close(descriptor)
        throw POSIXError(.EIO)
      }
      self.listener = descriptor
      self.url = url
      var response =
        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: \(body.count)\r\nConnection: close\r\n"
      for (name, value) in fields { response += "\(name): \(value)\r\n" }
      var output = Data((response + "\r\n").utf8)
      output.append(body)
      let responseBytes = output
      DispatchQueue(label: "repair2233618-receipt-replay").async { [self] in
        defer { finished.signal() }
        let connection = accept(listener, nil, nil)
        guard connection >= 0 else { return }
        defer { close(connection) }
        var timeout = timeval(tv_sec: 5, tv_usec: 0)
        _ = setsockopt(
          connection, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))
        var request = Data()
        var buffer = [UInt8](repeating: 0, count: 4_096)
        while request.range(of: Data("\r\n\r\n".utf8)) == nil && request.count < 65_536 {
          let count = recv(connection, &buffer, buffer.count, 0)
          guard count > 0 else { return }
          request.append(contentsOf: buffer.prefix(count))
        }
        responseBytes.withUnsafeBytes { bytes in
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
            guard count > 0 else { return }
            sent += count
          }
        }
      }
    }

    func stop() {
      _ = shutdown(listener, Int32(SHUT_RDWR))
      close(listener)
      _ = finished.wait(timeout: .now() + 5)
    }
  }
}
