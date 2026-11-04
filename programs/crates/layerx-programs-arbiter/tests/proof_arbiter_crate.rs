//! Proves: Executor + signed admission store -> ChainCommitProof + ExistenceProof + public Rust arbiter items -> VerifiedReplay::{verify, verify_with_batch_context, verify_step}, VerifiedReplayAuthority::{derive, derive_program_call}, and VerifiedMarketSandbox::verify(MarketReplayEvidence).
//!
//! The application harness owns its signing keys, state trees, code, and runtime.
//! It commits actual execution output and transports encoded proofs to the public
//! consumers used by authority_from_prestate.rs and verified_step.rs. Each pass
//! reconstructs verified objects from the producer bytes, with no retained cache.
//! This target deliberately does not include any other integration-test target.
//!
//! Repairs authority.rs::Grant::check_fee: the zero-padded native policy name
//! included its NUL in a copy into a slice sized for the name alone, panicking
//! for every authenticated caller. The harness installs the active policy so
//! successful authority derivation exercises the corrected lookup.
//!
//! External defect: programs/crates/layerx-programs-runtime/src/execute.rs::
//! ProgramInstance::call_market_sandbox_untrusted refuses an unreachable instruction
//! with an UnsupportedState observer fault instead of producing a terminal trap
//! record. runtime_producer_records_integer_steps_and_terminal_traps
//! pins that current behavior without changing the original positive trap test.
//!
//! Authenticated execution binds the independently admitted receipt payer before
//! capturing boundaries. The producer proof requires successful replay of every
//! genuine adjacent pair against the receipt-derived authority.
//! The separate sandbox producer/consumer check proves actual instruction replay.

use layerx_programs_arbiter::{
    ics23::{decode_existence, verify_existence, ExistenceProof, InnerOp, LeafOp, ProofError},
    market::{MarketAdmissionEvidence, MarketCommittedNamespaces},
    AdmissionEvidence, AuthenticatedCatalogue, BoundaryProof, ChainCommitProof,
    MarketEvidenceError, MarketReplayEvidence, MarketStepVerdict, NativeNamespaceProof,
    ReplayError, VerifiedMarketSandbox, VerifiedReplay, VerifiedReplayAuthority,
};
use layerx_programs_runtime::{
    execute::{
        instantiate_market_sandbox_untrusted, market_sandbox_input_digest,
        observe_market_sandbox_step, replay_portable_step, ExecutionFault, MarketSandboxRequest,
        PortableReplayInputs, RuntimeBoundaryReplayError,
    },
    portable_replay::{decode_record, PortableBoundary, PortableReplayRecord},
    replay::{
        market_sandbox_baseline_root, market_sandbox_namespace, MarketSandboxReplayAuthority,
    },
    test_support::{
        code_section, export_section, func_body, function_section, module, raw_section,
        type_section, OP_END, OP_I32_CONST, TYPE_I32,
    },
    AbiError, ActivityBudgetBinding, AuthorizationContext, AuthorizedExecutionRequest,
    BudgetAdmissionRefusal, BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, CompositionRules, ContextRefusal, DeclaredBudget, ExecutionError, Executor,
    FeeSchedule, PrincipalId, ProgramCatalog, ProgramId, ProgramReplayProfile, ProgramResolver,
    ReceiptOracle, ResourceBudget, Storage, StorageNamespace, UnavailableReceiptOracle,
    V2ActivityOutcome, WasmEngine, WasmValue, RUNTIME_VERSION,
};
use layerx_proof::{
    inclusion::{verify_header, verify_receipt, VerifiedBatchHeader},
    merkle::build_proof,
    receipt::{verify_program_outcome, AuthorizedBatch, VerifiedReceipt},
    state_witness::{AccountPath, StateWitness},
};
use layerx_types::{
    activity::{Authority, EnvelopeBuilder, Signature, TimestampBound},
    amount::Amount,
    ids::{Did, IdempotencyKey},
    payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload},
};
use layerx_wire::{activity, encode::Encoder, hash, receipt};
use sha2::{Digest, Sha256};
use std::{collections::BTreeMap, ffi::c_void, rc::Rc};

const MAX: usize = 1_048_576;
const DID: &[u8] = b"did:lxp:arbiter-crate-proof";
const SEQUENCE: u64 = 7;
const BATCH: u64 = 10;
const TIME: u64 = 50;
const SIGNER: [u8; 32] = [0xa1; 32];
const ASSET: [u8; 32] = [0x31; 32];
type Records = BTreeMap<Vec<u8>, Vec<u8>>;

fn digest(parts: &[&[u8]]) -> [u8; 32] {
    let mut h = Sha256::new();
    for part in parts {
        h.update(part);
    }
    h.finalize().into()
}
fn vector(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(&u32::try_from(value.len()).unwrap().to_be_bytes());
    out.extend_from_slice(value);
}
fn encoder() -> Encoder {
    Encoder::new(64 * MAX)
}

fn unavailable_balance_transcript(capabilities: &CapabilitySet) -> Vec<u8> {
    let mut hosts = encoder();
    hosts.fixed(b"LXP/program-replay-hosts/v1\0").unwrap();
    let grants = CapabilitySet::decode_v2_canonical(&capabilities.canonical_encoding()).unwrap();
    for grant in grants {
        if let Capability::BalanceView {
            account,
            asset,
            receipt_digest,
        } = grant
        {
            // Transport the same production oracle refusal captured by Abi::new.
            // Omitting it would make replay lose a declared balance capability.
            assert_eq!(
                UnavailableReceiptOracle.verified_balance(account, asset, receipt_digest),
                Err(AbiError::BalanceEvidenceUnavailable),
            );
            let mut record = encoder();
            record.fixed(&account).unwrap();
            record.fixed(&asset).unwrap();
            record.fixed(&receipt_digest).unwrap();
            hosts.u8(2).unwrap();
            hosts.i32(-1).unwrap();
            hosts.bytes(&record.finish(), MAX).unwrap();
        }
    }
    hosts.finish()
}

fn encoded_vector(e: &mut Encoder, bytes: &[u8]) {
    e.bytes(bytes, 64 * MAX).unwrap();
}

// OpenSSL's real in-process Ed25519 producer. No private key or service is
// provisioned by the operator, and all three signature domains use core hashes.
#[link(name = "crypto")]
unsafe extern "C" {
    fn EVP_PKEY_new_raw_private_key(
        kind: i32,
        engine: *mut c_void,
        key: *const u8,
        len: usize,
    ) -> *mut c_void;
    fn EVP_PKEY_get_raw_public_key(key: *const c_void, out: *mut u8, len: *mut usize) -> i32;
    fn EVP_PKEY_free(key: *mut c_void);
    fn EVP_MD_CTX_new() -> *mut c_void;
    fn EVP_MD_CTX_free(ctx: *mut c_void);
    fn EVP_DigestSignInit(
        ctx: *mut c_void,
        pctx: *mut *mut c_void,
        md: *const c_void,
        engine: *mut c_void,
        key: *mut c_void,
    ) -> i32;
    fn EVP_DigestSign(
        ctx: *mut c_void,
        sig: *mut u8,
        len: *mut usize,
        msg: *const u8,
        msg_len: usize,
    ) -> i32;
}
struct SigningKey(*mut c_void);
impl SigningKey {
    fn new(seed: [u8; 32]) -> Self {
        let key = unsafe {
            EVP_PKEY_new_raw_private_key(1087, std::ptr::null_mut(), seed.as_ptr(), seed.len())
        };
        assert!(!key.is_null(), "in-process OpenSSL Ed25519 producer");
        Self(key)
    }
    fn public(&self) -> [u8; 32] {
        let mut out = [0; 32];
        let mut len = out.len();
        assert_eq!(
            unsafe { EVP_PKEY_get_raw_public_key(self.0, out.as_mut_ptr(), &mut len) },
            1
        );
        assert_eq!(len, out.len());
        out
    }
    fn sign(&self, message: &[u8; 32]) -> [u8; 64] {
        let ctx = unsafe { EVP_MD_CTX_new() };
        assert!(!ctx.is_null());
        assert_eq!(
            unsafe {
                EVP_DigestSignInit(
                    ctx,
                    std::ptr::null_mut(),
                    std::ptr::null(),
                    std::ptr::null_mut(),
                    self.0,
                )
            },
            1
        );
        let mut out = [0; 64];
        let mut len = out.len();
        let result = unsafe {
            EVP_DigestSign(
                ctx,
                out.as_mut_ptr(),
                &mut len,
                message.as_ptr(),
                message.len(),
            )
        };
        unsafe { EVP_MD_CTX_free(ctx) };
        assert_eq!(result, 1);
        assert_eq!(len, out.len());
        out
    }
}
impl Drop for SigningKey {
    fn drop(&mut self) {
        unsafe { EVP_PKEY_free(self.0) };
    }
}

fn varint(mut n: u64) -> Vec<u8> {
    let mut bytes = Vec::new();
    while n >= 128 {
        bytes.push((n as u8 & 127) | 128);
        n >>= 7;
    }
    bytes.push(n as u8);
    bytes
}
fn proto_bytes(out: &mut Vec<u8>, field: u8, value: &[u8]) {
    out.extend(varint(u64::from(field) * 8 + 2));
    out.extend(varint(value.len() as u64));
    out.extend(value);
}
fn leaf_op(prefix: &[u8]) -> Vec<u8> {
    let mut out = vec![8, 1, 24, 1, 32, 1];
    proto_bytes(&mut out, 5, prefix);
    out
}
fn encode_proof(proof: &ExistenceProof) -> Vec<u8> {
    let mut out = Vec::new();
    proto_bytes(&mut out, 1, &proof.key);
    proto_bytes(&mut out, 2, &proof.value);
    proto_bytes(&mut out, 3, &leaf_op(&proof.leaf.prefix));
    for op in &proof.path {
        let mut inner = vec![8, 1];
        proto_bytes(&mut inner, 2, &op.prefix);
        if !op.suffix.is_empty() {
            proto_bytes(&mut inner, 3, &op.suffix);
        }
        proto_bytes(&mut out, 4, &inner);
    }
    out
}
fn commitment_leaf(prefix: &[u8], key: &[u8], value: &[u8]) -> [u8; 32] {
    digest(&[
        prefix,
        &varint(key.len() as u64),
        key,
        &[32],
        &digest(&[value]),
    ])
}

// The harness commits a balanced, versioned IAVL tree of its actual store.
// Proofs are produced from the tree, never assigned a caller-chosen root.
struct CommittedStore {
    root: [u8; 32],
    proofs: BTreeMap<Vec<u8>, ExistenceProof>,
}
impl CommittedStore {
    fn commit(records: Records) -> Self {
        assert!(!records.is_empty());
        fn tree(entries: &[(Vec<u8>, Vec<u8>)]) -> (u64, [u8; 32], Vec<ExistenceProof>) {
            if entries.len() == 1 {
                let (key, value) = &entries[0];
                let prefix = vec![0, 2, 2];
                return (
                    0,
                    commitment_leaf(&prefix, key, value),
                    vec![ExistenceProof {
                        key: key.clone(),
                        value: value.clone(),
                        leaf: LeafOp {
                            hash: 1,
                            prehash_key: 0,
                            prehash_value: 1,
                            length: 1,
                            prefix,
                        },
                        path: vec![],
                    }],
                );
            }
            let mid = entries.len() / 2;
            let (lh, left, mut lp) = tree(&entries[..mid]);
            let (rh, right, mut rp) = tree(&entries[mid..]);
            let height = lh.max(rh) + 1;
            let mut header = varint(height * 2);
            header.extend(varint(entries.len() as u64 * 2));
            header.push(2);
            let mut prefix = header.clone();
            prefix.push(32);
            let mut suffix = vec![32];
            suffix.extend(right);
            for proof in &mut lp {
                proof.path.push(InnerOp {
                    hash: 1,
                    prefix: prefix.clone(),
                    suffix: suffix.clone(),
                });
            }
            let root = digest(&[&prefix, &left, &suffix]);
            let mut prefix = header;
            prefix.push(32);
            prefix.extend(left);
            prefix.push(32);
            for proof in &mut rp {
                proof.path.push(InnerOp {
                    hash: 1,
                    prefix: prefix.clone(),
                    suffix: vec![],
                });
            }
            lp.extend(rp);
            (height, root, lp)
        }
        let (_, root, proofs) = tree(&records.into_iter().collect::<Vec<_>>());
        Self {
            root,
            proofs: proofs.into_iter().map(|p| (p.key.clone(), p)).collect(),
        }
    }
    fn transported(&self, key: &[u8]) -> ExistenceProof {
        let bytes = encode_proof(self.proofs.get(key).expect("committed producer key"));
        let decoded = decode_existence(&bytes).expect("consumer decodes producer protobuf");
        verify_existence(&decoded, self.root, &decoded.key, &decoded.value).unwrap();
        let mut malformed = bytes;
        malformed.push(0);
        assert_eq!(decode_existence(&malformed), Err(ProofError::Malformed));
        decoded
    }
}
fn multistore(host: [u8; 32], sequencer: [u8; 32]) -> ([u8; 32], Vec<u8>, Vec<u8>) {
    let left = commitment_leaf(&[0], b"lxphost", &host);
    let right = commitment_leaf(&[0], b"sequencer", &sequencer);
    let app = digest(&[&[1], &left, &right]);
    let make = |name: &[u8], root: [u8; 32], prefix: Vec<u8>, suffix: &[u8]| {
        let mut existence = Vec::new();
        proto_bytes(&mut existence, 1, name);
        proto_bytes(&mut existence, 2, &root);
        proto_bytes(&mut existence, 3, &leaf_op(&[0]));
        let mut inner = vec![8, 1];
        proto_bytes(&mut inner, 2, &prefix);
        if !suffix.is_empty() {
            proto_bytes(&mut inner, 3, suffix);
        }
        proto_bytes(&mut existence, 4, &inner);
        let mut commitment = Vec::new();
        proto_bytes(&mut commitment, 1, &existence);
        commitment
    };
    let mut prefix = vec![1];
    prefix.extend(left);
    (
        app,
        make(b"lxphost", host, vec![1], &right),
        make(b"sequencer", sequencer, prefix, &[]),
    )
}
fn state_leaf(key: &[u8], value: &[u8]) -> [u8; 32] {
    digest(&[
        b"LXP/v1/state-leaf\0",
        &(key.len() as u32).to_be_bytes(),
        &(value.len() as u32).to_be_bytes(),
        key,
        value,
    ])
}
fn state_tree(mut hashes: Vec<[u8; 32]>, mut position: usize) -> ([u8; 32], Vec<[u8; 32]>) {
    if hashes.is_empty() {
        return (digest(&[b"LXP/v1/state-leaf\0"]), vec![]);
    }
    let mut siblings = Vec::new();
    while hashes.len() > 1 {
        siblings.push(hashes[(position ^ 1).min(hashes.len() - 1)]);
        hashes = hashes
            .chunks(2)
            .map(|p| digest(&[b"LXP/v1/state-node\0", &p[0], p.get(1).unwrap_or(&p[0])]))
            .collect();
        position /= 2;
    }
    (hashes[0], siblings)
}
struct ApplicationState {
    modules: Vec<Records>,
    account_id: [u8; 32],
    account: Vec<u8>,
}
impl ApplicationState {
    fn roots(&self) -> Vec<[u8; 32]> {
        self.modules
            .iter()
            .map(|r| state_tree(r.iter().map(|(k, v)| state_leaf(k, v)).collect(), 0).0)
            .collect()
    }
    fn root(&self) -> [u8; 32] {
        state_tree(
            self.roots()
                .iter()
                .enumerate()
                .map(|(i, r)| state_leaf(&(i as u16).to_be_bytes(), r))
                .collect(),
            0,
        )
        .0
    }
    fn witnesses(&self, module: u16) -> Vec<StateWitness> {
        let records = &self.modules[module as usize];
        let hashes: Vec<_> = records.iter().map(|(k, v)| state_leaf(k, v)).collect();
        let composite: Vec<_> = self
            .roots()
            .iter()
            .enumerate()
            .map(|(i, r)| state_leaf(&(i as u16).to_be_bytes(), r))
            .collect();
        records
            .iter()
            .enumerate()
            .map(|(i, (key, value))| StateWitness {
                module_id: module,
                key: key.clone(),
                value: value.clone(),
                account_path: None,
                leaf_index_a: i as u32,
                leaf_count_a: records.len() as u32,
                siblings_a: state_tree(hashes.clone(), i).1,
                leaf_count_b: composite.len() as u32,
                siblings_b: state_tree(composite.clone(), module as usize).1,
            })
            .collect()
    }
    fn write_module(&self, e: &mut Encoder, module: u16) {
        let roots = self.roots();
        let witnesses = self.witnesses(module);
        let composite = roots
            .iter()
            .enumerate()
            .map(|(i, r)| state_leaf(&(i as u16).to_be_bytes(), r))
            .collect();
        let path = state_tree(composite, module as usize).1;
        e.u16(module).unwrap();
        e.fixed(&roots[module as usize]).unwrap();
        e.u32(module as u32).unwrap();
        e.u32(roots.len() as u32).unwrap();
        e.u8(path.len() as u8).unwrap();
        for hash in path {
            e.fixed(&hash).unwrap();
        }
        e.u32(witnesses.len() as u32).unwrap();
        for witness in witnesses {
            encoded_vector(e, &witness.encode().unwrap());
        }
    }
    fn admission(&self, activity_id: [u8; 32], signed: &[u8], receipt_digest: [u8; 32]) -> Vec<u8> {
        let mut v1 = encoder();
        v1.u16(1).unwrap();
        v1.u32(7).unwrap();
        v1.fixed(&activity_id).unwrap();
        v1.u64(SEQUENCE).unwrap();
        v1.fixed(&self.root()).unwrap();
        let roots = self.roots();
        v1.u16(roots.len() as u16).unwrap();
        for root in roots {
            v1.fixed(&root).unwrap();
        }
        self.write_module(&mut v1, 0);
        self.write_module(&mut v1, 9);
        let mut account = self
            .witnesses(0)
            .into_iter()
            .find(|w| w.key == b"account-tree")
            .unwrap();
        account.key = [vec![4], self.account_id.to_vec()].concat();
        account.value = self.account.clone();
        account.account_path = Some(AccountPath {
            index: 0,
            count: 1,
            siblings: vec![],
        });
        account.verify(self.root()).unwrap();
        v1.u32(1).unwrap();
        encoded_vector(&mut v1, &account.encode().unwrap());
        let mut v2 = encoder();
        v2.u16(2).unwrap();
        encoded_vector(&mut v2, &v1.finish());
        v2.u16(2).unwrap();
        self.write_module(&mut v2, 6);
        self.write_module(&mut v2, 11);
        let mut v3 = encoder();
        v3.u16(3).unwrap();
        encoded_vector(&mut v3, &v2.finish());
        encoded_vector(&mut v3, signed);
        v3.u16(3).unwrap();
        for module in [1, 3, 7] {
            self.write_module(&mut v3, module);
        }
        let mut out = b"LXP/arbiter-admission/v1\0".to_vec();
        out.extend(receipt_digest);
        vector(&mut out, &v3.finish());
        out
    }
}

fn guest() -> Vec<u8> {
    let mut exports = export_section(&[("layerx_reserve", 0), ("layerx_call", 1)]);
    exports[1] += 9;
    exports[2] += 1;
    exports.extend_from_slice(&[6, b'm', b'e', b'm', b'o', b'r', b'y', 2, 0]);
    module(&[
        type_section(&[
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32, TYPE_I32], &[TYPE_I32]),
        ]),
        function_section(&[0, 1]),
        raw_section(5, &[1, 1, 0, 0]),
        exports,
        code_section(&[
            func_body(&[], &[OP_I32_CONST, 0, OP_END]),
            func_body(&[], &[OP_I32_CONST, 0, OP_END]),
        ]),
    ])
}
fn signed_activity(
    program: [u8; 32],
    owner: &SigningKey,
    profile: bool,
    abi: u16,
    capabilities: &CapabilitySet,
) -> (Vec<u8>, [u8; 32]) {
    let entry = b"layerx_call";
    let caps = capabilities.canonical_encoding();
    let mut call = program.to_vec();
    call.extend(abi.to_be_bytes());
    call.extend((entry.len() as u16).to_be_bytes());
    call.extend(0u32.to_be_bytes());
    call.extend((caps.len() as u16).to_be_bytes());
    call.extend(0u32.to_be_bytes());
    call.extend(0u32.to_be_bytes());
    for n in [
        1_000_000u64,
        16_777_216,
        1_048_576,
        1_048_576,
        64,
        1_048_576,
        4096,
    ] {
        call.extend(n.to_be_bytes());
    }
    call.extend(entry);
    call.extend(caps);
    let payload = if profile {
        let mut p = vec![0; 34];
        p.extend(b"LXP/program-replay-profile/v1\0");
        p.extend(1u16.to_be_bytes());
        p.extend(128u32.to_be_bytes());
        p.extend((MAX as u32).to_be_bytes());
        vector(&mut p, &call);
        p
    } else {
        call
    };
    let activity_type = ActivityType::new(ModuleId::Programs, 3).unwrap();
    let registry =
        ModuleRegistry::new(&[
            ModuleRegistration::new(ModuleId::Programs, &[activity_type]).unwrap(),
        ])
        .unwrap();
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(3)
        .unwrap()
        .network_id(7)
        .unwrap()
        .activity_type(activity_type)
        .unwrap()
        .actor_did(Did::new(DID).unwrap())
        .unwrap()
        .authority(Authority::owner(&owner.public()).unwrap())
        .unwrap()
        .account_sequence(0)
        .unwrap()
        .timestamp_bound(TimestampBound::new(1, 100).unwrap())
        .unwrap()
        .idempotency_key(IdempotencyKey::new([0x42; 32]))
        .unwrap()
        .fee_limit(Amount::from_u128(1_000_000))
        .unwrap()
        .payload_hash(digest(&[hash::Domain::PayloadHash.tag(), &payload]))
        .unwrap()
        .payload(Payload::new(&registry, activity_type, &payload).unwrap())
        .unwrap();
    let unsigned = builder.build().unwrap();
    let signature = owner.sign(
        layerx_wire::sign::preimage_unsigned(&unsigned)
            .unwrap()
            .as_bytes(),
    );
    let bytes = activity::encode_signed_envelope(
        &unsigned.attach_signature(Signature::new(&signature).unwrap()),
    )
    .unwrap();
    let decoded = activity::decode_signed(&bytes, &registry).unwrap();
    let id = hash::activity_id(&decoded).unwrap();
    (bytes, id)
}
fn application_state(
    code_hash: [u8; 32],
    principal: [u8; 32],
    owner: [u8; 32],
    abi: u16,
) -> ApplicationState {
    let mut modules = vec![Records::new(); 12];
    let mut name = b"agent:".to_vec();
    name.extend(DID);
    name.extend(b":main");
    let account_id = digest(&[b"LX:ACCOUNT:v1", &(name.len() as u32).to_be_bytes(), &name]);
    let mut a = encoder();
    a.u16(name.len() as u16).unwrap();
    a.fixed(&name).unwrap();
    a.u8(1).unwrap();
    a.u128(1_000_000_000).unwrap();
    a.fixed(&ASSET).unwrap();
    a.u8(1).unwrap();
    a.u64(0).unwrap();
    a.u64(1).unwrap();
    a.u8(0).unwrap();
    a.u8(0).unwrap();
    a.fixed(&owner).unwrap();
    a.u8(1).unwrap();
    let account = a.finish();
    layerx_proof::state::decode_account_value(account_id, &account).unwrap();
    let mut account_key = vec![4];
    account_key.extend(account_id);
    modules[0].insert(
        b"account-tree".to_vec(),
        state_leaf(&account_key, &account).to_vec(),
    );
    modules[0].insert(b"sequence".to_vec(), SEQUENCE.to_be_bytes().to_vec());
    let mut registration = vec![1, 1];
    registration.extend(0u64.to_be_bytes());
    registration.extend(u64::MAX.to_be_bytes());
    registration.extend(0x0009_0003u32.to_be_bytes());
    modules[0].insert(vec![3, 0, 0, 0, 9, 0, 3], registration);
    let mut identity = vec![0; 223];
    identity[..5].copy_from_slice(b"LXGI1");
    identity[5..37].copy_from_slice(&principal);
    identity[37..69].copy_from_slice(&owner);
    modules[7].insert(principal.to_vec(), identity);
    let mut policy_key = [0; 32];
    let policy_name = b"native-fee-authority-version";
    policy_key[..policy_name.len()].copy_from_slice(policy_name);
    let mut policy_value = vec![0; 32];
    policy_value[31] = 2;
    modules[7].insert(policy_key.to_vec(), policy_value);
    let mut fee = vec![0; 217];
    fee[..5].copy_from_slice(b"LXFR1");
    fee[5..9].copy_from_slice(&1u32.to_be_bytes());
    let fees = FeeSchedule::declared();
    for (i, p) in fee_prices(fees).iter().enumerate() {
        fee[9 + i * 8..17 + i * 8].copy_from_slice(&p.to_be_bytes());
    }
    fee[65..97].copy_from_slice(&ASSET);
    fee[145..153].copy_from_slice(&1u64.to_be_bytes());
    modules[9].insert(
        [b"progfee/history/v1/".as_slice(), &1u32.to_be_bytes()].concat(),
        fee,
    );
    let mut meter = vec![0; 122];
    meter[..5].copy_from_slice(b"LXMR1");
    meter[5..9].copy_from_slice(&1u32.to_be_bytes());
    for (i, p) in [1u64, 1, 1, 1, 1, 8, 8, 64, 8].iter().enumerate() {
        meter[9 + i * 8..17 + i * 8].copy_from_slice(&p.to_be_bytes());
    }
    meter[81..89].copy_from_slice(&1u64.to_be_bytes());
    meter[89] = 1;
    meter[90..].copy_from_slice(&digest(&[b"genesis metering"]));
    modules[9].insert(
        [b"progmet/history/v1/".as_slice(), &1u32.to_be_bytes()].concat(),
        meter,
    );
    let mut program = vec![0; 33];
    program.extend(code_hash);
    program.extend(abi.to_be_bytes());
    program.extend(1u32.to_be_bytes());
    modules[9].insert([b"program\0".as_slice(), &code_hash].concat(), program);
    ApplicationState {
        modules,
        account_id,
        account,
    }
}
fn fee_prices(f: FeeSchedule) -> [u64; 7] {
    [
        f.cpu_price(),
        f.memory_byte_price(),
        f.storage_read_byte_price(),
        f.storage_write_byte_price(),
        f.output_value_price(),
        f.output_byte_price(),
        f.occupancy_byte_batch_price(),
    ]
}

fn produce_receipt(
    execution: &layerx_programs_runtime::V2AuthorizedExecutionRecord,
    activity_id: [u8; 32],
    previous: [u8; 32],
    signer: &SigningKey,
    abi: u16,
) -> Vec<u8> {
    let usage = execution.execution().usage();
    let mut e = encoder();
    e.structure_header_version(0x5201, 3).unwrap();
    e.u16(3).unwrap();
    encoded_vector(&mut e, &activity_id);
    e.u64(SEQUENCE).unwrap();
    encoded_vector(&mut e, &previous);
    encoded_vector(&mut e, &previous);
    encoded_vector(
        &mut e,
        &digest(&[b"application activity root", &activity_id]),
    );
    e.i32(0).unwrap();
    e.u32(0).unwrap();
    e.u128(usage.fee_units).unwrap();
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&BATCH.to_be_bytes());
    encoded_vector(&mut e, &batch_id);
    e.u16(9).unwrap();
    e.u32(4).unwrap();
    e.u32(1).unwrap();
    e.u8(3).unwrap();
    encoded_vector(&mut e, &[0; 32]);
    e.u128(0).unwrap();
    encoded_vector(&mut e, &[0; 32]);
    e.u128(0).unwrap();
    e.u128(0).unwrap();
    e.u64(0).unwrap();
    encoded_vector(&mut e, &[0; 32]);
    e.u128(0).unwrap();
    e.u128(0).unwrap();
    encoded_vector(&mut e, &[0; 32]);
    encoded_vector(&mut e, &[0; 32]);
    encoded_vector(&mut e, &SIGNER);
    e.u64(TIME).unwrap();
    e.u32(0x50524733).unwrap();
    e.u8(1).unwrap();
    e.i32(0).unwrap();
    e.u16(RUNTIME_VERSION).unwrap();
    e.u16(abi).unwrap();
    e.u32(1).unwrap();
    e.u32(1).unwrap();
    for n in [
        usage.cpu_fuel,
        usage.memory_bytes,
        usage.storage_read_bytes,
        usage.storage_write_bytes,
    ] {
        e.u64(n).unwrap();
    }
    e.u32(usage.output_values).unwrap();
    e.u64(usage.output_bytes).unwrap();
    e.u128(0).unwrap();
    e.u128(0).unwrap();
    for price in fee_prices(FeeSchedule::declared()) {
        e.u64(price).unwrap();
    }
    encoded_vector(&mut e, &ASSET);
    encoded_vector(&mut e, &digest(&[b"zero occupancy", &activity_id]));
    encoded_vector(&mut e, &[0; 32]);
    e.u128(usage.fee_units).unwrap();
    encoded_vector(
        &mut e,
        &digest(&[&execution.call_graph().canonical_evidence()]),
    );
    encoded_vector(
        &mut e,
        &digest(&[&execution.receipt_projection().canonical_encode()]),
    );
    encoded_vector(&mut e, &[0; 32]);
    e.u8(0).unwrap();
    let unsigned = e.finish();
    let parsed = receipt::decode(&unsigned).expect("application receipt producer canonical output");
    assert_eq!(receipt::encode_unsigned(&parsed).unwrap(), unsigned);
    let signature = signer.sign(&hash::receipt_digest(&unsigned).unwrap());
    let mut signed = unsigned;
    *signed.last_mut().unwrap() = 1;
    vector(&mut signed, &signature);
    assert_eq!(
        receipt::encode(&receipt::decode(&signed).unwrap()).unwrap(),
        signed
    );
    signed
}
fn produce_header(
    receipt_bytes: &[u8],
    signed_activity: &[u8],
    root: [u8; 32],
    signer: &SigningKey,
) -> (Vec<u8>, [u8; 64]) {
    let mut e = encoder();
    e.structure_header_version(0x1701, 3).unwrap();
    e.u8(15).unwrap();
    for field in 1..=15 {
        e.u8(field).unwrap();
        match field {
            1 => e.u16(3).unwrap(),
            2 => e.u32(7).unwrap(),
            3 => e.u64(1).unwrap(),
            4 => e.u64(BATCH).unwrap(),
            5 => e.u64(SEQUENCE).unwrap(),
            6 => e.u64(SEQUENCE).unwrap(),
            7 | 8 => encoded_vector(&mut e, &root),
            9 => encoded_vector(&mut e, &build_proof(&[signed_activity], 0).unwrap().1),
            10 => encoded_vector(&mut e, &build_proof(&[receipt_bytes], 0).unwrap().1),
            11..=13 => encoded_vector(&mut e, &digest(&[&[field]])),
            14 => e.u64(TIME).unwrap(),
            15 => encoded_vector(&mut e, &SIGNER),
            _ => unreachable!(),
        }
    }
    let bytes = e.finish();
    let canonical = receipt::decode_batch_header(&bytes).unwrap();
    assert_eq!(receipt::encode_batch_header(&canonical).unwrap(), bytes);
    let signature = signer.sign(&hash::batch_header_digest(&bytes).unwrap());
    (bytes, signature)
}
fn native_metadata(
    record: &layerx_programs_runtime::ProgramReplayRecord,
    id: [u8; 32],
    root: [u8; 32],
    authority: &[u8],
    hosts: &[u8],
    abi: u16,
) -> Vec<u8> {
    let mut e = encoder();
    e.fixed(b"LXP/program-replay-native/v1\0").unwrap();
    e.u16(1).unwrap();
    e.u32(7).unwrap();
    e.fixed(&id).unwrap();
    e.u64(SEQUENCE).unwrap();
    e.fixed(&root).unwrap();
    e.fixed(&record.code_hash()).unwrap();
    e.fixed(&record.code_hash()).unwrap();
    e.fixed(&record.input_digest()).unwrap();
    e.u16(RUNTIME_VERSION).unwrap();
    e.u16(abi).unwrap();
    e.u32(1).unwrap();
    e.u32(1).unwrap();
    e.u16(1).unwrap();
    e.u32(128).unwrap();
    e.u32(MAX as u32).unwrap();
    e.u8(record.terminal_status()).unwrap();
    e.u32(record.boundary_count()).unwrap();
    e.fixed(&digest(&[authority])).unwrap();
    e.fixed(&digest(&[hosts])).unwrap();
    e.fixed(&record.boundary_root()).unwrap();
    e.fixed(&record.witness_digest()).unwrap();
    e.i32(0).unwrap();
    let mut blob = b"LXP/program-replay-native-blob/v1\0".to_vec();
    vector(&mut blob, record.canonical_bytes());
    vector(&mut blob, authority);
    vector(&mut blob, hosts);
    e.fixed(&digest(&[&blob])).unwrap();
    let bytes = e.finish();
    assert_eq!(bytes.len(), 394);
    bytes
}
struct Corpus {
    activity_id: [u8; 32],
    program: [u8; 32],
    principal: [u8; 32],
    receipt: Vec<u8>,
    header: Vec<u8>,
    signature: [u8; 64],
    host: CommittedStore,
    sequencer: CommittedStore,
    authority: Vec<u8>,
    hosts: Vec<u8>,
    record: layerx_programs_runtime::ProgramReplayRecord,
    manifest: Vec<u8>,
    values: BTreeMap<[u8; 32], Vec<u8>>,
    namespace_cells: Records,
    code: BTreeMap<[u8; 32], Vec<u8>>,
}
impl Corpus {
    fn produce(ordinary: bool) -> Self {
        Self::produce_versioned(ordinary, 2, 2, CapabilitySet::empty())
            .expect("authenticated producer authority")
    }
    fn produce_versioned(
        ordinary: bool,
        execution_abi: u16,
        call_abi: u16,
        capabilities: CapabilitySet,
    ) -> Result<Self, ReplayError> {
        let signer = SigningKey::new([0x11; 32]);
        let owner = SigningKey::new([0x22; 32]);
        // Empty calldata and this integer-only guest use no linear memory.
        // Avoid metering snapshots of an unused page for either ABI.
        let wasm = guest();
        let program = digest(&[&wasm]);
        let principal = digest(&[b"LXP/v1/did-id\0", &(DID.len() as u16).to_be_bytes(), DID]);
        let state = application_state(program, principal, owner.public(), execution_abi);
        let (signed, activity_id) =
            signed_activity(program, &owner, !ordinary, call_abi, &capabilities);
        let hosts = unavailable_balance_transcript(&capabilities);
        let validated = WasmEngine::declared()
            .unwrap()
            .validate_versioned(execution_abi, &wasm)
            .unwrap();
        let executor = Executor::new(ResourceBudget::declared(), FeeSchedule::declared())
            .with_program_replay_profile(ProgramReplayProfile::new(128, MAX as u32).unwrap());
        let request = AuthorizedExecutionRequest {
            module: &validated,
            program: ProgramId::new(program).unwrap(),
            authorization: AuthorizationContext::new(
                PrincipalId::new(principal).unwrap(),
                capabilities,
            ),
            receipts: &UnavailableReceiptOracle,
            entrypoint: "layerx_call",
            calldata: &[],
            composition: CompositionContext::new(
                Rc::new(ProgramCatalog::new()),
                CompositionRules::declared(),
            ),
            response_capacity: 0,
        };
        let payer = PrincipalId::new(state.account_id).unwrap();
        let binding = ActivityBudgetBinding::new(activity_id).unwrap();
        let token = executor
            .admit_activity_budget_for_qualification(
                DeclaredBudget::protocol_maximum(),
                payer,
                binding,
                u128::MAX,
            )
            .unwrap();
        let execution = executor.execute_authorized_authenticated(
            &mut Storage::new(),
            BudgetedAuthorizedExecutionRequest::new(request, token, payer, binding),
            SEQUENCE,
            BATCH,
        )
        .expect(
            "programs/crates/layerx-programs-runtime/src/execute.rs::Executor in-process producer",
        );
        assert!(matches!(
            execution.outcome(),
            V2ActivityOutcome::Success { .. }
        ));
        assert_eq!(ResourceBudget::declared().cpu_fuel(), 1_000_000);
        assert!(execution.execution().usage().cpu_fuel <= ResourceBudget::declared().cpu_fuel());
        assert_eq!(execution.execution().usage().memory_bytes, 0);
        let record = execution
            .replay_record()
            .expect("real runtime boundary corpus")
            .clone();
        let receipt = produce_receipt(
            &execution,
            activity_id,
            state.root(),
            &signer,
            execution_abi,
        );
        let receipt_digest = hash::receipt_digest(
            &receipt::encode_unsigned(&receipt::decode(&receipt).unwrap()).unwrap(),
        )
        .unwrap();
        let (header, signature) = produce_header(&receipt, &signed, state.root(), &signer);
        let mut records = Records::new();
        let admission = state.admission(activity_id, &signed, receipt_digest);
        records.insert(
            [
                vec![1, 0, 9],
                b"admission/v1/".to_vec(),
                activity_id.to_vec(),
            ]
            .concat(),
            admission,
        );
        for module in [7usize, 9] {
            for (key, value) in &state.modules[module] {
                records.insert(
                    [vec![1, 0, module as u8], key.clone()].concat(),
                    value.clone(),
                );
            }
        }
        records.insert(
            [vec![6], state.account_id.to_vec()].concat(),
            state.account.clone(),
        );
        records.insert([vec![5, 0, 9], program.to_vec()].concat(), wasm.clone());
        let mut key_table = SIGNER.to_vec();
        key_table.extend(signer.public());
        key_table.extend(1u64.to_be_bytes());
        key_table.extend(100u64.to_be_bytes());
        let sequencer = CommittedStore::commit(BTreeMap::from([(vec![3], key_table)]));
        // Produce namespace cells through the real storage transaction API.
        let mut storage = Storage::new();
        let namespace = StorageNamespace::shared(ProgramId::new(program).unwrap());
        let mut tx = storage.transaction(namespace);
        tx.write(
            b"execution",
            &execution.receipt_projection().canonical_encode(),
        )
        .unwrap();
        assert_eq!(tx.commit(), 1);
        let cells: Records = storage
            .protocol_namespace_entries(namespace)
            .unwrap()
            .into_iter()
            .collect();
        let mut m = encoder();
        m.u16(1).unwrap();
        m.u32(cells.len() as u32).unwrap();
        let mut values = BTreeMap::new();
        for (key, value) in &cells {
            m.u16(key.len() as u16).unwrap();
            m.fixed(key).unwrap();
            m.fixed(&digest(&[value])).unwrap();
            m.u32(value.len() as u32).unwrap();
            values.insert(digest(&[value]), value.clone());
        }
        let manifest = m.finish();
        let mut head = 1u16.to_be_bytes().to_vec();
        head.extend((cells.len() as u32).to_be_bytes());
        head.extend(digest(&[&manifest]));
        records.insert(
            [
                vec![1, 0, 9],
                b"progstor".to_vec(),
                namespace.canonical_bytes(),
            ]
            .concat(),
            head,
        );
        let mut corpus = Self {
            activity_id,
            program,
            principal,
            receipt,
            header,
            signature,
            host: CommittedStore::commit(records.clone()),
            sequencer,
            authority: vec![],
            hosts,
            record,
            manifest,
            values,
            namespace_cells: cells,
            code: BTreeMap::from([(program, wasm)]),
        };
        let (receipt, admission, header, commit, _) = corpus.consume();
        let authority = if ordinary {
            VerifiedReplayAuthority::derive_program_call(&receipt, &admission, &header, &commit)
        } else {
            VerifiedReplayAuthority::derive(&receipt, &admission, &header, &commit)
        }?;
        corpus.authority = authority.canonical_bytes().to_vec();
        records.insert(
            [
                vec![1, 0, 9],
                b"progreplay/v1/".to_vec(),
                activity_id.to_vec(),
            ]
            .concat(),
            native_metadata(
                &corpus.record,
                activity_id,
                state.root(),
                &corpus.authority,
                &corpus.hosts,
                execution_abi,
            ),
        );
        corpus.host = CommittedStore::commit(records);
        Ok(corpus)
    }
    fn consume(
        &self,
    ) -> (
        VerifiedReceipt,
        AdmissionEvidence,
        VerifiedBatchHeader,
        ChainCommitProof,
        Vec<u8>,
    ) {
        let (app, host_proof, seq_proof) = multistore(self.host.root, self.sequencer.root);
        let commit = ChainCommitProof {
            height: BATCH,
            app_hash: app,
            store_root: self.sequencer.root,
            multistore_proof: seq_proof,
            receipt_key: vec![3],
            receipt_proof: self.sequencer.transported(&[3]),
        };
        let keys = commit.receipt_keys().unwrap();
        assert_eq!(keys.len(), 1);
        let auth = &keys[0];
        let header = verify_header(&self.header, &self.signature, auth).unwrap();
        let path = build_proof(&[self.receipt.as_slice()], 0).unwrap().0;
        verify_receipt(&self.receipt, &path, &self.header, &self.signature, auth).unwrap();
        let decoded = receipt::decode(&self.receipt).unwrap();
        let protocol = decoded.protocol().unwrap();
        let batch = AuthorizedBatch::from_receipt_keys(
            &self.receipt,
            keys.as_slice(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
        )
        .unwrap();
        let verified = verify_program_outcome(&self.receipt, &batch).unwrap();
        commit.verify_receipt(&verified).unwrap();
        let proofs = self
            .host
            .proofs
            .keys()
            .filter(|key| {
                key.starts_with(&[6])
                    || key.starts_with(&[1, 0, 7])
                    || key.starts_with(&[1, 0, 9])
                        && !key[3..].starts_with(b"progreplay/v1/")
                        && !key[3..].starts_with(b"progstor")
            })
            .map(|key| (key.clone(), self.host.transported(key)))
            .collect();
        let admission = AdmissionEvidence::verify_under_app_hash(
            app,
            self.host.root,
            &host_proof,
            &self.activity_id,
            &proofs,
        )
        .unwrap();
        (verified, admission, header, commit, host_proof)
    }
    fn catalogue(
        &self,
        admission: &AdmissionEvidence,
        commit: &ChainCommitProof,
        host_proof: &[u8],
    ) -> AuthenticatedCatalogue {
        let code_proofs = self
            .code
            .keys()
            .map(|id| {
                (
                    *id,
                    self.host
                        .transported(&[vec![5, 0, 9], id.to_vec()].concat()),
                )
            })
            .collect();
        AuthenticatedCatalogue::verify_with_module(
            admission,
            commit,
            self.host.root,
            host_proof,
            &code_proofs,
            &self.code,
        )
        .unwrap()
    }
    fn namespace_head(&self) -> ExistenceProof {
        self.host.transported(
            &[
                vec![1, 0, 9],
                b"progstor".to_vec(),
                self.program.to_vec(),
                vec![1],
            ]
            .concat(),
        )
    }
    fn namespace<'a>(
        &'a self,
        head: &'a ExistenceProof,
        host_proof: &'a [u8],
    ) -> NativeNamespaceProof<'a> {
        NativeNamespaceProof {
            head,
            store_root: self.host.root,
            multistore_proof: host_proof,
            manifest: &self.manifest,
            value_blobs: &self.values,
        }
    }
    fn paths(&self) -> Vec<BoundaryProof> {
        let (record, leaves) = decode_record(self.record.canonical_bytes(), MAX).unwrap();
        let portable = PortableReplayRecord {
            record,
            leaves,
            authority_bytes: self.authority.clone(),
            hosts_bytes: self.hosts.clone(),
        };
        portable
            .leaves
            .iter()
            .enumerate()
            .map(|(i, leaf)| BoundaryProof {
                index: i as u32,
                leaf: leaf.clone(),
                siblings: portable.merkle_path_untrusted(i as u32).unwrap(),
            })
            .collect()
    }

    fn runtime_pair_result(
        &self,
        pre: &BoundaryProof,
        post: &BoundaryProof,
    ) -> Result<(), RuntimeBoundaryReplayError> {
        // Reconstruct the production authority and authenticated code catalogue
        // from transported proofs, just as the arbiter does. This diagnostic
        // never changes an executor leaf or substitutes for verify_step.
        let (receipt, admission, header, commit, host_proof) = self.consume();
        let authority = VerifiedReplayAuthority::derive(&receipt, &admission, &header, &commit)
            .expect("production replay authority constructor");
        authority.verify_native(&self.authority).unwrap();
        let catalogue = Rc::new(self.catalogue(&admission, &commit, &host_proof));
        let inputs = PortableReplayInputs::new(
            authority.canonical_bytes(),
            &self.hosts,
            Some(catalogue.clone()),
            MAX,
        )?;
        let before = PortableBoundary::decode_untrusted(&pre.leaf, MAX)?;
        let after = PortableBoundary::decode_untrusted(&post.leaf, MAX)?;
        replay_portable_step(
            catalogue
                .program_module(ProgramId::new(self.program).unwrap())
                .unwrap(),
            &before,
            &after,
            &inputs,
        )
    }
}

pub fn repair_v5_authority() {
    let capabilities = CapabilitySet::new([Capability::BalanceView {
        account: [0x61; 32],
        asset: ASSET,
        receipt_digest: [0x62; 32],
    }])
    .unwrap();
    // This grant requires V2 encoding, so a V1 decoder cannot satisfy this proof.
    assert!(CapabilitySet::decode_canonical(&capabilities.canonical_encoding()).is_err());
    for ordinary in [true, false] {
        let corpus = Corpus::produce_versioned(ordinary, 5, 5, capabilities.clone())
            .expect("admitted V5 call must cross authority derivation");
        let (receipt, admission, header, commit, host_proof) = corpus.consume();
        let authority = if ordinary {
            VerifiedReplayAuthority::derive_program_call(&receipt, &admission, &header, &commit)
        } else {
            VerifiedReplayAuthority::derive(&receipt, &admission, &header, &commit)
        }
        .unwrap();
        assert_eq!(authority.capabilities(), &capabilities);
        assert_eq!(authority.principal(), corpus.principal);
        assert_eq!(authority.activity_binding(), corpus.activity_id);
        authority.verify_native(&corpus.authority).unwrap();
        let payer_key = corpus.host.proofs.keys().find(|key| key[0] == 6).unwrap();
        let payer: [u8; 32] = payer_key[1..].try_into().unwrap();
        assert_eq!(authority.payer(), payer);
        assert_ne!(authority.payer(), authority.principal());
        let mut wrong_payer = corpus.authority.clone();
        let payer_offset = wrong_payer.len() - 240;
        wrong_payer[payer_offset] ^= 1;
        assert_eq!(
            authority.verify_native(&wrong_payer),
            Err(ReplayError::Authority)
        );
        if !ordinary {
            let metadata = corpus.host.transported(
                &[
                    vec![1, 0, 9],
                    b"progreplay/v1/".to_vec(),
                    corpus.activity_id.to_vec(),
                ]
                .concat(),
            );
            let replay = VerifiedReplay::verify_with_batch_context(
                &receipt,
                &admission,
                &header,
                &commit,
                corpus.host.root,
                &host_proof,
                &metadata,
                &corpus.authority,
                &corpus.hosts,
                corpus.catalogue(&admission, &commit, &host_proof),
            )
            .expect("admitted V5 authority must reach the replay consumer");
            assert_eq!(replay.metadata().abi_version, 5);
            assert_eq!(
                replay.metadata().boundary_root,
                corpus.record.boundary_root()
            );
            assert_eq!(
                replay.metadata().witness_digest,
                corpus.record.witness_digest()
            );
            let paths = corpus.paths();
            assert!(paths.len() > 2);
            for pair in paths.windows(2) {
                let step = replay.verify_step(&pair[0], &pair[1]).unwrap_or_else(|error| {
                    panic!(
                        "admitted V5 Executor boundary pair {} -> {} must replay: {error:?}; inner runtime result: {:?}",
                        pair[0].index,
                        pair[1].index,
                        corpus.runtime_pair_result(&pair[0], &pair[1]),
                    )
                });
                assert_eq!(step.activity_id(), corpus.activity_id);
                assert_eq!(step.pre_index(), pair[0].index);
                assert_eq!(step.post_index(), pair[1].index);
            }
        }
    }
}

pub fn repair_unknown_abi_refusal() {
    for ordinary in [true, false] {
        for unknown in [0, 6, u16::MAX] {
            // The genuine V2 executor remains the producer; the signed caller
            // independently requests an unsupported ABI at the authority boundary.
            assert!(matches!(
                Corpus::produce_versioned(ordinary, 2, unknown, CapabilitySet::empty()),
                Err(ReplayError::Authority)
            ));
        }
    }
}

pub fn repair_executor_payer_replay() {
    authenticated_producer_crosses_authority_and_bounded_replay_consumers_twice();
}

pub fn repair_authenticated_execution_refusals() {
    let wasm = guest();
    let program = ProgramId::new(digest(&[&wasm])).unwrap();
    let principal = PrincipalId::new([0x21; 32]).unwrap();
    let payer = PrincipalId::new([0x31; 32]).unwrap();
    let other_payer = PrincipalId::new([0x32; 32]).unwrap();
    let binding = ActivityBudgetBinding::new([0x41; 32]).unwrap();
    let other_binding = ActivityBudgetBinding::new([0x42; 32]).unwrap();
    let executor = Executor::declared()
        .with_program_replay_profile(ProgramReplayProfile::new(128, MAX as u32).unwrap());
    for abi in [2, 5] {
        let validated = WasmEngine::declared()
            .unwrap()
            .validate_versioned(abi, &wasm)
            .unwrap();
        for (carried_payer, carried_binding, sequence, batch, expected) in [
            (
                other_payer,
                binding,
                SEQUENCE,
                BATCH,
                ExecutionError::Budget(BudgetAdmissionRefusal::PayerMismatch),
            ),
            (
                payer,
                other_binding,
                SEQUENCE,
                BATCH,
                ExecutionError::Budget(BudgetAdmissionRefusal::ActivityBindingMismatch),
            ),
            (
                payer,
                binding,
                0,
                BATCH,
                ExecutionError::Context(ContextRefusal::Unauthenticated),
            ),
            (
                payer,
                binding,
                SEQUENCE,
                0,
                ExecutionError::Context(ContextRefusal::Unauthenticated),
            ),
        ] {
            let token = executor
                .admit_activity_budget_for_qualification(
                    DeclaredBudget::protocol_maximum(),
                    payer,
                    binding,
                    u128::MAX,
                )
                .unwrap();
            let request = AuthorizedExecutionRequest {
                module: &validated,
                program,
                authorization: AuthorizationContext::new(principal, CapabilitySet::empty()),
                receipts: &UnavailableReceiptOracle,
                entrypoint: "layerx_call",
                calldata: &[],
                composition: CompositionContext::new(
                    Rc::new(ProgramCatalog::new()),
                    CompositionRules::declared(),
                ),
                response_capacity: 0,
            };
            let mut storage = Storage::new();
            let before = storage.replay_state_bytes(MAX).unwrap();
            let result = executor.execute_authorized_authenticated(
                &mut storage,
                BudgetedAuthorizedExecutionRequest::new(
                    request,
                    token,
                    carried_payer,
                    carried_binding,
                ),
                sequence,
                batch,
            );
            assert_eq!(result.err(), Some(expected));
            assert_eq!(storage.replay_state_bytes(MAX).unwrap(), before);
        }
    }
}

#[test]
fn authenticated_producer_crosses_authority_and_bounded_replay_consumers_twice() {
    let corpus = Corpus::produce(false);
    for _ in 0..2 {
        let (receipt, admission, header, commit, host_proof) = corpus.consume();
        let authority =
            VerifiedReplayAuthority::derive(&receipt, &admission, &header, &commit).unwrap();
        assert_eq!(authority.principal(), corpus.principal);
        assert_eq!(authority.activity_binding(), corpus.activity_id);
        authority.verify_native(&corpus.authority).unwrap();
        let mut malformed = corpus.authority.clone();
        malformed.push(0);
        assert_eq!(
            authority.verify_native(&malformed),
            Err(ReplayError::Authority)
        );
        let metadata = corpus.host.transported(
            &[
                vec![1, 0, 9],
                b"progreplay/v1/".to_vec(),
                corpus.activity_id.to_vec(),
            ]
            .concat(),
        );
        let verify = |with_batch: bool, authority_bytes: &[u8]| {
            let catalogue = corpus.catalogue(&admission, &commit, &host_proof);
            if with_batch {
                VerifiedReplay::verify_with_batch_context(
                    &receipt,
                    &admission,
                    &header,
                    &commit,
                    corpus.host.root,
                    &host_proof,
                    &metadata,
                    authority_bytes,
                    &corpus.hosts,
                    catalogue,
                )
            } else {
                VerifiedReplay::verify(
                    &receipt,
                    &admission,
                    &commit,
                    corpus.host.root,
                    &host_proof,
                    &metadata,
                    authority_bytes,
                    &corpus.hosts,
                    catalogue,
                )
            }
        };
        for with_batch in [false, true] {
            let replay = verify(with_batch, &corpus.authority).unwrap();
            assert_eq!(
                replay.metadata().boundary_root,
                corpus.record.boundary_root()
            );
            assert_eq!(
                replay.metadata().boundary_count,
                corpus.record.boundary_count()
            );
            assert_eq!(
                replay.metadata().witness_digest,
                corpus.record.witness_digest()
            );
            let paths = corpus.paths();
            assert!(paths.len() > 2);
            let mut bad = paths[0].clone();
            bad.index = replay.metadata().boundary_count;
            assert_eq!(
                replay.verify_step(&bad, &paths[1]),
                Err(ReplayError::Bounds)
            );
            let mut bad = paths[0].clone();
            bad.leaf[0] ^= 1;
            assert!(replay.verify_step(&bad, &paths[1]).is_err());
            for pair in paths.windows(2) {
                let step = replay.verify_step(&pair[0], &pair[1]).unwrap_or_else(|error| {
                    panic!(
                        "authenticated Executor boundary pair {} -> {} must replay with the receipt payer: {error:?}; inner runtime result: {:?}",
                        pair[0].index,
                        pair[1].index,
                        corpus.runtime_pair_result(&pair[0], &pair[1]),
                    )
                });
                assert_eq!(step.activity_id(), corpus.activity_id);
                assert_eq!(step.pre_index(), pair[0].index);
                assert_eq!(step.post_index(), pair[1].index);
            }
        }
        assert!(matches!(
            verify(true, &malformed),
            Err(ReplayError::Authority)
        ));
        let mut wrong = commit.clone();
        wrong.receipt_proof.value[0] ^= 1;
        assert!(matches!(
            VerifiedReplayAuthority::derive(&receipt, &admission, &header, &wrong),
            Err(ReplayError::Receipt)
        ));
        // Authenticate a malformed producer table under its own genuine tree
        // root, so refusal happens in the table decoder, after transport.
        let malformed_sequencer = CommittedStore::commit(BTreeMap::from([(
            vec![3],
            commit.receipt_proof.value[..79].to_vec(),
        )]));
        let (app_hash, _, multistore_proof) =
            multistore(corpus.host.root, malformed_sequencer.root);
        let malformed_commit = ChainCommitProof {
            app_hash,
            store_root: malformed_sequencer.root,
            multistore_proof,
            receipt_proof: malformed_sequencer.transported(&[3]),
            ..commit.clone()
        };
        assert!(matches!(
            malformed_commit.receipt_keys(),
            Err(ReplayError::Receipt)
        ));
        assert_eq!(commit.receipt_keys().unwrap().len(), 1);
        let mut bad_signature = corpus.signature;
        bad_signature[0] ^= 1;
        assert!(verify_header(
            &corpus.header,
            &bad_signature,
            &commit.receipt_keys().unwrap()[0]
        )
        .is_err());
    }
}

#[test]
fn keeper_admission_reaches_existing_step_consumer_twice() {
    let corpus = Corpus::produce(true);
    for _ in 0..2 {
        let (receipt, admission, header, commit, host_proof) = corpus.consume();
        let authority =
            VerifiedReplayAuthority::derive_program_call(&receipt, &admission, &header, &commit)
                .unwrap();
        assert_eq!(authority.activity_binding(), admission.activity_id());
        assert_eq!(authority.principal(), corpus.principal);
        authority.verify_native(&corpus.authority).unwrap();
        assert!(matches!(
            VerifiedReplayAuthority::derive(&receipt, &admission, &header, &commit),
            Err(ReplayError::Authority)
        ));
        let mut unknown = admission.activity_id();
        unknown[0] ^= 1;
        let proofs = corpus.host.proofs.clone();
        assert!(AdmissionEvidence::verify_under_app_hash(
            commit.app_hash,
            corpus.host.root,
            &host_proof,
            &unknown,
            &proofs
        )
        .is_err());
    }
}

#[test]
fn keeper_namespaces_cross_public_items_into_market_consumer_twice() {
    let corpus = Corpus::produce(false);
    for _ in 0..2 {
        let (receipt, admission, header, commit, host_proof) = corpus.consume();
        let head = corpus.namespace_head();
        let observe = |program| {
            MarketCommittedNamespaces {
                program,
                tenant_receipt: &corpus.receipt,
                provider_receipt: &corpus.receipt,
                tenant_commit: &commit,
                provider_commit: &commit,
                tenant_state: corpus.namespace(&head, &host_proof),
                provider_state: corpus.namespace(&head, &host_proof),
            }
            .verify()
        };
        let observed = observe(corpus.program).unwrap();
        assert_eq!(observed.tenant, corpus.namespace_cells);
        assert_eq!(observed.provider, corpus.namespace_cells);
        let mut unknown = corpus.program;
        unknown[0] ^= 1;
        assert!(matches!(
            observe(unknown),
            Err(MarketStepVerdict::InvalidEvidence(
                MarketEvidenceError::Storage
            ))
        ));
        let mut malformed = corpus.manifest.clone();
        malformed[0] ^= 1;
        assert_eq!(
            NativeNamespaceProof {
                manifest: &malformed,
                ..corpus.namespace(&head, &host_proof)
            }
            .verify(commit.app_hash, corpus.program),
            Err(MarketEvidenceError::Storage)
        );
        let mut values = corpus.values.clone();
        values.values_mut().next().unwrap()[0] ^= 1;
        assert_eq!(
            NativeNamespaceProof {
                value_blobs: &values,
                ..corpus.namespace(&head, &host_proof)
            }
            .verify(commit.app_hash, corpus.program),
            Err(MarketEvidenceError::Storage)
        );
        let mut bad_receipt = corpus.receipt.clone();
        *bad_receipt.last_mut().unwrap() ^= 1;
        assert!(commit.verify_receipt_bytes(&bad_receipt).is_err());
        // An ordinary program's genuine receipt/namespace cannot be presented
        // as a Market profile. Exercise the aggregate consumer's own ingress.
        let paths = corpus.paths();
        let evidence = MarketReplayEvidence {
            expected_market_program: unknown,
            tenant_receipt: &receipt,
            tenant_admission: MarketAdmissionEvidence {
                prestate: &admission,
                header: &header,
            },
            tenant_commit: &commit,
            provider_receipt: &receipt,
            provider_admission: MarketAdmissionEvidence {
                prestate: &admission,
                header: &header,
            },
            provider_commit: &commit,
            tenant_state: corpus.namespace(&head, &host_proof),
            provider_state: corpus.namespace(&head, &host_proof),
            code_blobs: &corpus.code,
            baseline_storage_bytes: &Storage::new().replay_state_bytes(MAX).unwrap(),
            baseline_state: None,
            input_bytes: &[],
            initial_boundary: &paths[0],
            final_boundary: paths.last().unwrap(),
        };
        assert!(matches!(
            VerifiedMarketSandbox::verify(evidence),
            Err(MarketEvidenceError::Admission)
        ));
        assert_eq!(
            observe(corpus.program).unwrap().tenant,
            corpus.namespace_cells
        );
    }
}

#[test]
fn runtime_producer_records_integer_steps_and_terminal_traps() {
    for _ in 0..2 {
        for trapped in [false, true] {
            let body = if trapped {
                vec![0, OP_END]
            } else {
                vec![OP_I32_CONST, 7, OP_I32_CONST, 5, 0x6a, OP_END]
            };
            let wasm = module(&[
                type_section(&[(&[], &[TYPE_I32])]),
                function_section(&[0]),
                export_section(&[("compute", 0)]),
                code_section(&[func_body(&[], &body)]),
            ]);
            let validated = WasmEngine::declared()
                .unwrap()
                .validate_versioned(2, &wasm)
                .unwrap();
            let program = ProgramId::new(digest(&[&wasm])).unwrap();
            let tenant = PrincipalId::new(digest(&[b"raw-engine-tenant"])).unwrap();
            let lease = digest(&[b"raw-engine-lease"]);
            let baseline = Storage::new();
            let fees = FeeSchedule::declared();
            let authority = MarketSandboxReplayAuthority {
                profile_binding: digest(&[b"raw-engine-profile"]),
                namespace: market_sandbox_namespace(program, lease).unwrap(),
                lease_id: lease,
                namespace_limit: 1024,
                program,
                tenant,
                payment_account: tenant.bytes(),
                code_hash: validated.code_hash(),
                input_digest: market_sandbox_input_digest("compute", &[]).unwrap(),
                runtime_version: RUNTIME_VERSION,
                abi_version: 2,
                fee_schedule_version: fees.version(),
                metering_schedule_version: validated.metering_schedule_version(),
                budget: ResourceBudget::new_complete(100_000, 65_536, 1024, 1024, 1, 64, 0),
                fees,
                fee_budget: 1_000_000_000,
                baseline_state_root: market_sandbox_baseline_root(&baseline).unwrap(),
                baseline_storage: baseline,
            };
            let mut instance =
                instantiate_market_sandbox_untrusted(&validated, &authority).unwrap();
            let result = instance.call_market_sandbox_untrusted(MarketSandboxRequest {
                module: &validated,
                entrypoint: "compute",
                args: &[],
                authority: &authority,
                replay_profile: ProgramReplayProfile::new(128, MAX as u32).unwrap(),
            });
            if trapped {
                let execution = result.unwrap();
                assert_eq!(
                    execution.terminal_fault,
                    Some(ExecutionFault::UnreachableExecuted)
                );
                assert_eq!(execution.record.terminal_status(), 1);
                let terminal = PortableBoundary::decode_untrusted(
                    execution.boundary_leaves.last().unwrap(),
                    MAX,
                )
                .unwrap();
                assert!(terminal.trap.is_some());
                let observed =
                    observe_market_sandbox_step(&validated, &terminal, &authority, MAX).unwrap();
                assert_eq!(
                    observed.reencode_untrusted(MAX).unwrap(),
                    terminal.reencode_untrusted(MAX).unwrap()
                );
            } else {
                let execution = result.unwrap();
                assert_eq!(execution.values, vec![WasmValue::I32(12)]);
                assert!(execution.terminal_fault.is_none());
                let boundaries: Vec<_> = execution
                    .boundary_leaves
                    .iter()
                    .map(|leaf| PortableBoundary::decode_untrusted(leaf, MAX).unwrap())
                    .collect();
                assert_eq!(boundaries.len(), execution.record.boundary_count() as usize);
                let mut steps = 0;
                for pair in boundaries.windows(2) {
                    if let Ok(observed) =
                        observe_market_sandbox_step(&validated, &pair[0], &authority, MAX)
                    {
                        if observed.replay == pair[1].replay
                            && observed.semantic_bytes == pair[1].semantic_bytes
                        {
                            steps += 1;
                        }
                    }
                }
                assert!(steps > 0, "consumer replays actual producer instruction");
                let mut malformed = execution.boundary_leaves[0].clone();
                malformed.push(0);
                assert!(PortableBoundary::decode_untrusted(&malformed, MAX).is_err());
            }
        }
    }
}
