use std::collections::BTreeMap;
use std::io::{self, Read};
use std::path::Path;

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::{
    sign_digest, signer_address, Answer, Attestation, AttestorSet, Level, Ready,
};
use x_websearch::program::observation_bytes;
use x_websearch::watch::{hex0x, keccak, unhex0x, EvmRpc, RequestWatcher};

fn number(input: &Value, field: &str) -> u64 {
    input[field].as_u64().unwrap()
}

fn open(input: &Value) -> RequestWatcher {
    RequestWatcher::open(
        EvmRpc::new(input["rpc"].as_str().unwrap()).unwrap(),
        0,
        Path::new(input["state"].as_str().unwrap()),
        Some(number(input, "block")),
    )
    .unwrap()
    .with_chain_id(number(input, "chain_id"))
    .unwrap()
}

fn ready(input: &Value, attestation: Attestation, callback_gas: u64, timeout: u64) -> Ready {
    let keys: Vec<_> = input["keys"]
        .as_array()
        .unwrap()
        .iter()
        .map(|key| SigningKey::from_slice(&unhex0x(key.as_str().unwrap()).unwrap()).unwrap())
        .collect();
    let digest = attestation.digest();
    let signatures: BTreeMap<_, _> = keys
        .iter()
        .map(|key| (signer_address(key), sign_digest(key, &digest).unwrap()))
        .collect();
    let answer = Answer {
        attestation,
        level: Level::Majority,
        response: unhex0x(input["response"].as_str().unwrap()).unwrap(),
        callback_gas,
        timeout_height: timeout,
        digest,
        signer: signer_address(&keys[0]),
        signature: sign_digest(&keys[0], &digest).unwrap(),
    };
    Ready::from_signatures(
        &answer,
        &AttestorSet {
            signers: input["signers"]
                .as_array()
                .unwrap()
                .iter()
                .map(|signer| {
                    unhex0x(signer.as_str().unwrap())
                        .unwrap()
                        .try_into()
                        .unwrap()
                })
                .collect(),
            threshold: 2,
        },
        &signatures,
    )
    .unwrap()
}

fn main() {
    let mut bytes = Vec::new();
    io::stdin().read_to_end(&mut bytes).unwrap();
    let input: Value = serde_json::from_slice(&bytes).unwrap();
    let mut watcher = open(&input);
    let expected = input["programs"].as_array().unwrap();
    let evm_count = usize::from(input["evm"].as_bool().unwrap());
    if input["mode"] == "retire" {
        assert_eq!(watcher.program_work().len(), expected.len());
        assert_eq!(watcher.work().len(), evm_count);
        for request in watcher.program_work() {
            watcher
                .retire_program(request.program_id, request.request_id)
                .unwrap();
        }
        for request in watcher.work() {
            watcher.retire(request.request_id).unwrap();
        }
        drop(watcher);
        let mut watcher = open(&input);
        assert!(watcher.program_work().is_empty());
        assert!(watcher.work().is_empty());
        assert_eq!(watcher.next_block(), Some(number(&input, "block") + 1));
        assert!(watcher.poll().unwrap().is_empty());
        println!("{}", json!({"retired": true}));
        return;
    }

    watcher.poll().unwrap();
    assert_eq!(watcher.program_work().len(), expected.len());
    assert_eq!(watcher.work().len(), evm_count);
    assert_eq!(watcher.next_block(), Some(number(&input, "block") + 1));
    let programs = watcher.program_work();
    let evm = watcher.work();
    drop(watcher);
    let mut watcher = open(&input);
    assert_eq!(watcher.program_work(), programs);
    assert_eq!(watcher.work(), evm);
    assert_eq!(watcher.next_block(), Some(number(&input, "block") + 1));
    assert!(watcher.poll().unwrap().is_empty());

    let response = unhex0x(input["response"].as_str().unwrap()).unwrap();
    let digest = keccak(&response);
    let full_length = u32::try_from(response.len()).unwrap();
    let network = u32::try_from(number(&input, "network_id")).unwrap();
    let mut answers = Vec::new();
    for request in watcher.program_work() {
        let record = expected
            .iter()
            .find(|record| record["program_id"] == hex0x(&request.program_id))
            .unwrap();
        assert_eq!(request.request_id, number(record, "request_id"));
        assert_eq!(u64::from(request.kind), number(record, "kind"));
        assert_eq!(request.sequence, number(record, "sequence"));
        assert_ne!(request.sequence, 0);
        assert_eq!(hex0x(&request.payload), record["payload"]);
        let ready = ready(
            &input,
            request.attestation(network, digest, &response, full_length),
            0,
            u64::MAX,
        );
        answers.push(json!({
            "program_id": hex0x(&request.program_id),
            "observation": hex0x(&observation_bytes(network, &request, &ready).unwrap()),
        }));
    }
    let evm_answer = watcher.work().first().map(|request| {
        assert_eq!(request.request_id, number(&input, "request_id"));
        assert_eq!(request.timeout_height, number(&input, "timeout"));
        assert_eq!(hex0x(&request.paid), input["paid"]);
        assert_eq!(request.callback_gas, number(&input, "callback_gas"));
        assert_eq!(hex0x(&request.payload), input["evm_payload"]);
        let ready = ready(
            &input,
            Attestation::evm(
                number(&input, "chain_id"),
                request,
                digest,
                &response,
                full_length,
            ),
            request.callback_gas,
            request.timeout_height,
        );
        json!({"signatures": ready.signatures.iter().map(|sig| hex0x(sig)).collect::<Vec<_>>()})
    });
    println!("{}", json!({"programs": answers, "evm": evm_answer}));
}
