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

  func testAgentEnvelopeRefusesResponseWithoutSchemaFields() throws {
    let success: [String: JSONValue] = [
      "request_id": .string("42"), "value": .object(["account": .string("major-two")]),
      "verification_status": .object([
        "state": .string("achieved"), "rung": .string("submitted"),
      ]),
    ]
    for field in ["verification_status", "request_id"] {
      var envelope = success
      envelope.removeValue(forKey: field)
      let response = try JSONEncoder().encode(JSONValue.object(envelope))
      var result: AgentEnvelopeResult?
      XCTAssertThrowsError(
        result = try AgentEnvelopeTransport.decodeResponse(
          status: 200, data: response, requestID: "42"),
        "Accepted a response without \(field)"
      ) { error in
        guard let error = error as? PlatformSDKError else {
          return XCTFail("Unexpected error: \(error)")
        }
        XCTAssertEqual(error.code, .decodeFailure, field)
        XCTAssertEqual(error.retry, .never, field)
      }
      XCTAssertNil(result, field)
    }
  }

  func testAgentEnvelopeAcceptsNotReadyVerificationStatus() throws {
    var root = URL(fileURLWithPath: #filePath)
    for _ in 0..<6 { root.deleteLastPathComponent() }
    for state in ["pending", "absent"] {
      let data = try Data(
        contentsOf: root.appendingPathComponent(
          "services/agent/schema/agent-api/golden/agent-http-success-\(state).json"))
      let document = try JSONDecoder().decode(JSONValue.self, from: data)
      let envelope = try XCTUnwrap(document.objectValue)
      let verification = try XCTUnwrap(envelope["verification_status"]?.objectValue)
      let requestID = try XCTUnwrap(envelope["request_id"]?.stringValue)
      var expectedVerification: [String: JSONValue] = [
        "state": .string(state), "requested": .string("receipted"),
      ]
      if state == "pending" { expectedVerification["achieved"] = .string("submitted") }
      XCTAssertEqual(verification, expectedVerification)
      for mutating in [false, true] {
        let result = try AgentEnvelopeTransport.classifyResponse(
          mutating: mutating, status: 200, data: data, requestID: requestID)
        XCTAssertEqual(result.requestID, requestID)
        XCTAssertEqual(result.value, envelope["value"])
        XCTAssertEqual(result.value.objectValue?["found"], .boolean(false))
        XCTAssertEqual(result.verificationStatus, .object(expectedVerification))
        XCTAssertNotEqual(result.verificationStatus.objectValue?["state"], .string("achieved"))
        XCTAssertNil(result.verificationStatus.objectValue?["rung"])
      }

      var invalidStatuses: [[String: JSONValue]] = []
      var extraField = verification
      extraField["extra"] = .null
      invalidStatuses.append(extraField)
      var withReason = verification
      withReason["reason"] = .string("verification.pending")
      invalidStatuses.append(withReason)
      var missingRequested = verification
      missingRequested.removeValue(forKey: "requested")
      invalidStatuses.append(missingRequested)
      let invalidRungs: [JSONValue] = [.string("unknown"), .null, .integer(1)]
      for invalidRung in invalidRungs {
        var invalidRequested = verification
        invalidRequested["requested"] = invalidRung
        invalidStatuses.append(invalidRequested)
        if state == "pending" {
          var invalidAchieved = verification
          invalidAchieved["achieved"] = invalidRung
          invalidStatuses.append(invalidAchieved)
        }
      }
      if state == "pending" {
        var missingAchieved = verification
        missingAchieved.removeValue(forKey: "achieved")
        invalidStatuses.append(missingAchieved)
        for (requested, achieved) in [
          ("submitted", "submitted"), ("submitted", "receipted"), ("submitted", "committed"),
          ("receipted", "receipted"), ("receipted", "committed"), ("committed", "committed"),
        ] {
          invalidStatuses.append([
            "state": .string(state), "requested": .string(requested),
            "achieved": .string(achieved),
          ])
        }
      } else {
        var withAchieved = verification
        withAchieved["achieved"] = .string("submitted")
        invalidStatuses.append(withAchieved)
      }
      for status in invalidStatuses {
        var invalidEnvelope = envelope
        invalidEnvelope["verification_status"] = .object(status)
        let invalidData = try JSONEncoder().encode(JSONValue.object(invalidEnvelope))
        for mutating in [false, true] {
          XCTAssertThrowsError(
            try AgentEnvelopeTransport.classifyResponse(
              mutating: mutating, status: 200, data: invalidData, requestID: requestID),
            "Accepted invalid verification status: \(status)"
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
