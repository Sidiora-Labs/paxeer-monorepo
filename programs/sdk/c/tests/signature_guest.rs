#![no_std]

use layerx_program_sdk::{crypto, entry, ProgramError};

layerx_program_sdk::trap_on_panic!();

#[no_mangle]
pub extern "C" fn layerx_reserve(length: i32) -> i32 {
    entry::reserve_call_input(length)
}

#[no_mangle]
pub extern "C" fn layerx_main(_: i64) -> i64 {
    0
}

fn verify(input: &[u8]) -> Result<(), ProgramError> {
    if input.len() < 3 || input.len() != 3 + usize::from(input[1]) + usize::from(input[2]) + 64 {
        panic!("invalid test calldata");
    }
    let message_end = 3 + usize::from(input[2]);
    let key_end = message_end + usize::from(input[1]);
    let message = &input[3..message_end];
    let key = &input[message_end..key_end];
    let signature = input[key_end..]
        .try_into()
        .unwrap_or_else(|_| panic!("signature length"));
    match input[0] {
        1 => crypto::ed25519_verify(
            crypto::Ed25519Message::new(message)?,
            key.try_into()
                .unwrap_or_else(|_| panic!("Ed25519 key length")),
            signature,
        ),
        2 => crypto::secp256k1_verify(
            message
                .try_into()
                .unwrap_or_else(|_| panic!("digest length")),
            &crypto::Secp256k1PublicKey::Uncompressed(
                key.try_into()
                    .unwrap_or_else(|_| panic!("secp256k1 key length")),
            ),
            signature,
        ),
        _ => panic!("invalid test algorithm"),
    }
}

#[no_mangle]
pub extern "C" fn layerx_call(pointer: i32, length: i32) -> i32 {
    entry::with_call_input(pointer, length, |input| match verify(input) {
        Ok(()) => 0,
        Err(error) => error.abi_code(),
    })
    .unwrap_or_else(|error| error.abi_code())
}
