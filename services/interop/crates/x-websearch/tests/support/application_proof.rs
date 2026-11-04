use std::collections::BTreeMap;
use std::error::Error;
use std::io::{Read, Write};
use std::path::PathBuf;
use std::sync::Arc;

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::{
    network_word, recover_signer, Answer, Attestor, AttestorSet, ProgramAttestError, Ready,
};
use x_websearch::config::FetchLimits;
use x_websearch::content::ContentStore;
use x_websearch::fetch::Fetcher;
use x_websearch::index::WebIndex;
use x_websearch::program::{observation_bytes, ProgramRequest};
use x_websearch::watch::{hex0x, unhex0x, EvmError, EvmRpc};
use x_websearch::{Limits, RouteTable, Server};

type Outcome<T = ()> = Result<T, Box<dyn Error>>;
const CHAIN_ID: u64 = 713_714;

fn fail(message: impl Into<String>) -> Box<dyn Error> {
    message.into().into()
}

fn text<'a>(value: &'a Value, pointer: &str) -> Outcome<&'a str> {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .ok_or_else(|| fail(pointer))
}

fn fixed<const N: usize>(value: &Value, pointer: &str) -> Outcome<[u8; N]> {
    unhex0x(text(value, pointer)?)
        .and_then(|bytes| bytes.try_into().ok())
        .ok_or_else(|| fail(format!("{pointer} is not {N} bytes")))
}

fn attestor_key(index: u8) -> Outcome<SigningKey> {
    let mut secret = [0_u8; 32];
    secret[0] = 0x3c;
    secret[31] = index;
    Ok(SigningKey::from_slice(&secret)?)
}

struct Scratch(PathBuf);
impl Scratch {
    fn new(name: &str) -> Outcome<Self> {
        let path = std::env::temp_dir().join(format!(
            "xweb-application-proof-{}-{name}",
            std::process::id()
        ));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn loopback_fetcher() -> Outcome<Fetcher> {
    Ok(Fetcher::new(FetchLimits {
        connect_timeout_ms: 3_000,
        total_timeout_ms: 10_000,
        max_body_bytes: 2_097_152,
        max_redirects: 3,
        allow_loopback: true,
    })?)
}

// This entrypoint is owned by the live Go application producer. It never
// selects a replay, discovers an endpoint or reads ambient process stdin.
pub fn xweb_0x1019_application_proof(record: &Value) -> Outcome<Value> {
    let endpoint = text(record, "/rpc")?;
    let request = ProgramRequest {
        program_id: fixed(record, "/program_id")?,
        sequence: record["sequence"]
            .as_u64()
            .ok_or_else(|| fail("sequence missing"))?,
        request_id: record["request_id"]
            .as_u64()
            .ok_or_else(|| fail("request id missing"))?,
        kind: u8::try_from(
            record["kind"]
                .as_u64()
                .ok_or_else(|| fail("kind missing"))?,
        )?,
        payload: unhex0x(text(record, "/payload")?).ok_or_else(|| fail("payload is not hex"))?,
    };
    let scratch = Scratch::new("application-proof")?;
    let rpc = EvmRpc::new(&endpoint)?;
    let mut signatures = BTreeMap::new();
    let mut answers = Vec::new();
    for index in 1..=3 {
        let engine = Attestor::new(
            attestor_key(index)?,
            CHAIN_ID,
            Arc::new(loopback_fetcher()?),
            Arc::new(WebIndex::open(&scratch.0.join(format!("index-{index}")))?),
            Arc::new(ContentStore::open(
                &scratch.0.join(format!("store-{index}")),
                &[],
            )?),
        );
        let answer = Answer::try_from((&engine, &rpc, &request))?;
        let again = Answer::try_from((&engine, &rpc, &request))?;
        assert_eq!(answer, again);
        assert_eq!(answer.attestation.network_id, network_word(1));
        assert_eq!(answer.response, b"[]");
        assert_eq!(
            recover_signer(&answer.digest, &answer.signature)?,
            engine.signer()
        );
        let malformed = EvmRpc::new(&format!("{endpoint}/zero"))?;
        // watch.rs::kernel_network_id currently admits zero; attestation must refuse it.
        assert_eq!(x_websearch::watch::kernel_network_id(&malformed)?, 0);
        assert_eq!(
            Answer::try_from((&engine, &malformed, &request)),
            Err(ProgramAttestError::Network(EvmError::Malformed))
        );
        signatures.insert(answer.signer, answer.signature);
        answers.push(answer);
    }
    let first = answers.first().ok_or_else(|| fail("no answers"))?;
    assert!(answers.iter().all(|answer| answer.digest == first.digest));
    let set = AttestorSet {
        signers: signatures.keys().copied().collect(),
        threshold: 2,
    };
    let ready = Ready::from_signatures(first, &set, &signatures)
        .ok_or_else(|| fail("real signatures did not reach threshold"))?;
    let observation = observation_bytes(1, &request, &ready)?;
    Ok(json!({
        "observation": hex0x(&observation), "response": hex0x(&first.response),
        "content_digest": hex0x(&first.attestation.content_digest),
        "full_length": first.attestation.full_length,
    }))
}

pub fn run(input: impl Read, mut output: impl Write) -> Outcome {
    let record = serde_json::from_reader(input)?;
    let answer = xweb_0x1019_application_proof(&record)?;
    writeln!(
        output,
        "XWEB_PROOF_ANSWER={}",
        serde_json::to_string(&answer)?
    )?;
    Ok(())
}

pub fn explicit_input_and_transport_refusal() -> Outcome {
    let mut output = Vec::new();
    let empty = run(&b""[..], &mut output)
        .err()
        .ok_or("empty producer input accepted")?;
    assert!(empty
        .downcast_ref::<serde_json::Error>()
        .ok_or("JSON input error")?
        .is_eof());
    assert!(output.is_empty());
    let server =
        Server::bind("127.0.0.1:0".parse()?, Limits::default(), RouteTable::new())?.spawn()?;
    let input = json!({
        "rpc": format!("http://{}/health", server.local_addr()),
        "program_id": hex0x(&[0xa0; 32]), "sequence": 41, "request_id": 7,
        "kind": 2, "payload": hex0x(b"paxeer"),
    });
    // The actual GET-only sidecar refuses the RPC POST. This proves the
    // entrypoint propagates EvmRpc's exact HTTP-refusal classification. It
    // cannot turn an unavailable RPC into a successful EVM proof.
    let refused = run(serde_json::to_vec(&input)?.as_slice(), &mut output)
        .err()
        .ok_or("HTTP method refusal was accepted as an application answer")?;
    assert_eq!(
        refused.downcast_ref::<ProgramAttestError>(),
        Some(&ProgramAttestError::Network(EvmError::Unavailable))
    );
    assert!(output.is_empty());
    server.shutdown()?;
    Ok(())
}
