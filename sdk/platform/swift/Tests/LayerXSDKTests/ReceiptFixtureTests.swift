import Crypto
import Foundation
import XCTest

@testable import LayerXSDK

final class ReceiptFixtureTests: XCTestCase {
  func testNativeAccountAuthorizationVectors() throws {
    let raw = try Data(
      contentsOf: fixtureURL(
        "../../../../programs/fixtures/pay5/account-authorization-vectors.json"))
    let vectors = try XCTUnwrap(JSONSerialization.jsonObject(with: raw) as? [[String: Any]])
    for vector in vectors {
      let encoded = try hexField(vector, "encoded")
      let root = try hexField(vector, "root")
      if try XCTUnwrap(vector["accept"] as? Bool) {
        XCTAssertEqual(try ProgramsWireTestSupport.authorizationRoot(encoded), root)
      } else {
        let actual: Data
        do { actual = try ProgramsWireTestSupport.authorizationRoot(encoded) } catch { continue }
        XCTAssertNotEqual(actual, root)
      }
    }
  }

  func testNativeLifecycleCFixtures() throws {
    for name in [
      "deploy", "upgrade", "wind-down-route", "wind-down-deprecate", "wind-down-tombstone",
      "wind-down-exit",
    ] {
      let raw = try Data(contentsOf: fixtureURL("native-program-\(name)-v3.json"))
      let fixture = try XCTUnwrap(JSONSerialization.jsonObject(with: raw) as? [String: Any])
      XCTAssertEqual(fixture["protocol_version"] as? Int, 3)
      XCTAssertEqual(fixture["module"] as? Int, 9)
      let ordinal = UInt16(try XCTUnwrap(fixture["ordinal"] as? Int))
      let payload = try hexField(fixture, "payload_hex")
      let signed = try hexField(fixture, "signed_activity_hex")
      let value = try decodeNativeProgramLifecycle(ordinal, payload)
      XCTAssertEqual(value.encode(), payload)
      let request = try NativeProgramLifecycleRequest(operation: value, signedActivity: signed)
      XCTAssertEqual(request.activityID, try hexField(fixture, "activity_id_hex"))
      XCTAssertEqual(request.idempotencyKey, try hexField(fixture, "idempotency_key_hex"))
      for length in 0..<payload.count {
        XCTAssertThrowsError(try decodeNativeProgramLifecycle(ordinal, payload.prefix(length)))
      }
      var trailing = payload
      trailing.append(0)
      XCTAssertThrowsError(try decodeNativeProgramLifecycle(ordinal, trailing))
      var changedPayload = payload
      changedPayload[0] ^= 1
      XCTAssertThrowsError(
        try NativeProgramLifecycleRequest(
          operation: decodeNativeProgramLifecycle(ordinal, changedPayload), signedActivity: signed))
      for offset in [1, 7, 17] {
        var changed = signed
        changed[offset] ^= 1
        XCTAssertThrowsError(
          try NativeProgramLifecycleRequest(operation: value, signedActivity: changed))
      }
      for length in 0..<signed.count {
        XCTAssertThrowsError(
          try NativeProgramLifecycleRequest(operation: value, signedActivity: signed.prefix(length))
        )
      }
      if ordinal == 1 || ordinal == 2 {
        for offset in [35, 68, payload.count - 1] {
          var changed = payload
          changed[offset] ^= 1
          XCTAssertThrowsError(try decodeNativeProgramLifecycle(ordinal, changed))
        }
      }
    }
  }

  private static let programOutcomeV3 =
    "505247330100000000000100010000000700000001000000000000000b000000000000000c000000000000000d000000000000000e00000001000000000000000f0000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000020000000000000003000000000000000400000000000000050000000000000006000000000000000700000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000201111111111111111111111111111111111111111111111111111111111111111000000202222222222222222222222222222222222222222222222222222222222222222000000200000000000000000000000000000000000000000000000000000000000000000"

  func testNativeSignedBinding() throws {
    let raw = try Data(contentsOf: fixtureURL("native-program-call-v3.json"))
    let fixture = try XCTUnwrap(JSONSerialization.jsonObject(with: raw) as? [String: Any])
    let payload = try hexField(fixture, "payload_hex")
    let native = try NativeProgramCall.decode(payload)
    XCTAssertEqual(try native.encode(), payload)
    let signed = try hexField(fixture, "signed_activity_hex")
    let call = try ProgramCall(
      nativeCall: native, feeLimit: ProtocolAmount("1000"), signedActivity: signed)
    XCTAssertEqual(try decodeSignedCall(call).activityID, try hexField(fixture, "activity_id_hex"))
    let wrongFee = try ProgramCall(
      nativeCall: native, feeLimit: ProtocolAmount("999"), signedActivity: signed)
    XCTAssertThrowsError(try decodeSignedCall(wrongFee))
    let changed = NativeProgramCall(
      programID: native.programID, guestABI: native.guestABI, entrypoint: native.entrypoint,
      calldata: native.calldata, capabilities: native.capabilities,
      accessDeclaration: native.accessDeclaration,
      responseCapacity: (native.responseCapacity + 1) % 1_048_577, resources: native.resources)
    XCTAssertNotEqual(native.responseCapacity, changed.responseCapacity)
    _ = try changed.encode()
    XCTAssertThrowsError(
      try decodeSignedCall(
        ProgramCall(nativeCall: changed, feeLimit: ProtocolAmount("1000"), signedActivity: signed)))
    for length in 0..<payload.count {
      XCTAssertThrowsError(try NativeProgramCall.decode(payload.prefix(length)))
    }
  }

  func testProgramOutcomeV3Vector() throws {
    let hex = Self.programOutcomeV3
    let encoded = Data(
      stride(from: 0, to: hex.count, by: 2).map {
        UInt8(
          hex[
            hex.index(hex.startIndex, offsetBy: $0)..<hex.index(hex.startIndex, offsetBy: $0 + 2)],
          radix: 16)!
      })
    let outcome = try LocalVerifier.decodeProgramReceiptOutcome(encoded, protocolVersion: 1)
    XCTAssertEqual(outcome.encodingVersion, 3)
    XCTAssertEqual(outcome.abiVersion, 1)
    XCTAssertEqual(outcome.feeUnits, UInt128Value(high: 0, low: 16))
    XCTAssertEqual(outcome.callGraphRoot, Data(repeating: 0x11, count: 32))
    XCTAssertEqual(outcome.terminalPayloadRoot, Data(repeating: 0x22, count: 32))
  }
  private func fixtureURL(_ name: String) -> URL {
    var url = URL(fileURLWithPath: #filePath)
    for _ in 0..<6 { url.deleteLastPathComponent() }
    return
      url
      .appendingPathComponent("sdk/platform/conformance/fixtures")
      .appendingPathComponent(name)
  }

  private func hexData(_ value: String) throws -> Data {
    XCTAssertEqual(value.count % 2, 0, "odd hex length")
    var bytes = Data(capacity: value.count / 2)
    var index = value.startIndex
    while index < value.endIndex {
      let next = value.index(index, offsetBy: 2)
      let byte = try XCTUnwrap(UInt8(value[index..<next], radix: 16), "invalid hex byte")
      bytes.append(byte)
      index = next
    }
    return bytes
  }

  private func hexField(_ object: [String: Any], _ key: String) throws -> Data {
    try hexData(try XCTUnwrap(object[key] as? String, "missing \(key)"))
  }

}
