import Foundation
import XCTest

@testable import LayerXSDK

final class SequencerReceiptTests: XCTestCase {
  private struct Fixture: Decodable {
    let cases: [Vector]
  }
  private struct Vector: Decodable {
    let id: String
    let vector: String
    let rawTransaction: String
    let signedReceipt: String
    let receiptDigest: String
    let receiptJson: [String: JSONValue]
    let prevSeqHash: String
    let pinnedKey: Key
    let expected: Expected
  }
  private struct Key: Decodable {
    let keyId: String
    let publicKey: String
    let firstHeight: UInt64
    let lastHeight: UInt64
  }
  private struct Expected: Decodable {
    let rung: String?
    let refusedAt: String?
    let check: String?
  }
  private func vectors(_ kind: String) throws -> [Vector] {
    let files = [
      "V1": "vector-v1-receipted-positive.json",
      "V2": "vector-v2-receipted-revert.json",
      "V3": "vector-v3-receipt-refusals.json",
    ]
    let file = try XCTUnwrap(files[kind])
    let path = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("conformance/fixtures/\(file)")
    let cases = try JSONDecoder().decode(Fixture.self, from: Data(contentsOf: path)).cases
    for vector in cases {
      XCTAssertEqual(vector.vector, kind, vector.id)
    }
    return cases
  }
  private func hex(_ s: String) throws -> Data {
    XCTAssertTrue(s.hasPrefix("0x"))
    let chars = Array(s.dropFirst(2))
    XCTAssertEqual(chars.count % 2, 0)
    return try Data(
      stride(from: 0, to: chars.count, by: 2).map {
        try XCTUnwrap(UInt8(String(chars[$0...($0 + 1)]), radix: 16))
      })
  }
  private func verifySequencerReceipt(_ v: Vector, standby: Bool = false) throws
    -> SequencerReceiptVerification
  {
    let key = ReceiptKeys(
      keyId: try hex(v.pinnedKey.keyId), publicKey: try hex(v.pinnedKey.publicKey),
      firstHeight: v.pinnedKey.firstHeight, lastHeight: v.pinnedKey.lastHeight)
    var keys = [key]
    if standby {
      keys.insert(
        ReceiptKeys(
          keyId: Data(repeating: 7, count: 32),
          publicKey: key.publicKey, firstHeight: 1, lastHeight: UInt64.max), at: 0)
    }
    return try LocalVerifier.verifySequencerReceipt(
      hex(v.signedReceipt),
      rawTransaction: hex(v.rawTransaction), keys: keys,
      receiptJSON: v.receiptJson, prevSeqHash: hex(v.prevSeqHash))
  }
  func testSequencerReceiptV1CasesVerifyReceipted() throws {
    let cases = try vectors("V1")
    XCTAssertEqual(cases.count, 4)
    for v in cases {
      let result = try verifySequencerReceipt(v, standby: true)
      XCTAssertEqual(result.level, v.expected.rung, v.id)
      XCTAssertEqual(result.status, 1, v.id)
      XCTAssertEqual(
        v.receiptJson["status"], .string("0x" + String(result.status, radix: 16)), v.id)
      XCTAssertNil(result.revert, v.id)
      XCTAssertEqual(result.receiptDigest, try hex(v.receiptDigest), v.id)
      XCTAssertEqual(result.canonicalBytes, try hex(v.signedReceipt), v.id)
      XCTAssertEqual(result.receipt.contextHash, try hex(v.pinnedKey.keyId), v.id)
      XCTAssertEqual(try verifySequencerReceipt(v).receiptDigest, result.receiptDigest, v.id)
    }
  }
  func testSequencerReceiptV2RevertDecodesReason() throws {
    let cases = try vectors("V2")
    XCTAssertEqual(cases.count, 1)
    for v in cases {
      let result = try verifySequencerReceipt(v)
      XCTAssertEqual(result.level, v.expected.rung)
      XCTAssertEqual(result.status, 0)
      XCTAssertEqual(
        v.receiptJson["status"], .string("0x" + String(result.status, radix: 16)), v.id)
      XCTAssertEqual(result.receipt.resultCode, 1)
      XCTAssertEqual(result.receiptDigest, try hex(v.receiptDigest))
      XCTAssertEqual(
        try XCTUnwrap(result.revert).reason,
        "LXP_INSUFFICIENT_BALANCE: insufficient balance")
    }
  }
  func testSequencerReceiptV3RefusalsNameTheirCheck() throws {
    let expected: [String: (rung: String, check: ReceiptCheck, description: String)] = [
      "V3-key-not-pinned": (
        "R3", .keyId, "R3: keyId is not in the pinned ReceiptKeys"
      ),
      "V3-height-outside-key-range": (
        "R3", .keyHeight, "R3: batch_id height is outside the pinned key range"
      ),
      "V3-activity-id-not-raw-hash": (
        "R4", .rawTransaction, "R4: activity_id is not keccak256 of the raw transaction"
      ),
      "V3-seq-hash-mismatch": (
        "R6", .sequenceHash, "R6: seqHash does not chain prevSeqHash, seq and activity_id"
      ),
      "V3-tampered-log": (
        "R5", .logs, "sequencer receipt: JSON view differs from receipt bytes"
      ),
      "V3-non-canonical-bytes": (
        "R2", .canonicalEncoding, "layerx codec: non-canonical"
      ),
    ]
    let cases = try vectors("V3")
    XCTAssertEqual(cases.count, expected.count)
    XCTAssertEqual(Set(cases.map(\.id)), Set(expected.keys))
    for v in cases {
      let refusal = try XCTUnwrap(expected[v.id])
      XCTAssertEqual(v.expected.refusedAt, refusal.rung, v.id)
      XCTAssertEqual(v.expected.check, refusal.description, v.id)
      XCTAssertThrowsError(try verifySequencerReceipt(v), v.id) { error in
        XCTAssertEqual((error as? PlatformSDKError)?.receiptCheck, refusal.check, v.id)
      }
    }
  }

  func testPublishedVectorFilesMatchRecordedCases() throws {
    try testSequencerReceiptV1CasesVerifyReceipted()
    try testSequencerReceiptV2RevertDecodesReason()
    try testSequencerReceiptV3RefusalsNameTheirCheck()
  }
}
