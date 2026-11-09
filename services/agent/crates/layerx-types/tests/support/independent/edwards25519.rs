// From ed25519-compact 2.1.1 (MIT); see PROVENANCE.md.
use core::cmp::min;
use core::ops::{Add, Sub};

use super::error::*;
use super::field25519::*;

#[derive(Clone, Copy)]
pub struct GeP2 {
    x: Fe,
    y: Fe,
    z: Fe,
}

#[derive(Clone, Copy)]
pub struct GeP3 {
    x: Fe,
    y: Fe,
    z: Fe,
    t: Fe,
}

#[derive(Clone, Copy, Default)]
pub struct GeP1P1 {
    x: Fe,
    y: Fe,
    z: Fe,
    t: Fe,
}

#[derive(Clone, Copy)]
pub struct GePrecomp {
    y_plus_x: Fe,
    y_minus_x: Fe,
    xy2d: Fe,
}

#[derive(Clone, Copy, Default)]
pub struct GeCached {
    y_plus_x: Fe,
    y_minus_x: Fe,
    z: Fe,
    t2d: Fe,
}

impl GeCached {}

impl GeP1P1 {
    fn to_p2(self) -> GeP2 {
        GeP2 {
            x: self.x * self.t,
            y: self.y * self.z,
            z: self.z * self.t,
        }
    }

    fn to_p3(self) -> GeP3 {
        GeP3 {
            x: self.x * self.t,
            y: self.y * self.z,
            z: self.z * self.t,
            t: self.x * self.y,
        }
    }
}

impl From<GeP2> for GeP3 {
    fn from(p: GeP2) -> GeP3 {
        GeP3 {
            x: p.x,
            y: p.y,
            z: p.z,
            t: p.x * p.y,
        }
    }
}

impl GeP2 {
    fn zero() -> GeP2 {
        GeP2 {
            x: FE_ZERO,
            y: FE_ONE,
            z: FE_ONE,
        }
    }

    fn dbl(&self) -> GeP1P1 {
        let xx = self.x.square();
        let yy = self.y.square();
        let second = self.z.square_and_double();
        let first = self.x + self.y;
        let aa = first.square();
        let y3 = yy + xx;
        let z3 = yy - xx;
        let x3 = aa - y3;
        let t3 = second - z3;

        GeP1P1 {
            x: x3,
            y: y3,
            z: z3,
            t: t3,
        }
    }

    fn slide(first: &[u8]) -> [i8; 256] {
        let mut result = [0i8; 256];
        for index in 0..256 {
            result[index] = {
                #[allow(
                    clippy::cast_possible_truncation,
                    clippy::cast_possible_wrap,
                    clippy::cast_sign_loss,
                    clippy::cast_lossless,
                    reason = "Preserve the reference bit representation under the original limb masks and carry bounds."
                )]
                let converted_bits = (1 & (first[index >> 3] >> (index & 7))) as i8;
                converted_bits
            };
        }
        for index in 0..256 {
            if result[index] != 0 {
                for second in 1..min(7, 256 - index) {
                    if result[index + second] != 0 {
                        if result[index] + (result[index + second] << second) <= 15 {
                            result[index] += result[index + second] << second;
                            result[index + second] = 0;
                        } else if result[index] - (result[index + second] << second) >= -15 {
                            result[index] -= result[index + second] << second;
                            for digit in &mut result[index + second..] {
                                if *digit == 0 {
                                    *digit = 1;
                                    break;
                                }
                                *digit = 0;
                            }
                        } else {
                            break;
                        }
                    }
                }
            }
        }

        result
    }

    #[allow(clippy::comparison_chain)]
    pub fn double_scalarmult_vartime(a_scalar: &[u8], a_point: GeP3, b_scalar: &[u8]) -> GeP2 {
        let point_slides = GeP2::slide(a_scalar);
        let base_slides = GeP2::slide(b_scalar);

        let mut ai = [GeCached {
            y_plus_x: FE_ZERO,
            y_minus_x: FE_ZERO,
            z: FE_ZERO,
            t2d: FE_ZERO,
        }; 8]; // A,3A,5A,7A,9A,11A,13A,15A
        ai[0] = a_point.to_cached();
        let a2 = a_point.dbl().to_p3();
        ai[1] = (a2 + ai[0]).to_p3().to_cached();
        ai[2] = (a2 + ai[1]).to_p3().to_cached();
        ai[3] = (a2 + ai[2]).to_p3().to_cached();
        ai[4] = (a2 + ai[3]).to_p3().to_cached();
        ai[5] = (a2 + ai[4]).to_p3().to_cached();
        ai[6] = (a2 + ai[5]).to_p3().to_cached();
        ai[7] = (a2 + ai[6]).to_p3().to_cached();

        let mut result = GeP2::zero();

        let mut index: usize = 255;
        loop {
            if point_slides[index] != 0 || base_slides[index] != 0 {
                break;
            }
            if index == 0 {
                return result;
            }
            index -= 1;
        }

        loop {
            let mut temporary = result.dbl();
            if point_slides[index] > 0 {
                temporary = temporary.to_p3()
                    + ai[{
                        #[allow(
                            clippy::cast_possible_truncation,
                            clippy::cast_possible_wrap,
                            clippy::cast_sign_loss,
                            clippy::cast_lossless,
                            reason = "Preserve the reference bit representation under the original limb masks and carry bounds."
                        )]
                        let converted_bits = (point_slides[index] / 2) as usize;
                        converted_bits
                    }];
            } else if point_slides[index] < 0 {
                temporary = temporary.to_p3()
                    - ai[{
                        #[allow(
                            clippy::cast_possible_truncation,
                            clippy::cast_possible_wrap,
                            clippy::cast_sign_loss,
                            clippy::cast_lossless,
                            reason = "Preserve the reference bit representation under the original limb masks and carry bounds."
                        )]
                        let converted_bits = (-point_slides[index] / 2) as usize;
                        converted_bits
                    }];
            }

            if base_slides[index] > 0 {
                temporary = temporary.to_p3()
                    + BI[{
                        #[allow(
                            clippy::cast_possible_truncation,
                            clippy::cast_possible_wrap,
                            clippy::cast_sign_loss,
                            clippy::cast_lossless,
                            reason = "Preserve the reference bit representation under the original limb masks and carry bounds."
                        )]
                        let converted_bits = (base_slides[index] / 2) as usize;
                        converted_bits
                    }];
            } else if base_slides[index] < 0 {
                temporary = temporary.to_p3()
                    - BI[{
                        #[allow(
                            clippy::cast_possible_truncation,
                            clippy::cast_possible_wrap,
                            clippy::cast_sign_loss,
                            clippy::cast_lossless,
                            reason = "Preserve the reference bit representation under the original limb masks and carry bounds."
                        )]
                        let converted_bits = (-base_slides[index] / 2) as usize;
                        converted_bits
                    }];
            }

            result = temporary.to_p2();

            if index == 0 {
                return result;
            }
            index -= 1;
        }
    }
}

impl GeP3 {
    pub fn from_bytes_negate_vartime(bytes: &[u8; 32]) -> Option<GeP3> {
        let vertical = Fe::from_bytes(bytes);
        let projective = FE_ONE;
        let y_squared = vertical.square();
        let numerator = y_squared - FE_ONE;
        let denominator = (y_squared * FE_D) + FE_ONE;
        let mut horizontal = (numerator * denominator).pow25523() * numerator;

        let vxx = horizontal.square() * denominator;
        let check = vxx - numerator;
        if !check.is_zero() {
            let check2 = vxx + numerator;
            if !check2.is_zero() {
                return None;
            }
            horizontal = horizontal * FE_SQRTM1;
        }

        if horizontal.is_negative() == ((bytes[31] >> 7) != 0) {
            horizontal = horizontal.neg();
        }

        let temporary = horizontal * vertical;

        Some(GeP3 {
            x: horizontal,
            y: vertical,
            z: projective,
            t: temporary,
        })
    }

    pub fn from_bytes_vartime(bytes: &[u8; 32]) -> Option<GeP3> {
        Self::from_bytes_negate_vartime(bytes).map(|p| GeP3 {
            x: p.x.neg(),
            y: p.y,
            z: p.z,
            t: p.t.neg(),
        })
    }

    fn to_p2(self) -> GeP2 {
        GeP2 {
            x: self.x,
            y: self.y,
            z: self.z,
        }
    }

    fn to_cached(self) -> GeCached {
        GeCached {
            y_plus_x: self.y + self.x,
            y_minus_x: self.y - self.x,
            z: self.z,
            t2d: self.t * FE_D2,
        }
    }

    fn dbl(&self) -> GeP1P1 {
        self.to_p2().dbl()
    }

    pub fn has_small_order(&self) -> bool {
        let recip = self.z.invert();
        let horizontal = self.x * recip;
        let vertical = self.y * recip;
        let x_neg = horizontal.neg();
        let y_sqrtm1 = vertical * FE_SQRTM1;
        horizontal.is_zero() | vertical.is_zero() | (y_sqrtm1 == horizontal) | (y_sqrtm1 == x_neg)
    }
}

impl Add<GeP3> for GeP3 {
    type Output = GeP3;

    fn add(self, other: GeP3) -> GeP3 {
        (self + other.to_cached()).to_p3()
    }
}

impl Sub<GeP3> for GeP3 {
    type Output = GeP3;

    fn sub(self, other: GeP3) -> GeP3 {
        (self - other.to_cached()).to_p3()
    }
}

impl Add<GeCached> for GeP3 {
    type Output = GeP1P1;

    fn add(self, rhs: GeCached) -> GeP1P1 {
        let y1_plus_x1 = self.y + self.x;
        let y1_minus_x1 = self.y - self.x;
        let first = y1_plus_x1 * rhs.y_plus_x;
        let second = y1_minus_x1 * rhs.y_minus_x;
        let third = rhs.t2d * self.t;
        let zz = self.z * rhs.z;
        let fourth = zz + zz;
        let x3 = first - second;
        let y3 = first + second;
        let z3 = fourth + third;
        let t3 = fourth - third;

        GeP1P1 {
            x: x3,
            y: y3,
            z: z3,
            t: t3,
        }
    }
}

impl Add<GePrecomp> for GeP3 {
    type Output = GeP1P1;

    fn add(self, rhs: GePrecomp) -> GeP1P1 {
        let y1_plus_x1 = self.y + self.x;
        let y1_minus_x1 = self.y - self.x;
        let first = y1_plus_x1 * rhs.y_plus_x;
        let second = y1_minus_x1 * rhs.y_minus_x;
        let third = rhs.xy2d * self.t;
        let fourth = self.z + self.z;
        let x3 = first - second;
        let y3 = first + second;
        let z3 = fourth + third;
        let t3 = fourth - third;

        GeP1P1 {
            x: x3,
            y: y3,
            z: z3,
            t: t3,
        }
    }
}

impl Sub<GeCached> for GeP3 {
    type Output = GeP1P1;

    fn sub(self, rhs: GeCached) -> GeP1P1 {
        let y1_plus_x1 = self.y + self.x;
        let y1_minus_x1 = self.y - self.x;
        let first = y1_plus_x1 * rhs.y_minus_x;
        let second = y1_minus_x1 * rhs.y_plus_x;
        let third = rhs.t2d * self.t;
        let zz = self.z * rhs.z;
        let fourth = zz + zz;
        let x3 = first - second;
        let y3 = first + second;
        let z3 = fourth - third;
        let t3 = fourth + third;

        GeP1P1 {
            x: x3,
            y: y3,
            z: z3,
            t: t3,
        }
    }
}

impl Sub<GePrecomp> for GeP3 {
    type Output = GeP1P1;

    fn sub(self, rhs: GePrecomp) -> GeP1P1 {
        let y1_plus_x1 = self.y + self.x;
        let y1_minus_x1 = self.y - self.x;
        let first = y1_plus_x1 * rhs.y_minus_x;
        let second = y1_minus_x1 * rhs.y_plus_x;
        let third = rhs.xy2d * self.t;
        let fourth = self.z + self.z;
        let x3 = first - second;
        let y3 = first + second;
        let z3 = fourth - third;
        let t3 = fourth + third;

        GeP1P1 {
            x: x3,
            y: y3,
            z: z3,
            t: t3,
        }
    }
}

fn fold_scalar(limbs: &mut [i64; 24], index: usize) {
    let high = limbs[index];
    limbs[index - 12] += high * 666_643;
    limbs[index - 11] += high * 470_296;
    limbs[index - 10] += high * 654_183;
    limbs[index - 9] -= high * 997_805;
    limbs[index - 8] += high * 136_657;
    limbs[index - 7] -= high * 683_901;
}

fn carry_scalar(limbs: &mut [i64; 24], index: usize, rounding: i64) {
    let carry = (limbs[index] + rounding) >> 21;
    limbs[index + 1] += carry;
    limbs[index] -= carry << 21;
}

fn encode_scalar(bytes: &mut [u8], limbs: &[i64; 24]) {
    bytes[0] = (limbs[0]).to_le_bytes()[0];
    bytes[1] = (limbs[0] >> 8).to_le_bytes()[0];
    bytes[2] = ((limbs[0] >> 16) | (limbs[1] << 5)).to_le_bytes()[0];
    bytes[3] = (limbs[1] >> 3).to_le_bytes()[0];
    bytes[4] = (limbs[1] >> 11).to_le_bytes()[0];
    bytes[5] = ((limbs[1] >> 19) | (limbs[2] << 2)).to_le_bytes()[0];
    bytes[6] = (limbs[2] >> 6).to_le_bytes()[0];
    bytes[7] = ((limbs[2] >> 14) | (limbs[3] << 7)).to_le_bytes()[0];
    bytes[8] = (limbs[3] >> 1).to_le_bytes()[0];
    bytes[9] = (limbs[3] >> 9).to_le_bytes()[0];
    bytes[10] = ((limbs[3] >> 17) | (limbs[4] << 4)).to_le_bytes()[0];
    bytes[11] = (limbs[4] >> 4).to_le_bytes()[0];
    bytes[12] = (limbs[4] >> 12).to_le_bytes()[0];
    bytes[13] = ((limbs[4] >> 20) | (limbs[5] << 1)).to_le_bytes()[0];
    bytes[14] = (limbs[5] >> 7).to_le_bytes()[0];
    bytes[15] = ((limbs[5] >> 15) | (limbs[6] << 6)).to_le_bytes()[0];
    bytes[16] = (limbs[6] >> 2).to_le_bytes()[0];
    bytes[17] = (limbs[6] >> 10).to_le_bytes()[0];
    bytes[18] = ((limbs[6] >> 18) | (limbs[7] << 3)).to_le_bytes()[0];
    bytes[19] = (limbs[7] >> 5).to_le_bytes()[0];
    bytes[20] = (limbs[7] >> 13).to_le_bytes()[0];
    bytes[21] = (limbs[8]).to_le_bytes()[0];
    bytes[22] = (limbs[8] >> 8).to_le_bytes()[0];
    bytes[23] = ((limbs[8] >> 16) | (limbs[9] << 5)).to_le_bytes()[0];
    bytes[24] = (limbs[9] >> 3).to_le_bytes()[0];
    bytes[25] = (limbs[9] >> 11).to_le_bytes()[0];
    bytes[26] = ((limbs[9] >> 19) | (limbs[10] << 2)).to_le_bytes()[0];
    bytes[27] = (limbs[10] >> 6).to_le_bytes()[0];
    bytes[28] = ((limbs[10] >> 14) | (limbs[11] << 7)).to_le_bytes()[0];
    bytes[29] = (limbs[11] >> 1).to_le_bytes()[0];
    bytes[30] = (limbs[11] >> 9).to_le_bytes()[0];
    bytes[31] = (limbs[11] >> 17).to_le_bytes()[0];
}

pub fn sc_reduce(bytes: &mut [u8]) {
    let mut limbs = [
        2_097_151 & load_3i(bytes),
        2_097_151 & (load_4i(&bytes[2..6]) >> 5),
        2_097_151 & (load_3i(&bytes[5..8]) >> 2),
        2_097_151 & (load_4i(&bytes[7..11]) >> 7),
        2_097_151 & (load_4i(&bytes[10..14]) >> 4),
        2_097_151 & (load_3i(&bytes[13..16]) >> 1),
        2_097_151 & (load_4i(&bytes[15..19]) >> 6),
        2_097_151 & (load_3i(&bytes[18..21]) >> 3),
        2_097_151 & load_3i(&bytes[21..24]),
        2_097_151 & (load_4i(&bytes[23..27]) >> 5),
        2_097_151 & (load_3i(&bytes[26..29]) >> 2),
        2_097_151 & (load_4i(&bytes[28..32]) >> 7),
        2_097_151 & (load_4i(&bytes[31..35]) >> 4),
        2_097_151 & (load_3i(&bytes[34..37]) >> 1),
        2_097_151 & (load_4i(&bytes[36..40]) >> 6),
        2_097_151 & (load_3i(&bytes[39..42]) >> 3),
        2_097_151 & load_3i(&bytes[42..45]),
        2_097_151 & (load_4i(&bytes[44..48]) >> 5),
        2_097_151 & (load_3i(&bytes[47..50]) >> 2),
        2_097_151 & (load_4i(&bytes[49..53]) >> 7),
        2_097_151 & (load_4i(&bytes[52..56]) >> 4),
        2_097_151 & (load_3i(&bytes[55..58]) >> 1),
        2_097_151 & (load_4i(&bytes[57..61]) >> 6),
        load_4i(&bytes[60..64]) >> 3,
    ];
    for index in (18..24).rev() {
        fold_scalar(&mut limbs, index);
    }
    for index in (6..17).step_by(2) {
        carry_scalar(&mut limbs, index, 1 << 20);
    }
    for index in (7..16).step_by(2) {
        carry_scalar(&mut limbs, index, 1 << 20);
    }
    for index in (12..18).rev() {
        fold_scalar(&mut limbs, index);
    }
    limbs[12] = 0;
    for index in (0..11).step_by(2) {
        carry_scalar(&mut limbs, index, 1 << 20);
    }
    for index in (1..12).step_by(2) {
        carry_scalar(&mut limbs, index, 1 << 20);
    }
    fold_scalar(&mut limbs, 12);
    limbs[12] = 0;
    for index in 0..12 {
        carry_scalar(&mut limbs, index, 0);
    }
    fold_scalar(&mut limbs, 12);
    for index in 0..11 {
        carry_scalar(&mut limbs, index, 0);
    }
    encode_scalar(bytes, &limbs);
}

pub fn sc_reject_noncanonical(bytes: &[u8]) -> Result<(), Error> {
    static L: [u8; 32] = [
        0xed, 0xd3, 0xf5, 0x5c, 0x1a, 0x63, 0x12, 0x58, 0xd6, 0x9c, 0xf7, 0xa2, 0xde, 0xf9, 0xde,
        0x14, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x10,
    ];
    assert_eq!(bytes.len(), 32, "Invalid compressed length");
    let mut third: u8 = 0;
    let mut count: u8 = 1;

    let mut index = 31;
    loop {
        third |= {
            #[allow(
                clippy::cast_possible_truncation,
                clippy::cast_sign_loss,
                reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
            )]
            let converted_bits = ((i32::from(bytes[index]) - i32::from(L[index])) >> 8) as u8;
            converted_bits
        } & count;
        count &= {
            #[allow(
                clippy::cast_possible_truncation,
                clippy::cast_sign_loss,
                reason = "Preserve the reference field/scalar low-bit extraction and two's-complement representation under the original masks and carry bounds."
            )]
            let converted_bits = ((i32::from(bytes[index] ^ L[index]) - 1) >> 8) as u8;
            converted_bits
        };
        if index == 0 {
            break;
        }
        index -= 1;
    }
    if third != 0 {
        Ok(())
    } else {
        Err(Error::NonCanonical)
    }
}

pub fn is_identity(bytes: &[u8; 32]) -> bool {
    let mut third = bytes[0] ^ 0x01;
    for byte in &bytes[1..31] {
        third |= byte;
    }
    third |= bytes[31] & 0x7f;
    third == 0
}

static BI: [GePrecomp; 8] = [
    GePrecomp {
        y_plus_x: Fe([
            1_288_382_639_258_501,
            245_678_601_348_599,
            269_427_782_077_623,
            1_462_984_067_271_730,
            137_412_439_391_563,
        ]),
        y_minus_x: Fe([
            62_697_248_952_638,
            204_681_361_388_450,
            631_292_143_396_476,
            338_455_783_676_468,
            1_213_667_448_819_585,
        ]),
        xy2d: Fe([
            301_289_933_810_280,
            1_259_582_250_014_073,
            1_422_107_436_869_536,
            796_239_922_652_654,
            1_953_934_009_299_142,
        ]),
    },
    GePrecomp {
        y_plus_x: Fe([
            1_601_611_775_252_272,
            1_720_807_796_594_148,
            1_132_070_835_939_856,
            1_260_455_018_889_551,
            2_147_779_492_816_911,
        ]),
        y_minus_x: Fe([
            316_559_037_616_741,
            2_177_824_224_946_892,
            1_459_442_586_438_991,
            1_461_528_397_712_656,
            751_590_696_113_597,
        ]),
        xy2d: Fe([
            1_850_748_884_277_385,
            1_200_145_853_858_453,
            1_068_094_770_532_492,
            672_251_375_690_438,
            1_586_055_907_191_707,
        ]),
    },
    GePrecomp {
        y_plus_x: Fe([
            769_950_342_298_419,
            132_954_430_919_746,
            844_085_933_195_555,
            974_092_374_476_333,
            726_076_285_546_016,
        ]),
        y_minus_x: Fe([
            425_251_763_115_706,
            608_463_272_472_562,
            442_562_545_713_235,
            837_766_094_556_764,
            374_555_092_627_893,
        ]),
        xy2d: Fe([
            1_086_255_230_780_037,
            274_979_815_921_559,
            1_960_002_765_731_872,
            929_474_102_396_301,
            1_190_409_889_297_339,
        ]),
    },
    GePrecomp {
        y_plus_x: Fe([
            665_000_864_555_967,
            2_065_379_846_933_859,
            370_231_110_385_876,
            350_988_370_788_628,
            1_233_371_373_142_985,
        ]),
        y_minus_x: Fe([
            2_019_367_628_972_465,
            676_711_900_706_637,
            110_710_997_811_333,
            1_108_646_842_542_025,
            517_791_959_672_113,
        ]),
        xy2d: Fe([
            965_130_719_900_578,
            247_011_430_587_952,
            526_356_006_571_389,
            91_986_625_355_052,
            2_157_223_321_444_601,
        ]),
    },
    GePrecomp {
        y_plus_x: Fe([
            1_802_695_059_465_007,
            1_664_899_123_557_221,
            593_559_490_740_857,
            2_160_434_469_266_659,
            927_570_450_755_031,
        ]),
        y_minus_x: Fe([
            1_725_674_970_513_508,
            1_933_645_953_859_181,
            1_542_344_539_275_782,
            1_767_788_773_573_747,
            1_297_447_965_928_905,
        ]),
        xy2d: Fe([
            1_381_809_363_726_107,
            1_430_341_051_343_062,
            2_061_843_536_018_959,
            1_551_778_050_872_521,
            2_036_394_857_967_624,
        ]),
    },
    GePrecomp {
        y_plus_x: Fe([
            1_970_894_096_313_054,
            528_066_325_833_207,
            1_619_374_932_191_227,
            2_207_306_624_415_883,
            1_169_170_329_061_080,
        ]),
        y_minus_x: Fe([
            2_070_390_218_572_616,
            1_458_919_061_857_835,
            624_171_843_017_421,
            1_055_332_792_707_765,
            433_987_520_732_508,
        ]),
        xy2d: Fe([
            893_653_801_273_833,
            1_168_026_499_324_677,
            1_242_553_501_121_234,
            1_306_366_254_304_474,
            1_086_752_658_510_815,
        ]),
    },
    GePrecomp {
        y_plus_x: Fe([
            213_454_002_618_221,
            939_771_523_987_438,
            1_159_882_208_056_014,
            317_388_369_627_517,
            621_213_314_200_687,
        ]),
        y_minus_x: Fe([
            1_971_678_598_905_747,
            338_026_507_889_165,
            762_398_079_972_271,
            655_096_486_107_477,
            42_299_032_696_322,
        ]),
        xy2d: Fe([
            177_130_678_690_680,
            1_754_759_263_300_204,
            1_864_311_296_286_618,
            1_180_675_631_479_880,
            1_292_726_903_152_791,
        ]),
    },
    GePrecomp {
        y_plus_x: Fe([
            1_913_163_449_625_248,
            460_779_200_291_993,
            2_193_883_288_642_314,
            1_008_900_146_920_800,
            1_721_983_679_009_502,
        ]),
        y_minus_x: Fe([
            1_070_401_523_076_875,
            1_272_492_007_800_961,
            1_910_153_608_563_310,
            2_075_579_521_696_771,
            1_191_169_788_841_221,
        ]),
        xy2d: Fe([
            692_896_803_108_118,
            500_174_642_072_499,
            2_068_223_309_439_677,
            1_162_190_621_851_337,
            1_426_986_007_309_901,
        ]),
    },
];
