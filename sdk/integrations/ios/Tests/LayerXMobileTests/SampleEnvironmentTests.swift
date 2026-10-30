import Foundation
import LayerXSDK
import XCTest

@testable import LayerXMobileSampleKit

final class SampleEnvironmentTests: XCTestCase {
  func testDecodesSignedReceiptPayload() throws {
    let keyID = String(repeating: "01", count: 32)
    let publicKey = String(repeating: "ab", count: 32)
    let prevSeqHash = String(repeating: "cd", count: 32)
    let json = """
      {
        "raw_transaction": "0x02abcdef",
        "signed_receipt": "0x01020304",
        "receipt_key": {
          "key_id": "\(keyID)",
          "public_key": "\(publicKey)",
          "first_height": 42,
          "last_height": 18446744073709551615
        },
        "receipt_json": {
          "status": "0x1",
          "blockNumber": "0x2a",
          "logs": [{"address": "0x1234", "topics": [], "data": "0x"}]
        },
        "prev_seq_hash": "\(prevSeqHash)"
      }
      """
    let data = Data(json.utf8)
    let decoder = JSONDecoder()
    let payload = try decoder.decode(RelayReceiptPayload.self, from: data)

    XCTAssertEqual(payload.rawTransaction, "0x02abcdef")
    XCTAssertEqual(payload.signedReceipt, "0x01020304")
    XCTAssertEqual(payload.receiptKey.keyID, keyID)
    XCTAssertEqual(payload.receiptKey.publicKey, publicKey)
    XCTAssertEqual(payload.receiptKey.firstHeight, 42)
    XCTAssertEqual(payload.receiptKey.lastHeight, UInt64.max)
    XCTAssertEqual(
      payload.receiptJSON,
      [
        "status": .string("0x1"),
        "blockNumber": .string("0x2a"),
        "logs": .array([
          .object([
            "address": .string("0x1234"), "topics": .array([]), "data": .string("0x"),
          ])
        ]),
      ]
    )
    XCTAssertEqual(payload.prevSeqHash, prevSeqHash)

    let object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    for field in [
      "raw_transaction", "signed_receipt", "receipt_key", "receipt_json", "prev_seq_hash",
    ] {
      var incomplete = object
      incomplete.removeValue(forKey: field)
      let incompleteData = try JSONSerialization.data(withJSONObject: incomplete)
      XCTAssertThrowsError(try decoder.decode(RelayReceiptPayload.self, from: incompleteData)) {
        error in
        guard case let DecodingError.keyNotFound(key, _) = error else {
          return XCTFail("Expected a missing coding key for \(field), got \(error)")
        }
        XCTAssertEqual(key.stringValue, field)
      }
    }

    let legacyData = Data(
      #"{"canonical_receipt_base64":"AQID","authorized_batch":{}}"#.utf8)
    XCTAssertThrowsError(try decoder.decode(RelayReceiptPayload.self, from: legacyData)) {
      error in
      guard case let DecodingError.keyNotFound(key, _) = error else {
        return XCTFail("Expected a missing signed receipt field, got \(error)")
      }
      XCTAssertEqual(key.stringValue, "raw_transaction")
    }
  }
}
