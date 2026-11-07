import Crypto
import Foundation
import XCTest

@testable import LayerXSDK

final class ProgramsContractTests: XCTestCase {
  private final class ProgramTransport: PlatformTransport, @unchecked Sendable {
    let response: JSONValue
    init(_ response: JSONValue) { self.response = response }
    func send(_ call: TransportCall) async throws -> JSONValue {
      throw PlatformSDKError(code: .unavailableCapability, retry: .never)
    }
  }

  func testProgramsClientRequiresIndependentNonzeroSequencerPin() throws {
    let client = PlatformClient(transport: ProgramTransport(.emptyObject))
    XCTAssertThrowsError(
      try ProgramsClient(client: client, sequencerPublicKey: Data(repeating: 0, count: 32)))
    XCTAssertNoThrow(
      try ProgramsClient(client: client, sequencerPublicKey: Data(repeating: 1, count: 32)))
  }

  func testPendingReceiptMayOmitRetainedBytesButMustBindExpectedActivity() async throws {
    try await Self.proveRetainedPending1398()
  }

  func testOperationValueVerificationStatusMatrixIsExact() throws {
    try Self.proveCurrentDaemonStatuses1398()
  }

  func testTransferSetV1AndV2ProduceTheSameCanonicalKernelRoot() throws {
    let v1 = transferAuthorization(version: 1)
    let v2 = transferAuthorization(version: 2)
    let rootV1 = try ProgramsWireTestSupport.authorizationRoot(v1)
    let rootV2 = try ProgramsWireTestSupport.authorizationRoot(v2)
    XCTAssertEqual(rootV1, rootV2)
    XCTAssertTrue(rootV1.contains(where: { $0 != 0 }))
    var mutated = v2
    mutated[mutated.count - 33] ^= 1
    XCTAssertNotEqual(try ProgramsWireTestSupport.authorizationRoot(mutated), rootV2)
  }

  func testOccupancyV1V2V3AndAggregateBindings() throws {
    let asset = Data(repeating: 0x66, count: 32)
    for version in 1...3 {
      let binding = try ProgramsWireTestSupport.occupancyBinding(
        emptyOccupancy(version: version), asset: asset)
      XCTAssertEqual(binding.0, UInt128Value(high: 0, low: 0))
      XCTAssertEqual(binding.1, UInt128Value(high: 0, low: 0))
      XCTAssertEqual(binding.2, Data(repeating: 0, count: 32))
    }
    let evidence = chargedOccupancy()
    let binding = try ProgramsWireTestSupport.occupancyBinding(evidence, asset: asset)
    XCTAssertEqual(binding.0, UInt128Value(high: 0, low: 3))
    XCTAssertEqual(binding.1, UInt128Value(high: 0, low: 6))
    XCTAssertTrue(binding.2.contains(where: { $0 != 0 }))
    var mutated = evidence
    let declaredFeeLowByte =
      Data("LXP/storage-occupancy-settlement/v3\0".utf8).count + 8 + 4 + 7 * 8 + 16 + 15
    mutated[declaredFeeLowByte] ^= 1
    XCTAssertThrowsError(try ProgramsWireTestSupport.occupancyBinding(mutated, asset: asset))
  }

  func testNamedGuestABIPolicyAndNativeCodecBounds() throws {
    let fixture = try nativeCallFixture()
    let payload = try nativeTransportHex(fixture["payload_hex"] as? String)
    let native = try NativeProgramCall.decode(payload)
    for abi in [ProgramGuestABI.v1, .v2, .v3, .v4, .v5] {
      XCTAssertEqual(abi.capabilityEncoding, abi == .v1 ? 1 : 2)
      XCTAssertEqual(abi.accountProfile2Supported, abi != .v1)
      let call = nativeCall(native, abi: abi.rawValue)
      XCTAssertEqual(try NativeProgramCall.decode(call.encode()).guestABI, abi.rawValue)
    }
    for abi: UInt16 in [0, 6, UInt16.max] {
      XCTAssertNil(ProgramGuestABI(rawValue: abi))
      XCTAssertThrowsError(try nativeCall(native, abi: abi).encode())
    }
    XCTAssertThrowsError(try nativeCall(native, abi: 4, capacity: 1_048_577).encode())
    var trailing = payload
    trailing.append(0)
    XCTAssertThrowsError(try NativeProgramCall.decode(trailing))
  }

  func testNamedGuestABILifecycleCodecsRetainWasmAndInterfaceBounds() throws {
    let fixture = try fixtureDocument("native-program-deploy-v3.json")
    let payload = try nativeTransportHex(fixture["payload_hex"] as? String)
    _ = try NativeProgramDeploy.decode(payload)
    let length = payload[100..<104].reduce(UInt32(0)) { ($0 << 8) | UInt32($1) }
    let wasm = Data(payload.suffix(Int(length)))
    let program = Data(payload.prefix(32))
    let hash = Data(payload[68..<100])
    let authority = Data(payload[36..<68])
    for abi: UInt16 in [1, 2, 3, 4, 5] {
      let deploy = try NativeProgramDeploy(
        programID: program, guestABI: abi, policy: 1,
        authority: authority, newHash: hash,
        programInterface: abi >= 3 ? lifecycleInterface(abi: abi) : nil, wasm: wasm)
      XCTAssertEqual(try NativeProgramDeploy.decode(deploy.encode()).encode(), deploy.encode())
      let upgrade = try NativeProgramUpgrade(
        programID: program, guestABI: abi,
        oldHash: hash, newHash: hash,
        programInterface: abi >= 3 ? lifecycleInterface(abi: abi) : nil, wasm: wasm)
      XCTAssertEqual(try NativeProgramUpgrade.decode(upgrade.encode()).encode(), upgrade.encode())
    }
    for abi: UInt16 in [0, 6, UInt16.max] {
      XCTAssertThrowsError(
        try NativeProgramDeploy(
          programID: program, guestABI: abi, policy: 1,
          authority: authority, newHash: hash, wasm: wasm))
      XCTAssertThrowsError(
        try NativeProgramUpgrade(
          programID: program, guestABI: abi,
          oldHash: hash, newHash: hash, wasm: wasm))
    }
    XCTAssertThrowsError(
      try NativeProgramDeploy(
        programID: program, guestABI: 4, policy: 1,
        authority: authority, newHash: hash, programInterface: Data(repeating: 1, count: 953),
        wasm: wasm))
    XCTAssertThrowsError(
      try NativeProgramUpgrade(
        programID: program, guestABI: 4,
        oldHash: hash, newHash: Data(repeating: 0, count: 32), wasm: wasm))
  }

  func testSignedDiscoveryRequiresExactCanonicalProofAndIndependentPin() async throws {
    try await Self.proveCurrentRegistry1398()
    let signer = try Curve25519.Signing.PrivateKey(
      rawRepresentation: Data(repeating: 0x42, count: 32))
    let pin = signer.publicKey.rawRepresentation
    let program = Data(repeating: 0x11, count: 32).hexString
    let achieved: JSONValue = .object([
      "state": .string("achieved"), "rung": .string("receipted"),
    ])
    for abi: UInt16 in [1, 2, 3, 4, 5] {
      let object = try discoveryDocument(abi: abi, signer: signer)
      let verified = try XCTUnwrap(
        verifiedDiscovery(
          .object(object), programID: program,
          interface: false, now: 150, pinnedKey: pin
        ).discovery)
      XCTAssertEqual(verified.abiVersion, abi)
      XCTAssertEqual(
        verified.receiptDigest, try nativeTransportHex(object["receipt_digest"]?.stringValue))
      XCTAssertTrue(
        AgentEnvelopeTransport.validVerification(achieved))
      XCTAssertFalse(
        AgentEnvelopeTransport.validVerification(
          .object([
            "state": .string("Achieved"), "level": .string("SequencerSigned"),
          ])))
      XCTAssertThrowsError(
        try verifiedDiscovery(
          .object(object), programID: program,
          interface: false, now: 201, pinnedKey: pin))
      XCTAssertThrowsError(
        try verifiedDiscovery(
          .object(object), programID: program,
          interface: false, now: 150, pinnedKey: Data(repeating: 0x43, count: 32)))
      let mutations: [(String, JSONValue)] = [
        ("code_hash", .string(Data(repeating: 0x21, count: 32).hexString)),
        ("state_root", .string(Data(repeating: 0x51, count: 32).hexString)),
        ("observed_sequence", .string("8")), ("version", .integer(2)),
        ("discovery_signature", .string(Data(repeating: 0, count: 64).hexString)),
        ("unexpected", .integer(1)),
      ]
      for (field, replacement) in mutations {
        var changed = object
        changed[field] = replacement
        XCTAssertThrowsError(
          try verifiedDiscovery(
            .object(changed), programID: program,
            interface: false, now: 150, pinnedKey: pin), field)
      }
      var halfProof = object
      halfProof.removeValue(forKey: "discovery_signature")
      XCTAssertThrowsError(
        try verifiedDiscovery(
          .object(halfProof), programID: program,
          interface: false, now: 150, pinnedKey: pin))
      var unsigned = halfProof
      unsigned.removeValue(forKey: "discovery_public_key")
      if abi == 1 || abi == 2 {
        XCTAssertEqual(
          try verifiedDiscovery(
            .object(unsigned), programID: program,
            interface: false, now: 150, pinnedKey: pin
          ).discovery?.verification,
          "server-side-receipt-verification-only")
      } else {
        XCTAssertThrowsError(
          try verifiedDiscovery(
            .object(unsigned), programID: program,
            interface: false, now: 150, pinnedKey: pin))
      }
    }
  }

  func testHigherABIInterfaceRequiresTheSameSignedDiscoveryHead() throws {
    let signer = try Curve25519.Signing.PrivateKey(
      rawRepresentation: Data(repeating: 0x42, count: 32))
    let pin = signer.publicKey.rawRepresentation
    for abi: UInt16 in [3, 4, 5] {
      let document = try discoveryDocument(abi: abi, signer: signer)
      let program = try XCTUnwrap(document["program_id"]?.stringValue)
      let discovery = try XCTUnwrap(
        verifiedDiscovery(
          .object(document), programID: program,
          interface: false, now: 150, pinnedKey: pin
        ).discovery)
      var object = document
      for field in [
        "lifecycle", "discovery_signature", "discovery_public_key", "deployment_receipt_digest",
      ] {
        object.removeValue(forKey: field)
      }
      let bytes = try canonicalInterface(abi: abi)
      object["interface"] = .string(bytes.hexString)
      object["interface_digest"] = .string(Data(SHA256.hash(data: bytes)).hexString)
      object["receipt_digest"] = document["deployment_receipt_digest"]
      object["source"] = .object(["status": .string("unpublished")])
      object["verification"] = .string("deployment-interface-and-current-head-verified")
      let value = try XCTUnwrap(
        verifiedDiscovery(
          .object(object), programID: program,
          interface: true, now: 150, pinnedKey: pin
        ).interface)
      XCTAssertNoThrow(try bindProgramInterface(value, discovery: discovery))
      XCTAssertEqual(value.verification, "server-side-receipt-verification-only")
      let mutations: [(String, JSONValue)] = [
        ("code_hash", .string(Data(repeating: 0x21, count: 32).hexString)),
        ("state_root", .string(Data(repeating: 0x51, count: 32).hexString)),
        ("receipt_digest", .string(Data(repeating: 0x61, count: 32).hexString)),
        ("observed_sequence", .string("8")), ("version", .integer(2)),
        ("abi_version", .integer(abi == 3 ? 4 : 3)), ("valid_through", .string("201")),
      ]
      for (field, replacement) in mutations {
        var changed = object
        changed[field] = replacement
        let mismatch = try XCTUnwrap(
          verifiedDiscovery(
            .object(changed), programID: program,
            interface: true, now: 150, pinnedKey: pin
          ).interface)
        XCTAssertThrowsError(try bindProgramInterface(mismatch, discovery: discovery), field)
      }
      var oversized = object
      oversized["interface"] = .string(Data(repeating: 1, count: 953).hexString)
      XCTAssertThrowsError(
        try verifiedDiscovery(
          .object(oversized), programID: program,
          interface: true, now: 150, pinnedKey: pin))
      var digestMismatch = object
      digestMismatch["interface_digest"] = .string(Data(repeating: 0, count: 32).hexString)
      XCTAssertThrowsError(
        try verifiedDiscovery(
          .object(digestMismatch), programID: program,
          interface: true, now: 150, pinnedKey: pin))
    }
  }

  private func nativeCallFixture() throws -> [String: Any] {
    try fixtureDocument("native-program-call-v3.json")
  }

  private func fixtureDocument(_ name: String) throws -> [String: Any] {
    var root = URL(fileURLWithPath: #filePath)
    for _ in 0..<6 { root.deleteLastPathComponent() }
    let bytes = try Data(
      contentsOf: root.appendingPathComponent("sdk/platform/conformance/fixtures")
        .appendingPathComponent(name))
    return try XCTUnwrap(JSONSerialization.jsonObject(with: bytes) as? [String: Any])
  }

  private func canonicalInterface(abi: UInt16) throws -> Data {
    let fixture = try fixtureDocument("native-program-deploy-v3.json")
    let payload = try nativeTransportHex(fixture["payload_hex"] as? String)
    _ = try NativeProgramDeploy.decode(payload)
    let interfaceLength = payload[104..<108].reduce(UInt32(0)) { ($0 << 8) | UInt32($1) }
    var encoded = Data(payload[108..<108 + Int(interfaceLength)])
    let domainBytes = Data("LayerX/program-interface/v1\0".utf8).count
    encoded.replaceSubrange(domainBytes..<domainBytes + 32, with: Data(repeating: 0x22, count: 32))
    encoded.replaceSubrange(domainBytes + 32..<domainBytes + 34, with: word(abi.bigEndian))
    return encoded
  }

  private func lifecycleInterface(abi: UInt16) throws -> Data {
    let fixture = try fixtureDocument("native-program-deploy-v3.json")
    let payload = try nativeTransportHex(fixture["payload_hex"] as? String)
    let length = payload[104..<108].reduce(UInt32(0)) { ($0 << 8) | UInt32($1) }
    var bytes = Data(payload[108..<108 + Int(length)])
    let offset = Data("LayerX/program-interface/v1\0".utf8).count + 32
    bytes.replaceSubrange(offset..<offset + 2, with: word(abi.bigEndian))
    return bytes
  }

  private func nativeCall(_ value: NativeProgramCall, abi: UInt16, capacity: UInt32? = nil)
    -> NativeProgramCall
  {
    NativeProgramCall(
      programID: value.programID, guestABI: abi, entrypoint: value.entrypoint,
      calldata: value.calldata, capabilities: value.capabilities,
      accessDeclaration: value.accessDeclaration,
      responseCapacity: capacity ?? value.responseCapacity, resources: value.resources)
  }

  private func discoveryDocument(abi: UInt16, signer: Curve25519.Signing.PrivateKey) throws
    -> [String: JSONValue]
  {
    let program = Data(repeating: 0x11, count: 32)
    let code = Data(repeating: 0x22, count: 32)
    let state = Data(repeating: 0x55, count: 32)
    var proof = Data("LayerX/program-discovery-proof/v1\0".utf8)
    proof.append(program)
    proof.append(1)
    proof.append(be32(1))
    proof.append(code)
    proof.append(word(abi.bigEndian))
    proof.append(be64(7))
    proof.append(be64(100))
    proof.append(be64(200))
    proof.append(state)
    let digest = Data(SHA256.hash(data: proof))
    return [
      "program_id": .string(program.hexString), "lifecycle": .string("active"),
      "version": .integer(1),
      "code_hash": .string(code.hexString), "abi_version": .integer(Int64(abi)),
      "receipt_digest": .string(digest.hexString),
      "deployment_receipt_digest": .string(Data(repeating: 0x66, count: 32).hexString),
      "state_root": .string(state.hexString), "observed_sequence": .string("7"),
      "observed_at": .string("100"), "valid_through": .string("200"),
      "verification": .string("registry-receipt-and-current-head-verified"),
      "discovery_public_key": .string(signer.publicKey.rawRepresentation.hexString),
      "discovery_signature": .string(try signer.signature(for: digest).hexString),
    ]
  }

  private func transferAuthorization(version: Int) -> Data {
    let program = Data(repeating: 1, count: 32)
    let principal = Data(repeating: 2, count: 32)
    let asset = Data(repeating: 4, count: 32)
    let destination = Data(repeating: 5, count: 32)
    var encoded = Data("LayerX/programs/402LXP/transfer-set/v\(version)\0".utf8)
    encoded.append(program)
    encoded.append(principal)
    encoded.append(Data(repeating: 3, count: 32))
    encoded.append(Data(repeating: 0, count: 9))
    var events = Data("LayerX/programs/events/v1\0".utf8)
    events.append(be32(0))
    encoded.append(be32(UInt32(events.count)))
    encoded.append(events)
    encoded.append(be64(0))
    encoded.append(be64(1))
    encoded.append(Data(repeating: 0, count: 9))
    if version == 2 {
      encoded.append(1)
      encoded.append(principal)
    }
    encoded.append(asset)
    encoded.append(destination)
    encoded.append(be128(7))
    encoded.append(program)
    return encoded
  }

  private func emptyOccupancy(version: Int) -> Data {
    var encoded = Data("LXP/storage-occupancy-settlement/v\(version)\0".utf8)
    encoded.append(be64(1))
    if version > 1 { encoded.append(be32(1)) }
    for value in 1...7 { encoded.append(be64(UInt64(value))) }
    if version == 3 {
      encoded.append(Data(repeating: 0, count: 16 * 4))
      encoded.append(be32(0))
    } else {
      encoded.append(Data(repeating: 0, count: 16 * 2))
      encoded.append(be64(0))
    }
    return encoded
  }

  private func chargedOccupancy() -> Data {
    let program = Data(repeating: 0x11, count: 32)
    let payer = Data(repeating: 0x77, count: 32)
    var encoded = Data("LXP/storage-occupancy-settlement/v3\0".utf8)
    encoded.append(be64(2))
    encoded.append(be32(1))
    for value: UInt64 in [0, 0, 0, 0, 0, 0, 2] { encoded.append(be64(value)) }
    encoded.append(be128(3))
    encoded.append(be128(6))
    encoded.append(be128(6))
    encoded.append(be128(0))
    encoded.append(be32(1))
    encoded.append(65)
    encoded.append(program)
    encoded.append(0)
    encoded.append(payer)
    encoded.append(payer)
    encoded.append(program)
    encoded.append(Data(repeating: 0x88, count: 32))
    encoded.append(be64(1))
    encoded.append(be64(2))
    encoded.append(be64(3))
    encoded.append(be64(3))
    encoded.append(be128(3))
    encoded.append(be64(2))
    encoded.append(be128(6))
    encoded.append(be128(0))
    encoded.append(be128(6))
    encoded.append(be128(0))
    encoded.append(1)
    encoded.append(be128(0))
    encoded.append(be64(3))
    encoded.append(be64(2))
    encoded.append(be128(0))
    encoded.append(Data(repeating: 0x99, count: 32))
    return encoded
  }

  private func be32(_ value: UInt32) -> Data { word(value.bigEndian) }
  private func be64(_ value: UInt64) -> Data { word(value.bigEndian) }
  private func be128(_ value: UInt64) -> Data { Data(repeating: 0, count: 8) + be64(value) }
  private func word<T>(_ value: T) -> Data {
    var copy = value
    return withUnsafeBytes(of: &copy) { Data($0) }
  }
}

extension Data {
  fileprivate var hexString: String { map { String(format: "%02x", $0) }.joined() }
}

extension ProgramsContractTests {
  static func proveRetainedPending1398() async throws {
    typealias E = RecordedProgramEvidence1398
    let (vector, _) = try E.recordedCommitted()
    let view = try E.object(vector, "receiptJson")
    let hashText = try E.text(view, "txHash")
    let hash = try E.hex(hashText)
    let key = try IdempotencyKey(String(hashText.dropFirst(2)))
    // rpc/sequencer_receipt.go's submitted branch: the transaction is retained but
    // no sequencer execution receipt exists yet. No old kernel receipt is invented.
    let submitted: JSONValue = .object([
      "rung": .string("submitted"), "txHash": .string(hashText),
      "nonce": try XCTUnwrap(view["nonce"]),
    ])
    let steps: [ProgramReplayServer1398.Exchange] = [
      .init("eth_getTransactionByHash", [.string(hashText)], .object(["hash": .string(hashText)])),
      .init("pax_sequencerReceipt", [.string(hashText)], submitted),
    ]
    let server = try ProgramReplayServer1398(steps)
    defer { server.stop() }
    let pending = try await E.client(server).receipt(
      idempotencyKey: key,
      expectedActivityID: hash, verificationLevel: "pending")
    XCTAssertEqual(pending.state, "pending")
    XCTAssertFalse(pending.isUnknown)
    XCTAssertEqual(pending.transactionHash, hash)
    XCTAssertEqual(pending.idempotencyKey, key.rawValue)
    XCTAssertNil(pending.retainedSignedActivity)
    XCTAssertNil(pending.sequencerReceipt)
    XCTAssertNil(pending.execution)
    server.assertConsumed(2)

    let wrongHash = Data(repeating: 0x12, count: 32)
    let wrongText = E.wireHex(wrongHash)
    let mismatch = try ProgramReplayServer1398([
      .init("eth_getTransactionByHash", [.string(wrongText)], .object(["hash": .string(hashText)]))
    ])
    defer { mismatch.stop() }
    do {
      _ = try await E.client(mismatch).receipt(
        idempotencyKey: key,
        expectedActivityID: wrongHash, verificationLevel: "pending")
      XCTFail("Accepted a retained transaction for another expected activity")
    } catch let error as PlatformSDKError {
      XCTAssertEqual(error.code, .verificationFailure)
    }
    mismatch.assertConsumed(1)
    let unknown = try ProgramReplayServer1398([
      .init("eth_getTransactionByHash", [.string(hashText)], .null)
    ])
    defer { unknown.stop() }
    let absent = try await E.client(unknown).receipt(
      idempotencyKey: key,
      expectedActivityID: hash, verificationLevel: "pending")
    XCTAssertTrue(absent.isUnknown)
    XCTAssertNil(absent.sequencerReceipt)
    unknown.assertConsumed(1)
    for oldLevel in ["sequencer-signed", "SequencerSigned"] {
      do {
        _ = try await E.client(server).receipt(
          idempotencyKey: key,
          expectedActivityID: hash, verificationLevel: oldLevel)
        XCTFail("Accepted retired verification level")
      } catch let error as PlatformSDKError {
        XCTAssertEqual(error.code, .invalidArgument)
      }
    }
    server.assertConsumed(2)
  }

  static func proveCurrentDaemonStatuses1398() throws {
    typealias E = RecordedProgramEvidence1398
    let directory = "services/agent/schema/agent-api/golden/"
    for name in ["agent-http-success", "agent-http-success-pending", "agent-http-success-absent"] {
      let url = E.root.appendingPathComponent(directory + name + ".json")
      let data = try Data(contentsOf: url)
      let envelope = try JSONDecoder().decode(E.Record.self, from: data)
      let requestID = try E.text(envelope, "request_id")
      let result = try AgentEnvelopeTransport.decodeResponse(
        status: 200, data: data, requestID: requestID)
      XCTAssertEqual(result.value, envelope["value"])
      XCTAssertEqual(result.verificationStatus, envelope["verification_status"])
      XCTAssertEqual(result.requestID, requestID)
      let status = try E.object(envelope, "verification_status")
      XCTAssertTrue(AgentEnvelopeTransport.validVerification(.object(status)))
      var invalid: [E.Record] = [
        ["state": .string("Achieved"), "level": .string("SequencerSigned")],
        [
          "state": .string("Unverified"), "requested": .string("SequencerSigned"),
          "achieved": .string("Unverified"), "reason": .string("receipt_pending"),
        ],
        [
          "state": .string("Unverified"), "level": .string("SequencerSigned"),
          "reason": .string("server_side_receipt_verification_only"),
        ],
      ]
      for field in status.keys {
        var missing = status
        missing.removeValue(forKey: field)
        invalid.append(missing)
      }
      var extra = status
      extra["unexpected"] = .null
      invalid.append(extra)
      for changedStatus in invalid {
        var changed = envelope
        changed["verification_status"] = .object(changedStatus)
        XCTAssertFalse(AgentEnvelopeTransport.validVerification(.object(changedStatus)))
        XCTAssertThrowsError(
          try AgentEnvelopeTransport.decodeResponse(
            status: 200, data: E.encode(changed), requestID: requestID)
        ) {
          XCTAssertEqual(($0 as? PlatformSDKError)?.code, .decodeFailure)
        }
      }
    }
    for rung in ["submitted", "receipted", "committed"] {
      XCTAssertTrue(
        AgentEnvelopeTransport.validVerification(
          .object([
            "state": .string("achieved"), "rung": .string(rung),
          ])))
    }
    for (requested, achieved) in [("receipted", "submitted"), ("committed", "receipted")] {
      XCTAssertTrue(
        AgentEnvelopeTransport.validVerification(
          .object([
            "state": .string("unverified"), "requested": .string(requested),
            "achieved": .string(achieved), "reason": .string("verification.pending"),
          ])))
      XCTAssertFalse(
        AgentEnvelopeTransport.validVerification(
          .object([
            "state": .string("unverified"), "requested": .string(achieved),
            "achieved": .string(requested), "reason": .string("verification.pending"),
          ])))
    }
  }

  static func proveCurrentRegistry1398() async throws {
    typealias E = RecordedProgramEvidence1398
    let document = try E.fixture("native-program-deploy-v4")
    let payload = try nativeTransportHex(E.text(document, "payload_hex"))
    let operation = try NativeProgramDeploy.decode(payload)
    let calldata = try NativeProgramLifecycleRequest.calldata(operation: operation)
    let program = Data(payload.prefix(32))
    let authority = Data(payload[36..<68])
    let codeHash = Data(payload[68..<100])
    XCTAssertEqual(calldata, try E.bytes(E.object(document, "evm"), "calldata"))
    let interface = try lifecycleDynamicFields1398(calldata, offsets: [160, 192], headSize: 224)[0]
    // ABI-encoded registry return values use the deployed fixture's real code and interface.
    // The ABI encoder is production code; these are pending reads and claim no receipt proof.
    let registry = try ProgramABI.encode(
      selector: "00000000",
      fields: [
        .word(ProgramABI.word(1)), .word(authority), .word(codeHash),
        .word(ProgramABI.word(2)), .word(ProgramABI.word(1)), .word(ProgramABI.word(1)),
      ]
    ).dropFirst(4)
    let interfaceResult = try ProgramABI.encode(
      selector: "00000000",
      fields: [
        .word(ProgramABI.word(1)), .word(Data(SHA256.hash(data: interface))), .bytes(interface),
      ]
    ).dropFirst(4)
    func params(_ selector: String) throws -> [JSONValue] {
      let data = try ProgramABI.encode(selector: selector, fields: [.word(program)])
      return [
        .object([
          "from": .string("0x" + String(repeating: "55", count: 20)),
          "to": .string(Programs.address), "data": .string(E.wireHex(data)),
        ]),
        .string("pending"),
      ]
    }
    let server = try ProgramReplayServer1398([
      .init("eth_call", try params("b1ffb3d4"), .string(E.wireHex(Data(registry)))),
      .init("eth_call", try params("8efc7240"), .string(E.wireHex(Data(interfaceResult)))),
    ])
    defer { server.stop() }
    let client = try E.client(server)
    let record = try await client.discover(programID: program)
    XCTAssertEqual(record.programID, program)
    XCTAssertEqual(record.codeHash, codeHash)
    XCTAssertEqual(record.authority, authority)
    XCTAssertEqual(record.policy, 1)
    XCTAssertEqual(record.abiVersion, 2)
    XCTAssertEqual(record.version, 1)
    XCTAssertTrue(record.active)
    let publishedInterface = try await client.interface(programID: program)
    XCTAssertEqual(publishedInterface.programID, record.programID)
    XCTAssertEqual(publishedInterface.version, record.version)
    XCTAssertEqual(publishedInterface.encoding, interface)
    XCTAssertEqual(publishedInterface.digest, Data(SHA256.hash(data: interface)))
    for level in ["sequencer-signed", "SequencerSigned", "receipted", "committed"] {
      do {
        _ = try await client.discover(programID: program, verificationLevel: level)
        XCTFail("A pending registry read claimed authenticated discovery")
      } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .invalidArgument) }
      do {
        _ = try await client.interface(programID: program, verificationLevel: level)
        XCTFail("A pending interface read claimed authenticated discovery")
      } catch let error as PlatformSDKError { XCTAssertEqual(error.code, .invalidArgument) }
    }
    server.assertConsumed(2)
  }

  private static func lifecycleDynamicFields1398(_ calldata: Data, offsets: [Int], headSize: Int)
    throws -> [Data]
  {
    let body = Data(calldata.dropFirst(4))
    var position = headSize
    var fields: [Data] = []
    for offset in offsets {
      let declaredPosition = try ProgramABI.integer(body, at: offset)
      guard declaredPosition == UInt64(position) else { throw NativeProgramCallError.invalid }
      let length = try ProgramABI.integer(body, at: position)
      guard length <= 1_048_576 else { throw NativeProgramCallError.invalid }
      let end = position + 32 + ((Int(length) + 31) / 32) * 32
      guard end <= body.count else { throw NativeProgramCallError.invalid }
      // dynamic() intentionally accepts one final tail. Frame that tail exactly;
      // retain its production offset, length and zero-padding checks for every field.
      fields.append(try ProgramABI.dynamic(Data(body.prefix(end)), at: offset, headSize: position))
      position = end
    }
    guard position == body.count else { throw NativeProgramCallError.invalid }
    return fields
  }

  static func proveABIFiveRegistration1398() async throws {
    typealias E = RecordedProgramEvidence1398
    let document = try E.fixture("native-program-deploy-v4")
    let payload = try nativeTransportHex(E.text(document, "payload_hex"))
    let decoded = try NativeProgramDeploy.decode(payload)
    let originalCalldata = try NativeProgramLifecycleRequest.calldata(operation: decoded)
    XCTAssertEqual(originalCalldata, try E.bytes(E.object(document, "evm"), "calldata"))
    let originalFields = try lifecycleDynamicFields1398(
      originalCalldata, offsets: [160, 192], headSize: 224)
    let wasm = originalFields[1]
    var interface = originalFields[0]
    let abiOffset = Data("LayerX/program-interface/v1\0".utf8).count + 32
    interface.replaceSubrange(abiOffset..<abiOffset + 2, with: [0, 5])
    let program = Data(payload.prefix(32))
    let authority = Data(payload[36..<68])
    let hash = Data(SHA256.hash(data: wasm))
    XCTAssertEqual(hash, Data(payload[68..<100]))
    XCTAssertEqual(Data(interface[abiOffset - 32..<abiOffset]), hash)
    XCTAssertEqual(ProgramGuestABI.v5.rawValue, 5)
    XCTAssertEqual(ProgramGuestABI.v5.capabilityEncoding, 2)
    XCTAssertTrue(ProgramGuestABI.v5.accountProfile2Supported)
    let operation = try NativeProgramDeploy(
      programID: program, guestABI: ProgramGuestABI.v5.rawValue,
      policy: 1, authority: authority, newHash: hash, programInterface: interface, wasm: wasm)
    XCTAssertEqual(try NativeProgramDeploy.decode(operation.encode()).encode(), operation.encode())
    let calldata = try NativeProgramLifecycleRequest.calldata(operation: operation)
    let abiBody = Data(calldata.dropFirst(4))
    XCTAssertEqual(calldata.prefix(4), try E.hex("0xbde3ff66"))
    XCTAssertEqual(try ProgramABI.integer(abiBody, at: 32, width: 2), 5)
    let fields = try lifecycleDynamicFields1398(calldata, offsets: [160, 192], headSize: 224)
    XCTAssertEqual(fields[0], interface)
    XCTAssertEqual(fields[1], wasm)
    let sender = Data(repeating: 0x55, count: 20)
    let unsigned = UnsignedProgramTransaction(
      sender: sender, nonce: 7, gas: 0,
      maxFeePerGas: 100, maxPriorityFeePerGas: 2, calldata: calldata)
    let server = try ProgramReplayServer1398([
      .init(
        "eth_getTransactionCount", [.string(E.wireHex(sender)), .string("pending")], .string("0x7")),
      .init(
        "eth_estimateGas", [unsigned.rpcValue(includeGas: false), .string("pending")],
        .string("0x186a0")),
    ])
    defer { server.stop() }
    let transaction = try await E.client(server).deploy(operation)
    XCTAssertEqual(transaction.calldata, calldata)
    XCTAssertEqual(transaction.chainID, 125)
    XCTAssertEqual(transaction.to, Programs.address)
    XCTAssertEqual(transaction.nonce, 7)
    XCTAssertEqual(transaction.gas, 100_000)
    server.assertConsumed(2)
    let upgrade = try NativeProgramUpgrade(
      programID: program, guestABI: 5,
      oldHash: hash, newHash: hash, programInterface: interface, wasm: wasm)
    XCTAssertEqual(try NativeProgramUpgrade.decode(upgrade.encode()).encode(), upgrade.encode())
    let upgradeCalldata = try NativeProgramLifecycleRequest.calldata(
      operation: upgrade,
      feeLimit: ProtocolAmount("1000"))
    XCTAssertEqual(upgradeCalldata.prefix(4), try E.hex("0xa860929f"))
    XCTAssertEqual(try ProgramABI.integer(Data(upgradeCalldata.dropFirst(4)), at: 32, width: 2), 5)
    let upgradeFields = try lifecycleDynamicFields1398(
      upgradeCalldata, offsets: [160, 192, 224], headSize: 288)
    XCTAssertTrue(upgradeFields[0].isEmpty)
    XCTAssertEqual(upgradeFields[1], interface)
    XCTAssertEqual(upgradeFields[2], wasm)
    for abi: UInt16 in [0, 6, UInt16.max] {
      XCTAssertNil(ProgramGuestABI(rawValue: abi))
      XCTAssertThrowsError(
        try NativeProgramDeploy(
          programID: program, guestABI: abi,
          policy: 1, authority: authority, newHash: hash, programInterface: interface, wasm: wasm))
      XCTAssertThrowsError(
        try NativeProgramUpgrade(
          programID: program, guestABI: abi,
          oldHash: hash, newHash: hash, programInterface: interface, wasm: wasm))
    }
    XCTAssertThrowsError(
      try NativeProgramDeploy(
        programID: program, guestABI: 5,
        policy: 1, authority: authority, newHash: hash,
        programInterface: Data(repeating: 1, count: 953), wasm: wasm))
    XCTAssertThrowsError(
      try NativeProgramUpgrade(
        programID: program, guestABI: 5,
        oldHash: hash, newHash: Data(repeating: 0, count: 32), programInterface: interface,
        wasm: wasm))
    let oversizedWasm = wasm + Data(repeating: 0, count: 1_048_577 - wasm.count)
    XCTAssertThrowsError(
      try NativeProgramDeploy(
        programID: program, guestABI: 5,
        policy: 1, authority: authority, newHash: Data(SHA256.hash(data: oversizedWasm)),
        programInterface: interface, wasm: oversizedWasm))
  }
}
