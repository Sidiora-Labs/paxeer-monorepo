import Foundation
import LayerXMobile
import LayerXMobileSampleKit
import LayerXSDK
import XCTest

final class ReceiptGateTests: XCTestCase {
  private struct Fixture: Decodable {
    let cases: [Vector]
  }

  private struct Vector: Decodable {
    struct Key: Decodable {
      let keyId: String
      let publicKey: String
      let firstHeight: UInt64
      let lastHeight: UInt64
    }

    let id: String
    let rawTransaction: String
    let signedReceipt: String
    let receiptDigest: String
    let receiptJson: [String: JSONValue]
    let prevSeqHash: String
    let pinnedKey: Key
  }

  private func vector() throws -> Vector {
    let fixtures = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent()
      .appendingPathComponent("platform/conformance/fixtures/vector-v1-receipted-positive.json")
    let fixture = try JSONDecoder().decode(Fixture.self, from: Data(contentsOf: fixtures))
    return try XCTUnwrap(fixture.cases.first { $0.id == "V1-pax-transfer" })
  }

  private func hex(_ value: String) throws -> Data {
    XCTAssertTrue(value.hasPrefix("0x"))
    let characters = Array(value.dropFirst(2))
    XCTAssertEqual(characters.count % 2, 0)
    return try Data(
      stride(from: 0, to: characters.count, by: 2).map {
        try XCTUnwrap(UInt8(String(characters[$0...($0 + 1)]), radix: 16))
      })
  }

  private func evidence(_ vector: Vector) throws -> ReceiptGate.Evidence {
    ReceiptGate.Evidence(
      rawTransaction: try hex(vector.rawTransaction),
      signedReceipt: try hex(vector.signedReceipt),
      receiptKey: ReceiptKeys(
        keyId: try hex(vector.pinnedKey.keyId),
        publicKey: try hex(vector.pinnedKey.publicKey),
        firstHeight: vector.pinnedKey.firstHeight,
        lastHeight: vector.pinnedKey.lastHeight
      ),
      receiptJSON: vector.receiptJson,
      prevSeqHash: try hex(vector.prevSeqHash)
    )
  }

  private func expectation() throws -> MobileSettlementExpectation {
    try MobileSettlementExpectation(
      asset: hex("0x0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73"),
      recipient: hex("0x0000000000000000000000008ba1f109551bd432803012645ac136ddd64dba72"),
      amount: "1000000000000000000"
    )
  }

  private func gate(keys: [ReceiptKeys]) throws -> ReceiptGate {
    let resolver = try RelayReceiptResolver(
      relayURL: XCTUnwrap(URL(string: "https://relay.example.invalid/receipts")),
      session: SampleEnvironment.session(timeoutSeconds: 1)
    )
    return ReceiptGate(receipts: resolver, receiptKeys: keys)
  }

  private func configuration(receiptKeysJSON: String?) throws -> PublishableConfiguration {
    var environment = [
      "LAYERX_SERVICE_URL": "https://service.example.invalid",
      "LAYERX_SESSION_BROKER_URL": "https://broker.example.invalid",
      "LAYERX_EVENT_KEY_ID": "events",
      "LAYERX_EVENT_PUBLIC_KEY": String(repeating: "01", count: 32),
    ]
    environment["LAYERX_RECEIPT_KEYS_JSON"] = receiptKeysJSON
    return try PublishableConfiguration(environment: environment)
  }

  func testVerifiesSignedSequencerReceipt() async throws {
    let vector = try vector()
    let evidence = try evidence(vector)
    let gate = try gate(keys: [evidence.receiptKey])
    let expected = try expectation()
    let state = try await gate.verify(evidence: evidence, expecting: expected)
    XCTAssertEqual(
      state, .verified(level: "receipted", receiptDigest: String(vector.receiptDigest.dropFirst(2)))
    )

    let keyJSON: [[String: Any]] = [
      [
        "keyId": vector.pinnedKey.keyId,
        "publicKey": vector.pinnedKey.publicKey,
        "firstHeight": vector.pinnedKey.firstHeight,
        "lastHeight": vector.pinnedKey.lastHeight,
      ]
    ]
    let keysJSON = String(
      decoding: try JSONSerialization.data(withJSONObject: keyJSON), as: UTF8.self)
    let configuration = try configuration(receiptKeysJSON: keysJSON)
    XCTAssertEqual(configuration.receiptKeys.count, 1)
    XCTAssertEqual(configuration.receiptKeys.first?.keyId, evidence.receiptKey.keyId)
    XCTAssertEqual(configuration.receiptKeys.first?.publicKey, evidence.receiptKey.publicKey)
    XCTAssertEqual(configuration.receiptKeys.first?.firstHeight, evidence.receiptKey.firstHeight)
    XCTAssertEqual(configuration.receiptKeys.first?.lastHeight, evidence.receiptKey.lastHeight)
    for value in [vector.pinnedKey.keyId, vector.pinnedKey.publicKey] {
      XCTAssertTrue(configuration.exemptScannerValues.contains(value))
      XCTAssertTrue(configuration.exemptScannerValues.contains(String(value.dropFirst(2))))
    }
    let mobile = try LayerXMobile(
      configuration: configuration, deliveries: InMemoryEventDeliveryStore())
    let resolver = try RelayReceiptResolver(
      relayURL: XCTUnwrap(URL(string: "https://relay.example.invalid/receipts")),
      session: SampleEnvironment.session(timeoutSeconds: 1)
    )
    let configuredState = try await mobile.gate(receipts: resolver).verify(
      evidence: evidence, expecting: expected)
    XCTAssertEqual(configuredState, state)

    var damagedSignature = evidence.signedReceipt
    damagedSignature[damagedSignature.count - 1] ^= 1
    var damagedJSON = evidence.receiptJSON
    damagedJSON["status"] = .string("0x0")
    let invalidEvidence = [
      ReceiptGate.Evidence(
        rawTransaction: evidence.rawTransaction, signedReceipt: damagedSignature,
        receiptKey: evidence.receiptKey, receiptJSON: evidence.receiptJSON,
        prevSeqHash: evidence.prevSeqHash
      ),
      ReceiptGate.Evidence(
        rawTransaction: evidence.rawTransaction + Data([0]), signedReceipt: evidence.signedReceipt,
        receiptKey: evidence.receiptKey, receiptJSON: evidence.receiptJSON,
        prevSeqHash: evidence.prevSeqHash
      ),
      ReceiptGate.Evidence(
        rawTransaction: evidence.rawTransaction, signedReceipt: evidence.signedReceipt,
        receiptKey: evidence.receiptKey, receiptJSON: damagedJSON,
        prevSeqHash: evidence.prevSeqHash
      ),
      ReceiptGate.Evidence(
        rawTransaction: evidence.rawTransaction, signedReceipt: evidence.signedReceipt,
        receiptKey: evidence.receiptKey, receiptJSON: evidence.receiptJSON,
        prevSeqHash: Data(repeating: 0xff, count: 32)
      ),
    ]
    for invalid in invalidEvidence {
      do {
        _ = try await gate.verify(evidence: invalid, expecting: expected)
        XCTFail("Invalid signed sequencer evidence must be refused")
      } catch {
        XCTAssertEqual(
          error as? MobileIntegrationError, MobileIntegrationError(.verificationFailure))
      }
    }

    let mismatches = [
      try MobileSettlementExpectation(
        asset: Data(repeating: 0xff, count: 32), recipient: expected.recipient,
        amount: expected.amount),
      try MobileSettlementExpectation(
        asset: expected.asset, recipient: Data(repeating: 0xff, count: 32), amount: expected.amount),
      try MobileSettlementExpectation(
        asset: expected.asset, recipient: expected.recipient, amount: "1"),
    ]
    for mismatch in mismatches {
      do {
        _ = try await gate.verify(evidence: evidence, expecting: mismatch)
        XCTFail("A signed receipt must match the expected settlement")
      } catch {
        XCTAssertEqual(
          error as? MobileIntegrationError, MobileIntegrationError(.verificationFailure))
      }
    }
  }

  func testRefusesReceiptUnderUnpinnedKey() async throws {
    let configuration = try configuration(receiptKeysJSON: nil)
    XCTAssertTrue(configuration.receiptKeys.isEmpty)
    let mobile = try LayerXMobile(
      configuration: configuration, deliveries: InMemoryEventDeliveryStore())
    let resolver = try RelayReceiptResolver(
      relayURL: XCTUnwrap(URL(string: "https://relay.example.invalid/receipts")),
      session: SampleEnvironment.session(timeoutSeconds: 1)
    )
    XCTAssertThrowsError(try mobile.gate(receipts: resolver)) {
      XCTAssertEqual($0 as? MobileIntegrationError, MobileIntegrationError(.invalidConfiguration))
    }
    for invalidJSON in ["[]", "null", "{}", "not JSON", "[{}]"] {
      XCTAssertThrowsError(try self.configuration(receiptKeysJSON: invalidJSON)) {
        XCTAssertEqual($0 as? MobileIntegrationError, MobileIntegrationError(.invalidConfiguration))
      }
    }
    let evidence = try evidence(vector())
    let expected = try expectation()
    let key = evidence.receiptKey
    let unpinnedSets: [[ReceiptKeys]] = [
      [],
      [
        ReceiptKeys(
          keyId: Data(repeating: 0xff, count: 32), publicKey: key.publicKey,
          firstHeight: key.firstHeight, lastHeight: key.lastHeight)
      ],
      [
        ReceiptKeys(
          keyId: key.keyId, publicKey: Data(repeating: 0xff, count: 32),
          firstHeight: key.firstHeight, lastHeight: key.lastHeight)
      ],
      [
        ReceiptKeys(
          keyId: key.keyId, publicKey: key.publicKey, firstHeight: key.firstHeight + 1,
          lastHeight: key.lastHeight)
      ],
      [
        ReceiptKeys(
          keyId: key.keyId, publicKey: key.publicKey, firstHeight: key.firstHeight,
          lastHeight: key.lastHeight + 1)
      ],
    ]
    for pins in unpinnedSets {
      do {
        _ = try await gate(keys: pins).verify(evidence: evidence, expecting: expected)
        XCTFail("Evidence must not supply its own trusted key or height range")
      } catch {
        XCTAssertEqual(
          error as? MobileIntegrationError, MobileIntegrationError(.verificationFailure))
      }
    }
    let state = try await gate(keys: [key]).verify(evidence: evidence, expecting: expected)
    XCTAssertEqual(
      state,
      .verified(level: "receipted", receiptDigest: String(try vector().receiptDigest.dropFirst(2))))
  }
}
