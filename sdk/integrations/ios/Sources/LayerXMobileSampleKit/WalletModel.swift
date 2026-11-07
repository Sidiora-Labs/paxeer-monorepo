import Foundation
import LayerXMobile
import LayerXSDK

private struct HumanVersionResponse: Decodable {
  let service: String
}

private struct HumanActivityPage: Decodable {
  struct Group: Decodable {
    struct Entry: Decodable {
      let entry_id: String
    }

    let entries: [Entry]
  }

  let groups: [Group]
}

public struct WalletSnapshot: Sendable, Equatable {
  public var serviceVersion: String
  public var displayName: String
  public var activityCount: Int
  public var settlement: MobileSettlementState?
  public var deliveries: [String]
  public var refusal: String?

  public init(
    serviceVersion: String = "",
    displayName: String = "",
    activityCount: Int = 0,
    settlement: MobileSettlementState? = nil,
    deliveries: [String] = [],
    refusal: String? = nil
  ) {
    self.serviceVersion = serviceVersion
    self.displayName = displayName
    self.activityCount = activityCount
    self.settlement = settlement
    self.deliveries = deliveries
    self.refusal = refusal
  }
}

public actor WalletModel {
  private let mobile: LayerXMobile
  private let gate: ReceiptGate
  private var snapshot = WalletSnapshot()

  public init(mobile: LayerXMobile, receipts: MobileReceiptResolver) throws {
    self.mobile = mobile
    self.gate = try mobile.gate(receipts: receipts)
  }

  public func current() -> WalletSnapshot { snapshot }

  @discardableResult
  public func refresh() async -> WalletSnapshot {
    do {
      let version = try decode(HumanVersionResponse.self, from: await mobile.client.version())
      let profile = try await mobile.client.profile()
      let activity = try decode(
        HumanActivityPage.self,
        from: await mobile.client.activity(.object(["page_limit": .integer(25)])))
      snapshot.serviceVersion = version.service
      snapshot.displayName = string(profile, "display_name") ?? string(profile, "email") ?? ""
      snapshot.activityCount = activity.groups.reduce(0) { $0 + $1.entries.count }
      snapshot.refusal = nil
    } catch {
      snapshot.refusal = refusalCode(error)
    }
    return snapshot
  }

  @discardableResult
  public func pay(
    quote request: JSONValue,
    expecting expectation: MobileSettlementExpectation,
    idempotencyKey key: IdempotencyKey
  ) async -> WalletSnapshot {
    do {
      let quote = try await mobile.client.quote(request)
      guard let object = quote.objectValue, case let .string(quoteID)? = object["quote_id"] else {
        throw MobileIntegrationError(.decodeFailure)
      }
      let journey = try await mobile.client.commit(
        .object(["quote_id": .string(quoteID)]), idempotencyKey: key)
      snapshot.settlement = try await gate.project(journey: journey, expecting: expectation)
      snapshot.refusal = nil
    } catch {
      snapshot.settlement = nil
      snapshot.refusal = refusalCode(error)
    }
    return snapshot
  }

  @discardableResult
  public func awaitSettlement(
    journeyID: String,
    expecting expectation: MobileSettlementExpectation,
    attempts: Int,
    wait: @Sendable (UInt64) async throws -> Void
  ) async -> WalletSnapshot {
    for attempt in 0..<max(attempts, 1) {
      do {
        let journey = try await mobile.client.journey(id: journeyID)
        let state = try await gate.project(journey: journey, expecting: expectation)
        snapshot.settlement = state
        snapshot.refusal = nil
        if case .pending = state {
          try await wait(UInt64(min(attempt + 1, 10)) * 250_000_000)
          continue
        }
        return snapshot
      } catch {
        snapshot.refusal = refusalCode(error)
        return snapshot
      }
    }
    return snapshot
  }

  @discardableResult
  public func deliver(rawBody: Data, headerFields: [String: String]) async -> WalletSnapshot {
    do {
      let outcome = try await mobile.consume(rawBody: rawBody, headerFields: headerFields) {
        event, deliveryID in
        await self.record(event: event, deliveryID: deliveryID)
      }
      if outcome != .processed {
        snapshot.deliveries.append(outcome.rawValue)
      }
      snapshot.refusal = nil
    } catch {
      snapshot.refusal = refusalCode(error)
    }
    return snapshot
  }

  private func record(event: JSONValue, deliveryID: String) {
    let kind = string(event, "type") ?? string(event, "kind") ?? "event"
    snapshot.deliveries.append("\(deliveryID):\(kind)")
    if snapshot.deliveries.count > 64 {
      snapshot.deliveries.removeFirst(snapshot.deliveries.count - 64)
    }
  }

  private func string(_ value: JSONValue, _ field: String) -> String? {
    guard let object = value.objectValue, case let .string(text)? = object[field] else {
      return nil
    }
    return text
  }

  private func decode<Value: Decodable>(_ type: Value.Type, from value: JSONValue) throws -> Value {
    do {
      return try JSONDecoder().decode(type, from: JSONEncoder().encode(value))
    } catch {
      throw MobileIntegrationError(.decodeFailure)
    }
  }

  private func refusalCode(_ error: Error) -> String {
    if let mobile = error as? MobileIntegrationError { return mobile.code.rawValue }
    if let sdk = error as? PlatformSDKError { return sdk.code.rawValue }
    return "transport-failure"
  }
}
