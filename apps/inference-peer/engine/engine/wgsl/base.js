// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/base.js (MIT; see LICENSE-ENGINE).
// Base WGSL kernels shared by both engines: legacy matvecs, norms, rope, attention, activations.

export const WGSL = /* wgsl */ `
struct Config {
  dim: u32, kvDim: u32, nH: u32, nKV: u32,
  headDim: u32, inter: u32, vocab: u32, maxSeq: u32,
  eps: f32, theta: f32, qDim: u32,
};
struct Frame { pos: u32, seqLen: u32, nCols: u32, snap: u32 };
struct Shape { dOut: u32, dIn: u32 };

@group(0) @binding(0) var<uniform> cfg: Config;
@group(0) @binding(1) var<uniform> frame: Frame;

// --- matvec: y = W x, W is [dOut, dIn] row-major ---
@group(1) @binding(0) var<storage, read> mv_w: array<f32>;
@group(1) @binding(1) var<storage, read> mv_x: array<f32>;
@group(1) @binding(2) var<storage, read_write> mv_y: array<f32>;
@group(1) @binding(3) var<uniform> mv_shape: Shape;
@compute @workgroup_size(64)
fn matvec(@builtin(global_invocation_id) gid: vec3<u32>) {
  let r = gid.x;
  if (r >= mv_shape.dOut) { return; }
  var acc: f32 = 0.0;
  let off = r * mv_shape.dIn;
  for (var c: u32 = 0u; c < mv_shape.dIn; c++) {
    acc += mv_w[off + c] * mv_x[c];
  }
  mv_y[r] = acc;
}

// --- matvec_q8: y = W x with W in Q8_0 (int8 + per-32-block f16 scale) ---
@group(1) @binding(0) var<storage, read> q8_qs: array<u32>;   // int8s packed 4/word
@group(1) @binding(1) var<storage, read> q8_sc: array<u32>;   // f16 scales, 2 per word
fn q8s(i: u32) -> f32 { return unpack2x16float(q8_sc[i >> 1u])[i & 1u]; }
@group(1) @binding(2) var<storage, read> q8_x: array<f32>;
@group(1) @binding(3) var<storage, read_write> q8_y: array<f32>;
@group(1) @binding(4) var<uniform> q8_shape: Shape;
@compute @workgroup_size(64)
fn matvec_q8(@builtin(global_invocation_id) gid: vec3<u32>) {
  let r = gid.x;
  if (r >= q8_shape.dOut) { return; }
  let dIn = q8_shape.dIn;
  let nb = dIn / 32u;
  var acc: f32 = 0.0;
  let rowWords = r * dIn / 4u;
  let rowSc = r * nb;
  for (var b: u32 = 0u; b < nb; b++) {
    var sum: f32 = 0.0;
    let wBase = rowWords + b * 8u;
    let xBase = b * 32u;
    for (var w: u32 = 0u; w < 8u; w++) {
      let word = bitcast<i32>(q8_qs[wBase + w]);
      let x0 = xBase + w * 4u;
      sum += f32(extractBits(word, 0u, 8u)) * q8_x[x0]
           + f32(extractBits(word, 8u, 8u)) * q8_x[x0 + 1u]
           + f32(extractBits(word, 16u, 8u)) * q8_x[x0 + 2u]
           + f32(extractBits(word, 24u, 8u)) * q8_x[x0 + 3u];
    }
    acc += q8s(rowSc + b) * sum;
  }
  q8_y[r] = acc;
}

// --- per-head rmsnorm (qwen3 QK-norm): each head's headDim slice normalized,
// scaled by a shared [headDim] weight ---
@group(1) @binding(0) var<storage, read_write> hn_v: array<f32>;
@group(1) @binding(1) var<storage, read> hn_w: array<f32>;
@group(1) @binding(2) var<uniform> hn_nheads: u32;
@compute @workgroup_size(32)
fn head_norm(@builtin(global_invocation_id) gid: vec3<u32>) {
  let h = gid.x;
  if (h >= hn_nheads) { return; }
  let off = h * cfg.headDim;
  var ss: f32 = 0.0;
  for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = hn_v[off + i]; ss += v * v; }
  let inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
  for (var i: u32 = 0u; i < cfg.headDim; i++) { hn_v[off + i] *= inv * hn_w[i]; }
}

// --- matvec_q4: y = W x with W in Q4_0 (packed nibbles + per-32-block f16 scale) ---
// nibble layout per block: byte j holds elem j (low nibble) and elem j+16 (high)
@group(1) @binding(0) var<storage, read> q4_qs: array<u32>;
@group(1) @binding(1) var<storage, read> q4_sc: array<u32>;   // f16 scales, 2 per word
fn q4s(i: u32) -> f32 { return unpack2x16float(q4_sc[i >> 1u])[i & 1u]; }
@group(1) @binding(2) var<storage, read> q4_x: array<f32>;
@group(1) @binding(3) var<storage, read_write> q4_y: array<f32>;
@group(1) @binding(4) var<uniform> q4_shape: Shape;
@compute @workgroup_size(64)
fn matvec_q4(@builtin(global_invocation_id) gid: vec3<u32>) {
  let r = gid.x;
  if (r >= q4_shape.dOut) { return; }
  let dIn = q4_shape.dIn;
  let nb = dIn / 32u;
  var acc: f32 = 0.0;
  let rowWords = r * dIn / 8u;   // 4 bits/weight -> dIn/8 u32 words per row
  let rowSc = r * nb;
  for (var b: u32 = 0u; b < nb; b++) {
    var sum: f32 = 0.0;
    let wBase = rowWords + b * 4u;
    let xBase = b * 32u;
    for (var w: u32 = 0u; w < 4u; w++) {
      let word = q4_qs[wBase + w];
      let j = xBase + w * 4u;
      sum += (f32(extractBits(word, 0u, 4u)) - 8.0) * q4_x[j]
           + (f32(extractBits(word, 4u, 4u)) - 8.0) * q4_x[j + 16u]
           + (f32(extractBits(word, 8u, 4u)) - 8.0) * q4_x[j + 1u]
           + (f32(extractBits(word, 12u, 4u)) - 8.0) * q4_x[j + 17u]
           + (f32(extractBits(word, 16u, 4u)) - 8.0) * q4_x[j + 2u]
           + (f32(extractBits(word, 20u, 4u)) - 8.0) * q4_x[j + 18u]
           + (f32(extractBits(word, 24u, 4u)) - 8.0) * q4_x[j + 3u]
           + (f32(extractBits(word, 28u, 4u)) - 8.0) * q4_x[j + 19u];
    }
    acc += q4s(rowSc + b) * sum;
  }
  q4_y[r] = acc;
}

// --- rmsnorm: y = x * invRms(x) * w ---
@group(1) @binding(0) var<storage, read> rn_x: array<f32>;
@group(1) @binding(1) var<storage, read> rn_w: array<f32>;
@group(1) @binding(2) var<storage, read_write> rn_y: array<f32>;
@group(1) @binding(3) var<uniform> rn_n: u32;
var<workgroup> rn_partial: array<f32, 256>;
@compute @workgroup_size(256)
fn rmsnorm(@builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  var ss: f32 = 0.0;
  // four loads in flight, then the same in-order sum (bit-identical to the one-at-a-time loop)
  var i: u32 = t;
  for (; i + 768u < rn_n; i += 1024u) {
    let v0 = rn_x[i]; let v1 = rn_x[i + 256u]; let v2 = rn_x[i + 512u]; let v3 = rn_x[i + 768u];
    ss += v0 * v0; ss += v1 * v1; ss += v2 * v2; ss += v3 * v3;
  }
  for (; i < rn_n; i += 256u) { let v = rn_x[i]; ss += v * v; }
  rn_partial[t] = ss;
  workgroupBarrier();
  var stride: u32 = 128u;
  while (stride > 0u) {
    if (t < stride) { rn_partial[t] += rn_partial[t + stride]; }
    workgroupBarrier();
    stride = stride / 2u;
  }
  let inv = inverseSqrt(rn_partial[0] / f32(rn_n) + cfg.eps);
  var j: u32 = t;
  for (; j + 768u < rn_n; j += 1024u) {
    let x0 = rn_x[j]; let x1 = rn_x[j + 256u]; let x2 = rn_x[j + 512u]; let x3 = rn_x[j + 768u];
    let w0 = rn_w[j]; let w1 = rn_w[j + 256u]; let w2 = rn_w[j + 512u]; let w3 = rn_w[j + 768u];
    rn_y[j] = x0 * inv * w0; rn_y[j + 256u] = x1 * inv * w1; rn_y[j + 512u] = x2 * inv * w2; rn_y[j + 768u] = x3 * inv * w3;
  }
  for (; j < rn_n; j += 256u) { rn_y[j] = rn_x[j] * inv * rn_w[j]; }
}

// --- rope: rotate pairs (i, i+half) in each head, at frame.pos ---
@group(1) @binding(0) var<storage, read_write> rp_v: array<f32>;
@group(1) @binding(1) var<uniform> rp_nheads: u32;
@compute @workgroup_size(64)
fn rope(@builtin(global_invocation_id) gid: vec3<u32>) {
  let half = cfg.headDim / 2u;
  let total = rp_nheads * half;
  let idx = gid.x;
  if (idx >= total) { return; }
  let h = idx / half;
  let i = idx % half;
  let off = h * cfg.headDim;
  let freq = pow(cfg.theta, -f32(2u * i) / f32(cfg.headDim));
  let ang = f32(frame.pos) * freq;
  let c = cos(ang); let s = sin(ang);
  let a = rp_v[off + i]; let b = rp_v[off + i + half];
  rp_v[off + i] = a * c - b * s;
  rp_v[off + i + half] = b * c + a * s;
}

// --- attention scores: scores[h*maxSeq+t] = dot(q_h, kCache[t]_kvH) / sqrt(hd) ---
@group(1) @binding(0) var<storage, read> at_q: array<f32>;
@group(1) @binding(1) var<storage, read> at_kc: array<f32>;
@group(1) @binding(2) var<storage, read_write> at_scores: array<f32>;
@compute @workgroup_size(64)
fn attn_scores(@builtin(global_invocation_id) gid: vec3<u32>) {
  let total = cfg.nH * frame.seqLen;
  let idx = gid.x;
  if (idx >= total) { return; }
  let h = idx / frame.seqLen;
  let t = idx % frame.seqLen;
  let kvH = h / (cfg.nH / cfg.nKV);
  let qOff = h * cfg.headDim;
  let kOff = t * cfg.kvDim + kvH * cfg.headDim;
  var acc: f32 = 0.0;
  for (var i: u32 = 0u; i < cfg.headDim; i++) {
    acc += at_q[qOff + i] * at_kc[kOff + i];
  }
  at_scores[h * cfg.maxSeq + t] = acc / sqrt(f32(cfg.headDim));
}

// --- attention softmax: per head, serial (seqLen is small) ---
@group(1) @binding(0) var<storage, read_write> sm_scores: array<f32>;
@compute @workgroup_size(1)
fn attn_softmax(@builtin(global_invocation_id) gid: vec3<u32>) {
  let h = gid.x;
  if (h >= cfg.nH) { return; }
  let off = h * cfg.maxSeq;
  var mx: f32 = -3.0e38;
  for (var t: u32 = 0u; t < frame.seqLen; t++) { mx = max(mx, sm_scores[off + t]); }
  var sum: f32 = 0.0;
  for (var t: u32 = 0u; t < frame.seqLen; t++) {
    let e = exp(sm_scores[off + t] - mx);
    sm_scores[off + t] = e;
    sum += e;
  }
  for (var t: u32 = 0u; t < frame.seqLen; t++) { sm_scores[off + t] /= sum; }
}

// --- attention softmax, one 256-thread workgroup per head, bit-identical to attn_softmax ---
// The max (order-free) and the exponentials run in parallel; the sum is still one thread adding
// the same values in the same order, but from workgroup memory instead of three serial passes
// over global memory, which is what made the old kernel grow with the context. Needs seqLen <=
// 2048 (the staging array); the engine falls back to attn_softmax above that.
var<workgroup> smw_e: array<f32, 2048>;
var<workgroup> smw_r: array<f32, 256>;
@compute @workgroup_size(256)
fn attn_softmax_wg(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let t0 = lid.x; let n = frame.seqLen;
  if (h >= cfg.nH) { return; }
  let off = h * cfg.maxSeq;
  var m: f32 = -3.0e38;
  for (var t: u32 = t0; t < n; t += 256u) { m = max(m, sm_scores[off + t]); }
  smw_r[t0] = m;
  workgroupBarrier();
  for (var s: u32 = 128u; s > 0u; s >>= 1u) {
    if (t0 < s) { smw_r[t0] = max(smw_r[t0], smw_r[t0 + s]); }
    workgroupBarrier();
  }
  let mx = smw_r[0];
  for (var t: u32 = t0; t < n; t += 256u) { smw_e[t] = exp(sm_scores[off + t] - mx); }
  workgroupBarrier();
  if (t0 == 0u) {
    var sum: f32 = 0.0;
    for (var t: u32 = 0u; t < n; t++) { sum += smw_e[t]; }
    smw_r[0] = sum;
  }
  workgroupBarrier();
  let sum = smw_r[0];
  for (var t: u32 = t0; t < n; t += 256u) { sm_scores[off + t] = smw_e[t] / sum; }
}

// --- attention out: out[h*hd+i] = sum_t scores[h,t] * vCache[t, kvH*hd+i] ---
@group(1) @binding(0) var<storage, read> ao_scores: array<f32>;
@group(1) @binding(1) var<storage, read> ao_vc: array<f32>;
@group(1) @binding(2) var<storage, read_write> ao_out: array<f32>;
@compute @workgroup_size(64)
fn attn_out(@builtin(global_invocation_id) gid: vec3<u32>) {
  let idx = gid.x;
  if (idx >= cfg.qDim) { return; }
  let h = idx / cfg.headDim;
  let i = idx % cfg.headDim;
  let kvH = h / (cfg.nH / cfg.nKV);
  var acc: f32 = 0.0;
  for (var t: u32 = 0u; t < frame.seqLen; t++) {
    acc += ao_scores[h * cfg.maxSeq + t] * ao_vc[t * cfg.kvDim + kvH * cfg.headDim + i];
  }
  ao_out[idx] = acc;
}

// --- batched attention: every column of a verify / prefill pass in one dispatch per stage ---
// Column c (gid.y / wg.y) attends to positions [0, frame.seqLen + c) (frame is column 0's, the
// K/V of all columns are in the cache already) and has its own score rows at c * nH * maxSeq.
// Per (column, head, position) the arithmetic is attn_scores / attn_softmax_wg / attn_out's, so
// the result is bit-identical to running the columns one after another.
struct AMC { n: u32, s0: u32, s1: u32, s2: u32 };
@group(1) @binding(0) var<storage, read> asm_q: array<f32>;
@group(1) @binding(1) var<storage, read> asm_kc: array<f32>;
@group(1) @binding(2) var<storage, read_write> asm_scores: array<f32>;
@group(1) @binding(3) var<uniform> asm_mc: AMC;          // s0 = q column stride
@compute @workgroup_size(64)
fn attn_scores_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let col = gid.y; let h = gid.z; let t = gid.x;   // x: position, y: column, z: head
  let seqLen = frame.seqLen + col;
  if (t >= seqLen || h >= cfg.nH) { return; }
  let kvH = h / (cfg.nH / cfg.nKV);
  let qOff = col * asm_mc.s0 + h * cfg.headDim;
  let kOff = t * cfg.kvDim + kvH * cfg.headDim;
  var acc: f32 = 0.0;
  for (var i: u32 = 0u; i < cfg.headDim; i++) {
    acc += asm_q[qOff + i] * asm_kc[kOff + i];
  }
  asm_scores[(col * cfg.nH + h) * cfg.maxSeq + t] = acc / sqrt(f32(cfg.headDim));
}

@group(1) @binding(0) var<storage, read_write> smm2_scores: array<f32>;
@compute @workgroup_size(256)
fn attn_softmax_wg_mc(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let t0 = lid.x; let n = frame.seqLen + wg.y;
  if (h >= cfg.nH) { return; }
  let off = (wg.y * cfg.nH + h) * cfg.maxSeq;
  var m: f32 = -3.0e38;
  for (var t: u32 = t0; t < n; t += 256u) { m = max(m, smm2_scores[off + t]); }
  smw_r[t0] = m;
  workgroupBarrier();
  for (var s: u32 = 128u; s > 0u; s >>= 1u) {
    if (t0 < s) { smw_r[t0] = max(smw_r[t0], smw_r[t0 + s]); }
    workgroupBarrier();
  }
  let mx = smw_r[0];
  for (var t: u32 = t0; t < n; t += 256u) { smw_e[t] = exp(smm2_scores[off + t] - mx); }
  workgroupBarrier();
  if (t0 == 0u) {
    var sum: f32 = 0.0;
    for (var t: u32 = 0u; t < n; t++) { sum += smw_e[t]; }
    smw_r[0] = sum;
  }
  workgroupBarrier();
  let sum = smw_r[0];
  for (var t: u32 = t0; t < n; t += 256u) { smm2_scores[off + t] = smw_e[t] / sum; }
}

@group(1) @binding(0) var<storage, read> aom_scores: array<f32>;
@group(1) @binding(1) var<storage, read> aom_vc: array<f32>;
@group(1) @binding(2) var<storage, read_write> aom_out: array<f32>;
@group(1) @binding(3) var<uniform> aom_mc: AMC;          // s0 = out column stride
@compute @workgroup_size(64)
fn attn_out_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let idx = gid.x;
  if (idx >= cfg.qDim) { return; }
  let col = gid.y;
  let seqLen = frame.seqLen + col;
  let h = idx / cfg.headDim;
  let i = idx % cfg.headDim;
  let kvH = h / (cfg.nH / cfg.nKV);
  let so = (col * cfg.nH + h) * cfg.maxSeq;
  var acc: f32 = 0.0;
  for (var t: u32 = 0u; t < seqLen; t++) {
    acc += aom_scores[so + t] * aom_vc[t * cfg.kvDim + kvH * cfg.headDim + i];
  }
  aom_out[col * aom_mc.s0 + idx] = acc;
}

// --- silu-gate: g = silu(g) * u ---
@group(1) @binding(0) var<storage, read_write> sg_g: array<f32>;
@group(1) @binding(1) var<storage, read> sg_u: array<f32>;
@compute @workgroup_size(64)
fn silu_mul(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x;
  if (i >= cfg.inter) { return; }
  let g = sg_g[i];
  sg_g[i] = (g / (1.0 + exp(-g))) * sg_u[i];
}

// --- residual add: x += y (n = dim) ---
@group(1) @binding(0) var<storage, read_write> ad_x: array<f32>;
@group(1) @binding(1) var<storage, read> ad_y: array<f32>;
@compute @workgroup_size(64)
fn add_res(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x;
  if (i >= cfg.dim) { return; }
  ad_x[i] += ad_y[i];
}

`;
