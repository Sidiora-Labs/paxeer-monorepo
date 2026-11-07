include!("native_send_preview_evm.rs");

use layerx_agent_api::identity::{NativeSendPurposeV2, SignedNativeSendPurposeV2};
use layerx_agentd::agent_rpc::{handle_rpc, AgentRpcResponse};
use layerx_agentd::capability::binding::native_send_purpose_digest;
use layerx_agentd::prepare::DurablePreparation;
use layerx_agentd::sign::SigningPreparation;

type CommittedNativeCalls = BTreeMap<(String, Vec<u8>), Vec<u8>>;

fn committed_native_send_calls(binding: &SignedBinding) -> CommittedNativeCalls {
    let word = |value: u128| {
        let mut bytes = [0; 32];
        bytes[16..].copy_from_slice(&value.to_be_bytes());
        bytes
    };
    let public = binding.public_key();
    let did = must(Did::new(
        format!("did:layerx:{}", hex::encode(public)).as_bytes(),
    ));
    let owner = must(layerx_wire::hash::did_id_for_protocol(&did, 3));
    let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
    let account = must(AccountId::for_asset(
        must(std::str::from_utf8(did.as_bytes())),
        asset,
        asset,
    ));
    let main = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
    let mut address = [0; 32];
    address[12..].copy_from_slice(&binding.binding().evm_address());
    let mut did_binding = [public, word(64), word(did.as_bytes().len() as u128)].concat();
    did_binding.extend_from_slice(did.as_bytes());
    did_binding.resize(did_binding.len().div_ceil(32) * 32, 0);

    // getAsset returns the twelve-word ledger metadata tuple, including three
    // dynamic fields. The fee denomination is the same upax asset as the send.
    let mut metadata = vec![[0; 32]; 12];
    metadata[0] = asset;
    metadata[1] = word(384);
    metadata[2] = word(448);
    metadata[3] = word(6);
    metadata[9] = word(512);
    let mut metadata = metadata.concat();
    for value in [b"PAX".as_slice(), b"Paxeer", b""] {
        metadata.extend_from_slice(&word(value.len() as u128));
        metadata.extend_from_slice(value);
        metadata.resize(metadata.len().div_ceil(32) * 32, 0);
    }

    // Use the production budget codec and the record used by the Human budget
    // checkpoint fixture, with its owner and asset bound to this fixture.
    let vectors: Value = must(serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../tests/vectors/native-budget-records.json"
    ))));
    let record = must(layerx_agentd::budget::ProtocolBudgetRecord::decode(&must(
        hex::decode(text(&vectors, "v1").trim_start_matches("0x")),
    )));
    let mut budget = vec![[0; 32]; 20 + record.delegates.len()];
    budget[0] = word(32);
    budget[1] = owner;
    budget[2] = record.budget_account;
    budget[3] = asset;
    budget[4] = record.purpose_hash;
    for (index, value) in [
        (5, record.per_period_limit),
        (6, record.configured_period_limit),
        (7, record.carry_cap),
        (8, record.spent_this_period),
        (9, record.carried),
        (10, u128::from(record.period_length)),
        (11, u128::from(record.period_start)),
        (12, u128::from(record.expiry)),
        (13, u128::from(record.revocation_sequence)),
        (14, u128::from(record.rollover_policy)),
        (15, u128::from(record.closed)),
        (16, u128::from(record.revoked)),
        (17, 576),
        (19, record.delegates.len() as u128),
    ] {
        budget[index] = word(value);
    }
    budget[18] = record.source_account.unwrap_or([0; 32]);
    budget[20..].copy_from_slice(&record.delegates);
    let budget = budget.concat();
    let decoded = must(
        layerx_agentd::budget::ProtocolBudgetState::decode_get_budget(
            record.budget_id,
            HEAD,
            &budget,
        ),
    );
    assert_eq!(decoded.record().owner, owner);
    assert_eq!(decoded.record().asset_id, asset);
    assert_eq!(decoded.record().remaining(), record.remaining());

    let mut calls = BTreeMap::new();
    let mut add = |target: [u8; 20], signature: &[u8], args: &[u8], answer: Vec<u8>| {
        let selector: [u8; 4] = layerx_crypto::evm_transaction::transaction_hash(signature)[..4]
            .try_into()
            .expect("four-byte selector");
        let calldata = [selector.as_slice(), args].concat();
        assert!(calls
            .insert((format!("0x{}", hex::encode(target)), calldata), answer)
            .is_none());
    };
    let layerx = layerx_client::account_binding::ADDR_PRECOMPILE;
    let mut ledger = [0; 20];
    ledger[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut budget_precompile = ledger;
    budget_precompile[19] = 0x1b;
    add(
        layerx,
        b"getEvmAddrByLayerX(bytes32)",
        &public,
        address.to_vec(),
    );
    add(
        layerx,
        b"getUnifiedAccount(address)",
        &address,
        [address, word(128), public, main, word(0)].concat(),
    );
    add(layerx, b"getLayerXDid(address)", &address, did_binding);
    add(ledger, b"getAsset(bytes32)", &asset, metadata);
    add(
        ledger,
        b"accountId(bytes32,bytes32)",
        &[owner, asset].concat(),
        main.to_vec(),
    );
    add(ledger, b"accountOwner(bytes32)", &main, owner.to_vec());
    add(
        ledger,
        b"nextSequence(bytes32,bytes32)",
        &[main, asset].concat(),
        word(7).to_vec(),
    );
    add(
        budget_precompile,
        b"getBudget(bytes32)",
        &record.budget_id,
        budget,
    );
    add(
        budget_precompile,
        b"budgetRemaining(bytes32)",
        &record.budget_id,
        word(record.remaining()).to_vec(),
    );
    // balanceOf has two legitimate accounts, selected by the full calldata.
    // The ledger account balance covers both the send and its declared fee.
    add(
        ledger,
        b"balanceOf(bytes32,bytes32)",
        &[main, asset].concat(),
        word(2_000_000_000_000).to_vec(),
    );
    add(
        ledger,
        b"balanceOf(bytes32,bytes32)",
        &[record.budget_account, asset].concat(),
        word(record.remaining()).to_vec(),
    );
    calls
}

// The recorded checkpoint has no captured predecessor or result ranges. Build
// the surrounding empty-block history with its real fixture validator, using
// the production header hash and precommit encoder. Both production verifiers
// authenticate every generated commit before the peer can serve it.
fn native_send_history_commits(
    recorded: &Value,
    validators: &Value,
    upstream: &str,
) -> BTreeMap<u64, Value> {
    use layerx_agentd::read::{
        BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
    };
    let verifier = layerx_programs::chain_proof::ChainProofVerifier {
        chain_id: "125".to_owned(),
        validators: must(layerx_client::rpc::commit::decode_validators(validators)),
    };
    let original = must(layerx_client::rpc::commit::verify_commit_material(
        &recorded["signedHeader"],
        &verifier,
    ));
    let key = ed25519_dalek::SigningKey::from_bytes(&[1; 32]);
    let public_key = key.verifying_key().to_bytes();
    assert_eq!(verifier.validators.validators().len(), 1);
    assert_eq!(verifier.validators.validators()[0].public_key, public_key);
    let validator = CommitValidator {
        address: CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: verifier.validators.validators()[0].voting_power,
    };
    let h = &recorded["signedHeader"]["header"];
    let digest = |name: &str| -> [u8; 32] {
        must(hex::decode(text(h, name)))
            .try_into()
            .expect("recorded header digest")
    };
    let empty_results = layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[]);
    let head_seconds = must(i64::try_from(original.time_ms())) / 1000;
    assert_eq!(h["time"], "2026-09-14T05:59:04Z");
    let mut parent: [u8; 32] = must(hex::decode(text(&h["last_block_id"], "hash")))
        .try_into()
        .expect("recorded predecessor");
    let mut commits = BTreeMap::new();
    for (height, seconds, timestamp) in [
        (HEAD - 1, head_seconds - 11, "2026-09-14T05:58:53Z"),
        (HEAD, head_seconds, "2026-09-14T05:59:04Z"),
        (HEAD + 1, head_seconds + 1, "2026-09-14T05:59:05Z"),
    ] {
        // The two blocks whose complete native result sets are served below
        // contain no transactions on the real loopback EVM node either.
        if height <= HEAD {
            let block = rpc(
                upstream,
                &json!({"jsonrpc":"2.0", "id":1,
                "method":"eth_getBlockByNumber", "params":[format!("0x{height:x}"), false]}),
            );
            assert!(block.get("error").is_none(), "{block}");
            assert_eq!(block["result"]["transactions"], json!([]));
        }
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "125".to_owned(),
            height,
            time: CommitTimestamp { seconds, nanos: 0 },
            last_block_id: Some(BlockId {
                hash: parent,
                part_set_total: 1,
                part_set_hash: [2; 32],
            }),
            last_commit_hash: Some(digest("last_commit_hash")),
            data_hash: Some(digest("data_hash")),
            validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
            next_validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
            consensus_hash: digest("consensus_hash"),
            app_hash: original.app_hash(),
            last_results_hash: Some(empty_results),
            evidence_hash: Some(digest("evidence_hash")),
            proposer_address: validator.address,
        };
        let mut commit = BlockCommit {
            height,
            round: 1,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [9; 32],
            },
            votes: Vec::new(),
        };
        let signature = key
            .sign(&commit.vote_sign_bytes("125", header.time))
            .to_bytes();
        commit.votes.push(CommitVote {
            validator_address: validator.address,
            voting_power: validator.voting_power,
            timestamp: header.time,
            signature,
        });
        let checkpoint = must(layerx_agentd::read::checkpoint(
            height,
            "125",
            &header,
            &commit,
            &[validator],
        ));
        let mut signed = recorded["signedHeader"].clone();
        signed["header"]["height"] = json!(height.to_string());
        signed["header"]["time"] = json!(timestamp);
        signed["header"]["last_block_id"]["hash"] = json!(hex::encode(parent));
        signed["header"]["last_results_hash"] = json!(hex::encode(empty_results));
        signed["commit"]["height"] = json!(height.to_string());
        signed["commit"]["block_id"]["hash"] = json!(hex::encode(checkpoint.block_hash));
        signed["commit"]["signatures"][0]["timestamp"] = json!(timestamp);
        signed["commit"]["signatures"][0]["signature"] =
            json!(openssl::base64::encode_block(&signature));
        let verified = must(layerx_client::rpc::commit::verify_commit_material(
            &signed, &verifier,
        ));
        assert_eq!(verified.header_hash(), checkpoint.block_hash);
        assert_eq!(verified.app_hash(), original.app_hash());
        assert_eq!(verified.time_ms(), seconds * 1000);
        parent = checkpoint.block_hash;
        assert!(commits.insert(height, signed).is_none());
    }
    commits
}

fn fixture_with_receipt_keys() -> Fixture {
    let mut fixture = Fixture::new();
    let upstream = must(NodeRpcConfig::from_environment()).url;
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let url = format!("http://{}/", must(listener.local_addr()));
    let receipt_key = ed25519_dalek::SigningKey::from_bytes(fixture.account.layerx_seed());
    let key_id: [u8; 32] = Sha256::digest(receipt_key.verifying_key().as_bytes()).into();
    let keys = json!({"keys": [{
        "keyId": format!("0x{}", hex::encode(key_id)),
        "publicKey": format!("0x{}", hex::encode(receipt_key.verifying_key().as_bytes())),
        "firstHeight": 1,
        "lastHeight": HEAD + 1000
    }]});
    let decoded = must(layerx_client::sequencer::decode_receipt_keys(&keys));
    assert_eq!(decoded.len(), 1);
    assert_eq!(decoded[0].key_id, key_id);
    assert_eq!(
        decoded[0].public_key,
        receipt_key.verifying_key().to_bytes()
    );
    assert!((decoded[0].first_height..=decoded[0].last_height).contains(&HEAD));
    let checkpoint_reads: Vec<Value> = must(serde_json::from_str(include_str!(
        "support/human_budget_checkpoint.json"
    )));
    let checkpoint_result = |method: &str| {
        checkpoint_reads
            .iter()
            .find(|recorded| {
                recorded["method"] == method && recorded["params"] == json!([HEAD.to_string()])
            })
            .expect("checkpoint RPC recorded at the fixture head")["result"]
            .clone()
    };
    let commit = checkpoint_result("px_getCommit");
    let validators = checkpoint_result("px_getValidators");
    let verifier = layerx_programs::chain_proof::ChainProofVerifier {
        chain_id: "125".to_owned(),
        validators: must(layerx_client::rpc::commit::decode_validators(&validators)),
    };
    let history_commits = native_send_history_commits(&commit, &validators, &upstream);
    let commit = json!({"height":HEAD.to_string(), "signedHeader":history_commits[&HEAD]});
    let head = must(layerx_client::rpc::commit::verify_commit_material(
        &commit["signedHeader"],
        &verifier,
    ));
    assert_eq!(head.chain_id(), "125");
    assert_eq!(must(u64::try_from(head.height())), HEAD);
    let timestamp = must(u64::try_from(head.time_ms())) / 1000;
    let binding =
        Binding::new(125, fixture.account.evm_address(), 3).sign(fixture.account.layerx_seed());
    must(binding.verify());
    let committed_calls = committed_native_send_calls(&binding);
    let history_account = format!("0x{}", hex::encode(fixture.account.evm_address()));
    let peer = Peer::spawn(move |stop| {
        let _receipt_key = receipt_key;
        while !stop.load(Ordering::Acquire) {
            let Some(mut stream) = incoming(&listener) else {
                continue;
            };
            let (_, body) = http_request(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            let method = text(&request, "method");
            let response = if method == "px_getReceiptKeys" {
                json!({"jsonrpc": "2.0", "id": request["id"], "result": keys})
            } else if method == "px_getCommit" || method == "px_getValidators" {
                assert_eq!(request["params"], json!([HEAD.to_string()]));
                let result = if method == "px_getCommit" {
                    &commit
                } else {
                    &validators
                };
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
            } else if method == "px_getHistory" {
                assert_eq!(request["params"][0], history_account);
                assert!(request["params"][1].is_null() || request["params"][1].is_string());
                assert_eq!(request["params"][2], 100);
                json!({"jsonrpc":"2.0", "id":request["id"], "result":{
                    "account":history_account, "items":[], "next_cursor":null,
                    "indexedHeight":HEAD, "chainHeight":HEAD,
                    "freshness":{"version":1, "database":"ok", "status":"ready",
                        "sources":[{"source":"comet", "source_head":HEAD,
                            "indexed_cursor":HEAD, "finalized_cursor":HEAD,
                            "last_success_at":timestamp, "freshness_secs":0,
                            "last_error_at":null, "reconciled":true,
                            "consecutive_failures":0, "state":"ready"}]}
                }})
            } else if method == "commit" || method == "block_results" {
                let height = must(text(&request["params"], "height").parse::<u64>());
                let signed = history_commits
                    .get(&height)
                    .expect("covered policy history block");
                let result = if method == "commit" {
                    json!({"signed_header":signed})
                } else {
                    assert!((HEAD - 1..=HEAD).contains(&height));
                    json!({"height":height.to_string(), "txs_results":[]})
                };
                eprintln!("repair_22_31_560 chain peer: authenticated history method={method} height={height}");
                json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
            } else if method == "eth_call" && request["params"][1] == format!("0x{HEAD:x}") {
                let call = &request["params"][0];
                let data = call
                    .get("data")
                    .or_else(|| call.get("input"))
                    .and_then(Value::as_str)
                    .expect("committed call data");
                let data = must(hex::decode(data.trim_start_matches("0x")));
                assert!(data.len() >= 4, "committed call selector: {request}");
                let target = text(call, "to").to_owned();
                if let Some(answer) = committed_calls.get(&(target.clone(), data.clone())) {
                    eprintln!("repair_22_31_560 chain peer: served committed call target={target} selector=0x{}", hex::encode(&data[..4]));
                    json!({"jsonrpc": "2.0", "id": request["id"],
                        "result": format!("0x{}", hex::encode(answer))})
                } else {
                    assert!(
                        !committed_calls
                            .keys()
                            .any(|(address, expected)| address == &target
                                && expected[..4] == data[..4]),
                        "committed call arguments: {request}"
                    );
                    let response = rpc(&upstream, &request);
                    if response.get("error").is_some() {
                        eprintln!(
                            "repair_22_31_560 chain peer: request={request} response={response}"
                        );
                    }
                    response
                }
            } else {
                if method == "eth_getProof" || method == "abci_query" {
                    eprintln!("repair_22_31_560 chain peer: committed proof request={request}");
                }
                let mut response = rpc(&upstream, &request);
                if response.get("error").is_some() {
                    eprintln!("repair_22_31_560 chain peer: request={request} response={response}");
                }
                if method == "eth_getBlockByNumber"
                    && (request["params"][0] == format!("0x{HEAD:x}")
                        || request["params"][0] == "latest")
                {
                    assert!(response.get("error").is_none(), "{response}");
                    let block = &mut response["result"];
                    assert_eq!(block["number"], format!("0x{HEAD:x}"));
                    block["hash"] = json!(format!("0x{}", hex::encode(head.header_hash())));
                    block["stateRoot"] = json!(format!("0x{}", hex::encode(head.app_hash())));
                    block["timestamp"] = json!(format!("0x{timestamp:x}"));
                }
                response
            };
            http_response(&mut stream, &response);
        }
    });
    fixture.peers.push(peer);
    std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &url);
    std::env::set_var("LAYERX_AGENT_NODE_COMET_RPC_URL", &url);
    fixture.endpoint = must(SequencerEndpoint::new(&url, &[]));
    reopen_daemon(&mut fixture);
    fixture
}

fn owner_preview_over_unix(
    fixture: &mut Fixture,
    prepare: &HumanPrepare,
    grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1,
) -> Vec<u8> {
    let bytes = |out: &mut Vec<u8>, value: &[u8]| {
        out.extend_from_slice(&must(u32::try_from(value.len())).to_be_bytes());
        out.extend_from_slice(value);
    };
    let mut request = b"LXHAGT01".to_vec();
    request.push(layerx_agentd::human::NATIVE_SEND_OWNER_PREVIEW_V1);
    request.extend_from_slice(&1_u16.to_be_bytes());
    bytes(
        &mut request,
        fixture.credential.tenant().as_str().as_bytes(),
    );
    request.extend_from_slice(&fixture.credential.session_id().0);
    request.extend_from_slice(&fixture.credential.token_id());
    request.extend_from_slice(&fixture.credential.generation().to_be_bytes());
    request.extend_from_slice(&71_u64.to_be_bytes());
    request.extend_from_slice(&prepare.activity_type.to_be_bytes());
    bytes(&mut request, prepare.actor.as_bytes());
    bytes(&mut request, prepare.authority.as_bytes());
    request.extend_from_slice(&prepare.nonce.to_be_bytes());
    request.extend_from_slice(&prepare.not_before_height.to_be_bytes());
    request.extend_from_slice(&prepare.not_after_height.to_be_bytes());
    bytes(&mut request, prepare.idempotency_key.as_bytes());
    request.extend_from_slice(&prepare.fee_limit.to_be_bytes());
    bytes(&mut request, &prepare.payload);
    request.extend_from_slice(&prepare.payload_hash);
    request.extend_from_slice(&prepare.capability_id.expect("capability"));
    request.extend_from_slice(&fixture.account.layerx_public_key());
    request.extend_from_slice(&prepare.not_after_height.to_be_bytes());
    request.extend_from_slice(&[11; 32]);
    request.extend_from_slice(&fixture.credential.session_id().0);
    bytes(&mut request, &grant.capability);
    bytes(&mut request, &grant.session_scope);
    request.extend_from_slice(&grant.expires_at_ms.to_be_bytes());
    request.extend_from_slice(&grant.owner_public_key);
    request.extend_from_slice(&grant.signature);

    let deadline = Duration::from_secs(10);
    let maximum = 1_048_576;
    let (mut client, server) = must(UnixStream::pair());
    must(client.set_read_timeout(Some(deadline)));
    must(client.set_write_timeout(Some(deadline)));
    let mut transport = must(AcceptedTransport::new(server, maximum, deadline));
    must(client.write_all(&must(u32::try_from(request.len())).to_be_bytes()));
    must(client.write_all(&request));
    must(serve_one(&mut transport, &fixture.peer, &mut fixture.owner));
    let mut prefix = [0; 4];
    must(client.read_exact(&mut prefix));
    let length = u32::from_be_bytes(prefix) as usize;
    assert!(length <= maximum);
    let mut response = vec![0; length];
    must(client.read_exact(&mut response));
    response
}

fn prepare_rpc(
    fixture: &Fixture,
    human: &HumanPrepare,
    purpose: NativeSendPurposeV2,
    grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1,
    request_id: u64,
) -> AgentRpcResponse {
    let key = ed25519_dalek::SigningKey::from_bytes(fixture.account.layerx_seed());
    let signed = SignedNativeSendPurposeV2 {
        signature: key
            .sign(&must(native_send_purpose_digest(&purpose)))
            .to_bytes(),
        purpose,
    };
    let purpose = &signed.purpose;
    let envelope = json!({
        "version": 1, "request_id": request_id.to_string(), "operation": "prepare",
        "idempotency_key": hex::encode(random::<32>()),
        "credential": {
            "tenant": fixture.credential.tenant().as_str(),
            "session_id": hex::encode(fixture.credential.session_id().0),
            "token_id": hex::encode(fixture.credential.token_id()),
            "generation": fixture.credential.generation().to_string()
        },
        "request": {
            "variant": "native_send_v1",
            "activity": {"version": "1", "module": "1", "ordinal": "5"},
            "actor": human.actor, "authority": human.authority,
            "account_sequence": human.nonce.to_string(),
            "not_before": human.not_before_height.to_string(),
            "not_after": human.not_after_height.to_string(),
            "idempotency_key": human.idempotency_key,
            "fee_limit": human.fee_limit.to_string(),
            "payload": hex::encode(&human.payload),
            "payload_hash": hex::encode(human.payload_hash),
            "capability_id": purpose.capability_id.as_str(),
            "local_grant": {
                "version": "1",
                "capability": hex::encode(&grant.capability),
                "session_scope": hex::encode(&grant.session_scope),
                "expires_at_ms": grant.expires_at_ms.to_string(),
                "owner_public_key": hex::encode(grant.owner_public_key),
                "signature": hex::encode(grant.signature)
            },
            "purpose": {
                "purpose": {
                    "version": "2", "tenant": purpose.tenant.as_str(),
                    "agent_did": purpose.agent_did.as_str(),
                    "owner_did": purpose.owner_did.as_str(),
                    "consent_public_key": hex::encode(purpose.consent_public_key),
                    "evm_sender": hex::encode(purpose.evm_sender),
                    "session_id": purpose.session_id.as_str(),
                    "generation": purpose.generation.to_string(),
                    "expires_at_ms": purpose.expires_at_ms.to_string(),
                    "capability_id": purpose.capability_id.as_str(),
                    "transaction_type": purpose.transaction_type.to_string(),
                    "chain_id": purpose.chain_id.to_string(),
                    "activity": {"version": "1", "module": "1", "ordinal": "5"},
                    "preparation_id": hex::encode(purpose.preparation_id),
                    "canonical_digest": hex::encode(purpose.canonical_digest),
                    "economic_action": hex::encode(purpose.economic_action),
                    "idempotency_key": hex::encode(purpose.idempotency_key),
                    "commitment": hex::encode(purpose.commitment)
                },
                "signature": hex::encode(signed.signature)
            }
        }
    });
    handle_rpc(&fixture.owner, &must(serde_json::to_vec(&envelope)))
}

fn preview(
    fixture: &mut Fixture,
    human: &HumanPrepare,
    grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1,
) -> (Vec<u8>, Vec<u8>, NativeSendPurposeV2) {
    fixture.owner = must(fixture.owner.clone().with_idempotency(
        fixture.root.join("rpc-idempotency"),
        1000,
        1000,
    ));
    let response = owner_preview_over_unix(fixture, human, grant);
    assert_eq!(&response[..9], b"LXHAGT01\x00");
    let mut input = &response[9..];
    assert_eq!(&input[..2], &1_u16.to_be_bytes());
    input = &input[2..];
    let canonical = read_bytes(&mut input).to_vec();
    let preimage = read_bytes(&mut input).to_vec();
    let purpose = must(NativeSendPurposeV2::from_canonical_bytes(read_bytes(
        &mut input,
    )));
    assert_eq!(
        purpose.preparation_id,
        <[u8; 32]>::from(Sha256::digest(&canonical))
    );
    assert_eq!(purpose.canonical_digest, purpose.preparation_id);
    assert_eq!(
        preimage,
        must(layerx_crypto::evm_transaction::decode_unsigned(&canonical)).signing_hash()
    );
    (canonical, preimage, purpose)
}

fn successful_preparation(
    response: AgentRpcResponse,
) -> layerx_agent_api::identity::NativePrepareResultV1 {
    assert_eq!(
        response.status,
        200,
        "{}",
        String::from_utf8_lossy(&response.body)
    );
    let response: Value = must(serde_json::from_slice(&response.body));
    layerx_sdk::agent_envelope::decode_native_preparation(&response["value"])
        .unwrap_or_else(|| panic!("native preparation result: {response}"))
}

fn assert_durable(fixture: &Fixture, id: [u8; 32], canonical: &[u8]) {
    let retained = must(fixture.owner.lock()).session_control.store();
    let store = must(retained.lock());
    let tenant = fixture.credential.tenant();
    let key = must(DurablePreparation::store_key(tenant, id));
    let durable = must(DurablePreparation::decode(
        tenant.clone(),
        store
            .get(&key)
            .expect("durable preparation at preview identifier")
            .bytes(),
    ));
    assert_eq!(durable.preparation_id, id);
    assert_eq!(durable.session_id, fixture.credential.session_id().0);
    assert_eq!(durable.generation, fixture.credential.generation());
    let signing = must(SigningPreparation::restore(&store, tenant.clone(), id));
    assert_eq!(signing.transaction.call.encode_unsigned(), canonical);
    let references = store
        .list_object_ids(tenant, layerx_agentd::store::ObjectKind::Configuration)
        .into_iter()
        .filter(|key| key.starts_with(b"prepare/authorization-reference/"))
        .collect::<Vec<_>>();
    assert_eq!(references.len(), 1);
    let reference_key = must(layerx_agentd::store::TenantKey::new(
        tenant.clone(),
        layerx_agentd::store::ObjectKind::Configuration,
        references[0].clone(),
    ));
    assert_eq!(
        store
            .get(&reference_key)
            .expect("authorization reference")
            .bytes(),
        id
    );
    let transport_tenant = must(TenantId::new(fixture.peer.transport_tenant().to_owned()));
    assert_ne!(tenant, &transport_tenant);
    assert!(store
        .get(&must(DurablePreparation::store_key(&transport_tenant, id)))
        .is_none());
    assert!(SigningPreparation::restore(&store, transport_tenant.clone(), id).is_err());
    let transport_reference_key = must(layerx_agentd::store::TenantKey::new(
        transport_tenant,
        layerx_agentd::store::ObjectKind::Configuration,
        references[0].clone(),
    ));
    assert!(store.get(&transport_reference_key).is_none());
}

fn reopen_daemon(fixture: &mut Fixture) {
    let retained = must(fixture.owner.lock()).session_control.store();
    {
        let mut store = must(retained.lock());
        let replacement = must(Store::open(fixture.root.join("closed-store")));
        drop(std::mem::replace(&mut *store, replacement));
        *store = must(Store::open(fixture.root.join("store")));
    }
    let public_key = fixture.account.layerx_public_key();
    let activity = must(ActivityType::new(ModuleId::Asset, 5));
    let (authority, authority_worker) = authority_peer(
        fixture.peer.clone(),
        CoreIdentity {
            canonical_bytes: public_key.to_vec(),
            head_sequence: HEAD,
            revocation_sequence: 1,
            verification_level: layerx_types::verify::VerificationLevel::CHECKPOINT_FINALISED,
            frozen: false,
            authorities: vec![ProtocolAuthority::PrimaryKey(public_key)],
        },
        activity,
    );
    let restart = hex::encode(random::<4>());
    let clock_root = fixture.root.join(format!("restart-clock-{restart}"));
    must(fs::create_dir(&clock_root));
    must(fs::set_permissions(
        &clock_root,
        fs::Permissions::from_mode(0o700),
    ));
    let config = must(NodeRpcConfig::from_environment());
    let endpoint = must(SequencerEndpoint::new(&config.url, &[]));
    let checkpoint = must(
        layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
            &endpoint, HEAD, None, "125",
        ),
    );
    let core_time_ms = must(u64::try_from(checkpoint.header.time.seconds))
        .checked_mul(1000)
        .expect("checkpoint time in milliseconds");
    let (clock, clock_worker) = clock_peer_at(&clock_root, core_time_ms);
    let trust = must(layerx_agentd::export::ExportTrustSource::new(
        must(endpoint.receipt_keys()),
        checkpoint.validators,
    ));
    let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
    let account = must(layerx_wire::hash::account_id_for_protocol(
        &must(AccountId::for_asset(&fixture.account.did(), asset, asset)),
        3,
    ));
    let commitment = [11_u8; 32];
    let destination = [9_u8; 32];
    let policy = json!({
        "schema": "layerx.native-effect-policy.v1", "version": "native-send-test-v1",
        "network_id": 125, "step_limit": 100,
        "rules": [
            {"id": "operation", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
             "scope": {"kind": "Operation"}, "required_approval": false, "purpose_commitment": commitment},
            {"id": "transfer", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
             "scope": {"kind": "Transfer", "from": account, "to": destination,
                       "asset": asset, "maximum_amount": "10"},
             "required_approval": false, "purpose_commitment": commitment},
            {"id": "fee", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
             "scope": {"kind": "Fee", "source_account": account, "asset": asset,
                       "maximum_amount": "1000000000000"},
             "required_approval": false, "purpose_commitment": commitment}
        ]
    });
    let policy_tenant = must(TenantId::new(fixture.peer.transport_tenant().to_owned()));
    let policy_path = fixture
        .root
        .join(format!("native-effect-policy-{restart}.json"));
    must(fs::write(
        &policy_path,
        must(serde_json::to_vec(&json!({
            "version": "layerx.native-effect-policy-source.v1",
            "tenant": policy_tenant.as_str(), "policy": policy.to_string(),
        }))),
    ));
    must(fs::set_permissions(
        &policy_path,
        fs::Permissions::from_mode(0o600),
    ));
    let peers = BTreeMap::from([(
        7,
        ("bootstrap-owner".to_owned(), "bootstrap-service".to_owned()),
    )]);
    let mut operations = must(ProductionHumanOperations::new(
        authority,
        must(SequencerEndpoint::new(&config.url, &[])),
        Box::new(must(NodeRpc::connect(&config))),
        Arc::clone(&retained),
        &peers,
        1_048_576,
        30_000,
        clock,
    ));
    must(operations.attach_native_effect_policies(&BTreeMap::from([(policy_tenant, policy_path)])));
    let limit = LimitConfig {
        id: LimitId([12; 16]),
        name: "preview".to_owned(),
        scope: LimitScope::Tenant(
            Sha256::digest(fixture.credential.tenant().as_str().as_bytes()).into(),
        ),
        ceiling: u128::MAX,
        consumed: 0,
    };
    let stable_id = layerx_agentd::budget::enrolment_limit_id(
        fixture.credential.tenant(),
        &limit.scope,
        &limit.id,
    );
    let denominations_path = fixture
        .root
        .join(format!("native-send-denominations-{restart}.json"));
    must(fs::write(
        &denominations_path,
        must(serde_json::to_vec(&json!({
            "version": 1, "tenant": fixture.credential.tenant().as_str(),
            "limits": [{"stable_id": hex::encode(stable_id),
                "asset": hex::encode(asset), "source": hex::encode(account)}]
        }))),
    ));
    must(fs::set_permissions(
        &denominations_path,
        fs::Permissions::from_mode(0o600),
    ));
    must(
        operations.attach_program_budget_denominations(&BTreeMap::from([(
            fixture.credential.tenant().clone(),
            denominations_path,
        )])),
    );
    operations.install_export_trust(trust, Duration::from_secs(3));
    must(operations.authorize_subject(&fixture.peer));
    operations.recover_tenants(std::slice::from_ref(&fixture.peer), u128::MAX);
    assert!(matches!(
        operations.write_admission(&fixture.peer.tenant),
        Some(Ok(()))
    ));
    let keys = fixture.root.join(format!("restart-keys-{restart}"));
    must(fs::create_dir(&keys));
    must(fs::set_permissions(
        &keys,
        fs::Permissions::from_mode(0o700),
    ));
    let registry = must(SessionKeyRegistry::open(
        keys,
        random::<32>().to_vec(),
        125,
        must(fs::metadata(&fixture.root)).uid(),
    ));
    let mut owner = must(UnifiedAgentOwner::new(
        operations,
        retained,
        &peers,
        vec![limit],
        registry,
    ));
    layerx_agentd::degraded::enter(
        &mut owner.degraded,
        layerx_agentd::degraded::Observation::Ready {
            reference: layerx_agentd::degraded::Reference {
                head_sequence: HEAD,
                checkpoint: [0; 32],
            },
            maximum_rung: layerx_types::rung::Rung::Submitted,
        },
    );
    fixture.owner = must(SharedAgentOwner::new(owner).with_idempotency(
        fixture.root.join("rpc-idempotency"),
        1000,
        1000,
    ));
    fixture.peers.extend([authority_worker, clock_worker]);
}

fn assert_native_send_policy_history(fixture: &Fixture) {
    let config = must(NodeRpcConfig::from_environment());
    let checkpoint = must(
        layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
            &fixture.endpoint,
            HEAD,
            None,
            "125",
        ),
    );
    let observed_core_ms = must(u64::try_from(checkpoint.header.time.seconds)) * 1_000
        + u64::from(checkpoint.header.time.nanos / 1_000_000);
    let observed_batch_id = checkpoint.header.block_hash();
    let trust = must(layerx_agentd::export::ExportTrustSource::new(
        must(fixture.endpoint.receipt_keys()),
        checkpoint.validators,
    ));
    let actor = must(Did::new(fixture.account.did().as_bytes()));
    let history = must(
        layerx_agentd::policy_history::read_policy_history_from_sources(
            &config,
            &config.url,
            &actor,
            &trust,
            layerx_agentd::policy_history::PolicyHistoryWindow::Time {
                head: HEAD,
                seconds: 10,
            },
        ),
    );
    let usage = must(history.time_window(&actor, 10));
    assert_eq!(usage.actor(), &actor);
    assert_eq!(usage.first_covered_sequence(), HEAD - 1);
    assert_eq!(usage.through_sequence(), HEAD);
    assert_eq!(usage.observed_core_ms(), observed_core_ms);
    assert_eq!(usage.observed_batch_id(), observed_batch_id);
    assert_eq!(usage.window_seconds(), 10);
    assert_eq!(usage.count(), 0);
    assert_eq!(usage.successful_count(), 0);
    assert!(usage.activity_ids().is_empty());
    eprintln!("repair_22_31_560: production authenticated_native_policy_usage inputs verified for the complete ten-second empty window");
}

fn with_fixture_lock(work: impl FnOnce() + std::panic::UnwindSafe) {
    let guard = must(FIXTURE_LOCK.lock());
    let outcome = std::panic::catch_unwind(work);
    drop(guard);
    if let Err(panic) = outcome {
        std::panic::resume_unwind(panic);
    }
}

#[test]
fn native_send_first_prepare_and_reopened_prepare_match_owner_preview() {
    with_fixture_lock(|| {
        let mut fixture = fixture_with_receipt_keys();
        let (human, grant) = fixture.request();
        let (canonical, preimage, purpose) = preview(&mut fixture, &human, &grant);
        assert_native_send_policy_history(&fixture);
        let response = prepare_rpc(&fixture, &human, purpose.clone(), &grant, 80);
        {
            let retained = must(fixture.owner.lock()).session_control.store();
            let store = must(retained.lock());
            let signing = must(SigningPreparation::restore(
                &store,
                fixture.credential.tenant().clone(),
                purpose.preparation_id,
            ));
            assert_eq!(signing.transaction.call.encode_unsigned(), canonical);
            assert_eq!(signing.transaction.signing_hash.as_slice(), preimage);
        }
        let first = successful_preparation(response);
        assert_eq!(first.preparation_id, purpose.preparation_id);
        assert_eq!(first.canonical_bytes, canonical);
        assert_eq!(first.signing_preimage.as_slice(), preimage);
        assert_durable(&fixture, purpose.preparation_id, &canonical);
        reopen_daemon(&mut fixture);
        assert_durable(&fixture, purpose.preparation_id, &canonical);
        let second = successful_preparation(prepare_rpc(&fixture, &human, purpose, &grant, 81));
        assert_eq!(second, first);
    });
}

#[test]
fn native_send_prepare_refuses_a_different_signed_preparation_identifier() {
    with_fixture_lock(|| {
        let mut fixture = fixture_with_receipt_keys();
        let (human, grant) = fixture.request();
        let (_, _, mut purpose) = preview(&mut fixture, &human, &grant);
        purpose.preparation_id[0] ^= 1;
        purpose.canonical_digest = purpose.preparation_id;
        let response = prepare_rpc(&fixture, &human, purpose, &grant, 82);
        assert_eq!(
            response.status,
            403,
            "{}",
            String::from_utf8_lossy(&response.body)
        );
        let response: Value = must(serde_json::from_slice(&response.body));
        assert_eq!(response["reason"], "owner.refused", "{response}");
    });
}
