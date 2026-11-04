use layerx_program_sdk::{
    storage::{shared::SharedStorageKey, StorageValue},
    CallResult, Principal, ProgramDeposit, ProgramError, ProgramId,
};

pub trait Host {
    fn immediate_caller(&mut self) -> Result<Option<ProgramId>, ProgramError>;
    fn invoking_principal(&mut self) -> Result<Principal, ProgramError>;
    fn executing_program(&mut self) -> Result<ProgramId, ProgramError>;
    fn batch_height(&mut self) -> Result<u64, ProgramError>;
    fn read(
        &mut self,
        key: SharedStorageKey<'_>,
        output: &mut [u8],
    ) -> Result<Option<usize>, ProgramError>;
    fn write(
        &mut self,
        key: SharedStorageKey<'_>,
        value: StorageValue<'_>,
    ) -> Result<(), ProgramError>;
    fn delete(&mut self, key: SharedStorageKey<'_>) -> Result<(), ProgramError>;
    fn fund_program_account(&mut self, deposit: ProgramDeposit<'_>) -> Result<(), ProgramError>;
    fn publish_response(&mut self, result: CallResult, bytes: &[u8]) -> Result<(), ProgramError>;
}

#[cfg(target_arch = "wasm32")]
pub struct GuestHost;

#[cfg(target_arch = "wasm32")]
impl Host for GuestHost {
    fn immediate_caller(&mut self) -> Result<Option<ProgramId>, ProgramError> {
        layerx_program_sdk::Context::immediate_caller()
    }

    fn invoking_principal(&mut self) -> Result<Principal, ProgramError> {
        layerx_program_sdk::Context::invoking_principal()
    }

    fn executing_program(&mut self) -> Result<ProgramId, ProgramError> {
        layerx_program_sdk::Context::executing_program()
    }

    fn batch_height(&mut self) -> Result<u64, ProgramError> {
        layerx_program_sdk::Context::batch_height()
    }

    fn read(
        &mut self,
        key: SharedStorageKey<'_>,
        output: &mut [u8],
    ) -> Result<Option<usize>, ProgramError> {
        layerx_program_sdk::storage::shared::read(key, output)
    }

    fn write(
        &mut self,
        key: SharedStorageKey<'_>,
        value: StorageValue<'_>,
    ) -> Result<(), ProgramError> {
        layerx_program_sdk::storage::shared::write(key, value)
    }

    fn delete(&mut self, key: SharedStorageKey<'_>) -> Result<(), ProgramError> {
        layerx_program_sdk::storage::shared::delete(key)
    }

    fn fund_program_account(&mut self, deposit: ProgramDeposit<'_>) -> Result<(), ProgramError> {
        layerx_program_sdk::transfer::fund_program_account(deposit)
    }

    fn publish_response(&mut self, result: CallResult, bytes: &[u8]) -> Result<(), ProgramError> {
        layerx_program_sdk::call::publish_response(result, bytes)
    }
}
