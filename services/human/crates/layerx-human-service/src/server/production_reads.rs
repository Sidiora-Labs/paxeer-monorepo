//! Direct projections over durable Human journey and receipt owners.

use std::collections::{BTreeMap, BTreeSet};

use super::backend::{ApiFailure, BackendResponse, ScopedRequest};
use crate::activity::{
    verification_status, EntryDetail, EvidenceBundle, ExportError, Feed, VerificationStatus,
    VerifyError,
};
use crate::journeys::{
    JourneyEngine, JourneyKind, JourneyPhase, JourneyState, VerifiedLegEvidence,
};
use crate::notify::ActivityEntryId;
use crate::notify::JourneyId;
use crate::store::{PrincipalScope, Table};
use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_portable::PortableReceiptV2;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

pub(super) fn execute(
    scope: &PrincipalScope<'_>,
    request: &ScopedRequest<'_>,
) -> Option<Result<BackendResponse, ApiFailure>> {
    Some(match request.operation.name.as_str() {
        "journey.get" => journey_get(scope, request),
        "journey.list" => journey_list(scope, None),
        "journey.page" => journey_list(
            scope,
            request.path_parameters.get("cursor").map(String::as_str),
        ),
        "evidence.get" => super::ProductionComponents::evidence_get(scope, request),
        _ => return None,
    })
}

pub(super) fn activity_entry(
    scope: &PrincipalScope<'_>,
    request: &ScopedRequest<'_>,
) -> Result<BackendResponse, ApiFailure> {
    let id = ActivityEntryId::new(
        request
            .path_parameters
            .get("entry_id")
            .cloned()
            .ok_or_else(|| ApiFailure::invalid_request(Some("entry_id")))?,
    )
    .map_err(|_| ApiFailure::invalid_request(Some("entry_id")))?;
    Ok(response(activity_entry_json(scope, &id)?))
}

pub(super) fn activity_entry_json(
    scope: &PrincipalScope<'_>,
    id: &ActivityEntryId,
) -> Result<Value, ApiFailure> {
    activity_entry_json_with(scope, id, receipt_keys, || {
        layerx_network_gateway::GatewayEndpoint::from_environment()
            .and_then(|gateway| gateway.list_assets())
            .map_err(|_| ApiFailure::upstream_degraded())
    })
}

pub(super) fn activity_entry_json_with(
    scope: &PrincipalScope<'_>,
    id: &ActivityEntryId,
    keys: impl Fn() -> Result<Vec<ReceiptKeyEntry>, ApiFailure>,
    assets: impl Fn() -> Result<layerx_network_gateway::AssetTable, ApiFailure>,
) -> Result<Value, ApiFailure> {
    let entry = Feed::entry(scope, id)
        .map_err(|error| feed_failure(&error))?
        .ok_or_else(ApiFailure::not_found)?;
    let mut actuals = Vec::with_capacity(entry.receipts().len());
    let mut labels = BTreeMap::new();
    let mut finality = None;
    for receipt in entry.receipts() {
        let bytes = receipt
            .canonical()
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let digest =
            decode_digest(receipt.reference()).ok_or_else(ApiFailure::upstream_degraded)?;
        let row = Some(ReceiptRow {
            entry_id: id.clone(),
            canonical: bytes,
            rung: receipt.rung(),
        });
        labels.insert(
            receipt.reference().to_owned(),
            receipt_label_with_keys(scope, row, digest, &keys()?)?,
        );
        if receipt.rung() == Rung::Committed {
            let proof = committed_proof(scope, digest)?;
            let checkpoint = proof
                .checkpoint_id()
                .map_err(|_| ApiFailure::upstream_degraded())?;
            labels.insert(hex(&checkpoint), "committed");
            finality = Some(
                crate::activity::detail::FinalityReference::new(checkpoint, Rung::Committed)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
            );
        }
        actuals.push(
            crate::activity::detail::ReceiptActual::from_verified_journey_bytes(
                bytes,
                receipt.reference(),
            )
            .map_err(|_| ApiFailure::upstream_degraded())?,
        );
    }
    let detail = EntryDetail::assemble(&entry, actuals, finality)
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let assets = if detail.actuals().is_empty() {
        None
    } else {
        Some(assets()?)
    };
    activity_detail_json(&entry, &detail, &labels, assets.as_ref())
}

fn activity_detail_json(
    entry: &crate::activity::ActivityEntry,
    detail: &EntryDetail,
    labels: &BTreeMap<String, &str>,
    assets: Option<&layerx_network_gateway::AssetTable>,
) -> Result<Value, ApiFailure> {
    let id = entry.entry_id();
    let stages = detail
        .stages()
        .iter()
        .enumerate()
        .map(|(index, stage)| {
            Ok(json!({
                "stage_id": format!("stg_{}_{index}", id.as_str()),
                "copy_key": activity_stage_key(stage.label())?,
                "state": stage_state(stage.state()),
                "evidence": [],
            }))
        })
        .collect::<Result<Vec<_>, ApiFailure>>()?;
    let evidence = detail
        .evidence()
        .iter()
        .map(|link| {
            let verification = labels
                .get(link.reference())
                .copied()
                .ok_or_else(ApiFailure::unavailable)?;
            Ok(
                json!({"evidence_id":format!("evd_{}",link.reference()),"class":match link.kind() { crate::activity::detail::EvidenceKind::Receipt => "layerx-receipt", crate::activity::detail::EvidenceKind::Checkpoint => "checkpoint-proof" },"verification":verification}),
            )
        })
        .collect::<Result<Vec<_>, ApiFailure>>()?;
    let timestamp = crate::time::rfc3339(entry.occurred_at());
    if crate::time::seconds_from_rfc3339(&timestamp) != Some(entry.occurred_at()) {
        return Err(ApiFailure::upstream_degraded());
    }
    let mut result = json!({
        "entry_id": id.as_str(),
        "kind": activity_kind(detail.kind()),
        "state": activity_status(detail.status()),
        "state_copy_key": activity_state_key(detail.status()),
        "summary_copy_key": activity_summary_key(detail.kind(), detail.status()),
        "occurred_at": timestamp,
        "stages": stages,
        "evidence": evidence,
    });
    if let Some(actual) = detail.actuals().first() {
        let currency = activity_currency(
            assets.ok_or_else(ApiFailure::upstream_degraded)?,
            actual.asset(),
        )?;
        result["money"] = json!({"amount": actual.amount().to_string(), "currency": currency});
        result["fees"] = json!({"amount": actual.fee().to_string(), "currency": currency});
        match detail.kind() {
            crate::activity::ActivityKind::Deposit => result["direction"] = json!("in"),
            crate::activity::ActivityKind::Withdrawal => result["direction"] = json!("out"),
            _ => {}
        }
    }
    Ok(result)
}

fn activity_currency(
    assets: &layerx_network_gateway::AssetTable,
    asset: [u8; 32],
) -> Result<&str, ApiFailure> {
    let encoded = hex(&asset);
    let mut matching = assets.assets.iter().filter(|entry| {
        entry.asset_id == encoded
            || entry.asset_id == format!("0x{encoded}")
            || entry.denom.as_ref().is_some_and(|denom| {
                entry.asset_id == format!("denom:{denom}")
                    && <[u8; 32]>::from(Sha256::digest(denom.as_bytes())) == asset
            })
    });
    let entry = matching.next().ok_or_else(ApiFailure::upstream_degraded)?;
    if matching.next().is_some() {
        return Err(ApiFailure::upstream_degraded());
    }
    entry
        .symbol
        .as_deref()
        .filter(|symbol| symbol.len() == 3 && symbol.bytes().all(|byte| byte.is_ascii_uppercase()))
        .ok_or_else(ApiFailure::upstream_degraded)
}

fn activity_state_key(status: crate::activity::ActivityStatus) -> String {
    use crate::activity::{ActivityStatus as S, DepositStage as D, WithdrawalStage as W};
    let key = match status {
        S::Deposit(D::WaitingForWallet) => "waiting_for_wallet".to_owned(),
        S::Deposit(D::ConfirmingOnPaxeer) => "confirming_on_paxeer".to_owned(),
        S::Deposit(D::Crediting) => "crediting".to_owned(),
        S::Withdrawal(W::WaitingForSettlement) => "waiting_for_settlement".to_owned(),
        S::Withdrawal(W::ReadyToClaim) => "ready_to_claim".to_owned(),
        S::Withdrawal(W::PaidOut) => "paid_out".to_owned(),
        _ => activity_status(status).replace('-', "_"),
    };
    format!("status.{key}")
}

fn activity_summary_key(
    kind: crate::activity::ActivityKind,
    status: crate::activity::ActivityStatus,
) -> String {
    use crate::activity::{ActivityKind as K, ActivityStatus as S};
    if let S::DidntGoThrough { money_left } = status {
        return if money_left {
            "activity.refused.money_left"
        } else {
            "activity.refused.money_kept"
        }
        .to_owned();
    }
    let kind = match kind {
        K::Deposit => "deposit",
        K::Withdrawal => "withdrawal",
        K::Movement => "move",
        K::AgentAction => "agent",
        K::Approval => "approval",
        K::Security => "security",
    };
    let suffix = if matches!(activity_status(status), "done" | "done-finalised") {
        "summary"
    } else {
        "progress"
    };
    format!("activity.{kind}.{suffix}")
}

fn activity_stage_key(label: &str) -> Result<&'static str, ApiFailure> {
    match label {
        "Waiting for wallet" => Ok("status.waiting_for_wallet"),
        "Confirming on Paxeer" => Ok("status.confirming_on_paxeer"),
        "Crediting" => Ok("status.crediting"),
        "Done" => Ok("status.done"),
        "Processing" => Ok("status.processing"),
        "Waiting for settlement" => Ok("status.waiting_for_settlement"),
        "Ready to claim" => Ok("status.ready_to_claim"),
        "Paid out" => Ok("status.paid_out"),
        "Getting ready" => Ok("status.getting_ready"),
        "Sending" => Ok("status.sending"),
        "Waiting for you" => Ok("status.waiting_for_you"),
        _ => Err(ApiFailure::upstream_degraded()),
    }
}

#[cfg(test)]
mod activity_detail_contract_tests {
    use super::*;
    use crate::activity::{ActivityKind, AgentActivity, PendingStatus, VerifiedStatus};
    use crate::server::schema::ApiSchema;
    use crate::store::{
        AgentTenantId, PrincipalId, PrincipalStore, RetentionPeriod, RetentionPolicy, TenancyMap,
    };
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
    use layerx_agent_api::track::ReceiptRef;
    use layerx_proof::inclusion::SequencerAuthorization;
    use layerx_proof::receipt::{verify, AuthorizedBatch};
    use std::io::{BufRead as _, BufReader, Read as _, Write as _};
    use std::net::{TcpListener, TcpStream};
    use std::time::{Duration, Instant};

    fn pinned_activity_receipt(signature: Option<[u8; 64]>) -> Vec<u8> {
        let bytes = layerx_intents::vectors::receipt(None).expect("producer receipt");
        let decoded = layerx_wire::receipt::decode(&bytes).expect("canonical producer receipt");
        let receipt = decoded.protocol().expect("protocol receipt");
        let mut batch_id = [0; 32];
        batch_id[24..].copy_from_slice(&42_u64.to_be_bytes());
        let mut encoder = layerx_wire::encode::Encoder::new(4096);
        encoder
            .structure_header_version(0x5201, receipt.protocol_version())
            .expect("header");
        encoder.u16(receipt.protocol_version()).expect("version");
        encoder.bytes(&receipt.activity_id(), 32).expect("activity");
        encoder.u64(receipt.global_sequence()).expect("sequence");
        encoder
            .bytes(&receipt.previous_state_root(), 32)
            .expect("previous root");
        encoder
            .bytes(&receipt.resulting_state_root(), 32)
            .expect("resulting root");
        encoder
            .bytes(&receipt.activity_root(), 32)
            .expect("activity root");
        encoder.i32(receipt.result_code()).expect("result");
        assert!(receipt.effects().is_empty());
        encoder.sequence_length(0, 512).expect("effects");
        encoder.u128(receipt.fee_charged()).expect("fee");
        encoder.bytes(&batch_id, 32).expect("height-encoded batch");
        encoder.u16(receipt.module_id()).expect("module");
        encoder
            .u32(receipt.module_version())
            .expect("module version");
        encoder
            .u32(receipt.parameter_version())
            .expect("parameter version");
        encoder.u8(receipt.operation()).expect("operation");
        encoder.bytes(&receipt.asset(), 32).expect("asset");
        encoder.u128(receipt.amount()).expect("amount");
        encoder.bytes(&receipt.from(), 32).expect("from");
        encoder
            .u128(receipt.debit_balance_before())
            .expect("debit before");
        encoder
            .u128(receipt.debit_balance_after())
            .expect("debit after");
        encoder
            .u64(receipt.debit_sequence())
            .expect("debit sequence");
        encoder.bytes(&receipt.to(), 32).expect("to");
        encoder
            .u128(receipt.credit_balance_before())
            .expect("credit before");
        encoder
            .u128(receipt.credit_balance_after())
            .expect("credit after");
        encoder
            .bytes(&receipt.transfer_set_root(), 32)
            .expect("transfer root");
        encoder
            .bytes(&receipt.authorization_hash(), 32)
            .expect("authorization");
        encoder.bytes(&receipt.context_hash(), 32).expect("key id");
        encoder.u64(receipt.timestamp()).expect("timestamp");
        encoder
            .u8(u8::from(signature.is_some()))
            .expect("signature presence");
        if let Some(signature) = signature {
            encoder.bytes(&signature, 64).expect("signature");
        }
        encoder.finish()
    }

    #[test]
    fn activity_detail_http_contract() {
        let root =
            std::env::temp_dir().join(format!("activity-detail-contract-{}", std::process::id()));
        let principal = PrincipalId::new("activity-contract").expect("principal");
        let tenant = AgentTenantId::new("activity-contract").expect("tenant");
        let tenancy = TenancyMap::new([(principal.clone(), tenant)]).expect("tenancy");
        let digest = tenancy.install(&root).expect("install tenancy");
        let period = RetentionPeriod::new(86_400);
        let retention = RetentionPolicy {
            journeys: period,
            notifications: period,
            audit: period,
            telemetry: period,
            cache: period,
        };
        let mut store = PrincipalStore::open(&root, retention, digest).expect("open store");
        let mut scope = store.principal(&principal).expect("scope");
        let signer = SigningKey::from_bytes(&[0x16; 32]);
        let unsigned = pinned_activity_receipt(None);
        let signature = signer.sign(
            &layerx_intents::vectors::receipt_signing_digest(&unsigned).expect("receipt digest"),
        );
        let canonical = pinned_activity_receipt(Some(signature.to_bytes()));
        let keys = [SequencerAuthorization::new(
            [11; 32],
            signer.verifying_key().to_bytes(),
            41,
            43,
        )];
        let authority =
            AuthorizedBatch::from_receipt_keys(&canonical, &keys, [5; 32], [2; 32], [3; 32])
                .expect("receipt authority pinned to its key id and height");
        let verified = verify(&canonical, &authority).expect("verify genuine canonical receipt");
        assert_eq!(verified.rung(), Rung::Receipted);
        let actual = crate::activity::detail::ReceiptActual::from_verified(&verified)
            .expect("verified actuals");
        assert_eq!(actual.amount(), 25);
        assert_eq!(actual.fee(), 1);
        let mut assets = layerx_network_gateway::decode_assets(&json!({
            "height": "9", "joined_limit": "16", "assets": [{
                "asset_id": hex(&actual.asset()), "kind": "bank", "symbol": "LXP", "decimals": "18"
            }]
        }))
        .expect("asset catalog codec");
        assert_eq!(
            activity_currency(&assets, actual.asset()).expect("display symbol"),
            "LXP"
        );
        let original = assets.assets[0].clone();
        assets.assets.push(original.clone());
        assert!(activity_currency(&assets, actual.asset()).is_err());
        assets.assets.pop();
        for symbol in [None, Some("lxp".to_owned()), Some(hex(&actual.asset()))] {
            assets.assets[0].symbol = symbol;
            assert!(activity_currency(&assets, actual.asset()).is_err());
        }
        assets.assets[0] = original;
        assert!(activity_currency(&assets, [0xff; 32]).is_err());
        let bank_asset = Sha256::digest(b"alxp").into();
        assets.assets[0].asset_id = "denom:alxp".to_owned();
        assets.assets[0].denom = Some("alxp".to_owned());
        assert_eq!(
            activity_currency(&assets, bank_asset).expect("bank asset join"),
            "LXP"
        );
        assets.assets[0].asset_id = hex(&actual.asset());
        assets.assets[0].denom = None;

        let cases = [
            (
                "act_repairdeposit",
                ActivityKind::Deposit,
                PendingStatus::Processing,
                VerifiedStatus::DepositDone,
                true,
            ),
            (
                "act_repairwithdrawal",
                ActivityKind::Withdrawal,
                PendingStatus::Processing,
                VerifiedStatus::WithdrawalPaidOut,
                true,
            ),
            (
                "act_repairapproval",
                ActivityKind::Approval,
                PendingStatus::WaitingForYou,
                VerifiedStatus::Done,
                false,
            ),
            (
                "act_repairsecurity",
                ActivityKind::Security,
                PendingStatus::Processing,
                VerifiedStatus::Done,
                false,
            ),
            (
                "act_repairrefusedkept",
                ActivityKind::Movement,
                PendingStatus::DidntGoThrough { money_left: false },
                VerifiedStatus::Done,
                false,
            ),
            (
                "act_repairrefusedleft",
                ActivityKind::Movement,
                PendingStatus::DidntGoThrough { money_left: true },
                VerifiedStatus::Done,
                false,
            ),
        ];
        let schema = ApiSchema::v1().expect("real HTTP schema");
        let operation = schema
            .operation("activity.entry")
            .expect("activity operation");
        let mut responses = BTreeMap::new();
        for (index, (name, kind, pending, completed, monetary)) in cases.iter().copied().enumerate()
        {
            let id = ActivityEntryId::new(name).expect("entry id");
            let descriptor =
                AgentActivity::new(id.clone(), kind, None, 1_786_789_800, pending, completed)
                    .expect("activity");
            let reference = if monetary {
                ReceiptReference::Verified {
                    receipt_ref: ReceiptRef::new(actual.reference()).expect("receipt reference"),
                    rung: verified.rung(),
                }
            } else {
                ReceiptReference::None
            };
            let sequence = u8::try_from(index + 1).expect("case sequence");
            let delivery = EventDelivery::new(
                EventIdentity::new([sequence; 32]),
                canonical.clone(),
                Cursor {
                    block_height: u64::from(sequence),
                    tx_index: 0,
                    log_index: 0,
                },
                reference,
            )
            .expect("delivery");
            Feed::record_agent_event(&mut scope, &descriptor, &delivery, 1_786_789_801)
                .expect("durable feed producer");
            let entry = Feed::entry(&scope, &id)
                .expect("read feed")
                .expect("stored entry");
            let actuals = if monetary {
                vec![actual.clone()]
            } else {
                Vec::new()
            };
            let detail =
                EntryDetail::assemble(&entry, actuals, None).expect("real detail assembly");
            let receipt_keys = [ReceiptKeyEntry {
                key_id: [11; 32],
                public_key: signer.verifying_key().to_bytes(),
                first_height: 41,
                last_height: 43,
            }];
            let fact = EvidenceBundle::receipt_fact(&id, canonical.clone(), verified.rung())
                .expect("canonical receipt fact");
            let verification =
                verification_status(EvidenceBundle::verify_receipt(&fact, &receipt_keys));
            let label =
                verification_label(&verification).expect("independently verified receipt label");
            assert_eq!(label, "receipt-verified");
            let labels = BTreeMap::from([(actual.reference().to_owned(), label)]);
            let value = if monetary {
                assert!(activity_detail_json(&entry, &detail, &labels, None).is_err());
                activity_detail_json(&entry, &detail, &labels, Some(&assets))
                    .expect("production detail codec")
            } else {
                // Exercise the actual route projection as well: no metadata or receipt authority is needed.
                activity_entry_json(&scope, &id).expect("nonmonetary production detail")
            };
            schema
                .encode_response(operation, &value)
                .expect("actual HTTP response schema");
            assert_eq!(value.get("money").is_some(), monetary);
            assert_eq!(value.get("fees").is_some(), monetary);
            responses.insert(format!("/v1/activity/{name}"), value);
        }
        drop(scope);
        drop(store);
        std::fs::remove_dir_all(&root).expect("remove private store");

        let listener = TcpListener::bind("127.0.0.1:0").expect("bind HTTP codec host");
        let address = listener.local_addr().expect("HTTP address");
        listener.set_nonblocking(true).expect("bounded accept");
        let internal_client = std::thread::spawn(move || {
            let mut recordings = BTreeMap::new();
            for _ in 0..2 {
                for (name, ..) in cases {
                    let mut stream = TcpStream::connect(address).expect("connect real HTTP");
                    stream
                        .set_read_timeout(Some(Duration::from_secs(10)))
                        .expect("bounded client");
                    write!(stream, "GET /v1/activity/{name} HTTP/1.1\r\nHost: {address}\r\nConnection: close\r\n\r\n").expect("HTTP request");
                    let mut answer = String::new();
                    stream.read_to_string(&mut answer).expect("HTTP response");
                    assert!(answer.starts_with("HTTP/1.1 200 OK\r\n"));
                    let (_, body) = answer.split_once("\r\n\r\n").expect("HTTP body");
                    let envelope: Value = serde_json::from_str(body).expect("HTTP envelope");
                    assert_eq!(envelope["ok"], true);
                    assert_eq!(envelope["result"]["entry_id"], name);
                    if let Some(previous) =
                        recordings.insert(format!("/v1/activity/{name}"), answer.clone())
                    {
                        assert_eq!(previous, answer, "stable producer response");
                    }
                }
            }
            recordings
        });
        let started = Instant::now();
        for _ in 0..cases.len() * 2 {
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(
                            started.elapsed() < Duration::from_secs(60),
                            "HTTP consumer deadline"
                        );
                        std::thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("HTTP accept: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(10)))
                .expect("bounded request");
            stream
                .set_write_timeout(Some(Duration::from_secs(10)))
                .expect("bounded response");
            let mut reader = BufReader::new(&mut stream);
            let mut line = String::new();
            reader.read_line(&mut line).expect("request line");
            let path = line
                .split_whitespace()
                .nth(1)
                .expect("request path")
                .to_owned();
            loop {
                line.clear();
                assert_ne!(reader.read_line(&mut line).expect("request headers"), 0);
                if line == "\r\n" {
                    break;
                }
            }
            let route = schema
                .route("GET", &path)
                .expect("HTTP route")
                .expect("activity route");
            assert_eq!(route.operation.name, "activity.entry");
            let value = responses.get(&path).expect("requested producer output");
            schema
                .encode_response(route.operation, value)
                .expect("HTTP schema encoding");
            let body = serde_json::to_vec(
                &json!({"ok": true, "result": value, "trace": "trc_activity_detail_contract"}),
            )
            .expect("HTTP envelope");
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("HTTP headers");
            stream.write_all(&body).expect("HTTP detail output");
        }
        let recordings = internal_client.join().expect("HTTP consumer");
        assert_eq!(recordings.len(), cases.len());
        println!(
            "ACTIVITY_DETAIL_FIXTURE {}",
            serde_json::to_string(&recordings).expect("record genuine HTTP output")
        );
    }
}

fn journey_get(
    scope: &PrincipalScope<'_>,
    request: &ScopedRequest<'_>,
) -> Result<BackendResponse, ApiFailure> {
    let value = request
        .path_parameters
        .get("journey_id")
        .ok_or_else(|| ApiFailure::invalid_request(Some("journey_id")))?;
    let id = JourneyId::new(value.clone())
        .map_err(|_| ApiFailure::invalid_request(Some("journey_id")))?;
    let families = public_journeys(scope)?;
    let found = families
        .into_iter()
        .find(|entry| entry.id == id.as_str())
        .ok_or_else(ApiFailure::not_found)?;
    Ok(response(found.value))
}

struct PublicJourney {
    id: String,
    updated_at: u64,
    value: Value,
}

fn public_journeys(scope: &PrincipalScope<'_>) -> Result<Vec<PublicJourney>, ApiFailure> {
    let mut values = Vec::new();
    for entry in JourneyEngine::list(scope).map_err(|error| failure(&error))? {
        let status = entry.status().map_err(|error| failure(&error))?;
        values.push(PublicJourney {
            id: status.journey_id().as_str().to_owned(),
            updated_at: entry.updated_at(),
            value: native_journey_json(scope, &entry)?,
        });
    }
    let mut ids = BTreeSet::new();
    for entry in &values {
        if !ids.insert(&entry.id) {
            return Err(ApiFailure::upstream_degraded());
        }
    }
    values.sort_by(|left, right| {
        right
            .updated_at
            .cmp(&left.updated_at)
            .then_with(|| right.id.cmp(&left.id))
    });
    Ok(values)
}

fn journey_list(
    scope: &PrincipalScope<'_>,
    cursor: Option<&str>,
) -> Result<BackendResponse, ApiFailure> {
    journey_list_with(scope, cursor, Vec::new())
}

pub(super) fn journey_list_with(
    scope: &PrincipalScope<'_>,
    cursor: Option<&str>,
    additional: Vec<(u64, Value)>,
) -> Result<BackendResponse, ApiFailure> {
    const PAGE_SIZE: usize = 50;
    let mut journeys = public_journeys(scope)?;
    let mut additional = additional;
    for key in scope
        .keys(Table::Journeys)
        .into_iter()
        .filter(|key| key.as_str().starts_with("intent-evm-"))
    {
        let row = scope
            .get(Table::Journeys, &key)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let record: Value =
            serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
        additional.push((row.written_at(), intent_journey_json(&record)?));
    }
    for (updated_at, value) in additional {
        let id = value["journey_id"]
            .as_str()
            .ok_or_else(ApiFailure::upstream_degraded)?
            .to_owned();
        JourneyId::new(id.clone()).map_err(|_| ApiFailure::upstream_degraded())?;
        journeys.push(PublicJourney {
            id,
            updated_at,
            value,
        });
    }
    let mut ids = BTreeSet::new();
    for entry in &journeys {
        if !ids.insert(&entry.id) {
            return Err(ApiFailure::upstream_degraded());
        }
    }
    journeys.sort_by(|left, right| {
        right
            .updated_at
            .cmp(&left.updated_at)
            .then_with(|| right.id.cmp(&left.id))
    });
    let mut digest = Sha256::new();
    digest.update(b"layerx-human-journey-page/v1\0");
    digest.update(scope.principal().as_str().as_bytes());
    for entry in &journeys {
        digest.update(entry.updated_at.to_be_bytes());
        digest.update((entry.id.len() as u64).to_be_bytes());
        digest.update(entry.id.as_bytes());
        digest.update(Sha256::digest(
            serde_json::to_vec(&entry.value).map_err(|_| ApiFailure::upstream_degraded())?,
        ));
    }
    let snapshot = hex(&digest.finalize().into());
    let offset = match cursor {
        None | Some("cur_start") => 0,
        Some("cur_end") => journeys.len(),
        Some(value) => {
            if value.len() > 100 {
                return Err(ApiFailure::invalid_request(Some("cursor")));
            }
            let suffix = value
                .strip_prefix("cur_jrn1_")
                .ok_or_else(|| ApiFailure::invalid_request(Some("cursor")))?;
            let (bound, offset) = suffix
                .split_once('_')
                .ok_or_else(|| ApiFailure::invalid_request(Some("cursor")))?;
            let parsed = offset
                .parse::<usize>()
                .map_err(|_| ApiFailure::invalid_request(Some("cursor")))?;
            if bound != snapshot
                || offset != parsed.to_string()
                || parsed == 0
                || parsed >= journeys.len()
                || parsed % PAGE_SIZE != 0
            {
                return Err(ApiFailure::invalid_request(Some("cursor")));
            }
            parsed
        }
    };
    let through = offset.saturating_add(PAGE_SIZE).min(journeys.len());
    let values = journeys[offset..through]
        .iter()
        .map(|entry| entry.value.clone())
        .collect::<Vec<_>>();
    let next = if through == journeys.len() {
        "cur_end".to_owned()
    } else {
        format!("cur_jrn1_{snapshot}_{through}")
    };
    Ok(response(json!({"journeys": values, "next_cursor": next})))
}

impl super::ProductionComponents {
    pub fn managed_receipt_material(
        id: &str,
        evidence: &VerifiedLegEvidence,
    ) -> Result<Value, ApiFailure> {
        if id.strip_prefix("evd_").and_then(decode_digest) != Some(evidence.receipt_digest)
            || <[u8; 32]>::from(Sha256::digest(&evidence.canonical_receipt))
                != evidence.receipt_digest
            || evidence.canonical_receipt != evidence.signed_receipt
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let verification = match evidence.rung {
            Rung::Receipted => "receipt-verified",
            Rung::Committed => "committed",
            Rung::Submitted => return Err(ApiFailure::upstream_degraded()),
        };
        let key_entry = layerx_proof::inclusion::SequencerAuthorization::new(
            evidence.receipt_key.key_id,
            evidence.receipt_key.public_key,
            evidence.receipt_key.first_height,
            evidence.receipt_key.last_height,
        );
        let portable = PortableReceiptV2::export(
            &evidence.signed_receipt,
            &key_entry,
            evidence.inclusion_height,
        )
        .and_then(|receipt| receipt.to_json())
        .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(
            json!({"evidence_id":id,"class":"layerx-receipt","verification":verification,
            "content_type":"application/vnd.layerx.receipt-proof-v2+json","bytes_base64":STANDARD.encode(portable),
            "rung":evidence.rung.name(),"inclusion_height":evidence.inclusion_height}),
        )
    }

    pub fn evidence_get(
        scope: &PrincipalScope<'_>,
        request: &ScopedRequest<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        evidence_get(scope, request)
    }
}

fn evidence_get(
    scope: &PrincipalScope<'_>,
    request: &ScopedRequest<'_>,
) -> Result<BackendResponse, ApiFailure> {
    let id = request
        .path_parameters
        .get("evidence_id")
        .ok_or_else(|| ApiFailure::invalid_request(Some("evidence_id")))?;
    let expected = id
        .strip_prefix("evd_")
        .and_then(decode_digest)
        .ok_or_else(|| ApiFailure::invalid_request(Some("evidence_id")))?;
    let export_key = crate::store::RowKey::new(format!("activity-export-{}", hex(&expected)))
        .map_err(|_| ApiFailure::invalid_request(Some("evidence_id")))?;
    if let Some(row) = scope.get(crate::store::Table::Cache, &export_key) {
        return Ok(response(
            json!({"evidence_id": id, "class": "local-journey-state", "verification": "unverified",
            "content_type": "text/csv; charset=utf-8", "bytes_base64": STANDARD.encode(row.bytes())}),
        ));
    }
    let bundle_key = crate::store::RowKey::new(format!("activity-evidence-{}", hex(&expected)))
        .map_err(|_| ApiFailure::invalid_request(Some("evidence_id")))?;
    if let Some(row) = scope.get(crate::store::Table::Cache, &bundle_key) {
        let bundle =
            EvidenceBundle::decode(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
        let receipt_keys = receipt_keys()?;
        let status = verification_status(bundle.verify(expected, scope.principal(), &receipt_keys));
        let verification = verification_label(&status)?;
        return Ok(response(
            json!({"evidence_id":id,"class":"local-journey-state","verification":verification,"content_type":"application/vnd.layerx.evidence-bundle","bytes_base64":STANDARD.encode(row.bytes())}),
        ));
    }
    let cache_key = crate::store::RowKey::new(format!("state-proof-{}", hex(&expected)))
        .map_err(|_| ApiFailure::invalid_request(Some("evidence_id")))?;
    if let Some(row) = scope.get(crate::store::Table::Cache, &cache_key) {
        let (class, verification) = cached_balance_verification(row.bytes(), expected)?;
        return Ok(response(
            json!({"evidence_id": id, "class": class, "verification": verification,
            "content_type": "application/vnd.layerx.state-proof", "bytes_base64": STANDARD.encode(row.bytes())}),
        ));
    }
    if let Some(journey) = crate::onboarding::OnboardingJourney::load(scope)
        .map_err(|_| ApiFailure::upstream_degraded())?
    {
        for stage in journey.status().stages() {
            for pointer in stage.evidence() {
                if let Some(row) = scope.get(Table::Journeys, pointer.row()) {
                    if <[u8; 32]>::from(Sha256::digest(row.bytes())) == expected {
                        let (class, verification, content_type) = match pointer.class() {
                            crate::onboarding::EvidenceClass::LayerxReceipt => (
                                "layerx-receipt",
                                "receipt-verified",
                                "application/vnd.layerx.receipt",
                            ),
                            crate::onboarding::EvidenceClass::SubmissionRecord => (
                                "submission-record",
                                "unverified",
                                "application/vnd.layerx.submission",
                            ),
                            crate::onboarding::EvidenceClass::LocalJourneyState
                            | crate::onboarding::EvidenceClass::CustodyKey => (
                                "local-journey-state",
                                "unverified",
                                "application/octet-stream",
                            ),
                        };
                        return Ok(response(
                            json!({"evidence_id":id,"class":class,"verification":verification,
                            "content_type":content_type,"bytes_base64":STANDARD.encode(row.bytes())}),
                        ));
                    }
                }
            }
        }
    }
    if let Some((entry_id, evidence)) = journey_receipt(scope, expected)? {
        let verification = receipt_label(
            scope,
            Some(ReceiptRow {
                entry_id,
                canonical: &evidence.canonical_receipt,
                rung: evidence.rung,
            }),
            expected,
        )?;
        if verification != "receipt-verified" {
            return Err(ApiFailure::upstream_degraded());
        }
        return Ok(response(
            super::ProductionComponents::managed_receipt_material(id, &evidence)?,
        ));
    }
    let mut records = Vec::new();
    for key in scope
        .keys(Table::Journeys)
        .into_iter()
        .filter(|key| key.as_str().starts_with("intent-evm-"))
    {
        let row = scope
            .get(Table::Journeys, &key)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        records.push(
            serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?,
        );
    }
    if let Some(answer) = intent_evm_receipt(&records, expected, id)? {
        return Ok(response(answer));
    }
    if let Some((class, verification, content_type, bytes)) =
        intent_evm_evidence(&records, expected)
    {
        return Ok(response(
            json!({"evidence_id": id, "class": class, "verification": verification,
            "content_type": content_type, "bytes_base64": STANDARD.encode(bytes)}),
        ));
    }
    Err(ApiFailure::not_found())
}

fn intent_evm_receipt(
    records: &[Value],
    digest: [u8; 32],
    id: &str,
) -> Result<Option<Value>, ApiFailure> {
    use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
    use layerx_client::sequencer::SequencerReceipt;

    for leg in records
        .iter()
        .filter_map(|record| record["legs"].as_array())
        .flatten()
    {
        let matches = |name: &str| {
            leg[name]
                .as_str()
                .and_then(|value| value.strip_prefix("0x"))
                .and_then(decode_digest)
                == Some(digest)
        };
        if !matches("tx_hash") && !matches("receipt_hash") {
            continue;
        }
        let raw: Vec<u8> = serde_json::from_value(leg["raw_transaction"].clone())
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
        let configuration =
            NodeRpcConfig::from_environment().map_err(|_| ApiFailure::unavailable())?;
        let node = NodeRpc::connect(&configuration).map_err(|_| ApiFailure::unavailable())?;
        let answer = node
            .sequencer_receipt(&hash)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let (receipt, landing) = match answer {
            None | Some(SequencerReceipt::Submitted { .. }) => {
                if leg["verified"] == true || leg["refused"] == true {
                    return Err(ApiFailure::upstream_degraded());
                }
                return Ok(None);
            }
            Some(SequencerReceipt::Receipted(receipt)) => (receipt, None),
            Some(SequencerReceipt::Committed {
                receipt,
                block_number,
                block_hash,
            }) => (receipt, Some((block_number, block_hash))),
        };
        let keys = node.receipt_keys().map_err(|_| ApiFailure::unavailable())?;
        let key = keys
            .iter()
            .find(|key| key.key_id == receipt.key_id && key.public_key == receipt.signing_key)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let authorization = layerx_proof::inclusion::SequencerAuthorization::new(
            key.key_id,
            key.public_key,
            key.first_height,
            key.last_height,
        );
        let (rung, inclusion_height) = match landing {
            None => (Rung::Receipted, receipt.height),
            Some((height, block_hash)) => {
                let block = node
                    .block_by_number(height)
                    .map_err(|_| ApiFailure::upstream_degraded())?
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                let index =
                    usize::try_from(receipt.index).map_err(|_| ApiFailure::upstream_degraded())?;
                if block.hash != block_hash
                    || block.transactions.get(index) != Some(&raw)
                    || height < receipt.height
                {
                    return Err(ApiFailure::upstream_degraded());
                }
                (Rung::Committed, height)
            }
        };
        let verification = match rung {
            Rung::Receipted => "receipt-verified",
            Rung::Committed => "committed",
            Rung::Submitted => return Err(ApiFailure::upstream_degraded()),
        };
        let portable = PortableReceiptV2::export_with_transaction(
            &receipt.receipt,
            &authorization,
            inclusion_height,
            &raw,
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let verified = portable
            .verify_v2(&[authorization])
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let signed = layerx_wire::receipt::decode(&receipt.receipt)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let protocol = signed
            .protocol()
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let mut signed_height = [0_u8; 32];
        signed_height[24..].copy_from_slice(&receipt.height.to_be_bytes());
        let (call, _) = layerx_crypto::evm_transaction::decode_signed(&raw)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        if verified.activity_id() != hash
            || protocol.batch_id() != signed_height
            || protocol.debit_sequence() != call.nonce
            || receipt.nonce != call.nonce
            || (!matches("tx_hash") && Sha256::digest(&receipt.receipt)[..] != digest)
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let bytes = portable
            .to_json()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        return Ok(Some(json!({"evidence_id": id, "class": "layerx-receipt",
            "verification": verification, "rung": rung.name(), "inclusion_height": inclusion_height,
            "content_type": "application/vnd.layerx.receipt-proof-v2+json",
            "bytes_base64": STANDARD.encode(bytes)})));
    }
    Ok(None)
}

fn intent_evm_evidence(
    records: &[Value],
    digest: [u8; 32],
) -> Option<(&'static str, &'static str, &'static str, Vec<u8>)> {
    let matches_digest = |value: &Value| {
        value
            .as_str()
            .and_then(|hash| hash.strip_prefix("0x"))
            .and_then(|hash| decode_digest(&hash.to_ascii_lowercase()))
            == Some(digest)
    };
    for leg in records
        .iter()
        .filter_map(|record| record["legs"].as_array())
        .flatten()
    {
        if leg["verified"] == true || leg["refused"] == true {
            if matches_digest(&leg["receipt_hash"]) {
                if let Ok(bytes) = serde_json::from_value::<Vec<u8>>(leg["receipt"].clone()) {
                    if Sha256::digest(&bytes)[..] == digest {
                        return Some((
                            "layerx-receipt",
                            "receipt-verified",
                            "application/vnd.layerx.receipt",
                            bytes,
                        ));
                    }
                }
            }
        } else if leg["submitted"] == true && matches_digest(&leg["tx_hash"]) {
            if let Ok(bytes) = serde_json::from_value::<Vec<u8>>(leg["raw_transaction"].clone()) {
                return Some((
                    "submission-record",
                    "unverified",
                    "application/vnd.layerx.evm-transaction",
                    bytes,
                ));
            }
        }
    }
    None
}

pub(super) fn intent_journey_json(record: &Value) -> Result<Value, ApiFailure> {
    let id = record["journey_id"]
        .as_str()
        .ok_or_else(ApiFailure::upstream_degraded)?;
    JourneyId::new(id.to_owned()).map_err(|_| ApiFailure::upstream_degraded())?;
    let state = record["state"]
        .as_str()
        .filter(|state| matches!(*state, "processing" | "done" | "refused" | "still-checking"))
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let started_at = record["started_at"]
        .as_str()
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let updated_at = record["updated_at"]
        .as_str()
        .ok_or_else(ApiFailure::upstream_degraded)?;
    crate::time::seconds_from_rfc3339(started_at).ok_or_else(ApiFailure::upstream_degraded)?;
    crate::time::seconds_from_rfc3339(updated_at).ok_or_else(ApiFailure::upstream_degraded)?;
    let legs = record["legs"]
        .as_array()
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let mut stages = Vec::with_capacity(legs.len());
    let mut evidence = Vec::with_capacity(legs.len());
    for (index, leg) in legs.iter().enumerate() {
        let hash = leg["tx_hash"]
            .as_str()
            .ok_or_else(ApiFailure::upstream_degraded)?;
        hash.strip_prefix("0x")
            .filter(|hex| hex.len() == 64 && hex.bytes().all(|byte| byte.is_ascii_hexdigit()))
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let nonce = leg["nonce"]
            .as_u64()
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let (phase, stage_state, class, verification) = if leg["refused"] == true {
            ("refused", "refused", "layerx-receipt", "receipt-verified")
        } else if leg["verified"] == true {
            ("confirmed", "done", "layerx-receipt", "receipt-verified")
        } else if leg["submitted"] == true {
            ("submitted", "processing", "submission-record", "unverified")
        } else {
            return Err(ApiFailure::upstream_degraded());
        };
        let evidence_hash = if leg["refused"] == true || leg["verified"] == true {
            leg["receipt_hash"]
                .as_str()
                .ok_or_else(ApiFailure::upstream_degraded)?
        } else {
            hash
        };
        let digest = evidence_hash
            .strip_prefix("0x")
            .filter(|hex| hex.len() == 64 && hex.bytes().all(|byte| byte.is_ascii_hexdigit()))
            .ok_or_else(ApiFailure::upstream_degraded)?
            .to_ascii_lowercase();
        let reference = json!({"evidence_id": format!("evd_{digest}"), "class": class, "verification": verification});
        stages.push(json!({"stage_id": format!("stg_{id}_{index}"), "copy_key": format!("journey.stage.{phase}"),
            "state": stage_state, "evidence": [reference.clone()], "tx_hash": hash, "nonce": nonce}));
        evidence.push(reference);
    }
    Ok(
        json!({"journey_id": id, "kind": "move", "state": state, "state_copy_key": format!("journey.state.{state}"),
        "stages": stages, "evidence": evidence, "started_at": started_at, "updated_at": updated_at}),
    )
}

pub(super) fn journey_json(
    scope: &PrincipalScope<'_>,
    journey: &JourneyEngine,
) -> Result<Value, ApiFailure> {
    native_journey_json(scope, journey)
}

fn native_journey_json(
    scope: &PrincipalScope<'_>,
    journey: &JourneyEngine,
) -> Result<Value, ApiFailure> {
    native_journey_json_with(scope, journey, receipt_keys)
}

pub(super) fn native_journey_json_with(
    scope: &PrincipalScope<'_>,
    journey: &JourneyEngine,
    keys: impl Fn() -> Result<Vec<ReceiptKeyEntry>, ApiFailure>,
) -> Result<Value, ApiFailure> {
    let status = journey.status().map_err(|error| failure(&error))?;
    if status.state() == JourneyState::Closed {
        return Ok(json!({
            "journey_id": status.journey_id().as_str(),
            "kind": journey.historical_kind().ok_or_else(ApiFailure::upstream_degraded)?,
            "state": "closed", "state_copy_key": "journey.state.closed",
            "stages": [], "evidence": [],
            "started_at": crate::time::rfc3339(journey.started_at()),
            "updated_at": crate::time::rfc3339(journey.updated_at()),
        }));
    }
    let entry_id = crate::activity::stable_entry_id(status.journey_id().as_str())
        .map_err(|error| feed_failure(&error))?;
    let mut references = Vec::with_capacity(status.phases().len());
    for index in 0..status.phases().len() {
        let Some(digest) = status.receipt_digests().get(index).copied().flatten() else {
            references.push(None);
            continue;
        };
        let material = journey
            .verified_leg_evidence(index)
            .map_err(|error| failure(&error))?;
        let row = material.as_ref().map(|evidence| ReceiptRow {
            entry_id: entry_id.clone(),
            canonical: &evidence.canonical_receipt,
            rung: evidence.rung,
        });
        let verification = receipt_label_with_keys(scope, row, digest, &keys()?)?;
        references.push(Some(evidence_ref(digest, verification)));
    }
    let stages = status.phases().iter().enumerate().map(|(index, phase)| {
        let evidence = references.get(index).cloned().flatten().into_iter().collect::<Vec<_>>();
        json!({"stage_id": format!("stg_{}_{index}", status.journey_id().as_str()),
            "copy_key": format!("journey.stage.{}", phase_label(*phase)), "state": phase_state(*phase), "evidence": evidence})
    }).collect::<Vec<_>>();
    let evidence = references.iter().flatten().cloned().collect::<Vec<_>>();
    let state = state_label(status.state());
    Ok(
        json!({"journey_id": status.journey_id().as_str(), "kind": journey.kind().map(kind_label).or_else(|| journey.historical_kind()).ok_or_else(ApiFailure::upstream_degraded)?, "state": state,
        "state_copy_key": format!("journey.state.{state}"), "stages": stages, "evidence": evidence,
        "started_at": crate::time::rfc3339(journey.started_at()), "updated_at": crate::time::rfc3339(journey.updated_at())}),
    )
}

/// Labels one custody receipt digest through the verifier over the receipt
/// the service holds for it.
pub(super) fn custody_receipt_label(
    scope: &PrincipalScope<'_>,
    digest: [u8; 32],
) -> Result<&'static str, ApiFailure> {
    let found = journey_receipt(scope, digest)?;
    let row = found.as_ref().map(|(entry_id, evidence)| ReceiptRow {
        entry_id: entry_id.clone(),
        canonical: &evidence.canonical_receipt,
        rung: Rung::Receipted,
    });
    receipt_label(scope, row, digest)
}

/// Maps a verifier status onto the wire verification level; an unavailable
/// verdict has no wire level and refuses the read instead.
pub(super) fn verification_label(status: &VerificationStatus) -> Result<&'static str, ApiFailure> {
    if status.is_unavailable() {
        Err(ApiFailure::unavailable())
    } else {
        Ok(status.label())
    }
}

fn receipt_keys() -> Result<Vec<ReceiptKeyEntry>, ApiFailure> {
    let configuration = layerx_agentd::node_rpc::NodeRpcConfig::from_environment()
        .map_err(|_| ApiFailure::unavailable())?;
    layerx_agentd::node_rpc::NodeRpc::connect(&configuration)
        .and_then(|node| node.receipt_keys())
        .map_err(|_| ApiFailure::unavailable())
}

struct ReceiptRow<'a> {
    entry_id: ActivityEntryId,
    canonical: &'a [u8],
    rung: Rung,
}

fn receipt_label(
    _scope: &PrincipalScope<'_>,
    row: Option<ReceiptRow<'_>>,
    digest: [u8; 32],
) -> Result<&'static str, ApiFailure> {
    receipt_label_with_keys(_scope, row, digest, &receipt_keys()?)
}

fn committed_proof(
    scope: &PrincipalScope<'_>,
    digest: [u8; 32],
) -> Result<crate::journeys::BankCommitEvidence, ApiFailure> {
    for journey in JourneyEngine::list(scope).map_err(|error| failure(&error))? {
        for (index, observed) in journey
            .status()
            .map_err(|error| failure(&error))?
            .receipt_digests()
            .iter()
            .enumerate()
        {
            if *observed == Some(digest) {
                return journey
                    .bank_commit_evidence(index)
                    .cloned()
                    .ok_or_else(ApiFailure::upstream_degraded);
            }
        }
    }
    Err(ApiFailure::upstream_degraded())
}

fn receipt_label_with_keys(
    scope: &PrincipalScope<'_>,
    row: Option<ReceiptRow<'_>>,
    digest: [u8; 32],
    keys: &[ReceiptKeyEntry],
) -> Result<&'static str, ApiFailure> {
    if let Some(row) = &row {
        if row.rung == Rung::Committed {
            if <[u8; 32]>::from(Sha256::digest(row.canonical)) != digest {
                return Err(ApiFailure::upstream_degraded());
            }
            let signed = EvidenceBundle::receipt_fact(
                &row.entry_id,
                row.canonical.to_vec(),
                Rung::Receipted,
            )
            .and_then(|fact| EvidenceBundle::verify_receipt(&fact, keys));
            if verification_label(&verification_status(signed))? != "receipt-verified" {
                return Err(ApiFailure::upstream_degraded());
            }
            let decoded = layerx_wire::receipt::decode(row.canonical)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            let receipt = decoded
                .protocol()
                .ok_or_else(ApiFailure::upstream_degraded)?;
            committed_proof(scope, digest)?
                .verify_retained(receipt, keys)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            return Ok("committed");
        }
    }
    let status = match row {
        Some(row) => verification_status(
            EvidenceBundle::receipt_fact(&row.entry_id, row.canonical.to_vec(), row.rung).and_then(
                |mut fact| {
                    fact.receipt_hash = digest;
                    EvidenceBundle::verify_receipt(&fact, keys)
                },
            ),
        ),
        None => verification_status(Err(VerifyError::Unavailable(
            ExportError::AuthorityUnavailable {
                reference: hex(&digest),
            },
        ))),
    };
    verification_label(&status)
}

fn journey_receipt(
    scope: &PrincipalScope<'_>,
    digest: [u8; 32],
) -> Result<Option<(ActivityEntryId, VerifiedLegEvidence)>, ApiFailure> {
    for journey in JourneyEngine::list(scope).map_err(|error| failure(&error))? {
        let status = journey.status().map_err(|error| failure(&error))?;
        for index in 0..status.phases().len() {
            let Some(evidence) = journey
                .verified_leg_evidence(index)
                .map_err(|error| failure(&error))?
            else {
                continue;
            };
            if evidence.receipt_digest == digest {
                let entry_id = crate::activity::stable_entry_id(status.journey_id().as_str())
                    .map_err(|error| feed_failure(&error))?;
                return Ok(Some((entry_id, evidence)));
            }
        }
    }
    Ok(None)
}

fn evidence_ref(digest: [u8; 32], verification: &'static str) -> Value {
    json!({"evidence_id": format!("evd_{}", hex(&digest)), "class": "layerx-receipt", "verification": verification})
}
fn phase_label(value: JourneyPhase) -> &'static str {
    match value {
        JourneyPhase::Compiled => "compiled",
        JourneyPhase::Preparing => "preparing",
        JourneyPhase::AwaitingApproval => "awaiting-approval",
        JourneyPhase::Prepared => "prepared",
        JourneyPhase::Signed => "signed",
        JourneyPhase::Submitted => "submitted",
        JourneyPhase::StillChecking => "still-checking",
        JourneyPhase::ReceiptVerified => "receipt-verified",
        JourneyPhase::Refused => "refused",
    }
}
fn phase_state(value: JourneyPhase) -> &'static str {
    match value {
        JourneyPhase::AwaitingApproval => "awaiting-approval",
        JourneyPhase::Compiled | JourneyPhase::Preparing | JourneyPhase::Prepared => {
            "getting-ready"
        }
        JourneyPhase::Signed => "sending",
        JourneyPhase::Submitted => "processing",
        JourneyPhase::StillChecking => "still-checking",
        JourneyPhase::ReceiptVerified => "done",
        JourneyPhase::Refused => "refused",
    }
}
fn state_label(value: JourneyState) -> &'static str {
    match value {
        JourneyState::GettingReady => "getting-ready",
        JourneyState::Sending => "sending",
        JourneyState::Processing => "processing",
        JourneyState::StillChecking => "still-checking",
        JourneyState::Done => "done",
        JourneyState::Refused => "refused",
        JourneyState::Closed => "closed",
    }
}
fn kind_label(value: JourneyKind) -> &'static str {
    match value {
        JourneyKind::Onboarding => "onboarding",
        JourneyKind::Move => "move",
        JourneyKind::AgentCreate => "agent-create",
        JourneyKind::AgentFund => "agent-fund",
        JourneyKind::AgentPause => "agent-pause",
        JourneyKind::AgentRetire => "agent-retire",
    }
}
fn decode_digest(value: &str) -> Option<[u8; 32]> {
    if value.len() != 64 {
        return None;
    }
    let mut out = [0; 32];
    for (i, p) in value.as_bytes().chunks_exact(2).enumerate() {
        out[i] = nibble(p[0])? << 4 | nibble(p[1])?;
    }
    Some(out)
}
fn nibble(value: u8) -> Option<u8> {
    match value {
        b'0'..=b'9' => Some(value - b'0'),
        b'a'..=b'f' => Some(value - b'a' + 10),
        _ => None,
    }
}
fn hex(value: &[u8; 32]) -> String {
    const D: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(64);
    for byte in value {
        out.push(char::from(D[usize::from(byte >> 4)]));
        out.push(char::from(D[usize::from(byte & 15)]));
    }
    out
}
fn response(result: Value) -> BackendResponse {
    BackendResponse {
        result,
        session: None,
    }
}
fn failure(error: &crate::journeys::JourneyError) -> ApiFailure {
    match error {
        crate::journeys::JourneyError::Store(_) => ApiFailure::unavailable(),
        _ => ApiFailure::upstream_degraded(),
    }
}
fn feed_failure(error: &crate::activity::FeedError) -> ApiFailure {
    match error {
        crate::activity::FeedError::Store(_) => ApiFailure::unavailable(),
        _ => ApiFailure::upstream_degraded(),
    }
}
fn activity_kind(value: crate::activity::ActivityKind) -> &'static str {
    match value {
        crate::activity::ActivityKind::Deposit => "deposit",
        crate::activity::ActivityKind::Withdrawal => "withdrawal",
        crate::activity::ActivityKind::Movement => "movement",
        crate::activity::ActivityKind::AgentAction => "agent-action",
        crate::activity::ActivityKind::Approval => "approval",
        crate::activity::ActivityKind::Security => "security-event",
    }
}
fn activity_status(value: crate::activity::ActivityStatus) -> &'static str {
    use crate::activity::{ActivityStatus as S, DepositStage as D, WithdrawalStage as W};
    match value {
        S::GettingReady => "getting-ready",
        S::Sending => "sending",
        S::Processing
        | S::Deposit(D::ConfirmingOnPaxeer | D::Crediting)
        | S::Withdrawal(W::Processing | W::WaitingForSettlement) => "processing",
        S::StillChecking => "still-checking",
        S::WaitingForYou | S::Deposit(D::WaitingForWallet) | S::Withdrawal(W::ReadyToClaim) => {
            "waiting-for-you"
        }
        S::Done | S::Deposit(D::Done) | S::Withdrawal(W::PaidOut) => "done",
        S::DoneFinalised => "done-finalised",
        S::DidntGoThrough { .. } => "refused",
    }
}
fn stage_state(value: crate::activity::detail::StageState) -> &'static str {
    match value {
        crate::activity::detail::StageState::Complete => "done",
        crate::activity::detail::StageState::Current => "processing",
        crate::activity::detail::StageState::Upcoming => "getting-ready",
        crate::activity::detail::StageState::Failed => "refused",
    }
}

fn cached_balance_verification(
    bytes: &[u8],
    expected: [u8; 32],
) -> Result<(&'static str, &'static str), ApiFailure> {
    if Sha256::digest(bytes)[..] != expected {
        return Err(ApiFailure::upstream_degraded());
    }
    match super::production_components::AccountBalance::material_rung(bytes)? {
        None | Some(Rung::Submitted) => Ok(("submission-record", "unverified")),
        Some(Rung::Receipted) => Ok(("layerx-receipt", "receipt-verified")),
        Some(Rung::Committed) => Ok(("checkpoint-proof", "committed")),
    }
}

#[cfg(test)]
mod intent_evm_evidence_tests {
    use super::{hex, intent_evm_evidence, intent_journey_json, ApiFailure};
    use serde_json::{json, Value};
    use sha2::{Digest as _, Sha256};

    #[test]
    fn intent_evm_evidence_references_resolve_to_their_material() {
        let receipt = b"canonical intent EVM receipt".to_vec();
        let receipt_hash: [u8; 32] = Sha256::digest(&receipt).into();
        let raw_transaction = layerx_crypto::evm_transaction::Eip1559Call {
            chain_id: 125,
            nonce: 1,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 21_000,
            to: [3; 20],
            data: Vec::new(),
        }
        .sign(&[1; 32])
        .expect("signed EVM transaction");
        let tx_hash = layerx_crypto::evm_transaction::transaction_hash(&raw_transaction);
        let mut record = json!({
            "journey_id": "jrn_intentevidence",
            "state": "still-checking",
            "started_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:01Z",
            "legs": [
                {"tx_hash": format!("0x{}", hex(&[4; 32])), "nonce": 0,
                 "submitted": true, "verified": true,
                 "receipt_hash": format!("0x{}", hex(&receipt_hash)), "receipt": receipt},
                {"tx_hash": format!("0x{}", hex(&tx_hash)), "nonce": 1,
                 "submitted": true, "verified": false, "raw_transaction": raw_transaction}
            ]
        });
        let receipt_reference = json!({"evidence_id": format!("evd_{}", hex(&receipt_hash)),
            "class": "layerx-receipt", "verification": "receipt-verified"});
        let submission_reference = json!({"evidence_id": format!("evd_{}", hex(&tx_hash)),
            "class": "submission-record", "verification": "unverified"});
        let journey = intent_journey_json(&record).expect("intent journey projection");
        assert_eq!(journey["stages"][0]["evidence"], json!([receipt_reference]));
        assert_eq!(
            journey["stages"][1]["evidence"],
            json!([submission_reference])
        );
        assert_eq!(
            journey["evidence"],
            json!([receipt_reference, submission_reference])
        );
        let receipt_material = (
            "layerx-receipt",
            "receipt-verified",
            "application/vnd.layerx.receipt",
            receipt,
        );
        assert_eq!(
            intent_evm_evidence(std::slice::from_ref(&record), receipt_hash),
            Some(receipt_material.clone())
        );
        assert_eq!(
            intent_evm_evidence(std::slice::from_ref(&record), tx_hash),
            Some((
                "submission-record",
                "unverified",
                "application/vnd.layerx.evm-transaction",
                raw_transaction
            ))
        );
        assert_eq!(
            intent_evm_evidence(std::slice::from_ref(&record), [4; 32]),
            None
        );

        record["legs"][0]["verified"] = json!(false);
        record["legs"][0]["refused"] = json!(true);
        let refused = intent_journey_json(&record).expect("refused intent journey projection");
        assert_eq!(refused["stages"][0]["state"], "refused");
        assert_eq!(refused["stages"][0]["evidence"], json!([receipt_reference]));
        assert_eq!(
            intent_evm_evidence(std::slice::from_ref(&record), receipt_hash),
            Some(receipt_material)
        );
        record["legs"][0]["receipt"] = json!(b"tampered receipt".to_vec());
        assert_eq!(
            intent_evm_evidence(std::slice::from_ref(&record), receipt_hash),
            None
        );
        for invalid in [
            Value::Null,
            json!("0x12"),
            json!(format!("0x{}", "gg".repeat(32))),
        ] {
            record["legs"][0]["receipt_hash"] = invalid;
            assert_eq!(
                intent_journey_json(&record),
                Err(ApiFailure::upstream_degraded())
            );
        }
    }
}

#[cfg(test)]
mod balance_evidence_tests {
    use super::cached_balance_verification;
    use sha2::{Digest as _, Sha256};

    #[test]
    fn cached_balance_preserves_achieved_level_and_authenticates_the_label() {
        use super::super::agent_runtime::VerifiedBalance;
        use super::super::production_components::AccountBalance;
        use layerx_types::rung::Rung;
        for (rung, class, verification) in [
            (None, "submission-record", "unverified"),
            (Some(Rung::Submitted), "submission-record", "unverified"),
            (Some(Rung::Receipted), "layerx-receipt", "receipt-verified"),
            (Some(Rung::Committed), "checkpoint-proof", "committed"),
        ] {
            let balance = AccountBalance::from_agent(VerifiedBalance {
                account: [1; 32],
                asset: [2; 32],
                currency: "LXP".into(),
                observed_at: "2023-11-14T22:13:20Z".into(),
                age_seconds: 0,
                amount: 123,
                verification: rung,
                global_sequence: 10,
                batch_number: 1,
                observed_head_sequence: 10,
                observed_checkpoint: [0; 32],
                canonical_bytes: 123u128.to_be_bytes().to_vec(),
                proof_material: vec![1; 32],
            })
            .expect("production balance");
            let mut bytes = balance.evidence_material().expect("production material");
            let digest = Sha256::digest(&bytes).into();
            assert_eq!(
                cached_balance_verification(&bytes, digest).ok(),
                Some((class, verification))
            );
            bytes[5] ^= 1;
            assert!(cached_balance_verification(&bytes, digest).is_err());
        }
        for bytes in [
            b"legacy evidence".as_slice(),
            b"LXHB1",
            b"LXHB1\0",
            b"LXHB1\x06",
        ] {
            assert!(cached_balance_verification(bytes, Sha256::digest(bytes).into()).is_err());
        }
    }
}

#[cfg(test)]
mod audit_golden_receipt_tests {
    use super::*;
    use crate::activity::{
        ActivityKind, AgentActivity, EvidenceExport, FilterDraft, PendingStatus, ReceiptExport,
        VerifiedStatus,
    };
    use crate::server::schema::ApiSchema;
    use crate::store::{
        AgentTenantId, PrincipalId, PrincipalStore, RetentionPeriod, RetentionPolicy, TenancyMap,
    };
    use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
    use layerx_agent_api::track::ReceiptRef;
    use layerx_proof::inclusion::SequencerAuthorization;
    use layerx_proof::receipt::{verify, AuthorizedBatch};
    use std::path::Path;

    fn golden(name: &str) -> Value {
        let path = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../schema/human-api/golden")
            .join(name);
        serde_json::from_slice(&std::fs::read(path).expect("committed HTTP golden"))
            .expect("HTTP envelope")
    }

    fn fixture_bytes(value: &Value) -> Vec<u8> {
        value
            .as_str()
            .expect("hex string")
            .strip_prefix("0x")
            .expect("hex prefix")
            .as_bytes()
            .chunks_exact(2)
            .map(|pair| (nibble(pair[0]).expect("hex") << 4) | nibble(pair[1]).expect("hex"))
            .collect()
    }

    fn recorded_receipt() -> (VerifiedLegEvidence, Vec<u8>) {
        let document: Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        )))
        .expect("node producer recordings");
        let case = document["cases"]
            .as_array()
            .expect("cases")
            .iter()
            .find(|case| case["id"] == "V1-pax-transfer")
            .expect("recorded transfer");
        let signed = fixture_bytes(&case["signedReceipt"]);
        let key = ReceiptKeyEntry {
            key_id: fixture_bytes(&case["pinnedKey"]["keyId"])
                .try_into()
                .expect("key id"),
            public_key: fixture_bytes(&case["pinnedKey"]["publicKey"])
                .try_into()
                .expect("key"),
            first_height: case["pinnedKey"]["firstHeight"]
                .as_u64()
                .expect("first height"),
            last_height: case["pinnedKey"]["lastHeight"]
                .as_u64()
                .expect("last height"),
        };
        let authorization = SequencerAuthorization::new(
            key.key_id,
            key.public_key,
            key.first_height,
            key.last_height,
        );
        let decoded = layerx_wire::receipt::decode(&signed).expect("recorded canonical receipt");
        let receipt = decoded.protocol().expect("protocol receipt");
        let batch = AuthorizedBatch::from_receipt_keys(
            &signed,
            &[authorization],
            receipt.asset(),
            receipt.previous_state_root(),
            receipt.resulting_state_root(),
        )
        .expect("pinned producer authority");
        let height = u64::from_str_radix(
            case["receiptJson"]["height"]
                .as_str()
                .expect("height")
                .strip_prefix("0x")
                .expect("quantity"),
            16,
        )
        .expect("height quantity");
        let tx_hash = fixture_bytes(&case["receiptJson"]["txHash"])
            .try_into()
            .expect("transaction hash");
        let evidence = VerifiedLegEvidence {
            action_key: tx_hash,
            activity_id: receipt.activity_id(),
            canonical_receipt: signed.clone(),
            receipt_digest: Sha256::digest(&signed).into(),
            authorised_batch: batch,
            tx_hash,
            block_height: height,
            rung: Rung::Receipted,
            signed_receipt: signed,
            receipt_key: key,
            inclusion_height: height,
        };
        (evidence, fixture_bytes(&case["rawTransaction"]))
    }

    #[test]
    fn golden_evidence_verifier_round_trip() {
        let (evidence, raw) = recorded_receipt();
        let id = format!("evd_{}", hex(&evidence.receipt_digest));
        let material = super::super::ProductionComponents::managed_receipt_material(&id, &evidence)
            .expect("real evidence producer");
        assert_eq!(
            material,
            golden("evidence.get.response.json")["body"]["result"]
        );
        let schema = ApiSchema::v1().expect("schema");
        schema
            .encode_response(
                schema.operation("evidence.get").expect("operation"),
                &material,
            )
            .expect("transport schema");
        let bytes = STANDARD
            .decode(material["bytes_base64"].as_str().expect("base64"))
            .expect("material bytes");
        let key = &evidence.receipt_key;
        let pins = [SequencerAuthorization::new(
            key.key_id,
            key.public_key,
            key.first_height,
            key.last_height,
        )];
        let portable = PortableReceiptV2::from_json(&bytes).expect("portable consumer");
        assert_eq!(portable.to_json().expect("stable codec"), bytes);
        let verified =
            PortableReceiptV2::verify_json(&bytes, &pins).expect("real verifier round trip");
        assert_eq!(verified.signed_receipt(), evidence.signed_receipt);
        assert_eq!(verified.activity_id(), evidence.tx_hash);
        assert_eq!(verified.inclusion_height(), evidence.inclusion_height);
        assert_eq!(verified.receipt_key(), &pins[0]);
        let bound = PortableReceiptV2::export_with_transaction(
            &evidence.signed_receipt,
            &pins[0],
            evidence.inclusion_height,
            &raw,
        )
        .expect("bind recorded transaction");
        assert_eq!(
            bound
                .verify_v2(&pins)
                .expect("transaction verifier")
                .raw_transaction(),
            Some(raw.as_slice())
        );
        let wrong_key = [SequencerAuthorization::new(
            key.key_id,
            [0xff; 32],
            key.first_height,
            key.last_height,
        )];
        assert!(portable.verify_v2(&wrong_key).is_err());
        assert!(PortableReceiptV2::from_json(b"{").is_err());
        assert!(STANDARD.decode("not base64!").is_err());
        let mut tampered = evidence.clone();
        let last = tampered.signed_receipt.len() - 1;
        tampered.signed_receipt[last] ^= 1;
        tampered.canonical_receipt = tampered.signed_receipt.clone();
        tampered.receipt_digest = Sha256::digest(&tampered.signed_receipt).into();
        let tampered_id = format!("evd_{}", hex(&tampered.receipt_digest));
        assert!(
            super::super::ProductionComponents::managed_receipt_material(&tampered_id, &tampered)
                .is_err()
        );
        assert!(
            super::super::ProductionComponents::managed_receipt_material(
                &format!("evd_{}", hex(&[0; 32])),
                &evidence
            )
            .is_err()
        );
    }

    #[test]
    fn golden_current_activity_projections() {
        let (evidence, _) = recorded_receipt();
        let root =
            std::env::temp_dir().join(format!("audit-golden-receipt-{}", std::process::id()));
        let principal = PrincipalId::new("golden-receipt").expect("principal");
        let tenancy = TenancyMap::new([(
            principal.clone(),
            AgentTenantId::new("golden-receipt").expect("tenant"),
        )])
        .expect("tenancy");
        let tenancy_digest = tenancy.install(&root).expect("install tenancy");
        let period = RetentionPeriod::new(86_400);
        let retention = RetentionPolicy {
            journeys: period,
            notifications: period,
            audit: period,
            telemetry: period,
            cache: period,
        };
        let mut store = PrincipalStore::open(&root, retention, tenancy_digest).expect("store");
        let mut scope = store.principal(&principal).expect("scope");
        let expected_entry = golden("activity.entry.response.json")["body"]["result"].clone();
        let expected_export =
            golden("activity.export.evidence.response.json")["body"]["result"].clone();
        let occurred_at = crate::time::seconds_from_rfc3339(
            expected_entry["occurred_at"].as_str().expect("occurred at"),
        )
        .expect("timestamp");
        let now = crate::time::seconds_from_rfc3339(
            expected_export["created_at"].as_str().expect("created at"),
        )
        .expect("timestamp");
        let id = ActivityEntryId::new("act_receiptgolden").expect("entry id");
        let descriptor = AgentActivity::new(
            id.clone(),
            ActivityKind::Movement,
            None,
            occurred_at,
            PendingStatus::Processing,
            VerifiedStatus::Done,
        )
        .expect("activity");
        let reference = hex(&evidence.receipt_digest);
        let delivery = EventDelivery::new(
            EventIdentity::new([0x29; 32]),
            evidence.canonical_receipt.clone(),
            Cursor {
                block_height: evidence.block_height,
                tx_index: 0,
                log_index: 0,
            },
            ReceiptReference::Verified {
                receipt_ref: ReceiptRef::new(reference.as_str()).expect("reference"),
                rung: evidence.rung,
            },
        )
        .expect("recorded receipt event");
        Feed::record_agent_event(&mut scope, &descriptor, &delivery, now)
            .expect("durable feed producer");
        let entry = Feed::entry(&scope, &id).expect("feed").expect("entry");
        let verified = verify(&evidence.canonical_receipt, &evidence.authorised_batch)
            .expect("receipt verifier");
        let actual =
            crate::activity::detail::ReceiptActual::from_verified(&verified).expect("actuals");
        let assets = layerx_network_gateway::decode_assets(&json!({"height":"1000","joined_limit":"16","assets":[{"asset_id":hex(&actual.asset()),"kind":"bank","symbol":"HPX","decimals":"18"}]})).expect("asset catalog");
        let detail = EntryDetail::assemble(&entry, vec![actual], None).expect("detail assembler");
        let fact =
            EvidenceBundle::receipt_fact(&id, evidence.canonical_receipt.clone(), evidence.rung)
                .expect("receipt fact");
        let keys = [evidence.receipt_key.clone()];
        let label = verification_label(&verification_status(EvidenceBundle::verify_receipt(
            &fact, &keys,
        )))
        .expect("verified label");
        let value = activity_detail_json(
            &entry,
            &detail,
            &BTreeMap::from([(reference, label)]),
            Some(&assets),
        )
        .expect("activity route producer");
        assert_eq!(value, expected_entry);
        let (bundle, _) = EvidenceExport::new(Feed::new(60).expect("feed"), 1_048_576)
            .expect("exporter")
            .evidence(
                &scope,
                &Feed::apply_filters(FilterDraft::new()).expect("filters"),
                &[id],
                vec![ReceiptExport {
                    receipts: vec![fact],
                    inclusions: Vec::new(),
                    derived_aggregates: Vec::new(),
                }],
                &keys,
                now,
                evidence.block_height,
            )
            .expect("production export path");
        let bytes = bundle.encode().expect("production bundle codec");
        assert_eq!(
            bytes.as_slice(),
            include_bytes!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/../../schema/human-api/golden/current/activity.evidence.bundle"
            ))
            .as_slice()
        );
        let digest = Sha256::digest(&bytes).into();
        let decoded = EvidenceBundle::decode(&bytes).expect("bundle consumer");
        let verification = verification_label(&verification_status(
            decoded.verify(digest, &principal, &keys),
        ))
        .expect("bundle verifier");
        let references = vec![
            json!({"evidence_id":format!("evd_{}", hex(&evidence.receipt_digest)),"class":"layerx-receipt","verification":verification}),
        ];
        let export = super::super::production_components::cache_activity_export(
            &mut scope, bytes, true, references, now,
        )
        .expect("export route producer");
        assert_eq!(export, expected_export);
        let schema = ApiSchema::v1().expect("schema");
        for (operation, value) in [
            ("activity.entry", &value),
            ("activity.export.evidence", &export),
        ] {
            schema
                .encode_response(schema.operation(operation).expect("operation"), value)
                .expect("transport schema");
            let serialized = serde_json::to_vec(value).expect("JSON transport");
            assert_eq!(
                serde_json::from_slice::<Value>(&serialized).expect("JSON consumer"),
                *value
            );
            for retired in ["paxeer-finality", "paxeer-finalised"] {
                assert!(!String::from_utf8_lossy(&serialized).contains(retired));
            }
        }
        drop(scope);
        drop(store);
        std::fs::remove_dir_all(root).expect("remove private store");
    }

    #[test]
    fn golden_retired_binding_producer_equivalence() {
        let failure = ApiFailure::operation_retired();
        for operation in [
            "binding.rebind.action",
            "binding.rebind",
            "binding.statement",
            "binding.status",
            "binding.submit",
        ] {
            let fixture = golden(&format!("{operation}.failure.json"));
            assert_eq!(fixture["status"], failure.status);
            assert_eq!(fixture["body"]["ok"], false);
            assert_eq!(fixture["body"]["error"], failure.envelope());
            assert_eq!(failure.retry_after_ms, None);
            assert_eq!(failure.field, None);
        }
    }
}
