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

    [Fact]
    public async Task ProgramDiscoverReadsWindDownLifecycle()
    {
        (bool Active, uint Status, int Length, ProgramLifecycle? Lifecycle)[] scenarios =
        [
            (true, 1, 192, ProgramLifecycle.Active),
            (false, 2, 192, ProgramLifecycle.Deprecated),
            (false, 3, 192, ProgramLifecycle.Tombstoned),
            (false, 1, 192, null), (false, 0, 192, null),
            (false, 4, 192, null), (false, 258, 192, null),
            (false, 2, 0, null), (false, 2, 31, null),
            (false, 2, 32, null), (false, 3, 191, null)
        ];
        foreach (var scenario in scenarios)
        {
            var programId = new byte[32]; programId[0] = 0x42;
            var head = new byte[192];
            head[63] = 0x11; head[95] = 0x22; head[127] = 1; head[159] = 7;
            head[191] = scenario.Active ? (byte)1 : (byte)0;
            var windDown = new byte[192];
            System.Buffers.Binary.BinaryPrimitives.WriteUInt32BigEndian(windDown.AsSpan(28, 4), scenario.Status);
            Array.Fill(windDown, (byte)0x33, 32, 32);
            for (var i = 2; i < 6; i++) windDown[i * 32 + 31] = (byte)(9 - i);
            var getProgram = "0xb1ffb3d4" + Convert.ToHexString(programId).ToLowerInvariant();
            var getWindDown = "0x98e4e41d" + Convert.ToHexString(programId).ToLowerInvariant();
            using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
            using var listener = new TcpListener(IPAddress.Loopback, 0);
            listener.Start();
            using var rpc = new Programs(new Uri($"http://127.0.0.1:{((IPEndPoint)listener.LocalEndpoint).Port}/rpc"));
            var programs = new ProgramsClient(rpc);
            var requests = new List<JsonElement>();
            var expectedCalls = scenario.Active ? 1 : 2;

            async Task ServeAsync()
            {
                for (var i = 0; i < expectedCalls; i++)
                {
                    using var connection = await listener.AcceptTcpClientAsync(timeout.Token);
                    await using var stream = connection.GetStream();
                    using var reader = new StreamReader(stream, Encoding.UTF8, leaveOpen: true);
                    Assert.Equal("POST /rpc HTTP/1.1", await reader.ReadLineAsync(timeout.Token));
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
                    using var document = JsonDocument.Parse(new string(chars));
                    var request = document.RootElement;
                    requests.Add(request.Clone());
                    Assert.Equal("eth_call", request.GetProperty("method").GetString());
                    var calldata = request.GetProperty("params")[0].GetProperty("data").GetString();
                    var answer = calldata switch
                    {
                        var value when value == getProgram => head,
                        var value when value == getWindDown => windDown[..scenario.Length],
                        _ => throw new InvalidOperationException("Unexpected calldata: " + calldata)
                    };
                    var response = JsonSerializer.SerializeToUtf8Bytes(new
                    {
                        jsonrpc = "2.0", id = request.GetProperty("id").GetInt64(),
                        result = "0x" + Convert.ToHexString(answer).ToLowerInvariant()
                    });
                    var headers = Encoding.ASCII.GetBytes($"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {response.Length}\r\nConnection: close\r\n\r\n");
                    await stream.WriteAsync(headers, timeout.Token);
                    await stream.WriteAsync(response, timeout.Token);
                }
            }

            var server = ServeAsync();
            var discovery = programs.DiscoverAsync(programId, "receipted", timeout.Token);
            if (scenario.Lifecycle is null)
            {
                var error = await Assert.ThrowsAsync<PlatformSdkException>(() => discovery);
                Assert.Equal(SdkErrorCode.DecodeFailure, error.Code);
            }
            else
            {
                var result = await discovery;
                Assert.Equal(scenario.Lifecycle.Value, result.Lifecycle);
                Assert.Equal(programId, result.ProgramId);
                Assert.Equal(7U, result.Version); Assert.Equal((ushort)1, result.AbiVersion);
                Assert.Equal(head[64..96], result.CodeHash);
            }
            await server;
            Assert.Equal(expectedCalls, requests.Count);
            Assert.False(listener.Pending());
            for (var i = 0; i < requests.Count; i++)
            {
                Assert.Equal("2.0", requests[i].GetProperty("jsonrpc").GetString());
                var parameters = requests[i].GetProperty("params");
                Assert.Equal(2, parameters.GetArrayLength());
                Assert.Equal("pending", parameters[1].GetString());
                Assert.Equal(Programs.AddressValue, parameters[0].GetProperty("to").GetString());
                Assert.Equal(i == 0 ? getProgram : getWindDown, parameters[0].GetProperty("data").GetString());
            }
        }
    }

    [Fact]
    public async Task UpgradeSubmissionBindsPreparedFeeLimit()
    {
        using var fixture = Fixture("native-program-upgrade-v4.json");
        var vector = fixture.RootElement;
        var upgrade = NativeProgramUpgrade.Decode(Hex(vector, "payload_hex"));
        var feeLimit = new ProtocolAmount("1000");
        using var receiptFixture = Fixture("sequencer-receipt-v1.json");
        var receiptVector = receiptFixture.RootElement.GetProperty("cases")[0];
        var pinned = receiptVector.GetProperty("pinnedKey");
        byte[] ReceiptHex(JsonElement value, string field) => Convert.FromHexString(value.GetProperty(field).GetString()![2..]);
        var keys = new ReceiptKeys([new ReceiptKey(ReceiptHex(pinned, "keyId"), ReceiptHex(pinned, "publicKey"),
            pinned.GetProperty("firstHeight").GetUInt64(), pinned.GetProperty("lastHeight").GetUInt64())],
            ReceiptHex(receiptVector, "prevSeqHash"));
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
        using var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        var endpoint = new Uri($"http://127.0.0.1:{((IPEndPoint)listener.LocalEndpoint).Port}/rpc");
        using var rpc = new Programs(endpoint);
        var programs = new ProgramsClient(rpc, keys);

        async Task<JsonDocument> ReceiveRequestAsync(string method, string? result = null)
        {
            using var connection = await listener.AcceptTcpClientAsync(timeout.Token);
            await using var stream = connection.GetStream();
            using var reader = new StreamReader(stream, Encoding.UTF8, leaveOpen: true);
            Assert.Equal("POST /rpc HTTP/1.1", await reader.ReadLineAsync(timeout.Token));
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
            var request = JsonDocument.Parse(new string(chars));
            Assert.Equal("2.0", request.RootElement.GetProperty("jsonrpc").GetString());
            Assert.Equal(method, request.RootElement.GetProperty("method").GetString());
            if (result is not null)
            {
                var response = JsonSerializer.SerializeToUtf8Bytes(new
                {
                    jsonrpc = "2.0", id = request.RootElement.GetProperty("id").GetInt64(), result
                });
                var headers = Encoding.ASCII.GetBytes($"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {response.Length}\r\nConnection: close\r\n\r\n");
                await stream.WriteAsync(headers, timeout.Token);
                await stream.WriteAsync(response, timeout.Token);
            }
            return request;
        }

        var preparation = programs.UpgradeAsync(upgrade, feeLimit, Sender, 100, 2, timeout.Token);
        using var nonceRequest = await ReceiveRequestAsync("eth_getTransactionCount", "0x7");
        Assert.Equal(Sender, nonceRequest.RootElement.GetProperty("params")[0].GetString());
        Assert.Equal("pending", nonceRequest.RootElement.GetProperty("params")[1].GetString());
        using var gasRequest = await ReceiveRequestAsync("eth_estimateGas", "0x3d090");
        Assert.Equal("pending", gasRequest.RootElement.GetProperty("params")[1].GetString());
        Assert.Equal(vector.GetProperty("evm").GetProperty("calldata").GetString(),
            gasRequest.RootElement.GetProperty("params")[0].GetProperty("data").GetString());
        var transaction = await preparation;
        AssertTransaction(transaction, vector.GetProperty("evm").GetProperty("calldata").GetString()!, 250_000);
        Assert.Equal(7UL, transaction.Nonce);
        Assert.Equal(new BigInteger(100), transaction.MaxFeePerGas);
        Assert.Equal(new BigInteger(2), transaction.MaxPriorityFeePerGas);

        byte[] Wrap(byte[] bytes, int prefix)
        {
            if (bytes.Length < 56) return [(byte)(prefix + bytes.Length), .. bytes];
            var length = new BigInteger(bytes.Length).ToByteArray(true, true);
            return [(byte)(prefix + 55 + length.Length), .. length, .. bytes];
        }
        byte[] RlpBytes(byte[] bytes) => bytes.Length == 1 && bytes[0] < 0x80 ? bytes : Wrap(bytes, 0x80);
        byte[] RlpNumber(BigInteger number) => RlpBytes(number == 0 ? [] : number.ToByteArray(true, true));
        byte[] signed = [2, .. Wrap([
            .. RlpNumber(transaction.ChainId), .. RlpNumber(transaction.Nonce),
            .. RlpNumber(transaction.MaxPriorityFeePerGas), .. RlpNumber(transaction.MaxFeePerGas),
            .. RlpNumber(transaction.Gas), .. RlpBytes(Convert.FromHexString(transaction.To[2..])),
            .. RlpNumber(transaction.Value), .. RlpBytes(transaction.Data), 0xc0,
            .. RlpNumber(0), .. RlpNumber(1), .. RlpNumber(1)], 0xc0)];
        var key = new IdempotencyKey(Convert.ToHexString(Programs.TransactionHash(signed)).ToLowerInvariant());
        var submission = programs.UpgradeAsync(upgrade, feeLimit, signed, key, timeout.Token);
        using var sendRequest = await ReceiveRequestAsync("eth_sendRawTransaction");
        var parameters = sendRequest.RootElement.GetProperty("params");
        Assert.Equal(1, parameters.GetArrayLength());
        Assert.Equal("0x" + Convert.ToHexString(signed).ToLowerInvariant(), parameters[0].GetString());
        var disconnected = await Assert.ThrowsAsync<PlatformSdkException>(() => submission);
        Assert.Equal(SdkErrorCode.UnknownOutcome, disconnected.Code);
        Assert.False(listener.Pending());

        var mismatch = await Assert.ThrowsAsync<ArgumentException>(() =>
            programs.UpgradeAsync(upgrade, new ProtocolAmount("0"), signed, key, timeout.Token));
        Assert.Equal("Signed transaction does not match the program operation", mismatch.Message);
        Assert.False(listener.Pending());
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
