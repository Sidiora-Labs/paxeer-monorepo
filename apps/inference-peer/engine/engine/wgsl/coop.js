// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/coop.js (MIT; see LICENSE-ENGINE).
// Generated WGSL: cooperative GEMV, batched/twin variants, fused gate/up, accumulate variants.
// See docs/kernels.md for the rules these kernels follow.
//
// One workgroup of WG threads computes ROWS output rows together (the shape
// llama.cpp's WebGPU backend, web-llm's generated kernels and zero-tvm all
// converge on for decode GEMV). Thread t = (block-lane bl = t/4) x (quarter
// qt = t%4, an 8-element slice of a 32-element quant block): consecutive
// threads read consecutive words of the same row (coalesced) and each
// thread's activation slice is loaded once and reused across all ROWS rows.
// Scalar accumulators only: a dynamically-indexed local array spills to
// scratch memory and ran 3x slower. Reduction is a portable shared-memory
// halving tree (no subgroups: absent from shipping Safari 26).

export async function probeUnpack(device) {
  if (device.__unpackOk !== undefined) return device.__unpackOk;
  device.pushErrorScope("validation");
  const m = device.createShaderModule({ code: `@compute @workgroup_size(1) fn p() { let v = vec4<f32>(unpack4xU8(0x0F0F0F0Fu)) + vec4<f32>(unpack4xI8(1u)); }` });
  const info = await m.getCompilationInfo();
  const err = await device.popErrorScope();
  device.__unpackOk = !err && !info.messages.some((x) => x.type === "error");
  return device.__unpackOk;
}

export function coopWGSL(WG = 256, ROWS = 4, WGB = 64, COLS = 4, ROWSB = ROWS, UNPACK = true, CV = false, NRM = false) {
  // Rows per workgroup for a C-column batched kernel: hold accumulators/thread
  // constant, so the 8- and 4-column twins are not starved of rows when COLS=16.
  const rowsFor = (C) => Math.max(1, Math.min(8, Math.round(ROWSB * COLS / C)));
  // dequant snippets: unpack4xU8/unpack4xI8 turn 4 packed bytes into a vec4 in
  // one instruction (Q4: two masked unpacks per word; Q8: one), vs 3 ALU ops
  // per element. Same values bit-for-bit; the GEMVs are instruction-bound.
  const q4lo = (w) => UNPACK ? `vec4<f32>(unpack4xU8(${w} & 0x0F0F0F0Fu)) - vec4<f32>(8.0)`
    : `vec4<f32>(f32(${w} & 0xFu), f32((${w} >> 8u) & 0xFu), f32((${w} >> 16u) & 0xFu), f32((${w} >> 24u) & 0xFu)) - vec4<f32>(8.0)`;
  const q4hi = (w) => UNPACK ? `vec4<f32>(unpack4xU8((${w} >> 4u) & 0x0F0F0F0Fu)) - vec4<f32>(8.0)`
    : `vec4<f32>(f32((${w} >> 4u) & 0xFu), f32((${w} >> 12u) & 0xFu), f32((${w} >> 20u) & 0xFu), f32((${w} >> 28u) & 0xFu)) - vec4<f32>(8.0)`;
  const i8x4 = (w) => UNPACK ? `vec4<f32>(unpack4xI8(bitcast<u32>(${w})))`
    : `vec4<f32>(f32((${w} << 24u) >> 24u), f32((${w} << 16u) >> 24u), f32((${w} << 8u) >> 24u), f32(${w} >> 24u))`;
  const LANES = WG / 4;
  const LANESB = WGB / 4;   // batched kernels: smaller workgroups amortize the reductions
  const accDecl = Array.from({ length: ROWS }, (_, r) => `var acc${r} = 0.0;`).join(" ");
  const fullBody = (term) => Array.from({ length: ROWS }, (_, r) => `      acc${r} += ${term(r)};`).join("\n");
  const tailBody = (term, dOut) => Array.from({ length: ROWS - 1 }, (_, r) =>
    `      if (row0 + ${r}u < ${dOut}) { acc${r} += ${term(r)}; }`).join("\n");
  const store = Array.from({ length: ROWS }, (_, r) => `  mvc_part[${r * WG}u + t] = acc${r};`).join("\n");
  const treeAdd = Array.from({ length: ROWS }, (_, r) =>
    `      mvc_part[${r * WG}u + t] += mvc_part[${r * WG}u + t + stride];`).join("\n");
  const reduce = `
${store}
  workgroupBarrier();
  var stride: u32 = ${WG / 2}u;
  while (stride > 0u) {
    if (t < stride) {
${treeAdd}
    }
    workgroupBarrier();
    stride = stride >> 1u;
  }`;
  // fused gate/up: two GEMVs sharing x, SiLU(g)*u epilogue in-kernel.
  // Saves one full-size dispatch + the silu dispatch + the u round-trip per
  // layer. Helpers are generated per concrete buffer (no pointer params:
  // unrestricted_pointer_parameters is not guaranteed in Safari 26).
  const guKernel = (kind, batched) => {
    const P = kind === "f32" ? "guf" : kind === "q8" ? "gu8" : "gu4";
    const shape = `${P}_shape`;
    const decl = kind === "f32" ? `
@group(1) @binding(0) var<storage, read> ${P}_gw: array<vec4<f32>>;
@group(1) @binding(1) var<storage, read> ${P}_uw: array<vec4<f32>>;
@group(1) @binding(2) var<storage, read> ${P}_x: array<vec4<f32>>;
@group(1) @binding(3) var<storage, read_write> ${P}_y: array<f32>;
@group(1) @binding(4) var<uniform> ${shape}: BShape;` : `
@group(1) @binding(0) var<storage, read> ${P}_gqs: array<u32>;
@group(1) @binding(1) var<storage, read> ${P}_gsc: array<u32>;
@group(1) @binding(2) var<storage, read> ${P}_uqs: array<u32>;
@group(1) @binding(3) var<storage, read> ${P}_usc: array<u32>;
@group(1) @binding(4) var<storage, read> ${P}_x: array<vec4<f32>>;
@group(1) @binding(5) var<storage, read_write> ${P}_y: array<f32>;
@group(1) @binding(6) var<uniform> ${shape}: BShape;`;
    const helpers = kind === "f32" ? "" : ["g", "u"].map((w) => kind === "q8" ? `
fn ${P}_${w}row(wBase: u32, si: u32, xa: vec4<f32>, xb: vec4<f32>) -> f32 {
  let sc = unpack2x16float(${P}_${w}sc[si >> 1u])[si & 1u];
  let w0 = bitcast<i32>(${P}_${w}qs[wBase]);
  let w1 = bitcast<i32>(${P}_${w}qs[wBase + 1u]);
  let d0 = ${i8x4("w0")};
  let d1 = ${i8x4("w1")};
  return sc * (dot(d0, xa) + dot(d1, xb));
}` : `
fn ${P}_${w}row(wIdx: u32, si: u32, xlo: vec4<f32>, xhi: vec4<f32>) -> f32 {
  let sc = unpack2x16float(${P}_${w}sc[si >> 1u])[si & 1u];
  let word = ${P}_${w}qs[wIdx];
  let lo = ${q4lo("word")};
  let hi = ${q4hi("word")};
  return sc * (dot(lo, xlo) + dot(hi, xhi));
}`).join("");
    const rowTerm = (which, r) => kind === "f32"
      ? `dot(${P}_${which}w[off4 + ${r}u * dIn4], xa) + dot(${P}_${which}w[off4 + ${r}u * dIn4 + 1u], xb)`
      : `${P}_${which}row(${kind === "q8" ? "wBase" : "wIdx"} + ${r}u * rowWords, scBase + ${r}u * nb, xa, xb)`;
    const loads = kind === "q4" ? `
      let xa = ${P}_x[xcol + b * 8u + qt];
      let xb = ${P}_x[xcol + b * 8u + qt + 4u];
      let wIdx = row0 * rowWords + b * 4u + qt;
      let scBase = row0 * nb + b;` : kind === "q8" ? `
      let xa = ${P}_x[xcol + b * 8u + qt * 2u];
      let xb = ${P}_x[xcol + b * 8u + qt * 2u + 1u];
      let wBase = row0 * rowWords + b * 8u + qt * 2u;
      let scBase = row0 * nb + b;` : `
      let xa = ${P}_x[xcol + b * 8u + qt * 2u];
      let xb = ${P}_x[xcol + b * 8u + qt * 2u + 1u];
      let off4 = row0 * dIn4 + b * 8u + qt * 2u;`;
    const accs = (w) => Array.from({ length: ROWS }, (_, r) => `var ${w}${r} = 0.0;`).join(" ");
    const body = (w, which) => `      if (full) {
${Array.from({ length: ROWS }, (_, r) => `        ${w}${r} += ${rowTerm(which, r)};`).join("\n")}
      } else {
${Array.from({ length: ROWS - 1 }, (_, r) => `        if (row0 + ${r}u < dOut) { ${w}${r} += ${rowTerm(which, r)}; }`).join("\n")}
      }`;
    const reduceTo = (w, dest) => `
${Array.from({ length: ROWS }, (_, r) => `  mvc_part[${r * WG}u + t] = ${w}${r};`).join("\n")}
  workgroupBarrier();
  {
    var stride: u32 = ${WG / 2}u;
    while (stride > 0u) {
      if (t < stride) {
${Array.from({ length: ROWS }, (_, r) => `        mvc_part[${r * WG}u + t] += mvc_part[${r * WG}u + t + stride];`).join("\n")}
      }
      workgroupBarrier();
      stride = stride >> 1u;
    }
  }
  ${dest}
  workgroupBarrier();`;
    const colBody = (m) => `
  {
    let xcol = ${batched ? `${m}u * ${shape}.xs4 * 0u + ${m}u * ${shape}.xs4` : "0u"};
    ${accs("ag")}
    ${accs("au")}
    for (var b: u32 = bl; b < nb; b += ${LANES}u) {${loads}
${body("ag", "g")}
${body("au", "u")}
    }
${reduceTo("ag", `if (t < ${ROWS}u) { gu_res[t] = mvc_part[t * ${WG}u]; }`)}
${reduceTo("au", `if (t < ${ROWS}u) {
    let row = row0 + t;
    if (row < dOut) {
      let g = gu_res[t];
      ${P}_y[${batched ? `${m}u * ${shape}.ys + row` : "row"}] = (g / (1.0 + exp(-g))) * mvc_part[t * ${WG}u];
    }
  }`)}
  }`;
    return `${batched ? "" : decl + helpers}
@compute @workgroup_size(${WG})
fn matvec${kind === "f32" ? "" : "_" + kind}_gu${batched ? "_b" : ""}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let qt = t & 3u;
  let bl = t >> 2u;
  let dIn = ${shape}.dIn;
  let dOut = ${shape}.dOut;
  let nb = dIn / 32u;
  let dIn4 = dIn / 4u;
  let rowWords = ${kind === "q4" ? "dIn / 8u" : "dIn / 4u"};
  let row0 = (wg.y * 32768u + wg.x) * ${ROWS}u;
  let full = row0 + ${ROWS - 1}u < dOut;
${batched ? [0, 1, 2, 3].map(colBody).join("\n") : colBody(0)}
}`;
  };
  // batched gate/up: ONE pass over the weights; each g/u word is loaded and
  // decoded once and applied to all 4 columns (2*ROWS*4 accumulators), then
  // one reduction round per column.
  const guKernelB = (kind, C = COLS) => {
    const P = kind === "f32" ? "guf" : kind === "q8" ? "gu8" : "gu4";
    const shape = `${P}_shape`;
    const RB = rowsFor(C), rows = Array.from({ length: RB }, (_, r) => r);
    const cols = Array.from({ length: C }, (_, m) => m);
    const xLoads = kind === "q4"
      ? cols.map((m) => `let xa${m} = ${P}_x[${m}u * xs4 + b * 8u + qt]; let xb${m} = ${P}_x[${m}u * xs4 + b * 8u + qt + 4u];`).join("\n      ")
      : cols.map((m) => `let xa${m} = ${P}_x[${m}u * xs4 + b * 8u + qt * 2u]; let xb${m} = ${P}_x[${m}u * xs4 + b * 8u + qt * 2u + 1u];`).join("\n      ");
    const idx = kind === "q4" ? `let wIdx = row0 * rowWords + b * 4u + qt; let scBase = row0 * nb + b;`
      : kind === "q8" ? `let wBase = row0 * rowWords + b * 8u + qt * 2u; let scBase = row0 * nb + b;`
      : `let off4 = row0 * dIn4 + b * 8u + qt * 2u;`;
    const nib = (w, v) => `let ${v}lo = ${q4lo(w)};
        let ${v}hi = ${q4hi(w)};`;
    const sx = (w, v) => `let ${v} = ${i8x4(w)};`;
    const rowBlock = (r) => {
      const acc = (ga, gb, ua, ub, gs, us) => cols.map((m) =>
        `ag${r}_${m} += ${gs}(dot(${ga}, xa${m}) + dot(${gb}, xb${m})); au${r}_${m} += ${us}(dot(${ua}, xa${m}) + dot(${ub}, xb${m}));`).join(" ");
      if (kind === "f32") return `if (full || row0 + ${r}u < dOut) {
        let ga = ${P}_gw[off4 + ${r}u * dIn4]; let gb = ${P}_gw[off4 + ${r}u * dIn4 + 1u];
        let ua = ${P}_uw[off4 + ${r}u * dIn4]; let ub = ${P}_uw[off4 + ${r}u * dIn4 + 1u];
        ${acc("ga", "gb", "ua", "ub", "", "")}
      }`;
      if (kind === "q8") return `if (full || row0 + ${r}u < dOut) {
        let si = scBase + ${r}u * nb;
        let gs = unpack2x16float(${P}_gsc[si >> 1u])[si & 1u]; let us = unpack2x16float(${P}_usc[si >> 1u])[si & 1u];
        let g0 = bitcast<i32>(${P}_gqs[wBase + ${r}u * rowWords]); let g1 = bitcast<i32>(${P}_gqs[wBase + ${r}u * rowWords + 1u]);
        let u0 = bitcast<i32>(${P}_uqs[wBase + ${r}u * rowWords]); let u1 = bitcast<i32>(${P}_uqs[wBase + ${r}u * rowWords + 1u]);
        ${sx("g0", "gd0")} ${sx("g1", "gd1")} ${sx("u0", "ud0")} ${sx("u1", "ud1")}
        ${acc("gd0", "gd1", "ud0", "ud1", "gs * ", "us * ")}
      }`;
      return `if (full || row0 + ${r}u < dOut) {
        let si = scBase + ${r}u * nb;
        let gs = unpack2x16float(${P}_gsc[si >> 1u])[si & 1u]; let us = unpack2x16float(${P}_usc[si >> 1u])[si & 1u];
        let gw = ${P}_gqs[wIdx + ${r}u * rowWords]; let uw = ${P}_uqs[wIdx + ${r}u * rowWords];
        ${nib("gw", "g")}
        ${nib("uw", "u")}
        ${acc("glo", "ghi", "ulo", "uhi", "gs * ", "us * ")}
      }`;
    };
    const K2 = 2 * RB;
    return `
@compute @workgroup_size(${WGB})
fn matvec${kind === "f32" ? "" : "_" + kind}_gu_b${C === COLS ? "" : C}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let qt = t & 3u;
  let bl = t >> 2u;
  let dIn = ${shape}.dIn;
  let dOut = ${shape}.dOut;
  let xs4 = ${shape}.xs4;
  let ys = ${shape}.ys;
  let nb = dIn / 32u;
  let dIn4 = dIn / 4u;
  let rowWords = ${kind === "q4" ? "dIn / 8u" : "dIn / 4u"};
  let row0 = (wg.y * 32768u + wg.x) * ${RB}u;
  let full = row0 + ${RB - 1}u < dOut;
  ${rows.map((r) => cols.map((m) => `var ag${r}_${m} = 0.0; var au${r}_${m} = 0.0;`).join(" ")).join("\n  ")}
  for (var b: u32 = bl; b < nb; b += ${LANESB}u) {
      ${xLoads}
      ${idx}
      ${rows.map(rowBlock).join("\n      ")}
  }
${cols.map((m) => `
  ${rows.map((r) => `gub_part[${r * WGB}u + t] = ag${r}_${m}; gub_part[${(RB + r) * WGB}u + t] = au${r}_${m};`).join(" ")}
  workgroupBarrier();
  {
    var stride: u32 = ${WGB / 2}u;
    while (stride > 0u) {
      if (t < stride) { ${Array.from({ length: K2 }, (_, k) => `gub_part[${k * WGB}u + t] += gub_part[${k * WGB}u + t + stride];`).join(" ")} }
      workgroupBarrier();
      stride = stride >> 1u;
    }
  }
  if (t < ${RB}u) {
    let row = row0 + t;
    if (row < dOut) {
      let g = gub_part[t * ${WGB}u];
      ${P}_y[${m}u * ys + row] = (g / (1.0 + exp(-g))) * gub_part[(${RB}u + t) * ${WGB}u];
    }
  }
  workgroupBarrier();`).join("\n")}
}`;
  };
  const guAll = `
var<workgroup> gu_res: array<f32, ${ROWS}>;
var<workgroup> gub_part: array<f32, ${2 * rowsFor(4) * WGB}>;
` + ["f32", "q8", "q4"].map((k) => guKernel(k, false)).join("\n")
    + ["f32", "q8", "q4"].map((k) => guKernelB(k)).join("\n")
    + (COLS > 8 ? ["f32", "q8", "q4"].map((k) => guKernelB(k, 8)).join("\n") : "")
    + (COLS > 4 ? ["f32", "q8", "q4"].map((k) => guKernelB(k, 4)).join("\n") : "");
  const singleCoop = `@compute @workgroup_size(${WG})
fn matvec_coop(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let qt = t & 3u;
  let bl = t >> 2u;
  let dIn = mv_shape.dIn;
  let nb = dIn / 32u;
  let dIn4 = dIn / 4u;
  let row0 = (wg.y * 32768u + wg.x) * ${ROWS}u;
  let full = row0 + ${ROWS - 1}u < mv_shape.dOut;
  ${accDecl}
  for (var b: u32 = bl; b < nb; b += ${LANES}u) {
    let c4 = b * 8u + qt * 2u;
    let xa = mv_x4[c4];
    let xb = mv_x4[c4 + 1u];
    let off4 = row0 * dIn4 + c4;
    if (full) {
${fullBody((r) => `mvf_row(off4 + ${r}u * dIn4, xa, xb)`)}
    } else {
${tailBody((r) => `mvf_row(off4 + ${r}u * dIn4, xa, xb)`, "mv_shape.dOut")}
    }
  }
${reduce}
  if (t < ${ROWS}u) {
    let row = row0 + t;
    if (row < mv_shape.dOut) { mv_y[row] = mvc_part[t * ${WG}u]; }
  }
}

fn q8_row(wBase: u32, sc: f32, xa: vec4<f32>, xb: vec4<f32>) -> f32 {
  let w0 = bitcast<i32>(q8_qs[wBase]);
  let w1 = bitcast<i32>(q8_qs[wBase + 1u]);
  // shift-based sign extension (extractBits goes through slow polyfill paths)
  let d0 = ${i8x4("w0")};
  let d1 = ${i8x4("w1")};
  return sc * (dot(d0, xa) + dot(d1, xb));
}
@compute @workgroup_size(${WG})
fn matvec_q8_coop(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let qt = t & 3u;
  let bl = t >> 2u;
  let dIn = q8_shape.dIn;
  let nb = dIn / 32u;
  let rowWords = dIn / 4u;
  let row0 = (wg.y * 32768u + wg.x) * ${ROWS}u;
  let full = row0 + ${ROWS - 1}u < q8_shape.dOut;
  ${accDecl}
  for (var b: u32 = bl; b < nb; b += ${LANES}u) {
    let x4 = b * 8u + qt * 2u;
    let xa = q8_x4[x4];
    let xb = q8_x4[x4 + 1u];
    let wBase = row0 * rowWords + b * 8u + qt * 2u;
    let scBase = row0 * nb + b;
    if (full) {
${fullBody((r) => `q8_row(wBase + ${r}u * rowWords, q8s(scBase + ${r}u * nb), xa, xb)`)}
    } else {
${tailBody((r) => `q8_row(wBase + ${r}u * rowWords, q8s(scBase + ${r}u * nb), xa, xb)`, "q8_shape.dOut")}
    }
  }
${reduce}
  if (t < ${ROWS}u) {
    let row = row0 + t;
    if (row < q8_shape.dOut) { q8_y[row] = mvc_part[t * ${WG}u]; }
  }
}

fn q4_row(wIdx: u32, sc: f32, xlo: vec4<f32>, xhi: vec4<f32>) -> f32 {
  let word = q4_qs[wIdx];
  let lo = ${q4lo("word")};
  let hi = ${q4hi("word")};
  return sc * (dot(lo, xlo) + dot(hi, xhi));
}
@compute @workgroup_size(${WG})
fn matvec_q4_coop(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let qt = t & 3u;
  let bl = t >> 2u;
  let dIn = q4_shape.dIn;
  let nb = dIn / 32u;
  let rowWords = dIn / 8u;   // 4 bits/weight -> dIn/8 u32 words per row
  let row0 = (wg.y * 32768u + wg.x) * ${ROWS}u;
  let full = row0 + ${ROWS - 1}u < q4_shape.dOut;
  ${accDecl}
  for (var b: u32 = bl; b < nb; b += ${LANES}u) {
    // word qt of block b covers x[j..j+3] (low nibbles) and x[j+16..j+19] (high)
    let xlo = q4_x4[b * 8u + qt];
    let xhi = q4_x4[b * 8u + qt + 4u];
    let wIdx = row0 * rowWords + b * 4u + qt;
    let scBase = row0 * nb + b;
    if (full) {
${fullBody((r) => `q4_row(wIdx + ${r}u * rowWords, q4s(scBase + ${r}u * nb), xlo, xhi)`)}
    } else {
${tailBody((r) => `q4_row(wIdx + ${r}u * rowWords, q4s(scBase + ${r}u * nb), xlo, xhi)`, "q4_shape.dOut")}
    }
  }
${reduce}
  if (t < ${ROWS}u) {
    let row = row0 + t;
    if (row < q4_shape.dOut) { q4_y[row] = mvc_part[t * ${WG}u]; }
  }
}
`;
  // accumulate variants: y[row] += W x (residual add folded into the matvec)
  const singleCoopAcc = singleCoop.replace(/fn (q8|q4)_row\([\s\S]*?\n}\n/g, "")   // helpers already defined once
    .replace(/_coop\(/g, "_coop_acc(").replace(/_y\[row\] = /g, "_y[row] += ");
  // conv variants (CV, engine layerFuse "conv"): the DeltaNet [qkv | z] GEMV with dn_conv in its
  // epilogue. Rows below convDim are the qkv channels: the row's thread runs dn_conv's statements on
  // the reduced value (causal conv over the rolling state, SiLU, state shift) and writes the conv
  // output; the other rows (padding, z) are stored as usual. Each channel belongs to one workgroup,
  // so the state update has no race. Needs the DN struct (WGSL2) in the same module.
  const cvDecl = (P, b0) => `
@group(1) @binding(${b0}) var<storage, read> ${P}_w: array<f32>;        // conv weights [convDim, 4]
@group(1) @binding(${b0 + 1}) var<storage, read_write> ${P}_st: array<f32>;  // rolling state [convDim, 3]
@group(1) @binding(${b0 + 2}) var<storage, read_write> ${P}_y: array<f32>;   // conv output [convDim]
@group(1) @binding(${b0 + 3}) var<uniform> ${P}_dn: DN;
fn ${P}_conv(c: u32, x: f32) {
  var acc = ${P}_w[c * 4u + 3u] * x;
  acc += ${P}_w[c * 4u + 0u] * ${P}_st[c * 3u + 0u];
  acc += ${P}_w[c * 4u + 1u] * ${P}_st[c * 3u + 1u];
  acc += ${P}_w[c * 4u + 2u] * ${P}_st[c * 3u + 2u];
  ${P}_y[c] = acc / (1.0 + exp(-acc));
  ${P}_st[c * 3u + 0u] = ${P}_st[c * 3u + 1u];
  ${P}_st[c * 3u + 1u] = ${P}_st[c * 3u + 2u];
  ${P}_st[c * 3u + 2u] = x;
}`;
  // normed variants (NRM, engine layerFuse "norm"): rmsnorm folded into a small GEMV. Every workgroup
  // recomputes the norm of the raw residual x exactly as the rmsnorm kernel does (256 partial sums, each
  // strided by 256 with four loads in flight, then the same halving tree; WG < 256 threads each run
  // several of the 256 partials), normalises its own x slices in registers ((x * inv) * w, as rmsnorm
  // writes them) and workgroup 0 also writes the whole normed vector for the layer's later dispatches.
  // Same bits as rmsnorm followed by the plain GEMV. For GEMVs with few workgroups (router, beta/alpha, k/v).
  const nrmDecl = (P, b0) => `
@group(1) @binding(${b0}) var<storage, read> ${P}_w4: array<vec4<f32>>;   // norm weight
@group(1) @binding(${b0 + 1}) var<storage, read_write> ${P}_xn: array<f32>;  // normed x, written by workgroup 0`;
  const nrmPro = (X4, P, n) => {
    const X = (i) => `${X4}[(${i}) >> 2u][(${i}) & 3u]`;
    return `
  for (var v: u32 = t; v < 256u; v += ${WG}u) {
    var ss: f32 = 0.0;
    var i: u32 = v;
    for (; i + 768u < ${n}; i += 1024u) {
      let v0 = ${X("i")}; let v1 = ${X("i + 256u")}; let v2 = ${X("i + 512u")}; let v3 = ${X("i + 768u")};
      ss += v0 * v0; ss += v1 * v1; ss += v2 * v2; ss += v3 * v3;
    }
    for (; i < ${n}; i += 256u) { let v = ${X("i")}; ss += v * v; }
    nrm_p[v] = ss;
  }
  workgroupBarrier();
  var nstride: u32 = 128u;
  while (nstride > 0u) {
    // a trip count every thread shares, the body guarded: FXC (Windows D3D12) rejects a thread-dependent
    // loop here (for v = t; v < nstride) with X4026 at the barrier below. Same adds, same order.
    for (var v0: u32 = 0u; v0 < nstride; v0 += ${WG}u) { let v = v0 + t; if (v < nstride) { nrm_p[v] += nrm_p[v + nstride]; } }
    workgroupBarrier();
    nstride = nstride >> 1u;
  }
  let ninv = inverseSqrt(nrm_p[0] / f32(${n}) + cfg.eps);
  if (wg.x == 0u && wg.y == 0u) {
    for (var j: u32 = t; j < ${n}; j += ${WG}u) { ${P}_xn[j] = ${X("j")} * ninv * ${P}_w4[j >> 2u][j & 3u]; }
  }`;
  };
  const singleCoopN = !NRM ? "" : "var<workgroup> nrm_p: array<f32, 256>;" + nrmDecl("nrf", 4) + nrmDecl("nrq", 5) + "\n"
    + singleCoop.replace(/fn (q8|q4)_row\([\s\S]*?\n}\n/g, "")
      .split(/(?=@compute)/).map((k) => {
        const m = k.match(/fn matvec(_q8|_q4)?_coop\(/);
        if (!m) return k;
        const X4 = m[1] === "_q8" ? "q8_x4" : m[1] === "_q4" ? "q4_x4" : "mv_x4", P = m[1] ? "nrq" : "nrf";
        const shape = m[1] === "_q8" ? "q8_shape" : m[1] === "_q4" ? "q4_shape" : "mv_shape";
        return k.replace(/_coop\(/, "_coop_n(")
          .replace(new RegExp(`let (\\w+) = ${X4}\\[([^;]+)\\];`, "g"), (_, v, idx) => `let ${v} = (${X4}[${idx}] * ninv) * ${P}_w4[${idx}];`)
          .replace("let t = lid.x;\n", `let t = lid.x;${nrmPro(X4, P, `${shape}.dIn`)}\n`);
      }).join("");
  const singleCoopCv = !CV ? "" : cvDecl("cvf", 4) + cvDecl("cvq", 5) + "\n" + singleCoop.replace(/fn (q8|q4)_row\([\s\S]*?\n}\n/g, "")
    .replace(/_coop\(/g, "_coop_cv(")
    .replace(/(\w+)_y\[row\] = (mvc_part\[t \* \d+u\]);/g, (m, P, v) => {
      const C = P === "mv" ? "cvf" : "cvq";
      return `let v = ${v}; if (row < ${C}_dn.convDim) { ${C}_conv(row, v); } else { ${P}_y[row] = v; }`;
    });
  return /* wgsl */ `
var<workgroup> mvc_part: array<f32, ${WG * ROWS}>;   // [${ROWS} rows][${WG} threads]

// vec4 views of the same buffers the scalar kernels bind (same @group/@binding
// is legal as long as no single entry point references both views). All x/w
// buffers are multiples of 16 bytes (dims divisible by 4).
@group(1) @binding(0) var<storage, read> mv_w4: array<vec4<f32>>;
@group(1) @binding(1) var<storage, read> mv_x4: array<vec4<f32>>;
@group(1) @binding(2) var<storage, read> q8_x4: array<vec4<f32>>;
@group(1) @binding(2) var<storage, read> q4_x4: array<vec4<f32>>;

fn mvf_row(off4: u32, xa: vec4<f32>, xb: vec4<f32>) -> f32 {
  return dot(mv_w4[off4], xa) + dot(mv_w4[off4 + 1u], xb);
}
${singleCoop}
${singleCoopAcc}
${singleCoopCv}
${singleCoopN}

// ---- batched (COLS-column) variants for prefill / verify: each weight word is
// loaded and decoded once and applied to C token columns (rowsFor(C) rows per WG). x is [C][xs4] vec4s,
// y is [C][ys] f32s (strides in BShape; slices are 256-byte aligned by the
// engine). Workgroup ${WGB}: more loop work per thread, cheaper reductions.
struct BShape { dOut: u32, dIn: u32, xs4: u32, ys: u32 };
@group(1) @binding(3) var<uniform> mvb_shape: BShape;
@group(1) @binding(4) var<uniform> qb_shape: BShape;
var<workgroup> mvb_part: array<f32, ${2 * rowsFor(4) * WGB}>;
${[COLS, 8, 4].filter((c, i) => i === 0 || c < COLS).map((C) => [false, true].map((ACC) => ["", "_q8", "_q4"].map((kind) => {
  const shp = kind === "" ? "mvb_shape" : "qb_shape";
  const xbuf = kind === "" ? "mv_x4" : kind === "_q8" ? "q8_x4" : "q4_x4";
  const ybuf = kind === "" ? "mv_y" : kind === "_q8" ? "q8_y" : "q4_y";
  const RB = rowsFor(C), rows = Array.from({ length: RB }, (_, r) => r);
  const cols = Array.from({ length: C }, (_, m) => m);
  const xLoads = kind === "_q4"
    ? cols.map((m) => `let xa${m} = ${xbuf}[${m}u * xs4 + b * 8u + qt]; let xb${m} = ${xbuf}[${m}u * xs4 + b * 8u + qt + 4u];`).join("\n    ")
    : cols.map((m) => `let xa${m} = ${xbuf}[${m}u * xs4 + b * 8u + qt * 2u]; let xb${m} = ${xbuf}[${m}u * xs4 + b * 8u + qt * 2u + 1u];`).join("\n    ");
  const idx = kind === "_q4" ? "let wIdx = row0 * rowWords + b * 4u + qt; let scBase = row0 * nb + b;"
    : kind === "_q8" ? "let wBase = row0 * rowWords + b * 8u + qt * 2u; let scBase = row0 * nb + b;"
    : "let off4 = row0 * rowWords + b * 8u + qt * 2u;";
  const rowBlock = (r) => kind === "_q4" ? `if (full || row0 + ${r}u < dOut) {
      let word = q4_qs[wIdx + ${r}u * rowWords];
      let lo = ${q4lo("word")};
      let hi = ${q4hi("word")};
      let s = q4s(scBase + ${r}u * nb);
      ${cols.map((m) => `a${r}_${m} += s * (dot(lo, xa${m}) + dot(hi, xb${m}));`).join(" ")}
    }` : kind === "_q8" ? `if (full || row0 + ${r}u < dOut) {
      let w0 = bitcast<i32>(q8_qs[wBase + ${r}u * rowWords]);
      let w1 = bitcast<i32>(q8_qs[wBase + ${r}u * rowWords + 1u]);
      let d0 = ${i8x4("w0")};
      let d1 = ${i8x4("w1")};
      let s = q8s(scBase + ${r}u * nb);
      ${cols.map((m) => `a${r}_${m} += s * (dot(d0, xa${m}) + dot(d1, xb${m}));`).join(" ")}
    }` : `if (full || row0 + ${r}u < dOut) {
      let wa = mv_w4[off4 + ${r}u * rowWords]; let wb = mv_w4[off4 + ${r}u * rowWords + 1u];
      ${cols.map((m) => `a${r}_${m} += dot(wa, xa${m}) + dot(wb, xb${m});`).join(" ")}
    }`;
  return `
@compute @workgroup_size(${WGB})
fn matvec${kind}_coop_b${C === COLS ? "" : C}${ACC ? "_acc" : ""}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x;
  let qt = t & 3u;
  let bl = t >> 2u;
  let dIn = ${shp}.dIn;
  let dOut = ${shp}.dOut;
  let xs4 = ${shp}.xs4;
  let ys = ${shp}.ys;
  let nb = dIn / 32u;
  let rowWords = ${kind === "_q4" ? "dIn / 8u" : "dIn / 4u"};
  let row0 = (wg.y * 32768u + wg.x) * ${RB}u;
  let full = row0 + ${RB - 1}u < dOut;
  ${rows.map((r) => cols.map((m) => `var a${r}_${m} = 0.0;`).join(" ")).join("\n  ")}
  for (var b: u32 = bl; b < nb; b += ${LANESB}u) {
    ${xLoads}
    ${idx}
    ${rows.map(rowBlock).join("\n    ")}
  }
${Array.from({ length: C / 2 }, (_, h) => h).map((h) => `
  ${rows.map((r) => `mvb_part[${r * WGB}u + t] = a${r}_${2 * h}; mvb_part[${(RB + r) * WGB}u + t] = a${r}_${2 * h + 1};`).join(" ")}
  workgroupBarrier();
  {
    var stride: u32 = ${WGB / 2}u;
    while (stride > 0u) {
      if (t < stride) { ${Array.from({ length: 2 * RB }, (_, k) => `mvb_part[${k * WGB}u + t] += mvb_part[${k * WGB}u + t + stride];`).join(" ")} }
      workgroupBarrier();
      stride = stride >> 1u;
    }
  }
  if (t < ${2 * RB}u) {
    let r = t % ${RB}u;
    let m = ${2 * h}u + t / ${RB}u;
    let row = row0 + r;
    if (row < dOut) { ${ybuf}[m * ys + row] ${ACC ? "+=" : "="} mvb_part[t * ${WGB}u]; }
  }
  workgroupBarrier();`).join("")}
}`;
}).join("\n")).join("\n")).join("\n")}
${guAll}
`;
}

