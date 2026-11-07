// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/qwen4exp_batch.js (MIT; see LICENSE-ENGINE).
// Multi-column WGSL for Qwen3.8-Flash-Next (qwen4exp): the batched prefill and verify passes of
// engine/qwen4exp_batch.js, and the gathered (QSA) attention that both those passes and the decode engine use.
// Roadmap 33 PR plan items 5 and 9.
//
// One pass = nCols consecutive tokens at positions pos .. pos + nCols - 1. Every kernel here is a decode kernel of
// engine/wgsl/qwen4exp.js (the fused ones: HC sites with the combine folded in and per-stream inject partials,
// conv + l2, delta rule + gated norm, the one-kernel PLE, the up projection + mix) with a column index added: per
// column it does the same arithmetic in the same order, so a prefill column gives the decode step's bits
// (tests/test_q38flash_mc.js per kernel, tests/test_q38flash_batch.js on the model). The kernels with a sequential
// dependency across tokens walk the columns in order inside one dispatch:
//   - dn_conv: one workgroup per 128-channel head, as the decode kernel; the conv history stays in registers;
//   - dn_delta: one workgroup per V head; each thread keeps its 64 entries of S in registers across the columns;
//   - ple: one workgroup per stream, as the decode kernel; each thread owns its channels in every column, so a
//     column's ring write is seen by the later columns' conv taps with no barrier (a pass can be longer than the ring).
// The weight GEMVs (gemv_mc) take C columns per workgroup: each weight block is read and dequantized once for those
// columns, and each column keeps the decode GEMV's lanes per row (gemvLanes) and its reduction tree. The decode
// GEMV's 4-rows-per-lane variant changes only which lanes load x, not any row's sum, so it needs no counterpart.
//
// Column strides are in the uniforms. Bindings stay within WebGPU's default of 8 storage buffers per stage.

import { gemvLanes, kvCheck, kvDecl, kvK4, kvV1, kvStoreRow, KV_F16_READ } from "./qwen4exp.js";
export { gemvLanes };

const DQ = /* wgsl */ `
fn q4lo(w: u32) -> vec4<f32> { return vec4<f32>(f32(w & 15u), f32((w >> 8u) & 15u), f32((w >> 16u) & 15u), f32((w >> 24u) & 15u)) - vec4<f32>(8.0); }
fn q4hi(w: u32) -> vec4<f32> { return vec4<f32>(f32((w >> 4u) & 15u), f32((w >> 12u) & 15u), f32((w >> 20u) & 15u), f32((w >> 28u) & 15u)) - vec4<f32>(8.0); }
fn i8x4(u: u32) -> vec4<f32> { let v = bitcast<i32>(u); return vec4<f32>(f32((v << 24u) >> 24u), f32((v << 16u) >> 24u), f32((v << 8u) >> 24u), f32(v >> 24u)); }
fn sigm(v: f32) -> f32 { return 1.0 / (1.0 + exp(-v)); }
fn silu(v: f32) -> f32 { return v / (1.0 + exp(-v)); }
`;

// The decode kernels' 32-wide block dot (engine/wgsl/qwen4exp.js blockDot), the same expression, into `d`.
const blockDot = (kind, QS, SC, X) => kind === "q4" ? `
  {
    let s = unpack2x16float(${SC}[blk >> 1u])[blk & 1u];
    let wv = ${QS}[blk];
    let w0 = wv.x; let w1 = wv.y; let w2 = wv.z; let w3 = wv.w;
    d = s * (dot(q4lo(w0), ${X}[xb4]) + dot(q4lo(w1), ${X}[xb4 + 1u]) + dot(q4lo(w2), ${X}[xb4 + 2u]) + dot(q4lo(w3), ${X}[xb4 + 3u])
           + dot(q4hi(w0), ${X}[xb4 + 4u]) + dot(q4hi(w1), ${X}[xb4 + 5u]) + dot(q4hi(w2), ${X}[xb4 + 6u]) + dot(q4hi(w3), ${X}[xb4 + 7u]));
  }` : `
  {
    let s = unpack2x16float(${SC}[blk >> 1u])[blk & 1u];
    let wa = ${QS}[blk * 2u]; let wb = ${QS}[blk * 2u + 1u];
    let a = dot(i8x4(wa.x), ${X}[xb4]) + dot(i8x4(wa.y), ${X}[xb4 + 1u]) + dot(i8x4(wa.z), ${X}[xb4 + 2u]) + dot(i8x4(wa.w), ${X}[xb4 + 3u])
          + dot(i8x4(wb.x), ${X}[xb4 + 4u]) + dot(i8x4(wb.y), ${X}[xb4 + 5u]) + dot(i8x4(wb.z), ${X}[xb4 + 6u]) + dot(i8x4(wb.w), ${X}[xb4 + 7u]);
    d = s * a;
  }`;

// The pass frame: first position and column count (a uniform), and per column the rope (cos, sin, 0, 0) of each
// rotary pair (a storage buffer, MC_FRAME_COL floats per column: the decode frame's table).
const MCF = `struct McF { pos: u32, nCols: u32, p0: u32, p1: u32 };`;
export const MC_FRAME_COL = 64 * 4;

export const GEMV_MC_COLS = 4;   // columns per gemv_mc workgroup

// GEMV over nCols columns: y[col * ys + yOff + r] = epilogue(W[rowOff + r, :] . x[col * xs4 * 4 + xOff4 * 4 ..]).
// LANES lanes per row as the decode GEMV (gemvLanes), 256 / LANES rows per workgroup; workgroups x (and y past
// 32768) over the rows, z = column group of C. mode 1: silu(acc * scale).
export function gemvMcWGSL(kind, LANES = 32, C = GEMV_MC_COLS) {
  const RPW = 256 / LANES, cols = [...Array(C).keys()];
  const W = kind === "f32" ? `@group(0) @binding(0) var<storage, read> fx_w: array<vec4<f32>>;` : `
@group(0) @binding(0) var<storage, read> fx_qs: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> fx_sc: array<u32>;`;
  const xb = (c) => `fx_P.xOff4 + cc${c} * fx_P.xs4`;
  // the decode GEMV's block dot with its x block in xv0..xv7, the dequantized weights shared by the columns
  const dotPre = kind === "q4" ? `d = s * (dot(l0, xv0) + dot(l1, xv1) + dot(l2, xv2) + dot(l3, xv3)
               + dot(h0, xv4) + dot(h1, xv5) + dot(h2, xv6) + dot(h3, xv7));`
    : `d = s * (dot(e0, xv0) + dot(e1, xv1) + dot(e2, xv2) + dot(e3, xv3)
               + dot(e4, xv4) + dot(e5, xv5) + dot(e6, xv6) + dot(e7, xv7));`;
  const body = kind === "f32" ? `
    let n4 = fx_P.dIn / 4u;
    let ro = (fx_P.rowOff + r0) * n4;
    for (var i = lane; i < n4; i += ${LANES}u) {
      let wv = fx_w[ro + i];
      ${cols.map((c) => `acc${c} += dot(wv, fx_x[${xb(c)} + i]);`).join("\n      ")}
    }` : `
    let nb = fx_P.dIn / 32u;
    for (var b = lane; b < nb; b += ${LANES}u) {
      let blk = (fx_P.rowOff + r0) * nb + b;
      let s = unpack2x16float(fx_sc[blk >> 1u])[blk & 1u];
      ${kind === "q4" ? `let wv = fx_qs[blk];
      let l0 = q4lo(wv.x); let l1 = q4lo(wv.y); let l2 = q4lo(wv.z); let l3 = q4lo(wv.w);
      let h0 = q4hi(wv.x); let h1 = q4hi(wv.y); let h2 = q4hi(wv.z); let h3 = q4hi(wv.w);` : `let wa = fx_qs[blk * 2u]; let wb = fx_qs[blk * 2u + 1u];
      let e0 = i8x4(wa.x); let e1 = i8x4(wa.y); let e2 = i8x4(wa.z); let e3 = i8x4(wa.w);
      let e4 = i8x4(wb.x); let e5 = i8x4(wb.y); let e6 = i8x4(wb.z); let e7 = i8x4(wb.w);`}
      ${cols.map((c) => `{
        let xb4 = ${xb(c)} + b * 8u;
        let xv0 = fx_x[xb4]; let xv1 = fx_x[xb4 + 1u]; let xv2 = fx_x[xb4 + 2u]; let xv3 = fx_x[xb4 + 3u];
        let xv4 = fx_x[xb4 + 4u]; let xv5 = fx_x[xb4 + 5u]; let xv6 = fx_x[xb4 + 6u]; let xv7 = fx_x[xb4 + 7u];
        var d = 0.0;
        ${dotPre}
        acc${c} += d;
      }`).join("\n      ")}
    }`;
  return DQ + `
struct GP { dOut: u32, dIn: u32, rowOff: u32, xOff4: u32, yOff: u32, mode: u32, scale: f32, xs4: u32, ys: u32, p0: u32, p1: u32, p2: u32 };
${MCF}
${W}
@group(0) @binding(2) var<storage, read> fx_x: array<vec4<f32>>;
@group(0) @binding(3) var<storage, read_write> fx_y: array<f32>;
@group(0) @binding(4) var<uniform> fx_P: GP;
@group(0) @binding(5) var<uniform> fx_F: McF;
var<workgroup> fx_red: array<f32, ${256 * C}>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let lid = l.x; let lane = lid % ${LANES}u;
  let r0 = (wg.y * 32768u + wg.x) * ${RPW}u + lid / ${LANES}u;
  let c0 = wg.z * ${C}u;
  ${cols.map((c) => `let cc${c} = min(c0 + ${c}u, fx_F.nCols - 1u); var acc${c} = 0.0;`).join("\n  ")}
  if (r0 < fx_P.dOut) {${body}
  }
  ${cols.map((c) => `fx_red[${c * 256}u + lid] = acc${c};`).join(" ")}
  workgroupBarrier();
  for (var s = ${LANES / 2}u; s > 0u; s >>= 1u) {
    if (lane < s) { ${cols.map((c) => `fx_red[${c * 256}u + lid] += fx_red[${c * 256}u + lid + s];`).join(" ")} }
    workgroupBarrier();
  }
  if (lane == 0u && r0 < fx_P.dOut) {
    ${cols.map((c) => `if (c0 + ${c}u < fx_F.nCols) {
      var v = fx_red[${c * 256}u + lid];
      if (fx_P.mode == 1u) { v = silu(v * fx_P.scale); }
      fx_y[(c0 + ${c}u) * fx_P.ys + fx_P.yOff + r0] = v;
    }`).join("\n    ")}
  }
}`;
}

// The HC up projection fused with the mix (gemvWGSL's MIX form), one column per workgroup z: lane group g owns
// column i = g of every stream (rows i + k * d, k < hc = 4), writes the gates and
// mixed[col][i] = mean over k of xn[col][k * d + i] * sigmoid(gate_k). Strides: x (lo) xs4 vec4s, gates and xn hc*d,
// mixed d (P.pad).
export function gemvMixMcWGSL(kind, LANES = 8) {
  const R = 4, RPW = 256 / LANES, RR = [...Array(R).keys()];
  const dotX = (blk) => kind === "q4" ? `{
        let s = unpack2x16float(fx_sc[${blk} >> 1u])[${blk} & 1u]; let wv = fx_qs[${blk}];
        d = s * (dot(q4lo(wv.x), xv0) + dot(q4lo(wv.y), xv1) + dot(q4lo(wv.z), xv2) + dot(q4lo(wv.w), xv3)
               + dot(q4hi(wv.x), xv4) + dot(q4hi(wv.y), xv5) + dot(q4hi(wv.z), xv6) + dot(q4hi(wv.w), xv7));
      }` : `{
        let s = unpack2x16float(fx_sc[${blk} >> 1u])[${blk} & 1u]; let wa = fx_qs[${blk} * 2u]; let wb = fx_qs[${blk} * 2u + 1u];
        d = s * (dot(i8x4(wa.x), xv0) + dot(i8x4(wa.y), xv1) + dot(i8x4(wa.z), xv2) + dot(i8x4(wa.w), xv3)
               + dot(i8x4(wb.x), xv4) + dot(i8x4(wb.y), xv5) + dot(i8x4(wb.z), xv6) + dot(i8x4(wb.w), xv7));
      }`;
  return DQ + `
struct GP { dOut: u32, dIn: u32, rowOff: u32, xOff4: u32, yOff: u32, mode: u32, scale: f32, pad: u32, xs4: u32, ys: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_qs: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> fx_sc: array<u32>;
@group(0) @binding(2) var<storage, read> fx_x: array<vec4<f32>>;
@group(0) @binding(3) var<storage, read_write> fx_y: array<f32>;
@group(0) @binding(4) var<uniform> fx_P: GP;
@group(0) @binding(5) var<storage, read> fx_xn: array<f32>;
@group(0) @binding(6) var<storage, read_write> fx_mixed: array<f32>;
var<workgroup> fx_red: array<f32, ${256 * R}>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let lid = l.x; let lane = lid % ${LANES}u;
  let r0 = (wg.y * 32768u + wg.x) * ${RPW}u + lid / ${LANES}u;
  let col = wg.z;
  ${RR.map((k) => `var acc${k} = 0.0;`).join(" ")}
  if (r0 < fx_P.pad) {
    let nb = fx_P.dIn / 32u;
    for (var b = lane; b < nb; b += ${LANES}u) {
      let xb4 = fx_P.xOff4 + col * fx_P.xs4 + b * 8u;
      let xv0 = fx_x[xb4]; let xv1 = fx_x[xb4 + 1u]; let xv2 = fx_x[xb4 + 2u]; let xv3 = fx_x[xb4 + 3u];
      let xv4 = fx_x[xb4 + 4u]; let xv5 = fx_x[xb4 + 5u]; let xv6 = fx_x[xb4 + 6u]; let xv7 = fx_x[xb4 + 7u];
      var d = 0.0;
${RR.map((k) => `      if (r0 < fx_P.pad) { let blk = (fx_P.rowOff + r0 + ${k}u * fx_P.pad) * nb + b; ${dotX("blk")} acc${k} += d; }`).join("\n")}
    }
  }
  ${RR.map((k) => `fx_red[${k * 256}u + lid] = acc${k};`).join(" ")}
  workgroupBarrier();
  for (var s = ${LANES / 2}u; s > 0u; s >>= 1u) {
    if (lane < s) { ${RR.map((k) => `fx_red[${k * 256}u + lid] += fx_red[${k * 256}u + lid + s];`).join(" ")} }
    workgroupBarrier();
  }
  if (lane == 0u && r0 < fx_P.pad) {
    let yb = col * fx_P.ys;
    var m = 0.0;
${RR.map((k) => `    { let g = fx_red[${k * 256}u + lid]; let row = r0 + ${k}u * fx_P.pad; fx_y[yb + fx_P.yOff + row] = g; m += fx_xn[yb + row] * sigm(g); }`).join("\n")}
    fx_mixed[col * fx_P.pad + r0] = m / ${R}.0;
  }
}`;
}

// Routed (or shared) experts, gate and up, per column: workgroup (row group, k, column).
// h[col * hs + k * dOut + r] = silu(Wg[e] x_col)[r] * (Wu[e] x_col)[r], e = ids[col * is + k]; x column stride xs4.
export function moeGuMcWGSL(kind) {
  return DQ + `
struct MP { dOut: u32, dIn: u32, K: u32, xs4: u32, hs: u32, is: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_gq: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> fx_gs: array<u32>;
@group(0) @binding(2) var<storage, read> fx_uq: array<vec4<u32>>;
@group(0) @binding(3) var<storage, read> fx_us: array<u32>;
@group(0) @binding(4) var<storage, read> fx_x: array<vec4<f32>>;
@group(0) @binding(5) var<storage, read_write> fx_h: array<f32>;
@group(0) @binding(6) var<storage, read> fx_ids: array<u32>;
@group(0) @binding(7) var<uniform> fx_P: MP;
var<workgroup> fx_red: array<f32, 256>;
var<workgroup> fx_red2: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let lid = l.x; let lane = lid & 31u;
  let r = wg.x * 8u + (lid >> 5u);
  let k = wg.y; let col = wg.z;
  var ag = 0.0; var au = 0.0;
  if (r < fx_P.dOut) {
    let e = fx_ids[col * fx_P.is + k];
    let nb = fx_P.dIn / 32u;
    let rb = (e * fx_P.dOut + r) * nb;
    for (var b = lane; b < nb; b += 32u) {
      let blk = rb + b;
      let xb4 = col * fx_P.xs4 + b * 8u;
      var d = 0.0;
      ${blockDot(kind, "fx_gq", "fx_gs", "fx_x")}
      ag += d;
      ${blockDot(kind, "fx_uq", "fx_us", "fx_x")}
      au += d;
    }
  }
  fx_red[lid] = ag; fx_red2[lid] = au;
  workgroupBarrier();
  for (var s = 16u; s > 0u; s >>= 1u) {
    if (lane < s) { fx_red[lid] += fx_red[lid + s]; fx_red2[lid] += fx_red2[lid + s]; }
    workgroupBarrier();
  }
  if (lane == 0u && r < fx_P.dOut) { fx_h[col * fx_P.hs + k * fx_P.dOut + r] = silu(fx_red[lid]) * fx_red2[lid]; }
}`;
}

// Expert down projections summed with their weights, per column (workgroup z):
// y[col * dOut + r] = sum_k w_k * (Wd[ids[col * is + k]] h[col][k])[r]; w_k = wts[col * ws + wOff + k] (its
// sigmoid when sig = 1: the shared expert's gate); h column stride xs4 vec4s.
export function moeDnMcWGSL(kind) {
  return DQ + `
struct MP { dOut: u32, dIn: u32, K: u32, sig: u32, wOff: u32, ws: u32, is: u32, xs4: u32 };
@group(0) @binding(0) var<storage, read> fx_qs: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> fx_sc: array<u32>;
@group(0) @binding(2) var<storage, read> fx_x: array<vec4<f32>>;
@group(0) @binding(3) var<storage, read_write> fx_y: array<f32>;
@group(0) @binding(4) var<storage, read> fx_ids: array<u32>;
@group(0) @binding(5) var<storage, read> fx_wts: array<f32>;
@group(0) @binding(6) var<uniform> fx_P: MP;
var<workgroup> fx_red: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let lid = l.x; let lane = lid & 31u;
  let r = wg.x * 8u + (lid >> 5u);
  let col = wg.z;
  var acc = 0.0;
  if (r < fx_P.dOut) {
    let nb = fx_P.dIn / 32u;
    for (var p = lane; p < fx_P.K * nb; p += 32u) {
      let k = p / nb; let b = p % nb;
      var wk = fx_wts[col * fx_P.ws + fx_P.wOff + k];
      if (fx_P.sig == 1u) { wk = sigm(wk); }
      let blk = (fx_ids[col * fx_P.is + k] * fx_P.dOut + r) * nb + b;
      let xb4 = col * fx_P.xs4 + (k * fx_P.dIn) / 4u + b * 8u;
      var d = 0.0;
      ${blockDot(kind, "fx_qs", "fx_sc", "fx_x")}
      acc += wk * d;
    }
  }
  fx_red[lid] = acc;
  workgroupBarrier();
  for (var s = 16u; s > 0u; s >>= 1u) {
    if (lane < s) { fx_red[lid] += fx_red[lid + s]; }
    workgroupBarrier();
  }
  if (lane == 0u && r < fx_P.dOut) { fx_y[col * fx_P.dOut + r] = fx_red[lid]; }
}`;
}

// Router per column (one workgroup each): MOE_ROUTE_WGSL on the column's logits (stride ls: the router's nExp
// logits and the shared gate's) -> ids / weights (stride K).
export const MOE_ROUTE_MC_WGSL = DQ + `
struct RP { nExp: u32, K: u32, ls: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_logits: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_ids: array<u32>;
@group(0) @binding(2) var<storage, read_write> fx_wts: array<f32>;
@group(0) @binding(3) var<uniform> fx_P: RP;
var<workgroup> fx_rv: array<f32, 256>;
var<workgroup> fx_ri: array<u32, 256>;
var<workgroup> fx_taken: array<u32, 1024>;
var<workgroup> fx_pk: array<f32, 32>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let lo = wg.x * fx_P.ls; let ko = wg.x * fx_P.K;
  var m = -3.0e38;
  for (var e = t; e < fx_P.nExp; e += 256u) { m = max(m, fx_logits[lo + e]); fx_taken[e] = 0u; }
  fx_rv[t] = m;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_rv[t] = max(fx_rv[t], fx_rv[t + s]); } workgroupBarrier(); }
  let mx = fx_rv[0];
  workgroupBarrier();
  var sum = 0.0;
  for (var e = t; e < fx_P.nExp; e += 256u) { sum += exp(fx_logits[lo + e] - mx); }
  fx_rv[t] = sum;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_rv[t] += fx_rv[t + s]; } workgroupBarrier(); }
  let tot = fx_rv[0];
  workgroupBarrier();
  for (var k = 0u; k < fx_P.K; k++) {
    var bv = -3.0e38; var bi = 0xffffffffu;
    for (var e = t; e < fx_P.nExp; e += 256u) {
      if (fx_taken[e] == 0u && fx_logits[lo + e] > bv) { bv = fx_logits[lo + e]; bi = e; }
    }
    fx_rv[t] = bv; fx_ri[t] = bi;
    workgroupBarrier();
    for (var s = 128u; s > 0u; s >>= 1u) {
      if (t < s) {
        let ov = fx_rv[t + s]; let oi = fx_ri[t + s];
        if (ov > fx_rv[t] || (ov == fx_rv[t] && oi < fx_ri[t])) { fx_rv[t] = ov; fx_ri[t] = oi; }
      }
      workgroupBarrier();
    }
    if (t == 0u) { let e = fx_ri[0]; fx_ids[ko + k] = e; fx_taken[e] = 1u; fx_pk[k] = exp(fx_logits[lo + e] - mx) / tot; }
    workgroupBarrier();
  }
  if (t == 0u) {
    var ws = 0.0;
    for (var k = 0u; k < fx_P.K; k++) { ws += fx_pk[k]; }
    ws = max(ws, 6.103515625e-5);
    for (var k = 0u; k < fx_P.K; k++) { fx_wts[ko + k] = fx_pk[k] / ws; }
  }
}`;

// HC site per column (HC_SITE_WGSL): workgroup (stream c, column). Strides: R / xn hc*d, a / b d, the inject
// partials hc*hc.
export const HC_SITE_MC_WGSL = DQ + `
struct NP { d: u32, hc: u32, eps: f32, comb: u32, inj: u32, p0: u32, p1: u32, p2: u32 };
@group(0) @binding(0) var<storage, read_write> fx_R: array<f32>;
@group(0) @binding(1) var<storage, read> fx_a: array<f32>;
@group(0) @binding(2) var<storage, read> fx_b: array<f32>;
@group(0) @binding(3) var<storage, read> fx_injPrev: array<f32>;
@group(0) @binding(4) var<storage, read> fx_w: array<f32>;
@group(0) @binding(5) var<storage, read_write> fx_xn: array<f32>;
@group(0) @binding(6) var<storage, read> fx_injW: array<f32>;
@group(0) @binding(7) var<storage, read_write> fx_injOut: array<f32>;
@group(0) @binding(8) var<uniform> fx_P: NP;
var<workgroup> fx_red: array<f32, 1024>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let c = wg.x; let col = wg.y; let d = fx_P.d; let hc = fx_P.hc; let o = c * d;
  let HD = d * hc;
  let rb = col * HD; let ab = col * d; let ib = col * hc * hc;
  var gsc = 0.0;
  if (fx_P.comb > 0u) {
    var inj = 0.0;
    for (var k = 0u; k < hc; k++) { inj += fx_injPrev[ib + c * hc + k]; }
    gsc = 2.0 * sigm(inj / f32(hc));
  }
  var ss = 0.0;
  for (var i = t; i < d; i += 256u) {
    var v = fx_R[rb + o + i];
    if (fx_P.comb > 0u) {
      var u = fx_a[ab + i];
      if (fx_P.comb == 2u) { u += fx_b[ab + i]; }
      v += u * gsc;
      fx_R[rb + o + i] = v;
    }
    ss += v * v;
  }
  fx_red[t] = ss;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] += fx_red[t + s]; } workgroupBarrier(); }
  let inv = 1.0 / sqrt(fx_red[0] / f32(d) + fx_P.eps);
  workgroupBarrier();
  var p0 = 0.0; var p1 = 0.0; var p2 = 0.0; var p3 = 0.0;
  for (var i = t; i < d; i += 256u) {
    let x = (fx_R[rb + o + i] * inv) * fx_w[o + i];
    fx_xn[rb + o + i] = x;
    if (fx_P.inj == 1u) {
      p0 += fx_injW[o + i] * x; p1 += fx_injW[HD + o + i] * x;
      p2 += fx_injW[2u * HD + o + i] * x; p3 += fx_injW[3u * HD + o + i] * x;
    }
  }
  fx_red[t] = p0; fx_red[256u + t] = p1; fx_red[512u + t] = p2; fx_red[768u + t] = p3;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) {
    if (t < s) { fx_red[t] += fx_red[t + s]; fx_red[256u + t] += fx_red[256u + t + s]; fx_red[512u + t] += fx_red[512u + t + s]; fx_red[768u + t] += fx_red[768u + t + s]; }
    workgroupBarrier();
  }
  if (fx_P.inj == 1u && t < hc) { fx_injOut[ib + t * hc + c] = fx_red[t * 256u]; }
}`;

// HC mixer part 2 per column (HC_MIX_WGSL; global y = column), for an f32 up projection.
export const HC_MIX_MC_WGSL = DQ + `
struct XP { d: u32, hc: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_xn: array<f32>;
@group(0) @binding(1) var<storage, read> fx_gate: array<f32>;
@group(0) @binding(2) var<storage, read_write> fx_mixed: array<f32>;
@group(0) @binding(3) var<uniform> fx_P: XP;
@compute @workgroup_size(256)
fn main(@builtin(global_invocation_id) g: vec3<u32>) {
  let i = g.x; let col = g.y;
  if (i >= fx_P.d) { return; }
  let cb = col * fx_P.d * fx_P.hc;
  var s = 0.0;
  for (var c = 0u; c < fx_P.hc; c++) { let j = cb + c * fx_P.d + i; s += fx_xn[j] * sigm(fx_gate[j]); }
  fx_mixed[col * fx_P.d + i] = s / f32(fx_P.hc);
}`;

// hc_combine per column (HC_COMBINE_WGSL; global y = column): R[col][c][i] += (a + b) * 2 sigmoid(inj[col][c] / hc),
// inj = the sum of the site's per-stream partials.
export const HC_COMBINE_MC_WGSL = DQ + `
struct CP { d: u32, hc: u32, useB: u32, p1: u32 };
@group(0) @binding(0) var<storage, read_write> fx_R: array<f32>;
@group(0) @binding(1) var<storage, read> fx_a: array<f32>;
@group(0) @binding(2) var<storage, read> fx_b: array<f32>;
@group(0) @binding(3) var<storage, read> fx_inj: array<f32>;
@group(0) @binding(4) var<uniform> fx_P: CP;
@compute @workgroup_size(256)
fn main(@builtin(global_invocation_id) g: vec3<u32>) {
  let j = g.x; let col = g.y;
  if (j >= fx_P.d * fx_P.hc) { return; }
  let c = j / fx_P.d; let i = j % fx_P.d;
  var v = fx_a[col * fx_P.d + i];
  if (fx_P.useB == 1u) { v += fx_b[col * fx_P.d + i]; }
  var inj = 0.0;
  let ib = col * fx_P.hc * fx_P.hc;
  for (var k = 0u; k < fx_P.hc; k++) { inj += fx_inj[ib + c * fx_P.hc + k]; }
  fx_R[col * fx_P.d * fx_P.hc + j] += v * (2.0 * sigm(inj / f32(fx_P.hc)));
}`;

// DeltaNet conv + l2 (DN_CONV_WGSL) over the pass's columns in order: one workgroup of dS threads per 128-channel
// head; each thread keeps its channel's convK - 1 older inputs in registers. Strides: xin xs, co cs.
export const DN_CONV_MC_WGSL = DQ + `
struct DP { K: u32, nQK: u32, dS: u32, eps: f32, xs: u32, cs: u32, p0: u32, p1: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> fx_xin: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_st: array<f32>;
@group(0) @binding(2) var<storage, read> fx_w: array<f32>;
@group(0) @binding(3) var<storage, read_write> fx_co: array<f32>;
@group(0) @binding(4) var<uniform> fx_P: DP;
@group(0) @binding(5) var<uniform> fx_F: McF;
var<workgroup> fx_red: array<f32, 128>;
@compute @workgroup_size(128)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let c = wg.x * fx_P.dS + t;
  let K1 = fx_P.K - 1u;
  var h: array<f32, 8>;   // convK - 1 <= 8
  for (var j = 0u; j < K1; j++) { h[j] = fx_st[c * K1 + j]; }
  for (var col = 0u; col < fx_F.nCols; col++) {
    let xv = fx_xin[col * fx_P.xs + c];
    var acc = fx_w[c * fx_P.K + K1] * xv;
    for (var j = 0u; j < K1; j++) { acc += fx_w[c * fx_P.K + j] * h[j]; }
    let v = silu(acc);
    for (var j = 0u; j + 1u < K1; j++) { h[j] = h[j + 1u]; }
    h[K1 - 1u] = xv;
    if (wg.x >= fx_P.nQK) { fx_co[col * fx_P.cs + c] = v; }
    else {
      fx_red[t] = v * v;
      workgroupBarrier();
      for (var s = 64u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] += fx_red[t + s]; } workgroupBarrier(); }
      fx_co[col * fx_P.cs + c] = v * (1.0 / max(sqrt(fx_red[0]), fx_P.eps));
      workgroupBarrier();
    }
  }
  for (var j = 0u; j < K1; j++) { fx_st[c * K1 + j] = h[j]; }
}`;

// The delta rule + gated output norm (DN_DELTA_WGSL) over the pass's columns in order: one workgroup per V head,
// thread t owns column j = t % 128 and rows i0 .. i0 + 63 of the head's S (i0 = 64 * (t / 128)), kept in registers.
// Strides: co cs, ba bs ([beta | alpha] per column), z zs from zOff (the [qkv | z] buffer), o / y os.
export const DN_DELTA_MC_WGSL = DQ + `
struct EP { nKH: u32, dS: u32, keyDim: u32, nVH: u32, eps: f32, cs: u32, bs: u32, zs: u32, zOff: u32, os: u32, p0: u32, p1: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> fx_co: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_S: array<f32>;
@group(0) @binding(2) var<storage, read> fx_ba: array<f32>;
@group(0) @binding(3) var<storage, read> fx_dta: array<f32>;
@group(0) @binding(4) var<storage, read> fx_z: array<f32>;
@group(0) @binding(5) var<storage, read> fx_nw: array<f32>;
@group(0) @binding(6) var<storage, read_write> fx_o: array<f32>;
@group(0) @binding(7) var<storage, read_write> fx_y: array<f32>;
@group(0) @binding(8) var<uniform> fx_P: EP;
@group(0) @binding(9) var<uniform> fx_F: McF;
var<workgroup> fx_kq: array<f32, 256>;
var<workgroup> fx_red: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let h = wg.x; let t = l.x; let dS = fx_P.dS;
  let j = t & 127u; let i0 = (t >> 7u) * 64u;
  let kh = h % fx_P.nKH;
  let base = h * dS * dS;
  var S: array<f32, 64>;
  for (var i = 0u; i < 64u; i++) { S[i] = fx_S[base + (i0 + i) * dS + j]; }
  for (var col = 0u; col < fx_F.nCols; col++) {
    let cb = col * fx_P.cs; let bb = col * fx_P.bs;
    workgroupBarrier();
    if (t < 128u) { fx_kq[t] = fx_co[cb + fx_P.keyDim + kh * dS + t]; }   // k
    else { fx_kq[t] = fx_co[cb + kh * dS + t - 128u]; }                  // q
    workgroupBarrier();
    let araw = fx_ba[bb + fx_P.nVH + h] + fx_dta[h];
    var sp = araw;
    if (araw <= 20.0) { sp = log(1.0 + exp(araw)); }
    let decay = exp(sp * fx_dta[fx_P.nVH + h]);
    let b = sigm(fx_ba[bb + h]);
    let v = fx_co[cb + 2u * fx_P.keyDim + h * dS + j];
    var vh = 0.0;
    for (var i = 0u; i < 64u; i++) {
      let s = S[i] * decay;
      S[i] = s;
      vh += s * fx_kq[i0 + i];
    }
    fx_red[t] = vh;
    workgroupBarrier();
    vh = (v - (fx_red[j] + fx_red[128u + j])) * b;
    workgroupBarrier();
    var acc = 0.0;
    for (var i = 0u; i < 64u; i++) {
      let s = S[i] + fx_kq[i0 + i] * vh;
      S[i] = s;
      acc += s * fx_kq[128u + i0 + i];
    }
    fx_red[t] = acc;
    workgroupBarrier();
    let ov = (fx_red[j] + fx_red[128u + j]) * (1.0 / sqrt(f32(dS)));
    workgroupBarrier();
    if (t < 128u) { fx_red[t] = ov * ov; } else { fx_red[t] = 0.0; }
    workgroupBarrier();
    for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] += fx_red[t + s]; } workgroupBarrier(); }
    let inv = 1.0 / sqrt(fx_red[0] / f32(dS) + fx_P.eps);
    if (t < 128u) {
      let oi = h * dS + j;
      fx_o[col * fx_P.os + oi] = ov;
      fx_y[col * fx_P.os + oi] = (ov * inv) * fx_nw[j] * sigm(fx_z[col * fx_P.zs + fx_P.zOff + oi]);
    }
  }
  for (var i = 0u; i < 64u; i++) { fx_S[base + (i0 + i) * dS + j] = S[i]; }
}`;

// Attention prep per column (attnPrepWGSL): workgroup (head, column), the column's rope table and position. q,
// the gate, k and v come from the [q | gate, k, v] buffer (column stride qs; k at kOff, v at vOff). kv: the cache
// format (engine/wgsl/qwen4exp.js kvStoreRow: the decode kernel's store, the same bits).
export function attnPrepMcWGSL(kv = "f32") {
  kvCheck(kv);
  const D = kvDecl(kv, true);
  return `${D.f16}
struct AP { nH: u32, nKV: u32, hd: u32, nRot: u32, eps: f32, p0: u32, p1: u32, p2: u32 };
struct MS { qs: u32, kOff: u32, vOff: u32, p0: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> fx_qkv: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_qn: array<f32>;
@group(0) @binding(2) var<storage, read_write> fx_kc: ${D.k};
@group(0) @binding(3) var<storage, read_write> fx_vc: ${D.v};
@group(0) @binding(4) var<storage, read> fx_qw: array<f32>;
@group(0) @binding(5) var<storage, read> fx_kw: array<f32>;
@group(0) @binding(6) var<storage, read> fx_cs: array<vec4<f32>>;   // [col][64] (cos, sin, 0, 0)
@group(0) @binding(7) var<uniform> fx_P: AP;
@group(0) @binding(8) var<uniform> fx_S: MS;
@group(0) @binding(9) var<uniform> fx_F: McF;
var<workgroup> fx_red: array<f32, 256>;
var<workgroup> fx_hv: array<f32, 256>;${kv === "f16" ? "\nvar<workgroup> fx_kr: array<f32, 256>;" : ""}
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let g = wg.x; let col = wg.y; let hd = fx_P.hd;
  let cb = col * fx_S.qs;
  let isQ = g < fx_P.nH;
  var x = 0.0;
  if (t < hd) {
    if (isQ) { x = fx_qkv[cb + g * 2u * hd + t]; } else { x = fx_qkv[cb + fx_S.kOff + (g - fx_P.nH) * hd + t]; }
  }
  fx_red[t] = x * x;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] += fx_red[t + s]; } workgroupBarrier(); }
  let inv = 1.0 / sqrt(fx_red[0] / f32(hd) + fx_P.eps);
  var nv = 0.0;
  if (t < hd) { if (isQ) { nv = (x * inv) * fx_qw[t]; } else { nv = (x * inv) * fx_kw[t]; } }
  fx_hv[t] = nv;
  workgroupBarrier();
  let hr = fx_P.nRot / 2u;
  var r = nv;
  if (t < hr) { let c = fx_cs[col * 64u + t]; r = nv * c.x - fx_hv[t + hr] * c.y; }
  else if (t < fx_P.nRot) { let c = fx_cs[col * 64u + t - hr]; r = fx_hv[t - hr] * c.y + nv * c.x; }
  if (isQ && t < hd) { fx_qn[col * fx_P.nH * hd + g * hd + t] = r; }
  let h = select(g - fx_P.nH, 0u, isQ);
  let vb = cb + fx_S.vOff + h * hd;${kvStoreRow(kv, { isK: "!isQ", t: "t", hd: "hd", o0: "(fx_F.pos + col) * fx_P.nKV * hd + h * hd", r: "r", vAt: (e) => `fx_qkv[vb + ${e}]`, K: "fx_kc", V: "fx_vc", st: "fx_kr" })}
}`;
}
export const ATTN_PREP_MC_WGSL = attnPrepMcWGSL("f32");

// Attention per column (attnDecWGSL): workgroup (q head, column). gather = 0: dense causal over cells
// 0 .. pos + col; gather = stride: over the column's row list from qsa_select (sel[col * stride] = n, then n rows
// ascending). With the list 0 .. p the gathered kernel does the dense arithmetic in the same order: the same bits.
// At most ATTN_MAX_CELLS cells per column either way (dense runs only while every column has <= 2051 cells), so the
// scores fit in workgroup memory. The gate is read from the [q | gate, k, v] buffer (column stride MS.qs). Out:
// [col][nH * hd]. The gathered kernel does not read the frame (binding 6 is not in its layout); the decode engine
// runs it with one column. kv: the cache format (as the decode kernel reads it).
export const ATTN_MAX_CELLS = 2052;
export function attnDecMcWGSL({ gather = 0, kv = "f32" } = {}) {
  kvCheck(kv);
  const D = kvDecl(kv, false);
  const row = (p) => gather ? `fx_sel[ib + ${p}]` : `${p}`;
  return DQ + D.f16 + (kv === "f16" ? KV_F16_READ : "") + `
struct AP { nH: u32, nKV: u32, hd: u32, nRot: u32, eps: f32, p0: u32, p1: u32, p2: u32 };
struct MS { qs: u32, kOff: u32, vOff: u32, p0: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> fx_qn: array<vec4<f32>>;
@group(0) @binding(1) var<storage, read> fx_kc: ${D.k};
@group(0) @binding(2) var<storage, read> fx_vc: ${D.v};
@group(0) @binding(3) var<storage, read> fx_qkv: array<f32>;
@group(0) @binding(4) var<storage, read_write> fx_out: array<f32>;
@group(0) @binding(5) var<uniform> fx_P: AP;
${gather ? "" : "@group(0) @binding(6) var<uniform> fx_F: McF;\n"}@group(0) @binding(7) var<uniform> fx_S: MS;${gather ? `
@group(0) @binding(8) var<storage, read> fx_sel: array<u32>;` : ""}
var<workgroup> fx_sc: array<f32, ${ATTN_MAX_CELLS}>;
var<workgroup> fx_red: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let h = wg.x; let col = wg.y; let hd = fx_P.hd; let hd4 = hd / 4u;
  let kvh = h / (fx_P.nH / fx_P.nKV);
  ${gather ? `let ib = col * ${gather}u + 1u; let n = min(fx_sel[col * ${gather}u], ${ATTN_MAX_CELLS}u);` : `let n = min(fx_F.pos + col + 1u, ${ATTN_MAX_CELLS}u);`}
  let qb = col * fx_P.nH * hd4 + h * hd4;
  let scale = 1.0 / sqrt(f32(hd));
  var m = -3.0e38;
  for (var p = t; p < n; p += 256u) {
    let ko = (${row("p")} * fx_P.nKV + kvh) * hd4;
    var a = 0.0;
    for (var i = 0u; i < hd4; i++) { a += dot(fx_qn[qb + i], ${kvK4(kv, "fx_kc", "ko + i")}); }
    a *= scale;
    fx_sc[p] = a;
    m = max(m, a);
  }
  fx_red[t] = m;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] = max(fx_red[t], fx_red[t + s]); } workgroupBarrier(); }
  let mx = fx_red[0];
  workgroupBarrier();
  var sum = 0.0;
  for (var p = t; p < n; p += 256u) { let e = exp(fx_sc[p] - mx); fx_sc[p] = e; sum += e; }
  fx_red[t] = sum;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] += fx_red[t + s]; } workgroupBarrier(); }
  let tot = fx_red[0];
  if (t < hd) {
    var acc = 0.0;
    for (var p = 0u; p < n; p++) { let ve = (${row("p")} * fx_P.nKV + kvh) * hd + t; acc += fx_sc[p] * ${kvV1(kv, "fx_vc", "ve")}; }
    fx_out[col * fx_P.nH * hd + h * hd + t] = (acc / tot) * sigm(fx_qkv[col * fx_S.qs + h * 2u * hd + hd + t]);
  }
}`;
}

// The n-gram (PLE) injection (PLE_WGSL) over the pass's columns in order: one workgroup per stream, as the decode
// kernel. Thread t owns the channels o + t, o + t + 256, .. of every column, so a column's ring write (slot pos %
// slots) is seen by the later columns' taps (pos - back) in program order, and the ring ends as decode leaves it.
// Strides: R / pk hc*d, pv d. dbg as the decode kernel's (overwritten per column; the per-layer checks read it).
export const PLE_MC_WGSL = DQ + `
struct PP { d: u32, K: u32, dil: u32, slots: u32, eps: f32, hc: u32, p1: u32, p2: u32 };
${MCF}
@group(0) @binding(0) var<storage, read_write> fx_R: array<f32>;
@group(0) @binding(1) var<storage, read> fx_pk: array<f32>;
@group(0) @binding(2) var<storage, read> fx_pv: array<f32>;
@group(0) @binding(3) var<storage, read> fx_pw: array<f32>;            // [nk | nq | nc] (hc*d each) then cw [hc*d][K]
@group(0) @binding(4) var<storage, read_write> fx_ring: array<f32>;
@group(0) @binding(5) var<storage, read_write> fx_dbg: array<f32>;     // [g (hc*d) | conv (hc*d) | gate (hc)]
@group(0) @binding(6) var<uniform> fx_P: PP;
@group(0) @binding(7) var<uniform> fx_F: McF;
var<workgroup> fx_r1: array<f32, 256>;
var<workgroup> fx_r2: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let c = wg.x; let d = fx_P.d; let o = c * d;
  let HD = d * fx_P.hc;   // ring row: every stream's channels
  let NQ = HD; let NC = 2u * HD; let CW = 3u * HD;
  for (var col = 0u; col < fx_F.nCols; col++) {
    let pos = fx_F.pos + col;
    let rb = col * HD; let vb = col * d;
    workgroupBarrier();
    var sk = 0.0; var sq = 0.0;
    for (var i = t; i < d; i += 256u) { let a = fx_pk[rb + o + i]; let b = fx_R[rb + o + i]; sk += a * a; sq += b * b; }
    fx_r1[t] = sk; fx_r2[t] = sq;
    workgroupBarrier();
    for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_r1[t] += fx_r1[t + s]; fx_r2[t] += fx_r2[t + s]; } workgroupBarrier(); }
    let ik = 1.0 / sqrt(fx_r1[0] / f32(d) + fx_P.eps);
    let iq = 1.0 / sqrt(fx_r2[0] / f32(d) + fx_P.eps);
    workgroupBarrier();
    var dt = 0.0;
    for (var i = t; i < d; i += 256u) { dt += ((fx_pk[rb + o + i] * ik) * fx_pw[o + i]) * ((fx_R[rb + o + i] * iq) * fx_pw[NQ + o + i]); }
    fx_r1[t] = dt;
    workgroupBarrier();
    for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_r1[t] += fx_r1[t + s]; } workgroupBarrier(); }
    let sv = fx_r1[0] / sqrt(f32(d));
    let gate = sigm(sign(sv) * sqrt(max(abs(sv), 1e-6)));
    workgroupBarrier();
    var sg = 0.0;
    for (var i = t; i < d; i += 256u) { let g = fx_pv[vb + i] * gate; sg += g * g; }
    fx_r1[t] = sg;
    workgroupBarrier();
    for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_r1[t] += fx_r1[t + s]; } workgroupBarrier(); }
    let ig = 1.0 / sqrt(fx_r1[0] / f32(d) + fx_P.eps);
    if (t == 0u) { fx_dbg[2u * HD + c] = gate; }
    let slot = pos % fx_P.slots;
    for (var i = t; i < d; i += 256u) {
      let ch = o + i;
      let g = fx_pv[vb + i] * gate;
      let nv = (g * ig) * fx_pw[NC + ch];
      fx_ring[slot * HD + ch] = nv;
      var acc = fx_pw[CW + ch * fx_P.K + fx_P.K - 1u] * nv;
      for (var k = 0u; k + 1u < fx_P.K; k++) {
        let back = (fx_P.K - 1u - k) * fx_P.dil;
        if (pos >= back) { acc += fx_pw[CW + ch * fx_P.K + k] * fx_ring[((pos - back) % fx_P.slots) * HD + ch]; }
      }
      let cv = silu(acc);
      fx_dbg[ch] = g; fx_dbg[HD + ch] = cv;
      fx_R[rb + ch] = fx_R[rb + ch] + (g + cv);
    }
  }
}`;

// Greedy head per column: ARGMAX1 / ARGMAX2 (u32 partials (index, bits of the value)), workgroup y (stage 1) or
// x (stage 2) = column.
export const ARGMAX1_MC_WGSL = `
struct AM { n: u32, per: u32, nPart: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_lg: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_part: array<vec2<u32>>;
@group(0) @binding(2) var<uniform> fx_P: AM;
var<workgroup> fx_rv: array<f32, 256>;
var<workgroup> fx_ri: array<u32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let lo = wg.x * fx_P.per; let hi = min(lo + fx_P.per, fx_P.n); let cb = wg.y * fx_P.n;
  var bv = -3.0e38; var bi = 0xffffffffu;
  for (var i = lo + t; i < hi; i += 256u) { let v = fx_lg[cb + i]; if (v > bv) { bv = v; bi = i; } }
  fx_rv[t] = bv; fx_ri[t] = bi;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) {
    if (t < s) { let ov = fx_rv[t + s]; let oi = fx_ri[t + s]; if (ov > fx_rv[t] || (ov == fx_rv[t] && oi < fx_ri[t])) { fx_rv[t] = ov; fx_ri[t] = oi; } }
    workgroupBarrier();
  }
  if (t == 0u) { fx_part[wg.y * fx_P.nPart + wg.x] = vec2<u32>(fx_ri[0], bitcast<u32>(fx_rv[0])); }
}`;
export const ARGMAX2_MC_WGSL = `
struct AM { n: u32, per: u32, nPart: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_part: array<vec2<u32>>;
@group(0) @binding(1) var<storage, read_write> fx_outv: array<u32>;
@group(0) @binding(2) var<uniform> fx_P: AM;
var<workgroup> fx_rv: array<f32, 256>;
var<workgroup> fx_ri: array<u32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let pb = wg.x * fx_P.nPart;
  var bv = -3.0e38; var bi = 0xffffffffu;
  for (var i = t; i < fx_P.nPart; i += 256u) { let p = fx_part[pb + i]; let pv = bitcast<f32>(p.y); if (pv > bv || (pv == bv && p.x < bi)) { bv = pv; bi = p.x; } }
  fx_rv[t] = bv; fx_ri[t] = bi;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) {
    if (t < s) { let ov = fx_rv[t + s]; let oi = fx_ri[t + s]; if (ov > fx_rv[t] || (ov == fx_rv[t] && oi < fx_ri[t])) { fx_rv[t] = ov; fx_ri[t] = oi; } }
    workgroupBarrier();
  }
  if (t == 0u) { fx_outv[2u * wg.x] = fx_ri[0]; fx_outv[2u * wg.x + 1u] = bitcast<u32>(fx_rv[0]); }
}`;
