import Foundation
import XCTest

@testable import LayerXSDK

final class KernelVerifierRetiredTests: XCTestCase {
  func testVerificationRungsAreReceiptedAndCommittedOnly() {
    XCTAssertEqual(VerificationLevel.allCases.map(\.rawValue), ["receipted", "committed"])
    for retired in [
      "submitted", "sequencer-signed", "batch-included", "state-proven", "checkpoint-finalised",
      "settlement-anchored",
    ] {
      XCTAssertNil(VerificationLevel(rawValue: retired), retired)
    }
  }

  func testKernelReceiptBytesAreRefusedBySequencerVerifier() throws {
    let kernelReceipt = try hex(Self.kernelReceiptHex)
    XCTAssertThrowsError(
      try LocalVerifier.verifySequencerReceipt(
        kernelReceipt, rawTransaction: Data(), keys: [], receiptJSON: [:],
        prevSeqHash: Data(repeating: 0, count: 32))
    ) {
      let error = $0 as? PlatformSDKError
      XCTAssertEqual(error?.code, .verificationFailure)
      XCTAssertEqual(error?.receiptCheck, .decode)
    }
  }

  func testCommittedVectorV4Verifies() throws {
    let url = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("conformance/fixtures/vector-v4-committed.json")
    let decoder = JSONDecoder()
    decoder.keyDecodingStrategy = .custom { path in
      let name = path.last!.stringValue
      return FixtureKey(stringValue: name.prefix(1).lowercased() + name.dropFirst())!
    }
    let vector = try decoder.decode(Vector.self, from: Data(contentsOf: url))
    let v = vector.positive
    let receipt = try LocalVerifier.verifySequencerReceipt(
      hex(v.signedReceipt), rawTransaction: hex(v.rawTransaction),
      keys: [
        ReceiptKeys(
          keyId: try hex(v.pinnedKey.keyId), publicKey: try hex(v.pinnedKey.publicKey),
          firstHeight: v.pinnedKey.firstHeight, lastHeight: v.pinnedKey.lastHeight)
      ],
      receiptJSON: v.receiptJson, prevSeqHash: hex(v.prevSeqHash))
    let header = try v.blockHeader.value()
    let inclusion = try v.receiptInclusion.value()
    let commit = try v.commit.value()
    let validators = v.validatorSet.map(\.value)
    let verified = try Committed.verify(
      receipt, blockHeader: header, receiptInclusion: inclusion,
      commit: commit, validatorSet: validators)
    XCTAssertEqual(receipt.level, VerificationLevel.receipted.rawValue)
    XCTAssertEqual(verified.rung, VerificationLevel.committed.rawValue)
    XCTAssertEqual(verified.rung, v.expected.rung)
    XCTAssertEqual(verified.receiptDigest, try hex(v.receiptDigest))
    XCTAssertEqual(verified.height, UInt64(header.height))
    XCTAssertEqual(verified.blockHash, header.hash)
    XCTAssertFalse(vector.refusals.isEmpty)
    for refusal in vector.refusals {
      var refusedInclusion = inclusion
      refusedInclusion.nextCommit = try refusal.nextCommit.value()
      XCTAssertThrowsError(
        try Committed.verify(
          receipt, blockHeader: header,
          receiptInclusion: refusedInclusion, commit: commit, validatorSet: validators), refusal.id
      ) {
        XCTAssertEqual($0 as? CommittedError, .nextCommit, refusal.id)
      }
      XCTAssertEqual(refusal.expectedCheck, "committed-next-commit", refusal.id)
    }
  }

  private struct FixtureKey: CodingKey {
    let stringValue: String
    var intValue: Int? { nil }
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
  }
  private struct Vector: Decodable {
    let positive: Positive
    let refusals: [Refusal]
  }
  private struct Key: Decodable {
    let keyId: String
    let publicKey: String
    let firstHeight: UInt64
    let lastHeight: UInt64
  }
  private struct Positive: Decodable {
    let rawTransaction: String
    let signedReceipt: String
    let receiptJson: [String: JSONValue]
    let prevSeqHash: String
    let receiptDigest: String
    let pinnedKey: Key
    let blockHeader: Header
    let receiptInclusion: Inclusion
    let commit: Commit
    let validatorSet: [Validator]
    let expected: Expected
  }
  private struct Expected: Decodable { let rung: String }
  private struct Refusal: Decodable {
    let id: String
    let nextCommit: Commit
    let expectedCheck: String
  }
  private struct BlockID: Decodable {
    let hash: Data
    let partSetTotal: UInt32
    let partSetHash: Data
    var value: CometBlockID {
      .init(hash: hash, partSetTotal: partSetTotal, partSetHash: partSetHash)
    }
  }
  private struct Header: Decodable {
    let versionBlock: UInt64
    let versionApp: UInt64
    let chainID: String
    let height: Int64
    let time: String
    let lastBlockID: BlockID
    let lastCommitHash: Data
    let dataHash: Data
    let validatorsHash: Data
    let nextValidatorsHash: Data
    let consensusHash: Data
    let appHash: Data
    let lastResultsHash: Data
    let evidenceHash: Data
    let proposerAddress: Data
    func value() throws -> CommittedHeader {
      .init(
        versionBlock: versionBlock, versionApp: versionApp, chainID: chainID, height: height,
        time: try KernelVerifierRetiredTests.timestamp(time), lastBlockID: lastBlockID.value,
        lastCommitHash: lastCommitHash, dataHash: dataHash, validatorsHash: validatorsHash,
        nextValidatorsHash: nextValidatorsHash, consensusHash: consensusHash, appHash: appHash,
        lastResultsHash: lastResultsHash, evidenceHash: evidenceHash,
        proposerAddress: proposerAddress)
    }
  }
  private struct Signature: Decodable {
    let blockIDFlag: UInt8
    let validatorAddress: Data
    let timestamp: String
    let signature: Data?
    func value() throws -> CometCommitSignature {
      let flag = try XCTUnwrap(CometBlockIDFlag(rawValue: blockIDFlag))
      let bytes: Data
      if flag == .absent {
        XCTAssertTrue(signature == nil || signature?.isEmpty == true)
        bytes = Data()
      } else {
        bytes = try XCTUnwrap(signature)
      }
      return .init(
        blockIDFlag: flag, validatorAddress: validatorAddress,
        timestamp: try KernelVerifierRetiredTests.timestamp(timestamp), signature: bytes)
    }
  }
  private struct Commit: Decodable {
    let height: Int64
    let round: Int32
    let blockID: BlockID
    let signatures: [Signature]
    func value() throws -> CometCommit {
      .init(
        height: height, round: round, blockID: blockID.value,
        signatures: try signatures.map { try $0.value() })
    }
  }
  private struct Validator: Decodable {
    let publicKey: [UInt8]
    let votingPower: Int64
    var value: CometValidator { .init(publicKey: Data(publicKey), votingPower: votingPower) }
  }
  private struct Proof: Decodable {
    let total: Int64
    let index: Int64
    let aunts: [[UInt8]]
    var value: CommittedProof { .init(total: total, index: index, aunts: aunts.map { Data($0) }) }
  }
  private struct Result: Decodable {
    let code: UInt32
    let data: Data
    let gasWanted: Int64
    let gasUsed: Int64
    var value: ExecTxResult {
      .init(code: code, data: data, gasWanted: gasWanted, gasUsed: gasUsed)
    }
  }
  private struct Inclusion: Decodable {
    let transaction: Data
    let transactionProof: Proof
    let result: Result
    let resultProof: Proof
    let batchTransaction: Data
    let batchProof: Proof
    let nextHeader: Header
    let nextCommit: Commit
    let nextValidators: [Validator]
    func value() throws -> ReceiptInclusion {
      .init(
        transaction: transaction, transactionProof: transactionProof.value, result: result.value,
        resultProof: resultProof.value, batchTransaction: batchTransaction,
        batchProof: batchProof.value,
        nextHeader: try nextHeader.value(), nextCommit: try nextCommit.value(),
        nextValidators: nextValidators.map(\.value))
    }
  }
  private static func timestamp(_ text: String) throws -> CometTimestamp {
    let parts = text.dropLast().split(separator: ".", omittingEmptySubsequences: false)
    let formatter = ISO8601DateFormatter()
    let date = try XCTUnwrap(formatter.date(from: String(parts[0]) + "Z"))
    let nanos: Int32
    if parts.count == 2 {
      XCTAssertLessThanOrEqual(parts[1].count, 9)
      nanos = try XCTUnwrap(Int32(parts[1] + String(repeating: "0", count: 9 - parts[1].count)))
    } else {
      XCTAssertEqual(parts.count, 1)
      nanos = 0
    }
    return .init(seconds: Int64(date.timeIntervalSince1970), nanos: nanos)
  }
  private func hex(_ text: String) throws -> Data {
    let chars = Array(text.hasPrefix("0x") ? String(text.dropFirst(2)) : text)
    XCTAssertEqual(chars.count % 2, 0)
    return try Data(
      stride(from: 0, to: chars.count, by: 2).map {
        try XCTUnwrap(UInt8(String(chars[$0...($0 + 1)]), radix: 16))
      })
  }

  private static let kernelReceiptHex =
    "0002520100020000002053d33a514fd1feb9e9c17afd4f4507a5239277784d50e73ea100fe2015880b080000000000000001000000202c5be6f4cfc4d20849574bad4afeb9c921ae6ad7faa4381c5f0928aef883ed5e00000020f02e56a048f7b6bfa2219ade4df64ffd7952db88013ba598c8f7d52d0a5635cc000000200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000204486764c3b2d0850bd0b874654a7801e4c6e659a282f7f1ae2ceec85e01d86500001000000010000000105000000200300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000061a800000020d3cc758efcc208be7b260b4f23b6e6d3d4aeb0c634c21ee9651f5d758a55a5c1000000000000000000000000000f4240000000000000000000000000000ee09800000000000000000000002033c79d4de1950af8f2079dbfa23d26f2ee4fe5e5386529737be0325513d0b2ac00000000000000000000000000000000000000000000000000000000000061a8000000209af46bda8dcf50b22b6242fbbdecc82c4f1e068fe0a8188f8ae7ecaa09d56fa300000020ebd12851ef3ef4771c8a9ebeeaeb9a964745e14482b1cc8f899ddcc55f8e10c8000000203ea6fe1a50ad5807e0443199b096c9fcc90ab4e90e2d1567669b11b33f0c548500000191dd9dec00010000004042ab7c52374cdf33da7dda9bfaaa08f8e35ea0136f32ba2ebe3c7c3c35c5eb56098a980b0905b5487176d2aa47f673bda7fdb1c1dcf6c52da340b80b02c22e01"
}
