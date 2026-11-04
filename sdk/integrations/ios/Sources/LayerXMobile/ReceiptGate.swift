import Crypto
import Foundation
import LayerXSDK

public struct MobileReceiptEvidence: Sendable {
  public let rawTransaction: Data
  public let signedReceipt: Data
  public let receiptKey: ReceiptKeys
  public let receiptJSON: [String: JSONValue]
  public let prevSeqHash: Data

  public init(
    rawTransaction: Data,
    signedReceipt: Data,
    receiptKey: ReceiptKeys,
    receiptJSON: [String: JSONValue],
    prevSeqHash: Data
  ) {
    self.rawTransaction = rawTransaction
    self.signedReceipt = signedReceipt
    self.receiptKey = receiptKey
    self.receiptJSON = receiptJSON
    self.prevSeqHash = prevSeqHash
  }
}

public struct MobileSettlementExpectation: Sendable, Equatable {
  public let asset: Data
  public let recipient: Data
  public let amount: String

  public init(asset: Data, recipient: Data, amount: String) throws {
    guard asset.count == 32, recipient.count == 32,
      !amount.isEmpty, amount.utf8.allSatisfy({ $0 >= 48 && $0 <= 57 }),
      amount == "0" || amount.first != "0"
    else {
      throw MobileIntegrationError(.invalidConfiguration)
    }
    self.asset = asset
    self.recipient = recipient
    self.amount = amount
  }
}

public enum MobileSettlementState: Sendable, Equatable {
  case pending(reference: String)
  case verified(level: String, receiptDigest: String)
  case refused(code: String)
}

public protocol MobileReceiptResolver: Sendable {
  func resolve(receiptReference: String) async throws -> MobileReceiptEvidence
}

public struct ReceiptGate: Sendable {
  public typealias Evidence = MobileReceiptEvidence

  private let receipts: MobileReceiptResolver?
  private let client: LayerXMobileClient?
  private let receiptKeys: [ReceiptKeys]

  public init(receipts: MobileReceiptResolver, receiptKeys: [ReceiptKeys] = []) {
    self.receipts = receipts
    self.client = nil
    self.receiptKeys = receiptKeys
  }

  public init(client: LayerXMobileClient, receiptKeys: [ReceiptKeys]) {
    self.receipts = nil
    self.client = client
    self.receiptKeys = receiptKeys
  }

  public func settle(
    receiptReference: String,
    expecting expectation: MobileSettlementExpectation
  ) async throws -> MobileSettlementState {
    guard !receiptReference.isEmpty, receiptReference.utf8.count <= 512,
      !receiptReference.contains("\0")
    else {
      throw MobileIntegrationError(.decodeFailure)
    }
    guard let receipts else { throw MobileIntegrationError(.invalidConfiguration) }
    let evidence = try await receipts.resolve(receiptReference: receiptReference)
    return try await verify(evidence: evidence, expecting: expectation)
  }

  public func verify(
    evidence: MobileReceiptEvidence,
    expecting expectation: MobileSettlementExpectation
  ) async throws -> MobileSettlementState {
    let pinned = receiptKeys.filter {
      $0.keyId == evidence.receiptKey.keyId && $0.publicKey == evidence.receiptKey.publicKey
        && $0.firstHeight == evidence.receiptKey.firstHeight
        && $0.lastHeight == evidence.receiptKey.lastHeight
    }
    guard !pinned.isEmpty else {
      throw MobileIntegrationError(.verificationFailure)
    }
    let verification: SequencerReceiptVerification
    do {
      verification = try LocalVerifier.verifySequencerReceipt(
        evidence.signedReceipt,
        rawTransaction: evidence.rawTransaction,
        keys: pinned,
        receiptJSON: evidence.receiptJSON,
        prevSeqHash: evidence.prevSeqHash
      )
    } catch {
      throw MobileIntegrationError(.verificationFailure)
    }
    guard verification.status == 1,
      verification.receipt.asset == expectation.asset,
      verification.receipt.to == expectation.recipient,
      decimalString(verification.receipt.amount) == expectation.amount
    else {
      throw MobileIntegrationError(.verificationFailure)
    }
    return .verified(
      level: verification.level, receiptDigest: hexadecimal(verification.receiptDigest))
  }

  public func project(journey: JSONValue, expecting expectation: MobileSettlementExpectation)
    async throws -> MobileSettlementState
  {
    guard let object = journey.objectValue, case let .string(state)? = object["state"] else {
      throw MobileIntegrationError(.decodeFailure)
    }
    guard case let .string(journeyID)? = object["journey_id"], !journeyID.isEmpty else {
      throw MobileIntegrationError(.decodeFailure)
    }
    switch state {
    case "done", "done-finalised":
      guard case let .array(evidence)? = object["evidence"],
        case let .array(stages)? = object["stages"]
      else {
        throw MobileIntegrationError(.decodeFailure)
      }
      var references = evidence
      for stage in stages {
        guard let stage = stage.objectValue, case let .array(entries)? = stage["evidence"] else {
          throw MobileIntegrationError(.decodeFailure)
        }
        references.append(contentsOf: entries)
      }
      var seen = Set<String>()
      for value in references {
        guard let reference = value.objectValue,
          case let .string(identifier)? = reference["evidence_id"], !identifier.isEmpty,
          case let .string(kind)? = reference["class"],
          case let .string(verification)? = reference["verification"]
        else {
          throw MobileIntegrationError(.decodeFailure)
        }
        guard kind == "layerx-receipt",
          verification == "receipt-verified" || verification == "checkpoint-finalised",
          seen.insert(identifier).inserted
        else { continue }
        if let client {
          let material = try await client.evidence(id: identifier)
          let payment = try HumanPaymentReceipt.verify(
            material, identifier: identifier, verification: verification, keys: receiptKeys)
          if payment.asset == expectation.asset, payment.recipient == expectation.recipient,
            decimalString(payment.amount) == expectation.amount
          {
            return .verified(level: "receipted", receiptDigest: hexadecimal(payment.digest))
          }
        } else {
          guard let receipts else { throw MobileIntegrationError(.invalidConfiguration) }
          let material = try await receipts.resolve(receiptReference: identifier)
          guard identifier == "evd_" + hexadecimal(Data(SHA256.hash(data: material.signedReceipt)))
          else {
            throw MobileIntegrationError(.verificationFailure)
          }
          return try await verify(evidence: material, expecting: expectation)
        }
      }
      throw MobileIntegrationError(.verificationFailure)
    case "refused":
      return .refused(code: state)
    case "getting-ready", "sending", "processing", "still-checking", "waiting-for-you":
      return .pending(reference: journeyID)
    default:
      throw MobileIntegrationError(.decodeFailure)
    }
  }
}

public func decimalString(_ value: UInt128Value) -> String {
  var high = value.high
  var low = value.low
  if high == 0 && low == 0 { return "0" }
  var digits: [UInt8] = []
  while high != 0 || low != 0 {
    let quotientHigh = high / 10
    let remainderHigh = high % 10
    let (quotientLow, remainder) = UInt64(10).dividingFullWidth((high: remainderHigh, low: low))
    digits.append(UInt8(48 + remainder))
    high = quotientHigh
    low = quotientLow
  }
  return String(decoding: digits.reversed(), as: UTF8.self)
}

public func hexadecimal(_ bytes: Data) -> String {
  bytes.map { String(format: "%02x", $0) }.joined()
}
