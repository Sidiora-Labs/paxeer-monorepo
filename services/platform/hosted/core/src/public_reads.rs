use layerx_client::px::PxAssetEntry;
use layerx_client::rpc::evidence::{verification_label, Rung};
use layerx_client::rpc::{account as account_rpc, evidence as evidence_rpc, history, payments};

use super::{connect_client, fixed_hex, hex_encode, refusal, success, Config, Request, Response};

pub(super) fn account(config: &Config, id: &str) -> Response {
    let Ok(account_id) = fixed_hex::<32>("account_id", id) else {
        return refusal(400, "invalid_account_id", None);
    };
    let Ok(joined) = config.network.account(&hex_encode(&account_id)) else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let Some(address) = joined.account.evm_address else {
        return refusal(503, "account_evidence_unavailable", Some(5));
    };
    let Ok(head) = config.network.network() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let Ok(requested) = account_rpc::Requested::new(Rung::Committed) else {
        return refusal(503, "account_evidence_unavailable", Some(5));
    };
    let Ok(value) = account_rpc::account(
        &config.sequencer,
        address,
        account_rpc::ReadContext {
            committed_height: head.latest_committed_height,
            requested,
        },
    ) else {
        return refusal(503, "account_evidence_unavailable", Some(5));
    };
    let mut result = serde_json::Map::new();
    if let Some(layerx) = joined.layerx {
        for key in ["name", "asset_id", "balance", "frozen"] {
            if let Some(value) = layerx.get(key) {
                result.insert(key.to_owned(), value.clone());
            }
        }
    }
    result.insert("account_id".to_owned(), hex_encode(&account_id).into());
    result.insert("next_sequence".to_owned(), value.nonce.to_string().into());
    result.insert(
        "batch_number".to_owned(),
        head.latest_committed_height.to_string().into(),
    );
    success(&serde_json::Value::Object(result))
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum AssetSnapshotError {
    InvalidSymbol,
}

impl AssetSnapshotError {
    fn response(self) -> Response {
        match self {
            Self::InvalidSymbol => refusal(502, "invalid_asset_symbol", None),
        }
    }
}

fn asset_json(asset: &PxAssetEntry) -> Result<serde_json::Value, AssetSnapshotError> {
    let mut metadata = serde_json::Map::new();
    if let Some(layerx) = &asset.layerx {
        let symbol = layerx
            .get("symbol")
            .and_then(serde_json::Value::as_str)
            .ok_or(AssetSnapshotError::InvalidSymbol)?;
        if !(1..=16).contains(&symbol.len()) || !symbol.is_ascii() {
            return Err(AssetSnapshotError::InvalidSymbol);
        }
        for key in [
            "symbol",
            "name",
            "decimals",
            "custody_kind",
            "custody_reference",
            "paused",
            "supply_cap",
            "issuer_did",
            "issuer_kind",
            "total_units",
            "salt",
        ] {
            if let Some(value) = layerx.get(key) {
                metadata.insert(key.to_owned(), value.clone());
            }
        }
    }
    metadata.insert("asset_id".to_owned(), hex_encode(&asset.asset_id).into());
    if let Some(custody) = &asset.paxeer {
        metadata.insert("paused".to_owned(), custody.paused.into());
    }
    Ok(serde_json::Value::Object(metadata))
}

fn assets(config: &Config, id: Option<[u8; 32]>) -> Response {
    if let Some(id) = id {
        return match payments::get_asset(&config.network, id) {
            Ok(asset) => match asset_json(&asset) {
                Ok(asset) => success(&serde_json::json!({"asset": asset})),
                Err(error) => error.response(),
            },
            Err(_) => refusal(503, "asset_evidence_unavailable", Some(5)),
        };
    }
    match payments::list_assets(&config.network) {
        Ok(table) => {
            let assets = match table
                .assets
                .iter()
                .map(asset_json)
                .collect::<Result<Vec<_>, _>>()
            {
                Ok(assets) => assets,
                Err(error) => return error.response(),
            };
            success(&serde_json::json!({"assets": assets}))
        }
        Err(_) => refusal(503, "asset_evidence_unavailable", Some(5)),
    }
}

fn did_accounts(config: &Config, did: &str) -> Response {
    if layerx_types::ids::Did::new(did.as_bytes()).is_err() {
        return refusal(400, "invalid_did", None);
    }
    match history::did_accounts(&config.network, did) {
        Ok(accounts) => success(&serde_json::json!({"did": did, "accounts": accounts})),
        Err(_) => refusal(503, "did_account_listing_unavailable", Some(5)),
    }
}

fn estimate_fee(config: &Config, body: &[u8]) -> Response {
    let Ok(value) = serde_json::from_slice::<serde_json::Value>(body) else {
        return refusal(400, "invalid_fee_request", None);
    };
    let Some(hex) = value
        .get("canonical_hex")
        .and_then(serde_json::Value::as_str)
    else {
        return refusal(400, "invalid_fee_request", None);
    };
    let Ok(canonical) = layerx_platform_core::hex_decode(hex) else {
        return refusal(400, "invalid_fee_request", None);
    };
    let Ok((transaction, signature)) = layerx_crypto::evm_transaction::decode_signed(&canonical)
    else {
        return refusal(400, "invalid_activity", None);
    };
    let Ok(sender) = layerx_crypto::evm_transaction::recover_sender(&transaction, &signature)
    else {
        return refusal(400, "invalid_activity", None);
    };
    let Ok(chain_id) = config.sequencer.chain_id() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    if transaction.chain_id != chain_id {
        return refusal(400, "invalid_activity_domain", None);
    }
    let call = payments::CallRequest {
        from: Some(sender),
        to: Some(transaction.to),
        value: 0,
        data: transaction.data,
    };
    match payments::estimate_fee(&config.sequencer, &call) {
        Ok(estimate) => success(&serde_json::json!({
            "fee": estimate.fee.to_string(),
            "parameter_version": estimate.parameter_version,
            "canonical_schedule": hex_encode(&estimate.canonical_schedule),
            "canonical_bytes": canonical.len()
        })),
        Err(_) => refusal(503, "fee_evidence_unavailable", Some(5)),
    }
}

fn asset_route(config: &Config, request: &Request) -> Response {
    if request.method != "GET" {
        refusal(405, "method_not_allowed", None)
    } else if request.query.is_some() || !request.body.is_empty() {
        refusal(400, "invalid_request", None)
    } else if request.path != "/v1/assets"
        && request
            .path
            .strip_prefix("/v1/assets/")
            .and_then(|id| fixed_hex::<32>("asset_id", id).ok())
            .is_none_or(|id| id == [0; 32])
    {
        refusal(400, "invalid_asset_id", None)
    } else {
        assets(
            config,
            request
                .path
                .strip_prefix("/v1/assets/")
                .and_then(|id| fixed_hex::<32>("asset_id", id).ok()),
        )
    }
}

pub(super) fn route(config: &Config, request: &Request) -> Option<Response> {
    if request.path == "/v1/assets" || request.path.starts_with("/v1/assets/") {
        return Some(asset_route(config, request));
    }
    if request.path == "/v1/fees/estimate" {
        return Some(if request.method != "POST" {
            refusal(405, "method_not_allowed", None)
        } else if request.query.is_some() || !valid_fee_request(&request.body) {
            refusal(400, "invalid_fee_request", None)
        } else {
            estimate_fee(config, &request.body)
        });
    }
    if request.path == "/v1/node-info" {
        return Some(if request.method != "GET" {
            refusal(405, "method_not_allowed", None)
        } else if request.query.is_some() {
            refusal(400, "invalid_request", None)
        } else {
            node_info(config)
        });
    }
    if let Some(sequence) = request.path.strip_prefix("/internal/v1/receipt-events/") {
        return Some(receipt_event(config, request, sequence));
    }
    let parts: Vec<_> = request.path.split('/').collect();
    let target = match parts.as_slice() {
        ["", "v1", "accounts", id, "balance"] | ["", "v1", "accounts", id] => Some(*id),
        ["", "v1", "batches" | "checkpoints", id] => {
            return Some(if request.method != "GET" {
                refusal(405, "method_not_allowed", None)
            } else if request.query.is_some() {
                refusal(400, "invalid_request", None)
            } else {
                evidence(config, parts[2], id)
            });
        }
        ["", "v1", "proofs", kind, id] => {
            return Some(if request.method != "GET" {
                refusal(405, "method_not_allowed", None)
            } else if request.query.is_some() {
                refusal(400, "invalid_request", None)
            } else {
                proof(config, kind, id, None)
            });
        }
        ["", "v1", "proofs", "account", activity, account] => {
            return Some(if request.method != "GET" {
                refusal(405, "method_not_allowed", None)
            } else if request.query.is_some() {
                refusal(400, "invalid_request", None)
            } else {
                proof(config, "account", activity, Some(account))
            });
        }
        ["", "v1", "dids", did, "sequence"] => {
            return Some(if request.method != "GET" {
                refusal(405, "method_not_allowed", None)
            } else if request.query.is_some() {
                refusal(400, "invalid_request", None)
            } else {
                sequence(config, did)
            });
        }
        ["", "v1", "dids", did, "accounts"] => {
            return Some(if request.method != "GET" {
                refusal(405, "method_not_allowed", None)
            } else if request.query.is_some()
                || layerx_types::ids::Did::new(did.as_bytes()).is_err()
            {
                refusal(400, "invalid_did", None)
            } else {
                did_accounts(config, did)
            });
        }
        _ => return None,
    };
    Some(if request.method != "GET" {
        refusal(405, "method_not_allowed", None)
    } else if request.query.is_some() {
        refusal(400, "invalid_request", None)
    } else {
        account(config, target?)
    })
}

fn node_info(config: &Config) -> Response {
    let Ok(client) = connect_client(config) else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let node = client.handshake().node();
    success(&serde_json::json!({
        "protocol_version": node.protocol_version,
        "network_id": node.network_id,
        "chain_head_sequence": node.chain_head_sequence.to_string(),
        "latest_sealed_batch": node.latest_sealed_batch.to_string(),
        "latest_finalised_checkpoint": hex_encode(&node.latest_finalised_checkpoint),
        "authorised_sequencer_key": hex_encode(&node.authorised_sequencer_key),
        "capabilities": node.advertised_capabilities
    }))
}

fn evidence(config: &Config, kind: &str, id: &str) -> Response {
    if kind == "batches" {
        let Ok(number) = id.parse::<u64>() else {
            return refusal(400, "invalid_batch", None);
        };
        if number == 0 || number.to_string() != id {
            return refusal(400, "invalid_batch", None);
        }
        return match layerx_client::rpc::batch::lookup(&config.sequencer, number) {
            Ok(block) => success(&serde_json::json!({"batch_number": block.number.to_string()})),
            Err(_) => refusal(503, "batch_evidence_unavailable", Some(5)),
        };
    }
    let Ok(identifier) = fixed_hex::<32>("checkpoint_id", id) else {
        return refusal(400, "invalid_checkpoint", None);
    };
    if identifier == [0; 32] {
        return refusal(400, "invalid_checkpoint", None);
    }
    let Ok(head) = config.network.network() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    match evidence_rpc::checkpoint(&config.sequencer, head.latest_committed_height) {
        Ok(checkpoint) if checkpoint.hash == identifier => success(&serde_json::json!({
            "checkpoint_id": hex_encode(&checkpoint.hash)
        })),
        _ => refusal(503, "checkpoint_evidence_unavailable", Some(5)),
    }
}

fn sequence(config: &Config, did: &str) -> Response {
    let Ok(actor) = layerx_types::ids::Did::new(did.as_bytes()) else {
        return refusal(400, "invalid_did", None);
    };
    let Ok(mut client) = connect_client(config) else {
        return refusal(503, "node_unavailable", Some(5));
    };
    match client.preparation_state(&actor, 1) {
        Ok(snapshot) => success(&serde_json::json!({
            "did": did, "next_sequence": snapshot.account_sequence.to_string(),
            "observed_head_sequence": snapshot.observed_head_sequence.to_string(),
            "state_root": hex_encode(&snapshot.observed_state_root),
            "verification": "authenticated_node_snapshot"
        })),
        Err(_) => refusal(503, "sequence_unavailable", Some(5)),
    }
}

fn proof(config: &Config, kind: &str, id: &str, account: Option<&str>) -> Response {
    let Ok(identifier) = fixed_hex::<32>("activity_id", id) else {
        return refusal(400, "invalid_proof_selector", None);
    };
    if identifier == [0; 32] || !matches!(kind, "activity" | "receipt" | "account") {
        return refusal(400, "invalid_proof_selector", None);
    }
    let selected_account = if kind == "account" {
        let Some(account) = account else {
            return refusal(400, "invalid_proof_selector", None);
        };
        let Ok(account_id) = fixed_hex::<32>("account_id", account) else {
            return refusal(400, "invalid_proof_selector", None);
        };
        if account_id == [0; 32] {
            return refusal(400, "invalid_proof_selector", None);
        }
        Some(account_id)
    } else {
        None
    };
    let Ok(keys) = config.sequencer.receipt_keys() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let Ok(layerx_client::rpc::receipt::Lookup::Verified(receipt)) =
        layerx_client::rpc::receipt::lookup(
            &config.sequencer,
            identifier,
            layerx_client::rpc::receipt::LookupContext {
                receipt_keys: &keys,
            },
        )
    else {
        return refusal(503, "proof_evidence_unavailable", Some(5));
    };
    let address = if let Some(account_id) = selected_account {
        let Ok(joined) = config.network.account(&hex_encode(&account_id)) else {
            return refusal(503, "node_unavailable", Some(5));
        };
        let Some(address) = joined.account.evm_address else {
            return refusal(503, "proof_evidence_unavailable", Some(5));
        };
        address
    } else {
        receipt.execution().from
    };
    let Ok(head) = config.network.network() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    if receipt.rung() != Rung::Committed || receipt.height() > head.latest_committed_height {
        return refusal(503, "proof_evidence_unavailable", Some(5));
    }
    let bundle = if kind == "account" {
        evidence_rpc::proof_bundle_with_history(
            &config.sequencer,
            address,
            &[],
            head.latest_committed_height,
        )
    } else {
        evidence_rpc::proof_bundle(
            &config.sequencer,
            address,
            &[],
            head.latest_committed_height,
        )
    };
    let Ok(bundle) = bundle else {
        return refusal(503, "proof_evidence_unavailable", Some(5));
    };
    let mut result = serde_json::json!({
        "kind": kind, "activity_id": id, "account_id": account,
        "proof": {"address": hex_encode(&bundle.account_proof.address),
            "account_proof": bundle.account_proof.account_proof.iter().map(|node| hex_encode(node)).collect::<Vec<_>>()},
        "verification": verification_label(receipt.rung())
    });
    if kind == "receipt" {
        result["canonical_value"] = hex_encode(receipt.canonical_bytes()).into();
    } else if kind == "activity" {
        let Some(landing) = receipt.landing() else {
            return refusal(503, "proof_evidence_unavailable", Some(5));
        };
        let Ok(block) = evidence_rpc::checkpoint(&config.sequencer, landing.block_number) else {
            return refusal(503, "proof_evidence_unavailable", Some(5));
        };
        let Some(transaction) = block.transactions.iter().find(|transaction| {
            layerx_crypto::evm_transaction::transaction_hash(transaction) == identifier
        }) else {
            return refusal(503, "proof_evidence_unavailable", Some(5));
        };
        result["canonical_value"] = hex_encode(transaction).into();
    }
    success(&result)
}

fn receipt_event(config: &Config, request: &Request, sequence: &str) -> Response {
    use subtle::ConstantTimeEq;
    let Some(token) = &config.receipt_events_token else {
        return refusal(503, "receipt_events_not_configured", None);
    };
    let supplied = request
        .headers
        .get("authorization")
        .and_then(|value| value.strip_prefix("Bearer "))
        .unwrap_or("");
    if !bool::from(supplied.as_bytes().ct_eq(token.as_bytes())) {
        return refusal(401, "unauthorized", None);
    }
    if request.method != "GET" {
        return refusal(405, "method_not_allowed", None);
    }
    let Ok(number) = sequence.parse::<u64>() else {
        return refusal(400, "invalid_sequence", None);
    };
    if number == 0
        || number.to_string() != sequence
        || request.query.is_some()
        || !request.body.is_empty()
    {
        return refusal(400, "invalid_sequence", None);
    }
    let Ok((mut transport, handshake)) = super::connect_raw(config) else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let mut selector = vec![3];
    selector.extend_from_slice(&number.to_be_bytes());
    match super::lookup_receipt_selector(&mut transport, &handshake, selector, 1, true) {
        Ok(Some(bytes)) => {
            match super::receipt_facts(&bytes, handshake.node().authorised_sequencer_key) {
                Ok(facts) if facts.global_sequence == number => {
                    success(&super::receipt_result(&facts))
                }
                _ => refusal(502, "invalid_receipt_event", None),
            }
        }
        Ok(None) => super::json_response(202, &serde_json::json!({"result":{"state":"pending"}})),
        Err(_) => refusal(503, "receipt_events_unavailable", Some(5)),
    }
}

fn valid_fee_request(body: &[u8]) -> bool {
    let Ok(serde_json::Value::Object(value)) = serde_json::from_slice(body) else {
        return false;
    };
    value.len() == 1
        && value
            .get("canonical_hex")
            .and_then(serde_json::Value::as_str)
            .is_some_and(|hex| {
                !hex.is_empty()
                    && hex.len() <= 1024 * 1024
                    && hex.len().is_multiple_of(2)
                    && hex.bytes().all(|b| b.is_ascii_hexdigit())
            })
}

#[cfg(test)]
mod tests {
    fn asset(symbol: &[u8]) -> super::PxAssetEntry {
        let symbol = match std::str::from_utf8(symbol) {
            Ok(symbol) => serde_json::Value::String(symbol.to_owned()),
            Err(_) => serde_json::json!(symbol),
        };
        super::PxAssetEntry {
            asset_id: [1; 32],
            layerx: Some(serde_json::Map::from_iter([
                ("symbol".to_owned(), symbol),
                ("name".to_owned(), "Test asset".into()),
                ("decimals".to_owned(), 6.into()),
            ])),
            paxeer: None,
        }
    }

    #[test]
    fn asset_json_refuses_symbols_outside_the_shared_encoding() {
        let valid = super::asset_json(&asset(b"LXP"))
            .unwrap_or_else(|error| panic!("valid symbol: {error:?}"));
        assert_eq!(valid["symbol"], "LXP");
        for symbol in [&[][..], &[b'X'; 17][..], &[0xff][..]] {
            assert_eq!(
                super::asset_json(&asset(symbol)),
                Err(super::AssetSnapshotError::InvalidSymbol)
            );
        }
        let response = super::AssetSnapshotError::InvalidSymbol.response();
        assert_eq!(response.status, 502);
        let body: serde_json::Value = serde_json::from_str(&response.body)
            .unwrap_or_else(|error| panic!("typed response: {error}"));
        assert_eq!(body["error"]["code"], "invalid_asset_symbol");
    }

    #[test]
    fn fee_request_requires_bounded_canonical_bytes() {
        assert!(super::valid_fee_request(br#"{"canonical_hex":"abcd"}"#));
        for body in [
            br"{}".as_slice(),
            br#"{"canonical_hex":""}"#,
            br#"{"canonical_hex":"abc"}"#,
            br#"{"canonical_hex":"zz"}"#,
            br#"{"canonical_hex":"ab","extra":1}"#,
        ] {
            assert!(!super::valid_fee_request(body));
        }
    }
}
