use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread;
use std::time::{Duration, Instant};

use layerx_agentd::budget::{LocalAccounting, ProtocolBudgetState};
use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_agentd::policy::{DecisionReason, Outcome};
use layerx_policy_harness::{
    agent_policy_harness, captured, harness_cases, required_blocked_constraints, run_cases,
    ConstraintDimension, HarnessEvidence,
};
use layerx_types::json::{parse, JsonValue};

fn evidence() -> HarnessEvidence {
    HarnessEvidence::decode(
        &captured::checkpoint().unwrap(),
        &captured::budget_result().unwrap(),
    )
    .unwrap()
}

fn serve_budget() -> (NodeRpcConfig, thread::JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let config = NodeRpcConfig {
        url: format!("http://{}/", listener.local_addr().unwrap()),
        chain_id: 125,
        deadline: Duration::from_secs(5),
    };
    let server = thread::spawn(move || {
        let transcript = parse(include_str!(
            "../../../../../tests/vectors/budget-proof-v2/node-answers.json"
        ))
        .unwrap();
        for method in ["eth_chainId", "eth_call"] {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).unwrap(), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(value.trim().parse::<usize>().unwrap());
                }
            }
            let length = length.unwrap();
            assert!(length <= 4096);
            let mut bytes = vec![0; length];
            reader.read_exact(&mut bytes).unwrap();
            let request = parse(std::str::from_utf8(&bytes).unwrap()).unwrap();
            assert_eq!(request.str_at("jsonrpc").unwrap(), "2.0");
            assert_eq!(request.str_at("method").unwrap(), method);
            let result = if method == "eth_chainId" {
                assert_eq!(request.array_at("params").unwrap(), &[]);
                "0x7d"
            } else {
                assert_eq!(
                    request.path("params").unwrap(),
                    transcript.path("4.params").unwrap()
                );
                transcript.str_at("4.result").unwrap()
            };
            let id = request.u64_at("id").unwrap();
            let body = format!(r#"{{"jsonrpc":"2.0","id":{id},"result":"{result}"}}"#);
            write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap();
            reader.get_mut().flush().unwrap();
        }
    });
    (config, server)
}

#[test]
fn policy_harness_evidence_gap_real_owner_rpc_proves_each_constraint() {
    let (config, server) = serve_budget();
    let checkpoint = captured::checkpoint().unwrap();
    let evidence = HarnessEvidence::read(&config, &checkpoint).unwrap();
    server.join().unwrap();
    assert_eq!(evidence.budget.observed_head_sequence(), captured::HEIGHT);
    assert_eq!(evidence.budget.local_after(), 23);
    assert_eq!(evidence.budget.remaining(), 77);
    let report = run_cases(&evidence, &harness_cases(&evidence).unwrap()).unwrap();
    assert_eq!(report, agent_policy_harness().unwrap());
    assert_eq!(report.observed_height, 8);
    assert_eq!(report.protocol_consumed, 23);
    assert_eq!(report.blocked_constraints, required_blocked_constraints());
    assert_eq!(report.blocked_constraints.len(), 12);
    assert_eq!(report.decisions.len(), 14);
    let control = &report.decisions[0];
    assert_eq!(control.case, "intended-control");
    assert_eq!(control.outcome, Outcome::Allow);
    assert_eq!(control.reason, DecisionReason::PermittedByRule);
    assert_eq!(control.dimension, None);
    assert_eq!(control.matched_rules, ["intended-research"]);
    assert_eq!(control.deciding_rule.as_deref(), Some("intended-research"));
    for decision in &report.decisions[1..] {
        assert_eq!(decision.outcome, Outcome::Deny);
        assert_eq!(decision.policy_version, "corpus-v1");
        if decision.dimension == Some(ConstraintDimension::RequiredApproval) {
            assert_eq!(decision.reason, DecisionReason::ApprovalRequired);
            assert_eq!(decision.matched_rules, ["constraint-RequiredApproval"]);
            assert_eq!(
                decision.deciding_rule.as_deref(),
                Some("constraint-RequiredApproval")
            );
        } else {
            assert_eq!(decision.reason, DecisionReason::NoPermittingRule);
            assert!(decision.matched_rules.is_empty());
            assert_eq!(decision.deciding_rule, None);
        }
    }
}

#[test]
fn policy_harness_evidence_gap_labels_and_unrelated_denials_cannot_claim_coverage() {
    let evidence = evidence();
    for index in 0..harness_cases(&evidence).unwrap().len() {
        let mut cases = harness_cases(&evidence).unwrap();
        cases[index].session.open = false;
        let error = run_cases(&evidence, &cases).unwrap_err();
        assert!(
            error.contains("does not isolate its declared constraint"),
            "{error}"
        );
    }
    let mut cases = harness_cases(&evidence).unwrap();
    cases[0].dimension = ConstraintDimension::RequiredApproval;
    assert!(run_cases(&evidence, &cases)
        .unwrap_err()
        .contains("does not isolate"));
    let mut cases = harness_cases(&evidence).unwrap();
    cases.retain(|case| case.dimension != ConstraintDimension::CumulativeRate);
    assert!(run_cases(&evidence, &cases)
        .unwrap_err()
        .contains("coverage incomplete"));
}

#[test]
fn policy_harness_evidence_gap_stale_budget_cannot_turn_missing_context_into_coverage() {
    let mut evidence = evidence();
    let stale = ProtocolBudgetState::decode_get_budget(
        captured::BUDGET_ID,
        captured::HEIGHT - 1,
        &captured::budget_result().unwrap(),
    )
    .unwrap();
    let mut local = LocalAccounting {
        consumed: 999,
        window_start_sequence: 0,
        last_receipt: None,
    };
    evidence.budget = stale.reconcile(&mut local, &[]).unwrap();
    assert_eq!(local.consumed, 23);
    let error = run_cases(&evidence, &harness_cases(&evidence).unwrap()).unwrap_err();
    assert!(error.contains("intended-control"), "{error}");
    assert!(error.contains("InvalidContext"), "{error}");
}

#[test]
fn policy_harness_evidence_gap_owner_codec_refuses_corrupt_and_inactive_evidence() {
    let checkpoint = captured::checkpoint().unwrap();
    let bytes = captured::budget_result().unwrap();
    for length in [0, 31, 32, bytes.len() - 1] {
        assert!(HarnessEvidence::decode(&checkpoint, &bytes[..length]).is_err());
    }
    let mut trailing = bytes.clone();
    trailing.push(0);
    assert!(HarnessEvidence::decode(&checkpoint, &trailing).is_err());
    let mut oversized_amount = bytes.clone();
    oversized_amount[8 * 32] = 1;
    assert!(HarnessEvidence::decode(&checkpoint, &oversized_amount).is_err());
    for index in [15, 16] {
        let mut inactive = bytes.clone();
        inactive[index * 32 + 31] = 1;
        let error = HarnessEvidence::decode(&checkpoint, &inactive)
            .err()
            .unwrap();
        assert!(error.contains("not current"), "{error}");
    }
    let mut expired = bytes.clone();
    expired[12 * 32..13 * 32].copy_from_slice(&bytes[11 * 32..12 * 32]);
    assert!(HarnessEvidence::decode(&checkpoint, &expired).is_err());
    let mut bad_checkpoint = checkpoint;
    bad_checkpoint.commit.votes[0].signature[0] ^= 1;
    assert!(HarnessEvidence::decode(&bad_checkpoint, &bytes).is_err());
}

#[test]
fn policy_harness_evidence_gap_cumulative_probe_uses_protocol_spend_at_the_boundary() {
    let evidence = evidence();
    let cases = harness_cases(&evidence).unwrap();
    let cumulative = cases
        .iter()
        .find(|case| case.dimension == ConstraintDimension::CumulativeRate)
        .unwrap();
    assert_eq!(
        cumulative.policy.rules[0]
            .constraints
            .maximum_cumulative_amount,
        Some(73)
    );
    assert_eq!(cumulative.policy.rules[0].constraints.maximum_amount, None);
    assert_eq!(cumulative.request.amount, 51);
    assert_eq!(
        evidence.budget.local_after() + cumulative.request.amount,
        74
    );
    let report = run_cases(&evidence, &cases).unwrap();
    let refusal = report
        .decisions
        .iter()
        .find(|decision| decision.dimension == Some(ConstraintDimension::CumulativeRate))
        .unwrap();
    assert_eq!(refusal.reason, DecisionReason::NoPermittingRule);
    assert_eq!(refusal.outcome, Outcome::Deny);
}

#[test]
fn policy_harness_evidence_gap_recording_matches_the_owner_transcript() {
    let transcript = parse(include_str!(
        "../../../../../tests/vectors/budget-proof-v2/node-answers.json"
    ))
    .unwrap();
    assert!(matches!(transcript, JsonValue::Array(_)));
    assert_eq!(
        transcript.hex_at("4.params.0.data").unwrap(),
        captured::CALL_DATA
    );
    assert_eq!(
        transcript.hex_at("4.result").unwrap(),
        captured::budget_result().unwrap()
    );
    let checkpoint = captured::checkpoint().unwrap();
    assert_eq!(
        transcript.hex_at("1.result.hash").unwrap(),
        checkpoint.block_hash
    );
    assert_eq!(
        transcript.hex_at("1.result.stateRoot").unwrap(),
        checkpoint.app_hash
    );
    assert_eq!(transcript.str_at("4.params.1").unwrap(), "0x8");
}
