using System.Reflection;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class AgentEnvelopeMajor2Tests
{
    private static byte[] Golden(string name, [CallerFilePath] string source = "") => File.ReadAllBytes(
        Path.GetFullPath(Path.Combine(Path.GetDirectoryName(source)!, "../../../../../services/agent/schema/agent-api/golden", name)));

    [Fact]
    public async Task AgentEnvelopeEncodesTransportVersionOne()
    {
        using var golden = JsonDocument.Parse(Golden("agent-http-request.json"));
        var session = golden.RootElement.GetProperty("credential");
        using var credential = new AgentSessionCredential(session.GetProperty("tenant").GetString()!,
            Convert.FromHexString(session.GetProperty("session_id").GetString()!),
            Convert.FromHexString(session.GetProperty("token_id").GetString()!),
            ulong.Parse(session.GetProperty("generation").GetString()!, System.Globalization.CultureInfo.InvariantCulture));
        using var transport = new AgentEnvelopeTransport(new Uri("http://127.0.0.1:1/"), credential);
        var encode = typeof(AgentEnvelopeTransport).GetMethod("EnvelopeRequest", BindingFlags.Instance | BindingFlags.NonPublic)!;
        var operations = Enum.GetValues<PlatformOperation>()
            .Where(operation => operation.Descriptor().Plane == PlatformPlane.Agent).ToArray();
        Assert.NotEmpty(operations);
        foreach (var operation in operations)
        {
            var descriptor = operation.Descriptor();
            var call = new TransportCall(operation, JsonValue.EmptyObject, new Dictionary<string, string>(),
                descriptor.RequiresIdempotency ? new IdempotencyKey(new string('a', 64)) : null);
            using var request = Assert.IsType<HttpRequestMessage>(encode.Invoke(transport, [descriptor, call, "7"]));
            var encoded = await request.Content!.ReadAsByteArrayAsync();
            using var document = JsonDocument.Parse(encoded);
            Assert.Equal(1, version(document.RootElement));
            Assert.Equal(descriptor.Name, document.RootElement.GetProperty("operation").GetString());
            Assert.True(JsonNode.DeepEquals(JsonNode.Parse(session.GetRawText()), JsonNode.Parse(document.RootElement.GetProperty("credential").GetRawText())));
            if (descriptor.Name == "tenant.readiness") Assert.Equal(Golden("agent-http-request.json"), encoded);
        }
    }

    [Fact]
    public async Task AgentEnvelopeAcceptsUnversionedResponses()
    {
        foreach (var (filename, state) in new[]
        {
            ("agent-http-success.json", "achieved"),
            ("agent-http-success-pending.json", "pending"),
            ("agent-http-success-absent.json", "absent")
        })
        {
            var golden = Golden(filename);
            var result = await Exchange(200, golden);
            var status = Assert.IsType<JsonValue.ObjectValue>(result.VerificationStatus).Value;
            Assert.Equal(JsonValue.String(state), status["state"]);
            using var document = JsonDocument.Parse(golden);
            Assert.True(JsonNode.DeepEquals(JsonNode.Parse(document.RootElement.GetProperty("value").GetRawText()),
                JsonSerializer.SerializeToNode(result.Value)));
        }
        var error = await Assert.ThrowsAsync<PlatformSdkException>(() => Exchange(403, Golden("agent-http-refusal.json")));
        Assert.Equal(SdkErrorCode.PolicyRefusal, error.Code);
        Assert.Equal(RetryClass.Never, error.Retry);

        foreach (var (status, golden) in new[] { (200, Golden("agent-http-success.json")), (403, Golden("agent-http-refusal.json")) })
        {
            foreach (var invalidVersion in new[] { "1", "2", "0", "3", "\"2\"", "null" })
            {
                var response = Encoding.UTF8.GetString(golden).Insert(1, $"\"version\":{invalidVersion},");
                var refusal = await Assert.ThrowsAsync<PlatformSdkException>(() => Exchange(status, Encoding.UTF8.GetBytes(response)));
                Assert.Equal(SdkErrorCode.DecodeFailure, refusal.Code);
                Assert.Equal(RetryClass.Never, refusal.Retry);
                Assert.False(string.IsNullOrEmpty(refusal.RequestId));
            }
        }
    }

    private static async Task<AgentEnvelopeResult> Exchange(int status, byte[] golden)
    {
        await using var server = new RepairM1368Http();
        using var requestGolden = JsonDocument.Parse(Golden("agent-http-request.json"));
        var session = requestGolden.RootElement.GetProperty("credential");
        using var credential = new AgentSessionCredential(session.GetProperty("tenant").GetString()!,
            Convert.FromHexString(session.GetProperty("session_id").GetString()!),
            Convert.FromHexString(session.GetProperty("token_id").GetString()!), 7);
        using var transport = new AgentEnvelopeTransport(server.Endpoint, credential);
        var call = new TransportCall(PlatformOperation.AgentTenantReadiness, JsonValue.EmptyObject, new Dictionary<string, string>(), null);
        var operation = transport.SendEnvelopeAsync(call);
        string? requestId = null;
        await server.Exchange(request =>
        {
            requestId = request.GetProperty("request_id").GetString();
            Assert.Equal(1, version(request));
            Assert.Equal("tenant.readiness", request.GetProperty("operation").GetString());
            var normalized = JsonNode.Parse(request.GetRawText())!.AsObject();
            normalized["request_id"] = "7";
            Assert.True(JsonNode.DeepEquals(JsonNode.Parse(requestGolden.RootElement.GetRawText()), normalized));
            // Correlation IDs are the only transcript field changed by the loopback peer.
            var response = JsonNode.Parse(golden)!.AsObject();
            response["request_id"] = request.GetProperty("request_id").GetString();
            return (status, JsonSerializer.SerializeToUtf8Bytes(response));
        }, AgentEnvelopeTransport.RoutePath);
        try
        {
            var result = await operation;
            Assert.Equal(requestId, result.RequestId);
            return result;
        }
        catch (PlatformSdkException error)
        {
            Assert.Equal(requestId, error.RequestId);
            throw;
        }
    }

    private static int version(JsonElement request)
    {
        var value = request.GetProperty("version");
        Assert.Equal(JsonValueKind.Number, value.ValueKind);
        return value.GetInt32();
    }
}
