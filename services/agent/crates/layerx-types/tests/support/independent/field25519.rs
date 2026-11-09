// From ed25519-compact 2.1.1 (MIT); see PROVENANCE.md.

use core::cmp::{Eq, PartialEq};
use core::ops::{Add, Mul, Sub};

fn low_limb(value: u128) -> u64 {
    #[allow(
        clippy::cast_possible_truncation,
        reason = "Fiat arithmetic extracts the low 64 bits after its limb masks and carry shifts."
    )]
    let low = value as u64;
    low
}

pub type FiatU1 = u8;
pub type FiatI1 = i8;

pub fn fiat_25519_addcarryx_u51(
    out1: &mut u64,
    out2: &mut FiatU1,
    arg1: FiatU1,
    arg2: u64,
    arg3: u64,
) {
    let x1: u64 = u64::from(arg1).wrapping_add(arg2).wrapping_add(arg3);
    let x2: u64 = x1 & 0x7_ffff_ffff_ffff;
    let x3: FiatU1 = {
        #[allow(
            clippy::cast_possible_truncation,
            reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
        )]
        let converted_bits = (x1 >> 51) as u8;
        converted_bits
    };
    *out1 = x2;
    *out2 = x3;
}

pub fn fiat_25519_subborrowx_u51(
    out1: &mut u64,
    out2: &mut FiatU1,
    arg1: FiatU1,
    arg2: u64,
    arg3: u64,
) {
    let x1: i64 = {
        #[allow(clippy::cast_possible_truncation, reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds.")] let converted_bits = i128::from({ #[allow(clippy::cast_possible_truncation, reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds.")] let converted_bits = i128::from(arg2).wrapping_sub(i128::from(arg1)) as i64; converted_bits })
        .wrapping_sub(i128::from(arg3)) as i64;
        converted_bits
    };
    let x2: FiatI1 = {
        #[allow(
            clippy::cast_possible_truncation,
            reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
        )]
        let converted_bits = (x1 >> 51) as i8;
        converted_bits
    };
    let x3: u64 = {
        #[allow(
            clippy::cast_possible_truncation,
            clippy::cast_sign_loss,
            reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
        )]
        let converted_bits = (i128::from(x1) & 0x7_ffff_ffff_ffff_i128) as u64;
        converted_bits
    };
    *out1 = x3;
    *out2 = {
        #[allow(
            clippy::cast_sign_loss,
            reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
        )]
        let converted_bits = 0x0_i8.wrapping_sub(x2) as u8;
        converted_bits
    };
}

pub fn fiat_25519_cmovznz_u64(out1: &mut u64, arg1: FiatU1, arg2: u64, arg3: u64) {
    let x1: FiatU1 = arg1;
    let x2: u64 = {
        #[allow(clippy::cast_possible_truncation, clippy::cast_sign_loss, reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds.")] let converted_bits = (i128::from(0x0_i8.wrapping_sub({ #[allow(clippy::cast_possible_wrap, reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds.")] let converted_bits = x1 as i8; converted_bits }))
        & 0xffff_ffff_ffff_ffff_i128) as u64;
        converted_bits
    };
    let x3: u64 = (x2 & arg3) | ((!x2) & arg2);
    *out1 = x3;
}

pub fn fiat_25519_carry_mul(out1: &mut [u64; 5], arg1: &[u64; 5], arg2: &[u64; 5]) {
    let x1: u128 = u128::from(arg1[4]).wrapping_mul(u128::from(arg2[4].wrapping_mul(0x13)));
    let x2: u128 = u128::from(arg1[4]).wrapping_mul(u128::from(arg2[3].wrapping_mul(0x13)));
    let x3: u128 = u128::from(arg1[4]).wrapping_mul(u128::from(arg2[2].wrapping_mul(0x13)));
    let x4: u128 = u128::from(arg1[4]).wrapping_mul(u128::from(arg2[1].wrapping_mul(0x13)));
    let x5: u128 = u128::from(arg1[3]).wrapping_mul(u128::from(arg2[4].wrapping_mul(0x13)));
    let x6: u128 = u128::from(arg1[3]).wrapping_mul(u128::from(arg2[3].wrapping_mul(0x13)));
    let x7: u128 = u128::from(arg1[3]).wrapping_mul(u128::from(arg2[2].wrapping_mul(0x13)));
    let x8: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(arg2[4].wrapping_mul(0x13)));
    let x9: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(arg2[3].wrapping_mul(0x13)));
    let x10: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(arg2[4].wrapping_mul(0x13)));
    let x11: u128 = u128::from(arg1[4]).wrapping_mul(u128::from(arg2[0]));
    let x12: u128 = u128::from(arg1[3]).wrapping_mul(u128::from(arg2[1]));
    let x13: u128 = u128::from(arg1[3]).wrapping_mul(u128::from(arg2[0]));
    let x14: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(arg2[2]));
    let x15: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(arg2[1]));
    let x16: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(arg2[0]));
    let x17: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(arg2[3]));
    let x18: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(arg2[2]));
    let x19: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(arg2[1]));
    let x20: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(arg2[0]));
    let x21: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(arg2[4]));
    let x22: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(arg2[3]));
    let x23: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(arg2[2]));
    let x24: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(arg2[1]));
    let x25: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(arg2[0]));
    let x26: u128 = x25.wrapping_add(x10.wrapping_add(x9.wrapping_add(x7.wrapping_add(x4))));
    let x27: u64 = low_limb(x26 >> 51);
    let x28: u64 = low_limb(x26 & 0x7_ffff_ffff_ffff_u128);
    let x29: u128 = x21.wrapping_add(x17.wrapping_add(x14.wrapping_add(x12.wrapping_add(x11))));
    let x30: u128 = x22.wrapping_add(x18.wrapping_add(x15.wrapping_add(x13.wrapping_add(x1))));
    let x31: u128 = x23.wrapping_add(x19.wrapping_add(x16.wrapping_add(x5.wrapping_add(x2))));
    let x32: u128 = x24.wrapping_add(x20.wrapping_add(x8.wrapping_add(x6.wrapping_add(x3))));
    let x33: u128 = u128::from(x27).wrapping_add(x32);
    let x34: u64 = low_limb(x33 >> 51);
    let x35: u64 = low_limb(x33 & 0x7_ffff_ffff_ffff_u128);
    let x36: u128 = u128::from(x34).wrapping_add(x31);
    let x37: u64 = low_limb(x36 >> 51);
    let x38: u64 = low_limb(x36 & 0x7_ffff_ffff_ffff_u128);
    let x39: u128 = u128::from(x37).wrapping_add(x30);
    let x40: u64 = low_limb(x39 >> 51);
    let x41: u64 = low_limb(x39 & 0x7_ffff_ffff_ffff_u128);
    let x42: u128 = u128::from(x40).wrapping_add(x29);
    let x43: u64 = low_limb(x42 >> 51);
    let x44: u64 = low_limb(x42 & 0x7_ffff_ffff_ffff_u128);
    let x45: u64 = x43.wrapping_mul(0x13);
    let x46: u64 = x28.wrapping_add(x45);
    let x47: u64 = x46 >> 51;
    let x48: u64 = x46 & 0x7_ffff_ffff_ffff;
    let x49: u64 = x47.wrapping_add(x35);
    let x50: FiatU1 = (x49 >> 51).to_le_bytes()[0];
    let x51: u64 = x49 & 0x7_ffff_ffff_ffff;
    let x52: u64 = u64::from(x50).wrapping_add(x38);
    out1[0] = x48;
    out1[1] = x51;
    out1[2] = x52;
    out1[3] = x41;
    out1[4] = x44;
}

pub fn fiat_25519_carry_square(out1: &mut [u64; 5], arg1: &[u64; 5]) {
    let x1: u64 = arg1[4].wrapping_mul(0x13);
    let x2: u64 = x1.wrapping_mul(0x2);
    let x3: u64 = arg1[4].wrapping_mul(0x2);
    let x4: u64 = arg1[3].wrapping_mul(0x13);
    let x5: u64 = x4.wrapping_mul(0x2);
    let x6: u64 = arg1[3].wrapping_mul(0x2);
    let x7: u64 = arg1[2].wrapping_mul(0x2);
    let x8: u64 = arg1[1].wrapping_mul(0x2);
    let x9: u128 = u128::from(arg1[4]).wrapping_mul(u128::from(x1));
    let x10: u128 = u128::from(arg1[3]).wrapping_mul(u128::from(x2));
    let x11: u128 = u128::from(arg1[3]).wrapping_mul(u128::from(x4));
    let x12: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(x2));
    let x13: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(x5));
    let x14: u128 = u128::from(arg1[2]).wrapping_mul(u128::from(arg1[2]));
    let x15: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(x2));
    let x16: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(x6));
    let x17: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(x7));
    let x18: u128 = u128::from(arg1[1]).wrapping_mul(u128::from(arg1[1]));
    let x19: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(x3));
    let x20: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(x6));
    let x21: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(x7));
    let x22: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(x8));
    let x23: u128 = u128::from(arg1[0]).wrapping_mul(u128::from(arg1[0]));
    let x24: u128 = x23.wrapping_add(x15.wrapping_add(x13));
    let x25: u64 = low_limb(x24 >> 51);
    let x26: u64 = low_limb(x24 & 0x7_ffff_ffff_ffff_u128);
    let x27: u128 = x19.wrapping_add(x16.wrapping_add(x14));
    let x28: u128 = x20.wrapping_add(x17.wrapping_add(x9));
    let x29: u128 = x21.wrapping_add(x18.wrapping_add(x10));
    let x30: u128 = x22.wrapping_add(x12.wrapping_add(x11));
    let x31: u128 = u128::from(x25).wrapping_add(x30);
    let x32: u64 = low_limb(x31 >> 51);
    let x33: u64 = low_limb(x31 & 0x7_ffff_ffff_ffff_u128);
    let x34: u128 = u128::from(x32).wrapping_add(x29);
    let x35: u64 = low_limb(x34 >> 51);
    let x36: u64 = low_limb(x34 & 0x7_ffff_ffff_ffff_u128);
    let x37: u128 = u128::from(x35).wrapping_add(x28);
    let x38: u64 = low_limb(x37 >> 51);
    let x39: u64 = low_limb(x37 & 0x7_ffff_ffff_ffff_u128);
    let x40: u128 = u128::from(x38).wrapping_add(x27);
    let x41: u64 = low_limb(x40 >> 51);
    let x42: u64 = low_limb(x40 & 0x7_ffff_ffff_ffff_u128);
    let x43: u64 = x41.wrapping_mul(0x13);
    let x44: u64 = x26.wrapping_add(x43);
    let x45: u64 = x44 >> 51;
    let x46: u64 = x44 & 0x7_ffff_ffff_ffff;
    let x47: u64 = x45.wrapping_add(x33);
    let x48: FiatU1 = (x47 >> 51).to_le_bytes()[0];
    let x49: u64 = x47 & 0x7_ffff_ffff_ffff;
    let x50: u64 = u64::from(x48).wrapping_add(x36);
    out1[0] = x46;
    out1[1] = x49;
    out1[2] = x50;
    out1[3] = x39;
    out1[4] = x42;
}

pub fn fiat_25519_carry(out1: &mut [u64; 5], arg1: &[u64; 5]) {
    let x1: u64 = arg1[0];
    let x2: u64 = (x1 >> 51).wrapping_add(arg1[1]);
    let x3: u64 = (x2 >> 51).wrapping_add(arg1[2]);
    let x4: u64 = (x3 >> 51).wrapping_add(arg1[3]);
    let x5: u64 = (x4 >> 51).wrapping_add(arg1[4]);
    let x6: u64 = (x1 & 0x7_ffff_ffff_ffff).wrapping_add((x5 >> 51).wrapping_mul(0x13));
    let x7: u64 = u64::from({ #[allow(clippy::cast_possible_truncation, reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds.")] let converted_bits = (x6 >> 51) as u8; converted_bits }).wrapping_add(x2 & 0x7_ffff_ffff_ffff);
    let x8: u64 = x6 & 0x7_ffff_ffff_ffff;
    let x9: u64 = x7 & 0x7_ffff_ffff_ffff;
    let x10: u64 = u64::from({ #[allow(clippy::cast_possible_truncation, reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds.")] let converted_bits = (x7 >> 51) as u8; converted_bits }).wrapping_add(x3 & 0x7_ffff_ffff_ffff);
    let x11: u64 = x4 & 0x7_ffff_ffff_ffff;
    let x12: u64 = x5 & 0x7_ffff_ffff_ffff;
    out1[0] = x8;
    out1[1] = x9;
    out1[2] = x10;
    out1[3] = x11;
    out1[4] = x12;
}

pub fn fiat_25519_add(out1: &mut [u64; 5], arg1: &[u64; 5], arg2: &[u64; 5]) {
    let x1: u64 = arg1[0].wrapping_add(arg2[0]);
    let x2: u64 = arg1[1].wrapping_add(arg2[1]);
    let x3: u64 = arg1[2].wrapping_add(arg2[2]);
    let x4: u64 = arg1[3].wrapping_add(arg2[3]);
    let x5: u64 = arg1[4].wrapping_add(arg2[4]);
    out1[0] = x1;
    out1[1] = x2;
    out1[2] = x3;
    out1[3] = x4;
    out1[4] = x5;
}

pub fn fiat_25519_sub(out1: &mut [u64; 5], arg1: &[u64; 5], arg2: &[u64; 5]) {
    let x1: u64 = 0xf_ffff_ffff_ffda_u64
        .wrapping_add(arg1[0])
        .wrapping_sub(arg2[0]);
    let x2: u64 = 0xf_ffff_ffff_fffe_u64
        .wrapping_add(arg1[1])
        .wrapping_sub(arg2[1]);
    let x3: u64 = 0xf_ffff_ffff_fffe_u64
        .wrapping_add(arg1[2])
        .wrapping_sub(arg2[2]);
    let x4: u64 = 0xf_ffff_ffff_fffe_u64
        .wrapping_add(arg1[3])
        .wrapping_sub(arg2[3]);
    let x5: u64 = 0xf_ffff_ffff_fffe_u64
        .wrapping_add(arg1[4])
        .wrapping_sub(arg2[4]);
    out1[0] = x1;
    out1[1] = x2;
    out1[2] = x3;
    out1[3] = x4;
    out1[4] = x5;
}

pub fn fiat_25519_opp(out1: &mut [u64; 5], arg1: &[u64; 5]) {
    let x1: u64 = 0xf_ffff_ffff_ffda_u64.wrapping_sub(arg1[0]);
    let x2: u64 = 0xf_ffff_ffff_fffe_u64.wrapping_sub(arg1[1]);
    let x3: u64 = 0xf_ffff_ffff_fffe_u64.wrapping_sub(arg1[2]);
    let x4: u64 = 0xf_ffff_ffff_fffe_u64.wrapping_sub(arg1[3]);
    let x5: u64 = 0xf_ffff_ffff_fffe_u64.wrapping_sub(arg1[4]);
    out1[0] = x1;
    out1[1] = x2;
    out1[2] = x3;
    out1[3] = x4;
    out1[4] = x5;
}

fn canonical_field_limbs(arg1: &[u64; 5]) -> [u64; 5] {
    let mut x1: u64 = 0;
    let mut x2: FiatU1 = 0;
    fiat_25519_subborrowx_u51(&mut x1, &mut x2, 0x0, arg1[0], 0x7_ffff_ffff_ffed);
    let mut x3: u64 = 0;
    let mut x4: FiatU1 = 0;
    fiat_25519_subborrowx_u51(&mut x3, &mut x4, x2, arg1[1], 0x7_ffff_ffff_ffff);
    let mut x5: u64 = 0;
    let mut x6: FiatU1 = 0;
    fiat_25519_subborrowx_u51(&mut x5, &mut x6, x4, arg1[2], 0x7_ffff_ffff_ffff);
    let mut x7: u64 = 0;
    let mut x8: FiatU1 = 0;
    fiat_25519_subborrowx_u51(&mut x7, &mut x8, x6, arg1[3], 0x7_ffff_ffff_ffff);
    let mut x9: u64 = 0;
    let mut x10: FiatU1 = 0;
    fiat_25519_subborrowx_u51(&mut x9, &mut x10, x8, arg1[4], 0x7_ffff_ffff_ffff);
    let mut x11: u64 = 0;
    fiat_25519_cmovznz_u64(&mut x11, x10, 0x0_u64, 0xffff_ffff_ffff_ffff);
    let mut x12: u64 = 0;
    let mut x13: FiatU1 = 0;
    fiat_25519_addcarryx_u51(&mut x12, &mut x13, 0x0, x1, x11 & 0x7_ffff_ffff_ffed);
    let mut x14: u64 = 0;
    let mut x15: FiatU1 = 0;
    fiat_25519_addcarryx_u51(&mut x14, &mut x15, x13, x3, x11 & 0x7_ffff_ffff_ffff);
    let mut x16: u64 = 0;
    let mut x17: FiatU1 = 0;
    fiat_25519_addcarryx_u51(&mut x16, &mut x17, x15, x5, x11 & 0x7_ffff_ffff_ffff);
    let mut x18: u64 = 0;
    let mut x19: FiatU1 = 0;
    fiat_25519_addcarryx_u51(&mut x18, &mut x19, x17, x7, x11 & 0x7_ffff_ffff_ffff);
    let mut x20: u64 = 0;
    let mut x21: FiatU1 = 0;
    fiat_25519_addcarryx_u51(&mut x20, &mut x21, x19, x9, x11 & 0x7_ffff_ffff_ffff);
    [x12, x14, x16, x18, x20]
}

pub fn fiat_25519_to_bytes(out1: &mut [u8; 32], arg1: &[u64; 5]) {
    let [x12, x14, x16, x18, x20] = canonical_field_limbs(arg1);
    let x22: u64 = x20 << 4;
    let x23: u64 = x18.wrapping_mul(0x2_u64);
    let x24: u64 = x16 << 6;
    let x25: u64 = x14 << 3;
    let x26: u8 = (x12 & 0xff_u64).to_le_bytes()[0];
    let x27: u64 = x12 >> 8;
    let x28: u8 = (x27 & 0xff_u64).to_le_bytes()[0];
    let x29: u64 = x27 >> 8;
    let x30: u8 = (x29 & 0xff_u64).to_le_bytes()[0];
    let x31: u64 = x29 >> 8;
    let x32: u8 = (x31 & 0xff_u64).to_le_bytes()[0];
    let x33: u64 = x31 >> 8;
    let x34: u8 = (x33 & 0xff_u64).to_le_bytes()[0];
    let x35: u64 = x33 >> 8;
    let x36: u8 = (x35 & 0xff_u64).to_le_bytes()[0];
    let x37: u8 = (x35 >> 8).to_le_bytes()[0];
    let x38: u64 = x25.wrapping_add(u64::from(x37));
    let x39: u8 = (x38 & 0xff_u64).to_le_bytes()[0];
    let x40: u64 = x38 >> 8;
    let x41: u8 = (x40 & 0xff_u64).to_le_bytes()[0];
    let x42: u64 = x40 >> 8;
    let x43: u8 = (x42 & 0xff_u64).to_le_bytes()[0];
    let x44: u64 = x42 >> 8;
    let x45: u8 = (x44 & 0xff_u64).to_le_bytes()[0];
    let x46: u64 = x44 >> 8;
    let x47: u8 = (x46 & 0xff_u64).to_le_bytes()[0];
    let x48: u64 = x46 >> 8;
    let x49: u8 = (x48 & 0xff_u64).to_le_bytes()[0];
    let x50: u8 = (x48 >> 8).to_le_bytes()[0];
    let x51: u64 = x24.wrapping_add(u64::from(x50));
    let x52: u8 = (x51 & 0xff_u64).to_le_bytes()[0];
    let x53: u64 = x51 >> 8;
    let x54: u8 = (x53 & 0xff_u64).to_le_bytes()[0];
    let x55: u64 = x53 >> 8;
    let x56: u8 = (x55 & 0xff_u64).to_le_bytes()[0];
    let x57: u64 = x55 >> 8;
    let x58: u8 = (x57 & 0xff_u64).to_le_bytes()[0];
    let x59: u64 = x57 >> 8;
    let x60: u8 = (x59 & 0xff_u64).to_le_bytes()[0];
    let x61: u64 = x59 >> 8;
    let x62: u8 = (x61 & 0xff_u64).to_le_bytes()[0];
    let x63: u64 = x61 >> 8;
    let x64: u8 = (x63 & 0xff_u64).to_le_bytes()[0];
    let x65: FiatU1 = (x63 >> 8).to_le_bytes()[0];
    let x66: u64 = x23.wrapping_add(u64::from(x65));
    let x67: u8 = (x66 & 0xff_u64).to_le_bytes()[0];
    let x68: u64 = x66 >> 8;
    let x69: u8 = (x68 & 0xff_u64).to_le_bytes()[0];
    let x70: u64 = x68 >> 8;
    let x71: u8 = (x70 & 0xff_u64).to_le_bytes()[0];
    let x72: u64 = x70 >> 8;
    let x73: u8 = (x72 & 0xff_u64).to_le_bytes()[0];
    let x74: u64 = x72 >> 8;
    let x75: u8 = (x74 & 0xff_u64).to_le_bytes()[0];
    let x76: u64 = x74 >> 8;
    let x77: u8 = (x76 & 0xff_u64).to_le_bytes()[0];
    let x78: u8 = (x76 >> 8).to_le_bytes()[0];
    let x79: u64 = x22.wrapping_add(u64::from(x78));
    let x80: u8 = (x79 & 0xff_u64).to_le_bytes()[0];
    let x81: u64 = x79 >> 8;
    let x82: u8 = (x81 & 0xff_u64).to_le_bytes()[0];
    let x83: u64 = x81 >> 8;
    let x84: u8 = (x83 & 0xff_u64).to_le_bytes()[0];
    let x85: u64 = x83 >> 8;
    let x86: u8 = (x85 & 0xff_u64).to_le_bytes()[0];
    let x87: u64 = x85 >> 8;
    let x88: u8 = (x87 & 0xff_u64).to_le_bytes()[0];
    let x89: u64 = x87 >> 8;
    let x90: u8 = (x89 & 0xff_u64).to_le_bytes()[0];
    let x91: u8 = (x89 >> 8).to_le_bytes()[0];
    *out1 = [
        x26, x28, x30, x32, x34, x36, x39, x41, x43, x45, x47, x49, x52, x54, x56, x58, x60, x62,
        x64, x67, x69, x71, x73, x75, x77, x80, x82, x84, x86, x88, x90, x91,
    ];
}

#[derive(Clone, Default, Copy)]
pub struct Fe(pub [u64; 5]);

impl PartialEq for Fe {
    fn eq(&self, other: &Fe) -> bool {
        let &Fe(self_elems) = self;
        let &Fe(other_elems) = other;
        self_elems == other_elems
    }
}
impl Eq for Fe {}

pub static FE_ZERO: Fe = Fe([0, 0, 0, 0, 0]);
pub static FE_ONE: Fe = Fe([1, 0, 0, 0, 0]);
pub static FE_SQRTM1: Fe = Fe([
    1_718_705_420_411_056,
    234_908_883_556_509,
    2_233_514_472_574_048,
    2_117_202_627_021_982,
    765_476_049_583_133,
]);
pub(crate) static FE_D: Fe = Fe([
    929_955_233_495_203,
    466_365_720_129_213,
    1_662_059_464_998_953,
    2_033_849_074_728_123,
    1_442_794_654_840_575,
]);
pub(crate) static FE_D2: Fe = Fe([
    1_859_910_466_990_425,
    932_731_440_258_426,
    1_072_319_116_312_658,
    1_815_898_335_770_999,
    633_789_495_995_903,
]);

fn load_8u(bytes: &[u8]) -> u64 {
    u64::from(bytes[0])
        | (u64::from(bytes[1]) << 8)
        | (u64::from(bytes[2]) << 16)
        | (u64::from(bytes[3]) << 24)
        | (u64::from(bytes[4]) << 32)
        | (u64::from(bytes[5]) << 40)
        | (u64::from(bytes[6]) << 48)
        | (u64::from(bytes[7]) << 56)
}

pub fn load_4u(bytes: &[u8]) -> u64 {
    u64::from(bytes[0])
        | (u64::from(bytes[1]) << 8)
        | (u64::from(bytes[2]) << 16)
        | (u64::from(bytes[3]) << 24)
}

pub fn load_4i(bytes: &[u8]) -> i64 {
    {
        #[allow(
            clippy::cast_possible_wrap,
            reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
        )]
        let converted_bits = load_4u(bytes) as i64;
        converted_bits
    }
}

pub fn load_3u(bytes: &[u8]) -> u64 {
    u64::from(bytes[0]) | (u64::from(bytes[1]) << 8) | (u64::from(bytes[2]) << 16)
}

pub fn load_3i(bytes: &[u8]) -> i64 {
    {
        #[allow(
            clippy::cast_possible_wrap,
            reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
        )]
        let converted_bits = load_3u(bytes) as i64;
        converted_bits
    }
}

impl Add for Fe {
    type Output = Fe;

    fn add(self, rhs: Fe) -> Fe {
        let Fe(lhs) = self;
        let Fe(rhs) = rhs;
        let mut output = Fe::default();
        fiat_25519_add(&mut output.0, &lhs, &rhs);
        output
    }
}

impl Sub for Fe {
    type Output = Fe;

    fn sub(self, rhs: Fe) -> Fe {
        let Fe(lhs) = self;
        let Fe(rhs) = rhs;
        let mut output = Fe::default();
        fiat_25519_sub(&mut output.0, &lhs, &rhs);
        output.carry()
    }
}

impl Mul for Fe {
    type Output = Fe;

    fn mul(self, rhs: Fe) -> Fe {
        let Fe(lhs) = self;
        let Fe(rhs) = rhs;
        let mut output = Fe::default();
        fiat_25519_carry_mul(&mut output.0, &lhs, &rhs);
        output
    }
}

impl Fe {
    pub fn from_bytes(bytes: &[u8]) -> Fe {
        assert_eq!(bytes.len(), 32, "Invalid compressed length");
        let mut output = Fe::default();
        let mask = 0x7_ffff_ffff_ffff;
        output.0[0] = load_8u(&bytes[0..]) & mask;
        output.0[1] = (load_8u(&bytes[6..]) >> 3) & mask;
        output.0[2] = (load_8u(&bytes[12..]) >> 6) & mask;
        output.0[3] = (load_8u(&bytes[19..]) >> 1) & mask;
        output.0[4] = (load_8u(&bytes[24..]) >> 12) & mask;
        output
    }

    #[allow(
        clippy::wrong_self_convention,
        reason = "Retain the public reference verifier's borrowed field-encoding signature."
    )]
    pub fn to_bytes(&self) -> [u8; 32] {
        let Fe(es) = self.carry();
        let mut encoded_field = [0u8; 32];
        fiat_25519_to_bytes(&mut encoded_field, &es);
        encoded_field
    }

    pub fn carry(&self) -> Fe {
        let mut output = Fe::default();
        fiat_25519_carry(&mut output.0, &self.0);
        output
    }

    pub fn square(&self) -> Fe {
        let &Fe(lhs) = self;
        let mut output = Fe::default();
        fiat_25519_carry_square(&mut output.0, &lhs);
        output
    }

    pub fn square_and_double(&self) -> Fe {
        let output = self.square();
        output + output
    }

    pub fn invert(&self) -> Fe {
        let z1 = *self;
        let z2 = z1.square();
        let z8 = z2.square().square();
        let z9 = z1 * z8;
        let z11 = z2 * z9;
        let z22 = z11.square();
        let power5 = z9 * z22;
        let squared5 = (0..5).fold(power5, |z_5_n, _| z_5_n.square());
        let power10 = squared5 * power5;
        let squared10 = (0..10).fold(power10, |x, _| x.square());
        let power20 = squared10 * power10;
        let squared20 = (0..20).fold(power20, |x, _| x.square());
        let power40 = squared20 * power20;
        let squared40 = (0..10).fold(power40, |x, _| x.square());
        let power50 = squared40 * power10;
        let squared50 = (0..50).fold(power50, |x, _| x.square());
        let power100 = squared50 * power50;
        let squared100 = (0..100).fold(power100, |x, _| x.square());
        let power200 = squared100 * power100;
        let squared200 = (0..50).fold(power200, |x, _| x.square());
        let accumulated_power = squared200 * power50;
        let final_squares = (0..5).fold(accumulated_power, |x, _| x.square());
        final_squares * z11
    }

    pub fn is_zero(&self) -> bool {
        self.to_bytes()
            .iter()
            .fold(0, |acc, horizontal| acc | horizontal)
            == 0
    }

    pub fn is_negative(&self) -> bool {
        (self.to_bytes()[0] & 1) != 0
    }

    pub fn neg(&self) -> Fe {
        let &Fe(lhs) = self;
        let mut output = Fe::default();
        fiat_25519_opp(&mut output.0, &lhs);
        output
    }

    pub fn pow25523(&self) -> Fe {
        let z2 = self.square();
        let z8 = (0..2).fold(z2, |horizontal, _| horizontal.square());
        let z9 = *self * z8;
        let z11 = z2 * z9;
        let z22 = z11.square();
        let power5 = z9 * z22;
        let squared5 = (0..5).fold(power5, |horizontal, _| horizontal.square());
        let power10 = squared5 * power5;
        let squared10 = (0..10).fold(power10, |horizontal, _| horizontal.square());
        let power20 = squared10 * power10;
        let squared20 = (0..20).fold(power20, |horizontal, _| horizontal.square());
        let power40 = squared20 * power20;
        let squared40 = (0..10).fold(power40, |horizontal, _| horizontal.square());
        let power50 = squared40 * power10;
        let squared50 = (0..50).fold(power50, |horizontal, _| horizontal.square());
        let power100 = squared50 * power50;
        let squared100 = (0..100).fold(power100, |horizontal, _| horizontal.square());
        let power200 = squared100 * power100;
        let squared200 = (0..50).fold(power200, |horizontal, _| horizontal.square());
        let accumulated_power = squared200 * power50;
        let final_squares = (0..2).fold(accumulated_power, |horizontal, _| horizontal.square());
        final_squares * *self
    }
}
