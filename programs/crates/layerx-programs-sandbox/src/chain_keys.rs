use layerx_programs::chain_evidence::programs_state_key;

use crate::activity::SandboxActivityRefusal;

pub fn sandbox_state_key(kind: u8, lease_id: [u8; 32]) -> Result<Vec<u8>, SandboxActivityRefusal> {
    if kind > 2 {
        return Err(SandboxActivityRefusal::NonCanonical);
    }
    let mut kernel_key = [0_u8; 34];
    kernel_key[0] = b's';
    kernel_key[1] = kind;
    kernel_key[2..].copy_from_slice(&lease_id);
    Ok(programs_state_key(&kernel_key))
}
