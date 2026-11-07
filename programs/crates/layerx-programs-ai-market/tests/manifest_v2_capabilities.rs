use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::{
    codec::{
        self, Envelope, Writer, PARTICIPANT_METADATA_DOMAIN_V1, PARTICIPANT_METADATA_DOMAIN_V2,
    },
    dispatch,
    errors::{CodecResult, BAD_VERSION, CAPACITY, F02_METADATA_INTEGRITY_FAILURE, NON_CANONICAL},
    registry::{derive_rewards_account, MarketHeader},
    roster::ParticipantRosterEntry,
    state::{self, ActorSlot, Control, ReplayTable, Section, SharedState},
    types::{
        AccountId, AssetId, Authentication, ChainDomain, MarketId, MetadataDigest, Presence,
        PrincipalId, ProgramId, PublicKey32, RequestId, Signature64, Version, WorkerId,
        WorkerRosterEntry,
    },
    workers::{
        self, decode_manifest, CapabilityFields, ManifestV2, WorkerCurrent, WorkerState,
        WorkerTable, MAX_MANIFEST_BYTES,
    },
    MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_STATE_BYTES,
};
use sha2::{Digest, Sha256};

fn manifest_v1() -> CodecResult<Vec<u8>> {
    let mut bytes = vec![0; MAX_MANIFEST_BYTES];
    let mut w = Writer::new(&mut bytes);
    w.u16(1)?;
    w.put(&[10; 32])?;
    w.put(&[11; 32])?;
    w.put(&[12; 32])?;
    for value in [2, 3, 5, 300, 428] {
        w.u64(value)?;
    }
    w.put(&[13; 32])?;
    w.u16(1)?;
    w.u8(1)?;
    w.u8(1)?;
    for value in 20..25 {
        w.put(&[value; 32])?;
    }
    for value in [1024, 2048, 4096, 8192] {
        w.u32(value)?;
    }
    w.u8(1)?;
    w.u32(60_000)?;
    w.u16(4)?;
    w.u8(1)?;
    w.u16(1)?;
    w.u8(1)?;
    w.bytes(b"https://worker.example/paxai/v1", workers::MAX_URI_BYTES)?;
    w.put(&[30; 32])?;
    w.u8(1)?;
    w.u16(1)?;
    w.u16(1)?;
    w.put(&[31; 32])?;
    w.put(&[32; 32])?;
    w.u32(0)?;
    let len = w.len();
    bytes.truncate(len);
    Ok(bytes)
}

fn capabilities() -> CapabilityFields {
    CapabilityFields {
        accelerator_memory_mib: 24_576,
        system_memory_mib: 65_536,
        bandwidth_class: 4,
        region_code: *b"US",
        attestation_family: 255,
        runtime_kind: 2,
    }
}

fn manifest_v2(fields: CapabilityFields) -> CodecResult<Vec<u8>> {
    let v1 = manifest_v1()?;
    let mut bytes = vec![0; MAX_MANIFEST_BYTES];
    let len = codec::encode_manifest_v2(
        &ManifestV2 {
            manifest_v1: &v1,
            capabilities: fields,
        },
        &mut bytes,
    )?;
    bytes.truncate(len);
    Ok(bytes)
}

fn worker_roster(manifest: &[u8]) -> CodecResult<WorkerRosterEntry> {
    let summary = decode_manifest(manifest)?;
    Ok(WorkerRosterEntry {
        worker: summary.worker,
        owner: summary.owner,
        recipient: AccountId::new([40; 32])?,
        generation: Version::new(summary.generation)?,
        key_version: Version::new(summary.key_version)?,
        public_key: PublicKey32([41; 32]),
        metadata: summary.digest,
    })
}

fn hash(domain: &str, bytes: &[u8]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update(domain.as_bytes());
    hash.update([0]);
    hash.update(bytes);
    hash.finalize().into()
}

#[test]
fn v2_decodes_all_inference_capability_fields() -> CodecResult<()> {
    let v1 = manifest_v1()?;
    let bytes = manifest_v2(capabilities())?;
    assert_eq!(bytes.len(), v1.len() + 13);
    assert_eq!(&bytes[..2], &2u16.to_be_bytes());
    assert_eq!(&bytes[2..v1.len()], &v1[2..]);
    assert_eq!(
        &bytes[v1.len()..],
        &[0, 0, 0x60, 0, 0, 1, 0, 0, 4, b'U', b'S', 255, 2]
    );
    let decoded = decode_manifest(&bytes)?;
    assert_eq!(decoded.version, 2);
    assert_eq!(decoded.capabilities, Some(capabilities()));
    let legacy = decode_manifest(&v1)?;
    assert_eq!(decoded.market, legacy.market);
    assert_eq!(decoded.worker, legacy.worker);
    assert_eq!(decoded.owner, legacy.owner);
    assert_eq!(decoded.generation, legacy.generation);
    assert_eq!(decoded.key_version, legacy.key_version);
    assert_eq!(decoded.revision, legacy.revision);
    assert_eq!(decoded.valid_from, legacy.valid_from);
    assert_eq!(decoded.expiry, legacy.expiry);
    Ok(())
}

#[test]
fn v1_decodes_unchanged_with_its_original_digest() -> CodecResult<()> {
    let bytes = manifest_v1()?;
    let decoded = decode_manifest(&bytes)?;
    assert_eq!(decoded.version, 1);
    assert_eq!(decoded.capabilities, None);
    assert_eq!(decoded.market, MarketId::new([10; 32])?);
    assert_eq!(decoded.worker, WorkerId::new([11; 32])?);
    assert_eq!(decoded.owner, PrincipalId::new([12; 32])?);
    assert_eq!(
        (decoded.generation, decoded.key_version, decoded.revision),
        (2, 3, 5)
    );
    assert_eq!((decoded.valid_from, decoded.expiry), (300, 428));
    assert_eq!(
        decoded.digest.bytes(),
        hash("PAXAI/worker-metadata/v1", &bytes)
    );
    Ok(())
}

#[test]
fn metadata_digest_uses_versioned_domains_and_commits_every_capability() -> CodecResult<()> {
    let v1 = manifest_v1()?;
    let v2 = manifest_v2(capabilities())?;
    assert_eq!(PARTICIPANT_METADATA_DOMAIN_V1, "PAXAI/worker-metadata/v1");
    assert_eq!(
        PARTICIPANT_METADATA_DOMAIN_V2,
        "PAXAI/participant-metadata/v2"
    );
    let digest = decode_manifest(&v2)?.digest;
    assert_eq!(digest.bytes(), hash("PAXAI/participant-metadata/v2", &v2));
    assert_ne!(digest.bytes(), hash("PAXAI/worker-metadata/v1", &v2));
    assert_ne!(digest, decode_manifest(&v1)?.digest);
    for offset in [0, 4, 8, 9, 10, 11, 12] {
        let mut changed = v2.clone();
        match offset {
            8 => changed[v1.len() + offset] = 3,
            12 => changed[v1.len() + offset] = 1,
            _ => changed[v1.len() + offset] ^= 1,
        }
        assert_ne!(decode_manifest(&changed)?.digest, digest);
    }
    Ok(())
}

#[test]
fn v2_refuses_unknown_runtime_and_bandwidth_values() -> CodecResult<()> {
    let offset = manifest_v1()?.len();
    for (field, values) in [(12, [3, 255]), (8, [5, 255])] {
        for value in values {
            let mut bytes = manifest_v2(capabilities())?;
            bytes[offset + field] = value;
            assert_eq!(decode_manifest(&bytes), Err(NON_CANONICAL));
        }
    }
    Ok(())
}

#[test]
fn v2_refuses_three_letter_and_non_ascii_letter_regions() -> CodecResult<()> {
    let offset = manifest_v1()?.len();
    let mut fields = capabilities();
    fields.attestation_family = 0;
    fields.runtime_kind = 1;
    let mut three_letters = manifest_v2(fields)?;
    three_letters.insert(offset + 11, b'A');
    assert_eq!(&three_letters[offset + 9..offset + 12], b"USA");
    assert_eq!(decode_manifest(&three_letters), Err(NON_CANONICAL));
    for region in [*b"U1", *b"U\0", [0xc3, 0xa9]] {
        let mut bytes = manifest_v2(fields)?;
        bytes[offset + 9..offset + 11].copy_from_slice(&region);
        assert_eq!(decode_manifest(&bytes), Err(NON_CANONICAL));
    }
    Ok(())
}

#[test]
fn v2_accepts_declared_runtime_bandwidth_and_memory_boundaries() -> CodecResult<()> {
    for runtime_kind in 0..=2 {
        for bandwidth_class in 0..=4 {
            for memory in [0, u32::MAX] {
                let fields = CapabilityFields {
                    accelerator_memory_mib: memory,
                    system_memory_mib: memory,
                    runtime_kind,
                    bandwidth_class,
                    region_code: *b"gb",
                    attestation_family: 255,
                };
                assert_eq!(
                    decode_manifest(&manifest_v2(fields)?)?.capabilities,
                    Some(fields)
                );
            }
        }
    }
    Ok(())
}

#[test]
fn manifest_cap_version_and_exact_framing_are_enforced() -> CodecResult<()> {
    assert_eq!(MAX_MANIFEST_BYTES, 8192);
    assert_eq!(decode_manifest(&vec![0; 8193]), Err(CAPACITY));
    assert_ne!(decode_manifest(&vec![0; 8192]), Err(CAPACITY));
    let v1 = manifest_v1()?;
    let v2 = manifest_v2(capabilities())?;
    for len in v1.len()..v2.len() {
        assert_eq!(decode_manifest(&v2[..len]), Err(NON_CANONICAL));
    }
    let mut trailing = v2.clone();
    trailing.push(0);
    assert_eq!(decode_manifest(&trailing), Err(NON_CANONICAL));
    let mut wrong_version = v2;
    wrong_version[..2].copy_from_slice(&3u16.to_be_bytes());
    assert_eq!(decode_manifest(&wrong_version), Err(BAD_VERSION));
    let mut short_out = vec![0; v1.len() + 12];
    assert_eq!(
        codec::encode_manifest_v2(
            &ManifestV2 {
                manifest_v1: &v1,
                capabilities: capabilities()
            },
            &mut short_out
        ),
        Err(CAPACITY)
    );
    Ok(())
}

#[test]
fn roster_preserves_capabilities_and_v1_encoding_across_reload() -> CodecResult<()> {
    for manifest in [manifest_v1()?, manifest_v2(capabilities())?] {
        let worker = worker_roster(&manifest)?;
        let entry = ParticipantRosterEntry::from_manifest(worker, &manifest)?;
        assert_eq!(entry.capabilities, decode_manifest(&manifest)?.capabilities);
        assert_eq!(entry.worker.metadata, decode_manifest(&manifest)?.digest);
        let mut stored = vec![0; codec::WORKER_ROSTER_BYTES + CapabilityFields::ENCODED_BYTES];
        let len = codec::encode_participant_roster(&entry, &mut stored)?;
        stored.truncate(len);
        let reloaded = codec::decode_participant_roster(&stored, &manifest)?;
        assert_eq!(reloaded, entry);
        if entry.capabilities.is_none() {
            let mut legacy = vec![0; codec::WORKER_ROSTER_BYTES];
            codec::encode_worker_roster(&worker, &mut legacy)?;
            assert_eq!(stored, legacy);
            assert_eq!(codec::decode_worker_roster(&stored)?, worker);
        } else {
            assert_eq!(stored.len(), codec::WORKER_ROSTER_BYTES + 13);
            stored[codec::WORKER_ROSTER_BYTES] ^= 1;
            assert_eq!(
                codec::decode_participant_roster(&stored, &manifest),
                Err(F02_METADATA_INTEGRITY_FAILURE)
            );
        }
    }
    Ok(())
}

#[test]
fn roster_refuses_manifest_identity_version_and_digest_mismatches() -> CodecResult<()> {
    let manifest = manifest_v2(capabilities())?;
    let worker = worker_roster(&manifest)?;
    let mut wrong_worker = worker;
    wrong_worker.worker = WorkerId::new([90; 32])?;
    let mut wrong_owner = worker;
    wrong_owner.owner = PrincipalId::new([91; 32])?;
    let mut wrong_generation = worker;
    wrong_generation.generation = Version::new(3)?;
    let mut wrong_key = worker;
    wrong_key.key_version = Version::new(4)?;
    let mut wrong_digest = worker;
    wrong_digest.metadata = MetadataDigest::new([92; 32])?;
    for entry in [
        wrong_worker,
        wrong_owner,
        wrong_generation,
        wrong_key,
        wrong_digest,
    ] {
        assert_eq!(
            ParticipantRosterEntry::from_manifest(entry, &manifest),
            Err(F02_METADATA_INTEGRITY_FAILURE)
        );
    }
    let mut legacy_bytes = vec![0; codec::WORKER_ROSTER_BYTES];
    codec::encode_worker_roster(&worker, &mut legacy_bytes)?;
    assert_eq!(
        codec::decode_participant_roster(&legacy_bytes, &manifest),
        Err(F02_METADATA_INTEGRITY_FAILURE)
    );
    Ok(())
}

#[test]
fn signed_v2_publication_persists_digest_for_the_capability_roster() -> CodecResult<()> {
    let manifest = manifest_v2(capabilities())?;
    let summary = decode_manifest(&manifest)?;
    let chain = ChainDomain::new([50; 32])?;
    let program = ProgramId::new([51; 32])?;
    let asset = AssetId::new([52; 32])?;
    let market = MarketHeader {
        format_version: 1,
        market_id: summary.market,
        deployment_chain_domain: chain,
        program_id: program,
        owner_principal: PrincipalId::new([53; 32])?,
        funding_asset: asset,
        rewards_account: derive_rewards_account(program, asset)?,
        refund_recipient_account: AccountId::new([54; 32])?,
        treasury_principal: Presence::Absent,
        origin_height: 0,
        lifecycle: 2,
        state_revision: 1,
        highest_config_version: 1,
        active_config_version: 1,
        activation_epoch: 0,
        activation_scheduled: false,
        closure_requested_at: 0,
        close_phase: 0,
        close_cursor: 0,
        suspension_reason_digest: [0; 32],
        metadata_digest: MetadataDigest::new([55; 32])?,
        closing_request_digest: [0; 32],
        reserved: [0; 8],
    };
    let signing_key = SigningKey::from_bytes(&[56; 32]);
    let delegate = PublicKey32(signing_key.verifying_key().to_bytes());
    let mut table = WorkerTable::new();
    table.insert(&WorkerCurrent {
        worker: summary.worker,
        owner: summary.owner,
        delegate,
        metadata: MetadataDigest::new([57; 32])?,
        generation: summary.generation,
        key_version: summary.key_version,
        metadata_revision: summary.revision - 1,
        valid_from: 200,
        expiry: 400,
        revocation_sequence: 0,
        effective_epoch: 1,
        last_sequence: 0,
        last_request_id: [0; 32],
        last_request_digest: [0; 32],
        last_result_digest: [0; 32],
        state: WorkerState::Available,
        slot: 0,
        last_metadata_height: 200,
    })?;
    let mut identity = vec![0; workers::WORKER_TABLE_MAX_BYTES];
    let identity_len = table.encode(&mut identity)?;
    let mut replay = ReplayTable::new();
    replay.bind(ActorSlot::worker(0)?, summary.owner, Version::new(1)?)?;
    let shared = SharedState {
        revision: 1,
        feature_sections: [&[], &identity[..identity_len], &[], &[], &[]],
        control: Control {
            replay,
            feature_bytes: &[],
        },
    };
    let mut current = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; workers::CONTROL_SCRATCH_BYTES];
    let current_len = state::encode_shared_state(&shared, &mut current, &mut scratch)?;
    let mut payload = vec![0; MAX_MANIFEST_BYTES + 100];
    let mut w = Writer::new(&mut payload);
    w.put(summary.worker.as_bytes())?;
    w.u64(summary.revision - 1)?;
    w.u64(summary.revision)?;
    w.put(summary.digest.as_bytes())?;
    w.u64(summary.valid_from)?;
    w.u64(summary.expiry)?;
    w.bytes(&manifest, MAX_MANIFEST_BYTES)?;
    let payload_len = w.len();
    let mut envelope = Envelope {
        operation: dispatch::PublishMetadata,
        chain,
        program,
        market: summary.market,
        actor: summary.owner,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 1000,
        request: RequestId::new([58; 32])?,
        payload: &payload[..payload_len],
        authentication: Authentication::Delegate {
            key: delegate,
            signature: Signature64([0; 64]),
        },
    };
    let mut wire = vec![0; MAX_ENVELOPE_BYTES];
    let len = codec::encode_envelope(&envelope, &mut wire)?;
    let request_digest = codec::decode_envelope(&wire[..len])?.request_digest()?;
    envelope.authentication = Authentication::Delegate {
        key: delegate,
        signature: Signature64(signing_key.sign(request_digest.as_bytes()).to_bytes()),
    };
    let len = codec::encode_envelope(&envelope, &mut wire)?;
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let applied = workers::apply(
        &current[..current_len],
        &workers::CallContext {
            market: &market,
            invoking_principal: summary.owner,
            height: 300,
        },
        &wire[..len],
        &mut next,
        &mut event,
        &mut scratch,
    )?;
    let workers::Applied::Applied { state_len, .. } = applied else {
        return Err(NON_CANONICAL);
    };
    let reloaded = state::decode_shared_state(&next[..state_len])?;
    let persisted = WorkerTable::decode(reloaded.section(Section::IdentityRoster)?)?
        .get(summary.worker)
        .ok_or(NON_CANONICAL)?;
    assert_eq!(persisted.metadata, summary.digest);
    assert_eq!(persisted.metadata_revision, summary.revision);
    assert_eq!((persisted.valid_from, persisted.expiry), (300, 428));
    let mut frozen = worker_roster(&manifest)?;
    frozen.metadata = persisted.metadata;
    frozen.public_key = persisted.delegate;
    let entry = ParticipantRosterEntry::from_manifest(frozen, &manifest)?;
    assert_eq!(entry.capabilities, Some(capabilities()));
    let mut stored = vec![0; codec::WORKER_ROSTER_BYTES + 13];
    let len = codec::encode_participant_roster(&entry, &mut stored)?;
    assert_eq!(
        codec::decode_participant_roster(&stored[..len], &manifest)?,
        entry
    );
    Ok(())
}
