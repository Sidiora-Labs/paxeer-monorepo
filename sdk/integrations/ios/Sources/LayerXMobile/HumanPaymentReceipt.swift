import Crypto
import Foundation
import LayerXSDK

// The Human plane exports PortableReceiptV2 (inclusionHeight, optional rawTransaction),
// rather than the SDK RPC export that requires receiptJson.
struct HumanPaymentReceipt {
  let asset: Data
  let recipient: Data
  let amount: UInt128Value
  let digest: Data

  static func verify(
    _ material: JSONValue, identifier: String, verification: String, keys: [ReceiptKeys]
  ) throws -> HumanPaymentReceipt {
    do {
      return try decodeAndVerify(
        material, identifier: identifier, verification: verification, keys: keys)
    } catch {
      throw MobileIntegrationError(.verificationFailure)
    }
  }

  private static func decodeAndVerify(
    _ material: JSONValue, identifier: String, verification: String, keys: [ReceiptKeys]
  ) throws -> HumanPaymentReceipt {
    guard let fields = material.objectValue,
      fields["evidence_id"] == .string(identifier), fields["class"] == .string("layerx-receipt"),
      fields["verification"] == .string(verification),
      fields["content_type"] == .string("application/vnd.layerx.receipt-proof-v2+json"),
      let encoded = fields["bytes_base64"]?.stringValue, encoded.utf8.count <= 2_000_000,
      let bytes = Data(base64Encoded: encoded), bytes.base64EncodedString() == encoded,
      !bytes.isEmpty, bytes.count <= 1_500_000,
      let proof = try JSONDecoder().decode(JSONValue.self, from: bytes).objectValue,
      proof["format"] == .string("layerx-receipt-proof-v2"),
      proof["verificationLevel"] == .string("receipted")
    else { throw MobileIntegrationError(.verificationFailure) }
    let required: Set<String> = [
      "format", "verificationLevel", "signedReceipt", "receiptDigest", "receiptKey",
      "inclusionHeight",
    ]
    guard Set(proof.keys) == required || Set(proof.keys) == required.union(["rawTransaction"]),
      let key = proof["receiptKey"]?.objectValue,
      Set(key.keys) == ["keyId", "publicKey", "firstHeight", "lastHeight"]
    else { throw MobileIntegrationError(.verificationFailure) }
    let keyID = try hex(key["keyId"], count: 32)
    let publicKey = try hex(key["publicKey"], count: 32)
    let first = try quantity(key["firstHeight"])
    let last = try quantity(key["lastHeight"])
    let signed = try hex(proof["signedReceipt"])
    guard signed.count <= 1_048_576,
      identifier == "evd_" + hexadecimal(Data(SHA256.hash(data: signed))),
      keys.contains(where: {
        $0.keyId == keyID && $0.publicKey == publicKey && $0.firstHeight == first
          && $0.lastHeight == last
      })
    else { throw MobileIntegrationError(.verificationFailure) }
    var wire = ReceiptReader(signed)
    let version = try wire.integer(2)
    guard version == 2 || version == 3, try wire.integer(2) == 0x5201,
      try wire.integer(2) == version
    else { throw MobileIntegrationError(.verificationFailure) }
    let activity = try wire.blob(32)
    let sequence = try wire.integer(8)
    let previous = try wire.blob(32)
    let resulting = try wire.blob(32)
    _ = try wire.blob(32)  // activity root
    let result = try wire.integer(4)
    let count = try wire.integer(4)
    guard count <= 512 else { throw MobileIntegrationError(.verificationFailure) }
    for _ in 0..<count {
      _ = try wire.integer(2)  // module
      _ = try wire.integer(2)  // ordinal
      _ = try wire.integer(2)  // event type
      let kind = try wire.integer(1)
      let monetary = try wire.integer(1)
      guard (1...3).contains(kind), monetary <= 1, monetary == 0 || kind == 2 else {
        throw MobileIntegrationError(.verificationFailure)
      }
      _ = try wire.blob(32)
      _ = try wire.blob(maximum: 256)
    }
    _ = try wire.fixed(16)  // fee
    let batch = try wire.blob(32)
    let height = batch.suffix(8).reduce(UInt64(0)) { ($0 << 8) | UInt64($1) }
    let module = try wire.integer(2)
    let moduleVersion = try wire.integer(4)
    _ = try wire.integer(4)  // parameter version
    _ = try wire.integer(1)  // operation
    let asset = try wire.blob(32)
    let amount = try UInt128Value(high: wire.integer(8), low: wire.integer(8))
    _ = try wire.blob(32)  // sender
    _ = try wire.fixed(32)  // sender balances
    _ = try wire.integer(8)  // sender sequence
    let recipient = try wire.blob(32)
    _ = try wire.fixed(32)  // recipient balances
    _ = try wire.blob(32)  // transfer root
    _ = try wire.blob(32)  // authorization
    let signingKeyID = try wire.blob(32)
    let timestamp = try wire.integer(8)
    let signatureOffset = wire.position
    guard try wire.integer(1) == 1 else { throw MobileIntegrationError(.verificationFailure) }
    let signature = try wire.blob(64)
    guard wire.position == signed.count, result == 0, sequence > 0, timestamp > 0,
      module > 11, moduleVersion > 0, activity.contains(where: { $0 != 0 }),
      batch.prefix(24).allSatisfy({ $0 == 0 }), first <= height, height <= last,
      try quantity(proof["inclusionHeight"]) >= height, signingKeyID == keyID
    else { throw MobileIntegrationError(.verificationFailure) }
    var preimage = Data("LXP/v1/receipt\0".utf8)
    preimage.append(signed.prefix(signatureOffset))
    preimage.append(0)
    let digest = Data(SHA256.hash(data: preimage))
    guard try hex(proof["receiptDigest"], count: 32) == digest,
      try Curve25519.Signing.PublicKey(rawRepresentation: publicKey).isValidSignature(
        signature, for: digest)
    else { throw MobileIntegrationError(.verificationFailure) }
    var sequenceInput = Data("PAXEER/v1/sequence\0".utf8)
    sequenceInput.append(previous)
    sequenceInput.append(
      contentsOf: (0..<8).map { UInt8(truncatingIfNeeded: sequence >> (56 - $0 * 8)) })
    sequenceInput.append(activity)
    guard resulting == Data(SHA256.hash(data: sequenceInput)) else {
      throw MobileIntegrationError(.verificationFailure)
    }
    if let raw = proof["rawTransaction"] {
      guard paymentKeccak256(try hex(raw)) == activity else {
        throw MobileIntegrationError(.verificationFailure)
      }
    }
    if let rung = fields["rung"] {
      guard rung == .string(verification == "checkpoint-finalised" ? "committed" : "receipted")
      else {
        throw MobileIntegrationError(.verificationFailure)
      }
    }
    if let inclusion = fields["inclusion_height"] {
      guard case let .integer(number) = inclusion, number >= 0,
        UInt64(number) == (try quantity(proof["inclusionHeight"]))
      else {
        throw MobileIntegrationError(.verificationFailure)
      }
    }
    return HumanPaymentReceipt(asset: asset, recipient: recipient, amount: amount, digest: digest)
  }

  private static func hex(_ value: JSONValue?, count: Int? = nil) throws -> Data {
    guard let text = value?.stringValue, text.hasPrefix("0x"), text.utf8.count <= 2_097_154 else {
      throw MobileIntegrationError(.verificationFailure)
    }
    let digits = Array(text.dropFirst(2).utf8)
    guard !digits.isEmpty, digits.count % 2 == 0,
      digits.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) })
    else {
      throw MobileIntegrationError(.verificationFailure)
    }
    let result = Data(
      stride(from: 0, to: digits.count, by: 2).map { index -> UInt8 in
        let high = digits[index] < 58 ? digits[index] - 48 : digits[index] - 87
        let low = digits[index + 1] < 58 ? digits[index + 1] - 48 : digits[index + 1] - 87
        return high * 16 + low
      })
    guard count == nil || result.count == count else {
      throw MobileIntegrationError(.verificationFailure)
    }
    return result
  }

  private static func quantity(_ value: JSONValue?) throws -> UInt64 {
    guard let text = value?.stringValue, text.hasPrefix("0x"),
      text == "0x0" || (text.count > 2 && text.dropFirst(2).first != "0"),
      text.dropFirst(2).utf8.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) }),
      let number = UInt64(text.dropFirst(2), radix: 16)
    else {
      throw MobileIntegrationError(.verificationFailure)
    }
    return number
  }
}

private struct ReceiptReader {
  let data: Data
  var position = 0
  init(_ data: Data) { self.data = data }

  mutating func fixed(_ count: Int) throws -> Data {
    guard count >= 0, count <= data.count - position else {
      throw MobileIntegrationError(.verificationFailure)
    }
    defer { position += count }
    return data.subdata(in: position..<(position + count))
  }

  mutating func integer(_ count: Int) throws -> UInt64 {
    try fixed(count).reduce(UInt64(0)) { ($0 << 8) | UInt64($1) }
  }

  mutating func blob(_ count: Int) throws -> Data {
    guard try integer(4) == UInt64(count) else {
      throw MobileIntegrationError(.verificationFailure)
    }
    return try fixed(count)
  }

  mutating func blob(maximum: Int) throws -> Data {
    let count = try integer(4)
    guard count <= UInt64(maximum) else { throw MobileIntegrationError(.verificationFailure) }
    return try fixed(Int(count))
  }
}

private func paymentKeccak256(_ data: Data) -> Data {
  let rc: [UInt64] = [
    0x1, 0x8082, 0x8000_0000_0000_808a, 0x8000_0000_8000_8000, 0x808b, 0x8000_0001,
    0x8000_0000_8000_8081, 0x8000_0000_0000_8009, 0x8a, 0x88, 0x8000_8009, 0x8000_000a, 0x8000_808b,
    0x8000_0000_0000_008b, 0x8000_0000_0000_8089, 0x8000_0000_0000_8003, 0x8000_0000_0000_8002,
    0x8000_0000_0000_0080, 0x800a, 0x8000_0000_8000_000a, 0x8000_0000_8000_8081,
    0x8000_0000_0000_8080,
    0x8000_0001, 0x8000_0000_8000_8008,
  ]
  let shifts = [
    0, 1, 62, 28, 27, 36, 44, 6, 55, 20, 3, 10, 43, 25, 39, 41, 45, 15, 21, 8, 18, 2, 61, 56, 14,
  ]
  func rotate(_ x: UInt64, _ n: Int) -> UInt64 { n == 0 ? x : (x << n) | (x >> (64 - n)) }
  var state = [UInt64](repeating: 0, count: 25)
  var bytes = [UInt8](data)
  let padding = 136 - bytes.count % 136
  bytes.append(contentsOf: repeatElement(0, count: padding))
  bytes[data.count] = 1
  bytes[bytes.count - 1] |= 128
  for block in stride(from: 0, to: bytes.count, by: 136) {
    for i in 0..<136 { state[i / 8] ^= UInt64(bytes[block + i]) << ((i % 8) * 8) }
    for round in rc {
      var c = [UInt64](repeating: 0, count: 5)
      for x in 0..<5 { for y in 0..<5 { c[x] ^= state[x + 5 * y] } }
      for x in 0..<5 {
        let d = c[(x + 4) % 5] ^ rotate(c[(x + 1) % 5], 1)
        for y in 0..<5 { state[x + 5 * y] ^= d }
      }
      var b = [UInt64](repeating: 0, count: 25)
      for x in 0..<5 {
        for y in 0..<5 {
          b[y + 5 * ((2 * x + 3 * y) % 5)] = rotate(state[x + 5 * y], shifts[x + 5 * y])
        }
      }
      for x in 0..<5 {
        for y in 0..<5 {
          state[x + 5 * y] = b[x + 5 * y] ^ ((~b[(x + 1) % 5 + 5 * y]) & b[(x + 2) % 5 + 5 * y])
        }
      }
      state[0] ^= round
    }
  }
  return Data((0..<32).map { UInt8(truncatingIfNeeded: state[$0 / 8] >> (($0 % 8) * 8)) })
}
