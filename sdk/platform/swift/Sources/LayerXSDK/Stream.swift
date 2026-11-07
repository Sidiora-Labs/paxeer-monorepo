import Foundation

public struct StreamCursor: RawRepresentable, Hashable, Sendable, Codable, CustomStringConvertible {
  public let rawValue: String

  public init?(rawValue: String) {
    guard !rawValue.isEmpty, rawValue.utf8.count <= 512, !rawValue.contains("\0") else {
      return nil
    }
    self.rawValue = rawValue
  }

  public init(_ value: String) throws {
    guard let cursor = Self(rawValue: value) else {
      throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    self = cursor
  }

  public var description: String { rawValue }
}

public struct StreamEvent<Value: Sendable>: Sendable {
  public let eventID: String
  public let previousCursor: StreamCursor
  public let cursor: StreamCursor
  public let value: Value

  public init(eventID: String, previousCursor: StreamCursor, cursor: StreamCursor, value: Value) {
    self.eventID = eventID
    self.previousCursor = previousCursor
    self.cursor = cursor
    self.value = value
  }
}

public struct StreamPage<Value: Sendable>: Sendable {
  public let requestedCursor: StreamCursor
  public let events: [StreamEvent<Value>]
  public let nextCursor: StreamCursor

  public init(requestedCursor: StreamCursor, events: [StreamEvent<Value>], nextCursor: StreamCursor)
  {
    self.requestedCursor = requestedCursor
    self.events = events
    self.nextCursor = nextCursor
  }
}

public typealias StreamPageSource<Value: Sendable> = @Sendable (StreamCursor) async throws ->
  StreamPage<Value>

public enum StreamLimits {
  public static let pageEvents = 128
  public static let replayEvents = 1024
  public static let bufferedEvents = 64
  public static let emptyPageDelayNanoseconds: UInt64 = 100_000_000
}

/// Drain the already buffered events, then reopen at resumeCursor to recover
/// every event that could not be queued. A page exceeding the limit is not accepted.
public struct StreamOverflow: Error, Sendable {
  public let resumeCursor: StreamCursor
}

public actor ResumableStream<Value: Sendable> {
  public private(set) var cursor: StreamCursor
  private var seenEventIDs: Set<String> = []
  private var replayOrder: [String] = []
  public var retainedReplayCount: Int { seenEventIDs.count }

  public init(cursor: StreamCursor) {
    self.cursor = cursor
  }

  public func accept(_ page: StreamPage<Value>) throws -> [StreamEvent<Value>] {
    guard page.requestedCursor == cursor else {
      throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }
    guard page.events.count <= StreamLimits.pageEvents else {
      throw StreamOverflow(resumeCursor: cursor)
    }
    var expected = cursor
    var pageEventIDs: Set<String> = []
    var accepted: [StreamEvent<Value>] = []
    accepted.reserveCapacity(page.events.count)
    for event in page.events {
      guard !event.eventID.isEmpty, event.eventID.utf8.count <= 512,
        event.previousCursor == expected,
        event.cursor != event.previousCursor,
        !seenEventIDs.contains(event.eventID),
        pageEventIDs.insert(event.eventID).inserted
      else {
        throw PlatformSDKError(code: .decodeFailure, retry: .never)
      }
      accepted.append(event)
      expected = event.cursor
    }
    guard page.nextCursor == expected else {
      throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }

    seenEventIDs.formUnion(pageEventIDs)
    replayOrder.append(contentsOf: accepted.map(\.eventID))
    if replayOrder.count > StreamLimits.replayEvents {
      let expired = replayOrder.count - StreamLimits.replayEvents
      for id in replayOrder.prefix(expired) { seenEventIDs.remove(id) }
      replayOrder.removeFirst(expired)
    }
    cursor = page.nextCursor
    return accepted
  }

  public func next(from source: StreamPageSource<Value>) async throws -> [StreamEvent<Value>] {
    let requested = cursor
    let page = try await source(requested)
    return try accept(page)
  }

  public nonisolated func events(from source: @escaping StreamPageSource<Value>)
    -> AsyncThrowingStream<StreamEvent<Value>, Error>
  {
    AsyncThrowingStream(bufferingPolicy: .bufferingOldest(StreamLimits.bufferedEvents)) {
      continuation in
      let task = Task {
        var resumeCursor = await self.cursor
        do {
          while !Task.isCancelled {
            let events = try await self.next(from: source)
            if events.isEmpty {
              try await Task.sleep(nanoseconds: StreamLimits.emptyPageDelayNanoseconds)
            }
            for event in events {
              switch continuation.yield(event) {
              case .enqueued: resumeCursor = event.cursor
              case .dropped: throw StreamOverflow(resumeCursor: resumeCursor)
              case .terminated: return
              @unknown default: throw StreamOverflow(resumeCursor: resumeCursor)
              }
            }
          }
          continuation.finish()
        } catch is CancellationError {
          continuation.finish()
        } catch {
          continuation.finish(throwing: error)
        }
      }
      continuation.onTermination = { _ in task.cancel() }
    }
  }
}

public struct HumanStreamSource: Sendable {
  private let client: PlatformClient

  public init(client: PlatformClient) {
    self.client = client
  }

  public func open() async throws -> StreamCursor {
    let response = try await client.humanStreamOpen()
    return try StreamCursor(try stringField("cursor", in: response))
  }

  public func next(after requested: StreamCursor) async throws -> StreamPage<JSONValue> {
    let response = try await client.humanStreamNext(pathParameters: ["cursor": requested.rawValue])
    guard let object = response.objectValue,
      case let .array(untrustedEvents)? = object["events"],
      case let .string(nextValue)? = object["next_cursor"]
    else { throw decodeFailure() }
    guard untrustedEvents.count <= StreamLimits.pageEvents else {
      throw StreamOverflow(resumeCursor: requested)
    }
    var previous = requested
    var events: [StreamEvent<JSONValue>] = []
    events.reserveCapacity(untrustedEvents.count)
    for value in untrustedEvents {
      let cursor = try StreamCursor(try stringField("cursor", in: value))
      events.append(
        StreamEvent(
          eventID: cursor.rawValue, previousCursor: previous, cursor: cursor, value: value))
      previous = cursor
    }
    return StreamPage(
      requestedCursor: requested, events: events, nextCursor: try StreamCursor(nextValue))
  }

  public func source() -> StreamPageSource<JSONValue> {
    { cursor in try await next(after: cursor) }
  }

  private func stringField(_ name: String, in value: JSONValue) throws -> String {
    guard let object = value.objectValue, case let .string(field)? = object[name] else {
      throw decodeFailure()
    }
    return field
  }

  private func decodeFailure() -> PlatformSDKError {
    PlatformSDKError(code: .decodeFailure, retry: .never)
  }
}
