using System.Globalization;
using System.Numerics;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;
using LayerX.Sdk;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class ChainsTests
{
    [Fact]
    public void ChainsVectorsMatchAndRoundTrip()
    {
        using var document = Load("tests/vectors/chains/abi-vectors.json");
        var vectors = document.RootElement;
        Assert.Equal(Chains.Address, vectors.GetProperty("address").GetString());
        var methods = vectors.GetProperty("methods").EnumerateArray().ToArray();
        Assert.Equal(12, methods.Length);
        Assert.Equal(12, methods.Select(method => method.GetProperty("name").GetString()).Distinct().Count());
        foreach (var method in methods)
        {
            var name = method.GetProperty("name").GetString()!;
            var arguments = method.GetProperty("args").EnumerateArray().Select(Argument).ToArray();
            var encoded = Chains.EncodeCall(name, arguments);
            Assert.Equal(Hex(method, "selector"), Chains.Selector(name));
            Assert.Equal(Hex(method, "selector"), encoded[..4]);
            Assert.Equal(Hex(method, "calldata"), encoded);
        }

        var chainVector = vectors.GetProperty("getChain");
        var chainBytes = Hex(chainVector, "returndata");
        var decoded = chainVector.GetProperty("decoded");
        var expectedChain = new ChainRecord(decoded.GetProperty("id").GetUInt32(),
            decoded.GetProperty("name").GetString()!, decoded.GetProperty("operator").GetString()!,
            decoded.GetProperty("status").GetByte(), decoded.GetProperty("chainId").GetUInt64(),
            decoded.GetProperty("gasAllowance").GetUInt64(), decoded.GetProperty("byteAllowance").GetUInt64(),
            decoded.GetProperty("feeShareBps").GetUInt16(), Number(decoded, "bond"),
            Number(decoded, "baseFeePerGas"), decoded.GetProperty("createdHeight").GetUInt64());
        var chain = Chains.DecodeGetChain(chainBytes);
        Assert.Equal(expectedChain, chain);
        Assert.Equal(Encoding.UTF8.GetBytes(expectedChain.Name), Encoding.UTF8.GetBytes(chain.Name));
        Assert.Equal(chainBytes, Chains.EncodeGetChain(expectedChain));
        Assert.Equal(expectedChain, Chains.DecodeGetChain(Chains.EncodeGetChain(expectedChain)));

        var paramsVector = vectors.GetProperty("params");
        var paramsBytes = Hex(paramsVector, "returndata");
        decoded = paramsVector.GetProperty("decoded");
        var expectedParams = new ChainRegistryParams(decoded.GetProperty("authority").GetString()!,
            Number(decoded, "minBond"), Number(decoded, "creationFee"),
            decoded.GetProperty("minGasAllowance").GetUInt64(), decoded.GetProperty("maxGasAllowance").GetUInt64(),
            decoded.GetProperty("totalGasAllowanceLeft").GetUInt64(), decoded.GetProperty("maxChains").GetUInt32());
        Assert.Equal(expectedParams, Chains.DecodeParams(paramsBytes));
        Assert.Equal(paramsBytes, Chains.EncodeParams(expectedParams));
        Assert.Equal(expectedParams, Chains.DecodeParams(Chains.EncodeParams(expectedParams)));

        var events = vectors.GetProperty("events").EnumerateArray().ToArray();
        Assert.Equal(4, events.Length);
        Assert.Equal(4, events.Select(value => value.GetProperty("name").GetString()).Distinct().Count());
        foreach (var value in events)
        {
            var name = value.GetProperty("name").GetString()!;
            Assert.Equal(Hex(value, "topic0"), Chains.EventTopic(name));
            ChainEvent expected = name switch
            {
                "ChainCreated" => new ChainCreated(chain.Id, chain.Name, chain.Operator, chain.ChainId),
                "ChainStatusChanged" => new ChainStatusChanged(chain.Id, chain.Status),
                "ChainFunded" => new ChainFunded(chain.Id, chain.Operator, chain.Bond),
                "ChainOperatorChanged" => new ChainOperatorChanged(chain.Id, chain.Operator),
                _ => throw new InvalidOperationException("Unknown vector event")
            };
            var encoded = Chains.EncodeEvent(expected);
            Assert.Equal(Hex(value, "topic0"), encoded.Topics[0]);
            Assert.Equal(chainBytes[..32], encoded.Topics[1]);
            if (expected is not ChainStatusChanged) Assert.Equal(chainBytes[64..96], encoded.Topics[2]);
            var expectedData = name switch
            {
                "ChainCreated" => Join(Word(64), chainBytes[128..160], chainBytes[(11 * 32)..]),
                "ChainStatusChanged" => chainBytes[96..128],
                "ChainFunded" => chainBytes[256..288],
                "ChainOperatorChanged" => Array.Empty<byte>(),
                _ => throw new InvalidOperationException("Unknown vector event")
            };
            Assert.Equal(expectedData, encoded.Data);
            Assert.Equal(expected, Chains.DecodeEvent(encoded.Topics, expectedData));
            var roundTrip = Chains.EncodeEvent(Chains.DecodeEvent(encoded.Topics, encoded.Data));
            Assert.Equal(encoded.Data, roundTrip.Data);
            Assert.Equal(encoded.Topics.Length, roundTrip.Topics.Length);
            for (var i = 0; i < encoded.Topics.Length; i++) Assert.Equal(encoded.Topics[i], roundTrip.Topics[i]);
        }

        Assert.Equal(chain.ChainId, Chains.VirtualChainId(chain.Id));
        Assert.Equal(125000000UL + uint.MaxValue, Chains.VirtualChainId(uint.MaxValue));
        Assert.Equal("https://rpc.example/chain/1", Chains.ChainRpcUrl("https://rpc.example", chain.Id));
        Assert.Equal("https://rpc.example/evm/chain/1", Chains.ChainRpcUrl("https://rpc.example/evm///", chain.Id));
        Assert.Equal("wss://rpc.example/chain/4294967295", Chains.ChainRpcUrl("wss://rpc.example/", uint.MaxValue));
        Assert.Throws<ArgumentOutOfRangeException>(() => Chains.VirtualChainId(0));
        Assert.Throws<ArgumentOutOfRangeException>(() => Chains.ChainRpcUrl("https://rpc.example", 0));

        AssertDynamicCalls(methods);
        AssertBoundaries(chain, expectedParams);
        AssertInvalidData(chainBytes, paramsBytes, chain);
    }

    private static void AssertDynamicCalls(JsonElement[] methods)
    {
        foreach (var name in new[] { "", "a", new string('x', 32), new string('x', 33), "héllo🌐" })
        {
            var bytes = Encoding.UTF8.GetBytes(name);
            var padded = new byte[(bytes.Length + 31) / 32 * 32];
            bytes.CopyTo(padded, 0);
            var create = methods.Single(method => method.GetProperty("name").GetString() == "createChain");
            Assert.Equal(Join(Hex(create, "selector"), Word(128), Word(ulong.MaxValue), Word(ulong.MaxValue),
                Word(ushort.MaxValue), Word((ulong)bytes.Length), padded),
                Chains.EncodeCall("createChain", name, ulong.MaxValue, ulong.MaxValue, ushort.MaxValue));
            var byName = methods.Single(method => method.GetProperty("name").GetString() == "chainByName");
            Assert.Equal(Join(Hex(byName, "selector"), Word(32), Word((ulong)bytes.Length), padded),
                Chains.EncodeCall("chainByName", name));
        }
        var governance = methods.Single(method => method.GetProperty("name").GetString() == "governancePause");
        Assert.Equal(Join(Hex(governance, "selector"), Word(uint.MaxValue), Word(0)),
            Chains.EncodeCall("governancePause", uint.MaxValue, false));
    }

    private static void AssertBoundaries(ChainRecord chain, ChainRegistryParams parameters)
    {
        var maximum = (BigInteger.One << 256) - 1;
        var wideChain = chain with
        {
            Id = uint.MaxValue,
            Name = "héllo🌐" + new string('x', 33),
            Status = byte.MaxValue,
            ChainId = ulong.MaxValue,
            GasAllowance = ulong.MaxValue,
            ByteAllowance = ulong.MaxValue,
            FeeShareBps = ushort.MaxValue,
            Bond = maximum,
            BaseFeePerGas = maximum,
            CreatedHeight = ulong.MaxValue
        };
        Assert.Equal(wideChain, Chains.DecodeGetChain(Chains.EncodeGetChain(wideChain)));
        var wideParams = parameters with
        {
            MinBond = maximum,
            CreationFee = maximum,
            MinGasAllowance = ulong.MaxValue,
            MaxGasAllowance = ulong.MaxValue,
            TotalGasAllowanceLeft = ulong.MaxValue,
            MaxChains = uint.MaxValue
        };
        Assert.Equal(wideParams, Chains.DecodeParams(Chains.EncodeParams(wideParams)));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("missing"));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("fundChain"));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("chainCount", 1));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("fundChain", -1));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("fundChain", (BigInteger)uint.MaxValue + 1));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("createChain", "demo", BigInteger.One << 64, 1, 1));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("createChain", "demo", 1, BigInteger.One << 64, 1));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("createChain", "demo", 1, 1, 65536));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("governancePause", 1, 1));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("fundChain", 1.5));
        Assert.Throws<ArgumentException>(() => Chains.EncodeCall("setOperator", 1, "0x11"));
        Assert.Throws<ArgumentException>(() => Chains.EncodeGetChain(chain with { Bond = BigInteger.One << 256 }));
        Assert.Throws<ArgumentException>(() => Chains.EncodeParams(parameters with { MinBond = -1 }));
    }

    private static void AssertInvalidData(byte[] chainBytes, byte[] paramsBytes, ChainRecord chain)
    {
        Assert.Throws<ArgumentException>(() => Chains.DecodeGetChain(chainBytes[..^1]));
        Assert.Throws<ArgumentException>(() => Chains.DecodeGetChain(Join(chainBytes, new byte[32])));
        Assert.Throws<ArgumentException>(() => Chains.DecodeParams(paramsBytes[..^1]));
        Assert.Throws<ArgumentException>(() => Chains.DecodeParams(Join(paramsBytes, new byte[32])));
        foreach (var index in new[] { 0, 2, 3, 4, 5, 6, 7, 10 })
        {
            var malformed = chainBytes.ToArray();
            malformed[index * 32] = 1;
            Assert.Throws<ArgumentException>(() => Chains.DecodeGetChain(malformed));
        }
        var badOffset = chainBytes.ToArray();
        Word(32).CopyTo(badOffset, 32);
        Assert.Throws<ArgumentException>(() => Chains.DecodeGetChain(badOffset));
        var badPadding = chainBytes.ToArray();
        badPadding[^1] = 1;
        Assert.Throws<ArgumentException>(() => Chains.DecodeGetChain(badPadding));
        var badUtf8 = chainBytes.ToArray();
        badUtf8[12 * 32] = 255;
        Assert.Throws<DecoderFallbackException>(() => Chains.DecodeGetChain(badUtf8));
        foreach (var index in new[] { 0, 3, 4, 5, 6 })
        {
            var malformed = paramsBytes.ToArray();
            malformed[index * 32] = 1;
            Assert.Throws<ArgumentException>(() => Chains.DecodeParams(malformed));
        }
        var status = Chains.EncodeEvent(new ChainStatusChanged(chain.Id, chain.Status));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent([], status.Data));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent([new byte[32], status.Topics[1]], status.Data));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent([status.Topics[0], new byte[31]], status.Data));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent([.. status.Topics, status.Topics[1]], status.Data));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent(status.Topics, Word(256)));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent(status.Topics, []));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent([status.Topics[0], Word((ulong)uint.MaxValue + 1)], status.Data));
        var created = Chains.EncodeEvent(new ChainCreated(chain.Id, chain.Name, chain.Operator, chain.ChainId));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent(created.Topics[..2], created.Data));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent(created.Topics, created.Data[..^1]));
        var operatorChanged = Chains.EncodeEvent(new ChainOperatorChanged(chain.Id, chain.Operator));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent(operatorChanged.Topics, Word(0)));
        var funded = Chains.EncodeEvent(new ChainFunded(chain.Id, chain.Operator, chain.Bond));
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent(funded.Topics, []));
        var badAddress = funded.Topics.Select(topic => topic.ToArray()).ToArray();
        badAddress[2][0] = 1;
        Assert.Throws<ArgumentException>(() => Chains.DecodeEvent(badAddress, funded.Data));
    }

    private static object Argument(JsonElement value) => value.ValueKind switch
    {
        JsonValueKind.String => value.GetString()!,
        JsonValueKind.Number => BigInteger.Parse(value.GetRawText(), CultureInfo.InvariantCulture),
        JsonValueKind.True => true,
        JsonValueKind.False => false,
        _ => throw new InvalidOperationException("Unsupported vector argument")
    };

    private static BigInteger Number(JsonElement value, string name) => BigInteger.Parse(value.GetProperty(name).GetString()!, CultureInfo.InvariantCulture);
    private static byte[] Hex(JsonElement value, string name) => Convert.FromHexString(value.GetProperty(name).GetString()![2..]);
    private static byte[] Word(ulong value)
    {
        var bytes = new byte[32];
        System.Buffers.Binary.BinaryPrimitives.WriteUInt64BigEndian(bytes.AsSpan(24), value);
        return bytes;
    }
    private static byte[] Join(params byte[][] values) => values.SelectMany(value => value).ToArray();
    private static JsonDocument Load(string relative, [CallerFilePath] string source = "") => JsonDocument.Parse(File.ReadAllText(
        Path.GetFullPath(Path.Combine(Path.GetDirectoryName(source)!, "../../../../..", relative))));
}
