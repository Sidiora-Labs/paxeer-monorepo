using System.Buffers.Binary;
using System.Security.Cryptography;
using System.Text;
using Org.BouncyCastle.Crypto.Parameters;
using Org.BouncyCastle.Crypto.Signers;

namespace LayerX.Sdk;

public sealed record CometTimestamp(long Seconds, int Nanoseconds);
public sealed record CometBlockId(byte[] Hash, uint PartsTotal, byte[] PartsHash);
public sealed record CometHeader(ulong BlockVersion, ulong AppVersion, string ChainId, long Height,
    CometTimestamp Time, CometBlockId LastBlockId, byte[] LastCommitHash, byte[] DataHash,
    byte[] ValidatorsHash, byte[] NextValidatorsHash, byte[] ConsensusHash, byte[] AppHash,
    byte[] LastResultsHash, byte[] EvidenceHash, byte[] ProposerAddress);
public sealed record CometValidator(byte[] PublicKey, long VotingPower);
public sealed record CometCommitSignature(byte BlockIdFlag, byte[] ValidatorAddress,
    CometTimestamp Timestamp, byte[] Signature);
public sealed record CometCommit(long Height, int Round, CometBlockId BlockId,
    IReadOnlyList<CometCommitSignature> Signatures);
public sealed record CometExecutionResult(uint Code, byte[] Data, long GasWanted, long GasUsed);
public sealed record CommittedReceiptInclusion(byte[] BatchTransaction, MerkleProof BatchProof,
    IReadOnlyList<byte[]> ActivityIds, CometExecutionResult Result, MerkleProof ResultProof,
    CometHeader ResultsHeader, CometCommit ResultsCommit, IReadOnlyList<CometValidator> ResultsValidators);

public static class Committed
{
    private static readonly byte[] BatchDomain = Encoding.UTF8.GetBytes("PAXEER/v1/sequence-batch\0");
    private static readonly byte[] SequenceDomain = Encoding.UTF8.GetBytes(GeneratedReceiptContract.SequenceDomain + "\0");

    public static SequencerReceiptVerification Verify(SequencerReceiptVerification receiptVerification,
        CometHeader blockHeader, CommittedReceiptInclusion receiptInclusion, CometCommit commit,
        IReadOnlyList<CometValidator> validatorSet)
    {
        try
        {
            var receipt = receiptVerification.Receipt;
            Require(receiptVerification.Rung == "receipted" && receipt.BatchId.Length == 32 &&
                receipt.BatchId.AsSpan(0, 24).IndexOfAnyExcept((byte)0) < 0);
            var height = BinaryPrimitives.ReadUInt64BigEndian(receipt.BatchId.AsSpan(24));
            Require(blockHeader.Height > 0 && (ulong)blockHeader.Height >= height);
            VerifyCommit(blockHeader, commit, validatorSet);

            var next = receiptInclusion.ResultsHeader;
            Require(blockHeader.Height < long.MaxValue && next.Height == blockHeader.Height + 1 &&
                next.ChainId == blockHeader.ChainId && Equal(next.LastBlockId.Hash, commit.BlockId.Hash) &&
                next.LastBlockId.PartsTotal == commit.BlockId.PartsTotal &&
                Equal(next.LastBlockId.PartsHash, commit.BlockId.PartsHash) &&
                Equal(next.ValidatorsHash, blockHeader.NextValidatorsHash));
            VerifyCommit(next, receiptInclusion.ResultsCommit, receiptInclusion.ResultsValidators);

            var batch = receiptInclusion.BatchTransaction;
            var offset = BatchDomain.Length;
            Require(batch.Length == offset + 24 + 128 + 64 && batch.AsSpan(0, offset).SequenceEqual(BatchDomain));
            var batchHeight = BinaryPrimitives.ReadUInt64BigEndian(batch.AsSpan(offset));
            var from = BinaryPrimitives.ReadUInt64BigEndian(batch.AsSpan(offset + 8));
            var to = BinaryPrimitives.ReadUInt64BigEndian(batch.AsSpan(offset + 16));
            var ids = receiptInclusion.ActivityIds;
            Require(batchHeight == (ulong)blockHeader.Height && from > 0 && to >= from &&
                to - from < int.MaxValue && (ulong)ids.Count == to - from + 1 &&
                receipt.GlobalSequence >= from && receipt.GlobalSequence <= to);
            Require(ids.All(id => id.Length == 32));
            Require(Equal(SHA256.HashData(Join(ids.ToArray())), batch.AsSpan(offset + 88, 32)));
            var position = checked((int)(receipt.GlobalSequence - from));
            Require(Equal(ids[position], receipt.ActivityId));
            var sequenceHash = batch.AsSpan(offset + 24, 32).ToArray();
            for (var i = 0; i < ids.Count; i++)
            {
                if (i == position) Require(Equal(sequenceHash, receipt.PreviousStateRoot));
                var sequence = new byte[8];
                BinaryPrimitives.WriteUInt64BigEndian(sequence, from + (ulong)i);
                sequenceHash = SHA256.HashData(Join(SequenceDomain, sequenceHash, sequence, ids[i]));
                if (i == position) Require(Equal(sequenceHash, receipt.ResultingStateRoot));
            }
            Require(Equal(sequenceHash, batch.AsSpan(offset + 56, 32)));
            Require(receiptInclusion.BatchProof.LeafIndex == 0 &&
                receiptInclusion.BatchProof.LeafCount == (uint)ids.Count + 1);
            VerifyProof(SHA256.HashData(batch), receiptInclusion.BatchProof, blockHeader.DataHash);

            var proof = receiptInclusion.ResultProof;
            Require(receipt.Effects.Count > 0 && receipt.Effects[0].Body.Length >= 108);
            var summary = receipt.Effects[0].Body;
            Require(proof.LeafCount == receiptInclusion.BatchProof.LeafCount &&
                proof.LeafIndex == (uint)position + 1 &&
                proof.LeafIndex == BinaryPrimitives.ReadUInt32BigEndian(summary.AsSpan(24)));
            var result = receiptInclusion.Result;
            Require(result.Code is 0 or 45 && receipt.ResultCode == (result.Code == 0 ? 0 : 1) &&
                receiptVerification.Status == (result.Code == 0 ? 1U : 0U) && result.GasUsed >= 0 &&
                (ulong)result.GasUsed == BinaryPrimitives.ReadUInt64BigEndian(summary));
            VerifyProof(EncodeResult(result), proof, next.LastResultsHash);
            VerifyOutputs(receiptVerification, result.Data);
            return receiptVerification with { Rung = "committed" };
        }
        catch (PlatformSdkException) { throw; }
        catch (Exception error) when (error is ArgumentException or InvalidOperationException or
            OverflowException or IndexOutOfRangeException or NullReferenceException or FormatException)
        {
            throw Failure();
        }
    }

    public static byte[] HeaderHash(CometHeader header)
    {
        Require(header.Height > 0 && !string.IsNullOrEmpty(header.ChainId) &&
            Encoding.UTF8.GetByteCount(header.ChainId) <= 50 && header.ProposerAddress.Length == 20 &&
            header.ValidatorsHash.Length == 32 && header.NextValidatorsHash.Length == 32);
        foreach (var hash in new[] { header.LastCommitHash, header.DataHash, header.ConsensusHash,
            header.LastResultsHash, header.EvidenceHash }) Require(hash.Length is 0 or 32);
        return Root(new[] {
            Join(Number(1, header.BlockVersion), Number(2, header.AppVersion)),
            Bytes(1, Encoding.UTF8.GetBytes(header.ChainId)), Number(1, (ulong)header.Height),
            Timestamp(header.Time), BlockId(header.LastBlockId), Bytes(1, header.LastCommitHash),
            Bytes(1, header.DataHash), Bytes(1, header.ValidatorsHash), Bytes(1, header.NextValidatorsHash),
            Bytes(1, header.ConsensusHash), Bytes(1, header.AppHash), Bytes(1, header.LastResultsHash),
            Bytes(1, header.EvidenceHash), Bytes(1, header.ProposerAddress) });
    }

    public static byte[] ValidatorSetHash(IReadOnlyList<CometValidator> validators)
    {
        Require(validators.Count > 0 && validators.Count <= 10000);
        var addresses = new HashSet<string>();
        foreach (var validator in validators)
        {
            Require(validator.PublicKey.Length == 32 && validator.VotingPower > 0 &&
                addresses.Add(Convert.ToHexString(SHA256.HashData(validator.PublicKey).AsSpan(0, 20))));
        }
        return Root(validators.Select(validator => Join(Bytes(1, Bytes(1, validator.PublicKey)),
            Number(2, (ulong)validator.VotingPower))).ToArray());
    }

    public static byte[] VoteSignBytes(string chainId, CometCommit commit, CometCommitSignature signature)
    {
        Require(commit.Height > 0 && commit.Round >= 0 && signature.BlockIdFlag is 2 or 3);
        var vote = Join(Number(1, 2), Fixed64(2, commit.Height), Fixed64(3, commit.Round),
            signature.BlockIdFlag == 2 ? Message(4, BlockId(commit.BlockId)) : Array.Empty<byte>(),
            Message(5, Timestamp(signature.Timestamp)), Bytes(6, Encoding.UTF8.GetBytes(chainId)));
        return Join(Varint((ulong)vote.Length), vote);
    }

    public static byte[] EncodeResult(CometExecutionResult result) => Join(Number(1, result.Code),
        Bytes(2, result.Data), Number(5, unchecked((ulong)result.GasWanted)),
        Number(6, unchecked((ulong)result.GasUsed)));

    private static void VerifyCommit(CometHeader header, CometCommit commit, IReadOnlyList<CometValidator> validators)
    {
        Require(commit.Height == header.Height && commit.Round >= 0 && commit.BlockId.PartsTotal > 0 &&
            commit.BlockId.PartsHash.Length == 32 && Equal(HeaderHash(header), commit.BlockId.Hash) &&
            Equal(ValidatorSetHash(validators), header.ValidatorsHash) && commit.Signatures.Count == validators.Count);
        System.Numerics.BigInteger total = 0, signed = 0;
        for (var i = 0; i < validators.Count; i++)
        {
            var validator = validators[i];
            var signature = commit.Signatures[i];
            total += validator.VotingPower;
            if (signature.BlockIdFlag == 1)
            {
                Require(signature.Signature.Length == 0 && signature.ValidatorAddress.Length == 0);
                continue;
            }
            Require(signature.BlockIdFlag is 2 or 3 && signature.Signature.Length == 64 &&
                Equal(signature.ValidatorAddress, SHA256.HashData(validator.PublicKey).AsSpan(0, 20)));
            var verifier = new Ed25519Signer();
            verifier.Init(false, new Ed25519PublicKeyParameters(validator.PublicKey, 0));
            var message = VoteSignBytes(header.ChainId, commit, signature);
            verifier.BlockUpdate(message, 0, message.Length);
            Require(verifier.VerifySignature(signature.Signature));
            if (signature.BlockIdFlag == 2) signed += validator.VotingPower;
        }
        Require(signed * 3 > total * 2);
    }

    private static void VerifyOutputs(SequencerReceiptVerification verified, byte[] data)
    {
        var tx = Parse(data);
        var messages = tx.Where(field => field.Tag == 1).ToArray();
        Require(messages.Length == 1);
        var message = Parse(messages[0].Data);
        var response = Parse(Single(message, 2).Data);
        var gas = Optional(response, 1).Value;
        var receipt = verified.Receipt;
        Require(gas == BinaryPrimitives.ReadUInt64BigEndian(receipt.Effects[0].Body) &&
            Equal(Optional(response, 3).Data, verified.ReturnData) &&
            Equal(SHA256.HashData(Optional(response, 2).Data), receipt.Effects[0].Body.AsSpan(68, 32)));
        var hash = Optional(response, 4).Data;
        if (hash.Length != 0) Require(Equal(Hex(Encoding.UTF8.GetString(hash), 32), receipt.ActivityId));
        var logs = response.Where(field => field.Tag == 5).ToArray();
        Require(logs.Length == receipt.Effects.Count - 1);
        for (var i = 0; i < logs.Length; i++)
        {
            var log = Parse(logs[i].Data);
            var topics = log.Where(field => field.Tag == 2).ToArray();
            var effect = receipt.Effects[i + 1].Body;
            Require(effect.Length == 21 + topics.Length * 32 + 32 && topics.Length <= 4 && effect[20] == topics.Length);
            Require(Equal(Hex(Encoding.UTF8.GetString(Single(log, 1).Data), 20), effect.AsSpan(0, 20)));
            for (var j = 0; j < topics.Length; j++)
                Require(Equal(Hex(Encoding.UTF8.GetString(topics[j].Data), 32), effect.AsSpan(21 + j * 32, 32)));
            Require(Equal(SHA256.HashData(Optional(log, 3).Data), effect.AsSpan(21 + topics.Length * 32, 32)));
        }
    }

    private static byte[] Hex(string value, int length)
    {
        Require(value.StartsWith("0x", StringComparison.OrdinalIgnoreCase) && value.Length == 2 + length * 2);
        return Convert.FromHexString(value.AsSpan(2));
    }

    private sealed record Field(int Tag, int Wire, byte[] Data, ulong Value);
    private static List<Field> Parse(byte[] bytes)
    {
        Require(bytes.Length <= 4 * 1024 * 1024);
        var fields = new List<Field>();
        var position = 0;
        ulong Read()
        {
            ulong value = 0;
            for (var shift = 0; shift < 70; shift += 7)
            {
                Require(position < bytes.Length);
                var next = bytes[position++];
                Require(shift != 63 || next <= 1);
                value |= (ulong)(next & 127) << shift;
                if (next < 128) return value;
            }
            throw Failure();
        }
        while (position < bytes.Length)
        {
            var key = Read();
            var wire = (int)(key & 7);
            Require(key >> 3 is > 0 and <= 536870911 && wire is 0 or 2);
            if (wire == 0) fields.Add(new((int)(key >> 3), wire, Array.Empty<byte>(), Read()));
            else
            {
                var length = Read();
                Require(length <= (ulong)(bytes.Length - position));
                fields.Add(new((int)(key >> 3), wire, bytes.AsSpan(position, (int)length).ToArray(), 0));
                position += (int)length;
            }
        }
        return fields;
    }

    private static Field Single(List<Field> fields, int tag)
    {
        var matches = fields.Where(field => field.Tag == tag).ToArray();
        Require(matches.Length == 1 && matches[0].Wire == 2);
        return matches[0];
    }

    private static Field Optional(List<Field> fields, int tag)
    {
        var matches = fields.Where(field => field.Tag == tag).ToArray();
        Require(matches.Length <= 1 && (matches.Length == 0 || matches[0].Wire == (tag == 1 ? 0 : 2)));
        return matches.Length == 0 ? new(tag, tag == 1 ? 0 : 2, Array.Empty<byte>(), 0) : matches[0];
    }

    private static void VerifyProof(byte[] leaf, MerkleProof proof, byte[] expected)
    {
        Require(proof.LeafCount > 0 && proof.LeafIndex < proof.LeafCount &&
            proof.Siblings.Count <= 32 && proof.Siblings.All(sibling => sibling.Length == 32) && expected.Length == 32);
        var cursor = proof.Siblings.Count;
        byte[] Reconstruct(uint index, uint count)
        {
            if (count == 1) { Require(cursor == 0); return Leaf(leaf); }
            Require(cursor > 0);
            var sibling = proof.Siblings[--cursor];
            var split = Split(count);
            return index < split ? Inner(Reconstruct(index, split), sibling) :
                Inner(sibling, Reconstruct(index - split, count - split));
        }
        Require(Equal(Reconstruct(proof.LeafIndex, proof.LeafCount), expected) && cursor == 0);
    }

    private static uint Split(uint count)
    {
        uint split = 1;
        while (split < count - split) split <<= 1;
        return split;
    }

    private static byte[] Root(IReadOnlyList<byte[]> leaves)
    {
        if (leaves.Count == 0) return SHA256.HashData(Array.Empty<byte>());
        if (leaves.Count == 1) return Leaf(leaves[0]);
        var split = (int)Split((uint)leaves.Count);
        return Inner(Root(leaves.Take(split).ToArray()), Root(leaves.Skip(split).ToArray()));
    }

    private static byte[] Leaf(byte[] bytes) => SHA256.HashData(Join(new byte[] { 0 }, bytes));
    private static byte[] Inner(byte[] left, byte[] right) => SHA256.HashData(Join(new byte[] { 1 }, left, right));
    private static byte[] BlockId(CometBlockId id) => Join(Bytes(1, id.Hash),
        Message(2, Join(Number(1, id.PartsTotal), Bytes(2, id.PartsHash))));
    private static byte[] Timestamp(CometTimestamp time)
    {
        Require(time.Seconds is >= -62135596800 and <= 253402300799 && time.Nanoseconds is >= 0 and < 1000000000);
        return Join(Number(1, unchecked((ulong)time.Seconds)), Number(2, (ulong)time.Nanoseconds));
    }
    private static byte[] Fixed64(int tag, long value)
    {
        if (value == 0) return Array.Empty<byte>();
        var bytes = new byte[9];
        bytes[0] = (byte)(tag * 8 + 1);
        BinaryPrimitives.WriteInt64LittleEndian(bytes.AsSpan(1), value);
        return bytes;
    }
    private static byte[] Number(int tag, ulong value) => value == 0 ? Array.Empty<byte>() : Join(Varint((ulong)tag * 8), Varint(value));
    private static byte[] Bytes(int tag, byte[] value) => value.Length == 0 ? Array.Empty<byte>() : Message(tag, value);
    private static byte[] Message(int tag, byte[] value) => Join(Varint((ulong)tag * 8 + 2), Varint((ulong)value.Length), value);
    private static byte[] Varint(ulong value)
    {
        var bytes = new List<byte>();
        while (value >= 128) { bytes.Add((byte)((value & 127) | 128)); value >>= 7; }
        bytes.Add((byte)value);
        return bytes.ToArray();
    }
    private static byte[] Join(params byte[][] bytes) => bytes.SelectMany(value => value).ToArray();
    private static bool Equal(ReadOnlySpan<byte> left, ReadOnlySpan<byte> right) => CryptographicOperations.FixedTimeEquals(left, right);
    private static void Require(bool condition) { if (!condition) throw Failure(); }
    private static PlatformSdkException Failure() => new(SdkErrorCode.VerificationFailure, RetryClass.Never);
}
