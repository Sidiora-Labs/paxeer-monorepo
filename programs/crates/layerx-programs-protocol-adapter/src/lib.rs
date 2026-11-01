#![deny(unsafe_code)]

#[allow(unsafe_code)]
mod ffi;

pub use ffi::{
    read_program_state, restore_verified, restore_verified_profile2, ProtocolAdapterError,
    ProtocolProgramStateRead,
};
