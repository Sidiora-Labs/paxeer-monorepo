use super::*;
use chain_fund::{NativeTransfer, SignedFunding};

#[derive(Clone, serde::Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct NativeFundingIntent {
    version: u8,
    key: String,
    request_digest: String,
    request_body: Vec<u8>,
    command: FundingCommand,
    network_id: u32,
    transfer: NativeTransfer,
    authorization: Vec<u8>,
}

impl NativeFundingIntent {
    fn digest(&self) -> Result<[u8; 32], String> {
        let bytes = serde_json::to_vec(&(
            "layerx-core/native-funding-intent/v2",
            self.version,
            &self.key,
            &self.request_digest,
            &self.request_body,
            &self.command,
            self.network_id,
            &self.transfer,
        ))
        .map_err(|error| error.to_string())?;
        Ok(Sha256::digest(bytes).into())
    }

    fn validate(&self, config: &Config) -> Result<(), String> {
        self.transfer.validate(config.treasury.public_key())?;
        let request = Request {
            method: "POST".into(),
            path: "/admin/v1/testnet/fund".into(),
            query: None,
            headers: BTreeMap::new(),
            body: self.request_body.clone(),
        };
        let command: FundingCommand =
            serde_json::from_slice(&self.request_body).map_err(|error| error.to_string())?;
        if self.version != 2
            || !valid_key(&self.key)
            || self.network_id != config.network_id
            || self.transfer.amount != self.command.amount
            || command.amount == 0
            || !valid_key(&command.funding_id)
            || !is_hex64(&command.public_key)
            || command.did != format!("did:layerx:{}", command.public_key.to_ascii_lowercase())
            || command.did == config.treasury_did
            || serde_json::to_vec(&command).map_err(|error| error.to_string())?
                != serde_json::to_vec(&self.command).map_err(|error| error.to_string())?
            || self.request_digest != request_digest(&request)
        {
            return Err("native funding request binding differs".into());
        }
        verify_authorization(config, &self.digest()?, &self.authorization)
    }
}

#[derive(Clone, serde::Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct NativeFundingArchive {
    version: u8,
    intent_digest: String,
    receipt: serde_json::Value,
    authorization: Vec<u8>,
}

#[derive(serde::Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct NativeFundingRecovery {
    version: u8,
    intent: NativeFundingIntent,
    archive: Option<NativeFundingArchive>,
}

fn verify_authorization(
    config: &Config,
    digest: &[u8; 32],
    signature: &[u8],
) -> Result<(), String> {
    let key = ed25519_dalek::VerifyingKey::from_bytes(&config.treasury.public_key())
        .map_err(|_| "native funding authorization key invalid")?;
    let signature = ed25519_dalek::Signature::from_slice(signature)
        .map_err(|_| "native funding authorization invalid")?;
    key.verify_strict(digest, &signature)
        .map_err(|_| "native funding authorization differs".into())
}

fn unavailable() -> Response {
    refusal(503, "journal_unavailable", Some(5))
}

fn command_digest(command: &FundingCommand) -> Result<String, Response> {
    serde_json::to_vec(command)
        .map(|bytes| hex_encode(&Sha256::digest(bytes)))
        .map_err(|_| unavailable())
}

fn store(config: &Config, intent: &NativeFundingIntent) -> Result<(), String> {
    intent.validate(config)?;
    funding_store(
        &funding_record_path(config, "funding-intents", &intent.key),
        &serde_json::to_vec(intent).map_err(|error| error.to_string())?,
    )
}

pub(super) fn is_native(bytes: &[u8]) -> bool {
    serde_json::from_slice::<serde_json::Value>(bytes).is_ok_and(|value| value["version"] == 2)
}

fn read(config: &Config, key: &str) -> Result<Option<NativeFundingIntent>, Response> {
    let bytes = funding_read(&funding_record_path(config, "funding-intents", key))
        .map_err(|_| unavailable())?;
    let Some(bytes) = bytes else {
        return Ok(None);
    };
    let intent: NativeFundingIntent = serde_json::from_slice(&bytes).map_err(|_| unavailable())?;
    intent.validate(config).map_err(|_| unavailable())?;
    if intent.key != key {
        return Err(unavailable());
    }
    Ok(Some(intent))
}

fn stage(config: &Config, intent: &NativeFundingIntent) -> Result<(), Response> {
    intent.validate(config).map_err(|_| unavailable())?;
    let path = journal_path(config, "fund-canonical", &intent.key);
    if let Some(prior) = journal_read(&path).map_err(|_| unavailable())? {
        let transfer = decode_stage(config, &intent.command, &prior.body)?;
        if prior.request_digest != command_digest(&intent.command)? || transfer != intent.transfer {
            return Err(refusal(409, "idempotency_conflict", None));
        }
    }
    journal_write(
        &path,
        &JournalEntry {
            request_digest: command_digest(&intent.command)?,
            status: 202,
            body: serde_json::to_string(&intent.transfer).map_err(|_| unavailable())?,
            retry_after: Some(5),
        },
    )
    .map_err(|_| unavailable())
}

fn decode_stage(
    config: &Config,
    command: &FundingCommand,
    body: &str,
) -> Result<NativeTransfer, Response> {
    if let Ok(transfer) = serde_json::from_str::<NativeTransfer>(body) {
        transfer
            .validate(config.treasury.public_key())
            .map_err(|_| unavailable())?;
        if transfer.amount != command.amount {
            return Err(unavailable());
        }
        return Ok(transfer);
    }
    // Explicit migration of the live pre-versioned native record. Ledger
    // PreparedFunding is deliberately not accepted as native transfer bytes.
    let signed: SignedFunding = serde_json::from_str(body).map_err(|_| unavailable())?;
    let funding = config
        .funding
        .get()
        .map_err(|_| refusal(503, "treasury_unavailable", Some(5)))?;
    let transfer =
        NativeTransfer::attest(signed, funding.sender_address(), config.treasury.as_ref())
            .map_err(|_| unavailable())?;
    if transfer.amount != command.amount {
        return Err(unavailable());
    }
    let key = fixed_hex::<32>("public_key", &command.public_key)
        .map_err(|_| refusal(400, "invalid_argument", None))?;
    if funding.bound_recipient(&key)? != transfer.recipient {
        return Err(unavailable());
    }
    Ok(transfer)
}

pub(super) fn prepare(
    config: &Config,
    request: &Request,
    command: &FundingCommand,
    key: &str,
) -> Result<NativeTransfer, Response> {
    if let Some(intent) = read(config, key)? {
        if intent.request_digest != request_digest(request) {
            return Err(refusal(409, "idempotency_conflict", None));
        }
        stage(config, &intent)?;
        return Ok(intent.transfer);
    }
    let path = journal_path(config, "fund-canonical", key);
    let transfer = if let Some(prior) = journal_read(&path).map_err(|_| unavailable())? {
        if prior.request_digest != command_digest(command)? {
            return Err(refusal(409, "idempotency_conflict", None));
        }
        decode_stage(config, command, &prior.body)?
    } else {
        let funding = config
            .funding
            .get()
            .map_err(|_| refusal(503, "treasury_unavailable", Some(5)))?;
        let public_key = fixed_hex::<32>("public_key", &command.public_key)
            .map_err(|_| refusal(400, "invalid_argument", None))?;
        let signed = funding.prepare(&public_key, command.amount)?;
        NativeTransfer::attest(signed, funding.sender_address(), config.treasury.as_ref())
            .map_err(|_| unavailable())?
    };
    persist_prepared(config, request, command, key, transfer)
}

pub(super) fn persist_prepared(
    config: &Config,
    request: &Request,
    command: &FundingCommand,
    key: &str,
    transfer: NativeTransfer,
) -> Result<NativeTransfer, Response> {
    let mut intent = NativeFundingIntent {
        version: 2,
        key: key.to_owned(),
        request_digest: request_digest(request),
        request_body: request.body.clone(),
        command: command.clone(),
        network_id: config.network_id,
        transfer,
        authorization: Vec::new(),
    };
    intent.authorization = config
        .treasury
        .sign_digest(&intent.digest().map_err(|_| unavailable())?)
        .map_err(|_| refusal(503, "treasury_signer_unavailable", Some(5)))?
        .to_vec();
    // The portable intent is durable before the broadcast stage. Recovery can
    // reconstruct a lost stage from this immutable identity without signing.
    store(config, &intent).map_err(|_| unavailable())?;
    stage(config, &intent)?;
    Ok(intent.transfer)
}

pub(super) fn route(config: &Config, request: &Request) -> Response {
    match route_result(config, request) {
        Ok(response) | Err(response) => response,
    }
}

fn route_result(config: &Config, request: &Request) -> Result<Response, Response> {
    if request.method == "POST" && request.path == "/admin/v1/testnet/funding-intents" {
        let document: serde_json::Value = serde_json::from_slice(&request.body)
            .map_err(|_| refusal(400, "invalid_funding_intent", None))?;
        let (intent, archive) = if document.get("intent").is_some() {
            let recovery: NativeFundingRecovery = serde_json::from_slice(&request.body)
                .map_err(|_| refusal(400, "invalid_funding_intent", None))?;
            if recovery.version != 2 {
                return Err(refusal(400, "invalid_funding_intent", None));
            }
            (recovery.intent, recovery.archive)
        } else {
            (
                serde_json::from_slice::<NativeFundingIntent>(&request.body)
                    .map_err(|_| refusal(400, "invalid_funding_intent", None))?,
                None,
            )
        };
        if request.headers.get("idempotency-key") != Some(&intent.key)
            || intent.validate(config).is_err()
        {
            return Err(refusal(400, "invalid_funding_intent", None));
        }
        if let Some(archive) = &archive {
            validate_archive(config, &intent, archive)
                .map_err(|_| refusal(400, "invalid_funding_archive", None))?;
        }
        if let Some(prior) = journal_read(&journal_path(config, "fund-canonical", &intent.key))
            .map_err(|_| unavailable())?
        {
            if prior.request_digest != command_digest(&intent.command)?
                || decode_stage(config, &intent.command, &prior.body)? != intent.transfer
            {
                return Err(refusal(409, "idempotency_conflict", None));
            }
        }
        store(config, &intent).map_err(|_| refusal(409, "idempotency_conflict", None))?;
        stage(config, &intent)?;
        if let Some(archive) = archive {
            funding_store(
                &funding_record_path(config, "funding-receipts", &intent.key),
                &serde_json::to_vec(&archive).map_err(|_| unavailable())?,
            )
            .map_err(|_| refusal(409, "idempotency_conflict", None))?;
        }
        return Ok(success(
            &serde_json::json!({"state": "prepared", "intent": intent}),
        ));
    }
    if request.method != "GET" {
        return Err(refusal(405, "method_not_allowed", None));
    }
    let key = request
        .path
        .strip_prefix("/admin/v1/testnet/funding-intents/")
        .filter(|key| valid_key(key))
        .ok_or_else(|| refusal(400, "invalid_funding_intent", None))?;
    let intent = read(config, key)?.ok_or_else(|| refusal(404, "not_found", None))?;
    let archive = funding_read(&funding_record_path(config, "funding-receipts", key))
        .map_err(|_| unavailable())?
        .map(|bytes| serde_json::from_slice::<NativeFundingArchive>(&bytes))
        .transpose()
        .map_err(|_| unavailable())?;
    if let Some(archive) = &archive {
        validate_archive(config, &intent, archive).map_err(|_| unavailable())?;
    }
    let recovery = NativeFundingRecovery {
        version: 2,
        intent: intent.clone(),
        archive,
    };
    Ok(success(&serde_json::json!({
        "state": "prepared", "transaction_hash": intent.transfer.signed.transaction_hash,
        "sender": hex_encode(&intent.transfer.sender), "canonical_hex": hex_encode(&intent.transfer.signed.raw),
        "intent": intent, "recovery": recovery,
    })))
}

fn archive_digest(archive: &NativeFundingArchive) -> Result<[u8; 32], String> {
    let bytes = serde_json::to_vec(&(
        "layerx-core/native-funding-archive/v2",
        archive.version,
        &archive.intent_digest,
        &archive.receipt,
    ))
    .map_err(|error| error.to_string())?;
    Ok(Sha256::digest(bytes).into())
}

fn validate_archive(
    config: &Config,
    intent: &NativeFundingIntent,
    archive: &NativeFundingArchive,
) -> Result<bool, String> {
    intent.validate(config)?;
    let intent_bytes = serde_json::to_vec(intent).map_err(|error| error.to_string())?;
    if archive.version != 2 || archive.intent_digest != hex_encode(&Sha256::digest(intent_bytes)) {
        return Err("native funding archive identity differs".into());
    }
    verify_authorization(config, &archive_digest(archive)?, &archive.authorization)?;
    chain_fund::validate_native_confirmation(&intent.transfer, &archive.receipt)
}

pub(super) fn archive(
    config: &Config,
    key: &str,
    receipt: serde_json::Value,
) -> Result<(), Response> {
    let intent = read(config, key)?.ok_or_else(unavailable)?;
    chain_fund::validate_native_confirmation(&intent.transfer, &receipt)
        .map_err(|_| unavailable())?;
    let intent_bytes = funding_read(&funding_record_path(config, "funding-intents", key))
        .map_err(|_| unavailable())?
        .ok_or_else(unavailable)?;
    let mut archive = NativeFundingArchive {
        version: 2,
        intent_digest: hex_encode(&Sha256::digest(intent_bytes)),
        receipt,
        authorization: Vec::new(),
    };
    archive.authorization = config
        .treasury
        .sign_digest(&archive_digest(&archive).map_err(|_| unavailable())?)
        .map_err(|_| refusal(503, "treasury_signer_unavailable", Some(5)))?
        .to_vec();
    funding_store(
        &funding_record_path(config, "funding-receipts", key),
        &serde_json::to_vec(&archive).map_err(|_| unavailable())?,
    )
    .map_err(|_| unavailable())
}

pub(super) fn archived_response(
    config: &Config,
    request: &Request,
    key: &str,
) -> Result<Option<Response>, Response> {
    let intent = read(config, key)?.ok_or_else(unavailable)?;
    if intent.request_digest != request_digest(request) {
        return Err(refusal(409, "idempotency_conflict", None));
    }
    let Some(bytes) = funding_read(&funding_record_path(config, "funding-receipts", key))
        .map_err(|_| unavailable())?
    else {
        return Ok(None);
    };
    let archive: NativeFundingArchive =
        serde_json::from_slice(&bytes).map_err(|_| unavailable())?;
    let funded = validate_archive(config, &intent, &archive).map_err(|_| unavailable())?;
    let mut response = if funded {
        json_response(
            200,
            &serde_json::json!({"funding_id": intent.command.funding_id,
            "state": "funded", "transaction_id": intent.transfer.signed.transaction_hash}),
        )
    } else {
        refusal(422, "funding_failed", None)
    };
    response.authoritative_replay = true;
    Ok(Some(response))
}

pub(super) fn retain(config: &Config, bytes: &[u8]) -> Result<(), Response> {
    let intent: NativeFundingIntent = serde_json::from_slice(bytes).map_err(|_| unavailable())?;
    intent.validate(config).map_err(|_| unavailable())?;
    let request = Request {
        method: "POST".into(),
        path: "/admin/v1/testnet/fund".into(),
        query: None,
        headers: BTreeMap::new(),
        body: intent.request_body.clone(),
    };
    if archived_response(config, &request, &intent.key)?.is_none() {
        let funding = config
            .funding
            .get()
            .map_err(|_| refusal(503, "treasury_unavailable", Some(5)))?;
        let receipt = funding
            .confirmation(&intent.transfer)?
            .ok_or_else(|| refusal(503, "receipt_unavailable", Some(5)))?;
        archive(config, &intent.key, receipt)?;
    }
    Ok(())
}
