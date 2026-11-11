use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::SigningKey;
use layerx_oracle_feeder::{
    hex_decode, hex_encode, Sequencer, ADDR_PRECOMPILE, GET_EVM_ADDR_BY_LAYERX_SELECTOR,
};
use serde_json::{json, Value};

#[test]
fn finding_030_ascii_hex_pairs_are_validated_explicitly() -> Result<(), String> {
    let digits = b"0123456789abcdefABCDEF";
    for high in 0u8..=127 {
        for low in 0u8..=127 {
            let pair = [high, low];
            let text = std::str::from_utf8(&pair).map_err(|error| error.to_string())?;
            if digits.contains(&high) && digits.contains(&low) {
                assert_eq!(
                    hex_decode(text),
                    Ok(vec![
                        u8::from_str_radix(text, 16).map_err(|error| error.to_string())?
                    ]),
                    "{pair:?}"
                );
            } else {
                assert_eq!(hex_decode(text), Err("invalid hex".to_owned()), "{pair:?}");
            }
        }
    }
    assert_eq!(hex_decode(""), Ok(vec![]));
    for text in ["0", "abc", "01234"] {
        assert_eq!(hex_decode(text), Err("odd hex length".to_owned()));
    }
    let bytes: Vec<u8> = (0..=255).collect();
    assert_eq!(hex_decode(&hex_encode(&bytes)), Ok(bytes.clone()));
    assert_eq!(hex_decode(&hex_encode(&bytes).to_uppercase()), Ok(bytes));
    Ok(())
}

#[test]
fn finding_030_non_ascii_hex_returns_errors_without_panicking() {
    for text in [
        "é", "0é0", "€0", "0€", "💰", "00💰", "１２", "00é00", "00€0", "éé",
    ] {
        assert_eq!(hex_decode(text), Err("invalid hex".to_owned()), "{text:?}");
    }
    for text in ["€", "0é", "é0", "0💰", "00€"] {
        assert_eq!(
            hex_decode(text),
            Err("odd hex length".to_owned()),
            "{text:?}"
        );
    }
}

fn read_rpc_request(stream: &mut TcpStream) -> Result<Value, String> {
    stream
        .set_read_timeout(Some(Duration::from_secs(3)))
        .map_err(|error| error.to_string())?;
    let mut bytes = Vec::new();
    let mut buffer = [0; 4096];
    loop {
        let count = stream
            .read(&mut buffer)
            .map_err(|error| error.to_string())?;
        assert_ne!(count, 0, "truncated HTTP request");
        bytes.extend_from_slice(&buffer[..count]);
        assert!(bytes.len() <= 65_536, "oversized HTTP request");
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let headers = std::str::from_utf8(&bytes[..end]).map_err(|error| error.to_string())?;
            assert!(headers.starts_with("POST / HTTP/1.1\r\n"));
            let length = headers
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then_some(value.trim())
                })
                .ok_or("JSON-RPC content length")?
                .parse::<usize>()
                .map_err(|error| error.to_string())?;
            if bytes.len() >= end + 4 + length {
                return serde_json::from_slice(&bytes[end + 4..end + 4 + length])
                    .map_err(|error| error.to_string());
            }
        }
    }
}

#[test]
fn finding_030_rpc_sender_binding_rejects_unicode_and_continues() -> Result<(), String> {
    let key = SigningKey::from_bytes(&[7; 32]);
    let sender = [0xab; 20];
    let mut calldata = GET_EVM_ADDR_BY_LAYERX_SELECTOR.to_vec();
    calldata.extend_from_slice(&key.verifying_key().to_bytes());
    let expected_params = json!([{
        "to": format!("0x{}", hex_encode(&ADDR_PRECOMPILE)),
        "data": format!("0x{}", hex_encode(&calldata)),
    }, "latest"]);
    let malformed = [
        format!("0x0€{}", "00".repeat(30)),
        format!("0x{}0é0{}", "00".repeat(12), "ab".repeat(18)),
        format!("0x{}💰{}", "00".repeat(12), "ab".repeat(18)),
        format!("0x{}é{}", "00".repeat(12), "ab".repeat(19)),
    ];
    for word in &malformed {
        assert_eq!(word.len(), 66, "malformed fixture retains ABI word length");
    }
    let valid = format!("0x{}{}", "00".repeat(12), hex_encode(&sender));
    let mut answers = malformed.to_vec();
    answers.push(valid.clone());
    answers.push(valid.to_uppercase().replacen("0X", "0x", 1));
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    let url = format!(
        "http://{}",
        listener.local_addr().map_err(|error| error.to_string())?
    );
    listener
        .set_nonblocking(true)
        .map_err(|error| error.to_string())?;
    let server = thread::spawn(move || -> Result<(), String> {
        for result in answers {
            let deadline = Instant::now() + Duration::from_secs(5);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing sender-binding request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("fixture listener: {error}"),
                }
            };
            stream
                .set_write_timeout(Some(Duration::from_secs(3)))
                .map_err(|error| error.to_string())?;
            let request = read_rpc_request(&mut stream)?;
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], "eth_call");
            assert_eq!(request["params"], expected_params);
            let body = serde_json::to_vec(&json!({
                "jsonrpc": "2.0", "id": request["id"], "result": result,
            }))
            .map_err(|error| error.to_string())?;
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                body.len()
            )
            .map_err(|error| error.to_string())?;
            stream.write_all(&body).map_err(|error| error.to_string())?;
        }
        Ok(())
    });
    let agent = ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(3)))
        .max_redirects(0)
        .build()
        .into();
    let sequencer = Sequencer::new(agent, url)?;
    for word in &malformed {
        assert_eq!(
            sequencer.check_sender(&key, sender),
            Err("invalid hex".to_owned()),
            "RPC word {word:?}"
        );
    }
    assert_eq!(sequencer.check_sender(&key, sender), Ok(()));
    assert_eq!(sequencer.check_sender(&key, sender), Ok(()));
    server
        .join()
        .map_err(|_| "sender-binding server panicked")??;
    Ok(())
}
