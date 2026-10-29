#nullable enable

using System.Buffers.Binary;
using System.Globalization;
using System.Numerics;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Org.BouncyCastle.Crypto.Digests;

namespace LayerX.Sdk;

public sealed record ProgramBudget(ulong Fuel, ProtocolAmount FeeLimit);

public enum ProgramCapability { StorageRead, StorageWrite, Transfer, EmitEvent, Compose }

public sealed record ProgramCall
{
    public NativeProgramCall? NativeCall { get; }
    public byte[] ProgramId { get; }
    public byte[] Calldata { get; }
    public ProgramBudget Budget { get; }
    public IReadOnlyList<ProgramCapability> Capabilities { get; }
    public byte[] SignedActivity { get; }
    public byte[] SignedTransaction => SignedActivity.ToArray();
    public byte[] TransactionHash => Programs.TransactionHash(SignedTransaction);

    public ProgramCall(byte[] programId, byte[] calldata, ProgramBudget budget,
        IEnumerable<ProgramCapability> capabilities, byte[] signedActivity)
    {
        var bounded = capabilities?.ToArray();
        if (programId?.Length != 32 || programId.All(value => value == 0) || calldata is null || calldata.Length > 1_048_576 || budget is null || budget.Fuel == 0 || bounded is null || bounded.Length > 5 ||
            bounded.Zip(bounded.Skip(1)).Any(pair => pair.First >= pair.Second) ||
            signedActivity is null || signedActivity.Length == 0 || signedActivity.Length > 1_048_576) throw Invalid();
        ProgramId = programId.ToArray(); Calldata = calldata.ToArray(); Budget = budget; Capabilities = Array.AsReadOnly(bounded);
        SignedActivity = signedActivity.ToArray();
    }
    public ProgramCall(NativeProgramCall nativeCall, ProtocolAmount feeLimit, byte[] signedActivity)
        : this(nativeCall.ProgramId, nativeCall.Calldata, new ProgramBudget(nativeCall.Resources[0], feeLimit), Array.Empty<ProgramCapability>(), signedActivity)
    {
        _ = nativeCall.Encode(); NativeCall = nativeCall;
    }
    public ProgramCall(NativeProgramCall nativeCall, ProtocolAmount feeLimit)
    {
        ArgumentNullException.ThrowIfNull(nativeCall);
        _ = nativeCall.Encode(); NativeCall = nativeCall;
        ProgramId = nativeCall.ProgramId.ToArray(); Calldata = nativeCall.Calldata.ToArray();
        Budget = new ProgramBudget(nativeCall.Resources[0], feeLimit);
        Capabilities = Array.Empty<ProgramCapability>(); SignedActivity = [];
    }

    public byte[] EncodeCalldata(bool simulate = false) =>
        (NativeCall ?? throw Invalid()).EncodeCalldata(Budget.FeeLimit, simulate);

    public UnsignedProgramTransaction BuildTransaction(string sender, ulong nonce, ulong gas,
        BigInteger maxFeePerGas, BigInteger maxPriorityFeePerGas) =>
        new(sender, EncodeCalldata(), nonce, gas, maxFeePerGas, maxPriorityFeePerGas);

    private static PlatformSdkException Invalid() => new(SdkErrorCode.InvalidArgument, RetryClass.Never);
}

public enum ProgramLifecycle { Active, Deprecated, Tombstoned }
public abstract record ProgramSource
{
    public sealed record Unpublished : ProgramSource;
    public sealed record Verified(byte[] SourceDigest, byte[] EnvironmentDigest, string Pipeline) : ProgramSource;
    public sealed record Mismatch(byte[] ExpectedCodeHash, byte[] ReproducedArtifactDigest) : ProgramSource;
}
public sealed record ProgramDiscovery(byte[] ProgramId, ProgramLifecycle Lifecycle, uint Version, byte[] CodeHash,
    ushort AbiVersion, byte[] ReceiptDigest, byte[] StateRoot, ulong ObservedSequence, ulong ObservedAt,
    ulong ValidThrough, string Verification)
{
    public byte[]? DeploymentReceiptDigest { get; init; }
}
public sealed record ProgramInterface(byte[] ProgramId, uint Version, byte[] CodeHash, ushort AbiVersion,
    byte[] Interface, byte[] InterfaceDigest, byte[] ReceiptDigest, byte[] StateRoot, ulong ObservedSequence,
    ulong ObservedAt, ulong ValidThrough, ProgramSource Source, string Verification);
public sealed class VerifiedProgramExecution
{
    public string TransferVerification { get; }
    public JsonValue Value { get; }
    public ReceiptVerification Receipt { get; }
    public byte[] TerminalPayload { get; }
    public byte[] CallGraph { get; }
    public ushort GuestAbiVersion => checked((ushort)((JsonValue.IntegerValue)((JsonValue.ObjectValue)Value).Value["guest_abi_version"]).Value);
    public byte[] TerminalPayloadRoot => Receipt.Receipt.ProgramOutcome!.TerminalPayloadRoot.ToArray();
    public byte[] CallGraphRoot => Receipt.Receipt.ProgramOutcome!.CallGraphRoot.ToArray();

    internal VerifiedProgramExecution(JsonValue value, ReceiptVerification receipt, byte[] terminalPayload, byte[] callGraph, string transferVerification)
    {
        Value = value; Receipt = receipt; TransferVerification = transferVerification;
        TerminalPayload = terminalPayload.ToArray(); CallGraph = callGraph.ToArray();
    }
}

public sealed record ProgramSimulation(JsonValue Value, int TerminalResult, byte TerminalKind, byte[] Response);
public sealed class ProgramSubmission
{
    public JsonValue Value { get; }
    public string State { get; }
    public bool IsUnknown => State == "unknown";
    public byte[] TransactionHash { get; }
    public byte[] ActivityId => TransactionHash.ToArray();
    public SequencerReceiptVerification? SequencerReceipt { get; }
    public string IdempotencyKey { get; }
    public byte[]? RetainedSignedActivity { get; }
    public byte[]? RetainedSignedTransaction => RetainedSignedActivity?.ToArray();
    public VerifiedProgramExecution? Execution { get; }
    internal ProgramSubmission(JsonValue value, string state, byte[] activityId, string idempotencyKey,
        byte[]? retainedSignedActivity, VerifiedProgramExecution? execution)
    {
        Value = value; State = state; TransactionHash = activityId.ToArray(); IdempotencyKey = idempotencyKey;
        RetainedSignedActivity = retainedSignedActivity?.ToArray(); Execution = execution;
    }
    internal ProgramSubmission(JsonValue value, string state, byte[] transactionHash, string idempotencyKey,
        byte[]? signedTransaction, SequencerReceiptVerification? receipt, bool evm)
        : this(value, state, transactionHash, idempotencyKey, signedTransaction, null)
    {
        SequencerReceipt = receipt;
    }

}

public sealed class ProgramsClient
{
    private readonly Programs? _rpc;
    private readonly ReceiptKeys? _receiptKeys;
    private readonly byte[]? _didId;
    private Programs Rpc => _rpc ?? throw new PlatformSdkException(SdkErrorCode.UnavailableCapability, RetryClass.Never);

    public ProgramsClient(Programs rpc, ReceiptKeys? receiptKeys = null, byte[]? didId = null)
    {
        _rpc = rpc ?? throw Invalid(); _receiptKeys = receiptKeys;
        if (didId is not null && didId.Length != 32) throw Invalid();
        _didId = didId?.ToArray(); _sequencerPublicKey = []; _protocolVersion = 3;
        _timeProvider = TimeProvider.System; _maximumSimulationAgeMilliseconds = 300_000;
    }

    public Task<UnsignedProgramTransaction> CallAsync(ProgramCall call, string sender,
        BigInteger maxFeePerGas, BigInteger maxPriorityFeePerGas, CancellationToken cancellationToken = default) =>
        BuildTransactionAsync(call.EncodeCalldata(), sender, maxFeePerGas, maxPriorityFeePerGas, cancellationToken);

    public Task<UnsignedProgramTransaction> DeployAsync(NativeProgramDeploy value, string sender,
        BigInteger maxFeePerGas, BigInteger maxPriorityFeePerGas, CancellationToken cancellationToken = default) =>
        BuildTransactionAsync(new NativeProgramLifecycleRequest(value).Payload, sender, maxFeePerGas, maxPriorityFeePerGas, cancellationToken);

    public Task<UnsignedProgramTransaction> UpgradeAsync(NativeProgramUpgrade value, ProtocolAmount feeLimit, string sender,
        BigInteger maxFeePerGas, BigInteger maxPriorityFeePerGas, CancellationToken cancellationToken = default) =>
        BuildTransactionAsync(new NativeProgramLifecycleRequest(value, feeLimit).Payload, sender, maxFeePerGas, maxPriorityFeePerGas, cancellationToken);

    public Task<UnsignedProgramTransaction> WindDownAsync(NativeProgramWindDown value, string sender,
        BigInteger maxFeePerGas, BigInteger maxPriorityFeePerGas, CancellationToken cancellationToken = default) =>
        BuildTransactionAsync(new NativeProgramLifecycleRequest(value).Payload, sender, maxFeePerGas, maxPriorityFeePerGas, cancellationToken);

    private async Task<UnsignedProgramTransaction> BuildTransactionAsync(byte[] data, string sender,
        BigInteger maxFeePerGas, BigInteger maxPriorityFeePerGas, CancellationToken cancellationToken)
    {
        Programs.Address(sender);
        var nonce = Programs.Quantity(await Rpc.SendAsync("eth_getTransactionCount", [JsonValue.String(sender), JsonValue.String("pending")], cancellationToken).ConfigureAwait(false));
        var transaction = new UnsignedProgramTransaction(sender, data, checked((ulong)nonce), 0, maxFeePerGas, maxPriorityFeePerGas);
        var gas = Programs.Quantity(await Rpc.SendAsync("eth_estimateGas", [transaction.ToJson(includeGas: false), JsonValue.String("pending")], cancellationToken).ConfigureAwait(false));
        return new UnsignedProgramTransaction(sender, data, checked((ulong)nonce), checked((ulong)gas), maxFeePerGas, maxPriorityFeePerGas);
    }

    public Task<SequencerReceiptVerification> SubmitAsync(NativeProgramLifecycleRequest request, byte[] signedTransaction,
        IdempotencyKey key, CancellationToken cancellationToken = default) =>
        SubmitLifecycleAsync(request, signedTransaction, key, cancellationToken);

    public Task<SequencerReceiptVerification> LifecycleReceiptAsync(NativeProgramLifecycleRequest request, CancellationToken cancellationToken = default) =>
        ReadReceiptAsync(request.TransactionHash, request.SignedTransaction, cancellationToken);

    public Task<SequencerReceiptVerification> DeployAsync(NativeProgramDeploy value, byte[] signedTransaction, IdempotencyKey key, CancellationToken cancellationToken = default) =>
        SubmitLifecycleAsync(new NativeProgramLifecycleRequest(value), signedTransaction, key, cancellationToken);
    public Task<SequencerReceiptVerification> UpgradeAsync(NativeProgramUpgrade value, ProtocolAmount feeLimit, byte[] signedTransaction, IdempotencyKey key, CancellationToken cancellationToken = default) =>
        SubmitLifecycleAsync(new NativeProgramLifecycleRequest(value, feeLimit), signedTransaction, key, cancellationToken);
    public Task<SequencerReceiptVerification> WindDownAsync(NativeProgramWindDown value, byte[] signedTransaction, IdempotencyKey key, CancellationToken cancellationToken = default) =>
        SubmitLifecycleAsync(new NativeProgramLifecycleRequest(value), signedTransaction, key, cancellationToken);

    private async Task<SequencerReceiptVerification> SubmitLifecycleAsync(NativeProgramLifecycleRequest request,
        byte[] signedTransaction, IdempotencyKey key, CancellationToken cancellationToken)
    {
        if (!Hex32(key.Value)) throw Invalid();
        _ = ReceiptPins();
        Programs.BindTransaction(signedTransaction, request.Payload);
        var hash = await SendTransactionAsync(signedTransaction, cancellationToken).ConfigureAwait(false);
        return await ReadReceiptAsync(hash, signedTransaction, cancellationToken).ConfigureAwait(false);
    }

    public const ushort ReceiptModuleId = 9; public const byte CallOperation = 3;
    private const int MaximumProgramBytes = 1_048_576;
    private const int MaximumInterfaceBytes = 952;
    private static readonly int MaximumCallGraphBytes = Encoding.UTF8.GetByteCount("LayerX/programs/call-graph/v1\0") + 32 + 16 + 8 + 64 * 68;
    private readonly byte[] _sequencerPublicKey;
    private readonly ushort _protocolVersion;
    private readonly TimeProvider _timeProvider;
    private readonly ulong _maximumSimulationAgeMilliseconds;
    private sealed record ActivityBinding(byte[] ActivityId, byte[] IdempotencyKey, ulong NotBefore, ulong NotAfter);
    private sealed record TerminalUsage(ulong Cpu, ulong Memory, ulong Read, ulong Write,
        uint Values, ulong OutputBytes, BigInteger Fee);
    private sealed record TerminalAttachments(byte[] Inner, byte[]? Occupancy, byte[]? Authorization, byte[]? TransferRoot);
    private sealed record ProgramAuthorityBinding(byte[] Owner, byte[] Frame, byte[] Source, byte[] Asset,
        byte[] Destination, BigInteger Amount);
    private sealed record ProgramFundingBinding(byte[] Owner, byte[] Destination, byte[] Asset);
    private sealed record OccupancyChargeBinding(byte[] Payer, BigInteger AmountDue, bool Paid, BigInteger ArrearsAfter);
    private sealed record OccupancySettlementBinding(BigInteger ByteBatches, BigInteger FeeUnits,
        IReadOnlyList<OccupancyChargeBinding> Charges);
    private sealed record StorageNamespaceBinding(byte[] Canonical, byte[] Wire, byte[] Program, byte[]? Principal);
    private sealed class OccupancyPayer
    {
        internal required byte[] Payer { get; init; }
        internal BigInteger Due { get; set; }
        internal BigInteger Paid { get; set; }
        internal BigInteger Arrears { get; set; }
    }

    public ProgramsClient(PlatformClient client, byte[] sequencerPublicKey,
        TimeProvider? timeProvider = null, TimeSpan? maximumSimulationAge = null, ushort protocolVersion = 2)
    {
        if (protocolVersion != 2 && protocolVersion != 3) throw Invalid();
        _protocolVersion = protocolVersion;
        ArgumentNullException.ThrowIfNull(client);
        if (sequencerPublicKey?.Length != 32 || sequencerPublicKey.All(value => value == 0)) throw Invalid();
        _sequencerPublicKey = sequencerPublicKey.ToArray(); _timeProvider = timeProvider ?? TimeProvider.System;
        var age = maximumSimulationAge ?? TimeSpan.FromMinutes(5);
        if (age <= TimeSpan.Zero || age.TotalMilliseconds > ulong.MaxValue) throw Invalid();
        _maximumSimulationAgeMilliseconds = checked((ulong)age.TotalMilliseconds);
    }

    public async Task<ProgramDiscovery> DiscoverAsync(byte[] programId, string verificationLevel, CancellationToken cancellationToken = default)
    {
        _ = Identifier(programId); _ = Level(verificationLevel);
        var data = await Rpc.CallAsync(Programs.AddressValue, ProgramsAbi.Encode("b1ffb3d4", ProgramsAbi.Fixed(programId)), cancellationToken).ConfigureAwait(false);
        if (data.Length != 192) throw Decode();
        _ = ProgramsAbi.ReadWord(data, 0, 8);
        var abi = checked((ushort)ProgramsAbi.ReadWord(data, 3, 16));
        var version = checked((uint)ProgramsAbi.ReadWord(data, 4, 32));
        var active = ProgramsAbi.ReadWord(data, 5, 1) == 1;
        return new(programId.ToArray(), active ? ProgramLifecycle.Active : ProgramLifecycle.Tombstoned, version,
            data[64..96], abi, [], [], 0, 0, 0, "pending-eth-call");
    }

    public async Task<ProgramInterface> InterfaceAsync(byte[] programId, string verificationLevel, CancellationToken cancellationToken = default)
    {
        _ = Identifier(programId); _ = Level(verificationLevel);
        var data = await Rpc.CallAsync(Programs.AddressValue, ProgramsAbi.Encode("8efc7240", ProgramsAbi.Fixed(programId)), cancellationToken).ConfigureAwait(false);
        var version = checked((uint)ProgramsAbi.ReadWord(data, 0, 32));
        var encoding = ProgramsAbi.ReadBytes(data, 2, 96, MaximumInterfaceBytes);
        var digest = data[32..64];
        if (!Fixed(SHA256.HashData(encoding), digest)) throw Verify();
        var head = await DiscoverAsync(programId, verificationLevel, cancellationToken).ConfigureAwait(false);
        if (head.Version != version || !InterfaceHeaderBound(encoding, head.CodeHash, head.AbiVersion)) throw Verify();
        return new(programId.ToArray(), version, head.CodeHash, head.AbiVersion, encoding, digest, [], [], 0, 0, 0,
            new ProgramSource.Unpublished(), "pending-eth-call");
    }

    public Task<ProgramSimulation> SimulateAsync(ProgramCall call, CancellationToken cancellationToken = default) =>
        SimulateCoreAsync(call, null, cancellationToken);

    public Task<ProgramSimulation> SimulateAsync(ProgramCall call, string sender, CancellationToken cancellationToken = default)
    {
        Programs.Address(sender);
        return SimulateCoreAsync(call, sender, cancellationToken);
    }

    private async Task<ProgramSimulation> SimulateCoreAsync(ProgramCall call, string? sender, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(call);
        var data = await Rpc.CallAsync(Programs.AddressValue, call.EncodeCalldata(simulate: true), cancellationToken, sender).ConfigureAwait(false);
        var result = ProgramsAbi.ReadWord(data, 0);
        var signed = result >= (BigInteger.One << 255) ? result - (BigInteger.One << 256) : result;
        if (signed < int.MinValue || signed > int.MaxValue) throw Decode();
        var kind = checked((byte)ProgramsAbi.ReadWord(data, 1, 8));
        if (kind is < 1 or > 3) throw Decode();
        var response = ProgramsAbi.ReadBytes(data, 2, 96, MaximumProgramBytes);
        return new(JsonValue.String(Programs.Hex(data)), (int)signed, kind, response);
    }

    public async Task<ProgramSubmission> SubmitAsync(ProgramCall call, IdempotencyKey idempotencyKey, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(call);
        if (!Hex32(idempotencyKey.Value)) throw Invalid();
        _ = ReceiptPins();
        var raw = call.SignedTransaction;
        Programs.BindTransaction(raw, call.EncodeCalldata());
        var hash = await SendTransactionAsync(raw, cancellationToken).ConfigureAwait(false);
        var value = await Rpc.SendAsync("pax_sequencerReceipt", [JsonValue.String(Programs.Hex(hash))], cancellationToken).ConfigureAwait(false);
        if (value is JsonValue.NullValue || Map(value).TryGetValue("rung", out var rung) && rung is JsonValue.StringValue { Value: "submitted" })
            return new(value, "pending", hash, idempotencyKey.Value, raw, null, true);
        var receipt = VerifyReceiptAsync(value, raw, ReceiptPins());
        return new(value, receipt.Status == 1 ? "executed" : "refused", hash, idempotencyKey.Value, raw, receipt, true);
    }

    public async Task<ProgramSubmission> ReceiptAsync(IdempotencyKey idempotencyKey, byte[] expectedTransactionHash, string verificationLevel, CancellationToken cancellationToken = default)
    {
        if (!Hex32(idempotencyKey.Value) || _didId is null) throw Invalid();
        _ = Identifier(expectedTransactionHash); _ = Level(verificationLevel);
        var data = ProgramsAbi.Encode("2eda9f2b", ProgramsAbi.Fixed(_didId), ProgramsAbi.Fixed(Convert.FromHexString(idempotencyKey.Value)));
        var output = await Rpc.CallAsync(Programs.AuthAddress, data, cancellationToken).ConfigureAwait(false);
        var recorded = ProgramsAbi.ReadBytes(output, 0, 32, MaximumProgramBytes);
        var submission = await ActivityAsync(expectedTransactionHash, verificationLevel, cancellationToken).ConfigureAwait(false);
        if (recorded.Length > 0 && (submission.SequencerReceipt is null || !Fixed(recorded, submission.SequencerReceipt.CanonicalBytes))) throw Verify();
        return new(submission.Value, submission.State, expectedTransactionHash, idempotencyKey.Value,
            submission.RetainedSignedActivity, submission.SequencerReceipt, true);
    }

    public async Task<ProgramSubmission> ActivityAsync(byte[] transactionHash, string verificationLevel, CancellationToken cancellationToken = default)
    {
        _ = Identifier(transactionHash); _ = Level(verificationLevel);
        var transaction = await Rpc.SendAsync("eth_getTransactionByHash", [JsonValue.String(Programs.Hex(transactionHash))], cancellationToken).ConfigureAwait(false);
        if (transaction is JsonValue.NullValue) return new(transaction, "unknown", transactionHash, "", null, null, true);
        var map = Map(transaction);
        if (!Fixed(Programs.Unhex(Text(map, "hash")), transactionHash)) throw Verify();
        var rawValue = await Rpc.SendAsync("eth_getRawTransactionByHash", [JsonValue.String(Programs.Hex(transactionHash))], cancellationToken).ConfigureAwait(false);
        var raw = Programs.Data(rawValue); Programs.RequireSignedTransaction(raw);
        if (!Fixed(Programs.TransactionHash(raw), transactionHash)) throw Verify();
        var view = await Rpc.SendAsync("pax_sequencerReceipt", [JsonValue.String(Programs.Hex(transactionHash))], cancellationToken).ConfigureAwait(false);
        if (view is JsonValue.NullValue || Map(view).TryGetValue("rung", out var rung) && rung is JsonValue.StringValue { Value: "submitted" })
            return new(view, "pending", transactionHash, "", raw, null, true);
        var receipt = VerifyReceiptAsync(view, raw, ReceiptPins());
        return new(view, receipt.Status == 1 ? "executed" : "refused", transactionHash, "", raw, receipt, true);
    }

    public static SequencerReceiptVerification VerifyReceiptAsync(JsonValue view, byte[] rawTransaction, ReceiptKeys keys) =>
        LocalVerifier.VerifySequencerReceipt(JsonSerializer.SerializeToUtf8Bytes(view), rawTransaction, keys);

    private ReceiptKeys ReceiptPins() => _receiptKeys ?? throw Verify();

    private async Task<byte[]> SendTransactionAsync(byte[] raw, CancellationToken cancellationToken)
    {
        Programs.RequireSignedTransaction(raw);
        var expected = Programs.TransactionHash(raw);
        var result = await Rpc.SendAsync("eth_sendRawTransaction", [JsonValue.String(Programs.Hex(raw))], cancellationToken).ConfigureAwait(false);
        if (!Fixed(Programs.Data(result), expected)) throw Verify();
        return expected;
    }

    private async Task<SequencerReceiptVerification> ReadReceiptAsync(byte[] hash, byte[] raw, CancellationToken cancellationToken)
    {
        var view = await Rpc.SendAsync("pax_sequencerReceipt", [JsonValue.String(Programs.Hex(hash))], cancellationToken).ConfigureAwait(false);
        return VerifyReceiptAsync(view, raw, ReceiptPins());
    }

    private static void VerifyRequestedAbi(ProgramCall call, VerifiedProgramExecution execution)
    {
        if (call.NativeCall is { } native && execution.Receipt.Receipt.ProgramOutcome!.AbiVersion != native.GuestAbi)
            throw Verify();
    }

    private object VerifyDiscovery(JsonValue value, string programId, bool @interface, ulong now)
    {
        var map = Map(value);
        string[] fields = @interface
            ? ["program_id",
                "version",
                "code_hash",
                "abi_version",
                "interface",
                "interface_digest",
                "receipt_digest",
                "state_root",
                "observed_sequence",
                "observed_at",
                "valid_through",
                "source",
                "verification"]
            : ["program_id",
                "lifecycle",
                "version",
                "code_hash",
                "abi_version",
                "receipt_digest",
                "state_root",
                "observed_sequence",
                "observed_at",
                "valid_through",
                "verification"];
        string[] optional = @interface ? [] : ["deployment_receipt_digest", "discovery_public_key", "discovery_signature"];
        RequireFields(map, fields.Concat(optional.Where(map.ContainsKey)));
        var observedAt = DecimalUInt64(map, "observed_at"); var validThrough = DecimalUInt64(map, "valid_through");
        var version = UInt32Integer(map, "version"); var abi = UInt16Integer(map, "abi_version");
        if (Text(map, "program_id") != programId || version == 0 || !GeneratedReceiptContract.SupportsProgramGuestAbi(abi) ||
            !Hex32(Text(map, "code_hash")) || !Hex32(Text(map, "receipt_digest")) || !Hex32(Text(map, "state_root")) ||
            validThrough < observedAt || observedAt > now || now > validThrough ||
            Text(map, "verification") != (@interface ? "deployment-interface-and-current-head-verified" :
                "registry-receipt-and-current-head-verified")) throw Verify();
        var observedSequence = DecimalUInt64(map, "observed_sequence");
        var program = Bytes(map, "program_id", 32, true); var codeHash = Bytes(map, "code_hash", 32, true);
        var receiptDigest = Bytes(map, "receipt_digest", 32, true); var stateRoot = Bytes(map, "state_root", 32, true);
        if (@interface)
        {
            var bytes = Bytes(map, "interface", MaximumInterfaceBytes); var digest = Bytes(map, "interface_digest", 32, true);
            if (bytes.Length == 0 || !Fixed(SHA256.HashData(bytes), digest) || !InterfaceHeaderBound(bytes, codeHash, abi)) throw Verify();
            var source = TypedSource(Map(Field(map, "source")));
            return new ProgramInterface(program, version, codeHash, abi, bytes, digest, receiptDigest, stateRoot,
                observedSequence, observedAt, validThrough, source, "server-side-receipt-verification-only");
        }
        var lifecycle = Text(map, "lifecycle") switch
        {
            "active" => ProgramLifecycle.Active,
            "deprecated" => ProgramLifecycle.Deprecated,
            "tombstoned" => ProgramLifecycle.Tombstoned,
            _ => throw Decode()
        };
        var signed = map.ContainsKey("discovery_public_key") || map.ContainsKey("discovery_signature");
        var deployment = map.ContainsKey("deployment_receipt_digest") ? Bytes(map, "deployment_receipt_digest", 32, true) : null;
        if (deployment is not null && deployment.All(item => item == 0)) throw Verify();
        if (signed)
        {
            if (deployment is null || !map.ContainsKey("discovery_public_key") || !map.ContainsKey("discovery_signature")) throw Verify();
            var key = Bytes(map, "discovery_public_key", 32, true);
            var signature = Bytes(map, "discovery_signature", 64, true);
            var digest = Digest(Encoding.UTF8.GetBytes("LayerX/program-discovery-proof/v1\0"), program, [1],
                BigEndian(version, 4), codeHash, BigEndian(abi, 2), BigEndian(observedSequence, 8),
                BigEndian(observedAt, 8), BigEndian(validThrough, 8), stateRoot);
            if (!Fixed(key, _sequencerPublicKey) || !Fixed(digest, receiptDigest) ||
                !LocalVerifier.VerifyEd25519Digest(key, signature, digest)) throw Verify();
        }
        if (abi is GeneratedReceiptContract.ProgramAbiV3 or GeneratedReceiptContract.ProgramAbiV4 && !signed) throw Verify();
        return new ProgramDiscovery(program, lifecycle, version, codeHash, abi, receiptDigest, stateRoot,
            observedSequence, observedAt, validThrough, signed ? "sequencer-signed-discovery-head" : "server-side-receipt-verification-only")
        { DeploymentReceiptDigest = deployment };
    }

    private static bool InterfaceHeaderBound(byte[] bytes, byte[] codeHash, ushort abi)
    {
        var domain = Encoding.UTF8.GetBytes("LayerX/program-interface/v1\0");
        if (bytes.Length < domain.Length + 36 || bytes.Length > MaximumInterfaceBytes) return false;
        var version = abi switch
        {
            GeneratedReceiptContract.ProgramAbiV1 => 1,
            GeneratedReceiptContract.ProgramAbiV2 => Starts(bytes, "LayerX/program-interface/v2\0") ? 2 : 1,
            GeneratedReceiptContract.ProgramAbiV3 => 3,
            GeneratedReceiptContract.ProgramAbiV4 => 4,
            _ => 0,
        };
        if (version == 0 || !Starts(bytes, "LayerX/program-interface/v" + version + "\0")) return false;
        var entries = BinaryPrimitives.ReadUInt16BigEndian(bytes.AsSpan(domain.Length + 34));
        return Fixed(bytes.AsSpan(domain.Length, 32).ToArray(), codeHash) &&
            BinaryPrimitives.ReadUInt16BigEndian(bytes.AsSpan(domain.Length + 32)) == abi && entries is > 0 and <= 256;
    }

    private static ProgramSource TypedSource(IReadOnlyDictionary<string, JsonValue> source)
    {
        ValidateSource(source);
        return Text(source, "status") switch
        {
            "unpublished" => new ProgramSource.Unpublished(),
            "verified" => new ProgramSource.Verified(Bytes(source, "source_digest", 32, true),
                Bytes(source, "environment_digest", 32, true), Text(source, "pipeline")),
            "mismatch" => new ProgramSource.Mismatch(Bytes(source, "expected_code_hash", 32, true),
                Bytes(source, "reproduced_artifact_digest", 32, true)),
            _ => throw Decode()
        };
    }

    private static void ValidateSource(IReadOnlyDictionary<string, JsonValue> source)
    {
        switch (Text(source, "status"))
        {
            case "unpublished": RequireFields(source, ["status"]); break;
            case "verified":
                RequireFields(source, ["status", "source_digest", "environment_digest", "pipeline"]);
                _ = Bytes(source, "source_digest", 32, true); _ = Bytes(source, "environment_digest", 32, true);
                if (Text(source, "pipeline") != "sha256-source-artifact-reproducible-build-v1") throw Decode();
                break;
            case "mismatch":
                RequireFields(source, ["status", "expected_code_hash", "reproduced_artifact_digest"]);
                _ = Bytes(source, "expected_code_hash", 32, true); _ = Bytes(source, "reproduced_artifact_digest", 32, true);
                break;
            default: throw Decode();
        }
    }

    private static string ValidateOutcome(IReadOnlyDictionary<string, JsonValue> outcome)
    {
        var kind = Text(outcome, "kind");
        switch (kind)
        {
            case "completed":
                RequireFields(outcome, ["kind", "code", "response"]);
                _ = Integer32(outcome, "code"); _ = Bytes(outcome, "response", MaximumProgramBytes, empty: true); break;
            case "legacy_completed":
                RequireFields(outcome, ["kind", "code", "values"]); _ = Integer32(outcome, "code");
                if (Field(outcome, "values") is not JsonValue.ArrayValue values || values.Value.Count > 512) throw Decode();
                foreach (var value in values.Value) ValidateLegacyValue(value); break;
            case "refused": RequireFields(outcome, ["kind", "failure"]); ValidateFailure(Map(Field(outcome, "failure"))); break;
            default: throw Decode();
        }
        return kind;
    }

    private static void ValidateLegacyValue(JsonValue value)
    {
        var map = Map(value); RequireFields(map, ["type", "value"]);
        if (Text(map, "type") == "i32") _ = Integer32(map, "value");
        else if (Text(map, "type") == "i64") _ = DecimalInt64(map, "value");
        else throw Decode();
    }

    private static void ValidateFailure(IReadOnlyDictionary<string, JsonValue> failure)
    {
        switch (Text(failure, "kind"))
        {
            case "unknown_program":
            case "reentrancy":
            case "authority":
            case "resource":
            case "response":
            case "fault":
                RequireFields(failure, ["kind"]); break;
            case "depth_exceeded":
            case "fanout_exceeded":
                RequireFields(failure, ["kind", "limit", "attempted"]);
                _ = UInt32Integer(failure, "limit"); _ = UInt32Integer(failure, "attempted"); break;
            case "guest_refused": RequireFields(failure, ["kind", "code"]); _ = Integer32(failure, "code"); break;
            default: throw Decode();
        }
    }

    private static string VerifyTerminal(byte[] encoded, byte[] availableGraph, byte[] expectedProgram,
        IReadOnlyDictionary<string, JsonValue> documentOutcome, ushort protocolVersion, ProgramReceiptOutcome receipt)
    {
        try
        {
            if (encoded.Length == 0 || encoded.Length > MaximumProgramBytes || !Fixed(SHA256.HashData(encoded), receipt.TerminalPayloadRoot) ||
                availableGraph.Length == 0 || !Fixed(SHA256.HashData(availableGraph), receipt.CallGraphRoot)) throw new InvalidDataException();
            var attachments = UnwrapTerminal(UnwrapAppliedTerminal(encoded, receipt)); var inner = attachments.Inner;
            var modern = Starts(inner, "LXP/program-execution/v5\0");
            var candidate = Starts(inner, "LXP/program-execution/v4\0") || modern; var successful = false;
            if (Starts(inner, "LXP/program-execution/v2\0") || Starts(inner, "LXP/program-execution/v3\0"))
            {
                var traced = Starts(inner, "LXP/program-execution/v3\0"); var domain = Encoding.UTF8.GetBytes(
                    traced ? "LXP/program-execution/v3\0" : "LXP/program-execution/v2\0");
                var cursor = new TerminalCursor(inner, domain.Length); var runtime = cursor.U16(); var abi = cursor.U16();
                var metering = cursor.U32(); var countValue = cursor.U128();
                if (countValue > cursor.Remaining / 5) throw new InvalidDataException(); var count = (int)countValue;
                if (Field(documentOutcome, "values") is not JsonValue.ArrayValue values || values.Value.Count != count ||
                    Text(documentOutcome, "kind") != "legacy_completed" || runtime == 0 || metering == 0 ||
                    runtime != receipt.RuntimeVersion || abi != 1 || abi != receipt.AbiVersion ||
                    metering != receipt.MeteringScheduleVersion) throw new InvalidDataException();
                for (var index = 0; index < count; index++)
                {
                    var value = Map(values.Value[index]); var tag = cursor.U8();
                    if (tag == 1 && (Text(value, "type") != "i32" || cursor.I32() != Integer32(value, "value"))) throw new InvalidDataException();
                    if (tag == 2 && (Text(value, "type") != "i64" || cursor.I64() != DecimalInt64(value, "value"))) throw new InvalidDataException();
                    if (tag is not (1 or 2)) throw new InvalidDataException();
                }
                var usage = new TerminalUsage(cursor.U64(), cursor.U64(), cursor.U64(), cursor.U64(), cursor.U32(), 0, cursor.U128());
                if (traced && (cursor.U8() != 1 || cursor.Sized64().Length > 34 + 65_536 * 52)) throw new InvalidDataException();
                cursor.Finish(); if (receipt.TerminalKind != 1 || Integer32(documentOutcome, "code") < 0) throw new InvalidDataException();
                MatchUsage(usage, receipt); successful = true;
            }
            else if (candidate)
            {
                var cursor = new TerminalCursor(inner, Encoding.UTF8.GetByteCount(modern ? "LXP/program-execution/v5\0" : "LXP/program-execution/v4\0"));
                var runtime = cursor.U16(); var feeSchedule = cursor.U32(); var metering = cursor.U32();
                var countValue = cursor.U64(); if (countValue > (ulong)(cursor.Remaining / 5)) throw new InvalidDataException();
                for (var index = 0; index < (int)countValue; index++)
                {
                    var tag = cursor.U8(); if (tag == 1) _ = cursor.I32();
                    else if (tag == 2) _ = cursor.I64();
                    else throw new InvalidDataException();
                }
                var usage = new TerminalUsage(cursor.U64(), cursor.U64(), cursor.U64(), cursor.U64(), cursor.U32(),
                    cursor.U64(), cursor.U128());
                var traceTag = cursor.U8(); if (traceTag == 1 && cursor.Sized64().Length > 34 + 65_536 * 52) throw new InvalidDataException();
                if (traceTag is not (0 or 1)) throw new InvalidDataException();
                var program = cursor.Take(32); var abi = cursor.U16(); var outcomeTag = cursor.U8(); string expectedKind;
                if (outcomeTag == 0)
                {
                    var code = cursor.I32(); var response = cursor.Sized64();
                    if (code < 0 || response.Length > MaximumProgramBytes || Text(documentOutcome, "kind") != "completed" ||
                        code != Integer32(documentOutcome, "code") || !Fixed(response, Bytes(documentOutcome, "response", MaximumProgramBytes, empty: true)))
                        throw new InvalidDataException();
                    expectedKind = "completed"; successful = true;
                }
                else if (outcomeTag == 1) { ValidateAuthenticatedProgramFailure(cursor.Sized64()); expectedKind = "guest_refused"; }
                else if (outcomeTag == 2) { ValidateCandidateResource(cursor, usage); expectedKind = "resource"; }
                else throw new InvalidDataException();
                var graph = cursor.Sized64(); cursor.Finish();
                if (graph.Length > MaximumCallGraphBytes || !Fixed(graph, availableGraph) || !Fixed(program, expectedProgram) ||
                    (modern ? protocolVersion != 3 || abi is not (GeneratedReceiptContract.ProgramAbiV3 or GeneratedReceiptContract.ProgramAbiV4) : abi != 2) ||
                    abi != receipt.AbiVersion || runtime == 0 || feeSchedule == 0 || metering == 0 ||
                    runtime != receipt.RuntimeVersion || feeSchedule != receipt.FeeScheduleVersion ||
                    metering != receipt.MeteringScheduleVersion) throw new InvalidDataException();
                MatchUsage(usage, receipt);
                if (outcomeTag == 0) { if (receipt.TerminalKind != 1) throw new InvalidDataException(); }
                else { RequireRefusal(documentOutcome, expectedKind, receipt.ResultCode); if (receipt.TerminalKind == 1) throw new InvalidDataException(); }
            }
            else if (Starts(inner, "LXP/programs/failure-detail/v1\0"))
            {
                var cursor = new TerminalCursor(inner, Encoding.UTF8.GetByteCount("LXP/programs/failure-detail/v1\0"));
                var family = cursor.U8(); var payload = cursor.Sized32(); cursor.Finish();
                if (family is < 1 or > 4 || payload.Length == 0) throw new InvalidDataException();
                ValidateFailureDetail(family, payload); RequireRefusal(documentOutcome, "guest_refused", receipt.ResultCode);
                if (receipt.TerminalKind != 2) throw new InvalidDataException();
            }
            else if (Starts(inner, "LXP/programs/resource-detail/v1\0"))
            {
                var cursor = new TerminalCursor(inner, Encoding.UTF8.GetByteCount("LXP/programs/resource-detail/v1\0"));
                ValidateLegacyResource(cursor); cursor.Finish(); RequireRefusal(documentOutcome, "resource", receipt.ResultCode);
                if (receipt.TerminalKind != 3) throw new InvalidDataException();
            }
            else if (Starts(inner, "LXP/programs/settlement-failure/v1\0"))
            {
                var cursor = new TerminalCursor(inner, Encoding.UTF8.GetByteCount("LXP/programs/settlement-failure/v1\0"));
                if (cursor.U8() is < 1 or > 12) throw new InvalidDataException(); cursor.Finish();
                RequireRefusal(documentOutcome, "guest_refused", receipt.ResultCode); if (receipt.TerminalKind != 2) throw new InvalidDataException();
            }
            else if (Starts(inner, "LXP/programs/callback-failure/v1\0"))
            {
                var cursor = new TerminalCursor(inner, Encoding.UTF8.GetByteCount("LXP/programs/callback-failure/v1\0"));
                _ = cursor.U8(); _ = cursor.I32(); cursor.Finish(); RequireRefusal(documentOutcome, "guest_refused", receipt.ResultCode);
                if (receipt.TerminalKind != 2) throw new InvalidDataException();
            }
            else throw new InvalidDataException();
            return VerifyTerminalAttachments(attachments, candidate, successful, protocolVersion, receipt);
        }
        catch { throw Verify(); }
    }

    private static void MatchUsage(TerminalUsage usage, ProgramReceiptOutcome receipt)
    {
        if (usage.Cpu != receipt.CpuFuel || usage.Memory != receipt.MemoryBytes || usage.Read != receipt.StorageReadBytes ||
            usage.Write != receipt.StorageWriteBytes || usage.Values != receipt.OutputValues || usage.OutputBytes != receipt.OutputBytes ||
            usage.Fee != UInt128Big(receipt.FeeUnits)) throw new InvalidDataException();
    }

    private static void RequireRefusal(IReadOnlyDictionary<string, JsonValue> outcome, string expected, int code)
    {
        if (Text(outcome, "kind") != "refused") throw new InvalidDataException(); var failure = Map(Field(outcome, "failure"));
        if (Text(failure, "kind") != expected || expected == "guest_refused" && Integer32(failure, "code") != code)
            throw new InvalidDataException();
    }

    private static TerminalAttachments UnwrapTerminal(byte[] encoded)
    {
        var current = encoded; byte[]? authorization = null; byte[]? transferRoot = null; byte[]? occupancy = null;
        var authorityDomain = Encoding.UTF8.GetBytes("LXP/program-execution-with-transfer-authority/v2\0");
        var occupancyDomain = Encoding.UTF8.GetBytes("LXP/program-execution-with-occupancy/v1\0");
        if (Starts(current, authorityDomain))
        {
            var cursor = new TerminalCursor(current, authorityDomain.Length); current = cursor.Sized32();
            authorization = cursor.Sized32(); transferRoot = cursor.Take(32); cursor.Finish();
        }
        if (Starts(current, occupancyDomain))
        {
            var cursor = new TerminalCursor(current, occupancyDomain.Length); current = cursor.Sized32();
            occupancy = cursor.Sized32(); cursor.Finish();
        }
        if (Starts(current, authorityDomain) || Starts(current, occupancyDomain)) throw new InvalidDataException();
        return new(current, occupancy, authorization, transferRoot);
    }

    private static string VerifyTerminalAttachments(TerminalAttachments attachments, bool candidate, bool successful,
        ushort protocolVersion, ProgramReceiptOutcome receipt)
    {
        if (protocolVersion is not (1 or 2 or 3)) throw new InvalidDataException();
        var occupancyRequired = (protocolVersion == 2 || protocolVersion == 3) && successful;
        if (occupancyRequired != (attachments.Occupancy is not null)) throw new InvalidDataException();
        if (attachments.Occupancy is { } occupancy)
        {
            if (occupancy.Length == 0)
            {
                if (receipt.OccupancyEvidenceDigest.Any(value => value != 0) || receipt.OccupancyTransferRoot.Any(value => value != 0) ||
                    UInt128Big(receipt.OccupancyByteBatches) != 0 || UInt128Big(receipt.OccupancyFeeUnits) != 0) throw new InvalidDataException();
            }
            else
            {
                if (!Fixed(SHA256.HashData(occupancy), receipt.OccupancyEvidenceDigest)) throw new InvalidDataException();
                var settlement = DecodeOccupancySettlement(occupancy);
                if (settlement.ByteBatches != UInt128Big(receipt.OccupancyByteBatches) ||
                    settlement.FeeUnits != UInt128Big(receipt.OccupancyFeeUnits) ||
                    !Fixed(OccupancyTransferRoot(settlement, receipt.OccupancyAssetId), receipt.OccupancyTransferRoot))
                    throw new InvalidDataException();
            }
        }
        else if (receipt.OccupancyEvidenceDigest.Any(value => value != 0) || receipt.OccupancyTransferRoot.Any(value => value != 0) ||
            UInt128Big(receipt.OccupancyByteBatches) != 0 || UInt128Big(receipt.OccupancyFeeUnits) != 0)
            throw new InvalidDataException();
        var transferPresent = receipt.TransferRoot.Any(value => value != 0);
        var recorded = receipt.EncodingVersion != 4 && attachments.Authorization is null && transferPresent;
        var authorityRequired = candidate || receipt.EncodingVersion == 4 && successful;
        if (!recorded && (authorityRequired ? (attachments.Authorization is not null) != transferPresent : attachments.Authorization is not null))
            throw new InvalidDataException();
        if (attachments.Authorization is { } authorization)
        {
            if (authorization.Length == 0 || attachments.TransferRoot is null || !Fixed(attachments.TransferRoot, receipt.TransferRoot))
                throw new InvalidDataException();
            if (receipt.EncodingVersion == 4 && !AuthorizationV2(authorization)) throw new InvalidDataException();
            VerifyAuthorizationRoot(authorization, receipt.TransferRoot);
        }
        return recorded ? "recorded_terminal_root_not_locally_reconstructable" : "reconstructed";
    }

    private static void VerifyAuthorizationRoot(byte[] encoded, byte[] expected)
    {
        if (!Fixed(DecodeAuthorizationRoot(encoded), expected)) throw new InvalidDataException();
    }

    private static bool AuthorizationV2(byte[] encoded)
    {
        var domain = Encoding.UTF8.GetBytes("LayerX/programs/402LXP/account-bound-set/v1\0");
        if (Starts(encoded, domain)) encoded = new TerminalCursor(encoded, domain.Length).Sized32();
        return Starts(encoded, "LayerX/programs/402LXP/transfer-set/v2\0");
    }

    private static byte[] PrincipalPaymentAccount(byte[] principal, byte[] asset, byte[] name)
    {
        if (name.Length > 512 || name.Any(b => !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == '-' || b == ':')))
            throw new InvalidDataException();
        var text = Encoding.ASCII.GetString(name);
        if (!text.StartsWith("agent:", StringComparison.Ordinal) || text.Contains("::", StringComparison.Ordinal)) throw new InvalidDataException();
        var suffix = text.EndsWith(":main", StringComparison.Ordinal) ? ":main" : ":asset:" + Convert.ToHexString(asset).ToLowerInvariant();
        if (!text.EndsWith(suffix, StringComparison.Ordinal) || text.Length <= 6 + suffix.Length) throw new InvalidDataException();
        var did = Encoding.ASCII.GetBytes(text[6..^suffix.Length]);
        if (did[0] == ':' || did[^1] == ':' || !Fixed(Digest(Encoding.UTF8.GetBytes("LXP/v1/did-id\0"), BigEndian((ulong)did.Length, 2), did), principal))
            throw new InvalidDataException();
        return Digest(Encoding.UTF8.GetBytes("LX:ACCOUNT:v1"), BigEndian((ulong)name.Length, 4), name);
    }

    private static byte[] DecodeAuthorizationRoot(byte[] encoded)
    {
        var boundDomain = Encoding.UTF8.GetBytes("LayerX/programs/402LXP/account-bound-set/v1\0");
        TerminalCursor? names = null;
        if (Starts(encoded, boundDomain))
        {
            names = new TerminalCursor(encoded, boundDomain.Length);
            encoded = names.Sized32();
            if (Starts(encoded, boundDomain)) throw new InvalidDataException();
        }
        var v1 = Encoding.UTF8.GetBytes("LayerX/programs/402LXP/transfer-set/v1\0");
        var v2 = Encoding.UTF8.GetBytes("LayerX/programs/402LXP/transfer-set/v2\0");
        var candidate = Starts(encoded, v2); var domain = candidate ? v2 : v1; var cursor = new TerminalCursor(encoded, 0);
        if (!Starts(encoded, domain) || !Fixed(cursor.Take(domain.Length), domain)) throw new InvalidDataException();
        RequireNonzero(cursor.Take(32)); var principal = cursor.Take(32); RequireNonzero(principal);
        RequireNonzero(cursor.Take(32)); _ = DecodeFrame(cursor);
        DecodeEventEnvelope(cursor.Take(checked((int)cursor.U32())));
        var callCount = cursor.U64(); if (callCount > 64) throw new InvalidDataException();
        for (var index = 0; index < (int)callCount; index++)
        {
            RequireNonzero(cursor.Take(32)); RequireNonzero(cursor.Take(32)); RequireNonzero(cursor.Take(32));
            _ = DecodeFrame(cursor); _ = DecodeFrame(cursor);
            DecodeCapabilitySet(cursor.Take(checked((int)cursor.U32())), candidate);
        }
        var legCount = cursor.U64(); if (legCount == 0 || legCount > 256) throw new InvalidDataException();
        var kernelLegs = new List<byte[]>(); var total = BigInteger.Zero;
        for (var index = 0; index < (int)legCount; index++)
        {
            var frame = DecodeFrame(cursor); var source = principal; ProgramAuthorityBinding? authority = null;
            ProgramFundingBinding? funding = null;
            if (candidate)
            {
                switch (cursor.U8())
                {
                    case 1:
                        source = cursor.Take(32); RequireNonzero(source);
                        if (!Fixed(source, principal)) throw new InvalidDataException(); break;
                    case 2:
                        authority = DecodeProgramAuthority(cursor.Sized32()); source = authority.Source; break;
                    case 3:
                        source = cursor.Take(32); RequireNonzero(source);
                        if (!Fixed(source, principal)) throw new InvalidDataException();
                        funding = DecodeProgramFunding(cursor.Sized32()); break;
                    default: throw new InvalidDataException();
                }
            }
            var asset = cursor.Take(32); var destination = cursor.Take(32); var amount = cursor.U128(); var program = cursor.Take(32);
            RequireNonzero(asset); RequireNonzero(destination); RequireNonzero(program); if (amount == 0) throw new InvalidDataException();
            if (authority is not null && (!Fixed(authority.Owner, program) || !Fixed(authority.Frame, frame) ||
                !Fixed(authority.Asset, asset) || !Fixed(authority.Destination, destination) || authority.Amount != amount))
                throw new InvalidDataException();
            if (funding is not null && (!Fixed(funding.Owner, program) || !Fixed(funding.Destination, destination) ||
                !Fixed(funding.Asset, asset))) throw new InvalidDataException();
            if (names is not null)
            {
                var name = names.Take(names.U16());
                if (authority is not null) { if (name.Length != 0) throw new InvalidDataException(); }
                else source = PrincipalPaymentAccount(source, asset, name);
            }
            total = CheckedU128Add(total, amount);
            kernelLegs.Add(Concatenate([0], source, destination, asset, UInt128Bytes(amount), BigEndian(1, 2)));
        }
        cursor.Finish(); names?.Finish(); _ = total;
        return MerkleRoot(kernelLegs);
    }

    private static ProgramAuthorityBinding DecodeProgramAuthority(byte[] encoded)
    {
        var domain = Encoding.UTF8.GetBytes("LayerX/programs/402LXP/program-authority/v1\0");
        var cursor = new TerminalCursor(encoded, 0); if (!Fixed(cursor.Take(domain.Length), domain)) throw new InvalidDataException();
        var owner = cursor.Take(32); RequireNonzero(owner); var seedLength = cursor.U16();
        if (seedLength > 128) throw new InvalidDataException(); var seed = cursor.Take(seedLength); var source = cursor.Take(32);
        var frame = DecodeFrame(cursor); var asset = cursor.Take(32); var destination = cursor.Take(32); var amount = cursor.U128();
        cursor.Finish(); RequireNonzero(asset); RequireNonzero(destination);
        if (amount == 0 || !Fixed(DeriveProgramAccount(owner, seed), source)) throw new InvalidDataException();
        return new(owner, frame, source, asset, destination, amount);
    }

    private static ProgramFundingBinding DecodeProgramFunding(byte[] encoded)
    {
        var domain = Encoding.UTF8.GetBytes("LayerX/programs/402LXP/program-funding/v1\0");
        var cursor = new TerminalCursor(encoded, 0); if (!Fixed(cursor.Take(domain.Length), domain)) throw new InvalidDataException();
        var owner = cursor.Take(32); RequireNonzero(owner); var seedLength = cursor.U16();
        if (seedLength > 128) throw new InvalidDataException(); var seed = cursor.Take(seedLength);
        var destination = cursor.Take(32); var asset = cursor.Take(32); cursor.Finish();
        RequireNonzero(destination); RequireNonzero(asset);
        if (!Fixed(DeriveProgramAccount(owner, seed), destination)) throw new InvalidDataException();
        return new(owner, destination, asset);
    }

    private static byte[] DeriveProgramAccount(byte[] owner, byte[] seed) => Digest(
        Encoding.UTF8.GetBytes("LayerX/programs/program-account/v1\0"), owner, BigEndian((ulong)seed.Length, 4), seed);

    private static void DecodeEventEnvelope(byte[] encoded)
    {
        var domain = Encoding.UTF8.GetBytes("LayerX/programs/events/v1\0"); var cursor = new TerminalCursor(encoded, 0);
        if (!Fixed(cursor.Take(domain.Length), domain)) throw new InvalidDataException();
        var count = cursor.U32(); if (count > 64) throw new InvalidDataException();
        for (var index = 0; index < count; index++)
        {
            RequireNonzero(cursor.Take(32)); RequireNonzero(cursor.Take(32)); _ = DecodeFrame(cursor);
            if (cursor.Sized32().Length > 64 || cursor.Sized32().Length > 65_536) throw new InvalidDataException();
        }
        cursor.Finish();
    }

    private static byte[] DecodeFrame(TerminalCursor cursor)
    {
        var path = cursor.Take(8); var depth = cursor.U8();
        if (depth > 8 || path.Take(depth).Any(value => value == 0) || path.Skip(depth).Any(value => value != 0))
            throw new InvalidDataException();
        return Concatenate(path, [depth]);
    }

    private static void DecodeCapabilitySet(byte[] encoded, bool v2)
    {
        foreach (var grant in NativeCapabilitySet.Decode(encoded))
        {
            if (!v2 && (grant is NativeCapabilitySet.ProgramSpend or NativeCapabilitySet.BalanceView))
                throw new InvalidDataException("ABI v2 capability required");
        }
    }

    private static OccupancySettlementBinding DecodeOccupancySettlement(byte[] encoded)
    {
        if (encoded.Length > 65_536) throw new InvalidDataException();
        var v1 = Encoding.UTF8.GetBytes("LXP/storage-occupancy-settlement/v1\0");
        var v2 = Encoding.UTF8.GetBytes("LXP/storage-occupancy-settlement/v2\0");
        var v3 = Encoding.UTF8.GetBytes("LXP/storage-occupancy-settlement/v3\0");
        if (Starts(encoded, v1) || Starts(encoded, v2)) return DecodeLegacyOccupancy(encoded, v1, v2);
        var cursor = new TerminalCursor(encoded, 0); if (!Fixed(cursor.Take(v3.Length), v3)) throw new InvalidDataException();
        var batch = cursor.U64(); var occupancyPrice = DecodeOccupancySchedule(cursor, true);
        var declaredUnits = cursor.U128(); var declaredFee = cursor.U128(); var declaredPaid = cursor.U128();
        var declaredArrears = cursor.U128(); var count = cursor.U32(); if (count > 256) throw new InvalidDataException();
        var byteBatches = BigInteger.Zero; var feeUnits = BigInteger.Zero; var paidUnits = BigInteger.Zero;
        var arrearsUnits = BigInteger.Zero; byte[]? priorNamespace = null; var charges = new List<OccupancyChargeBinding>();
        for (var index = 0; index < count; index++)
        {
            var storageNamespace = DecodeStorageNamespace(cursor);
            if (priorNamespace is not null && CompareBytes(priorNamespace, storageNamespace.Canonical) >= 0) throw new InvalidDataException();
            priorNamespace = storageNamespace.Canonical; var payer = cursor.Take(32); RequireNonzero(payer);
            if (storageNamespace.Principal is not null && !Fixed(storageNamespace.Principal, payer)) throw new InvalidDataException();
            var rootProgram = cursor.Take(32); RequireNonzero(rootProgram); var activity = cursor.Take(32);
            var fromBatch = cursor.U64(); var toBatch = cursor.U64(); var recordedBytes = cursor.U64(); var finalBytes = cursor.U64();
            var units = cursor.U128(); var price = cursor.U64(); var accrued = cursor.U128(); var priorArrears = cursor.U128();
            var amountDue = cursor.U128(); var authorizedAdded = cursor.U128(); var disposition = cursor.U8();
            if (disposition is < 1 or > 5) throw new InvalidDataException(); var arrearsAfter = cursor.U128();
            var maximumBytes = cursor.U64(); var maximumPrice = cursor.U64(); _ = cursor.U128(); var mandate = cursor.Take(32);
            if (toBatch < fromBatch) throw new InvalidDataException();
            var expectedUnits = CheckedU128Multiply(recordedBytes, toBatch - fromBatch);
            var expectedFee = CheckedU128Multiply(expectedUnits, price); var expectedDue = CheckedU128Add(priorArrears, expectedFee);
            var migration = disposition == 5;
            if (toBatch != batch || !migration && price != occupancyPrice || units != expectedUnits || accrued != expectedFee ||
                amountDue != expectedDue || finalBytes > maximumBytes || !migration && (mandate.All(value => value == 0) || activity.All(value => value == 0)) ||
                migration && (price != 0 || accrued != 0 || priorArrears != 0 || amountDue != 0 || arrearsAfter != 0 ||
                    mandate.Any(value => value != 0) || activity.Any(value => value != 0) || !Fixed(rootProgram, storageNamespace.Program)) ||
                (disposition == 4) != (price > maximumPrice) || disposition == 1 && arrearsAfter != 0 ||
                disposition != 1 && arrearsAfter != amountDue) throw new InvalidDataException();
            if (authorizedAdded != 0)
            {
                var expectedMandate = Digest(Encoding.UTF8.GetBytes("LXP/storage-occupancy-mandate/v1\0"), payer,
                    rootProgram, activity, storageNamespace.Wire, BigEndian(maximumBytes, 8), BigEndian(maximumPrice, 8),
                    UInt128Bytes(authorizedAdded));
                if (!Fixed(mandate, expectedMandate)) throw new InvalidDataException();
            }
            byteBatches = CheckedU128Add(byteBatches, units); feeUnits = CheckedU128Add(feeUnits, accrued);
            if (disposition == 1) paidUnits = CheckedU128Add(paidUnits, amountDue);
            else arrearsUnits = CheckedU128Add(arrearsUnits, arrearsAfter);
            charges.Add(new(payer, amountDue, disposition == 1, arrearsAfter));
        }
        cursor.Finish();
        if (byteBatches != declaredUnits || feeUnits != declaredFee || paidUnits != declaredPaid || arrearsUnits != declaredArrears)
            throw new InvalidDataException();
        return new(byteBatches, feeUnits, charges.AsReadOnly());
    }

    private static OccupancySettlementBinding DecodeLegacyOccupancy(byte[] encoded, byte[] v1, byte[] v2)
    {
        var versioned = Starts(encoded, v2); var domain = versioned ? v2 : v1; var cursor = new TerminalCursor(encoded, 0);
        if (!Fixed(cursor.Take(domain.Length), domain)) throw new InvalidDataException();
        var batch = cursor.U64(); var occupancyPrice = DecodeOccupancySchedule(cursor, versioned);
        var declaredUnits = cursor.U128(); var declaredFee = cursor.U128(); var count = cursor.U64();
        if (count > 256) throw new InvalidDataException(); var byteBatches = BigInteger.Zero; var feeUnits = BigInteger.Zero;
        var charges = new List<OccupancyChargeBinding>();
        for (var index = 0; index < (int)count; index++)
        {
            _ = DecodeStorageNamespace(cursor); var payer = cursor.Take(32); RequireNonzero(payer);
            var fromBatch = cursor.U64(); var toBatch = cursor.U64(); var recordedBytes = cursor.U64(); _ = cursor.U64();
            var units = cursor.U128(); var price = cursor.U64(); var accrued = cursor.U128();
            if (toBatch < fromBatch) throw new InvalidDataException(); var expectedUnits = CheckedU128Multiply(recordedBytes, toBatch - fromBatch);
            if (toBatch != batch || units != expectedUnits || price != occupancyPrice ||
                accrued != CheckedU128Multiply(units, price)) throw new InvalidDataException();
            byteBatches = CheckedU128Add(byteBatches, units); feeUnits = CheckedU128Add(feeUnits, accrued);
            charges.Add(new(payer, accrued, true, BigInteger.Zero));
        }
        cursor.Finish(); if (byteBatches != declaredUnits || feeUnits != declaredFee) throw new InvalidDataException();
        return new(byteBatches, feeUnits, charges.AsReadOnly());
    }

    private static ulong DecodeOccupancySchedule(TerminalCursor cursor, bool versioned)
    {
        var version = versioned ? cursor.U32() : 1; if (version == 0) throw new InvalidDataException();
        ulong occupancyPrice = 0; for (var index = 0; index < 7; index++) occupancyPrice = cursor.U64(); return occupancyPrice;
    }

    private static StorageNamespaceBinding DecodeStorageNamespace(TerminalCursor cursor)
    {
        var length = cursor.U8(); if (length is not (33 or 65)) throw new InvalidDataException();
        var canonical = cursor.Take(length); var program = canonical.AsSpan(0, 32).ToArray(); RequireNonzero(program);
        var tag = canonical[32]; byte[]? principal = null;
        if (tag == 0 && length == 65) { principal = canonical.AsSpan(33).ToArray(); RequireNonzero(principal); }
        else if (!(tag == 1 && length == 33) && !(tag == 2 && length == 65)) throw new InvalidDataException();
        return new(canonical, Concatenate([length], canonical), program, principal);
    }

    private static byte[] OccupancyTransferRoot(OccupancySettlementBinding settlement, byte[] asset)
    {
        if (asset.Length != 32) throw new InvalidDataException(); RequireNonzero(asset);
        var payers = new Dictionary<string, OccupancyPayer>(StringComparer.Ordinal);
        foreach (var charge in settlement.Charges)
        {
            var key = Convert.ToHexString(charge.Payer); if (!payers.TryGetValue(key, out var entry))
            {
                entry = new OccupancyPayer { Payer = charge.Payer }; payers.Add(key, entry);
            }
            entry.Due = CheckedU128Add(entry.Due, charge.AmountDue);
            if (charge.Paid) entry.Paid = CheckedU128Add(entry.Paid, charge.AmountDue);
            entry.Arrears = CheckedU128Add(entry.Arrears, charge.ArrearsAfter);
        }
        var treasury = Digest(Encoding.UTF8.GetBytes("LX:ACCOUNT:v1"), BigEndian(11, 4), Encoding.UTF8.GetBytes("system:fees"));
        var legs = new List<byte[]>();
        foreach (var entry in payers.Values.Where(value => value.Due != 0 || value.Arrears != 0)
            .OrderBy(value => value.Payer, Comparer<byte[]>.Create(CompareBytes)))
        {
            if (entry.Paid != 0) legs.Add(Concatenate([0], entry.Payer, treasury, asset, UInt128Bytes(entry.Paid), BigEndian(23, 2)));
        }
        return MerkleRoot(legs);
    }

    private static byte[] UnwrapAppliedTerminal(byte[] encoded, ProgramReceiptOutcome receipt)
    {
        if (receipt.EncodingVersion != 4) return encoded;
        var domain = Encoding.UTF8.GetBytes("LXP/programs/terminal-applied-legs/v1\0");
        var cursor = new TerminalCursor(encoded, 0);
        if (!Fixed(cursor.Take(domain.Length), domain)) throw new InvalidDataException();
        var inner = cursor.Sized32(); var legs = cursor.Sized32(); cursor.Finish();
        if (inner.Length == 0 || inner.Length > MaximumProgramBytes || !Fixed(SHA256.HashData(legs), receipt.AppliedLegsDigest))
            throw new InvalidDataException();
        VerifyAppliedLegs(legs, receipt.TransferRoot);
        return inner;
    }

    private static void VerifyAppliedLegs(byte[] encoded, byte[] expected)
    {
        if (encoded.Length > 256 * 115 || encoded.Length % 115 != 0) throw new InvalidDataException();
        var legs = new List<byte[]>();
        for (var offset = 0; offset < encoded.Length; offset += 115)
        {
            var leg = encoded.AsSpan(offset, 115).ToArray();
            if (leg[0] != 0 || leg[113] != 0 || leg[114] != 1 || leg[1..33].All(value => value == 0) ||
                leg[33..65].All(value => value == 0) || leg[65..97].All(value => value == 0) ||
                leg[97..113].All(value => value == 0)) throw new InvalidDataException();
            legs.Add(leg);
        }
        if (!Fixed(MerkleRoot(legs), expected)) throw new InvalidDataException("applied transfer root");
    }

    private static byte[] MerkleRoot(IReadOnlyList<byte[]> legs)
    {
        if (legs.Count == 0) return new byte[32];
        var level = legs.Select(leg => Digest(Encoding.UTF8.GetBytes("LXP/v1/merkle-leaf\0"), leg)).ToList();
        while (level.Count > 1)
        {
            var next = new List<byte[]>();
            for (var index = 0; index < level.Count; index += 2)
                next.Add(Digest(Encoding.UTF8.GetBytes("LXP/v1/merkle-internal\0"), level[index],
                    index + 1 < level.Count ? level[index + 1] : level[index]));
            level = next;
        }
        return level[0];
    }

    private static BigInteger CheckedU128Add(BigInteger left, BigInteger right)
    {
        var value = left + right; return value <= ((BigInteger.One << 128) - 1) ? value : throw new InvalidDataException();
    }

    private static BigInteger CheckedU128Multiply(BigInteger left, BigInteger right)
    {
        var value = left * right; return value <= ((BigInteger.One << 128) - 1) ? value : throw new InvalidDataException();
    }

    private static byte[] BigEndian(ulong value, int length)
    {
        if (length is < 1 or > 8 || length < 8 && value >= (BigInteger.One << (length * 8))) throw new InvalidDataException();
        var encoded = new byte[8]; BinaryPrimitives.WriteUInt64BigEndian(encoded, value); return encoded.AsSpan(8 - length).ToArray();
    }

    private static byte[] Concatenate(params byte[][] values)
    {
        var length = values.Sum(value => value.Length); var result = new byte[length]; var offset = 0;
        foreach (var value in values) { value.CopyTo(result, offset); offset += value.Length; }
        return result;
    }

    private static int CompareBytes(byte[] left, byte[] right)
    {
        for (var index = 0; index < Math.Min(left.Length, right.Length); index++)
            if (left[index] != right[index]) return left[index].CompareTo(right[index]);
        return left.Length.CompareTo(right.Length);
    }

    private static void ValidateAuthenticatedProgramFailure(byte[] encoded)
    {
        var cursor = new TerminalCursor(encoded, 0); var program = cursor.Take(32); var failureClass = cursor.U32();
        var reason = cursor.Sized32(); cursor.Finish();
        if (program.All(value => value == 0) || failureClass is not (1 or 2 or 3 or 4 or 5 or 254 or 255) ||
            reason.Length > 4_096 || failureClass is 254 or 255 && reason.Length != 0) throw new InvalidDataException();
    }

    private static void ValidateFailureDetail(int family, byte[] encoded)
    {
        if (family == 1) { ValidateAuthenticatedProgramFailure(encoded); return; }
        var cursor = new TerminalCursor(encoded, 0); var tag = cursor.U8();
        if (family == 2) ValidateCompositionFailure(cursor, tag);
        else if (family == 3) ValidateEntrypointFailure(cursor, tag);
        else if (family == 4) ValidateAbiFailure(cursor, tag);
        else throw new InvalidDataException();
        cursor.Finish();
    }

    private static void ValidateCompositionFailure(TerminalCursor cursor, int tag)
    {
        switch (tag)
        {
            case 1: case 9: case 10: case 11: case 20: case 21: case 22: break;
            case 2:
                if (cursor.U8() is < 1 or > 2 || cursor.U8() is < 1 or > 2) throw new InvalidDataException(); break;
            case 3: case 4: RequireNonzero(cursor.Take(32)); break;
            case 5: case 6: case 7: _ = cursor.U32(); _ = cursor.U32(); break;
            case 8: RequireNonzero(cursor.Take(32)); _ = cursor.U32(); _ = cursor.U32(); break;
            case 12: _ = cursor.I32(); break;
            case 13: _ = cursor.U64(); _ = cursor.U64(); break;
            case 14: RequireNonzero(cursor.Take(32)); _ = cursor.I32(); break;
            case 15: ValidateAuthenticatedProgramFailure(cursor.Rest()); break;
            case 16: ValidateAbiFailure(cursor, cursor.U8()); break;
            case 17: ValidateFault(cursor); break;
            case 18: ValidateMeterFailure(cursor); break;
            case 19: ValidateResponseFailure(cursor); break;
            case 23: _ = cursor.Take(76); _ = cursor.Take(76); break;
            default: throw new InvalidDataException();
        }
    }

    private static void ValidateEntrypointFailure(TerminalCursor cursor, int tag)
    {
        switch (tag)
        {
            case 1: _ = cursor.U64(); _ = cursor.U64(); break;
            case 2: case 3: case 4: break;
            case 5: case 6: _ = cursor.I32(); break;
            case 7: ValidateFault(cursor); break;
            case 8: ValidateMeterFailure(cursor); break;
            default: throw new InvalidDataException();
        }
    }

    private static void ValidateAbiFailure(TerminalCursor cursor, int tag)
    {
        if (tag is >= 1 and <= 10 or >= 13 and <= 15) return;
        if (tag == 11) { if (cursor.U8() is < 1 or > 11) throw new InvalidDataException(); }
        else if (tag == 12) ValidateMeterFailure(cursor); else throw new InvalidDataException();
    }

    private static void ValidateMeterFailure(TerminalCursor cursor)
    {
        var tag = cursor.U8();
        if (tag == 1)
        {
            var resource = cursor.U8(); var limit = cursor.U64(); var attempted = cursor.U64();
            if (resource is < 1 or > 7 || attempted <= limit) throw new InvalidDataException();
        }
        else if (tag == 2) { if (cursor.U8() is < 1 or > 7) throw new InvalidDataException(); }
        else if (tag != 3) throw new InvalidDataException();
    }

    private static void ValidateFault(TerminalCursor cursor)
    {
        var tag = cursor.U8();
        if (tag is 1 or 2 or 16)
        {
            var name = cursor.Sized32(); if (!Encoding.UTF8.GetBytes(Encoding.UTF8.GetString(name)).SequenceEqual(name))
                throw new InvalidDataException();
        }
        else if (tag is >= 3 and <= 13 or 15) return;
        else if (tag == 14) ValidateMeterFailure(cursor); else throw new InvalidDataException();
    }

    private static void ValidateResponseFailure(TerminalCursor cursor)
    {
        switch (cursor.U8())
        {
            case 1: case 2: _ = cursor.U64(); _ = cursor.U64(); break;
            case 3: case 4: break;
            case 5: _ = cursor.I32(); _ = cursor.I32(); break;
            case 6: ValidateMeterFailure(cursor); break;
            default: throw new InvalidDataException();
        }
    }

    private static void ValidateCandidateResource(TerminalCursor cursor, TerminalUsage usage)
    {
        var tag = cursor.U8(); var resource = cursor.U8(); if (resource > 6) throw new InvalidDataException();
        if (tag == 0)
        {
            var limit = cursor.U64(); var attempted = cursor.U64();
            if (attempted <= limit || CandidateUsage(usage, resource) > limit) throw new InvalidDataException();
        }
        else if (tag != 1) throw new InvalidDataException();
    }

    private static ulong CandidateUsage(TerminalUsage usage, int resource) => resource switch
    {
        0 => usage.Cpu,
        1 => usage.Memory,
        2 => usage.Read,
        3 => usage.Write,
        4 => usage.Values,
        5 => usage.OutputBytes,
        6 => 0,
        _ => throw new InvalidDataException(),
    };

    private static void ValidateLegacyResource(TerminalCursor cursor)
    {
        var tag = cursor.U8(); if (cursor.U8() is < 1 or > 7) throw new InvalidDataException();
        if (tag == 1) { var limit = cursor.U64(); if (cursor.U64() <= limit) throw new InvalidDataException(); }
        else if (tag != 2) throw new InvalidDataException();
    }

    private static void RequireNonzero(byte[] value)
    {
        if (value.All(item => item == 0)) throw new InvalidDataException();
    }

    private static ActivityBinding DecodeSignedCall(ProgramCall call)
    {
        try
        {
            var signed = call.SignedActivity; var cursor = new BinaryCursor(signed);
            var envelopeVersion = cursor.U16();
            if (envelopeVersion is not (1 or 2 or 3) || cursor.U16() != 0x1001 || cursor.U8() != 12) throw new InvalidDataException();
            cursor.Tag(1); var protocol = cursor.U16(); if (protocol != envelopeVersion) throw new InvalidDataException();
            cursor.Tag(2); _ = cursor.U32(); cursor.Tag(3);
            if (cursor.U32() != ((uint)ReceiptModuleId << 16 | CallOperation)) throw new InvalidDataException();
            cursor.Tag(4); _ = cursor.Bounded(255, true); cursor.Tag(5); _ = cursor.Bounded(524_288, true);
            cursor.Tag(6); _ = cursor.U64(); cursor.Tag(7); var notBefore = cursor.U64(); var notAfter = cursor.U64();
            if (notAfter < notBefore) throw new InvalidDataException(); cursor.Tag(8); var idempotency = cursor.Bounded(32, false);
            if (idempotency.Length != 32) throw new InvalidDataException(); cursor.Tag(9); var envelopeFee = new BigInteger(cursor.Take(16), true, true);
            if (call.NativeCall is not null && (protocol != 3 || envelopeFee != call.Budget.FeeLimit.Value)) throw new InvalidDataException();
            cursor.Tag(10); var payloadHash = cursor.Bounded(32, false); if (payloadHash.Length != 32) throw new InvalidDataException();
            cursor.Tag(11); var payload = cursor.Bounded(524_288, true); cursor.Tag(12); _ = cursor.Bounded(128, true); cursor.Finish();
            var expected = CanonicalCallPayload(call);
            if (!Fixed(payload, expected) || !Fixed(payloadHash, Digest(Encoding.UTF8.GetBytes("LXP/v1/payload-hash\0"), payload)))
                throw new InvalidDataException();
            return new(Digest(Encoding.UTF8.GetBytes("LXP/v1/activity-id\0"), signed), idempotency, notBefore, notAfter);
        }
        catch { throw Invalid(); }
    }

    private static byte[] CanonicalCallPayload(ProgramCall call)
    {
        if (call.NativeCall is { } n)
        {
            if (!n.ProgramId.SequenceEqual(call.ProgramId) || !n.Calldata.SequenceEqual(call.Calldata) || n.Resources[0] != call.Budget.Fuel || call.Capabilities.Count != 0) throw Invalid();
            return n.Encode();
        }
        var domain = Encoding.UTF8.GetBytes("LayerX/programs/call/v1\0"); using var stream = new MemoryStream();
        stream.Write(domain); stream.Write(call.ProgramId); var word = new byte[8]; BinaryPrimitives.WriteUInt64BigEndian(word, call.Budget.Fuel);
        stream.Write(word); stream.Write(UInt128Bytes(call.Budget.FeeLimit.Value));
        var count = new byte[2]; BinaryPrimitives.WriteUInt16BigEndian(count, checked((ushort)call.Capabilities.Count)); stream.Write(count);
        foreach (var capability in call.Capabilities) stream.WriteByte(checked((byte)((int)capability + 1)));
        var length = new byte[4]; BinaryPrimitives.WriteUInt32BigEndian(length, checked((uint)call.Calldata.Length)); stream.Write(length);
        stream.Write(call.Calldata); return stream.ToArray();
    }

    private static ProgramSubmission UnknownSubmission(byte[] activity, string key, byte[] retained)
    {
        var value = JsonValue.Object(new Dictionary<string, JsonValue>
        {
            ["state"] = JsonValue.String("unknown"),
            ["activity_id"] = JsonValue.String(Convert.ToHexString(activity).ToLowerInvariant()),
            ["idempotency_key"] = JsonValue.String(key),
            ["retained_signed_activity"] =
                JsonValue.String(Convert.ToHexString(retained).ToLowerInvariant()),
        });
        return new(value, "unknown", activity, key, retained, null);
    }

    private ulong NowMilliseconds()
    {
        var milliseconds = _timeProvider.GetUtcNow().ToUnixTimeMilliseconds();
        return milliseconds >= 0 ? checked((ulong)milliseconds) : throw Verify();
    }

    private static IReadOnlyDictionary<string, JsonValue> Map(JsonValue value) => value is JsonValue.ObjectValue map ? map.Value : throw Verify();
    private static void RequireFields(IReadOnlyDictionary<string, JsonValue> map, IEnumerable<string> fields)
    {
        var exact = fields.ToArray(); if (map.Count != exact.Length || exact.Any(field => !map.ContainsKey(field))) throw Decode();
    }
    private static JsonValue Field(IReadOnlyDictionary<string, JsonValue> map, string name) => map.TryGetValue(name, out var value) ? value : throw Decode();
    private static string Text(IReadOnlyDictionary<string, JsonValue> map, string name) => Field(map, name) is JsonValue.StringValue text && !string.IsNullOrEmpty(text.Value) ? text.Value : throw Decode();
    private static long Integer(IReadOnlyDictionary<string, JsonValue> map, string name) => Field(map, name) is JsonValue.IntegerValue integer ? integer.Value : throw Decode();
    private static int Integer32(IReadOnlyDictionary<string, JsonValue> map, string name)
    {
        var value = Integer(map, name); return value is >= int.MinValue and <= int.MaxValue ? (int)value : throw Decode();
    }
    private static ushort UInt16Integer(IReadOnlyDictionary<string, JsonValue> map, string name)
    {
        var value = Integer(map, name); return value is >= 0 and <= ushort.MaxValue ? (ushort)value : throw Decode();
    }
    private static uint UInt32Integer(IReadOnlyDictionary<string, JsonValue> map, string name)
    {
        var value = Integer(map, name); return value is >= 0 and <= uint.MaxValue ? (uint)value : throw Decode();
    }
    private static long DecimalInt64(IReadOnlyDictionary<string, JsonValue> map, string name)
    {
        var value = Text(map, name);
        if (value == "-0" || value.Length > 1 && value[0] == '0' || value.StartsWith("-0", StringComparison.Ordinal) ||
            !long.TryParse(value, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out var parsed) ||
            parsed.ToString(CultureInfo.InvariantCulture) != value) throw Decode();
        return parsed;
    }
    private static string Hex(IReadOnlyDictionary<string, JsonValue> map, string name, int bytes, bool exact, bool empty = false)
    {
        var value = Field(map, name) is JsonValue.StringValue text ? text.Value : throw Decode();
        if ((!empty && value.Length == 0) || value.Length % 2 != 0 || (exact ? value.Length != bytes * 2 : value.Length > bytes * 2) ||
            value.Any(character => !(character is >= '0' and <= '9' or >= 'a' and <= 'f'))) throw Verify();
        return value;
    }
    private static byte[] Bytes(IReadOnlyDictionary<string, JsonValue> map, string name, int bytes, bool exact = false,
        bool empty = false) => Convert.FromHexString(Hex(map, name, bytes, exact, empty));
    private static ulong DecimalUInt64(IReadOnlyDictionary<string, JsonValue> map, string name)
    {
        var value = Text(map, name);
        if (string.IsNullOrEmpty(value) || value.Length > 1 && value[0] == '0' || !ulong.TryParse(value, NumberStyles.None, CultureInfo.InvariantCulture, out var parsed)) throw Decode();
        return parsed;
    }
    private static BigInteger DecimalUInt128(IReadOnlyDictionary<string, JsonValue> map, string name)
    {
        var value = Text(map, name);
        if (value.Length > 1 && value[0] == '0' || !BigInteger.TryParse(value, NumberStyles.None,
                CultureInfo.InvariantCulture, out var parsed) || parsed < 0 || parsed >= (BigInteger.One << 128)) throw Decode();
        return parsed;
    }
    private static BigInteger UInt128Big(UInt128Value value) => (new BigInteger(value.High) << 64) + value.Low;
    private static byte[] UInt128Bytes(BigInteger value)
    {
        var raw = value.ToByteArray(true, true); var encoded = new byte[16];
        if (raw.Length > encoded.Length) throw Invalid(); raw.CopyTo(encoded, encoded.Length - raw.Length); return encoded;
    }
    private static bool Fixed(byte[] left, byte[] right) => left.Length == right.Length &&
        CryptographicOperations.FixedTimeEquals(left, right);
    private static byte[] Digest(params byte[][] values)
    {
        using var hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
        foreach (var value in values) hash.AppendData(value); return hash.GetHashAndReset();
    }
    private static bool Starts(byte[] value, string prefix) => Starts(value, Encoding.UTF8.GetBytes(prefix));
    private static bool Starts(byte[] value, byte[] prefix) => value.Length >= prefix.Length &&
        Fixed(value.AsSpan(0, prefix.Length).ToArray(), prefix);

    private static JsonValue Encode(ProgramCall call)
    {
        ArgumentNullException.ThrowIfNull(call);
        if (call.NativeCall is { } n)
        {
            return JsonValue.Object(new Dictionary<string, JsonValue>
            {
                ["payload"] = JsonValue.String(Convert.ToHexString(n.Encode()).ToLowerInvariant()),
                ["signed_activity"] = JsonValue.String(Convert.ToHexString(call.SignedActivity).ToLowerInvariant())
            });
        }
        return JsonValue.Object(new Dictionary<string, JsonValue>
        {
            ["program_id"] = JsonValue.String(Convert.ToHexString(call.ProgramId).ToLowerInvariant()),
            ["calldata"] = JsonValue.String(Convert.ToHexString(call.Calldata).ToLowerInvariant()),
            ["budget"] = JsonValue.Object(new Dictionary<string, JsonValue> { ["fuel"] = JsonValue.String(call.Budget.Fuel.ToString(CultureInfo.InvariantCulture)), ["fee_limit"] = JsonValue.String(call.Budget.FeeLimit.ToString()) }),
            ["capabilities"] = JsonValue.Array(call.Capabilities.Select(value => JsonValue.String(Capability(value)))),
            ["signed_activity"] = JsonValue.String(Convert.ToHexString(call.SignedActivity).ToLowerInvariant())
        });
    }
    private static string Capability(ProgramCapability value) => value switch { ProgramCapability.StorageRead => "storage_read", ProgramCapability.StorageWrite => "storage_write", ProgramCapability.Transfer => "transfer", ProgramCapability.EmitEvent => "emit_event", ProgramCapability.Compose => "compose", _ => throw Invalid() };
    private static string Identifier(byte[] value) => value?.Length == 32 ? Convert.ToHexString(value).ToLowerInvariant() : throw Invalid();
    private static string Level(string value) => value is "sequencer-signed" or "receipted" ? value : throw Invalid();
    private static bool Hex32(string value) => value.Length == 64 && value.All(character => character is >= '0' and <= '9' or >= 'a' and <= 'f');
    private static PlatformSdkException Invalid() => new(SdkErrorCode.InvalidArgument, RetryClass.Never);
    private static PlatformSdkException Decode() => new(SdkErrorCode.DecodeFailure, RetryClass.Never);
    private static PlatformSdkException Verify() => new(SdkErrorCode.VerificationFailure, RetryClass.Never);

    private sealed class BinaryCursor
    {
        private readonly byte[] _bytes; private int _offset;
        internal BinaryCursor(byte[] bytes) => _bytes = bytes;
        internal byte U8() => Take(1)[0]; internal ushort U16() => BinaryPrimitives.ReadUInt16BigEndian(Take(2));
        internal uint U32() => BinaryPrimitives.ReadUInt32BigEndian(Take(4)); internal ulong U64() => BinaryPrimitives.ReadUInt64BigEndian(Take(8));
        internal void Tag(byte expected) { if (U8() != expected) throw new InvalidDataException(); }
        internal byte[] Bounded(int maximum, bool empty)
        {
            var length = U32(); if (length > maximum || !empty && length == 0) throw new InvalidDataException(); return Take((int)length);
        }
        internal byte[] Take(int length)
        {
            if (length < 0 || _offset > _bytes.Length - length) throw new InvalidDataException();
            var value = _bytes.AsSpan(_offset, length).ToArray(); _offset += length; return value;
        }
        internal void Finish() { if (_offset != _bytes.Length) throw new InvalidDataException(); }
    }

    private sealed class TerminalCursor
    {
        private readonly byte[] _bytes; private int _offset;
        internal TerminalCursor(byte[] bytes, int offset)
        {
            if (offset < 0 || offset > bytes.Length) throw new InvalidDataException(); _bytes = bytes; _offset = offset;
        }
        internal byte U8() => Take(1)[0]; internal ushort U16() => BinaryPrimitives.ReadUInt16BigEndian(Take(2));
        internal uint U32() => BinaryPrimitives.ReadUInt32BigEndian(Take(4)); internal ulong U64() => BinaryPrimitives.ReadUInt64BigEndian(Take(8));
        internal int I32() => BinaryPrimitives.ReadInt32BigEndian(Take(4)); internal long I64() => BinaryPrimitives.ReadInt64BigEndian(Take(8));
        internal BigInteger U128() => new(Take(16), true, true);
        internal int Remaining => _bytes.Length - _offset;
        internal byte[] Sized32() { var length = U32(); return length <= int.MaxValue ? Take((int)length) : throw new InvalidDataException(); }
        internal byte[] Sized64() { var length = U64(); return length <= int.MaxValue ? Take((int)length) : throw new InvalidDataException(); }
        internal byte[] Take(int length)
        {
            if (length < 0 || _offset > _bytes.Length - length) throw new InvalidDataException();
            var value = _bytes.AsSpan(_offset, length).ToArray(); _offset += length; return value;
        }
        internal byte[] Rest() => Take(_bytes.Length - _offset);
        internal void Finish() { if (_offset != _bytes.Length) throw new InvalidDataException(); }
    }
}

public sealed class UnsignedProgramTransaction
{
    private readonly byte[] _data;
    public int Type => 2;
    public ulong ChainId => 125;
    public string To => Programs.AddressValue;
    public string From { get; }
    public byte[] Data => _data.ToArray();
    public ulong Nonce { get; }
    public ulong Gas { get; }
    public BigInteger MaxFeePerGas { get; }
    public BigInteger MaxPriorityFeePerGas { get; }
    public BigInteger Value => BigInteger.Zero;

    public UnsignedProgramTransaction(string sender, byte[] data, ulong nonce, ulong gas,
        BigInteger maxFeePerGas, BigInteger maxPriorityFeePerGas)
    {
        Programs.Address(sender);
        if (data.Length < 4 || maxPriorityFeePerGas < 0 || maxFeePerGas < maxPriorityFeePerGas || maxFeePerGas >= (BigInteger.One << 256))
            throw new ArgumentException("Invalid EIP-1559 transaction");
        From = sender; _data = data.ToArray(); Nonce = nonce; Gas = gas;
        MaxFeePerGas = maxFeePerGas; MaxPriorityFeePerGas = maxPriorityFeePerGas;
    }

    public JsonValue ToJson(bool includeGas = true)
    {
        var fields = new Dictionary<string, JsonValue>
        {
            ["type"] = JsonValue.String("0x2"),
            ["chainId"] = JsonValue.String("0x7d"),
            ["from"] = JsonValue.String(From),
            ["to"] = JsonValue.String(To),
            ["nonce"] = JsonValue.String(Programs.Quantity(Nonce)),
            ["value"] = JsonValue.String("0x0"),
            ["maxFeePerGas"] = JsonValue.String(Programs.Quantity(MaxFeePerGas)),
            ["maxPriorityFeePerGas"] = JsonValue.String(Programs.Quantity(MaxPriorityFeePerGas)),
            ["data"] = JsonValue.String(Programs.Hex(_data)),
            ["accessList"] = JsonValue.Array([])
        };
        if (includeGas) fields["gas"] = JsonValue.String(Programs.Quantity(Gas));
        return JsonValue.Object(fields);
    }

    public byte[] SigningPayload => [2, .. ProgramRlp.List(
        ProgramRlp.Integer(ChainId), ProgramRlp.Integer(Nonce), ProgramRlp.Integer(MaxPriorityFeePerGas),
        ProgramRlp.Integer(MaxFeePerGas), ProgramRlp.Integer(Gas), ProgramRlp.Bytes(Programs.Unhex(To)),
        ProgramRlp.Integer(0), ProgramRlp.Bytes(_data), ProgramRlp.List())];
}

public sealed class Programs : IDisposable
{
    public const string AddressValue = "0x000000000000000000000000000000000000101d";
    public const string AuthAddress = "0x000000000000000000000000000000000000101c";
    private readonly Uri _endpoint;
    private readonly HttpClient _http;
    private readonly bool _ownsHttp;
    private long _id;

    public Programs(Uri endpoint, HttpClient? httpClient = null)
    {
        if (!endpoint.IsAbsoluteUri || endpoint.Scheme is not ("http" or "https") ||
            !string.IsNullOrEmpty(endpoint.UserInfo) || !string.IsNullOrEmpty(endpoint.Fragment)) throw new ArgumentException("RPC endpoint");
        _endpoint = endpoint; _ownsHttp = httpClient is null;
        _http = httpClient ?? new HttpClient(new HttpClientHandler { AllowAutoRedirect = false });
    }

    public async Task<JsonValue> SendAsync(string method, IReadOnlyList<JsonValue> parameters, CancellationToken cancellationToken = default)
    {
        var id = Interlocked.Increment(ref _id);
        var request = JsonValue.Object(new Dictionary<string, JsonValue>
        {
            ["jsonrpc"] = JsonValue.String("2.0"),
            ["id"] = JsonValue.Integer(id),
            ["method"] = JsonValue.String(method),
            ["params"] = JsonValue.Array(parameters)
        });
        try
        {
            using var message = new HttpRequestMessage(HttpMethod.Post, _endpoint)
            {
                Content = new StringContent(JsonSerializer.Serialize(request), Encoding.UTF8, "application/json")
            };
            using var response = await _http.SendAsync(message, HttpCompletionOption.ResponseHeadersRead, cancellationToken).ConfigureAwait(false);
            response.EnsureSuccessStatusCode();
            await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
            using var buffer = new MemoryStream(); var block = new byte[8192]; int read;
            while ((read = await stream.ReadAsync(block, cancellationToken).ConfigureAwait(false)) != 0)
            {
                if (buffer.Length + read > 4 * 1_048_576) throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never);
                buffer.Write(block, 0, read);
            }
            var decoded = JsonSerializer.Deserialize<JsonValue>(buffer.ToArray()) as JsonValue.ObjectValue;
            if (decoded is null || !decoded.Value.TryGetValue("jsonrpc", out var version) || version is not JsonValue.StringValue { Value: "2.0" } ||
                !decoded.Value.TryGetValue("id", out var responseId) || responseId is not JsonValue.IntegerValue number || number.Value != id ||
                decoded.Value.ContainsKey("error") == decoded.Value.ContainsKey("result"))
                throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never);
            if (decoded.Value.ContainsKey("error")) throw new PlatformSdkException(SdkErrorCode.CoreRejection, RetryClass.Never);
            return decoded.Value["result"];
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested) { throw; }
        catch (PlatformSdkException) { throw; }
        catch (JsonException) { throw new PlatformSdkException(SdkErrorCode.DecodeFailure, RetryClass.Never); }
        catch (Exception error) when (error is HttpRequestException or IOException or OperationCanceledException)
        {
            throw method == "eth_sendRawTransaction"
                ? new PlatformSdkException(SdkErrorCode.UnknownOutcome, RetryClass.UnknownOutcome)
                : new PlatformSdkException(SdkErrorCode.TransportFailure, RetryClass.Safe);
        }
    }

    public async Task<byte[]> CallAsync(string to, byte[] data, CancellationToken cancellationToken = default, string? sender = null)
    {
        Address(to);
        var fields = new Dictionary<string, JsonValue>
        {
            ["to"] = JsonValue.String(to),
            ["data"] = JsonValue.String(Hex(data))
        };
        if (sender is not null) { Address(sender); fields["from"] = JsonValue.String(sender); }
        var call = JsonValue.Object(fields);
        return Data(await SendAsync("eth_call", [call, JsonValue.String("pending")], cancellationToken).ConfigureAwait(false));
    }

    public static byte[] TransactionHash(byte[] signedTransaction)
    {
        RequireSignedTransaction(signedTransaction);
        var digest = new KeccakDigest(256); digest.BlockUpdate(signedTransaction, 0, signedTransaction.Length);
        var output = new byte[32]; digest.DoFinal(output, 0); return output;
    }
    internal static void Address(string value)
    {
        if (Unhex(value).Length != 20) throw new ArgumentException("EVM address");
    }
    internal static string Hex(byte[] value) => "0x" + Convert.ToHexString(value).ToLowerInvariant();
    internal static byte[] Unhex(string value)
    {
        if (!value.StartsWith("0x", StringComparison.Ordinal) || value.Length % 2 != 0) throw new ArgumentException("RPC DATA");
        return Convert.FromHexString(value[2..]);
    }
    internal static byte[] Data(JsonValue value) => value is JsonValue.StringValue text ? Unhex(text.Value) : throw new ArgumentException("RPC DATA");
    internal static string Quantity(BigInteger value)
    {
        if (value < 0) throw new ArgumentException("RPC QUANTITY");
        return "0x" + (value == 0 ? "0" : value.ToString("x", CultureInfo.InvariantCulture).TrimStart('0'));
    }
    internal static BigInteger Quantity(JsonValue value)
    {
        if (value is not JsonValue.StringValue text || text.Value.Length < 3 || !text.Value.StartsWith("0x", StringComparison.Ordinal) ||
            text.Value.Length > 3 && text.Value[2] == '0' || text.Value[2..].Any(c => !(c is >= '0' and <= '9' or >= 'a' and <= 'f')))
            throw new ArgumentException("RPC QUANTITY");
        return BigInteger.Parse("0" + text.Value[2..], NumberStyles.AllowHexSpecifier, CultureInfo.InvariantCulture);
    }
    internal static void RequireSignedTransaction(byte[] raw) => _ = ProgramRlp.Transaction(raw);
    internal static void BindTransaction(byte[] raw, byte[] expectedData)
    {
        var fields = ProgramRlp.Transaction(raw);
        if (!fields[5].SequenceEqual(Unhex(AddressValue)) || !fields[7].SequenceEqual(expectedData) || fields[6].Length != 0)
            throw new ArgumentException("Signed transaction does not match the program operation");
    }
    public void Dispose() { if (_ownsHttp) _http.Dispose(); }
}

internal static class ProgramRlp
{
    internal static byte[] Integer(BigInteger value) => Bytes(value == 0 ? [] : value.ToByteArray(true, true));
    internal static byte[] Bytes(byte[] value) => value.Length == 1 && value[0] < 0x80 ? value.ToArray() : Wrap(value, 0x80);
    internal static byte[] List(params byte[][] fields) => Wrap(fields.SelectMany(value => value).ToArray(), 0xc0);
    private static byte[] Wrap(byte[] value, int start)
    {
        if (value.Length < 56) return [(byte)(start + value.Length), .. value];
        var size = new BigInteger(value.Length).ToByteArray(true, true);
        return [(byte)(start + 55 + size.Length), .. size, .. value];
    }
    internal static byte[][] Transaction(byte[] raw)
    {
        if (raw.Length < 2 || raw.Length > 4 * 1_048_576 || raw[0] != 2) throw new ArgumentException("Signed EIP-1559 transaction required");
        var offset = 1; var root = Read(raw, ref offset, raw.Length, true);
        if (offset != raw.Length) throw new ArgumentException("RLP trailing bytes");
        var fields = new List<byte[]>(); var cursor = 0;
        while (cursor < root.Length)
        {
            var list = root[cursor] >= 0xc0;
            if (list != (fields.Count == 8)) throw new ArgumentException("EIP-1559 field type");
            fields.Add(Read(root, ref cursor, root.Length, list));
        }
        if (fields.Count != 12) throw new ArgumentException("EIP-1559 field count");
        BigInteger Number(int index, int size)
        {
            var bytes = fields[index];
            if (bytes.Length > size || bytes.Length > 0 && bytes[0] == 0) throw new ArgumentException("EIP-1559 integer");
            return new BigInteger(bytes, true, true);
        }
        if (Number(0, 32) != 125 || Number(1, 8) > ulong.MaxValue || Number(2, 32) > Number(3, 32) ||
            Number(4, 8) == 0 || fields[5].Length != 20 || Number(6, 32) != 0 || Number(9, 1) > 1 ||
            Number(10, 32) == 0 || Number(11, 32) == 0) throw new ArgumentException("EIP-1559 fields");
        ValidateAccessList(fields[8]);
        return fields.ToArray();
    }
    private static void ValidateAccessList(byte[] bytes)
    {
        var offset = 0;
        while (offset < bytes.Length)
        {
            var entry = Read(bytes, ref offset, bytes.Length, true); var cursor = 0;
            if (Read(entry, ref cursor, entry.Length, false).Length != 20) throw new ArgumentException("Access-list address");
            var keys = Read(entry, ref cursor, entry.Length, true);
            if (cursor != entry.Length) throw new ArgumentException("Access-list entry");
            var keyOffset = 0;
            while (keyOffset < keys.Length)
                if (Read(keys, ref keyOffset, keys.Length, false).Length != 32) throw new ArgumentException("Access-list key");
        }
    }
    private static byte[] Read(byte[] bytes, ref int offset, int end, bool list)
    {
        if (offset >= end) throw new ArgumentException("RLP truncated");
        var prefix = bytes[offset++];
        if (prefix < 0x80)
        {
            if (list) throw new ArgumentException("RLP list expected");
            return [prefix];
        }
        if ((prefix >= 0xc0) != list) throw new ArgumentException("RLP type");
        var start = list ? 0xc0 : 0x80; var length = prefix - start;
        if (length > 55)
        {
            var size = length - 55;
            if (size > 4 || offset > end - size || bytes[offset] == 0) throw new ArgumentException("RLP length");
            length = 0;
            for (var i = 0; i < size; i++) length = checked(length * 256 + bytes[offset++]);
            if (length < 56) throw new ArgumentException("RLP noncanonical length");
        }
        if (offset > end - length || !list && length == 1 && bytes[offset] < 0x80) throw new ArgumentException("RLP value");
        var result = bytes.AsSpan(offset, length).ToArray(); offset += length; return result;
    }
}
