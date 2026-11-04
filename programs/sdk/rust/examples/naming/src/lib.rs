#![cfg_attr(target_arch = "wasm32", no_std)]

pub mod host;

use host::Host;

use layerx_program_sdk::{
    naming::{
        decode_label, encode_label, occupancy_expiry, occupancy_price, Name, Record, Request,
        DID_KEY_PREFIX, LABEL_BYTES, NAME_KEY_PREFIX, RECORD_BYTES, REFERENCE_ASSET,
        REFERENCE_OCCUPANCY_SEED,
    },
    payments::PreparedProgramAccount,
    storage::{shared::SharedStorageKey, StorageValue},
    AssetId, Bytes, CallResult, Field, Principal, ProgramError, Reason,
};

#[cfg(target_arch = "wasm32")]
layerx_program_sdk::trap_on_panic!();

const NAME_KEY_BYTES: usize = 82;
const DID_KEY_BYTES: usize = 50;
const RESPONSE_BYTES: usize = 70;

fn denied() -> ProgramError {
    ProgramError::value(Field::CallInput, Reason::Malformed)
}

fn direct(host: &mut impl Host) -> Result<Principal, ProgramError> {
    if host.immediate_caller()?.is_some() {
        return Err(denied());
    }
    host.invoking_principal()
}

fn name_key(name: Name<'_>) -> Result<Bytes<NAME_KEY_BYTES>, ProgramError> {
    let mut out = Bytes::empty();
    out.extend(NAME_KEY_PREFIX)?;
    out.extend(name.bytes())?;
    Ok(out)
}

fn did_key(did: Principal) -> Result<Bytes<DID_KEY_BYTES>, ProgramError> {
    let mut out = Bytes::empty();
    out.extend(DID_KEY_PREFIX)?;
    out.extend(&did.bytes())?;
    Ok(out)
}

fn occupancy(host: &mut impl Host) -> Result<PreparedProgramAccount<'static>, ProgramError> {
    PreparedProgramAccount::new(
        host.executing_program()?,
        REFERENCE_OCCUPANCY_SEED,
        AssetId::new(REFERENCE_ASSET)?,
    )
}

fn read_record(host: &mut impl Host, name: Name<'_>) -> Result<Option<Record>, ProgramError> {
    let key = name_key(name)?;
    let mut value = [0; RECORD_BYTES];
    match host.read(SharedStorageKey::new(key.as_slice())?, &mut value)? {
        None => Ok(None),
        Some(RECORD_BYTES) => Record::decode(&value).map(Some),
        Some(_) => Err(denied()),
    }
}

fn read_reverse(
    host: &mut impl Host,
    did: Principal,
    value: &mut [u8; LABEL_BYTES],
) -> Result<Option<usize>, ProgramError> {
    let key = did_key(did)?;
    host.read(SharedStorageKey::new(key.as_slice())?, value)
}

fn write_record(host: &mut impl Host, name: Name<'_>, record: Record) -> Result<(), ProgramError> {
    let key = name_key(name)?;
    host.write(
        SharedStorageKey::new(key.as_slice())?,
        StorageValue::new(record.encode()?.as_slice())?,
    )
}

fn write_reverse(host: &mut impl Host, did: Principal, name: Name<'_>) -> Result<(), ProgramError> {
    let key = did_key(did)?;
    host.write(
        SharedStorageKey::new(key.as_slice())?,
        StorageValue::new(encode_label(name)?.as_slice())?,
    )
}

fn clear_reverse(host: &mut impl Host, did: Principal, name: Name<'_>) -> Result<(), ProgramError> {
    let mut value = [0; LABEL_BYTES];
    if let Some(written) = read_reverse(host, did, &mut value)? {
        let encoded = value.get(..written).ok_or_else(denied)?;
        if decode_label(encoded)? == name {
            let key = did_key(did)?;
            host.delete(SharedStorageKey::new(key.as_slice())?)?;
        }
    }
    Ok(())
}

fn response(host: &mut impl Host, bytes: &[u8]) -> Result<CallResult, ProgramError> {
    let mut encoded = Bytes::<RESPONSE_BYTES>::empty();
    encoded.extend(&[1, 0x20])?;
    encoded.extend(
        &u32::try_from(bytes.len())
            .map_err(|_| denied())?
            .to_be_bytes(),
    )?;
    encoded.extend(bytes)?;
    host.publish_response(CallResult::OK, encoded.as_slice())?;
    Ok(CallResult::OK)
}

pub fn register_impl(host: &mut impl Host, input: &[u8]) -> Result<CallResult, ProgramError> {
    let caller = direct(host)?;
    let Request::Register { name, did, periods } = Request::decode(input)? else {
        return Err(denied());
    };
    if did != caller {
        return Err(denied());
    }
    let height = host.batch_height()?;
    let released = match read_record(host, name)? {
        None => None,
        Some(previous) if previous.expiry > height => return Err(denied()),
        Some(previous) => Some(previous.did),
    };
    let deposit = occupancy(host)?.deposit(occupancy_price(periods)?)?;
    host.fund_program_account(deposit)?;
    if let Some(previous) = released {
        if previous != did {
            clear_reverse(host, previous, name)?;
        }
    }
    write_record(
        host,
        name,
        Record {
            did,
            expiry: occupancy_expiry(height, periods)?,
        },
    )?;
    write_reverse(host, did, name)?;
    response(host, &[])
}

pub fn transfer_impl(host: &mut impl Host, input: &[u8]) -> Result<CallResult, ProgramError> {
    let caller = direct(host)?;
    let Request::Transfer { name, did } = Request::decode(input)? else {
        return Err(denied());
    };
    if did == caller {
        return Err(denied());
    }
    let height = host.batch_height()?;
    let record = read_record(host, name)?.ok_or_else(denied)?;
    if record.did != caller || record.expiry <= height {
        return Err(denied());
    }
    clear_reverse(host, caller, name)?;
    write_record(
        host,
        name,
        Record {
            did,
            expiry: record.expiry,
        },
    )?;
    write_reverse(host, did, name)?;
    response(host, &[])
}

pub fn renew_impl(host: &mut impl Host, input: &[u8]) -> Result<CallResult, ProgramError> {
    let caller = direct(host)?;
    let Request::Renew { name, periods } = Request::decode(input)? else {
        return Err(denied());
    };
    let height = host.batch_height()?;
    let record = read_record(host, name)?.ok_or_else(denied)?;
    if record.did != caller || record.expiry <= height {
        return Err(denied());
    }
    let deposit = occupancy(host)?.deposit(occupancy_price(periods)?)?;
    host.fund_program_account(deposit)?;
    write_record(
        host,
        name,
        Record {
            did: record.did,
            expiry: occupancy_expiry(record.expiry, periods)?,
        },
    )?;
    response(host, &[])
}

pub fn resolve_impl(host: &mut impl Host, input: &[u8]) -> Result<CallResult, ProgramError> {
    direct(host)?;
    let Request::Resolve { name } = Request::decode(input)? else {
        return Err(denied());
    };
    let height = host.batch_height()?;
    let record = read_record(host, name)?.ok_or_else(denied)?;
    if record.expiry <= height {
        return Err(denied());
    }
    response(host, record.encode()?.as_slice())
}

pub fn reverse_resolve_impl(
    host: &mut impl Host,
    input: &[u8],
) -> Result<CallResult, ProgramError> {
    direct(host)?;
    let Request::ReverseResolve { did } = Request::decode(input)? else {
        return Err(denied());
    };
    let height = host.batch_height()?;
    let mut value = [0; LABEL_BYTES];
    let written = read_reverse(host, did, &mut value)?.ok_or_else(denied)?;
    let encoded = value.get(..written).ok_or_else(denied)?;
    let name = decode_label(encoded)?;
    let record = read_record(host, name)?.ok_or_else(denied)?;
    if record.did != did || record.expiry <= height {
        return Err(denied());
    }
    response(host, encoded)
}

#[cfg(target_arch = "wasm32")]
macro_rules! export {
    ($name:ident, $handler:ident) => {
        #[allow(unsafe_code)]
        #[no_mangle]
        pub extern "C" fn $name(pointer: i32, length: i32) -> i32 {
            match layerx_program_sdk::entry::with_call_input(pointer, length, |input| {
                $handler(&mut host::GuestHost, input)
            }) {
                Ok(Ok(result)) => result.code(),
                Ok(Err(error)) | Err(error) => error.code(),
            }
        }
    };
}

#[cfg(target_arch = "wasm32")]
export!(register, register_impl);
#[cfg(target_arch = "wasm32")]
export!(transfer, transfer_impl);
#[cfg(target_arch = "wasm32")]
export!(renew, renew_impl);
#[cfg(target_arch = "wasm32")]
export!(resolve, resolve_impl);
#[cfg(target_arch = "wasm32")]
export!(reverse_resolve, reverse_resolve_impl);

#[cfg(target_arch = "wasm32")]
fn refuse(_: &[u8]) -> Result<CallResult, ProgramError> {
    Err(denied())
}
#[cfg(target_arch = "wasm32")]
fn legacy(_: i64) -> Result<i64, ProgramError> {
    Err(denied())
}
#[cfg(target_arch = "wasm32")]
layerx_program_sdk::entrypoint!(refuse);
#[cfg(target_arch = "wasm32")]
layerx_program_sdk::program!(legacy);
