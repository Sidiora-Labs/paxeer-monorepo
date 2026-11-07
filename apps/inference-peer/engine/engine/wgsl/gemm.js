// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/gemm.js (MIT; see LICENSE-ENGINE).
// Row-stationary Q4_0 GEMM for wide prefill passes (16 token columns).
//
// The batched GEMV (matvec_q4_coop_b) is column-stationary: it reloads the
// activation tile per row-group and runs at ~26 GB/s on 16 columns. This kernel
// makes each thread own R weight rows and ALL N columns, keeps the weight tile
// in shared memory as packed nibbles (vec4<u32>, never dequantized to shared),
// and broadcast-reads activations, giving 8 vec4 FMAs per shared load instead
// of ~5. Measured on a GB10: 0.757 ms vs 1.919 ms for the 4-column GEMV on
// 17408x5120x16, relDiff 6.3e-7. See docs/research/prefill-gemm-v2.md.
//
// Split-K (S workgroups per row tile, each covering 1/S of the reduction) keeps
// tall-tile shapes occupied; partials are summed by gemm_red in a fixed order,
// so results are run-to-run deterministic. S is PINNED per shape, never
// autotuned: every peer in a room must produce identical hidden states.
//
// Bindings reuse the matvec_q4_coop_b layout verbatim (qs, sc, x, y, shape).
// The only new declarations are type aliases of bindings 0 and 2, which is
// legal because no single entry point references two views of one binding.

export const GEMM_TILE = 128;          // rows per workgroup tile (T * R)

// Pinned split-K factor per output shape. Keys are `${dOut}x${dIn}`.
// Chosen by benchmark (docs/research/prefill-gemm-v2.md S5); treat edits as a
// protocol change under GOVERNANCE.md.
export const GEMM_S = {
  "17408x5120": 4, "5120x17408": 2, "5120x6144": 8, "10240x5120": 2,
  "6144x5120": 4, "12288x5120": 2, "1024x5120": 16, "5120x5120": 4,
};

// R16: also emit the prefillMath "f16" twins (gemm_q4_{dIn}_s{S}_r16, gemm_q8_..._r16 and
// gemm_xpose_r16): the same kernels with every dequantized weight and every activation rounded to
// f16 (pack2x16float, core WGSL), products and sums still f32. That is the operand precision of the
// tensor-core path (engine/wgsl/gemm_sgm.js) on the ALU, so its numerics can be A/B'd anywhere.
// Off by default; it changes prefill logits (docs/research/prefill-f16-subgroup.md).
export function gemmWGSL({ N = 16, T = 64, R = 2, KB = 2, splits = [2, 4, 8, 16],
                          dIns = [5120, 6144, 17408], pairs = null, pairs8 = [], UNPACK = true, R16 = false } = {}) {
  // pairs: [[dIn, S], ...] to emit exactly the kernels a model needs (each is
  // fully unrolled, so the module size and shader compile time scale with it)
  const rng = (n) => Array.from({ length: n }, (_, i) => i);
  const TM = T * R, WV = TM * KB, WPT = WV / T, XV = 32 * KB * (N / 4), XPT = XV / T;
  const RR = rng(R), QN = rng(N / 4);
  if (WV % T || XV % T) throw new Error("gemm: T must divide the stage sizes");
  if (N % 4) throw new Error("gemm: N must be a multiple of 4");
  const dq = (m, s) => UNPACK
    ? `(vec4<f32>(unpack4xU8(${m})) - vec4<f32>(8.0)) * ${s}`
    : `(vec4<f32>(f32(${m} & 0xFFu), f32((${m} >> 8u) & 0xFFu), f32((${m} >> 16u) & 0xFFu), f32(${m} >> 24u)) - vec4<f32>(8.0)) * ${s}`;

  const r16 = (e, on) => (on ? `gm_r16(${e})` : e);
  const kernel = (dIn, S, F16 = false) => {
    const nb = dIn / 32, nStages = nb / KB;
    if (nb % 2) throw new Error("gemm: dIn must be a multiple of 64 (f16 scales are paired)");
    if (nStages % S) throw new Error(`gemm: S=${S} must divide ${nStages} stages for dIn=${dIn}`);
    const stPerWG = nStages / S;
    return `
@compute @workgroup_size(${T})
fn gemm_q4_${dIn}_s${S}${F16 ? "_r16" : ""}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let dOut = qb_shape.dOut;
  let wgl = wg.y * 32768u + wg.x;                     // 2-D dispatch (trick 21)
  let tile = wgl / ${S}u; let split = wgl % ${S}u;
  let row0 = tile * ${TM}u; let st0 = split * ${stPerWG}u; let rb = row0 + t * ${R}u;
  ${RR.map((r) => QN.map((q) => `var a${r}_${q} = vec4<f32>(0.0);`).join(" ")).join("\n  ")}
  ${rng(WPT).map((j) => `let li${j} = t + ${j * T}u; let lr${j} = min(row0 + li${j} / ${KB}u, dOut - 1u); let lb${j} = li${j} % ${KB}u;`).join("\n  ")}
  ${rng(WPT).map((j) => `var w${j} = gm_qs4[lr${j} * ${nb}u + st0 * ${KB}u + lb${j}];`).join("\n  ")}
  ${rng(XPT).map((j) => `var xv${j} = gm_xT[st0 * ${XV}u + t + ${j * T}u];`).join("\n  ")}
  for (var s: u32 = 0u; s < ${stPerWG}u; s++) {
    workgroupBarrier();
    ${rng(WPT).map((j) => `gm_W[li${j}] = w${j};`).join(" ")}
    ${rng(XPT).map((j) => `gm_X[t + ${j * T}u] = xv${j};`).join(" ")}
    workgroupBarrier();
    let bs = (st0 + s) * ${KB}u;
    if (s + 1u < ${stPerWG}u) {                       // one-deep register prefetch
      ${rng(WPT).map((j) => `w${j} = gm_qs4[lr${j} * ${nb}u + bs + ${KB}u + lb${j}];`).join(" ")}
      ${rng(XPT).map((j) => `xv${j} = gm_xT[(st0 + s + 1u) * ${XV}u + t + ${j * T}u];`).join(" ")}
    }
    ${RR.map((r) => rng(Math.max(1, KB >> 1)).map((pp) => `let sw${r}_${pp} = q4_sc[((min(rb + ${r}u, dOut - 1u) * ${nb}u + bs) >> 1u) + ${pp}u];`).join(" ")).join(" ")}
    ${rng(KB).map((b) => `
    {
      ${RR.map((r) => `let sv${r} = unpack2x16float(sw${r}_${b >> 1})[${b & 1}u];`).join(" ")}
      ${RR.map((r) => `let wa${r} = gm_W[(t * ${R}u + ${r}u) * ${KB}u + ${b}u];`).join("\n      ")}
      ${rng(4).map((j) => `
      { ${RR.map((r) => `let lo${r} = ${r16(dq(`(wa${r}[${j}] & 0x0F0F0F0Fu)`, `sv${r}`), F16)}; let hi${r} = ${r16(dq(`((wa${r}[${j}] >> 4u) & 0x0F0F0F0Fu)`, `sv${r}`), F16)};`).join(" ")}
        ${rng(4).map((i) => { const kl = 32 * b + 4 * j + i, kh = kl + 16; return `
        { ${QN.map((q) => `let xl${q} = gm_X[${kl * (N / 4) + q}u];`).join(" ")} ${RR.map((r) => QN.map((q) => `a${r}_${q} += lo${r}[${i}] * xl${q};`).join(" ")).join(" ")} }
        { ${QN.map((q) => `let xh${q} = gm_X[${kh * (N / 4) + q}u];`).join(" ")} ${RR.map((r) => QN.map((q) => `a${r}_${q} += hi${r}[${i}] * xh${q};`).join(" ")).join(" ")} }`; }).join("")}
      }`).join("")}
    }`).join("")}
  }
  let pb = split * ${N}u * dOut;                      // partials: p[split][col][dOut]
  ${RR.map((r) => `if (rb + ${r}u < dOut) { ${QN.map((q) => rng(4).map((c) => `q4_y[pb + ${4 * q + c}u * dOut + rb + ${r}u] = a${r}_${q}[${c}];`).join(" ")).join(" ")} }`).join("\n  ")}
}`;
  };

  // Q8_0 twin of the kernel above (llama.cpp MMQ-style tiles, same row-stationary schedule and
  // split-K reduce): a block is 32 signed bytes = two vec4<u32> in sequence (no nibble
  // interleave), dequantized with unpack4xI8. The real 27B keeps ffn_down, ssm_out and
  // attn_output in Q8_0, so without this they stayed on the batched GEMV in prefill.
  const WV8 = TM * KB * 2, WPT8 = WV8 / T;
  if (WV8 % T) throw new Error("gemm q8: T must divide the stage size");
  const dq8 = (m, s) => UNPACK
    ? `vec4<f32>(unpack4xI8(${m})) * ${s}`
    : `vec4<f32>(f32(bitcast<i32>(${m} << 24u) >> 24u), f32(bitcast<i32>(${m} << 16u) >> 24u), f32(bitcast<i32>(${m} << 8u) >> 24u), f32(bitcast<i32>(${m}) >> 24u)) * ${s}`;
  const kernel8 = (dIn, S, F16 = false) => {
    const nb = dIn / 32, nStages = nb / KB;
    if (nb % 2) throw new Error("gemm q8: dIn must be a multiple of 64 (f16 scales are paired)");
    if (nStages % S) throw new Error(`gemm q8: S=${S} must divide ${nStages} stages for dIn=${dIn}`);
    const stPerWG = nStages / S, W2 = 2 * KB;   // vec4 words per row per stage
    return `
@compute @workgroup_size(${T})
fn gemm_q8_${dIn}_s${S}${F16 ? "_r16" : ""}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let dOut = qb_shape.dOut;
  let wgl = wg.y * 32768u + wg.x;
  let tile = wgl / ${S}u; let split = wgl % ${S}u;
  let row0 = tile * ${TM}u; let st0 = split * ${stPerWG}u; let rb = row0 + t * ${R}u;
  ${RR.map((r) => QN.map((q) => `var a${r}_${q} = vec4<f32>(0.0);`).join(" ")).join("\n  ")}
  ${rng(WPT8).map((j) => `let li${j} = t + ${j * T}u; let lr${j} = min(row0 + li${j} / ${W2}u, dOut - 1u); let lb${j} = li${j} % ${W2}u;`).join("\n  ")}
  ${rng(WPT8).map((j) => `var w${j} = gm_qs4[lr${j} * ${2 * nb}u + st0 * ${W2}u + lb${j}];`).join("\n  ")}
  ${rng(XPT).map((j) => `var xv${j} = gm_xT[st0 * ${XV}u + t + ${j * T}u];`).join("\n  ")}
  for (var s: u32 = 0u; s < ${stPerWG}u; s++) {
    workgroupBarrier();
    ${rng(WPT8).map((j) => `gm_W8[li${j}] = w${j};`).join(" ")}
    ${rng(XPT).map((j) => `gm_X[t + ${j * T}u] = xv${j};`).join(" ")}
    workgroupBarrier();
    let bs = (st0 + s) * ${KB}u;
    if (s + 1u < ${stPerWG}u) {
      ${rng(WPT8).map((j) => `w${j} = gm_qs4[lr${j} * ${2 * nb}u + (st0 + s + 1u) * ${W2}u + lb${j}];`).join(" ")}
      ${rng(XPT).map((j) => `xv${j} = gm_xT[(st0 + s + 1u) * ${XV}u + t + ${j * T}u];`).join(" ")}
    }
    ${RR.map((r) => rng(Math.max(1, KB >> 1)).map((pp) => `let sw${r}_${pp} = q4_sc[((min(rb + ${r}u, dOut - 1u) * ${nb}u + bs) >> 1u) + ${pp}u];`).join(" ")).join(" ")}
    ${rng(KB).map((b) => `
    {
      ${RR.map((r) => `let sv${r} = unpack2x16float(sw${r}_${b >> 1})[${b & 1}u];`).join(" ")}
      ${RR.map((r) => `let wa${r} = gm_W8[(t * ${R}u + ${r}u) * ${W2}u + ${2 * b}u]; let wb${r} = gm_W8[(t * ${R}u + ${r}u) * ${W2}u + ${2 * b + 1}u];`).join("\n      ")}
      ${rng(8).map((j) => `
      { ${RR.map((r) => `let d${r} = ${r16(dq8(`w${j < 4 ? "a" : "b"}${r}[${j % 4}]`, `sv${r}`), F16)};`).join(" ")}
        ${rng(4).map((i) => { const k = 32 * b + 4 * j + i; return `
        { ${QN.map((q) => `let x${q} = gm_X[${k * (N / 4) + q}u];`).join(" ")} ${RR.map((r) => QN.map((q) => `a${r}_${q} += d${r}[${i}] * x${q};`).join(" ")).join(" ")} }`; }).join("")}
      }`).join("")}
    }`).join("")}
  }
  let pb = split * ${N}u * dOut;
  ${RR.map((r) => `if (rb + ${r}u < dOut) { ${QN.map((q) => rng(4).map((c) => `q4_y[pb + ${4 * q + c}u * dOut + rb + ${r}u] = a${r}_${q}[${c}];`).join(" ")).join(" ")} }`).join("\n  ")}
}`;
  };

  // Fixed-order split-K reduce. `_acc` folds the residual add in, exactly like
  // matvec_*_coop_b_acc. Writes into the engine's column-strided y layout.
  const reduce = (S, ACC) => `
@compute @workgroup_size(64)
fn gemm_red_s${S}${ACC ? "_acc" : ""}(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x; let dOut = qb_shape.dOut; let n = ${N}u * dOut;
  if (i >= n) { return; }
  var acc = 0.0;
  ${rng(S).map((s) => `acc += gm_p[${s}u * n + i];`).join(" ")}
  q4_y[(i / dOut) * qb_shape.ys + (i % dOut)] ${ACC ? "+=" : "="} acc;
}`;

  // Column-major staging of the activations the GEMM broadcast-reads.
  // src is the engine's [col][dIn] layout with a 256-byte-aligned column stride.
  const xpose = (F16) => `
@compute @workgroup_size(64)
fn gemm_xpose${F16 ? "_r16" : ""}(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x; let dIn = qb_shape.dIn;
  if (i >= dIn * ${N}u) { return; }
  ${F16 ? `q4_y[i] = unpack2x16float(pack2x16float(vec2<f32>(clamp(gm_p[(i % ${N}u) * (qb_shape.xs4 * 4u) + i / ${N}u], -65504.0, 65504.0), 0.0))).x;`
    : `q4_y[i] = gm_p[(i % ${N}u) * (qb_shape.xs4 * 4u) + i / ${N}u];`}
}`;
  const r16fn = `
// round each component to the nearest f16 and back (the "f16" prefillMath operand precision)
fn gm_r16(v: vec4<f32>) -> vec4<f32> {
  // clamp first: WGSL's f32 -> f16 of a value outside the f16 range is undefined (not +-Inf) on GPUs
  let c = clamp(v, vec4<f32>(-65504.0), vec4<f32>(65504.0));
  return vec4<f32>(unpack2x16float(pack2x16float(c.xy)), unpack2x16float(pack2x16float(c.zw)));
}`;

  const kernels = [];
  const want = pairs ?? dIns.flatMap((dIn) => splits.map((S) => [dIn, S]));
  const seen = new Set();
  for (const [dIn, S] of want) {
    const key = `${dIn}:${S}`; if (seen.has(key)) continue; seen.add(key);
    if (((dIn / 32) / KB) % S === 0) { kernels.push(kernel(dIn, S)); if (R16) kernels.push(kernel(dIn, S, true)); }
  }
  const seen8 = new Set();
  for (const [dIn, S] of pairs8) {
    const key = `${dIn}:${S}`; if (seen8.has(key)) continue; seen8.add(key);
    if (((dIn / 32) / KB) % S === 0) { kernels.push(kernel8(dIn, S)); if (R16) kernels.push(kernel8(dIn, S, true)); }
  }
  const usedSplits = [...new Set([...want, ...pairs8].map(([, S]) => S))];
  return /* wgsl */ `
// ---- row-stationary Q4_0 GEMM (N=${N}, T=${T}, R=${R}, KB=${KB}) ----
// Aliases of bindings already declared by the matvec kernels. Legal because no
// entry point below references more than one view of the same binding.
@group(1) @binding(0) var<storage, read> gm_qs4: array<vec4<u32>>;   // packed nibbles
@group(1) @binding(0) var<storage, read> gm_p: array<f32>;           // split-K partials / xpose source
@group(1) @binding(2) var<storage, read> gm_xT: array<vec4<f32>>;    // column-major activations
var<workgroup> gm_W: array<vec4<u32>, ${WV}>;
${pairs8.length ? `var<workgroup> gm_W8: array<vec4<u32>, ${WV8}>;` : ""}
var<workgroup> gm_X: array<vec4<f32>, ${XV}>;
${R16 ? r16fn + "\n" : ""}${kernels.join("\n")}
${usedSplits.map((S) => reduce(S, false) + reduce(S, true)).join("\n")}
${xpose(false)}
${R16 ? xpose(true) + "\n" : ""}`;
}
