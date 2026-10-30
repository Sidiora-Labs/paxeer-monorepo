import Foundation
import LayerXMobile
import LayerXSDK

#if canImport(FoundationNetworking)
  import FoundationNetworking
#endif

public enum SampleEnvironment {
  public static func configuration(
    from environment: [String: String] = ProcessInfo.processInfo.environment
  ) throws -> PublishableConfiguration {
    try PublishableConfiguration(environment: environment)
  }

  #if canImport(Darwin)
    public static func configuration(bundle: Bundle) throws -> PublishableConfiguration {
      guard let raw = bundle.object(forInfoDictionaryKey: "LayerX") as? [String: Any] else {
        throw MobileIntegrationError(.invalidConfiguration)
      }
      var declared: [String: String] = [:]
      for (name, value) in raw {
        guard let text = value as? String else {
          throw MobileIntegrationError(.invalidConfiguration)
        }
        declared[name] = text
      }
      return try PublishableConfiguration(declaredKeys: declared)
    }
  #endif

  public static func session(timeoutSeconds: Double) -> URLSession {
    let configuration = URLSessionConfiguration.ephemeral
    configuration.timeoutIntervalForRequest = timeoutSeconds
    configuration.httpAdditionalHeaders = nil
    configuration.httpCookieAcceptPolicy = .onlyFromMainDocumentDomain
    return URLSession(configuration: configuration)
  }
}

public struct RelayReceiptResolver: MobileReceiptResolver, @unchecked Sendable {
  private let relayURL: URL
  private let session: URLSession

  public init(relayURL: URL, session: URLSession) throws {
    guard relayURL.user == nil, relayURL.password == nil, relayURL.query == nil,
      relayURL.fragment == nil,
      let host = relayURL.host, !host.isEmpty,
      relayURL.scheme == "https" || (relayURL.scheme == "http" && Self.isLoopback(host))
    else {
      throw MobileIntegrationError(.invalidConfiguration)
    }
    self.relayURL = relayURL
    self.session = session
  }

  public func resolve(receiptReference: String) async throws -> MobileReceiptEvidence {
    var request = URLRequest(url: relayURL.appendingPathComponent(receiptReference))
    request.httpMethod = "GET"
    request.setValue("application/json", forHTTPHeaderField: "Accept")
    request.setValue("layerx-ios/0.1.0", forHTTPHeaderField: "User-Agent")
    let (data, response) = try await perform(request)
    guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode),
      data.count <= 4 * 1024 * 1024,
      let payload = try? JSONDecoder().decode(RelayReceiptPayload.self, from: data)
    else {
      throw MobileIntegrationError(.decodeFailure)
    }
    guard payload.receiptKey.firstHeight <= payload.receiptKey.lastHeight else {
      throw MobileIntegrationError(.decodeFailure)
    }
    return ReceiptGate.Evidence(
      rawTransaction: try Self.hex(payload.rawTransaction),
      signedReceipt: try Self.hex(payload.signedReceipt),
      receiptKey: ReceiptKeys(
        keyId: try Self.hex32(payload.receiptKey.keyID),
        publicKey: try Self.hex32(payload.receiptKey.publicKey),
        firstHeight: payload.receiptKey.firstHeight,
        lastHeight: payload.receiptKey.lastHeight
      ),
      receiptJSON: payload.receiptJSON,
      prevSeqHash: try Self.hex32(payload.prevSeqHash)
    )
  }

  private func perform(_ request: URLRequest) async throws -> (Data, URLResponse) {
    try await withCheckedThrowingContinuation {
      (continuation: CheckedContinuation<(Data, URLResponse), Error>) in
      let task = session.dataTask(with: request) { data, response, error in
        if error != nil {
          continuation.resume(throwing: MobileIntegrationError(.transportFailure))
          return
        }
        guard let data, let response else {
          continuation.resume(throwing: MobileIntegrationError(.transportFailure))
          return
        }
        continuation.resume(returning: (data, response))
      }
      task.resume()
    }
  }

  public static func hex32(_ value: String) throws -> Data {
    let characters = Array(value)
    guard characters.count == 64, characters.allSatisfy({ $0.isASCII && $0.isHexDigit }) else {
      throw MobileIntegrationError(.decodeFailure)
    }
    var bytes = Data()
    bytes.reserveCapacity(32)
    for index in stride(from: 0, to: 64, by: 2) {
      guard let byte = UInt8(String(characters[index...index + 1]), radix: 16) else {
        throw MobileIntegrationError(.decodeFailure)
      }
      bytes.append(byte)
    }
    return bytes
  }

  private static func hex(_ value: String) throws -> Data {
    guard value.hasPrefix("0x") else {
      throw MobileIntegrationError(.decodeFailure)
    }
    let characters = Array(value.dropFirst(2))
    guard !characters.isEmpty, characters.count % 2 == 0,
      characters.allSatisfy({ $0.isASCII && $0.isHexDigit })
    else {
      throw MobileIntegrationError(.decodeFailure)
    }
    var bytes = Data()
    bytes.reserveCapacity(characters.count / 2)
    for index in stride(from: 0, to: characters.count, by: 2) {
      guard let byte = UInt8(String(characters[index...index + 1]), radix: 16) else {
        throw MobileIntegrationError(.decodeFailure)
      }
      bytes.append(byte)
    }
    return bytes
  }

  private static func isLoopback(_ host: String) -> Bool {
    let normalized = host.lowercased()
    return normalized == "localhost" || normalized == "127.0.0.1" || normalized == "::1"
      || normalized == "[::1]"
  }
}

private struct RelayReceiptPayload: Decodable {
  struct Key: Decodable {
    let keyID: String
    let publicKey: String
    let firstHeight: UInt64
    let lastHeight: UInt64

    enum CodingKeys: String, CodingKey {
      case keyID = "key_id"
      case publicKey = "public_key"
      case firstHeight = "first_height"
      case lastHeight = "last_height"
    }
  }

  let rawTransaction: String
  let signedReceipt: String
  let receiptKey: Key
  let receiptJSON: [String: JSONValue]
  let prevSeqHash: String

  enum CodingKeys: String, CodingKey {
    case rawTransaction = "raw_transaction"
    case signedReceipt = "signed_receipt"
    case receiptKey = "receipt_key"
    case receiptJSON = "receipt_json"
    case prevSeqHash = "prev_seq_hash"
  }
}
