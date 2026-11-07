// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/qwen4exp_wide.js (MIT; see LICENSE-ENGINE).
// Wide prefill kernels for Qwen3.8-Flash-Next (engine/qwen4exp_batch.js: the default for prefill passes of at least
// FAST_MIN_COLS columns; Qwen4ExpBatch's exact: true keeps the bit-exact kernels of engine/wgsl/qwen4exp_batch.js).
//
// The exact kernels are the decode kernels with a column index: each weight block is read once per GEMV_MC_COLS (4)
// columns, the routed experts once per (column, slot), and attention runs per (query head, column). These share
// every weight read across a tile of columns or an expert's pairs, and every K / V read across a KV head's query
// heads, in another summation order than decode: a pass is within a tolerance of the decode steps, not bit-equal
// (tests/test_q38flash_wide.js per kernel against float64, tests/test_q38flash_batch.js per layer on the model).
//
//   gemm_t_<fmt>      y[col * ys + yOff + r] = epi(W[r, :] . x[col]) over the pass's columns: a shared-memory tiled
//                     GEMM (64 x 64 tiles, 4 x 4 outputs per thread), the weight tile dequantized once per 64
//                     columns. epi: mode 0 plain, mode 1 silu(acc * scale) (the HC down projection), mode 2
//                     silu(aux) * acc (the shared expert's up projection, aux = its gate projection). Weights q4 / q8
//                     (the engine's repacked Q4_0 / Q8_0) or f32 rows. Split-K (workgroup z) into a partial buffer
//                     for shapes with few output tiles, then split_sum (in order, with the epilogue).
//   gemm_d_<fmt>      the same on dp4a (option dp4a): activations quantized per 32 values by quant_x (llama.cpp's
//                     q8_1 / MMQ numerics), exact i32 block dots, f32 * the scales.
//   moe_sort_w        one workgroup: the pass's nCols * K (column, slot) pairs counted per expert (workgroup atomics)
//                     and placed in a pair list grouped by expert, cut into tiles of at most BP pairs of one expert.
//                     The order inside an expert is free (atomics), and need not be fixed: a pair's sums do not
//                     depend on which tile or slot it lands in, so a pass is deterministic.
//   moe_gu_w_<fmt>    per (row block of 128, tile): gate and up rows of the tile's expert against the tile's pairs,
//                     h[pair][r] = silu(g) * u; each thread one row of both matrices and the BP pairs, the pairs'
//                     activations staged in workgroup memory: the expert's rows are read once per tile, not per pair.
//   moe_dn_w_<fmt>    per (row block of 128, tile): the down projection of each pair, y[pair][r].
//   moe_gu_wc / moe_dn_wc  the same on dp4a, with coalesced weight-tile loads (64 rows per workgroup).
//   moe_comb_w        moe[col][r] = sum_k w_k y[col * K + k][r] (k ascending) + sigmoid(shared gate) * sh[col][r].
//   attn_w            attention with a KV head's 12 query heads (and in the dense phase 5 columns) per workgroup
//                     sharing each K / V tile, online softmax; dense (causal) or gathered (QSA's row lists).
//   dn_delta_w / dn_gnorm_w  the exact DeltaNet recurrence (its state in registers, inputs loaded a column ahead)
//                     with its gated output norm moved out of the sequential loop; dn_conv_w / dn_conv_st_w the conv
//                     over every (head, column) at once. Same expressions: the exact kernels' bits.
//
// Pair p = col * K + slot, so h keeps the exact kernels' [col][K][expFF] layout. Offloaded layers bind the expert
// store's region (regionEntry) with the original expert ids, as the exact kernels do.

import { kvCheck, kvDecl, kvK4, KV_F16_READ } from "./qwen4exp.js";

const DQ = /* wgsl */ `
fn q4lo(w: u32) -> vec4<f32> { return vec4<f32>(f32(w & 15u), f32((w >> 8u) & 15u), f32((w >> 16u) & 15u), f32((w >> 24u) & 15u)) - vec4<f32>(8.0); }
fn q4hi(w: u32) -> vec4<f32> { return vec4<f32>(f32((w >> 4u) & 15u), f32((w >> 12u) & 15u), f32((w >> 20u) & 15u), f32((w >> 28u) & 15u)) - vec4<f32>(8.0); }
fn i8x4(u: u32) -> vec4<f32> { let v = bitcast<i32>(u); return vec4<f32>(f32((v << 24u) >> 24u), f32((v << 16u) >> 24u), f32((v << 8u) >> 24u), f32(v >> 24u)); }
fn sigm(v: f32) -> f32 { return 1.0 / (1.0 + exp(-v)); }
fn silu(v: f32) -> f32 { return v / (1.0 + exp(-v)); }
`;
const MCF = `struct McF { pos: u32, nCols: u32, p0: u32, p1: u32 };`;
const rng = (n) => Array.from({ length: n }, (_, i) => i);

export const FAST_MIN_COLS = 16;   // passes narrower than this keep the exact kernels
export const WIDE_T = Object.freeze({ BM: 64, BN: 64, TM: 4, TN: 4, KH: 2 });
export const MOE_BP = 16;          // pairs per expert tile
export const MOE_RW = 128;         // rows per expert workgroup (one per thread)

// The 8 vec4 of a quant block's 32 values (element 4i + c in e<i>[c]), scaled; blk = block index.
const deq = (kind, QS, SC, blk, pre = "e") => kind === "q4" ? `
      let ${pre}s = unpack2x16float(${SC}[(${blk}) >> 1u])[(${blk}) & 1u]; let ${pre}w = ${QS}[${blk}];
      let ${pre}0 = q4lo(${pre}w.x) * ${pre}s; let ${pre}1 = q4lo(${pre}w.y) * ${pre}s; let ${pre}2 = q4lo(${pre}w.z) * ${pre}s; let ${pre}3 = q4lo(${pre}w.w) * ${pre}s;
      let ${pre}4 = q4hi(${pre}w.x) * ${pre}s; let ${pre}5 = q4hi(${pre}w.y) * ${pre}s; let ${pre}6 = q4hi(${pre}w.z) * ${pre}s; let ${pre}7 = q4hi(${pre}w.w) * ${pre}s;` : `
      let ${pre}s = unpack2x16float(${SC}[(${blk}) >> 1u])[(${blk}) & 1u]; let ${pre}a = ${QS}[(${blk}) * 2u]; let ${pre}b = ${QS}[(${blk}) * 2u + 1u];
      let ${pre}0 = i8x4(${pre}a.x) * ${pre}s; let ${pre}1 = i8x4(${pre}a.y) * ${pre}s; let ${pre}2 = i8x4(${pre}a.z) * ${pre}s; let ${pre}3 = i8x4(${pre}a.w) * ${pre}s;
      let ${pre}4 = i8x4(${pre}b.x) * ${pre}s; let ${pre}5 = i8x4(${pre}b.y) * ${pre}s; let ${pre}6 = i8x4(${pre}b.z) * ${pre}s; let ${pre}7 = i8x4(${pre}b.w) * ${pre}s;`;

// ---- gemm_t: tiled GEMM over the pass's columns ----
// Tile BM rows x BN columns per workgroup of (BM / TM) x (BN / TN) = 256 threads, each TM x TN outputs: row quads
// ty + TY * i and column quads tx + TX * j (TY = BM / TM, TX = BN / TN; consecutive threads read consecutive vec4s of
// the staged tiles). Per stage KH halves of a quant block (16 values each: Q4_0's low or high nibbles, one Q8_0 vec4,
// four f32 vec4s): the weight tile dequantized once into workgroup memory (k-major, vec4 over 4 rows) and the
// activation tile (k-major, vec4 over 4 columns), then 16 * KH rank-1 updates. Workgroup memory 64 * KH * (BM + BN).
// Bindings: 0 qs (q4 / q8) or w (f32 rows; bf16 rows as u32 pairs, low half first: QSA's indexer weights), 1 sc, 2 x, 3 y, 4 P, 5 F (frame: nCols), 6 aux (mode 2).
// Workgroups (ceil(dOut / BM), ceil(nCols / BN)). x column c at x[(xOff4 + c * xs4) * 4 ..]; columns past nCols
// read column nCols - 1 and are not stored. One f32 accumulator per output, k ascending.
export function gemmTWGSL(kind, c = WIDE_T) {
  const { BM, BN, TM, TN, KH } = c, TY = BM / TM, TX = BN / TN, T = TY * TX, BM4 = BM / 4, BN4 = BN / 4, KS = 16 * KH;
  if (T !== 256 || TM % 4 || TN % 4 || 64 * KH * (BM + BN) > 16384) throw new Error(`gemm_t: tile ${JSON.stringify(c)}`);
  const R4 = rng(4), MI = rng(TM / 4), NJ = rng(TN / 4);
  // weight staging units per half: (row quad, j) -> 4 values of 4 rows (q4: word j's low / high nibbles; q8: word j of
  // the half's vec4; f32: the half's vec4 j)
  const wUnits = BM4 * 4 * KH, xUnits = BN4 * 4 * KH;
  const wval = (r) => kind === "q4" ? `select(q4hi(wt_q[g${r}][jj]), q4lo(wt_q[g${r}][jj]), hf == 0u) * unpack2x16float(wt_sc[g${r} >> 1u])[g${r} & 1u]`
    : kind === "q8" ? `i8x4(wt_q[g${r} * 2u + hf][jj]) * unpack2x16float(wt_sc[g${r} >> 1u])[g${r} & 1u]`
    : kind === "bf16" ? `bf16x4(wt_b[min(row0 + rq * 4u + ${r}u, dOut - 1u) * n4 + kb * 8u + hf * 4u + jj])`
    : `wt_w[min(row0 + rq * 4u + ${r}u, dOut - 1u) * n4 + kb * 8u + hf * 4u + jj]`;
  const wload = (j) => `
    { let li = t + ${j * T}u; if (li < ${wUnits}u) {
      let rq = li % ${BM4}u; let rest = li / ${BM4}u; let jj = rest % 4u; let hh = rest / 4u; let hf = (s * ${KH}u + hh) % 2u; let kb = (s * ${KH}u + hh) / 2u;
      ${kind === "f32" || kind === "bf16" ? "" : R4.map((r) => `let g${r} = min(row0 + rq * 4u + ${r}u, dOut - 1u) * nb + kb;`).join(" ")}
      ${R4.map((r) => `let d${r} = ${wval(r)};`).join("\n      ")}
      ${R4.map((i) => `wt_W[(hh * 16u + 4u * jj + ${i}u) * ${BM4}u + rq] = vec4<f32>(d0[${i}], d1[${i}], d2[${i}], d3[${i}]);`).join("\n      ")}
    } }`;
  const xload = (j) => `
    { let li = t + ${j * T}u; if (li < ${xUnits}u) {
      let cq = li % ${BN4}u; let rest = li / ${BN4}u; let jj = rest % 4u; let hh = rest / 4u; let k4 = (s * ${KH}u + hh) * 4u + jj;
      ${R4.map((cc) => `let v${cc} = wt_x[xo + min(col0 + cq * 4u + ${cc}u, nc - 1u) * xs4 + k4];`).join(" ")}
      ${R4.map((i) => `wt_X[(hh * 16u + 4u * jj + ${i}u) * ${BN4}u + cq] = vec4<f32>(v0[${i}], v1[${i}], v2[${i}], v3[${i}]);`).join("\n      ")}
    } }`;
  const acc = (i, rr, j) => `a${i}_${rr}_${j}`;
  const step = (k) => `
      { ${MI.map((i) => `let wv${i} = wt_W[${k * BM4}u + ty + ${i * TY}u];`).join(" ")} ${NJ.map((j) => `let xv${j} = wt_X[${k * BN4}u + tx + ${j * TX}u];`).join(" ")}
        ${MI.map((i) => R4.map((rr) => NJ.map((j) => `${acc(i, rr, j)} += wv${i}[${rr}] * xv${j};`).join(" ")).join(" ")).join("\n        ")} }`;
  const store = MI.map((i) => R4.map((rr) => `
  { let row = row0 + (ty + ${i * TY}u) * 4u + ${rr}u; if (row < dOut) {
    ${NJ.map((j) => R4.map((cc) => `{ let col = col0 + (tx + ${j * TX}u) * 4u + ${cc}u; if (col < nc) { put(${acc(i, rr, j)}[${cc}], col, row, wg.z, nc); } }`).join(" ")).join("\n    ")}
  } }`).join("")).join("");
  const W = kind === "f32" ? `@group(0) @binding(0) var<storage, read> wt_w: array<vec4<f32>>;`
    : kind === "bf16" ? `@group(0) @binding(0) var<storage, read> wt_b: array<vec2<u32>>;
fn bf16x4(w: vec2<u32>) -> vec4<f32> {
  return vec4<f32>(bitcast<f32>(w.x << 16u), bitcast<f32>(w.x & 0xFFFF0000u), bitcast<f32>(w.y << 16u), bitcast<f32>(w.y & 0xFFFF0000u));
}`
    : `@group(0) @binding(0) var<storage, read> wt_q: array<vec4<u32>>;\n@group(0) @binding(1) var<storage, read> wt_sc: array<u32>;`;
  return DQ + `
struct GP { dOut: u32, dIn: u32, xOff4: u32, xs4: u32, yOff: u32, ys: u32, mode: u32, scale: f32, kPer: u32, split: u32, p0: u32, p1: u32 };
${MCF}
${W}
@group(0) @binding(2) var<storage, read> wt_x: array<vec4<f32>>;
@group(0) @binding(3) var<storage, read_write> wt_y: array<f32>;
@group(0) @binding(4) var<uniform> wt_P: GP;
@group(0) @binding(5) var<uniform> wt_F: McF;
@group(0) @binding(6) var<storage, read> wt_aux: array<f32>;
var<workgroup> wt_W: array<vec4<f32>, ${KS * BM4}>;
var<workgroup> wt_X: array<vec4<f32>, ${KS * BN4}>;
fn epi(v: f32, o: u32) -> f32 {
  if (wt_P.mode == 1u) { return silu(v * wt_P.scale); }
  if (wt_P.mode == 2u) { return silu(wt_aux[o]) * v; }
  return v;
}
fn put(v: f32, col: u32, row: u32, z: u32, nc: u32) {
  if (wt_P.split == 1u) { wt_y[(z * nc + col) * wt_P.dOut + row] = v; return; }
  let o = col * wt_P.ys + wt_P.yOff + row; wt_y[o] = epi(v, o);
}
@compute @workgroup_size(${T})
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let tx = t % ${TX}u; let ty = t / ${TX}u;
  let dOut = wt_P.dOut; let nb = wt_P.dIn / 32u; let n4 = wt_P.dIn / 4u; let xs4 = wt_P.xs4; let xo = wt_P.xOff4;
  let nc = wt_F.nCols;
  let row0 = wg.x * ${BM}u; let col0 = wg.y * ${BN}u;
  ${MI.map((i) => R4.map((rr) => NJ.map((j) => `var ${acc(i, rr, j)} = vec4<f32>(0.0);`).join(" ")).join(" ")).join("\n  ")}
  // split-K (P.split = 1): workgroup z sums blocks z * kPer .. + kPer, raw, into the partial buffer y[(z * nc + col) * dOut + row]
  let kb0 = wg.z * wt_P.kPer; let kb1 = min(nb, kb0 + wt_P.kPer);
  for (var s = kb0 * ${2 / KH}u; s < kb1 * ${2 / KH}u; s++) {
    workgroupBarrier();
    ${rng(Math.ceil(wUnits / T)).map(wload).join("")}
    ${rng(Math.ceil(xUnits / T)).map(xload).join("")}
    workgroupBarrier();
    ${rng(KS).map(step).join("")}
  }
  ${store}
}`;
}
// ---- moe_sort_w ----
// grp layout (u32): [0] tile count; tiles at words 4 + 4 t = (first pair-list slot, pairs, expert, 0); the pair
// list at word PO = 4 + 4 * maxTiles. Bindings: 0 ids (col * K + k), 1 grp, 2 P { nExp, K, PO, 0 }, 3 F.
export function moeSortWSizes({ maxCols, K, nExp, BP = MOE_BP }) {
  const pairs = maxCols * K, maxTiles = Math.ceil(pairs / BP) + Math.min(nExp, pairs), PO = 4 + 4 * maxTiles;
  return { pairs, maxTiles, PO, words: PO + pairs };
}
export function moeSortWWGSL(nExpMax = 1024, BP = MOE_BP) {
  const QM = Math.ceil(nExpMax / 256);
  return `
struct SP { nExp: u32, K: u32, PO: u32, p0: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> ms_ids: array<u32>;
@group(0) @binding(1) var<storage, read_write> ms_grp: array<u32>;
@group(0) @binding(2) var<uniform> ms_P: SP;
@group(0) @binding(3) var<uniform> ms_F: McF;
var<workgroup> ms_h: array<atomic<u32>, ${QM * 256}>;
var<workgroup> ms_off: array<u32, ${QM * 256}>;
var<workgroup> ms_sp: array<u32, 256>;
var<workgroup> ms_st: array<u32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let nE = ms_P.nExp; let N = ms_F.nCols * ms_P.K;
  for (var q = 0u; q < ${QM}u; q++) { atomicStore(&ms_h[q * 256u + t], 0u); }
  workgroupBarrier();
  for (var i = t; i < N; i += 256u) { atomicAdd(&ms_h[min(ms_ids[i], nE - 1u)], 1u); }
  workgroupBarrier();
  // thread t owns experts t * QM .. t * QM + QM - 1: its pair and tile totals, then an exclusive scan over threads
  var sp = 0u; var st = 0u;
  for (var q = 0u; q < ${QM}u; q++) { let c = atomicLoad(&ms_h[t * ${QM}u + q]); sp += c; st += (c + ${BP - 1}u) / ${BP}u; }
  ms_sp[t] = sp; ms_st[t] = st;
  workgroupBarrier();
  for (var o = 1u; o < 256u; o <<= 1u) {
    var a = ms_sp[t]; var b = ms_st[t];
    if (t >= o) { a += ms_sp[t - o]; b += ms_st[t - o]; }
    workgroupBarrier();
    ms_sp[t] = a; ms_st[t] = b;
    workgroupBarrier();
  }
  var pb = ms_sp[t] - sp; var tb = ms_st[t] - st;
  if (t == 255u) { ms_grp[0] = ms_st[255]; }
  for (var q = 0u; q < ${QM}u; q++) {
    let e = t * ${QM}u + q; let c = atomicLoad(&ms_h[e]);
    ms_off[e] = pb;
    for (var k = 0u; k * ${BP}u < c; k++) {
      let ti = 4u + (tb + k) * 4u;
      ms_grp[ti] = pb + k * ${BP}u; ms_grp[ti + 1u] = min(${BP}u, c - k * ${BP}u); ms_grp[ti + 2u] = e; ms_grp[ti + 3u] = 0u;
    }
    pb += c; tb += (c + ${BP - 1}u) / ${BP}u;
  }
  workgroupBarrier();
  for (var q = 0u; q < ${QM}u; q++) { atomicStore(&ms_h[q * 256u + t], 0u); }
  workgroupBarrier();
  for (var i = t; i < N; i += 256u) {
    let e = min(ms_ids[i], nE - 1u);
    let s = atomicAdd(&ms_h[e], 1u);
    ms_grp[ms_P.PO + ms_off[e] + s] = i;
  }
}`;
}

// ---- moe_gu_w / moe_dn_w: row-stationary over a tile of BP pairs of one expert ----
// Shared: the tile's pair indices (ms_pp) and KS = 32 * KB activation values of each pair, k-major, vec4 over 4
// pairs (xs[(k) * (BP / 4) + pq]). Each thread: one row, BP / 4 vec4 accumulators per matrix.
// gu bindings: 0 gq, 1 gs, 2 uq, 3 us, 4 x (mixed, [col][dIn]), 5 h, 6 grp, 7 P { dOut, dIn, K, PO, xs, hs, 0, 0 }.
// dn bindings: 0 q, 1 s, 2 h ([pair][dIn]), 3 y ([pair][dOut]), 4 grp, 5 P { dOut, dIn, K, PO, 0, .. }.
function tileHead(P, BP) {
  return `
  let tile = wg.y;
  if (t == 0u) { ${P}_nt = ${P}_grp[0]; }
  if (tile >= workgroupUniformLoad(&${P}_nt)) { return; }
  let tb = 4u + tile * 4u; let p0 = ${P}_grp[tb]; let np = ${P}_grp[tb + 1u]; let e = ${P}_grp[tb + 2u];
  if (t < ${BP}u) { ${P}_pp[t] = ${P}_grp[${P}_P.PO + p0 + min(t, np - 1u)]; }
  workgroupBarrier();`;
}
const BPQ = MOE_BP / 4;
function fmaBlock(mats, KB) {
  // for every value of the stage's KB blocks: w (each matrix) x the pairs' vec4s
  return rng(KB).map((bb) => rng(8).map((i) => rng(4).map((cc) => {
    const k = bb * 32 + i * 4 + cc;
    return `{ ${rng(BPQ).map((q) => `let x${q} = SX[${k * BPQ}u + ${q}u];`).join(" ")} ${mats.map((m) => rng(BPQ).map((q) => `${m}a${q} += ${m}${bb}_${i}[${cc}] * x${q};`).join(" ")).join(" ")} }`;
  }).join("\n        ")).join("\n        ")).join("\n        ");
}
export function moeGuWWGSL(kind, KB = 2) {
  const RW = MOE_RW, BP = MOE_BP, KS = 32 * KB, xUnits = BP * KS / 4;
  return DQ + `
struct MP { dOut: u32, dIn: u32, K: u32, PO: u32, xs: u32, hs: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> mg_gq: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> mg_gs: array<u32>;
@group(0) @binding(2) var<storage, read> mg_uq: array<vec4<u32>>;
@group(0) @binding(3) var<storage, read> mg_us: array<u32>;
@group(0) @binding(4) var<storage, read> mg_x: array<vec4<f32>>;
@group(0) @binding(5) var<storage, read_write> mg_h: array<f32>;
@group(0) @binding(6) var<storage, read> mg_grp: array<u32>;
@group(0) @binding(7) var<uniform> mg_P: MP;
var<workgroup> mg_pp: array<u32, ${BP}>;
var<workgroup> mg_nt: u32;
var<workgroup> mg_xs: array<vec4<f32>, ${KS * BP / 4}>;
@compute @workgroup_size(${RW})
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x;${tileHead("mg", BP)}
  let r = wg.x * ${RW}u + t; let rr = min(r, mg_P.dOut - 1u);
  let nb = mg_P.dIn / 32u; let rb = (e * mg_P.dOut + rr) * nb;
  ${rng(BPQ).map((q) => `var ga${q} = vec4<f32>(0.0); var ua${q} = vec4<f32>(0.0);`).join(" ")}
  for (var b0 = 0u; b0 < nb; b0 += ${KB}u) {
    workgroupBarrier();
    for (var li = t; li < ${xUnits}u; li += ${RW}u) {   // (pair quad, k): 4 pairs' value k
      let pq = li % ${BPQ}u; let k = li / ${BPQ}u;
      let kk = b0 * 32u + k;
      var v = vec4<f32>(0.0);
      if (kk < mg_P.dIn) {
        ${rng(4).map((c) => `v[${c}] = mg_x[(mg_pp[pq * 4u + ${c}u] / mg_P.K) * mg_P.xs + kk / 4u][kk % 4u];`).join(" ")}
      }
      mg_xs[li] = v;
    }
    workgroupBarrier();
    ${rng(KB).map((bb) => `let bk${bb} = min(b0 + ${bb}u, nb - 1u);
    ${deq(kind, "mg_gq", "mg_gs", `rb + bk${bb}`, `g${bb}_`)}
    ${deq(kind, "mg_uq", "mg_us", `rb + bk${bb}`, `u${bb}_`)}`).join("\n    ")}
    ${fmaBlock(["g", "u"], KB).replace(/SX/g, "mg_xs")}
  }
  if (r < mg_P.dOut) {
    ${rng(BP).map((p) => `if (${p}u < np) { let a = ga${p >> 2}[${p & 3}]; mg_h[mg_pp[${p}u] * mg_P.hs + r] = silu(a) * ua${p >> 2}[${p & 3}]; }`).join("\n    ")}
  }
}`;
}
export function moeDnWWGSL(kind, KB = 1) {
  const RW = MOE_RW, BP = MOE_BP, KS = 32 * KB, xUnits = BP * KS / 4;
  return DQ + `
struct MP { dOut: u32, dIn: u32, K: u32, PO: u32, p0: u32, p1: u32, p2: u32, p3: u32 };
@group(0) @binding(0) var<storage, read> md_q: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> md_s: array<u32>;
@group(0) @binding(2) var<storage, read> md_x: array<vec4<f32>>;
@group(0) @binding(3) var<storage, read_write> md_y: array<f32>;
@group(0) @binding(4) var<storage, read> md_grp: array<u32>;
@group(0) @binding(5) var<uniform> md_P: MP;
var<workgroup> md_pp: array<u32, ${BP}>;
var<workgroup> md_nt: u32;
var<workgroup> md_xs: array<vec4<f32>, ${KS * BP / 4}>;
@compute @workgroup_size(${RW})
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x;${tileHead("md", BP)}
  let r = wg.x * ${RW}u + t; let rr = min(r, md_P.dOut - 1u);
  let nb = md_P.dIn / 32u; let rb = (e * md_P.dOut + rr) * nb; let x4 = md_P.dIn / 4u;
  ${rng(BPQ).map((q) => `var da${q} = vec4<f32>(0.0);`).join(" ")}
  for (var b0 = 0u; b0 < nb; b0 += ${KB}u) {
    workgroupBarrier();
    for (var li = t; li < ${xUnits}u; li += ${RW}u) {
      let pq = li % ${BPQ}u; let k = li / ${BPQ}u;
      let kk = b0 * 32u + k;
      var v = vec4<f32>(0.0);
      if (kk < md_P.dIn) {
        ${rng(4).map((c) => `v[${c}] = md_x[md_pp[pq * 4u + ${c}u] * x4 + kk / 4u][kk % 4u];`).join(" ")}
      }
      md_xs[li] = v;
    }
    workgroupBarrier();
    ${rng(KB).map((bb) => `let bk${bb} = min(b0 + ${bb}u, nb - 1u);
    ${deq(kind, "md_q", "md_s", `rb + bk${bb}`, `d${bb}_`)}`).join("\n    ")}
    ${fmaBlock(["d"], KB).replace(/SX/g, "md_xs")}
  }
  if (r < md_P.dOut) {
    ${rng(BP).map((p) => `if (${p}u < np) { md_y[md_pp[${p}u] * md_P.dOut + r] = da${p >> 2}[${p & 3}]; }`).join("\n    ")}
  }
}`;
}

// moe[col][r] = sum_k wts[col * K + k] * y[(col * K + k) * d + r] + sigmoid(logits[col * ls + nExp]) * sh[col][r];
// grid (ceil(d / 256), nCols).
export const MOE_COMB_W_WGSL = DQ + `
struct CP { d: u32, K: u32, ls: u32, nExp: u32 };
@group(0) @binding(0) var<storage, read> wc_y: array<f32>;
@group(0) @binding(1) var<storage, read> wc_wts: array<f32>;
@group(0) @binding(2) var<storage, read> wc_sh: array<f32>;
@group(0) @binding(3) var<storage, read> wc_lg: array<f32>;
@group(0) @binding(4) var<storage, read_write> wc_out: array<f32>;
@group(0) @binding(5) var<uniform> wc_P: CP;
@compute @workgroup_size(256)
fn main(@builtin(global_invocation_id) g: vec3<u32>) {
  let r = g.x; let col = g.y;
  if (r >= wc_P.d) { return; }
  var acc = 0.0;
  for (var k = 0u; k < wc_P.K; k++) { let p = col * wc_P.K + k; acc += wc_wts[p] * wc_y[p * wc_P.d + r]; }
  acc += sigm(wc_lg[col * wc_P.ls + wc_P.nExp]) * wc_sh[col * wc_P.d + r];
  wc_out[col * wc_P.d + r] = acc;
}`;

// ---- attn_w: attention for wide passes, one workgroup per (KV head, column group), the KV head's G query heads of
// CW columns (R = G * CW rows) sharing every K / V tile. TPR threads per row, each owning hd / TPR of the head dims
// (vec4 i * TPR + ds: conflict-free reads of the staged tile); q (pre-scaled) and the output accumulator stay in
// registers. Per tile of TK cells: K -> workgroup memory, the partial dot products -> sp, V -> the same memory, then
// every thread of a row sums its row's partials in the same order and runs the same online-softmax step (so the
// copies agree), o = o * alpha + sum p v. Dense (gather = 0): cells 0 .. pos + col of each column (causal mask
// inside the group); gather = stride: one column per group (CW = 1), the column's row list from qsa_select
// (sel[col * stride] = n, then n rows), as attnDecMcWGSL's gathered kernel. Out: [col][nH * hd], gated by
// sigmoid(gate) from the [q | gate, k, v] buffer, as the exact kernel. Workgroup memory: TK * hd * 4 + TK * NT * 4
// (NT threads: 256 dense, 64 gathered, where the 12 rows x 4 threads leave the rest idle).
// kv: the cache format (engine/wgsl/qwen4exp.js kvDecl): "f16" reads K and V as packed pairs (4 values per vec2<u32>)
// with the decode kernels' exact read (kv_k4) and stages them as f32, so the arithmetic is the f32 kernel's on the
// cache's values.
// Bindings: 0 qn, 1 kc, 2 vc, 3 qkv, 4 out, 5 AP, 6 F (dense), 7 MS, 8 sel (gather).
export const ATTN_W = Object.freeze({ dense: { CW: 5, TPR: 4, NT: 256 }, gather: { CW: 1, TPR: 4, NT: 64 }, TK: 8 });
export function attnWWGSL({ gather = 0, G = 12, hd = 256, TK = ATTN_W.TK, kv = "f32", ...o } = {}) {
  kvCheck(kv);
  const KD = kvDecl(kv, false).k;   // f32: array<vec4<f32>>; f16: array<vec2<u32>> (K and V both read 4 values at a time)
  const { CW, TPR, NT } = { ...(gather ? ATTN_W.gather : ATTN_W.dense), ...o }, R = G * CW, NV = hd / 4 / TPR, H4 = hd / 4;
  if (R * TPR > NT || gather && CW !== 1 || hd % (4 * TPR)) throw new Error(`attn_w: ${R} rows x ${TPR} threads, hd ${hd}`);
  const V = rng(NV), J = rng(TK);
  return DQ + (kv === "f16" ? kvDecl(kv, false).f16 + KV_F16_READ : "") + `
struct AP { nH: u32, nKV: u32, hd: u32, nRot: u32, eps: f32, p0: u32, p1: u32, p2: u32 };
struct MS { qs: u32, kOff: u32, vOff: u32, p0: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> aw_qn: array<vec4<f32>>;
@group(0) @binding(1) var<storage, read> aw_kc: ${KD};
@group(0) @binding(2) var<storage, read> aw_vc: ${KD};
@group(0) @binding(3) var<storage, read> aw_qkv: array<vec4<f32>>;
@group(0) @binding(4) var<storage, read_write> aw_out: array<vec4<f32>>;
@group(0) @binding(5) var<uniform> aw_P: AP;
@group(0) @binding(6) var<uniform> aw_F: McF;
@group(0) @binding(7) var<uniform> aw_S: MS;${gather ? `
@group(0) @binding(8) var<storage, read> aw_sel: array<u32>;` : ""}
var<workgroup> aw_kv: array<vec4<f32>, ${TK * H4}>;
var<workgroup> aw_sp: array<f32, ${TK * NT}>;${gather ? `
var<workgroup> aw_n: u32;` : ""}
@compute @workgroup_size(${NT})
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let tid = l.x; let kvh = wg.x; let nH = aw_P.nH; let nKV = aw_P.nKV;
  let r = min(tid / ${TPR}u, ${R - 1}u); let ds = tid % ${TPR}u;
  let c0 = wg.y * ${CW}u; let ci = r / ${G}u; let h = kvh * ${G}u + r % ${G}u;
  let nc = aw_F.nCols;
  let col = min(c0 + ci, nc - 1u);
  let live = tid < ${R * TPR}u && c0 + ci < nc;
  ${gather ? `let sb = c0 * ${gather}u;
  if (tid == 0u) { aw_n = aw_sel[sb]; }
  let nCell = workgroupUniformLoad(&aw_n);` : `let nCell = aw_F.pos + min(c0 + ${CW}u, nc);   // the group's last column sees cells 0 .. nCell - 1
  let lim = aw_F.pos + col;`}
  let scale = 1.0 / sqrt(f32(${hd}));
  let qb = (col * nH + h) * ${H4}u;
  ${V.map((i) => `let q${i} = aw_qn[qb + ${i * TPR}u + ds] * scale; var o${i} = vec4<f32>(0.0);`).join("\n  ")}
  var m = -3.0e38; var lsum = 0.0;
  for (var t0 = 0u; t0 < nCell; t0 += ${TK}u) {
    workgroupBarrier();
    for (var li = tid; li < ${TK * H4}u; li += ${NT}u) {
      let j = li / ${H4}u; let v = li % ${H4}u;
      let c = ${gather ? `aw_sel[sb + 1u + min(t0 + j, nCell - 1u)]` : `min(t0 + j, nCell - 1u)`};
      aw_kv[li] = ${kvK4(kv, "aw_kc", `(c * nKV + kvh) * ${H4}u + v`)};
    }
    workgroupBarrier();
    ${J.map((j) => `{ var p = 0.0; ${V.map((i) => `p += dot(q${i}, aw_kv[${j * H4 + i * TPR}u + ds]);`).join(" ")} aw_sp[${j * NT}u + tid] = p; }`).join("\n    ")}
    workgroupBarrier();
    for (var li = tid; li < ${TK * H4}u; li += ${NT}u) {
      let j = li / ${H4}u; let v = li % ${H4}u;
      let c = ${gather ? `aw_sel[sb + 1u + min(t0 + j, nCell - 1u)]` : `min(t0 + j, nCell - 1u)`};
      aw_kv[li] = ${kvK4(kv, "aw_vc", `(c * nKV + kvh) * ${H4}u + v`)};
    }
    workgroupBarrier();
    let rb = r * ${TPR}u;
    ${J.map((j) => `var s${j} = 0.0; for (var k = 0u; k < ${TPR}u; k++) { s${j} += aw_sp[${j * NT}u + rb + k]; }
    let ok${j} = t0 + ${j}u < nCell${gather ? "" : ` && t0 + ${j}u <= lim`};`).join("\n    ")}
    var mt = m;
    ${J.map((j) => `if (ok${j}) { mt = max(mt, s${j}); }`).join(" ")}
    let alpha = exp(m - mt);
    ${J.map((j) => `let p${j} = select(0.0, exp(s${j} - mt), ok${j});`).join(" ")}
    lsum = lsum * alpha${J.map((j) => ` + p${j}`).join("")};
    ${V.map((i) => `o${i} = o${i} * alpha${J.map((j) => ` + p${j} * aw_kv[${j * H4 + i * TPR}u + ds]`).join("")};`).join("\n    ")}
    m = mt;
  }
  if (live) {
    let ob = (col * nH + h) * ${H4}u;
    let gb = col * (aw_S.qs / 4u) + h * ${2 * H4}u + ${H4}u;
    let inv = 1.0 / lsum;
    ${V.map((i) => `{ let g = aw_qkv[gb + ${i * TPR}u + ds]; aw_out[ob + ${i * TPR}u + ds] = (o${i} * inv) * vec4<f32>(sigm(g.x), sigm(g.y), sigm(g.z), sigm(g.w)); }`).join("\n    ")}
  }
}`;
}

// ---- dp4a: integer GEMM on activations quantized per 32 values (llama.cpp's MMQ numerics: q8_1-style activations
// against the Q4_0 / Q8_0 weights, exact i32 dot products per block, then f32 * the two scales) ----
// quant_x: x column c (x[(xOff4 + c * xs4) * 4 ..], dIn values) -> xq[(c * nb + b) * 8 ..] (32 packed i8 per block),
// xd[c * nb + b] = amax / 127, for the nCols * mult columns (mult: the MoE's K pairs per column); grid
// (ceil(nb / 32), columns), 8 threads per block. Bindings: 0 x, 1 xq, 2 xd, 3 P { dIn, xOff4, xs4, mult }, 4 F.
export const QUANT_X_WGSL = `
struct QP { dIn: u32, xOff4: u32, xs4: u32, mult: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> qx_x: array<vec4<f32>>;
@group(0) @binding(1) var<storage, read_write> qx_q: array<u32>;
@group(0) @binding(2) var<storage, read_write> qx_d: array<f32>;
@group(0) @binding(3) var<uniform> qx_P: QP;
@group(0) @binding(4) var<uniform> qx_F: McF;
var<workgroup> qx_m: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let nb = qx_P.dIn / 32u; let t = lid.x; let blk = wg.x * 32u + t / 8u; let col = wg.y;
  let ok = blk < nb && col < qx_F.nCols * qx_P.mult;
  var v = vec4<f32>(0.0);
  if (ok) { v = qx_x[qx_P.xOff4 + col * qx_P.xs4 + blk * 8u + t % 8u]; }
  let a = abs(v);
  qx_m[t] = max(max(a.x, a.y), max(a.z, a.w));
  workgroupBarrier();
  let g0 = t & ~7u;
  var m = qx_m[g0];
  for (var i = 1u; i < 8u; i++) { m = max(m, qx_m[g0 + i]); }
  if (ok) {
    let d = m / 127.0; let id = select(0.0, 1.0 / d, d > 0.0);
    qx_q[(col * nb + blk) * 8u + t % 8u] = pack4xI8(vec4<i32>(round(v * id)));
    if (t % 8u == 0u) { qx_d[col * nb + blk] = d; }
  }
}`;

// gemm_d_<fmt>: y[col * ys + yOff + r] = epi(sum over blocks of f32(W_q . x_q) * dW * dX), the gemm_t epilogues.
// Tile BM x BN, TM x TN per thread (rows ty + TY * i, columns tx + TX * j), KB blocks per stage, weights staged as
// packed i8 (Q4_0 nibbles re-centred to q - 8) with their scales. Bindings: 0 qs, 1 sc, 2 xq, 3 xd, 4 y, 5 P (gemm_t's
// GP), 6 F, 7 aux. Needs dIn / 32 to be a multiple of KB.
export const DP4A_T = Object.freeze({ BM: 64, BN: 64, TM: 4, TN: 4, KB: 2 });
export function gemmDWGSL(kind, c = DP4A_T) {
  const { BM, BN, TM, TN, KB } = c, TY = BM / TM, TX = BN / TN, T = TY * TX;
  if (T !== 256 || KB * (BM + BN) * 36 > 16384) throw new Error(`gemm_d: tile ${JSON.stringify(c)}`);
  const wUnits = BM * KB, units = wUnits + BN * KB, MI = rng(TM), NJ = rng(TN);
  const wq = kind === "q4"
    ? `let q = wd_q[g]; wd_W[(bb * 2u) * ${BM}u + r] = ((q & vec4<u32>(0x0F0F0F0Fu)) + vec4<u32>(0x78787878u)) ^ vec4<u32>(0x80808080u);
        wd_W[(bb * 2u + 1u) * ${BM}u + r] = (((q >> vec4<u32>(4u)) & vec4<u32>(0x0F0F0F0Fu)) + vec4<u32>(0x78787878u)) ^ vec4<u32>(0x80808080u);`
    : `wd_W[(bb * 2u) * ${BM}u + r] = wd_q[g * 2u]; wd_W[(bb * 2u + 1u) * ${BM}u + r] = wd_q[g * 2u + 1u];`;
  const stage = rng(Math.ceil(units / T)).map((j) => `
    { let li = t + ${j * T}u;
      if (li < ${wUnits}u) {
        let r = li % ${BM}u; let bb = li / ${BM}u;
        let g = min(row0 + r, dOut - 1u) * nb + ks + bb;
        ${wq}
        wd_Wd[bb * ${BM}u + r] = unpack2x16float(wd_sc[g >> 1u])[g & 1u];
      } else if (li < ${units}u) {
        let l2 = li - ${wUnits}u; let cc = l2 % ${BN}u; let bb = l2 / ${BN}u;
        let xb = min(col0 + cc, nc - 1u) * nb + ks + bb;
        wd_X[(bb * 2u) * ${BN}u + cc] = wd_xq[xb * 2u]; wd_X[(bb * 2u + 1u) * ${BN}u + cc] = wd_xq[xb * 2u + 1u];
        wd_Xd[bb * ${BN}u + cc] = wd_xd[xb];
      } }`).join("");
  const dot = (a, b) => `dot4I8Packed(${a}.x, ${b}.x) + dot4I8Packed(${a}.y, ${b}.y) + dot4I8Packed(${a}.z, ${b}.z) + dot4I8Packed(${a}.w, ${b}.w)`;
  const block = (bb) => `
    {
      ${[0, 1].map((h) => `{
        ${MI.map((i) => `let w${i} = wd_W[${(bb * 2 + h) * BM}u + ty + ${i * TY}u];`).join(" ")}
        ${NJ.map((j) => `let x${j} = wd_X[${(bb * 2 + h) * BN}u + tx + ${j * TX}u];`).join(" ")}
        ${MI.map((i) => NJ.map((j) => `i${i}_${j} ${h ? "+" : ""}= ${dot(`w${i}`, `x${j}`)};`).join(" ")).join("\n        ")}
      }`).join("\n      ")}
      ${MI.map((i) => `let dw${i} = wd_Wd[${bb * BM}u + ty + ${i * TY}u];`).join(" ")}
      ${NJ.map((j) => `let dx${j} = wd_Xd[${bb * BN}u + tx + ${j * TX}u];`).join(" ")}
      ${MI.map((i) => NJ.map((j) => `a${i}_${j} += f32(i${i}_${j}) * (dw${i} * dx${j});`).join(" ")).join("\n      ")}
    }`;
  const store = MI.map((i) => `
  { let row = row0 + ty + ${i * TY}u; if (row < dOut) {
    ${NJ.map((j) => `{ let col = col0 + tx + ${j * TX}u; if (col < nc) { put(a${i}_${j}, col, row, wg.z, nc); } }`).join(" ")}
  } }`).join("");
  return DQ + `
struct GP { dOut: u32, dIn: u32, xOff4: u32, xs4: u32, yOff: u32, ys: u32, mode: u32, scale: f32, kPer: u32, split: u32, p0: u32, p1: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> wd_q: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> wd_sc: array<u32>;
@group(0) @binding(2) var<storage, read> wd_xq: array<vec4<u32>>;
@group(0) @binding(3) var<storage, read> wd_xd: array<f32>;
@group(0) @binding(4) var<storage, read_write> wd_y: array<f32>;
@group(0) @binding(5) var<uniform> wd_P: GP;
@group(0) @binding(6) var<uniform> wd_F: McF;
@group(0) @binding(7) var<storage, read> wd_aux: array<f32>;
var<workgroup> wd_W: array<vec4<u32>, ${2 * KB * BM}>;
var<workgroup> wd_X: array<vec4<u32>, ${2 * KB * BN}>;
var<workgroup> wd_Wd: array<f32, ${KB * BM}>;
var<workgroup> wd_Xd: array<f32, ${KB * BN}>;
fn epi(v: f32, o: u32) -> f32 {
  if (wd_P.mode == 1u) { return silu(v * wd_P.scale); }
  if (wd_P.mode == 2u) { return silu(wd_aux[o]) * v; }
  return v;
}
fn put(v: f32, col: u32, row: u32, z: u32, nc: u32) {
  if (wd_P.split == 1u) { wd_y[(z * nc + col) * wd_P.dOut + row] = v; return; }
  let o = col * wd_P.ys + wd_P.yOff + row; wd_y[o] = epi(v, o);
}
@compute @workgroup_size(${T})
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let tx = t % ${TX}u; let ty = t / ${TX}u;
  let dOut = wd_P.dOut; let nb = wd_P.dIn / 32u; let nc = wd_F.nCols;
  let row0 = wg.x * ${BM}u; let col0 = wg.y * ${BN}u;
  ${MI.map((i) => NJ.map((j) => `var a${i}_${j} = 0.0; var i${i}_${j} = 0i;`).join(" ")).join("\n  ")}
  let kb0 = wg.z * wd_P.kPer; let kb1 = min(nb, kb0 + wd_P.kPer);   // split-K, as gemm_t
  for (var ks = kb0; ks < kb1; ks += ${KB}u) {
    workgroupBarrier();
    ${stage}
    workgroupBarrier();
    ${rng(KB).map(block).join("")}
  }
  ${store}
}`;
}

// The split-K sum: y[col * ys + yOff + r] = epi(sum over z < nSplit (ascending) of part[(z * nc + col) * dOut + r]),
// the GEMMs' epilogues. Grid (ceil(dOut / 256), nCols). Bindings: 0 part, 1 y, 2 P (GP, kPer = nSplit), 3 F, 4 aux.
export const SPLIT_SUM_WGSL = DQ + `
struct GP { dOut: u32, dIn: u32, xOff4: u32, xs4: u32, yOff: u32, ys: u32, mode: u32, scale: f32, kPer: u32, split: u32, p0: u32, p1: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> ss_part: array<f32>;
@group(0) @binding(1) var<storage, read_write> ss_y: array<f32>;
@group(0) @binding(2) var<uniform> ss_P: GP;
@group(0) @binding(3) var<uniform> ss_F: McF;
@group(0) @binding(4) var<storage, read> ss_aux: array<f32>;
@compute @workgroup_size(256)
fn main(@builtin(global_invocation_id) g: vec3<u32>) {
  let r = g.x; let col = g.y; let nc = ss_F.nCols;
  if (r >= ss_P.dOut || col >= nc) { return; }
  var v = 0.0;
  for (var z = 0u; z < ss_P.kPer; z++) { v += ss_part[(z * nc + col) * ss_P.dOut + r]; }
  let o = col * ss_P.ys + ss_P.yOff + r;
  if (ss_P.mode == 1u) { v = silu(v * ss_P.scale); } else if (ss_P.mode == 2u) { v = silu(ss_aux[o]) * v; }
  ss_y[o] = v;
}`;

const dp = (a, b) => `dot4I8Packed(${a}.x, ${b}.x) + dot4I8Packed(${a}.y, ${b}.y) + dot4I8Packed(${a}.z, ${b}.z) + dot4I8Packed(${a}.w, ${b}.w)`;

// ---- the grouped MoE on dp4a: moe_gu_wc / moe_dn_wc. A workgroup of 256 threads takes 64 rows of the tile's
// expert and its BP = 16 pairs: thread t owns row t % 64 and pairs 4 (t / 64) .. + 3. Per stage of KB blocks the 64
// rows' weight blocks are copied into workgroup memory as they are stored (consecutive threads read consecutive 16 B
// of a row: coalesced; a row per thread, as moe_gu_w does, reads 1.3 KB apart and measured no faster than f32 here)
// with their scales, and the pairs' quantized activations (quant_x: the layer input per column for gate/up, h per
// pair for down) with theirs; Q4_0 nibbles are re-centred at use. Per block an exact i32 dot, then f32 * the two
// scales, blocks ascending.
// gu bindings: 0 gq, 1 gs, 2 uq, 3 us, 4 xq, 5 xd, 6 h, 7 grp, 8 P { dOut, dIn, K, PO, hs, 0, 0, 0 }.
// dn bindings: 0 q, 1 s, 2 hq, 3 hd, 4 y, 5 grp, 6 P { dOut, dIn, K, PO, 0, .. }.
export const MOE_RC = 64;   // rows per workgroup (the coalesced kernels)
function moeCoalesced(kind, gu, KB) {
  const RC = MOE_RC, BP = MOE_BP, P = gu ? "wu" : "mn", WV = kind === "q4" ? 1 : 2;   // vec4 per block
  const mats = gu ? ["g", "u"] : ["w"];
  const qs = gu ? { g: `${P}_gq`, u: `${P}_uq` } : { w: `${P}_q` }, scs = gu ? { g: `${P}_gs`, u: `${P}_us` } : { w: `${P}_s` };
  const wUnits = RC * KB * WV;   // per matrix
  const rec = (v) => `((${v} & vec4<u32>(0x0F0F0F0Fu)) + vec4<u32>(0x78787878u)) ^ vec4<u32>(0x80808080u)`;
  const rech = (v) => `(((${v} >> vec4<u32>(4u)) & vec4<u32>(0x0F0F0F0Fu)) + vec4<u32>(0x78787878u)) ^ vec4<u32>(0x80808080u)`;
  const PPG = BP / 4, Q = rng(PPG);   // pairs per thread (4 groups of 64 rows)
  return DQ + `
struct MP { dOut: u32, dIn: u32, K: u32, PO: u32, hs: u32, p0: u32, p1: u32, p2: u32 };
${gu ? `@group(0) @binding(0) var<storage, read> wu_gq: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> wu_gs: array<u32>;
@group(0) @binding(2) var<storage, read> wu_uq: array<vec4<u32>>;
@group(0) @binding(3) var<storage, read> wu_us: array<u32>;
@group(0) @binding(4) var<storage, read> wu_xq: array<vec4<u32>>;
@group(0) @binding(5) var<storage, read> wu_xd: array<f32>;
@group(0) @binding(6) var<storage, read_write> wu_out: array<f32>;
@group(0) @binding(7) var<storage, read> wu_grp: array<u32>;
@group(0) @binding(8) var<uniform> wu_P: MP;` : `@group(0) @binding(0) var<storage, read> mn_q: array<vec4<u32>>;
@group(0) @binding(1) var<storage, read> mn_s: array<u32>;
@group(0) @binding(2) var<storage, read> mn_xq: array<vec4<u32>>;
@group(0) @binding(3) var<storage, read> mn_xd: array<f32>;
@group(0) @binding(4) var<storage, read_write> mn_out: array<f32>;
@group(0) @binding(5) var<storage, read> mn_grp: array<u32>;
@group(0) @binding(6) var<uniform> mn_P: MP;`}
var<workgroup> ${P}_pp: array<u32, ${BP}>;
var<workgroup> ${P}_nt: u32;
${mats.map((m) => `var<workgroup> ${P}_W${m}: array<vec4<u32>, ${wUnits}>;\nvar<workgroup> ${P}_D${m}: array<f32, ${RC * KB}>;`).join("\n")}
var<workgroup> ${P}_xs: array<vec4<u32>, ${2 * KB * BP}>;
var<workgroup> ${P}_xds: array<f32, ${KB * BP}>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x;${tileHead(P, BP)}
  let row0 = wg.x * ${RC}u; let dOut = ${P}_P.dOut;
  let nb = ${P}_P.dIn / 32u; let eb = e * dOut;
  let rr = t % ${RC}u; let pg = t / ${RC}u;
  ${Q.map((q) => mats.map((m) => `var ${m}${q} = 0.0;`).join(" ")).join(" ")}
  for (var b0 = 0u; b0 < nb; b0 += ${KB}u) {
    workgroupBarrier();
    // weights: unit li -> (row, block, vec4) with the vec4 fastest, then the block: a row's KB blocks are contiguous
    for (var li = t; li < ${wUnits}u; li += 256u) {
      let v = li % ${WV}u; let bb = (li / ${WV}u) % ${KB}u; let r = li / ${WV * KB}u;
      let g = (eb + min(row0 + r, dOut - 1u)) * nb + min(b0 + bb, nb - 1u);
      ${mats.map((m) => `${P}_W${m}[(bb * ${WV}u + v) * ${RC}u + r] = ${qs[m]}[g * ${WV}u + v];
      if (v == 0u) { ${P}_D${m}[bb * ${RC}u + r] = unpack2x16float(${scs[m]}[g >> 1u])[g & 1u]; }`).join("\n      ")}
    }
    for (var li = t; li < ${BP * KB}u; li += 256u) {
      let j = li % ${BP}u; let bb = li / ${BP}u;
      let xb = ${gu ? `(${P}_pp[j] / ${P}_P.K)` : `${P}_pp[j]`} * nb + min(b0 + bb, nb - 1u);
      ${P}_xs[(bb * 2u) * ${BP}u + j] = ${P}_xq[xb * 2u]; ${P}_xs[(bb * 2u + 1u) * ${BP}u + j] = ${P}_xq[xb * 2u + 1u];
      ${P}_xds[bb * ${BP}u + j] = ${P}_xd[xb];
    }
    workgroupBarrier();
    for (var bb = 0u; bb < ${KB}u; bb++) {
      if (b0 + bb < nb) {
        ${mats.map((m) => kind === "q4"
    ? `let ${m}r = ${P}_W${m}[bb * ${RC}u + rr]; let ${m}w0 = ${rec(`${m}r`)}; let ${m}w1 = ${rech(`${m}r`)};`
    : `let ${m}w0 = ${P}_W${m}[(bb * 2u) * ${RC}u + rr]; let ${m}w1 = ${P}_W${m}[(bb * 2u + 1u) * ${RC}u + rr];`).join("\n        ")}
        ${mats.map((m) => `let ${m}s = ${P}_D${m}[bb * ${RC}u + rr];`).join(" ")}
        ${Q.map((q) => `{ let j = pg * ${PPG}u + ${q}u; let x0 = ${P}_xs[(bb * 2u) * ${BP}u + j]; let x1 = ${P}_xs[(bb * 2u + 1u) * ${BP}u + j]; let xd = ${P}_xds[bb * ${BP}u + j];
          ${mats.map((m) => `${m}${q} += f32(${dp(`${m}w0`, "x0")} + ${dp(`${m}w1`, "x1")}) * (${m}s * xd);`).join(" ")} }`).join("\n        ")}
      }
    }
  }
  let r = row0 + rr;
  if (r < dOut) {
    ${Q.map((q) => `if (pg * ${PPG}u + ${q}u < np) { ${gu ? `${P}_out[${P}_pp[pg * ${PPG}u + ${q}u] * ${P}_P.hs + r] = silu(g${q}) * u${q};` : `${P}_out[${P}_pp[pg * ${PPG}u + ${q}u] * dOut + r] = w${q};`} }`).join("\n    ")}
  }
}`;
}
export const moeGuCWGSL = (kind, KB = 4) => moeCoalesced(kind, true, KB);
export const moeDnCWGSL = (kind, KB = 4) => moeCoalesced(kind, false, KB);

// ---- DeltaNet for wide passes: dn_delta_w is the exact kernel's recurrence (DN_DELTA_MC_WGSL, the same
// arithmetic per column) without its gated output norm, which needs a 128-wide reduction (8 barriers) per column
// inside the sequential loop, and with each column's inputs loaded one column ahead; dn_gnorm_w applies the norm
// afterwards to every (V head, column) in parallel, with the exact kernel's expressions (so the pair gives the exact
// kernel's bits).
// dn_delta_w bindings: 0 co, 1 S, 2 ba, 3 dta, 4 o, 5 P (the exact kernel's EP), 6 F. Grid (nVHeads).
export const DN_DELTA_W_WGSL = DQ + `
struct EP { nKH: u32, dS: u32, keyDim: u32, nVH: u32, eps: f32, cs: u32, bs: u32, zs: u32, zOff: u32, os: u32, p0: u32, p1: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> wx_co: array<f32>;
@group(0) @binding(1) var<storage, read_write> wx_S: array<f32>;
@group(0) @binding(2) var<storage, read> wx_ba: array<f32>;
@group(0) @binding(3) var<storage, read> wx_dta: array<f32>;
@group(0) @binding(4) var<storage, read_write> wx_o: array<f32>;
@group(0) @binding(5) var<uniform> wx_P: EP;
@group(0) @binding(6) var<uniform> wx_F: McF;
var<workgroup> wx_kq: array<f32, 256>;
var<workgroup> wx_red: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let h = wg.x; let t = l.x; let dS = wx_P.dS;
  let j = t & 127u; let i0 = (t >> 7u) * 64u;
  let kh = h % wx_P.nKH;
  let base = h * dS * dS;
  // the thread's 64 entries of S as named registers (an array indexed in a loop may live in local memory)
  ${rng(64).map((i) => `var s${i} = wx_S[base + (i0 + ${i}u) * dS + j];`).join(" ")}
  // the column's inputs (this thread's k or q value, v, beta and alpha) are loaded one column ahead, so their global
  // latency overlaps the previous column's arithmetic (the values, and so the bits, are the same)
  let kqo = select(kh * dS + t - 128u, wx_P.keyDim + kh * dS + t, t < 128u);   // k (t < 128) or q
  let vo = 2u * wx_P.keyDim + h * dS + j;
  var nkq = wx_co[kqo]; var nv = wx_co[vo]; var nal = wx_ba[wx_P.nVH + h]; var nbe = wx_ba[h];
  for (var col = 0u; col < wx_F.nCols; col++) {
    let ckq = nkq; let v = nv; let cal = nal; let cbe = nbe;
    if (col + 1u < wx_F.nCols) {
      let cb = (col + 1u) * wx_P.cs; let bb = (col + 1u) * wx_P.bs;
      nkq = wx_co[cb + kqo]; nv = wx_co[cb + vo]; nal = wx_ba[bb + wx_P.nVH + h]; nbe = wx_ba[bb + h];
    }
    workgroupBarrier();
    wx_kq[t] = ckq;
    workgroupBarrier();
    let araw = cal + wx_dta[h];
    var sp = araw;
    if (araw <= 20.0) { sp = log(1.0 + exp(araw)); }
    let decay = exp(sp * wx_dta[wx_P.nVH + h]);
    let b = sigm(cbe);
    var vh = 0.0;
    ${rng(64).map((i) => `s${i} = s${i} * decay; vh += s${i} * wx_kq[i0 + ${i}u];`).join("\n    ")}
    wx_red[t] = vh;
    workgroupBarrier();
    vh = (v - (wx_red[j] + wx_red[128u + j])) * b;
    workgroupBarrier();
    var acc = 0.0;
    ${rng(64).map((i) => `s${i} = s${i} + wx_kq[i0 + ${i}u] * vh; acc += s${i} * wx_kq[128u + i0 + ${i}u];`).join("\n    ")}
    wx_red[t] = acc;
    workgroupBarrier();
    if (t < 128u) { wx_o[col * wx_P.os + h * dS + j] = (wx_red[j] + wx_red[128u + j]) * (1.0 / sqrt(f32(dS))); }
  }
  ${rng(64).map((i) => `wx_S[base + (i0 + ${i}u) * dS + j] = s${i};`).join(" ")}
}`;
// dn_gnorm_w bindings: 0 o, 1 z, 2 nw, 3 y, 4 P (EP). Grid (nVHeads, nCols).
export const DN_GNORM_W_WGSL = DQ + `
struct EP { nKH: u32, dS: u32, keyDim: u32, nVH: u32, eps: f32, cs: u32, bs: u32, zs: u32, zOff: u32, os: u32, p0: u32, p1: u32 };
@group(0) @binding(0) var<storage, read> wx_o: array<f32>;
@group(0) @binding(1) var<storage, read> wx_z: array<f32>;
@group(0) @binding(2) var<storage, read> wx_nw: array<f32>;
@group(0) @binding(3) var<storage, read_write> wx_y: array<f32>;
@group(0) @binding(4) var<uniform> wx_P: EP;
var<workgroup> wx_red: array<f32, 256>;
@compute @workgroup_size(256)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let h = wg.x; let col = wg.y; let t = l.x; let dS = wx_P.dS; let j = t & 127u;
  let oi = h * dS + j;
  let ov = wx_o[col * wx_P.os + oi];
  if (t < 128u) { wx_red[t] = ov * ov; } else { wx_red[t] = 0.0; }
  workgroupBarrier();
  for (var s = 128u; s > 0u; s >>= 1u) { if (t < s) { wx_red[t] += wx_red[t + s]; } workgroupBarrier(); }
  let inv = 1.0 / sqrt(wx_red[0] / f32(dS) + wx_P.eps);
  if (t < 128u) { wx_y[col * wx_P.os + oi] = (ov * inv) * wx_nw[j] * sigm(wx_z[col * wx_P.zs + wx_P.zOff + oi]); }
}`;

// ---- DeltaNet's causal conv + l2 for wide passes. The conv reads only inputs (the pass's columns and the K - 1
// before them, kept in the state), so the columns need not run in order: dn_conv_w takes one workgroup per
// (128-channel head, column) with the exact kernel's expressions (DN_CONV_MC_WGSL: the same bits), and
// dn_conv_st_w then moves the state to the pass's last K - 1 inputs. K <= 9 (the exact kernel's bound).
// dn_conv_w bindings: 0 xin, 1 st, 2 w, 3 co, 4 P (the exact kernel's DP). Grid (convDim / dS, nCols).
// dn_conv_st_w bindings: 0 xin, 1 st, 4 P (p0 = convDim / dS), 5 F. Grid (ceil(convDim / 256)).
const DN_CONV_HEAD = `
struct DP { K: u32, nQK: u32, dS: u32, eps: f32, xs: u32, cs: u32, p0: u32, p1: u32 };
${MCF}
@group(0) @binding(0) var<storage, read> wx_xin: array<f32>;
@group(0) @binding(4) var<uniform> wx_P: DP;
@group(0) @binding(5) var<uniform> wx_F: McF;`;
export const DN_CONV_W_WGSL = DQ + DN_CONV_HEAD + `
@group(0) @binding(1) var<storage, read> wx_st: array<f32>;
@group(0) @binding(2) var<storage, read> wx_w: array<f32>;
@group(0) @binding(3) var<storage, read_write> wx_co: array<f32>;
var<workgroup> wx_red: array<f32, 128>;
@compute @workgroup_size(128)
fn main(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) l: vec3<u32>) {
  let t = l.x; let c = wg.x * wx_P.dS + t; let col = wg.y;
  let K1 = wx_P.K - 1u;
  let xv = wx_xin[col * wx_P.xs + c];
  var acc = wx_w[c * wx_P.K + K1] * xv;
  for (var j = 0u; j < K1; j++) {
    // h[j] of the exact kernel at this column: input col - K1 + j, or the state before the pass
    var hj = 0.0;
    if (col + j >= K1) { hj = wx_xin[(col + j - K1) * wx_P.xs + c]; } else { hj = wx_st[c * K1 + col + j]; }
    acc += wx_w[c * wx_P.K + j] * hj;
  }
  let v = silu(acc);
  if (wg.x >= wx_P.nQK) { wx_co[col * wx_P.cs + c] = v; return; }
  wx_red[t] = v * v;
  workgroupBarrier();
  for (var s = 64u; s > 0u; s >>= 1u) { if (t < s) { wx_red[t] += wx_red[t + s]; } workgroupBarrier(); }
  wx_co[col * wx_P.cs + c] = v * (1.0 / max(sqrt(wx_red[0]), wx_P.eps));
}`;
export const DN_CONV_ST_W_WGSL = DN_CONV_HEAD + `
@group(0) @binding(1) var<storage, read_write> wx_st: array<f32>;
@compute @workgroup_size(256)
fn main(@builtin(global_invocation_id) g: vec3<u32>) {
  let c = g.x; let n = wx_F.nCols; let K1 = wx_P.K - 1u;
  if (c >= wx_P.dS * wx_P.p0) { return; }
  var h: array<f32, 8>;
  for (var j = 0u; j < K1; j++) {
    if (n + j >= K1) { h[j] = wx_xin[(n + j - K1) * wx_P.xs + c]; } else { h[j] = wx_st[c * K1 + n + j]; }
  }
  for (var j = 0u; j < K1; j++) { wx_st[c * K1 + j] = h[j]; }
}`;
