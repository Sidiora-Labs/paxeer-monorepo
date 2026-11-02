using System.Reflection;
using System.Text;
using System.Text.Json;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class AgentEnvelopeMajor2Tests
{
    private const string ResponseEnvelope = """
        {"version":2,"request_id":"17","value":{"ok":true},"verification_status":{"state":"achieved","level":"SequencerSigned"}}
        """;

    [Fact]
    public async Task AgentEnvelopeEncodesVersionTwo()
    {
        using var credential = new AgentSessionCredential("major-two", new byte[32], new byte[32], 1);
        using var transport = new AgentEnvelopeTransport(new Uri("http://127.0.0.1:8080"), credential);
        var encode = typeof(AgentEnvelopeTransport).GetMethod("EnvelopeRequest", BindingFlags.Instance | BindingFlags.NonPublic)!;
        var operations = Enum.GetValues<PlatformOperation>()
            .Where(operation => operation.Descriptor().Plane == PlatformPlane.Agent).ToArray();
        Assert.NotEmpty(operations);
        foreach (var operation in operations)
        {
            var descriptor = operation.Descriptor();
            var call = new TransportCall(operation, JsonValue.EmptyObject, new Dictionary<string, string>(),
                descriptor.RequiresIdempotency ? new IdempotencyKey(new string('a', 64)) : null);
            using var request = Assert.IsType<HttpRequestMessage>(encode.Invoke(transport, [descriptor, call, "17"]));
            using var document = JsonDocument.Parse(await request.Content!.ReadAsByteArrayAsync());
            Assert.Equal(2, version(document.RootElement));
            Assert.Equal(descriptor.Name, document.RootElement.GetProperty("operation").GetString());
        }
    }

    [Fact]
    public void AgentEnvelopeRefusesVersionOneResponse()
    {
        var result = agentEnvelope(200, ResponseEnvelope);
        Assert.Equal("17", result.RequestId);
        Assert.Equal(JsonValue.Boolean(true), Assert.IsType<JsonValue.ObjectValue>(result.Value).Value["ok"]);
        const string errorEnvelope = """
            {"version":2,"request_id":"17","class":"PolicyRefusal","protocol_result_code":null,"retriability":"Terminal","reason":"policy.refused"}
            """;
        var serviceFailure = Assert.Throws<PlatformSdkException>(() => agentEnvelope(403, errorEnvelope));
        Assert.Equal(SdkErrorCode.PolicyRefusal, serviceFailure.Code);

        foreach (var (status, response) in new[] { (200, ResponseEnvelope), (403, errorEnvelope) })
        {
            foreach (var invalidVersion in new[] { "1", "0", "3", "\"2\"", "null" })
            {
                var encoded = response.Replace("\"version\":2", $"\"version\":{invalidVersion}", StringComparison.Ordinal);
                var error = Assert.Throws<PlatformSdkException>(() => agentEnvelope(status, encoded));
                Assert.Equal(SdkErrorCode.DecodeFailure, error.Code);
                Assert.Equal(RetryClass.Never, error.Retry);
                Assert.Equal("17", error.RequestId);
            }
            var unversioned = response.Replace("\"version\":2,", "", StringComparison.Ordinal);
            var missingVersion = Assert.Throws<PlatformSdkException>(() => agentEnvelope(status, unversioned));
            Assert.Equal(SdkErrorCode.DecodeFailure, missingVersion.Code);
        }
    }

    private static int version(JsonElement request)
    {
        var value = request.GetProperty("version");
        Assert.Equal(JsonValueKind.Number, value.ValueKind);
        return value.GetInt32();
    }

    private static AgentEnvelopeResult agentEnvelope(int status, string response)
    {
        var decode = typeof(AgentEnvelopeTransport).GetMethod("DecodeResponse", BindingFlags.Static | BindingFlags.NonPublic)!
            .CreateDelegate<Func<int, byte[], string, AgentEnvelopeResult>>();
        return decode(status, Encoding.UTF8.GetBytes(response), "17");
    }
}
