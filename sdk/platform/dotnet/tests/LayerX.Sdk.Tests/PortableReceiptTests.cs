using System.Buffers.Binary;
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

public sealed class PortableReceiptTests
{
    [Fact]
    public void PortableReceiptRoundTripsAtReceipted()
    {
        using var document = LoadFixture();
        var cases = document.RootElement.GetProperty("cases").EnumerateArray()
            .Where(vector => vector.GetProperty("vector").GetString() == "V1").ToArray();
        Assert.Equal(4, cases.Length);
        foreach (var vector in cases)
        {
            var keys = Keys(vector);
            var bundle = PortableReceipt.export(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"), keys);
            var bytes = bundle.export();
            using var wire = JsonDocument.Parse(bytes);
            var root = wire.RootElement;
            Assert.Equal("layerx-receipt-proof-v2", root.GetProperty("format").GetString());
            Assert.Equal("receipted", root.GetProperty("verificationLevel").GetString());
            foreach (var field in new[] { "rawTransaction", "signedReceipt", "receiptDigest" })
                Assert.Equal(vector.GetProperty(field).GetString(), root.GetProperty(field).GetString());
            var key = root.GetProperty("receiptKey");
            Assert.Equal(vector.GetProperty("pinnedKey").GetProperty("keyId").GetString(), key.GetProperty("keyId").GetString());
            Assert.Equal(vector.GetProperty("pinnedKey").GetProperty("publicKey").GetString(), key.GetProperty("publicKey").GetString());
            Assert.Equal("0x3e8", key.GetProperty("firstHeight").GetString());
            Assert.Equal("0x7cf", key.GetProperty("lastHeight").GetString());
            foreach (var field in new[] { "batchId", "asset", "previousStateRoot", "resultingStateRoot",
                "blockHeader", "receiptInclusion", "commit", "validatorSet" })
                Assert.False(root.TryGetProperty(field, out _));
            var roundTrip = PortableReceipt.verify(bytes, keys);
            Assert.Equal("receipted", roundTrip.VerificationLevel);
            Assert.Equal(1U, roundTrip.Verification.Status);
            Assert.Equal(Hex(vector, "signedReceipt"), roundTrip.Verification.CanonicalBytes);
            Assert.Equal(Hex(vector, "receiptDigest"), roundTrip.Verification.ReceiptDigest);
            Assert.Equal(Hex(vector.GetProperty("receiptJson"), "returnData"), roundTrip.Verification.ReturnData);
            Assert.Equal(bytes, roundTrip.export());
            Assert.Equal(bytes, PortableReceipt.export(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"), keys).export());
            bytes[0] ^= 1;
            Assert.Equal((byte)'{', roundTrip.export()[0]);
        }

        var revert = document.RootElement.GetProperty("cases").EnumerateArray()
            .Single(vector => vector.GetProperty("vector").GetString() == "V2");
        var reverted = PortableReceipt.export(revert.GetProperty("receiptJson"), Hex(revert, "rawTransaction"), Keys(revert));
        Assert.Equal(0U, PortableReceipt.verify(reverted.export(), Keys(revert)).Verification.Status);
        Assert.NotNull(reverted.Verification.Revert);

        var committed = CreateCommitted(cases[0]);
        var committedBytes = committed.export();
        using var committedWire = JsonDocument.Parse(committedBytes);
        foreach (var field in new[] { "blockHeader", "receiptInclusion", "commit", "validatorSet" })
            Assert.True(committedWire.RootElement.TryGetProperty(field, out _));
        var verified = PortableReceipt.verify(committedBytes, Keys(cases[0]));
        Assert.Equal("committed", verified.VerificationLevel);
        Assert.Equal(Hex(cases[0], "receiptDigest"), verified.Verification.ReceiptDigest);
        Assert.Equal(committedBytes, verified.export());
        var tampered = JsonNode.Parse(committedBytes)!.AsObject();
        tampered["commit"]!["signatures"]![0]!["signature"] = "0x" + new string('0', 128);
        Refused(() => PortableReceipt.verify(JsonSerializer.SerializeToUtf8Bytes(tampered), Keys(cases[0])));
        tampered = JsonNode.Parse(committedBytes)!.AsObject();
        tampered["receiptInclusion"]!["batchProof"]!["siblings"]![0] = "0x" + new string('0', 64);
        Refused(() => PortableReceipt.verify(JsonSerializer.SerializeToUtf8Bytes(tampered), Keys(cases[0])));
    }

    [Fact]
    public void PortableReceiptRefusesVersionOneFormat()
    {
        using var document = LoadFixture();
        var vector = document.RootElement.GetProperty("cases").EnumerateArray()
            .First(value => value.GetProperty("vector").GetString() == "V1");
        var keys = Keys(vector);
        var bytes = PortableReceipt.export(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"), keys).export();
        Assert.Equal("receipted", PortableReceipt.verify(bytes, keys).VerificationLevel);
        void Reject(Action<JsonObject> mutate)
        {
            var changed = JsonNode.Parse(bytes)!.AsObject();
            mutate(changed);
            Refused(() => PortableReceipt.verify(JsonSerializer.SerializeToUtf8Bytes(changed), keys));
        }

        Reject(bundle => bundle["format"] = "layerx-receipt-proof-v1");
        foreach (var field in new[] { "batchId", "asset", "previousStateRoot", "resultingStateRoot" })
        {
            Reject(bundle => bundle[field] = "0x" + new string('0', 64));
            Reject(bundle => bundle[field] = null);
            Reject(bundle => bundle["receiptJson"]![field] = "0x" + new string('0', 64));
        }
        Reject(bundle => bundle["receiptDigest"] = "0x" + new string('0', 64));
        Reject(bundle => bundle["signedReceipt"] = "0x00");
        Reject(bundle => bundle["rawTransaction"] = "0x00");
        Reject(bundle => bundle["receiptKey"]!["publicKey"] = "0x" + new string('0', 64));
        Reject(bundle => bundle["receiptKey"]!["keyId"] = "0x" + new string('0', 64));
        Reject(bundle => bundle["receiptKey"]!["lastHeight"] = "0x7d0");
        Reject(bundle => bundle["receiptJson"]!["status"] = "0x0");
        Reject(bundle => bundle["verificationLevel"] = "committed");
        Reject(bundle => bundle["verificationLevel"] = "sequencer-signed");
        Reject(bundle => bundle["blockHeader"] = new JsonObject());
        foreach (var field in new[] { "rawTransaction", "signedReceipt", "receiptDigest", "receiptKey", "receiptJson" })
            Reject(bundle => bundle.Remove(field));
        Refused(() => PortableReceipt.verify(bytes, keys with { Pinned = Array.Empty<ReceiptKey>() }));
        Refused(() => PortableReceipt.verify(bytes, keys with { PreviousSequenceHash = Enumerable.Repeat((byte)1, 32).ToArray() }));
        Refused(() => PortableReceipt.verify(Encoding.UTF8.GetBytes("{"), keys));
        var duplicate = Encoding.UTF8.GetString(bytes).Replace("{\"format\":", "{\"format\":\"layerx-receipt-proof-v1\",\"format\":", StringComparison.Ordinal);
        Refused(() => PortableReceipt.verify(Encoding.UTF8.GetBytes(duplicate), keys));
    }

    private static PortableReceipt CreateCommitted(JsonElement vector)
    {
        var view = vector.GetProperty("receiptJson");
        var keys = Keys(vector);
        var receipt = LocalVerifier.VerifySequencerReceipt(view, Hex(vector, "rawTransaction"), keys).Receipt;
        var height = Convert.ToUInt64(view.GetProperty("height").GetString()![2..], 16);
        var privateKey = new Ed25519PrivateKeyParameters(Enumerable.Repeat((byte)1, 32).ToArray(), 0);
        var validators = new[] { new CometValidator(privateKey.GeneratePublicKey().GetEncoded(), 10) };
        var batch = Join(Encoding.UTF8.GetBytes("PAXEER/v1/sequence-batch\0"), BigEndian(height),
            BigEndian(receipt.GlobalSequence), BigEndian(receipt.GlobalSequence), receipt.PreviousStateRoot,
            receipt.ResultingStateRoot, SHA256.HashData(receipt.ActivityId), receipt.ContextHash, receipt.SequencerSignature);
        var transactionLeaves = new[] { SHA256.HashData(batch), SHA256.HashData(Hex(vector, "rawTransaction")) };
        var gas = BinaryPrimitives.ReadUInt64BigEndian(receipt.Effects[0].Body);
        var response = Join(Number(1, gas), Blob(2, Encoding.UTF8.GetBytes(view.GetProperty("vmError").GetString()!)),
            Blob(3, Hex(view, "returnData")), Blob(4, Encoding.UTF8.GetBytes(view.GetProperty("txHash").GetString()!)));
        var result = new CometExecutionResult(0, Message(1, Join(Blob(1, Encoding.UTF8.GetBytes("evm")), Message(2, response))),
            (long)gas + 1000, (long)gas);
        var resultLeaves = new[] { Array.Empty<byte>(), Committed.EncodeResult(result) };
        var hash = SHA256.HashData(Encoding.UTF8.GetBytes("portable-committed"));
        var header = new CometHeader(11, 1, "portable-receipt-test", (long)height, new(1700000000, 0),
            new(hash, 1, hash), hash, Root(transactionLeaves), Committed.ValidatorSetHash(validators),
            Committed.ValidatorSetHash(validators), hash, hash, hash, hash,
            SHA256.HashData(validators[0].PublicKey)[..20]);
        var commit = Sign(header, privateKey);
        var next = header with { Height = header.Height + 1, LastBlockId = commit.BlockId, LastResultsHash = Root(resultLeaves) };
        var inclusion = new CommittedReceiptInclusion(batch, new(0, 2, new[] { Leaf(transactionLeaves[1]) }),
            new[] { receipt.ActivityId }, result, new(1, 2, new[] { Leaf(resultLeaves[0]) }), next,
            Sign(next, privateKey), validators);
        return PortableReceipt.export(view, Hex(vector, "rawTransaction"), keys, header, inclusion, commit, validators);
    }

    private static CometCommit Sign(CometHeader header, Ed25519PrivateKeyParameters key)
    {
        var commit = new CometCommit(header.Height, 0,
            new(Committed.HeaderHash(header), 1, SHA256.HashData(Encoding.UTF8.GetBytes("parts"))),
            Array.Empty<CometCommitSignature>());
        var signature = new CometCommitSignature(2, SHA256.HashData(key.GeneratePublicKey().GetEncoded())[..20],
            header.Time, Array.Empty<byte>());
        var signer = new Ed25519Signer();
        signer.Init(true, key);
        var message = Committed.VoteSignBytes(header.ChainId, commit, signature);
        signer.BlockUpdate(message, 0, message.Length);
        return commit with { Signatures = new[] { signature with { Signature = signer.GenerateSignature() } } };
    }

    private static byte[] Root(byte[][] leaves) => SHA256.HashData(Join(new byte[] { 1 }, Leaf(leaves[0]), Leaf(leaves[1])));
    private static byte[] Leaf(byte[] bytes) => SHA256.HashData(Join(new byte[] { 0 }, bytes));
    private static byte[] BigEndian(ulong value)
    {
        var bytes = new byte[8];
        BinaryPrimitives.WriteUInt64BigEndian(bytes, value);
        return bytes;
    }
    private static byte[] Number(ulong tag, ulong value) => value == 0 ? Array.Empty<byte>() : Join(Varint(tag * 8), Varint(value));
    private static byte[] Blob(ulong tag, byte[] value) => value.Length == 0 ? Array.Empty<byte>() : Message(tag, value);
    private static byte[] Message(ulong tag, byte[] value) => Join(Varint(tag * 8 + 2), Varint((ulong)value.Length), value);
    private static byte[] Varint(ulong value)
    {
        var bytes = new List<byte>();
        while (value >= 128) { bytes.Add((byte)((value & 127) | 128)); value >>= 7; }
        bytes.Add((byte)value);
        return bytes.ToArray();
    }
    private static byte[] Join(params byte[][] bytes) => bytes.SelectMany(value => value).ToArray();

    private static void Refused(Action action) =>
        Assert.Equal(SdkErrorCode.VerificationFailure, Assert.Throws<PlatformSdkException>(action).Code);

    private static ReceiptKeys Keys(JsonElement vector)
    {
        var key = vector.GetProperty("pinnedKey");
        return new(new[] { new ReceiptKey(Hex(key, "keyId"), Hex(key, "publicKey"),
            key.GetProperty("firstHeight").GetUInt64(), key.GetProperty("lastHeight").GetUInt64()) }, Hex(vector, "prevSeqHash"));
    }

    private static byte[] Hex(JsonElement value, string name) => Convert.FromHexString(value.GetProperty(name).GetString()![2..]);

    private static JsonDocument LoadFixture([CallerFilePath] string source = "") => JsonDocument.Parse(File.ReadAllText(
        Path.GetFullPath(Path.Combine(Path.GetDirectoryName(source)!, "../../../conformance/fixtures/sequencer-receipt-v1.json"))));
}
