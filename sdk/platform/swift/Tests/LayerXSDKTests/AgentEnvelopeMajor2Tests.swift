import Foundation
import XCTest

@testable import LayerXSDK

final class AgentEnvelopeMajor2Tests: XCTestCase {
  func testAgentEnvelopeEncodesVersionOne() throws {
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
      XCTAssertEqual(envelope["version"], .integer(1), descriptor.name)
      XCTAssertEqual(envelope["operation"], .string(descriptor.name))
      XCTAssertEqual(envelope["request_id"], .string("42"))
      XCTAssertEqual(envelope["request"], .emptyObject)
      XCTAssertEqual(request.httpMethod, "POST")
      XCTAssertEqual(request.url?.path, AgentEnvelopeTransport.routePath)
    }
  }

  func testAgentEnvelopeDecodesDaemonResponseShapes() throws {
    let verification: JSONValue = .object([
      "state": .string("achieved"), "rung": .string("submitted"),
    ])
    let success: [String: JSONValue] = [
      "request_id": .string("42"), "value": .object(["account": .string("major-two")]),
      "verification_status": verification,
    ]
    let accepted = try AgentEnvelopeTransport.classifyResponse(
      mutating: false, status: 200,
      data: JSONEncoder().encode(JSONValue.object(success)), requestID: "42")
    XCTAssertEqual(accepted.requestID, "42")
    XCTAssertEqual(accepted.value, .object(["account": .string("major-two")]))
    XCTAssertEqual(accepted.verificationStatus, verification)

    let validStatuses: [JSONValue] = [
      .object(["state": .string("achieved"), "rung": .string("receipted")]),
      .object(["state": .string("achieved"), "rung": .string("committed")]),
      .object([
        "state": .string("unverified"), "requested": .string("receipted"),
        "achieved": .string("submitted"), "reason": .string("owner_payload_carries_no_level"),
      ]),
      .object([
        "state": .string("unverified"), "requested": .string("committed"),
        "achieved": .string("submitted"), "reason": .string("owner_payload_carries_no_level"),
      ]),
      .object([
        "state": .string("unverified"), "requested": .string("committed"),
        "achieved": .string("receipted"), "reason": .string("verification.pending"),
      ]),
    ]
    for status in validStatuses {
      var envelope = success
      envelope["verification_status"] = status
      let result = try AgentEnvelopeTransport.classifyResponse(
        mutating: true, status: 200,
        data: JSONEncoder().encode(JSONValue.object(envelope)), requestID: "42")
      XCTAssertEqual(result.verificationStatus, status)
      XCTAssertEqual(result.requestID, "42")
      XCTAssertEqual(result.value, accepted.value)
    }

    var invalidStatuses: [JSONValue] = [
      .object(["state": .string("achieved"), "level": .string("Unverified")]),
      .object(["state": .string("achieved"), "rung": .string("unknown")]),
      .object([
        "state": .string("achieved"), "rung": .string("submitted"), "extra": .null,
      ]),
      .object(["state": .string("achieved")]),
      .object([
        "state": .string("unverified"), "requested": .string("unknown"),
        "achieved": .string("submitted"), "reason": .string("verification.pending"),
      ]),
      .object([
        "state": .string("unverified"), "requested": .string("committed"),
        "achieved": .string("unknown"), "reason": .string("verification.pending"),
      ]),
      .object([
        "state": .string("unverified"), "requested": .string("committed"),
        "achieved": .string("submitted"), "reason": .string("invalid reason"),
      ]),
      .object([
        "state": .string("unverified"), "requested": .string("committed"),
        "achieved": .string("submitted"),
      ]),
      .object([
        "state": .string("unverified"), "requested": .string("committed"),
        "achieved": .string("submitted"), "reason": .string("verification.pending"),
        "extra": .null,
      ]),
    ]
    for (requested, achieved) in [
      ("submitted", "submitted"), ("submitted", "receipted"), ("submitted", "committed"),
      ("receipted", "receipted"), ("receipted", "committed"), ("committed", "committed"),
    ] {
      invalidStatuses.append(
        .object([
          "state": .string("unverified"), "requested": .string(requested),
          "achieved": .string(achieved), "reason": .string("verification.pending"),
        ]))
    }
    var invalidBodies = invalidStatuses.map { status in
      var envelope = success
      envelope["verification_status"] = status
      return (200, envelope)
    }
    var versionedSuccess = success
    versionedSuccess["version"] = .integer(1)
    invalidBodies.append((200, versionedSuccess))
    for field in success.keys {
      var envelope = success
      envelope.removeValue(forKey: field)
      invalidBodies.append((200, envelope))
    }

    let refusal: [String: JSONValue] = [
      "class": .string("InvalidRequest"), "protocol_result_code": .null,
      "retriability": .string("Terminal"), "request_id": .string("42"),
      "reason": .string("invalid_request"),
    ]
    for mutating in [false, true] {
      XCTAssertThrowsError(
        try AgentEnvelopeTransport.classifyResponse(
          mutating: mutating, status: 400,
          data: JSONEncoder().encode(JSONValue.object(refusal)), requestID: "42")
      ) { error in
        XCTAssertEqual(
          error as? PlatformSDKError,
          PlatformSDKError(code: .invalidArgument, retry: .never, requestID: "42"))
      }
    }
    var versionedRefusal = refusal
    versionedRefusal["version"] = .integer(1)
    invalidBodies.append((400, versionedRefusal))
    for field in refusal.keys {
      var envelope = refusal
      envelope.removeValue(forKey: field)
      invalidBodies.append((400, envelope))
    }
    invalidBodies.append((200, refusal))
    invalidBodies.append((201, success))

    for (status, envelope) in invalidBodies {
      let data = try JSONEncoder().encode(JSONValue.object(envelope))
      for mutating in [false, true] {
        XCTAssertThrowsError(
          try AgentEnvelopeTransport.classifyResponse(
            mutating: mutating, status: status, data: data, requestID: "42")
        ) { error in
          guard let error = error as? PlatformSDKError else {
            return XCTFail("Unexpected error: \(error)")
          }
          XCTAssertEqual(error.code, mutating ? .unknownOutcome : .decodeFailure)
          XCTAssertEqual(error.retry, mutating ? .unknownOutcome : .never)
        }
      }
    }
  }
}
