use std::collections::BTreeMap;

use layerx_proof::state::{decode_account_value, CanonicalAccount};
use layerx_proof::state_range::{
    verify_account_tree, verify_composite_roots, ModuleRangeWitness, MAX_MODULE_LEAVES,
    MAX_UNIVERSAL_LEAVES,
};
use layerx_proof::state_witness::StateWitness;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::activity::Activity;

use crate::{ics23, ReplayError};

const MAX_BYTES: usize = 64 * 1024 * 1024;
const MAX_WITNESS_BYTES: usize = 35 + 129 + 1_048_576 + 96 * 32;
const HEADER_DOMAIN: &[u8] = b"LXP/arbiter-admission/v1\0";

#[derive(Clone, Debug)]
pub struct AdmissionEvidence {
    network_id: u32,
    activity_id: [u8; 32],
    execution_sequence: u64,
    state_root: [u8; 32],
    receipt_digest: [u8; 32],
    activity: Activity,
    governance_records: BTreeMap<Vec<u8>, Vec<u8>>,
    legacy: AdmissionLegacy,
    header: Vec<u8>,
}

#[derive(Clone, Debug)]
pub struct AdmissionLegacy {
    legacy: AdmissionPrestate,
}

#[derive(Clone, Debug)]
pub struct AdmissionPrestate {
    program_records: BTreeMap<Vec<u8>, Vec<u8>>,
    accounts: BTreeMap<[u8; 32], CanonicalAccount>,
    canonical_bytes: Vec<u8>,
}

impl AdmissionEvidence {
    pub fn verify_under_app_hash(
        app_hash: [u8; 32],
        store_root: [u8; 32],
        multistore_proof: &[u8],
        activity_id: &[u8; 32],
        record_proofs: &BTreeMap<Vec<u8>, ics23::ExistenceProof>,
    ) -> Result<Self, ReplayError> {
        if multistore_proof.len() > 1_048_576
            || record_proofs.len() > 1 + 4096 + 2 * MAX_MODULE_LEAVES
        {
            return Err(ReplayError::Admission);
        }
        crate::multistore::verify_store_root("lxphost", store_root, multistore_proof, app_hash)
            .map_err(|_| ReplayError::Admission)?;
        let mut header_key = vec![1, 0, 9];
        header_key.extend_from_slice(b"admission/v1/");
        header_key.extend_from_slice(activity_id);
        let header = &record_proofs
            .get(&header_key)
            .ok_or(ReplayError::Admission)?
            .value;
        if header.len() > MAX_BYTES {
            return Err(ReplayError::Admission);
        }
        let mut expected = BTreeMap::new();
        expected.insert(header_key, header.clone());
        let mut h = Reader(header);
        if h.take(HEADER_DOMAIN.len())? != HEADER_DOMAIN {
            return Err(ReplayError::Admission);
        }
        let receipt_digest = h.array()?;
        if receipt_digest == [0; 32] {
            return Err(ReplayError::Admission);
        }
        let mut v3 = Reader(h.bytes(MAX_BYTES)?);
        h.done()?;
        if v3.u16()? != 3 {
            return Err(ReplayError::Admission);
        }
        let mut v2 = Reader(v3.bytes(MAX_BYTES)?);
        if v2.u16()? != 2 {
            return Err(ReplayError::Admission);
        }
        let native = v2.bytes(MAX_BYTES)?;
        let mut v1 = Reader(native);
        if v1.u16()? != 1 {
            return Err(ReplayError::Admission);
        }
        let network_id = v1.u32()?;
        let pinned_activity = v1.array()?;
        let execution_sequence = v1.u64()?;
        let state_root = v1.array()?;
        if network_id == 0 || pinned_activity != *activity_id || execution_sequence == 0 {
            return Err(ReplayError::Admission);
        }
        let count = v1.u16()?;
        if count == 0 || count > 64 {
            return Err(ReplayError::Admission);
        }
        let roots = (0..count)
            .map(|_| v1.array())
            .collect::<Result<Vec<_>, _>>()?;
        verify_composite_roots(&roots, state_root).map_err(|_| ReplayError::Admission)?;
        let universal = v1.module(0, &roots, state_root, None)?;
        let programs = v1.module(9, &roots, state_root, Some(b"progfee/history/v1/"))?;
        if !universal
            .leaves
            .iter()
            .any(|w| w.key == b"sequence" && w.value == execution_sequence.to_be_bytes())
        {
            return Err(ReplayError::Admission);
        }
        let account_count = v1.count(4096)?;
        let mut witnesses = Vec::with_capacity(account_count);
        let mut accounts = BTreeMap::new();
        for _ in 0..account_count {
            let witness = v1.witness()?;
            if witness.module_id != 0 || witness.key.len() != 33 || witness.key[0] != 4 {
                return Err(ReplayError::Admission);
            }
            let id = witness.key[1..]
                .try_into()
                .map_err(|_| ReplayError::Admission)?;
            let account =
                decode_account_value(id, &witness.value).map_err(|_| ReplayError::Admission)?;
            let mut key = vec![6];
            key.extend_from_slice(&id);
            if expected.insert(key, witness.value.clone()).is_some()
                || accounts.insert(id, account).is_some()
            {
                return Err(ReplayError::Admission);
            }
            witnesses.push(witness);
        }
        verify_account_tree(&witnesses, &universal, state_root)
            .map_err(|_| ReplayError::Admission)?;
        v1.done()?;
        if v2.u16()? != 2 {
            return Err(ReplayError::Admission);
        }
        v2.module(6, &roots, state_root, Some(b"oracle:"))?;
        v2.module(11, &roots, state_root, Some(b"web/answer"))?;
        v2.done()?;
        let signed = v3.bytes(layerx_wire::limits::MAX_MESSAGE_BYTES)?;
        let activities = (1..=10)
            .map(|n| ActivityType::new(ModuleId::Programs, n).map_err(|_| ReplayError::Admission))
            .collect::<Result<Vec<_>, _>>()?;
        let registration = ModuleRegistration::new(ModuleId::Programs, &activities)
            .map_err(|_| ReplayError::Admission)?;
        let registry = ModuleRegistry::new(&[registration]).map_err(|_| ReplayError::Admission)?;
        let activity = layerx_wire::activity::decode_signed(signed, &registry)
            .map_err(|_| ReplayError::Admission)?;
        if layerx_wire::activity::encode_signed(&activity).map_err(|_| ReplayError::Admission)?
            != signed
            || activity.network_id() != network_id
            || activity.protocol_version() != 3
            || activity.actor_did().is_empty()
            || layerx_wire::hash::activity_id(&activity).map_err(|_| ReplayError::Admission)?
                != *activity_id
            || layerx_wire::hash::payload_hash(&activity).map_err(|_| ReplayError::Admission)?
                != activity.payload_hash()
        {
            return Err(ReplayError::Admission);
        }
        let public = activity
            .authority()
            .try_into()
            .map_err(|_| ReplayError::Admission)?;
        let signature = activity
            .signature()
            .ok_or(ReplayError::Admission)?
            .try_into()
            .map_err(|_| ReplayError::Admission)?;
        let unsigned =
            layerx_wire::activity::signing_bytes(&activity).map_err(|_| ReplayError::Admission)?;
        let message = layerx_crypto::SignatureMessage::new(
            layerx_wire::hash::Domain::SignaturePreimage,
            activity.protocol_version(),
            network_id,
            unsigned.as_bytes(),
        )
        .map_err(|_| ReplayError::Admission)?;
        layerx_crypto::ed25519::verify(&public, &signature, message)
            .map_err(|_| ReplayError::Admission)?;
        if v3.u16()? != 3 {
            return Err(ReplayError::Admission);
        }
        v3.module(1, &roots, state_root, None)?;
        v3.module(3, &roots, state_root, None)?;
        let governance = v3.module(7, &roots, state_root, None)?;
        v3.done()?;
        let program_records = records(&programs, &mut expected)?;
        let governance_records = records(&governance, &mut expected)?;
        if record_proofs.len() != expected.len() {
            return Err(ReplayError::Admission);
        }
        for (key, value) in expected {
            let proof = record_proofs.get(&key).ok_or(ReplayError::Admission)?;
            if proof.key != key || proof.value != value {
                return Err(ReplayError::Admission);
            }
            ics23::verify_existence(proof, store_root, &key, &value)
                .map_err(|_| ReplayError::Admission)?;
        }
        Ok(Self {
            network_id,
            activity_id: *activity_id,
            execution_sequence,
            state_root,
            receipt_digest,
            activity,
            governance_records,
            legacy: AdmissionLegacy {
                legacy: AdmissionPrestate {
                    program_records,
                    accounts,
                    canonical_bytes: native.to_vec(),
                },
            },
            header: header.clone(),
        })
    }

    pub fn network_id(&self) -> u32 {
        self.network_id
    }
    pub fn activity_id(&self) -> [u8; 32] {
        self.activity_id
    }
    pub fn activity(&self) -> &Activity {
        &self.activity
    }
    pub fn execution_sequence(&self) -> u64 {
        self.execution_sequence
    }
    pub fn state_root(&self) -> [u8; 32] {
        self.state_root
    }
    pub fn receipt_digest(&self) -> [u8; 32] {
        self.receipt_digest
    }
    pub fn header(&self) -> &[u8] {
        &self.header
    }
    pub fn governance_records(&self) -> &BTreeMap<Vec<u8>, Vec<u8>> {
        &self.governance_records
    }
    pub fn legacy(&self) -> &AdmissionLegacy {
        &self.legacy
    }
}
impl AdmissionLegacy {
    pub fn legacy(&self) -> &AdmissionPrestate {
        &self.legacy
    }
}
impl AdmissionPrestate {
    pub fn program_records(&self) -> &BTreeMap<Vec<u8>, Vec<u8>> {
        &self.program_records
    }
    pub fn all_accounts(&self) -> &BTreeMap<[u8; 32], CanonicalAccount> {
        &self.accounts
    }
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical_bytes
    }
}

fn records(
    module: &ModuleRangeWitness,
    expected: &mut BTreeMap<Vec<u8>, Vec<u8>>,
) -> Result<BTreeMap<Vec<u8>, Vec<u8>>, ReplayError> {
    let mut records = BTreeMap::new();
    for witness in &module.leaves {
        let mut key = vec![1];
        key.extend_from_slice(&module.module_id.to_be_bytes());
        key.extend_from_slice(&witness.key);
        if expected.insert(key, witness.value.clone()).is_some()
            || records
                .insert(witness.key.clone(), witness.value.clone())
                .is_some()
        {
            return Err(ReplayError::Admission);
        }
    }
    Ok(records)
}

struct Reader<'a>(&'a [u8]);
impl<'a> Reader<'a> {
    fn take(&mut self, n: usize) -> Result<&'a [u8], ReplayError> {
        let value = self.0.get(..n).ok_or(ReplayError::Admission)?;
        self.0 = &self.0[n..];
        Ok(value)
    }
    fn array<const N: usize>(&mut self) -> Result<[u8; N], ReplayError> {
        self.take(N)?.try_into().map_err(|_| ReplayError::Admission)
    }
    fn u16(&mut self) -> Result<u16, ReplayError> {
        Ok(u16::from_be_bytes(self.array()?))
    }
    fn u32(&mut self) -> Result<u32, ReplayError> {
        Ok(u32::from_be_bytes(self.array()?))
    }
    fn u64(&mut self) -> Result<u64, ReplayError> {
        Ok(u64::from_be_bytes(self.array()?))
    }
    fn count(&mut self, maximum: usize) -> Result<usize, ReplayError> {
        let count = usize::try_from(self.u32()?).map_err(|_| ReplayError::Admission)?;
        if count > maximum {
            return Err(ReplayError::Admission);
        }
        Ok(count)
    }
    fn bytes(&mut self, maximum: usize) -> Result<&'a [u8], ReplayError> {
        let n = self.count(maximum)?;
        self.take(n)
    }
    fn done(&self) -> Result<(), ReplayError> {
        if self.0.is_empty() {
            Ok(())
        } else {
            Err(ReplayError::Admission)
        }
    }
    fn witness(&mut self) -> Result<StateWitness, ReplayError> {
        let bytes = self.bytes(MAX_WITNESS_BYTES)?;
        let witness = StateWitness::decode(bytes).map_err(|_| ReplayError::Admission)?;
        if witness.encode().map_err(|_| ReplayError::Admission)? != bytes {
            return Err(ReplayError::Admission);
        }
        Ok(witness)
    }
    fn module(
        &mut self,
        expected: u16,
        roots: &[[u8; 32]],
        root: [u8; 32],
        prefix: Option<&[u8]>,
    ) -> Result<ModuleRangeWitness, ReplayError> {
        let module_id = self.u16()?;
        let subtree_root = self.array()?;
        let composite_index = self.u32()?;
        let composite_count = self.u32()?;
        let depth = usize::from(self.array::<1>()?[0]);
        if module_id != expected
            || composite_index != u32::from(expected)
            || composite_count as usize != roots.len()
            || roots.get(usize::from(expected)) != Some(&subtree_root)
            || depth > 32
        {
            return Err(ReplayError::Admission);
        }
        let composite_siblings = (0..depth)
            .map(|_| self.array())
            .collect::<Result<Vec<_>, _>>()?;
        let count = self.count(if expected == 0 {
            MAX_UNIVERSAL_LEAVES
        } else {
            MAX_MODULE_LEAVES
        })?;
        let leaves = (0..count)
            .map(|_| self.witness())
            .collect::<Result<Vec<_>, _>>()?;
        let module = ModuleRangeWitness {
            module_id,
            subtree_root,
            composite_index,
            composite_count,
            composite_siblings,
            leaves,
        };
        match prefix {
            Some(prefix) => {
                module
                    .verify_prefix(root, prefix)
                    .map_err(|_| ReplayError::Admission)?;
            }
            None => {
                module
                    .verify_full_module(root)
                    .map_err(|_| ReplayError::Admission)?;
            }
        }
        Ok(module)
    }
}
