using System.Buffers.Binary;
using System.Globalization;
using System.Numerics;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.RegularExpressions;
using Org.BouncyCastle.Crypto.Digests;
using Org.BouncyCastle.Crypto.Parameters;
using Org.BouncyCastle.Crypto.Signers;

namespace LayerX.Sdk;

[JsonConverter(typeof(CometTimestampConverter))]
public sealed record CometTimestamp(long Seconds, int Nanoseconds);
public sealed record CometBlockId(byte[] Hash,
    [property: JsonPropertyName("PartSetTotal")] uint PartsTotal,
    [property: JsonPropertyName("PartSetHash")] byte[] PartsHash);
public sealed record CometHeader(
    [property: JsonPropertyName("VersionBlock")] ulong BlockVersion,
    [property: JsonPropertyName("VersionApp")] ulong AppVersion,
    [property: JsonPropertyName("ChainID")] string ChainId, long Height,
    CometTimestamp Time, [property: JsonPropertyName("LastBlockID")] CometBlockId LastBlockId,
    byte[] LastCommitHash, byte[] DataHash, byte[] ValidatorsHash, byte[] NextValidatorsHash,
    byte[] ConsensusHash, byte[] AppHash, byte[] LastResultsHash, byte[] EvidenceHash, byte[] ProposerAddress);
public sealed record CometValidator(
    [property: JsonConverter(typeof(CometArray32Converter))] byte[] PublicKey, long VotingPower);
public sealed record CometCommitSignature([property: JsonPropertyName("BlockIDFlag")] byte BlockIdFlag,
    byte[] ValidatorAddress, CometTimestamp Timestamp, byte[] Signature);
public sealed record CometCommit(long Height, int Round,
    [property: JsonPropertyName("BlockID")] CometBlockId BlockId,
    IReadOnlyList<CometCommitSignature> Signatures);
public sealed record CometExecutionResult(uint Code, byte[] Data, long GasWanted, long GasUsed);
[method: JsonConstructor]
public sealed record CommittedProof(long Total, long Index,
    [property: JsonConverter(typeof(CometAuntsConverter))] IReadOnlyList<byte[]> Aunts)
{
    [JsonIgnore] public long LeafIndex { get => Index; init => Index = value; }
    [JsonIgnore] public long LeafCount { get => Total; init => Total = value; }
    [JsonIgnore] public IReadOnlyList<byte[]> Siblings { get => Aunts; init => Aunts = value; }
    public static implicit operator CommittedProof(MerkleProof proof) =>
        new(proof.LeafCount, proof.LeafIndex, proof.Siblings);
}
[method: JsonConstructor]
public sealed record CommittedReceiptInclusion(byte[] Transaction, CommittedProof TransactionProof,
    CometExecutionResult Result, CommittedProof ResultProof, byte[] BatchTransaction, CommittedProof BatchProof,
    CometHeader NextHeader, CometCommit NextCommit, IReadOnlyList<CometValidator> NextValidators)
{
    [JsonIgnore] public CometHeader ResultsHeader { get => NextHeader; init => NextHeader = value; }
    [JsonIgnore] public CometCommit ResultsCommit { get => NextCommit; init => NextCommit = value; }
    [JsonIgnore] public IReadOnlyList<CometValidator> ResultsValidators { get => NextValidators; init => NextValidators = value; }
    [JsonIgnore] public IReadOnlyList<byte[]> ActivityIds { get; init; } = Array.Empty<byte[]>();

    public CommittedReceiptInclusion(byte[] batchTransaction, MerkleProof batchProof,
        IReadOnlyList<byte[]> activityIds, CometExecutionResult result, MerkleProof resultProof,
        CometHeader resultsHeader, CometCommit resultsCommit, IReadOnlyList<CometValidator> resultsValidators)
        : this(Array.Empty<byte>(), new(0, 0, Array.Empty<byte[]>()), result, resultProof,
            batchTransaction, batchProof, resultsHeader, resultsCommit, resultsValidators)
    {
        ActivityIds = activityIds;
    }
}

internal sealed class CometArray32Converter : JsonConverter<byte[]>
{
    public override byte[] Read(ref Utf8JsonReader reader, Type type, JsonSerializerOptions options)
    {
        if (reader.TokenType != JsonTokenType.StartArray) throw new JsonException();
        var bytes = new byte[32];
        for (var index = 0; index < bytes.Length; index++)
        {
            if (!reader.Read() || reader.TokenType != JsonTokenType.Number || !reader.TryGetByte(out bytes[index]))
                throw new JsonException();
        }
        if (!reader.Read() || reader.TokenType != JsonTokenType.EndArray) throw new JsonException();
        return bytes;
    }

    public override void Write(Utf8JsonWriter writer, byte[] value, JsonSerializerOptions options)
    {
        if (value.Length != 32) throw new JsonException();
        writer.WriteStartArray();
        foreach (var item in value) writer.WriteNumberValue(item);
        writer.WriteEndArray();
    }
}

internal sealed class CometAuntsConverter : JsonConverter<IReadOnlyList<byte[]>>
{
    private static readonly CometArray32Converter Array32 = new();
    public override IReadOnlyList<byte[]> Read(ref Utf8JsonReader reader, Type type, JsonSerializerOptions options)
    {
        if (reader.TokenType != JsonTokenType.StartArray) throw new JsonException();
        var aunts = new List<byte[]>();
        while (reader.Read() && reader.TokenType != JsonTokenType.EndArray)
        {
            if (aunts.Count == 63) throw new JsonException();
            aunts.Add(Array32.Read(ref reader, typeof(byte[]), options));
        }
        if (reader.TokenType != JsonTokenType.EndArray) throw new JsonException();
        return aunts;
    }

    public override void Write(Utf8JsonWriter writer, IReadOnlyList<byte[]> value, JsonSerializerOptions options)
    {
        writer.WriteStartArray();
        foreach (var aunt in value) Array32.Write(writer, aunt, options);
        writer.WriteEndArray();
    }
}

internal sealed class CometTimestampConverter : JsonConverter<CometTimestamp>
{
    public override CometTimestamp Read(ref Utf8JsonReader reader, Type type, JsonSerializerOptions options)
    {
        if (reader.TokenType != JsonTokenType.String) throw new JsonException();
        var match = Regex.Match(reader.GetString()!,
            @"^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$");
        if (!match.Success || !DateTimeOffset.TryParseExact(match.Groups[1].Value + match.Groups[3].Value,
            "yyyy-MM-dd'T'HH:mm:ssK", CultureInfo.InvariantCulture, DateTimeStyles.None, out var time))
            throw new JsonException();
        var nanos = match.Groups[2].Success ? int.Parse(match.Groups[2].Value.PadRight(9, '0'), CultureInfo.InvariantCulture) : 0;
        return new(time.ToUnixTimeSeconds(), nanos);
    }

    public override void Write(Utf8JsonWriter writer, CometTimestamp value, JsonSerializerOptions options)
    {
        if (value.Nanoseconds is < 0 or >= 1000000000) throw new JsonException();
        var time = DateTimeOffset.FromUnixTimeSeconds(value.Seconds).UtcDateTime;
        var fraction = value.Nanoseconds == 0 ? "" : "." + value.Nanoseconds.ToString("D9", CultureInfo.InvariantCulture).TrimEnd('0');
        writer.WriteStringValue(time.ToString("yyyy-MM-dd'T'HH:mm:ss", CultureInfo.InvariantCulture) + fraction + "Z");
    }
}

public static class Committed
{
    private static readonly byte[] BatchDomain = Encoding.UTF8.GetBytes("PAXEER/v1/sequence-batch\0");
    internal static readonly JsonSerializerOptions WireOptions = new()
    {
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
        Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping
    };

    public static SequencerReceiptVerification Verify(SequencerReceiptVerification receiptVerification,
        CometHeader blockHeader, CommittedReceiptInclusion receiptInclusion, CometCommit commit,
        IReadOnlyList<CometValidator> validatorSet)
    {
        try
        {
            var receipt = receiptVerification.Receipt;
            var canonical = EncodeReceipt(receipt, true);
            Require(receiptVerification.Rung == "receipted" && Equal(receiptVerification.CanonicalBytes, canonical) &&
                Equal(receiptVerification.ReceiptDigest, SHA256.HashData(Join(
                    Encoding.UTF8.GetBytes(GeneratedReceiptContract.ReceiptDomain + "\0"), EncodeReceipt(receipt, false)))) &&
                receipt.ResultCode is 0 or 1 && receiptVerification.Status == (uint)(1 - receipt.ResultCode) &&
                receipt.Effects.Count > 0 && receipt.Effects[0].Body.Length >= 28 && receipt.BatchId.Length == 32 &&
                receipt.BatchId.AsSpan(0, 24).IndexOfAnyExcept((byte)0) < 0);
            var height = BinaryPrimitives.ReadUInt64BigEndian(receipt.BatchId.AsSpan(24));
            Require(height > 0 && blockHeader.Height > 0 && blockHeader.Height < long.MaxValue &&
                (ulong)blockHeader.Height >= height);
            VerifyCommit(blockHeader, commit, validatorSet);

            var next = receiptInclusion.NextHeader;
            Require(next.Height == blockHeader.Height + 1 && next.ChainId == blockHeader.ChainId &&
                Equal(next.LastBlockId.Hash, commit.BlockId.Hash) &&
                next.LastBlockId.PartsTotal == commit.BlockId.PartsTotal &&
                Equal(next.LastBlockId.PartsHash, commit.BlockId.PartsHash) &&
                Equal(next.ValidatorsHash, blockHeader.NextValidatorsHash));
            VerifyCommit(next, receiptInclusion.NextCommit, receiptInclusion.NextValidators);

            var transaction = EVMTransaction(receiptInclusion.Transaction);
            var keccak = new KeccakDigest(256);
            keccak.BlockUpdate(transaction, 0, transaction.Length);
            var transactionHash = new byte[32];
            keccak.DoFinal(transactionHash, 0);
            var proof = receiptInclusion.TransactionProof;
            Require(Equal(transactionHash, receipt.ActivityId) && proof.Index > 0 &&
                proof.Index == receiptInclusion.ResultProof.Index && proof.Total == receiptInclusion.ResultProof.Total &&
                proof.Total == receiptInclusion.BatchProof.Total && receiptInclusion.BatchProof.Index == 0 &&
                (ulong)proof.Index == BinaryPrimitives.ReadUInt32BigEndian(receipt.Effects[0].Body.AsSpan(24)));
            VerifyProof(SHA256.HashData(receiptInclusion.Transaction), proof, blockHeader.DataHash);
            VerifyProof(SHA256.HashData(receiptInclusion.BatchTransaction), receiptInclusion.BatchProof, blockHeader.DataHash);
            var result = receiptInclusion.Result;
            VerifyProof(EncodeResult(result), receiptInclusion.ResultProof, next.LastResultsHash);
            Require(result.Code is 0 or 45 && receipt.ResultCode == (result.Code == 0 ? 0 : 1) &&
                result.GasUsed >= 0 && result.GasWanted >= result.GasUsed &&
                (ulong)result.GasUsed == BinaryPrimitives.ReadUInt64BigEndian(receipt.Effects[0].Body));

            var batch = receiptInclusion.BatchTransaction;
            var offset = BatchDomain.Length;
            Require(batch.Length == offset + 24 + 128 + 64 && batch.AsSpan(0, offset).SequenceEqual(BatchDomain));
            var batchHeight = BinaryPrimitives.ReadUInt64BigEndian(batch.AsSpan(offset));
            var from = BinaryPrimitives.ReadUInt64BigEndian(batch.AsSpan(offset + 8));
            var to = BinaryPrimitives.ReadUInt64BigEndian(batch.AsSpan(offset + 16));
            Require(batchHeight == (ulong)blockHeader.Height && from > 0 &&
                receipt.GlobalSequence >= from && receipt.GlobalSequence <= to);
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
        long total = 0, signed = 0;
        foreach (var validator in validators)
        {
            Require(validator.VotingPower <= long.MaxValue / 8 - total);
            total += validator.VotingPower;
        }
        for (var i = 0; i < validators.Count; i++)
        {
            var validator = validators[i];
            var signature = commit.Signatures[i];
            if (signature.BlockIdFlag == 1)
            {
                Require((signature.Signature is null || signature.Signature.Length == 0) &&
                    (signature.ValidatorAddress is null || signature.ValidatorAddress.Length == 0) &&
                    signature.Timestamp.Seconds == -62135596800 && signature.Timestamp.Nanoseconds == 0);
                continue;
            }
            Require(signature.BlockIdFlag is 2 or 3 && signature.Signature is not null &&
                signature.ValidatorAddress is not null && signature.Signature.Length == 64 &&
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

    internal static byte[] EVMTransaction(byte[] blockTransaction)
    {
        Require(blockTransaction.Length is > 0 and <= 1048576);
        var tx = Parse(blockTransaction);
        var body = Parse(Single(tx, 1).Data);
        var messages = body.Where(field => field.Tag == 1).ToArray();
        Require(messages.Length > 0 && messages[0].Wire == 2);
        var message = Parse(messages[0].Data);
        var type = Encoding.UTF8.GetString(Proto(message, 1, 2).Data);
        Require(type is "/paxprotocol.paxchain.evm.MsgEVMTransaction" or "/ethermint.evm.v1.MsgEthereumTx");
        var wrapped = Parse(Single(message, 2).Data);
        var data = Proto(wrapped, 1, 2).Data;
        Require(data.Length > 0);
        if (data[0] >= 0xc0 || data[0] is >= 1 and <= 4) return data;
        var transaction = Parse(data);
        return EncodeEVM(Encoding.UTF8.GetString(Proto(transaction, 1, 2).Data), Parse(Single(transaction, 2).Data));
    }

    private static byte[] EncodeEVM(string kind, List<Field> fields)
    {
        var (type, layout) = kind switch
        {
            "/paxprotocol.paxchain.eth.LegacyTx" => (0, "uduhdrbbb"),
            "/paxprotocol.paxchain.eth.AccessListTx" => (1, "duduhdrabbb"),
            "/paxprotocol.paxchain.eth.DynamicFeeTx" => (2, "dudduhdrabbb"),
            "/paxprotocol.paxchain.eth.BlobTx" => (3, "dudduhdradx_bbb"),
            "/paxprotocol.paxchain.eth.SetCodeTx" => (4, "dudduhdraobbb"),
            _ => throw Failure()
        };
        var payload = new List<byte[]>();
        for (var index = 0; index < layout.Length; index++)
        {
            var format = layout[index];
            if (format == '_') continue;
            payload.Add(format is 'a' or 'x' or 'o'
                ? EVMList(fields.Where(field => field.Tag == index + 1), format)
                : RLP(EVMValue(Proto(fields, index + 1, format == 'u' ? 0 : 2), format)));
        }
        var encoded = RLP(Join(payload.ToArray()), true);
        return type == 0 ? encoded : Join(new[] { (byte)type }, encoded);
    }

    private static byte[] EVMValue(Field field, char format)
    {
        var value = field.Data;
        switch (format)
        {
            case 'u': return Unsigned(new BigInteger(field.Value));
            case 'd':
                if (value.Length == 0) return value;
                Require(value.Length <= 78 && value.All(digit => digit is >= (byte)'0' and <= (byte)'9'));
                var number = BigInteger.Parse(Encoding.ASCII.GetString(value), CultureInfo.InvariantCulture);
                Require(number.GetBitLength() <= 256);
                return Unsigned(number);
            case 'h': return value.Length == 0 ? value : Hex(Encoding.UTF8.GetString(value), 20);
            case 'b':
                Require(value.Length <= 32);
                var index = 0;
                while (index < value.Length && value[index] == 0) index++;
                return value[index..];
            default: return value;
        }
    }

    private static byte[] EVMList(IEnumerable<Field> values, char kind)
    {
        var payload = new List<byte[]>();
        foreach (var value in values)
        {
            Require(value.Wire == 2);
            if (kind == 'x')
            {
                Require(value.Data.Length == 32);
                payload.Add(RLP(value.Data));
                continue;
            }
            var fields = Parse(value.Data);
            if (kind == 'o')
            {
                var formats = "dhubbb";
                var items = new List<byte[]>();
                for (var index = 0; index < formats.Length; index++)
                {
                    var field = Proto(fields, index + 1, formats[index] == 'u' ? 0 : 2);
                    if (formats[index] is 'd' or 'h') Require(field.Data.Length > 0);
                    items.Add(RLP(EVMValue(field, formats[index])));
                }
                payload.Add(RLP(Join(items.ToArray()), true));
                continue;
            }
            var address = Hex(Encoding.UTF8.GetString(Proto(fields, 1, 2).Data), 20);
            var keys = fields.Where(field => field.Tag == 2).Select(field =>
            {
                Require(field.Wire == 2);
                return RLP(Hex(Encoding.UTF8.GetString(field.Data), 32));
            }).ToArray();
            payload.Add(RLP(Join(RLP(address), RLP(Join(keys), true)), true));
        }
        return RLP(Join(payload.ToArray()), true);
    }

    private static byte[] Unsigned(BigInteger value) => value.IsZero ? Array.Empty<byte>() : value.ToByteArray(true, true);

    private static byte[] RLP(byte[] value, bool list = false)
    {
        if (!list && value.Length == 1 && value[0] < 0x80) return value;
        var prefix = list ? 0xc0 : 0x80;
        if (value.Length <= 55) return Join(new[] { (byte)(prefix + value.Length) }, value);
        var length = Unsigned(new BigInteger(value.Length));
        return Join(new[] { (byte)(prefix + 55 + length.Length) }, length, value);
    }

    private static byte[] EncodeReceipt(ProtocolReceipt receipt, bool signed)
    {
        using var stream = new MemoryStream();
        void U16(ushort value) { Span<byte> bytes = stackalloc byte[2]; BinaryPrimitives.WriteUInt16BigEndian(bytes, value); stream.Write(bytes); }
        void U32(uint value) { Span<byte> bytes = stackalloc byte[4]; BinaryPrimitives.WriteUInt32BigEndian(bytes, value); stream.Write(bytes); }
        void U64(ulong value) { Span<byte> bytes = stackalloc byte[8]; BinaryPrimitives.WriteUInt64BigEndian(bytes, value); stream.Write(bytes); }
        void U128(UInt128Value value) { U64(value.High); U64(value.Low); }
        void Bytes(byte[] value) { U32(checked((uint)value.Length)); stream.Write(value); }
        U16(receipt.ProtocolVersion); U16(0x5201); U16(receipt.ProtocolVersion);
        Bytes(receipt.ActivityId); U64(receipt.GlobalSequence);
        Bytes(receipt.PreviousStateRoot); Bytes(receipt.ResultingStateRoot); Bytes(receipt.ActivityRoot);
        U32(unchecked((uint)receipt.ResultCode)); U32(checked((uint)receipt.Effects.Count));
        foreach (var effect in receipt.Effects)
        {
            U16(effect.ModuleId); U16(effect.Ordinal); U16(effect.EventType);
            stream.WriteByte(effect.Kind); stream.WriteByte(effect.Monetary ? (byte)1 : (byte)0);
            Bytes(effect.TransferSetRoot); Bytes(effect.Body);
        }
        U128(receipt.FeeCharged); Bytes(receipt.BatchId); U16(receipt.ModuleId);
        U32(receipt.ModuleVersion); U32(receipt.ParameterVersion); stream.WriteByte(receipt.Operation);
        Bytes(receipt.Asset); U128(receipt.Amount); Bytes(receipt.From);
        U128(receipt.FromBalanceBefore); U128(receipt.FromBalanceAfter); U64(receipt.FromSequence);
        Bytes(receipt.To); U128(receipt.ToBalanceBefore); U128(receipt.ToBalanceAfter);
        Bytes(receipt.TransferSetRoot); Bytes(receipt.AuthorizationHash); Bytes(receipt.ContextHash); U64(receipt.Timestamp);
        stream.WriteByte(signed ? (byte)1 : (byte)0);
        if (signed) Bytes(receipt.SequencerSignature);
        return stream.ToArray();
    }

    private static byte[] Hex(string value, int length)
    {
        Require(value.StartsWith("0x", StringComparison.Ordinal) && value.Length == 2 + length * 2);
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
            Require(key >> 3 is > 0 and <= 536870911 && wire is 0 or 1 or 2 or 5);
            if (wire == 0) fields.Add(new((int)(key >> 3), wire, Array.Empty<byte>(), Read()));
            else if (wire is 1 or 5)
            {
                var length = wire == 1 ? 8 : 4;
                Require(length <= bytes.Length - position);
                fields.Add(new((int)(key >> 3), wire, bytes.AsSpan(position, length).ToArray(), 0));
                position += length;
            }
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
        Require(matches.Length == 1 && matches[0].Wire == 2 && matches[0].Data.Length > 0);
        return matches[0];
    }

    private static Field Proto(List<Field> fields, int tag, int wire)
    {
        var matches = fields.Where(field => field.Tag == tag).ToArray();
        Require(matches.Length <= 1 && (matches.Length == 0 || matches[0].Wire == wire));
        return matches.Length == 0 ? new(tag, wire, Array.Empty<byte>(), 0) : matches[0];
    }

    private static void VerifyProof(byte[] leaf, CommittedProof proof, byte[] expected)
    {
        Require(proof.Total > 0 && proof.Index >= 0 && proof.Index < proof.Total &&
            proof.Aunts.Count <= 63 && proof.Aunts.All(aunt => aunt.Length == 32) && expected.Length == 32);
        var cursor = proof.Aunts.Count;
        byte[] Reconstruct(long index, long count)
        {
            if (count == 1) { Require(cursor == 0); return Leaf(leaf); }
            Require(cursor > 0);
            var sibling = proof.Aunts[--cursor];
            long split = 1;
            while (split < count - split) split <<= 1;
            return index < split ? Inner(Reconstruct(index, split), sibling) :
                Inner(sibling, Reconstruct(index - split, count - split));
        }
        Require(Equal(Reconstruct(proof.Index, proof.Total), expected) && cursor == 0);
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
