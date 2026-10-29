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
