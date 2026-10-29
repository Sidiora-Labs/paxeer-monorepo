using System.Reflection;
using System.Runtime.CompilerServices;
using System.Text.Json;
using LayerX.Sdk;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class ReceiptFixtureTests
{
    [Fact]
    public void NativeAccountAuthorizationVectors()
    {
        using var document = JsonDocument.Parse(File.ReadAllText(FixturePath("../../../../programs/fixtures/pay5/account-authorization-vectors.json")));
        var decode = typeof(ProgramsClient).GetMethod("DecodeAuthorizationRoot", BindingFlags.NonPublic | BindingFlags.Static)!;
        foreach (var vector in document.RootElement.EnumerateArray())
        {
            var encoded = HexField(vector, "encoded"); var root = HexField(vector, "root");
            if (vector.GetProperty("accept").GetBoolean()) Assert.Equal(root, (byte[])decode.Invoke(null, new object[] { encoded })!);
            else
            {
                var error = Record.Exception(() => {
                    var actual = (byte[])decode.Invoke(null, new object[] { encoded })!;
                    if (!actual.SequenceEqual(root)) throw new InvalidDataException();
                });
                Assert.NotNull(error);
                Assert.True(error is InvalidDataException || error is TargetInvocationException { InnerException: InvalidDataException });
            }
        }
    }

    [Fact]
    public async Task NativeLifecycleCFixtures()
    {
        using var token = new AccessToken(System.Text.Encoding.ASCII.GetBytes("fixture-bearer"));
        var transport = new AgentHttpTransport(new Uri("http://127.0.0.1:8080"), accessToken: token);
        var build = typeof(AgentHttpTransport).GetMethod("ProgramRequest", System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.NonPublic)!;
        foreach (var name in new[] { "deploy", "upgrade", "wind-down-route", "wind-down-deprecate", "wind-down-tombstone", "wind-down-exit" })
        {
            using var document = JsonDocument.Parse(File.ReadAllText(FixturePath("native-program-" + name + "-v3.json")));
            var fixture = document.RootElement; Assert.Equal(3, fixture.GetProperty("protocol_version").GetInt32()); Assert.Equal(9, fixture.GetProperty("module").GetInt32());
            var ordinal = fixture.GetProperty("ordinal").GetInt32(); var payload = HexField(fixture, "payload_hex"); var signed = HexField(fixture, "signed_activity_hex");
            INativeProgramLifecycle Decode(byte[] bytes) => ordinal switch { 1 => NativeProgramDeploy.Decode(bytes), 2 => NativeProgramUpgrade.Decode(bytes), 7 => NativeProgramWindDown.Decode(bytes), _ => throw new ArgumentException() };
            var value = Decode(payload); Assert.Equal(payload, value.Encode()); var request = new NativeProgramLifecycleRequest(value, signed);
            Assert.Equal(HexField(fixture, "activity_id_hex"), request.ActivityId); Assert.Equal(fixture.GetProperty("idempotency_key_hex").GetString(), request.IdempotencyKey);
            var operation = ordinal switch { 1 => "program.deploy", 2 => "program.upgrade", _ => "program.wind-down" };
            var path = ordinal switch { 1 => "/v1/programs/deploy", 2 => "/v1/programs/upgrade", _ => "/v1/programs/wind-down" };
            var call = new ProgramTransportCall(operation, JsonValue.Object(new Dictionary<string, JsonValue>
            {
                ["payload"] = JsonValue.String(Convert.ToHexString(payload).ToLowerInvariant()),
                ["signed_activity"] = JsonValue.String(Convert.ToHexString(signed).ToLowerInvariant())
            }), new Dictionary<string, string>(), new IdempotencyKey(request.IdempotencyKey));
            using var http = (HttpRequestMessage)build.Invoke(transport, new object[] { call })!;
            Assert.Equal(path, http.RequestUri!.AbsolutePath); Assert.Equal(HttpMethod.Post, http.Method);
            Assert.Equal("application/octet-stream", http.Content!.Headers.ContentType!.MediaType); Assert.Equal(signed, await http.Content.ReadAsByteArrayAsync());
            Assert.Equal(request.IdempotencyKey, Assert.Single(http.Headers.GetValues("Idempotency-Key"))); Assert.Equal("Bearer fixture-bearer", http.Headers.Authorization!.ToString());
            Assert.Throws<System.Reflection.TargetInvocationException>(() => build.Invoke(transport, new object[] { call with { IdempotencyKey = null } }));
            for (var length = 0; length < payload.Length; length++) Assert.Throws<ArgumentException>(() => Decode(payload[..length]));
            Assert.Throws<ArgumentException>(() => Decode([.. payload, 0]));
            var changedPayload = payload.ToArray(); changedPayload[0] ^= 1;
            Assert.Throws<ArgumentException>(() => new NativeProgramLifecycleRequest(Decode(changedPayload), signed));
            foreach (var offset in new[] { 1, 7, 17 })
            {
                var changed = signed.ToArray(); changed[offset] ^= 1;
                Assert.Throws<ArgumentException>(() => new NativeProgramLifecycleRequest(value, changed));
            }
            for (var length = 0; length < signed.Length; length++) Assert.Throws<ArgumentException>(() => new NativeProgramLifecycleRequest(value, signed[..length]));
            if (ordinal is 1 or 2)
            {
                foreach (var offset in new[] { 35, 68, payload.Length - 1 })
                {
                    var changed = payload.ToArray(); changed[offset] ^= 1; Assert.Throws<ArgumentException>(() => Decode(changed));
                }
            }
        }
    }

    private const string ProgramOutcomeV3 = "505247330100000000000100010000000700000001000000000000000b000000000000000c000000000000000d000000000000000e00000001000000000000000f0000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000020000000000000003000000000000000400000000000000050000000000000006000000000000000700000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000201111111111111111111111111111111111111111111111111111111111111111000000202222222222222222222222222222222222222222222222222222222222222222000000200000000000000000000000000000000000000000000000000000000000000000";

    [Fact]
    public void NativeSignedBinding()
    {
        using var document = JsonDocument.Parse(File.ReadAllText(FixturePath("native-program-call-v3.json")));
        var root = document.RootElement;
        var payload = HexField(root, "payload_hex"); var nativeCall = NativeProgramCall.Decode(payload);
        Assert.Equal(payload, nativeCall.Encode()); var signed = HexField(root, "signed_activity_hex");
        var method = typeof(ProgramsClient).GetMethod("DecodeSignedCall", System.Reflection.BindingFlags.Static | System.Reflection.BindingFlags.NonPublic)!;
        method.Invoke(null, new object[] { new ProgramCall(nativeCall, new ProtocolAmount("1000"), signed) });
        Assert.Throws<System.Reflection.TargetInvocationException>(() => method.Invoke(null, new object[] { new ProgramCall(nativeCall, new ProtocolAmount("999"), signed) }));
        var changed = nativeCall with { ResponseCapacity = (nativeCall.ResponseCapacity + 1) % 1_048_577 };
        Assert.NotEqual(nativeCall.ResponseCapacity, changed.ResponseCapacity);
        changed.Encode();
        Assert.Throws<System.Reflection.TargetInvocationException>(() => method.Invoke(null, new object[] { new ProgramCall(changed, new ProtocolAmount("1000"), signed) }));
        for (var length = 0; length < payload.Length; length++) Assert.Throws<ArgumentException>(() => NativeProgramCall.Decode(payload[..length]));
    }

    [Fact]
    public void ProgramOutcomeV3VectorDecodes()
    {
        var outcome = LocalVerifier.DecodeProgramReceiptOutcome(Convert.FromHexString(ProgramOutcomeV3), 1);
        Assert.Equal((byte)3, outcome.EncodingVersion);
        Assert.Equal((ushort)1, outcome.AbiVersion);
        Assert.Equal(new UInt128Value(0, 16), outcome.FeeUnits);
        Assert.Equal(Enumerable.Repeat((byte)0x11, 32).ToArray(), outcome.CallGraphRoot);
        Assert.Equal(Enumerable.Repeat((byte)0x22, 32).ToArray(), outcome.TerminalPayloadRoot);
    }
    private static string RepoRoot([CallerFilePath] string sourcePath = "")
        => Path.GetFullPath(Path.Combine(
            Path.GetDirectoryName(sourcePath) ?? throw new InvalidOperationException("no source dir"),
            "..", "..", "..", "..", ".."));

    private static byte[] HexField(JsonElement element, string key)
        => Convert.FromHexString(element.GetProperty(key).GetString()
            ?? throw new InvalidOperationException($"missing {key}"));

    private static string FixturePath(string name) => Path.Combine(
        RepoRoot(), "sdk", "platform", "conformance", "fixtures", name);

}
