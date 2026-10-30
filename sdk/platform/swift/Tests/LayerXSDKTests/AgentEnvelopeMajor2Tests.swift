import Foundation
import XCTest

@testable import LayerXSDK

final class AgentEnvelopeMajor2Tests: XCTestCase {
  func testAgentEnvelopeEncodesVersionTwo() throws {
    let credential = try AgentSessionCredential(
      tenant: "major-two", sessionID: String(repeating: "a", count: 64),
      tokenID: Data(repeating: 0x42, count: 32), generation: 1)
    let key = try LayerXKeyCredential(
      keyID: "major-two", secret: Data(("lxp_live_" + String(repeating: "b", count: 64)).utf8))
    defer {
      credential.destroy()
      key.destroy()
    }
    let transport = try AgentEnvelopeTransport(
      baseURL: XCTUnwrap(URL(string: "https://localhost")), gatewayKey: key, credential: credential)
    let operations = PlatformOperation.allCases.filter { $0.descriptor.plane == .agent }
    XCTAssertFalse(operations.isEmpty)
    for operation in operations {
      let descriptor = operation.descriptor
      let idempotencyKey =
        descriptor.requiresIdempotency
        ? try IdempotencyKey(String(repeating: "c", count: 64)) : nil
      let call = TransportCall(
        operation: operation, request: .emptyObject, pathParameters: [:],
        idempotencyKey: idempotencyKey)
      let request = try transport.envelopeRequest(descriptor, call: call, requestID: "42")
      let document = try JSONDecoder().decode(JSONValue.self, from: XCTUnwrap(request.httpBody))
      let envelope = try XCTUnwrap(document.objectValue)
      XCTAssertEqual(envelope["version"], .integer(2), descriptor.name)
      XCTAssertEqual(envelope["operation"], .string(descriptor.name))
      XCTAssertEqual(envelope["request_id"], .string("42"))
      XCTAssertEqual(envelope["request"], .emptyObject)
      XCTAssertEqual(request.httpMethod, "POST")
      XCTAssertEqual(request.url?.path, AgentEnvelopeTransport.routePath)
    }
  }

  func testAgentEnvelopeRefusesVersionOneResponse() throws {
    let accepted = try AgentEnvelopeTransport.classifyResponse(
      mutating: false, status: 200, data: agentEnvelope(.integer(2)), requestID: "42")
    XCTAssertEqual(accepted.requestID, "42")
    XCTAssertEqual(accepted.value, .object(["account": .string("major-two")]))
    XCTAssertEqual(
      accepted.verificationStatus,
      .object(["state": .string("achieved"), "level": .string("Unverified")]))

    let refusedVersions: [JSONValue?] = [
      .integer(1), .integer(0), .integer(3), .string("2"), .boolean(true), .null, nil,
    ]
    for version in refusedVersions {
      let data = try agentEnvelope(version)
      for mutating in [false, true] {
        XCTAssertThrowsError(
          try AgentEnvelopeTransport.classifyResponse(
            mutating: mutating, status: 200, data: data, requestID: "42")
        ) { error in
          guard let error = error as? PlatformSDKError else {
            return XCTFail("Unexpected error: \(error)")
          }
          XCTAssertEqual(error.code, mutating ? .unknownOutcome : .decodeFailure)
          XCTAssertEqual(error.retry, mutating ? .unknownOutcome : .never)
        }
      }
    }

    let errorEnvelope = try JSONEncoder().encode(
      JSONValue.object([
        "version": .integer(1), "class": .string("InvalidRequest"), "protocol_result_code": .null,
        "retriability": .string("never"), "request_id": .string("42"),
        "reason": .string("invalid_request"),
      ]))
    XCTAssertThrowsError(
      try AgentEnvelopeTransport.decodeResponse(status: 400, data: errorEnvelope, requestID: "42")
    ) { error in
      XCTAssertEqual(
        error as? PlatformSDKError,
        PlatformSDKError(code: .decodeFailure, retry: .never, requestID: "42"))
    }
  }

  private func agentEnvelope(_ version: JSONValue?) throws -> Data {
    var envelope: [String: JSONValue] = [
      "request_id": .string("42"), "value": .object(["account": .string("major-two")]),
      "verification_status": .object(["state": .string("achieved"), "level": .string("Unverified")]
      ),
    ]
    envelope["version"] = version
    return try JSONEncoder().encode(JSONValue.object(envelope))
  }
}
