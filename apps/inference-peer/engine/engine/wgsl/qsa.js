// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/qsa.js (MIT; see LICENSE-ENGINE).
// Qwen Sparse Attention (QSA) kernels for Qwen3.8-Flash-Next's 12 attention layers (+ the MTP block).
// Math and llama.cpp citations: tests/reference/qsa_ref.mjs. Host side (buffers, pass order, dispatch sizes):
// engine/qsa.js. Notes: docs/kernels.md "QSA".
//
// Per token at position p, per QSA layer:
//   qsa_proj    [index_q_proj ; index_k_proj] . x  ->  proj [640] = q 4 x 128 | raw key 128   (BF16 weights)
//   qsa_prep    q heads: rmsnorm * q_norm, rope(p) -> qi [512];  raw key -> rawK[p] (f16, no norm, no rope)
//   qsa_pool    every block b this pass completes (4b+3 in the pass):
//               pooled[b] = rope(rmsnorm(mean(rawK[4b .. 4b+3])) * k_norm, 4b)  (f16)
//   qsa_score   s[b] = sum_h relu(qi_h . pooled[b]) / sqrt(128) for the visible blocks b < floor((p+1)/4)
//   qsa_select  top min(nv, 512) blocks by (score desc, index asc) -> the column's KV row list, ascending:
//               the chosen blocks' 4 rows each, then the tail rows 4nv .. p; n <= 2051
//   attn_dec_qsa (+ _combine / _combine_g)  engine/wgsl/attn_dec.js with gather: attention over the list
// With p + 1 <= 2051 every visible block is chosen, the list is 0 .. p and attn_dec_qsa gives attn_dec's bits.
//
// Bindings: group 0 is the engine's (cfg, frame: frame.pos is the pass's first position, max(frame.nCols, 1) its
// columns), group 1 the kernel's own buffers plus the QS uniform below. Workgroup memory <= 10.5 KB, no f16 or
// subgroup features, every barrier in uniform control flow (scripts/wgsl-barrier-lint.mjs).
// f16 stores (rawK, pooled keys) round to nearest even by hand (engine/wgsl/f16.js), not pack2x16float, whose
// rounding is the backend's (D3D12 differs from Vulkan / Metal), and rawK is read back exactly (no subnormal flush).
//
// Rope: theta_i = p * ts^i accumulated in f32 (ts = f32(1e7^(-2/64)), passed in QS) as ggml's rope cache builds
// it, and cos / sin after a Cody-Waite reduction to [-pi, pi]. The engine's rope_part takes cos(p * pow(...))
// directly, which at p ~ 1e4..1e5 drifts by ~1e-3 rad from ggml; for the indexer that drift would move scores and
// flip near-tie selections, so these kernels follow ggml's arithmetic instead.

import { F16_WGSL } from "./f16.js";

export const QSA_KPOOL = 4, QSA_NTOP = 512, QSA_MAXSEL = QSA_NTOP * QSA_KPOOL + QSA_KPOOL - 1;   // 2051
export const QSA_IH = 4, QSA_ID = 128, QSA_PROJ = QSA_IH * QSA_ID + QSA_ID;                      // 640
export const QSA_SEL_STRIDE = 2052;    // u32 per column in the row list: n, then up to 2051 rows
export const QSA_SCORE_WG = 256;       // blocks per qsa_score / qsa_select tile row

const header = /* wgsl */ `
struct Config {
  dim: u32, kvDim: u32, nH: u32, nKV: u32,
  headDim: u32, inter: u32, vocab: u32, maxSeq: u32,
  eps: f32, theta: f32, qDim: u32,
};
struct Frame { pos: u32, seqLen: u32, nCols: u32, snap: u32 };
// xs: x column stride (floats), ps: proj column stride, qs: qi column stride, ss: score column stride,
// ls: row-list column stride (u32), nRot: rotary dims, eps: rms eps, ts: rope theta scale f32(base^(-2/nRot))
struct QS { xs: u32, ps: u32, qs: u32, ss: u32, ls: u32, nRot: u32, eps: f32, ts: f32 };
@group(0) @binding(0) var<uniform> cfg: Config;
@group(0) @binding(1) var<uniform> frame: Frame;
${F16_WGSL}
// cos / sin of an f32 angle x >= 0 (x < 2^16 * 2 pi, so p < QSA_MAX_SEQ): x - k * 2 pi in three exact-product steps
// that must not be reassociated, then the builtins
fn qsa_cs(x: f32) -> vec2<f32> {
  let k = round(x * 0.15915494309189535);
  let r = ((x - k * 6.28125) - k * 0.0019378662109375) - k * -2.559031372584286e-6;
  return vec2<f32>(cos(r), sin(r));
}
// NeoX rope of pair i (dims i, i + nRot/2) at position p, ggml's f32 theta: p * ts^i, multiplied up in order
fn qsa_rope(a: f32, b: f32, p: u32, i: u32, ts: f32) -> vec2<f32> {
  var th = f32(p);
  for (var k: u32 = 0u; k < i; k++) { th = th * ts; }
  let cs = qsa_cs(th);
  return vec2<f32>(a * cs.x - b * cs.y, a * cs.y + b * cs.x);
}
`;

// split 2 pi for qsa_cs: c1 has 8 significant bits and c2 8, so k * c1 and k * c2 are exact for k < 2^16
// (positions below ~410K); c3 = 2 pi - c1 - c2 rounded. Checked in tests/unit/qsa_ref_test.js.
// The three subtractions must run in that order: a compiler that reassociates them (fast-math, e.g. Metal) loses
// the exactness. Only Vulkan (GB10) is tested so far; a Metal run should include tests/test_qsa.js.
export const QSA_TWO_PI_SPLIT = [6.28125, 0.0019378662109375, -2.559031372584286e-6];
// the largest maxSeq qsa_cs stays exact for: every angle is <= p, and k = round(p / 2 pi) must stay below 2^16
// (65535.5 * 2 pi = 411,771). engine/qsa.js qsaSizes refuses more. The Flash GGUF's context_length is 262,144.
export const QSA_MAX_SEQ = 411000;

// --- qsa_proj: proj[col][r] = sum_d W[r][d] * x[col][d], W BF16 [640][dim] as u32 pairs (low half = even d) ---
// 8 rows per workgroup, 32 lanes per row over d pairs lane, lane + 32, ...; lanes summed in a fixed tree.
const projWGSL = /* wgsl */ `
@group(1) @binding(0) var<storage, read> pj_x: array<f32>;
@group(1) @binding(1) var<storage, read> pj_w: array<u32>;
@group(1) @binding(2) var<storage, read_write> pj_y: array<f32>;
@group(1) @binding(3) var<uniform> pj_u: QS;
var<workgroup> pj_r: array<f32, 256>;
@compute @workgroup_size(256)
fn qsa_proj(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let tid = lid.x; let lr = tid >> 5u; let ln = tid & 31u;
  let r = wg.x * 8u + lr; let col = wg.y;
  let half = cfg.dim / 2u;
  var acc: f32 = 0.0;
  if (r < ${QSA_PROJ}u) {
    let xb = col * pj_u.xs; let wb = r * half;
    for (var j: u32 = ln; j < half; j += 32u) {
      let w = pj_w[wb + j];
      acc += bitcast<f32>(w << 16u) * pj_x[xb + 2u * j] + bitcast<f32>(w & 0xffff0000u) * pj_x[xb + 2u * j + 1u];
    }
  }
  pj_r[tid] = acc;
  workgroupBarrier();
  for (var s: u32 = 16u; s > 0u; s >>= 1u) { if (ln < s) { pj_r[tid] += pj_r[tid + s]; } workgroupBarrier(); }
  if (ln == 0u && r < ${QSA_PROJ}u) { pj_y[col * pj_u.ps + r] = pj_r[tid]; }
}
`;

// rawK as a ring (qsaWGSL rawRing = R, a power of two): row p lives in slot p % R. qsa_pool reads only the rows of
// the blocks a pass completes, 4 * floor(pos / 4) .. pos + nCols - 1 (at most nCols + 3 rows), so a ring of
// R >= nCols + 3 rows serves every pass of up to nCols columns with the same values as a whole-context rawK
// (the caller checks it: engine/qwen4exp_batch.js). rawRing 0: rawK holds every position (engine/qsa.js, the tests).
const rawRow = (ring, r) => (ring ? `((${r}) & ${ring - 1}u)` : r);
export const qsaRingOk = (ring) => ring === 0 || (Number.isInteger(ring) && ring >= 8 && (ring & (ring - 1)) === 0);

// --- qsa_prep: one workgroup per (head 0..4, column). Heads 0-3: rmsnorm * q_norm, rope(p) -> qi.
// Head 4: the raw key -> rawK[p] as f16 pairs. Sum of squares by a fixed tree. ---
const prepWGSL = (ring) => /* wgsl */ `
@group(1) @binding(0) var<storage, read> pr_proj: array<f32>;
@group(1) @binding(1) var<storage, read> pr_qn: array<f32>;
@group(1) @binding(2) var<storage, read_write> pr_qi: array<f32>;
@group(1) @binding(3) var<storage, read_write> pr_raw: array<u32>;   // [maxSeq or ring rows][64] f16 pairs
@group(1) @binding(4) var<uniform> pr_u: QS;
var<workgroup> pr_v: array<f32, 128>;
var<workgroup> pr_s: array<f32, 128>;
@compute @workgroup_size(128)
fn qsa_prep(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let col = wg.y; let i = lid.x;
  let p = frame.pos + col;
  let v = pr_proj[col * pr_u.ps + h * 128u + i];
  if (h == 4u) {   // raw key: no norm, no rope
    pr_v[i] = v;
    workgroupBarrier();
    if (i < 64u) { pr_raw[${rawRow(ring, "p")} * 64u + i] = pack2x16_rne(vec2<f32>(pr_v[2u * i], pr_v[2u * i + 1u])); }
    return;
  }
  pr_s[i] = v * v;
  workgroupBarrier();
  for (var s: u32 = 64u; s > 0u; s >>= 1u) { if (i < s) { pr_s[i] += pr_s[i + s]; } workgroupBarrier(); }
  let inv = 1.0 / sqrt(pr_s[0] / 128.0 + pr_u.eps);
  pr_v[i] = (v * inv) * pr_qn[i];
  workgroupBarrier();
  let half = pr_u.nRot / 2u;
  var o = pr_v[i];
  if (i < half) { o = qsa_rope(pr_v[i], pr_v[i + half], p, i, pr_u.ts).x; }
  else if (i < pr_u.nRot) { o = qsa_rope(pr_v[i - half], pr_v[i], p, i - half, pr_u.ts).y; }
  pr_qi[col * pr_u.qs + h * 128u + i] = o;
}
`;

// --- qsa_pool: workgroup w handles block b = floor(pos / 4) + w when the pass completes it (pos <= 4b+3 < pos +
// nCols); one thread per dim. Sum ((r0 + r1) + r2) + r3, * 0.25, rmsnorm (fixed tree) * k_norm, rope(4b). ---
const poolWGSL = (ring) => /* wgsl */ `
@group(1) @binding(0) var<storage, read> pl_raw: array<u32>;
@group(1) @binding(1) var<storage, read> pl_kn: array<f32>;
@group(1) @binding(2) var<storage, read_write> pl_out: array<u32>;   // [maxBlocks][64] f16 pairs
@group(1) @binding(3) var<uniform> pl_u: QS;
var<workgroup> pl_v: array<f32, 128>;
var<workgroup> pl_s: array<f32, 128>;
fn pl_raw_at(row: u32, d: u32) -> f32 { let w = unpack2x16_exact(pl_raw[${rawRow(ring, "row")} * 64u + d / 2u]); return select(w.x, w.y, (d & 1u) == 1u); }
@compute @workgroup_size(128)
fn qsa_pool(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let i = lid.x;
  let b = frame.pos / 4u + wg.x;
  let last = 4u * b + 3u;
  if (last < frame.pos || last >= frame.pos + max(frame.nCols, 1u)) { return; }
  var m = pl_raw_at(4u * b, i);
  m = m + pl_raw_at(4u * b + 1u, i);
  m = m + pl_raw_at(4u * b + 2u, i);
  m = m + pl_raw_at(4u * b + 3u, i);
  m = m * 0.25;
  pl_s[i] = m * m;
  workgroupBarrier();
  for (var s: u32 = 64u; s > 0u; s >>= 1u) { if (i < s) { pl_s[i] += pl_s[i + s]; } workgroupBarrier(); }
  let inv = 1.0 / sqrt(pl_s[0] / 128.0 + pl_u.eps);
  pl_v[i] = (m * inv) * pl_kn[i];
  workgroupBarrier();
  let half = pl_u.nRot / 2u;
  var o = pl_v[i];
  if (i < half) { o = qsa_rope(pl_v[i], pl_v[i + half], 4u * b, i, pl_u.ts).x; }
  else if (i < pl_u.nRot) { o = qsa_rope(pl_v[i - half], pl_v[i], 4u * b, i - half, pl_u.ts).y; }
  workgroupBarrier();
  pl_v[i] = o;
  workgroupBarrier();
  if (i < 64u) { pl_out[b * 64u + i] = pack2x16_rne(vec2<f32>(pl_v[2u * i], pl_v[2u * i + 1u])); }
}
`;

// --- qsa_score: workgroup (x: 256 blocks, y: CW columns); thread = block. Per (column, head) the dot runs over
// d in order, 8 dims per step (two vec4 dots), so a score has the same bits for every CW (decode CW = 1, prefill
// CW = 4). Only visible blocks (b < floor((p+1)/4)) are written. ---
function scoreWGSL(CW) {
  const C = [...Array(CW).keys()], H = [0, 1, 2, 3];
  const P = CW === 1 ? "sc" : `sc${CW}`;   // per-variant names: the variants share one module
  return /* wgsl */ `
@group(1) @binding(0) var<storage, read> ${P}_qi: array<f32>;
@group(1) @binding(1) var<storage, read> ${P}_pk: array<vec4<u32>>;   // pooled keys, 16 vec4 (128 f16) per block
@group(1) @binding(2) var<storage, read_write> ${P}_out: array<f32>;
@group(1) @binding(3) var<uniform> ${P}_u: QS;
var<workgroup> ${P}_q: array<vec4<f32>, ${CW * 128}>;
@compute @workgroup_size(256)
fn qsa_score${CW === 1 ? "" : "_c" + CW}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let tid = lid.x; let c0 = wg.y * ${CW}u;
  let nCols = max(frame.nCols, 1u);
  for (var w: u32 = tid; w < ${CW * 128}u; w += 256u) {
    let c = c0 + w / 128u; let k = w % 128u;
    var v = vec4<f32>(0.0);
    if (c < nCols) { let o = c * ${P}_u.qs + 4u * k; v = vec4<f32>(${P}_qi[o], ${P}_qi[o + 1u], ${P}_qi[o + 2u], ${P}_qi[o + 3u]); }
    ${P}_q[w] = v;
  }
  workgroupBarrier();
  let b = wg.x * 256u + tid;
  let nvMax = (frame.pos + min(c0 + ${CW}u, nCols)) / 4u;   // blocks the last column of the group sees
  if (b >= nvMax) { return; }
  ${C.map((c) => H.map((h) => `var a${c}_${h}: f32 = 0.0;`).join(" ")).join("\n  ")}
  for (var k: u32 = 0u; k < 16u; k++) {
    let kw = ${P}_pk[b * 16u + k];
    let ka = vec4<f32>(unpack2x16float(kw.x), unpack2x16float(kw.y));
    let kc = vec4<f32>(unpack2x16float(kw.z), unpack2x16float(kw.w));
    ${C.map((c) => H.map((h) => `a${c}_${h} = a${c}_${h} + dot(${P}_q[${c * 128 + h * 32}u + 2u * k], ka); a${c}_${h} = a${c}_${h} + dot(${P}_q[${c * 128 + h * 32}u + 2u * k + 1u], kc);`).join("\n    ")).join("\n    ")}
  }
  let w = 0.08838834764831845;   // f32(1 / sqrt(128))
  ${C.map((c) => `{
    let col = c0 + ${c}u;
    if (col < nCols && b < (frame.pos + col + 1u) / 4u) {
      var s: f32 = 0.0;
      ${H.map((h) => `s = s + max(a${c}_${h}, 0.0) * w;`).join(" ")}
      ${P}_out[col * ${P}_u.ss + b] = s;
    }
  }`).join("\n  ")}
}
`;
}

// --- qsa_select: one workgroup per column. nv = floor((p+1)/4) visible blocks; nv <= 512: the list is 0 .. p.
// Otherwise find T, the 512th largest key (key = the score's bits: scores are >= 0, so the u32 order is the float
// order; -0 and NaN count as 0), and `need`, how many keys equal to T to keep, by a radix select over 11-, 11- and
// 10-bit digits: per round a 2048-bin workgroup histogram of the keys that match the prefix so far, then a suffix
// scan over the bins (8 per thread) finds the digit holding the rank-th largest key. One workgroup serves a decode
// column alone, so the passes over the scores are latency bound: every loop issues SEL_U loads before using them.
// Then one compaction in block order: thread t takes the contiguous blocks [t*C, (t+1)*C), counts (key > T,
// key == T), one workgroup scan gives its output offset and its share of the ties (the first `need` ties in block
// order are kept), and it writes each kept block's 4 rows; then the tail rows 4nv .. p. Rows come out ascending. ---
const SEL_U = 16;
const U = [...Array(SEL_U).keys()];
const selectWGSL = /* wgsl */ `
@group(1) @binding(0) var<storage, read> se_sc: array<f32>;
@group(1) @binding(1) var<storage, read_write> se_out: array<u32>;
@group(1) @binding(2) var<uniform> se_u: QS;
var<workgroup> se_h: array<atomic<u32>, 2048>;
var<workgroup> se_x: array<vec2<u32>, 256>;   // scans
var<workgroup> se_st: vec2<u32>;              // prefix so far, rank left
fn se_key(s: f32) -> u32 { return select(0u, bitcast<u32>(s), s > 0.0); }
@compute @workgroup_size(256)
fn qsa_select(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let tid = lid.x; let col = wg.x;
  let p = frame.pos + col;
  let nv = (p + 1u) / 4u; let tail = (p + 1u) % 4u;
  let ob = col * se_u.ls; let sb = col * se_u.ss;
  if (nv <= ${QSA_NTOP}u) {
    for (var e: u32 = tid; e <= p; e += 256u) { se_out[ob + 1u + e] = e; }
    if (tid == 0u) { se_out[ob] = p + 1u; }
    return;
  }
  if (tid == 0u) { se_st = vec2<u32>(0u, ${QSA_NTOP}u); }
  for (var rd: u32 = 0u; rd < 3u; rd++) {
    let shift = select(21u - 11u * rd, 0u, rd == 2u);        // digits: bits 31..21, 20..10, 9..0
    let dmask = select(2047u, 1023u, rd == 2u);
    let hiMask = select(0u, 0xffffffffu << (shift + select(11u, 10u, rd == 2u)), rd > 0u);
    for (var i: u32 = 0u; i < 8u; i++) { atomicStore(&se_h[tid * 8u + i], 0u); }
    let st = workgroupUniformLoad(&se_st);
    for (var e0: u32 = 0u; e0 < nv; e0 += ${256 * SEL_U}u) {
      ${U.map((u) => `let x${u} = e0 + ${u * 256}u + tid; let s${u} = se_sc[sb + min(x${u}, nv - 1u)];`).join("\n      ")}
      ${U.map((u) => `{ let k = se_key(s${u}); if (x${u} < nv && (k & hiMask) == st.x) { atomicAdd(&se_h[(k >> shift) & dmask], 1u); } }`).join("\n      ")}
    }
    workgroupBarrier();
    // thread t owns bins 8t .. 8t+7; suffix sums over the threads (from the top bin down) find the digit
    var own: u32 = 0u;
    for (var i: u32 = 0u; i < 8u; i++) { own += atomicLoad(&se_h[tid * 8u + i]); }
    se_x[tid] = vec2<u32>(own, 0u);
    workgroupBarrier();
    for (var s: u32 = 1u; s < 256u; s <<= 1u) {
      var v = se_x[tid];
      if (tid + s < 256u) { v.x += se_x[tid + s].x; }
      workgroupBarrier();
      se_x[tid] = v;
      workgroupBarrier();
    }
    let above = se_x[tid].x - own;   // keys in higher threads' bins
    if (above < st.y && above + own >= st.y) {
      var a = above; var j: u32 = 7u;
      loop {
        let c = atomicLoad(&se_h[tid * 8u + j]);
        if (a + c >= st.y || j == 0u) { break; }
        a += c; j -= 1u;
      }
      se_st = vec2<u32>(st.x | ((tid * 8u + j) << shift), st.y - a);
    }
    workgroupBarrier();
  }
  let fin = workgroupUniformLoad(&se_st);
  let T = fin.x; let need = fin.y;   // keep keys > T, and the first 'need' keys == T in block order
  let C = (nv + 255u) / 256u;
  let b0 = tid * C;
  var gt: u32 = 0u; var tie: u32 = 0u;
  for (var i0: u32 = 0u; i0 < C; i0 += ${SEL_U}u) {
    ${U.map((u) => `let s${u} = se_sc[sb + min(b0 + i0 + ${u}u, nv - 1u)];`).join(" ")}
    ${U.map((u) => `if (i0 + ${u}u < C && b0 + i0 + ${u}u < nv) { let k = se_key(s${u}); gt += select(0u, 1u, k > T); tie += select(0u, 1u, k == T); }`).join("\n    ")}
  }
  // inclusive scan of (gt, tie) over the threads (Hillis-Steele, fixed order)
  se_x[tid] = vec2<u32>(gt, tie);
  workgroupBarrier();
  for (var s: u32 = 1u; s < 256u; s <<= 1u) {
    var v = se_x[tid];
    if (tid >= s) { v += se_x[tid - s]; }
    workgroupBarrier();
    se_x[tid] = v;
    workgroupBarrier();
  }
  let inc = se_x[tid];
  var tieRank = inc.y - tie;                              // ties in earlier blocks
  var o = (inc.x - gt) + min(tieRank, need);              // kept blocks before this thread's
  for (var i0: u32 = 0u; i0 < C; i0 += ${SEL_U}u) {
    ${U.map((u) => `let s${u} = se_sc[sb + min(b0 + i0 + ${u}u, nv - 1u)];`).join(" ")}
    ${U.map((u) => `if (i0 + ${u}u < C && b0 + i0 + ${u}u < nv) {
      let e = b0 + i0 + ${u}u; let k = se_key(s${u});
      var keep = k > T;
      if (k == T) { keep = tieRank < need; tieRank += 1u; }
      if (keep) { let r = ob + 1u + 4u * o; se_out[r] = 4u * e; se_out[r + 1u] = 4u * e + 1u; se_out[r + 2u] = 4u * e + 2u; se_out[r + 3u] = 4u * e + 3u; o += 1u; }
    }`).join("\n    ")}
  }
  if (tid < tail) { se_out[ob + 1u + ${4 * QSA_NTOP}u + tid] = 4u * nv + tid; }
  if (tid == 0u) { se_out[ob] = ${4 * QSA_NTOP}u + tail; }
}
`;

// every QSA kernel but the attention in one module (entry points qsa_proj, qsa_prep, qsa_pool, qsa_score,
// qsa_score_c4, qsa_select). rawRing: rawK's rows when it is a ring (a power of two), 0 for the whole context.
export function qsaWGSL({ scoreCW = [1, 4], rawRing = 0 } = {}) {
  if (!qsaRingOk(rawRing)) throw new Error(`QSA: rawK ring of ${rawRing} rows (0, or a power of two >= 8)`);
  return header + projWGSL + prepWGSL(rawRing) + poolWGSL(rawRing) + scoreCW.map(scoreWGSL).join("") + selectWGSL;
}
