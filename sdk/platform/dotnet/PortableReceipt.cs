using System.Security.Cryptography;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace LayerX.Sdk;

public sealed class PortableReceipt
{
    public const string Format = "layerx-receipt-proof-v2";
    private const int MaximumBundleBytes = 16 * 1024 * 1024;
    private static readonly string[] ReceiptFields = { "format", "verificationLevel", "rawTransaction",
        "signedReceipt", "receiptDigest", "receiptKey", "receiptJson" };
    private static readonly string[] CommittedFields = { "blockHeader", "receiptInclusion", "commit", "validatorSet" };
    private static readonly JsonSerializerOptions Options = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
        Converters = { new HexBytesConverter() }
    };
    private readonly byte[] bytes;
    public SequencerReceiptVerification Verification { get; }
    public string VerificationLevel => Verification.Rung;

    private PortableReceipt(byte[] bytes, SequencerReceiptVerification verification)
    {
        this.bytes = bytes;
        Verification = verification;
    }

    public byte[] export() => bytes.ToArray();

    public static PortableReceipt export(JsonElement receiptJson, ReadOnlyMemory<byte> rawTransaction,
        ReceiptKeys keys, CometHeader? blockHeader = null, CommittedReceiptInclusion? receiptInclusion = null,
        CometCommit? commit = null, IReadOnlyList<CometValidator>? validatorSet = null)
    {
        var verified = LocalVerifier.VerifySequencerReceipt(receiptJson, rawTransaction, keys);
        var key = keys.Pinned.FirstOrDefault(entry =>
            Equal(entry.KeyId, verified.Receipt.ContextHash) &&
            LocalVerifier.VerifyEd25519Digest(entry.PublicKey, verified.Receipt.SequencerSignature, verified.ReceiptDigest) &&
            Height(receiptJson, "height") >= entry.FirstHeight && Height(receiptJson, "height") <= entry.LastHeight)
            ?? throw Failure();
        var committed = blockHeader is not null || receiptInclusion is not null || commit is not null || validatorSet is not null;
        Require(!committed || (blockHeader is not null && receiptInclusion is not null && commit is not null && validatorSet is not null));
        var bundle = new Dictionary<string, object?>
        {
            ["format"] = Format,
            ["verificationLevel"] = committed ? "committed" : "receipted",
            ["rawTransaction"] = Hex(rawTransaction.Span),
            ["signedReceipt"] = Hex(verified.CanonicalBytes),
            ["receiptDigest"] = Hex(verified.ReceiptDigest),
            ["receiptKey"] = new { keyId = Hex(key.KeyId), publicKey = Hex(key.PublicKey),
                firstHeight = Quantity(key.FirstHeight), lastHeight = Quantity(key.LastHeight) },
            ["receiptJson"] = receiptJson
        };
        if (committed)
        {
            bundle["blockHeader"] = blockHeader;
            bundle["receiptInclusion"] = receiptInclusion;
            bundle["commit"] = commit;
            bundle["validatorSet"] = validatorSet;
        }
        return verify(JsonSerializer.SerializeToUtf8Bytes(bundle, Options), keys);
    }

    public static PortableReceipt verify(ReadOnlyMemory<byte> bundle, ReceiptKeys keys)
    {
        try
        {
            Require(!bundle.IsEmpty && bundle.Length <= MaximumBundleBytes);
            var owned = bundle.ToArray();
            using var document = JsonDocument.Parse(owned);
            var root = document.RootElement;
            CheckFields(root);
            Require(root.GetProperty("format").GetString() == Format);
            var level = root.GetProperty("verificationLevel").GetString();
            Require(level is "receipted" or "committed");
            var fields = level == "committed" ? ReceiptFields.Concat(CommittedFields).ToArray() : ReceiptFields;
            Require(root.EnumerateObject().Select(property => property.Name).ToHashSet().SetEquals(fields));
            var carried = root.GetProperty("receiptKey");
            Require(carried.EnumerateObject().Select(property => property.Name).ToHashSet()
                .SetEquals(new[] { "keyId", "publicKey", "firstHeight", "lastHeight" }));
            var keyId = ReadHex(carried.GetProperty("keyId"), 32);
            var publicKey = ReadHex(carried.GetProperty("publicKey"), 32);
            var first = Height(carried, "firstHeight");
            var last = Height(carried, "lastHeight");
            var pinned = keys.Pinned.Where(key => Equal(key.KeyId, keyId) && Equal(key.PublicKey, publicKey) &&
                key.FirstHeight == first && key.LastHeight == last).ToArray();
            Require(pinned.Length > 0);
            var view = root.GetProperty("receiptJson");
            var raw = ReadHex(root.GetProperty("rawTransaction"));
            var verified = LocalVerifier.VerifySequencerReceipt(view, raw, keys with { Pinned = pinned });
            Require(Equal(ReadHex(root.GetProperty("signedReceipt")), verified.CanonicalBytes) &&
                Equal(ReadHex(root.GetProperty("receiptDigest"), 32), verified.ReceiptDigest));
            if (level == "committed")
            {
                var header = Read<CometHeader>(root, "blockHeader");
                if (view.GetProperty("rung").GetString() == "committed")
                    Require(Height(view, "blockNumber") == (ulong)header.Height &&
                        Equal(ReadHex(view.GetProperty("blockHash"), 32), Committed.HeaderHash(header)));
                verified = Committed.Verify(verified, header, Read<CommittedReceiptInclusion>(root, "receiptInclusion"),
                    Read<CometCommit>(root, "commit"), Read<CometValidator[]>(root, "validatorSet"));
            }
            return new(owned, verified);
        }
        catch (PlatformSdkException) { throw; }
        catch (Exception error) when (error is JsonException or ArgumentException or InvalidOperationException or
            KeyNotFoundException or FormatException or OverflowException or NullReferenceException)
        {
            throw Failure();
        }
    }

    private static T Read<T>(JsonElement root, string name) where T : class =>
        root.GetProperty(name).Deserialize<T>(Options) ?? throw Failure();

    private static void CheckFields(JsonElement value)
    {
        if (value.ValueKind == JsonValueKind.Object)
        {
            var names = new HashSet<string>(StringComparer.Ordinal);
            foreach (var property in value.EnumerateObject())
            {
                Require(names.Add(property.Name) && property.Name is not
                    ("batchId" or "asset" or "previousStateRoot" or "resultingStateRoot"));
                CheckFields(property.Value);
            }
        }
        else if (value.ValueKind == JsonValueKind.Array)
            foreach (var element in value.EnumerateArray()) CheckFields(element);
    }

    private static byte[] ReadHex(JsonElement value, int? length = null)
    {
        var text = value.GetString() ?? throw Failure();
        Require(text.StartsWith("0x", StringComparison.Ordinal) && text.Length % 2 == 0);
        var decoded = Convert.FromHexString(text.AsSpan(2));
        Require(Hex(decoded) == text && (!length.HasValue || decoded.Length == length.Value));
        return decoded;
    }

    private static ulong Height(JsonElement value, string name)
    {
        var text = value.GetProperty(name).GetString() ?? throw Failure();
        Require(text.StartsWith("0x", StringComparison.Ordinal) && text.Length > 2);
        var height = Convert.ToUInt64(text[2..], 16);
        Require(Quantity(height) == text);
        return height;
    }

    private static string Hex(ReadOnlySpan<byte> value) => "0x" + Convert.ToHexString(value).ToLowerInvariant();
    private static string Quantity(ulong value) => "0x" + value.ToString("x");
    private static bool Equal(ReadOnlySpan<byte> left, ReadOnlySpan<byte> right) => CryptographicOperations.FixedTimeEquals(left, right);
    private static void Require(bool condition) { if (!condition) throw Failure(); }
    private static PlatformSdkException Failure() => new(SdkErrorCode.VerificationFailure, RetryClass.Never);

    private sealed class HexBytesConverter : JsonConverter<byte[]>
    {
        public override byte[] Read(ref Utf8JsonReader reader, Type type, JsonSerializerOptions options)
        {
            using var document = JsonDocument.ParseValue(ref reader);
            return ReadHex(document.RootElement);
        }

        public override void Write(Utf8JsonWriter writer, byte[] value, JsonSerializerOptions options) =>
            writer.WriteStringValue(Hex(value));
    }
}
