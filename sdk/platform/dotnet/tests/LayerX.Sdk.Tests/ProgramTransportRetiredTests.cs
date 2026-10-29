using System.Net;
using System.Net.Sockets;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;
using LayerX.Sdk;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class ProgramTransportRetiredTests
{
    private const string Sender = "0x2222222222222222222222222222222222222222";

    private sealed class RecordingTransport() : DelegatingHandler(new SocketsHttpHandler
    {
        AllowAutoRedirect = false,
        UseProxy = false,
    })
    {
        public List<(HttpMethod Method, Uri Uri, string Body)> Requests { get; } = [];

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            Requests.Add((request.Method, request.RequestUri!, await request.Content!.ReadAsStringAsync(cancellationToken)));
            return await base.SendAsync(request, cancellationToken);
        }
    }

    [Fact]
    public async Task ProgramOperationsNeverCallProgramsRestIngress()
    {
        var operations = programRoutes().ToArray();
        string[] names =
        [
            "program.activity", "program.call", "program.deploy", "program.discover", "program.interface",
            "program.receipt", "program.simulate", "program.upgrade", "program.wind-down",
        ];
        Assert.Equal(names, operations.Select(operation => operation.Name).Distinct().OrderBy(name => name, StringComparer.Ordinal));
        foreach (var operation in operations)
            await sendProgram(operation.RpcMethod, operation.Send);

        var legacy = new AgentHttpTransport(new Uri("http://127.0.0.1:8080"));
        var catalog = Enum.GetValues<PlatformOperation>()
            .Where(operation => operation.Descriptor().Plane == PlatformPlane.Agent &&
                operation.Descriptor().Name.StartsWith("program.", StringComparison.Ordinal)).ToArray();
        Assert.Equal(names, catalog.Select(operation => operation.Descriptor().Name).OrderBy(name => name, StringComparer.Ordinal));
        foreach (var operation in catalog)
        {
            var error = await Assert.ThrowsAsync<PlatformSdkException>(() => legacy.SendAsync(new TransportCall(
                operation, JsonValue.EmptyObject, new Dictionary<string, string>(),
                operation.Descriptor().RequiresIdempotency ? new IdempotencyKey(new string('a', 64)) : null)));
            Assert.Equal(SdkErrorCode.UnavailableCapability, error.Code);
            Assert.Equal(RetryClass.Never, error.Retry);
        }
        Assert.Null(typeof(IPlatformTransport).GetMethod("SendProgramAsync"));
        Assert.Null(typeof(AgentHttpTransport).GetMethod("SendProgramAsync"));
        Assert.Null(typeof(PlatformClient).Assembly.GetType("LayerX.Sdk.ProgramTransportCall"));
    }

    private static IEnumerable<(string Name, string RpcMethod, Func<ProgramsClient, CancellationToken, Task> Send)> programRoutes()
    {
        using var callFixture = Fixture("native-program-call-v4.json");
        var call = new ProgramCall(NativeProgramCall.Decode(Hex(callFixture.RootElement, "payload_hex")),
            new ProtocolAmount(callFixture.RootElement.GetProperty("fee_limit").GetString()!));
        using var deployFixture = Fixture("native-program-deploy-v4.json");
        var deploy = NativeProgramDeploy.Decode(Hex(deployFixture.RootElement, "payload_hex"));
        using var upgradeFixture = Fixture("native-program-upgrade-v4.json");
        var upgrade = NativeProgramUpgrade.Decode(Hex(upgradeFixture.RootElement, "payload_hex"));
        var program = call.ProgramId;
        var key = new IdempotencyKey(new string('a', 64));
        yield return ("program.discover", "eth_call", (client, token) => client.DiscoverAsync(program, "receipted", token));
        yield return ("program.interface", "eth_call", (client, token) => client.InterfaceAsync(program, "receipted", token));
        yield return ("program.simulate", "eth_call", (client, token) => client.SimulateAsync(call, token));
        yield return ("program.simulate", "eth_call", (client, token) => client.SimulateAsync(call, Sender, token));
        yield return ("program.call", "eth_getTransactionCount", (client, token) => client.CallAsync(call, Sender, 100, 2, token));
        yield return ("program.receipt", "eth_call", (client, token) => client.ReceiptAsync(key, program, "receipted", token));
        yield return ("program.activity", "eth_getTransactionByHash", (client, token) => client.ActivityAsync(program, "receipted", token));
        yield return ("program.deploy", "eth_getTransactionCount", (client, token) => client.DeployAsync(deploy, Sender, 100, 2, token));
        yield return ("program.upgrade", "eth_getTransactionCount", (client, token) => client.UpgradeAsync(upgrade, new ProtocolAmount("1000"), Sender, 100, 2, token));
        foreach (var windDown in new[]
        {
            NativeProgramWindDown.Route(program, program, program, program, []),
            NativeProgramWindDown.Deprecate(program, program, 100),
            NativeProgramWindDown.Tombstone(program),
            NativeProgramWindDown.Exit(program, program),
        })
            yield return ("program.wind-down", "eth_getTransactionCount", (client, token) => client.WindDownAsync(windDown, Sender, 100, 2, token));
    }

    private static async Task sendProgram(string rpcMethod, Func<ProgramsClient, CancellationToken, Task> send)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
        using var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        var endpoint = new Uri($"http://127.0.0.1:{((IPEndPoint)listener.LocalEndpoint).Port}/rpc");
        using var recording = new RecordingTransport();
        using var http = new HttpClient(recording);
        using var rpc = new Programs(endpoint, http);
        var operation = send(new ProgramsClient(rpc, didId: Enumerable.Repeat((byte)1, 32).ToArray()), timeout.Token);
        string requestLine;
        string body;
        using (var connection = await listener.AcceptTcpClientAsync(timeout.Token))
        {
            await using var stream = connection.GetStream();
            using var reader = new StreamReader(stream, Encoding.UTF8, leaveOpen: true);
            requestLine = (await reader.ReadLineAsync(timeout.Token))!;
            var contentLength = -1;
            string? line;
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
        var error = await Assert.ThrowsAsync<PlatformSdkException>(() => operation);
        Assert.Equal(SdkErrorCode.TransportFailure, error.Code);
        var recorded = Assert.Single(recording.Requests);
        Assert.Equal(HttpMethod.Post, recorded.Method);
        Assert.Equal(endpoint, recorded.Uri);
        Assert.False(recorded.Uri.AbsolutePath.StartsWith("/v1/programs", StringComparison.Ordinal));
        Assert.Equal("POST /rpc HTTP/1.1", requestLine);
        Assert.Equal(recorded.Body, body);
        using var request = JsonDocument.Parse(body);
        Assert.Equal("2.0", request.RootElement.GetProperty("jsonrpc").GetString());
        Assert.Equal(rpcMethod, request.RootElement.GetProperty("method").GetString());
    }

    private static byte[] Hex(JsonElement element, string name) => Convert.FromHexString(element.GetProperty(name).GetString()!);
    private static JsonDocument Fixture(string name, [CallerFilePath] string source = "") => JsonDocument.Parse(File.ReadAllText(
        Path.Combine(Path.GetDirectoryName(source)!, "..", "..", "..", "conformance", "fixtures", name)));
}
