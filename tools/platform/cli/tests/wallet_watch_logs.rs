use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::net::TcpListener;
use std::num::NonZeroU32;
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _};
use std::process::{Command, Output};
use std::thread;
use std::time::{Duration, Instant};

use layerx_platform_cli::rpc::{decode_response, request, RpcClient};
use ring::{aead, pbkdf2};
use serde_json::{json, Value};
use tungstenite::handshake::server::{Request, Response};
use tungstenite::Message;

const ACCOUNT: &str = "0x1234567890123456789012345678901234567890";
const SUBSCRIPTION: &str = "0x123abc";
const PASSPHRASE: &str = "wallet watch integration test passphrase";

fn log() -> Value {
    json!({
        "address": ACCOUNT,
        "topics": [format!("0x{}", "ab".repeat(32))],
        "data": "0x0102",
        "transactionHash": format!("0x{}", "cd".repeat(32)),
        "transactionIndex": "0x0",
        "blockHash": format!("0x{}", "ef".repeat(32)),
        "blockNumber": "0x2a",
        "logIndex": "0x0",
        "removed": false
    })
}

fn credential_vault(
    root: &std::path::Path,
    credential: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let directory = root.join("credentials");
    fs::DirBuilder::new().mode(0o700).create(&directory)?;
    let mut salt = [0; 16];
    let mut nonce = [0; 12];
    getrandom::fill(&mut salt)?;
    getrandom::fill(&mut nonce)?;
    let mut key = zeroize::Zeroizing::new([0; 32]);
    pbkdf2::derive(
        pbkdf2::PBKDF2_HMAC_SHA256,
        NonZeroU32::new(600_000).ok_or("invalid KDF iterations")?,
        &salt,
        PASSPHRASE.as_bytes(),
        key.as_mut(),
    );
    let key = aead::LessSafeKey::new(
        aead::UnboundKey::new(&aead::AES_256_GCM, key.as_ref()).map_err(|_| "invalid vault key")?,
    );
    let mut ciphertext = serde_json::to_vec(&json!({"gateway:watch": credential}))?;
    key.seal_in_place_append_tag(
        aead::Nonce::assume_unique_for_key(nonce),
        aead::Aad::from(b"LXVAULT1"),
        &mut ciphertext,
    )
    .map_err(|_| "vault encryption failed")?;
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(directory.join("vault"))?;
    for bytes in [b"LXVAULT1".as_slice(), &salt, &nonce, &ciphertext] {
        file.write_all(bytes)?;
    }
    Ok(())
}

fn watch(method: &'static str) -> Result<(Output, Value), Box<dyn std::error::Error>> {
    let root = tempfile::tempdir()?;
    let credential = format!("watch:lxp_live_{}", "a".repeat(64));
    credential_vault(root.path(), &credential)?;
    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let url = format!("http://{}/rpc", listener.local_addr()?);
    let server = thread::spawn(move || -> Result<Value, String> {
        let deadline = Instant::now() + Duration::from_secs(10);
        let stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    if Instant::now() >= deadline {
                        return Err("wallet watch did not connect".into());
                    }
                    thread::sleep(Duration::from_millis(10));
                }
                Err(error) => return Err(error.to_string()),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .map_err(|error| error.to_string())?;
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .map_err(|error| error.to_string())?;
        let mut socket =
            tungstenite::accept_hdr(stream, |upgrade: &Request, response: Response| {
                assert_eq!(upgrade.uri().path(), "/rpc/ws");
                assert_eq!(
                    upgrade.headers()["Authorization"],
                    format!("LayerX-Key {credential}")
                );
                Ok(response)
            })
            .map_err(|error| error.to_string())?;
        let message = socket.read().map_err(|error| error.to_string())?;
        let Message::Text(text) = message else {
            return Err("subscription request was not a JSON text frame".into());
        };
        let command: Value = serde_json::from_str(&text).map_err(|error| error.to_string())?;
        socket
            .send(Message::Text(
                json!({"jsonrpc":"2.0","id":command["id"],"result":SUBSCRIPTION})
                    .to_string()
                    .into(),
            ))
            .map_err(|error| error.to_string())?;
        socket
            .send(Message::Text(
                json!({"jsonrpc":"2.0","method":method,"params":{
                    "subscription":SUBSCRIPTION,"result":log()
                }})
                .to_string()
                .into(),
            ))
            .map_err(|error| error.to_string())?;
        Ok(command)
    });
    let output = Command::new(env!("CARGO_BIN_EXE_layerx"))
        .env_clear()
        .env("LAYERX_CONFIG", root.path().join("config.json"))
        .env("LAYERX_CREDENTIAL_STORE", "file")
        .env("LAYERX_CREDENTIAL_PASSPHRASE", PASSPHRASE)
        .args([
            "--json",
            "--rpc",
            &url,
            "--gateway-credential",
            "watch",
            "wallet",
            "watch",
            "account",
            "--account-id",
            ACCOUNT,
            "--timeout-seconds",
            "2",
        ])
        .output()?;
    let command = server.join().map_err(|_| "WebSocket server panicked")??;
    assert_eq!(
        command,
        json!({"jsonrpc":"2.0","id":1,"method":"eth_subscribe",
            "params":["logs",{"address":ACCOUNT,"topics":[]}]})
    );
    Ok((output, command))
}

#[test]
fn wallet_watch_subscribes_to_logs() -> Result<(), Box<dyn std::error::Error>> {
    let params = json!(["logs", {"address":ACCOUNT,"topics":[]}]);
    assert_eq!(request("eth_subscribe", &params)?["params"], params);
    let topic = format!("0x{}", "ab".repeat(32));
    request(
        "eth_subscribe",
        &json!(["logs", {"address":[ACCOUNT],"topics":[null,topic,[topic]]}]),
    )?;
    for invalid in [
        json!(["newHeads", {"address":ACCOUNT}]),
        json!(["logs"]),
        json!(["logs", {}]),
        json!(["logs", {"address":"12".repeat(32)}]),
        json!(["logs", {"address":[]}]),
        json!(["logs", {"address":ACCOUNT,"topics":["0x12"]}]),
        json!(["logs", {"address":ACCOUNT,"topics":[null,null,null,null,null]}]),
        json!(["logs", {"address":ACCOUNT,"topics":[[]]}]),
        json!(["logs", {"address":ACCOUNT,"topics":"invalid"}]),
        json!(["logs", {"address":ACCOUNT,"fromBlock":"latest"}]),
    ] {
        assert!(request("eth_subscribe", &invalid).is_err(), "{invalid}");
    }
    request("eth_unsubscribe", &json!([SUBSCRIPTION]))?;
    for invalid in [
        json!([]),
        json!([""]),
        json!([1]),
        json!(["x".repeat(129)]),
        json!([SUBSCRIPTION, SUBSCRIPTION]),
    ] {
        assert!(request("eth_unsubscribe", &invalid).is_err(), "{invalid}");
    }
    assert_eq!(
        decode_response(
            "eth_unsubscribe",
            &json!({"jsonrpc":"2.0","id":1,"result":true})
        )?,
        true
    );
    assert!(decode_response(
        "eth_unsubscribe",
        &json!({"jsonrpc":"2.0","id":1,"result":false})
    )
    .is_err());
    let client = RpcClient::new("http://127.0.0.1:1/rpc", None)?;
    for (method, params) in [
        ("eth_subscribe", params),
        ("eth_unsubscribe", json!([SUBSCRIPTION])),
    ] {
        let error = client
            .call(method, &params)
            .err()
            .ok_or("HTTP accepted a subscription method")?;
        assert!(error.starts_with("rpc_transport_required:"), "{error}");
    }
    let (output, _) = watch("eth_subscription")?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let output: Value = serde_json::from_slice(&output.stdout)?;
    assert_eq!(output["ok"], true);
    assert_eq!(output["kind"], "wallet.notification");
    assert_eq!(output["data"]["subscription"], SUBSCRIPTION);
    assert_eq!(output["data"]["verified"], false);
    assert_eq!(output["data"]["notification"], log());
    assert_eq!(
        output["data"]["notification"]["transactionHash"],
        log()["transactionHash"]
    );
    Ok(())
}

#[test]
fn wallet_watch_refuses_lx_subscription_notifications() -> Result<(), Box<dyn std::error::Error>> {
    let (output, _) = watch("lx_subscription")?;
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    let error: Value = serde_json::from_slice(&output.stderr)?;
    assert_eq!(error["ok"], false);
    assert_eq!(
        error["error"]["detail"],
        "unexpected subscription notification method"
    );
    assert!(!String::from_utf8_lossy(&output.stderr)
        .contains(log()["transactionHash"].as_str().ok_or("hash missing")?));
    Ok(())
}
