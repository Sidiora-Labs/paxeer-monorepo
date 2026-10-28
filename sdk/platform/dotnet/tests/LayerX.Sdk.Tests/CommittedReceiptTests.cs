using System.Buffers.Binary;
using System.Runtime.CompilerServices;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using LayerX.Sdk;
using Org.BouncyCastle.Crypto.Parameters;
using Org.BouncyCastle.Crypto.Signers;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class CommittedReceiptTests
{
    private sealed record Fixture(SequencerReceiptVerification Receipt, CometHeader Header,
        CommittedReceiptInclusion Inclusion, CometCommit Commit, CometValidator[] Validators,
        Ed25519PrivateKeyParameters[] Keys, byte[][] ResultLeaves);

    [Fact]
    public void CommittedRungAcceptsTwoThirdsCommit()
    {
        AssertCometVectors();
        using var document = Load("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
        foreach (var vector in document.RootElement.GetProperty("cases").EnumerateArray()
            .Where(value => value.GetProperty("vector").GetString() is "V1" or "V2"))
        {
            var fixture = Create(vector.GetProperty("id").GetString()!);
            var verified = Verify(fixture);
            Assert.Equal("committed", verified.Rung);
            Assert.Equal(fixture.Receipt.Status, verified.Status);
            Assert.Equal(fixture.Receipt.ReceiptDigest, verified.ReceiptDigest);
            Assert.Equal(fixture.Receipt.CanonicalBytes, verified.CanonicalBytes);
            Assert.Equal(fixture.Receipt.ReturnData, verified.ReturnData);
            Assert.Equal(fixture.Receipt.Revert, verified.Revert);
            Assert.Equal("receipted", fixture.Receipt.Rung);
        }
        var weighted = Create(powers: new long[] { 10, 1, 1, 1 }, signers: 1);
        Assert.Equal("committed", Verify(weighted).Rung);
    }

    [Fact]
    public void CommittedRungRefusesInsufficientPower()
    {
        var fixture = Create();
        Assert.Equal("committed", Verify(fixture).Rung);
        Refused(fixture with { Commit = Sign(fixture.Header, fixture.Keys, 2) });
        Refused(Create(powers: new long[] { 2, 2, 1, 1 }, signers: 2));
        Refused(Create(powers: new long[] { 1, 1, 1, 10 }, signers: 3));
        Refused(fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                ResultsCommit = Sign(fixture.Inclusion.ResultsHeader, fixture.Keys, 2)
            }
        });

        var signatures = fixture.Commit.Signatures.ToArray();
        signatures[0] = signatures[0] with { Signature = signatures[0].Signature.ToArray() };
        signatures[0].Signature[0] ^= 1;
        Refused(fixture with { Commit = fixture.Commit with { Signatures = signatures } });
        signatures = fixture.Commit.Signatures.ToArray();
        signatures[1] = signatures[0];
        Refused(fixture with { Commit = fixture.Commit with { Signatures = signatures } });
        signatures = fixture.Commit.Signatures.ToArray();
        var nil = signatures[0] with { BlockIdFlag = 3 };
        signatures[0] = nil with
        {
            Signature = SignBytes(fixture.Keys[0],
            Committed.VoteSignBytes(fixture.Header.ChainId, fixture.Commit, nil))
        };
        Refused(fixture with { Commit = fixture.Commit with { Signatures = signatures } });
        Refused(fixture with { Header = fixture.Header with { ChainId = "another-chain" } });
        var changedValidators = fixture.Validators.ToArray();
        changedValidators[0] = changedValidators[0] with { VotingPower = 11 };
        Refused(fixture with { Validators = changedValidators });

        var earlier = fixture.Header with { Height = fixture.Header.Height - 1 };
        Refused(fixture with { Header = earlier, Commit = Sign(earlier, fixture.Keys, 3) });
    }

    [Fact]
    public void CommittedRungRefusesTamperedInclusion()
    {
        var fixture = Create();
        Assert.Equal("committed", Verify(fixture).Rung);
        var proof = fixture.Inclusion.ResultProof;
        Assert.NotEmpty(proof.Siblings);
        var siblings = proof.Siblings.Select(bytes => bytes.ToArray()).ToArray();
        siblings[0][0] ^= 1;
        Refused(fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                ResultProof = proof with { Siblings = siblings }
            }
        });
        proof = fixture.Inclusion.BatchProof;
        siblings = proof.Siblings.Select(bytes => bytes.ToArray()).ToArray();
        siblings[0][0] ^= 1;
        Refused(fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                BatchProof = proof with { Siblings = siblings }
            }
        });
        Refused(fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                ResultProof = fixture.Inclusion.ResultProof with { LeafIndex = 0 }
            }
        });
        Refused(fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                ResultProof = fixture.Inclusion.ResultProof with { Siblings = Array.Empty<byte[]>() }
            }
        });
        Refused(fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                BatchProof = fixture.Inclusion.BatchProof with { LeafIndex = 1 }
            }
        });
        var result = fixture.Inclusion.Result;
        Refused(fixture with { Inclusion = fixture.Inclusion with { Result = result with { GasUsed = result.GasUsed + 1 } } });
        Refused(Reanchor(fixture, result with { Code = 45 }));
        Refused(Reanchor(fixture, result with { Code = 7 }));
        Refused(Reanchor(fixture, result with { Data = Array.Empty<byte>() }));
        var data = result.Data.ToArray();
        data[^1] ^= 1;
        Refused(Reanchor(fixture, result with { Data = data }));
        var next = fixture.Inclusion.ResultsHeader with
        {
            LastBlockId = fixture.Inclusion.ResultsHeader.LastBlockId with
            {
                Hash = Hash("unrelated-block")
            }
        };
        Refused(fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                ResultsHeader = next,
                ResultsCommit = Sign(next, fixture.Keys, 3)
            }
        });
        var ids = fixture.Inclusion.ActivityIds.Select(bytes => bytes.ToArray()).ToArray();
        ids[0][0] ^= 1;
        Refused(fixture with { Inclusion = fixture.Inclusion with { ActivityIds = ids } });
    }

    private static SequencerReceiptVerification Verify(Fixture fixture) => Committed.Verify(
        fixture.Receipt, fixture.Header, fixture.Inclusion, fixture.Commit, fixture.Validators);

    private static void Refused(Fixture fixture)
    {
        var error = Assert.Throws<PlatformSdkException>(() => Verify(fixture));
        Assert.Equal(SdkErrorCode.VerificationFailure, error.Code);
    }

    private static Fixture Create(string id = "V1-pax-transfer", long[]? powers = null, int signers = 3)
    {
        using var document = Load("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
        var vectors = document.RootElement.GetProperty("cases").EnumerateArray()
            .Where(value => value.GetProperty("vector").GetString() is "V1" or "V2").ToArray();
        var vector = vectors.Single(value => value.GetProperty("id").GetString() == id);
        var view = vector.GetProperty("receiptJson");
        var height = Quantity(view, "height");
        var batchVectors = vectors.Where(value => Quantity(value.GetProperty("receiptJson"), "height") == height).ToArray();
        var receipts = batchVectors.Select(Receipted).ToArray();
        var selected = Array.FindIndex(batchVectors, value => value.GetProperty("id").GetString() == id);
        var receipt = receipts[selected];
        var keys = Enumerable.Range(1, 4).Select(index => new Ed25519PrivateKeyParameters(
            Enumerable.Repeat((byte)index, 32).ToArray(), 0)).ToArray();
        var validators = keys.Select((key, index) => new CometValidator(key.GeneratePublicKey().GetEncoded(),
            powers?[index] ?? 10)).ToArray();
        var ids = receipts.Select(value => value.Receipt.ActivityId).ToArray();
        var unsignedBatch = Join(Encoding.UTF8.GetBytes("PAXEER/v1/sequence-batch\0"),
            BigEndian(height), BigEndian(receipts[0].Receipt.GlobalSequence), BigEndian(receipts[^1].Receipt.GlobalSequence),
            receipts[0].Receipt.PreviousStateRoot, receipts[^1].Receipt.ResultingStateRoot, SHA256.HashData(Join(ids)));
        var chain = "layerx-committed-test";
        var domainLength = Encoding.UTF8.GetByteCount("PAXEER/v1/sequence-batch\0");
        var digest = SHA256.HashData(Join(unsignedBatch[..domainLength], Encoding.UTF8.GetBytes(chain), unsignedBatch[domainLength..]));
        var batch = Join(unsignedBatch, receipt.Receipt.ContextHash, SignBytes(new Ed25519PrivateKeyParameters(
            Enumerable.Repeat((byte)0x5e, 32).ToArray(), 0), digest));
        var txLeaves = new[] { SHA256.HashData(batch) }.Concat(batchVectors.Select(value => SHA256.HashData(Hex(value, "rawTransaction")))).ToArray();
        var results = batchVectors.Select(value => Result(value.GetProperty("receiptJson"))).ToArray();
        var resultLeaves = new[] { Array.Empty<byte>() }.Concat(results.Select(EncodeResult)).ToArray();
        var header = new CometHeader(11, 1, chain, (long)height, new(1700000000, 123456789),
            new(Hash("previous"), 1, Hash("previous-parts")), Hash("last-commit"), Root(txLeaves),
            Committed.ValidatorSetHash(validators), Committed.ValidatorSetHash(validators), Hash("consensus"),
            Hash("application"), Hash("previous-results"), Hash("evidence"),
            SHA256.HashData(validators[0].PublicKey)[..20]);
        var commit = Sign(header, keys, signers);
        var next = header with
        {
            Height = header.Height + 1,
            LastBlockId = commit.BlockId,
            LastResultsHash = Root(resultLeaves),
            Time = new(1700000001, 123456789)
        };
        var inclusion = new CommittedReceiptInclusion(batch, Proof(txLeaves, 0), ids, results[selected],
            Proof(resultLeaves, selected + 1), next, Sign(next, keys, 3), validators);
        return new(receipt, header, inclusion, commit, validators, keys, resultLeaves);
    }

    private static Fixture Reanchor(Fixture fixture, CometExecutionResult result)
    {
        var leaves = fixture.ResultLeaves.ToArray();
        var index = (int)fixture.Inclusion.ResultProof.LeafIndex;
        leaves[index] = EncodeResult(result);
        var next = fixture.Inclusion.ResultsHeader with { LastResultsHash = Root(leaves) };
        return fixture with
        {
            Inclusion = fixture.Inclusion with
            {
                Result = result,
                ResultProof = Proof(leaves, index),
                ResultsHeader = next,
                ResultsCommit = Sign(next, fixture.Keys, 3)
            }
        };
    }

    private static SequencerReceiptVerification Receipted(JsonElement vector)
    {
        var key = vector.GetProperty("pinnedKey");
        var keys = new ReceiptKeys(new[] { new ReceiptKey(Hex(key, "keyId"), Hex(key, "publicKey"),
            key.GetProperty("firstHeight").GetUInt64(), key.GetProperty("lastHeight").GetUInt64()) }, Hex(vector, "prevSeqHash"));
        return LocalVerifier.VerifySequencerReceipt(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"), keys);
    }

    private static CometExecutionResult Result(JsonElement view)
    {
        var gas = Quantity(view, "gasUsed");
        var logs = view.GetProperty("logs").EnumerateArray().Select(log => Message(5, Join(
            Text(1, log.GetProperty("address").GetString()!),
            Join(log.GetProperty("topics").EnumerateArray().Select(topic => Text(2, topic.GetString()!)).ToArray()),
            Blob(3, Hex(log, "data"))))).ToArray();
        var response = Join(Number(1, gas), Text(2, view.GetProperty("vmError").GetString()!),
            Blob(3, Hex(view, "returnData")), Text(4, view.GetProperty("txHash").GetString()!), Join(logs));
        var data = Message(1, Join(Text(1, "evm"), Message(2, response)));
        return new(Quantity(view, "status") == 1 ? 0U : 45U, data, (long)gas + 1000, (long)gas);
    }

    private static CometCommit Sign(CometHeader header, Ed25519PrivateKeyParameters[] keys, int signers)
    {
        var commit = new CometCommit(header.Height, 2, new(Committed.HeaderHash(header), 3, Hash("parts")),
            Array.Empty<CometCommitSignature>());
        var signatures = keys.Select((key, index) =>
        {
            if (index >= signers) return new CometCommitSignature(1, Array.Empty<byte>(), new(0, 0), Array.Empty<byte>());
            var signature = new CometCommitSignature(2, SHA256.HashData(key.GeneratePublicKey().GetEncoded())[..20],
                new(1700000000, 124456789 + index * 1000000), Array.Empty<byte>());
            return signature with { Signature = SignBytes(key, Committed.VoteSignBytes(header.ChainId, commit, signature)) };
        }).ToArray();
        return commit with { Signatures = signatures };
    }

    private static byte[] SignBytes(Ed25519PrivateKeyParameters key, byte[] bytes)
    {
        var signer = new Ed25519Signer();
        signer.Init(true, key);
        signer.BlockUpdate(bytes, 0, bytes.Length);
        return signer.GenerateSignature();
    }

    private static void AssertCometVectors()
    {
        using var document = Load("consensus/types/testdata/light_commit_v1.json");
        var vector = document.RootElement;
        var h = vector.GetProperty("header");
        CometBlockId Id(JsonElement value)
        {
            var parts = value.GetProperty("parts");
            return new(Convert.FromHexString(value.GetProperty("hash").GetString()!), parts.GetProperty("total").GetUInt32(),
                Convert.FromHexString(parts.GetProperty("hash").GetString()!));
        }
        byte[] H(string name) => Convert.FromHexString(h.GetProperty(name).GetString()!);
        var header = new CometHeader(11, 1, h.GetProperty("chain_id").GetString()!, 42, new(1700000000, 123456789),
            Id(h.GetProperty("last_block_id")), H("last_commit_hash"), H("data_hash"), H("validators_hash"),
            H("next_validators_hash"), H("consensus_hash"), H("app_hash"), H("last_results_hash"), H("evidence_hash"), H("proposer_address"));
        Assert.Equal(Convert.FromHexString(vector.GetProperty("header_hash").GetString()!), Committed.HeaderHash(header));
        var validators = vector.GetProperty("validators").EnumerateArray().Select(value => new CometValidator(
            Convert.FromHexString(value.GetProperty("public_key").GetString()!), value.GetProperty("voting_power").GetInt64())).ToArray();
        Assert.Equal(header.ValidatorsHash, Committed.ValidatorSetHash(validators));
        var c = vector.GetProperty("commit");
        var commit = new CometCommit(42, 2, Id(c.GetProperty("block_id")), Array.Empty<CometCommitSignature>());
        foreach (var s in c.GetProperty("signatures").EnumerateArray())
        {
            var index = s.GetProperty("validator_index").GetInt32();
            var signature = new CometCommitSignature(2, Convert.FromHexString(s.GetProperty("validator_address").GetString()!),
                new(1700000000, 124456789 + index * 1000000), Convert.FromHexString(s.GetProperty("signature").GetString()!));
            var bytes = Committed.VoteSignBytes(header.ChainId, commit, signature);
            Assert.Equal(Convert.FromHexString(s.GetProperty("vote_sign_bytes").GetString()!), bytes);
            var verifier = new Ed25519Signer();
            verifier.Init(false, new Ed25519PublicKeyParameters(validators[index].PublicKey, 0));
            verifier.BlockUpdate(bytes, 0, bytes.Length);
            Assert.True(verifier.VerifySignature(signature.Signature));
        }
        using var results = Load("consensus/types/testdata/results_hash_v1.json");
        foreach (var result in results.RootElement.GetProperty("results").EnumerateArray())
        {
            var value = new CometExecutionResult(result.GetProperty("code").GetUInt32(),
                Convert.FromHexString(result.GetProperty("data").GetString()!), result.GetProperty("gas_wanted").GetInt64(),
                result.GetProperty("gas_used").GetInt64());
            Assert.Equal(Convert.FromHexString(result.GetProperty("protobuf").GetString()!), Committed.EncodeResult(value));
        }
    }

    private static MerkleProof Proof(byte[][] leaves, int index)
    {
        if (leaves.Length == 1) return new((uint)index, 1, Array.Empty<byte[]>());
        var split = Split(leaves.Length);
        var child = index < split ? Proof(leaves[..split], index) : Proof(leaves[split..], index - split);
        return new((uint)index, (uint)leaves.Length,
            child.Siblings.Append(Root(index < split ? leaves[split..] : leaves[..split])).ToArray());
    }

    private static byte[] Root(byte[][] leaves)
    {
        if (leaves.Length == 1) return SHA256.HashData(Join(new byte[] { 0 }, leaves[0]));
        var split = Split(leaves.Length);
        return SHA256.HashData(Join(new byte[] { 1 }, Root(leaves[..split]), Root(leaves[split..])));
    }
    private static int Split(int count) { var split = 1; while (split * 2 < count) split *= 2; return split; }
    private static byte[] EncodeResult(CometExecutionResult result) => Join(Number(1, result.Code), Blob(2, result.Data),
        Number(5, unchecked((ulong)result.GasWanted)), Number(6, unchecked((ulong)result.GasUsed)));
    private static byte[] Number(int tag, ulong value) => value == 0 ? Array.Empty<byte>() : Join(Varint((ulong)tag * 8), Varint(value));
    private static byte[] Text(int tag, string value) => Blob(tag, Encoding.UTF8.GetBytes(value));
    private static byte[] Blob(int tag, byte[] value) => value.Length == 0 ? Array.Empty<byte>() : Message(tag, value);
    private static byte[] Message(int tag, byte[] value) => Join(Varint((ulong)tag * 8 + 2), Varint((ulong)value.Length), value);
    private static byte[] Varint(ulong value)
    {
        var bytes = new List<byte>();
        while (value > 127) { bytes.Add((byte)(value & 127 | 128)); value >>= 7; }
        bytes.Add((byte)value);
        return bytes.ToArray();
    }
    private static byte[] BigEndian(ulong value) { var bytes = new byte[8]; BinaryPrimitives.WriteUInt64BigEndian(bytes, value); return bytes; }
    private static byte[] Join(params byte[][] values) => values.SelectMany(value => value).ToArray();
    private static byte[] Hash(string value) => SHA256.HashData(Encoding.UTF8.GetBytes(value));
    private static ulong Quantity(JsonElement value, string name) => Convert.ToUInt64(value.GetProperty(name).GetString()![2..], 16);
    private static byte[] Hex(JsonElement value, string name) => Convert.FromHexString(value.GetProperty(name).GetString()![2..]);
    private static JsonDocument Load(string relative, [CallerFilePath] string source = "") => JsonDocument.Parse(File.ReadAllText(
        Path.GetFullPath(Path.Combine(Path.GetDirectoryName(source)!, "../../../../..", relative))));
}
