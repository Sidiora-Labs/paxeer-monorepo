// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/dense.js (MIT; see LICENSE-ENGINE).
// Dense-engine attention kernels (f32 KV cache) that keep the exact arithmetic of base.js's
// attn_scores / attn_softmax / attn_out, bit for bit, but are not latency-bound at long context.
//
// What makes them exact: every value is produced by the same f32 operations on the same operands in
// the same order as the reference kernels.
//   - score(h, t) = (sum over i = 0..hd-1, in order, of q[h][i] * k[t][i]) / sqrt(f32(headDim))
//     (one thread per position; the G query heads of a KV head share each K row load)
//   - softmax: max (order-free), e_t = exp(s_t - max) in parallel, the sum of e_t added by ONE thread
//     in position order (loads issued 8 ahead so the chain is add-bound, not load-latency-bound),
//     then s_t = e_t / sum in parallel
//   - out(h, i) = sum over t = 0..n-1, in order, of p[h][t] * v[t][i]: one chain per (h, i) as before,
//     but the V and p tiles are loaded cooperatively by the whole workgroup into shared memory
//     (coalesced, with the next tile prefetched into registers while the chains run), so each chain
//     only reads shared memory
// Multi-column form: grid z / y is the batch column; column c attends to [0, frame.seqLen + c) (the
// frame is column 0's), exactly like base.js's _mc kernels, so a batched pass computes each column
// as the one-column kernels would.
//
// G (query heads per KV head) is a literal so the per-head accumulators are named scalars (a local
// array indexed by a loop variable spills to scratch). headDim stays a uniform read: sqrt of a
// literal would be constant-folded and could round differently from the runtime sqrt.
export function denseAttnWGSL({ G, hd }) {
  const SL = 32, T = 64;                         // attn_out: output slice width, positions per tile
  if (hd % SL || hd % 4 || G < 1 || G * T > 256 || G * hd > 1024) throw new Error(`denseAttnWGSL: unsupported G=${G} hd=${hd}`);
  const r = (n) => Array.from({ length: n }, (_, i) => i);
  const VL = (T * SL) / 256;                     // V elements each of the 256 threads loads per tile
  return /* wgsl */ `
struct DMC { s0: u32, s1: u32, s2: u32, s3: u32 };   // s0: q / out column stride (f32s)

// ---- scores: grid (ceil(maxLen / 64), nKV, nCols), one thread per position, G heads per thread ----
@group(1) @binding(0) var<storage, read> dsc_q: array<f32>;
@group(1) @binding(1) var<storage, read> dsc_k: array<vec4<f32>>;
@group(1) @binding(2) var<storage, read_write> dsc_s: array<f32>;
@group(1) @binding(3) var<uniform> dsc_mc: DMC;
var<workgroup> dsc_qs: array<f32, ${G * hd}>;
@compute @workgroup_size(64)
fn attn_scores_d(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let col = wg.z; let g = wg.y; let l = lid.x;
  let t = wg.x * 64u + l;
  let seqLen = frame.seqLen + col;
  let hd = cfg.headDim;
  for (var w: u32 = l; w < ${G}u * hd; w += 64u) { dsc_qs[w] = dsc_q[col * dsc_mc.s0 + g * ${G}u * hd + w]; }
  workgroupBarrier();
  if (t >= seqLen) { return; }
  let kb = (t * cfg.kvDim + g * hd) / 4u;
  ${r(G).map((h) => `var a${h}: f32 = 0.0;`).join(" ")}
  for (var i4: u32 = 0u; i4 < hd / 4u; i4++) {
    let k = dsc_k[kb + i4];
    let i = i4 * 4u;
    ${r(G).map((h) => `a${h} += dsc_qs[${h}u * hd + i] * k.x; a${h} += dsc_qs[${h}u * hd + i + 1u] * k.y; a${h} += dsc_qs[${h}u * hd + i + 2u] * k.z; a${h} += dsc_qs[${h}u * hd + i + 3u] * k.w;`).join("\n    ")}
  }
  let rs = sqrt(f32(cfg.headDim));
  ${r(G).map((h) => `dsc_s[(col * cfg.nH + g * ${G}u + ${h}u) * cfg.maxSeq + t] = a${h} / rs;`).join("\n  ")}
}

// ---- softmax: grid (nH, nCols), 256 threads; the sum is one thread's in-order chain ----
@group(1) @binding(0) var<storage, read_write> dsm_s: array<f32>;
var<workgroup> dsm_r: array<f32, 256>;
@compute @workgroup_size(256)
fn attn_softmax_d(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let l = lid.x; let n = frame.seqLen + wg.y;
  let off = (wg.y * cfg.nH + h) * cfg.maxSeq;
  var m: f32 = -3.0e38;
  for (var t: u32 = l; t < n; t += 256u) { m = max(m, dsm_s[off + t]); }
  dsm_r[l] = m;
  workgroupBarrier();
  for (var s: u32 = 128u; s > 0u; s >>= 1u) {
    if (l < s) { dsm_r[l] = max(dsm_r[l], dsm_r[l + s]); }
    workgroupBarrier();
  }
  let mx = dsm_r[0];
  for (var t: u32 = l; t < n; t += 256u) { dsm_s[off + t] = exp(dsm_s[off + t] - mx); }
  storageBarrier();
  workgroupBarrier();
  if (l == 0u) {
    var sum: f32 = 0.0;
    var t: u32 = 0u;
    for (; t + 8u <= n; t += 8u) {
      ${r(8).map((j) => `let e${j} = dsm_s[off + t + ${j}u];`).join(" ")}
      ${r(8).map((j) => `sum += e${j};`).join(" ")}
    }
    for (; t < n; t++) { sum += dsm_s[off + t]; }
    dsm_r[0] = sum;
  }
  workgroupBarrier();
  let sum = dsm_r[0];
  for (var t: u32 = l; t < n; t += 256u) { dsm_s[off + t] = dsm_s[off + t] / sum; }
}

// ---- out: grid (nKV * hd / ${SL}, nCols), 256 threads; ${G * SL} of them run the (h, i) chains ----
@group(1) @binding(0) var<storage, read> dao_p: array<f32>;
@group(1) @binding(1) var<storage, read> dao_v: array<f32>;
@group(1) @binding(2) var<storage, read_write> dao_o: array<f32>;
@group(1) @binding(3) var<uniform> dao_mc: DMC;
var<workgroup> dao_vt: array<f32, ${T * SL}>;    // [${T} positions][${SL} dims]
var<workgroup> dao_pt: array<f32, ${G * T}>;     // [${G} heads][${T} positions]
@compute @workgroup_size(256)
fn attn_out_d(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let col = wg.y; let l = lid.x;
  let hd = cfg.headDim;
  let ns = hd / ${SL}u;
  let g = wg.x / ns; let i0 = (wg.x % ns) * ${SL}u;
  let n = frame.seqLen + col;
  let vb = g * hd + i0;
  let pb = (col * cfg.nH + g * ${G}u) * cfg.maxSeq;
  // chain of this thread (if l < ${G * SL}): head g*G + ch, dim i0 + ci
  let ch = l / ${SL}u; let ci = l % ${SL}u;
  // loader roles: V element e = l + 256 k -> (row e / ${SL}, dim e % ${SL}); p element l -> (head l / ${T}, pos l % ${T})
  var acc: f32 = 0.0;
  // prefetch tile 0 into registers
  ${r(VL).map((k) => `var v${k}: f32 = 0.0;`).join(" ")}
  var pr: f32 = 0.0;
  {
    ${r(VL).map((k) => `if (${(256 * k) / SL}u + l / ${SL}u < n) { v${k} = dao_v[(${(256 * k) / SL}u + l / ${SL}u) * cfg.kvDim + vb + l % ${SL}u]; }`).join("\n    ")}
    if (l < ${G * T}u && l % ${T}u < n) { pr = dao_p[pb + (l / ${T}u) * cfg.maxSeq + l % ${T}u]; }
  }
  for (var t0: u32 = 0u; t0 < n; t0 += ${T}u) {
    ${r(VL).map((k) => `dao_vt[${256 * k}u + l] = v${k};`).join(" ")}
    if (l < ${G * T}u) { dao_pt[l] = pr; }
    workgroupBarrier();
    // prefetch the next tile while the chains run
    let t1 = t0 + ${T}u;
    if (t1 < n) {
      ${r(VL).map((k) => `if (t1 + ${(256 * k) / SL}u + l / ${SL}u < n) { v${k} = dao_v[(t1 + ${(256 * k) / SL}u + l / ${SL}u) * cfg.kvDim + vb + l % ${SL}u]; }`).join("\n      ")}
      if (l < ${G * T}u && t1 + l % ${T}u < n) { pr = dao_p[pb + (l / ${T}u) * cfg.maxSeq + t1 + l % ${T}u]; }
    }
    if (l < ${G * SL}u) {
      let m = min(${T}u, n - t0);
      for (var tt: u32 = 0u; tt < m; tt++) { acc += dao_pt[ch * ${T}u + tt] * dao_vt[tt * ${SL}u + ci]; }
    }
    workgroupBarrier();
  }
  if (l < ${G * SL}u) { dao_o[col * dao_mc.s0 + (g * ${G}u + ch) * hd + i0 + ci] = acc; }
}
`;
}

// Dense-engine glue kernels, bit-identical to the dispatches they replace (engine/dense.js fuse):
//   attn_glue_d: per head, the Qwen3 q/k head_norm (one thread's in-order sum of squares, then
//     x = x * (inv * w)), rope on (i, i + hd/2) at frame.pos + column, and for the K heads the store
//     of the rotated K row and of the V row into the caches (replaces 2 head_norm + 2 rope dispatches
//     and the two cache copies, which also split the layer into two compute passes).
//   rmsnorm_dmc: base.js rmsnorm for column wg.y (strided x / y), one dispatch for every column.
export const DENSE_GLUE_WGSL = /* wgsl */ `
struct DGL { qs: u32, ks: u32, vs: u32, norm: u32 };   // norm: 1 = apply QK-norm here, 0 = already applied (or none)   // column strides (f32s) of q, k, v; norm: 1 = QK-norm
@group(1) @binding(0) var<storage, read_write> dgl_q: array<f32>;
@group(1) @binding(1) var<storage, read> dgl_k: array<f32>;
@group(1) @binding(2) var<storage, read> dgl_v: array<f32>;
@group(1) @binding(3) var<storage, read> dgl_qw: array<f32>;
@group(1) @binding(4) var<storage, read> dgl_kw: array<f32>;
@group(1) @binding(5) var<storage, read_write> dgl_kc: array<f32>;
@group(1) @binding(6) var<storage, read_write> dgl_vc: array<f32>;
@group(1) @binding(7) var<uniform> dgl: DGL;
var<workgroup> dgl_inv: f32;
@compute @workgroup_size(64)
fn attn_glue_d(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let hh = wg.x; let col = wg.y; let j = lid.x;
  let hd = cfg.headDim; let half = hd / 2u;
  let isQ = hh < cfg.nH;
  let h = select(hh - cfg.nH, hh, isQ);
  let off = select(col * dgl.ks, col * dgl.qs, isQ) + h * hd;
  let pos = frame.pos + col;
  if (dgl.norm != 0u) {
    if (j == 0u) {
      var ss: f32 = 0.0;
      if (isQ) { for (var i: u32 = 0u; i < hd; i++) { let v = dgl_q[off + i]; ss += v * v; } }
      else { for (var i: u32 = 0u; i < hd; i++) { let v = dgl_k[off + i]; ss += v * v; } }
      dgl_inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
    }
    workgroupBarrier();
  }
  let inv = dgl_inv;
  // one pair per thread (hd / 2 <= 64), straight-line like the rope kernel: inside a loop the compiler
  // may hoist 1 / headDim out of the division and change the frequencies' rounding
  let i = j;
  if (i < half) {
    var a: f32; var b: f32;
    if (isQ) { a = dgl_q[off + i]; b = dgl_q[off + i + half]; } else { a = dgl_k[off + i]; b = dgl_k[off + i + half]; }
    if (dgl.norm != 0u) {
      if (isQ) { a *= inv * dgl_qw[i]; b *= inv * dgl_qw[i + half]; }
      else { a *= inv * dgl_kw[i]; b *= inv * dgl_kw[i + half]; }
    }
    let freq = pow(cfg.theta, -f32(2u * i) / f32(cfg.headDim));
    let ang = f32(pos) * freq;
    let c = cos(ang); let s = sin(ang);
    let ra = a * c - b * s;
    let rb = b * c + a * s;
    if (isQ) { dgl_q[off + i] = ra; dgl_q[off + i + half] = rb; }
    else {
      let cb = pos * cfg.kvDim + h * hd;
      dgl_kc[cb + i] = ra; dgl_kc[cb + i + half] = rb;
      let vo = col * dgl.vs + h * hd;
      dgl_vc[cb + i] = dgl_v[vo + i]; dgl_vc[cb + i + half] = dgl_v[vo + i + half];
    }
  }
}

struct DRN { n: u32, xs: u32, ys: u32, pad: u32 };
@group(1) @binding(0) var<storage, read> drn_x: array<f32>;
@group(1) @binding(1) var<storage, read> drn_w: array<f32>;
@group(1) @binding(2) var<storage, read_write> drn_y: array<f32>;
@group(1) @binding(3) var<uniform> drn: DRN;
var<workgroup> drn_partial: array<f32, 256>;
@compute @workgroup_size(256)
fn rmsnorm_dmc(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let n = drn.n;
  let xo = wg.y * drn.xs; let yo = wg.y * drn.ys;
  var ss: f32 = 0.0;
  for (var i: u32 = t; i < n; i += 256u) { let v = drn_x[xo + i]; ss += v * v; }
  drn_partial[t] = ss;
  workgroupBarrier();
  var stride: u32 = 128u;
  while (stride > 0u) {
    if (t < stride) { drn_partial[t] += drn_partial[t + stride]; }
    workgroupBarrier();
    stride = stride / 2u;
  }
  let inv = inverseSqrt(drn_partial[0] / f32(n) + cfg.eps);
  for (var i: u32 = t; i < n; i += 256u) { drn_y[yo + i] = drn_x[xo + i] * inv * drn_w[i]; }
}
`;

// The same glue as three plain kernels for every column at once, each a copy of the reference kernel
// with the column in gid.y (head_norm, rope) plus a cache store, so the arithmetic compiles like the
// reference: head_norm_dmc (q heads then k heads), rope_dmc (q pairs then k pairs, at frame.pos + column),
// kv_store_d (the rotated K row and the V row into the caches at frame.pos + column).
export const DENSE_GLUE3_WGSL = /* wgsl */ `
@group(1) @binding(0) var<storage, read_write> hnd_q: array<f32>;
@group(1) @binding(1) var<storage, read_write> hnd_k: array<f32>;
@group(1) @binding(2) var<storage, read> hnd_qw: array<f32>;
@group(1) @binding(3) var<storage, read> hnd_kw: array<f32>;
@group(1) @binding(4) var<uniform> hnd: DGL;
@compute @workgroup_size(32)
fn head_norm_dmc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let hh = gid.x; let col = gid.y;
  if (hh >= cfg.nH + cfg.nKV) { return; }
  if (hh < cfg.nH) {
    let off = col * hnd.qs + hh * cfg.headDim;
    var ss: f32 = 0.0;
    for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = hnd_q[off + i]; ss += v * v; }
    let inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
    for (var i: u32 = 0u; i < cfg.headDim; i++) { hnd_q[off + i] *= inv * hnd_qw[i]; }
  } else {
    let off = col * hnd.ks + (hh - cfg.nH) * cfg.headDim;
    var ss: f32 = 0.0;
    for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = hnd_k[off + i]; ss += v * v; }
    let inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
    for (var i: u32 = 0u; i < cfg.headDim; i++) { hnd_k[off + i] *= inv * hnd_kw[i]; }
  }
}
@group(1) @binding(0) var<storage, read_write> rpd_q: array<f32>;
@group(1) @binding(1) var<storage, read_write> rpd_k: array<f32>;
@group(1) @binding(2) var<uniform> rpd: DGL;
@compute @workgroup_size(64)
fn rope_dmc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let half = cfg.headDim / 2u;
  let nq = cfg.nH * half;
  let idx = gid.x; let col = gid.y;
  if (idx >= nq + cfg.nKV * half) { return; }
  let pos = frame.pos + col;
  if (idx < nq) {
    let h = idx / half;
    let i = idx % half;
    let off = col * rpd.qs + h * cfg.headDim;
    let freq = pow(cfg.theta, -f32(2u * i) / f32(cfg.headDim));
    let ang = f32(pos) * freq;
    let c = cos(ang); let s = sin(ang);
    let a = rpd_q[off + i]; let b = rpd_q[off + i + half];
    rpd_q[off + i] = a * c - b * s;
    rpd_q[off + i + half] = b * c + a * s;
  } else {
    let h = (idx - nq) / half;
    let i = (idx - nq) % half;
    let off = col * rpd.ks + h * cfg.headDim;
    let freq = pow(cfg.theta, -f32(2u * i) / f32(cfg.headDim));
    let ang = f32(pos) * freq;
    let c = cos(ang); let s = sin(ang);
    let a = rpd_k[off + i]; let b = rpd_k[off + i + half];
    rpd_k[off + i] = a * c - b * s;
    rpd_k[off + i + half] = b * c + a * s;
  }
}
@group(1) @binding(0) var<storage, read> kvd_k: array<f32>;
@group(1) @binding(1) var<storage, read> kvd_v: array<f32>;
@group(1) @binding(2) var<storage, read_write> kvd_kc: array<f32>;
@group(1) @binding(3) var<storage, read_write> kvd_vc: array<f32>;
@group(1) @binding(4) var<uniform> kvd: DGL;
@compute @workgroup_size(64)
fn kv_store_d(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x; let col = gid.y;
  if (i >= cfg.kvDim) { return; }
  let cb = (frame.pos + col) * cfg.kvDim + i;
  kvd_kc[cb] = kvd_k[col * kvd.ks + i];
  kvd_vc[cb] = kvd_v[col * kvd.vs + i];
}
// Single-binding forms for a merged q | k | v buffer (mergeQKV): Dawn rejects two writable bindings of
// one buffer in a dispatch, so q and k are one read_write binding and k starts at koff. Same code as
// head_norm_dmc / rope_dmc above otherwise.
struct DGL1 { qs: u32, ks: u32, vs: u32, norm: u32, koff: u32, p0: u32, p1: u32, p2: u32 };
@group(1) @binding(0) var<storage, read_write> hn1_x: array<f32>;
@group(1) @binding(1) var<storage, read> hn1_qw: array<f32>;
@group(1) @binding(2) var<storage, read> hn1_kw: array<f32>;
@group(1) @binding(3) var<uniform> hn1: DGL1;
@compute @workgroup_size(32)
fn head_norm_dmc1(@builtin(global_invocation_id) gid: vec3<u32>) {
  let hh = gid.x; let col = gid.y;
  if (hh >= cfg.nH + cfg.nKV) { return; }
  if (hh < cfg.nH) {
    let off = col * hn1.qs + hh * cfg.headDim;
    var ss: f32 = 0.0;
    for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = hn1_x[off + i]; ss += v * v; }
    let inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
    for (var i: u32 = 0u; i < cfg.headDim; i++) { hn1_x[off + i] *= inv * hn1_qw[i]; }
  } else {
    let off = hn1.koff + col * hn1.ks + (hh - cfg.nH) * cfg.headDim;
    var ss: f32 = 0.0;
    for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = hn1_x[off + i]; ss += v * v; }
    let inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
    for (var i: u32 = 0u; i < cfg.headDim; i++) { hn1_x[off + i] *= inv * hn1_kw[i]; }
  }
}
@group(1) @binding(0) var<storage, read_write> rp1_x: array<f32>;
@group(1) @binding(1) var<uniform> rp1: DGL1;
@compute @workgroup_size(64)
fn rope_dmc1(@builtin(global_invocation_id) gid: vec3<u32>) {
  let half = cfg.headDim / 2u;
  let nq = cfg.nH * half;
  let idx = gid.x; let col = gid.y;
  if (idx >= nq + cfg.nKV * half) { return; }
  let pos = frame.pos + col;
  if (idx < nq) {
    let h = idx / half;
    let i = idx % half;
    let off = col * rp1.qs + h * cfg.headDim;
    let freq = pow(cfg.theta, -f32(2u * i) / f32(cfg.headDim));
    let ang = f32(pos) * freq;
    let c = cos(ang); let s = sin(ang);
    let a = rp1_x[off + i]; let b = rp1_x[off + i + half];
    rp1_x[off + i] = a * c - b * s;
    rp1_x[off + i + half] = b * c + a * s;
  } else {
    let h = (idx - nq) / half;
    let i = (idx - nq) % half;
    let off = rp1.koff + col * rp1.ks + h * cfg.headDim;
    let freq = pow(cfg.theta, -f32(2u * i) / f32(cfg.headDim));
    let ang = f32(pos) * freq;
    let c = cos(ang); let s = sin(ang);
    let a = rp1_x[off + i]; let b = rp1_x[off + i + half];
    rp1_x[off + i] = a * c - b * s;
    rp1_x[off + i + half] = b * c + a * s;
  }
}
`;

