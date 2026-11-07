// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/moe.js (MIT; see LICENSE-ENGINE).
// Mixture-of-experts FFN kernels (Qwen3.5 / 3.6 MoE: top-k routed experts + one shared expert).
//
// Experts are stored as the GGUF stacks them, [nExp][dOut][dIn], and uploaded exactly like any
// Q4_0 / Q8_0 matrix with nExp * dOut rows, so expert e's row r is row e * dOut + r: the kernels
// only add an offset, read from the routing result on the GPU (no readback to pick experts).
//
// One launch covers every (column, slot) pair: workgroup y = col * K + slot. Each pair's math is
// the same whether a pass has 1 column (decode) or many (verify / prefill), so the batched path
// is meant to give the same bits as the one-token path, which keeps speculative decoding exact.
// Known open failure: tests/e2e/moe_synth.mjs's "batched prefill == one token" check still differs
// in 202 logits (docs/bench-log.md, 2026-09-26; roadmap/31-moe-split-correctness.md).
//
//   moe_router      logits [col][nExp] -> sel [col][K] (expert ids), selw [col][K] (weights)
//   moe_gu_{q4,q8}  h[col,slot] = silu(Wgate[e] x[col]) * (Wup[e] x[col])
//   moe_dn_{q4,q8}  y[col,slot] = Wdown[e] h[col,slot]
//   moe_combine     x[col] += sum_k selw[col,k] * y[col,k] + sigmoid(sg[col]) * shared[col]

// ---- expert GEMV kernels: configurable layout (MOE_DEFAULT below; MOE_LEGACY reproduces the first coop build) ----
//
// A workgroup of WG threads is cut into WG / TPR row groups of TPR threads. Each group owns R consecutive
// rows (R accumulators per thread per matrix), so a workgroup covers RPW = (WG / TPR) * R rows and a
// (column, slot) pair needs ceil(dOut / RPW) workgroups in x. Inside a group:
//   wide = false: 4 threads share a 32-weight block (one Q4 word / two Q8 words each), TPR / 4 blocks per step
//   wide = true:  one thread owns a whole block, loaded as one vec4<u32> (Q4, 16 B) or two (Q8, 32 B),
//                 TPR blocks per step, with 16 B loads (research D4)
// U unrolls the block loop (U independent block loads in flight per row); tail steps past nb are clamped to
// the last block and dropped with select(), so every load stays in bounds and in one basic block.
// xsh = true stages the (column, slot)'s input vector in workgroup memory once, transposed to
// [slot-of-8][block] with a padded stride (nb + 1), so threads reading consecutive blocks hit different banks.
// The partial sums are reduced by a halving tree over the TPR lanes of each group (log2 TPR barriers).
//
// Every (column, slot) pair runs the same code whatever the pass width, so the batched path (verify /
// prefill) is bit-identical to the one-token path for any config. Different configs sum in different
// orders, so they give different (equally valid) MoE bits; the MoE goldens are llama.cpp text.
export const MOE_LEGACY = Object.freeze({
  gu: Object.freeze({ WG: 256, TPR: 256, R: 4, U: 1, wide: false, xsh: false }),
  dn: Object.freeze({ WG: 64, TPR: 64, R: 4, U: 1, wide: false, xsh: false }),
});
export const MOE_DEFAULT = Object.freeze({
  // Picked by tests/bench/moe_kernel_sweep.js on the GB10 (2026-09-26): in-model (prof_ts) moe_gu_q4 69.8 -> 60.7 µs,
  // moe_dn_q4 43.0 -> 38.4 µs vs legacy. The first guess (gu 128/16/2/2, dn 128/4/2/4) was no faster than legacy.
  // gate/up (512 x 2048 per expert): 32 threads x 16 B per row, 1 row per group, 4 rows per workgroup
  gu: Object.freeze({ WG: 128, TPR: 32, R: 1, U: 1, wide: true, xsh: true }),
  // down (2048 x 512 per expert, 16 blocks per row): 8 threads x 2 blocks per row, 16 rows per workgroup
  dn: Object.freeze({ WG: 128, TPR: 8, R: 1, U: 1, wide: true, xsh: true }),
});
const WG_MEM = 16384;   // WebGPU default maxComputeWorkgroupStorageSize
const pow2 = (n) => Number.isInteger(n) && n > 0 && (n & (n - 1)) === 0;

// opt: undefined (= "legacy") | "default" | "legacy" | { gu?: {...}, dn?: {...} } (partial overrides of MOE_DEFAULT,
// or of MOE_LEGACY when opt.base === "legacy"). dims: { dim, inter } (inter = expert FFN width).
// Returns { gu, dn } with every field resolved plus rows (= RPW) and the baked input width dIn.
export function moeKernelConfig(opt, { dim, inter }) {
  // opt undefined / null: MOE_LEGACY (the tuned layout showed no end-to-end gain, so it is opt-in: "default")
  if (opt == null) opt = { base: "legacy" };
  if (typeof opt === "string") opt = opt === "legacy" ? { base: "legacy" } : opt === "default" ? {} : (() => { throw new Error(`moeKernel: unknown preset ${opt}`); })();
  const base = opt.base === "legacy" ? MOE_LEGACY : MOE_DEFAULT;
  const one = (kind, dIn) => {
    const c = { ...base[kind], ...(opt[kind] || {}) };
    const { WG, TPR, R, U } = c;
    c.wide = !!c.wide; c.xsh = !!c.xsh;
    if (!pow2(WG) || WG > 256 || WG < 4) throw new Error(`moeKernel.${kind}.WG must be a power of two in [4, 256] (got ${WG})`);
    if (!pow2(TPR) || TPR > WG || (!c.wide && TPR < 4)) throw new Error(`moeKernel.${kind}.TPR must be a power of two <= WG${c.wide ? "" : " and >= 4"} (got ${TPR})`);
    if (!Number.isInteger(R) || R < 1 || R > TPR || R > 8) throw new Error(`moeKernel.${kind}.R must be an integer in [1, min(8, TPR)] (got ${R})`);
    if (!Number.isInteger(U) || U < 1 || U > 8) throw new Error(`moeKernel.${kind}.U must be an integer in [1, 8] (got ${U})`);
    if (dIn % 32) throw new Error(`MoE ${kind} input width ${dIn} is not a multiple of 32`);
    const NA = kind === "gu" ? 2 : 1, red = NA * R * WG * 4, xs = 8 * (dIn / 32 + 1) * 16;
    if (red > WG_MEM) throw new Error(`moeKernel.${kind}: ${red} B of reduction scratch exceeds ${WG_MEM} B of workgroup memory`);
    if (c.xsh && red + xs > WG_MEM) c.xsh = false;   // input too wide to stage: read it from the storage buffer
    c.rows = (WG / TPR) * R; c.dIn = dIn;
    return c;
  };
  return { gu: one("gu", dim), dn: one("dn", inter) };
}

// Code emitters. js = true emits a JavaScript generator body with the same control flow and index math
// (tests/unit/moe_kernels_test.js runs it on the CPU); the WGSL-only constructs go through these helpers.
function emit(js) {
  return {
    q4lo: (w) => js ? `q4lo(${w})` : `(vec4<f32>(unpack4xU8(${w} & 0x0F0F0F0Fu)) - vec4<f32>(8.0))`,
    q4hi: (w) => js ? `q4hi(${w})` : `(vec4<f32>(unpack4xU8((${w} >> 4u) & 0x0F0F0F0Fu)) - vec4<f32>(8.0))`,
    i8x4: (w) => js ? `i8x4(${w})` : `vec4<f32>(unpack4xI8(${w}))`,
    div: (a, b) => js ? `Math.floor((${a}) / (${b}))` : `((${a}) / (${b}))`,
  };
}
export function wgslToJs(body) {
  return body.replace(/\bvar (\w+): [^=;]+ = /g, "let $1 = ").replace(/\bvar (\w+) = /g, "let $1 = ")
    .replace(/\b(0x[0-9A-Fa-f]+)u\b/g, "$1").replace(/\b(\d+)u\b/g, "$1").replace(/workgroupBarrier\(\);/g, "yield;");
}

// One expert GEMV kernel. kind "gu": h[cs][row] = silu(Wg[e] x) * (Wu[e] x); kind "dn": y[cs][row] = Wd[e] h[cs].
// Returns { name, decl, body, P }: decl = bindings + workgroup arrays (WGSL only), body = the function body.
export function expertKernel(kind, fmt, c, js = false) {
  const E = emit(js);
  const P = `m${kind === "gu" ? "g" : "d"}${fmt}`, name = `moe_${kind}_${fmt}`;
  const { WG, TPR, R, U, wide, xsh } = c;
  const LANES = wide ? TPR : TPR / 4, RPW = (WG / TPR) * R, NA = kind === "gu" ? 2 : 1;
  const mats = kind === "gu" ? [["g", `${P}_gq`, `${P}_gs`], ["u", `${P}_uq`, `${P}_us`]] : [["y", `${P}_q`, `${P}_sc`]];
  const X = `${P}_x`, XS = `${P}_xs`, RED = `${P}_red`;
  const rs = Array.from({ length: R }, (_, r) => r);
  const xl = (s) => xsh ? `${XS}[(${s}) * nbp + b]` : `${X}[xc + b * 8u + ${s}]`;
  const scale = (SC, r) => `unpack2x16float(${SC}[(er${r} * nb + b) >> 1u])[(er${r} * nb + b) & 1u]`;
  // one row's contribution of block b
  const term = (Q, SC, r, m) => {
    const sc = scale(SC, r);
    if (!wide && fmt === "q4") return `${sc} * (dot(${E.q4lo(`${Q}[(er${r} * nb + b) * 4u + qt]`)}, xa) + dot(${E.q4hi(`${Q}[(er${r} * nb + b) * 4u + qt]`)}, xb))`;
    if (!wide) return `${sc} * (dot(${E.i8x4(`${Q}[(er${r} * nb + b) * 8u + qt * 2u]`)}, xa) + dot(${E.i8x4(`${Q}[(er${r} * nb + b) * 8u + qt * 2u + 1u]`)}, xb))`;
    const w = `w${m}${r}`;
    if (fmt === "q4") {
      const p = (i) => `(dot(${E.q4lo(`${w}[${i}u]`)}, x${i}) + dot(${E.q4hi(`${w}[${i}u]`)}, x${i + 4}))`;
      return `${sc} * ((${p(0)} + ${p(1)}) + (${p(2)} + ${p(3)}))`;
    }
    const d = (v, i, s) => `dot(${E.i8x4(`${v}[${i}u]`)}, x${s})`;
    const h = (v, o) => `((${d(v, 0, o)} + ${d(v, 1, o + 1)}) + (${d(v, 2, o + 2)} + ${d(v, 3, o + 3)}))`;
    return `${sc} * (${h(`${w}a`, 0)} + ${h(`${w}b`, 4)})`;
  };
  const wload = (Q, r, m) => !wide ? "" : fmt === "q4" ? `let w${m}${r} = ${Q}[er${r} * nb + b];`
    : `let w${m}${r}a = ${Q}[(er${r} * nb + b) * 2u]; let w${m}${r}b = ${Q}[(er${r} * nb + b) * 2u + 1u];`;
  const xloads = wide ? Array.from({ length: 8 }, (_, s) => `let x${s} = ${xl(`${s}u`)};`).join(" ")
    : fmt === "q4" ? `let xa = ${xl("qt")}; let xb = ${xl("qt + 4u")};` : `let xa = ${xl("qt * 2u")}; let xb = ${xl("qt * 2u + 1u")};`;
  const step = (u) => {
    const head = U === 1 ? `let b = b0;` : `let bu = b0 + ${u * LANES}u; let ok = bu < nb; let b = min(bu, nb - 1u);`;
    const loads = mats.map(([m, Q]) => rs.map((r) => wload(Q, r, m)).join(" ")).join(" ");
    const adds = mats.map(([m, Q, SC]) => rs.map((r) => U === 1 ? `      ${m}${r} += ${term(Q, SC, r, m)};` : `      ${m}${r} += select(0.0, ${term(Q, SC, r, m)}, ok);`).join("\n")).join("\n");
    return `    {\n      ${head}\n      ${xloads}\n      ${loads}\n${adds}\n    }`;
  };
  const k = (mi, r) => (mi * R + r) * WG;
  const out = kind === "gu"
    ? `let gg = ${RED}[lane * ${WG}u + grp * ${TPR}u]; ${P}_h[cs * S.ys + row] = gg / (1.0 + exp(-gg)) * ${RED}[(${R}u + lane) * ${WG}u + grp * ${TPR}u];`
    : `${P}_y[cs * S.ys + row] = ${RED}[lane * ${WG}u + grp * ${TPR}u];`;
  const body = `
  let S = ${P}_s; let t = lid.x; let cs = wg.y; let lane = t % ${TPR}u; let grp = ${E.div("t", `${TPR}u`)};
  let e = ${P}_sel[cs]; let nb = ${E.div("S.dIn", "32u")}; let row0 = wg.x * ${RPW}u + grp * ${R}u;
  let xc = ${kind === "gu" ? `${E.div("cs", "S.K")} * ${E.div("S.xs", "4u")}` : `cs * ${E.div("S.xs", "4u")}`};   // ${kind === "gu" ? "the column's x" : "each (column, slot) has its own input h"}
${wide ? "" : "  let qt = lane & 3u; let bl = lane >> 2u;\n"}${xsh ? `  let nbp = nb + 1u;
  for (var i: u32 = t; i < nb * 8u; i += ${WG}u) { ${XS}[(i & 7u) * nbp + (i >> 3u)] = ${X}[xc + i]; }
  workgroupBarrier();
` : ""}${mats.map(([m]) => rs.map((r) => `  var ${m}${r}: f32 = 0.0;`).join("\n")).join("\n")}
${rs.map((r) => `  let er${r} = e * S.dOut + min(row0 + ${r}u, S.dOut - 1u);`).join("\n")}
  for (var b0: u32 = ${wide ? "lane" : "bl"}; b0 < nb; b0 += ${LANES * U}u) {
${Array.from({ length: U }, (_, u) => step(u)).join("\n")}
  }
${mats.map(([m], mi) => rs.map((r) => `  ${RED}[${k(mi, r)}u + t] = ${m}${r};`).join("\n")).join("\n")}
  workgroupBarrier();
${TPR === 1 ? "" : `  for (var st: u32 = ${TPR / 2}u; st > 0u; st >>= 1u) {
    if (lane < st) {
${mats.map((_, mi) => rs.map((r) => `      ${RED}[${k(mi, r)}u + t] += ${RED}[${k(mi, r)}u + t + st];`).join("\n")).join("\n")}
    }
    workgroupBarrier();
  }
`}  if (lane < ${R}u) { let row = row0 + lane; if (row < S.dOut) { ${out} } }
`;
  const qT = wide ? "array<vec4<u32>>" : "array<u32>";
  const decl = kind === "gu" ? `
@group(1) @binding(0) var<storage, read> ${P}_gq: ${qT};
@group(1) @binding(1) var<storage, read> ${P}_gs: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_uq: ${qT};
@group(1) @binding(3) var<storage, read> ${P}_us: array<u32>;
@group(1) @binding(4) var<storage, read> ${X}: array<vec4<f32>>;
@group(1) @binding(5) var<storage, read_write> ${P}_h: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_sel: array<u32>;
@group(1) @binding(7) var<uniform> ${P}_s: MOE;` : `
@group(1) @binding(0) var<storage, read> ${P}_q: ${qT};
@group(1) @binding(1) var<storage, read> ${P}_sc: array<u32>;
@group(1) @binding(2) var<storage, read> ${X}: array<vec4<f32>>;
@group(1) @binding(3) var<storage, read_write> ${P}_y: array<f32>;
@group(1) @binding(4) var<storage, read> ${P}_sel: array<u32>;
@group(1) @binding(5) var<uniform> ${P}_s: MOE;`;
  const wgDecl = `
var<workgroup> ${RED}: array<f32, ${NA * R * WG}>;${xsh ? `\nvar<workgroup> ${XS}: array<vec4<f32>, ${8 * (c.dIn / 32 + 1)}>;` : ""}`;
  return { name, P, decl: decl + wgDecl, body: js ? wgslToJs(body) : body, wgslBody: body };
}

// ---------------- fused MoE FFN (moeFuse, the default when every MoE layer qualifies) ----------------
// The fused kernels keep the first coop build's thread layout (4 threads per 32-weight block, ROWS rows per
// workgroup); the moeKernel layouts above only apply to the unfused moe_gu / moe_dn kernels.
const ROWS = 4;
const q4lo = (w) => `vec4<f32>(unpack4xU8(${w} & 0x0F0F0F0Fu)) - vec4<f32>(8.0)`;
const q4hi = (w) => `vec4<f32>(unpack4xU8((${w} >> 4u) & 0x0F0F0F0Fu)) - vec4<f32>(8.0)`;
const i8x4 = (w) => `vec4<f32>(unpack4xI8(${w}))`;
// Op spellings for the fused (and expert-grouped, engine/wgsl/moe_group.js) kernels: WGSL, or JavaScript for the
// CPU tests (tests/unit/moe_group_test.js runs the kernel bodies with one generator per thread). The WGSL
// spellings are the ones the fused kernels always had, so their generated code does not change.
// (div parenthesizes a compound dividend: "c + 7u" / "8u" must not become c + 7u / 8u)
export const FOPS = Object.freeze({ q4lo, q4hi, i8x4, div: (a, b) => `${/[^\w.]/.test(a) ? `(${a})` : a} / ${b}`, v4: (...c) => `vec4<f32>(${c.join(", ")})` });
export const FOPS_JS = Object.freeze({ q4lo: (w) => `q4lo(${w})`, q4hi: (w) => `q4hi(${w})`, i8x4: (w) => `i8x4(${w})`,
  div: (a, b) => `Math.floor((${a}) / (${b}))`, v4: (...c) => `[${c.join(", ")}]` });
export const tree = (WG, n, red) => `
  workgroupBarrier();
  for (var st: u32 = ${WG / 2}u; st > 0u; st >>= 1u) {
    if (t < st) {
${Array.from({ length: n }, (_, r) => `      ${red}[${r * WG}u + t] += ${red}[${r * WG}u + t + st];`).join("\n")}
    }
    workgroupBarrier();
  }`;
// Per layer and column: router GEMV (the shared-expert gate row appended as row nExp, one launch; with moe_nrt it
// also does the post-attention RMSNorm, normRouterKernel below),
// moe_route (softmax, top-K, sigmoid of the shared gate), moe_gus (K routed slots + the shared expert as
// slot K, gate/up + SiLU), moe_dnc (down for all K + 1 slots, combine and residual add in one epilogue).
// Slot K reads the shared expert's own weights, which may be in another format than the routed experts
// (Q8_0 shared, Q4_0 routed in the bartowski Q4_0 file): the branch on the slot index is uniform per
// workgroup. K is baked into the kernels (KS = K + 1 slots). As before, workgroup y = (column, slot) or
// column, so every column does the same math in a 1-column pass as in an N-column pass.
//
// Uniform MOEF: dOut, dIn (routed matrix), sDim (shared expert FFN width: rows for gate/up, dIn for down),
// nExp, xs (input / residual column stride, floats), ys (h stride per (column, slot), floats), norm,
// shOff (unused; the engine writes 1), oUq / oGs / oUs (word offsets of the shared up qs, gate scales, up scales in the
// packed shared gate/up buffer, whose gate qs start at 0), pad.

// The arithmetic of one fused-kernel term from its scale sc and weight words w0 (Q4: the word; Q8: the first
// word) and w1 (Q8: the second word), x from vec4 array X at vec4 offset xc. termOff (the fused per-pair
// kernels, loads inline) and the expert-grouped kernels (loads hoisted, shared by the chunk's pairs) both
// build their terms from this one template, so a pair's expression tree is the same in both.
export function termW(fmt, sc, w0, w1, X, xc, b = "b", O = FOPS) {
  if (fmt === "q4") return `${sc} * (dot(${O.q4lo(w0)}, ${X}[${xc} + ${b} * 8u + qt]) + dot(${O.q4hi(w0)}, ${X}[${xc} + ${b} * 8u + qt + 4u]))`;
  return `${sc} * (dot(${O.i8x4(w0)}, ${X}[${xc} + ${b} * 8u + qt * 2u]) + dot(${O.i8x4(w1)}, ${X}[${xc} + ${b} * 8u + qt * 2u + 1u]))`;
}
// the scale and weight-word loads of termOff's term (block index bi = er * nb + b)
export const scOff = (SC, so, er, nb = "nb", b = "b") => { const bi = `(${er} * ${nb} + ${b})`; return `unpack2x16float(${SC}[${so} + (${bi} >> 1u)])[${bi} & 1u]`; };
export const wOff = (fmt, Q, qo, er, nb = "nb", b = "b") => { const bi = `(${er} * ${nb} + ${b})`;
  return fmt === "q4" ? [`${Q}[${qo} + ${bi} * 4u + qt]`, ""] : [`${Q}[${qo} + ${bi} * 8u + qt * 2u]`, `${Q}[${qo} + ${bi} * 8u + qt * 2u + 1u]`]; };
// termOff: like term(), with word offsets into Q / SC and explicit block count / index names
function termOff(fmt, Q, qo, SC, so, X, er, xc, nb = "nb", b = "b", O = FOPS) {
  return termW(fmt, scOff(SC, so, er, nb, b), ...wOff(fmt, Q, qo, er, nb, b), X, xc, b, O);
}

// Router. With norm (renormalized top-K weights: Qwen3.5 / 3.6), the experts are ranked by their logits and the
// weights are a softmax over the K picked logits alone, exp(l_k - l_0) / sum_j exp(l_j - l_0): the full softmax's
// sum cancels in the renormalization, so this is the same math as llama.cpp's softmax -> top-K -> renormalize without
// the two 256-wide trees (the probabilities order the experts as the logits do; only exp() rounding ties could
// differ). The weights' last bits differ from moe_router's; every pass width runs this kernel, so spec == plain.
// Without norm: the same softmax as moe_router (max and sum trees, same order), then the same top-K selection.
// Top-K by a merge network. Each expert's value becomes a u32 sort key, larger = ahead (the float order: sign-flipped
// bits; -0 keyed as +0, so the two tie as they compare equal; NaN 0, behind every number), and the order is (key desc,
// id asc), exactly the order of moe_router's K argmax rounds (ties to the lower index), so the ids and weights are the
// same bits (the key maps back to the value's bits; only -0 comes back +0, which no weight can tell apart). Phase 0:
// the experts in chunks of CH = K * F (ids >= nExp pad the last chunk with key 0: behind NaN, as their ids are higher),
// each thread ranks one expert in its chunk and writes it to the chunk's sorted top-K list if the rank is below K
// (F 1, 2, 4, 8 all measured the same on the GB10; F 1 is the default). Then ceil(log2(chunks)) rounds merge list
// pairs (2m, 2m + 1) into list m keeping K: an element's slot is its index plus the count of partner-list elements
// ahead of it (binary search), written if below K; the order is total (distinct ids), so the slots are a permutation.
// Lists hold ids (ping-pong halves of rt_ix), one barrier per round. NaN and padding are never written to a slot (any
// slot left over keeps id 0, weight 0). Slot K gets the shared gate sigmoid(logit[nExp]) with moe_combine's
// expression. Unlike the key-free comparator (the class tests per compare cost more than they saved), this takes the
// GB10 sweep (tests/bench/moe_fused_sweep.js) from 12.40 to 10.35 µs. (Until 2026-10: a candidate-threshold rank
// count over the float values; same ids and weights for every finite input.)
const rtKey = (v, o) => `let ${o}b = bitcast<u32>(${v}); let ${o}a = ${o}b & 0x7FFFFFFFu;
      let ${o} = select(select(select(0xFFFFFFFFu - ${o}b, ${o}b + 0x80000000u, ${o}b < 0x80000000u), 0x80000000u, ${o}a == 0u), 0u, ${o}a > 0x7F800000u);`;
export function routeKernel(K, F = 1) {
  const KS = K + 1, CH = K * F, H = 1024 + CH;   // CH: phase-0 chunk; H: ceil(nExp / CH) * CH <= nExp + CH - 1 ids
  const ahead = (ka, a, kb, b) => `(${ka} > ${kb} || (${ka} == ${kb} && ${a} < ${b}))`;
  return `
@group(1) @binding(0) var<storage, read> rt_l: array<f32>;
@group(1) @binding(1) var<storage, read_write> rt_sel: array<u32>;
@group(1) @binding(2) var<storage, read_write> rt_w: array<f32>;
@group(1) @binding(3) var<uniform> rt_s: MOEF;
var<workgroup> rt_k: array<u32, ${H}>;
var<workgroup> rt_ix: array<u32, ${2 * H}>;
var<workgroup> rt_v: array<f32, 256>;
var<workgroup> rt_ki: array<u32, ${K}>;
var<workgroup> rt_kv: array<f32, ${K}>;
@compute @workgroup_size(256)
fn moe_route(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let col = wg.x; let t = lid.x; let n = rt_s.nExp;
  let lb = col * rt_s.xs;
  var nl: u32 = (n + ${CH - 1}u) / ${CH}u;
  if (t < ${K}u) { rt_ki[t] = 0u; rt_kv[t] = 0.0; }
  for (var i: u32 = n + t; i < nl * ${CH}u; i += 256u) { rt_k[i] = 0u; }
  if (rt_s.norm == 1u) {   // renormalized: rank the logits themselves (no softmax over all nExp)
    for (var i: u32 = t; i < n; i += 256u) { ${rtKey("rt_l[lb + i]", "kk")} rt_k[i] = kk; }
    workgroupBarrier();
  } else {
    var m: f32 = -3.0e38;
    for (var i: u32 = t; i < n; i += 256u) { m = max(m, rt_l[lb + i]); }
    rt_v[t] = m;
    workgroupBarrier();
    for (var st: u32 = 128u; st > 0u; st >>= 1u) { if (t < st) { rt_v[t] = max(rt_v[t], rt_v[t + st]); } workgroupBarrier(); }
    let mx = rt_v[0];
    workgroupBarrier();
    var s: f32 = 0.0;
    for (var i: u32 = t; i < n; i += 256u) { let p = exp(rt_l[lb + i] - mx); rt_k[i] = bitcast<u32>(p); s += p; }
    rt_v[t] = s;
    workgroupBarrier();
    for (var st: u32 = 128u; st > 0u; st >>= 1u) { if (t < st) { rt_v[t] += rt_v[t + st]; } workgroupBarrier(); }
    let inv = 1.0 / rt_v[0];
    for (var i: u32 = t; i < n; i += 256u) { ${rtKey("bitcast<f32>(rt_k[i]) * inv", "kk")} rt_k[i] = kk; }
    workgroupBarrier();
  }
  // phase 0: sorted chunk lists
  let reps0 = (nl * ${CH}u + 255u) / 256u;
  for (var q: u32 = 0u; q < reps0; q++) {
    let e = t + q * 256u;
    if (e < nl * ${CH}u) {
      let c0 = e - e % ${CH}u; let ke = rt_k[e];
      var r: u32 = 0u;
      for (var j: u32 = c0; j < c0 + ${CH}u; j++) { let kj = rt_k[j]; r += select(0u, 1u, ${ahead("kj", "j", "ke", "e")}); }
      if (r < ${K}u) { rt_ix[(c0 / ${CH}u) * ${K}u + r] = e; }
    }
  }
  let reps = (nl * ${K}u + 255u) / 256u;
  workgroupBarrier();
  var src: u32 = 0u;
  for (var rd: u32 = 0u; rd < 16u; rd++) {
    if (nl <= 1u) { break; }
    let dst = ${H}u - src;
    for (var q: u32 = 0u; q < reps; q++) {
      let e = t + q * 256u;
      if (e < nl * ${K}u) {
        let l = e / ${K}u; let i = e % ${K}u; let x = rt_ix[src + e]; let kx = rt_k[x]; let pl = l ^ 1u;
        var r: u32 = i;
        if (pl < nl) {
          var lo: u32 = 0u; var hi: u32 = ${K}u;
          for (var bs: u32 = 0u; bs < 5u; bs++) {
            if (lo < hi) { let mid = (lo + hi) / 2u; let y = rt_ix[src + pl * ${K}u + mid]; let ky = rt_k[y]; if (${ahead("ky", "y", "kx", "x")}) { lo = mid + 1u; } else { hi = mid; } }
          }
          r = i + lo;
        }
        if (r < ${K}u) { rt_ix[dst + (l >> 1u) * ${K}u + r] = x; }
      }
    }
    workgroupBarrier();
    src = dst; nl = (nl + 1u) / 2u;
  }
  if (t < ${K}u) {
    let x = rt_ix[src + t]; let kx = rt_k[x];
    if (kx != 0u) { rt_ki[t] = x; rt_kv[t] = bitcast<f32>(select(0xFFFFFFFFu - kx, kx - 0x80000000u, kx >= 0x80000000u)); }
  }
  workgroupBarrier();
  if (t == 0u) {
    // norm: w_k = exp(l_k - l_top) / sum over the K picked (the softmax's full sum cancels in the renormalization)
    if (rt_s.norm == 1u) { let l0 = rt_kv[0]; for (var k: u32 = 0u; k < ${K}u; k++) { rt_kv[k] = exp(rt_kv[k] - l0); } }
    var tot: f32 = 0.0;
    for (var k: u32 = 0u; k < ${K}u; k++) { tot += rt_kv[k]; }
    for (var k: u32 = 0u; k < ${K}u; k++) {
      rt_sel[col * ${KS}u + k] = rt_ki[k];
      rt_w[col * ${KS}u + k] = select(rt_kv[k], rt_kv[k] / tot, rt_s.norm == 1u);
    }
    let g = rt_l[lb + n];
    rt_sel[col * ${KS}u + ${K}u] = 0u;
    rt_w[col * ${KS}u + ${K}u] = 1.0 / (1.0 + exp(-g));
  }
}`;
}

// gate/up for K routed slots + the shared expert (slot K). Grid: x = ceil(max(dOut, sDim) / ROWS),
// y = column * KS + slot. Same per-thread layout and reduction as moe_gu.
// O = FOPS_JS: the same kernel with JavaScript op spellings (CPU tests: tests/unit/moe_group_test.js).
export function gusKernel(fmt, sfmt, K, WG = 256, O = FOPS) {
  const KS = K + 1, P = `gs${fmt}${sfmt}`, LANES = WG / 4, D = O.div;
  const rows = (f) => Array.from({ length: ROWS }, (_, r) => f(r)).join("\n");
  return `
@group(1) @binding(0) var<storage, read> ${P}_gq: array<u32>;
@group(1) @binding(1) var<storage, read> ${P}_gs: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_uq: array<u32>;
@group(1) @binding(3) var<storage, read> ${P}_us: array<u32>;
@group(1) @binding(4) var<storage, read> ${P}_x: array<vec4<f32>>;
@group(1) @binding(5) var<storage, read_write> ${P}_h: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_sel: array<u32>;
@group(1) @binding(7) var<storage, read> ${P}_sh: array<u32>;
@group(1) @binding(8) var<uniform> ${P}_s: MOEF;
var<workgroup> ${P}_red: array<f32, ${2 * ROWS * WG}>;
@compute @workgroup_size(${WG})
fn moe_gus_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x; let cs = wg.y; let col = ${D("cs", `${KS}u`)}; let slot = cs - col * ${KS}u;
  let qt = t & 3u; let bl = t >> 2u; let nb = ${D("S.dIn", "32u")}; let row0 = wg.x * ${ROWS}u; let xc = col * (${D("S.xs", "4u")});
${rows((r) => `  var g${r}: f32 = 0.0; var u${r}: f32 = 0.0;`)}
  var dOut = S.dOut;
  if (slot < ${K}u) {
    let e = ${P}_sel[cs];
${rows((r) => `    let er${r} = e * S.dOut + min(row0 + ${r}u, S.dOut - 1u);`)}
    for (var b: u32 = bl; b < nb; b += ${LANES}u) {
${rows((r) => `      g${r} += ${termOff(fmt, `${P}_gq`, "0u", `${P}_gs`, "0u", `${P}_x`, `er${r}`, "xc", "nb", "b", O)};\n      u${r} += ${termOff(fmt, `${P}_uq`, "0u", `${P}_us`, "0u", `${P}_x`, `er${r}`, "xc", "nb", "b", O)};`)}
    }
  } else {
    dOut = S.sDim;
${rows((r) => `    let sr${r} = min(row0 + ${r}u, S.sDim - 1u);`)}
    for (var b: u32 = bl; b < nb; b += ${LANES}u) {
${rows((r) => `      g${r} += ${termOff(sfmt, `${P}_sh`, "0u", `${P}_sh`, "S.oGs", `${P}_x`, `sr${r}`, "xc", "nb", "b", O)};\n      u${r} += ${termOff(sfmt, `${P}_sh`, "S.oUq", `${P}_sh`, "S.oUs", `${P}_x`, `sr${r}`, "xc", "nb", "b", O)};`)}
    }
  }
${rows((r) => `  ${P}_red[${r * WG}u + t] = g${r}; ${P}_red[${(ROWS + r) * WG}u + t] = u${r};`)}
${tree(WG, 2 * ROWS, `${P}_red`)}
  if (t < ${ROWS}u) {
    let row = row0 + t;
    if (row < dOut) { let gg = ${P}_red[t * ${WG}u]; ${P}_h[cs * S.ys + row] = gg / (1.0 + exp(-gg)) * ${P}_red[(${ROWS}u + t) * ${WG}u]; }
  }
}`;
}

// down for all KS slots of one column + combine + residual: workgroup (row block, column) runs every
// slot's GEMV rows (the same per-thread terms and one tree for all slots), then thread r writes
// x[row] += sum_k w_k y_k (k = 0 .. K - 1 in order) + w_K y_shared, moe_combine's expression and order.
export function dncKernel(fmt, sfmt, K, R, WG = 64, O = FOPS) {
  const KS = K + 1, P = `dc${fmt}${sfmt}`, LANES = WG / 4, D = O.div;
  const ks = Array.from({ length: K }, (_, k) => k), rs = Array.from({ length: R }, (_, r) => r);
  const acc = (k, r) => `y${k}_${r}`;
  return `
@group(1) @binding(0) var<storage, read> ${P}_q: array<u32>;
@group(1) @binding(1) var<storage, read> ${P}_sc: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_h: array<vec4<f32>>;
@group(1) @binding(3) var<storage, read_write> ${P}_x: array<f32>;
@group(1) @binding(4) var<storage, read> ${P}_sel: array<u32>;
@group(1) @binding(5) var<storage, read> ${P}_w: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_sq: array<u32>;
@group(1) @binding(7) var<storage, read> ${P}_ss: array<u32>;
@group(1) @binding(8) var<uniform> ${P}_s: MOEF;
var<workgroup> ${P}_red: array<f32, ${KS * R * WG}>;
@compute @workgroup_size(${WG})
fn moe_dnc_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x; let col = wg.y; let qt = t & 3u; let bl = t >> 2u;
  let nb = ${D("S.dIn", "32u")}; let nbs = ${D("S.sDim", "32u")}; let row0 = wg.x * ${R}u; let hs4 = ${D("S.ys", "4u")};
${rs.map((r) => `  let rr${r} = min(row0 + ${r}u, S.dOut - 1u);`).join("\n")}
${ks.map((k) => `  let e${k} = ${P}_sel[col * ${KS}u + ${k}u]; let xc${k} = (col * ${KS}u + ${k}u) * hs4;
  ${rs.map((r) => `var ${acc(k, r)}: f32 = 0.0; let er${k}_${r} = e${k} * S.dOut + rr${r};`).join(" ")}`).join("\n")}
  ${rs.map((r) => `var ${acc(K, r)}: f32 = 0.0;`).join(" ")}
  let xcs = (col * ${KS}u + ${K}u) * hs4;
  for (var b: u32 = bl; b < nb; b += ${LANES}u) {
${ks.map((k) => rs.map((r) => `    ${acc(k, r)} += ${termOff(fmt, `${P}_q`, "0u", `${P}_sc`, "0u", `${P}_h`, `er${k}_${r}`, `xc${k}`, "nb", "b", O)};`).join("\n")).join("\n")}
  }
  for (var b: u32 = bl; b < nbs; b += ${LANES}u) {
${rs.map((r) => `    ${acc(K, r)} += ${termOff(sfmt, `${P}_sq`, "0u", `${P}_ss`, "0u", `${P}_h`, `rr${r}`, "xcs", "nbs", "b", O)};`).join("\n")}
  }
${Array.from({ length: KS }, (_, k) => rs.map((r) => `  ${P}_red[${(k * R + r) * WG}u + t] = ${acc(k, r)};`).join("\n")).join("\n")}
${tree(WG, KS * R, `${P}_red`)}
  if (t < ${R}u) {
    let row = row0 + t;
    if (row < S.dOut) {
      let wb = col * ${KS}u;
      var o: f32 = 0.0;
      for (var k: u32 = 0u; k < ${K}u; k++) { o += ${P}_w[wb + k] * ${P}_red[(k * ${R}u + t) * ${WG}u]; }
      o += ${P}_w[wb + ${K}u] * ${P}_red[(${K * R}u + t) * ${WG}u];
      ${P}_x[col * S.xs + row] += o;
    }
  }
}`;
}

// ---- fused kernels, wide layout (engine option moeFusedLayout: the default on Apple GPUs) ----
// The kernels above give each thread a quarter of one 32-weight block per row, then reduce 2 x ROWS (moe_gus) or
// (K + 1) x R (moe_dnc) arrays over all 256 / 64 threads. At the 35B-A3B shape (dIn 2048 / 512) that is one block
// per thread and an 8- / 6-level workgroup-memory tree: on an M5 Max (Chrome / Metal) they are barrier bound
// (~280 / ~230 GB/s against the ~380-440 GB/s of the dense GEMVs; docs/bench-log.md, 2026-09-27).
// The wide layout applies the unfused kernels' "wide" option to the fused ones: a workgroup of WG threads is cut
// into groups of TPR threads, each group owns R consecutive rows, each thread owns whole blocks (one 16 B
// vec4<u32> load per Q4 block, two per Q8 block, x as 8 vec4s) and adds nb / TPR of them per row, and each group
// reduces its own rows with a log2(TPR)-level tree. Each (column, slot) pair still runs the same code whatever
// the pass width, so the batched path (verify / prefill) stays bit-identical to the one-token path. The sums are
// in another order than the legacy layout's, so the MoE bits differ from it (equally valid; the MoE goldens are
// llama.cpp text). The shared expert's packed gate/up buffer and its down weights are read as vec4<u32> too
// (the engine rounds the packed buffer up to 16 B; the up qs offset oUq must be a multiple of 4 words).
// Picked by tests/bench/moe_fused_sweep.js on the M5 Max (Deno / Metal, interleaved, 2026-09-28): per launch vs the
// legacy kernels, moe_gus ~0.55x (TPR 16 / 32 at R 1 best; R 4 and TPR 8 slower), moe_dnc ~0.65x (TPR 8 / 16 best;
// TPR 1 / 2 and R 2 much slower). On the GB10 (Vulkan) the wide moe_dnc is slower than the legacy one.
export const MOEF_WIDE = Object.freeze({
  // gate/up (512 x 2048 per expert, 64 blocks per row): 16 threads x 4 blocks per row, 8 rows per workgroup
  gu: Object.freeze({ WG: 128, TPR: 16, R: 1 }),
  // down (2048 x 512 per expert, 16 blocks per row): 8 threads x 2 blocks per row, 8 rows per workgroup
  dn: Object.freeze({ WG: 64, TPR: 8, R: 1 }),
});
// opt: undefined / "legacy" -> null (the legacy fused kernels above) | "wide" (MOEF_WIDE) | { gu?: {...}, dn?: {...} }
// (partial overrides of MOEF_WIDE). Returns null or { gu, dn } with WG, TPR, R and rows (= (WG / TPR) * R) resolved.
export function moeFusedLayout(opt, K = 8) {
  if (opt == null || opt === "legacy") return null;
  if (typeof opt === "string" && opt !== "wide") throw new Error(`moeFusedLayout: unknown preset ${opt}`);
  const o = typeof opt === "string" ? {} : opt;
  const one = (kind) => {
    const c = { ...MOEF_WIDE[kind], ...(o[kind] || {}) }, { WG, TPR, R } = c;
    if (!pow2(WG) || WG > 256 || WG < 4) throw new Error(`moeFusedLayout.${kind}.WG must be a power of two in [4, 256] (got ${WG})`);
    if (!pow2(TPR) || TPR > WG) throw new Error(`moeFusedLayout.${kind}.TPR must be a power of two <= WG (got ${TPR})`);
    if (!Number.isInteger(R) || R < 1 || R > 4 || R > TPR) throw new Error(`moeFusedLayout.${kind}.R must be an integer in [1, min(4, TPR)] (got ${R})`);
    const red = (kind === "gu" ? 2 : K + 1) * R * WG * 4;
    if (red > WG_MEM) throw new Error(`moeFusedLayout.${kind}: ${red} B of reduction scratch exceeds ${WG_MEM} B of workgroup memory`);
    return Object.freeze({ WG, TPR, R, rows: (WG / TPR) * R });
  };
  return Object.freeze({ gu: one("gu"), dn: one("dn") });
}

// One wide block term: scale sc times the dot of weight vec4 w (Q4) or wa / wb (Q8) with x0 .. x7 (the block's 32 inputs).
// Q4_0 word i holds weights 4i .. 4i + 3 (low nibbles) and 16 + 4i .. (high nibbles); Q8_0 word i holds weights 4i .. 4i + 3.
function termWide(fmt, sc, w, O) {
  if (fmt === "q4") {
    const p = (i) => `(dot(${O.q4lo(`${w}[${i}u]`)}, x${i}) + dot(${O.q4hi(`${w}[${i}u]`)}, x${i + 4}))`;
    return `${sc} * ((${p(0)} + ${p(1)}) + (${p(2)} + ${p(3)}))`;
  }
  const d = (v, i, s) => `dot(${O.i8x4(`${v}[${i}u]`)}, x${s})`;
  const h = (v, o) => `((${d(v, 0, o)} + ${d(v, 1, o + 1)}) + (${d(v, 2, o + 2)} + ${d(v, 3, o + 3)}))`;
  return `${sc} * (${h(`${w}a`, 0)} + ${h(`${w}b`, 4)})`;
}
// the weight vec4 load(s) of block bi from vec4 array Q at vec4 offset qo ("" or "o + ")
const wLoadWide = (fmt, w, Q, qo, bi) => fmt === "q4" ? `let ${w} = ${Q}[${qo}${bi}];`
  : `let ${w}a = ${Q}[${qo}${bi} * 2u]; let ${w}b = ${Q}[${qo}${bi} * 2u + 1u];`;
// block bi's f16 scale from u32 array SC, or (vec4 = true) from vec4<u32> array SC at word offset so
const scWide = (SC, bi, vec4 = false, so = "") => vec4 ? `unpack2x16float(${SC}[(${so} + (${bi} >> 1u)) >> 2u][(${so} + (${bi} >> 1u)) & 3u])[${bi} & 1u]`
  : `unpack2x16float(${SC}[${bi} >> 1u])[${bi} & 1u]`;
const xWide = (X, xc) => Array.from({ length: 8 }, (_, s) => `let x${s} = ${X}[${xc} + b * 8u + ${s}u];`).join(" ");
// reduction of n arrays of WG floats over the TPR lanes of each group (log2 TPR barrier levels)
const groupTree = (WG, TPR, n, red) => `
  workgroupBarrier();${TPR === 1 ? "" : `
  for (var st: u32 = ${TPR / 2}u; st > 0u; st >>= 1u) {
    if (lane < st) {
${Array.from({ length: n }, (_, a) => `      ${red}[${a * WG}u + t] += ${red}[${a * WG}u + t + st];`).join("\n")}
    }
    workgroupBarrier();
  }`}`;

// gate/up, wide layout: same bindings, grid y and output as gusKernel; grid x = ceil(max(dOut, sDim) / c.rows).
export function gusKernelWide(fmt, sfmt, K, c, O = FOPS) {
  const KS = K + 1, P = `gs${fmt}${sfmt}`, D = O.div, { WG, TPR, R } = c, RPW = (WG / TPR) * R;
  const rs = Array.from({ length: R }, (_, r) => r), each = (f, sep = "\n") => rs.map(f).join(sep);
  return `
@group(1) @binding(0) var<storage, read> ${P}_gq: array<vec4<u32>>;
@group(1) @binding(1) var<storage, read> ${P}_gs: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_uq: array<vec4<u32>>;
@group(1) @binding(3) var<storage, read> ${P}_us: array<u32>;
@group(1) @binding(4) var<storage, read> ${P}_x: array<vec4<f32>>;
@group(1) @binding(5) var<storage, read_write> ${P}_h: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_sel: array<u32>;
@group(1) @binding(7) var<storage, read> ${P}_sh: array<vec4<u32>>;
@group(1) @binding(8) var<uniform> ${P}_s: MOEF;
var<workgroup> ${P}_red: array<f32, ${2 * R * WG}>;
@compute @workgroup_size(${WG})
fn moe_gus_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x; let cs = wg.y; let col = ${D("cs", `${KS}u`)}; let slot = cs - col * ${KS}u;
  let lane = t % ${TPR}u; let grp = ${D("t", `${TPR}u`)}; let nb = ${D("S.dIn", "32u")}; let row0 = wg.x * ${RPW}u + grp * ${R}u; let xc = col * (${D("S.xs", "4u")});
${each((r) => `  var g${r}: f32 = 0.0; var u${r}: f32 = 0.0;`)}
  var dOut = S.dOut;
  if (slot < ${K}u) {
    let e = ${P}_sel[cs];
${each((r) => `    let er${r} = e * S.dOut + min(row0 + ${r}u, S.dOut - 1u);`)}
    for (var b: u32 = lane; b < nb; b += ${TPR}u) {
      ${xWide(`${P}_x`, "xc")}
${each((r) => `      let bi${r} = er${r} * nb + b; ${wLoadWide(fmt, `qg${r}`, `${P}_gq`, "", `bi${r}`)} ${wLoadWide(fmt, `qu${r}`, `${P}_uq`, "", `bi${r}`)}
      g${r} += ${termWide(fmt, scWide(`${P}_gs`, `bi${r}`), `qg${r}`, O)};
      u${r} += ${termWide(fmt, scWide(`${P}_us`, `bi${r}`), `qu${r}`, O)};`)}
    }
  } else {
    dOut = S.sDim;
    let ou = S.oUq >> 2u;
${each((r) => `    let sr${r} = min(row0 + ${r}u, S.sDim - 1u);`)}
    for (var b: u32 = lane; b < nb; b += ${TPR}u) {
      ${xWide(`${P}_x`, "xc")}
${each((r) => `      let bi${r} = sr${r} * nb + b; ${wLoadWide(sfmt, `qg${r}`, `${P}_sh`, "", `bi${r}`)} ${wLoadWide(sfmt, `qu${r}`, `${P}_sh`, "ou + ", `bi${r}`)}
      g${r} += ${termWide(sfmt, scWide(`${P}_sh`, `bi${r}`, true, "S.oGs"), `qg${r}`, O)};
      u${r} += ${termWide(sfmt, scWide(`${P}_sh`, `bi${r}`, true, "S.oUs"), `qu${r}`, O)};`)}
    }
  }
${each((r) => `  ${P}_red[${r * WG}u + t] = g${r}; ${P}_red[${(R + r) * WG}u + t] = u${r};`)}
${groupTree(WG, TPR, 2 * R, `${P}_red`)}
  if (lane < ${R}u) {
    let row = row0 + lane;
    if (row < dOut) { let gg = ${P}_red[lane * ${WG}u + grp * ${TPR}u]; ${P}_h[cs * S.ys + row] = gg / (1.0 + exp(-gg)) * ${P}_red[(${R}u + lane) * ${WG}u + grp * ${TPR}u]; }
  }
}`;
}

// down + combine + residual, wide layout: same bindings, grid y and epilogue (order of the combine) as dncKernel;
// grid x = ceil(dOut / c.rows).
export function dncKernelWide(fmt, sfmt, K, c, O = FOPS) {
  const KS = K + 1, P = `dc${fmt}${sfmt}`, D = O.div, { WG, TPR, R } = c, RPW = (WG / TPR) * R;
  const ks = Array.from({ length: K }, (_, k) => k), rs = Array.from({ length: R }, (_, r) => r);
  const acc = (k, r) => `y${k}_${r}`;
  return `
@group(1) @binding(0) var<storage, read> ${P}_q: array<vec4<u32>>;
@group(1) @binding(1) var<storage, read> ${P}_sc: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_h: array<vec4<f32>>;
@group(1) @binding(3) var<storage, read_write> ${P}_x: array<f32>;
@group(1) @binding(4) var<storage, read> ${P}_sel: array<u32>;
@group(1) @binding(5) var<storage, read> ${P}_w: array<f32>;
@group(1) @binding(6) var<storage, read> ${P}_sq: array<vec4<u32>>;
@group(1) @binding(7) var<storage, read> ${P}_ss: array<u32>;
@group(1) @binding(8) var<uniform> ${P}_s: MOEF;
var<workgroup> ${P}_red: array<f32, ${KS * R * WG}>;
@compute @workgroup_size(${WG})
fn moe_dnc_${fmt}_${sfmt}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let S = ${P}_s; let t = lid.x; let col = wg.y; let lane = t % ${TPR}u; let grp = ${D("t", `${TPR}u`)};
  let nb = ${D("S.dIn", "32u")}; let nbs = ${D("S.sDim", "32u")}; let row0 = wg.x * ${RPW}u + grp * ${R}u; let hs4 = ${D("S.ys", "4u")};
${rs.map((r) => `  let rr${r} = min(row0 + ${r}u, S.dOut - 1u);`).join("\n")}
${ks.map((k) => `  let e${k} = ${P}_sel[col * ${KS}u + ${k}u]; let xc${k} = (col * ${KS}u + ${k}u) * hs4;
  ${rs.map((r) => `var ${acc(k, r)}: f32 = 0.0; let er${k}_${r} = e${k} * S.dOut + rr${r};`).join(" ")}`).join("\n")}
  ${rs.map((r) => `var ${acc(K, r)}: f32 = 0.0;`).join(" ")}
  let xcs = (col * ${KS}u + ${K}u) * hs4;
  for (var b: u32 = lane; b < nb; b += ${TPR}u) {
${ks.map((k) => `    {
      ${xWide(`${P}_h`, `xc${k}`)}
${rs.map((r) => `      let bi${r} = er${k}_${r} * nb + b; ${wLoadWide(fmt, `q${r}`, `${P}_q`, "", `bi${r}`)}
      ${acc(k, r)} += ${termWide(fmt, scWide(`${P}_sc`, `bi${r}`), `q${r}`, O)};`).join("\n")}
    }`).join("\n")}
  }
  for (var b: u32 = lane; b < nbs; b += ${TPR}u) {
    ${xWide(`${P}_h`, "xcs")}
${rs.map((r) => `    let bi${r} = rr${r} * nbs + b; ${wLoadWide(sfmt, `q${r}`, `${P}_sq`, "", `bi${r}`)}
    ${acc(K, r)} += ${termWide(sfmt, scWide(`${P}_ss`, `bi${r}`), `q${r}`, O)};`).join("\n")}
  }
${Array.from({ length: KS }, (_, k) => rs.map((r) => `  ${P}_red[${(k * R + r) * WG}u + t] = ${acc(k, r)};`).join("\n")).join("\n")}
${groupTree(WG, TPR, KS * R, `${P}_red`)}
  if (lane < ${R}u) {
    let row = row0 + lane;
    if (row < S.dOut) {
      let wb = col * ${KS}u; let rb = grp * ${TPR}u;
      var o: f32 = 0.0;
      for (var k: u32 = 0u; k < ${K}u; k++) { o += ${P}_w[wb + k] * ${P}_red[(k * ${R}u + lane) * ${WG}u + rb]; }
      o += ${P}_w[wb + ${K}u] * ${P}_red[(${K * R}u + lane) * ${WG}u + rb];
      ${P}_x[col * S.xs + row] += o;
    }
  }
}`;
}

// Post-attention RMSNorm + router GEMV in one launch (engine option moeNormRouter, on by default with the fused FFN):
// moe_nrt replaces the rmsnorm launch and the [nExp + 1]-row router GEMV (2 launches per MoE layer -> 1). Each
// workgroup (ROWS router rows, one column: wg.y) streams x, the norm weight w and its rows once: sum(x * x) and
// W_r . (x * w) in the same loop, both reduced in one tree, then logit_r = inv * (W_r . (x * w)) with
// inv = inverseSqrt(sum / dim + eps); workgroup x 0 also writes xn = (x * inv) * w for moe_gus. The router is kept
// as BF16 when the file's router is exactly BF16 (the loader widens it to f32 exactly): half the router bytes, the
// same values. The sums are in this kernel's order (not rmsnorm's and matvec_coop's), in every pass width alike:
// decode, verify and prefill all run moe_nrt, so spec == plain holds; the MoE goldens are llama.cpp text.
// Uniform NRT: dim, dOut (nExp + 1), xs (x column stride), ls (logits column stride), xns (xn column stride).
// name / P: entry point and binding prefix (dn_nba: the same kernel for a DeltaNet layer's input norm and its merged
// beta / alpha GEMV, engine option dnNormBA); struct: false when an earlier instance already declared NRT.
export function normRouterKernel({ ROWS = 4, bf16 = true, WG = 256, name = "moe_nrt", P = "nrt", struct = true } = {}) {
  const rows = (f, sep = "\n") => Array.from({ length: ROWS }, (_, r) => f(r)).join(sep);
  const wv = (r) => bf16 ? `let q${r} = ${P}_W[er${r} + c]; let w${r} = vec4<f32>(bitcast<f32>(q${r}.x << 16u), bitcast<f32>(q${r}.x & 0xffff0000u), bitcast<f32>(q${r}.y << 16u), bitcast<f32>(q${r}.y & 0xffff0000u));`
    : `let w${r} = ${P}_W[er${r} + c];`;
  return `
${struct ? "struct NRT { dim: u32, dOut: u32, xs: u32, ls: u32, xns: u32, p0: u32, p1: u32, p2: u32 };\n" : ""}@group(1) @binding(0) var<storage, read> ${P}_x: array<f32>;
@group(1) @binding(1) var<storage, read> ${P}_nw: array<f32>;
@group(1) @binding(2) var<storage, read> ${P}_W: array<${bf16 ? "vec2<u32>" : "vec4<f32>"}>;
@group(1) @binding(3) var<storage, read_write> ${P}_xn: array<f32>;
@group(1) @binding(4) var<storage, read_write> ${P}_lg: array<f32>;
@group(1) @binding(5) var<uniform> ${P}_s: NRT;
var<workgroup> ${P}_red: array<f32, ${(ROWS + 1) * WG}>;
@compute @workgroup_size(${WG})
fn ${name}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let col = wg.y; let n = ${P}_s.dim; let xo = col * ${P}_s.xs; let d4 = n / 4u; let row0 = wg.x * ${ROWS}u;
  var ss: f32 = 0.0;
${rows((r) => `  var a${r}: f32 = 0.0; let er${r} = min(row0 + ${r}u, ${P}_s.dOut - 1u) * d4;`)}
  for (var c: u32 = t; c < d4; c += ${WG}u) {
    let j = c * 4u;
    let x4 = vec4<f32>(${P}_x[xo + j], ${P}_x[xo + j + 1u], ${P}_x[xo + j + 2u], ${P}_x[xo + j + 3u]);
    let xw = x4 * vec4<f32>(${P}_nw[j], ${P}_nw[j + 1u], ${P}_nw[j + 2u], ${P}_nw[j + 3u]);
    ss += dot(x4, x4);
${rows((r) => `    ${wv(r)} a${r} += dot(w${r}, xw);`)}
  }
  ${P}_red[t] = ss;
${rows((r) => `  ${P}_red[${(r + 1) * WG}u + t] = a${r};`)}
${tree(WG, ROWS + 1, `${P}_red`)}
  let inv = inverseSqrt(${P}_red[0] / f32(n) + cfg.eps);
  if (t < ${ROWS}u) { let row = row0 + t; if (row < ${P}_s.dOut) { ${P}_lg[col * ${P}_s.ls + row] = ${P}_red[(t + 1u) * ${WG}u] * inv; } }
  if (wg.x == 0u) { for (var j: u32 = t; j < n; j += ${WG}u) { ${P}_xn[col * ${P}_s.xns + j] = ${P}_x[xo + j] * inv * ${P}_nw[j]; } }
}`;
}

// The fused kernels for one engine: K (top-k), R (output rows per legacy moe_dnc workgroup), the
// (routed, shared) format pairs the model's layers need for gate/up and for down, and layout
// (moeFusedLayout(...): null = the legacy kernels, else the wide ones).
// nrt: null or { ROWS, bf16 } (moe_nrt, normRouterKernel).
export function moeFusedWGSL({ K, R = 2, gu = [], dn = [], layout = null, nrt = null }) {
  if (!(K >= 1 && K <= 16) || ![1, 2, 4].includes(R)) throw new Error(`moeFusedWGSL: K ${K}, R ${R}`);
  return /* wgsl */ `
// ---------------- fused mixture of experts (engine/wgsl/moe.js moeFusedWGSL) ----------------
struct MOEF { dOut: u32, dIn: u32, sDim: u32, nExp: u32, xs: u32, ys: u32, norm: u32, shOff: u32, oUq: u32, oGs: u32, oUs: u32, pad: u32 };
${routeKernel(K)}
${nrt ? normRouterKernel(nrt) : ""}
${gu.map(([f, s]) => layout ? gusKernelWide(f, s, K, layout.gu) : gusKernel(f, s, K)).join("\n")}
${dn.map(([f, s]) => layout ? dncKernelWide(f, s, K, layout.dn) : dncKernel(f, s, K, R)).join("\n")}
`;
}


function expertWGSL(kind, fmt, c) {
  const { name, decl, body } = expertKernel(kind, fmt, c);
  return `${decl}
@compute @workgroup_size(${c.WG})
fn ${name}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {${body}}
`;
}

// cfg: moeKernelConfig(...) result (MOE_DEFAULT at dim 2048, expert width 512 when omitted)
export function moeWGSL(cfg = moeKernelConfig("default", { dim: 2048, inter: 512 })) {
  return /* wgsl */ `
// ---------------- mixture of experts (engine/wgsl/moe.js) ----------------
struct MOE { dOut: u32, dIn: u32, K: u32, nExp: u32, xs: u32, ys: u32, norm: u32, pad: u32 };

// Router: softmax over all experts, the K largest probabilities (ties: lower index), their probabilities as
// weights, renormalised to sum 1 when norm = 1 (norm_topk_prob). One workgroup of 256 per column; max and sum
// are tree reductions and each of the K picks is an argmax tree over (probability, index), so every step is
// parallel and the order of every sum is fixed.
@group(1) @binding(0) var<storage, read> mr_l: array<f32>;
@group(1) @binding(1) var<storage, read_write> mr_sel: array<u32>;
@group(1) @binding(2) var<storage, read_write> mr_w: array<f32>;
@group(1) @binding(3) var<uniform> mr_s: MOE;
var<workgroup> mr_p: array<f32, 1024>;
var<workgroup> mr_v: array<f32, 256>;
var<workgroup> mr_i: array<u32, 256>;
var<workgroup> mr_k: array<f32, 16>;
@compute @workgroup_size(256)
fn moe_router(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let col = wg.x; let t = lid.x; let n = mr_s.nExp; let K = mr_s.K;
  let lb = col * mr_s.xs;
  var m: f32 = -3.0e38;
  for (var i: u32 = t; i < n; i += 256u) { m = max(m, mr_l[lb + i]); }
  mr_v[t] = m;
  workgroupBarrier();
  for (var st: u32 = 128u; st > 0u; st >>= 1u) { if (t < st) { mr_v[t] = max(mr_v[t], mr_v[t + st]); } workgroupBarrier(); }
  let mx = mr_v[0];
  workgroupBarrier();
  var s: f32 = 0.0;
  for (var i: u32 = t; i < n; i += 256u) { let p = exp(mr_l[lb + i] - mx); mr_p[i] = p; s += p; }
  mr_v[t] = s;
  workgroupBarrier();
  for (var st: u32 = 128u; st > 0u; st >>= 1u) { if (t < st) { mr_v[t] += mr_v[t + st]; } workgroupBarrier(); }
  let inv = 1.0 / mr_v[0];
  for (var i: u32 = t; i < n; i += 256u) { mr_p[i] = mr_p[i] * inv; }
  workgroupBarrier();
  for (var k: u32 = 0u; k < K; k++) {
    var bv: f32 = -1.0; var bi: u32 = 0u;
    for (var i: u32 = t; i < n; i += 256u) { if (mr_p[i] > bv) { bv = mr_p[i]; bi = i; } }
    mr_v[t] = bv; mr_i[t] = bi;
    workgroupBarrier();
    for (var st: u32 = 128u; st > 0u; st >>= 1u) {
      if (t < st) { let ov = mr_v[t + st]; let oi = mr_i[t + st]; if (ov > mr_v[t] || (ov == mr_v[t] && oi < mr_i[t])) { mr_v[t] = ov; mr_i[t] = oi; } }
      workgroupBarrier();
    }
    if (t == 0u) { mr_sel[col * K + k] = mr_i[0]; mr_k[k] = mr_v[0]; mr_p[mr_i[0]] = -2.0; }   // taken
    workgroupBarrier();
  }
  if (t == 0u) {
    var tot: f32 = 0.0;
    for (var k: u32 = 0u; k < K; k++) { tot += mr_k[k]; }
    for (var k: u32 = 0u; k < K; k++) { mr_w[col * K + k] = select(mr_k[k], mr_k[k] / tot, mr_s.norm == 1u); }
  }
}
${expertWGSL("gu", "q4", cfg.gu)}
${expertWGSL("gu", "q8", cfg.gu)}
${expertWGSL("dn", "q4", cfg.dn)}
${expertWGSL("dn", "q8", cfg.dn)}

// Combine: x[col] += sum_k w_k y[col,k] (k in order) + sigmoid(sg[col]) * shared[col].
// dOut = dim, xs = x column stride, ys = y (col,slot) stride, nExp (reused) = shared column stride,
// norm (reused) = 1 when there is a shared expert, pad (reused) = shared-gate logit column stride.
@group(1) @binding(0) var<storage, read_write> mc_x: array<f32>;
@group(1) @binding(1) var<storage, read> mc_y: array<f32>;
@group(1) @binding(2) var<storage, read> mc_w: array<f32>;
@group(1) @binding(3) var<storage, read> mc_sh: array<f32>;
@group(1) @binding(4) var<storage, read> mc_sg: array<f32>;
@group(1) @binding(5) var<uniform> mc_s: MOE;
@compute @workgroup_size(64)
fn moe_combine(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x; let col = gid.y; let S = mc_s;
  if (i >= S.dOut) { return; }
  var o: f32 = 0.0;
  for (var k: u32 = 0u; k < S.K; k++) { o += mc_w[col * S.K + k] * mc_y[(col * S.K + k) * S.ys + i]; }
  if (S.norm == 1u) {
    let g = mc_sg[col * S.pad];
    o += (1.0 / (1.0 + exp(-g))) * mc_sh[col * S.nExp + i];
  }
  mc_x[col * S.xs + i] += o;
}
`;
}
