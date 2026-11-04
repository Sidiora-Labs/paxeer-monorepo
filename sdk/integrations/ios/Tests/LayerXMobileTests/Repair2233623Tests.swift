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

final class Repair2233623Tests: XCTestCase {
  private var repositoryURL: URL {
    var url = URL(fileURLWithPath: #filePath)
    for _ in 0..<6 { url.deleteLastPathComponent() }
    return url
  }

  private func recorded(_ name: String) throws -> JSONValue {
    let url = repositoryURL.appendingPathComponent(
      "services/human/schema/human-api/golden/\(name).response.json")
    let recording = try JSONDecoder().decode(JSONValue.self, from: Data(contentsOf: url))
    XCTAssertEqual(recording.objectValue?["status"], .integer(200))
    return try XCTUnwrap(recording.objectValue?["body"])
  }

  private func replacingResult(_ envelope: JSONValue, with result: JSONValue) throws -> JSONValue {
    var fields = try XCTUnwrap(envelope.objectValue)
    fields["result"] = result
    return .object(fields)
  }

  private func refresh(
    version: JSONValue? = nil, activity: JSONValue? = nil,
    expectedPaths: [String] = ["/broker", "/v1/version", "/v1/profile", "/v1/activity/query"]
  )
    async throws -> WalletSnapshot
  {
    let server = try RefreshTranscriptServer(routes: [
      "/v1/version": try version ?? recorded("version"),
      "/v1/profile": try recorded("profile.get"),
      "/v1/activity/query": try activity ?? recorded("activity.query"),
    ])
    defer { server.stop() }
    let config = URLSessionConfiguration.ephemeral
    config.timeoutIntervalForRequest = 5
    config.timeoutIntervalForResource = 10
    config.connectionProxyDictionary = [:]
    let session = URLSession(configuration: config)
    defer { session.invalidateAndCancel() }
    let fixture = try JSONDecoder().decode(
      JSONValue.self,
      from: Data(
        contentsOf:
          repositoryURL.appendingPathComponent(
            "sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json")))
    guard case let .array(vectors)? = fixture.objectValue?["cases"] else {
      throw XCTUnwrapFailure()
    }
    let vector = try XCTUnwrap(
      vectors.first { $0.objectValue?["id"]?.stringValue == "V1-pax-transfer" })
    let key = try XCTUnwrap(vector.objectValue?["pinnedKey"])
    let configuration = try PublishableConfiguration(declaredKeys: [
      PublishableConfiguration.serviceURLKey: server.baseURL.absoluteString,
      PublishableConfiguration.sessionBrokerURLKey: server.baseURL.appendingPathComponent("broker")
        .absoluteString,
      PublishableConfiguration.eventPublicKeyPrefix + "test": String(repeating: "01", count: 32),
      PublishableConfiguration.receiptKeysKey: String(
        decoding: try JSONEncoder().encode([key]), as: UTF8.self),
    ])
    let mobile = try LayerXMobile(
      configuration: configuration, session: session,
      deliveries: InMemoryEventDeliveryStore())
    let receipts = try RelayReceiptResolver(
      relayURL: server.baseURL.appendingPathComponent("receipts"), session: session)
    let model = try WalletModel(mobile: mobile, receipts: receipts)
    let snapshot = await model.refresh()
    let current = await model.current()
    XCTAssertEqual(current, snapshot)
    XCTAssertEqual(server.paths, expectedPaths)
    XCTAssertEqual(server.invalidRequests, [])
    return snapshot
  }

  func testFinding034RefreshUsesRecordedHumanResponses() async throws {
    let snapshot = try await refresh()
    XCTAssertNil(snapshot.refusal)
    XCTAssertEqual(snapshot.serviceVersion, "0.1.0")
    XCTAssertEqual(snapshot.displayName, "Mara Okafor")
    XCTAssertEqual(snapshot.activityCount, 2)
    XCTAssertNil(snapshot.settlement)
    XCTAssertEqual(snapshot.deliveries, [])
  }

  func testFinding034RefreshCountsEveryGroupAndAcceptsEmptyPage() async throws {
    let envelope = try recorded("activity.query")
    var page = try XCTUnwrap(envelope.objectValue?["result"]?.objectValue)
    guard case let .array(groups)? = page["groups"] else { throw XCTUnwrapFailure() }
    var group = try XCTUnwrap(groups.first?.objectValue)
    guard case let .array(entries)? = group["entries"] else { throw XCTUnwrapFailure() }
    XCTAssertEqual(entries.count, 2)
    // Partition the recorded entries into separate groups to exercise summing,
    // preserving each producer entry without fabricating activity or receipts.
    let splitGroups = entries.map { entry -> JSONValue in
      var partition = group
      partition["entries"] = .array([entry])
      return .object(partition)
    }
    group["entries"] = .array([])
    for groups in [splitGroups + [.object(group)], []] {
      page["groups"] = .array(groups)
      let snapshot = try await refresh(activity: replacingResult(envelope, with: .object(page)))
      XCTAssertNil(snapshot.refusal)
      XCTAssertEqual(snapshot.serviceVersion, "0.1.0")
      XCTAssertEqual(snapshot.activityCount, groups.isEmpty ? 0 : 2)
    }
  }

  func testFinding034MalformedShapesAreDecodeRefusals() async throws {
    let version = try recorded("version")
    for result in [
      .object(["version": .string("0.1.0"), "protocol_version": .string("1")]),
      .object(["service": .integer(1)]),
    ] as [JSONValue] {
      let snapshot = try await refresh(
        version: replacingResult(version, with: result),
        expectedPaths: ["/broker", "/v1/version"])
      XCTAssertEqual(snapshot.refusal, MobileErrorCode.decodeFailure.rawValue)
      XCTAssertEqual(snapshot.serviceVersion, "")
    }
    let activity = try recorded("activity.query")
    for result in [
      .object(["entries": .array([])]),
      .object(["groups": .null]),
      .object(["groups": .array([.object([:])])]),
      .object(["groups": .array([.object(["entries": .string("invalid")])])]),
      .object(["groups": .array([.object(["entries": .array([.integer(1)])])])]),
    ] as [JSONValue] {
      let snapshot = try await refresh(activity: replacingResult(activity, with: result))
      XCTAssertEqual(snapshot.refusal, MobileErrorCode.decodeFailure.rawValue)
      XCTAssertEqual(snapshot.serviceVersion, "")
      XCTAssertEqual(snapshot.displayName, "")
      XCTAssertEqual(snapshot.activityCount, 0)
    }
  }

  private struct XCTUnwrapFailure: Error {}
}

private final class RefreshTranscriptServer: @unchecked Sendable {
  let baseURL: URL
  private let listener: Int32
  private let lock = NSLock()
  private let finished = DispatchSemaphore(value: 0)
  private var recordedPaths: [String] = []
  private var rejectedRequests: [String] = []
  private var stopped = false

  var paths: [String] {
    lock.lock()
    defer { lock.unlock() }
    return recordedPaths
  }

  var invalidRequests: [String] {
    lock.lock()
    defer { lock.unlock() }
    return rejectedRequests
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
    DispatchQueue(label: "refresh-2233623-transcript").async { [self] in
      defer { finished.signal() }
      serve(routes)
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

  private func serve(_ routes: [String: JSONValue]) {
    while true {
      let connection = accept(listener, nil, nil)
      if connection < 0 { return }
      defer { close(connection) }
      var timeout = timeval(tv_sec: 5, tv_usec: 0)
      _ = setsockopt(
        connection, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))
      var request = Data()
      var buffer = [UInt8](repeating: 0, count: 4096)
      let separator = Data("\r\n\r\n".utf8)
      while request.range(of: separator) == nil && request.count < 65_536 {
        let count = recv(connection, &buffer, buffer.count, 0)
        if count <= 0 { break }
        request.append(contentsOf: buffer.prefix(count))
      }
      guard let header = request.range(of: separator) else { continue }
      let lines = String(decoding: request[..<header.lowerBound], as: UTF8.self).components(
        separatedBy: "\r\n")
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
      guard let length = Int(headers["content-length"] ?? "0"), length >= 0, length <= 65_536 else {
        continue
      }
      while request.count < header.upperBound + length {
        let count = recv(connection, &buffer, buffer.count, 0)
        if count <= 0 { break }
        request.append(contentsOf: buffer.prefix(count))
      }
      guard request.count == header.upperBound + length else { continue }
      let requestBody = try? JSONDecoder().decode(
        JSONValue.self, from: Data(request[header.upperBound...]))
      let body: JSONValue
      let valid: Bool
      if path == "/broker" {
        valid =
          method == "POST" && requestBody == .object(["audience": .string("layerx-human-api")])
        let now = Int64(Date().timeIntervalSince1970 * 1000)
        body = .object([
          "session_token": .string("recorded-refresh-session"),
          "issued_at_ms": .integer(now), "expires_at_ms": .integer(now + 300_000),
        ])
      } else if let recorded = routes[path] {
        let activity = path == "/v1/activity/query"
        valid =
          headers["authorization"] == "Bearer recorded-refresh-session"
          && method == (activity ? "POST" : "GET")
          && (activity ? requestBody == .object(["page_limit": .integer(25)]) : length == 0)
        body = recorded
      } else {
        valid = false
        body = .object(["unexpected_request": .string(path)])
      }
      lock.lock()
      recordedPaths.append(path)
      if !valid { rejectedRequests.append(method + " " + path) }
      lock.unlock()
      guard let data = try? JSONEncoder().encode(body) else { return }
      let status = valid ? "200 OK" : "400 Bad Request"
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
    }
  }
}
