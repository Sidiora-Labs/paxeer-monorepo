// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/moe_group.js (MIT; see LICENSE-ENGINE).
// Expert-grouped MoE FFN for wide prefill ubatches (engine option moeGroupPrefill; engine/qwen35.js
// _prefillGrouped). Candidate (D) of docs/research/prefill-profile-2026-09.md.
//
// The per-(column, slot) fused kernels (engine/wgsl/moe.js moe_gus / moe_dnc) stream an expert's rows once
// for every token that picked it. A 16-token pass picks ~102 distinct experts for 128 assignments, so
// there is little to share inside one pass. The grouped prefill therefore runs layer-major over a ubatch
// of U tokens (U a multiple of the pass width): each layer's attention / DeltaNet part runs as the usual
// 16-column passes (same kernels, same bits), the router output of all U tokens is kept, and then:
//
//   moe_gsort                one workgroup: counting sort of the U * (K + 1) (column, slot) pairs by expert
//                            (the shared expert is expert nExp), stable by pair index, cut into chunks of up
//                            to UC pairs; writes the chunk list and the indirect launch sizes.
//   moe_gusg_{f}_{s}         one workgroup per (row block, chunk): gate/up + SiLU for every pair of the chunk
//   moe_dng_{f}_{s}          one workgroup per (row block, chunk): down projection per pair -> y[pair]
//   moe_combw                x[col] += sum_k w_k y[col, k] (k = 0 .. K - 1 in order) + w_K y[col, K]
//
// Exactness (the whole point): every pair keeps the per-pair kernels' arithmetic. The grouped kernels have the
// same workgroup sizes, the same thread -> (block, quarter) map, build every term from the same template
// (moe.js termW), add the blocks into the pair's own accumulator in the same order, reduce with the same tree
// and apply the same epilogue expressions (moe_combw is moe_dnc's combine, in the same order). Only the weight
// loads are shared between the pairs of a chunk; a load does not change a value. So a token's h, y and x bits
// do not depend on grouping, on the chunk size or on which other tokens are in the ubatch. The layer-major
// order does not change any value either: layer l of token t still sees exactly the inputs it saw before
// (DeltaNet state and KV cache of layer l after the earlier passes of that layer).
//
// Chunk list layout (one u32 buffer "grp"): chunk c at words [4c .. 4c + 3] = (first pair index into the pair
// list, pair count n, expert, 0), then the pair list itself at word CO (= 4 * maxChunks): pair indices
// cs = col * KS + slot, grouped by expert (experts ascending), ascending within an expert.
import { FOPS, termW, scOff, wOff, tree } from "./moe.js";

const ROWS = 4;   // the fused gate/up kernel's rows per workgroup (moe.js ROWS)
const WG_MEM = 16384;

// Sizes for a ubatch of U tokens: pairs, the chunk bound (sum over experts of ceil(count / UC) <=
// pairs / UC + experts), the pair list offset and the buffer size in words.
export function moeGroupSizes({ U, K, nExp, UC }) {
  const pairs = U * (K + 1), maxChunks = Math.ceil(pairs / UC) + nExp + 1, CO = 4 * maxChunks;
  return { pairs, maxChunks, CO, words: CO + pairs };
}

// ---- moe_gsort ----
// One workgroup of 256 threads. Pass 1 counts each expert's pairs with workgroup atomics (an order-free sum), a scan
// in expert order (thread t owns the experts e with e % 256 == t, QM = ceil((nExp + 1) / 256) of them) gives each
// expert its first chunk and first pair-list slot, and pass 2 places the pairs in rounds of CR whole columns (one pair
// per thread): each expert gets a CR-bit mask of the round's columns that picked it (atomicOr), a pair's slot is its
// expert's running count plus the mask bits below its column, and the round's last such column advances the count.
// A column picks an expert at most once (top-K picks distinct experts; the shared expert is one slot), so this is the
// stable order (by pair index), the same output as the counting loops it replaced, in O(pairs / 256) steps per
// thread instead of O(pairs) (those took ~0.5 ms a launch on the GB10, ~3% of MoE prefill). The result is a pure
// function of sel.
function sortKernel(K, UC, QM, CO, O) {
  const KS = K + 1, D = O.div, Q = Array.from({ length: QM }, (_, q) => q), NE = 256 * QM;
  const CR = Math.min(28, Math.floor(256 / KS));   // columns per placement round (lc < 28: the masks stay below 2^28)
  return `
@group(1) @binding(0) var<storage, read> gso_sel: array<u32>;
@group(1) @binding(1) var<storage, read_write> gso_grp: array<u32>;
@group(1) @binding(2) var<storage, read_write> gso_ind: array<u32>;
@group(1) @binding(3) var<uniform> gso_s: MOEG;
var<workgroup> gso_h: array<atomic<u32>, ${NE}>;
var<workgroup> gso_m: array<atomic<u32>, ${NE}>;
var<workgroup> gso_b: array<u32, ${NE}>;
var<workgroup> gso_r: array<u32, ${NE}>;
var<workgroup> gso_sc: array<u32, 256>;
var<workgroup> gso_sp: array<u32, 256>;
var<workgroup> gso_su: array<u32, 256>;
@compute @workgroup_size(256)
fn moe_gsort(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let N = gso_s.n; let nE = gso_s.nExp;
${Q.map((q) => `  atomicStore(&gso_h[${q * 256}u + t], 0u); gso_r[${q * 256}u + t] = 0u;`).join("\n")}
  workgroupBarrier();
  for (var i: u32 = t; i < N; i += 256u) {
    let slot = i % ${KS}u; let e = select(min(gso_sel[i], nE - 1u), nE, slot == ${K}u);
    atomicAdd(&gso_h[e], 1u);
  }
  workgroupBarrier();
  var carryC: u32 = 0u; var carryP: u32 = 0u; var carryU: u32 = 0u;
${Q.map((q) => `  let cnt${q} = atomicLoad(&gso_h[${q * 256}u + t]); var cbase${q}: u32 = 0u;
  {
    let c = cnt${q}; let nch = ${D(`c + ${UC - 1}u`, `${UC}u`)};
    gso_sc[t] = nch; gso_sp[t] = c; gso_su[t] = select(0u, 1u, c > 0u && ${q * 256}u + t < nE);
    workgroupBarrier();
    for (var off: u32 = 1u; off < 256u; off <<= 1u) {   // inclusive scan in expert order (Hillis-Steele)
      var a = gso_sc[t]; var b = gso_sp[t]; var u = gso_su[t];
      if (t >= off) { a += gso_sc[t - off]; b += gso_sp[t - off]; u += gso_su[t - off]; }
      workgroupBarrier();
      gso_sc[t] = a; gso_sp[t] = b; gso_su[t] = u;
      workgroupBarrier();
    }
    cbase${q} = carryC + gso_sc[t] - nch; gso_b[${q * 256}u + t] = carryP + gso_sp[t] - c;
    carryC += gso_sc[255]; carryP += gso_sp[255]; carryU += gso_su[255];
    workgroupBarrier();
  }`).join("\n")}
  let nCol = ${D("N", `${KS}u`)};
  let lc = ${D("t", `${KS}u`)}; let slot = t % ${KS}u;
  for (var c0: u32 = 0u; c0 < nCol; c0 += ${CR}u) {
    let col = c0 + lc; let ok = lc < ${CR}u && col < nCol;
    let i = col * ${KS}u + slot;
    var e: u32 = 0u;
    if (ok) { e = select(min(gso_sel[i], nE - 1u), nE, slot == ${K}u); atomicStore(&gso_m[e], 0u); }
    workgroupBarrier();
    if (ok) { atomicOr(&gso_m[e], 1u << lc); }
    workgroupBarrier();
    var m: u32 = 0u;
    if (ok) { m = atomicLoad(&gso_m[e]); gso_grp[${CO}u + gso_b[e] + gso_r[e] + countOneBits(m & ((1u << lc) - 1u))] = i; }
    workgroupBarrier();
    if (ok && (m >> (lc + 1u)) == 0u) { gso_r[e] += countOneBits(m); }
    workgroupBarrier();
  }
${Q.map((q) => `  {
    let e = ${q * 256}u + t; let c = cnt${q}; let pb = gso_b[e];
    for (var k: u32 = 0u; k * ${UC}u < c; k++) {
      let ci = (cbase${q} + k) * 4u;
      gso_grp[ci] = pb + k * ${UC}u; gso_grp[ci + 1u] = min(${UC}u, c - k * ${UC}u); gso_grp[ci + 2u] = e; gso_grp[ci + 3u] = 0u;
    }
  }`).join("\n")}
  if (t == 0u) {
    gso_ind[0] = gso_s.gx; gso_ind[1] = carryC; gso_ind[2] = 1u;
    gso_ind[3] = gso_s.dx; gso_ind[4] = carryC; gso_ind[5] = 1u;
    gso_ind[6] = carryU; gso_ind[7] = N;
  }
}`;
}

// the chunk header shared by the grouped kernels: n (workgroup-uniform), first pair, expert, and the chunk's
// pair indices cs0 .. cs{UC-1} (absent pairs alias the chunk's last pair and are never written)
function chunkHead(P, UC, CO) {
  return `  let cb = wg.y * 4u;
  if (t == 0u) { ${P}_n = ${P}_grp[cb + 1u]; }
  let n = workgroupUniformLoad(&${P}_n);
  let p0 = ${P}_grp[cb]; let e = ${P}_grp[cb + 2u];
${Array.from({ length: UC }, (_, u) => `  let cs${u} = ${P}_grp[${CO}u + p0${u ? ` + min(${u}u, n - 1u)` : ""}];`).join("\n")}`;
}
const users = (UC, f) => Array.from({ length: UC }, (_, u) => f(u)).join("\n");
const guard = (u, body) => u ? `if (n > ${u}u) { ${body} }` : `{ ${body} }`;

// ---- moe_gusg: gate/up + SiLU for a chunk of pairs of one expert (routed, or the shared expert e == nExp) ----
// Per pair: moe_gus's accumulators g_r / u_r, its block loop, its tree over 2 * ROWS arrays and its epilogue.
// The reduction scratch holds PP pairs at a time (8 KB per pair at WG 256), so the pairs reduce in turn.
export function gusGroupKernel(fmt, sfmt, K, UC, CO, WG = 256, O = FOPS) {
  const KS = K + 1, P = `gg${fmt}${sfmt}`, LANES = WG / 4, D = O.div, NACC = 2 * ROWS;
  const PP = Math.max(1, Math.min(UC, Math.floor((WG_MEM - 16) / (NACC * WG * 4))));
  const rows = (f) => Array.from({ length: ROWS }, (_, r) => f(r)).join("\n");
  const ld = (f, pre, Q, qo, SC, so, er) => rows((r) => { const [w0, w1] = wOff(f, Q, qo, `${er}${r}`);
    return `      let ${pre}w${r} = ${w0};${w1 ? ` let ${pre}v${r} = ${w1};` : ""} let ${pre}s${r} = ${scOff(SC, so, `${er}${r}`)};`; });
  const acc = (f) => users(UC, (u) => `      ${guard(u, rows((r) => `g${u}_${r} += ${termW(f, `gs${r}`, `gw${r}`, `gv${r}`, `${P}_x`, `xc${u}`, "b", O)}; u${u}_${r} += ${termW(f, `us${r}`, `uw${r}`, `uv${r}`, `${P}_x`, `xc${u}`, "b", O)};`).replace(/\n/g, " "))}`);
  let red = "";
  for (let q0 = 0; q0 < UC; q0 += PP) {
    const us = Array.from({ length: Math.min(PP, UC - q0) }, (_, i) => q0 + i);
    const body = `${us.map((u, i) => rows((r) => `  ${P}_red[${(i * NACC + r) * WG}u + t] = g${u}_${r}; ${P}_red[${(i * NACC + ROWS + r) * WG}u + t] = u${u}_${r};`)).join("\n")}
${tree(WG, NACC * us.length, `${P}_red`)}
  if (t < ${ROWS}u) {
    let row = row0 + t;
    if (row < dOut) {
${us.map((u, i) => `      ${guard(u, `let gg = ${P}_red[(${i * NACC}u + t) * ${WG}u]; ${P}_h[cs${u} * S.ys + row] = gg / (1.0 + exp(-gg)) * ${P}_red[(${i * NACC + ROWS}u + t) * ${WG}u];`)}`).join("\n")}
    }
  }
  workgroupBarrier();`;
    red += q0 ? `\n  if (n > ${q0}u) {\n${body}\n  }` : `\n${body}`;
  }
  return `
@group(1) @binding(0) var<storage, read> ${P}_gq: array<u32>;
@group(1) @binding(1) var<storage, read> ${P}_gs: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_uq: array<u32>;
@group(1) @binding(3) var<storage, read> ${P}_us: array<u32>;
@group(1) @binding(4) var<storage, read> ${P}_x: array<vec4<f32>>;
@group(1) @binding(5) var<storage, read_write> ${P}_h: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_grp: array<u32>;
@group(1) @binding(7) var<storage, read> ${P}_sh: array<u32>;
@group(1) @binding(8) var<uniform> ${P}_s: MOEF;
var<workgroup> ${P}_red: array<f32, ${NACC * PP * WG}>;
var<workgroup> ${P}_n: u32;
@compute @workgroup_size(${WG})
fn moe_gusg_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x;
  let qt = t & 3u; let bl = t >> 2u; let nb = ${D("S.dIn", "32u")}; let row0 = wg.x * ${ROWS}u;
${chunkHead(P, UC, CO)}
${users(UC, (u) => `  let xc${u} = (${D(`cs${u}`, `${KS}u`)}) * (${D("S.xs", "4u")});`)}
${users(UC, (u) => rows((r) => `  var g${u}_${r}: f32 = 0.0; var u${u}_${r}: f32 = 0.0;`))}
  var dOut = S.dOut;
  if (e < S.nExp) {
${rows((r) => `    let er${r} = e * S.dOut + min(row0 + ${r}u, S.dOut - 1u);`)}
    for (var b: u32 = bl; b < nb; b += ${LANES}u) {
${ld(fmt, "g", `${P}_gq`, "0u", `${P}_gs`, "0u", "er")}
${ld(fmt, "u", `${P}_uq`, "0u", `${P}_us`, "0u", "er")}
${acc(fmt)}
    }
  } else {
    dOut = S.sDim;
${rows((r) => `    let sr${r} = min(row0 + ${r}u, S.sDim - 1u);`)}
    for (var b: u32 = bl; b < nb; b += ${LANES}u) {
${ld(sfmt, "g", `${P}_sh`, "0u", `${P}_sh`, "S.oGs", "sr")}
${ld(sfmt, "u", `${P}_sh`, "S.oUq", `${P}_sh`, "S.oUs", "sr")}
${acc(sfmt)}
    }
  }${red}
}`;
}

// ---- moe_dng: down projection for a chunk of pairs -> y[cs][row] (moe_dnc's per-slot sums, before combine) ----
export function dnGroupKernel(fmt, sfmt, K, R, UC, CO, WG = 64, O = FOPS) {
  const P = `dg${fmt}${sfmt}`, LANES = WG / 4, D = O.div;
  const rs = Array.from({ length: R }, (_, r) => r), rows = (f) => rs.map(f).join("\n");
  if (UC * R * WG * 4 > WG_MEM - 16) throw new Error(`moe_dng: ${UC} pairs x ${R} rows need more than ${WG_MEM} B of workgroup memory`);
  const ld = (f, Q, SC, er, nb) => rows((r) => { const [w0, w1] = wOff(f, Q, "0u", `${er}${r}`, nb);
    return `      let w${r} = ${w0};${w1 ? ` let v${r} = ${w1};` : ""} let s${r} = ${scOff(SC, "0u", `${er}${r}`, nb)};`; });
  const acc = (f) => users(UC, (u) => `      ${guard(u, rows((r) => `y${u}_${r} += ${termW(f, `s${r}`, `w${r}`, `v${r}`, `${P}_h`, `xc${u}`, "b", O)};`).replace(/\n/g, " "))}`);
  return `
@group(1) @binding(0) var<storage, read> ${P}_q: array<u32>;
@group(1) @binding(1) var<storage, read> ${P}_sc: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_h: array<vec4<f32>>;
@group(1) @binding(3) var<storage, read_write> ${P}_y: array<f32>;
@group(1) @binding(4) var<storage, read> ${P}_grp: array<u32>;
@group(1) @binding(5) var<storage, read> ${P}_sq: array<u32>;
@group(1) @binding(6) var<storage, read> ${P}_ss: array<u32>;
@group(1) @binding(7) var<uniform> ${P}_s: MOEF;
var<workgroup> ${P}_red: array<f32, ${UC * R * WG}>;
var<workgroup> ${P}_n: u32;
@compute @workgroup_size(${WG})
fn moe_dng_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x; let qt = t & 3u; let bl = t >> 2u;
  let nb = ${D("S.dIn", "32u")}; let nbs = ${D("S.sDim", "32u")}; let row0 = wg.x * ${R}u; let hs4 = ${D("S.ys", "4u")};
${chunkHead(P, UC, CO)}
${rows((r) => `  let rr${r} = min(row0 + ${r}u, S.dOut - 1u);`)}
${users(UC, (u) => `  let xc${u} = cs${u} * hs4;`)}
${users(UC, (u) => `  ${rs.map((r) => `var y${u}_${r}: f32 = 0.0;`).join(" ")}`)}
  if (e < S.nExp) {
${rows((r) => `    let er${r} = e * S.dOut + rr${r};`)}
    for (var b: u32 = bl; b < nb; b += ${LANES}u) {
${ld(fmt, `${P}_q`, `${P}_sc`, "er", "nb")}
${acc(fmt)}
    }
  } else {
    for (var b: u32 = bl; b < nbs; b += ${LANES}u) {
${ld(sfmt, `${P}_sq`, `${P}_ss`, "rr", "nbs")}
${acc(sfmt)}
    }
  }
${users(UC, (u) => rows((r) => `  ${P}_red[${(u * R + r) * WG}u + t] = y${u}_${r};`))}
${tree(WG, UC * R, `${P}_red`)}
  if (t < ${R}u) {
    let row = row0 + t;
    if (row < S.dOut) {
${users(UC, (u) => `      ${guard(u, `${P}_y[cs${u} * S.dOut + row] = ${P}_red[(${u * R}u + t) * ${WG}u];`)}`)}
    }
  }
}`;
}

// ---- moe_combw: moe_dnc's combine + residual, per (row, column), over the grouped y ----
export function combKernel(K) {
  const KS = K + 1;
  return `
@group(1) @binding(0) var<storage, read_write> gcb_x: array<f32>;
@group(1) @binding(1) var<storage, read> gcb_y: array<f32>;
@group(1) @binding(2) var<storage, read> gcb_w: array<f32>;
@group(1) @binding(3) var<uniform> gcb_s: MOEF;
@compute @workgroup_size(64)
fn moe_combw(@builtin(global_invocation_id) gid: vec3<u32>) {
  let S = gcb_s; let row = gid.x; let col = gid.y;
  if (row >= S.dOut) { return; }
  let wb = col * ${KS}u;
  var o: f32 = 0.0;
  for (var k: u32 = 0u; k < ${K}u; k++) { o += gcb_w[wb + k] * gcb_y[(wb + k) * S.dOut + row]; }
  o += gcb_w[wb + ${K}u] * gcb_y[(wb + ${K}u) * S.dOut + row];
  gcb_x[col * S.xs + row] += o;
}`;
}

// Rows per moe_dng workgroup. A row's value does not depend on it (each row has its own accumulators and tree
// over the same thread map, exactly as moe_dnc's rows at any moeDnRows), so it is picked for speed: more rows
// per workgroup share each pair's h loads. The reduction scratch (UC * R * 64 floats) must fit in 16 KB.
export const dnGroupRows = (UC) => (UC <= 8 ? 4 : 2);

// The grouped kernels for one engine (appended after moeFusedWGSL, whose struct MOEF they use).
// K: top-k; R: rows per moe_dng workgroup (dnGroupRows(UC)); UC: pairs per chunk;
// U: ubatch width in tokens; nExp; gu / dn: the (routed, shared) format pairs.
export function moeGroupWGSL({ K, R = 1, UC = 8, U, nExp, gu = [], dn = [] }, O = FOPS) {
  if (!(K >= 1 && K <= 16) || ![1, 2, 4].includes(R)) throw new Error(`moeGroupWGSL: K ${K}, R ${R}`);
  if (![1, 2, 4, 8, 16].includes(UC)) throw new Error(`moeGroupWGSL: UC ${UC} is not 1, 2, 4, 8 or 16`);
  const { CO } = moeGroupSizes({ U, K, nExp, UC }), QM = Math.ceil((nExp + 1) / 256);
  return /* wgsl */ `
// ---------------- expert-grouped MoE prefill (engine/wgsl/moe_group.js) ----------------
struct MOEG { n: u32, nExp: u32, gx: u32, dx: u32 };
${sortKernel(K, UC, QM, CO, O)}
${gu.map(([f, s]) => gusGroupKernel(f, s, K, UC, CO, 256, O)).join("\n")}
${dn.map(([f, s]) => dnGroupKernel(f, s, K, R, UC, CO, 64, O)).join("\n")}
${combKernel(K)}
`;
}

// ---- tiled variant (engine option moeGroupTiled; NOT bit-identical to the per-pair kernels) ----
// The exact kernels above keep every pair's 256-lane reduction tree, so per pair they still pay the reduction
// and activation traffic the per-pair kernels pay; measured on GB10 they are slower than the per-pass path at
// every chunk size. These replace them with real per-expert tiles: a 256-thread workgroup owns TR = 32 * RPT
// rows of one expert (thread t: rows row0 + 32 r + (t >> 3), r < RPT) and a chunk of up to UC pairs; each row
// has 8 lanes, lane l takes block kt + l of every 8-block k-tile. The tile's activations of all the chunk's
// pairs are staged in workgroup memory (vec4 j of local block b at [pair][j][b], so the 8 lanes read 8
// consecutive vec4s), so one weight block load feeds UC pairs x RPT rows reuse each activation load. Per
// thread the blocks accumulate in k order; the 8 lanes are then summed in lane order. The summation order
// differs from moe_gus / moe_dnc (a different rounding of the same sum), so this path is validated against
// tolerances and the goldens, not bit-identity. Same bindings, entry points, chunk list, sort and combine as
// the exact kernels; launch x = ceil(rows / TR).
// The workgroup scratch is a scalar array<f32> (vec4 reads assemble 4 scalars): the reduction stores one float per
// invocation, and a per-component store into a workgroup array<vec4<f32>> (xt[i >> 2][i & 3] = ...) by different
// invocations loses writes on Apple GPUs (Metal lowers it to a whole-vec4 read-modify-write). Same layout and
// arithmetic order as the vec4 version, so the results elsewhere are unchanged. Do not reintroduce that pattern.
export const TILE_RPT = { gu: 2, dn: 4 };
export const tileRows = (kind) => 32 * TILE_RPT[kind];
function tiledKernel(kind, fmt, sfmt, K, UC, CO, O = FOPS) {
  const KS = K + 1, WG = 256, gu = kind === "gu", NM = gu ? 2 : 1, RPT = TILE_RPT[kind], TR = 32 * RPT, D = O.div;
  const P = `${gu ? "tg" : "td"}${fmt}${sfmt}`, XT = Math.max(UC * 64, 512), PR = Math.max(1, Math.min(UC, Math.floor(2048 / (WG * NM * RPT))));
  if (TR * PR > WG) throw new Error("tiled moe group: output threads");
  const mats = gu ? ["g", "u"] : ["y"], RS = Array.from({ length: RPT }, (_, r) => r);
  const US = Array.from({ length: UC }, (_, u) => u);
  const load = (f, m, r, Q, qo, SC, so, er) => {
    const bi = `((${er}) * nbk + b)`, W = f === "q4" ? 4 : 8;
    return `        let ${m}s${r} = unpack2x16float(${SC}[${so} + (${bi} >> 1u)])[${bi} & 1u];\n` +
      `        ${Array.from({ length: W }, (_, j) => `let ${m}w${r}_${j} = ${Q}[${qo} + ${bi} * ${W}u + ${j}u];`).join(" ")}`;
  };
  const dotb = (f, m, r) => f === "q4"
    ? `${m}s${r} * (${[0, 1, 2, 3].map((j) => `(dot(${O.q4lo(`${m}w${r}_${j}`)}, xv${j}) + dot(${O.q4hi(`${m}w${r}_${j}`)}, xv${j + 4}))`).join(" + ")})`
    : `${m}s${r} * (${[0, 1, 2, 3, 4, 5, 6, 7].map((j) => `dot(${O.i8x4(`${m}w${r}_${j}`)}, xv${j})`).join(" + ")})`;
  const accum = (f) => US.map((u) => `        ${guard(u, `${[0, 1, 2, 3, 4, 5, 6, 7].map((j) => `let xv${j} = ${O.v4(...[0, 1, 2, 3].map((c) => `${P}_xt[${4 * (u * 64 + j * 8) + c}u + 4u * ln]`))};`).join(" ")}\n${RS.map((r) => mats.map((m) => `          a${m}${r}_${u} += ${dotb(f, m, r)};`).join("\n")).join("\n")}`)}`).join("\n");
  const er = (r) => `e * S.dOut + tr${r}`;
  const routed = RS.map((r) => gu
    ? `${load(fmt, "g", r, `${P}_gq`, "0u", `${P}_gs`, "0u", er(r))}\n${load(fmt, "u", r, `${P}_uq`, "0u", `${P}_us`, "0u", er(r))}`
    : load(fmt, "y", r, `${P}_q`, "0u", `${P}_sc`, "0u", er(r))).join("\n");
  const shared = RS.map((r) => gu
    ? `${load(sfmt, "g", r, `${P}_sh`, "0u", `${P}_sh`, "S.oGs", `tr${r}`)}\n${load(sfmt, "u", r, `${P}_sh`, "S.oUq", `${P}_sh`, "S.oUs", `tr${r}`)}`
    : load(sfmt, "y", r, `${P}_sq`, "0u", `${P}_ss`, "0u", `tr${r}`)).join("\n");
  let red = "";
  for (let q0 = 0; q0 < UC; q0 += PR) {
    const np = Math.min(PR, UC - q0);
    const body = `  workgroupBarrier();
${Array.from({ length: np }, (_, i) => RS.map((r) => mats.map((m, mi) => `  { let ri = ${((i * NM + mi) * RPT + r) * WG}u + t; ${P}_xt[ri] = a${m}${r}_${q0 + i}; }`).join("\n")).join("\n")).join("\n")}
  workgroupBarrier();
  if (t < ${TR * np}u) {
    let lr = t & ${TR - 1}u; let pi = t >> ${Math.log2(TR)}u; let u = ${q0}u + pi; let orow = row0 + lr; let rr = lr >> 5u; let rw = lr & 31u;
    if (u < n && orow < dOut) {
      let cs = ${P}_cs[u];
${mats.map((m, mi) => `      var s${m}: f32 = 0.0;
      for (var l: u32 = 0u; l < 8u; l++) { let ri = ((pi * ${NM}u + ${mi}u) * ${RPT}u + rr) * ${WG}u + rw * 8u + l; s${m} += ${P}_xt[ri]; }`).join("\n")}
      ${gu ? `${P}_h[cs * S.ys + orow] = sg / (1.0 + exp(-sg)) * su;` : `${P}_y[cs * S.dOut + orow] = sy;`}
    }
  }`;
    red += q0 ? `\n  if (n > ${q0}u) {\n${body}\n  }` : `\n${body}`;
  }
  const decl = gu ? `
@group(1) @binding(0) var<storage, read> ${P}_gq: array<u32>;
@group(1) @binding(1) var<storage, read> ${P}_gs: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_uq: array<u32>;
@group(1) @binding(3) var<storage, read> ${P}_us: array<u32>;
@group(1) @binding(4) var<storage, read> ${P}_x: array<vec4<f32>>;
@group(1) @binding(5) var<storage, read_write> ${P}_h: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_grp: array<u32>;
@group(1) @binding(7) var<storage, read> ${P}_sh: array<u32>;
@group(1) @binding(8) var<uniform> ${P}_s: MOEF;` : `
@group(1) @binding(0) var<storage, read> ${P}_q: array<u32>;
@group(1) @binding(1) var<storage, read> ${P}_sc: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_x: array<vec4<f32>>;
@group(1) @binding(3) var<storage, read_write> ${P}_y: array<f32>;
@group(1) @binding(4) var<storage, read> ${P}_grp: array<u32>;
@group(1) @binding(5) var<storage, read> ${P}_sq: array<u32>;
@group(1) @binding(6) var<storage, read> ${P}_ss: array<u32>;
@group(1) @binding(7) var<uniform> ${P}_s: MOEF;`;
  return `${decl}
var<workgroup> ${P}_xt: array<f32, ${XT * 4}>;
var<workgroup> ${P}_xo: array<u32, ${UC}>;
var<workgroup> ${P}_cs: array<u32, ${UC}>;
var<workgroup> ${P}_n: u32;
@compute @workgroup_size(${WG})
fn ${gu ? "moe_gusg" : "moe_dng"}_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x; let ln = t & 7u; let row0 = wg.x * ${TR}u;
  let cb = wg.y * 4u;
  if (t == 0u) { ${P}_n = ${P}_grp[cb + 1u]; }
  let n = workgroupUniformLoad(&${P}_n);
  let p0 = ${P}_grp[cb]; let e = ${P}_grp[cb + 2u];
  if (t < n) { let c = ${P}_grp[${CO}u + p0 + t]; ${P}_cs[t] = c; ${P}_xo[t] = ${gu ? `(${D("c", `${KS}u`)}) * (${D("S.xs", "4u")})` : `c * (${D("S.ys", "4u")})`}; }
  let sh = e >= S.nExp;
  let dOut = ${gu ? "select(S.dOut, S.sDim, sh)" : "S.dOut"};
  let nbk = ${gu ? D("S.dIn", "32u") : `select(${D("S.dIn", "32u")}, ${D("S.sDim", "32u")}, sh)`};
${RS.map((r) => `  let tr${r} = min(row0 + ${32 * r}u + (t >> 3u), dOut - 1u);`).join("\n")}
${US.map((u) => `  ${RS.map((r) => mats.map((m) => `var a${m}${r}_${u}: f32 = 0.0;`).join(" ")).join(" ")}`).join("\n")}
  for (var kt: u32 = 0u; kt < nbk; kt += 8u) {
    workgroupBarrier();
    for (var i: u32 = t; i < ${UC * 64}u; i += ${WG}u) {
      let u = i >> 6u; let v = i & 63u; let kb = kt * 8u + v;
      if (u < n && kb < nbk * 8u) { let xi = 4u * ((i & ${~63 >>> 0}u) + (v & 7u) * 8u + (v >> 3u)); let xx = ${P}_x[${P}_xo[u] + kb]; ${P}_xt[xi] = xx[0]; ${P}_xt[xi + 1u] = xx[1]; ${P}_xt[xi + 2u] = xx[2]; ${P}_xt[xi + 3u] = xx[3]; }
    }
    workgroupBarrier();
    let b = kt + ln;
    if (b < nbk) {
      if (!sh) {
${routed}
${accum(fmt)}
      } else {
${shared}
${accum(sfmt)}
      }
    }
  }${red}
}`;
}
export const tiledGroupWGSL = ({ K, UC, U, nExp, gu = [], dn = [] }, O = FOPS) => {
  if (![1, 2, 4, 8].includes(UC)) throw new Error(`tiled moe group: UC ${UC} is not 1, 2, 4 or 8`);
  const { CO } = moeGroupSizes({ U, K, nExp, UC }), QM = Math.ceil((nExp + 1) / 256);
  return /* wgsl */ `
// ---------------- expert-grouped MoE prefill, tiled (engine/wgsl/moe_group.js) ----------------
struct MOEG { n: u32, nExp: u32, gx: u32, dx: u32 };
${sortKernel(K, UC, QM, CO, O)}
${gu.map(([f, s]) => tiledKernel("gu", f, s, K, UC, CO, O)).join("\n")}
${dn.map(([f, s]) => tiledKernel("dn", f, s, K, UC, CO, O)).join("\n")}
${combKernel(K)}
`;
};

// ---- dp4a variant (engine option moeGroupDp4a; NOT bit-identical: llama.cpp's MMQ numerics) ----
// The tiled kernels above dequantize every weight block to f32 once per pair of the chunk (the unpack sits inside the
// pair loop: hoisting 32 floats per row and matrix does not fit the registers), so they are ALU-bound well before the
// expert weights are read once. Here the activations are quantized first, per 32 values (moe_qx: d = amax / 127,
// q = round(x / d) as packed i8, the dense dp4a GEMM's quant_q8_w), and each thread keeps its weight blocks as packed
// i8 words in registers (Q4_0 nibbles re-centred to q - 8 once per block, as gemm_d_q4 does; Q8_0 words as they are),
// so a pair costs 8 dot4I8Packed per row and matrix plus one f32 multiply-add: f32(isum) * (dW * dX), k ascending per
// lane, then the 8 lanes summed in lane order. Larger chunks fit (the chunk's activations are 9 words per block in
// workgroup memory instead of 32 floats), so each expert's rows are read fewer times per ubatch.
// Thread map as the tiled kernels: 256 threads, lane ln = t & 7 takes block kt + ln of each 8-block k tile, rows
// row0 + 32 r + (t >> 3), r < RPT. Activation tile [pair][half][lane] vec4<u32> (8 lanes read 128 contiguous bytes).
// Inputs: gate/up reads the quantized normed input (moe_qx over the ubatch's columns: xq[(col * nb + blk) * 2 + h]
// vec4s, xd[col * nb + blk]); down reads the quantized h (moe_qx over the (column, slot) pairs, nb = ys / 32 per pair).
export const DP4A_RPT = TILE_RPT;
export const dp4aRows = (kind) => 32 * DP4A_RPT[kind];
function dp4aGroupKernel(kind, fmt, sfmt, K, UC, CO, U, O = FOPS) {
  const KS = K + 1, WG = 256, gu = kind === "gu", NM = gu ? 2 : 1, RPT = DP4A_RPT[kind], TR = 32 * RPT, D = O.div;
  const P = `${gu ? "qg" : "qd"}${fmt}${sfmt}`;
  // pairs reduced per round: their partials (NM * RPT * WG floats each) fit next to the activation tile in 16 KB
  const PR = Math.min(UC, WG / TR, Math.floor((WG_MEM - 64 - UC * (16 * 16 + 8 * 4 + 8)) / (NM * RPT * WG * 4)));
  if (PR < 1) throw new Error(`dp4a moe group: UC ${UC} leaves no workgroup memory for the reduction`);
  const RED = PR * NM * RPT * WG;
  const mats = gu ? ["g", "u"] : ["y"], RS = Array.from({ length: RPT }, (_, r) => r), US = Array.from({ length: UC }, (_, u) => u);
  // the block's weight words of (matrix m, row r) as 8 packed-i8 words w{m}{r}_{0..7} and its scale
  const load = (f, m, r, Q, qo, SC, so, er, vec) => {
    const bi = `((${er}) * nbk + b)`;
    const sc = `        let ${m}s${r} = unpack2x16float(${SC}[${so} + (${bi} >> 1u)])[${bi} & 1u];`;
    if (f === "q4") {
      const q = vec ? `let ${m}q${r} = ${Q}[${bi}];` : `let ${m}q${r} = vec4<u32>(${[0, 1, 2, 3].map((j) => `${Q}[${qo} + ${bi} * 4u + ${j}u]`).join(", ")});`;
      return `${sc}\n        ${q}
        let ${m}l${r} = ((${m}q${r} & vec4<u32>(0x0F0F0F0Fu)) + vec4<u32>(0x78787878u)) ^ vec4<u32>(0x80808080u);
        let ${m}h${r} = (((${m}q${r} >> vec4<u32>(4u)) & vec4<u32>(0x0F0F0F0Fu)) + vec4<u32>(0x78787878u)) ^ vec4<u32>(0x80808080u);`;
    }
    return `${sc}\n        let ${m}l${r} = vec4<u32>(${[0, 1, 2, 3].map((j) => `${Q}[${qo} + ${bi} * 8u + ${j}u]`).join(", ")});
        let ${m}h${r} = vec4<u32>(${[4, 5, 6, 7].map((j) => `${Q}[${qo} + ${bi} * 8u + ${j}u]`).join(", ")});`;
  };
  const d4 = (a, b) => `dot4I8Packed(${a}.x, ${b}.x) + dot4I8Packed(${a}.y, ${b}.y) + dot4I8Packed(${a}.z, ${b}.z) + dot4I8Packed(${a}.w, ${b}.w)`;
  const accum = US.map((u) => `        ${guard(u, `let x0 = ${P}_xt[${u * 16}u + ln]; let x1 = ${P}_xt[${u * 16 + 8}u + ln]; let dx = ${P}_xs[${u * 8}u + ln];
${RS.map((r) => mats.map((m) => `          a${m}${r}_${u} += f32(${d4(`${m}l${r}`, "x0")} + ${d4(`${m}h${r}`, "x1")}) * (${m}s${r} * dx);`).join("\n")).join("\n")}`)}`).join("\n");
  const er = (r) => `e * S.dOut + tr${r}`;
  const routed = RS.map((r) => gu
    ? `${load(fmt, "g", r, `${P}_gq`, "0u", `${P}_gs`, "0u", er(r), true)}\n${load(fmt, "u", r, `${P}_uq`, "0u", `${P}_us`, "0u", er(r), true)}`
    : load(fmt, "y", r, `${P}_q`, "0u", `${P}_sc`, "0u", er(r), true)).join("\n");
  const shared = RS.map((r) => gu
    ? `${load(sfmt, "g", r, `${P}_sh`, "0u", `${P}_sh`, "S.oGs", `tr${r}`, false)}\n${load(sfmt, "u", r, `${P}_sh`, "S.oUq", `${P}_sh`, "S.oUs", `tr${r}`, false)}`
    : load(sfmt, "y", r, `${P}_sq`, "0u", `${P}_ss`, "0u", `tr${r}`, false)).join("\n");
  const qdecl = (n, f) => f === "q4" ? `array<vec4<u32>>` : `array<u32>`;
  let red = "";
  for (let q0 = 0; q0 < UC; q0 += PR) {
    const np = Math.min(PR, UC - q0);
    const body = `  workgroupBarrier();
${Array.from({ length: np }, (_, i) => RS.map((r) => mats.map((m, mi) => `  ${P}_red[${((i * NM + mi) * RPT + r) * WG}u + t] = a${m}${r}_${q0 + i};`).join("\n")).join("\n")).join("\n")}
  workgroupBarrier();
  if (t < ${TR * np}u) {
    let lr = t & ${TR - 1}u; let pi = t >> ${Math.log2(TR)}u; let u = ${q0}u + pi; let orow = row0 + lr; let rr = lr >> 5u; let rw = lr & 31u;
    if (u < n && orow < dOut) {
      let cs = ${P}_cs[u];
${mats.map((m, mi) => `      var s${m}: f32 = 0.0;
      for (var l: u32 = 0u; l < 8u; l++) { s${m} += ${P}_red[((pi * ${NM}u + ${mi}u) * ${RPT}u + rr) * ${WG}u + rw * 8u + l]; }`).join("\n")}
      ${gu ? `${P}_h[cs * S.ys + orow] = sg / (1.0 + exp(-sg)) * su;` : `${P}_y[cs * S.dOut + orow] = sy;`}
    }
  }`;
    red += q0 ? `\n  if (n > ${q0}u) {\n${body}\n  }` : `\n${body}`;
  }
  const decl = gu ? `
@group(1) @binding(0) var<storage, read> ${P}_gq: ${qdecl(0, fmt)};
@group(1) @binding(1) var<storage, read> ${P}_gs: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_uq: ${qdecl(0, fmt)};
@group(1) @binding(3) var<storage, read> ${P}_us: array<u32>;
@group(1) @binding(4) var<storage, read> ${P}_xq: array<vec4<u32>>;
@group(1) @binding(5) var<storage, read_write> ${P}_h: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_grp: array<u32>;
@group(1) @binding(7) var<storage, read> ${P}_sh: array<u32>;
@group(1) @binding(8) var<uniform> ${P}_s: MOEF;` : `
@group(1) @binding(0) var<storage, read> ${P}_q: ${qdecl(0, fmt)};
@group(1) @binding(1) var<storage, read> ${P}_sc: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_xq: array<vec4<u32>>;
@group(1) @binding(3) var<storage, read_write> ${P}_y: array<f32>;
@group(1) @binding(4) var<storage, read> ${P}_grp: array<u32>;
@group(1) @binding(5) var<storage, read> ${P}_sq: array<u32>;
@group(1) @binding(6) var<storage, read> ${P}_ss: array<u32>;
@group(1) @binding(7) var<uniform> ${P}_s: MOEF;`;
  return `${decl}
var<workgroup> ${P}_xt: array<vec4<u32>, ${UC * 16}>;
var<workgroup> ${P}_xs: array<f32, ${UC * 8}>;
var<workgroup> ${P}_red: array<f32, ${RED}>;
var<workgroup> ${P}_xo: array<u32, ${UC}>;
var<workgroup> ${P}_cs: array<u32, ${UC}>;
var<workgroup> ${P}_n: u32;
@compute @workgroup_size(${WG})
fn ${gu ? "moe_gusq" : "moe_dnq"}_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x; let ln = t & 7u; let row0 = wg.x * ${TR}u;
  let cb = wg.y * 4u;
  if (t == 0u) { ${P}_n = ${P}_grp[cb + 1u]; }
  let n = workgroupUniformLoad(&${P}_n);
  let p0 = ${P}_grp[cb]; let e = ${P}_grp[cb + 2u];
  let sh = e >= S.nExp;
  let dOut = ${gu ? "select(S.dOut, S.sDim, sh)" : "S.dOut"};
  let nbk = ${gu ? D("S.dIn", "32u") : `select(${D("S.dIn", "32u")}, ${D("S.sDim", "32u")}, sh)`};
  let nbx = ${gu ? D("S.dIn", "32u") : D("S.ys", "32u")};   // activation blocks per column (gate/up) or per pair (down)
  let dOff = ${gu ? U : U * KS}u * nbx * 8u;   // word offset of the block scales behind the packed blocks (moe_qx)
  if (t < n) { let c = ${P}_grp[${CO}u + p0 + t]; ${P}_cs[t] = c; ${P}_xo[t] = ${gu ? `(${D("c", `${KS}u`)}) * nbx` : "c * nbx"}; }
${RS.map((r) => `  let tr${r} = min(row0 + ${32 * r}u + (t >> 3u), dOut - 1u);`).join("\n")}
${US.map((u) => `  ${RS.map((r) => mats.map((m) => `var a${m}${r}_${u}: f32 = 0.0;`).join(" ")).join(" ")}`).join("\n")}
  for (var kt: u32 = 0u; kt < nbk; kt += 8u) {
    workgroupBarrier();
    for (var i: u32 = t; i < ${UC * 16}u; i += ${WG}u) {
      let u = i >> 4u; let hl = (i >> 3u) & 1u; let bl = i & 7u; let kb = kt + bl;
      if (u < n && kb < nbk) {
        let xb = ${P}_xo[u] + kb;
        ${P}_xt[i] = ${P}_xq[xb * 2u + hl];
        if (hl == 0u) { let di = dOff + xb; ${P}_xs[u * 8u + bl] = bitcast<f32>(${P}_xq[di >> 2u][di & 3u]); }
      }
    }
    workgroupBarrier();
    let b = kt + ln;
    if (b < nbk) {
      if (!sh) {
${routed}
${accum}
      } else {
${shared}
${accum}
      }
    }
  }${red}
}`;
}
// moe_qx: Q8_1-style quantization of n values per column (column stride s0 vec4s) into compact blocks:
// q[(col * nb + blk) * 8 + j] (packed i8) and the scales as f32 bits behind them, q[dOff + col * nb + blk] (one binding:
// the grouped kernels already use 8 storage buffers); grid (ceil(nb / 32), columns). quant_q8_w's code.
export function moeQuantKernel() {
  return `
struct MOEQ { n: u32, s0: u32, dOff: u32, p1: u32 };
@group(1) @binding(0) var<storage, read> mqx_x: array<vec4<f32>>;
@group(1) @binding(1) var<storage, read_write> mqx_q: array<u32>;
@group(1) @binding(2) var<uniform> mqx_s: MOEQ;
var<workgroup> mqx_m: array<f32, 256>;
@compute @workgroup_size(256)
fn moe_qx(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let nb = mqx_s.n / 32u; let t = lid.x; let blk = wg.x * 32u + t / 8u; let col = wg.y;
  let ok = blk < nb;
  var v = vec4<f32>(0.0);
  if (ok) { v = mqx_x[col * mqx_s.s0 + blk * 8u + t % 8u]; }
  let a = abs(v);
  mqx_m[t] = max(max(a.x, a.y), max(a.z, a.w));
  workgroupBarrier();
  let g0 = t & ~7u;
  var m = mqx_m[g0];
  for (var i = 1u; i < 8u; i++) { m = max(m, mqx_m[g0 + i]); }
  if (!ok) { return; }
  let d = m / 127.0; let id = select(0.0, 1.0 / d, d > 0.0);
  mqx_q[(col * nb + blk) * 8u + t % 8u] = pack4xI8(vec4<i32>(round(v * id)));
  if (t % 8u == 0u) { mqx_q[mqx_s.dOff + col * nb + blk] = bitcast<u32>(d); }
}`;
}
// sort: false when the module already has the tiled kernels' moe_gsort / moe_combw (the engine: same UC, same chunk
// list); the expert launches use the same grid as the tiled kernels (DP4A_RPT == TILE_RPT), so one sort serves both.
export const dp4aGroupWGSL = ({ K, UC, U, nExp, gu = [], dn = [], sort = true }, O = FOPS) => {
  if (![1, 2, 4, 8, 16, 32].includes(UC)) throw new Error(`dp4a moe group: UC ${UC} is not 1, 2, 4, 8, 16 or 32`);
  const { CO } = moeGroupSizes({ U, K, nExp, UC }), QM = Math.ceil((nExp + 1) / 256);
  return /* wgsl */ `
// ---------------- expert-grouped MoE prefill, dp4a (engine/wgsl/moe_group.js) ----------------
${sort ? `struct MOEG { n: u32, nExp: u32, gx: u32, dx: u32 };
${sortKernel(K, UC, QM, CO, O)}
${combKernel(K)}` : ""}
${gu.map(([f, s]) => dp4aGroupKernel("gu", f, s, K, UC, CO, U, O)).join("\n")}
${dn.map(([f, s]) => dp4aGroupKernel("dn", f, s, K, UC, CO, U, O)).join("\n")}
${moeQuantKernel()}
`;
};
