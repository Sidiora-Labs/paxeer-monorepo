use crate::seal::StoreKey;
use crate::signup::{self, Desk, Refused, SignupRequest};
use crate::store::{SignupDelivery, Store, StoredSession, VerificationResult};
use serde_json::{json, Value};
use std::fs::{self, File, OpenOptions};
use std::io::Write;
use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt};
use std::path::{Path, PathBuf};
use zeroize::Zeroizing;

pub fn start(
    desk: &mut Desk,
    store: &mut Store,
    key: &StoreKey,
    outbox: &Path,
    request: &SignupRequest<'_>,
    token: &str,
    now: u64,
) -> Result<crate::store::PendingSignup, Refused> {
    let (email, _) = signup::normalize_email(request.email).ok_or(Refused::Invalid)?;
    let expires_at = now.saturating_add(desk.limits().token_ttl_seconds);
    let message = Zeroizing::new(
        json!({
            "kind": "signup_verification", "email": email, "token": token, "expires_at": expires_at,
        })
        .to_string(),
    );
    let delivery = SignupDelivery {
        sealed_message: key.seal(message.as_bytes()).map_err(|_| Refused::Store)?,
        expires_at,
    };
    let digest = signup::sha256_hex(token.as_bytes());
    let pending = desk.start_with_delivery(store, request, &digest, now, delivery.clone())?;
    publish(outbox, &digest, &delivery, key).map_err(|_| Refused::Delivery)?;
    Ok(pending)
}

pub struct StagedDelivery {
    temporary: PathBuf,
    published: PathBuf,
    directory: PathBuf,
}

impl StagedDelivery {
    pub fn publish(self) -> Result<(), String> {
        fs::rename(&self.temporary, &self.published).map_err(|e| e.to_string())?;
        sync(&self.directory)
    }
}

pub fn stage(
    outbox: &Path,
    digest: &str,
    delivery: &SignupDelivery,
    key: &StoreKey,
) -> Result<StagedDelivery, String> {
    if digest.len() != 64 || !digest.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err("invalid verification digest".to_owned());
    }
    let message = key.open(&delivery.sealed_message)?;
    let document: Value = serde_json::from_slice(&message).map_err(|e| e.to_string())?;
    if document["token"]
        .as_str()
        .map(|token| signup::sha256_hex(token.as_bytes()))
        .as_deref()
        != Some(digest)
        || document["expires_at"].as_u64() != Some(delivery.expires_at)
    {
        return Err("verification delivery binding mismatch".to_owned());
    }
    create_outbox(outbox)?;
    let temporary = outbox.join(format!("{digest}.tmp"));
    let mut file = OpenOptions::new()
        .create(true)
        .truncate(true)
        .write(true)
        .mode(0o600)
        .open(&temporary)
        .map_err(|e| e.to_string())?;
    file.write_all(&message)
        .and_then(|()| file.sync_all())
        .map_err(|e| e.to_string())?;
    Ok(StagedDelivery {
        temporary,
        published: outbox.join(format!("{digest}.json")),
        directory: outbox.to_owned(),
    })
}

pub fn publish(
    outbox: &Path,
    digest: &str,
    delivery: &SignupDelivery,
    key: &StoreKey,
) -> Result<(), String> {
    stage(outbox, digest, delivery, key)?.publish()
}

pub fn reconcile(store: &mut Store, key: &StoreKey, outbox: &Path, now: u64) -> Result<(), String> {
    for (digest, delivery) in store.signup_deliveries() {
        if delivery.expires_at <= now {
            for extension in ["tmp", "json"] {
                match fs::remove_file(outbox.join(format!("{digest}.{extension}"))) {
                    Ok(()) => sync(outbox)?,
                    Err(e) if e.kind() == std::io::ErrorKind::NotFound => (),
                    Err(e) => return Err(e.to_string()),
                }
            }
        } else {
            publish(outbox, &digest, &delivery, key)?;
        }
    }
    // Files whose metadata was retired by verification or admission cleanup still expire.
    if outbox.is_dir() {
        for entry in fs::read_dir(outbox).map_err(|e| e.to_string())? {
            let path = entry.map_err(|e| e.to_string())?.path();
            if !path
                .extension()
                .is_some_and(|ext| ext == "json" || ext == "tmp")
            {
                continue;
            }
            if fs::metadata(&path).map_err(|e| e.to_string())?.len() > 4096 {
                return Err("verification message exceeds its bound".to_owned());
            }
            let raw = Zeroizing::new(fs::read(&path).map_err(|e| e.to_string())?);
            // A torn temporary file will be replaced from the durable delivery record.
            let Ok(message) = serde_json::from_slice::<Value>(&raw) else {
                if path.extension().is_some_and(|ext| ext == "tmp") {
                    fs::remove_file(&path).map_err(|e| e.to_string())?;
                    sync(outbox)?;
                    continue;
                }
                return Err("invalid verification message".to_owned());
            };
            if message["expires_at"]
                .as_u64()
                .is_some_and(|expiry| expiry <= now)
            {
                fs::remove_file(&path).map_err(|e| e.to_string())?;
                sync(outbox)?;
            }
        }
    }
    store.reclaim_signups(now)
}

fn create_outbox(directory: &Path) -> Result<(), String> {
    if directory.is_dir() {
        return Ok(());
    }
    let parent = directory
        .parent()
        .filter(|p| !p.as_os_str().is_empty())
        .unwrap_or(Path::new("."));
    if !parent.is_dir() {
        create_outbox(parent)?;
    }
    fs::DirBuilder::new()
        .mode(0o700)
        .create(directory)
        .map_err(|e| e.to_string())?;
    sync(parent)
}

fn sync(directory: &Path) -> Result<(), String> {
    File::open(directory)
        .and_then(|f| f.sync_all())
        .map_err(|e| e.to_string())
}

fn random_hex(bytes: usize) -> Result<Zeroizing<String>, Refused> {
    let mut random = Zeroizing::new(vec![0_u8; bytes]);
    getrandom::fill(&mut random).map_err(|_| Refused::Entropy)?;
    Ok(Zeroizing::new(crate::seal::hex(&random)))
}

pub fn complete(
    store: &mut Store,
    key: &StoreKey,
    token: &str,
    sub: &str,
    ttl: u64,
    now: u64,
) -> Result<Value, Refused> {
    store.check_available().map_err(|_| Refused::Store)?;
    let digest = signup::sha256_hex(token.as_bytes());
    if let Some(result) = store.verification_result(&digest) {
        if result.sub != sub {
            return Err(Refused::TokenUnknown);
        }
        if result.expires_at <= now {
            return Err(Refused::TokenExpired);
        }
        let session = store.session(&result.session_id).ok_or(Refused::Store)?;
        if session.revoked_at.is_some() || session.expires_at <= now {
            return Err(Refused::TokenExpired);
        }
        let response = key
            .open(&result.sealed_response)
            .map_err(|_| Refused::Store)?;
        return serde_json::from_slice(&response).map_err(|_| Refused::Store);
    }
    let pending = store
        .pending_signup(&digest)
        .ok_or(Refused::TokenUnknown)?
        .clone();
    if pending.sub != sub {
        return Err(Refused::TokenUnknown);
    }
    if pending.expires_at <= now {
        return Err(Refused::TokenExpired);
    }
    if store.email_in_use(&pending.email_digest) {
        return Err(Refused::EmailRegistered);
    }
    if let Some(tenant) = store.subject_tenant(sub) {
        return Err(if tenant == pending.tenant {
            Refused::SubjectExists
        } else {
            Refused::SubjectConflict
        });
    }
    if ttl == 0 {
        return Err(Refused::Invalid);
    }
    // All entropy and response encoding precede the single durable verification/session commit.
    let session_id = random_hex(16)?;
    let secret = random_hex(32)?;
    let csrf = random_hex(32)?;
    let token = Zeroizing::new(format!("ses_{}.{}", session_id.as_str(), secret.as_str()));
    let expires_at = now.saturating_add(ttl);
    let response = json!({
        "session_id": session_id.as_str(), "tenant": pending.tenant, "sub": sub,
        "token": token.as_str(), "csrf_token": csrf.as_str(), "expires_at": expires_at,
    });
    let encoded = Zeroizing::new(response.to_string());
    let result = VerificationResult {
        sub: sub.to_owned(),
        session_id: session_id.to_string(),
        sealed_response: key.seal(encoded.as_bytes()).map_err(|_| Refused::Entropy)?,
        expires_at: pending.expires_at.min(expires_at),
    };
    let session = StoredSession {
        session_id: session_id.to_string(),
        tenant: pending.tenant,
        principal: sub.to_owned(),
        token_digest: signup::sha256_hex(secret.as_bytes()),
        csrf_digest: signup::sha256_hex(csrf.as_bytes()),
        csrf_sealed: key.seal(csrf.as_bytes()).map_err(|_| Refused::Entropy)?,
        issued_at: now,
        expires_at,
        revoked_at: None,
    };
    store
        .verify_signup_session(&digest, now, session, result)
        .map_err(signup::store_refusal)?;
    let _ = store.audit(&json!({"event":"signup_verified", "at":now, "sub":sub, "email_digest":pending.email_digest, "outcome":"accepted"}));
    Ok(response)
}
