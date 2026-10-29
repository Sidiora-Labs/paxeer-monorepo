include!("../src/main.rs");

mod files {
    use std::error::Error;
    use std::fs;
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::{Ipv4Addr, TcpListener, TcpStream};
    use std::os::unix::fs::{symlink, DirBuilderExt};
    use std::path::{Path, PathBuf};
    use std::process::{Child, Command, Output, Stdio};
    use std::sync::atomic::{AtomicU64, Ordering};
    use std::time::{Duration, Instant};

    use super::MAX_INPUT_BYTES as MAX_MESSAGE_BYTES;
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_types::result::KnownResult;
    use layerx_wire::encode::Encoder;
    use layerx_wire::hash::receipt_digest;
    use layerx_wire::limits::PROTOCOL_VERSION;

    static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
    const TARGET: &str = "0x0000000000000000000000000000000000000123";
    const UNKNOWN_REVERT: &str =
        "{\"code\":\"LXP_UNKNOWN\",\"result_code\":null,\"class\":\"terminal\",\"reason\":null}";

    type TestResult = Result<(), Box<dyn Error>>;

    struct Directory(PathBuf);

    impl Directory {
        fn new() -> Result<Self, Box<dyn Error>> {
            let path = std::env::temp_dir().join(format!(
                "layerxctl-files-{}-{}",
                std::process::id(),
                NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
            ));
            fs::DirBuilder::new().mode(0o700).create(&path)?;
            Ok(Self(path))
        }
    }

    impl Drop for Directory {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    struct Evm {
        child: Child,
        url: String,
    }

    impl Evm {
        fn start() -> Result<Self, Box<dyn Error>> {
            let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
            let address = listener.local_addr()?;
            drop(listener);
            let child = Command::new("anvil")
                .args([
                    "--quiet",
                    "--threads",
                    "1",
                    "--accounts",
                    "1",
                    "--chain-id",
                    "125",
                    "--port",
                ])
                .arg(address.port().to_string())
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()?;
            let mut evm = Self {
                child,
                url: format!("http://{address}"),
            };
            let deadline = Instant::now() + Duration::from_secs(20);
            loop {
                if let Some(status) = evm.child.try_wait()? {
                    return Err(format!("EVM process exited during startup: {status}").into());
                }
                if TcpStream::connect(address).is_ok() {
                    return Ok(evm);
                }
                if Instant::now() >= deadline {
                    return Err("EVM process did not become ready".into());
                }
                std::thread::sleep(Duration::from_millis(20));
            }
        }

        fn rpc(&self, method: &str, params: &[&str]) -> Result<String, Box<dyn Error>> {
            let output = Command::new("cast")
                .args(["rpc", "--rpc-url", &self.url, method])
                .env("RAYON_NUM_THREADS", "1")
                .args(params)
                .output()?;
            if !output.status.success() {
                return Err(
                    format!("RPC {method}: {}", String::from_utf8_lossy(&output.stderr)).into(),
                );
            }
            Ok(String::from_utf8(output.stdout)?.trim().to_owned())
        }

        fn sender(&self) -> Result<String, Box<dyn Error>> {
            self.rpc("eth_accounts", &[])?
                .split('"')
                .nth(1)
                .map(str::to_owned)
                .ok_or_else(|| "EVM returned no unlocked account".into())
        }

        fn deploy(&self, runtime: &[u8]) -> Result<String, Box<dyn Error>> {
            let init = format!(
                "0x61{:04x}61000f60003961{:04x}6000f3{}",
                runtime.len(),
                runtime.len(),
                hex(runtime)
            );
            let transaction = format!(
                "{{\"from\":\"{}\",\"data\":\"{init}\",\"gas\":\"0x1e8480\"}}",
                self.sender()?
            );
            let hash = self.rpc("eth_sendTransaction", &[&transaction])?;
            let receipt = self.rpc("eth_getTransactionReceipt", &[hash.trim_matches('"')])?;
            assert_eq!(json_string(&receipt, "status")?, "0x1");
            Ok(json_string(&receipt, "contractAddress")?.to_owned())
        }
    }

    impl Drop for Evm {
        fn drop(&mut self) {
            let _ = self.child.kill();
            let _ = self.child.wait();
        }
    }

    fn json_string<'a>(text: &'a str, key: &str) -> Result<&'a str, Box<dyn Error>> {
        let (_, value) = text
            .split_once(&format!("\"{key}\""))
            .ok_or_else(|| format!("missing JSON field {key}"))?;
        let (_, value) = value.split_once(':').ok_or("missing JSON colon")?;
        value
            .trim_start()
            .strip_prefix('"')
            .and_then(|text| text.split('"').next())
            .ok_or_else(|| format!("JSON field {key} is not a string").into())
    }

    fn hex(bytes: &[u8]) -> String {
        use std::fmt::Write as _;
        bytes.iter().fold(String::new(), |mut text, byte| {
            let _ = write!(text, "{byte:02x}");
            text
        })
    }

    fn unhex(text: &str) -> Result<Vec<u8>, Box<dyn Error>> {
        let digits = text.strip_prefix("0x").ok_or("missing hex prefix")?;
        if !digits.len().is_multiple_of(2) || !digits.is_ascii() {
            return Err("invalid hex".into());
        }
        digits
            .as_bytes()
            .chunks_exact(2)
            .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
            .collect()
    }

    fn abi_reason(reason: &[u8]) -> Vec<u8> {
        let mut bytes = vec![0x08, 0xc3, 0x79, 0xa0];
        let mut offset = [0_u8; 32];
        offset[31] = 32;
        bytes.extend_from_slice(&offset);
        let mut length = [0_u8; 32];
        length[24..].copy_from_slice(&(reason.len() as u64).to_be_bytes());
        bytes.extend_from_slice(&length);
        bytes.extend_from_slice(reason);
        bytes.resize(68 + reason.len().next_multiple_of(32), 0);
        bytes
    }

    fn revert_runtime(data: &[u8]) -> Result<Vec<u8>, Box<dyn Error>> {
        let length = u16::try_from(data.len())?.to_be_bytes();
        let mut code = Vec::new();
        code.push(0x61);
        code.extend_from_slice(&length);
        code.extend_from_slice(&[0x61, 0, 15, 0x60, 0, 0x39, 0x61]);
        code.extend_from_slice(&length);
        code.extend_from_slice(&[0x60, 0, 0xfd]);
        code.extend_from_slice(data);
        Ok(code)
    }

    fn ctl(arguments: &[&str]) -> Result<Output, Box<dyn Error>> {
        Ok(Command::new(env!("CARGO_BIN_EXE_layerxctl"))
            .args(arguments)
            .output()?)
    }

    fn bounded_file_refusal(path: &Path, certificate: bool) -> Result<Output, Box<dyn Error>> {
        let path = path.to_str().ok_or("test path is not UTF-8")?;
        let arguments = if certificate {
            vec![
                "read-state",
                "--rpc-url",
                "invalid",
                "--to",
                TARGET,
                "--data",
                "0x",
                "--ca-cert",
                path,
            ]
        } else {
            vec!["submit", "--rpc-url", "invalid", "--transaction", path]
        };
        let mut child = Command::new(env!("CARGO_BIN_EXE_layerxctl"))
            .args(arguments)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()?;
        let deadline = Instant::now() + Duration::from_secs(3);
        let timed_out = loop {
            if child.try_wait()?.is_some() {
                break false;
            }
            if Instant::now() >= deadline {
                child.kill()?;
                break true;
            }
            std::thread::sleep(Duration::from_millis(10));
        };
        let output = child.wait_with_output()?;
        assert!(!timed_out, "file input blocked before its type was checked");
        assert_eq!(output.status.code(), Some(1));
        assert!(output.stdout.is_empty());
        Ok(output)
    }

    #[test]
    fn rejects_fifo_without_waiting_for_a_writer() -> TestResult {
        let directory = Directory::new()?;
        let fifo = directory.0.join("transaction");
        assert!(Command::new("mkfifo").arg(&fifo).status()?.success());
        for certificate in [false, true] {
            let output = bounded_file_refusal(&fifo, certificate)?;
            assert!(String::from_utf8(output.stderr)?.contains("nonempty regular file"));
        }
        Ok(())
    }

    #[test]
    fn rejects_symlinks_directories_empty_and_oversized_files() -> TestResult {
        let directory = Directory::new()?;
        let empty = directory.0.join("empty");
        fs::write(&empty, [])?;
        let linked = directory.0.join("linked");
        symlink(&empty, &linked)?;
        let oversized = directory.0.join("oversized");
        fs::File::create(&oversized)?.set_len(MAX_MESSAGE_BYTES as u64 + 1)?;
        for path in [&empty, &directory.0, &oversized] {
            for certificate in [false, true] {
                let output = bounded_file_refusal(path, certificate)?;
                assert!(String::from_utf8(output.stderr)?.contains("nonempty regular file"));
            }
        }
        for certificate in [false, true] {
            let output = bounded_file_refusal(&linked, certificate)?;
            assert!(String::from_utf8(output.stderr)?.contains("file:"));
        }
        Ok(())
    }

    #[test]
    fn rejects_retired_and_command_inappropriate_options() -> TestResult {
        for arguments in [
            vec!["read-state", "--socket", "invalid"],
            vec!["read-state", "--network-id", "125"],
            vec!["read-state", "--actor", "did:layerx:test"],
            vec!["read-state", "--protocol-version", "3"],
            vec!["submit", "--public-key", "0x01"],
            vec!["submit", "--activity", "invalid"],
            vec!["submit", "--to", TARGET],
            vec!["read-state", "--transaction", "invalid"],
        ] {
            let output = ctl(&arguments)?;
            assert_eq!(output.status.code(), Some(1));
            assert_eq!(
                String::from_utf8(output.stderr)?,
                format!("layerxctl: unknown option {}\n", arguments[1])
            );
        }
        Ok(())
    }

    #[test]
    fn rejects_invalid_options_before_connecting() -> TestResult {
        for (arguments, error) in [
            (vec!["read-state", "--to"], "option requires a value"),
            (
                vec!["read-state", "--to", TARGET, "--to", TARGET],
                "empty or duplicate option --to",
            ),
            (
                vec!["read-state", "--to", ""],
                "empty or duplicate option --to",
            ),
            (
                vec!["read-state", "--rpc-url", "invalid"],
                "--to is required",
            ),
            (
                vec!["submit", "--rpc-url", "invalid"],
                "--transaction is required",
            ),
            (
                vec!["read-state", "--to", "0x01", "--data", "0x"],
                "--to requires a 20-byte address",
            ),
            (
                vec!["read-state", "--to", TARGET, "--data", "0x0"],
                "hex bytes must be even-length and within the message bound",
            ),
            (
                vec!["read-state", "--to", TARGET, "--data", "0xé"],
                "hex bytes must be even-length and within the message bound",
            ),
            (
                vec!["read-state", "--to", TARGET, "--data", "00"],
                "hex bytes require the 0x prefix",
            ),
        ] {
            let output = ctl(&arguments)?;
            assert_eq!(output.status.code(), Some(1));
            assert_eq!(
                String::from_utf8(output.stderr)?,
                format!("layerxctl: {error}\n")
            );
        }
        Ok(())
    }

    #[test]
    fn refuses_https_without_a_pinned_root() -> TestResult {
        let output = ctl(&[
            "read-state",
            "--rpc-url",
            "https://localhost",
            "--to",
            TARGET,
            "--data",
            "0x",
        ])?;
        assert_eq!(output.status.code(), Some(1));
        assert_eq!(
            String::from_utf8(output.stderr)?,
            "layerxctl: RPC endpoint: {\"transport_error\":\"Endpoint\"}\n"
        );
        Ok(())
    }

    #[test]
    fn help_describes_evm_inputs_and_unverified_submission() -> TestResult {
        let output = ctl(&["--help"])?;
        assert_eq!(output.status.code(), Some(0));
        let text = String::from_utf8(output.stdout)?;
        for option in [
            "--rpc-url",
            "--to",
            "--data",
            "--block-height",
            "--ca-cert",
            "--transaction",
        ] {
            assert!(text.contains(option));
        }
        assert!(text.contains("prints receipted only after the receipt signature verifies under a pinned receipt key and submitted otherwise"));
        assert!(!text.contains("--socket"));
        Ok(())
    }

    #[test]
    fn read_state_executes_pending_and_committed_evm_views() -> TestResult {
        let evm = Evm::start()?;
        let contract = evm.deploy(&unhex("0x60003560005260206000f3")?)?;
        let data = format!("0x{}2a", "00".repeat(31));
        for height in [None, Some("1")] {
            let mut arguments = vec![
                "read-state",
                "--rpc-url",
                &evm.url,
                "--to",
                &contract,
                "--data",
                &data,
            ];
            if let Some(height) = height {
                arguments.extend(["--block-height", height]);
            }
            let output = ctl(&arguments)?;
            assert_eq!(
                output.status.code(),
                Some(0),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
            assert_eq!(
                String::from_utf8(output.stdout)?,
                format!(
                    "{{\"block\":\"{}\",\"to\":\"{contract}\",\"data\":\"{data}\"}}\n",
                    if height.is_some() { "0x1" } else { "pending" }
                )
            );
        }
        Ok(())
    }

    #[test]
    fn submit_executes_real_signed_bytes_without_claiming_a_verified_rung() -> TestResult {
        let evm = Evm::start()?;
        let directory = Directory::new()?;
        let snapshot = evm.rpc("evm_snapshot", &[])?;
        let sender = evm.sender()?;
        let transaction = format!("{{\"from\":\"{sender}\",\"to\":\"{sender}\",\"gas\":\"0x5208\",\"value\":\"0x0\",\"type\":\"0x2\",\"maxFeePerGas\":\"0x77359400\",\"maxPriorityFeePerGas\":\"0x3b9aca00\"}}");
        let hash = evm.rpc("eth_sendTransaction", &[&transaction])?;
        let hash = hash.trim_matches('"');
        let raw = evm.rpc("eth_getRawTransactionByHash", &[hash])?;
        let raw = unhex(raw.trim_matches('"'))?;
        assert_eq!(raw.first(), Some(&2));
        assert_eq!(
            evm.rpc("evm_revert", &[snapshot.trim_matches('"')])?,
            "true"
        );
        let path = directory.0.join("transaction");
        fs::write(&path, &raw)?;
        let output = ctl(&[
            "submit",
            "--rpc-url",
            &evm.url,
            "--transaction",
            path.to_str().ok_or("path is not UTF-8")?,
        ])?;
        assert_eq!(
            output.status.code(),
            Some(3),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let text = String::from_utf8(output.stdout)?;
        assert_eq!(json_string(&text, "state")?, "submitted");
        assert_eq!(json_string(&text, "transaction_hash")?, hash);
        assert!(text.contains("\"receipt_error\":{\"rpc_code\":-32601,"));
        let receipt = evm.rpc("eth_getTransactionReceipt", &[hash])?;
        assert_eq!(json_string(&receipt, "status")?, "0x1");
        Ok(())
    }

    fn answer_send(
        listener: &TcpListener,
        method: &str,
        answer: &str,
    ) -> Result<String, Box<dyn Error>> {
        let (stream, _) = listener.accept()?;
        let mut reader = BufReader::new(stream);
        let mut length = 0_usize;
        loop {
            let mut line = String::new();
            reader.read_line(&mut line)?;
            if line == "\r\n" {
                break;
            }
            if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = value.trim().parse()?;
            }
        }
        let mut body = vec![0_u8; length];
        reader.read_exact(&mut body)?;
        let body = String::from_utf8(body)?;
        assert!(body.contains(&format!("\"method\":\"{method}\"")));
        let id: u64 = body
            .split_once("\"id\":")
            .ok_or("request has no id")?
            .1
            .split(|character: char| !character.is_ascii_digit())
            .next()
            .ok_or("request id is not a number")?
            .parse()?;
        let answer = answer
            .replace("{id}", &id.to_string())
            .replace("{other}", &(id + 1).to_string());
        let mut stream = reader.into_inner();
        write!(
            stream,
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
            answer.len()
        )?;
        stream.flush()?;
        Ok(body)
    }

    fn submit_against(answer: &'static str) -> Result<Output, Box<dyn Error>> {
        let directory = Directory::new()?;
        let path = directory.0.join("transaction");
        fs::write(&path, [0x02, 0xf8, 0x6b])?;
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
        let url = format!("http://{}/", listener.local_addr()?);
        let server = std::thread::spawn(move || {
            answer_send(&listener, "eth_sendRawTransaction", answer)
                .map_err(|error| error.to_string())
        });
        let output = ctl(&[
            "submit",
            "--rpc-url",
            &url,
            "--transaction",
            path.to_str().ok_or("path is not UTF-8")?,
        ])?;
        let body = server.join().map_err(|_| "server thread panicked")??;
        assert!(body.contains("\"params\":[\"0x02f86b\"]"));
        Ok(output)
    }

    fn transaction_hash(raw: &[u8]) -> String {
        format!(
            "0x{}",
            hex(&layerx_crypto::evm_transaction::transaction_hash(raw))
        )
    }

    fn encoded_receipt(
        transaction: [u8; 32],
        key_id: [u8; 32],
        height: [u8; 32],
        result: i32,
        signature: Option<[u8; 64]>,
    ) -> Vec<u8> {
        let mut encoder = Encoder::new(4096);
        assert_eq!(
            encoder.structure_header_version(0x5201, PROTOCOL_VERSION),
            Ok(())
        );
        assert_eq!(encoder.u16(PROTOCOL_VERSION), Ok(()));
        assert_eq!(encoder.bytes(&transaction, 32), Ok(()));
        assert_eq!(encoder.u64(7), Ok(()));
        assert_eq!(encoder.bytes(&[2; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[3; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[4; 32], 32), Ok(()));
        assert_eq!(encoder.i32(result), Ok(()));
        assert_eq!(encoder.sequence_length(0, 512), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&height, 32), Ok(()));
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.bytes(&[5; 32], 32), Ok(()));
        assert_eq!(encoder.u128(25), Ok(()));
        assert_eq!(encoder.bytes(&[6; 32], 32), Ok(()));
        assert_eq!(encoder.u128(100), Ok(()));
        assert_eq!(encoder.u128(75), Ok(()));
        assert_eq!(encoder.u64(1), Ok(()));
        assert_eq!(encoder.bytes(&[7; 32], 32), Ok(()));
        assert_eq!(encoder.u128(10), Ok(()));
        assert_eq!(encoder.u128(35), Ok(()));
        assert_eq!(encoder.bytes(&[8; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[9; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&key_id, 32), Ok(()));
        assert_eq!(encoder.u64(1_000), Ok(()));
        assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
        if let Some(signature) = signature {
            assert_eq!(encoder.bytes(&signature, 64), Ok(()));
        }
        encoder.finish()
    }

    fn signed_receipt(
        signer: &SigningKey,
        transaction: [u8; 32],
        key_id: [u8; 32],
        height: [u8; 32],
        result: i32,
    ) -> Result<Vec<u8>, Box<dyn Error>> {
        let unsigned = encoded_receipt(transaction, key_id, height, result, None);
        let digest =
            receipt_digest(&unsigned).map_err(|error| format!("receipt digest: {error:?}"))?;
        Ok(encoded_receipt(
            transaction,
            key_id,
            height,
            result,
            Some(signer.sign(&digest).to_bytes()),
        ))
    }

    fn pinned_key(key_id: [u8; 32], public_key: [u8; 32], first: u64, last: u64) -> String {
        format!("[{{\"keyId\":\"0x{}\",\"publicKey\":\"0x{}\",\"firstHeight\":\"{first:#x}\",\"lastHeight\":\"{last:#x}\"}}]", hex(&key_id), hex(&public_key))
    }

    fn submit_receipt(bytes: &[u8], keys: &str, committed: bool) -> Result<Output, Box<dyn Error>> {
        let directory = Directory::new()?;
        let path = directory.0.join("transaction");
        let raw = [0x02, 0xf8, 0x6b];
        fs::write(&path, raw)?;
        let hash = transaction_hash(&raw);
        let rung = if committed { "committed" } else { "receipted" };
        let landing = if committed {
            format!(
                ",\"blockNumber\":\"0x96\",\"blockHash\":\"0x{}\"",
                hex(&[10; 32])
            )
        } else {
            String::new()
        };
        let receipt = format!(
            "{{\"rung\":\"{rung}\",\"txHash\":\"{hash}\",\"nonce\":\"0x1\",\"seq\":\"0x7\",\"height\":\"0x96\",\"index\":\"0x0\",\"timestamp\":\"0x3e8\",\"status\":\"0x1\",\"returnData\":\"0x\",\"logs\":[],\"gasUsed\":\"0x5208\",\"cumulativeGasUsed\":\"0x5208\",\"effectiveGasPrice\":\"0x1\",\"feeDenom\":\"upax\",\"feeAmount\":\"0x5208\",\"sponsor\":\"0x{}\",\"from\":\"0x{}\",\"to\":\"0x{}\",\"contractAddress\":\"0x{}\",\"vmError\":\"\",\"keyId\":\"0x{}\",\"receipt\":\"0x{}\"{landing}}}",
            hex(&[0; 20]), hex(&[6; 20]), hex(&[7; 20]), hex(&[0; 20]), hex(&[11; 32]), hex(bytes)
        );
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
        let url = format!("http://{}/", listener.local_addr()?);
        let exchanges = [
            (
                "eth_sendRawTransaction",
                "[\"0x02f86b\"]".to_owned(),
                format!("\"{hash}\""),
            ),
            ("pax_sequencerReceipt", format!("[\"{hash}\"]"), receipt),
            ("px_getReceiptKeys", "[]".to_owned(), keys.to_owned()),
        ];
        let server = std::thread::spawn(move || -> Result<(), String> {
            for (method, params, result) in exchanges {
                let answer = format!("{{\"jsonrpc\":\"2.0\",\"id\":{{id}},\"result\":{result}}}");
                let body =
                    answer_send(&listener, method, &answer).map_err(|error| error.to_string())?;
                assert!(body.contains(&format!("\"params\":{params}")));
            }
            Ok(())
        });
        let output = ctl(&[
            "submit",
            "--rpc-url",
            &url,
            "--transaction",
            path.to_str().ok_or("path is not UTF-8")?,
        ])?;
        server.join().map_err(|_| "server thread panicked")??;
        assert!(
            output.stderr.is_empty(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        Ok(output)
    }

    #[test]
    fn submit_reports_receipted_only_with_a_pinned_key_signature() -> TestResult {
        let signer = SigningKey::from_bytes(&[0x47; 32]);
        let key_id = [11; 32];
        let transaction = layerx_crypto::evm_transaction::transaction_hash(&[0x02, 0xf8, 0x6b]);
        let mut height = [0; 32];
        height[24..].copy_from_slice(&150_u64.to_be_bytes());
        let bytes = signed_receipt(&signer, transaction, key_id, height, 0)?;
        for (first, last, committed) in [
            (100, 200, false),
            (150, 200, false),
            (100, 150, false),
            (150, 150, true),
        ] {
            let keys = pinned_key(key_id, signer.verifying_key().to_bytes(), first, last);
            let output = submit_receipt(&bytes, &keys, committed)?;
            assert_eq!(output.status.code(), Some(0));
            assert_eq!(String::from_utf8(output.stdout)?, format!("{{\"state\":\"receipted\",\"transaction_hash\":\"0x{}\",\"code\":\"LXP_OK\",\"result_code\":0,\"class\":\"terminal\"}}\n", hex(&transaction)));
        }
        for keys in [
            "[]".to_owned(),
            pinned_key([12; 32], signer.verifying_key().to_bytes(), 100, 200),
            pinned_key(
                key_id,
                SigningKey::from_bytes(&[0x49; 32])
                    .verifying_key()
                    .to_bytes(),
                100,
                200,
            ),
            pinned_key(key_id, signer.verifying_key().to_bytes(), 151, 200),
            pinned_key(key_id, signer.verifying_key().to_bytes(), 100, 149),
        ] {
            let output = submit_receipt(&bytes, &keys, false)?;
            assert_eq!(output.status.code(), Some(3));
            assert_eq!(String::from_utf8(output.stdout)?, format!("{{\"state\":\"submitted\",\"transaction_hash\":\"0x{}\",\"verification_failure\":\"SequencerSignature\"}}\n", hex(&transaction)));
        }
        for (result, code, class) in [
            (-400, "LXP_INSUFFICIENT_BALANCE", "retriable"),
            (-300, "LXP_SEQUENCE_GAP", "terminal"),
        ] {
            let bytes = signed_receipt(&signer, transaction, key_id, height, result)?;
            let keys = pinned_key(key_id, signer.verifying_key().to_bytes(), 100, 200);
            let output = submit_receipt(&bytes, &keys, false)?;
            assert_eq!(output.status.code(), Some(1));
            assert_eq!(String::from_utf8(output.stdout)?, format!("{{\"state\":\"receipted\",\"transaction_hash\":\"0x{}\",\"code\":\"{code}\",\"result_code\":{result},\"class\":\"{class}\"}}\n", hex(&transaction)));
        }
        Ok(())
    }

    #[test]
    fn submit_names_the_failing_verification_step() -> TestResult {
        let signer = SigningKey::from_bytes(&[0x48; 32]);
        let key_id = [11; 32];
        let keys = pinned_key(key_id, signer.verifying_key().to_bytes(), 100, 200);
        let transaction = layerx_crypto::evm_transaction::transaction_hash(&[0x02, 0xf8, 0x6b]);
        let mut height = [0; 32];
        height[24..].copy_from_slice(&150_u64.to_be_bytes());
        let mut tampered = signed_receipt(&signer, transaction, key_id, height, 0)?;
        let last = tampered.last_mut().ok_or("empty signed receipt")?;
        *last ^= 1;
        let mut invalid_height = height;
        invalid_height[0] = 1;
        for (bytes, check) in [
            (vec![0xff], "Decode"),
            (
                encoded_receipt(transaction, key_id, height, 0, None),
                "MissingSignature",
            ),
            (tampered, "SequencerSignature"),
            (
                signed_receipt(&signer, transaction, key_id, invalid_height, 0)?,
                "BatchId",
            ),
            (
                signed_receipt(&signer, [0; 32], key_id, height, 0)?,
                "ActivityId",
            ),
            (
                signed_receipt(&signer, [13; 32], key_id, height, 0)?,
                "ActivityId",
            ),
        ] {
            let output = submit_receipt(&bytes, &keys, false)?;
            assert_eq!(output.status.code(), Some(3), "{check}");
            assert_eq!(String::from_utf8(output.stdout)?, format!("{{\"state\":\"submitted\",\"transaction_hash\":\"0x{}\",\"verification_failure\":\"{check}\"}}\n", hex(&transaction)));
        }
        Ok(())
    }

    #[test]
    fn unusable_send_answer_stays_submitted_unknown() -> TestResult {
        let local = transaction_hash(&[0x02, 0xf8, 0x6b]);
        for answer in [
            "{}",
            "{\"jsonrpc\":\"2.0\",\"id\":{other},\"result\":\"0x8f1c4b2a9e7d3c5b1a0f2e4d6c8b0a1928374655647382910a1b2c3d4e5f6071\"}",
            "{\"jsonrpc\":\"2.0\",\"id\":{id},\"result\":\"0x\"}",
        ] {
            let output = submit_against(answer)?;
            assert_eq!(
                output.status.code(),
                Some(3),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
            assert_eq!(
                String::from_utf8(output.stdout)?,
                format!("{{\"state\":\"submitted\",\"outcome\":\"unknown\",\"transaction_hash\":\"{local}\"}}\n")
            );
            assert!(output.stderr.is_empty());
        }
        Ok(())
    }

    #[test]
    fn submit_refuses_a_node_hash_that_differs_from_the_raw_bytes() -> TestResult {
        let local = transaction_hash(&[0x02, 0xf8, 0x6b]);
        let node = "0x8f1c4b2a9e7d3c5b1a0f2e4d6c8b0a1928374655647382910a1b2c3d4e5f6071";
        assert_ne!(local, node);
        let output = submit_against(
            "{\"jsonrpc\":\"2.0\",\"id\":{id},\"result\":\"0x8f1c4b2a9e7d3c5b1a0f2e4d6c8b0a1928374655647382910a1b2c3d4e5f6071\"}",
        )?;
        assert_eq!(output.status.code(), Some(1));
        assert!(output.stdout.is_empty());
        assert_eq!(
            String::from_utf8(output.stderr)?,
            format!("layerxctl: submission refused: transaction hash mismatch (local transaction hash: {local})\n")
        );
        Ok(())
    }

    #[test]
    fn definitive_send_refusal_stays_refused() -> TestResult {
        let output = submit_against(
            "{\"jsonrpc\":\"2.0\",\"id\":{id},\"error\":{\"code\":-32000,\"message\":\"nonce too low\"}}",
        )?;
        assert_eq!(output.status.code(), Some(1));
        assert!(output.stdout.is_empty());
        assert_eq!(
            String::from_utf8(output.stderr)?,
            "layerxctl: submission refused: {\"rpc_code\":-32000,\"message\":\"nonce too low\",\"data\":null}\n"
        );
        Ok(())
    }

    #[test]
    fn rejects_unsigned_text_and_wrong_transaction_type() -> TestResult {
        let directory = Directory::new()?;
        let path = directory.0.join("transaction");
        for bytes in [b"0x02".as_slice(), &[1, 0xc0], &[4, 0xc0]] {
            fs::write(&path, bytes)?;
            let output = bounded_file_refusal(&path, false)?;
            assert_eq!(
                String::from_utf8(output.stderr)?,
                "layerxctl: transaction must contain binary signed EIP-1559 (type 2) bytes\n"
            );
        }
        Ok(())
    }

    #[test]
    fn every_kernel_revert_reports_its_exact_registry_code_and_class() -> TestResult {
        let header = include_str!("../../../include/layerx/lxp_result.h");
        let registry = include_str!("../../../spec/layerx-merge/contracts/address-table.md");
        let mut checked = 0;
        for entry in header
            .lines()
            .filter_map(|line| line.trim().strip_prefix("X("))
        {
            let Some((name, tail)) = entry.split_once(',') else {
                continue;
            };
            let Some(wire) = name.trim().strip_prefix("LXP_ERR_") else {
                continue;
            };
            let raw = tail
                .split(')')
                .next()
                .ok_or("header code has no value")?
                .trim()
                .parse::<i32>()?;
            let code = format!("LXP_{wire}");
            let prefix = format!("| {code} |");
            let row = registry
                .lines()
                .find(|line| line.starts_with(&prefix))
                .ok_or_else(|| format!("registry has no row for {code}"))?;
            let retryable = row
                .trim_end_matches('|')
                .rsplit('|')
                .next()
                .ok_or("registry has no retryability")?
                .trim();
            let class = match retryable {
                "yes" => "retriable",
                "no" => "terminal",
                _ => return Err("invalid registry class".into()),
            };
            let reason = format!("{code}: registry message: detail");
            assert_eq!(
                KnownResult::from_revert_reason(&reason)
                    .ok_or_else(|| format!("mirror has no {code}"))?
                    .raw(),
                raw
            );
            assert_eq!(super::revert_error(&abi_reason(reason.as_bytes())), format!("{{\"code\":\"{code}\",\"result_code\":{raw},\"class\":\"{class}\",\"reason\":\"{reason}\"}}"));
            checked += 1;
        }
        assert_eq!(
            checked,
            KnownResult::ALL
                .iter()
                .filter(|result| result.raw() < 0 && result.raw() > -1000)
                .count()
        );
        assert!(checked > 0);
        Ok(())
    }

    #[test]
    fn unknown_and_malformed_reverts_stay_terminal() {
        let reason = b"LXP_SEQUENCE_GAP: sequence gap";
        let canonical = abi_reason(reason);
        let mut malformed = vec![
            Vec::new(),
            canonical[..67].to_vec(),
            canonical[..canonical.len() - 1].to_vec(),
        ];
        for (index, value) in [
            (0, 0),
            (4, 1),
            (35, 64),
            (36, 1),
            (67, 255),
            (68, 255),
            (canonical.len() - 1, 1),
        ] {
            let mut bytes = canonical.clone();
            bytes[index] = value;
            malformed.push(bytes);
        }
        let mut trailing = canonical.clone();
        trailing.push(0);
        malformed.push(trailing);
        for data in malformed {
            assert_eq!(super::revert_error(&data), UNKNOWN_REVERT);
        }
        for (reason, code) in [
            ("LXP_FUTURE_CODE: future code", "LXP_FUTURE_CODE"),
            ("LXP_OK: success", "LXP_OK"),
            ("LXP_FATAL_STATE_ROOT: fatal", "LXP_FATAL_STATE_ROOT"),
            ("LXP_SEQUENCE_GAP", "LXP_UNKNOWN"),
            ("untyped refusal", "LXP_UNKNOWN"),
        ] {
            assert_eq!(super::revert_error(&abi_reason(reason.as_bytes())), format!("{{\"code\":\"{code}\",\"result_code\":null,\"class\":\"terminal\",\"reason\":\"{reason}\"}}"));
        }
    }

    #[test]
    fn real_evm_reverts_use_the_same_refusal_mapping() -> TestResult {
        let evm = Evm::start()?;
        for (reason, code, result, class) in [
            (
                "LXP_PROJECTION_STALE: projection stale",
                "LXP_PROJECTION_STALE",
                "-903",
                "retriable",
            ),
            (
                "LXP_NON_CANONICAL: non canonical",
                "LXP_NON_CANONICAL",
                "-3",
                "terminal",
            ),
            (
                "LXP_FUTURE_CODE: future code",
                "LXP_FUTURE_CODE",
                "null",
                "terminal",
            ),
        ] {
            let contract = evm.deploy(&revert_runtime(&abi_reason(reason.as_bytes()))?)?;
            let arguments = [
                "read-state",
                "--rpc-url",
                &evm.url,
                "--to",
                &contract,
                "--data",
                "0x",
            ]
            .map(str::to_owned);
            let error = super::run(&arguments)
                .err()
                .ok_or("reverting view was accepted")?;
            let expected = format!("\"revert\":{{\"code\":\"{code}\",\"result_code\":{result},\"class\":\"{class}\",\"reason\":\"{reason}\"}}");
            assert!(error.contains(&expected), "{error}");
        }
        Ok(())
    }
}
