import Foundation

#if canImport(Security)
  import Security
#endif
#if canImport(FoundationNetworking)
  import FoundationNetworking
#endif

private let maximumHTTPResponseBytes = 8 * 1024 * 1024

public final class AccessToken: @unchecked Sendable, CustomStringConvertible {
  private let secret: SecretBytes

  public init(_ bytes: Data) throws {
    guard String(data: bytes, encoding: .utf8) != nil else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    secret = try SecretBytes(bytes)
  }

  fileprivate func authorize(_ request: inout URLRequest) throws {
    try secret.withBytes { bytes in
      guard let value = String(data: bytes, encoding: .utf8), !value.contains("\r"),
        !value.contains("\n")
      else {
        throw PlatformSDKError(code: .invalidArgument, retry: .never)
      }
      request.setValue("Bearer \(value)", forHTTPHeaderField: "Authorization")
    }
  }

  public func destroy() { secret.destroy() }
  public var description: String { "[REDACTED]" }
}

public final class LayerXKeyCredential: @unchecked Sendable, CustomStringConvertible {
  private let keyID: String
  private let secret: SecretBytes

  public init(keyID: String, secret: Data) throws {
    let validID =
      !keyID.isEmpty && keyID.utf8.count <= 64
      && keyID.utf8.allSatisfy {
        ($0 >= 48 && $0 <= 57) || ($0 >= 65 && $0 <= 90) || ($0 >= 97 && $0 <= 122) || $0 == 45
          || $0 == 95
      }
    guard validID else { throw PlatformSDKError(code: .invalidArgument, retry: .never) }
    self.keyID = keyID
    self.secret = try SecretBytes(secret)
  }

  fileprivate func authorize(_ request: inout URLRequest) throws {
    try secret.withBytes { bytes in
      guard let value = String(data: bytes, encoding: .ascii), value.hasPrefix("lxp_live_"),
        value.utf8.count == 73,
        value.dropFirst(9).utf8.allSatisfy({ ($0 >= 48 && $0 <= 57) || ($0 >= 97 && $0 <= 102) })
      else {
        throw PlatformSDKError(code: .invalidArgument, retry: .never)
      }
      request.setValue("LayerX-Key \(keyID):\(value)", forHTTPHeaderField: "Authorization")
    }
  }

  public func destroy() { secret.destroy() }
  public var description: String { "[REDACTED]" }
}

public final class AgentHTTPTransport: PlatformTransport, @unchecked Sendable {
  public init(
    baseURL: URL, session: URLSession = .shared, credential: LayerXKeyCredential? = nil,
    accessToken: AccessToken? = nil
  ) throws {
    guard credential == nil || accessToken == nil else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    guard baseURL.user == nil, baseURL.password == nil, baseURL.host != nil,
      baseURL.query == nil, baseURL.fragment == nil,
      baseURL.scheme == "https" || (baseURL.scheme == "http" && Self.isLoopback(baseURL.host))
    else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
  }

  public func send(_ call: TransportCall) async throws -> JSONValue {
    throw PlatformSDKError(code: .unavailableCapability, retry: .never)
  }

  static func validVerification(_ operation: String, value: JSONValue, status: JSONValue?) -> Bool {
    guard let object = status?.objectValue else { return false }
    if operation == "program.discover" || operation == "program.interface" {
      if operation == "program.discover", let discovery = value.objectValue,
        canonicalHex(discovery["discovery_public_key"], bytes: 32, empty: false),
        canonicalHex(discovery["discovery_signature"], bytes: 64, empty: false),
        canonicalHex(discovery["receipt_digest"], bytes: 32, empty: false),
        exact(object, ["state", "level"]), object["state"]?.stringValue == "Achieved",
        object["level"]?.stringValue == "SequencerSigned"
      {
        return true
      }
      return exact(object, ["state", "requested", "achieved", "reason"])
        && object["state"]?.stringValue == "Unverified"
        && object["requested"]?.stringValue == "SequencerSigned"
        && object["achieved"]?.stringValue == "Unverified"
        && object["reason"]?.stringValue == "server_side_receipt_verification_only"
    }
    let pending =
      ["program.call", "program.receipt", "program.activity"].contains(operation)
      && ["unknown", "pending"].contains(value.objectValue?["state"]?.stringValue ?? "")
    if pending {
      return exact(object, ["state", "requested", "achieved", "reason"])
        && object["state"]?.stringValue == "Unverified"
        && object["requested"]?.stringValue == "SequencerSigned"
        && object["achieved"]?.stringValue == "Unverified"
        && object["reason"]?.stringValue == "receipt_pending"
    }
    return ["program.simulate", "program.call", "program.receipt", "program.activity"].contains(
      operation)
      && exact(object, ["state", "level"])
      && object["state"]?.stringValue == "Achieved"
      && object["level"]?.stringValue == "SequencerSigned"
  }

  private static func exact(_ object: [String: JSONValue], _ fields: Set<String>) -> Bool {
    object.count == fields.count && Set(object.keys) == fields
  }

  private static func canonicalHex(_ value: JSONValue?, bytes: Int, empty: Bool) -> Bool {
    guard let text = value?.stringValue else { return false }
    return empty && text.isEmpty || text.utf8.count == bytes * 2 && canonicalHexText(text)
  }

  private static func canonicalHexText(_ value: String) -> Bool {
    value.utf8.allSatisfy { ($0 >= 48 && $0 <= 57) || ($0 >= 97 && $0 <= 102) }
  }

  private static func isLoopback(_ host: String?) -> Bool {
    guard let host = host?.lowercased() else { return false }
    if host == "localhost" || host == "::1" || host == "[::1]" { return true }
    let octets = host.split(separator: ".")
    return octets.count == 4 && octets.first == "127" && octets.allSatisfy { UInt8($0) != nil }
  }
}

public final class HumanHTTPTransport: PlatformTransport, @unchecked Sendable {
  private let baseURL: URL
  private let session: URLSession
  private let accessToken: AccessToken?

  public init(baseURL: URL, session: URLSession = .shared, accessToken: AccessToken? = nil) throws {
    guard baseURL.user == nil, baseURL.password == nil, baseURL.host != nil,
      baseURL.scheme == "https" || (baseURL.scheme == "http" && Self.isLoopback(baseURL.host))
    else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    self.baseURL = baseURL
    self.session = session
    self.accessToken = accessToken
  }

  public func send(_ call: TransportCall) async throws -> JSONValue {
    let descriptor = call.operation.descriptor
    guard descriptor.plane == .human else {
      throw PlatformSDKError(code: .unavailableCapability, retry: .never)
    }
    let path = try Self.resolvePath(descriptor.path, parameters: call.pathParameters)
    guard let target = URL(string: path, relativeTo: baseURL)?.absoluteURL,
      target.scheme == baseURL.scheme, target.host == baseURL.host, target.port == baseURL.port
    else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    var request = URLRequest(url: target)
    request.httpMethod = descriptor.method.rawValue
    request.setValue("application/json", forHTTPHeaderField: "Accept")
    request.setValue("layerx-swift/0.1.0", forHTTPHeaderField: "User-Agent")
    if !descriptor.bodyless {
      request.httpBody = try JSONEncoder().encode(call.request)
      request.setValue("application/json", forHTTPHeaderField: "Content-Type")
    }
    if let key = call.idempotencyKey {
      request.setValue(key.rawValue, forHTTPHeaderField: "Idempotency-Key")
    }
    try accessToken?.authorize(&request)

    let (data, response) = try await perform(request)
    guard data.count <= maximumHTTPResponseBytes, let http = response as? HTTPURLResponse else {
      throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }
    let envelope: HumanEnvelope
    do {
      envelope = try JSONDecoder().decode(HumanEnvelope.self, from: data)
    } catch {
      throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }
    guard !envelope.trace.isEmpty, envelope.trace.utf8.count <= 512,
      !envelope.trace.contains("\0"), !envelope.trace.contains("\r"), !envelope.trace.contains("\n")
    else {
      throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }
    if envelope.ok {
      guard (200..<300).contains(http.statusCode), envelope.error == nil,
        let result = envelope.result
      else {
        throw PlatformSDKError(code: .decodeFailure, retry: .never)
      }
      return result
    }
    guard !(200..<300).contains(http.statusCode), envelope.result == nil, let error = envelope.error
    else {
      throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }
    throw error.sdkError(trace: envelope.trace)
  }

  private func perform(_ request: URLRequest) async throws -> (Data, URLResponse) {
    let box = URLSessionTaskBox()
    return try await withTaskCancellationHandler(
      operation: {
        try await withCheckedThrowingContinuation {
          (continuation: CheckedContinuation<(Data, URLResponse), Error>) in
          let task = session.dataTask(with: request) { data, response, error in
            if let error {
              continuation.resume(throwing: error)
              return
            }
            guard let data, let response else {
              continuation.resume(throwing: PlatformSDKError(code: .transportFailure, retry: .safe))
              return
            }
            continuation.resume(returning: (data, response))
          }
          box.install(task)
          task.resume()
        }
      },
      onCancel: {
        box.cancel()
      })
  }

  private static func resolvePath(_ template: String, parameters: [String: String]) throws -> String
  {
    var path = template
    for (name, value) in parameters {
      guard !name.isEmpty, !value.isEmpty, !name.contains("{") && !name.contains("}") else {
        throw PlatformSDKError(code: .invalidArgument, retry: .never)
      }
      path = path.replacingOccurrences(of: "{\(name)}", with: percentEncodePathSegment(value))
    }
    guard !path.contains("{") && !path.contains("}") && path.first == "/" else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    return path
  }

  private static func percentEncodePathSegment(_ value: String) -> String {
    let allowed = CharacterSet(
      charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
    return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? ""
  }

  private static func isLoopback(_ host: String?) -> Bool {
    guard let host = host?.lowercased() else { return false }
    return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]"
  }
}

private final class URLSessionTaskBox: @unchecked Sendable {
  private let lock = NSLock()
  private var task: URLSessionTask?
  private var cancelled = false

  func install(_ task: URLSessionTask) {
    lock.lock()
    self.task = task
    let shouldCancel = cancelled
    lock.unlock()
    if shouldCancel { task.cancel() }
  }

  func cancel() {
    lock.lock()
    cancelled = true
    let task = task
    lock.unlock()
    task?.cancel()
  }
}

private struct HumanEnvelope: Decodable {
  let ok: Bool
  let result: JSONValue?
  let error: HumanAPIError?
  let trace: String
}

private struct AgentEnvelope: Decodable {
  let requestID: String
  let value: JSONValue?
  let verificationStatus: JSONValue?
  let errorClass: String?
  let protocolResultCode: Int32?
  let retriability: String?
  let reason: String?

  enum CodingKeys: String, CodingKey {
    case value, retriability, reason
    case requestID = "request_id"
    case verificationStatus = "verification_status"
    case errorClass = "class"
    case protocolResultCode = "protocol_result_code"
  }
}

extension String {
  fileprivate func sdkError(_ envelope: AgentEnvelope) throws -> PlatformSDKError {
    guard !envelope.requestID.isEmpty, let retriability = envelope.retriability,
      let reason = envelope.reason, !reason.isEmpty,
      reason.utf8.allSatisfy({
        ($0 >= 48 && $0 <= 57) || ($0 >= 97 && $0 <= 122) || $0 == 95 || $0 == 46
      })
    else {
      throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }
    let code: SDKErrorCode
    switch self {
    case "InvalidRequest": code = .invalidArgument
    case "TransportFailure": code = .transportFailure
    case "Deadline": code = .deadline
    case "ProtocolIncompatibility": code = .protocolIncompatibility
    case "UnavailableCapability": code = .unavailableCapability
    case "CoreRejection": code = .coreRejection
    case "VerificationFailure": code = .verificationFailure
    case "PolicyRefusal": code = .policyRefusal
    case "CapabilityRefusal": code = .capabilityRefusal
    case "BudgetRefusal": code = .budgetRefusal
    case "RateLimit": code = .rateLimit
    case "IdempotencyConflict": code = .idempotencyConflict
    case "InternalFault": code = .internalFault
    default:
      throw PlatformSDKError(code: .decodeFailure, retry: .never, requestID: envelope.requestID)
    }
    let retry: RetryClass
    switch retriability {
    case "Terminal": retry = .never
    case "Retriable": retry = .safe
    default:
      throw PlatformSDKError(code: .decodeFailure, retry: .never, requestID: envelope.requestID)
    }
    return PlatformSDKError(
      code: code, retry: retry, requestID: envelope.requestID,
      protocolResultCode: envelope.protocolResultCode)
  }
}

private struct HumanAPIError: Decodable {
  let code: String
  let retry: String
  let retryAfterMilliseconds: UInt64?

  enum CodingKeys: String, CodingKey {
    case code, retry
    case retryAfterMilliseconds = "retry_after_ms"
  }

  func sdkError(trace: String) -> PlatformSDKError {
    let mapped: SDKErrorCode
    switch code {
    case "rate-limited": mapped = .rateLimit
    case "unavailable", "upstream-degraded": mapped = .transportFailure
    case "refused-by-policy": mapped = .policyRefusal
    case "refused-by-budget", "refused-by-limit": mapped = .budgetRefusal
    case "refused-by-capability", "forbidden", "unauthenticated", "session-expired",
      "step-up-required":
      mapped = .capabilityRefusal
    case "conflict": mapped = .idempotencyConflict
    case "refused-by-protocol": mapped = .coreRejection
    default: mapped = .coreRejection
    }
    let retryClass: RetryClass
    switch retry {
    case "retriable": retryClass = .safe
    case "retriable-after":
      guard retryAfterMilliseconds != nil else {
        return PlatformSDKError(code: .decodeFailure, retry: .never)
      }
      retryClass = .after
    case "structural", "final": retryClass = .never
    default: return PlatformSDKError(code: .decodeFailure, retry: .never)
    }
    return PlatformSDKError(
      code: mapped, retry: retryClass, requestID: trace,
      retryAfterMilliseconds: retryAfterMilliseconds)
  }
}

private let maximumAgentEnvelopeBytes = 1_048_576

public final class AgentSessionCredential: @unchecked Sendable, CustomStringConvertible {
  public let tenant: String
  public let sessionID: String
  public let generation: UInt64
  private let tokenID: SecretBytes

  public init(tenant: String, sessionID: String, tokenID: Data, generation: UInt64) throws {
    guard !tenant.isEmpty, tenant.utf8.count <= 255, !tenant.utf8.contains(0),
      AgentEnvelopeTransport.lowerHex32(sessionID), tokenID.count == 32
    else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    self.tenant = tenant
    self.sessionID = sessionID
    self.generation = generation
    self.tokenID = try SecretBytes(tokenID)
  }

  fileprivate func encoded() throws -> JSONValue {
    let token = try tokenID.withBytes { bytes in bytes.map { String(format: "%02x", $0) }.joined() }
    return .object([
      "tenant": .string(tenant), "session_id": .string(sessionID),
      "token_id": .string(token), "generation": .string(String(generation)),
    ])
  }

  public func destroy() { tokenID.destroy() }
  public var description: String { "[REDACTED]" }
}

public struct AgentEnvelopeResult: Sendable, Equatable {
  public let requestID: String
  public let value: JSONValue
  public let verificationStatus: JSONValue
}

public final class AgentEnvelopeTransport: PlatformTransport, @unchecked Sendable {
  private static let version: Int64 = 1
  public static let routePath = "/v1/agent/rpc"
  public static let daemonRoutePath = "/rpc"
  static let bootstrapOperations: Set<String> = ["agent.register", "session.open"]
  private static let verificationRungs = ["submitted", "receipted", "committed"]
  private let endpoint: URL
  private let session: URLSession
  private let gatewayKey: LayerXKeyCredential?
  private let credential: AgentSessionCredential?
  private let delegate: AgentEnvelopeTaskDelegate

  public convenience init(
    baseURL: URL, gatewayKey: LayerXKeyCredential, credential: AgentSessionCredential?,
    certificateAuthority: AgentCertificateAuthority? = nil, session: URLSession = .shared
  ) throws {
    try self.init(
      baseURL: baseURL, path: Self.routePath, gatewayKey: gatewayKey, credential: credential,
      certificateAuthority: certificateAuthority, clientIdentity: nil, session: session)
  }

  public static func daemon(
    baseURL: URL, credential: AgentSessionCredential?,
    certificateAuthority: AgentCertificateAuthority, clientIdentity: URLCredential,
    session: URLSession = .shared
  ) throws -> AgentEnvelopeTransport {
    #if canImport(Security)
      guard clientIdentity.identity != nil else {
        throw PlatformSDKError(code: .invalidArgument, retry: .never)
      }
      return try AgentEnvelopeTransport(
        baseURL: baseURL, path: daemonRoutePath, gatewayKey: nil,
        credential: credential, certificateAuthority: certificateAuthority,
        clientIdentity: clientIdentity,
        session: session)
    #else
      throw PlatformSDKError(code: .unavailableCapability, retry: .never)
    #endif
  }

  private init(
    baseURL: URL, path: String, gatewayKey: LayerXKeyCredential?,
    credential: AgentSessionCredential?,
    certificateAuthority: AgentCertificateAuthority?, clientIdentity: URLCredential?,
    session: URLSession
  ) throws {
    guard baseURL.user == nil, baseURL.password == nil, baseURL.host != nil,
      baseURL.scheme == "https",
      baseURL.query == nil, baseURL.fragment == nil,
      var components = URLComponents(url: baseURL, resolvingAgainstBaseURL: false)
    else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    components.path = path
    guard let endpoint = components.url else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    #if canImport(FoundationNetworking)
      if let certificateAuthority {
        let name = "URLSessionCertificateAuthorityInfoFile"
        let path = certificateAuthority.pemFile.path
        if let configured = ProcessInfo.processInfo.environment[name] {
          guard configured == path else {
            throw PlatformSDKError(code: .invalidArgument, retry: .never)
          }
        } else {
          guard setenv(name, path, 0) == 0 else {
            throw PlatformSDKError(code: .invalidArgument, retry: .never)
          }
        }
      }
    #endif
    self.endpoint = endpoint
    self.session = session
    self.gatewayKey = gatewayKey
    self.credential = credential
    self.delegate = AgentEnvelopeTaskDelegate(certificateAuthority, clientIdentity: clientIdentity)
  }

  public func send(_ call: TransportCall) async throws -> JSONValue {
    try await sendEnvelope(call).value
  }

  public func sendEnvelope(_ call: TransportCall) async throws -> AgentEnvelopeResult {
    let mutating = call.operation.descriptor.requiresIdempotency
    let exchanged = try await exchange(call)
    return try Self.classifyResponse(
      mutating: mutating, status: exchanged.status, data: exchanged.body,
      requestID: exchanged.requestID)
  }

  static func classifyResponse(mutating: Bool, status: Int, data: Data, requestID: String) throws
    -> AgentEnvelopeResult
  {
    guard let document = try? JSONDecoder().decode(JSONValue.self, from: data),
      !edgeAmbiguity(status, document)
    else {
      throw mutating
        ? Self.unknownOutcome() : PlatformSDKError(code: .transportFailure, retry: .safe)
    }
    do {
      return try Self.decodeResponse(status: status, data: data, requestID: requestID)
    } catch let error as PlatformSDKError {
      if mutating, error.code == .decodeFailure || error.code == .verificationFailure {
        throw Self.unknownOutcome()
      }
      throw error
    }
  }

  private static func edgeAmbiguity(_ status: Int, _ document: JSONValue) -> Bool {
    guard status == 502 || status == 503, let object = document.objectValue,
      exact(object, ["ok", "error"]),
      object["ok"] == .boolean(false), let error = object["error"]?.objectValue,
      exact(error, ["code"]),
      error["code"]?.stringValue != nil
    else { return false }
    return true
  }

  func exchange(_ call: TransportCall) async throws -> (requestID: String, status: Int, body: Data)
  {
    let descriptor = call.operation.descriptor
    guard descriptor.plane == .agent else {
      throw PlatformSDKError(code: .unavailableCapability, retry: .never)
    }
    let mutating = descriptor.requiresIdempotency
    let requestID = String(UInt64.random(in: 1...UInt64.max))
    let request = try envelopeRequest(descriptor, call: call, requestID: requestID)
    let data: Data
    let response: URLResponse
    do {
      (data, response) = try await session.data(for: request, delegate: delegate)
    } catch {
      throw mutating
        ? Self.unknownOutcome() : PlatformSDKError(code: .transportFailure, retry: .safe)
    }
    guard data.count <= maximumHTTPResponseBytes, let http = response as? HTTPURLResponse,
      http.value(forHTTPHeaderField: "Content-Type")?.split(separator: ";", maxSplits: 1).first?
        .trimmingCharacters(in: .whitespaces).lowercased() == "application/json"
    else {
      throw mutating
        ? Self.unknownOutcome() : PlatformSDKError(code: .transportFailure, retry: .safe)
    }
    return (requestID, http.statusCode, data)
  }

  func envelopeRequest(_ descriptor: OperationDescriptor, call: TransportCall, requestID: String)
    throws -> URLRequest
  {
    guard call.pathParameters.isEmpty, call.request.objectValue != nil else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    var envelope: [String: JSONValue] = [
      "version": .integer(Self.version), "request_id": .string(requestID),
      "operation": .string(descriptor.name), "request": call.request,
    ]
    if Self.bootstrapOperations.contains(descriptor.name) {
      envelope["credential"] = .null
    } else {
      guard let credential else { throw PlatformSDKError(code: .capabilityRefusal, retry: .never) }
      envelope["credential"] = try credential.encoded()
    }
    if descriptor.requiresIdempotency {
      guard let key = call.idempotencyKey, Self.lowerHex32(key.rawValue) else {
        throw PlatformSDKError(code: .idempotencyRequired, retry: .never)
      }
      envelope["idempotency_key"] = .string(key.rawValue)
    } else {
      guard call.idempotencyKey == nil else {
        throw PlatformSDKError(code: .invalidArgument, retry: .never)
      }
      envelope["idempotency_key"] = .null
    }
    let body = try JSONEncoder().encode(JSONValue.object(envelope))
    guard body.count <= maximumAgentEnvelopeBytes else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    var request = URLRequest(url: endpoint)
    request.httpMethod = "POST"
    request.httpBody = body
    request.setValue("application/json", forHTTPHeaderField: "Content-Type")
    request.setValue("application/json", forHTTPHeaderField: "Accept")
    request.setValue("layerx-swift/0.1.0", forHTTPHeaderField: "User-Agent")
    if let gatewayKey { try gatewayKey.authorize(&request) }
    return request
  }

  static func decodeResponse(status: Int, data: Data, requestID: String) throws
    -> AgentEnvelopeResult
  {
    let document: JSONValue
    do { document = try JSONDecoder().decode(JSONValue.self, from: data) } catch { throw decode() }
    guard let object = document.objectValue else { throw decode() }
    if object["class"] != nil {
      guard !(200..<300).contains(status),
        exact(
          object,
          ["class", "protocol_result_code", "retriability", "request_id", "reason"]),
        let responseID = object["request_id"]?.stringValue,
        responseID == requestID || responseID == "0",
        let errorClass = object["class"]?.stringValue,
        let reason = object["reason"]?.stringValue, reasonCode(reason)
      else {
        throw decode()
      }
      let resultCode: Int32?
      switch object["protocol_result_code"] {
      case .null?: resultCode = nil
      case let .integer(value)?:
        guard let exact = Int32(exactly: value) else { throw decode(responseID) }
        resultCode = exact
      default: throw decode(responseID)
      }
      throw try errorClass.sdkError(
        .init(
          requestID: responseID, value: nil, verificationStatus: nil,
          errorClass: errorClass, protocolResultCode: resultCode,
          retriability: object["retriability"]?.stringValue, reason: reason))
    }
    guard status == 200, exact(object, ["request_id", "value", "verification_status"]),
      object["request_id"]?.stringValue == requestID, let value = object["value"],
      let verification = object["verification_status"], validVerification(verification)
    else {
      throw decode(object["request_id"]?.stringValue)
    }
    return .init(requestID: requestID, value: value, verificationStatus: verification)
  }

  static func validVerification(_ status: JSONValue) -> Bool {
    guard let object = status.objectValue else { return false }
    if exact(object, ["state", "rung"]), object["state"]?.stringValue == "achieved",
      let rung = object["rung"]?.stringValue
    {
      return verificationRungs.contains(rung)
    }
    guard exact(object, ["state", "requested", "achieved", "reason"]),
      object["state"]?.stringValue == "unverified",
      let requested = object["requested"]?.stringValue.flatMap({
        verificationRungs.firstIndex(of: $0)
      }),
      let achieved = object["achieved"]?.stringValue.flatMap({
        verificationRungs.firstIndex(of: $0)
      }),
      let reason = object["reason"]?.stringValue
    else { return false }
    return achieved < requested && reasonCode(reason)
  }

  static func lowerHex32(_ value: String) -> Bool {
    value.utf8.count == 64
      && value.utf8.allSatisfy { ($0 >= 48 && $0 <= 57) || ($0 >= 97 && $0 <= 102) }
  }

  private static func reasonCode(_ value: String) -> Bool {
    !value.isEmpty && value.utf8.count <= 128
      && value.utf8.allSatisfy {
        ($0 >= 48 && $0 <= 57) || ($0 >= 97 && $0 <= 122) || $0 == 95 || $0 == 46
      }
  }

  private static func exact(_ object: [String: JSONValue], _ fields: Set<String>) -> Bool {
    object.count == fields.count && Set(object.keys) == fields
  }

  private static func decode(_ requestID: String? = nil) -> PlatformSDKError {
    .init(code: .decodeFailure, retry: .never, requestID: requestID)
  }

  private static func unknownOutcome() -> PlatformSDKError {
    .init(code: .unknownOutcome, retry: .unknownOutcome)
  }
}

public struct AgentCertificateAuthority: Sendable {
  public let pemFile: URL
  public let der: Data

  public init(pemFile: URL) throws {
    guard pemFile.isFileURL, let text = try? String(contentsOf: pemFile, encoding: .utf8) else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    let lines = text.split(whereSeparator: { $0 == "\n" || $0 == "\r" }).map(String.init)
    guard lines.first == "-----BEGIN CERTIFICATE-----", lines.last == "-----END CERTIFICATE-----",
      lines.filter({ $0.hasPrefix("-----") }).count == 2,
      let der = Data(base64Encoded: lines.dropFirst().dropLast().joined()), !der.isEmpty
    else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    self.pemFile = pemFile
    self.der = der
  }
}

private final class AgentEnvelopeTaskDelegate: NSObject, URLSessionTaskDelegate, @unchecked Sendable
{
  private let certificateAuthority: AgentCertificateAuthority?
  private let clientIdentity: URLCredential?

  init(_ certificateAuthority: AgentCertificateAuthority?, clientIdentity: URLCredential? = nil) {
    self.certificateAuthority = certificateAuthority
    self.clientIdentity = clientIdentity
  }

  func urlSession(
    _ session: URLSession, task: URLSessionTask,
    willPerformHTTPRedirection response: HTTPURLResponse,
    newRequest request: URLRequest,
    completionHandler: @escaping (URLRequest?) -> Void
  ) {
    completionHandler(nil)
  }

  #if canImport(Security)
    func urlSession(
      _ session: URLSession, task: URLSessionTask, didReceive challenge: URLAuthenticationChallenge,
      completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void
    ) {
      if let clientIdentity,
        challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodClientCertificate
      {
        completionHandler(.useCredential, clientIdentity)
        return
      }
      guard let certificateAuthority,
        challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodServerTrust
      else {
        completionHandler(.performDefaultHandling, nil)
        return
      }
      guard let trust = challenge.protectionSpace.serverTrust,
        let anchor = SecCertificateCreateWithData(nil, certificateAuthority.der as CFData),
        SecTrustSetPolicies(
          trust, SecPolicyCreateSSL(true, challenge.protectionSpace.host as CFString))
          == errSecSuccess,
        SecTrustSetAnchorCertificates(trust, [anchor] as CFArray) == errSecSuccess,
        SecTrustSetAnchorCertificatesOnly(trust, true) == errSecSuccess,
        SecTrustEvaluateWithError(trust, nil)
      else {
        completionHandler(.cancelAuthenticationChallenge, nil)
        return
      }
      completionHandler(.useCredential, URLCredential(trust: trust))
    }
  #endif
}
