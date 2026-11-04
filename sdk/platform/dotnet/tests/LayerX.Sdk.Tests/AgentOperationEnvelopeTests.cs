using System.Net.Security;
using System.Reflection;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using LayerX.Sdk;
using Xunit;
using Xunit.Abstractions;

namespace LayerX.Sdk.Tests;

[Trait("Category", "AgentOperationEnvelopeProbe")]
public sealed class AgentOperationEnvelopeTests(ITestOutputHelper output)
{
    private static readonly Dictionary<string, string> NoPathParameters = new(StringComparer.Ordinal);
    private static readonly Dictionary<string, PlatformOperation> AgentOperations =
        Enum.GetValues<PlatformOperation>().Where(value => value.Descriptor().Plane == PlatformPlane.Agent)
            .ToDictionary(value => value.Descriptor().Name, StringComparer.Ordinal);

    internal sealed class CaseFile : IDisposable
    {
        public required Uri Gateway { get; init; }
        public required X509Certificate2 Root { get; init; }
        public required string KeyId { get; init; }
        public required byte[] KeySecret { get; init; }
        public required string Tenant { get; init; }
        public required byte[] SessionId { get; init; }
        public required byte[] TokenId { get; init; }
        public required ulong Generation { get; init; }
        public required JsonElement Requests { get; init; }
        public required string[] Cases { get; init; }
        public required string Phase { get; init; }
        public required string ResponseDirectory { get; init; }
        public Uri? DaemonEndpoint { get; init; }
        public string? ClientCertFile { get; init; }
        public string? ClientKeyFile { get; init; }
        public string? ServerCaFile { get; init; }

        public void Dispose()
        {
            Root.Dispose();
            Array.Clear(KeySecret);
            Array.Clear(TokenId);
        }
    }

    internal static InvalidOperationException Refused(string reason) => new($"agent envelope probe input refused: {reason}");

    private static string Text(JsonElement value, string field) =>
        value.TryGetProperty(field, out var raw) && raw.ValueKind == JsonValueKind.String && raw.GetString() is { Length: > 0 } text
            ? text : throw Refused(field);

    private static byte[] Hex32(JsonElement value, string field)
    {
        var text = Text(value, field);
        if (text.Length != 64 || text.Any(character => !(character is >= '0' and <= '9' or >= 'a' and <= 'f'))) throw Refused(field);
        return Convert.FromHexString(text);
    }

    private static string AbsoluteFile(JsonElement value, string field)
    {
        var path = Text(value, field);
        return Path.IsPathRooted(path) && File.Exists(path) ? path : throw Refused(field);
    }

    internal static CaseFile Load()
    {
        var path = Environment.GetEnvironmentVariable("PAXEER_X_AGENT_ENVELOPE_CASE");
        if (string.IsNullOrEmpty(path) || !Path.IsPathRooted(path) || !File.Exists(path)) throw Refused("PAXEER_X_AGENT_ENVELOPE_CASE");
        using var document = JsonDocument.Parse(File.ReadAllBytes(path));
        var root = document.RootElement;
        if (root.ValueKind != JsonValueKind.Object) throw Refused("case file");

        var endpoint = new Uri(Text(root, "endpoint"), UriKind.Absolute);
        if (endpoint.Scheme != Uri.UriSchemeHttps || endpoint.AbsolutePath != AgentEnvelopeTransport.RoutePath ||
            !string.IsNullOrEmpty(endpoint.Query) || !string.IsNullOrEmpty(endpoint.Fragment) ||
            !string.Equals(endpoint.Host, Text(root, "server_name"), StringComparison.OrdinalIgnoreCase)) throw Refused("endpoint");

        X509Certificate2 ca;
        if (root.TryGetProperty("ca_pem", out _)) ca = X509Certificate2.CreateFromPemFile(AbsoluteFile(root, "ca_pem"));
        else ca = new X509Certificate2(File.ReadAllBytes(AbsoluteFile(root, "ca_der")));

        var key = File.ReadAllText(AbsoluteFile(root, "gateway_api_key_file"), new UTF8Encoding(false, true));
        if (key.EndsWith('\n')) key = key[..^1];
        var separator = key.IndexOf(':');
        if (separator <= 0 || key.Contains('\n') || key.Contains('\r')) throw Refused("gateway_api_key_file");

        using var credentialDocument = JsonDocument.Parse(File.ReadAllBytes(AbsoluteFile(root, "credential_file")));
        var credential = credentialDocument.RootElement;
        if (credential.ValueKind != JsonValueKind.Object || credential.EnumerateObject().Count() != 4) throw Refused("credential_file");
        var generation = Text(credential, "generation");
        if ((generation != "0" && generation[0] == '0') || generation.Any(character => character is < '0' or > '9') ||
            !ulong.TryParse(generation, out var parsedGeneration)) throw Refused("credential_file generation");

        if (!root.TryGetProperty("cases", out var cases) || cases.ValueKind != JsonValueKind.Array ||
            !root.TryGetProperty("requests", out var requests) || requests.ValueKind != JsonValueKind.Object) throw Refused("cases/requests");
        var directory = Text(root, "response_dir");
        if (!Path.IsPathRooted(directory) || !Directory.Exists(directory)) throw Refused("response_dir");

        string[] daemonFields = ["daemon_endpoint", "client_cert_file", "client_key_file", "server_ca_file"];
        var daemonPresent = daemonFields.Count(field => root.TryGetProperty(field, out _));
        if (daemonPresent is not 0 and not 4) throw Refused("daemon surface fields");
        Uri? daemon = null;
        if (daemonPresent == 4)
        {
            daemon = new Uri(Text(root, "daemon_endpoint"), UriKind.Absolute);
            if (daemon.Scheme != Uri.UriSchemeHttps || daemon.AbsolutePath != AgentEnvelopeTransport.DaemonRoutePath ||
                !string.IsNullOrEmpty(daemon.UserInfo) || !string.IsNullOrEmpty(daemon.Query) || !string.IsNullOrEmpty(daemon.Fragment))
                throw Refused("daemon_endpoint");
        }

        return new CaseFile
        {
            Gateway = new Uri(endpoint.GetLeftPart(UriPartial.Authority) + "/"),
            Root = ca,
            KeyId = key[..separator],
            KeySecret = Encoding.UTF8.GetBytes(key[(separator + 1)..]),
            Tenant = Text(credential, "tenant"),
            SessionId = Hex32(credential, "session_id"),
            TokenId = Hex32(credential, "token_id"),
            Generation = parsedGeneration,
            Requests = requests.Clone(),
            Cases = cases.EnumerateArray().Select(item => item.ValueKind == JsonValueKind.String ? item.GetString()! : throw Refused("cases")).ToArray(),
            Phase = Text(root, "phase"),
            ResponseDirectory = directory,
            DaemonEndpoint = daemon is null ? null : new Uri(daemon.GetLeftPart(UriPartial.Authority) + "/"),
            ClientCertFile = daemon is null ? null : AbsoluteFile(root, "client_cert_file"),
            ClientKeyFile = daemon is null ? null : AbsoluteFile(root, "client_key_file"),
            ServerCaFile = daemon is null ? null : AbsoluteFile(root, "server_ca_file"),
        };
    }

    internal static AgentEnvelopeTransport Transport(CaseFile input, AgentSessionCredential? credential, LayerXKeyCredential key) =>
        new(input.Gateway, credential, key, input.Root);

    internal static AgentSessionCredential Credential(CaseFile input, ulong generation) =>
        new(input.Tenant, input.SessionId, input.TokenId, generation);

    internal static TransportCall Call(CaseFile input, string caseId)
    {
        if (!input.Requests.TryGetProperty(caseId, out var entry) || entry.ValueKind != JsonValueKind.Object) throw Refused($"requests.{caseId}");
        var name = Text(entry, "operation");
        if (!AgentOperations.TryGetValue(name, out var operation)) throw Refused($"requests.{caseId}.operation");
        if (!entry.TryGetProperty("request", out var request) || request.ValueKind != JsonValueKind.Object) throw Refused($"requests.{caseId}.request");
        IdempotencyKey? idempotency = entry.TryGetProperty("idempotency_key", out _)
            ? new IdempotencyKey(Convert.ToHexString(Hex32(entry, "idempotency_key")).ToLowerInvariant()) : null;
        var value = JsonSerializer.Deserialize<JsonValue>(request.GetRawText())!;
        return new TransportCall(operation, value, NoPathParameters, idempotency);
    }

    private static void AssertSuccess(AgentEnvelopeResult result)
    {
        Assert.False(string.IsNullOrEmpty(result.RequestId));
        Assert.IsType<JsonValue.ObjectValue>(result.VerificationStatus);
        Assert.IsNotType<JsonValue.NullValue>(result.Value);
    }

    private static async Task AssertOwnerlessDaemonRefusal(AgentEnvelopeTransport transport, TransportCall call)
    {
        const string requestId = "1368";
        var encode = typeof(AgentEnvelopeTransport).GetMethod("EnvelopeRequest", BindingFlags.Instance | BindingFlags.NonPublic)!;
        using var request = Assert.IsType<HttpRequestMessage>(encode.Invoke(transport, [call.Operation.Descriptor(), call, requestId]));
        var body = JsonNode.Parse(await request.Content!.ReadAsByteArrayAsync())!.AsObject();
        body["credential"] = null;
        request.Content.Dispose();
        request.Content = new ByteArrayContent(JsonSerializer.SerializeToUtf8Bytes(body));
        request.Content.Headers.ContentType = new("application/json");
        var http = Assert.IsType<HttpClient>(typeof(AgentEnvelopeTransport).GetField("_httpClient", BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(transport));
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(30));
        using var reply = await http.SendAsync(request, timeout.Token);
        Assert.Equal(400, (int)reply.StatusCode);
        var bytes = await reply.Content.ReadAsByteArrayAsync(timeout.Token);
        using var parsed = JsonDocument.Parse(bytes);
        Assert.Equal("envelope.credential", parsed.RootElement.GetProperty("reason").GetString());
        var decode = typeof(AgentEnvelopeTransport).GetMethod("DecodeResponse", BindingFlags.Static | BindingFlags.NonPublic)!
            .CreateDelegate<Func<int, byte[], string, AgentEnvelopeResult>>();
        var error = Assert.Throws<PlatformSdkException>(() => decode((int)reply.StatusCode, bytes, requestId));
        Assert.Equal(SdkErrorCode.ProtocolIncompatibility, error.Code);
        Assert.Equal(RetryClass.Never, error.Retry);
        Assert.Equal(requestId, error.RequestId);
    }

    private static void AssertPrepared(CaseFile input, string caseId, AgentEnvelopeResult result)
    {
        var response = Assert.IsType<JsonValue.ObjectValue>(result.Value).Value;
        Assert.Equal(new[] { "authority", "value" }, response.Keys.OrderBy(value => value, StringComparer.Ordinal));
        var authority = Assert.IsType<JsonValue.ObjectValue>(response["authority"]).Value;
        Assert.Equal(JsonValue.String(input.Tenant), authority["tenant"]);
        var prepared = Assert.IsType<JsonValue.ObjectValue>(response["value"]).Value;
        string[] fields = ["chain_id", "nonce", "max_priority_fee_per_gas", "max_fee_per_gas", "gas_limit", "to", "value", "data", "signing_hash", "observed_head_height"];
        Assert.Equal(fields.OrderBy(value => value, StringComparer.Ordinal), prepared.Keys.OrderBy(value => value, StringComparer.Ordinal));
        var entry = input.Requests.GetProperty(caseId);
        var expected = entry.GetProperty("expected_preparation");
        Assert.Equal(fields.OrderBy(value => value, StringComparer.Ordinal), expected.EnumerateObject().Select(property => property.Name).OrderBy(value => value, StringComparer.Ordinal));
        foreach (var field in fields)
            Assert.Equal(expected.GetProperty(field).GetString(), Assert.IsType<JsonValue.StringValue>(prepared[field]).Value);
        Assert.Equal(JsonValue.String("125"), prepared["chain_id"]);
        Assert.Equal(JsonValue.String("0"), prepared["value"]);
        Assert.Equal(JsonValue.String(new string('0', 36) + (caseId == "agent_register" ? "1004" : "101c")), prepared["to"]);
        Assert.StartsWith(caseId == "agent_register" ? "dd9aa628" : "1107b49e", Assert.IsType<JsonValue.StringValue>(prepared["data"]).Value);
        Assert.Equal(64, Assert.IsType<JsonValue.StringValue>(prepared["signing_hash"]).Value.Length);
        Assert.True(ulong.Parse(Assert.IsType<JsonValue.StringValue>(prepared["gas_limit"]).Value, System.Globalization.CultureInfo.InvariantCulture) > 0);
    }

    private void Emit(string line)
    {
        Console.Out.WriteLine(line);
        Console.Out.Flush();
        output.WriteLine(line);
    }

    [Fact]
    public async Task ProcessCasesReachTheDaemonThroughTheGateway()
    {
        using var input = Load();
        Assert.Equal("read", input.Phase);
        string[] expected = ["read", "program_read", "approval_list", "agent_register", "session_open"];
        Assert.Equal(expected, input.Cases);
        string[] operations = ["read.account", "program.interface", "approval.list", "agent.register", "session.open"];
        using var key = new LayerXKeyCredential(input.KeyId, input.KeySecret);
        using var credential = Credential(input, input.Generation);
        using var transport = Transport(input, credential, key);
        var passed = 0;
        for (var index = 0; index < input.Cases.Length; index++)
        {
            var caseId = input.Cases[index];
            var call = Call(input, caseId);
            Assert.Equal(operations[index], call.Operation.Descriptor().Name);
            int? status = null; byte[]? body = null;
            var result = await transport.SendEnvelopeAsync(call, observeResponse: (code, bytes) => { status = code; body = bytes; });
            Assert.Equal(200, status);
            using (var parsed = JsonDocument.Parse(body!))
            {
                var record = new Dictionary<string, object> { ["status"] = status!.Value, ["body"] = parsed.RootElement };
                await File.WriteAllBytesAsync(Path.Combine(input.ResponseDirectory, caseId + ".json"),
                    JsonSerializer.SerializeToUtf8Bytes(record));
            }
            AssertSuccess(result);
            if (caseId is "agent_register" or "session_open")
            {
                Assert.NotNull(call.IdempotencyKey);
                AssertPrepared(input, caseId, result);
                using var ownerless = Transport(input, null, key);
                var refusal = await Assert.ThrowsAsync<PlatformSdkException>(() => ownerless.SendEnvelopeAsync(call));
                Assert.Equal(SdkErrorCode.CapabilityRefusal, refusal.Code);
                Assert.Equal(RetryClass.Never, refusal.Retry);
                await AssertOwnerlessDaemonRefusal(transport, call);
            }
            passed++;
            Emit($"PAXEER_X_AGENT_ENVELOPE_CASE {caseId} passed");
        }
        Assert.Equal(expected.Length, passed);
        Emit($"PAXEER_X_AGENT_ENVELOPE_CASES={passed}");
    }

    [Fact]
    public async Task UnverifiedRungIsPreservedFromTheDaemon()
    {
        using var input = Load();
        using var key = new LayerXKeyCredential(input.KeyId, input.KeySecret);
        using var credential = Credential(input, input.Generation);
        using var transport = Transport(input, credential, key);
        var call = Call(input, "read_unverified");
        Assert.False(call.Operation.Descriptor().RequiresIdempotency);
        Assert.Null(call.IdempotencyKey);
        var result = await transport.SendEnvelopeAsync(call);
        AssertSuccess(result);
        var verification = Assert.IsType<JsonValue.ObjectValue>(result.VerificationStatus).Value;
        Assert.Equal(new[] { "achieved", "reason", "requested", "state" }, verification.Keys.OrderBy(value => value, StringComparer.Ordinal));
        Assert.Equal(JsonValue.String("unverified"), verification["state"]);
        Assert.Equal(JsonValue.String("receipted"), verification["requested"]);
        Assert.Equal(JsonValue.String("submitted"), verification["achieved"]);
        Assert.Equal(input.Requests.GetProperty("read_unverified").GetProperty("expected_reason").GetString(),
            Assert.IsType<JsonValue.StringValue>(verification["reason"]).Value);
    }

    [Fact]
    public async Task WrongGenerationIsRefusedByTheDaemonSessionAuthority()
    {
        using var input = Load();
        using var key = new LayerXKeyCredential(input.KeyId, input.KeySecret);
        using var credential = Credential(input, input.Generation == ulong.MaxValue ? input.Generation - 1 : input.Generation + 1);
        using var transport = Transport(input, credential, key);
        var error = await Assert.ThrowsAsync<PlatformSdkException>(() => transport.SendEnvelopeAsync(Call(input, "read")));
        Assert.Equal(SdkErrorCode.PolicyRefusal, error.Code);
        Assert.Equal(RetryClass.Never, error.Retry);
    }

    [Fact]
    public async Task CatalogueReadWithoutSessionCredentialIsRefusedBeforeSending()
    {
        using var input = Load();
        using var key = new LayerXKeyCredential(input.KeyId, input.KeySecret);
        using var transport = Transport(input, null, key);
        var error = await Assert.ThrowsAsync<PlatformSdkException>(() => transport.SendEnvelopeAsync(Call(input, "read")));
        Assert.Equal(SdkErrorCode.CapabilityRefusal, error.Code);
    }

    [Fact]
    public async Task MutationWithoutIdempotencyKeyIsRefusedBeforeSending()
    {
        using var input = Load();
        using var key = new LayerXKeyCredential(input.KeyId, input.KeySecret);
        using var credential = Credential(input, input.Generation);
        using var transport = Transport(input, credential, key);
        var error = await Assert.ThrowsAsync<PlatformSdkException>(() =>
            transport.SendEnvelopeAsync(new TransportCall(PlatformOperation.AgentBudgetCreate, JsonValue.EmptyObject, NoPathParameters, null)));
        Assert.Equal(SdkErrorCode.IdempotencyRequired, error.Code);
    }

    [Fact]
    public void DirectDaemonSurfaceRefusesPlaintextAndMissingClientCertificateBeforeSending()
    {
        using (var handler = new HttpClientHandler { AllowAutoRedirect = false, UseCookies = false, UseProxy = false })
        {
            var plaintext = Assert.Throws<PlatformSdkException>(() =>
                AgentEnvelopeTransport.ForDaemon(new Uri("http://localhost/"), null, handler));
            Assert.Equal(SdkErrorCode.InvalidArgument, plaintext.Code);
            var noCertificate = Assert.Throws<PlatformSdkException>(() =>
                AgentEnvelopeTransport.ForDaemon(new Uri("https://localhost/"), null, handler));
            Assert.Equal(SdkErrorCode.InvalidArgument, noCertificate.Code);
        }
        using (var redirecting = new HttpClientHandler { UseCookies = false, UseProxy = false, ClientCertificateOptions = ClientCertificateOption.Automatic })
        {
            var redirect = Assert.Throws<PlatformSdkException>(() =>
                AgentEnvelopeTransport.ForDaemon(new Uri("https://localhost/"), null, redirecting));
            Assert.Equal(SdkErrorCode.InvalidArgument, redirect.Code);
        }
    }
}

[Trait("Category", "AgentOperationEnvelopeDecodeProbe")]
public sealed class AgentOperationEnvelopeDecodeTests(ITestOutputHelper output)
{
    private static readonly string[] DecodeCases = ["read_decode_failure", "mutation_decode_unknown"];

    private void Emit(string line)
    {
        Console.Out.WriteLine(line);
        Console.Out.Flush();
        output.WriteLine(line);
    }

    private static bool TrustedDaemon(X509Certificate2 root, X509Certificate2? certificate, X509Chain? presented, SslPolicyErrors errors)
    {
        if (certificate is null || (errors & ~SslPolicyErrors.RemoteCertificateChainErrors) != SslPolicyErrors.None) return false;
        using var chain = new X509Chain();
        chain.ChainPolicy.TrustMode = X509ChainTrustMode.CustomRootTrust;
        chain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
        chain.ChainPolicy.CustomTrustStore.Add(root);
        if (presented is not null)
            foreach (var element in presented.ChainElements) chain.ChainPolicy.ExtraStore.Add(element.Certificate);
        return chain.Build(certificate);
    }

    [Fact]
    public async Task WellFormedSchemaViolationIsReadDecodeFailureAndMutationUnknown()
    {
        using var input = AgentOperationEnvelopeTests.Load();
        Assert.NotEmpty(input.Cases);
        Assert.Equal(input.Cases.Length, input.Cases.Distinct(StringComparer.Ordinal).Count());
        Assert.All(input.Cases, caseId => Assert.Contains(caseId, DecodeCases));
        using var key = new LayerXKeyCredential(input.KeyId, input.KeySecret);
        using var credential = AgentOperationEnvelopeTests.Credential(input, input.Generation);
        using var gateway = AgentOperationEnvelopeTests.Transport(input, credential, key);
        X509Certificate2? serverCa = null, client = null;
        HttpClientHandler? handler = null;
        AgentEnvelopeTransport? daemon = null;
        try
        {
            if (input.DaemonEndpoint is not null)
            {
                var trusted = X509Certificate2.CreateFromPemFile(input.ServerCaFile!);
                serverCa = trusted;
                client = X509Certificate2.CreateFromPemFile(input.ClientCertFile!, input.ClientKeyFile!);
                handler = new HttpClientHandler
                {
                    AllowAutoRedirect = false, UseCookies = false, UseProxy = false,
                    ClientCertificateOptions = ClientCertificateOption.Manual,
                    ServerCertificateCustomValidationCallback = (_, certificate, presented, errors) =>
                        TrustedDaemon(trusted, certificate, presented, errors),
                };
                handler.ClientCertificates.Add(client);
                daemon = AgentEnvelopeTransport.ForDaemon(input.DaemonEndpoint, credential, handler);
            }
            var passed = 0;
            foreach (var caseId in input.Cases)
            {
                var call = AgentOperationEnvelopeTests.Call(input, caseId);
                var mutating = call.Operation.Descriptor().RequiresIdempotency;
                Assert.Equal(caseId == "mutation_decode_unknown", mutating);
                if (mutating) Assert.NotNull(call.IdempotencyKey);
                else Assert.Null(call.IdempotencyKey);
                var transport = !mutating && daemon is not null ? daemon : gateway;
                var replies = 0; int? status = null; byte[]? body = null;
                var error = await Assert.ThrowsAsync<PlatformSdkException>(() => transport.SendEnvelopeAsync(call,
                    observeResponse: (code, bytes) => { replies++; status = code; body = bytes; }));
                Assert.Equal(1, replies);
                using (var parsed = JsonDocument.Parse(body!))
                {
                    var record = new Dictionary<string, object> { ["status"] = status!.Value, ["body"] = parsed.RootElement };
                    await File.WriteAllBytesAsync(Path.Combine(input.ResponseDirectory, caseId + ".json"),
                        JsonSerializer.SerializeToUtf8Bytes(record));
                }
                Assert.False(string.IsNullOrEmpty(error.RequestId));
                if (mutating)
                {
                    Assert.Equal(SdkErrorCode.UnknownOutcome, error.Code);
                    Assert.Equal(RetryClass.UnknownOutcome, error.Retry);
                }
                else
                {
                    Assert.Equal(SdkErrorCode.DecodeFailure, error.Code);
                    Assert.Equal(RetryClass.Never, error.Retry);
                }
                passed++;
                Emit($"PAXEER_X_AGENT_ENVELOPE_CASE {caseId} passed");
            }
            Assert.Equal(input.Cases.Length, passed);
            Emit($"PAXEER_X_AGENT_ENVELOPE_CASES={passed}");
        }
        finally
        {
            daemon?.Dispose();
            handler?.Dispose();
            client?.Dispose();
            serverCa?.Dispose();
        }
    }
}
