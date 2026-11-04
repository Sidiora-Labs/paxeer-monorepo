#nullable enable

using System.Runtime.CompilerServices;
using System.Text;

namespace LayerX.Sdk;

public readonly record struct StreamCursor
{
    public string Value { get; }

    [System.Text.Json.Serialization.JsonConstructor]
    public StreamCursor(string value)
    {
        if (string.IsNullOrEmpty(value) || Encoding.UTF8.GetByteCount(value) > 512 || value.Contains('\0'))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        Value = value;
    }

    internal bool IsValid => !string.IsNullOrEmpty(Value);
    public override string ToString() => Value ?? string.Empty;
}

public sealed record StreamEvent<T>(string EventId, StreamCursor PreviousCursor, StreamCursor Cursor, T Value);
public sealed record StreamPage<T>(StreamCursor RequestedCursor, IReadOnlyList<StreamEvent<T>> Events, StreamCursor NextCursor);

public sealed record StreamRecoveryState(StreamCursor Cursor, IReadOnlyList<string> EventIds);

/// <summary>Rejects replayed event IDs in the last 4096 acknowledged events. IDs are at most 512 UTF-8 bytes.
/// Persist RecoveryState to preserve this entire replay window when reopening; a cursor alone starts a new window.</summary>
public sealed class ResumableStream<T>
{
    public const int ReplayWindowCapacity = 4096;
    private readonly object _gate = new();
    private readonly Queue<string> _replayWindow = new();
    private readonly HashSet<string> _seenEventIds = new(StringComparer.Ordinal);
    private StreamCursor _cursor;

    public ResumableStream(StreamCursor cursor)
    {
        if (!cursor.IsValid) throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        _cursor = cursor;
    }

    public static ResumableStream<T> Restore(StreamRecoveryState state)
    {
        ArgumentNullException.ThrowIfNull(state);
        if (state.EventIds is null || state.EventIds.Count > ReplayWindowCapacity)
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        var stream = new ResumableStream<T>(state.Cursor);
        foreach (var id in state.EventIds)
        {
            if (!ValidEventId(id) || !stream._seenEventIds.Add(id))
                throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
            stream._replayWindow.Enqueue(id);
        }
        return stream;
    }

    public StreamCursor Cursor { get { lock (_gate) return _cursor; } }
    public StreamRecoveryState RecoveryState
    {
        get { lock (_gate) return new(_cursor, Array.AsReadOnly(_replayWindow.ToArray())); }
    }

    private static bool ValidEventId(string id) => !string.IsNullOrEmpty(id) && Encoding.UTF8.GetByteCount(id) <= 512;

    // Accept acknowledges a complete batch returned to a batch consumer.
    public IReadOnlyList<StreamEvent<T>> Accept(StreamPage<T> page)
    {
        lock (_gate)
        {
            var accepted = ValidatePage(page);
            foreach (var item in accepted) Acknowledge(item);
            return accepted;
        }
    }

    private IReadOnlyList<StreamEvent<T>> ValidatePage(StreamPage<T> page)
    {
        ArgumentNullException.ThrowIfNull(page);
        lock (_gate)
        {
            if (page.RequestedCursor != _cursor || page.Events is null || !page.NextCursor.IsValid)
                throw DecodeFailure();
            var expected = _cursor;
            var pageEventIds = new HashSet<string>(StringComparer.Ordinal);
            var accepted = new List<StreamEvent<T>>(page.Events.Count);
            foreach (var item in page.Events)
            {
                if (item is null || !ValidEventId(item.EventId) || !item.Cursor.IsValid || !item.PreviousCursor.IsValid ||
                    item.PreviousCursor != expected || item.Cursor == item.PreviousCursor ||
                    _seenEventIds.Contains(item.EventId) || !pageEventIds.Add(item.EventId))
                    throw DecodeFailure();
                accepted.Add(item);
                expected = item.Cursor;
            }
            if (page.NextCursor != expected) throw DecodeFailure();
            return accepted.AsReadOnly();
        }
    }

    private void Acknowledge(StreamEvent<T> item)
    {
        lock (_gate)
        {
            if (item.PreviousCursor != _cursor || !_seenEventIds.Add(item.EventId)) throw DecodeFailure();
            _replayWindow.Enqueue(item.EventId);
            if (_replayWindow.Count > ReplayWindowCapacity) _seenEventIds.Remove(_replayWindow.Dequeue());
            _cursor = item.Cursor;
        }
    }

    public async Task<IReadOnlyList<StreamEvent<T>>> NextAsync(
        Func<StreamCursor, CancellationToken, Task<StreamPage<T>>> source,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(source);
        var requested = Cursor;
        var page = await source(requested, cancellationToken).ConfigureAwait(false);
        return Accept(page);
    }

    public async IAsyncEnumerable<StreamEvent<T>> EventsAsync(
        Func<StreamCursor, CancellationToken, Task<StreamPage<T>>> source,
        [EnumeratorCancellation] CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(source);
        while (!cancellationToken.IsCancellationRequested)
        {
            var page = await source(Cursor, cancellationToken).ConfigureAwait(false);
            var accepted = ValidatePage(page);
            foreach (var item in accepted)
            {
                cancellationToken.ThrowIfCancellationRequested();
                Acknowledge(item);
                yield return item;
            }
        }
    }

    private static PlatformSdkException DecodeFailure() =>
        new(SdkErrorCode.DecodeFailure, RetryClass.Never);
}

public sealed class HumanStreamSource
{
    private readonly PlatformClient _client;

    public HumanStreamSource(PlatformClient client) =>
        _client = client ?? throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);

    public async Task<StreamCursor> OpenAsync(CancellationToken cancellationToken = default)
    {
        var response = await _client.HumanStreamOpenAsync(cancellationToken: cancellationToken).ConfigureAwait(false);
        return new StreamCursor(StringField(response, "cursor"));
    }

    public async Task<StreamPage<JsonValue>> NextAsync(StreamCursor requested, CancellationToken cancellationToken = default)
    {
        var response = await _client.HumanStreamNextAsync(
            pathParameters: new Dictionary<string, string> { ["cursor"] = requested.Value },
            cancellationToken: cancellationToken).ConfigureAwait(false);
        if (response is not JsonValue.ObjectValue map ||
            !map.Value.TryGetValue("events", out var untrustedEvents) || untrustedEvents is not JsonValue.ArrayValue eventArray ||
            !map.Value.TryGetValue("next_cursor", out var untrustedNext) || untrustedNext is not JsonValue.StringValue nextValue)
            throw DecodeFailure();
        var previous = requested;
        var events = new List<StreamEvent<JsonValue>>(eventArray.Value.Count);
        foreach (var value in eventArray.Value)
        {
            var cursor = new StreamCursor(StringField(value, "cursor"));
            events.Add(new(cursor.Value, previous, cursor, value));
            previous = cursor;
        }
        return new(requested, events.AsReadOnly(), new StreamCursor(nextValue.Value));
    }

    public Func<StreamCursor, CancellationToken, Task<StreamPage<JsonValue>>> Source() => NextAsync;

    private static string StringField(JsonValue value, string name) =>
        value is JsonValue.ObjectValue map && map.Value.TryGetValue(name, out var field) && field is JsonValue.StringValue text
            ? text.Value
            : throw DecodeFailure();

    private static PlatformSdkException DecodeFailure() => new(SdkErrorCode.DecodeFailure, RetryClass.Never);
}
