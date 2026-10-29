using System.Buffers.Binary;
using System.Numerics;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using LayerX.Sdk;
using Org.BouncyCastle.Crypto.Parameters;
using Org.BouncyCastle.Crypto.Signers;
using Xunit;

namespace LayerX.Sdk.Tests;

public sealed class ProgramsContractTests
{
    [Fact]
    public void ProgramsClientRequiresIndependentNonzeroSequencerPin()
    {
        var client = new PlatformClient(new AgentHttpTransport(new Uri("http://127.0.0.1:8080")));
        Assert.Throws<PlatformSdkException>(() => new ProgramsClient(client, new byte[32]));
        _ = new ProgramsClient(client, Enumerable.Repeat((byte)1, 32).ToArray());
    }

    [Fact]
    public async Task PendingReceiptMayOmitRetainedBytesButMustBindExpectedActivity()
    {
        var key = new string('a', 64); var activity = Enumerable.Repeat((byte)0x11, 32).ToArray();
        var value = JsonValue.Object(new Dictionary<string, JsonValue>
        {
            ["state"] = JsonValue.String("unknown"),
            ["activity_id"] = JsonValue.String(Convert.ToHexString(activity).ToLowerInvariant()),
            ["idempotency_key"] = JsonValue.String(key),
        });
        var programs = new ProgramsClient(new PlatformClient(new AgentHttpTransport(new Uri("http://127.0.0.1:8080"))),
            Enumerable.Repeat((byte)1, 32).ToArray());
        var pending = await programs.ReceiptAsync(new IdempotencyKey(key), activity, "sequencer-signed");
        Assert.True(pending.IsUnknown); Assert.Null(pending.RetainedSignedActivity);
        var error = await Assert.ThrowsAsync<PlatformSdkException>(() => programs.ReceiptAsync(new IdempotencyKey(key),
            Enumerable.Repeat((byte)0x12, 32).ToArray(), "sequencer-signed"));
        Assert.Equal(SdkErrorCode.VerificationFailure, error.Code);
    }

    [Fact]
    public async Task DiscoveryClientReturnsBoundedTypedResponseWithHonestVerificationLevel()
    {
        var program = new string('1', 64); var now = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        var value = JsonValue.Object(new Dictionary<string, JsonValue>
        {
            ["program_id"] = JsonValue.String(program), ["lifecycle"] = JsonValue.String("active"),
            ["version"] = JsonValue.Integer(7), ["code_hash"] = JsonValue.String(new string('2', 64)),
            ["abi_version"] = JsonValue.Integer(2), ["receipt_digest"] = JsonValue.String(new string('3', 64)),
            ["state_root"] = JsonValue.String(new string('4', 64)), ["observed_sequence"] = JsonValue.String("9"),
            ["observed_at"] = JsonValue.String(now.ToString()), ["valid_through"] = JsonValue.String((now + 60_000).ToString()),
            ["verification"] = JsonValue.String("registry-receipt-and-current-head-verified"),
        });
        var programs = new ProgramsClient(new PlatformClient(new AgentHttpTransport(new Uri("http://127.0.0.1:8080"))),
            Enumerable.Repeat((byte)1, 32).ToArray());

        var discovered = await programs.DiscoverAsync(Convert.FromHexString(program), "sequencer-signed");
        Assert.Equal(ProgramLifecycle.Active, discovered.Lifecycle);
        Assert.Equal((uint)7, discovered.Version); Assert.Equal((ulong)9, discovered.ObservedSequence);
        Assert.Equal("server-side-receipt-verification-only", discovered.Verification);
    }

    [Fact]
    public void OperationValueVerificationStatusMatrixIsExact()
    {
        var achieved = Status("Achieved", null); var discovery = Status("Unverified", "server_side_receipt_verification_only");
        var pending = Status("Unverified", "receipt_pending");
        var unknown = JsonValue.Object(new Dictionary<string, JsonValue> { ["state"] = JsonValue.String("unknown") });
        var inFlight = JsonValue.Object(new Dictionary<string, JsonValue> { ["state"] = JsonValue.String("pending") });
        var terminal = JsonValue.Object(new Dictionary<string, JsonValue> { ["state"] = JsonValue.String("executed") });
        var oldUnverified = JsonValue.Object(new Dictionary<string, JsonValue>
        {
            ["state"] = JsonValue.String("Unverified"), ["level"] = JsonValue.String("SequencerSigned"),
            ["reason"] = JsonValue.String("server_side_receipt_verification_only"),
        });
        Assert.True(TransportStatus("program.discover", JsonValue.EmptyObject, discovery));
        Assert.False(TransportStatus("program.discover", JsonValue.EmptyObject, achieved));
        Assert.False(TransportStatus("program.discover", JsonValue.EmptyObject, oldUnverified));
        Assert.True(TransportStatus("program.receipt", unknown, pending));
        Assert.True(TransportStatus("program.activity", inFlight, pending));
        Assert.False(TransportStatus("program.receipt", unknown, achieved));
        Assert.False(TransportStatus("program.call", inFlight, achieved));
        Assert.True(TransportStatus("program.call", terminal, achieved));
        Assert.True(TransportStatus("program.simulate", JsonValue.EmptyObject, achieved));
        Assert.False(TransportStatus("program.simulate", JsonValue.EmptyObject, discovery));
    }

    [Fact]
    public void TransferSetV1AndV2ProduceTheSameCanonicalKernelRoot()
    {
        var v1 = TransferAuthorization(1); var v2 = TransferAuthorization(2);
        var rootV1 = (byte[])Invoke("DecodeAuthorizationRoot", v1)!;
        var rootV2 = (byte[])Invoke("DecodeAuthorizationRoot", v2)!;
        Assert.Equal(rootV1, rootV2); Assert.Contains(rootV1, value => value != 0);
        var mutated = v2.ToArray(); mutated[^33] ^= 1;
        Assert.NotEqual(rootV2, (byte[])Invoke("DecodeAuthorizationRoot", mutated)!);
    }

    [Fact]
    public void OccupancyV1V2V3AndAggregateBindingsAreExact()
    {
        var asset = Enumerable.Repeat((byte)0x66, 32).ToArray();
        for (var version = 1; version <= 3; version++)
        {
            var binding = Invoke("DecodeOccupancySettlement", EmptyOccupancy(version))!;
            Assert.Equal(BigInteger.Zero, Property<BigInteger>(binding, "ByteBatches"));
            Assert.Equal(BigInteger.Zero, Property<BigInteger>(binding, "FeeUnits"));
            Assert.Equal(new byte[32], (byte[])Invoke("OccupancyTransferRoot", binding, asset)!);
        }
        var evidence = ChargedOccupancy(); var charged = Invoke("DecodeOccupancySettlement", evidence)!;
        Assert.Equal(new BigInteger(3), Property<BigInteger>(charged, "ByteBatches"));
        Assert.Equal(new BigInteger(6), Property<BigInteger>(charged, "FeeUnits"));
        Assert.Contains((byte[])Invoke("OccupancyTransferRoot", charged, asset)!, value => value != 0);
        var mutated = evidence.ToArray();
        var declaredFeeLowByte = Encoding.UTF8.GetByteCount("LXP/storage-occupancy-settlement/v3\0") + 8 + 4 + 7 * 8 + 16 + 15;
        mutated[declaredFeeLowByte] ^= 1;
        var exception = Assert.Throws<TargetInvocationException>(() => Invoke("DecodeOccupancySettlement", mutated));
        Assert.IsType<InvalidDataException>(exception.InnerException);
    }

    [Theory]
    [InlineData("native-program-call-v3.json")]
    [InlineData("native-program-call-v4.json")]
    public void NativeSignedFixturesPreserveCanonicalCallBindings(string name)
    {
        using var document = Fixture(name);
        var vector = document.RootElement;
        var payload = FixtureBytes(vector, "payload_hex");
        var signed = FixtureBytes(vector, "signed_activity_hex");
        var native = NativeProgramCall.Decode(payload);
        var call = new ProgramCall(native, new ProtocolAmount(vector.GetProperty("fee_limit").GetString()!), signed);
        var encoded = Assert.IsType<JsonValue.ObjectValue>(Invoke("Encode", call));
        Assert.Equal(Convert.ToHexString(payload).ToLowerInvariant(), Assert.IsType<JsonValue.StringValue>(encoded.Value["payload"]).Value);
        Assert.Equal(Convert.ToHexString(signed).ToLowerInvariant(), Assert.IsType<JsonValue.StringValue>(encoded.Value["signed_activity"]).Value);
        var changedCall = new ProgramCall(native with { ResponseCapacity = native.ResponseCapacity + 1 }, call.Budget.FeeLimit, signed);
        Assert.Throws<TargetInvocationException>(() => Invoke("DecodeSignedCall", changedCall));
        foreach (var abi in new ushort[] { 0, 6, ushort.MaxValue })
            Assert.Throws<ArgumentException>(() => (native with { GuestAbi = abi }).Encode());
        foreach (var abi in new ushort[] { 3, 4 })
        {
            var changed = native with { GuestAbi = abi };
            Assert.Equal(abi, NativeProgramCall.Decode(changed.Encode()).GuestAbi);
            Assert.Throws<TargetInvocationException>(() => Invoke("DecodeSignedCall", new ProgramCall(changed, call.Budget.FeeLimit, signed)));
        }
    }

    [Theory]
    [InlineData("native-program-deploy-v4.json", 1)]
    [InlineData("native-program-upgrade-v4.json", 2)]
    public void NativeLifecycleFixturesPreserveSignedBindingAcrossNamedAbiPolicies(string name, int ordinal)
    {
        using var document = Fixture(name);
        var vector = document.RootElement;
        var payload = FixtureBytes(vector, "payload_hex");
        INativeProgramLifecycle operation = ordinal == 1 ? NativeProgramDeploy.Decode(payload) : NativeProgramUpgrade.Decode(payload);
        Assert.Equal((ushort)2, BinaryPrimitives.ReadUInt16BigEndian(payload.AsSpan(32)));
        Assert.Equal(payload, operation.Encode());
        var request = new NativeProgramLifecycleRequest(operation, FixtureBytes(vector, "signed_activity_hex"));
        Assert.Equal(FixtureBytes(vector, "activity_id_hex"), request.ActivityId);
        foreach (var abi in new ushort[] { 3, 4 })
        {
            var changed = payload.ToArray(); BinaryPrimitives.WriteUInt16BigEndian(changed.AsSpan(32), abi);
            INativeProgramLifecycle updated = ordinal == 1 ? NativeProgramDeploy.Decode(changed) : NativeProgramUpgrade.Decode(changed);
            Assert.Equal(changed, updated.Encode());
            Assert.Throws<ArgumentException>(() => new NativeProgramLifecycleRequest(updated, FixtureBytes(vector, "signed_activity_hex")));
        }
        foreach (var abi in new ushort[] { 0, 6, ushort.MaxValue })
        {
            var changed = payload.ToArray(); BinaryPrimitives.WriteUInt16BigEndian(changed.AsSpan(32), abi);
            Assert.Throws<ArgumentException>(() => ordinal == 1 ? (INativeProgramLifecycle)NativeProgramDeploy.Decode(changed) : NativeProgramUpgrade.Decode(changed));
        }
    }

    [Fact]
    public void SignedDiscoveryHeadsBindEveryCanonicalFieldToTheIndependentPin()
    {
        var signer = new Ed25519PrivateKeyParameters(Enumerable.Repeat((byte)0x42, 32).ToArray(), 0);
        var pin = signer.GeneratePublicKey().GetEncoded();
        var programs = new ProgramsClient(new PlatformClient(new AgentHttpTransport(new Uri("http://127.0.0.1:8080"))), pin);
        var verify = typeof(ProgramsClient).GetMethod("VerifyDiscovery", BindingFlags.Instance | BindingFlags.NonPublic)!;
        foreach (var abi in new ushort[] { 1, 2, 3, 4 })
        {
            var head = SignedHead(abi, signer);
            ProgramDiscovery Verify(IReadOnlyDictionary<string, JsonValue> value) => Assert.IsType<ProgramDiscovery>(
                verify.Invoke(programs, [JsonValue.Object(value), new string('1', 64), false, 2000UL]));
            var accepted = Verify(head);
            Assert.Equal(abi, accepted.AbiVersion);
            Assert.Equal("sequencer-signed-discovery-head", accepted.Verification);
            Assert.Equal(Repeat(0x44), accepted.DeploymentReceiptDigest);
            foreach (var (field, changed) in new (string, JsonValue)[]
            {
                ("version", JsonValue.Integer(4)), ("code_hash", JsonValue.String(new string('9', 64))),
                ("abi_version", JsonValue.Integer(abi == 4 ? 3 : 4)),
                ("observed_sequence", JsonValue.String("78")), ("observed_at", JsonValue.String("1001")),
                ("valid_through", JsonValue.String("9999")), ("state_root", JsonValue.String(new string('9', 64))),
                ("receipt_digest", JsonValue.String(new string('9', 64))),
                ("discovery_public_key", JsonValue.String(new string('9', 64))),
                ("discovery_signature", JsonValue.String(new string('9', 128))),
                ("abi_version", JsonValue.Integer(5)), ("unexpected", JsonValue.String("extra")),
            })
            {
                var altered = new Dictionary<string, JsonValue>(head) { [field] = changed };
                Assert.IsType<PlatformSdkException>(Assert.Throws<TargetInvocationException>(() => Verify(altered)).InnerException);
            }
            foreach (var field in new[] { "discovery_public_key", "discovery_signature", "deployment_receipt_digest" })
            {
                var altered = new Dictionary<string, JsonValue>(head); altered.Remove(field);
                Assert.IsType<PlatformSdkException>(Assert.Throws<TargetInvocationException>(() => Verify(altered)).InnerException);
            }
            if (abi is 3 or 4)
            {
                var unsigned = new Dictionary<string, JsonValue>(head);
                unsigned.Remove("discovery_public_key"); unsigned.Remove("discovery_signature");
                Assert.IsType<PlatformSdkException>(Assert.Throws<TargetInvocationException>(() => Verify(unsigned)).InnerException);
            }
        }
    }

    [Fact]
    public void NativeDeploymentInterfaceHeaderBindsTheActualCodeAndAbi()
    {
        using var document = Fixture("native-program-deploy-v3.json");
        var payload = FixtureBytes(document.RootElement, "payload_hex");
        var length = checked((int)BinaryPrimitives.ReadUInt32BigEndian(payload.AsSpan(104)));
        var encoded = payload.AsSpan(108, length).ToArray(); var codeHash = payload[68..100];
        var abi = BinaryPrimitives.ReadUInt16BigEndian(payload.AsSpan(32));
        Assert.True((bool)Invoke("InterfaceHeaderBound", encoded, codeHash, abi)!);
        Assert.False((bool)Invoke("InterfaceHeaderBound", encoded, Repeat(0x99), abi)!);
        foreach (var wrongAbi in new ushort[] { 0, 1, 3, 4, 5, ushort.MaxValue })
            Assert.False((bool)Invoke("InterfaceHeaderBound", encoded, codeHash, wrongAbi)!);
        for (var size = 0; size < Encoding.UTF8.GetByteCount("LayerX/program-interface/v1\0") + 36; size++)
            Assert.False((bool)Invoke("InterfaceHeaderBound", encoded[..size], codeHash, abi)!);
        var changed = encoded.ToArray(); changed[0] ^= 1;
        Assert.False((bool)Invoke("InterfaceHeaderBound", changed, codeHash, abi)!);
    }

    private static ProgramCall NativeCallFromSignedActivity(byte[] signed)
    {
        Assert.True(signed.Length >= 5); Assert.Equal((ushort)3, BinaryPrimitives.ReadUInt16BigEndian(signed));
        Assert.Equal((ushort)0x1001, BinaryPrimitives.ReadUInt16BigEndian(signed.AsSpan(2))); Assert.Equal((byte)12, signed[4]);
        var offset = 5; byte[]? payload = null; BigInteger fee = 0;
        for (byte tag = 1; tag <= 12; tag++)
        {
            Assert.Equal(tag, signed[offset++]);
            var length = tag switch { 1 => 2, 2 or 3 => 4, 6 => 8, 7 or 9 => 16, _ => -1 };
            if (length < 0) { length = checked((int)BinaryPrimitives.ReadUInt32BigEndian(signed.AsSpan(offset))); offset += 4; }
            var bytes = signed.AsSpan(offset, length).ToArray(); offset += length;
            if (tag == 9) fee = new BigInteger(bytes, true, true);
            if (tag == 11) payload = bytes;
        }
        Assert.Equal(signed.Length, offset);
        return new ProgramCall(NativeProgramCall.Decode(payload!), new ProtocolAmount(fee.ToString()), signed);
    }

    private static JsonValue NativeTerminalOutcome(byte[] terminal, ProgramReceiptOutcome receipt)
    {
        var unwrapped = Assert.IsType<byte[]>(Invoke("UnwrapAppliedTerminal", terminal, receipt));
        var inner = Property<byte[]>(Invoke("UnwrapTerminal", unwrapped)!, "Inner");
        var domain = Encoding.UTF8.GetBytes("LXP/program-execution/v5\0");
        if (inner.AsSpan().StartsWith(domain))
        {
            var offset = CandidateTerminalAbiOffset(inner, domain.Length);
            Assert.Equal(receipt.AbiVersion, BinaryPrimitives.ReadUInt16BigEndian(inner.AsSpan(offset)));
            var tag = inner[offset + 2]; offset += 3;
            if (tag == 0)
            {
                Assert.Equal((byte)1, receipt.TerminalKind);
                var code = BinaryPrimitives.ReadInt32BigEndian(inner.AsSpan(offset)); offset += 4;
                Assert.Equal(receipt.ResultCode, code);
                var length = checked((int)BinaryPrimitives.ReadUInt64BigEndian(inner.AsSpan(offset))); offset += 8;
                var response = inner.AsSpan(offset, length).ToArray();
                return JsonValue.Object(new Dictionary<string, JsonValue> { ["kind"] = JsonValue.String("completed"),
                    ["code"] = JsonValue.Integer(code), ["response"] = JsonValue.String(Convert.ToHexString(response).ToLowerInvariant()) });
            }
            Assert.True(tag is 1 or 2); Assert.Equal((byte)(tag == 1 ? 2 : 3), receipt.TerminalKind);
        }
        else
        {
            Assert.True(inner.AsSpan().StartsWith(Encoding.UTF8.GetBytes("LXP/programs/callback-failure/v1\0")) ||
                inner.AsSpan().StartsWith(Encoding.UTF8.GetBytes("LXP/programs/settlement-failure/v1\0")), "unknown genuine native terminal domain");
            Assert.Equal((byte)2, receipt.TerminalKind);
        }
        var failure = new Dictionary<string, JsonValue> { ["kind"] = JsonValue.String(receipt.TerminalKind == 3 ? "resource" : "guest_refused") };
        if (receipt.TerminalKind == 2) failure["code"] = JsonValue.Integer(receipt.ResultCode);
        return JsonValue.Object(new Dictionary<string, JsonValue> { ["kind"] = JsonValue.String("refused"), ["failure"] = JsonValue.Object(failure) });
    }

    private static int CandidateTerminalAbiOffset(byte[] terminal, int offset)
    {
        offset += 10;
        var values = BinaryPrimitives.ReadUInt64BigEndian(terminal.AsSpan(offset)); offset += 8;
        Assert.True(values <= (ulong)(terminal.Length / 5));
        for (ulong index = 0; index < values; index++)
        {
            var tag = terminal[offset++]; Assert.True(tag is 1 or 2); offset += tag == 1 ? 4 : 8;
        }
        offset += 60;
        var trace = terminal[offset++]; Assert.True(trace is 0 or 1);
        if (trace == 1)
        {
            var length = checked((int)BinaryPrimitives.ReadUInt64BigEndian(terminal.AsSpan(offset))); offset += 8 + length;
        }
        offset += 32; Assert.True(offset <= terminal.Length - 2); return offset;
    }

    private static Dictionary<string, JsonValue> SignedHead(ushort abi, Ed25519PrivateKeyParameters signer)
    {
        var program = Repeat(0x11); var code = Repeat(0x22); var state = Repeat(0x33);
        var material = Encoding.UTF8.GetBytes("LayerX/program-discovery-proof/v1\0").Concat(program).Concat(new byte[] { 1 })
            .Concat(Be(3, 4)).Concat(code).Concat(Be(abi, 2)).Concat(Be(77, 8)).Concat(Be(1000, 8)).Concat(Be(10000, 8)).Concat(state).ToArray();
        var digest = SHA256.HashData(material); var signature = new Ed25519Signer(); signature.Init(true, signer);
        signature.BlockUpdate(digest, 0, digest.Length);
        string Hex(byte[] bytes) => Convert.ToHexString(bytes).ToLowerInvariant();
        return new()
        {
            ["program_id"] = JsonValue.String(Hex(program)), ["lifecycle"] = JsonValue.String("active"),
            ["version"] = JsonValue.Integer(3), ["code_hash"] = JsonValue.String(Hex(code)), ["abi_version"] = JsonValue.Integer(abi),
            ["receipt_digest"] = JsonValue.String(Hex(digest)), ["deployment_receipt_digest"] = JsonValue.String(Hex(Repeat(0x44))),
            ["state_root"] = JsonValue.String(Hex(state)), ["observed_sequence"] = JsonValue.String("77"),
            ["observed_at"] = JsonValue.String("1000"), ["valid_through"] = JsonValue.String("10000"),
            ["verification"] = JsonValue.String("registry-receipt-and-current-head-verified"),
            ["discovery_public_key"] = JsonValue.String(Hex(signer.GeneratePublicKey().GetEncoded())),
            ["discovery_signature"] = JsonValue.String(Hex(signature.GenerateSignature())),
        };
    }

    private static JsonDocument Fixture(string name, [CallerFilePath] string source = "") => JsonDocument.Parse(File.ReadAllText(
        Path.Combine(Path.GetDirectoryName(source)!, "..", "..", "..", "conformance", "fixtures", name)));
    private static byte[] FixtureBytes(JsonElement element, string name) => Convert.FromHexString(element.GetProperty(name).GetString()!);

    private static object? Invoke(string name, params object[] arguments)
    {
        var method = typeof(ProgramsClient).GetMethod(name, BindingFlags.NonPublic | BindingFlags.Static)
            ?? throw new InvalidOperationException($"missing {name}");
        return method.Invoke(null, arguments);
    }

    private static bool TransportStatus(string operation, JsonValue value, JsonValue status)
    {
        var method = typeof(AgentHttpTransport).GetMethod("ValidProgramVerification", BindingFlags.NonPublic | BindingFlags.Static)
            ?? throw new InvalidOperationException("missing ValidProgramVerification");
        return (bool)(method.Invoke(null, [operation, value, status]) ?? false);
    }

    private static JsonValue Status(string state, string? reason)
    {
        var fields = new Dictionary<string, JsonValue>
        {
            ["state"] = JsonValue.String(state),
        };
        if (reason is null) fields["level"] = JsonValue.String("SequencerSigned");
        else
        {
            fields["requested"] = JsonValue.String("SequencerSigned");
            fields["achieved"] = JsonValue.String("Unverified");
            fields["reason"] = JsonValue.String(reason);
        }
        return JsonValue.Object(fields);
    }

    private static T Property<T>(object value, string name) => (T)(value.GetType().GetProperty(name)?.GetValue(value)
        ?? throw new InvalidOperationException($"missing {name}"));

    private static byte[] TransferAuthorization(int version)
    {
        var program = Repeat(1); var principal = Repeat(2); var asset = Repeat(4); var destination = Repeat(5);
        using var stream = new MemoryStream(); Write(stream, Encoding.UTF8.GetBytes($"LayerX/programs/402LXP/transfer-set/v{version}\0"));
        Write(stream, program); Write(stream, principal); Write(stream, Repeat(3)); Write(stream, new byte[9]);
        using var events = new MemoryStream(); Write(events, Encoding.UTF8.GetBytes("LayerX/programs/events/v1\0")); Write(events, Be(0, 4));
        Write(stream, Be((ulong)events.Length, 4)); Write(stream, events.ToArray()); Write(stream, Be(0, 8)); Write(stream, Be(1, 8));
        Write(stream, new byte[9]); if (version == 2) { stream.WriteByte(1); Write(stream, principal); }
        Write(stream, asset); Write(stream, destination); Write(stream, U128(7)); Write(stream, program); return stream.ToArray();
    }

    private static byte[] EmptyOccupancy(int version)
    {
        using var stream = new MemoryStream(); Write(stream, Encoding.UTF8.GetBytes($"LXP/storage-occupancy-settlement/v{version}\0"));
        Write(stream, Be(1, 8)); if (version > 1) Write(stream, Be(1, 4));
        for (ulong value = 1; value <= 7; value++) Write(stream, Be(value, 8));
        if (version == 3) { Write(stream, new byte[16 * 4]); Write(stream, Be(0, 4)); }
        else { Write(stream, new byte[16 * 2]); Write(stream, Be(0, 8)); }
        return stream.ToArray();
    }

    private static byte[] ChargedOccupancy()
    {
        var program = Repeat(0x11); var payer = Repeat(0x77); using var stream = new MemoryStream();
        Write(stream, Encoding.UTF8.GetBytes("LXP/storage-occupancy-settlement/v3\0")); Write(stream, Be(2, 8)); Write(stream, Be(1, 4));
        foreach (ulong value in new ulong[] { 0, 0, 0, 0, 0, 0, 2 }) Write(stream, Be(value, 8));
        Write(stream, U128(3)); Write(stream, U128(6)); Write(stream, U128(6)); Write(stream, U128(0)); Write(stream, Be(1, 4));
        stream.WriteByte(65); Write(stream, program); stream.WriteByte(0); Write(stream, payer);
        Write(stream, payer); Write(stream, program); Write(stream, Repeat(0x88));
        Write(stream, Be(1, 8)); Write(stream, Be(2, 8)); Write(stream, Be(3, 8)); Write(stream, Be(3, 8));
        Write(stream, U128(3)); Write(stream, Be(2, 8)); Write(stream, U128(6)); Write(stream, U128(0));
        Write(stream, U128(6)); Write(stream, U128(0)); stream.WriteByte(1); Write(stream, U128(0));
        Write(stream, Be(3, 8)); Write(stream, Be(2, 8)); Write(stream, U128(0)); Write(stream, Repeat(0x99));
        return stream.ToArray();
    }

    private static byte[] Repeat(byte value) => Enumerable.Repeat(value, 32).ToArray();
    private static byte[] U128(ulong value) => new byte[8].Concat(Be(value, 8)).ToArray();
    private static byte[] Be(ulong value, int length)
    {
        var encoded = new byte[8]; BinaryPrimitives.WriteUInt64BigEndian(encoded, value); return encoded[(8 - length)..];
    }
    private static void Write(Stream stream, byte[] value) => stream.Write(value);
}
