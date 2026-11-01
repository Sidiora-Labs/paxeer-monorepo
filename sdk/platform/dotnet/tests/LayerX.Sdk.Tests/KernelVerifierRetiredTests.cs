using System.Runtime.CompilerServices;
using System.Text.Json;
using System.Text.Json.Nodes;
using LayerX.Sdk;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class KernelVerifierRetiredTests
{
    [Fact]
    public void VerificationRungsAreReceiptedAndCommittedOnly()
    {
        Assert.Equal(new[] { "Receipted", "Committed" }, Enum.GetNames<VerificationLevel>());
        Assert.Equal(new[] { VerificationLevel.Receipted, VerificationLevel.Committed },
            Enum.GetValues<VerificationLevel>());

        VerifyBatchInclusion();
        VerifyCheckpoint();
        AuthorizedBatch();

        using var document = LoadFixture("vector-v4-committed.json");
        var vector = document.RootElement.GetProperty("positive");
        var receipt = Receipted(vector);
        Assert.Equal("receipted", receipt.Rung);
        Assert.Equal(VerificationLevel.Receipted, receipt.Level);
        var committed = VerifyCommitted(vector);
        Assert.Equal("committed", committed.Rung);
        Assert.Equal(VerificationLevel.Committed, committed.Level);
    }

    [Fact]
    public void KernelReceiptBytesAreRefusedBySequencerVerifier()
    {
        using var document = LoadFixture("vector-v4-committed.json");
        var vector = document.RootElement.GetProperty("positive");
        var keys = Keys(vector);
        var raw = Hex(vector, "rawTransaction");
        Assert.Equal("receipted", Receipted(vector).Rung);

        var canonical = Convert.FromHexString(
            "0003520100030000002001010101010101010101010101010101010101010101010101010101010101010000000000000009" +
            "0000002002020202020202020202020202020202020202020202020202020202020202020000002003030303030303030303" +
            "0303030303030303030303030303030303030303030300000020080808080808080808080808080808080808080808080808" +
            "0808080808080808000000000000000000000000000000000000000000000001000000200404040404040404040404040404" +
            "0404040404040404040404040404040404040001000000010000000101000000200505050505050505050505050505050505" +
            "0505050505050505050505050505050000000000000000000000000000001900000020060606060606060606060606060606" +
            "0606060606060606060606060606060606000000000000000000000000000000640000000000000000000000000000004b00" +
            "0000000000000100000020070707070707070707070707070707070707070707070707070707070707070700000000000000" +
            "00000000000000000a0000000000000000000000000000002300000020090909090909090909090909090909090909090909" +
            "0909090909090909090909000000200a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a000000" +
            "200b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b00000000000003e80100000040d933bba9" +
            "e0fed523766e49289851174abbce76403c16c574f0a516553d105e34c61ca73bd7cb501d2b6d106c43841c5c6d737733cfdd" +
            "8e885a3d3e308a634c0a");
        Assert.NotEmpty(canonical);
        Assert.Equal(new byte[] { 0, 3, 0x52, 1, 0, 3 }, canonical[..6]);
        RefusedAtDecode(() => LocalVerifier.VerifySequencerReceipt(canonical, raw, keys));

        var view = JsonNode.Parse(vector.GetProperty("receiptJson").GetRawText())!.AsObject();
        view["receipt"] = "0x" + Convert.ToHexString(canonical).ToLowerInvariant();
        var encoded = JsonSerializer.SerializeToUtf8Bytes(view);
        RefusedAtDecode(() => LocalVerifier.VerifySequencerReceipt(encoded, raw, keys));
        using var wrapped = JsonDocument.Parse(encoded);
        RefusedAtDecode(() => LocalVerifier.VerifySequencerReceipt(wrapped.RootElement, raw, keys));
    }

    [Fact]
    public void CommittedVectorV4Verifies()
    {
        using var document = LoadFixture("vector-v4-committed.json");
        var vector = document.RootElement.GetProperty("positive");
        Assert.Equal("committed", vector.GetProperty("expected").GetProperty("rung").GetString());
        var receipt = VerifyCommitted(vector);
        Assert.Equal("committed", receipt.Rung);
        Assert.Equal(VerificationLevel.Committed, receipt.Level);
        Assert.Equal(1U, receipt.Status);
        Assert.Equal(Hex(vector, "signedReceipt"), receipt.CanonicalBytes);
        Assert.Equal(Hex(vector, "receiptDigest"), receipt.ReceiptDigest);

        var refusals = document.RootElement.GetProperty("refusals").EnumerateArray().ToArray();
        Assert.NotEmpty(refusals);
        foreach (var refusal in refusals)
        {
            Assert.Equal("committed-next-commit", refusal.GetProperty("expectedCheck").GetString());
            var inclusion = vector.GetProperty("receiptInclusion").Deserialize<CommittedReceiptInclusion>()!;
            var nextCommit = refusal.GetProperty("nextCommit").Deserialize<CometCommit>()!;
            var error = Assert.Throws<PlatformSdkException>(() => VerifyCommitted(vector,
                inclusion with { NextCommit = nextCommit }));
            Assert.Equal(SdkErrorCode.VerificationFailure, error.Code);
        }
    }

    private static void VerifyBatchInclusion()
    {
        var assembly = typeof(LocalVerifier).Assembly;
        foreach (var name in new[] { "BatchHeader", "SequencerAuthorization", "InclusionKind", "InclusionVerification" })
            Assert.Null(assembly.GetType("LayerX.Sdk." + name));
        foreach (var name in new[] { "DecodeBatchHeader", "VerifyBatchInclusion", "VerifyBatchInclusionAsync" })
            Assert.DoesNotContain(typeof(LocalVerifier).GetMethods(), method => method.Name == name);
    }

    private static void VerifyCheckpoint()
    {
        var assembly = typeof(LocalVerifier).Assembly;
        foreach (var name in new[] { "CheckpointAttestation", "GuarantorKey", "CheckpointCertificate",
            "CheckpointVerificationInput", "CheckpointVerification", "ILocalSignatureVerifier", "BouncyCastleSignatureVerifier" })
            Assert.Null(assembly.GetType("LayerX.Sdk." + name));
        foreach (var name in new[] { "VerifyCheckpoint", "VerifyCheckpointAsync" })
            Assert.DoesNotContain(typeof(LocalVerifier).GetMethods(), method => method.Name == name);
    }

    private static void AuthorizedBatch()
    {
        var assembly = typeof(LocalVerifier).Assembly;
        Assert.Null(assembly.GetType("LayerX.Sdk.AuthorizedBatch"));
        Assert.Null(assembly.GetType("LayerX.Sdk.AuthorizedReceiptBatch"));
        foreach (var name in new[] { "VerifyReceiptAsync", "VerifyReceiptOutcomeAsync" })
            Assert.DoesNotContain(typeof(LocalVerifier).GetMethods(), method => method.Name == name);
    }

    private static SequencerReceiptVerification VerifyCommitted(JsonElement vector,
        CommittedReceiptInclusion? inclusion = null) => Committed.Verify(Receipted(vector),
        vector.GetProperty("blockHeader").Deserialize<CometHeader>()!,
        inclusion ?? vector.GetProperty("receiptInclusion").Deserialize<CommittedReceiptInclusion>()!,
        vector.GetProperty("commit").Deserialize<CometCommit>()!,
        vector.GetProperty("validatorSet").Deserialize<CometValidator[]>()!);

    private static SequencerReceiptVerification Receipted(JsonElement vector) =>
        LocalVerifier.VerifySequencerReceipt(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"), Keys(vector));

    private static void RefusedAtDecode(Action verify)
    {
        var error = Assert.Throws<PlatformSdkException>(verify);
        Assert.Equal(SdkErrorCode.VerificationFailure, error.Code);
        Assert.Equal(ReceiptCheck.Decode, error.ReceiptCheck);
    }

    private static ReceiptKeys Keys(JsonElement vector)
    {
        var key = vector.GetProperty("pinnedKey");
        return new(new[] { new ReceiptKey(Hex(key, "keyId"), Hex(key, "publicKey"),
            key.GetProperty("firstHeight").GetUInt64(), key.GetProperty("lastHeight").GetUInt64()) }, Hex(vector, "prevSeqHash"));
    }

    private static byte[] Hex(JsonElement value, string name) =>
        Convert.FromHexString(value.GetProperty(name).GetString()![2..]);

    private static JsonDocument LoadFixture(string name, [CallerFilePath] string source = "") =>
        JsonDocument.Parse(File.ReadAllText(Path.GetFullPath(Path.Combine(
            Path.GetDirectoryName(source)!, "../../../conformance/fixtures", name))));
}
