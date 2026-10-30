import Crypto
import Foundation
import XCTest

@testable import LayerXSDK

final class PortableReceiptTests: XCTestCase {
  private typealias PortableReceipt = LayerXSDK.PortableReceipt

  private struct PublishedFixture: Decodable {
    let bundles: [[String: JSONValue]]
  }
  private struct Fixture: Decodable { let cases: [Vector] }
  private struct Vector: Decodable {
    let id: String
    let vector: String
    let rawTransaction: String
    let signedReceipt: String
    let receiptDigest: String
    let receiptJson: [String: JSONValue]
    let prevSeqHash: String
    let pinnedKey: Key
  }
  private struct Key: Decodable {
    let keyId: String
    let publicKey: String
    let firstHeight: UInt64
    let lastHeight: UInt64
  }
  private struct Evidence {
    let header: CommittedHeader
    var inclusion: ReceiptInclusion
    var commit: CometCommit
    let validators: [CometValidator]
  }

  private func vectors() throws -> [Vector] {
    let url = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("conformance/fixtures/sequencer-receipt-v1.json")
    return try JSONDecoder().decode(Fixture.self, from: Data(contentsOf: url)).cases.filter {
      $0.vector == "V1"
    }
  }

  private func hex(_ text: String) throws -> Data {
    XCTAssertTrue(text.hasPrefix("0x"))
    let bytes = Array(text.dropFirst(2))
    XCTAssertEqual(bytes.count % 2, 0)
    return try Data(
      stride(from: 0, to: bytes.count, by: 2).map {
        try XCTUnwrap(UInt8(String(bytes[$0...$0 + 1]), radix: 16))
      })
  }

  private func key(_ v: Vector) throws -> ReceiptKeys {
    try key(v.pinnedKey)
  }

  private func key(_ pin: Key) throws -> ReceiptKeys {
    ReceiptKeys(
      keyId: try hex(pin.keyId), publicKey: try hex(pin.publicKey),
      firstHeight: pin.firstHeight, lastHeight: pin.lastHeight)
  }

  private func portableVectorV5Sources(_ directory: URL) throws
    -> [String: (pin: ReceiptKeys, prevSeqHash: Data)]
  {
    struct CommittedFixture: Decodable {
      struct Source: Decodable {
        let pinnedKey: Key
        let prevSeqHash: String
      }
      let positive: Source
    }
    let receipted = try XCTUnwrap(
      JSONDecoder().decode(
        Fixture.self,
        from: Data(
          contentsOf: directory.appendingPathComponent("vector-v1-receipted-positive.json"))
      ).cases.first)
    XCTAssertEqual(receipted.id, "V1-pax-transfer")
    let committed = try JSONDecoder().decode(
      CommittedFixture.self,
      from: Data(contentsOf: directory.appendingPathComponent("vector-v4-committed.json"))
    ).positive
    return [
      "receipted": (try key(receipted), try hex(receipted.prevSeqHash)),
      "committed": (try key(committed.pinnedKey), try hex(committed.prevSeqHash)),
    ]
  }

  private func export(_ v: Vector, evidence: Evidence? = nil) throws -> PortableReceipt {
    try PortableReceipt.export(
      rawTransaction: hex(v.rawTransaction), signedReceipt: hex(v.signedReceipt),
      receiptJSON: v.receiptJson, receiptKey: key(v), prevSeqHash: hex(v.prevSeqHash),
      blockHeader: evidence?.header, receiptInclusion: evidence?.inclusion,
      commit: evidence?.commit, validatorSet: evidence?.validators)
  }

  private func document(_ receipt: PortableReceipt) throws -> [String: JSONValue] {
    try JSONDecoder().decode([String: JSONValue].self, from: receipt.export())
  }

  private func encode(_ value: [String: JSONValue]) throws -> Data {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
    return try encoder.encode(value)
  }

  private func evidence(_ v: Vector, _ receipt: PortableReceipt) throws -> Evidence {
    let signer = try Curve25519.Signing.PrivateKey(
      rawRepresentation: Data(repeating: 0x35, count: 32))
    let standby = try Curve25519.Signing.PrivateKey(
      rawRepresentation: Data(repeating: 0x36, count: 32))
    let validators = [
      CometValidator(publicKey: signer.publicKey.rawRepresentation, votingPower: 10),
      CometValidator(publicKey: standby.publicKey.rawRepresentation, votingPower: 1),
    ]
    let setHash = CommittedWire.validatorsHash(validators)
    let r = receipt.receiptVerification.receipt
    let height = Int64(CommittedWire.bigEndian(r.batchID.suffix(8)))
    func be(_ value: UInt64) -> Data {
      Data((0..<8).reversed().map { UInt8(truncatingIfNeeded: value >> ($0 * 8)) })
    }
    let domain = Data("PAXEER/v1/sequence-batch\0".utf8)
    var batch =
      domain + be(UInt64(height)) + be(r.globalSequence) + be(r.globalSequence)
      + r.previousStateRoot + r.resultingStateRoot + CommittedWire.sha(r.activityID) + r.contextHash
    batch += try signer.signature(for: CommittedWire.sha(batch))
    let raw = try hex(v.rawTransaction)
    let message =
      CommittedWire.bytes(1, Data("/ethermint.evm.v1.MsgEthereumTx".utf8))
      + CommittedWire.bytes(2, CommittedWire.bytes(1, raw))
    let transaction = CommittedWire.bytes(1, CommittedWire.bytes(1, message))
    let leaves = [batch, transaction].map(CommittedWire.sha)
    let header = CommittedHeader(
      versionApp: 2, chainID: "portable-receipt-test", height: height,
      time: .init(seconds: 1_700_000_123, nanos: 987_654_321),
      dataHash: CommittedWire.root(leaves), validatorsHash: setHash, nextValidatorsHash: setHash)
    func commit(_ header: CommittedHeader) throws -> CometCommit {
      var result = CometCommit(
        height: header.height, round: 1,
        blockID: .init(hash: header.hash, partSetTotal: 1, partSetHash: setHash), signatures: [])
      var signature = CometCommitSignature(
        blockIDFlag: .block,
        validatorAddress: Data(CommittedWire.sha(signer.publicKey.rawRepresentation).prefix(20)),
        timestamp: header.time)
      signature.signature = try signer.signature(
        for: CommittedWire.voteBytes(header.chainID, result, signature))
      result.signatures = [signature, CometCommitSignature(blockIDFlag: .absent)]
      return result
    }
    let signed = try commit(header)
    let gas = Int64(CommittedWire.bigEndian(r.effects[0].body.prefix(8)))
    let result = ExecTxResult(gasWanted: gas + 1000, gasUsed: gas)
    let results = [ExecTxResult().encoded, result.encoded]
    var next = header
    next.height += 1
    next.time.seconds += 1
    next.lastBlockID = signed.blockID
    next.lastResultsHash = CommittedWire.root(results)
    return Evidence(
      header: header,
      inclusion: ReceiptInclusion(
        transaction: transaction,
        transactionProof: .init(total: 2, index: 1, aunts: [CommittedWire.leaf(leaves[0])]),
        result: result,
        resultProof: .init(total: 2, index: 1, aunts: [CommittedWire.leaf(results[0])]),
        batchTransaction: batch,
        batchProof: .init(total: 2, index: 0, aunts: [CommittedWire.leaf(leaves[1])]),
        nextHeader: next, nextCommit: try commit(next), nextValidators: validators),
      commit: signed, validators: validators)
  }

  func testPortableVectorV5Verifies() throws {
    let url = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("conformance/fixtures/vector-v5-portable.json")
    let bundles = try JSONDecoder().decode(
      PublishedFixture.self, from: Data(contentsOf: url)
    ).bundles
    XCTAssertEqual(bundles.count, 2)
    let levels = try bundles.map { try XCTUnwrap($0["verificationLevel"]?.stringValue) }
    XCTAssertEqual(Set(levels), ["receipted", "committed"])
    let sources = try portableVectorV5Sources(url.deletingLastPathComponent())

    for bundle in bundles {
      let level = try XCTUnwrap(bundle["verificationLevel"]?.stringValue)
      let source = try XCTUnwrap(sources[level])
      let pinned = source.pin
      let carriedKey = try XCTUnwrap(bundle["receiptKey"]?.objectValue)
      func height(_ field: String) throws -> UInt64 {
        let text = try XCTUnwrap(carriedKey[field]?.stringValue)
        XCTAssertTrue(text.hasPrefix("0x"), field)
        let value = try XCTUnwrap(UInt64(text.dropFirst(2), radix: 16))
        XCTAssertEqual(text, "0x" + String(value, radix: 16), field)
        return value
      }
      XCTAssertEqual(try hex(XCTUnwrap(carriedKey["keyId"]?.stringValue)), pinned.keyId, level)
      XCTAssertEqual(
        try hex(XCTUnwrap(carriedKey["publicKey"]?.stringValue)), pinned.publicKey, level)
      XCTAssertEqual(try height("firstHeight"), pinned.firstHeight, level)
      XCTAssertEqual(try height("lastHeight"), pinned.lastHeight, level)
      let bytes = try encode(bundle)
      let verified = try PortableReceipt.verify(
        bytes, keys: [pinned], prevSeqHash: source.prevSeqHash)
      XCTAssertEqual(verified.verificationLevel, level, level)
      XCTAssertEqual(verified.status, 1, level)
      XCTAssertEqual(
        verified.receiptDigest, try hex(XCTUnwrap(bundle["receiptDigest"]?.stringValue)), level)
      XCTAssertEqual(
        verified.receiptVerification.canonicalBytes,
        try hex(XCTUnwrap(bundle["signedReceipt"]?.stringValue)), level)
      XCTAssertEqual(verified.committed?.level, level == "committed" ? level : nil, level)
      XCTAssertEqual(verified.export(), bytes, level)
    }
  }

  func testPortableVectorV5RefusesForeignPin() throws {
    let url = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("conformance/fixtures/vector-v5-portable.json")
    let bundles = try JSONDecoder().decode(
      PublishedFixture.self, from: Data(contentsOf: url)
    ).bundles
    XCTAssertEqual(bundles.count, 2)
    let levels = try bundles.map { try XCTUnwrap($0["verificationLevel"]?.stringValue) }
    XCTAssertEqual(Set(levels), ["receipted", "committed"])
    let sources = try portableVectorV5Sources(url.deletingLastPathComponent())

    for bundle in bundles {
      let level = try XCTUnwrap(bundle["verificationLevel"]?.stringValue)
      let source = try XCTUnwrap(sources[level])
      let foreign = try XCTUnwrap(sources[level == "receipted" ? "committed" : "receipted"])
      let bytes = try encode(bundle)
      XCTAssertThrowsError(
        try PortableReceipt.verify(
          bytes, keys: [foreign.pin], prevSeqHash: source.prevSeqHash), level
      ) {
        XCTAssertEqual(($0 as? PlatformSDKError)?.code, .verificationFailure, level)
      }
    }
  }

  func testPortableReceiptRoundTripsAtReceipted() throws {
    let cases = try vectors()
    XCTAssertEqual(cases.count, 4)
    for v in cases {
      let exported = try export(v)
      XCTAssertEqual(exported.verificationLevel, "receipted", v.id)
      XCTAssertEqual(exported.status, 1, v.id)
      XCTAssertNil(exported.committed, v.id)
      let fields = try document(exported)
      XCTAssertEqual(
        Set(fields.keys),
        [
          "format", "verificationLevel", "rawTransaction", "signedReceipt", "receiptDigest",
          "receiptKey", "receiptJson",
        ], v.id)
      XCTAssertEqual(fields["format"], .string("layerx-receipt-proof-v2"), v.id)
      XCTAssertEqual(fields["verificationLevel"], .string("receipted"), v.id)
      XCTAssertEqual(fields["rawTransaction"], .string(v.rawTransaction), v.id)
      XCTAssertEqual(fields["signedReceipt"], .string(v.signedReceipt), v.id)
      XCTAssertEqual(fields["receiptDigest"], .string(v.receiptDigest), v.id)
      XCTAssertEqual(fields["receiptJson"], .object(v.receiptJson), v.id)
      let pinned = try key(v)
      let standby = ReceiptKeys(
        keyId: Data(repeating: 7, count: 32), publicKey: pinned.publicKey,
        firstHeight: 0, lastHeight: UInt64.max)
      let verified = try PortableReceipt.verify(
        exported.export(), keys: [standby, pinned], prevSeqHash: hex(v.prevSeqHash))
      XCTAssertEqual(verified.verificationLevel, "receipted", v.id)
      XCTAssertEqual(verified.receiptDigest, try hex(v.receiptDigest), v.id)
      XCTAssertEqual(verified.receiptVerification.canonicalBytes, try hex(v.signedReceipt), v.id)
      XCTAssertEqual(verified.export(), exported.export(), v.id)
      XCTAssertEqual(
        try PortableReceipt.verify(exported.export(), keys: [pinned]).export(),
        exported.export(), v.id)
      let spaced = Data(" \n".utf8) + exported.export() + Data("\n".utf8)
      XCTAssertEqual(try PortableReceipt.verify(spaced, keys: [pinned]).export(), spaced, v.id)
      XCTAssertThrowsError(try PortableReceipt.verify(exported.export(), keys: [standby]), v.id)
      XCTAssertThrowsError(
        try PortableReceipt.verify(
          exported.export(), keys: [pinned], prevSeqHash: Data(repeating: 0xff, count: 32)), v.id)
      for field in ["rawTransaction", "signedReceipt", "receiptDigest"] {
        var tampered = fields
        let original = try XCTUnwrap(fields[field]?.stringValue)
        tampered[field] = .string(String(original.dropLast()) + (original.last == "0" ? "1" : "0"))
        XCTAssertThrowsError(try PortableReceipt.verify(encode(tampered), keys: [pinned]), field)
      }
    }

    let v = try XCTUnwrap(cases.first { $0.id == "V1-pax-transfer" })
    let pinned = try key(v)
    let receipted = try export(v)
    let proof = try evidence(v, receipted)
    let exported = try export(v, evidence: proof)
    let verified = try PortableReceipt.verify(exported.export(), keys: [pinned])
    XCTAssertEqual(verified.verificationLevel, "committed")
    XCTAssertEqual(verified.status, 1)
    XCTAssertEqual(try XCTUnwrap(verified.committed).blockHash, proof.header.hash)
    XCTAssertEqual(verified.receiptDigest, receipted.receiptDigest)
    XCTAssertEqual(verified.export(), exported.export())
    let fields = try document(exported)
    for field in ["blockHeader", "receiptInclusion", "commit", "validatorSet"] {
      XCTAssertNotNil(fields[field], field)
      var missing = fields
      missing.removeValue(forKey: field)
      XCTAssertThrowsError(try PortableReceipt.verify(encode(missing), keys: [pinned]), field)
    }
    var invalid = proof
    invalid.commit.signatures[0].signature[0] ^= 1
    XCTAssertThrowsError(try export(v, evidence: invalid))
    invalid = proof
    invalid.inclusion.resultProof.aunts[0][0] ^= 1
    XCTAssertThrowsError(try export(v, evidence: invalid))
    XCTAssertThrowsError(
      try PortableReceipt.export(
        rawTransaction: hex(v.rawTransaction), signedReceipt: hex(v.signedReceipt),
        receiptJSON: v.receiptJson, receiptKey: pinned, blockHeader: proof.header))
  }

  func testPortableReceiptRefusesVersionOneFormat() throws {
    let cases = try vectors()
    XCTAssertEqual(cases.count, 4)
    for v in cases {
      let exported = try export(v)
      let pinned = try key(v)
      let fields = try document(exported)
      var legacy = fields
      legacy["format"] = .string("layerx-receipt-proof-v1")
      XCTAssertThrowsError(try PortableReceipt.verify(encode(legacy), keys: [pinned]), v.id) {
        XCTAssertEqual(
          $0 as? PlatformSDKError,
          PlatformSDKError(code: .verificationFailure, retry: .never))
      }
      for field in ["batchId", "asset", "previousStateRoot", "resultingStateRoot"] {
        for value in [JSONValue.null, .string("0x" + String(repeating: "00", count: 32))] {
          var retired = fields
          retired[field] = value
          XCTAssertThrowsError(try PortableReceipt.verify(encode(retired), keys: [pinned]), field)
          var view = v.receiptJson
          view[field] = value
          retired = fields
          retired["receiptJson"] = .object(view)
          XCTAssertThrowsError(try PortableReceipt.verify(encode(retired), keys: [pinned]), field)
          XCTAssertThrowsError(
            try PortableReceipt.export(
              rawTransaction: hex(v.rawTransaction), signedReceipt: hex(v.signedReceipt),
              receiptJSON: view, receiptKey: pinned), field)
          var carried = try XCTUnwrap(fields["receiptKey"]?.objectValue)
          carried[field] = value
          retired = fields
          retired["receiptKey"] = .object(carried)
          XCTAssertThrowsError(try PortableReceipt.verify(encode(retired), keys: [pinned]), field)
        }
      }
      var unpinned = try XCTUnwrap(fields["receiptKey"]?.objectValue)
      for field in ["keyId", "publicKey", "firstHeight", "lastHeight"] {
        var changed = unpinned
        changed[field] = .string(
          field.contains("Height") ? "0x0" : "0x" + String(repeating: "00", count: 32))
        var bundle = fields
        bundle["receiptKey"] = .object(changed)
        XCTAssertThrowsError(try PortableReceipt.verify(encode(bundle), keys: [pinned]), field)
      }
      unpinned["firstHeight"] = .string("0x00")
      var malformed = fields
      malformed["receiptKey"] = .object(unpinned)
      XCTAssertThrowsError(try PortableReceipt.verify(encode(malformed), keys: [pinned]))
      malformed = fields
      malformed["verificationLevel"] = .string("sequencer-signed")
      XCTAssertThrowsError(try PortableReceipt.verify(encode(malformed), keys: [pinned]))
      malformed = fields
      malformed["commit"] = .null
      XCTAssertThrowsError(try PortableReceipt.verify(encode(malformed), keys: [pinned]))
      let text = try XCTUnwrap(String(data: exported.export(), encoding: .utf8))
      let duplicate = "{\"format\":\"layerx-receipt-proof-v1\"," + text.dropFirst()
      XCTAssertThrowsError(try PortableReceipt.verify(Data(duplicate.utf8), keys: [pinned]))
      XCTAssertThrowsError(
        try PortableReceipt.verify(exported.export() + Data("{}".utf8), keys: [pinned]))
    }
  }
}
