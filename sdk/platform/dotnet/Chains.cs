using System.Globalization;
using System.Numerics;
using System.Text;
using Org.BouncyCastle.Crypto.Digests;

namespace LayerX.Sdk;

public sealed record ChainRecord(uint Id, string Name, string Operator, byte Status, ulong ChainId,
    ulong GasAllowance, ulong ByteAllowance, ushort FeeShareBps, BigInteger Bond,
    BigInteger BaseFeePerGas, ulong CreatedHeight);

public sealed record ChainRegistryParams(string Authority, BigInteger MinBond, BigInteger CreationFee,
    ulong MinGasAllowance, ulong MaxGasAllowance, ulong TotalGasAllowanceLeft, uint MaxChains);

public abstract record ChainEvent(uint Id);
public sealed record ChainCreated(uint Id, string Name, string Operator, ulong ChainId) : ChainEvent(Id);
public sealed record ChainStatusChanged(uint Id, byte Status) : ChainEvent(Id);
public sealed record ChainFunded(uint Id, string From, BigInteger Amount) : ChainEvent(Id);
public sealed record ChainOperatorChanged(uint Id, string Operator) : ChainEvent(Id);

public static class Chains
{
    public const string Address = "0x000000000000000000000000000000000000101e";
    public const ulong VirtualChainIdBase = 125000000;

    private static readonly UTF8Encoding Utf8 = new(false, true);
    private static readonly IReadOnlyDictionary<string, string[]> Methods = new Dictionary<string, string[]>
    {
        ["createChain"] = ["string", "uint64", "uint64", "uint16"],
        ["fundChain"] = ["uint32"],
        ["pauseChain"] = ["uint32"],
        ["resumeChain"] = ["uint32"],
        ["retireChain"] = ["uint32"],
        ["governancePause"] = ["uint32", "bool"],
        ["setOperator"] = ["uint32", "address"],
        ["getChain"] = ["uint32"],
        ["chainByName"] = ["string"],
        ["chainCount"] = [],
        ["chainIdOf"] = ["uint32"],
        ["params"] = []
    };
    private static readonly IReadOnlyDictionary<string, string> Events = new Dictionary<string, string>
    {
        ["ChainCreated"] = "ChainCreated(uint32,string,address,uint64)",
        ["ChainStatusChanged"] = "ChainStatusChanged(uint32,uint8)",
        ["ChainFunded"] = "ChainFunded(uint32,address,uint256)",
        ["ChainOperatorChanged"] = "ChainOperatorChanged(uint32,address)"
    };

    public static byte[] Selector(string method)
    {
        if (!Methods.TryGetValue(method, out var types)) throw new ArgumentException("Unknown chain method", nameof(method));
        return Hash($"{method}({string.Join(',', types)})")[..4];
    }

    public static byte[] EventTopic(string name)
    {
        if (!Events.TryGetValue(name, out var signature)) throw new ArgumentException("Unknown chain event", nameof(name));
        return Hash(signature);
    }

    public static byte[] EncodeCall(string method, params object[] arguments)
    {
        var selector = Selector(method);
        var types = Methods[method];
        if (arguments.Length != types.Length) throw new ArgumentException("Chain method argument count", nameof(arguments));
        return ProgramsAbi.Encode(Convert.ToHexString(selector), types.Select((type, i) => EncodeValue(type, arguments[i])).ToArray());
    }

    public static ChainRecord DecodeGetChain(byte[] data)
    {
        var name = Utf8.GetString(ProgramsAbi.ReadBytes(data, 1, 11 * 32, int.MaxValue));
        return new ChainRecord((uint)ProgramsAbi.ReadWord(data, 0, 32), name, ReadAddress(data, 2),
            (byte)ProgramsAbi.ReadWord(data, 3, 8), (ulong)ProgramsAbi.ReadWord(data, 4, 64),
            (ulong)ProgramsAbi.ReadWord(data, 5, 64), (ulong)ProgramsAbi.ReadWord(data, 6, 64),
            (ushort)ProgramsAbi.ReadWord(data, 7, 16), ProgramsAbi.ReadWord(data, 8),
            ProgramsAbi.ReadWord(data, 9), (ulong)ProgramsAbi.ReadWord(data, 10, 64));
    }

    public static byte[] EncodeGetChain(ChainRecord chain) => ProgramsAbi.Encode("",
        ProgramsAbi.Number(chain.Id, 32), EncodeValue("string", chain.Name), EncodeValue("address", chain.Operator),
        ProgramsAbi.Number(chain.Status, 8), ProgramsAbi.Number(chain.ChainId, 64),
        ProgramsAbi.Number(chain.GasAllowance, 64), ProgramsAbi.Number(chain.ByteAllowance, 64),
        ProgramsAbi.Number(chain.FeeShareBps, 16), ProgramsAbi.Number(chain.Bond),
        ProgramsAbi.Number(chain.BaseFeePerGas), ProgramsAbi.Number(chain.CreatedHeight, 64));

    public static ChainRegistryParams DecodeParams(byte[] data)
    {
        RequireLength(data, 7 * 32);
        return new ChainRegistryParams(ReadAddress(data, 0), ProgramsAbi.ReadWord(data, 1),
            ProgramsAbi.ReadWord(data, 2), (ulong)ProgramsAbi.ReadWord(data, 3, 64),
            (ulong)ProgramsAbi.ReadWord(data, 4, 64), (ulong)ProgramsAbi.ReadWord(data, 5, 64),
            (uint)ProgramsAbi.ReadWord(data, 6, 32));
    }

    public static byte[] EncodeParams(ChainRegistryParams parameters) => ProgramsAbi.Encode("",
        EncodeValue("address", parameters.Authority), ProgramsAbi.Number(parameters.MinBond),
        ProgramsAbi.Number(parameters.CreationFee), ProgramsAbi.Number(parameters.MinGasAllowance, 64),
        ProgramsAbi.Number(parameters.MaxGasAllowance, 64), ProgramsAbi.Number(parameters.TotalGasAllowanceLeft, 64),
        ProgramsAbi.Number(parameters.MaxChains, 32));

    public static ChainEvent DecodeEvent(IReadOnlyList<byte[]> topics, byte[] data)
    {
        if (topics.Count < 2 || topics.Any(topic => topic.Length != 32)) throw new ArgumentException("Chain event topics", nameof(topics));
        var name = Events.Keys.SingleOrDefault(name => topics[0].SequenceEqual(EventTopic(name)))
            ?? throw new ArgumentException("Unknown chain event topic", nameof(topics));
        var expectedTopics = name == "ChainStatusChanged" ? 2 : 3;
        if (topics.Count != expectedTopics) throw new ArgumentException("Chain event topic count", nameof(topics));
        var id = (uint)ProgramsAbi.ReadWord(topics[1], 0, 32);
        switch (name)
        {
            case "ChainCreated":
                var chainName = Utf8.GetString(ProgramsAbi.ReadBytes(data, 0, 64, int.MaxValue));
                return new ChainCreated(id, chainName, ReadAddress(topics[2], 0), (ulong)ProgramsAbi.ReadWord(data, 1, 64));
            case "ChainStatusChanged":
                RequireLength(data, 32);
                return new ChainStatusChanged(id, (byte)ProgramsAbi.ReadWord(data, 0, 8));
            case "ChainFunded":
                RequireLength(data, 32);
                return new ChainFunded(id, ReadAddress(topics[2], 0), ProgramsAbi.ReadWord(data, 0));
            case "ChainOperatorChanged":
                RequireLength(data, 0);
                return new ChainOperatorChanged(id, ReadAddress(topics[2], 0));
            default:
                throw new ArgumentException("Unknown chain event", nameof(topics));
        }
    }

    public static (byte[][] Topics, byte[] Data) EncodeEvent(ChainEvent value)
    {
        var id = ProgramsAbi.Word(value.Id, 32);
        return value switch
        {
            ChainCreated created => ([EventTopic("ChainCreated"), id, EncodeValue("address", created.Operator).Data],
                ProgramsAbi.Encode("", EncodeValue("string", created.Name), ProgramsAbi.Number(created.ChainId, 64))),
            ChainStatusChanged changed => ([EventTopic("ChainStatusChanged"), id], ProgramsAbi.Word(changed.Status, 8)),
            ChainFunded funded => ([EventTopic("ChainFunded"), id, EncodeValue("address", funded.From).Data], ProgramsAbi.Word(funded.Amount)),
            ChainOperatorChanged changed => ([EventTopic("ChainOperatorChanged"), id, EncodeValue("address", changed.Operator).Data], []),
            _ => throw new ArgumentException("Unknown chain event", nameof(value))
        };
    }

    public static ulong VirtualChainId(uint id)
    {
        if (id == 0) throw new ArgumentOutOfRangeException(nameof(id));
        return VirtualChainIdBase + id;
    }

    public static string ChainRpcUrl(string baseUrl, uint id)
    {
        _ = VirtualChainId(id);
        return $"{baseUrl.TrimEnd('/')}/chain/{id.ToString(CultureInfo.InvariantCulture)}";
    }

    private static (byte[] Data, bool Dynamic) EncodeValue(string type, object value)
    {
        if (type == "string" && value is string text) return ProgramsAbi.Bytes(Utf8.GetBytes(text));
        if (type == "bool" && value is bool boolean) return ProgramsAbi.Number(boolean ? 1 : 0, 1);
        if (type == "address" && value is string address)
        {
            if (address.Length != 42 || !address.StartsWith("0x", StringComparison.Ordinal)) throw new ArgumentException("ABI address");
            var bytes = Convert.FromHexString(address[2..]);
            return ProgramsAbi.Fixed([.. new byte[12], .. bytes]);
        }
        if (type.StartsWith("uint", StringComparison.Ordinal))
        {
            var number = value switch
            {
                BigInteger n => n,
                byte n => new BigInteger(n),
                ushort n => new BigInteger(n),
                uint n => new BigInteger(n),
                ulong n => new BigInteger(n),
                sbyte n => new BigInteger(n),
                short n => new BigInteger(n),
                int n => new BigInteger(n),
                long n => new BigInteger(n),
                _ => throw new ArgumentException("ABI integer type")
            };
            return ProgramsAbi.Number(number, int.Parse(type[4..], CultureInfo.InvariantCulture));
        }
        throw new ArgumentException($"Invalid ABI value for {type}");
    }

    private static string ReadAddress(byte[] data, int index)
    {
        _ = ProgramsAbi.ReadWord(data, index, 160);
        return "0x" + Convert.ToHexString(data.AsSpan(index * 32 + 12, 20)).ToLowerInvariant();
    }

    private static void RequireLength(byte[] data, int length)
    {
        if (data.Length != length) throw new ArgumentException("ABI output length", nameof(data));
    }

    private static byte[] Hash(string signature)
    {
        var bytes = Encoding.ASCII.GetBytes(signature);
        var digest = new KeccakDigest(256);
        digest.BlockUpdate(bytes, 0, bytes.Length);
        var result = new byte[32];
        digest.DoFinal(result, 0);
        return result;
    }
}
