package dotnet

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The continuation ruling for 22.33.1368 requires source qualification here:
// the matching C# behavior tests run in the owners' .NET environment. These
// checks inspect production members and their executable test/project wiring;
// they do not claim to execute C# or replace it with a Go implementation.
func repairM1368Read(t *testing.T, name string) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate SDK sources")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Keep offsets, but mask comments and string/character literals when locating
// a member's braces. In particular, JSON strings and interpolated URLs contain
// braces and comment-like text that are not C# member boundaries.
func repairM1368Mask(source string, literals bool) string {
	masked := []byte(source)
	blank := func(start, end int) {
		for i := start; i < end; i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	for i := 0; i < len(source); {
		start := i
		switch {
		case strings.HasPrefix(source[i:], "//"):
			for i < len(source) && source[i] != '\n' {
				i++
			}
			blank(start, i)
		case strings.HasPrefix(source[i:], "/*"):
			i += 2
			for i < len(source) && !strings.HasPrefix(source[i:], "*/") {
				i++
			}
			if i < len(source) {
				i += 2
			}
			blank(start, i)
		case source[i] == '"' || source[i] == '\'':
			quote := source[i]
			verbatim := quote == '"' && i > 0 && source[i-1] == '@'
			i++
			for i < len(source) {
				if !verbatim && source[i] == '\\' && i+1 < len(source) {
					i += 2
				} else if source[i] == quote {
					i++
					if verbatim && i < len(source) && source[i] == quote {
						i++
						continue
					}
					break
				} else {
					i++
				}
			}
			if literals {
				blank(start, i)
			}
		default:
			i++
		}
	}
	return string(masked)
}

func repairM1368Member(t *testing.T, file, declaration string) string {
	t.Helper()
	source := repairM1368Mask(repairM1368Read(t, file), false)
	code := repairM1368Mask(source, true)
	start := strings.Index(code, declaration)
	if start < 0 || strings.Contains(code[start+len(declaration):], declaration) {
		t.Fatalf("%s: expected one declaration %q", file, declaration)
	}
	opening := strings.IndexByte(code[start:], '{')
	if opening < 0 {
		t.Fatalf("%s: no body for %q", file, declaration)
	}
	depth := 0
	for i := start + opening; i < len(code); i++ {
		switch code[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : i+1]
			}
		}
	}
	t.Fatalf("%s: unclosed body for %q", file, declaration)
	return ""
}

func repairM1368Contains(t *testing.T, source string, fragments ...string) {
	t.Helper()
	source = strings.Join(strings.Fields(repairM1368Mask(source, false)), " ")
	for _, fragment := range fragments {
		if !strings.Contains(source, strings.Join(strings.Fields(fragment), " ")) {
			t.Errorf("missing source contract: %s", fragment)
		}
	}
}

func repairM1368Before(t *testing.T, source, first, second string) {
	t.Helper()
	a, b := strings.Index(source, first), strings.Index(source, second)
	if a < 0 || b < 0 || a >= b {
		t.Errorf("expected %q before %q", first, second)
	}
}

func repairM1368Case(t *testing.T, file, declaration string) string {
	t.Helper()
	path := filepath.Join("tests/LayerX.Sdk.Tests", file)
	source := repairM1368Mask(repairM1368Read(t, path), false)
	index := strings.Index(source, declaration)
	if index < 0 || !strings.HasSuffix(strings.TrimSpace(source[:index]), "[Fact]") {
		t.Fatalf("%s: %s must remain an enabled Fact", path, declaration)
	}
	var project struct {
		Framework  string `xml:"PropertyGroup>TargetFramework"`
		References []struct {
			Include string `xml:"Include,attr"`
		} `xml:"ItemGroup>ProjectReference"`
	}
	if err := xml.Unmarshal([]byte(repairM1368Read(t, "tests/LayerX.Sdk.Tests/LayerX.Sdk.Tests.csproj")), &project); err != nil {
		t.Fatal(err)
	}
	if project.Framework != "net8.0" || len(project.References) != 1 || project.References[0].Include != "../../LayerX.Sdk.csproj" {
		t.Fatalf("C# behavior tests must reference the production SDK: %+v", project)
	}
	if !strings.Contains(repairM1368Read(t, "repair_m1368.kvx"), `test = "`+t.Name()+`"`) {
		t.Errorf("repair record does not map this finding to %s", t.Name())
	}
	return repairM1368Member(t, path, declaration)
}

func TestRepairM_1368_program_deployment_limit(t *testing.T) {
	source := repairM1368Read(t, "NativeProgramLifecycle.cs")
	repairM1368Contains(t, source,
		"internal const int MaximumPayloadBytes = 524288;",
		"internal static void PayloadSize(long length) => Require(length <= MaximumPayloadBytes);",
		"wasm.Length is >= 8 and <= MaximumPayloadBytes")
	for _, entry := range []struct{ declaration, size string }{
		{"public NativeProgramDeploy(byte[] programId", "104L + wasm.Length + (programInterface is null ? 0L : 4L + programInterface.Length)"},
		{"public NativeProgramUpgrade(byte[] programId", "106L + migrationHook.Length + wasm.Length + (programInterface is null ? 0L : 4L + programInterface.Length)"},
	} {
		member := repairM1368Member(t, "NativeProgramLifecycle.cs", entry.declaration)
		repairM1368Contains(t, member, "LifecycleWire.PayloadSize("+entry.size+");")
		repairM1368Before(t, member, "LifecycleWire.PayloadSize(", "new MemoryStream()")
	}
	for _, name := range []string{"Deploy", "Upgrade"} {
		member := repairM1368Member(t, "NativeProgramLifecycle.cs", "public static NativeProgram"+name+" Decode(byte[] payload)")
		repairM1368Before(t, member, "LifecycleWire.PayloadSize(payload.Length);", "LifecycleWire.Get(")
	}
	repairM1368Contains(t, repairM1368Read(t, "Programs.cs"),
		"BuildTransactionAsync(new NativeProgramLifecycleRequest(value).Payload, sender",
		"BuildTransactionAsync(new NativeProgramLifecycleRequest(value, feeLimit).Payload, sender")
	body := repairM1368Case(t, "RepairM1368Tests.cs", "public async Task CompleteLifecyclePayloadIsBoundedBeforeNativePreparation()")
	repairM1368Contains(t, body,
		"new[] { -1, 0, 1, 952 }", "new[] { false, true }", "new[] { -1, 0 }",
		"Assert.Equal(524288 + delta, payload.Length);", "Assert.Equal(payload, decoded.Encode());",
		"client.UpgradeAsync((NativeProgramUpgrade)decoded", "client.DeployAsync((NativeProgramDeploy)decoded",
		"Assert.Equal(\"eth_getTransactionCount\", call.GetProperty(\"method\").GetString());",
		"Assert.Equal(\"eth_estimateGas\", call.GetProperty(\"method\").GetString());",
		"Assert.Equal(request.Payload, transaction.Data);",
		"Assert.Throws<ArgumentException>(() => Create(524289 - overhead));",
		"var oversized = new byte[524289];",
		"Assert.Throws<ArgumentException>(() => upgrade ? (INativeProgramLifecycle)NativeProgramUpgrade.Decode(oversized) : NativeProgramDeploy.Decode(oversized));")
}

func TestRepairM_1368_programs_legacy_constructor(t *testing.T) {
	retired := repairM1368Member(t, "Programs.cs", "public ProgramsClient(PlatformClient client, byte[] sequencerPublicKey,")
	repairM1368Contains(t, retired, "throw new PlatformSdkException(SdkErrorCode.UnavailableCapability, RetryClass.Never);")
	if !strings.Contains(repairM1368Read(t, "Programs.cs"), "Retired REST constructor.") {
		t.Error("retired constructor must document the native migration")
	}
	for _, entry := range []struct{ declaration, assignment string }{
		{"public ProgramsClient(Programs rpc, ReceiptKeys? receiptKeys = null, byte[]? didId = null)", "_rpc = rpc ?? throw Invalid();"},
		{"public ProgramsClient(PlatformClient client, Programs nativePrograms, byte[] sequencerPublicKey,", "_rpc = nativePrograms ?? throw Invalid();"},
	} {
		repairM1368Contains(t, repairM1368Member(t, "Programs.cs", entry.declaration), entry.assignment, "_receiptKeys = receiptKeys;")
	}
	body := repairM1368Case(t, "RepairM1368Tests.cs", "public async Task SupportedProgramsConstructorsUseTheNativeBackend()")
	repairM1368Contains(t, body,
		"Assert.Equal(SdkErrorCode.UnavailableCapability, retired.Code);", "Assert.Equal(RetryClass.Never, retired.Retry);",
		"migration < 2", "new ProgramsClient(rpc)", "new ProgramsClient(platform, rpc,",
		"client.DiscoverAsync(ProgramId, \"receipted\")", "await server.Exchange(call =>",
		"Assert.Equal(\"eth_call\", call.GetProperty(\"method\").GetString());",
		"Assert.Equal(ProgramId, result.ProgramId);", "Assert.Equal(ProgramLifecycle.Active, result.Lifecycle);",
		"Assert.Equal(7U, result.Version);", "Assert.Equal((ushort)2, result.AbiVersion);",
		"Assert.Equal(\"pending-eth-call\", result.Verification);")
	repairM1368Case(t, "ProgramsContractTests.cs", "public async Task PendingReceiptMayOmitRetainedBytesButMustBindExpectedActivity()")
}

func TestRepairM_1368_stream_history_growth(t *testing.T) {
	source := repairM1368Read(t, "Stream.cs")
	repairM1368Contains(t, source,
		"public const int ReplayWindowCapacity = 4096;", "private readonly Queue<string> _replayWindow = new();",
		"Encoding.UTF8.GetByteCount(id) <= 512", "Array.AsReadOnly(_replayWindow.ToArray())",
		"[System.Text.Json.Serialization.JsonConstructor]")
	ack := repairM1368Member(t, "Stream.cs", "private void Acknowledge(StreamEvent<T> item)")
	repairM1368Contains(t, ack, "!_seenEventIds.Add(item.EventId)", "_replayWindow.Enqueue(item.EventId);",
		"if (_replayWindow.Count > ReplayWindowCapacity) _seenEventIds.Remove(_replayWindow.Dequeue());")
	validate := repairM1368Member(t, "Stream.cs", "private IReadOnlyList<StreamEvent<T>> ValidatePage(StreamPage<T> page)")
	repairM1368Contains(t, validate, "!ValidEventId(item.EventId)", "_seenEventIds.Contains(item.EventId)", "!pageEventIds.Add(item.EventId)")
	restore := repairM1368Member(t, "Stream.cs", "public static ResumableStream<T> Restore(StreamRecoveryState state)")
	repairM1368Contains(t, restore, "state.EventIds.Count > ReplayWindowCapacity", "new ResumableStream<T>(state.Cursor)",
		"!ValidEventId(id) || !stream._seenEventIds.Add(id)", "stream._replayWindow.Enqueue(id);")
	body := repairM1368Case(t, "RepairM1368Tests.cs", "public async Task ReplayHistoryIsBoundedAndRecovered()")
	repairM1368Contains(t, body,
		"index < capacity * 3", "Assert.InRange(stream.RecoveryState.EventIds.Count, 1, capacity);",
		"await File.WriteAllBytesAsync(path, JsonSerializer.SerializeToUtf8Bytes(snapshot));",
		"stream = ResumableStream<int>.Restore(JsonSerializer.Deserialize<StreamRecoveryState>(await File.ReadAllBytesAsync(path))!);",
		"foreach (var duplicate in snapshot.EventIds)", "Assert.Equal(SdkErrorCode.DecodeFailure, error.Code);",
		"Assert.Equal(snapshot.Cursor, stream.Cursor);", "Assert.Equal(snapshot.EventIds, stream.RecoveryState.EventIds);",
		"Assert.Equal(123, Assert.Single(accepted).Value);", "Enumerable.Repeat(\"duplicate\", 2)",
		"Enumerable.Range(0, capacity + 1)", "new string('x', 513)")
}

func TestRepairM_1368_agent_envelope_tests(t *testing.T) {
	request := repairM1368Case(t, "AgentEnvelopeMajor2Tests.cs", "public async Task AgentEnvelopeEncodesTransportVersionOne()")
	repairM1368Contains(t, request, "Golden(\"agent-http-request.json\")", "Assert.Equal(1, version(document.RootElement));",
		"Assert.Equal(Golden(\"agent-http-request.json\"), encoded);", "Enum.GetValues<PlatformOperation>()")
	response := repairM1368Case(t, "AgentEnvelopeMajor2Tests.cs", "public async Task AgentEnvelopeAcceptsUnversionedResponses()")
	repairM1368Contains(t, response,
		"(\"agent-http-success.json\", \"achieved\")", "(\"agent-http-success-pending.json\", \"pending\")",
		"(\"agent-http-success-absent.json\", \"absent\")", "await Exchange(200, golden)",
		"Assert.Equal(JsonValue.String(state), status[\"state\"]);", "Exchange(403, Golden(\"agent-http-refusal.json\"))",
		"Assert.Equal(SdkErrorCode.PolicyRefusal, error.Code);", "Assert.Equal(SdkErrorCode.DecodeFailure, refusal.Code);")
	exchange := repairM1368Member(t, "tests/LayerX.Sdk.Tests/AgentEnvelopeMajor2Tests.cs", "private static async Task<AgentEnvelopeResult> Exchange(int status, byte[] golden)")
	repairM1368Contains(t, exchange, "new AgentEnvelopeTransport(server.Endpoint, credential)", "transport.SendEnvelopeAsync(call)",
		"await server.Exchange(request =>", "AgentEnvelopeTransport.RoutePath", "Assert.Equal(requestId, result.RequestId);",
		"Assert.Equal(requestId, error.RequestId);")
	unverified := repairM1368Case(t, "AgentOperationEnvelopeTests.cs", "public async Task UnverifiedRungIsPreservedFromTheDaemon()")
	repairM1368Contains(t, unverified, "Call(input, \"read_unverified\")", "await transport.SendEnvelopeAsync(call)",
		"Assert.Equal(JsonValue.String(\"unverified\"), verification[\"state\"]);",
		"Assert.Equal(JsonValue.String(\"receipted\"), verification[\"requested\"]);",
		"Assert.Equal(JsonValue.String(\"submitted\"), verification[\"achieved\"]);")
	for _, golden := range []struct{ name, state string }{
		{"agent-http-success.json", "achieved"}, {"agent-http-success-pending.json", "pending"}, {"agent-http-success-absent.json", "absent"},
	} {
		var wire struct {
			Version      *int `json:"version"`
			Verification struct {
				State string `json:"state"`
			} `json:"verification_status"`
		}
		data := repairM1368Read(t, "../../../services/agent/schema/agent-api/golden/"+golden.name)
		if err := json.Unmarshal([]byte(data), &wire); err != nil {
			t.Fatal(err)
		}
		if wire.Version != nil || wire.Verification.State != golden.state {
			t.Errorf("daemon golden %s must be unversioned with state %s", golden.name, golden.state)
		}
	}
	var requestGolden struct {
		Version    int    `json:"version"`
		Operation  string `json:"operation"`
		Credential struct {
			Generation string `json:"generation"`
		} `json:"credential"`
	}
	if err := json.Unmarshal([]byte(repairM1368Read(t, "../../../services/agent/schema/agent-api/golden/agent-http-request.json")), &requestGolden); err != nil {
		t.Fatal(err)
	}
	if requestGolden.Version != 1 || requestGolden.Operation != "tenant.readiness" || requestGolden.Credential.Generation != "7" {
		t.Fatalf("request case must track the real daemon's V1 owner-session golden: %+v", requestGolden)
	}
	var refusalGolden map[string]json.RawMessage
	if err := json.Unmarshal([]byte(repairM1368Read(t, "../../../services/agent/schema/agent-api/golden/agent-http-refusal.json")), &refusalGolden); err != nil {
		t.Fatal(err)
	}
	if len(refusalGolden) != 5 || string(refusalGolden["class"]) != `"PolicyRefusal"` || refusalGolden["version"] != nil {
		t.Fatal("refusal case must consume the daemon's unversioned PolicyRefusal golden")
	}
	repairM1368Contains(t, repairM1368Read(t, "Transport.cs"), "EnvelopeVersion = 1;",
		`"unverified" => Exact(status, "state", "requested", "achieved", "reason")`,
		`!Exact(envelope, "request_id", "value", "verification_status")`)
}

func TestRepairM_1368_agent_probe_registration_gap(t *testing.T) {
	body := repairM1368Case(t, "AgentOperationEnvelopeTests.cs", "public async Task ProcessCasesReachTheDaemonThroughTheGateway()")
	repairM1368Contains(t, body,
		`["read", "program_read", "approval_list", "agent_register", "session_open"]`,
		`["read.account", "program.interface", "approval.list", "agent.register", "session.open"]`,
		"Assert.Equal(expected, input.Cases);", "Credential(input, input.Generation)",
		"transport.SendEnvelopeAsync(call, observeResponse:", "Assert.Equal(200, status);",
		`if (caseId is "agent_register" or "session_open")`, "Assert.NotNull(call.IdempotencyKey);",
		"AssertPrepared(input, caseId, result);", "Transport(input, null, key)",
		"Assert.Equal(SdkErrorCode.CapabilityRefusal, refusal.Code);", "await AssertOwnerlessDaemonRefusal(transport, call);")
	prepared := repairM1368Member(t, "tests/LayerX.Sdk.Tests/AgentOperationEnvelopeTests.cs", "private static void AssertPrepared(CaseFile input, string caseId, AgentEnvelopeResult result)")
	repairM1368Contains(t, prepared,
		`["chain_id", "nonce", "max_priority_fee_per_gas", "max_fee_per_gas", "gas_limit", "to", "value", "data", "signing_hash", "observed_head_height"]`,
		`entry.GetProperty("expected_preparation")`, "foreach (var field in fields)",
		"Assert.Equal(expected.GetProperty(field).GetString(), Assert.IsType<JsonValue.StringValue>(prepared[field]).Value);",
		`Assert.Equal(JsonValue.String(input.Tenant), authority["tenant"]);`, `? "1004" : "101c"`, `? "dd9aa628" : "1107b49e"`)
	refusal := repairM1368Member(t, "tests/LayerX.Sdk.Tests/AgentOperationEnvelopeTests.cs", "private static async Task AssertOwnerlessDaemonRefusal(AgentEnvelopeTransport transport, TransportCall call)")
	repairM1368Contains(t, refusal, `body["credential"] = null;`, "await http.SendAsync(request, timeout.Token)",
		"Assert.Equal(400, (int)reply.StatusCode);", `Assert.Equal("envelope.credential", parsed.RootElement.GetProperty("reason").GetString());`,
		"Assert.Equal(SdkErrorCode.ProtocolIncompatibility, error.Code);", "Assert.Equal(requestId, error.RequestId);")
}

func TestRepairM_1368_portable_positive_vector_count(t *testing.T) {
	body := repairM1368Case(t, "PortableReceiptTests.cs", "public void PortableReceiptRoundTripsAtReceipted()")
	names := []string{"V1-pax-transfer", "V1-erc20-approve", "V1-layerxledger-pay-exact", "V1-layerxbridge-bridge-out", "V1-layerxprograms-call"}
	var fixture struct {
		Cases []struct {
			ID     string `json:"id"`
			Vector string `json:"vector"`
		} `json:"cases"`
	}
	if err := json.Unmarshal([]byte(repairM1368Read(t, "../conformance/fixtures/sequencer-receipt-v1.json")), &fixture); err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, vector := range fixture.Cases {
		if vector.Vector == "V1" {
			actual = append(actual, vector.ID)
		}
	}
	if strings.Join(actual, ",") != strings.Join(names, ",") {
		t.Fatalf("the C# test must pin the entire current V1 producer corpus: %v", actual)
	}
	repairM1368Contains(t, body, `Assert.Equal(new[] { "`+strings.Join(names, `", "`)+`" }, cases.Select(vector => vector.GetProperty("id").GetString()).ToArray());`,
		"foreach (var vector in cases)", "var keys = Keys(vector);",
		`PortableReceipt.export(vector.GetProperty("receiptJson"), Hex(vector, "rawTransaction"), keys)`,
		"PortableReceipt.verify(bytes, keys)", `Assert.Equal(Hex(vector, "signedReceipt"), roundTrip.Verification.CanonicalBytes);`,
		`Assert.Equal(Hex(vector, "receiptDigest"), roundTrip.Verification.ReceiptDigest);`,
		`Assert.Equal(Hex(vector.GetProperty("receiptJson"), "returnData"), roundTrip.Verification.ReturnData);`,
		"Assert.Equal(bytes, roundTrip.export());", "bytes[0] ^= 1;", "Assert.Equal((byte)'{', roundTrip.export()[0]);")
	refusals := repairM1368Case(t, "PortableReceiptTests.cs", "public void PortableReceiptRefusesVersionOneFormat()")
	repairM1368Contains(t, refusals, `Reject(bundle => bundle["signedReceipt"] = "0x00");`,
		`Reject(bundle => bundle["receiptKey"]!["publicKey"] = "0x" + new string('0', 64));`,
		"Refused(() => PortableReceipt.verify(bytes, keys with { Pinned = Array.Empty<ReceiptKey>() }));")
}

func TestRepairM_1368_portable_consensus_test_trust(t *testing.T) {
	verify := repairM1368Member(t, "PortableReceipt.cs", "public static PortableReceipt verify(ReadOnlyMemory<byte> bundle, ReceiptKeys keys, PortableReceiptTrust? trust = null)")
	repairM1368Before(t, verify, "Require(trust is not null && trust.Authenticates(header));", "verified = Committed.Verify(")
	repairM1368Contains(t, repairM1368Read(t, "PortableReceipt.cs"), "header.ChainId == ChainId && header.Height == Height",
		"CryptographicOperations.FixedTimeEquals(headerHash, Committed.HeaderHash(header))")
	anchor := repairM1368Member(t, "tests/LayerX.Sdk.Tests/PortableReceiptTests.cs", "private static PortableReceiptTrust PublishedTrust()")
	repairM1368Contains(t, anchor, "LoadCommittedFixture()", `GetProperty("execution_commit")`,
		`new PortableReceiptTrust("committed-fixture", 10,`, `GetProperty("block_id").GetProperty("hash")`)
	body := repairM1368Case(t, "PortableReceiptTests.cs", "public void PortableCommittedRefusesCoherentConsensusReplacement()")
	repairM1368Contains(t, body,
		"var trust = PublishedTrust();", "var original = PortableReceipt.verify(bytes, keys, trust);",
		"Refused(() => PortableReceipt.verify(bytes, keys));", "var forgedHeader = header with",
		"ValidatorsHash = validatorHash", "NextValidatorsHash = validatorHash", "Sign(forgedHeader, new[] { attacker }, 1)",
		"var forgedNext = inclusion.NextHeader with", "LastBlockId = forgedCommit.BlockId",
		"NextHeader = forgedNext", "NextCommit = Sign(forgedNext, new[] { attacker }, 1)", "NextValidators = validators",
		`Assert.Equal("committed", Committed.Verify(receipt, forgedHeader, forgedInclusion, forgedCommit, validators).Rung);`,
		`forged["blockHeader"] = JsonSerializer.SerializeToNode(forgedHeader);`,
		`forged["commit"] = JsonSerializer.SerializeToNode(forgedCommit);`,
		`forged["validatorSet"] = JsonSerializer.SerializeToNode(validators);`,
		`forged["receiptInclusion"] = JsonSerializer.SerializeToNode(forgedInclusion);`,
		"Refused(() => PortableReceipt.verify(forgedBytes, keys, trust));", "Refused(() => PortableReceipt.verify(forgedBytes, keys));",
		"forgedHeader, forgedInclusion, forgedCommit, validators, trust));", "Assert.Equal(bytes, original.export());")
	positive := repairM1368Case(t, "PortableReceiptTests.cs", "public void PortableExportMatchesPublishedVectorV5()")
	repairM1368Contains(t, positive, "var trust = PublishedTrust();", "PortableReceipt.verify(bytes, keys, trust)",
		"Assert.Equal(expectedBytes, exported);")
}

func TestRepairM_1368_retired_program_transport_test_path(t *testing.T) {
	file := "tests/LayerX.Sdk.Tests/ProgramTransportRetiredTests.cs"
	transport := repairM1368Member(t, file, "private static async Task sendProgram(string rpcMethod, Func<ProgramsClient, CancellationToken, Task> send)")
	repairM1368Contains(t, transport,
		`new Uri($"http://127.0.0.1:{((IPEndPoint)listener.LocalEndpoint).Port}/")`,
		"new Programs(endpoint, http)", "listener.AcceptTcpClientAsync(timeout.Token)",
		`Assert.Equal("POST / HTTP/1.1", requestLine);`, "Assert.Equal(recorded.Body, body);",
		"Assert.Equal(rpcMethod, request.RootElement.GetProperty(\"method\").GetString());")
	if strings.Contains(transport, "/rpc") {
		t.Error("native node proof must not route through the retired /rpc ingress")
	}
	body := repairM1368Case(t, "ProgramTransportRetiredTests.cs", "public async Task ProgramOperationsNeverCallProgramsRestIngress()")
	repairM1368Contains(t, body, "foreach (var operation in operations)", "await sendProgram(operation.RpcMethod, operation.Send);",
		"foreach (var operation in catalog)", "legacy.SendAsync(new TransportCall(",
		"Assert.Equal(SdkErrorCode.UnavailableCapability, error.Code);", "Assert.Equal(RetryClass.Never, error.Retry);")
	routes := repairM1368Member(t, file, "private static IEnumerable<(string Name, string RpcMethod, Func<ProgramsClient, CancellationToken, Task> Send)> programRoutes()")
	for _, method := range []string{"DiscoverAsync", "InterfaceAsync", "SimulateAsync", "CallAsync", "ReceiptAsync", "ActivityAsync", "DeployAsync", "UpgradeAsync", "WindDownAsync"} {
		repairM1368Contains(t, routes, "client."+method+"(")
	}
}
