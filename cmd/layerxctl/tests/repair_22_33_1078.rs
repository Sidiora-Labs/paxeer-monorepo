include!("../src/main.rs");

#[path = "support/receipt_cases.rs"]
mod receipt_cases;

mod contract {
    use std::error::Error;
    use std::fs;
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::{Ipv4Addr, TcpListener};
    use std::os::unix::fs::DirBuilderExt;
    use std::path::PathBuf;
    use std::process::ExitCode;
    use std::time::{Duration, Instant};

    type TestResult = Result<(), Box<dyn Error>>;
    const TARGET: &str = "0x0000000000000000000000000000000000000123";

    fn arguments(values: &[&str]) -> Vec<String> {
        values.iter().map(|value| (*value).to_owned()).collect()
    }

    #[test]
    fn f032_read_state_uses_pending_or_explicit_height() -> TestResult {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
        listener.set_nonblocking(true)?;
        let url = format!("http://{}/", listener.local_addr()?);
        let server = std::thread::spawn(move || -> Result<(), String> {
            for block in ["pending", "0x2a"] {
                let deadline = Instant::now() + Duration::from_secs(5);
                let stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            if Instant::now() >= deadline {
                                return Err(format!("timed out waiting for {block} read"));
                            }
                            std::thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => return Err(error.to_string()),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .map_err(|e| e.to_string())?;
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .map_err(|e| e.to_string())?;
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    if reader.read_line(&mut line).map_err(|e| e.to_string())? == 0 {
                        return Err("truncated request".to_owned());
                    }
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().map_err(|e| e.to_string())?);
                    }
                }
                let length = length.ok_or("missing content length")?;
                assert!(length <= 4096);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).map_err(|e| e.to_string())?;
                let body = String::from_utf8(body).map_err(|e| e.to_string())?;
                assert!(body.contains("\"method\":\"eth_call\""), "{body}");
                assert!(body.contains(&format!("\"to\":\"{TARGET}\"")), "{body}");
                assert!(body.contains("\"data\":\"0xabcd\""), "{body}");
                assert!(body.contains(&format!(",\"{block}\"]")), "{body}");
                let id = body
                    .split_once("\"id\":")
                    .ok_or("missing id")?
                    .1
                    .split(|c: char| !c.is_ascii_digit())
                    .next()
                    .ok_or("invalid id")?
                    .parse::<u64>()
                    .map_err(|e| e.to_string())?;
                let answer = format!("{{\"jsonrpc\":\"2.0\",\"id\":{id},\"result\":\"0x0102\"}}");
                write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()).map_err(|e| e.to_string())?;
                reader.get_mut().flush().map_err(|e| e.to_string())?;
            }
            Ok(())
        });
        let result = (|| -> TestResult {
            for (height, block) in [(None, "pending"), (Some("42"), "0x2a")] {
                let mut args = arguments(&[
                    "read-state",
                    "--rpc-url",
                    &url,
                    "--to",
                    TARGET,
                    "--data",
                    "0xabcd",
                ]);
                if let Some(height) = height {
                    args.extend(arguments(&["--block-height", height]));
                }
                let (output, status) = crate::run(&args)?;
                assert_eq!(status, ExitCode::SUCCESS);
                assert_eq!(
                    output,
                    format!("{{\"block\":\"{block}\",\"to\":\"{TARGET}\",\"data\":\"0x0102\"}}")
                );
            }
            Ok(())
        })();
        server.join().map_err(|_| "read server panicked")??;
        result
    }

    struct Directory(PathBuf);

    impl Drop for Directory {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    #[test]
    fn f032_local_bounds_and_supported_commands() -> TestResult {
        let directory = Directory(
            std::env::temp_dir().join(format!("layerxctl-contract-{}", std::process::id())),
        );
        fs::DirBuilder::new().mode(0o700).create(&directory.0)?;
        let path = directory.0.join("input");
        let path_text = path.to_str().ok_or("non-UTF-8 path")?;
        let file = fs::File::create(&path)?;
        assert_eq!(crate::MAX_INPUT_BYTES, 1_048_576);
        assert_eq!(crate::MAX_CERTIFICATE_BYTES, 65_536);
        for maximum in [crate::MAX_INPUT_BYTES, crate::MAX_CERTIFICATE_BYTES] {
            file.set_len(maximum as u64)?;
            assert_eq!(
                crate::input_file(path_text, "input", maximum)?.len(),
                maximum
            );
            file.set_len(maximum as u64 + 1)?;
            assert_eq!(
                crate::input_file(path_text, "input", maximum)
                    .err()
                    .ok_or("oversized input accepted")?,
                format!("input must be a nonempty regular file within the {maximum}-byte bound")
            );
        }
        file.set_len(crate::MAX_INPUT_BYTES as u64 + 1)?;
        let error = crate::run(&arguments(&[
            "submit",
            "--rpc-url",
            "invalid-url",
            "--transaction",
            path_text,
        ]))
        .err()
        .ok_or("oversized transaction accepted")?;
        assert_eq!(
            error,
            "transaction must be a nonempty regular file within the 1048576-byte bound"
        );
        let (help, status) = crate::run(&arguments(&["--help"]))?;
        assert_eq!(status, ExitCode::SUCCESS);
        assert!(help.contains("read-state --rpc-url URL --to HEX40 --data HEX"));
        assert!(help.contains("submit --rpc-url URL --transaction FILE"));
        for (args, expected) in [
            (
                vec!["submit", "--block-height", "42"],
                "unknown option --block-height",
            ),
            (
                vec!["read-state", "--transaction", path_text],
                "unknown option --transaction",
            ),
            (
                vec![
                    "read-state",
                    "--rpc-url",
                    "http://localhost",
                    "--rpc-url",
                    "http://localhost",
                ],
                "empty or duplicate option --rpc-url",
            ),
        ] {
            assert_eq!(
                crate::run(&arguments(&args))
                    .err()
                    .ok_or("unsupported options accepted")?,
                expected
            );
        }
        assert_eq!(
            crate::run(&arguments(&["unknown-command"]))
                .err()
                .ok_or("unknown command accepted")?,
            help
        );
        Ok(())
    }
}
