using System.Runtime.CompilerServices;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using LayerX.Sdk;
using Org.BouncyCastle.Crypto.Parameters;
using Org.BouncyCastle.Crypto.Signers;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class SequencerReceiptTests
{
    [Fact]
    public void SequencerReceiptV1CasesVerifyReceipted()
    {
        using var document = LoadFixture();
        var cases = Cases(document, "V1");
        Assert.Equal(4, cases.Length);
        foreach (var vector in cases)
        {
            var keys = Keys(vector);
            var view = vector.GetProperty("receiptJson");
            var raw = Hex(vector, "rawTransaction");
            var result = LocalVerifier.VerifySequencerReceipt(JsonSerializer.SerializeToUtf8Bytes(view), raw, keys);
            Assert.Equal(vector.GetProperty("expected").GetProperty("rung").GetString(), result.Rung);
            Assert.Equal("receipted", result.Rung);
            Assert.Equal(1U, result.Status);
            Assert.Equal(0, result.Receipt.ResultCode);
            Assert.Equal((ushort)2, result.Receipt.ProtocolVersion);
            Assert.Equal(Hex(vector, "signedReceipt"), result.CanonicalBytes);
            Assert.Equal(Hex(vector, "receiptDigest"), result.ReceiptDigest);
            Assert.Equal(Hex(view, "returnData"), result.ReturnData);
            Assert.Equal(Hex(view, "keyId"), result.Receipt.ContextHash);
            Assert.Null(result.Revert);

            var primary = keys.Pinned[0];
            var unrelated = primary with { KeyId = SHA256.HashData(primary.KeyId) };
            var standby = keys with { Pinned = new[] { unrelated, primary } };
            Assert.Equal(result.ReceiptDigest, LocalVerifier.VerifySequencerReceipt(view, raw, standby).ReceiptDigest);
            var height = Convert.ToUInt64(view.GetProperty("height").GetString()![2..], 16);
            var boundary = keys with { Pinned = new[] { primary with { FirstHeight = height, LastHeight = height } } };
            Assert.Equal(result.ReceiptDigest, LocalVerifier.VerifySequencerReceipt(view, raw, boundary).ReceiptDigest);
            var tooEarly = keys with { Pinned = new[] { primary with { FirstHeight = height + 1 } } };
            AssertCheck(ReceiptCheck.KeyHeight, () => LocalVerifier.VerifySequencerReceipt(view, raw, tooEarly));
            var tooLate = keys with { Pinned = new[] { primary with { LastHeight = height - 1 } } };
            AssertCheck(ReceiptCheck.KeyHeight, () => LocalVerifier.VerifySequencerReceipt(view, raw, tooLate));

            var v3 = JsonNode.Parse(view.GetRawText())!.AsObject();
            var canonical = Hex(vector, "signedReceipt");
            canonical[1] = 3;
            canonical[5] = 3;
            var privateKey = new Ed25519PrivateKeyParameters(RandomNumberGenerator.GetBytes(32), 0);
            var unsigned = canonical[..^68];
            unsigned[^1] = 0;
            var digest = SHA256.HashData(Encoding.UTF8.GetBytes("LXP/v1/receipt\0").Concat(unsigned).ToArray());
            var signer = new Ed25519Signer();
            signer.Init(true, privateKey);
            signer.BlockUpdate(digest, 0, digest.Length);
            signer.GenerateSignature().CopyTo(canonical, canonical.Length - 64);
            v3["receipt"] = "0x" + Convert.ToHexString(canonical);
            var v3Keys = keys with { Pinned = new[] { primary with { PublicKey = privateKey.GeneratePublicKey().GetEncoded() } } };
            var v3Result = LocalVerifier.VerifySequencerReceipt(JsonSerializer.SerializeToUtf8Bytes(v3), raw, v3Keys);
            Assert.Equal((ushort)3, v3Result.Receipt.ProtocolVersion);
            Assert.Equal("receipted", v3Result.Rung);
            Assert.Equal(1U, v3Result.Status);
            Assert.Equal(digest, v3Result.ReceiptDigest);
        }
    }

    [Fact]
    public void SequencerReceiptV2RevertDecodesReason()
    {
        using var document = LoadFixture();
        var vector = Assert.Single(Cases(document, "V2"));
        var result = LocalVerifier.VerifySequencerReceipt(JsonSerializer.SerializeToUtf8Bytes(vector.GetProperty("receiptJson")),
            Hex(vector, "rawTransaction"), Keys(vector));
        Assert.Equal("receipted", result.Rung);
        Assert.Equal(0U, result.Status);
        Assert.Equal(1, result.Receipt.ResultCode);
        Assert.Equal(Hex(vector, "signedReceipt"), result.CanonicalBytes);
        Assert.Equal(Hex(vector, "receiptDigest"), result.ReceiptDigest);
        var revert = Assert.IsType<SequencerRevert>(result.Revert);
        Assert.Equal(vector.GetProperty("lxpCode").GetString(), revert.Code);
        Assert.Equal("LXP_INSUFFICIENT_BALANCE: insufficient balance", revert.Reason);
        Assert.Equal("execution reverted", revert.VmError);
        Assert.Equal(Hex(vector.GetProperty("receiptJson"), "returnData"), revert.ReturnData);
        Assert.Equal(revert.ReturnData, result.ReturnData);
        Assert.Empty(result.Receipt.Effects.Skip(1));
    }

    [Fact]
    public void SequencerReceiptV3RefusalsNameTheirCheck()
    {
        using var document = LoadFixture();
        var cases = Cases(document, "V3");
        Assert.Equal(6, cases.Length);
        var checks = new Dictionary<string, ReceiptCheck>
        {
            ["R3: keyId is not in the pinned ReceiptKeys"] = ReceiptCheck.KeyId,
            ["R3: batch_id height is outside the pinned key range"] = ReceiptCheck.KeyHeight,
            ["R4: activity_id is not keccak256 of the raw transaction"] = ReceiptCheck.RawTransaction,
            ["R6: seqHash does not chain prevSeqHash, seq and activity_id"] = ReceiptCheck.SequenceHash,
            ["sequencer receipt: JSON view differs from receipt bytes"] = ReceiptCheck.Logs,
            ["layerx codec: non-canonical"] = ReceiptCheck.CanonicalEncoding,
        };
        foreach (var vector in cases)
        {
            var expected = checks[vector.GetProperty("expected").GetProperty("check").GetString()!];
            AssertCheck(expected, () => LocalVerifier.VerifySequencerReceipt(
                JsonSerializer.SerializeToUtf8Bytes(vector.GetProperty("receiptJson")), Hex(vector, "rawTransaction"), Keys(vector)));
        }

        var success = Cases(document, "V1")[1];
        var raw = Hex(success, "rawTransaction");
        var keys = Keys(success);
        var original = JsonNode.Parse(success.GetProperty("receiptJson").GetRawText())!.AsObject();
        void Refuse(ReceiptCheck check, Action<JsonObject> mutate)
        {
            var view = original.DeepClone().AsObject();
            mutate(view);
            AssertCheck(check, () => LocalVerifier.VerifySequencerReceipt(JsonSerializer.SerializeToUtf8Bytes(view), raw, keys));
        }
        Refuse(ReceiptCheck.Fields, view => view["status"] = "0x0");
        Refuse(ReceiptCheck.Fields, view => view["gasUsed"] = "0x1");
        Refuse(ReceiptCheck.ReturnData, view => view["returnData"] = "0x");
        Refuse(ReceiptCheck.Logs, view => view["logs"]![0]!["topics"]![0] = "0x" + new string('0', 64));
        Refuse(ReceiptCheck.Signature, view =>
        {
            var bytes = Hex(success, "signedReceipt");
            bytes[^1] ^= 1;
            view["receipt"] = "0x" + Convert.ToHexString(bytes);
        });
        Refuse(ReceiptCheck.ReceiptVersion, view =>
        {
            var bytes = Hex(success, "signedReceipt");
            bytes[1] = 1;
            view["receipt"] = "0x" + Convert.ToHexString(bytes);
        });
        Refuse(ReceiptCheck.ActivityId, view =>
        {
            var bytes = Hex(success, "signedReceipt");
            Array.Clear(bytes, 10, 32);
            view["receipt"] = "0x" + Convert.ToHexString(bytes);
        });
        AssertCheck(ReceiptCheck.Decode, () => LocalVerifier.VerifySequencerReceipt(Encoding.UTF8.GetBytes("{"), raw, keys));
        AssertCheck(ReceiptCheck.SequenceHash, () => LocalVerifier.VerifySequencerReceipt(
            success.GetProperty("receiptJson"), raw, keys with { PreviousSequenceHash = new byte[32] }));
    }

    private static void AssertCheck(ReceiptCheck check, Action verify)
    {
        var error = Assert.Throws<PlatformSdkException>(verify);
        Assert.Equal(SdkErrorCode.VerificationFailure, error.Code);
        Assert.Equal(check, error.ReceiptCheck);
    }

    private static ReceiptKeys Keys(JsonElement vector)
    {
        var key = vector.GetProperty("pinnedKey");
        return new(new[] { new ReceiptKey(Hex(key, "keyId"), Hex(key, "publicKey"),
            key.GetProperty("firstHeight").GetUInt64(), key.GetProperty("lastHeight").GetUInt64()) }, Hex(vector, "prevSeqHash"));
    }

    private static JsonElement[] Cases(JsonDocument document, string vector) => document.RootElement.GetProperty("cases")
        .EnumerateArray().Where(value => value.GetProperty("vector").GetString() == vector).ToArray();

    private static byte[] Hex(JsonElement value, string name) => Convert.FromHexString(value.GetProperty(name).GetString()![2..]);

    private static JsonDocument LoadFixture([CallerFilePath] string source = "") => JsonDocument.Parse(File.ReadAllText(
        Path.GetFullPath(Path.Combine(Path.GetDirectoryName(source)!, "../../../conformance/fixtures/sequencer-receipt-v1.json"))));
}
