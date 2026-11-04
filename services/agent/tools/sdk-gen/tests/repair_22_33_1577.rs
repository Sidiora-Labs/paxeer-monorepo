#[allow(dead_code)]
#[path = "../src/main.rs"]
mod generator;

use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

static NEXT: AtomicUsize = AtomicUsize::new(0);

fn schema() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/agent-api")
}

struct SchemaCopy(PathBuf);

impl SchemaCopy {
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "sdk-gen-22-33-1577-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).expect("create isolated schema");
        for entry in fs::read_dir(schema()).expect("read canonical schema directory") {
            let entry = entry.expect("canonical schema entry");
            if entry.path().extension().is_some_and(|value| value == "kvx") {
                fs::copy(entry.path(), root.join(entry.file_name()))
                    .expect("copy canonical module");
            }
        }
        Self(root)
    }

    fn programs(&self) -> String {
        fs::read_to_string(self.0.join("programs.kvx")).expect("read Programs schema")
    }

    fn replace(&self, source: &str) {
        fs::write(self.0.join("programs.kvx"), source).expect("write Programs schema");
    }
}

impl Drop for SchemaCopy {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove isolated schema");
    }
}

fn output<'a>(generated: &'a generator::Generated, path: &str) -> &'a str {
    generated
        .files
        .get(Path::new(path))
        .expect("generated file")
}

fn constant<'a>(source: &'a str, name: &str) -> &'a str {
    source
        .split_once(&format!("{name} = '"))
        .and_then(|(_, tail)| tail.split_once('\''))
        .map(|(value, _)| value)
        .expect("generated JSON constant")
}

#[test]
fn generator_retired_lifecycle_generates_current_evm_metadata() {
    let generated = generator::agent_sdk_generator(&schema()).expect("real SDK generator");
    assert_eq!(generated.files.len(), 6);
    let ts = output(&generated, "typescript/src/generated/client.ts");
    let py = output(&generated, "python/layerx_sdk/generated/client.py");
    let metadata = constant(ts, "PROGRAM_LIFECYCLE_METADATA_JSON");
    assert_eq!(metadata, constant(py, "PROGRAM_LIFECYCLE_METADATA_JSON"));
    assert!(metadata.contains("\"method\":\"eth_sendRawTransaction\""));
    assert!(metadata.contains("\"transaction_type\":\"EIP-1559\""));
    assert!(metadata.contains("\"chain_id\":125"));
    for (operation, selectors) in [
        ("deploy", vec!["0xbde3ff66"]),
        ("upgrade", vec!["0xa860929f"]),
        (
            "wind-down",
            vec!["0x86ba1a99", "0x4dc50730", "0x23330dd5", "0xa512f1a9"],
        ),
    ] {
        let selectors = selectors
            .iter()
            .map(|value| format!("\"{value}\""))
            .collect::<Vec<_>>()
            .join(",");
        assert!(metadata.contains(&format!(
            "\"program.{operation}\":{{\"precompile\":\"0x000000000000000000000000000000000000101d\",\"selectors\":[{selectors}]}}"
        )));
    }
    for (rung, evidence) in [
        (
            "submitted",
            "eth_sendRawTransaction_accepted_the_exact_transaction_hash",
        ),
        ("receipted", "verified_transaction_receipt"),
        ("committed", "verified_block_commit"),
    ] {
        assert!(metadata.contains(&format!("\"{rung}\":\"{evidence}\"")));
    }
    assert!(metadata.contains("retain_exact_signed_EIP1559_bytes_and_resolve_by_transaction_hash"));
    for source in [ts, py] {
        assert!(!source.contains("/v1/programs/deploy"));
        assert!(!source.contains("/v1/programs/upgrade"));
        assert!(!source.contains("/v1/programs/wind-down"));
        assert!(!source.contains("application/octet-stream"));
        assert!(!source.contains("{{LIFECYCLE_METADATA}}"));
        assert!(source.contains("2097154"));
    }
    let stub = output(&generated, "python/layerx_sdk/generated/client.pyi");
    assert!(stub.contains("def program_lifecycle_request("));
    assert!(stub.contains("ProgramLifecycleOperation: TypeAlias"));
}

#[test]
fn generator_retired_lifecycle_does_not_require_retired_ingress() {
    let fixture = SchemaCopy::new();
    let original = fixture.programs();
    let mut native = original.clone();
    for operation in ["deploy", "upgrade", "wind-down"] {
        let start = native
            .find(&format!("[operation.program.{operation}]"))
            .expect("lifecycle operation");
        let end = native[start + 1..]
            .find("\n[")
            .map_or(native.len(), |offset| start + 1 + offset);
        let section = native[start..end].to_owned();
        let revised = section
            .lines()
            .filter(|line| {
                !line.starts_with("method =")
                    && !line.starts_with("path =")
                    && !line.starts_with("content_type =")
            })
            .collect::<Vec<_>>()
            .join("\n");
        native.replace_range(start..end, &format!("{revised}\n"));
    }
    assert_ne!(native, original);
    fixture.replace(&native);
    assert_eq!(
        generator::agent_sdk_generator(&fixture.0)
            .expect("native contract without retired ingress"),
        generator::agent_sdk_generator(&schema()).expect("canonical typed transaction contract")
    );
}

#[test]
fn generator_retired_lifecycle_rejects_transaction_and_rung_drift() {
    let fixture = SchemaCopy::new();
    let original = fixture.programs();
    for (before, after) in [
        ("canonical_signed_EIP1559_transaction_bytes_at_most_1048576", "native_signed_activity_bytes"),
        ("required = [\"signed_activity\"]", "required = []"),
        ("0x000000000000000000000000000000000000101d", "0x000000000000000000000000000000000000101a"),
        ("0xbde3ff66", "0xbde3ff67"),
        ("0xa860929f", "0xa860929e"),
        ("0x23330dd5", "0x23330dd4"),
        ("0x86ba1a99", "0x86ba1a98"),
        ("0xa512f1a9", "0xa512f1a8"),
        ("0x4dc50730", "0x4dc50731"),
        ("payload = \"NativeProgramUpgrade\"", "payload = \"NativeProgramDeploy\""),
        ("exact_ABI_calldata_signed_by_the_bound_actor_and_HTTP_Idempotency_Key_equals_transaction_hash", "unbound_calldata"),
        ("achieved_rung_from_the_retained_EIP1559_transaction_progress", "requested_rung"),
        ("keccak256_of_exact_signed_EIP1559_transaction_bytes", "native_activity_id"),
        ("submission_ref = \"transaction_hash\"", "submission_ref = \"opaque_id\""),
        ("eth_sendRawTransaction_accepted_the_exact_transaction_hash", "HTTP_accepted"),
        ("verified_transaction_receipt", "unverified_receipt"),
        ("verified_block_commit", "receipt_only"),
        ("retain_exact_signed_EIP1559_bytes_and_resolve_by_transaction_hash", "retry_with_new_bytes"),
        ("rung = [\"submitted\",\"receipted\",\"committed\"]", "rung = [\"submitted\",\"committed\"]"),
    ] {
        let modified = original.replace(before, after);
        assert_ne!(modified, original, "mutation must change canonical schema: {before}");
        fixture.replace(&modified);
        assert!(
            generator::agent_sdk_generator(&fixture.0).is_err(),
            "generator must refuse drift: {before}"
        );
    }
}

fn fixture_value<'a>(fixture: &'a str, key: &str) -> &'a str {
    fixture
        .split_once(&format!("\"{key}\": \""))
        .and_then(|(_, tail)| tail.split_once('"'))
        .map(|(value, _)| value)
        .expect("recorded producer field")
}

#[test]
fn generator_retired_lifecycle_generated_request_reaches_101d_transport() {
    // The recorded producer used RFC6979 secp256k1 signing on chain 125.
    let transcript =
        include_str!("../../../../../sdk/platform/go/testdata/repair141/native-deprecate.json");
    let raw = fixture_value(transcript, "rawTransaction");
    let hash = fixture_value(transcript, "transactionHash");
    assert!(raw.starts_with("0x02"));
    assert!(raw.contains("94000000000000000000000000000000000000101d"));
    assert!(raw.contains(fixture_value(transcript, "calldata").trim_start_matches("0x")));
    let generated = generator::agent_sdk_generator(&schema()).expect("production generator");
    for path in [
        "typescript/src/generated/client.ts",
        "python/layerx_sdk/generated/client.py",
    ] {
        let source = output(&generated, path);
        // Exercise the actual emitted wire template, with its documented substitutions.
        let request = constant(source, "PROGRAM_LIFECYCLE_REQUEST_JSON")
            .replace("{{request_id}}", "17")
            .replace("{{signed_activity}}", raw);
        let expected = format!(
            "{{\"jsonrpc\":\"2.0\",\"id\":17,\"method\":\"eth_sendRawTransaction\",\"params\":[\"{raw}\"]}}"
        );
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback transcript peer");
        let address = listener.local_addr().expect("peer address");
        let response_body = format!("{{\"jsonrpc\":\"2.0\",\"id\":17,\"result\":\"{hash}\"}}");
        let expected_response = response_body.clone();
        let peer = std::thread::spawn(move || {
            let (stream, _) = listener.accept().expect("accept generated request");
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("peer read bound");
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .expect("peer write bound");
            let mut reader = BufReader::new(stream);
            let mut line = String::new();
            reader.read_line(&mut line).expect("HTTP request line");
            assert_eq!(line, "POST / HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                assert_ne!(reader.read_line(&mut line).expect("HTTP header"), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.strip_prefix("Content-Length: ") {
                    length = Some(value.trim().parse::<usize>().expect("content length"));
                }
            }
            let mut body = vec![0; length.expect("framed JSON-RPC request")];
            reader.read_exact(&mut body).expect("generated wire body");
            assert_eq!(
                body,
                expected.as_bytes(),
                "exact signed producer bytes must reach eth_sendRawTransaction"
            );
            write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response_body}", response_body.len()).expect("recorded transaction-hash response");
        });
        let mut client = TcpStream::connect(address).expect("connect loopback transport");
        client
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("client read bound");
        client
            .set_write_timeout(Some(Duration::from_secs(5)))
            .expect("client write bound");
        write!(client, "POST / HTTP/1.1\r\nHost: {address}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{request}", request.len()).expect("send emitted lifecycle request");
        let mut response = String::new();
        client
            .read_to_string(&mut response)
            .expect("read submitted hash");
        peer.join().expect("recorded consumer assertions");
        assert_eq!(
            response
                .split_once("\r\n\r\n")
                .expect("HTTP response body")
                .1,
            expected_response
        );
        assert!(!response.contains("committed"));
        assert!(!response.contains("receipted"));
    }
}
