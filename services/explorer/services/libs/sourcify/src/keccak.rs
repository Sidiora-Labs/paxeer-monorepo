// Keccak-256 for Solidity library identifiers. The legacy Keccak suffix (0x01)
// is required here; SHA3-256 uses a different suffix and different digests.
pub(crate) fn keccak256(input: &[u8]) -> [u8; 32] {
    const RATE: usize = 136;
    let mut state = [0_u64; 25];
    let mut blocks = input.chunks_exact(RATE);
    for block in &mut blocks {
        absorb(&mut state, block);
        permute(&mut state);
    }
    let remainder = blocks.remainder();
    let mut last = [0_u8; RATE];
    last[..remainder.len()].copy_from_slice(remainder);
    last[remainder.len()] = 0x01;
    last[RATE - 1] |= 0x80;
    absorb(&mut state, &last);
    permute(&mut state);
    let mut output = [0_u8; 32];
    for (bytes, lane) in output.chunks_exact_mut(8).zip(state) {
        bytes.copy_from_slice(&lane.to_le_bytes());
    }
    output
}

fn absorb(state: &mut [u64; 25], block: &[u8]) {
    for (lane, bytes) in state.iter_mut().zip(block.chunks_exact(8)) {
        *lane ^= u64::from_le_bytes(bytes.try_into().expect("eight-byte lane"));
    }
}

fn permute(state: &mut [u64; 25]) {
    const ROUND_CONSTANTS: [u64; 24] = [
        0x0000000000000001,
        0x0000000000008082,
        0x800000000000808a,
        0x8000000080008000,
        0x000000000000808b,
        0x0000000080000001,
        0x8000000080008081,
        0x8000000000008009,
        0x000000000000008a,
        0x0000000000000088,
        0x0000000080008009,
        0x000000008000000a,
        0x000000008000808b,
        0x800000000000008b,
        0x8000000000008089,
        0x8000000000008003,
        0x8000000000008002,
        0x8000000000000080,
        0x000000000000800a,
        0x800000008000000a,
        0x8000000080008081,
        0x8000000000008080,
        0x0000000080000001,
        0x8000000080008008,
    ];
    const ROTATIONS: [[u32; 5]; 5] = [
        [0, 36, 3, 41, 18],
        [1, 44, 10, 45, 2],
        [62, 6, 43, 15, 61],
        [28, 55, 25, 21, 56],
        [27, 20, 39, 8, 14],
    ];
    for constant in ROUND_CONSTANTS {
        let mut columns = [0; 5];
        for x in 0..5 {
            for y in 0..5 {
                columns[x] ^= state[x + 5 * y];
            }
        }
        for x in 0..5 {
            let delta = columns[(x + 4) % 5] ^ columns[(x + 1) % 5].rotate_left(1);
            for y in 0..5 {
                state[x + 5 * y] ^= delta;
            }
        }
        let mut rotated = [0; 25];
        for x in 0..5 {
            for y in 0..5 {
                rotated[y + 5 * ((2 * x + 3 * y) % 5)] =
                    state[x + 5 * y].rotate_left(ROTATIONS[x][y]);
            }
        }
        for x in 0..5 {
            for y in 0..5 {
                state[x + 5 * y] = rotated[x + 5 * y]
                    ^ (!rotated[(x + 1) % 5 + 5 * y] & rotated[(x + 2) % 5 + 5 * y]);
            }
        }
        state[0] ^= constant;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use blockscout_display_bytes::ToHex;

    #[test]
    fn keccak256_standard_vectors() {
        assert_eq!(
            keccak256(b"").to_hex(),
            "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
        );
        assert_eq!(
            keccak256(b"abc").to_hex(),
            "0x4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"
        );
        // The library token is also asserted by the live source-files test.
        assert_eq!(
            &keccak256(b"contracts/Example.sol:Lib").to_hex()[2..36],
            "50698f9fab9190debff1c0247749d3c3d0"
        );
    }
}
