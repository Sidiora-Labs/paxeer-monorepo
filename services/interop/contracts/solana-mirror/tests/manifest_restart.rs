//! Task 22.33.89 repair record.
//! Finding solana_v4_registry: runtime::zero_block_hash_and_version_3_instructions_are_refused
//! proves live v3 refusal; runtime::version_4_manifest_binds_the_committed_block_hash and
//! runtime::solana_v4_registry_recovers_after_runtime_restart prove the v4 layout and recovery.
//! Wiring request (domain 11, interop): update the three 11.fmt-solana-mirror-* rows in spec/layerx-merge/interfaces.kvx from src/lib.rs and the actual publisher codecs, then regenerate spec/layerx-merge/contracts/interop-mirror-contracts.md; qualify the publisher/reader against these runtime accounts in services/interop/crates/layerx-mirror.
//! Baseline: no pre-change gate was run; the ordered gate rules require complete implementation, hf-build, then hf-done and prohibit a separate baseline verify.
//! Gotcha: fixtures/finalized.manifest is a v3 record; v4 recovery below uses accounts produced by the live v4 program instead.

mod runtime {
    // Reuse the committed runtime fixture driver and retain every original assertion.
    include!("runtime.rs");

    #[tokio::test]
    async fn solana_v4_registry_recovers_after_runtime_restart() {
        let program = Pubkey::new_unique();
        let mut context = ProgramTest::new(
            "layerx_solana_mirror_program",
            program,
            processor!(process_instruction),
        )
        .start_with_context()
        .await;
        let publisher = context.payer.pubkey();
        let commitment =
            solana_program::hash::hashv(&[b"LXP/mirror/archive/v2\0", ARCHIVE]).to_bytes();
        let manifest =
            Pubkey::find_program_address(&[b"manifest", publisher.as_ref(), &commitment], &program)
                .0;
        let chunks: Vec<_> = ARCHIVE.chunks(720).collect();
        assert!(chunks.len() > 1, "fixture must span a restart");
        let mut chain = [0; 32];
        for (index, value) in chunks.iter().enumerate() {
            chain = solana_program::hash::hashv(&[
                &chain,
                &(index as u32).to_be_bytes(),
                &Sha256::digest(value),
                &(value.len() as u32).to_be_bytes(),
            ])
            .to_bytes();
        }
        let height = 42_u64;
        let block_hash = solana_program::hash::hash(b"committed v4 recovery block").to_bytes();
        let mut begin = Vec::new();
        begin.extend_from_slice(&77_u32.to_be_bytes());
        begin.extend_from_slice(&height.to_be_bytes());
        begin.extend_from_slice(&block_hash);
        begin.extend_from_slice(&(ARCHIVE.len() as u64).to_be_bytes());
        begin.extend_from_slice(&(chunks.len() as u32).to_be_bytes());
        begin.extend_from_slice(&Sha256::digest(ARCHIVE));
        begin.extend_from_slice(&chain);
        let initialize = Instruction {
            program_id: program,
            accounts: vec![
                AccountMeta::new(publisher, true),
                AccountMeta::new(manifest, false),
                AccountMeta::new_readonly(system_program::id(), false),
            ],
            data: instruction(1, commitment, &begin),
        };
        let mut addresses = Vec::new();
        let mut appends = Vec::new();
        for (index, value) in chunks.iter().enumerate() {
            let index = (index as u32).to_be_bytes();
            let address = Pubkey::find_program_address(
                &[b"chunk", publisher.as_ref(), &commitment, &index],
                &program,
            )
            .0;
            addresses.push(address);
            let mut payload = index.to_vec();
            payload.extend_from_slice(&(value.len() as u16).to_be_bytes());
            payload.extend_from_slice(value);
            payload.extend_from_slice(&Sha256::digest(value));
            appends.push(Instruction {
                program_id: program,
                accounts: vec![
                    AccountMeta::new(publisher, true),
                    AccountMeta::new(manifest, false),
                    AccountMeta::new(address, false),
                    AccountMeta::new_readonly(system_program::id(), false),
                ],
                data: instruction(2, commitment, &payload),
            });
        }
        let finalize = Instruction {
            program_id: program,
            accounts: vec![
                AccountMeta::new(publisher, true),
                AccountMeta::new(manifest, false),
            ],
            data: instruction(3, commitment, &[]),
        };
        send(&mut context, std::slice::from_ref(&initialize))
            .await
            .expect("initialize v4 through the real runtime");
        send(&mut context, std::slice::from_ref(&appends[0]))
            .await
            .expect("persist the first chunk and a partial SHA-256 block");
        let partial = manifest_data(&mut context, manifest).await;
        assert_eq!(partial.len(), 334);
        assert_eq!(&partial[..8], b"LXMMAN04");
        assert_eq!(&partial[76..84], &height.to_be_bytes());
        assert_eq!(&partial[84..116], &block_hash);
        assert_eq!(&partial[224..232], &720_u64.to_be_bytes());
        assert_eq!(&partial[232..236], &1_u32.to_be_bytes());
        assert_eq!(partial[236], 0);
        assert_eq!(
            partial[333], 16,
            "recovery must restore a nonempty hash tail"
        );

        let payer = solana_sdk::signature::Keypair::from_bytes(&context.payer.to_bytes())
            .expect("retain the publisher identity across restart");
        let mut saved = Vec::new();
        for address in [publisher, manifest, addresses[0]] {
            let account = context
                .banks_client
                .get_account(address)
                .await
                .expect("snapshot real runtime account")
                .expect("account exists before restart");
            saved.push((address, account));
        }
        drop(context);
        let mut reopened = ProgramTest::new(
            "layerx_solana_mirror_program",
            program,
            processor!(process_instruction),
        );
        for (address, account) in saved {
            reopened.add_account(address, account);
        }
        let mut context = reopened.start_with_context().await;
        context.payer = payer;
        assert_eq!(manifest_data(&mut context, manifest).await, partial);
        send(&mut context, std::slice::from_ref(&initialize))
            .await
            .expect("reopening preserves idempotent initialization");
        send(&mut context, std::slice::from_ref(&appends[0]))
            .await
            .expect("replaying the persisted chunk preserves hash state");
        assert_eq!(manifest_data(&mut context, manifest).await, partial);
        assert_eq!(
            send(&mut context, std::slice::from_ref(&finalize))
                .await
                .expect_err("reopened incomplete archive must refuse finalization")
                .unwrap(),
            TransactionError::InstructionError(0, InstructionError::Custom(7)),
        );
        assert_eq!(manifest_data(&mut context, manifest).await, partial);
        for append in &appends[1..] {
            send(&mut context, std::slice::from_ref(append))
                .await
                .expect("resume appending after runtime restart");
        }
        send(&mut context, &[finalize.clone(), finalize])
            .await
            .expect("recovered archive digest and chunk chain finalize idempotently");
        let complete = manifest_data(&mut context, manifest).await;
        assert_eq!(&complete[8..40], &commitment);
        assert_eq!(&complete[40..72], publisher.as_ref());
        assert_eq!(&complete[72..76], &77_u32.to_be_bytes());
        assert_eq!(&complete[76..84], &height.to_be_bytes());
        assert_eq!(&complete[84..116], &block_hash);
        assert_eq!(&complete[116..124], &(ARCHIVE.len() as u64).to_be_bytes());
        assert_eq!(&complete[124..128], &(chunks.len() as u32).to_be_bytes());
        assert_eq!(&complete[128..160], Sha256::digest(ARCHIVE).as_slice());
        assert_eq!(&complete[160..192], &chain);
        assert_eq!(&complete[192..224], &chain);
        assert_eq!(&complete[224..232], &(ARCHIVE.len() as u64).to_be_bytes());
        assert_eq!(&complete[232..236], &(chunks.len() as u32).to_be_bytes());
        assert_eq!(complete[236], 1);
        let mut recovered = Vec::new();
        for (index, address) in addresses.iter().enumerate() {
            let account = context
                .banks_client
                .get_account(*address)
                .await
                .expect("read real chunk account")
                .expect("chunk survives publication");
            assert_eq!(account.owner, program);
            let data = account.data;
            assert_eq!(data.len(), 80 + chunks[index].len());
            assert_eq!(&data[..8], b"LXMCHK04");
            assert_eq!(&data[8..40], &commitment);
            assert_eq!(&data[40..44], &(index as u32).to_be_bytes());
            assert_eq!(&data[44..76], Sha256::digest(chunks[index]).as_slice());
            assert_eq!(&data[76..80], &(chunks[index].len() as u32).to_be_bytes());
            recovered.extend_from_slice(&data[80..]);
        }
        assert_eq!(recovered, ARCHIVE);
        send(&mut context, std::slice::from_ref(&initialize))
            .await
            .expect("finalized state survives initialization retry");
        assert_eq!(manifest_data(&mut context, manifest).await, complete);
    }
}
