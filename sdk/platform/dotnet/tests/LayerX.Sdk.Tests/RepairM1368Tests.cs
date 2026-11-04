using System.Net;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using LayerX.Sdk;
using Xunit;

namespace LayerX.Sdk.Tests;

internal sealed class RepairM1368Http : IAsyncDisposable
{
    private readonly TcpListener listener = new(IPAddress.Loopback, 0);
    private readonly CancellationTokenSource timeout = new(TimeSpan.FromSeconds(15));
    internal Uri Endpoint { get; }

    internal RepairM1368Http()
    {
        listener.Start();
        Endpoint = new Uri($"http://127.0.0.1:{((IPEndPoint)listener.LocalEndpoint).Port}/");
    }

    internal async Task Exchange(Func<JsonElement, (int Status, byte[] Body)> response, string path = "/")
    {
        using var connection = await listener.AcceptTcpClientAsync(timeout.Token);
        await using var stream = connection.GetStream();
        using var reader = new StreamReader(stream, Encoding.UTF8, leaveOpen: true);
        Assert.Equal($"POST {path} HTTP/1.1", await reader.ReadLineAsync(timeout.Token));
        var length = -1;
        string? line;
        while (!string.IsNullOrEmpty(line = await reader.ReadLineAsync(timeout.Token)))
            if (line.StartsWith("Content-Length:", StringComparison.OrdinalIgnoreCase))
                length = int.Parse(line["Content-Length:".Length..], System.Globalization.CultureInfo.InvariantCulture);
        Assert.InRange(length, 1, 2_000_000);
        var body = new char[length];
        var offset = 0;
        while (offset < length)
        {
            var count = await reader.ReadAsync(body.AsMemory(offset), timeout.Token);
            Assert.True(count > 0);
            offset += count;
        }
        using var document = JsonDocument.Parse(new string(body));
        var reply = response(document.RootElement);
        var headers = Encoding.ASCII.GetBytes($"HTTP/1.1 {reply.Status} Result\r\nContent-Type: application/json\r\nContent-Length: {reply.Body.Length}\r\nConnection: close\r\n\r\n");
        await stream.WriteAsync(headers, timeout.Token);
        await stream.WriteAsync(reply.Body, timeout.Token);
    }

    public ValueTask DisposeAsync()
    {
        timeout.Cancel();
        listener.Stop();
        timeout.Dispose();
        return ValueTask.CompletedTask;
    }
}

public sealed class RepairM1368Tests
{
    private const string Sender = "0x2222222222222222222222222222222222222222";
    private static readonly byte[] ProgramId = Enumerable.Repeat((byte)0x11, 32).ToArray();

    private static byte[] Wasm(int length)
    {
        // A valid module containing one custom section, so boundary sizes do not rely on invalid WASM padding.
        var bytes = new byte[length];
        new byte[] { 0, 97, 115, 109, 1, 0, 0, 0 }.CopyTo(bytes, 0);
        var contentLength = length - 12;
        bytes[8] = 0;
        bytes[9] = (byte)((contentLength & 127) | 128);
        bytes[10] = (byte)(((contentLength >> 7) & 127) | 128);
        bytes[11] = (byte)(contentLength >> 14);
        return bytes;
    }

    [Fact]
    public async Task CompleteLifecyclePayloadIsBoundedBeforeNativePreparation()
    {
        foreach (var interfaceLength in new[] { -1, 0, 1, 952 })
        {
            // Deploy has no empty-interface representation; upgrade supports it when clearing the interface.
            foreach (var upgrade in new[] { false, true })
            {
                if (!upgrade && interfaceLength == 0) continue;
                var programInterface = interfaceLength < 0 ? null : new byte[interfaceLength];
                var hook = upgrade ? new byte[] { 1, 2, 3 } : [];
                var overhead = (upgrade ? 106 : 104) + hook.Length + (programInterface is null ? 0 : 4 + programInterface.Length);
                INativeProgramLifecycle Create(int length)
                {
                    var wasm = Wasm(length);
                    return upgrade
                        ? new NativeProgramUpgrade(ProgramId, 2, new byte[32], SHA256.HashData(wasm), hook,
                            interfaceLength == 0, programInterface, wasm)
                        : new NativeProgramDeploy(ProgramId, 2, 0, new byte[32], SHA256.HashData(wasm), programInterface, wasm);
                }
                foreach (var delta in new[] { -1, 0 })
                {
                    var operation = Create(524288 - overhead + delta);
                    var payload = operation.Encode();
                    Assert.Equal(524288 + delta, payload.Length);
                    INativeProgramLifecycle decoded = upgrade ? NativeProgramUpgrade.Decode(payload) : NativeProgramDeploy.Decode(payload);
                    Assert.Equal(payload, decoded.Encode());
                    var request = new NativeProgramLifecycleRequest(decoded);
                    await using var server = new RepairM1368Http();
                    using var rpc = new Programs(server.Endpoint);
                    var client = new ProgramsClient(rpc);
                    var preparation = upgrade
                        ? client.UpgradeAsync((NativeProgramUpgrade)decoded, default, Sender, 100, 2)
                        : client.DeployAsync((NativeProgramDeploy)decoded, Sender, 100, 2);
                    var nonce = server.Exchange(call =>
                    {
                        Assert.Equal("eth_getTransactionCount", call.GetProperty("method").GetString());
                        Assert.Equal(Sender, call.GetProperty("params")[0].GetString());
                        return (200, JsonSerializer.SerializeToUtf8Bytes(new { jsonrpc = "2.0", id = call.GetProperty("id"), result = "0x7" }));
                    });
                    await nonce;
                    await server.Exchange(call =>
                    {
                        Assert.Equal("eth_estimateGas", call.GetProperty("method").GetString());
                        Assert.Equal("pending", call.GetProperty("params")[1].GetString());
                        Assert.Equal("0x" + Convert.ToHexString(request.Payload).ToLowerInvariant(),
                            call.GetProperty("params")[0].GetProperty("data").GetString());
                        return (200, JsonSerializer.SerializeToUtf8Bytes(new { jsonrpc = "2.0", id = call.GetProperty("id"), result = "0x3d090" }));
                    });
                    var transaction = await preparation;
                    Assert.Equal(7UL, transaction.Nonce);
                    Assert.Equal(250000UL, transaction.Gas);
                    Assert.Equal(request.Payload, transaction.Data);
                }
                Assert.Throws<ArgumentException>(() => Create(524289 - overhead));
                var oversized = new byte[524289];
                Assert.Throws<ArgumentException>(() => upgrade ? (INativeProgramLifecycle)NativeProgramUpgrade.Decode(oversized) : NativeProgramDeploy.Decode(oversized));
            }
        }
    }

    [Fact]
    public async Task SupportedProgramsConstructorsUseTheNativeBackend()
    {
        var platform = new PlatformClient(new AgentHttpTransport(new Uri("http://127.0.0.1:1/")));
        var retired = Assert.Throws<PlatformSdkException>(() => new ProgramsClient(platform, Enumerable.Repeat((byte)1, 32).ToArray()));
        Assert.Equal(SdkErrorCode.UnavailableCapability, retired.Code);
        Assert.Equal(RetryClass.Never, retired.Retry);
        for (var migration = 0; migration < 2; migration++)
        {
            await using var server = new RepairM1368Http();
            using var rpc = new Programs(server.Endpoint);
            var client = migration == 0 ? new ProgramsClient(rpc)
                : new ProgramsClient(platform, rpc, Enumerable.Repeat((byte)1, 32).ToArray());
            var discovery = client.DiscoverAsync(ProgramId, "receipted");
            await server.Exchange(call =>
            {
                Assert.Equal("eth_call", call.GetProperty("method").GetString());
                Assert.Equal("pending", call.GetProperty("params")[1].GetString());
                var transaction = call.GetProperty("params")[0];
                Assert.Equal(Programs.AddressValue, transaction.GetProperty("to").GetString());
                Assert.Equal("0xb1ffb3d4" + Convert.ToHexString(ProgramId).ToLowerInvariant(), transaction.GetProperty("data").GetString());
                var head = new byte[192];
                head[31] = 1;
                Enumerable.Repeat((byte)0x22, 32).ToArray().CopyTo(head, 64);
                head[127] = 2;
                head[159] = 7;
                head[191] = 1;
                return (200, JsonSerializer.SerializeToUtf8Bytes(new { jsonrpc = "2.0", id = call.GetProperty("id"), result = "0x" + Convert.ToHexString(head).ToLowerInvariant() }));
            });
            var result = await discovery;
            Assert.Equal(ProgramId, result.ProgramId);
            Assert.Equal(ProgramLifecycle.Active, result.Lifecycle);
            Assert.Equal(7U, result.Version);
            Assert.Equal((ushort)2, result.AbiVersion);
            Assert.Equal(Enumerable.Repeat((byte)0x22, 32), result.CodeHash);
            Assert.Equal("pending-eth-call", result.Verification);
            Assert.Empty(result.ReceiptDigest);
            Assert.Empty(result.StateRoot);
            Assert.Equal(0UL, result.ObservedSequence);
            Assert.Equal(0UL, result.ObservedAt);
            Assert.Equal(0UL, result.ValidThrough);
        }
    }

    [Fact]
    public async Task ReplayHistoryIsBoundedAndRecovered()
    {
        var stream = new ResumableStream<int>(new StreamCursor("0"));
        var capacity = ResumableStream<int>.ReplayWindowCapacity;
        for (var index = 0; index < capacity * 3; index++)
        {
            var current = new StreamCursor(index.ToString(System.Globalization.CultureInfo.InvariantCulture));
            var next = new StreamCursor((index + 1).ToString(System.Globalization.CultureInfo.InvariantCulture));
            Assert.Single(stream.Accept(new(current, [new("event-" + index, current, next, index)], next)));
            Assert.InRange(stream.RecoveryState.EventIds.Count, 1, capacity);
        }
        var snapshot = stream.RecoveryState;
        Assert.Equal(capacity, snapshot.EventIds.Count);
        Assert.Equal("event-" + capacity * 2, snapshot.EventIds[0]);
        var path = Path.Combine(Path.GetTempPath(), "repair-m1368-" + Guid.NewGuid().ToString("N") + ".json");
        try
        {
            await File.WriteAllBytesAsync(path, JsonSerializer.SerializeToUtf8Bytes(snapshot));
            // Discard the consumer and reopen only its durable cursor and bounded replay window.
            stream = ResumableStream<int>.Restore(JsonSerializer.Deserialize<StreamRecoveryState>(await File.ReadAllBytesAsync(path))!);
            Assert.Equal(snapshot.Cursor, stream.Cursor);
            Assert.Equal(snapshot.EventIds, stream.RecoveryState.EventIds);
            foreach (var duplicate in snapshot.EventIds)
            {
                var cursor = stream.Cursor;
                var error = Assert.Throws<PlatformSdkException>(() => stream.Accept(new(cursor,
                    [new(duplicate, cursor, new StreamCursor("after-reopen"), 0)], new StreamCursor("after-reopen"))));
                Assert.Equal(SdkErrorCode.DecodeFailure, error.Code);
                Assert.Equal(snapshot.Cursor, stream.Cursor);
                Assert.Equal(snapshot.EventIds, stream.RecoveryState.EventIds);
            }
            var accepted = stream.Accept(new(stream.Cursor,
                [new("new-event", stream.Cursor, new StreamCursor("after-reopen"), 123)], new StreamCursor("after-reopen")));
            Assert.Equal(123, Assert.Single(accepted).Value);
            Assert.Equal(capacity, stream.RecoveryState.EventIds.Count);
            Assert.Equal(snapshot.EventIds[1], stream.RecoveryState.EventIds[0]);
            Assert.Throws<PlatformSdkException>(() => ResumableStream<int>.Restore(new StreamRecoveryState(stream.Cursor,
                Enumerable.Repeat("duplicate", 2).ToArray())));
            Assert.Throws<PlatformSdkException>(() => ResumableStream<int>.Restore(new StreamRecoveryState(stream.Cursor,
                Enumerable.Range(0, capacity + 1).Select(index => "id-" + index).ToArray())));
            Assert.Throws<PlatformSdkException>(() => ResumableStream<int>.Restore(new StreamRecoveryState(stream.Cursor, [new string('x', 513)])));
        }
        finally { File.Delete(path); }
    }
}
