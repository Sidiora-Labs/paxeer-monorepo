using System.Buffers.Binary;
using System.Text;
using System.Numerics;
using System.Text.RegularExpressions;

namespace LayerX.Sdk;

public sealed record NativeProgramCall(byte[] ProgramId, ushort GuestAbi, string Entrypoint, byte[] Calldata,
    byte[] Capabilities, byte[] AccessDeclaration, uint ResponseCapacity, ulong[] Resources)
{
    public byte[] EncodeCalldata(ProtocolAmount feeLimit, bool simulate = false)
    {
        _ = Encode();
        return ProgramsAbi.Encode(simulate ? "3044bd56" : "54bd2b48",
            ProgramsAbi.Fixed(ProgramId), ProgramsAbi.Number(GuestAbi, 16),
            ProgramsAbi.Bytes(Encoding.ASCII.GetBytes(Entrypoint)), ProgramsAbi.Bytes(Calldata),
            ProgramsAbi.Bytes(Capabilities), ProgramsAbi.Bytes(AccessDeclaration),
            ProgramsAbi.Number(ResponseCapacity, 32),
            ProgramsAbi.Fixed(Resources.SelectMany(value => ProgramsAbi.Word(value, 64)).ToArray()),
            ProgramsAbi.Number(feeLimit.Value, 128));
    }

    public byte[] Encode()
    {
        if (ProgramId.Length != 32 || ProgramId.All(value => value == 0) || !GeneratedReceiptContract.SupportsProgramGuestAbi(GuestAbi)
            || !Regex.IsMatch(Entrypoint, "\\A[A-Za-z0-9_.]{1,128}\\z") || Calldata.Length > 1048576
            || Capabilities.Length > 65535 || AccessDeclaration.Length > 1048576 || ResponseCapacity > 1048576 || Resources.Length != 7)
            throw new ArgumentException("invalid native program call");
        var entry = Encoding.ASCII.GetBytes(Entrypoint);
        var output = new byte[106 + entry.Length + Calldata.Length + Capabilities.Length + AccessDeclaration.Length];
        ProgramId.CopyTo(output, 0); BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(32), GuestAbi);
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(34), (ushort)entry.Length);
        BinaryPrimitives.WriteUInt32BigEndian(output.AsSpan(36), (uint)Calldata.Length);
        BinaryPrimitives.WriteUInt16BigEndian(output.AsSpan(40), (ushort)Capabilities.Length);
        BinaryPrimitives.WriteUInt32BigEndian(output.AsSpan(42), (uint)AccessDeclaration.Length);
        BinaryPrimitives.WriteUInt32BigEndian(output.AsSpan(46), ResponseCapacity);
        for (var i = 0; i < 7; i++) BinaryPrimitives.WriteUInt64BigEndian(output.AsSpan(50 + i * 8), Resources[i]);
        var offset = 106;
        foreach (var body in new[] { entry, Calldata, Capabilities, AccessDeclaration }) { body.CopyTo(output, offset); offset += body.Length; }
        return output;
    }

    public static NativeProgramCall Decode(byte[] payload)
    {
        if (payload.Length < 106) throw new ArgumentException("invalid native program call");
        var sizes = new ulong[] { BinaryPrimitives.ReadUInt16BigEndian(payload.AsSpan(34)), BinaryPrimitives.ReadUInt32BigEndian(payload.AsSpan(36)), BinaryPrimitives.ReadUInt16BigEndian(payload.AsSpan(40)), BinaryPrimitives.ReadUInt32BigEndian(payload.AsSpan(42)) };
        if (106UL + sizes[0] + sizes[1] + sizes[2] + sizes[3] != (ulong)payload.Length) throw new ArgumentException("invalid native program call");
        var resources = new ulong[7]; for (var i = 0; i < 7; i++) resources[i] = BinaryPrimitives.ReadUInt64BigEndian(payload.AsSpan(50 + i * 8));
        var bodies = new byte[4][]; var offset = 106;
        for (var i = 0; i < 4; i++) { bodies[i] = payload.AsSpan(offset, (int)sizes[i]).ToArray(); offset += (int)sizes[i]; }
        var call = new NativeProgramCall(payload[..32], BinaryPrimitives.ReadUInt16BigEndian(payload.AsSpan(32)), Encoding.ASCII.GetString(bodies[0]), bodies[1], bodies[2], bodies[3], BinaryPrimitives.ReadUInt32BigEndian(payload.AsSpan(46)), resources);
        if (!call.Encode().SequenceEqual(payload)) throw new ArgumentException("invalid native program call");
        return call;
    }
}

internal static class ProgramsAbi
{
    internal static (byte[] Data, bool Dynamic) Fixed(byte[] value)
    {
        if (value.Length == 0 || value.Length % 32 != 0) throw new ArgumentException("ABI word length");
        return (value.ToArray(), false);
    }
    internal static (byte[] Data, bool Dynamic) Number(BigInteger value, int bits = 256) => (Word(value, bits), false);
    internal static byte[] Word(BigInteger value, int bits = 256)
    {
        if (bits < 1 || bits > 256 || value < 0 || value >= (BigInteger.One << bits)) throw new ArgumentException("ABI integer range");
        var raw = value.ToByteArray(true, true); var word = new byte[32]; raw.CopyTo(word, 32 - raw.Length); return word;
    }
    internal static (byte[] Data, bool Dynamic) Bytes(byte[] value)
    {
        var output = new byte[32 + checked((value.Length + 31) / 32 * 32)];
        Word(value.Length).CopyTo(output, 0); value.CopyTo(output, 32); return (output, true);
    }
    internal static byte[] Encode(string selector, params (byte[] Data, bool Dynamic)[] arguments)
    {
        var headSize = arguments.Sum(value => value.Dynamic ? 32 : value.Data.Length);
        using var head = new MemoryStream(); using var tail = new MemoryStream();
        foreach (var value in arguments)
        {
            if (value.Dynamic) { head.Write(Word(headSize + tail.Length)); tail.Write(value.Data); }
            else head.Write(value.Data);
        }
        return [.. Convert.FromHexString(selector), .. head.ToArray(), .. tail.ToArray()];
    }
    internal static BigInteger ReadWord(byte[] value, int index, int bits = 256)
    {
        var offset = checked(index * 32);
        if (offset < 0 || offset > value.Length - 32) throw new ArgumentException("ABI output length");
        var result = new BigInteger(value.AsSpan(offset, 32), true, true);
        if (result >= (BigInteger.One << bits)) throw new ArgumentException("ABI output integer range");
        return result;
    }
    internal static byte[] ReadBytes(byte[] value, int index, int headSize, int maximum)
    {
        var offset = checked((int)ReadWord(value, index));
        if (offset != headSize || offset % 32 != 0 || offset > value.Length - 32) throw new ArgumentException("ABI output offset");
        var length = checked((int)ReadWord(value, offset / 32));
        var padded = checked((length + 31) / 32 * 32);
        if (length > maximum || offset + 32 + padded != value.Length || value.AsSpan(offset + 32 + length, padded - length).ContainsAnyExcept((byte)0))
            throw new ArgumentException("ABI output bytes");
        return value.AsSpan(offset + 32, length).ToArray();
    }
}
