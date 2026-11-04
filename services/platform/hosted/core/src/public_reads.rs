use layerx_client::px::PxAssetEntry;
use layerx_client::rpc::{evidence as evidence_rpc, history, payments};
use layerx_client::sequencer::SequencerError;

use super::{fixed_hex, hex_encode, refusal, success, Config, Request, Response};

fn account_selector(id: &str) -> Result<String, Response> {
    let supported = if let Some(address) = id.strip_prefix("0x") {
        address.len() == 40 && address.bytes().all(|byte| byte.is_ascii_hexdigit())
    } else if let Some(key) = id.strip_prefix("did:layerx:") {
        key.len() == 64 && fixed_hex::<32>("did_key", key).is_ok_and(|key| key != [0; 32])
    } else {
        false
    };
    if supported {
        Ok(id.to_owned())
    } else {
        Err(refusal(400, "unsupported_account_selector", None))
    }
}

fn account_lookup_error(
    error: SequencerError,
    selector_code: &str,
    evidence_code: &str,
) -> Response {
    match error {
        SequencerError::Tls | SequencerError::Unavailable => {
            refusal(503, "node_unavailable", Some(5))
        }
        SequencerError::Rpc(error)
            if error.code == -32602
                || error
                    .data
                    .as_ref()
                    .and_then(|data| data.get("code"))
                    .and_then(serde_json::Value::as_str)
                    == Some("native_account_reverse_resolution_unsupported") =>
        {
            refusal(400, selector_code, None)
        }
        _ => refusal(502, evidence_code, None),
    }
}

pub(super) fn account(config: &Config, id: &str) -> Response {
    let account_id = match account_selector(id) {
        Ok(selector) => selector,
        Err(response) => return response,
    };
    let (Ok(chain_id), Ok(height)) = (config.sequencer.chain_id(), config.sequencer.block_number())
    else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let joined = match config.network.account(&account_id) {
        Ok(joined) => joined,
        Err(error) => {
            return account_lookup_error(
                error,
                "unsupported_account_selector",
                "account_evidence_unavailable",
            )
        }
    };
    let Some(address) = joined.account.evm_address else {
        return refusal(503, "account_evidence_unavailable", Some(5));
    };
    let (Ok(balance), Ok(nonce)) = (
        config.sequencer.balance_at(&address, height),
        config.sequencer.nonce_at(&address, height),
    ) else {
        return refusal(503, "account_evidence_unavailable", Some(5));
    };
    let (Ok(confirmed_chain), Ok(confirmed_height)) =
        (config.sequencer.chain_id(), config.sequencer.block_number())
    else {
        return refusal(503, "node_unavailable", Some(5));
    };
    if confirmed_height != height || confirmed_chain != chain_id {
        return refusal(503, "account_snapshot_changed", Some(5));
    }
    native_account_result(joined, &account_id, height, balance, nonce)
}

pub(super) fn native_account_result(
    joined: layerx_client::px::PxAccountJoin,
    account_id: &str,
    height: u64,
    committed_balance: [u8; 32],
    committed_nonce: u64,
) -> Response {
    let Some(native) = joined.paxeer else {
        return refusal(503, "account_evidence_unavailable", Some(5));
    };
    let mut expected_balance = [0; 32];
    expected_balance[16..].copy_from_slice(&native.balance.to_be_bytes());
    if height == 0
        || joined.account.evm_address != Some(native.address)
        || committed_balance != expected_balance
        || committed_nonce != native.nonce
    {
        return refusal(502, "account_snapshot_mismatch", None);
    }
    let mut result = serde_json::Map::new();
    if let Some(layerx) = joined.layerx {
        for key in ["name", "asset_id", "frozen"] {
            if let Some(value) = layerx.get(key) {
                result.insert(key.to_owned(), value.clone());
            }
        }
    }
    result.insert("account_id".to_owned(), account_id.into());
    result.insert("balance".to_owned(), native.balance.to_string().into());
    result.insert("balance_unit".to_owned(), "wei".into());
    result.insert("balance_decimals".to_owned(), 18.into());
    result.insert("height".to_owned(), height.to_string().into());
    result.insert("next_sequence".to_owned(), native.nonce.to_string().into());
    result.insert("batch_number".to_owned(), height.to_string().into());
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

fn validate_fee_payload(
    body: &[u8],
) -> Result<(layerx_crypto::evm_transaction::Eip1559Call, usize), Response> {
    if !valid_fee_request(body) {
        return Err(refusal(400, "invalid_fee_request", None));
    }
    let value: serde_json::Value =
        serde_json::from_slice(body).map_err(|_| refusal(400, "invalid_fee_request", None))?;
    let hex = value["canonical_hex"]
        .as_str()
        .ok_or_else(|| refusal(400, "invalid_fee_request", None))?;
    let canonical = layerx_platform_core::hex_decode(hex)
        .map_err(|_| refusal(400, "invalid_fee_request", None))?;
    let transaction = layerx_crypto::evm_transaction::decode_unsigned(&canonical)
        .map_err(|_| refusal(400, "invalid_activity", None))?;
    if transaction.chain_id != 125 {
        return Err(refusal(400, "invalid_activity_domain", None));
    }
    if transaction.gas_limit == 0
        || transaction.max_priority_fee_per_gas > transaction.max_fee_per_gas
        || u128::from(transaction.gas_limit)
            .checked_mul(transaction.max_fee_per_gas)
            .is_none()
    {
        return Err(refusal(400, "invalid_fee_request", None));
    }
    Ok((transaction, canonical.len()))
}

fn estimate_fee(_config: &Config, body: &[u8]) -> Response {
    let (transaction, canonical_bytes) = match validate_fee_payload(body) {
        Ok(value) => value,
        Err(response) => return response,
    };
    let Some(fee) = u128::from(transaction.gas_limit).checked_mul(transaction.max_fee_per_gas)
    else {
        return refusal(400, "invalid_fee_request", None);
    };
    success(&serde_json::json!({
        "fee": fee.to_string(),
        "parameter_version": 0,
        "canonical_schedule": "",
        "canonical_bytes": canonical_bytes
    }))
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
    if let Some(hash) = request.path.strip_prefix("/internal/v1/receipt-events/") {
        return Some(receipt_event(config, request, hash));
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
    let Ok(head) = config.network.network() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let Ok(keys) = config.sequencer.receipt_keys() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let Some(key) = keys.iter().max_by_key(|key| key.last_height) else {
        return refusal(503, "node_unavailable", Some(5));
    };
    let Ok(checkpoint) = evidence_rpc::checkpoint(&config.sequencer, head.latest_committed_height)
    else {
        return refusal(503, "node_unavailable", Some(5));
    };
    success(&serde_json::json!({
        "protocol_version": layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
        "network_id": head.network_id,
        "chain_head_sequence": head.latest_committed_height.to_string(),
        "latest_sealed_batch": head.paxeer.latest_block.to_string(),
        "latest_finalised_checkpoint": hex_encode(&checkpoint.hash),
        "authorised_sequencer_key": hex_encode(&key.public_key),
        "capabilities": head.layerx.unwrap_or_default()
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
    if layerx_types::ids::Did::new(did.as_bytes()).is_err() {
        return refusal(400, "invalid_did", None);
    }
    let Ok(address) = history::did_evm_address(&config.network, did) else {
        return refusal(503, "sequence_unavailable", Some(5));
    };
    let Some(address) = address else {
        return refusal(404, "unknown_did", None);
    };
    let Ok(next_sequence) = config.sequencer.pending_nonce(&address) else {
        return refusal(503, "sequence_unavailable", Some(5));
    };
    let Ok(head) = config.network.network() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    success(&serde_json::json!({
        "did": did, "next_sequence": next_sequence.to_string(),
        "observed_head_sequence": head.latest_committed_height.to_string(),
        "verification": "sequencer_pending_nonce"
    }))
}

fn landing_block_json(block: &layerx_client::sequencer::SequencerBlock) -> serde_json::Value {
    serde_json::json!({
        "number": block.number,
        "hash": hex_encode(&block.hash),
        "parent_hash": hex_encode(&block.parent_hash),
        "timestamp": block.timestamp,
        "transactions_root": hex_encode(&block.transactions_root),
        "receipts_root": hex_encode(&block.receipts_root)
    })
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
        let Ok(selector) = account_selector(account) else {
            return refusal(400, "invalid_proof_selector", None);
        };
        Some(selector)
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
    let address = if let Some(selector) = selected_account {
        let joined = match config.network.account(&selector) {
            Ok(joined) => joined,
            Err(error) => {
                return account_lookup_error(
                    error,
                    "invalid_proof_selector",
                    "proof_evidence_unavailable",
                )
            }
        };
        let Some(address) = joined.account.evm_address else {
            return refusal(503, "proof_evidence_unavailable", Some(5));
        };
        address
    } else {
        receipt.execution().from
    };
    let Some(landing) = receipt.landing() else {
        return refusal(503, "proof_evidence_unavailable", Some(5));
    };
    let Ok((block, app_hash)) =
        layerx_platform_core::native_committed_state(&config.sequencer, landing.block_number)
    else {
        return refusal(503, "proof_evidence_unavailable", Some(5));
    };
    if super::validate_receipt_landing(&receipt, &block).is_err() {
        return refusal(502, "proof_landing_mismatch", None);
    }
    if kind == "account" {
        // eth_getProof witnesses EVM storage slots, not bank account balances.
        // No empty slot list or block header can prove this account's value.
        return refusal(503, "native_account_inclusion_unavailable", Some(5));
    }
    let Some(header) = receipt.committed_header() else {
        return refusal(503, "native_receipt_inclusion_unavailable", Some(5));
    };
    if u64::try_from(header.height()).ok() != Some(landing.block_number)
        || header.header_hash() != block.hash
        || header.app_hash() != app_hash
    {
        return refusal(502, "proof_root_mismatch", None);
    }
    // The client must carry the batch transaction's inclusion path through to
    // the public contract. A verified header without that path is insufficient.
    let _ = (address, header);
    refusal(503, "native_receipt_inclusion_unavailable", Some(5))
}

fn receipt_event(config: &Config, request: &Request, hash: &str) -> Response {
    use layerx_client::rpc::receipt::Lookup;
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
    let Ok(hash) = fixed_hex::<32>("tx_hash", hash) else {
        return refusal(400, "invalid_tx_hash", None);
    };
    if request.query.is_some() || !request.body.is_empty() {
        return refusal(400, "invalid_tx_hash", None);
    }
    match super::await_authenticated_receipt(config, hash, std::time::Duration::ZERO) {
        Ok(Lookup::Verified(receipt)) => success(&serde_json::json!({
            "state": if receipt.execution().success { "completed" } else { "refused" },
            "rung": receipt.rung().name(),
            "activity_id": hex_encode(&receipt.activity_id()),
            "global_sequence": receipt.global_sequence().to_string(),
            "height": receipt.height().to_string(),
            "index": receipt.execution().index.to_string(),
            "result_code": receipt.result_code().raw(),
            "timestamp": receipt.execution().timestamp.to_string()
        })),
        Ok(Lookup::Submitted { tx_hash, nonce }) => super::json_response(
            202,
            &serde_json::json!({"result":{
                "state":"pending", "rung":"submitted",
                "activity_id":hex_encode(&tx_hash), "nonce":nonce.to_string()
            }}),
        ),
        Ok(Lookup::Absent) => refusal(404, "not_found", None),
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
    #[test]
    fn landing_block_json_names_the_six_header_fields() {
        let block = layerx_client::sequencer::SequencerBlock {
            number: 42,
            hash: [0xab; 32],
            parent_hash: [0xcd; 32],
            timestamp: 1_700_000_000,
            transactions_root: [0xef; 32],
            receipts_root: [0x01; 32],
            transactions: vec![vec![0x02; 64]],
        };
        assert_eq!(
            super::landing_block_json(&block),
            serde_json::json!({
                "number": 42,
                "hash": "ab".repeat(32),
                "parent_hash": "cd".repeat(32),
                "timestamp": 1_700_000_000,
                "transactions_root": "ef".repeat(32),
                "receipts_root": "01".repeat(32)
            })
        );
    }

    #[test]
    fn account_selector_forwards_only_supported_identities() {
        for selector in [
            format!("0x{}", "aB".repeat(20)),
            format!("did:layerx:{}", "aB".repeat(32)),
            format!("0x{}", "00".repeat(20)),
        ] {
            let forwarded = super::account_selector(&selector)
                .unwrap_or_else(|response| panic!("supported selector refused: {}", response.body));
            assert_eq!(forwarded, selector);
        }
        for selector in [
            "ab".repeat(32),
            format!("did:layerx:{}", "00".repeat(32)),
            format!("0x{}", "ab".repeat(32)),
            "ab".repeat(20),
            format!("did:layerx:0x{}", "ab".repeat(32)),
            format!("did:layerx:{}", "zz".repeat(32)),
            format!("0x{}", "zz".repeat(20)),
        ] {
            let response =
                super::account_selector(&selector).expect_err("unsupported selector was forwarded");
            assert_eq!(response.status, 400);
            assert_eq!(response.retry_after, None);
            let body: serde_json::Value = serde_json::from_str(&response.body)
                .unwrap_or_else(|error| panic!("typed response: {error}"));
            assert_eq!(body["error"]["code"], "unsupported_account_selector");
            assert_eq!(body["error"]["retry"], "never");
        }
        for (error, status, retry_after) in [
            (super::SequencerError::Unavailable, 503, Some(5)),
            (super::SequencerError::Tls, 503, Some(5)),
            (super::SequencerError::Malformed, 502, None),
            (super::SequencerError::Endpoint, 502, None),
            (super::SequencerError::HttpStatus(400), 502, None),
            (
                super::SequencerError::Rpc(layerx_client::sequencer::RpcRefusal {
                    code: -32602,
                    message: "Invalid params".to_owned(),
                    data: None,
                }),
                400,
                None,
            ),
            (
                super::SequencerError::Rpc(layerx_client::sequencer::RpcRefusal {
                    code: -32001,
                    message: "Read unavailable".to_owned(),
                    data: Some(
                        serde_json::json!({"code": "native_account_reverse_resolution_unsupported"}),
                    ),
                }),
                400,
                None,
            ),
        ] {
            for selector_code in ["unsupported_account_selector", "invalid_proof_selector"] {
                let response = super::account_lookup_error(
                    error.clone(),
                    selector_code,
                    "account_evidence_unavailable",
                );
                assert_eq!(response.status, status);
                assert_eq!(response.retry_after, retry_after);
                if status == 400 {
                    let body: serde_json::Value = serde_json::from_str(&response.body)
                        .unwrap_or_else(|error| panic!("typed response: {error}"));
                    assert_eq!(body["error"]["code"], selector_code);
                }
            }
        }
    }

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
