using System.Net;
using System.Net.Sockets;
using System.Numerics;
using System.Text;
using System.Text.Json;
using LayerX.Sdk;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class ProgramsEvmTests
{
    private const string Sender = "0x2222222222222222222222222222222222222222";

    [Fact]
    public void ProgramCallBuildsLayerxprogramsCallTransaction()
    {
        using var fixture = Fixture("native-program-call-v4.json");
        var vector = fixture.RootElement;
        var native = NativeProgramCall.Decode(Hex(vector, "payload_hex"));
        var call = new ProgramCall(native, new ProtocolAmount(vector.GetProperty("fee_limit").GetString()!));
        var transaction = call.BuildTransaction(Sender, 7, 150_000, 100, 2);
        AssertTransaction(transaction, vector.GetProperty("evm").GetProperty("calldata").GetString()!, 150_000);
        Assert.Equal("0x54bd2b48", vector.GetProperty("evm").GetProperty("selector").GetString());
        Assert.Equal(7UL, transaction.Nonce);
        Assert.Equal(new BigInteger(100), transaction.MaxFeePerGas);
        Assert.Equal(new BigInteger(2), transaction.MaxPriorityFeePerGas);
        Assert.Empty(call.SignedTransaction);
        Assert.Throws<ArgumentException>(() => _ = call.TransactionHash);
        Assert.Throws<ArgumentException>(() => call.BuildTransaction(Sender, 7, 150_000, 1, 2));
        Assert.Throws<ArgumentException>(() => call.BuildTransaction("0x101d", 7, 150_000, 100, 2));
        var changed = new ProgramCall(native with { Entrypoint = "another_entry" }, call.Budget.FeeLimit);
        Assert.NotEqual(transaction.Data, changed.BuildTransaction(Sender, 7, 150_000, 100, 2).Data);
    }

    [Fact]
    public async Task ProgramSimulateUsesPendingEthCall()
    {
        using var fixture = Fixture("native-program-call-v4.json");
        var vector = fixture.RootElement;
        var call = new ProgramCall(NativeProgramCall.Decode(Hex(vector, "payload_hex")),
            new ProtocolAmount(vector.GetProperty("fee_limit").GetString()!));
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
        using var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        var endpoint = new Uri($"http://127.0.0.1:{((IPEndPoint)listener.LocalEndpoint).Port}/rpc");
        using var rpc = new Programs(endpoint);
        var programs = new ProgramsClient(rpc);
        var simulation = programs.SimulateAsync(call, timeout.Token);
        string requestLine; string body;
        using (var connection = await listener.AcceptTcpClientAsync(timeout.Token))
        {
            await using var stream = connection.GetStream();
            using var reader = new StreamReader(stream, Encoding.UTF8, leaveOpen: true);
            requestLine = (await reader.ReadLineAsync(timeout.Token))!;
            var contentLength = -1; string? line;
            while (!string.IsNullOrEmpty(line = await reader.ReadLineAsync(timeout.Token)))
                if (line.StartsWith("Content-Length:", StringComparison.OrdinalIgnoreCase))
                    contentLength = int.Parse(line["Content-Length:".Length..].Trim(), System.Globalization.CultureInfo.InvariantCulture);
            Assert.InRange(contentLength, 1, 16_384);
            var chars = new char[contentLength];
            var offset = 0;
            while (offset < chars.Length)
            {
                var read = await reader.ReadAsync(chars.AsMemory(offset), timeout.Token);
                Assert.True(read > 0, "The real HTTP request ended before its declared body length.");
                offset += read;
            }
            body = new string(chars);
        }
        listener.Stop();
        var error = await Assert.ThrowsAsync<PlatformSdkException>(() => simulation);
        Assert.Equal(SdkErrorCode.TransportFailure, error.Code);
        Assert.Equal("POST /rpc HTTP/1.1", requestLine);
        Assert.DoesNotContain("/v1/programs", requestLine);
        using var request = JsonDocument.Parse(body);
        var root = request.RootElement;
        Assert.Equal("2.0", root.GetProperty("jsonrpc").GetString());
        Assert.Equal("eth_call", root.GetProperty("method").GetString());
        var parameters = root.GetProperty("params");
        Assert.Equal(2, parameters.GetArrayLength());
        Assert.Equal("pending", parameters[1].GetString());
        Assert.Equal(Programs.AddressValue, parameters[0].GetProperty("to").GetString());
        var expected = "0x3044bd56" + vector.GetProperty("evm").GetProperty("calldata").GetString()![10..];
        Assert.Equal(expected, parameters[0].GetProperty("data").GetString());
        Assert.Equal(expected, "0x" + Convert.ToHexString(call.EncodeCalldata(simulate: true)).ToLowerInvariant());
    }

    [Fact]
    public void ProgramDeployBuildsLayerxprogramsDeployTransaction()
    {
        using var fixture = Fixture("native-program-deploy-v4.json");
        var vector = fixture.RootElement;
        var deploy = NativeProgramDeploy.Decode(Hex(vector, "payload_hex"));
        var request = new NativeProgramLifecycleRequest(deploy);
        var transaction = request.BuildTransaction(Sender, 7, 250_000, 100, 2);
        AssertTransaction(transaction, vector.GetProperty("evm").GetProperty("calldata").GetString()!, 250_000);
        Assert.Equal((ushort)1, request.Ordinal);
        Assert.Equal(transaction.Data, request.Payload);
        Assert.Empty(request.SignedTransaction);
        Assert.Throws<ArgumentException>(() => _ = request.TransactionHash);
        Assert.Throws<ArgumentException>(() => new NativeProgramLifecycleRequest(deploy, Hex(vector, "signed_activity_hex")));

        using var upgradeFixture = Fixture("native-program-upgrade-v4.json");
        var upgradeVector = upgradeFixture.RootElement;
        var upgrade = new NativeProgramLifecycleRequest(NativeProgramUpgrade.Decode(Hex(upgradeVector, "payload_hex")), new ProtocolAmount("1000"));
        Assert.Equal(upgradeVector.GetProperty("evm").GetProperty("calldata").GetString(),
            "0x" + Convert.ToHexString(upgrade.Payload).ToLowerInvariant());
        Assert.Equal((ushort)2, upgrade.Ordinal);
    }

    private static void AssertTransaction(UnsignedProgramTransaction transaction, string expectedCalldata, ulong gas)
    {
        Assert.Equal(2, transaction.Type);
        Assert.Equal(125UL, transaction.ChainId);
        Assert.Equal(Programs.AddressValue, transaction.To);
        Assert.Equal(Sender, transaction.From);
        Assert.Equal(BigInteger.Zero, transaction.Value);
        Assert.Equal(gas, transaction.Gas);
        Assert.Equal(expectedCalldata, "0x" + Convert.ToHexString(transaction.Data).ToLowerInvariant());
        Assert.Equal((byte)2, transaction.SigningPayload[0]);
        var json = Assert.IsType<JsonValue.ObjectValue>(transaction.ToJson()).Value;
        Assert.Equal("0x2", Assert.IsType<JsonValue.StringValue>(json["type"]).Value);
        Assert.Equal("0x7d", Assert.IsType<JsonValue.StringValue>(json["chainId"]).Value);
        Assert.Equal(expectedCalldata, Assert.IsType<JsonValue.StringValue>(json["data"]).Value);
        Assert.Equal("0x0", Assert.IsType<JsonValue.StringValue>(json["value"]).Value);
        Assert.Empty(Assert.IsType<JsonValue.ArrayValue>(json["accessList"]).Value);
        Assert.False(json.ContainsKey("r")); Assert.False(json.ContainsKey("s")); Assert.False(json.ContainsKey("v"));
        var data = transaction.Data; data[0] ^= 1;
        Assert.Equal(expectedCalldata, "0x" + Convert.ToHexString(transaction.Data).ToLowerInvariant());
    }

    private static byte[] Hex(JsonElement value, string field) => Convert.FromHexString(value.GetProperty(field).GetString()!);
    private static JsonDocument Fixture(string name)
    {
        var directory = new DirectoryInfo(AppContext.BaseDirectory);
        while (directory is not null)
        {
            var path = Path.Combine(directory.FullName, "sdk", "platform", "conformance", "fixtures", name);
            if (File.Exists(path)) return JsonDocument.Parse(File.ReadAllBytes(path));
            directory = directory.Parent;
        }
        throw new FileNotFoundException(name);
    }
}
