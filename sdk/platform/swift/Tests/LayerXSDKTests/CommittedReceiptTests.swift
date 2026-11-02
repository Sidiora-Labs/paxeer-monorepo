import Crypto
import Foundation
import XCTest

@testable import LayerXSDK

final class CommittedReceiptTests: XCTestCase {
  private struct Vectors: Decodable { let cases: [Vector] }
  private struct Vector: Decodable {
    let id: String
    let rawTransaction: String
    let signedReceipt: String
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
  private struct Fixture {
    var receipt: SequencerReceiptVerification
    var header: CommittedHeader
    var inclusion: ReceiptInclusion
    var commit: CometCommit
    var validators: [CometValidator]
    var keys: [Curve25519.Signing.PrivateKey]
    func verify() throws -> Committed {
      try Committed.verify(
        receipt, blockHeader: header, receiptInclusion: inclusion,
        commit: commit, validatorSet: validators)
    }
  }
  private func hex(_ string: String) throws -> Data {
    let chars = Array(string.hasPrefix("0x") ? String(string.dropFirst(2)) : string)
    XCTAssertEqual(chars.count % 2, 0)
    return try Data(
      stride(from: 0, to: chars.count, by: 2).map {
        try XCTUnwrap(UInt8(String(chars[$0...($0 + 1)]), radix: 16))
      })
  }
  private func sha(_ data: Data) -> Data { Data(SHA256.hash(data: data)) }
  private func be(_ n: UInt64) -> Data {
    Data((0..<8).reversed().map { UInt8(truncatingIfNeeded: n >> ($0 * 8)) })
  }
  private func tree(_ leaves: [Data], index: Int) -> (Data, CommittedProof) {
    func split(_ count: Int) -> Int {
      var k = 1
      while k * 2 < count { k *= 2 }
      return k
    }
    func root(_ items: [Data]) -> Data {
      if items.count == 1 { return sha(Data([0]) + items[0]) }
      let k = split(items.count)
      return sha(Data([1]) + root(Array(items[..<k])) + root(Array(items[k...])))
    }
    func aunts(_ items: [Data], _ position: Int) -> [Data] {
      if items.count == 1 { return [] }
      let k = split(items.count)
      let left = Array(items[..<k])
      let right = Array(items[k...])
      return position < k
        ? aunts(left, position) + [root(right)]
        : aunts(right, position - k) + [root(left)]
    }
    return (
      root(leaves),
      CommittedProof(
        total: Int64(leaves.count), index: Int64(index),
        aunts: aunts(leaves, index))
    )
  }
  private func signedCommit(
    _ header: CommittedHeader,
    keys: [Curve25519.Signing.PrivateKey], signers: Int
  ) throws -> CometCommit {
    var commit = CometCommit(
      height: header.height, round: 7,
      blockID: CometBlockID(
        hash: header.hash, partSetTotal: 2,
        partSetHash: Data(repeating: 0x22, count: 32)), signatures: [])
    for (n, key) in keys.enumerated() {
      if n >= signers {
        commit.signatures.append(.init(blockIDFlag: .absent))
        continue
      }
      var sig = CometCommitSignature(
        blockIDFlag: .block,
        validatorAddress: Data(sha(key.publicKey.rawRepresentation).prefix(20)),
        timestamp: .init(seconds: 1_700_000_123 + Int64(n), nanos: 987_654_321 - Int32(n)))
      sig.signature = try key.signature(for: CommittedWire.voteBytes(header.chainID, commit, sig))
      commit.signatures.append(sig)
    }
    return commit
  }
  private func fixture(
    _ id: String = "V1-pax-transfer", powers: [Int64] = [1, 1, 1, 1],
    offset: Int64 = 0
  ) throws -> Fixture {
    let url = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("conformance/fixtures/sequencer-receipt-v1.json")
    let vectors = try JSONDecoder().decode(Vectors.self, from: Data(contentsOf: url))
    let v = try XCTUnwrap(vectors.cases.first { $0.id == id })
    let raw = try hex(v.rawTransaction)
    let receipt = try LocalVerifier.verifySequencerReceipt(
      hex(v.signedReceipt), rawTransaction: raw,
      keys: [
        ReceiptKeys(
          keyId: try hex(v.pinnedKey.keyId), publicKey: try hex(v.pinnedKey.publicKey),
          firstHeight: v.pinnedKey.firstHeight, lastHeight: v.pinnedKey.lastHeight)
      ],
      receiptJSON: v.receiptJson, prevSeqHash: hex(v.prevSeqHash))
    XCTAssertEqual(powers.count, 4)
    let keys = try (1...4).map {
      try Curve25519.Signing.PrivateKey(rawRepresentation: Data(repeating: UInt8($0), count: 32))
    }
    let validators = zip(keys, powers).map {
      CometValidator(publicKey: $0.0.publicKey.rawRepresentation, votingPower: $0.1)
    }
    let r = receipt.receipt
    let height = Int64(CommittedWire.bigEndian(r.batchID.suffix(8))) + offset
    let domain = Data("PAXEER/v1/sequence-batch\0".utf8)
    var batch =
      domain + be(UInt64(height)) + be(r.globalSequence) + be(r.globalSequence)
      + r.previousStateRoot + r.resultingStateRoot + sha(r.activityID) + r.contextHash
    let batchKey = try Curve25519.Signing.PrivateKey(
      rawRepresentation: Data(repeating: 0x5e, count: 32))
    let batchDigest = sha(
      domain + Data("committed-sdk-test".utf8)
        + Data(batch.dropFirst(domain.count).prefix(24 + 96)))
    batch += try batchKey.signature(for: batchDigest)
    let gas = Int64(CommittedWire.bigEndian(r.effects[0].body.prefix(8)))
    let result = ExecTxResult(
      code: r.resultCode == 0 ? 0 : 45,
      data: Data([1, 2, 3]), gasWanted: gas + 1000, gasUsed: gas)
    let message =
      CommittedWire.bytes(1, Data("/ethermint.evm.v1.MsgEthereumTx".utf8))
      + CommittedWire.bytes(2, CommittedWire.bytes(1, raw))
    let transaction = CommittedWire.bytes(1, CommittedWire.bytes(1, message))
    let transactions = [batch, transaction, Data("another block transaction".utf8)].map(sha)
    let results = [
      ExecTxResult().encoded, result.encoded,
      ExecTxResult(gasWanted: 60000, gasUsed: 46220).encoded,
    ]
    let (dataHash, txProof) = tree(transactions, index: 1)
    let (resultsHash, resultProof) = tree(results, index: 1)
    let setHash = CommittedWire.validatorsHash(validators)
    let header = CommittedHeader(
      versionApp: 2, chainID: "committed-sdk-test", height: height,
      time: .init(seconds: 1_700_000_123, nanos: 987_654_321), dataHash: dataHash,
      validatorsHash: setHash, nextValidatorsHash: setHash)
    let commit = try signedCommit(header, keys: keys, signers: 3)
    var next = header
    next.height += 1
    next.time.seconds += 1
    next.lastBlockID = commit.blockID
    next.lastResultsHash = resultsHash
    return Fixture(
      receipt: receipt, header: header,
      inclusion: ReceiptInclusion(
        transaction: transaction, transactionProof: txProof,
        result: result, resultProof: resultProof, batchTransaction: batch,
        batchProof: tree(transactions, index: 0).1, nextHeader: next,
        nextCommit: try signedCommit(next, keys: keys, signers: 3), nextValidators: validators),
      commit: commit, validators: validators, keys: keys)
  }
  private func reanchorResult(_ f: inout Fixture) throws {
    let results = [
      ExecTxResult().encoded, f.inclusion.result.encoded,
      ExecTxResult(gasWanted: 60000, gasUsed: 46220).encoded,
    ]
    let (hash, proof) = tree(results, index: 1)
    f.inclusion.resultProof = proof
    f.inclusion.nextHeader.lastResultsHash = hash
    f.inclusion.nextCommit = try signedCommit(f.inclusion.nextHeader, keys: f.keys, signers: 3)
  }
  private func refusal(
    _ f: Fixture, _ expected: CommittedError, file: StaticString = #filePath,
    line: UInt = #line
  ) {
    XCTAssertThrowsError(try f.verify(), file: file, line: line) {
      XCTAssertEqual($0 as? CommittedError, expected, file: file, line: line)
    }
  }

  func testCommittedRungAcceptsTwoThirdsCommit() throws {
    for id in ["V1-pax-transfer", "V2-layerxledger-revert"] {
      for offset: Int64 in [0, 1] {
        let f = try fixture(id, offset: offset)
        let result = try f.verify()
        XCTAssertEqual(result.level, "committed")
        XCTAssertEqual(result.rung, "committed")
        XCTAssertEqual(result.height, UInt64(f.header.height))
        XCTAssertEqual(result.blockHash, f.header.hash)
        XCTAssertEqual(result.votingPower, 3)
        XCTAssertEqual(result.status, id.hasPrefix("V1") ? 1 : 0)
        XCTAssertEqual(result.receiptDigest, f.receipt.receiptDigest)
        XCTAssertEqual(result.receiptVerification.canonicalBytes, f.receipt.canonicalBytes)
        XCTAssertEqual(f.receipt.level, "receipted")
      }
    }
    let raw = try hex(
      "02f8727d80843b9aca0084b2d05e00825208948ba1f109551bd432803012645ac136ddd64dba72880de0b6b3a764000080c080a0c10d1d3ded8a2ac5bf5652235628abf4963e5618e1f0b374794de957bc2401cfa079c223a5baf08e5d5b17ab51b244e2d1d528df9d692adc690653eaa3ddfb0981"
    )
    let fields =
      CommittedWire.bytes(1, Data("125".utf8))
      + CommittedWire.bytes(3, Data("1000000000".utf8))
      + CommittedWire.bytes(4, Data("3000000000".utf8))
      + CommittedWire.number(5, 21000)
      + CommittedWire.bytes(6, Data("0x8ba1f109551bd432803012645ac136ddd64dba72".utf8))
      + CommittedWire.bytes(7, Data("1000000000000000000".utf8))
      + CommittedWire.bytes(
        11, try hex("c10d1d3ded8a2ac5bf5652235628abf4963e5618e1f0b374794de957bc2401cf"))
      + CommittedWire.bytes(
        12, try hex("79c223a5baf08e5d5b17ab51b244e2d1d528df9d692adc690653eaa3ddfb0981"))
    let any =
      CommittedWire.bytes(1, Data("/paxprotocol.paxchain.eth.DynamicFeeTx".utf8))
      + CommittedWire.bytes(2, fields)
    let message =
      CommittedWire.bytes(1, Data("/paxprotocol.paxchain.evm.MsgEVMTransaction".utf8))
      + CommittedWire.bytes(2, CommittedWire.bytes(1, any))
    let wrapped = CommittedWire.bytes(1, CommittedWire.bytes(1, message))
    XCTAssertEqual(try CommittedWire.evmTransaction(wrapped), raw)
    let weighted = try fixture(powers: [40, 30, 20, 10])
    XCTAssertEqual(try weighted.verify().votingPower, 90)
    let setHash = CommittedWire.validatorsHash(weighted.validators)
    XCTAssertEqual(
      setHash, try hex("0321b92eefbfb6129dcbfb8c9066c149bb1091c312f388e2142ef4bb4ba86fb0"))
    let header = CommittedHeader(
      versionApp: 2, chainID: "paxeer-committed-π", height: 1_234_567_890_123,
      time: .init(seconds: 1_700_000_123, nanos: 987_654_321),
      lastBlockID: .init(
        hash: Data(repeating: 0x11, count: 32), partSetTotal: 257,
        partSetHash: Data(repeating: 0x22, count: 32)),
      lastCommitHash: Data(repeating: 0x33, count: 32),
      dataHash: Data(repeating: 0x44, count: 32), validatorsHash: setHash,
      nextValidatorsHash: Data(repeating: 0x55, count: 32),
      consensusHash: Data(repeating: 0x66, count: 32),
      appHash: Data([0, 1, 2, 255, 128]), lastResultsHash: Data(repeating: 0x77, count: 32),
      evidenceHash: Data(repeating: 0x88, count: 32),
      proposerAddress: try hex("34750f98bd59fcfc946da45aaabe933be154a4b5"))
    XCTAssertEqual(
      header.hash, try hex("6ea46ec595bac2dc17d1a567a4bff602c059522020a3b5afafa4e8607ea4db99"))
    let commit = CometCommit(
      height: header.height, round: 7,
      blockID: .init(
        hash: Data(repeating: 0x11, count: 32), partSetTotal: 2,
        partSetHash: Data(repeating: 0x22, count: 32)), signatures: [])
    let sig = CometCommitSignature(blockIDFlag: .block, timestamp: header.time)
    XCTAssertEqual(
      try weighted.keys[0].signature(
        for: CommittedWire.voteBytes("paxeer-commit-reference", commit, sig)),
      try hex(
        "5b0029c887146fb4be46a5fabcacaf811b9159c73d50a2f326e85edf91c57c4839af327a00ef0afb74114f08cd1645efaff7216d2c244dfa026b9eab90540d0e"
      ))
    XCTAssertEqual(
      ExecTxResult(code: 45, data: Data([1, 2]), gasWanted: 300, gasUsed: 210).encoded,
      try hex("082d1202010228ac0230d201"))
    let leaves = [Data("foo".utf8), Data("bar".utf8), Data("baz".utf8)].map(sha)
    for index in 0..<3 {
      let (root, proof) = tree(leaves, index: index)
      XCTAssertTrue(CommittedWire.verifyProof(leaves[index], proof, root))
      XCTAssertFalse(CommittedWire.verifyProof(Data("foo".utf8), proof, root))
    }
  }

  func testCommittedRungRefusesInsufficientPower() throws {
    for powers: [Int64] in [[1, 1, 1, 1], [2, 2, 2, 3], [1, 1, 1, 97]] {
      var f = try fixture(powers: powers)
      f.commit = try signedCommit(f.header, keys: f.keys, signers: powers == [1, 1, 1, 1] ? 2 : 3)
      refusal(f, .commit)
    }
    var f = try fixture()
    f.commit.signatures[0].signature[0] ^= 1
    refusal(f, .commit)
    f = try fixture()
    f.validators[1] = f.validators[0]
    refusal(f, .commit)
    f = try fixture()
    f.commit.round += 1
    refusal(f, .commit)
    f = try fixture()
    f.inclusion.nextCommit = try signedCommit(f.inclusion.nextHeader, keys: f.keys, signers: 2)
    refusal(f, .nextCommit)
    f = try fixture()
    f.commit.signatures[2].blockIDFlag = .nil
    f.commit.signatures[2].signature = try f.keys[2].signature(
      for: CommittedWire.voteBytes(f.header.chainID, f.commit, f.commit.signatures[2]))
    refusal(f, .commit)
  }

  func testCommittedRungRefusesTamperedInclusion() throws {
    var f = try fixture()
    f.inclusion.resultProof.aunts[0][0] ^= 1
    refusal(f, .inclusion)
    f = try fixture()
    f.inclusion.transactionProof.aunts[0][0] ^= 1
    refusal(f, .inclusion)
    f = try fixture()
    f.inclusion.batchProof.aunts[0][0] ^= 1
    refusal(f, .inclusion)
    f = try fixture()
    f.inclusion.result.data[0] ^= 1
    refusal(f, .inclusion)
    f = try fixture()
    f.inclusion.result.gasUsed += 1
    refusal(f, .inclusion)
    f = try fixture()
    f.inclusion.transactionProof.index = 2
    refusal(f, .inclusion)
    f = try fixture()
    f.inclusion.resultProof.aunts.append(Data(repeating: 0, count: 32))
    refusal(f, .inclusion)
    f = try fixture()
    f.inclusion.nextHeader.lastResultsHash[0] ^= 1
    refusal(f, .nextCommit)
    f = try fixture()
    f.inclusion.nextHeader.lastBlockID.hash[0] ^= 1
    refusal(f, .nextHeader)
    f = try fixture(offset: -1)
    refusal(f, .height)
    for code: UInt32 in [1, 45] {
      f = try fixture()
      f.inclusion.result.code = code
      try reanchorResult(&f)
      refusal(f, .result)
    }
    f = try fixture("V2-layerxledger-revert")
    f.inclusion.result.code = 0
    try reanchorResult(&f)
    refusal(f, .result)
    f = try fixture()
    f.inclusion.result.gasUsed -= 1
    try reanchorResult(&f)
    refusal(f, .result)
  }
}
