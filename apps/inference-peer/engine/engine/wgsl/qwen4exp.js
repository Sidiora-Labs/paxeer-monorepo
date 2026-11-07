// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/qwen4exp.js (MIT; see LICENSE-ENGINE).
// WGSL for the Qwen3.8-Flash-Next (llama.cpp arch "qwen4exp") decode path, engine/qwen4exp.js.
// Roadmap 33 section 2 "New code" and PR plan item 4. Every kernel is its own module with its bindings in group 0,
// so a pipeline's "auto" layout is exactly its bindings (none are left unused).
//
// Weight formats are the loader's (engine/gguf.js convertEntry): "q4" = Q4_0 repacked (16 nibble bytes per 32-block,
// byte j holds element j low and element j + 16 high; f16 scales two per word), "q8" = Q8_0 repacked (32 int8 per
// block, four per word; f16 scales two per word), "f32" = row-major floats. Activations are f32 throughout (the
// reference's --act f32 mode, tests/reference/ref_q38flash.mjs).

import { F16_WGSL } from "./f16.js";

// Dequant helpers: 4 elements of a q4 word (low or high nibbles) and of a q8 word, as vec4<f32>.
const DQ = /* wgsl */ `
fn q4lo(w: u32) -> vec4<f32> { return vec4<f32>(f32(w & 15u), f32((w >> 8u) & 15u), f32((w >> 16u) & 15u), f32((w >> 24u) & 15u)) - vec4<f32>(8.0); }
fn q4hi(w: u32) -> vec4<f32> { return vec4<f32>(f32((w >> 4u) & 15u), f32((w >> 12u) & 15u), f32((w >> 20u) & 15u), f32((w >> 28u) & 15u)) - vec4<f32>(8.0); }
fn i8x4(u: u32) -> vec4<f32> { let v = bitcast<i32>(u); return vec4<f32>(f32((v << 24u) >> 24u), f32((v << 16u) >> 24u), f32((v << 8u) >> 24u), f32(v >> 24u)); }
fn sigm(v: f32) -> f32 { return 1.0 / (1.0 + exp(-v)); }
fn silu(v: f32) -> f32 { return v / (1.0 + exp(-v)); }
`;

// The 32-wide block dot for one row block `blk` (row * nb + b) against x[xb4 .. xb4 + 8) (vec4 units).
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

// Lane reduction: 8 rows x 32 lanes per 256-thread workgroup, red[lid] holds each lane's partial sum.
const LANE_REDUCE = (R = "fx_red") => `
  workgroupBarrier();
  for (var s = 16u; s > 0u; s >>= 1u) {
    if (lane < s) { ${R}[lid] += ${R}[lid + s]; }
    workgroupBarrier();
  }`;

// GEMV y[yOff + r] = epilogue(W[rowOff + r, :] . x[xOff4*4 ..]), r < dOut. LANES threads per row (a power of two,
// 8..256; gemvLanes picks it from the row length), 256 / LANES rows per workgroup.
// mode 0: store; 1: silu(acc * scale) (the HC down projection: silu(x / hc)).
// Workgroups: x = ceil(dOut / rows per workgroup) up to 32768, then y.
export function gemvLanes(kind, dOut, dIn) {
  const units = dIn / 32;                                // 32-wide blocks (8 vec4 of an f32 row)
  if (dOut <= 16) return 256;                            // a few long rows (the HC inject): spread each row
  let l = 8;
  while (l < 256 && l * 2 < units) l *= 2;               // about two blocks per lane
  return l;
}
// Rows per lane group: quantized matrices with many rows take 4, so each lane loads its x block once for 4 rows
// (x is read 8 times per weight byte otherwise).
export const gemvRows = (kind, dOut) => (kind !== "f32" && dOut >= 2048 ? 4 : 1);
// MIX (q4/q8, R = hc): the HC up projection fused with the mix: lane group g owns column i = g of every stream
// (rows i + k * stride, stride = d in P.pad), writes the gates (for the per-layer check) and
// mixed[i] = mean over k of xn[k * d + i] * sigmoid(gate_k).
export function gemvWGSL(kind, LANES = 32, R = 1, MIX = false) {
  const RPW = 256 / LANES, RR = [...Array(R).keys()];
  const rowOf = (k) => (MIX ? `r0 + ${k}u * fx_P.pad` : `r0 + ${k}u`);
  const inRange = (k) => (MIX ? "r0 < fx_P.pad" : `r0 + ${k}u < fx_P.dOut`);
  const W = kind === "f32" ? `@group(0) @binding(0) var<storage, read> fx_w: array<vec4<f32>>;` : `
@group(0) @binding(0) var<storage, read> fx_qs: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> fx_sc: array<u32>;`;
  // one row's block against the x block held in xv0..xv7
  const dotX = (blk) => kind === "q4" ? `{
        let s = unpack2x16float(fx_sc[${blk} >> 1u])[${blk} & 1u]; let wv = fx_qs[${blk}];
        d = s * (dot(q4lo(wv.x), xv0) + dot(q4lo(wv.y), xv1) + dot(q4lo(wv.z), xv2) + dot(q4lo(wv.w), xv3)
               + dot(q4hi(wv.x), xv4) + dot(q4hi(wv.y), xv5) + dot(q4hi(wv.z), xv6) + dot(q4hi(wv.w), xv7));
      }` : `{
        let s = unpack2x16float(fx_sc[${blk} >> 1u])[${blk} & 1u]; let wa = fx_qs[${blk} * 2u]; let wb = fx_qs[${blk} * 2u + 1u];
        d = s * (dot(i8x4(wa.x), xv0) + dot(i8x4(wa.y), xv1) + dot(i8x4(wa.z), xv2) + dot(i8x4(wa.w), xv3)
               + dot(i8x4(wb.x), xv4) + dot(i8x4(wb.y), xv5) + dot(i8x4(wb.z), xv6) + dot(i8x4(wb.w), xv7));
      }`;
  const body = kind === "f32" ? `
    let n4 = fx_P.dIn / 4u;
    let ro = (fx_P.rowOff + r0) * n4;
    for (var i = lane; i < n4; i += ${LANES}u) { acc0 += dot(fx_w[ro + i], fx_x[fx_P.xOff4 + i]); }` : `
    let nb = fx_P.dIn / 32u;
    for (var b = lane; b < nb; b += ${LANES}u) {
      let xb4 = fx_P.xOff4 + b * 8u;
      let xv0 = fx_x[xb4]; let xv1 = fx_x[xb4 + 1u]; let xv2 = fx_x[xb4 + 2u]; let xv3 = fx_x[xb4 + 3u];
      let xv4 = fx_x[xb4 + 4u]; let xv5 = fx_x[xb4 + 5u]; let xv6 = fx_x[xb4 + 6u]; let xv7 = fx_x[xb4 + 7u];
      var d = 0.0;
${RR.map((k) => `      if (${inRange(k)}) { let blk = (fx_P.rowOff + ${rowOf(k)}) * nb + b; ${dotX("blk")} acc${k} += d; }`).join("\n")}
    }`;
  return DQ + `
struct GP { dOut: u32, dIn: u32, rowOff: u32, xOff4: u32, yOff: u32, mode: u32, scale: f32, pad: u32 };
${W}
@group(0) @binding(2) var<storage, read> fx_x: array<vec4<f32>>;
@group(0) @binding(3) var<storage, read_write> fx_y: array<f32>;
@group(0) @binding(4) var<uniform> fx_P: GP;${MIX ? `
@group(0) @binding(5) var<storage, read> fx_xn: array<f32>;
@group(0) @binding(6) var<storage, read_write> fx_mixed: array<f32>;` : ""}
var<workgroup> fx_red: array<f32, ${256 * R}>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let lid = l.x; let lane = lid % ${LANES}u;
  let r0 = ((wg.y * 32768u + wg.x) * ${RPW}u + lid / ${LANES}u)${MIX ? "" : ` * ${R}u`};
  ${RR.map((k) => `var acc${k} = 0.0;`).join(" ")}
  if (${MIX ? "r0 < fx_P.pad" : "r0 < fx_P.dOut"}) {${body}
  }
  ${RR.map((k) => `fx_red[${k * 256}u + lid] = acc${k};`).join(" ")}
  workgroupBarrier();
  for (var s = ${LANES / 2}u; s > 0u; s >>= 1u) {
    if (lane < s) { ${RR.map((k) => `fx_red[${k * 256}u + lid] += fx_red[${k * 256}u + lid + s];`).join(" ")} }
    workgroupBarrier();
  }
${MIX ? `  if (lane == 0u && r0 < fx_P.pad) {
    var m = 0.0;
${RR.map((k) => `    { let g = fx_red[${k * 256}u + lid]; let row = ${rowOf(k)}; fx_y[fx_P.yOff + row] = g; m += fx_xn[row] * sigm(g); }`).join("\n")}
    fx_mixed[r0] = m / ${R}.0;
  }` : `  if (lane < ${R}u && r0 + lane < fx_P.dOut) {
    var v = fx_red[lane * 256u + lid - lane];
    if (fx_P.mode == 1u) { v = silu(v * fx_P.scale); }
    fx_y[fx_P.yOff + r0 + lane] = v;
  }`}
}`;
}

// Routed (or shared) experts, gate and up together: h[k * dOut + r] = silu(Wg[e] x)[r] * (Wu[e] x)[r], e = ids[k];
// workgroup y = k. The shared expert runs it with ids = [0] and K = 1.
export function moeGuWGSL(kind) {
  return DQ + `
struct MP { dOut: u32, dIn: u32, K: u32, sig: u32, wOff: u32, p0: u32, p1: u32, p2: u32 };
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
  let k = wg.y;
  var ag = 0.0; var au = 0.0;
  if (r < fx_P.dOut) {
    let e = fx_ids[k];
    let nb = fx_P.dIn / 32u;
    let rb = (e * fx_P.dOut + r) * nb;
    for (var b = lane; b < nb; b += 32u) {
      let blk = rb + b;
      let xb4 = b * 8u;
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
  if (lane == 0u && r < fx_P.dOut) { fx_h[k * fx_P.dOut + r] = silu(fx_red[lid]) * fx_red2[lid]; }
}`;
}

// Expert down projections summed with their weights: y[r] = sum_k w_k * (Wd[ids[k]] h[k])[r]; w_k = wts[k], or
// sigmoid(wts[k]) when sig = 1 (the shared expert's gate, sigmoid(ffn_gate_inp_shexp . x)).
export function moeDnWGSL(kind) {
  return DQ + `
struct MP { dOut: u32, dIn: u32, K: u32, sig: u32, wOff: u32, p0: u32, p1: u32, p2: u32 };
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
  var acc = 0.0;
  if (r < fx_P.dOut) {
    let nb = fx_P.dIn / 32u;
    for (var p = lane; p < fx_P.K * nb; p += 32u) {
      let k = p / nb; let b = p % nb;
      var wk = fx_wts[fx_P.wOff + k];
      if (fx_P.sig == 1u) { wk = sigm(wk); }
      let blk = (fx_ids[k] * fx_P.dOut + r) * nb + b;
      let xb4 = (k * fx_P.dIn) / 4u + b * 8u;
      var d = 0.0;
      ${blockDot(kind, "fx_qs", "fx_sc", "fx_x")}
      acc += wk * d;
    }
  }
  fx_red[lid] = acc;
  ${LANE_REDUCE()}
  if (lane == 0u && r < fx_P.dOut) { fx_y[r] = fx_red[lid]; }
}`;
}

// Router: softmax over all nExp logits, top-K (equal values: lowest index first), weights renormalised to sum 1
// (the sum clamped at 6.103515625e-5 as llama.cpp's build_moe_ffn). One workgroup.
export const MOE_ROUTE_WGSL = DQ + `
struct RP { nExp: u32, K: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_logits: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_ids: array<u32>;
@group(0) @binding(2) var<storage, read_write> fx_wts: array<f32>;
@group(0) @binding(3) var<uniform> fx_P: RP;
var<workgroup> fx_rv: array<f32, 256>;
var<workgroup> fx_ri: array<u32, 256>;
var<workgroup> fx_taken: array<u32, 1024>;
var<workgroup> fx_pk: array<f32, 32>;
@compute @workgroup_size(256)
fn main(@builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x;
  var m = -3.0e38;
  for (var e = t; e < fx_P.nExp; e += 256u) { m = max(m, fx_logits[e]); fx_taken[e] = 0u; }
  fx_rv[t] = m;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_rv[t] = max(fx_rv[t], fx_rv[t + s]); } workgroupBarrier(); }
  let mx = fx_rv[0];
  workgroupBarrier();
  var sum = 0.0;
  for (var e = t; e < fx_P.nExp; e += 256u) { sum += exp(fx_logits[e] - mx); }
  fx_rv[t] = sum;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_rv[t] += fx_rv[t + s]; } workgroupBarrier(); }
  let tot = fx_rv[0];
  workgroupBarrier();
  for (var k = 0u; k < fx_P.K; k++) {
    var bv = -3.0e38; var bi = 0xffffffffu;
    for (var e = t; e < fx_P.nExp; e += 256u) {
      if (fx_taken[e] == 0u && fx_logits[e] > bv) { bv = fx_logits[e]; bi = e; }
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
    if (t == 0u) { let e = fx_ri[0]; fx_ids[k] = e; fx_taken[e] = 1u; fx_pk[k] = exp(fx_logits[e] - mx) / tot; }
    workgroupBarrier();
  }
  if (t == 0u) {
    var ws = 0.0;
    for (var k = 0u; k < fx_P.K; k++) { ws += fx_pk[k]; }
    ws = max(ws, 6.103515625e-5);
    for (var k = 0u; k < fx_P.K; k++) { fx_wts[k] = fx_pk[k] / ws; }
  }
}`;

// HC site: [the previous site's combine] + the mixer's per-stream RMS norm + [the inject projection, as per-stream
// partial sums]. One workgroup per stream c:
//   comb > 0:  R[c] += (a + b if comb == 2) * 2 * sigmoid(inj[c] / hc), inj[c] = sum over c' of injPrev[c * hc + c']
//   xn[c] = rmsnorm(R[c]) * w[c]                      (the converter folded 1 + w)
//   inj = 1:   injOut[r * hc + c] = injW[r][c * d ..] . xn[c]   (the consumer sums over c)
// Combining stream c needs only stream c, so the combine before a mixer folds into its norm (vLLM's delayed
// combine; roadmap 33 prior art) and the inject GEMV of 4 long rows becomes 4 per-stream dots.
export const HC_SITE_WGSL = DQ + `
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
  let t = l.x; let c = wg.x; let d = fx_P.d; let hc = fx_P.hc; let o = c * d;
  var gsc = 0.0;
  if (fx_P.comb > 0u) {
    var inj = 0.0;
    for (var k = 0u; k < hc; k++) { inj += fx_injPrev[c * hc + k]; }
    gsc = 2.0 * sigm(inj / f32(hc));
  }
  var ss = 0.0;
  for (var i = t; i < d; i += 256u) {
    var v = fx_R[o + i];
    if (fx_P.comb > 0u) {
      var u = fx_a[i];
      if (fx_P.comb == 2u) { u += fx_b[i]; }
      v += u * gsc;
      fx_R[o + i] = v;
    }
    ss += v * v;
  }
  fx_red[t] = ss;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] += fx_red[t + s]; } workgroupBarrier(); }
  let inv = 1.0 / sqrt(fx_red[0] / f32(d) + fx_P.eps);
  workgroupBarrier();
  var p0 = 0.0; var p1 = 0.0; var p2 = 0.0; var p3 = 0.0;
  let HD = d * hc;
  for (var i = t; i < d; i += 256u) {
    let x = (fx_R[o + i] * inv) * fx_w[o + i];
    fx_xn[o + i] = x;
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
  if (fx_P.inj == 1u && t < hc) { fx_injOut[t * hc + c] = fx_red[t * 256u]; }
}`;

// HC mixer, part 2: mixed[i] = mean over c of xn[c][i] * sigmoid(gate[c][i]).
export const HC_MIX_WGSL = DQ + `
struct XP { d: u32, hc: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_xn: array<f32>;
@group(0) @binding(1) var<storage, read> fx_gate: array<f32>;
@group(0) @binding(2) var<storage, read_write> fx_mixed: array<f32>;
@group(0) @binding(3) var<uniform> fx_P: XP;
@compute @workgroup_size(256)
fn main(@builtin(global_invocation_id) g: vec3<u32>) {
  let i = g.x;
  if (i >= fx_P.d) { return; }
  var s = 0.0;
  for (var c = 0u; c < fx_P.hc; c++) { let j = c * fx_P.d + i; s += fx_xn[j] * sigm(fx_gate[j]); }
  fx_mixed[i] = s / f32(fx_P.hc);
}`;

// hc_combine: R[c][i] += (a[i] + b[i]) * 2 * sigmoid(inj[c] / hc) (b only when useB = 1: the MoE's shared expert),
// inj[c] = the sum of HC_SITE's partials injPrev[c * hc + c'].
export const HC_COMBINE_WGSL = DQ + `
struct CP { d: u32, hc: u32, useB: u32, p1: u32 };
@group(0) @binding(0) var<storage, read_write> fx_R: array<f32>;
@group(0) @binding(1) var<storage, read> fx_a: array<f32>;
@group(0) @binding(2) var<storage, read> fx_b: array<f32>;
@group(0) @binding(3) var<storage, read> fx_inj: array<f32>;
@group(0) @binding(4) var<uniform> fx_P: CP;
@compute @workgroup_size(256)
fn main(@builtin(global_invocation_id) g: vec3<u32>) {
  let j = g.x;
  if (j >= fx_P.d * fx_P.hc) { return; }
  let c = j / fx_P.d; let i = j % fx_P.d;
  var v = fx_a[i];
  if (fx_P.useB == 1u) { v += fx_b[i]; }
  var inj = 0.0;
  for (var k = 0u; k < fx_P.hc; k++) { inj += fx_inj[c * fx_P.hc + k]; }
  fx_R[j] += v * (2.0 * sigm(inj / f32(fx_P.hc)));
}`;

// Gated DeltaNet, step 1: one workgroup of dS threads per 128-channel head of the conv input [q | k | v]: the causal
// conv over the last convK inputs of each channel (state: the convK - 1 older inputs, oldest first; w is
// [ch][convK], the newest tap last), silu, then for the q and k heads (workgroups < 2 * nKH) the l2 normalisation
// v / max(|v|, eps).
export const DN_CONV_WGSL = DQ + `
struct DP { K: u32, nQK: u32, dS: u32, eps: f32 };
@group(0) @binding(0) var<storage, read> fx_xin: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_st: array<f32>;
@group(0) @binding(2) var<storage, read> fx_w: array<f32>;
@group(0) @binding(3) var<storage, read_write> fx_co: array<f32>;
@group(0) @binding(4) var<uniform> fx_P: DP;
var<workgroup> fx_red: array<f32, 128>;
@compute @workgroup_size(128)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let c = wg.x * fx_P.dS + t;
  let K1 = fx_P.K - 1u;
  let xv = fx_xin[c];
  var acc = fx_w[c * fx_P.K + K1] * xv;
  for (var j = 0u; j < K1; j++) { acc += fx_w[c * fx_P.K + j] * fx_st[c * K1 + j]; }
  let v = silu(acc);
  for (var j = 0u; j + 1u < K1; j++) { fx_st[c * K1 + j] = fx_st[c * K1 + j + 1u]; }
  fx_st[c * K1 + K1 - 1u] = xv;
  if (wg.x >= fx_P.nQK) { fx_co[c] = v; return; }
  fx_red[t] = v * v;
  workgroupBarrier();
  for (var s = 64u; s > 0u; s >>= 1u) { if (t < s) { fx_red[t] += fx_red[t + s]; } workgroupBarrier(); }
  fx_co[c] = v * (1.0 / max(sqrt(fx_red[0]), fx_P.eps));
}`;

// Step 2: the delta rule, then the gated output norm; one workgroup per V head, thread j owns column j of the
// head's state S[i][j] (dS x dS). g = softplus(alpha + dt_bias) * A, decay = exp(g), beta = sigmoid(beta_raw):
//   S *= decay; vhat = (v - S^T k) * beta; S += k vhat^T; o = S^T q / sqrt(dS)
//   y = rmsnorm(o) * ssm_norm * sigmoid(z)   (qwen4exp: sigmoid, where Qwen3.5 has silu)
// The V-head order is the Qwen3.5 converter's: V head h reads QK head h % nKH. ba = [beta_raw (nVH) | alpha (nVH)],
// dta = [dt_bias (nVH) | A (nVH)]; o keeps the raw delta-rule output (the per-layer check's attn_output).
export const DN_DELTA_WGSL = DQ + `
struct EP { nKH: u32, dS: u32, keyDim: u32, nVH: u32, eps: f32, p0: u32, p1: u32, p2: u32 };
@group(0) @binding(0) var<storage, read> fx_co: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_S: array<f32>;
@group(0) @binding(2) var<storage, read> fx_ba: array<f32>;
@group(0) @binding(3) var<storage, read> fx_dta: array<f32>;
@group(0) @binding(4) var<storage, read> fx_z: array<f32>;
@group(0) @binding(5) var<storage, read> fx_nw: array<f32>;
@group(0) @binding(6) var<storage, read_write> fx_o: array<f32>;
@group(0) @binding(7) var<storage, read_write> fx_y: array<f32>;
@group(0) @binding(8) var<uniform> fx_P: EP;
var<workgroup> fx_kq: array<f32, 256>;
var<workgroup> fx_red: array<f32, 256>;
// 256 threads: column j = t % 128, rows i of half t / 128 (the state is 128 x 128: dS = 128)
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let h = wg.x; let t = l.x; let dS = fx_P.dS;
  let j = t & 127u; let i0 = (t >> 7u) * 64u;
  let kh = h % fx_P.nKH;
  if (t < 128u) { fx_kq[t] = fx_co[fx_P.keyDim + kh * dS + t]; }   // k
  else { fx_kq[t] = fx_co[kh * dS + t - 128u]; }                  // q
  workgroupBarrier();
  let araw = fx_ba[fx_P.nVH + h] + fx_dta[h];
  var sp = araw;
  if (araw <= 20.0) { sp = log(1.0 + exp(araw)); }
  let decay = exp(sp * fx_dta[fx_P.nVH + h]);
  let b = sigm(fx_ba[h]);
  let v = fx_co[2u * fx_P.keyDim + h * dS + j];
  let base = h * dS * dS;
  var vh = 0.0;
  for (var i = i0; i < i0 + 64u; i++) {
    let s = fx_S[base + i * dS + j] * decay;
    fx_S[base + i * dS + j] = s;
    vh += s * fx_kq[i];
  }
  fx_red[t] = vh;
  workgroupBarrier();
  vh = (v - (fx_red[j] + fx_red[128u + j])) * b;
  workgroupBarrier();
  var acc = 0.0;
  for (var i = i0; i < i0 + 64u; i++) {
    let s = fx_S[base + i * dS + j] + fx_kq[i] * vh;
    fx_S[base + i * dS + j] = s;
    acc += s * fx_kq[128u + i];
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
    fx_o[oi] = ov;
    fx_y[oi] = (ov * inv) * fx_nw[j] * sigm(fx_z[oi]);
  }
}`;

// Per-token frame: the position and the RoPE cos/sin of each rotary pair at that position (computed on the CPU
// with the reference's f32 recurrence, theta_i = pos * base^(-2i/nRot)).
const FRAME = `struct Frame { pos: u32, seqLen: u32, p0: u32, p1: u32, cs: array<vec4<f32>, 64> };`;
export const FRAME_FLOATS = 4 + 64 * 4;   // pos, seqLen, pad, pad, then (cos, sin, 0, 0) per rotary pair (<= 64 pairs)

// The K / V cache formats (engine option kv): "f32" (4 B per value) or "f16" (2 B: packed pairs, low half = the
// even element). f16 values are stored with f16.js's round-to-nearest-even (pack2x16_rne: pack2x16float's rounding
// is the backend's, and D3D12's differs) and read back exactly (unpack2x16_exact: no subnormal flush), so every
// backend keeps the same cache bits. The attention arithmetic is the f32 kernels' in the same order, on the f16
// values: decode, prefill and verify passes still agree bit for bit with each other.
export const KV_FORMATS = ["f32", "f16"];
export const kvBytes = (kv) => (kv === "f16" ? 2 : 4);
export function kvCheck(kv) { if (!KV_FORMATS.includes(kv)) throw new Error(`qwen4exp: KV format ${kv} (one of ${KV_FORMATS.join(", ")})`); }
// WGSL pieces per format: the cache bindings' element type (K as vec4 units for the dot), a K vec4 read at a vec4
// index, a V value read at an element index
export const kvDecl = (kv, rw) => {
  const a = rw ? "read_write" : "read";
  return kv === "f16"
    ? { k: `array<${rw ? "u32" : "vec2<u32>"}>`, v: "array<u32>", a, f16: F16_WGSL }
    : { k: `array<${rw ? "f32" : "vec4<f32>"}>`, v: "array<f32>", a, f16: "" };
};
export const kvK4 = (kv, K, i) => (kv === "f16" ? `kv_k4(${K}[${i}])` : `${K}[${i}]`);
// (e: an identifier, it is used twice)
export const kvV1 = (kv, V, e) => (kv === "f16" ? `kv_h1((${V}[${e} >> 1u] >> ((${e} & 1u) << 4u)) & 0xffffu)` : `${V}[${e}]`);
// unpack2x16float is exact for normal f16 values on every backend (only subnormals may flush); a word with a zero
// exponent (zero or subnormal) takes f16.js's exact path, so the values are unpack2x16_exact's at the builtin's cost
export const KV_F16_READ = /* wgsl */ `
fn kv_h2(w: u32) -> vec2<f32> {
  if ((w & 0x7c00u) == 0u || (w & 0x7c000000u) == 0u) { return unpack2x16_exact(w); }
  return unpack2x16float(w);
}
fn kv_h1(h: u32) -> f32 { if ((h & 0x7c00u) == 0u) { return f16_f32(h); } return unpack2x16float(h).x; }
fn kv_k4(w: vec2<u32>) -> vec4<f32> { return vec4<f32>(kv_h2(w.x), kv_h2(w.y)); }
`;
// The cache write of one K / V head row (hd threads, thread t holds element t of the roped key r): f32 stores
// directly; f16 stages the key in workgroup memory (st) so thread t < hd / 2 packs elements 2t, 2t + 1. The barrier
// is at the top level (uniform control flow); o0: the row's first element, vAt(e): the V input element e of the row.
export const kvStoreRow = (kv, { isK, t, hd, o0, r, vAt, K, V, st }) => kv === "f16" ? `
  ${st}[${t}] = select(0.0, ${r}, ${isK} && ${t} < ${hd});
  workgroupBarrier();
  if (${isK} && ${t} < ${hd} / 2u) {
    let o = (${o0}) / 2u + ${t};
    ${K}[o] = pack2x16_rne(vec2<f32>(${st}[2u * ${t}], ${st}[2u * ${t} + 1u]));
    ${V}[o] = pack2x16_rne(vec2<f32>(${vAt(`2u * ${t}`)}, ${vAt(`2u * ${t} + 1u`)}));
  }` : `
  if (${isK} && ${t} < ${hd}) {
    let o = ${o0} + ${t};
    ${K}[o] = ${r};
    ${V}[o] = ${vAt(t)};
  }`;

// Gated attention, step 1 (one workgroup per Q head, then one per KV head; hd threads): per-head RMS norm,
// partial NeoX RoPE (pairs i, i + nRot/2 for i < nRot/2), then the K/V cache write at frame.pos. q and the gate are
// interleaved per head in qf: [q_h (hd) | gate_h (hd)].
export function attnPrepWGSL(kv = "f32") {
  kvCheck(kv);
  const D = kvDecl(kv, true);
  return `${D.f16}
${FRAME}
struct AP { nH: u32, nKV: u32, hd: u32, nRot: u32, eps: f32, p0: u32, p1: u32, p2: u32 };
@group(0) @binding(0) var<storage, read> fx_qf: array<f32>;
@group(0) @binding(1) var<storage, read> fx_kin: array<f32>;
@group(0) @binding(2) var<storage, read> fx_vin: array<f32>;
@group(0) @binding(3) var<storage, read_write> fx_qn: array<f32>;
@group(0) @binding(4) var<storage, read_write> fx_kc: ${D.k};
@group(0) @binding(5) var<storage, read_write> fx_vc: ${D.v};
@group(0) @binding(6) var<storage, read> fx_qw: array<f32>;
@group(0) @binding(7) var<storage, read> fx_kw: array<f32>;
@group(0) @binding(8) var<uniform> fx_P: AP;
@group(0) @binding(9) var<uniform> fx_F: Frame;
var<workgroup> fx_red: array<f32, 256>;
var<workgroup> fx_hv: array<f32, 256>;${kv === "f16" ? "\nvar<workgroup> fx_kr: array<f32, 256>;" : ""}
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let g = wg.x; let hd = fx_P.hd;
  let isQ = g < fx_P.nH;
  var x = 0.0;
  if (t < hd) {
    if (isQ) { x = fx_qf[g * 2u * hd + t]; } else { x = fx_kin[(g - fx_P.nH) * hd + t]; }
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
  if (t < hr) { let c = fx_F.cs[t]; r = nv * c.x - fx_hv[t + hr] * c.y; }
  else if (t < fx_P.nRot) { let c = fx_F.cs[t - hr]; r = fx_hv[t - hr] * c.y + nv * c.x; }
  if (isQ && t < hd) { fx_qn[g * hd + t] = r; }
  let h = select(g - fx_P.nH, 0u, isQ);
  let vb = h * hd;${kvStoreRow(kv, { isK: "!isQ", t: "t", hd: "hd", o0: "fx_F.pos * fx_P.nKV * hd + h * hd", r: "r", vAt: (e) => `fx_vin[vb + ${e}]`, K: "fx_kc", V: "fx_vc", st: "fx_kr" })}
}`;
}
export const ATTN_PREP_WGSL = attnPrepWGSL("f32");

// Step 2: dense causal attention for one decode token over cache cells 0..frame.pos, one workgroup per Q head
// (hd = 256 threads), then out = attn * sigmoid(gate). Scores live in workgroup memory: seqLen <= 2052 (dense runs up
// to 2051 cells; past that QSA's gathered kernel, engine/wgsl/qwen4exp_batch.js attnDecMcWGSL, reads the selected ones).
export function attnDecWGSL(kv = "f32") {
  kvCheck(kv);
  const D = kvDecl(kv, false);
  return DQ + D.f16 + (kv === "f16" ? KV_F16_READ : "") + `
${FRAME}
struct AP { nH: u32, nKV: u32, hd: u32, nRot: u32, eps: f32, p0: u32, p1: u32, p2: u32 };
@group(0) @binding(0) var<storage, read> fx_qn: array<vec4<f32>>;
@group(0) @binding(1) var<storage, read> fx_kc: ${D.k};
@group(0) @binding(2) var<storage, read> fx_vc: ${D.v};
@group(0) @binding(3) var<storage, read> fx_qf: array<f32>;
@group(0) @binding(4) var<storage, read_write> fx_out: array<f32>;
@group(0) @binding(5) var<uniform> fx_P: AP;
@group(0) @binding(6) var<uniform> fx_F: Frame;
var<workgroup> fx_sc: array<f32, 2052>;
var<workgroup> fx_red: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let h = wg.x; let hd = fx_P.hd; let hd4 = hd / 4u;
  let kvh = h / (fx_P.nH / fx_P.nKV);
  let n = fx_F.pos + 1u;
  let scale = 1.0 / sqrt(f32(hd));
  var m = -3.0e38;
  for (var p = t; p < n; p += 256u) {
    let ko = (p * fx_P.nKV + kvh) * hd4;
    var a = 0.0;
    for (var i = 0u; i < hd4; i++) { a += dot(fx_qn[h * hd4 + i], ${kvK4(kv, "fx_kc", "ko + i")}); }
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
    for (var p = 0u; p < n; p++) { let ve = (p * fx_P.nKV + kvh) * hd + t; acc += fx_sc[p] * ${kvV1(kv, "fx_vc", "ve")}; }
    fx_out[h * hd + t] = (acc / tot) * sigm(fx_qf[h * 2u * hd + hd + t]);
  }
}`;
}
export const ATTN_DEC_WGSL = attnDecWGSL("f32");

// The n-gram (PLE) injection at its layer (roadmap 33 "N-gram embedding (PLE)"), one workgroup per stream c:
//   key[c] = rmsnorm(pk[c]) * nk[c]; query[c] = rmsnorm(R[c]) * nq[c]; s = key[c] . query[c] / sqrt(d)
//   gate[c] = sigmoid(sign(s) * sqrt(max(|s|, 1e-6))); g[c] = pv * gate[c]; n[c] = rmsnorm(g[c]) * nc[c]
//   conv[c] = silu(sum over k of cw[ch][k] * n at t - (K-1-k) * dil)  (depthwise, causal, dilated; a ring of
//   (K-1)*dil + 1 positions, positions before the sequence start read as 0)
//   R[c] += g[c] + conv[c]
// dbg keeps g, conv and gate for the per-layer checks.
export const PLE_WGSL = DQ + `
${FRAME}
struct PP { d: u32, K: u32, dil: u32, slots: u32, eps: f32, hc: u32, p1: u32, p2: u32 };
@group(0) @binding(0) var<storage, read_write> fx_R: array<f32>;
@group(0) @binding(1) var<storage, read> fx_pk: array<f32>;
@group(0) @binding(2) var<storage, read> fx_pv: array<f32>;
@group(0) @binding(3) var<storage, read> fx_pw: array<f32>;            // [nk | nq | nc] (hc*d each) then cw [hc*d][K]
@group(0) @binding(4) var<storage, read_write> fx_ring: array<f32>;
@group(0) @binding(5) var<storage, read_write> fx_dbg: array<f32>;     // [g (hc*d) | conv (hc*d) | gate (hc)]
@group(0) @binding(6) var<uniform> fx_P: PP;
@group(0) @binding(7) var<uniform> fx_F: Frame;
var<workgroup> fx_r1: array<f32, 256>;
var<workgroup> fx_r2: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let c = wg.x; let d = fx_P.d; let o = c * d;
  let HD = d * fx_P.hc;   // ring row: every stream's channels
  let NQ = HD; let NC = 2u * HD; let CW = 3u * HD;
  var sk = 0.0; var sq = 0.0;
  for (var i = t; i < d; i += 256u) { let a = fx_pk[o + i]; let b = fx_R[o + i]; sk += a * a; sq += b * b; }
  fx_r1[t] = sk; fx_r2[t] = sq;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_r1[t] += fx_r1[t + s]; fx_r2[t] += fx_r2[t + s]; } workgroupBarrier(); }
  let ik = 1.0 / sqrt(fx_r1[0] / f32(d) + fx_P.eps);
  let iq = 1.0 / sqrt(fx_r2[0] / f32(d) + fx_P.eps);
  workgroupBarrier();
  var dt = 0.0;
  for (var i = t; i < d; i += 256u) { dt += ((fx_pk[o + i] * ik) * fx_pw[o + i]) * ((fx_R[o + i] * iq) * fx_pw[NQ + o + i]); }
  fx_r1[t] = dt;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_r1[t] += fx_r1[t + s]; } workgroupBarrier(); }
  let sv = fx_r1[0] / sqrt(f32(d));
  let gate = sigm(sign(sv) * sqrt(max(abs(sv), 1e-6)));
  workgroupBarrier();
  var sg = 0.0;
  for (var i = t; i < d; i += 256u) { let g = fx_pv[i] * gate; sg += g * g; }
  fx_r1[t] = sg;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { fx_r1[t] += fx_r1[t + s]; } workgroupBarrier(); }
  let ig = 1.0 / sqrt(fx_r1[0] / f32(d) + fx_P.eps);
  if (t == 0u) { fx_dbg[2u * HD + c] = gate; }
  let slot = fx_F.pos % fx_P.slots;
  for (var i = t; i < d; i += 256u) {
    let ch = o + i;
    let g = fx_pv[i] * gate;
    let nv = (g * ig) * fx_pw[NC + ch];
    fx_ring[slot * HD + ch] = nv;
    var acc = fx_pw[CW + ch * fx_P.K + fx_P.K - 1u] * nv;
    for (var k = 0u; k + 1u < fx_P.K; k++) {
      let back = (fx_P.K - 1u - k) * fx_P.dil;
      if (fx_F.pos >= back) { acc += fx_pw[CW + ch * fx_P.K + k] * fx_ring[((fx_F.pos - back) % fx_P.slots) * HD + ch]; }
    }
    let cv = silu(acc);
    fx_dbg[ch] = g; fx_dbg[HD + ch] = cv;
    fx_R[ch] = fx_R[ch] + (g + cv);
  }
}`;

// Greedy head, stage 1: per workgroup (256 threads over a strided slice), the largest logit, lowest index on ties.
// Partials are (index, bits of the value) as u32, as qwen35.js's argmax: an index bitcast to f32 is a denormal that
// some backends (D3D/DXC, fast-math Metal) flush to 0.
export const ARGMAX1_WGSL = `
struct AM { n: u32, per: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_lg: array<f32>;
@group(0) @binding(1) var<storage, read_write> fx_part: array<vec2<u32>>;
@group(0) @binding(2) var<uniform> fx_P: AM;
var<workgroup> fx_rv: array<f32, 256>;
var<workgroup> fx_ri: array<u32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let lo = wg.x * fx_P.per; let hi = min(lo + fx_P.per, fx_P.n);
  var bv = -3.0e38; var bi = 0xffffffffu;
  for (var i = lo + t; i < hi; i += 256u) { let v = fx_lg[i]; if (v > bv) { bv = v; bi = i; } }
  fx_rv[t] = bv; fx_ri[t] = bi;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) {
    if (t < s) { let ov = fx_rv[t + s]; let oi = fx_ri[t + s]; if (ov > fx_rv[t] || (ov == fx_rv[t] && oi < fx_ri[t])) { fx_rv[t] = ov; fx_ri[t] = oi; } }
    workgroupBarrier();
  }
  if (t == 0u) { fx_part[wg.x] = vec2<u32>(fx_ri[0], bitcast<u32>(fx_rv[0])); }
}`;
// Stage 2: one workgroup over the stage-1 partials -> out[0] = index, out[1] = bits of the value.
export const ARGMAX2_WGSL = `
struct AM { n: u32, per: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> fx_part: array<vec2<u32>>;
@group(0) @binding(1) var<storage, read_write> fx_outv: array<u32>;
@group(0) @binding(2) var<uniform> fx_P: AM;
var<workgroup> fx_rv: array<f32, 256>;
var<workgroup> fx_ri: array<u32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x;
  var bv = -3.0e38; var bi = 0xffffffffu;
  for (var i = t; i < fx_P.n; i += 256u) { let p = fx_part[i]; let pv = bitcast<f32>(p.y); if (pv > bv || (pv == bv && p.x < bi)) { bv = pv; bi = p.x; } }
  fx_rv[t] = bv; fx_ri[t] = bi;
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) {
    if (t < s) { let ov = fx_rv[t + s]; let oi = fx_ri[t + s]; if (ov > fx_rv[t] || (ov == fx_rv[t] && oi < fx_ri[t])) { fx_rv[t] = ov; fx_ri[t] = oi; } }
    workgroupBarrier();
  }
  if (t == 0u) { fx_outv[0] = fx_ri[0]; fx_outv[1] = bitcast<u32>(fx_rv[0]); }
}`;
