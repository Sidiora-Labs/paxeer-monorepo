"""Exercise daemon producer codecs and the generated JVM consumer over HTTP."""

import importlib.util
import json
import os
import subprocess
import threading
import urllib.request
import xml.etree.ElementTree as ET
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


REPO = Path(__file__).resolve().parents[4]
DAEMON = REPO / "services/agent/crates/layerx-agentd"
API = REPO / "services/agent/crates/layerx-agent-api/src"
JVM = REPO / "sdk/platform/jvm"


def generator():
    spec = importlib.util.spec_from_file_location(
        "generate_jvm", REPO / "sdk/platform/generators/generate_jvm.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def run(command, **kwargs):
    result = subprocess.run(
        command, text=True, capture_output=True, timeout=600, **kwargs
    )
    assert result.returncode == 0, result.stdout + result.stderr
    return result.stdout


def rust_item(path, declaration):
    """Compile the named production item verbatim, including its derive attributes."""
    source = path.read_text()
    start = source.index(declaration)
    previous = source[:start].rstrip()
    if previous.splitlines()[-1].lstrip().startswith("#[derive("):
        start = source.rfind("#[derive(", 0, start)
    opening = source.index("{", start)
    semicolon = source.find(";", start, opening)
    if semicolon != -1:
        return source[start : semicolon + 1]
    depth = 1
    end = opening + 1
    while depth:
        depth += (source[end] == "{") - (source[end] == "}")
        end += 1
    return source[start:end]


def producer_source():
    """Isolate the real codec items without building the entire daemon workspace."""
    wire = DAEMON / "src/agent_rpc_wire.rs"
    dispatch = DAEMON / "src/agent_rpc_dispatch.rs"
    rpc = DAEMON / "src/agent_rpc.rs"
    identity = API / "identity.rs"
    error = API / "error.rs"
    items = [
        (identity, "pub enum ContractError"),
        (identity, "pub struct NativeActivity"),
        (identity, "impl NativeActivity"),
        (identity, "pub struct NativePrepareResultV1"),
        (error, "pub struct RequestId"),
        (error, "pub struct ReasonCode"),
        (error, "impl ReasonCode"),
        (error, "pub enum ErrorClass"),
        (error, "pub enum RungStatus"),
        (error, "impl RungStatus"),
        (rpc, "pub struct Rejection"),
        (rpc, "pub struct AgentRpcResponse"),
        (rpc, "const fn level_name"),
        (rpc, "fn verification_json"),
        (rpc, "pub fn success_body"),
        (dispatch, "pub(crate) struct Dispatched"),
        (dispatch, "pub(crate) fn lower_hex"),
        (dispatch, "pub(crate) struct Reader"),
        (dispatch, "impl<'a> Reader<'a>"),
        (dispatch, "fn dec("),
        (dispatch, "fn hexv("),
        (dispatch, "pub(crate) fn decode_preparation"),
        (wire, "pub(crate) trait Canonical"),
        (wire, "impl Canonical for NativeActivity"),
        (wire, "impl Canonical for NativePrepareResultV1"),
        (wire, "fn purpose_body()"),
        (wire, "fn signed_purpose_body()"),
        (wire, "fn native_prepare_body()"),
        (DAEMON / "tests/prepare_eip1559.rs", "fn call()"),
        (DAEMON / "tests/prepare_eip1559.rs", "fn preparation("),
    ]
    constructor = rust_item(rpc, "pub(crate) const fn new(")
    return "\n".join(
        [
            "use serde_json::{json, Map, Value};",
            "use layerx_types::rung::Rung;",
            "use layerx_types::result::Retriability;",
            "use layerx_crypto::evm_transaction::Eip1559Call;",
            "use layerx_wire::encode::Encoder;",
            "use sha2::{Digest, Sha256};",
            "type VerificationStatus = RungStatus;",
            "const MAX_BYTES: usize = 1_048_576; const MAX_TEXT: usize = 255;",
            *[rust_item(path, declaration) for path, declaration in items],
            "impl Rejection {" + constructor + "}",
            RUST_MAIN,
        ]
    )


RUST_MAIN = r'''
fn main() {
    use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
    use layerx_types::ids::{Did, IdempotencyKey};
    let kind = ActivityType::new(ModuleId::Programs, 1).unwrap();
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Programs, &[kind]).unwrap()
    ]).unwrap();
    let payload = [1, 2];
    let mut builder = EnvelopeBuilder::new();
    builder.protocol_version(layerx_wire::limits::PROTOCOL_VERSION).unwrap();
    builder.network_id(125).unwrap();
    builder.activity_type(kind).unwrap();
    builder.actor_did(Did::new(b"did:layerx:alice").unwrap()).unwrap();
    builder.authority(Authority::owner(b"owner-primary").unwrap()).unwrap();
    builder.account_sequence(4).unwrap();
    builder.timestamp_bound(TimestampBound::new(10, 20).unwrap()).unwrap();
    builder.idempotency_key(IdempotencyKey::new([0x88; 32])).unwrap();
    builder.fee_limit(layerx_types::amount::Amount::from_u128(123)).unwrap();
    builder.payload_hash(Sha256::digest(payload).into()).unwrap();
    builder.payload(Payload::new(&registry, kind, &payload).unwrap()).unwrap();
    let envelope = builder.build().unwrap();
    let canonical = layerx_wire::activity::encode_unsigned_envelope(&envelope).unwrap();
    let id = Sha256::digest(&canonical).into();
    let mut results = Vec::new();
    for approval_required in [true, false] {
        let result = NativePrepareResultV1 {
            preparation_id: id,
            canonical_bytes: canonical.clone(),
            signing_preimage: *layerx_wire::sign::preimage_unsigned(&envelope).unwrap().as_bytes(),
            activity: NativeActivity::new(9, 1).unwrap(),
            approval_required,
            approval_id: approval_required.then_some(id),
        };
        let answer = success_body(RequestId(42), &Dispatched {
            value: result.canonical(), verification: None,
        }, None).unwrap();
        assert_eq!(answer.status, 200);
        results.push(serde_json::from_slice::<Value>(&answer.body).unwrap());
    }
    let encoded = preparation(&call());
    let mut reader = Reader { bytes: &encoded, offset: 0 };
    let (value, _) = decode_preparation(&mut reader).unwrap();
    reader.finish().unwrap();
    let mut evm_request = Map::new();
    for field in ["activity_type", "actor", "authority", "account_sequence", "not_before",
        "not_after", "idempotency_key", "fee_limit", "payload", "payload_hash"] {
        evm_request.insert(field.to_string(), value[field].clone());
    }
    evm_request.insert("capability_id".into(), Value::Null);
    let answer = success_body(RequestId(42), &Dispatched {
        value, verification: None,
    }, None).unwrap();
    results.push(serde_json::from_slice::<Value>(&answer.body).unwrap());
    println!("{}", json!({"answers": results, "request": native_prepare_body(), "evm_request": evm_request}));
}
'''


JAVA_CLIENT = r'''
package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.Arrays;
import java.util.Set;
import java.util.stream.Collectors;

public final class NativePrepareRoundTrip {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static void check(boolean condition, String message) {
        if (!condition) throw new AssertionError(message);
    }
    private static void refused(ObjectNode body) throws Exception {
        try {
            JSON.treeToValue(body, GeneratedSchema.AgentModels.NativePrepareResultV1.class);
        } catch (com.fasterxml.jackson.core.JsonProcessingException expected) {
            return;
        }
        throw new AssertionError("invalid native record accepted: " + body);
    }
    public static void main(String[] args) throws Exception {
        JsonNode recording = JSON.readTree(Files.readString(Path.of(args[1])));
        URI endpoint = URI.create(args[0]);
        HttpClient http = HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NEVER).build();
        try (var session = new HttpProductionTransport.AgentSessionCredential(
                "tenant-a", "11".repeat(32),
                new SecretBytes("22".repeat(32).getBytes(StandardCharsets.US_ASCII)), "1")) {
            var transport = new HttpProductionTransport(http, JSON, endpoint, endpoint,
                Duration.ofSeconds(10), null, session);
            var nativeOperation = GeneratedSchema.AgentOperations.NATIVE_PREPARE;
            check(nativeOperation.wireName().equals("prepare"), "native operation route");
            var nativeRequest = JSON.treeToValue(recording.get("request"), nativeOperation.requestType());
            ObjectNode body = JSON.valueToTree(nativeRequest);
            check(body.equals(recording.get("request")), "native request fields changed");
            var call = new ProductionTransport.Call(nativeOperation, body,
                SchemaTypes.PathParameters.none(), new IdempotencyKey("88".repeat(32)));
            for (int index = 0; index < 3; index++) {
                var activeCall = call;
                if (index == 2) {
                    var operation = GeneratedSchema.AgentOperations.EIP1559_PREPARE;
                    var evmRequest = JSON.treeToValue(recording.get("evm_request"), operation.requestType());
                    ObjectNode evmBody = JSON.valueToTree(evmRequest);
                    check(evmBody.equals(recording.get("evm_request")), "EIP-1559 request shape");
                    activeCall = new ProductionTransport.Call(operation, evmBody,
                        SchemaTypes.PathParameters.none(), new IdempotencyKey("88".repeat(32)));
                }
                byte[] encoded = transport.agentEnvelope(activeCall, "42", true);
                var request = HttpRequest.newBuilder(endpoint.resolve("/rpc"))
                    .header("Content-Type", "application/json").header("Accept", "application/json")
                    .POST(HttpRequest.BodyPublishers.ofByteArray(encoded)).build();
                var response = http.send(request, HttpResponse.BodyHandlers.ofByteArray());
                check(response.statusCode() == 200, "HTTP status");
                check(response.headers().firstValue("Content-Type").orElseThrow()
                    .equals("application/json"), "HTTP codec");
                JsonNode answer = JSON.readTree(response.body());
                check(answer.equals(recording.get("answers").get(index)), "daemon envelope changed");
                check(answer.get("request_id").asText().equals("42"), "response identity");
                check(answer.get("verification_status").get("rung").asText().equals("submitted"),
                    "preparation must not claim execution");
                JsonNode value = answer.get("value");
                if (index < 2) {
                    var prepared = JSON.treeToValue(value, nativeOperation.responseType());
                    check(JSON.valueToTree(prepared).equals(value), "native result lost fields");
                    check(prepared.version().equals("1"), "native version");
                    check(prepared.canonical_bytes().length() > 4, "actual canonical activity");
                    check(prepared.signing_preimage().length() == 64, "signing disclosure");
                    check(prepared.activity().module().equals("9")
                        && prepared.activity().ordinal().equals("1"), "native activity identity");
                    check(prepared.approval_required() == (index == 0), "approval decision");
                    check(index == 0 ? prepared.approval_id().equals(prepared.preparation_id())
                        : prepared.approval_id() == null, "approval identity or explicit null");
                    for (String field : Set.of("version", "preparation_id", "canonical_bytes",
                            "signing_preimage", "activity", "approval_required", "approval_id")) {
                        ObjectNode missing = ((ObjectNode) value).deepCopy();
                        missing.remove(field);
                        refused(missing);
                    }
                    ObjectNode invalid = ((ObjectNode) value).deepCopy();
                    invalid.put("version", "2");
                    refused(invalid);
                    invalid = ((ObjectNode) value).deepCopy();
                    invalid.put("approval_required", index != 0);
                    refused(invalid);
                    invalid = ((ObjectNode) value).deepCopy();
                    invalid.put("chainId", "125");
                    refused(invalid);
                } else {
                    var operation = GeneratedSchema.AgentOperations.EIP1559_PREPARE;
                    var prepared = JSON.treeToValue(value, operation.responseType());
                    check(JSON.valueToTree(prepared).equals(value), "EIP-1559 result lost fields");
                    check(prepared.chain_id().equals("125") && prepared.nonce().equals("7"),
                        "transaction identity");
                    check(prepared.max_priority_fee_per_gas().equals("1000000000")
                        && prepared.max_fee_per_gas().equals("3000000000")
                        && prepared.gas_limit().equals("100000"), "EIP-1559 fee disclosure");
                    check(prepared.signing_hash().length() == 64, "EIP-1559 signing hash");
                    check(!prepared.signing_hash().equals(
                        recording.get("answers").get(0).get("value").get("signing_preimage").asText()),
                        "independent signing contracts");
                    refused((ObjectNode) value);
                }
            }
            Set<String> fields = Arrays.stream(
                GeneratedSchema.AgentModels.NativePrepareResultV1.class.getRecordComponents())
                .map(component -> component.getName()).collect(Collectors.toSet());
            check(fields.equals(Set.of("version", "preparation_id", "canonical_bytes",
                "signing_preimage", "activity", "approval_required", "approval_id")), "native shape");
        }
        System.out.println("native approval, native no-approval and EIP-1559 daemon answers round-tripped");
    }
}
'''


def test_native_prepare_contract_matches_declared_schema():
    module = generator()
    plane = module.agent_plane(REPO)
    declared = module.sections(REPO / "services/agent/schema/agent-api")
    assert plane.models["NativePrepareResultV1"] == declared["record.NativePrepareResultV1"]
    assert plane.values["operation.prepare"] == declared["operation.prepare"]
    assert plane.models["McpNativeWriteV1"]["preparation"] == "NativePrepareResultV1"
    native = {name for name, _, _ in plane.fields("NativePrepareResultV1", [])}
    evm = {name for name, _, _ in plane.fields("Eip1559PrepareResult", [])}
    assert native == {
        "version", "preparation_id", "canonical_bytes", "signing_preimage",
        "activity", "approval_required", "approval_id",
    }
    assert {"max_priority_fee_per_gas", "max_fee_per_gas", "signing_hash"} <= evm
    assert native.isdisjoint(evm)


def test_native_daemon_answers_round_trip_in_jvm_client(tmp_path):
    module = generator()
    producer = tmp_path / "producer"
    producer.mkdir()
    (producer / "src").mkdir()
    dependencies = "\n".join(
        f'{name} = {{ path = {json.dumps(str(REPO / "services/agent/crates" / name))} }}'
        for name in ("layerx-types", "layerx-wire", "layerx-crypto")
    )
    (producer / "Cargo.toml").write_text(
        '[package]\nname = "native-jvm-codec-proof"\nversion = "0.0.0"\nedition = "2021"\n'
        '[workspace]\n[dependencies]\nserde_json = "=1.0.151"\nsha2 = "=0.10.9"\n'
        + dependencies + "\n"
    )
    (producer / "src/main.rs").write_text(producer_source())
    recording = json.loads(run(["cargo", "run", "--quiet", "--manifest-path", str(producer / "Cargo.toml")]))
    recording_path = tmp_path / "daemon-answers.json"
    recording_path.write_text(json.dumps(recording))
    namespace = {"m": "http://maven.apache.org/POM/4.0.0"}
    pom = ET.parse(JVM / "pom.xml")
    jackson = pom.findtext("m:properties/m:jackson.version", namespaces=namespace)
    bc = pom.findtext("m:properties/m:bouncycastle.version", namespaces=namespace)
    jars = []
    for group, artifact, version in [
        ("com/fasterxml/jackson/core", "jackson-databind", jackson),
        ("com/fasterxml/jackson/core", "jackson-core", jackson),
        ("com/fasterxml/jackson/core", "jackson-annotations", jackson),
        ("org/bouncycastle", "bcprov-jdk18on", bc),
    ]:
        jar = tmp_path / f"{artifact}-{version}.jar"
        urllib.request.urlretrieve(
            f"https://repo.maven.apache.org/maven2/{group}/{artifact}/{version}/{jar.name}", jar
        )
        jars.append(str(jar))
    sources = tmp_path / "java"
    sources.mkdir()
    for name, content in {
        "GeneratedSchema.java": module.generate_jvm(REPO),
        "GeneratedContract.java": module.generate_contract(REPO),
        "NativePrepareRoundTrip.java": JAVA_CLIENT,
    }.items():
        (sources / name).write_text(content)
    original = [
        str(path) for path in (JVM / "src/main/java").rglob("*.java")
        if path.name not in {"GeneratedSchema.java", "GeneratedContract.java"}
    ]
    classes = tmp_path / "classes"
    classes.mkdir()
    classpath = os.pathsep.join(jars)
    run(["javac", "--release", "21", "-cp", classpath, "-d", str(classes),
         *original, *map(str, sources.glob("*.java"))])
    received = []
    errors = []

    class RecordedDaemon(BaseHTTPRequestHandler):
        def do_POST(self):
            try:
                assert self.path == "/rpc"
                assert self.headers["Content-Type"] == "application/json"
                assert self.headers["Accept"] == "application/json"
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                assert set(request) == {
                    "version", "request_id", "operation", "request", "credential", "idempotency_key"
                }
                assert request["version"] == 2
                assert request["request_id"] == "42"
                assert request["operation"] == "prepare"
                expected = "evm_request" if len(received) == 2 else "request"
                assert request["request"] == recording[expected]
                assert request["idempotency_key"] == "88" * 32
                assert request["credential"] == {
                    "tenant": "tenant-a", "session_id": "11" * 32,
                    "token_id": "22" * 32, "generation": "1",
                }
                answer = recording["answers"][len(received)]
                received.append(request)
                body = json.dumps(answer).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            except Exception as error:
                errors.append(error)
                self.send_error(500)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), RecordedDaemon)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        output = run([
            "java", "-cp", os.pathsep.join([str(classes), classpath]),
            "com.sidiora.layerx.sdk.NativePrepareRoundTrip",
            f"http://127.0.0.1:{server.server_port}", str(recording_path),
        ])
    finally:
        server.shutdown()
        server.server_close()
        worker.join()
    assert not errors
    assert len(received) == 3
    assert "native approval, native no-approval and EIP-1559 daemon answers round-tripped" in output
