//! Append-only artifacts used to reproduce authorized disputes after upgrades.

use crate::{
    CompiledModule, FeeSchedule, FeeScheduleParameters, FuelSchedule, ModuleCacheKey, ProgramId,
};
use std::{collections::BTreeMap, sync::Arc};

const DOMAIN: &[u8] = b"LayerX/programs/dispute-artifacts/v1\0";
const MAX_ARCHIVE_BYTES: usize = 64 * 1024 * 1024;

type ArtifactKey = (ProgramId, [u8; 32], u16, u32);

#[derive(Debug, Default)]
pub struct DisputeArtifacts {
    modules: BTreeMap<ArtifactKey, Arc<CompiledModule>>,
    sources: BTreeMap<ArtifactKey, (Vec<u8>, FuelSchedule)>,
    fees: BTreeMap<u32, FeeSchedule>,
}

impl DisputeArtifacts {
    /// Retains an admitted artifact and schedule without replacing an earlier version.
    pub fn retain(
        &mut self,
        program: ProgramId,
        wasm: &[u8],
        abi: u16,
        metering: FuelSchedule,
        fees: FeeSchedule,
    ) -> Result<(), crate::replay::ReplayWitnessError> {
        use crate::replay::ReplayWitnessError as E;
        if !fees.is_valid()
            || self
                .fees
                .get(&fees.version())
                .is_some_and(|prior| *prior != fees)
        {
            return Err(E::Binding);
        }
        let hash = crate::hash_bytes(crate::HashAlgorithm::Sha256, wasm).map_err(|_| E::Binding)?;
        let key = ModuleCacheKey::for_wasm_with_schedule(
            hash,
            crate::RUNTIME_VERSION,
            abi,
            wasm,
            metering,
        )
        .map_err(|_| E::Binding)?;
        let module = CompiledModule::compile(
            &crate::WasmEngine::declared().map_err(|_| E::Binding)?,
            key,
            wasm,
        )
        .map_err(|_| E::Binding)?;
        let identity = (program, hash, abi, metering.version());
        self.modules
            .entry(identity)
            .or_insert_with(|| Arc::new(module));
        self.sources
            .entry(identity)
            .or_insert_with(|| (wasm.to_vec(), metering));
        self.fees.entry(fees.version()).or_insert(fees);
        Ok(())
    }

    pub fn module(
        &self,
        program: ProgramId,
        hash: [u8; 32],
        abi: u16,
        metering: u32,
    ) -> Option<Arc<CompiledModule>> {
        self.modules.get(&(program, hash, abi, metering)).cloned()
    }

    pub fn fee_schedule(&self, version: u32) -> Option<FeeSchedule> {
        self.fees.get(&version).copied()
    }

    /// Canonical durable data; retention has no eviction tied to the current deployment.
    pub fn canonical_bytes(&self) -> Result<Vec<u8>, crate::replay::ReplayWitnessError> {
        use crate::replay::ReplayWitnessError as E;
        let mut bytes = DOMAIN.to_vec();
        bytes.extend_from_slice(
            &u32::try_from(self.fees.len())
                .map_err(|_| E::Bounds)?
                .to_be_bytes(),
        );
        for schedule in self.fees.values() {
            bytes.extend_from_slice(&schedule.version().to_be_bytes());
            for coefficient in [
                schedule.cpu_price(),
                schedule.memory_byte_price(),
                schedule.storage_read_byte_price(),
                schedule.storage_write_byte_price(),
                schedule.output_value_price(),
                schedule.output_byte_price(),
                schedule.occupancy_byte_batch_price(),
            ] {
                bytes.extend_from_slice(&coefficient.to_be_bytes());
            }
        }
        bytes.extend_from_slice(
            &u32::try_from(self.sources.len())
                .map_err(|_| E::Bounds)?
                .to_be_bytes(),
        );
        for ((program, hash, abi, _), (wasm, schedule)) in &self.sources {
            bytes.extend_from_slice(&program.bytes());
            bytes.extend_from_slice(hash);
            bytes.extend_from_slice(&abi.to_be_bytes());
            bytes.extend_from_slice(&schedule.canonical_bytes());
            bytes.extend_from_slice(
                &u32::try_from(wasm.len())
                    .map_err(|_| E::Bounds)?
                    .to_be_bytes(),
            );
            bytes.extend_from_slice(wasm);
        }
        if bytes.len() > MAX_ARCHIVE_BYTES {
            return Err(E::Bounds);
        }
        Ok(bytes)
    }

    /// Reopens data recovered from an authenticated protocol store.
    pub fn from_authenticated_bytes(
        bytes: &[u8],
    ) -> Result<Self, crate::replay::ReplayWitnessError> {
        use crate::replay::{ReplayCursor, ReplayWitnessError as E};
        if bytes.len() > MAX_ARCHIVE_BYTES {
            return Err(E::Bounds);
        }
        let mut cursor = ReplayCursor::new(bytes);
        if cursor.take(DOMAIN.len())? != DOMAIN {
            return Err(E::Encoding);
        }
        let count = u32::from_be_bytes(cursor.array()?);
        let mut archive = Self::default();
        for _ in 0..count {
            let schedule = FeeSchedule::new_complete(FeeScheduleParameters {
                version: u32::from_be_bytes(cursor.array()?),
                fee_units_per_cpu_fuel: u64::from_be_bytes(cursor.array()?),
                fee_units_per_memory_byte: u64::from_be_bytes(cursor.array()?),
                fee_units_per_storage_read_byte: u64::from_be_bytes(cursor.array()?),
                fee_units_per_storage_write_byte: u64::from_be_bytes(cursor.array()?),
                fee_units_per_output_value: u64::from_be_bytes(cursor.array()?),
                fee_units_per_output_byte: u64::from_be_bytes(cursor.array()?),
                fee_units_per_occupancy_byte_batch: u64::from_be_bytes(cursor.array()?),
            });
            if !schedule.is_valid() || archive.fees.insert(schedule.version(), schedule).is_some() {
                return Err(E::Binding);
            }
        }
        let count = u32::from_be_bytes(cursor.array()?);
        for _ in 0..count {
            let program = ProgramId::new(cursor.array()?).map_err(|_| E::Binding)?;
            let hash = cursor.array()?;
            let abi = u16::from_be_bytes(cursor.array()?);
            let schedule =
                FuelSchedule::from_protocol_bytes(cursor.take(76)?).map_err(|_| E::Binding)?;
            let length = u32::from_be_bytes(cursor.array()?) as usize;
            let wasm = cursor.take(length)?;
            let key = ModuleCacheKey::for_wasm_with_schedule(
                hash,
                crate::RUNTIME_VERSION,
                abi,
                wasm,
                schedule,
            )
            .map_err(|_| E::Binding)?;
            let module = CompiledModule::compile(
                &crate::WasmEngine::declared().map_err(|_| E::Binding)?,
                key,
                wasm,
            )
            .map_err(|_| E::Binding)?;
            let identity = (program, hash, abi, schedule.version());
            if archive.modules.insert(identity, Arc::new(module)).is_some() {
                return Err(E::Binding);
            }
            archive.sources.insert(identity, (wasm.to_vec(), schedule));
        }
        if !cursor.done() || archive.canonical_bytes()? != bytes {
            return Err(E::Encoding);
        }
        Ok(archive)
    }
}
