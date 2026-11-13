using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using LayerX.Sdk;
using Org.BouncyCastle.Crypto.Parameters;
using Org.BouncyCastle.Crypto.Signers;
using SdkJson = LayerX.Sdk.JsonValue;

internal static class Program
{
    private static string Root = "";
    private static readonly Dictionary<string, string> NoParameters = new();

    public static async Task Main(string[] args)
    {
        Root = args[0];
        switch (args[1])
        {
            case "request": await RequestVersion(); break;
            case "response": await ResponseEnvelope(); break;
            case "verification": await VerificationShapes(); break;
            case "credential": await BootstrapCredential(); break;
            case "trust": ValidatorTrust(); break;
            case "stream": await StreamRecovery(args[2]); break;
            case "stream-reopen": await ReopenStream(args[2]); break;
            default: throw new ArgumentException("unknown repair case");
        }
        Console.WriteLine("repair138 " + args[1] + " passed");
    }

    private static void Check(bool condition, string message)
    {
        if (!condition) throw new InvalidOperationException(message);
    }

    private static void Refused(Action action, SdkErrorCode code)
    {
        try { action(); }
        catch (PlatformSdkException error) { Check(error.Code == code, $"expected {code}, got {error.Code}"); return; }
        throw new InvalidOperationException("invalid evidence was accepted");
    }

    private static async Task RefusedAsync(Func<Task> action, SdkErrorCode code)
    {
        try { await action(); }
        catch (PlatformSdkException error) { Check(error.Code == code, $"expected {code}, got {error.Code}"); return; }
        throw new InvalidOperationException("invalid request was accepted");
    }

    private static AgentSessionCredential Session() => new("tenant-a", Enumerable.Repeat((byte)0xcd, 32).ToArray(),
        Enumerable.Repeat((byte)0xab, 32).ToArray(), 7);

    private static TransportCall Call(string operation, SdkJson? request = null) => new(
        Enum.GetValues<PlatformOperation>().Single(item => item.Descriptor().Plane == PlatformPlane.Agent && item.Descriptor().Name == operation),
        request ?? SdkJson.EmptyObject, NoParameters,
        Enum.GetValues<PlatformOperation>().Single(item => item.Descriptor().Plane == PlatformPlane.Agent && item.Descriptor().Name == operation).Descriptor().RequiresIdempotency
            ? new IdempotencyKey(new string('a', 64)) : null);

    private static async Task RequestVersion()
    {
        using var session = Session();
        using var transport = new AgentEnvelopeTransport(new Uri("http://127.0.0.1:1/"), session);
        var encode = typeof(AgentEnvelopeTransport).GetMethod("EnvelopeRequest", BindingFlags.Instance | BindingFlags.NonPublic)!;
        foreach (var operation in Enum.GetValues<PlatformOperation>().Where(item => item.Descriptor().Plane == PlatformPlane.Agent))
        {
            var call = Call(operation.Descriptor().Name);
            using var request = (HttpRequestMessage)encode.Invoke(transport, [operation.Descriptor(), call, "7"])!;
            using var document = JsonDocument.Parse(await request.Content!.ReadAsByteArrayAsync());
            Check(document.RootElement.GetProperty("version").GetInt32() == 1, "transport version differs from daemon version 1");
            Check(document.RootElement.GetProperty("credential").ValueKind == JsonValueKind.Object, "owner session omitted");
        }
        await OwnerPath("agent.register");
    }

    // The real daemon producer is invoked by this HTTP host; it is not a replacement codec.
    private sealed class ProducerHttp : IAsyncDisposable
    {
        private readonly HttpListener listener = new();
        private readonly Task worker;
        public Uri Endpoint { get; }

        public ProducerHttp(Func<byte[], Task<(int Status, byte[] Body)>> produce, string route = AgentEnvelopeTransport.RoutePath)
        {
            var reservation = new TcpListener(IPAddress.Loopback, 0);
            reservation.Start();
            var port = ((IPEndPoint)reservation.LocalEndpoint).Port;
            reservation.Stop();
            Endpoint = new Uri($"http://127.0.0.1:{port}/");
            listener.Prefixes.Add(Endpoint.AbsoluteUri);
            listener.Start();
            worker = Run(produce, route);
        }

        private async Task Run(Func<byte[], Task<(int Status, byte[] Body)>> produce, string route)
        {
            while (listener.IsListening)
            {
                HttpListenerContext context;
                try { context = await listener.GetContextAsync(); }
                catch (HttpListenerException) when (!listener.IsListening) { return; }
                catch (ObjectDisposedException) when (!listener.IsListening) { return; }
                Check(context.Request.HttpMethod == "POST" && context.Request.RawUrl == route,
                    "wrong HTTP ingress");
                using var buffer = new MemoryStream();
                await context.Request.InputStream.CopyToAsync(buffer);
                var response = await produce(buffer.ToArray());
                context.Response.StatusCode = response.Status;
                context.Response.ContentType = "application/json";
                context.Response.ContentLength64 = response.Body.Length;
                await context.Response.OutputStream.WriteAsync(response.Body);
                context.Response.Close();
            }
        }

        public async ValueTask DisposeAsync()
        {
            listener.Stop();
            await worker;
            listener.Close();
        }
    }

    private static async Task<(int, byte[])> Daemon(byte[] request, string state, params string[] arguments)
    {
        var binary = Environment.GetEnvironmentVariable("REPAIR138_DAEMON_CODEC")
            ?? throw new InvalidOperationException("REPAIR138_DAEMON_CODEC is required");
        using var process = new Process
        {
            StartInfo = new(binary)
            {
                RedirectStandardInput = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
            }
        };
        process.StartInfo.ArgumentList.Add(state);
        foreach (var argument in arguments) process.StartInfo.ArgumentList.Add(argument);
        Check(process.Start(), "daemon producer did not start");
        await process.StandardInput.BaseStream.WriteAsync(request);
        process.StandardInput.Close();
        var output = process.StandardOutput.ReadToEndAsync();
        var errors = process.StandardError.ReadToEndAsync();
        await process.WaitForExitAsync();
        Check(process.ExitCode == 0, "daemon codec failed: " + await errors);
        using var emitted = JsonDocument.Parse(await output);
        var result = emitted.RootElement.GetProperty("body");
        if (state is "achieved" or "pending" or "absent" or "refusal")
        {
            var goldenName = state switch
            {
                "achieved" => "agent-http-success.json",
                "refusal" => "agent-http-refusal.json",
                _ => "agent-http-success-" + state + ".json",
            };
            var golden = JsonNode.Parse(await File.ReadAllBytesAsync(Path.Combine(Root,
                "services/agent/schema/agent-api/golden", goldenName)))!.AsObject();
            golden["request_id"] = result.GetProperty("request_id").GetString();
            Check(JsonNode.DeepEquals(golden, JsonNode.Parse(result.GetRawText())), "daemon output differs from its published golden");
        }
        return (emitted.RootElement.GetProperty("status").GetInt32(), Encoding.UTF8.GetBytes(result.GetRawText()));
    }

    private static async Task ResponseEnvelope()
    {
        foreach (var state in new[] { "achieved", "refusal" })
        {
            await using var producer = new ProducerHttp(request => Daemon(request, state));
            using var session = Session();
            using var transport = new AgentEnvelopeTransport(producer.Endpoint, session);
            if (state == "achieved")
            {
                var result = await transport.SendEnvelopeAsync(Call("tenant.readiness"));
                Check(result.Value is SdkJson.ObjectValue, "daemon result lost");
            }
            else
                await RefusedAsync(() => transport.SendEnvelopeAsync(Call("tenant.readiness")), SdkErrorCode.PolicyRefusal);
        }
        foreach (var state in new[] { "achieved", "refusal" })
        {
            await using var producer = new ProducerHttp(async request =>
            {
                var (status, body) = await Daemon(request, state);
                var changed = JsonNode.Parse(body)!.AsObject();
                changed["version"] = 1;
                return (status, JsonSerializer.SerializeToUtf8Bytes(changed));
            });
            using var session = Session();
            using var transport = new AgentEnvelopeTransport(producer.Endpoint, session);
            await RefusedAsync(() => transport.SendEnvelopeAsync(Call("tenant.readiness")), SdkErrorCode.DecodeFailure);
        }
    }

    private static async Task VerificationShapes()
    {
        foreach (var state in new[] { "achieved", "unverified", "pending", "absent" })
        {
            await using var producer = new ProducerHttp(request => Daemon(request, state));
            using var session = Session();
            using var transport = new AgentEnvelopeTransport(producer.Endpoint, session);
            var result = await transport.SendEnvelopeAsync(Call("tenant.readiness"));
            Check(result.VerificationStatus is SdkJson.ObjectValue status &&
                status.Value["state"] == SdkJson.String(state), "daemon verification state changed");
        }
        foreach (var state in new[] { "achieved", "absent" })
        {
            await using var producer = new ProducerHttp(async request =>
            {
                var (code, body) = await Daemon(request, state);
                var changed = JsonNode.Parse(body)!.AsObject();
                changed["verification_status"]!["extra"] = true;
                return (code, JsonSerializer.SerializeToUtf8Bytes(changed));
            });
            using var session = Session();
            using var transport = new AgentEnvelopeTransport(producer.Endpoint, session);
            await RefusedAsync(() => transport.SendEnvelopeAsync(Call("tenant.readiness")), SdkErrorCode.DecodeFailure);
        }
        var mutations = new Action<JsonObject>[]
        {
            status => status["achieved"] = "receipted",
            status => status["achieved"] = "committed",
            status => status["achieved"] = "SequencerSigned",
            status => status["requested"] = "unknown",
            status => status["extra"] = true,
            status => status.Remove("achieved"),
        };
        foreach (var state in new[] { "pending", "unverified" })
            foreach (var mutate in mutations)
            {
                await using var producer = new ProducerHttp(async request =>
                {
                    var (code, body) = await Daemon(request, state);
                    var changed = JsonNode.Parse(body)!.AsObject();
                    mutate(changed["verification_status"]!.AsObject());
                    return (code, JsonSerializer.SerializeToUtf8Bytes(changed));
                });
                using var session = Session();
                using var transport = new AgentEnvelopeTransport(producer.Endpoint, session);
                await RefusedAsync(() => transport.SendEnvelopeAsync(Call("tenant.readiness")), SdkErrorCode.DecodeFailure);
            }
    }

    private static async Task BootstrapCredential()
    {
        foreach (var operation in new[] { "agent.register", "session.open" })
        {
            using var transport = new AgentEnvelopeTransport(new Uri("http://127.0.0.1:1/"), null);
            await RefusedAsync(() => transport.SendEnvelopeAsync(Call(operation)), SdkErrorCode.CapabilityRefusal);
        }
        await OwnerPath("agent.register");
        await OwnerPath("session.open");
    }

    private static async Task OwnerPath(string operation)
    {
        using var rsa = RSA.Create(2048);
        var certificateRequest = new CertificateRequest("CN=localhost", rsa, HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1);
        using var certificate = certificateRequest.CreateSelfSigned(DateTimeOffset.UtcNow.AddMinutes(-1), DateTimeOffset.UtcNow.AddHours(1));
        var pemPath = Path.GetTempFileName();
        await File.WriteAllTextAsync(pemPath, certificate.ExportCertificatePem());
        try
        {
            await using var daemon = new ProducerHttp(request => Daemon(request, "authorize"), AgentEnvelopeTransport.DaemonRoutePath);
            await using var gateway = new ProducerHttp(request => Daemon(request, "gateway", daemon.Endpoint.Port.ToString(), pemPath));
            using var session = Session();
            using var transport = new AgentEnvelopeTransport(gateway.Endpoint, session);
            var call = Call(operation);
            Check(call.IdempotencyKey is not null, "catalogue proof requires idempotency");
            var result = await transport.SendEnvelopeAsync(call);
            Check(result.Value is SdkJson.ObjectValue value && value.Value["tenant"] == SdkJson.String(session.Tenant) &&
                value.Value["generation"] == SdkJson.String("7"), "owner session was not authorized");
            using var wrong = new AgentSessionCredential(session.Tenant,
                Convert.FromHexString(session.SessionId), Enumerable.Repeat((byte)0xab, 32).ToArray(), 8);
            using var refused = new AgentEnvelopeTransport(gateway.Endpoint, wrong);
            await RefusedAsync(() => refused.SendEnvelopeAsync(call), SdkErrorCode.PolicyRefusal);

            // Bypass only client-side refusal to prove the daemon itself refuses a missing
            // session. The envelope is still emitted by the production SDK encoder.
            var encode = typeof(AgentEnvelopeTransport).GetMethod("EnvelopeRequest", BindingFlags.Instance | BindingFlags.NonPublic)!;
            using var encoded = (HttpRequestMessage)encode.Invoke(transport, [call.Operation.Descriptor(), call, "17"])!;
            var envelope = JsonNode.Parse(await encoded.Content!.ReadAsByteArrayAsync())!.AsObject();
            envelope["credential"] = null;
            using var http = new HttpClient();
            using var response = await http.PostAsync(new Uri(gateway.Endpoint, AgentEnvelopeTransport.RoutePath),
                new StringContent(envelope.ToJsonString(), Encoding.UTF8, "application/json"));
            Check(response.StatusCode == HttpStatusCode.BadRequest, "daemon accepted missing owner session");
            using var refusal = JsonDocument.Parse(await response.Content.ReadAsByteArrayAsync());
            Check(refusal.RootElement.GetProperty("reason").GetString() == "envelope.credential", "wrong missing-session refusal");
        }
        finally { File.Delete(pemPath); }
    }

    private static byte[] Hex(JsonElement root, string field) => Convert.FromHexString(root.GetProperty(field).GetString()![2..]);
    private static ReceiptKeys Keys(JsonElement vector)
    {
        var pin = vector.GetProperty("pinnedKey");
        return new(new[] { new ReceiptKey(Hex(pin, "keyId"), Hex(pin, "publicKey"),
            pin.GetProperty("firstHeight").GetUInt64(), pin.GetProperty("lastHeight").GetUInt64()) }, Hex(vector, "prevSeqHash"));
    }

    private static void ValidatorTrust()
    {
        using var published = JsonDocument.Parse(File.ReadAllBytes(Path.Combine(Root, "sdk/platform/conformance/fixtures/vector-v5-portable.json")));
        using var vectorFile = JsonDocument.Parse(File.ReadAllBytes(Path.Combine(Root, "sdk/platform/conformance/fixtures/vector-v4-committed.json")));
        using var independent = JsonDocument.Parse(File.ReadAllBytes(Path.Combine(Root, "daemon/committed-fixture/vectors/vector-v4-committed.json")));
        var vector = vectorFile.RootElement.GetProperty("positive");
        var keys = Keys(vector);
        var bundle = published.RootElement.GetProperty("bundles")[1];
        var encoded = Encoding.UTF8.GetBytes(bundle.GetRawText());
        var header = bundle.GetProperty("blockHeader").Deserialize<CometHeader>()!;
        var commit = bundle.GetProperty("commit").Deserialize<CometCommit>()!;
        var inclusion = bundle.GetProperty("receiptInclusion").Deserialize<CommittedReceiptInclusion>()!;
        var validators = bundle.GetProperty("validatorSet").Deserialize<CometValidator[]>()!;
        var independentCommit = independent.RootElement.GetProperty("settlement").GetProperty("execution_commit");
        var hash = Convert.FromHexString(independentCommit.GetProperty("block_id").GetProperty("hash").GetString()!);
        Check(hash.SequenceEqual(Convert.FromHexString("a7c648246a5213d57d6a2f2aa7bdc8e0050249629e5e92a4b89f2bdb918e9108")), "committed recording checkpoint changed");
        var trust = new PortableReceiptTrust("committed-fixture", 10, hash);
        hash[0] ^= 1; // The context owns its pin even if the caller reuses the input buffer.
        Check(PortableReceipt.verify(encoded, keys, trust).VerificationLevel == "committed", "trusted committed evidence refused");
        var exported = PortableReceipt.export(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"),
            keys, header, inclusion, commit, validators, trust);
        Check(exported.VerificationLevel == "committed", "trusted export lost finality");
        Refused(() => PortableReceipt.verify(encoded, keys), SdkErrorCode.VerificationFailure);
        Refused(() => PortableReceipt.export(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"),
            keys, header, inclusion, commit, validators), SdkErrorCode.VerificationFailure);
        foreach (var wrong in new[] {
            new PortableReceiptTrust("foreign-network", 10, Committed.HeaderHash(header)),
            new PortableReceiptTrust(header.ChainId, 11, Committed.HeaderHash(header)),
            new PortableReceiptTrust(header.ChainId, 10, new byte[32]) })
            Refused(() => PortableReceipt.verify(encoded, keys, wrong), SdkErrorCode.VerificationFailure);

        // Preserve the genuine receipt and proofs but replace all consensus evidence with an attacker's own signed chain.
        var attacker = new Ed25519PrivateKeyParameters(RandomNumberGenerator.GetBytes(32), 0);
        var foreignValidators = new[] { new CometValidator(attacker.GeneratePublicKey().GetEncoded(), 100) };
        var foreignHash = Committed.ValidatorSetHash(foreignValidators);
        var foreignHeader = header with
        {
            ValidatorsHash = foreignHash,
            NextValidatorsHash = foreignHash,
            ProposerAddress = SHA256.HashData(foreignValidators[0].PublicKey)[..20]
        };
        var foreignCommit = Sign(foreignHeader, attacker);
        var foreignNext = inclusion.NextHeader with
        {
            LastBlockId = foreignCommit.BlockId,
            ValidatorsHash = foreignHash,
            NextValidatorsHash = foreignHash,
            ProposerAddress = foreignHeader.ProposerAddress
        };
        var foreignInclusion = inclusion with { NextHeader = foreignNext, NextCommit = Sign(foreignNext, attacker), NextValidators = foreignValidators };
        var receipt = LocalVerifier.VerifySequencerReceipt(bundle.GetProperty("receiptJson"), Hex(bundle, "rawTransaction"), keys);
        Check(Committed.Verify(receipt, foreignHeader, foreignInclusion, foreignCommit, foreignValidators).Rung == "committed",
            "attack fixture must have fully consistent signatures, proofs and validator transition");
        var forged = JsonNode.Parse(encoded)!.AsObject();
        forged["blockHeader"] = JsonSerializer.SerializeToNode(foreignHeader);
        forged["commit"] = JsonSerializer.SerializeToNode(foreignCommit);
        forged["validatorSet"] = JsonSerializer.SerializeToNode(foreignValidators);
        forged["receiptInclusion"] = JsonSerializer.SerializeToNode(foreignInclusion);
        Refused(() => PortableReceipt.verify(JsonSerializer.SerializeToUtf8Bytes(forged), keys, trust), SdkErrorCode.VerificationFailure);
        Refused(() => PortableReceipt.verify(JsonSerializer.SerializeToUtf8Bytes(forged), keys), SdkErrorCode.VerificationFailure);
        var changedTransition = JsonNode.Parse(encoded)!.AsObject();
        changedTransition["receiptInclusion"]!["NextValidators"] = JsonSerializer.SerializeToNode(foreignValidators);
        Refused(() => PortableReceipt.verify(JsonSerializer.SerializeToUtf8Bytes(changedTransition), keys, trust), SdkErrorCode.VerificationFailure);
    }

    private static CometCommit Sign(CometHeader header, Ed25519PrivateKeyParameters privateKey)
    {
        var commit = new CometCommit(header.Height, 0, new(Committed.HeaderHash(header), 1, SHA256.HashData("parts"u8)),
            Array.Empty<CometCommitSignature>());
        var signature = new CometCommitSignature(2, SHA256.HashData(privateKey.GeneratePublicKey().GetEncoded())[..20], header.Time, Array.Empty<byte>());
        var signer = new Ed25519Signer();
        signer.Init(true, privateKey);
        var message = Committed.VoteSignBytes(header.ChainId, commit, signature);
        signer.BlockUpdate(message, 0, message.Length);
        return commit with { Signatures = new[] { signature with { Signature = signer.GenerateSignature() } } };
    }

    private static async Task<StreamPage<string>> ReadJournal(string directory, StreamCursor cursor, CancellationToken cancellation)
    {
        var rows = await File.ReadAllLinesAsync(Path.Combine(directory, "events"), cancellation);
        var offset = int.Parse(cursor.Value);
        var events = rows.Skip(offset).Select((value, index) =>
            new StreamEvent<string>("event-" + (offset + index + 1), new((offset + index).ToString()),
                new((offset + index + 1).ToString()), value)).ToArray();
        return new(cursor, events, events.Length == 0 ? cursor : events[^1].Cursor);
    }

    private static async Task StreamRecovery(string directory)
    {
        await File.WriteAllLinesAsync(Path.Combine(directory, "events"), new[] { "one", "two", "three" });
        var stream = new ResumableStream<string>(new("0"));
        using var cancellation = new CancellationTokenSource();
        await using (var events = stream.EventsAsync((cursor, token) => ReadJournal(directory, cursor, token), cancellation.Token).GetAsyncEnumerator())
        {
            Check(await events.MoveNextAsync() && events.Current.Value == "one", "first event absent");
            Check(stream.Cursor.Value == "1", "cursor advanced past undelivered events");
            await File.WriteAllTextAsync(Path.Combine(directory, "cursor"), stream.Cursor.Value);
            cancellation.Cancel();
            try { await events.MoveNextAsync(); throw new InvalidOperationException("cancellation delivered a buffered event"); }
            catch (OperationCanceledException) { }
            Check(stream.Cursor.Value == "1", "cancellation acknowledged an undelivered event");
        }
        // The Go test starts a fresh process for stream-reopen, using only the saved cursor and journal.
    }

    private static async Task ReopenStream(string directory)
    {
        var stream = new ResumableStream<string>(new(await File.ReadAllTextAsync(Path.Combine(directory, "cursor"))));
        await using (var events = stream.EventsAsync((cursor, token) => ReadJournal(directory, cursor, token)).GetAsyncEnumerator())
        {
            Check(await events.MoveNextAsync() && events.Current.Value == "two" && stream.Cursor.Value == "2", "second event lost after restart");
        }
        var reopened = new ResumableStream<string>(stream.Cursor);
        await using (var events = reopened.EventsAsync((cursor, token) => ReadJournal(directory, cursor, token)).GetAsyncEnumerator())
        {
            Check(await events.MoveNextAsync() && events.Current.Value == "three" && reopened.Cursor.Value == "3", "third event lost after partial-page disposal");
        }
        var cancelled = new ResumableStream<string>(new("0"));
        using var cancellation = new CancellationTokenSource();
        await using (var events = cancelled.EventsAsync(async (cursor, token) =>
        {
            var page = await ReadJournal(directory, cursor, token);
            cancellation.Cancel();
            return page;
        }, cancellation.Token).GetAsyncEnumerator())
        {
            try { await events.MoveNextAsync(); throw new InvalidOperationException("cancelled page delivered events"); }
            catch (OperationCanceledException) { }
        }
        Check(cancelled.Cursor.Value == "0", "cancelled fetch changed recovery state");
        var invalid = new ResumableStream<string>(new("0"));
        await using (var events = invalid.EventsAsync(async (cursor, token) =>
        {
            var page = await ReadJournal(directory, cursor, token);
            return page with { NextCursor = new("wrong-tail") };
        }).GetAsyncEnumerator())
            await RefusedAsync(async () => { await events.MoveNextAsync(); }, SdkErrorCode.DecodeFailure);
        Check(invalid.Cursor.Value == "0", "invalid page partially advanced state");
    }
}
