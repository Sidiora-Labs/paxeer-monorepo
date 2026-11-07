// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/f16.js (MIT; see LICENSE-ENGINE).
// f32 <-> f16 by hand, for f16 values whose bits must match across backends and the CPU reference.
//
// WGSL leaves the rounding of pack2x16float (and of every f32 -> f16 conversion) to the implementation: either
// neighbour of an inexact value is allowed. Vulkan and Metal round to nearest even, D3D12 does not: Dawn/Tint and
// wgpu/naga both lower pack2x16float to HLSL f32tof16, which on the RTX 5070 (driver 610.88) rounds 77K of 131K
// QSA pooled-key values to the other neighbour. f16_rne / pack2x16_rne below round to nearest, ties to even, with
// subnormals, overflow to inf and NaN (quiet, 0x7e00 | sign) handled as ggml_fp32_to_fp16 and the CPU reference
// (tests/reference/qsa_ref.mjs f16bits) do; on a backend whose pack2x16float already rounds that way they give the
// same bits. f16_f32 is the exact inverse for every input (subnormals and -0 included; unpack2x16float may flush
// f16 subnormals). Checked against f16bits / h2f on the GPU in tests/test_qsa.js ("f16").
//
// Sites that must use these: the f16 values QSA stores and re-reads (engine/wgsl/qsa.js rawK, pooled keys).
export const F16_WGSL = /* wgsl */ `
// f32 -> f16 bits (low 16), round to nearest, ties to even
fn f16_rne(x: f32) -> u32 {
  let i = bitcast<u32>(x);
  let s = (i >> 16u) & 0x8000u;
  let ef = (i >> 23u) & 0xffu;
  let m = i & 0x7fffffu;
  if (ef == 0xffu) { return s | 0x7c00u | select(0u, 0x200u, m != 0u); }   // inf, NaN (quiet)
  if (ef > 142u) { return s | 0x7c00u; }                                    // |x| >= 2^16: inf
  if (ef < 113u) {                                                          // f16 subnormal or zero
    if (ef < 102u) { return s; }                                            // |x| < 2^-25: 0
    let mm = m | 0x800000u; let sh = 126u - ef;                             // 14 .. 24
    let h = mm >> sh; let rem = mm & ((1u << sh) - 1u); let hf = 1u << (sh - 1u);
    return s | (h + select(0u, 1u, rem > hf || (rem == hf && (h & 1u) == 1u)));
  }
  let h = ((ef - 112u) << 10u) | (m >> 13u);                                // a carry out of m rounds up the exponent,
  let r = m & 0x1fffu;                                                      // 0x7bff + 1 = inf
  return s | (h + select(0u, 1u, r > 0x1000u || (r == 0x1000u && (h & 1u) == 1u)));
}
fn pack2x16_rne(v: vec2<f32>) -> u32 { return f16_rne(v.x) | (f16_rne(v.y) << 16u); }
// f16 bits (low 16) -> f32, exact
fn f16_f32(h: u32) -> f32 {
  let e = (h >> 10u) & 0x1fu; let m = h & 0x3ffu;
  var v: f32;
  if (e == 0u) { v = f32(m) * 5.9604644775390625e-8; }                     // m * 2^-24, exact
  else if (e == 31u) { v = bitcast<f32>(0x7f800000u | (m << 13u)); }
  else { v = bitcast<f32>(((e + 112u) << 23u) | (m << 13u)); }
  return select(v, -v, (h & 0x8000u) != 0u);
}
fn unpack2x16_exact(w: u32) -> vec2<f32> { return vec2<f32>(f16_f32(w & 0xffffu), f16_f32(w >> 16u)); }
`;
