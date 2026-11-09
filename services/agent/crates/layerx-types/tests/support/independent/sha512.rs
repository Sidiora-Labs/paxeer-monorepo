// From ed25519-compact 2.1.1 (MIT); see PROVENANCE.md.
//! A small, self-contained SHA512 implementation
//! (C) Frank Denis, public domain

fn load_be(base: &[u8], offset: usize) -> u64 {
    let addr = &base[offset..];
    u64::from(addr[7])
        | u64::from(addr[6]) << 8
        | u64::from(addr[5]) << 16
        | u64::from(addr[4]) << 24
        | u64::from(addr[3]) << 32
        | u64::from(addr[2]) << 40
        | u64::from(addr[1]) << 48
        | u64::from(addr[0]) << 56
}

fn store_be(base: &mut [u8], offset: usize, horizontal: u64) {
    base[offset..offset + 8].copy_from_slice(&horizontal.to_be_bytes());
}

struct W([u64; 16]);

#[derive(Copy, Clone)]
struct State([u64; 8]);

impl W {
    fn new(input: &[u8]) -> Self {
        let mut words = [0u64; 16];
        for (index, element) in words.iter_mut().enumerate() {
            *element = load_be(input, index * 8);
        }
        W(words)
    }

    fn choose(horizontal: u64, vertical: u64, projective: u64) -> u64 {
        (horizontal & vertical) ^ (!horizontal & projective)
    }

    fn majority(horizontal: u64, vertical: u64, projective: u64) -> u64 {
        (horizontal & vertical) ^ (horizontal & projective) ^ (vertical & projective)
    }

    fn big_sigma_zero(horizontal: u64) -> u64 {
        horizontal.rotate_right(28) ^ horizontal.rotate_right(34) ^ horizontal.rotate_right(39)
    }

    fn big_sigma_one(horizontal: u64) -> u64 {
        horizontal.rotate_right(14) ^ horizontal.rotate_right(18) ^ horizontal.rotate_right(41)
    }

    fn small_sigma_zero(horizontal: u64) -> u64 {
        horizontal.rotate_right(1) ^ horizontal.rotate_right(8) ^ (horizontal >> 7)
    }

    fn small_sigma_one(horizontal: u64) -> u64 {
        horizontal.rotate_right(19) ^ horizontal.rotate_right(61) ^ (horizontal >> 6)
    }

    fn mix(&mut self, first: usize, second: usize, third: usize, fourth: usize) {
        let words = &mut self.0;
        words[first] = words[first]
            .wrapping_add(Self::small_sigma_one(words[second]))
            .wrapping_add(words[third])
            .wrapping_add(Self::small_sigma_zero(words[fourth]));
    }

    fn expand(&mut self) {
        self.mix(0, 14, 9, 1);
        self.mix(1, 15, 10, 2);
        self.mix(2, 0, 11, 3);
        self.mix(3, 1, 12, 4);
        self.mix(4, 2, 13, 5);
        self.mix(5, 3, 14, 6);
        self.mix(6, 4, 15, 7);
        self.mix(7, 5, 0, 8);
        self.mix(8, 6, 1, 9);
        self.mix(9, 7, 2, 10);
        self.mix(10, 8, 3, 11);
        self.mix(11, 9, 4, 12);
        self.mix(12, 10, 5, 13);
        self.mix(13, 11, 6, 14);
        self.mix(14, 12, 7, 15);
        self.mix(15, 13, 8, 0);
    }

    fn round(&mut self, state: &mut State, index: usize, carry: u64) {
        let temporary = &mut state.0;
        temporary[(16 - index + 7) & 7] = temporary[(16 - index + 7) & 7]
            .wrapping_add(Self::big_sigma_one(temporary[(16 - index + 4) & 7]))
            .wrapping_add(Self::choose(
                temporary[(16 - index + 4) & 7],
                temporary[(16 - index + 5) & 7],
                temporary[(16 - index + 6) & 7],
            ))
            .wrapping_add(carry)
            .wrapping_add(self.0[index]);
        temporary[(16 - index + 3) & 7] =
            temporary[(16 - index + 3) & 7].wrapping_add(temporary[(16 - index + 7) & 7]);
        temporary[(16 - index + 7) & 7] = temporary[(16 - index + 7) & 7]
            .wrapping_add(Self::big_sigma_zero(temporary[(16 - index) & 7]))
            .wrapping_add(Self::majority(
                temporary[(16 - index) & 7],
                temporary[(16 - index + 1) & 7],
                temporary[(16 - index + 2) & 7],
            ));
    }

    fn round_group(&mut self, state: &mut State, bytes: usize) {
        const ROUND_CONSTANTS: [u64; 80] = [
            0x428a_2f98_d728_ae22,
            0x7137_4491_23ef_65cd,
            0xb5c0_fbcf_ec4d_3b2f,
            0xe9b5_dba5_8189_dbbc,
            0x3956_c25b_f348_b538,
            0x59f1_11f1_b605_d019,
            0x923f_82a4_af19_4f9b,
            0xab1c_5ed5_da6d_8118,
            0xd807_aa98_a303_0242,
            0x1283_5b01_4570_6fbe,
            0x2431_85be_4ee4_b28c,
            0x550c_7dc3_d5ff_b4e2,
            0x72be_5d74_f27b_896f,
            0x80de_b1fe_3b16_96b1,
            0x9bdc_06a7_25c7_1235,
            0xc19b_f174_cf69_2694,
            0xe49b_69c1_9ef1_4ad2,
            0xefbe_4786_384f_25e3,
            0x0fc1_9dc6_8b8c_d5b5,
            0x240c_a1cc_77ac_9c65,
            0x2de9_2c6f_592b_0275,
            0x4a74_84aa_6ea6_e483,
            0x5cb0_a9dc_bd41_fbd4,
            0x76f9_88da_8311_53b5,
            0x983e_5152_ee66_dfab,
            0xa831_c66d_2db4_3210,
            0xb003_27c8_98fb_213f,
            0xbf59_7fc7_beef_0ee4,
            0xc6e0_0bf3_3da8_8fc2,
            0xd5a7_9147_930a_a725,
            0x06ca_6351_e003_826f,
            0x1429_2967_0a0e_6e70,
            0x27b7_0a85_46d2_2ffc,
            0x2e1b_2138_5c26_c926,
            0x4d2c_6dfc_5ac4_2aed,
            0x5338_0d13_9d95_b3df,
            0x650a_7354_8baf_63de,
            0x766a_0abb_3c77_b2a8,
            0x81c2_c92e_47ed_aee6,
            0x9272_2c85_1482_353b,
            0xa2bf_e8a1_4cf1_0364,
            0xa81a_664b_bc42_3001,
            0xc24b_8b70_d0f8_9791,
            0xc76c_51a3_0654_be30,
            0xd192_e819_d6ef_5218,
            0xd699_0624_5565_a910,
            0xf40e_3585_5771_202a,
            0x106a_a070_32bb_d1b8,
            0x19a4_c116_b8d2_d0c8,
            0x1e37_6c08_5141_ab53,
            0x2748_774c_df8e_eb99,
            0x34b0_bcb5_e19b_48a8,
            0x391c_0cb3_c5c9_5a63,
            0x4ed8_aa4a_e341_8acb,
            0x5b9c_ca4f_7763_e373,
            0x682e_6ff3_d6b2_b8a3,
            0x748f_82ee_5def_b2fc,
            0x78a5_636f_4317_2f60,
            0x84c8_7814_a1f0_ab72,
            0x8cc7_0208_1a64_39ec,
            0x90be_fffa_2363_1e28,
            0xa450_6ceb_de82_bde9,
            0xbef9_a3f7_b2c6_7915,
            0xc671_78f2_e372_532b,
            0xca27_3ece_ea26_619c,
            0xd186_b8c7_21c0_c207,
            0xeada_7dd6_cde0_eb1e,
            0xf57d_4f7f_ee6e_d178,
            0x06f0_67aa_7217_6fba,
            0x0a63_7dc5_a2c8_98a6,
            0x113f_9804_bef9_0dae,
            0x1b71_0b35_131c_471b,
            0x28db_77f5_2304_7d84,
            0x32ca_ab7b_40c7_2493,
            0x3c9e_be0a_15c9_bebc,
            0x431d_67c4_9c10_0d4c,
            0x4cc5_d4be_cb3e_42b6,
            0x597f_299c_fc65_7e2a,
            0x5fcb_6fab_3ad6_faec,
            0x6c44_198c_4a47_5817,
        ];
        let rc = &ROUND_CONSTANTS[bytes * 16..];
        self.round(state, 0, rc[0]);
        self.round(state, 1, rc[1]);
        self.round(state, 2, rc[2]);
        self.round(state, 3, rc[3]);
        self.round(state, 4, rc[4]);
        self.round(state, 5, rc[5]);
        self.round(state, 6, rc[6]);
        self.round(state, 7, rc[7]);
        self.round(state, 8, rc[8]);
        self.round(state, 9, rc[9]);
        self.round(state, 10, rc[10]);
        self.round(state, 11, rc[11]);
        self.round(state, 12, rc[12]);
        self.round(state, 13, rc[13]);
        self.round(state, 14, rc[14]);
        self.round(state, 15, rc[15]);
    }
}

impl State {
    fn new() -> Self {
        const IV: [u8; 64] = [
            0x6a, 0x09, 0xe6, 0x67, 0xf3, 0xbc, 0xc9, 0x08, 0xbb, 0x67, 0xae, 0x85, 0x84, 0xca,
            0xa7, 0x3b, 0x3c, 0x6e, 0xf3, 0x72, 0xfe, 0x94, 0xf8, 0x2b, 0xa5, 0x4f, 0xf5, 0x3a,
            0x5f, 0x1d, 0x36, 0xf1, 0x51, 0x0e, 0x52, 0x7f, 0xad, 0xe6, 0x82, 0xd1, 0x9b, 0x05,
            0x68, 0x8c, 0x2b, 0x3e, 0x6c, 0x1f, 0x1f, 0x83, 0xd9, 0xab, 0xfb, 0x41, 0xbd, 0x6b,
            0x5b, 0xe0, 0xcd, 0x19, 0x13, 0x7e, 0x21, 0x79,
        ];
        let mut temporary = [0u64; 8];
        for (index, element) in temporary.iter_mut().enumerate() {
            *element = load_be(&IV, index * 8);
        }
        State(temporary)
    }

    fn add(&mut self, horizontal: &State) {
        let sx = &mut self.0;
        let ex = &horizontal.0;
        sx[0] = sx[0].wrapping_add(ex[0]);
        sx[1] = sx[1].wrapping_add(ex[1]);
        sx[2] = sx[2].wrapping_add(ex[2]);
        sx[3] = sx[3].wrapping_add(ex[3]);
        sx[4] = sx[4].wrapping_add(ex[4]);
        sx[5] = sx[5].wrapping_add(ex[5]);
        sx[6] = sx[6].wrapping_add(ex[6]);
        sx[7] = sx[7].wrapping_add(ex[7]);
    }

    fn store(&self, out: &mut [u8]) {
        for (index, &element) in self.0.iter().enumerate() {
            store_be(out, index * 8, element);
        }
    }

    fn blocks(&mut self, mut input: &[u8]) -> usize {
        let mut temporary = *self;
        let mut inlen = input.len();
        while inlen >= 128 {
            let mut words = W::new(input);
            words.round_group(&mut temporary, 0);
            words.expand();
            words.round_group(&mut temporary, 1);
            words.expand();
            words.round_group(&mut temporary, 2);
            words.expand();
            words.round_group(&mut temporary, 3);
            words.expand();
            words.round_group(&mut temporary, 4);
            temporary.add(self);
            self.0 = temporary.0;
            input = &input[128..];
            inlen -= 128;
        }
        inlen
    }
}

#[derive(Copy, Clone)]
pub struct Hash {
    state: State,
    w: [u8; 128],
    r: usize,
    len: usize,
}

impl Hash {
    pub fn new() -> Self {
        Hash {
            state: State::new(),
            r: 0,
            w: [0u8; 128],
            len: 0,
        }
    }

    /// Absorb content
    pub fn update<T: AsRef<[u8]>>(&mut self, input: T) {
        let input = input.as_ref();
        let mut count = input.len();
        self.len += count;
        let av = 128 - self.r;
        let tc = ::core::cmp::min(count, av);
        self.w[self.r..self.r + tc].copy_from_slice(&input[0..tc]);
        self.r += tc;
        count -= tc;
        let pos = tc;
        if self.r == 128 {
            self.state.blocks(&self.w);
            self.r = 0;
        }
        if self.r == 0 && count > 0 {
            let rb = self.state.blocks(&input[pos..]);
            if rb > 0 {
                self.w[..rb].copy_from_slice(&input[pos + count - rb..]);
                self.r = rb;
            }
        }
    }

    /// Compute SHA512(absorbed content)
    pub fn finalize(mut self) -> [u8; 64] {
        let mut padded = [0u8; 256];
        padded[..self.r].copy_from_slice(&self.w[..self.r]);
        padded[self.r] = 0x80;
        let result = if self.r < 112 { 128 } else { 256 };
        let bits = self.len * 8;
        padded[result - 8..result].copy_from_slice(
            &u64::try_from(bits)
                .unwrap_or_else(|error| panic!("SHA512 bit length: {error}"))
                .to_be_bytes(),
        );
        self.state.blocks(&padded[..result]);
        let mut out = [0u8; 64];
        self.state.store(&mut out);
        out
    }
}

impl Default for Hash {
    fn default() -> Self {
        Self::new()
    }
}
