#nullable enable

using System.Net;
using System.Net.Http.Headers;
using System.Net.Security;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace LayerX.Sdk;

public sealed class AccessToken : IDisposable
{
    private static readonly UTF8Encoding StrictUtf8 = new(false, true);
    private readonly SecretBytes _secret;

    public AccessToken(ReadOnlySpan<byte> bytes)
    {
        try { _ = StrictUtf8.GetCharCount(bytes); }
        catch (DecoderFallbackException) { throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never); }
        _secret = new SecretBytes(bytes);
    }

    internal void Authorize(HttpRequestMessage request) => _secret.Use(bytes =>
    {
        var value = StrictUtf8.GetString(bytes.Span);
        if (value.Contains('\r') || value.Contains('\n'))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", value);
        return true;
    });

    public void Dispose() => _secret.Dispose();
    public override string ToString() => "[REDACTED]";
}

public sealed class LayerXKeyCredential : IDisposable
{
    private static readonly UTF8Encoding StrictUtf8 = new(false, true);
    private readonly string _keyId;
    private readonly SecretBytes _secret;

    public LayerXKeyCredential(string keyId, ReadOnlySpan<byte> secret)
    {
        if (string.IsNullOrEmpty(keyId) || keyId.Length > 64 || keyId.Any(character =>
            !(character is >= 'a' and <= 'z' or >= 'A' and <= 'Z' or >= '0' and <= '9' or '-' or '_')))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        _keyId = keyId;
        _secret = new SecretBytes(secret);
    }

    internal void Authorize(HttpRequestMessage request) => _secret.Use(bytes =>
    {
        string value;
        try { value = StrictUtf8.GetString(bytes.Span); }
        catch (DecoderFallbackException) { throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never); }
        if (!value.StartsWith("lxp_live_", StringComparison.Ordinal) || value.Length != 73 ||
            value.Skip(9).Any(character => !(character is >= '0' and <= '9' or >= 'a' and <= 'f')))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        request.Headers.TryAddWithoutValidation("Authorization", $"LayerX-Key {_keyId}:{value}");
        return true;
    });

    public void Dispose() => _secret.Dispose();
    public override string ToString() => "[REDACTED]";
}

public sealed class AgentHttpTransport : IPlatformTransport
{
    public AgentHttpTransport(Uri baseUri, HttpClient? httpClient = null, LayerXKeyCredential? credential = null, AccessToken? accessToken = null)
    {
        if (credential is not null && accessToken is not null) throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        if (!baseUri.IsAbsoluteUri || !string.IsNullOrEmpty(baseUri.UserInfo) || string.IsNullOrEmpty(baseUri.Host) ||
            !string.IsNullOrEmpty(baseUri.Query) || !string.IsNullOrEmpty(baseUri.Fragment) ||
            (baseUri.Scheme != Uri.UriSchemeHttps && (baseUri.Scheme != Uri.UriSchemeHttp || !IsLoopback(baseUri.Host))))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        if (httpClient is not null)
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
    }

    public Task<JsonValue> SendAsync(TransportCall call, CancellationToken cancellationToken = default) =>
        Task.FromException<JsonValue>(new PlatformSdkException(SdkErrorCode.UnavailableCapability, RetryClass.Never));

    private static bool ValidProgramVerification(string operation, JsonValue value, JsonValue verification)
    {
        var status = verification is JsonValue.ObjectValue objectValue ? objectValue.Value : null;
        if (status is null) return false;
        if (operation is "program.discover" or "program.interface")
            return Exact(status, "state", "requested", "achieved", "reason") && Text(status, "state") == "Unverified" &&
                Text(status, "requested") == "SequencerSigned" && Text(status, "achieved") == "Unverified" &&
                Text(status, "reason") == "server_side_receipt_verification_only";
        var pending = (operation is "program.call" or "program.receipt" or "program.activity") &&
            value is JsonValue.ObjectValue pendingValue && TryText(pendingValue.Value, "state") is "unknown" or "pending";
        if (pending)
            return Exact(status, "state", "requested", "achieved", "reason") && Text(status, "state") == "Unverified" &&
                Text(status, "requested") == "SequencerSigned" && Text(status, "achieved") == "Unverified" &&
                Text(status, "reason") == "receipt_pending";
        return (operation is "program.simulate" or "program.call" or "program.receipt" or "program.activity") &&
            Exact(status, "state", "level") && Text(status, "state") == "Achieved" &&
            Text(status, "level") == "SequencerSigned";
    }

    private static bool Exact(IReadOnlyDictionary<string, JsonValue> value, params string[] fields) =>
        value.Count == fields.Length && fields.All(value.ContainsKey);
    private static string Text(IReadOnlyDictionary<string, JsonValue> value, string field) =>
        TryText(value, field) ?? throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never);
    private static string? TryText(IReadOnlyDictionary<string, JsonValue> value, string field) =>
        value.TryGetValue(field, out var raw) && raw is JsonValue.StringValue text ? text.Value : null;
    private static bool IsLoopback(string host) => string.Equals(host, "localhost", StringComparison.OrdinalIgnoreCase) ||
        IPAddress.TryParse(host, out var address) && IPAddress.IsLoopback(address);
}

public sealed class HumanHttpTransport : IPlatformTransport
{
    private const int MaximumResponseBytes = 8 * 1024 * 1024;
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);
    private readonly Uri _baseUri;
    private readonly HttpClient _httpClient;
    private readonly AccessToken? _accessToken;

    public HumanHttpTransport(Uri baseUri, HttpClient? httpClient = null, AccessToken? accessToken = null)
    {
        if (!baseUri.IsAbsoluteUri || !string.IsNullOrEmpty(baseUri.UserInfo) || string.IsNullOrEmpty(baseUri.Host) ||
            (baseUri.Scheme != Uri.UriSchemeHttps && (baseUri.Scheme != Uri.UriSchemeHttp || !IsLoopback(baseUri.Host))))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        _baseUri = baseUri;
        _httpClient = httpClient ?? new HttpClient();
        _accessToken = accessToken;
    }

    public async Task<JsonValue> SendAsync(TransportCall call, CancellationToken cancellationToken = default)
    {
        var descriptor = call.Operation.Descriptor();
        if (descriptor.Plane != PlatformPlane.Human)
            throw new PlatformSdkException(SdkErrorCode.UnavailableCapability, RetryClass.Never);
        var path = ResolvePath(descriptor.Path, call.PathParameters);
        var target = new Uri(_baseUri, path);
        if (target.Scheme != _baseUri.Scheme || target.Host != _baseUri.Host || target.Port != _baseUri.Port)
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);

        using var request = new HttpRequestMessage(ToHttpMethod(descriptor.Method), target);
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        request.Headers.UserAgent.ParseAdd("layerx-dotnet/0.1.0");
        if (!descriptor.Bodyless)
        {
            var encoded = JsonSerializer.SerializeToUtf8Bytes(call.Request, JsonOptions);
            request.Content = new ByteArrayContent(encoded);
            request.Content.Headers.ContentType = new MediaTypeHeaderValue("application/json");
        }
        if (call.IdempotencyKey is { } key)
            request.Headers.TryAddWithoutValidation("Idempotency-Key", key.Value);
        _accessToken?.Authorize(request);

        using var response = await _httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken).ConfigureAwait(false);
        var encodedResponse = await ReadBoundedAsync(response.Content, cancellationToken).ConfigureAwait(false);
        HumanEnvelope? envelope;
        try { envelope = JsonSerializer.Deserialize<HumanEnvelope>(encodedResponse, JsonOptions); }
        catch (JsonException) { throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never); }
        if (envelope is null || string.IsNullOrEmpty(envelope.Trace) || Encoding.UTF8.GetByteCount(envelope.Trace) > 512 ||
            envelope.Trace.Contains('\0') || envelope.Trace.Contains('\r') || envelope.Trace.Contains('\n'))
            throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never);
        if (envelope.Ok)
        {
            if (!response.IsSuccessStatusCode || envelope.Error is not null || envelope.Result is null)
                throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never);
            return envelope.Result;
        }
        if (response.IsSuccessStatusCode || envelope.Error is null || envelope.Result is not null)
            throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never);
        throw envelope.Error.ToSdkException(envelope.Trace);
    }

    private static async Task<byte[]> ReadBoundedAsync(HttpContent content, CancellationToken cancellationToken)
    {
        await using var stream = await content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        using var output = new MemoryStream();
        var buffer = new byte[16 * 1024];
        while (true)
        {
            var count = await stream.ReadAsync(buffer.AsMemory(), cancellationToken).ConfigureAwait(false);
            if (count == 0) return output.ToArray();
            if (output.Length + count > MaximumResponseBytes)
                throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never);
            output.Write(buffer, 0, count);
        }
    }

    private static string ResolvePath(string template, IReadOnlyDictionary<string, string> parameters)
    {
        var path = template;
        foreach (var (name, value) in parameters)
        {
            var token = "{" + name + "}";
            if (string.IsNullOrEmpty(name) || string.IsNullOrEmpty(value) || !path.Contains(token, StringComparison.Ordinal))
                throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
            path = path.Replace(token, Uri.EscapeDataString(value), StringComparison.Ordinal);
        }
        if (!path.StartsWith("/", StringComparison.Ordinal) || path.Contains('{') || path.Contains('}'))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        return path;
    }

    private static HttpMethod ToHttpMethod(SdkHttpMethod method) => method switch
    {
        SdkHttpMethod.Get => HttpMethod.Get,
        SdkHttpMethod.Post => HttpMethod.Post,
        SdkHttpMethod.Put => HttpMethod.Put,
        SdkHttpMethod.Patch => HttpMethod.Patch,
        SdkHttpMethod.Delete => HttpMethod.Delete,
        _ => throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never),
    };

    private static bool IsLoopback(string host)
    {
        if (string.Equals(host, "localhost", StringComparison.OrdinalIgnoreCase)) return true;
        return IPAddress.TryParse(host, out var address) && IPAddress.IsLoopback(address);
    }

    private sealed record HumanEnvelope(
        [property: JsonPropertyName("ok")] bool Ok,
        [property: JsonPropertyName("result")] JsonValue? Result,
        [property: JsonPropertyName("error")] HumanApiError? Error,
        [property: JsonPropertyName("trace")] string Trace);

    private sealed record HumanApiError(
        [property: JsonPropertyName("code")] string Code,
        [property: JsonPropertyName("retry")] string Retry,
        [property: JsonPropertyName("retry_after_ms")] ulong? RetryAfterMilliseconds)
    {
        public PlatformSdkException ToSdkException(string trace)
        {
            var code = Code switch
            {
                "rate-limited" => SdkErrorCode.RateLimit,
                "unavailable" or "upstream-degraded" => SdkErrorCode.TransportFailure,
                "refused-by-policy" => SdkErrorCode.PolicyRefusal,
                "refused-by-budget" or "refused-by-limit" => SdkErrorCode.BudgetRefusal,
                "refused-by-capability" or "forbidden" or "unauthenticated" or "session-expired" or "step-up-required" => SdkErrorCode.CapabilityRefusal,
                "conflict" => SdkErrorCode.IdempotencyConflict,
                _ => SdkErrorCode.CoreRejection,
            };
            var retry = Retry switch
            {
                "retriable" => RetryClass.Safe,
                "retriable-after" when RetryAfterMilliseconds is not null => RetryClass.After,
                "structural" or "final" => RetryClass.Never,
                _ => throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never),
            };
            return new PlatformSdkException(code, retry, trace, retryAfterMilliseconds: RetryAfterMilliseconds);
        }
    }
}

public sealed class AgentSessionCredential : IDisposable
{
    private static readonly UTF8Encoding StrictUtf8 = new(false, true);
    private readonly SecretBytes _tokenId;

    public string Tenant { get; }
    public string SessionId { get; }
    public ulong Generation { get; }

    public AgentSessionCredential(string tenant, ReadOnlySpan<byte> sessionId, ReadOnlySpan<byte> tokenId, ulong generation)
    {
        int tenantBytes;
        try { tenantBytes = tenant is null ? 0 : StrictUtf8.GetByteCount(tenant); }
        catch (EncoderFallbackException) { throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never); }
        if (tenantBytes is 0 or > 255 || tenant!.Contains('\0') || sessionId.Length != 32 || tokenId.Length != 32)
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        Tenant = tenant;
        SessionId = Convert.ToHexString(sessionId).ToLowerInvariant();
        Generation = generation;
        _tokenId = new SecretBytes(tokenId);
    }

    internal void Write(Utf8JsonWriter writer) => _tokenId.Use(token =>
    {
        writer.WriteStartObject();
        writer.WriteString("tenant", Tenant);
        writer.WriteString("session_id", SessionId);
        writer.WriteString("token_id", Convert.ToHexString(token.Span).ToLowerInvariant());
        writer.WriteString("generation", Generation.ToString(System.Globalization.CultureInfo.InvariantCulture));
        writer.WriteEndObject();
        return true;
    });

    public void Dispose() => _tokenId.Dispose();
    public override string ToString() => "[REDACTED]";
}

public sealed record AgentEnvelopeResult(string RequestId, JsonValue Value, JsonValue VerificationStatus);

public sealed class AgentEnvelopeTransport : IPlatformTransport, IDisposable
{
    private const int EnvelopeVersion = 2;
    public const string RoutePath = "/v1/agent/rpc";
    public const string DaemonRoutePath = "/rpc";
    public const int MaximumRequestBytes = 1_048_576;
    private const int MaximumResponseBytes = 8 * 1024 * 1024;
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);
    private static readonly HashSet<string> BootstrapOperations = new(StringComparer.Ordinal) { "agent.register", "session.open" };
    private static readonly string[] Levels =
    [
        "Unverified", "SequencerSigned", "BatchIncluded", "StateProven", "CheckpointFinalised", "SettlementAnchored",
    ];
    private readonly Uri _endpoint;
    private readonly HttpClient _httpClient;
    private readonly AgentSessionCredential? _credential;
    private readonly LayerXKeyCredential? _gatewayKey;

    public AgentEnvelopeTransport(Uri baseUri, AgentSessionCredential? credential, LayerXKeyCredential? gatewayKey = null,
        X509Certificate2? trustedRoot = null)
    {
        if (!baseUri.IsAbsoluteUri || !string.IsNullOrEmpty(baseUri.UserInfo) || string.IsNullOrEmpty(baseUri.Host) ||
            !string.IsNullOrEmpty(baseUri.Query) || !string.IsNullOrEmpty(baseUri.Fragment) ||
            baseUri.AbsolutePath != "/" ||
            (baseUri.Scheme != Uri.UriSchemeHttps && (baseUri.Scheme != Uri.UriSchemeHttp || !IsLoopback(baseUri.Host))) ||
            (trustedRoot is not null && baseUri.Scheme != Uri.UriSchemeHttps))
            throw new PlatformSdkException(SdkErrorCode.InvalidArgument, RetryClass.Never);
        _endpoint = new UriBuilder(baseUri) { Path = RoutePath, Query = "", Fragment = "" }.Uri;
        _credential = credential;
        _gatewayKey = gatewayKey;
        var handler = new SocketsHttpHandler { AllowAutoRedirect = false, UseCookies = false, UseProxy = false };
        if (trustedRoot is not null)
        {
            var root = new X509Certificate2(trustedRoot);
            handler.SslOptions = new SslClientAuthenticationOptions
            {
                RemoteCertificateValidationCallback = (_, certificate, presented, errors) =>
                {
                    if (certificate is null || (errors & ~SslPolicyErrors.RemoteCertificateChainErrors) != SslPolicyErrors.None)
                        return false;
                    using var chain = new X509Chain();
                    chain.ChainPolicy.TrustMode = X509ChainTrustMode.CustomRootTrust;
                    chain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
                    chain.ChainPolicy.CustomTrustStore.Add(root);
                    if (presented is not null)
                        foreach (var element in presented.ChainElements) chain.ChainPolicy.ExtraStore.Add(element.Certificate);
                    using var leaf = new X509Certificate2(certificate);
                    return chain.Build(leaf);
                },
            };
        }
        _httpClient = new HttpClient(handler, true);
    }

    private AgentEnvelopeTransport(Uri endpoint, AgentSessionCredential? credential, HttpClientHandler handler)
    {
        _endpoint = endpoint;
        _credential = credential;
        _gatewayKey = null;
        _httpClient = new HttpClient(handler, false);
    }

    public static AgentEnvelopeTransport ForDaemon(Uri baseUri, AgentSessionCredential? credential, HttpClientHandler handler)
    {
        ArgumentNullException.ThrowIfNull(baseUri);
        ArgumentNullException.ThrowIfNull(handler);
        if (!baseUri.IsAbsoluteUri || baseUri.Scheme != Uri.UriSchemeHttps || !string.IsNullOrEmpty(baseUri.UserInfo) ||
            string.IsNullOrEmpty(baseUri.Host) || !string.IsNullOrEmpty(baseUri.Query) ||
            !string.IsNullOrEmpty(baseUri.Fragment) || baseUri.AbsolutePath != "/" ||
            handler.AllowAutoRedirect || handler.UseCookies || handler.UseProxy ||
            (handler.ClientCertificateOptions == ClientCertificateOption.Manual && handler.ClientCertificates.Count == 0))
            throw Invalid();
        return new AgentEnvelopeTransport(new UriBuilder(baseUri) { Path = DaemonRoutePath, Query = "", Fragment = "" }.Uri,
            credential, handler);
    }

    public async Task<JsonValue> SendAsync(TransportCall call, CancellationToken cancellationToken = default) =>
        (await SendEnvelopeAsync(call, cancellationToken).ConfigureAwait(false)).Value;

    public async Task<AgentEnvelopeResult> SendEnvelopeAsync(TransportCall call, CancellationToken cancellationToken = default,
        Action<int, byte[]>? observeResponse = null)
    {
        var descriptor = call.Operation.Descriptor();
        var mutating = descriptor.RequiresIdempotency;
        var requestId = NewRequestId();
        using var request = EnvelopeRequest(descriptor, call, requestId);
        HttpResponseMessage response;
        try
        {
            response = await _httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken).ConfigureAwait(false);
        }
        catch when (mutating) { throw UnknownOutcome(requestId); }
        catch (OperationCanceledException) { throw new PlatformSdkException(SdkErrorCode.Deadline, RetryClass.Safe, requestId); }
        catch { throw new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe, requestId); }
        using (response)
        {
            byte[] encoded;
            try
            {
                if (response.Content.Headers.ContentType?.MediaType is not string mediaType ||
                    !string.Equals(mediaType, "application/json", StringComparison.OrdinalIgnoreCase))
                    throw mutating ? UnknownOutcome(requestId) : new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe, requestId);
                encoded = await ReadBoundedAsync(response.Content, requestId, cancellationToken).ConfigureAwait(false);
            }
            catch (PlatformSdkException) when (mutating) { throw UnknownOutcome(requestId); }
            catch (PlatformSdkException) { throw; }
            catch when (mutating) { throw UnknownOutcome(requestId); }
            catch (OperationCanceledException) { throw new PlatformSdkException(SdkErrorCode.Deadline, RetryClass.Safe, requestId); }
            catch { throw new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe, requestId); }
            observeResponse?.Invoke((int)response.StatusCode, encoded);
            if ((int)response.StatusCode is 502 or 503 && IsEdgeReply(encoded))
                throw mutating ? UnknownOutcome(requestId) : new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe, requestId);
            if (!WellFormedJson(encoded))
                throw mutating ? UnknownOutcome(requestId) : new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe, requestId);
            try { return DecodeResponse((int)response.StatusCode, encoded, requestId); }
            catch (PlatformSdkException error) when (mutating && error.Code is SdkErrorCode.DecodeFailure or SdkErrorCode.VerificationFailure)
            {
                throw UnknownOutcome(requestId);
            }
        }
    }

    internal HttpRequestMessage EnvelopeRequest(OperationDescriptor descriptor, TransportCall call, string requestId)
    {
        if (descriptor.Plane != PlatformPlane.Agent || call.PathParameters.Count != 0 ||
            call.Request is not JsonValue.ObjectValue) throw Invalid();
        var bootstrap = BootstrapOperations.Contains(descriptor.Name);
        if (!bootstrap && _credential is null) throw new PlatformSdkException(SdkErrorCode.CapabilityRefusal, RetryClass.Never);
        if (descriptor.RequiresIdempotency)
        {
            if (call.IdempotencyKey is not { } key || !Hex32(key.Value))
                throw new PlatformSdkException(SdkErrorCode.IdempotencyRequired, RetryClass.Never);
        }
        else if (call.IdempotencyKey is not null) throw Invalid();
        using var buffer = new MemoryStream();
        using (var writer = new Utf8JsonWriter(buffer))
        {
            writer.WriteStartObject();
            writer.WriteNumber("version", EnvelopeVersion);
            writer.WriteString("request_id", requestId);
            writer.WriteString("operation", descriptor.Name);
            writer.WritePropertyName("request");
            JsonSerializer.Serialize(writer, call.Request, JsonOptions);
            writer.WritePropertyName("credential");
            if (bootstrap) writer.WriteNullValue();
            else _credential!.Write(writer);
            if (call.IdempotencyKey is { } idempotency) writer.WriteString("idempotency_key", idempotency.Value);
            else writer.WriteNull("idempotency_key");
            writer.WriteEndObject();
        }
        var encoded = buffer.ToArray();
        if (encoded.Length > MaximumRequestBytes) throw Invalid();
        var request = new HttpRequestMessage(HttpMethod.Post, _endpoint);
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        request.Headers.UserAgent.ParseAdd("layerx-dotnet/0.1.0");
        request.Headers.TryAddWithoutValidation("LayerX-Request-Id", requestId);
        request.Content = new ByteArrayContent(encoded);
        request.Content.Headers.ContentType = new MediaTypeHeaderValue("application/json");
        request.Content.Headers.ContentLength = encoded.Length;
        _gatewayKey?.Authorize(request);
        return request;
    }

    internal static AgentEnvelopeResult DecodeResponse(int status, byte[] encoded, string requestId)
    {
        if (!WellFormedJson(encoded)) throw new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe, requestId);
        JsonValue? document;
        try
        {
            using (var parsed = JsonDocument.Parse(encoded, new JsonDocumentOptions { MaxDepth = 64 }))
                if (HasDuplicateKey(parsed.RootElement)) throw Decode(requestId);
            document = JsonSerializer.Deserialize<JsonValue>(encoded, JsonOptions);
        }
        catch (JsonException) { throw Decode(requestId); }
        catch (ArgumentException) { throw Decode(requestId); }
        catch (PlatformSdkException) { throw Decode(requestId); }
        if (document is not JsonValue.ObjectValue map) throw Decode(requestId);
        var envelope = map.Value;
        if (!envelope.TryGetValue("version", out var responseVersion) ||
            responseVersion is not JsonValue.IntegerValue { Value: EnvelopeVersion }) throw Decode(requestId);
        if (envelope.ContainsKey("class"))
        {
            if (status is >= 200 and < 300 ||
                !Exact(envelope, "version", "class", "protocol_result_code", "retriability", "request_id", "reason")) throw Decode(requestId);
            var echoed = TryText(envelope, "request_id");
            if (echoed != requestId && echoed != "0") throw Decode(requestId);
            throw ServiceError(envelope, requestId);
        }
        if (status != 200 || !Exact(envelope, "version", "request_id", "value", "verification_status") ||
            TryText(envelope, "request_id") != requestId || envelope["value"] is JsonValue.NullValue ||
            !ValidVerification(envelope["verification_status"])) throw Decode(requestId);
        return new AgentEnvelopeResult(requestId, envelope["value"], envelope["verification_status"]);
    }

    private static bool ValidVerification(JsonValue value)
    {
        if (value is not JsonValue.ObjectValue map) return false;
        var status = map.Value;
        return TryText(status, "state") switch
        {
            "achieved" => Exact(status, "state", "level") && Level(TryText(status, "level")) >= 0,
            "unverified" => Exact(status, "state", "requested", "achieved", "reason") &&
                Level(TryText(status, "achieved")) is var achieved && achieved >= 0 &&
                achieved < Level(TryText(status, "requested")) && ValidReason(TryText(status, "reason")),
            _ => false,
        };
    }

    private static int Level(string? level) => level is null ? -1 : Array.IndexOf(Levels, level);

    private static PlatformSdkException ServiceError(IReadOnlyDictionary<string, JsonValue> envelope, string requestId)
    {
        if (!ValidReason(TryText(envelope, "reason"))) throw Decode(requestId);
        int? resultCode = envelope["protocol_result_code"] switch
        {
            JsonValue.NullValue => null,
            JsonValue.IntegerValue integer when integer.Value is >= int.MinValue and <= int.MaxValue => (int)integer.Value,
            _ => throw Decode(requestId),
        };
        var code = TryText(envelope, "class") switch
        {
            "TransportFailure" => SdkErrorCode.TransportFailure,
            "Deadline" => SdkErrorCode.Deadline,
            "ProtocolIncompatibility" => SdkErrorCode.ProtocolIncompatibility,
            "UnavailableCapability" => SdkErrorCode.UnavailableCapability,
            "CoreRejection" => SdkErrorCode.CoreRejection,
            "VerificationFailure" => SdkErrorCode.VerificationFailure,
            "PolicyRefusal" => SdkErrorCode.PolicyRefusal,
            "CapabilityRefusal" => SdkErrorCode.CapabilityRefusal,
            "BudgetRefusal" => SdkErrorCode.BudgetRefusal,
            "RateLimit" => SdkErrorCode.RateLimit,
            "IdempotencyConflict" => SdkErrorCode.IdempotencyConflict,
            "InternalFault" => SdkErrorCode.InternalFault,
            _ => throw Decode(requestId),
        };
        var retry = TryText(envelope, "retriability") switch
        {
            "Terminal" => RetryClass.Never,
            "Retriable" => RetryClass.Safe,
            _ => throw Decode(requestId),
        };
        return new PlatformSdkException(code, retry, requestId, resultCode);
    }

    private static bool IsEdgeReply(byte[] encoded)
    {
        try
        {
            using var parsed = JsonDocument.Parse(encoded);
            return parsed.RootElement.ValueKind == JsonValueKind.Object &&
                parsed.RootElement.TryGetProperty("ok", out var ok) && ok.ValueKind == JsonValueKind.False &&
                !parsed.RootElement.TryGetProperty("class", out _);
        }
        catch (JsonException) { return false; }
    }

    private static bool WellFormedJson(byte[] encoded)
    {
        if (encoded.Length == 0) return false;
        var reader = new Utf8JsonReader(new ReadOnlySpan<byte>(encoded), new JsonReaderOptions { MaxDepth = int.MaxValue });
        try
        {
            while (reader.Read()) { }
            return true;
        }
        catch (JsonException) { return false; }
    }

    private static bool HasDuplicateKey(JsonElement element)
    {
        switch (element.ValueKind)
        {
            case JsonValueKind.Object:
                var names = new HashSet<string>(StringComparer.Ordinal);
                foreach (var property in element.EnumerateObject())
                    if (!names.Add(property.Name) || HasDuplicateKey(property.Value)) return true;
                return false;
            case JsonValueKind.Array:
                foreach (var item in element.EnumerateArray())
                    if (HasDuplicateKey(item)) return true;
                return false;
            default:
                return false;
        }
    }

    private static async Task<byte[]> ReadBoundedAsync(HttpContent content, string requestId, CancellationToken cancellationToken)
    {
        await using var stream = await content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        using var output = new MemoryStream();
        var buffer = new byte[16 * 1024];
        while (true)
        {
            var count = await stream.ReadAsync(buffer.AsMemory(), cancellationToken).ConfigureAwait(false);
            if (count == 0) return output.ToArray();
            if (output.Length + count > MaximumResponseBytes)
                throw new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe, requestId);
            output.Write(buffer, 0, count);
        }
    }

    private static string NewRequestId()
    {
        Span<byte> bytes = stackalloc byte[8];
        ulong value;
        do
        {
            RandomNumberGenerator.Fill(bytes);
            value = System.Buffers.Binary.BinaryPrimitives.ReadUInt64LittleEndian(bytes);
        } while (value == 0);
        return value.ToString(System.Globalization.CultureInfo.InvariantCulture);
    }

    private static bool Exact(IReadOnlyDictionary<string, JsonValue> value, params string[] fields) =>
        value.Count == fields.Length && fields.All(value.ContainsKey);
    private static string? TryText(IReadOnlyDictionary<string, JsonValue> value, string field) =>
        value.TryGetValue(field, out var raw) && raw is JsonValue.StringValue text ? text.Value : null;
    private static bool ValidReason(string? reason) => !string.IsNullOrEmpty(reason) && reason.Length <= 128 &&
        reason.All(character => character is >= 'a' and <= 'z' or >= '0' and <= '9' or '_' or '.');
    private static bool IsLoopback(string host) => string.Equals(host, "localhost", StringComparison.OrdinalIgnoreCase) ||
        IPAddress.TryParse(host, out var address) && IPAddress.IsLoopback(address);
    private static bool Hex32(string value) => value.Length == 64 && value.All(character => character is >= '0' and <= '9' or >= 'a' and <= 'f');
    private static PlatformSdkException Invalid() => new(SdkErrorCode.InvalidArgument, RetryClass.Never);
    private static PlatformSdkException Decode(string requestId) => new(SdkErrorCode.DecodeFailure, RetryClass.Never, requestId);
    private static PlatformSdkException UnknownOutcome(string requestId) => new(SdkErrorCode.UnknownOutcome, RetryClass.UnknownOutcome, requestId);

    public void Dispose() => _httpClient.Dispose();
}
