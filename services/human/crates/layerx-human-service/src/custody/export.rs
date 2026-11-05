use std::fs::{self, File};
use std::io::Read as _;
use std::path::{Path, PathBuf};

use layerx_human_kms::export::{ExportAcknowledgement, ExportCeremony, ExportEnvelope};
use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};

use super::{
    encode_record_parts, replace_atomic, require_directory, write_atomic, CustodyError, KeyClass,
    KeyId, KeyRecord, Keystore, KmsError, PrincipalKeyBinding, KEY_FILE_SUFFIX, TEMP_FILE_SUFFIX,
};
use crate::store::PrincipalId;

const DELIVERY_LIMIT: u64 = 8192;

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct Delivery {
    binding: [u8; 32],
    reference_digest: [u8; 32],
    public_key: [u8; 32],
    ceremony: ExportCeremony,
    envelope: Option<ExportEnvelope>,
    acknowledged: bool,
}

impl Keystore {
    /// Publishes or recovers the same encrypted delivery until owner acknowledgement.
    /// The recipient must retain its ceremony identifier and decryption key.
    pub fn export_primary_envelope(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        ceremony: &ExportCeremony,
    ) -> Result<ExportEnvelope, CustodyError> {
        ceremony
            .validate()
            .map_err(|_| CustodyError::InvalidEvidence)?;
        let (directory, _lock) = self.delivery_lock(principal, key)?;
        let (record, binding) = self.export_binding(principal, key)?;
        let previous = read_delivery(&directory)?;
        let mut delivery = match previous {
            Some(delivery) => {
                validate_delivery(&delivery, &record, &binding, ceremony)?;
                delivery
            }
            None => {
                if record.self_custodied {
                    return Err(CustodyError::SelfCustodied);
                }
                let delivery = Delivery {
                    binding: binding.digest(),
                    reference_digest: Sha256::digest(record.provider_reference.as_bytes()).into(),
                    public_key: record.public_key,
                    ceremony: ceremony.clone(),
                    envelope: None,
                    acknowledged: false,
                };
                persist_delivery(&directory, &delivery, true)?;
                delivery
            }
        };
        if delivery.acknowledged {
            return Err(CustodyError::SelfCustodied);
        }
        if let Some(envelope) = delivery.envelope {
            return Ok(envelope);
        }
        let envelope = self.provider.export_envelope(
            &binding,
            &record.provider_reference,
            &delivery.ceremony,
        )?;
        validate_envelope(&envelope, &delivery.ceremony)?;
        delivery.envelope = Some(envelope.clone());
        persist_delivery(&directory, &delivery, false)?;
        Ok(envelope)
    }

    /// Accepts the recipient's decryption proof and durably retires local signing.
    /// An uncertain provider reply or local write failure can be retried after restart.
    pub fn acknowledge_primary_export(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        acknowledgement: &ExportAcknowledgement,
    ) -> Result<(), CustodyError> {
        acknowledgement
            .ceremony
            .validate()
            .map_err(|_| CustodyError::InvalidEvidence)?;
        let (directory, _lock) = self.delivery_lock(principal, key)?;
        let (record, binding) = self.export_binding(principal, key)?;
        let mut delivery = read_delivery(&directory)?.ok_or(CustodyError::InvalidEvidence)?;
        validate_delivery(&delivery, &record, &binding, &acknowledgement.ceremony)?;
        if delivery.envelope.is_none() && !delivery.acknowledged {
            return Err(CustodyError::InvalidEvidence);
        }
        self.provider
            .acknowledge_export(&binding, &record.provider_reference, acknowledgement)?;
        let bytes = encode_record_parts(
            record.class,
            record.public_key,
            binding.digest(),
            record.provider_reference.as_bytes(),
            true,
        )?;
        let temporary = self
            .principal_directory(principal)
            .join(format!("{}{TEMP_FILE_SUFFIX}", key.as_str()));
        match fs::symlink_metadata(&temporary) {
            Ok(metadata) if metadata.is_file() && metadata.len() <= bytes.len() as u64 => {
                let pending = fs::read(&temporary)?;
                if !bytes.starts_with(&pending) {
                    return Err(CustodyError::CorruptRecord(
                        "export marker recovery conflicted",
                    ));
                }
                fs::remove_file(&temporary)?;
                File::open(self.principal_directory(principal))?.sync_all()?;
            }
            Ok(_) => {
                return Err(CustodyError::CorruptRecord(
                    "export marker temporary invalid",
                ))
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => (),
            Err(error) => return Err(error.into()),
        }
        replace_atomic(
            &self.principal_directory(principal),
            &format!("{}{KEY_FILE_SUFFIX}", key.as_str()),
            &format!("{}{TEMP_FILE_SUFFIX}", key.as_str()),
            &bytes,
        )?;
        delivery.acknowledged = true;
        delivery.envelope = None;
        persist_delivery(&directory, &delivery, false)
    }

    fn export_binding(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
    ) -> Result<(KeyRecord, PrincipalKeyBinding), CustodyError> {
        let record = self.read_record(principal, key)?;
        if record.class != KeyClass::HumanPrimary {
            return Err(CustodyError::Kms(KmsError::Refused));
        }
        let binding = self.binding(principal, key, record.class)?;
        Self::require_record_binding(&binding, &record)?;
        Ok((record, binding))
    }

    fn delivery_lock(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
    ) -> Result<(PathBuf, File), CustodyError> {
        use rustix::fs::{flock, open, FlockOperation, Mode, OFlags};

        let mut directory = self.root.clone();
        for name in ["exports", principal.as_str(), key.as_str()] {
            let parent = directory.clone();
            directory.push(name);
            match fs::create_dir(&directory) {
                Ok(()) => File::open(parent)?.sync_all()?,
                Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => (),
                Err(error) => return Err(error.into()),
            }
            require_directory(&directory, "export delivery directory invalid")?;
        }
        let lock = File::from(
            open(
                directory.join("owner.lock"),
                OFlags::CREATE | OFlags::RDWR | OFlags::NOFOLLOW | OFlags::CLOEXEC,
                Mode::from_bits_truncate(0o600),
            )
            .map_err(std::io::Error::from)?,
        );
        flock(&lock, FlockOperation::LockExclusive).map_err(std::io::Error::from)?;
        // A pending rename never supersedes the last durable ceremony. The
        // provider is contacted only after that ceremony has been published.
        let pending = directory.join("delivery.next");
        match fs::symlink_metadata(&pending) {
            Ok(metadata) if metadata.is_file() => {
                fs::remove_file(pending)?;
                File::open(&directory)?.sync_all()?;
            }
            Ok(_) => {
                return Err(CustodyError::CorruptRecord(
                    "export temporary entry invalid",
                ))
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => (),
            Err(error) => return Err(error.into()),
        }
        Ok((directory, lock))
    }
}

fn read_delivery(directory: &Path) -> Result<Option<Delivery>, CustodyError> {
    use rustix::fs::{open, Mode, OFlags};

    let descriptor = match open(
        directory.join("delivery.json"),
        OFlags::RDONLY | OFlags::NOFOLLOW | OFlags::CLOEXEC,
        Mode::empty(),
    ) {
        Ok(descriptor) => descriptor,
        Err(rustix::io::Errno::NOENT) => return Ok(None),
        Err(error) => return Err(std::io::Error::from(error).into()),
    };
    let file = File::from(descriptor);
    let metadata = file.metadata()?;
    if !metadata.is_file() || metadata.len() > DELIVERY_LIMIT {
        return Err(CustodyError::CorruptRecord("export delivery entry invalid"));
    }
    let mut bytes = Vec::new();
    file.take(DELIVERY_LIMIT + 1).read_to_end(&mut bytes)?;
    if bytes.len() as u64 > DELIVERY_LIMIT {
        return Err(CustodyError::CorruptRecord("export delivery too large"));
    }
    serde_json::from_slice(&bytes)
        .map(Some)
        .map_err(|_| CustodyError::CorruptRecord("export delivery encoding invalid"))
}

fn persist_delivery(
    directory: &Path,
    delivery: &Delivery,
    create: bool,
) -> Result<(), CustodyError> {
    let bytes = serde_json::to_vec(delivery).map_err(|_| CustodyError::InvalidEvidence)?;
    if create {
        write_atomic(directory, "delivery.json", "delivery.next", &bytes)
    } else {
        replace_atomic(directory, "delivery.json", "delivery.next", &bytes)
    }
}

fn validate_delivery(
    delivery: &Delivery,
    record: &KeyRecord,
    binding: &PrincipalKeyBinding,
    ceremony: &ExportCeremony,
) -> Result<(), CustodyError> {
    let reference_digest: [u8; 32] = Sha256::digest(record.provider_reference.as_bytes()).into();
    if delivery.binding != binding.digest()
        || delivery.reference_digest != reference_digest
        || delivery.public_key != record.public_key
    {
        return Err(CustodyError::Kms(KmsError::Integrity));
    }
    if &delivery.ceremony != ceremony {
        return Err(CustodyError::Kms(KmsError::Conflict));
    }
    if delivery.acknowledged && delivery.envelope.is_some() {
        return Err(CustodyError::CorruptRecord(
            "acknowledged export retained delivery",
        ));
    }
    if let Some(envelope) = &delivery.envelope {
        validate_envelope(envelope, ceremony)?;
    }
    Ok(())
}

fn validate_envelope(
    envelope: &ExportEnvelope,
    ceremony: &ExportCeremony,
) -> Result<(), CustodyError> {
    if &envelope.ceremony != ceremony
        || envelope.ciphertext.len() != 80
        || envelope.ephemeral.len() != 33
        || k256::PublicKey::from_sec1_bytes(&envelope.ephemeral).is_err()
    {
        return Err(CustodyError::Kms(KmsError::Integrity));
    }
    Ok(())
}
