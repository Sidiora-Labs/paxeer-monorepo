using System.Numerics;
using System.Runtime.CompilerServices;
using System.Text.Json;
using LayerX.Sdk;
using Org.BouncyCastle.Asn1.Sec;
using Org.BouncyCastle.Crypto.Digests;
using Org.BouncyCastle.Crypto.Parameters;
using Org.BouncyCastle.Crypto.Signers;
using Org.BouncyCastle.Math.EC;
using Xunit;
using BcInteger = Org.BouncyCastle.Math.BigInteger;

namespace LayerX.Sdk.Tests;

internal static class RepairM1369Native
{
    // Public test key: secp256k1 scalar one. This signer produces genuine low-S
    // EIP-1559 signatures, including recovery parity, rather than RLP placeholders.
    internal const string Sender = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf";

    internal static JsonDocument RecordedVector([CallerFilePath] string source = "") => JsonDocument.Parse(File.ReadAllBytes(
        Path.GetFullPath(Path.Combine(Path.GetDirectoryName(source)!, "../../../conformance/fixtures/vector-v1-receipted-positive.json"))));

    internal static JsonElement ProgramsVector(JsonDocument document) => Assert.Single(document.RootElement.GetProperty("cases")
        .EnumerateArray().Where(value => value.GetProperty("id").GetString() == "V1-layerxprograms-call"));

    internal static byte[] Hex(JsonElement value, string name) => Convert.FromHexString(value.GetProperty(name).GetString()![2..]);

    internal static ReceiptKeys Keys(JsonElement vector)
    {
        var pin = vector.GetProperty("pinnedKey");
        return new([new ReceiptKey(Hex(pin, "keyId"), Hex(pin, "publicKey"),
            pin.GetProperty("firstHeight").GetUInt64(), pin.GetProperty("lastHeight").GetUInt64())], Hex(vector, "prevSeqHash"));
    }

    internal static (NativeProgramCall Call, ProtocolAmount Fee) RecordedCall([CallerFilePath] string source = "")
    {
        using var document = JsonDocument.Parse(File.ReadAllBytes(Path.Combine(Path.GetDirectoryName(source)!,
            "Fixtures/repair-m1369-recorded-call.json")));
        var value = document.RootElement;
        byte[] Bytes(string field) => Convert.FromHexString(value.GetProperty(field).GetString()!);
        var call = new NativeProgramCall(Bytes("program_id_hex"), value.GetProperty("guest_abi").GetUInt16(),
            value.GetProperty("entrypoint").GetString()!, Bytes("calldata_hex"), Bytes("capabilities_hex"),
            Bytes("access_declaration_hex"), value.GetProperty("response_capacity").GetUInt32(),
            value.GetProperty("resources").EnumerateArray().Select(number => number.GetUInt64()).ToArray());
        var fee = new ProtocolAmount(value.GetProperty("fee_limit").GetString()!);
        Assert.Equal(Bytes("evm_calldata_hex"), call.EncodeCalldata(fee));
        return (call, fee);
    }

    internal static (int Status, byte[] Body) Reply(JsonElement request, object? result) =>
        (200, JsonSerializer.SerializeToUtf8Bytes(new { jsonrpc = "2.0", id = request.GetProperty("id"), result }));

    internal static byte[] Sign(UnsignedProgramTransaction transaction)
    {
        Assert.Equal(Sender, transaction.From);
        var curve = SecNamedCurves.GetByName("secp256k1");
        var domain = new ECDomainParameters(curve.Curve, curve.G, curve.N, curve.H);
        var secret = BcInteger.One;
        var digest = Keccak(transaction.SigningPayload);
        var signer = new ECDsaSigner(new HMacDsaKCalculator(new Sha256Digest()));
        signer.Init(true, new ECPrivateKeyParameters(secret, domain));
        var signature = signer.GenerateSignature(digest);
        var r = signature[0];
        var s = signature[1].CompareTo(domain.N.ShiftRight(1)) > 0 ? domain.N.Subtract(signature[1]) : signature[1];
        var publicKey = domain.G.Multiply(secret).Normalize();
        var verifier = new ECDsaSigner();
        verifier.Init(false, new ECPublicKeyParameters(publicKey, domain));
        Assert.True(verifier.VerifySignature(digest, r, s));
        var parity = -1;
        for (var candidate = 0; candidate <= 1; candidate++)
        {
            var encoded = new byte[33]; encoded[0] = (byte)(2 + candidate);
            var x = r.ToByteArrayUnsigned(); x.CopyTo(encoded, encoded.Length - x.Length);
            var point = curve.Curve.DecodePoint(encoded);
            var recovered = ECAlgorithms.SumOfTwoMultiplies(domain.G,
                new BcInteger(1, digest).Negate().Mod(domain.N), point, s).Multiply(r.ModInverse(domain.N)).Normalize();
            if (recovered.Equals(publicKey)) parity = candidate;
        }
        Assert.InRange(parity, 0, 1);
        var signed = new byte[] { 2 }.Concat(List(
            Number(transaction.ChainId), Number(transaction.Nonce), Number(transaction.MaxPriorityFeePerGas),
            Number(transaction.MaxFeePerGas), Number(transaction.Gas), Bytes(Convert.FromHexString(transaction.To[2..])),
            Number(transaction.Value), Bytes(transaction.Data), List(), Number(parity),
            Bytes(r.ToByteArrayUnsigned()), Bytes(s.ToByteArrayUnsigned()))).ToArray();
        Assert.Equal(Keccak(signed), Programs.TransactionHash(signed));
        return signed;
    }

    internal static byte[] Keccak(byte[] value)
    {
        var hash = new KeccakDigest(256); hash.BlockUpdate(value, 0, value.Length);
        var output = new byte[32]; hash.DoFinal(output, 0); return output;
    }

    private static byte[] Number(BigInteger value) => Bytes(value == 0 ? [] : value.ToByteArray(true, true));
    private static byte[] Bytes(byte[] value) => value.Length == 1 && value[0] < 0x80 ? value : Wrap(value, 0x80);
    private static byte[] List(params byte[][] values) => Wrap(values.SelectMany(value => value).ToArray(), 0xc0);
    private static byte[] Wrap(byte[] value, int prefix)
    {
        if (value.Length < 56) return [(byte)(prefix + value.Length), .. value];
        var length = new BigInteger(value.Length).ToByteArray(true, true);
        return [(byte)(prefix + 55 + length.Length), .. length, .. value];
    }
}
