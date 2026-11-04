// Audit 22.30.111 findings and the out-of-scope wiring request are recorded in
// repair_22_33_103.kvx. Include the committed fixtures so this gate drives the
// same native source, concrete clients, remote signer and durable journal.

mod native {
    include!("committed_source.rs");

    #[test]
    fn native_root_source_archive_reaches_the_mirror_consumer() {
        use layerx_mirror::source::{
            MirrorCanonicalPosition, MirrorObservation, MirrorSourceId, MirrorTargetIdentity,
        };
        use layerx_mirror::{Archive, MirrorEvidenceLevel, MirrorVerifier, NodeHead};

        let document = support::fixture();
        let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
        assert_eq!(rpc.config.rpc.quorum, 2);
        for endpoint in &rpc.config.rpc.endpoints {
            assert!(endpoint.url.starts_with("https://"));
            assert!(endpoint.url.ends_with('/'));
        }
        let source = CommittedBlockSource::open(&rpc.config).expect("native root source");
        let block = source
            .acquire(committed_height(&document))
            .expect("committed block");
        let coordinate = CommittedCoordinate {
            height: block.header.height,
            block_hash: block.header.block_hash,
        };
        let archive = Archive::from_committed(
            &block,
            125,
            NodeHead {
                latest_sealed_batch: block.header.height,
                latest_committed: Some(coordinate),
            },
        )
        .expect("native archive");
        let observation = MirrorObservation::from_archive(
            MirrorSourceId::new("native-root-regression").expect("source id"),
            MirrorTargetIdentity::Ethereum {
                chain_id: 1,
                genesis_hash: [1; 32],
                contract: [2; 20],
                code_hash: [3; 32],
                publisher: [4; 20],
            },
            archive.bytes(),
            125,
            block.header.height,
            archive.commitment(),
            MirrorCanonicalPosition::Ethereum {
                block_number: 10,
                block_hash: [5; 32],
                reference_head_number: 12,
                reference_head_hash: [6; 32],
            },
        )
        .expect("archive observation");
        let verifier = MirrorVerifier::admit_committed(archive.bytes(), observation)
            .expect("mirror archive admission");
        let key = &document["receiptKey"];
        let keys = [SequencerAuthorization::new(
            support::bytes(&key["keyId"]).try_into().expect("key id"),
            support::bytes(&key["publicKey"])
                .try_into()
                .expect("public key"),
            key["firstHeight"].as_u64().expect("first height"),
            key["lastHeight"].as_u64().expect("last height"),
        )];
        assert_eq!(verifier.committed().expect("coordinate"), coordinate);
        assert_eq!(
            block.receipts.len(),
            document["receipts"].as_array().expect("receipts").len()
        );
        for (index, canonical) in block.receipts.iter().enumerate() {
            let verified = verifier
                .committed_receipt(index, &keys)
                .expect("mirror receipt");
            assert_eq!(verified.level(), MirrorEvidenceLevel::Committed);
            assert_eq!(verified.batch_number(), block.header.height);
            assert_eq!(
                layerx_wire::receipt::encode(verified.value()).expect("receipt bytes"),
                *canonical
            );
        }
        let requests = rpc.requests.lock().expect("request transcript");
        for endpoint in 0..2 {
            assert_eq!(
                requests
                    .iter()
                    .filter(|(peer, request)| *peer == endpoint
                        && request["method"] == "pax_sequencerReceipt")
                    .count(),
                block.receipts.len()
            );
        }
    }
}

mod publication {
    include!("repair_22_31_166.rs");

    fn recovered_record(
        fixture: &Fixture,
        directory: &Path,
        archive: &Archive,
        stage: PublicationStage,
    ) -> PublicationRecord {
        fixture
            .journal(directory)
            .record(archive.commitment(), stage)
            .expect("persisted stage")
            .clone()
    }

    fn finish_reopening_every_advance(fixture: &Fixture, directory: &Path, archive: &Archive) {
        let chunk_bytes = match fixture.chain {
            MirrorChain::Ethereum => 1024,
            MirrorChain::Solana => 720,
        };
        let stage_count = archive.bytes().len().div_ceil(chunk_bytes) + 2;
        let mut complete = false;
        for _ in 0..stage_count * 2 {
            let mut client = fixture.open(directory);
            let progress = client.advance(archive);
            drop(client);
            let journal = fixture.journal(directory);
            let record = journal
                .record(archive.commitment(), progress.stage)
                .expect("reopened stage");
            assert_eq!(record.phase, progress.phase);
            assert_eq!(record.commitment, archive.commitment());
            assert_eq!(record.committed, archive.data().committed_coordinate());
            if progress.phase == PublicationPhase::RetrievedVerified {
                assert_eq!(
                    journal.cursor().latest_batch,
                    Some(archive.data().batch_number)
                );
                assert_eq!(
                    journal.cursor().latest_committed,
                    archive.data().committed_coordinate()
                );
                complete = true;
                break;
            }
            assert_eq!(
                journal.cursor().latest_batch,
                None,
                "partial publication cannot advance freshness"
            );
        }
        assert!(
            complete,
            "recovery must finish within two advances per stage"
        );
        let client = fixture.open(directory);
        let retrieved = client.retrieve(archive);
        assert_eq!(
            retrieved,
            archive.bytes(),
            "byte-identical retrieval after reopening"
        );
        assert_eq!(
            layerx_mirror::archive_commitment(&retrieved),
            archive.commitment()
        );
        assert_eq!(
            layerx_mirror::ArchiveData::decode(&retrieved).expect("retrieved archive codec"),
            *archive.data()
        );
    }

    #[test]
    fn concrete_clients_recover_every_stage_to_byte_identical_retrieval() {
        for chain in [MirrorChain::Ethereum, MirrorChain::Solana] {
            let archive = archive();
            let fixture = Fixture::new(chain);
            let snapshots = fixture.snapshots(&archive);
            let mut phases = Vec::new();
            for snapshot in snapshots {
                let before = fixture.boundary_record(&snapshot, &archive);
                fixture.restore(&snapshot);
                if !phases.contains(&before.phase) {
                    phases.push(before.phase);
                }
                finish_reopening_every_advance(&fixture, &snapshot.directory, &archive);
                let after = recovered_record(&fixture, &snapshot.directory, &archive, before.stage);
                assert_eq!(after.stage_payload_digest, before.stage_payload_digest);
                if before.phase != PublicationPhase::Prepared {
                    assert_eq!(after.transaction, before.transaction);
                    assert_eq!(after.signed_payload, before.signed_payload);
                }
                let network = fixture.network.lock().unwrap();
                assert_eq!(
                    network
                        .transactions
                        .values()
                        .filter(|tx| tx.stage == before.stage)
                        .count(),
                    1
                );
                assert!(
                    network.chunks.len() > 1,
                    "recovery must retrieve multiple chunks"
                );
            }
            assert!(phases.contains(&PublicationPhase::Prepared));
            assert!(phases.contains(&PublicationPhase::Signed));
            assert!(phases.contains(&PublicationPhase::Pending));
        }
    }

    #[test]
    fn concrete_pending_recovery_retains_identity_then_retrieves_identical_archive() {
        for chain in [MirrorChain::Ethereum, MirrorChain::Solana] {
            let archive = archive();
            let fixture = Fixture::new(chain);
            let snapshots = fixture.snapshots(&archive);
            let snapshot = snapshots
                .iter()
                .find(|s| s.phase == PublicationPhase::Pending)
                .expect("pending snapshot");
            let before = fixture.boundary_record(snapshot, &archive);
            fixture.restore(snapshot);
            fixture.network.lock().unwrap().observation = Observation::Pending;
            for _ in 0..2 {
                let mut client = fixture.open(&snapshot.directory);
                assert_eq!(client.advance(&archive).phase, PublicationPhase::Pending);
                drop(client);
                let record =
                    recovered_record(&fixture, &snapshot.directory, &archive, before.stage);
                assert_eq!(record.phase, PublicationPhase::Pending);
                assert_eq!(record.transaction, before.transaction);
                assert_eq!(record.signed_payload, before.signed_payload);
                assert_eq!(
                    fixture.journal(&snapshot.directory).cursor().latest_batch,
                    None
                );
                let network = fixture.network.lock().unwrap();
                assert_eq!(network.signatures, snapshot.network.signatures);
                assert_eq!(network.broadcasts, snapshot.network.broadcasts);
            }
            fixture.network.lock().unwrap().observation = Observation::Finalized;
            finish_reopening_every_advance(&fixture, &snapshot.directory, &archive);
            let after = recovered_record(&fixture, &snapshot.directory, &archive, before.stage);
            assert_eq!(after.transaction, before.transaction);
            assert_eq!(after.signed_payload, before.signed_payload);
        }
    }

    #[test]
    fn concrete_solana_expiry_reopens_and_retrieves_identical_archive() {
        let archive = archive();
        let fixture = Fixture::new(MirrorChain::Solana);
        let snapshots = fixture.snapshots(&archive);
        let snapshot = snapshots
            .iter()
            .find(|s| s.phase == PublicationPhase::Pending)
            .expect("pending snapshot");
        let before = fixture.boundary_record(snapshot, &archive);
        fixture.restore(snapshot);
        {
            let mut network = fixture.network.lock().unwrap();
            network.observation = Observation::Missing;
            network.blockhash_valid = false;
            network.recent_blockhash = [0x44; 32];
        }
        let mut client = fixture.open(&snapshot.directory);
        assert_eq!(
            client.advance(&archive).phase,
            PublicationPhase::BroadcastExpired
        );
        drop(client);
        let expired = recovered_record(&fixture, &snapshot.directory, &archive, before.stage);
        assert_eq!(expired.phase, PublicationPhase::BroadcastExpired);
        assert_eq!(expired.transaction, before.transaction);
        assert_eq!(expired.signed_payload, before.signed_payload);
        assert_eq!(
            fixture.journal(&snapshot.directory).cursor().latest_batch,
            None
        );
        {
            let network = fixture.network.lock().unwrap();
            assert_eq!(network.signatures, snapshot.network.signatures);
            assert_eq!(network.broadcasts, snapshot.network.broadcasts);
        }
        fixture.network.lock().unwrap().observation = Observation::Pending;
        let mut client = fixture.open(&snapshot.directory);
        assert_eq!(client.advance(&archive).phase, PublicationPhase::Pending);
        drop(client);
        let replacement = recovered_record(&fixture, &snapshot.directory, &archive, before.stage);
        assert_ne!(replacement.transaction, before.transaction);
        assert_ne!(replacement.signed_payload, before.signed_payload);
        assert_eq!(
            replacement.stage_payload_digest,
            before.stage_payload_digest
        );
        fixture.network.lock().unwrap().observation = Observation::Finalized;
        finish_reopening_every_advance(&fixture, &snapshot.directory, &archive);
        assert_eq!(
            recovered_record(&fixture, &snapshot.directory, &archive, before.stage).transaction,
            replacement.transaction
        );
    }

    #[test]
    fn concrete_reorg_reopens_retreats_freshness_and_retrieves_identical_archive() {
        for chain in [MirrorChain::Ethereum, MirrorChain::Solana] {
            let archive = archive();
            let fixture = Fixture::new(chain);
            let _snapshots = fixture.snapshots(&archive);
            let directory = fixture.directory.0.join("live");
            let before =
                recovered_record(&fixture, &directory, &archive, PublicationStage::Finalize);
            assert_eq!(before.phase, PublicationPhase::RetrievedVerified);
            let (signatures, broadcasts) = {
                let mut network = fixture.network.lock().unwrap();
                network.observation = Observation::Missing;
                network.recent_blockhash = [0x45; 32];
                (network.signatures, network.broadcasts.len())
            };
            let mut client = fixture.open(&directory);
            assert_eq!(client.advance(&archive).phase, PublicationPhase::Reorged);
            drop(client);
            let reorged =
                recovered_record(&fixture, &directory, &archive, PublicationStage::Finalize);
            assert_eq!(reorged.phase, PublicationPhase::Reorged);
            assert_eq!(reorged.transaction, before.transaction);
            assert_eq!(reorged.signed_payload, before.signed_payload);
            assert_eq!(fixture.journal(&directory).cursor().latest_batch, None);
            assert_eq!(fixture.journal(&directory).cursor().latest_committed, None);
            fixture.network.lock().unwrap().observation = Observation::Finalized;
            finish_reopening_every_advance(&fixture, &directory, &archive);
            let after =
                recovered_record(&fixture, &directory, &archive, PublicationStage::Finalize);
            assert_eq!(after.stage_payload_digest, before.stage_payload_digest);
            let network = fixture.network.lock().unwrap();
            assert_eq!(network.broadcasts.len(), broadcasts + 1);
            match chain {
                MirrorChain::Ethereum => {
                    assert_eq!(after.transaction, before.transaction);
                    assert_eq!(after.signed_payload, before.signed_payload);
                    assert_eq!(network.signatures, signatures);
                }
                MirrorChain::Solana => {
                    assert_ne!(after.transaction, before.transaction);
                    assert_ne!(after.signed_payload, before.signed_payload);
                    assert_eq!(network.signatures, signatures + 1);
                }
            }
        }
    }
}

mod sequencer {
    include!("sequencer_vectors.rs");
}
