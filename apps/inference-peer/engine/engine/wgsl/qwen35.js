// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/qwen35.js (MIT; see LICENSE-ENGINE).
// WGSL for the hybrid Qwen 3.5/3.8 engine: Gated-DeltaNet recurrence (single and
// multi-column, with speculative snapshot slots), gated attention glue, fused pre-pass,
// and the logits argmax. Base kernels come from ./base.js; GEMVs from ./coop.js.
// The one-token DeltaNet column update (thread j owns state column j of its value head) as two sweeps over
// the state in global memory instead of 128 registers: sweep 1 loads the column in chunks of B rows and runs the
// three dot-product chains (vh, sq, kq), sweep 2 loads it again, decays it and adds k * d. Every value goes through
// the same expressions in the same order as the register version (sd = s * decay; the chains add in row order;
// s' = sd + k * d), so the state and the output are bit-identical. The register version wrote 128 loads as one
// straight line, which the GB10 (Vulkan) runs nearly serially: 39 us for the 32 heads in isolation against 14 us
// for two chunked sweeps (the second reads hit L2). Uses `decay`, `d` and declared vh / sq / kq.
export function dnTwoPass(S, K, Q, Sb = "Sb", j = "j", B = 16) {
  const r = Array.from({ length: B }, (_, x) => x);
  const ld = r.map((x) => `let a${x} = ${S}[${Sb} + (i + ${x}u) * 128u + ${j}];`).join(" ");
  return {
    pass1: `for (var i: u32 = 0u; i < 128u; i += ${B}u) {
    ${ld}
    ${r.map((x) => `{ let sd = a${x} * decay; vh += sd * ${K}[i + ${x}u]; sq += sd * ${Q}[i + ${x}u]; kq += ${K}[i + ${x}u] * ${Q}[i + ${x}u]; }`).join("\n    ")}
  }`,
    pass2: `for (var i: u32 = 0u; i < 128u; i += ${B}u) {
    ${ld}
    ${r.map((x) => `{ let sd = a${x} * decay; ${S}[${Sb} + (i + ${x}u) * 128u + ${j}] = sd + ${K}[i + ${x}u] * d; }`).join("\n    ")}
  }`,
  };
}

// Register-resident single-token dn_delta (decode): the same transform as dn_delta_mc below, for
// the one-column bindings. Same operations in the same order as the kernel it replaces, so the
// state and output are bit-identical. Requires dState = 128.
function dnDelta1RegsWGSL() {
  const tp = dnTwoPass("dl_s", "dl1_k", "dl1_q");
  return `
var<workgroup> dl1_k: array<f32, 128>;
var<workgroup> dl1_q: array<f32, 128>;
@compute @workgroup_size(128)
fn dn_delta(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let j = lid.x;
  let kh = h % dl_dn.nKH;
  let kOff = kh * 128u; let vOff = h * 128u; let Sb = h * 16384u;
  let decay = dl_decay[h];
  let scale = inverseSqrt(f32(dl_dn.dState));
  dl1_k[j] = dl_k[kOff + j]; dl1_q[j] = dl_q[kOff + j];
  workgroupBarrier();
  var vh: f32 = 0.0; var sq: f32 = 0.0; var kq: f32 = 0.0;
  ${tp.pass1}
  let d = (dl_v[vOff + j] - vh) * dl_beta[h];
  ${tp.pass2}
  dl_o[vOff + j] = (sq + d * kq) * scale;
}`;
}

// dn_delta + dn_gatenorm for one token in one dispatch: both use one 128-thread workgroup per
// value head, so the head's output stays in the workgroup and the gated norm (same tree
// reduction, same expression) follows a barrier. Bit-identical to the two kernels; q/k/v are
// read from the whole conv output (q | k | v), so the kernel fits in 7 storage bindings.
function dnDeltaGnWGSL() {
  const tp = dnTwoPass("dg_s", "dg1_k", "dg1_q");
  return `
@group(1) @binding(0) var<storage, read> dg_c: array<f32>;      // conv output [q | k | v]
@group(1) @binding(1) var<storage, read> dg_beta: array<f32>;
@group(1) @binding(2) var<storage, read> dg_decay: array<f32>;
@group(1) @binding(3) var<storage, read_write> dg_s: array<f32>;
@group(1) @binding(4) var<storage, read> dg_z: array<f32>;
@group(1) @binding(5) var<storage, read> dg_w: array<f32>;
@group(1) @binding(6) var<storage, read_write> dg_y: array<f32>;
@group(1) @binding(7) var<uniform> dg_dn: DN;
var<workgroup> dg1_k: array<f32, 128>;
var<workgroup> dg1_q: array<f32, 128>;
var<workgroup> dg_partial: array<f32, 128>;
@compute @workgroup_size(128)
fn dn_delta_gn(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let j = lid.x;
  let kh = h % dg_dn.nKH;
  let kOff = kh * 128u; let vOff = h * 128u; let Sb = h * 16384u;
  let decay = dg_decay[h];
  let scale = inverseSqrt(f32(dg_dn.dState));
  dg1_k[j] = dg_c[dg_dn.keyDim + kOff + j]; dg1_q[j] = dg_c[kOff + j];
  workgroupBarrier();
  var vh: f32 = 0.0; var sq: f32 = 0.0; var kq: f32 = 0.0;
  ${tp.pass1}
  let d = (dg_c[2u * dg_dn.keyDim + vOff + j] - vh) * dg_beta[h];
  ${tp.pass2}
  let o = (sq + d * kq) * scale;
  dg_partial[j] = o * o;
  workgroupBarrier();
  var stride: u32 = 64u;
  while (stride > 0u) {
    if (j < stride) { dg_partial[j] += dg_partial[j + stride]; }
    workgroupBarrier();
    stride = stride / 2u;
  }
  let inv = inverseSqrt(dg_partial[0] / f32(dg_dn.dState) + cfg.eps);
  let z = dg_z[vOff + j];
  dg_y[vOff + j] = o * inv * dg_w[j] * (z / (1.0 + exp(-z)));
}`;
}

// attn_flash (f16 KV) and attn_flash_q8 (int8 KV, one f32 scale per 32 values, llama.cpp q8_0
// style) from one template: same structure and arithmetic order, only the K/V reads differ.
function flashWGSL(q8) {
  const P = q8 ? "fq" : "fa";
  const binds = q8
    ? `@group(1) @binding(0) var<storage, read> ${P}_q: array<f32>;
@group(1) @binding(1) var<storage, read> ${P}_k: array<u32>;      // 4 int8 per word
@group(1) @binding(2) var<storage, read> ${P}_v: array<u32>;
@group(1) @binding(3) var<storage, read> ${P}_ks: array<f32>;     // one scale per 32 values
@group(1) @binding(4) var<storage, read> ${P}_vs: array<f32>;
@group(1) @binding(5) var<storage, read_write> ${P}_o: array<f32>;
@group(1) @binding(6) var<storage, read_write> ${P}_ml: array<f32>;
@group(1) @binding(7) var<uniform> ${P}: FA;`
    : `@group(1) @binding(0) var<storage, read> ${P}_q: array<f32>;
@group(1) @binding(1) var<storage, read> ${P}_k: array<u32>;      // f16 pairs
@group(1) @binding(2) var<storage, read> ${P}_v: array<u32>;
@group(1) @binding(3) var<storage, read_write> ${P}_o: array<f32>;
@group(1) @binding(4) var<storage, read_write> ${P}_ml: array<f32>;
@group(1) @binding(5) var<uniform> ${P}: FA;`;
  const dot = q8
    ? `let kb = (c0 + t) * (cfg.kvDim / 4u) + kvh * (hd / 4u);
        let sb = (c0 + t) * (cfg.kvDim / 32u) + kvh * (hd / 32u);
        let qb = h * hd;
        var s: f32 = 0.0;
        for (var p: u32 = 0u; p < hd / 4u; p++) {
          let w = ${P}_k[kb + p]; let sc = ${P}_ks[sb + p / 8u];
          s += fa_qs[qb + 4u * p] * (f32(bitcast<i32>(w << 24u) >> 24u) * sc);
          s += fa_qs[qb + 4u * p + 1u] * (f32(bitcast<i32>(w << 16u) >> 24u) * sc);
          s += fa_qs[qb + 4u * p + 2u] * (f32(bitcast<i32>(w << 8u) >> 24u) * sc);
          s += fa_qs[qb + 4u * p + 3u] * (f32(bitcast<i32>(w) >> 24u) * sc);
        }`
    : `let kb = (c0 + t) * kvw + kvh * hw;
        let qb = h * hd;
        var s: f32 = 0.0;
        for (var p: u32 = 0u; p < hw; p++) {
          let kk = unpack2x16float(${P}_k[kb + p]);
          s += fa_qs[qb + 2u * p] * kk.x;
          s += fa_qs[qb + 2u * p + 1u] * kk.y;
        }`;
  const vread = q8
    ? `let vw = ${P}_v[(c0 + t) * (cfg.kvDim / 4u) + kvh * (hd / 4u) + tid / 4u];
        let v = f32(bitcast<i32>(vw << (24u - 8u * (tid & 3u))) >> 24u) * ${P}_vs[(c0 + t) * (cfg.kvDim / 32u) + (kvh * hd + tid) / 32u];`
    : `let v = unpack2x16float(${P}_v[(c0 + t) * kvw + kvh * hw + tid / 2u])[tid & 1u];`;
  return `
${binds}
@compute @workgroup_size(256)
fn attn_flash${q8 ? "_q8" : ""}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let sp = wg.x; let col = wg.y; let g = wg.z; let tid = lid.x;
  let seqLen = frame.seqLen + col;
  let t0 = sp * ${P}.splitLen;
  if (t0 >= seqLen) { return; }
  let t1 = min(seqLen, t0 + ${P}.splitLen);
  // more than 8 query heads per kv head (Qwen3.5-122B: 16): R workgroups per kv head, each takes G = 8 of them
  // (wg.z runs over nKV * R); R = 1 (G <= 8) is the original kernel
  let GA = cfg.nH / cfg.nKV; let R = (GA + 7u) / 8u;
  let hd = cfg.headDim; let G = GA / R; let kvh = g / R;
  let hw = hd / 2u; let kvw = cfg.kvDim / 2u;
  let rs = sqrt(f32(hd));
  for (var w: u32 = tid; w < G * hd; w += 256u) { fa_qs[w] = ${P}_q[col * ${P}.s0 + g * G * hd + w]; }
  if (tid < G) { fa_m[tid] = -3.0e38; fa_l[tid] = 0.0; }
  var acc: array<f32, 8>;
  for (var h: u32 = 0u; h < 8u; h++) { acc[h] = 0.0; }
  workgroupBarrier();
  for (var c0: u32 = t0; c0 < t1; c0 += 64u) {
    let n = min(64u, t1 - c0);
    for (var w: u32 = tid; w < G * 64u; w += 256u) {
      let h = w / 64u; let t = w % 64u;
      if (t < n) {
        ${dot}
        fa_sc[w] = s / rs;
      }
    }
    workgroupBarrier();
    if (tid < G) {
      let b = tid * 64u;
      var cm = fa_m[tid];
      for (var t: u32 = 0u; t < n; t++) { cm = max(cm, fa_sc[b + t]); }
      let alpha = exp(fa_m[tid] - cm);
      var l = fa_l[tid] * alpha;
      for (var t: u32 = 0u; t < n; t++) { let e = exp(fa_sc[b + t] - cm); fa_sc[b + t] = e; l += e; }
      fa_m[tid] = cm; fa_l[tid] = l; fa_a[tid] = alpha;
    }
    workgroupBarrier();
    if (tid < hd) {
      for (var h: u32 = 0u; h < G; h++) { acc[h] *= fa_a[h]; }
      for (var t: u32 = 0u; t < n; t++) {
        ${vread}
        for (var h: u32 = 0u; h < G; h++) { acc[h] += fa_sc[h * 64u + t] * v; }
      }
    }
    workgroupBarrier();
  }
  if (tid < hd) {
    for (var h: u32 = 0u; h < G; h++) { ${P}_o[((col * cfg.nH + g * G + h) * ${P}.maxSplits + sp) * hd + tid] = acc[h]; }
  }
  if (tid < G) {
    let b = (col * cfg.nH + g * G + tid) * ${P}.maxSplits + sp;
    ${P}_ml[b * 2u] = fa_m[tid]; ${P}_ml[b * 2u + 1u] = fa_l[tid];
  }
}`;
}

// Register-resident dn_delta_mc (docs/deltanet-prefill-spec.md, RG=1): thread j keeps column j
// of the head's state S in 128 registers for the whole pass (loaded once, stored once) instead of
// two read-modify-write sweeps of global memory per column; q/k of each column are staged in
// workgroup memory. Same operations in the same order as the kernel it replaces, so S, the
// outputs and the snapshots are bit-identical (measured on the GB10 in isolation: 1.24x at 1
// column, 2.03x at 16). Every private-array index is a literal (codegen-unrolled): a loop over a
// constant bound does not unroll on every backend and spills the array. Requires dState = 128.
function dnDeltaRegsWGSL() {
  const rows = Array.from({ length: 128 }, (_, i) => i);
  const load = rows.map((i) => `s[${i}u] = dlm_s[Sb + ${i * 128}u + j];`).join(" ");
  const store = rows.map((i) => `dlm_s[Sb + ${i * 128}u + j] = s[${i}u];`).join(" ");
  const shadow = rows.map((i) => `dlm_shadow[so + ${i * 128}u] = s[${i}u];`).join(" ");
  const loadPre = rows.map((i) => `s[${i}u] = dlm_c[Sb + ${i * 128}u + j];`).join(" ");
  const storePre = rows.map((i) => `dlm_shadow[Sb + ${i * 128}u + j] = s[${i}u];`).join(" ");
  const loop1 = rows.map((i) => `{ let sd = s[${i}u] * decay; s[${i}u] = sd; vh += sd * dlr_k[${i}u]; sq += sd * dlr_q[${i}u]; kq += dlr_k[${i}u] * dlr_q[${i}u]; }`).join("\n      ");
  const loop2 = rows.map((i) => `s[${i}u] += dlr_k[${i}u] * d;`).join(" ");
  return `
var<workgroup> dlr_k: array<f32, 128>;
var<workgroup> dlr_q: array<f32, 128>;
@compute @workgroup_size(128)
fn dn_delta_mc(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let j = lid.x;
  let kh = h % dlm_dn.nKH;
  let kOff = kh * 128u; let vOff = h * 128u; let Sb = h * 16384u;
  let scale = inverseSqrt(f32(dlm_dn.dState));   // a uniform read, as before (a literal changes rounding)
  let nCols = max(frame.nCols, 1u);
  let sSize = dlm_dn.nVH * 16384u;
  var s: array<f32, 128>;
  // Replay rollback. L.rp = [S_pre | cap columns of conv | of beta | of decay] (cap = mc.n).
  // A verify pass (snap bit 31 + slot base) records into it through dlm_shadow: the state it
  // starts from (first chunk) and exactly the inputs each column reads, at slot base + col, so
  // no copies are needed around the pass. _restoreDN (bit 30) binds L.rp as dlm_c / dlm_beta /
  // dlm_decay (read-only) and replays the accepted columns from S_pre.
  let dlSB = frame.snap & 0xffu;               // snapshot slot base + 1 (0 = off)
  let rec = dlSB != 0u && (frame.snap & 0x80000000u) != 0u;
  let rpv = (frame.snap & 0x40000000u) != 0u;
  let cap = dlm_mc.n;
  let rG = sSize + cap * dlm_mc.s0; let rD = rG + cap * dlm_mc.s1;   // beta / decay regions of L.rp
  let cB = select(0u, sSize, rpv); let gB = select(0u, rG, rpv); let dB = select(0u, rD, rpv);
  if (rpv) { ${loadPre} } else { ${load} }
  if (rec && dlSB == 1u) { ${storePre} }
  for (var col: u32 = 0u; col < nCols; col++) {
    let qo = cB + col * dlm_mc.s0 + kOff;
    let ko = cB + col * dlm_mc.s0 + dlm_dn.keyDim + kOff;
    let vo = cB + col * dlm_mc.s0 + 2u * dlm_dn.keyDim + vOff;
    workgroupBarrier();                          // the previous column is done reading k/q
    dlr_k[j] = dlm_c[ko + j]; dlr_q[j] = dlm_c[qo + j];
    workgroupBarrier();
    let decay = dlm_decay[dB + col * dlm_mc.s1 + h];
    let beta = dlm_beta[gB + col * dlm_mc.s1 + h];
    if (rec) {
      let slot = dlSB - 1u + col; let rc = sSize + slot * dlm_mc.s0;
      dlm_shadow[rc + 2u * dlm_dn.keyDim + vOff + j] = dlm_c[vo + j];
      if (h < dlm_dn.nKH) { dlm_shadow[rc + kOff + j] = dlr_q[j]; dlm_shadow[rc + dlm_dn.keyDim + kOff + j] = dlr_k[j]; }
      if (j == 0u) { dlm_shadow[rG + slot * dlm_mc.s1 + h] = beta; dlm_shadow[rD + slot * dlm_mc.s1 + h] = decay; }
    }
    var vh: f32 = 0.0; var sq: f32 = 0.0; var kq: f32 = 0.0;
      ${loop1}
    let d = (dlm_c[vo + j] - vh) * beta;
    ${loop2}
    dlm_o[col * dlm_mc.s2 + vOff + j] = (sq + d * kq) * scale;
    // bit 31: replay rollback, no state snapshots
    if (dlSB != 0u && (frame.snap & 0x80000000u) == 0u && dlSB + col < ((frame.snap >> 8u) & 0xffu)) {
      let so = (dlSB - 1u + col) * sSize + Sb + j;
      ${shadow}
    }
  }
  ${store}
}`;
}

export const WGSL2 = /* wgsl */ `
struct DN {
  convDim: u32, dState: u32, nKH: u32, nVH: u32,
  keyDim: u32, nRot: u32, hd: u32, dInner: u32,
  ropeTheta: f32, eps2: f32, pad0: u32, pad1: u32,
};

// --- causal conv (K=4) + silu; updates rolling state ---
@group(1) @binding(0) var<storage, read> cv_x: array<f32>;      // qkv_mixed [convDim]
@group(1) @binding(1) var<storage, read> cv_w: array<f32>;      // [convDim, 4]
@group(1) @binding(2) var<storage, read_write> cv_st: array<f32>; // [convDim, 3]
@group(1) @binding(3) var<storage, read_write> cv_y: array<f32>;  // [convDim]
@group(1) @binding(4) var<uniform> cv_dn: DN;
@compute @workgroup_size(64)
fn dn_conv(@builtin(global_invocation_id) gid: vec3<u32>) {
  let c = gid.x;
  if (c >= cv_dn.convDim) { return; }
  let x = cv_x[c];
  var acc = cv_w[c * 4u + 3u] * x;
  acc += cv_w[c * 4u + 0u] * cv_st[c * 3u + 0u];
  acc += cv_w[c * 4u + 1u] * cv_st[c * 3u + 1u];
  acc += cv_w[c * 4u + 2u] * cv_st[c * 3u + 2u];
  cv_y[c] = acc / (1.0 + exp(-acc));
  cv_st[c * 3u + 0u] = cv_st[c * 3u + 1u];
  cv_st[c * 3u + 1u] = cv_st[c * 3u + 2u];
  cv_st[c * 3u + 2u] = x;
}

// --- per-head gates: beta = sigmoid(betaRaw); decay = exp(softplus(alpha+dt)*A) ---
@group(1) @binding(0) var<storage, read> gt_alpha: array<f32>;
@group(1) @binding(1) var<storage, read> gt_beta: array<f32>;
@group(1) @binding(2) var<storage, read> gt_dt: array<f32>;
@group(1) @binding(3) var<storage, read> gt_a: array<f32>;
@group(1) @binding(4) var<storage, read_write> gt_bout: array<f32>;
@group(1) @binding(5) var<storage, read_write> gt_dout: array<f32>;
@group(1) @binding(6) var<uniform> gt_dn: DN;
@compute @workgroup_size(64)
fn dn_gates(@builtin(global_invocation_id) gid: vec3<u32>) {
  let h = gid.x;
  if (h >= gt_dn.nVH) { return; }
  gt_bout[h] = 1.0 / (1.0 + exp(-gt_beta[h]));
  let av = gt_alpha[h] + gt_dt[h];
  var sp: f32;
  if (av > 20.0) { sp = av; } else { sp = log(1.0 + exp(av)); }
  gt_dout[h] = exp(sp * gt_a[h]);
}

// --- per-head L2 norm (q/k slices bound with offset) ---
@group(1) @binding(0) var<storage, read_write> l2_v: array<f32>;
@group(1) @binding(1) var<uniform> l2_heads: u32;
@group(1) @binding(2) var<uniform> l2_dn: DN;
@compute @workgroup_size(32)
fn dn_l2(@builtin(global_invocation_id) gid: vec3<u32>) {
  let h = gid.x;
  if (h >= l2_heads) { return; }
  let off = h * l2_dn.dState;
  var ss: f32 = 0.0;
  for (var i: u32 = 0u; i < l2_dn.dState; i++) { let v = l2_v[off + i]; ss += v * v; }
  let inv = 1.0 / max(sqrt(ss), l2_dn.eps2);
  for (var i: u32 = 0u; i < l2_dn.dState; i++) { l2_v[off + i] *= inv; }
}

// --- gated delta rule: one workgroup per v-head, thread j = state column ---
@group(1) @binding(0) var<storage, read> dl_q: array<f32>;      // [keyDim] (l2-normed)
@group(1) @binding(1) var<storage, read> dl_k: array<f32>;      // [keyDim]
@group(1) @binding(2) var<storage, read> dl_v: array<f32>;      // [dInner]
@group(1) @binding(3) var<storage, read> dl_beta: array<f32>;   // [nVH]
@group(1) @binding(4) var<storage, read> dl_decay: array<f32>;  // [nVH]
@group(1) @binding(5) var<storage, read_write> dl_s: array<f32>; // [nVH, dState, dState]
@group(1) @binding(6) var<storage, read_write> dl_o: array<f32>; // [dInner]
@group(1) @binding(7) var<uniform> dl_dn: DN;
${dnDelta1RegsWGSL()}
${dnDeltaGnWGSL()}

// --- gated norm: rmsnorm per head (w[dState]) * silu(z) ---
@group(1) @binding(0) var<storage, read> gn_x: array<f32>;   // [dInner]
@group(1) @binding(1) var<storage, read> gn_z: array<f32>;   // [dInner]
@group(1) @binding(2) var<storage, read> gn_w: array<f32>;   // [dState]
@group(1) @binding(3) var<storage, read_write> gn_y: array<f32>;
@group(1) @binding(4) var<uniform> gn_dn: DN;
var<workgroup> gn_partial: array<f32, 128>;
@compute @workgroup_size(128)
fn dn_gatenorm(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x;
  let j = lid.x;
  let off = h * gn_dn.dState;
  let v = gn_x[off + j];
  gn_partial[j] = v * v;
  workgroupBarrier();
  var stride: u32 = 64u;
  while (stride > 0u) {
    if (j < stride) { gn_partial[j] += gn_partial[j + stride]; }
    workgroupBarrier();
    stride = stride / 2u;
  }
  let inv = inverseSqrt(gn_partial[0] / f32(gn_dn.dState) + cfg.eps);
  let z = gn_z[off + j];
  gn_y[off + j] = gn_x[off + j] * inv * gn_w[j] * (z / (1.0 + exp(-z)));
}

// --- split interleaved [q|gate] per head from q_full ---
@group(1) @binding(0) var<storage, read> qs_full: array<f32>;  // [nH*2*hd]
@group(1) @binding(1) var<storage, read_write> qs_q: array<f32>;
@group(1) @binding(2) var<storage, read_write> qs_g: array<f32>;
@group(1) @binding(3) var<uniform> qs_dn: DN;
@compute @workgroup_size(64)
fn qsplit(@builtin(global_invocation_id) gid: vec3<u32>) {
  let idx = gid.x;
  let total = cfg.nH * qs_dn.hd;
  if (idx >= total) { return; }
  let h = idx / qs_dn.hd;
  let i = idx % qs_dn.hd;
  qs_q[idx] = qs_full[h * 2u * qs_dn.hd + i];
  qs_g[idx] = qs_full[h * 2u * qs_dn.hd + qs_dn.hd + i];
}

// --- partial neox rope: rotate first nRot dims of each head ---
@group(1) @binding(0) var<storage, read_write> rp2_v: array<f32>;
@group(1) @binding(1) var<uniform> rp2_heads: u32;
@group(1) @binding(2) var<uniform> rp2_dn: DN;
@compute @workgroup_size(64)
fn rope_part(@builtin(global_invocation_id) gid: vec3<u32>) {
  let half = rp2_dn.nRot / 2u;
  let total = rp2_heads * half;
  let idx = gid.x;
  if (idx >= total) { return; }
  let h = idx / half;
  let i = idx % half;
  let off = h * rp2_dn.hd;
  let ang = f32(frame.pos) * pow(rp2_dn.ropeTheta, -f32(2u * i) / f32(rp2_dn.nRot));
  let c = cos(ang); let s = sin(ang);
  let a = rp2_v[off + i]; let b = rp2_v[off + i + half];
  rp2_v[off + i] = a * c - b * s;
  rp2_v[off + i + half] = b * c + a * s;
}

// --- a *= sigmoid(g) ---
@group(1) @binding(0) var<storage, read_write> sm_a: array<f32>;
@group(1) @binding(1) var<storage, read> sm_g: array<f32>;
@group(1) @binding(2) var<uniform> sm_n: u32;
@compute @workgroup_size(64)
fn sigmoid_mul(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x;
  if (i >= sm_n) { return; }
  let g = sm_g[i];
  sm_a[i] = sm_a[i] * (1.0 / (1.0 + exp(-g)));
}
// ================= multi-column variants (batched prefill / verify) =================
// One dispatch covers every live column: y = column for the parallel ops; the
// recurrent ops (conv, delta rule) loop over columns inside the kernel and
// write rollback snapshots when frame.snap is set. Math is identical to the
// single-column kernels above (same operation order).
struct MC { n: u32, s0: u32, s1: u32, s2: u32 };

@group(1) @binding(0) var<storage, read> rnm_x: array<f32>;
@group(1) @binding(1) var<storage, read> rnm_w: array<f32>;
@group(1) @binding(2) var<storage, read_write> rnm_y: array<f32>;
@group(1) @binding(3) var<uniform> rnm_mc: MC;          // n, x stride, y stride
var<workgroup> rnm_partial: array<f32, 256>;
@compute @workgroup_size(256)
fn rmsnorm_mc(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let n = rnm_mc.n;
  let xo = wg.y * rnm_mc.s0; let yo = wg.y * rnm_mc.s1;
  var ss: f32 = 0.0;
  for (var i: u32 = t; i < n; i += 256u) { let v = rnm_x[xo + i]; ss += v * v; }
  rnm_partial[t] = ss;
  workgroupBarrier();
  var stride: u32 = 128u;
  while (stride > 0u) {
    if (t < stride) { rnm_partial[t] += rnm_partial[t + stride]; }
    workgroupBarrier();
    stride = stride / 2u;
  }
  let inv = inverseSqrt(rnm_partial[0] / f32(n) + cfg.eps);
  for (var i: u32 = t; i < n; i += 256u) { rnm_y[yo + i] = rnm_x[xo + i] * inv * rnm_w[i]; }
}

@group(1) @binding(0) var<storage, read_write> adm_a: array<f32>;
@group(1) @binding(1) var<storage, read> adm_b: array<f32>;
@group(1) @binding(2) var<uniform> adm_mc: MC;          // n, a stride, b stride
@compute @workgroup_size(64)
fn add_res_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x;
  if (i >= adm_mc.n) { return; }
  adm_a[gid.y * adm_mc.s0 + i] += adm_b[gid.y * adm_mc.s1 + i];
}

@group(1) @binding(0) var<storage, read> gtm_alpha: array<f32>;
@group(1) @binding(1) var<storage, read> gtm_beta: array<f32>;
@group(1) @binding(2) var<storage, read> gtm_dt: array<f32>;
@group(1) @binding(3) var<storage, read> gtm_a: array<f32>;
@group(1) @binding(4) var<storage, read_write> gtm_bout: array<f32>;
@group(1) @binding(5) var<storage, read_write> gtm_dout: array<f32>;
@group(1) @binding(6) var<uniform> gtm_mc: MC;          // n = heads, s0 = column stride (all four)
@compute @workgroup_size(64)
fn dn_gates_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let h = gid.x;
  if (h >= gtm_mc.n) { return; }
  let o = gid.y * gtm_mc.s0;
  gtm_bout[o + h] = 1.0 / (1.0 + exp(-gtm_beta[o + h]));
  let av = gtm_alpha[o + h] + gtm_dt[h];
  var sp: f32;
  if (av > 20.0) { sp = av; } else { sp = log(1.0 + exp(av)); }
  gtm_dout[o + h] = exp(sp * gtm_a[h]);
}

@group(1) @binding(0) var<storage, read> cvm_x: array<f32>;
@group(1) @binding(1) var<storage, read> cvm_w: array<f32>;
@group(1) @binding(2) var<storage, read_write> cvm_st: array<f32>;
@group(1) @binding(3) var<storage, read_write> cvm_y: array<f32>;
@group(1) @binding(4) var<uniform> cvm_mc: MC;          // n = convDim, x stride, y stride
@group(1) @binding(5) var<storage, read_write> cvm_shadow: array<f32>;   // [max(7, maxDrafts)][convDim*3]
@compute @workgroup_size(64)
fn dn_conv_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let c = gid.x; let n = cvm_mc.n;
  if (c >= n) { return; }
  let w0 = cvm_w[c * 4u]; let w1 = cvm_w[c * 4u + 1u]; let w2 = cvm_w[c * 4u + 2u]; let w3 = cvm_w[c * 4u + 3u];
  var s0 = cvm_st[c * 3u]; var s1 = cvm_st[c * 3u + 1u]; var s2 = cvm_st[c * 3u + 2u];
  let nCols = max(frame.nCols, 1u);
  for (var col: u32 = 0u; col < nCols; col++) {
    let x = cvm_x[col * cvm_mc.s0 + c];
    var acc = w3 * x;
    acc += w0 * s0;
    acc += w1 * s1;
    acc += w2 * s2;
    cvm_y[col * cvm_mc.s1 + c] = acc / (1.0 + exp(-acc));
    s0 = s1; s1 = s2; s2 = x;
    let cvSB = frame.snap & 0xffu;       // snapshot slot base + 1 (0 = off)
    if (cvSB != 0u && cvSB + col < ((frame.snap >> 8u) & 0xffu)) {
      let so = (cvSB - 1u + col) * n * 3u + c * 3u;
      cvm_shadow[so] = s0; cvm_shadow[so + 1u] = s1; cvm_shadow[so + 2u] = s2;
    }
  }
  cvm_st[c * 3u] = s0; cvm_st[c * 3u + 1u] = s1; cvm_st[c * 3u + 2u] = s2;
}

@group(1) @binding(0) var<storage, read_write> l2m_v: array<f32>;
@group(1) @binding(1) var<uniform> l2m_mc: MC;          // n = heads, s0 = column stride, s1 = part offset (z = 1 -> k)
@group(1) @binding(2) var<uniform> l2m_dn: DN;
@compute @workgroup_size(32)
fn dn_l2_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let h = gid.x;
  if (h >= l2m_mc.n) { return; }
  let off = gid.y * l2m_mc.s0 + gid.z * l2m_mc.s1 + h * l2m_dn.dState;
  var ss: f32 = 0.0;
  for (var i: u32 = 0u; i < l2m_dn.dState; i++) { let v = l2m_v[off + i]; ss += v * v; }
  let inv = 1.0 / max(sqrt(ss), l2m_dn.eps2);
  for (var i: u32 = 0u; i < l2m_dn.dState; i++) { l2m_v[off + i] *= inv; }
}

@group(1) @binding(0) var<storage, read> dlm_c: array<f32>;      // conv output columns: [q | k | v]
@group(1) @binding(1) var<storage, read> dlm_beta: array<f32>;
@group(1) @binding(2) var<storage, read> dlm_decay: array<f32>;
@group(1) @binding(3) var<storage, read_write> dlm_s: array<f32>;
@group(1) @binding(4) var<storage, read_write> dlm_o: array<f32>;
@group(1) @binding(5) var<uniform> dlm_mc: MC;          // s0 conv stride, s1 gate stride, s2 out stride
@group(1) @binding(6) var<uniform> dlm_dn: DN;
@group(1) @binding(7) var<storage, read_write> dlm_shadow: array<f32>;   // [7][nVH*dState*dState]; replay: L.rp
${dnDeltaRegsWGSL()}

// _restoreDN: conv state <- its snapshot after verify column frame.nCols - 1 (a compute copy in
// the replay pass instead of a blit per layer)
@group(1) @binding(0) var<storage, read> cvr_src: array<f32>;       // conv_shadow
@group(1) @binding(1) var<storage, read_write> cvr_st: array<f32>;  // convState [convDim*3]
@group(1) @binding(2) var<uniform> cvr_mc: MC;                      // n = convDim * 3
@compute @workgroup_size(64)
fn dn_conv_restore(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x;
  if (i >= cvr_mc.n) { return; }
  cvr_st[i] = cvr_src[(max(frame.nCols, 1u) - 1u) * cvr_mc.n + i];
}

// --- draft chain: the embedding row of the token the last argmax picked, dequantized on the GPU
// exactly as the host's _embedRowF32 does (f16 scale x small integer: exact in f32), so K drafts
// can run in one submit instead of K round trips through the CPU ---
@group(1) @binding(0) var<storage, read> eg_qs: array<u32>;
@group(1) @binding(1) var<storage, read> eg_sc: array<u32>;
@group(1) @binding(2) var<storage, read> eg_id: array<u32>;
@group(1) @binding(3) var<storage, read_write> eg_out: array<f32>;
@group(1) @binding(4) var<uniform> eg_u: vec4<u32>;       // dim, kind (0 q4, 1 q8), rows
// f16 bits -> f32 by hand (not unpack2x16float, which may flush f16 subnormals): the same value
// the host's f16ToF32 returns for every finite input, subnormals and -0 included. The speculative
// one-submit verify (engine specFuse) feeds these rows to the trunk, so they must match the host's.
fn eg_f16(h: u32) -> f32 {
  let e = (h >> 10u) & 0x1Fu; let m = h & 0x3FFu;
  var v: f32;
  if (e == 0u) { v = ldexp(f32(m), -24); }                       // subnormal: m * 2^-24, exact in f32
  else if (e == 31u) { v = bitcast<f32>(0x7F800000u | (m << 13u)); }
  else { v = bitcast<f32>(((e + 112u) << 23u) | (m << 13u)); }
  return select(v, -v, (h & 0x8000u) != 0u);
}
@compute @workgroup_size(64)
fn emb_gather(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x; let dim = eg_u.x;
  if (i >= dim) { return; }
  let id = min(eg_id[0], eg_u.z - 1u);
  let nb = dim / 32u; let b = i / 32u; let e = i % 32u;
  let si = id * nb + b;
  let s = eg_f16((eg_sc[si >> 1u] >> ((si & 1u) * 16u)) & 0xFFFFu);
  var q: f32;
  if (eg_u.y == 0u) {
    let bi = si * 16u + (e % 16u);
    let by = (eg_qs[bi >> 2u] >> ((bi & 3u) * 8u)) & 0xFFu;
    q = f32(i32(select(by & 0xFu, by >> 4u, e >= 16u)) - 8);
  } else {
    let bi = si * 32u + e;
    let by = (eg_qs[bi >> 2u] >> ((bi & 3u) * 8u)) & 0xFFu;
    q = f32(bitcast<i32>(by << 24u) >> 24u);
  }
  eg_out[i] = s * q;
}

// --- argmax over n floats (single workgroup): out = [index, bitcast(value)] ---
@group(1) @binding(0) var<storage, read> am_x: array<f32>;
@group(1) @binding(1) var<storage, read_write> am_out: array<u32>;
@group(1) @binding(2) var<uniform> am_n: vec4<u32>;
var<workgroup> am_v: array<f32, 256>;
var<workgroup> am_i: array<u32, 256>;
@compute @workgroup_size(256)
fn argmax(@builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let n = am_n.x;
  var bv: f32 = -3.402823e38; var bi: u32 = 0xffffffffu;
  for (var i: u32 = t; i < n; i += 256u) {
    let v = am_x[i];
    if (v > bv || (v == bv && i < bi)) { bv = v; bi = i; }
  }
  am_v[t] = bv; am_i[t] = bi;
  workgroupBarrier();
  for (var s: u32 = 128u; s > 0u; s >>= 1u) {
    if (t < s) {
      let ov = am_v[t + s]; let oi = am_i[t + s];
      if (ov > am_v[t] || (ov == am_v[t] && oi < am_i[t])) { am_v[t] = ov; am_i[t] = oi; }
    }
    workgroupBarrier();
  }
  if (t == 0u) { am_out[0] = am_i[0]; am_out[1] = bitcast<u32>(am_v[0]); }
}

// --- two-stage argmax / top-k over the columns of a logits matrix (GPU sampling) ---
// Sampling needs at most k (value, index) pairs per column, so the logits never leave the GPU.
// Order: value descending, index ascending (the same tie rule as argmax above and the host's
// greedy: the lowest index of the largest value). NaN and -Inf are never picked; every non-finite
// value (NaN, +-Inf) is counted, so the host keeps its NaN check without the logits.
// topk_a: grid (ceil(n / 4096), columns). Each workgroup holds a 4096-logit slice of its column in
//   registers (16 per thread) and runs k selection rounds: round r takes the best element that
//   comes after round r-1's pick in that order (no exclusion list needed). Record per (column,
//   workgroup): k x [idx, bits(value)] then [nonFiniteCount, 0]; stride 2k + 2 u32.
// topk_b: grid (1, columns). k rounds over the nw * k candidates of its column, same order.
//   out[col * (2k + 2) ..] = k x [idx, bits] then [nonFiniteCount, 0]. With k = 1 that is
//   [idx, bits, bad, 0]: the old argmax's 16-byte layout (emb_gather reads idx at [0]).
// Missing entries (fewer than k finite values > -Inf) are idx 0xffffffff; round 0 falls back to
// index 0 when a column has no finite value at all (what the host's greedy returns).
// tk_u: n, column stride (floats), k (1..64), nw.  tb_u: nw, k.
const TK_NONE: u32 = 0xffffffffu;
@group(1) @binding(0) var<storage, read> tk_x: array<f32>;
@group(1) @binding(1) var<storage, read_write> tk_p: array<u32>;
@group(1) @binding(2) var<uniform> tk_u: vec4<u32>;
@group(1) @binding(0) var<storage, read> tb_p: array<u32>;
@group(1) @binding(1) var<storage, read_write> tb_out: array<u32>;
@group(1) @binding(2) var<uniform> tb_u: vec4<u32>;
var<workgroup> tk_v: array<f32, 256>;
var<workgroup> tk_i: array<u32, 256>;
var<workgroup> tk_bad: atomic<u32>;
fn tk_better(v: f32, i: u32, bv: f32, bi: u32) -> bool { return v > bv || (v == bv && i < bi); }
fn tk_after(v: f32, i: u32, lv: f32, li: u32) -> bool { return v < lv || (v == lv && i > li); }
// workgroup-wide best (tk_better) of every thread's (bv, bi); every thread gets [idx, bits]
fn tk_reduce(t: u32, bv: f32, bi: u32) -> vec2<u32> {
  tk_v[t] = bv; tk_i[t] = bi;
  workgroupBarrier();
  for (var s: u32 = 128u; s > 0u; s >>= 1u) {
    if (t < s) {
      let ov = tk_v[t + s]; let oi = tk_i[t + s];
      if (tk_better(ov, oi, tk_v[t], tk_i[t])) { tk_v[t] = ov; tk_i[t] = oi; }
    }
    workgroupBarrier();
  }
  let r = vec2<u32>(tk_i[0], bitcast<u32>(tk_v[0]));
  workgroupBarrier();   // the next round overwrites slot 0
  return r;
}
@compute @workgroup_size(256)
fn topk_a(@builtin(local_invocation_id) lid: vec3<u32>, @builtin(workgroup_id) wid: vec3<u32>) {
  let t = lid.x; let n = tk_u.x; let k = tk_u.z; let nw = tk_u.w;
  let s0 = wid.x * 4096u; let base = wid.y * tk_u.y;
  var xs: array<f32, 16>;
  var bad: u32 = 0u;
  for (var j: u32 = 0u; j < 16u; j++) {
    let i = s0 + j * 256u + t;
    var v: f32 = 0.0;
    if (i < n) {
      v = tk_x[base + i];
      if ((bitcast<u32>(v) & 0x7f800000u) == 0x7f800000u) { bad += 1u; }
    }
    xs[j] = v;
  }
  if (bad > 0u) { atomicAdd(&tk_bad, bad); }
  let rec = (wid.y * nw + wid.x) * (2u * k + 2u);
  var lv: f32 = 0.0; var li: u32 = TK_NONE;
  for (var r: u32 = 0u; r < k; r++) {
    var bv: f32 = -3.402823e38; var bi: u32 = TK_NONE;
    for (var j: u32 = 0u; j < 16u; j++) {
      let i = s0 + j * 256u + t; let v = xs[j];
      if (i < n && (r == 0u || tk_after(v, i, lv, li)) && tk_better(v, i, bv, bi)) { bv = v; bi = i; }
    }
    let w = tk_reduce(t, bv, bi);
    li = w.x; lv = bitcast<f32>(w.y);
    if (t == 0u) { tk_p[rec + 2u * r] = w.x; tk_p[rec + 2u * r + 1u] = w.y; }
  }
  if (t == 0u) { tk_p[rec + 2u * k] = atomicLoad(&tk_bad); tk_p[rec + 2u * k + 1u] = 0u; }
}
@compute @workgroup_size(256)
fn topk_b(@builtin(local_invocation_id) lid: vec3<u32>, @builtin(workgroup_id) wid: vec3<u32>) {
  let t = lid.x; let nw = tb_u.x; let k = tb_u.y; let R = 2u * k + 2u;
  let pb = wid.y * nw * R; let ob = wid.y * R; let m = nw * k;
  var bad: u32 = 0u;
  for (var w: u32 = t; w < nw; w += 256u) { bad += tb_p[pb + w * R + 2u * k]; }
  if (bad > 0u) { atomicAdd(&tk_bad, bad); }
  var lv: f32 = 0.0; var li: u32 = TK_NONE;
  for (var r: u32 = 0u; r < k; r++) {
    var bv: f32 = -3.402823e38; var bi: u32 = TK_NONE;
    // the same per-thread order (e = t, t + 256, ...) with a trip count every thread shares: a loop
    // whose exit depends on t, ahead of tk_reduce's barriers in this round loop, fails under FXC
    for (var e0: u32 = 0u; e0 < m; e0 += 256u) {
      let e = e0 + t;
      if (e < m) {
        let o = pb + (e / k) * R + 2u * (e % k);
        let i = tb_p[o];
        if (i != TK_NONE) {
          let v = bitcast<f32>(tb_p[o + 1u]);
          if ((r == 0u || tk_after(v, i, lv, li)) && tk_better(v, i, bv, bi)) { bv = v; bi = i; }
        }
      }
    }
    let w = tk_reduce(t, bv, bi);
    li = w.x; lv = bitcast<f32>(w.y);
    if (t == 0u) { tb_out[ob + 2u * r] = select(w.x, 0u, r == 0u && w.x == TK_NONE); tb_out[ob + 2u * r + 1u] = w.y; }
  }
  if (t == 0u) { tb_out[ob + 2u * k] = atomicLoad(&tk_bad); tb_out[ob + 2u * k + 1u] = 0u; }
}


// --- fused DeltaNet pre-pass (after conv): gates (sigmoid beta, decay) on
// threads [2*nKH, 2*nKH+nVH), per-head L2 norm of the q and k heads on threads
// [0, 2*nKH). One dispatch instead of three. ---
@group(1) @binding(0) var<storage, read> pp_alpha: array<f32>;
@group(1) @binding(1) var<storage, read> pp_beta: array<f32>;
@group(1) @binding(2) var<storage, read> pp_dt: array<f32>;
@group(1) @binding(3) var<storage, read> pp_a: array<f32>;
@group(1) @binding(4) var<storage, read_write> pp_bout: array<f32>;
@group(1) @binding(5) var<storage, read_write> pp_dout: array<f32>;
@group(1) @binding(6) var<storage, read_write> pp_v: array<f32>;   // conv output [q heads | k heads | v]
@group(1) @binding(7) var<uniform> pp_dn: DN;
fn pp_l2(off: u32) {
  // eight loads in flight, then the same in-order sum (bit-identical to the one-at-a-time loop)
  var ss: f32 = 0.0;
  var i: u32 = 0u;
  for (; i + 8u <= pp_dn.dState; i += 8u) {
    let v0 = pp_v[off + i]; let v1 = pp_v[off + i + 1u]; let v2 = pp_v[off + i + 2u]; let v3 = pp_v[off + i + 3u];
    let v4 = pp_v[off + i + 4u]; let v5 = pp_v[off + i + 5u]; let v6 = pp_v[off + i + 6u]; let v7 = pp_v[off + i + 7u];
    ss += v0 * v0; ss += v1 * v1; ss += v2 * v2; ss += v3 * v3; ss += v4 * v4; ss += v5 * v5; ss += v6 * v6; ss += v7 * v7;
  }
  for (; i < pp_dn.dState; i++) { let v = pp_v[off + i]; ss += v * v; }
  let inv = 1.0 / max(sqrt(ss), pp_dn.eps2);
  var j: u32 = 0u;
  for (; j + 8u <= pp_dn.dState; j += 8u) {
    let v0 = pp_v[off + j]; let v1 = pp_v[off + j + 1u]; let v2 = pp_v[off + j + 2u]; let v3 = pp_v[off + j + 3u];
    let v4 = pp_v[off + j + 4u]; let v5 = pp_v[off + j + 5u]; let v6 = pp_v[off + j + 6u]; let v7 = pp_v[off + j + 7u];
    pp_v[off + j] = v0 * inv; pp_v[off + j + 1u] = v1 * inv; pp_v[off + j + 2u] = v2 * inv; pp_v[off + j + 3u] = v3 * inv;
    pp_v[off + j + 4u] = v4 * inv; pp_v[off + j + 5u] = v5 * inv; pp_v[off + j + 6u] = v6 * inv; pp_v[off + j + 7u] = v7 * inv;
  }
  for (; j < pp_dn.dState; j++) { pp_v[off + j] *= inv; }
}
@compute @workgroup_size(128)
fn dn_pre(@builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let nL2 = 2u * pp_dn.nKH;
  if (t < nL2) { pp_l2(t * pp_dn.dState); }
  else if (t < nL2 + pp_dn.nVH) {
    let h = t - nL2;
    pp_bout[h] = 1.0 / (1.0 + exp(-pp_beta[h]));
    let av = pp_alpha[h] + pp_dt[h];
    var sp: f32;
    if (av > 20.0) { sp = av; } else { sp = log(1.0 + exp(av)); }
    pp_dout[h] = exp(sp * pp_a[h]);
  }
}

@group(1) @binding(0) var<storage, read> ppm_alpha: array<f32>;
@group(1) @binding(1) var<storage, read> ppm_beta: array<f32>;
@group(1) @binding(2) var<storage, read> ppm_dt: array<f32>;
@group(1) @binding(3) var<storage, read> ppm_a: array<f32>;
@group(1) @binding(4) var<storage, read_write> ppm_bout: array<f32>;
@group(1) @binding(5) var<storage, read_write> ppm_dout: array<f32>;
@group(1) @binding(6) var<storage, read_write> ppm_v: array<f32>;
@group(1) @binding(7) var<uniform> ppm_mc: MC;          // s0 = gate column stride, s1 = conv-out column stride
@group(1) @binding(8) var<uniform> ppm_dn: DN;
// the same in-order sum and the same products, with the loads issued 16 at a time: one dependent load per element
// left the 32 norm threads of a column latency bound (dn_pre_mc 39.8 -> 29.7 us per layer per verify pass on the GB10)
fn ppm_l2(off: u32) {
  var ss: f32 = 0.0;
  if (ppm_dn.dState % 16u == 0u) {
    for (var i: u32 = 0u; i < ppm_dn.dState; i += 16u) {
      let v0 = ppm_v[off + i + 0u]; let v1 = ppm_v[off + i + 1u]; let v2 = ppm_v[off + i + 2u]; let v3 = ppm_v[off + i + 3u]; let v4 = ppm_v[off + i + 4u]; let v5 = ppm_v[off + i + 5u]; let v6 = ppm_v[off + i + 6u]; let v7 = ppm_v[off + i + 7u]; let v8 = ppm_v[off + i + 8u]; let v9 = ppm_v[off + i + 9u]; let v10 = ppm_v[off + i + 10u]; let v11 = ppm_v[off + i + 11u]; let v12 = ppm_v[off + i + 12u]; let v13 = ppm_v[off + i + 13u]; let v14 = ppm_v[off + i + 14u]; let v15 = ppm_v[off + i + 15u];
      ss += v0 * v0; ss += v1 * v1; ss += v2 * v2; ss += v3 * v3; ss += v4 * v4; ss += v5 * v5; ss += v6 * v6; ss += v7 * v7; ss += v8 * v8; ss += v9 * v9; ss += v10 * v10; ss += v11 * v11; ss += v12 * v12; ss += v13 * v13; ss += v14 * v14; ss += v15 * v15;
    }
  } else {
    for (var i: u32 = 0u; i < ppm_dn.dState; i++) { let v = ppm_v[off + i]; ss += v * v; }
  }
  let inv = 1.0 / max(sqrt(ss), ppm_dn.eps2);
  if (ppm_dn.dState % 16u == 0u) {
    for (var i: u32 = 0u; i < ppm_dn.dState; i += 16u) {
      let v0 = ppm_v[off + i + 0u]; let v1 = ppm_v[off + i + 1u]; let v2 = ppm_v[off + i + 2u]; let v3 = ppm_v[off + i + 3u]; let v4 = ppm_v[off + i + 4u]; let v5 = ppm_v[off + i + 5u]; let v6 = ppm_v[off + i + 6u]; let v7 = ppm_v[off + i + 7u]; let v8 = ppm_v[off + i + 8u]; let v9 = ppm_v[off + i + 9u]; let v10 = ppm_v[off + i + 10u]; let v11 = ppm_v[off + i + 11u]; let v12 = ppm_v[off + i + 12u]; let v13 = ppm_v[off + i + 13u]; let v14 = ppm_v[off + i + 14u]; let v15 = ppm_v[off + i + 15u];
      ppm_v[off + i + 0u] = v0 * inv; ppm_v[off + i + 1u] = v1 * inv; ppm_v[off + i + 2u] = v2 * inv; ppm_v[off + i + 3u] = v3 * inv; ppm_v[off + i + 4u] = v4 * inv; ppm_v[off + i + 5u] = v5 * inv; ppm_v[off + i + 6u] = v6 * inv; ppm_v[off + i + 7u] = v7 * inv; ppm_v[off + i + 8u] = v8 * inv; ppm_v[off + i + 9u] = v9 * inv; ppm_v[off + i + 10u] = v10 * inv; ppm_v[off + i + 11u] = v11 * inv; ppm_v[off + i + 12u] = v12 * inv; ppm_v[off + i + 13u] = v13 * inv; ppm_v[off + i + 14u] = v14 * inv; ppm_v[off + i + 15u] = v15 * inv;
    }
  } else {
    for (var i: u32 = 0u; i < ppm_dn.dState; i++) { ppm_v[off + i] *= inv; }
  }
}
@compute @workgroup_size(128)
fn dn_pre_mc(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let t = lid.x; let col = wg.y; let nL2 = 2u * ppm_dn.nKH;
  if (t < nL2) { ppm_l2(col * ppm_mc.s1 + t * ppm_dn.dState); }
  else if (t < nL2 + ppm_dn.nVH) {
    let h = t - nL2; let o = col * ppm_mc.s0;
    ppm_bout[o + h] = 1.0 / (1.0 + exp(-ppm_beta[o + h]));
    let av = ppm_alpha[o + h] + ppm_dt[h];
    var sp: f32;
    if (av > 20.0) { sp = av; } else { sp = log(1.0 + exp(av)); }
    ppm_dout[o + h] = exp(sp * ppm_a[h]);
  }
}

@group(1) @binding(0) var<storage, read> gnm_x: array<f32>;
@group(1) @binding(1) var<storage, read> gnm_z: array<f32>;
@group(1) @binding(2) var<storage, read> gnm_w: array<f32>;
@group(1) @binding(3) var<storage, read_write> gnm_y: array<f32>;
@group(1) @binding(4) var<uniform> gnm_mc: MC;          // s0 x stride, s1 z stride, s2 y stride
@group(1) @binding(5) var<uniform> gnm_dn: DN;
var<workgroup> gnm_partial: array<f32, 128>;
@compute @workgroup_size(128)
fn dn_gatenorm_mc(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let j = lid.x;
  let off = h * gnm_dn.dState;
  let xo = wg.y * gnm_mc.s0 + off; let zo = wg.y * gnm_mc.s1 + off; let yo = wg.y * gnm_mc.s2 + off;
  let v = gnm_x[xo + j];
  gnm_partial[j] = v * v;
  workgroupBarrier();
  var stride: u32 = 64u;
  while (stride > 0u) {
    if (j < stride) { gnm_partial[j] += gnm_partial[j + stride]; }
    workgroupBarrier();
    stride = stride / 2u;
  }
  let inv = inverseSqrt(gnm_partial[0] / f32(gnm_dn.dState) + cfg.eps);
  let z = gnm_z[zo + j];
  gnm_y[yo + j] = gnm_x[xo + j] * inv * gnm_w[j] * (z / (1.0 + exp(-z)));
}

@group(1) @binding(0) var<storage, read> qsm_full: array<f32>;
@group(1) @binding(1) var<storage, read_write> qsm_q: array<f32>;
@group(1) @binding(2) var<storage, read_write> qsm_g: array<f32>;
@group(1) @binding(3) var<uniform> qsm_mc: MC;          // s0 full stride, s1 q stride, s2 g stride
@group(1) @binding(4) var<uniform> qsm_dn: DN;
@compute @workgroup_size(64)
fn qsplit_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let idx = gid.x;
  let total = cfg.nH * qsm_dn.hd;
  if (idx >= total) { return; }
  let h = idx / qsm_dn.hd;
  let i = idx % qsm_dn.hd;
  let fo = gid.y * qsm_mc.s0 + h * 2u * qsm_dn.hd;
  qsm_q[gid.y * qsm_mc.s1 + idx] = qsm_full[fo + i];
  qsm_g[gid.y * qsm_mc.s2 + idx] = qsm_full[fo + qsm_dn.hd + i];
}

@group(1) @binding(0) var<storage, read_write> hnm_v: array<f32>;
@group(1) @binding(1) var<storage, read> hnm_w: array<f32>;
@group(1) @binding(2) var<uniform> hnm_mc: MC;          // n = heads, s0 = column stride
@compute @workgroup_size(32)
fn head_norm_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let h = gid.x;
  if (h >= hnm_mc.n) { return; }
  let off = gid.y * hnm_mc.s0 + h * cfg.headDim;
  var ss: f32 = 0.0;
  for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = hnm_v[off + i]; ss += v * v; }
  let inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
  for (var i: u32 = 0u; i < cfg.headDim; i++) { hnm_v[off + i] *= inv * hnm_w[i]; }
}

@group(1) @binding(0) var<storage, read_write> rpm_v: array<f32>;
@group(1) @binding(1) var<uniform> rpm_mc: MC;          // n = heads, s0 = column stride
@group(1) @binding(2) var<uniform> rpm_dn: DN;
@compute @workgroup_size(64)
fn rope_part_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let half = rpm_dn.nRot / 2u;
  let total = rpm_mc.n * half;
  let idx = gid.x;
  if (idx >= total) { return; }
  let h = idx / half;
  let i = idx % half;
  let off = gid.y * rpm_mc.s0 + h * rpm_dn.hd;
  let ang = f32(frame.pos + gid.y) * pow(rpm_dn.ropeTheta, -f32(2u * i) / f32(rpm_dn.nRot));
  let c = cos(ang); let s = sin(ang);
  let a = rpm_v[off + i]; let b = rpm_v[off + i + half];
  rpm_v[off + i] = a * c - b * s;
  rpm_v[off + i + half] = b * c + a * s;
}

@group(1) @binding(0) var<storage, read_write> smm_a: array<f32>;
@group(1) @binding(1) var<storage, read> smm_g: array<f32>;
@group(1) @binding(2) var<uniform> smm_mc: MC;          // n, a stride, g stride
@compute @workgroup_size(64)
fn sigmoid_mul_mc(@builtin(global_invocation_id) gid: vec3<u32>) {
  let i = gid.x;
  if (i >= smm_mc.n) { return; }
  let g = smm_g[gid.y * smm_mc.s1 + i];
  let ai = gid.y * smm_mc.s0 + i;
  smm_a[ai] = smm_a[ai] * (1.0 / (1.0 + exp(-g)));
}

// ================= long-context attention: f16 KV cache + split-K flash decoding =================
// kv_store writes each column's k and v rows into the caches as packed f16 pairs (half the
// memory and bandwidth of f32; 64 KB per token per full-attention layer set on the 27B).
@group(1) @binding(0) var<storage, read> ks_k: array<f32>;
@group(1) @binding(1) var<storage, read> ks_v: array<f32>;
@group(1) @binding(2) var<storage, read_write> ks_kc: array<u32>;
@group(1) @binding(3) var<storage, read_write> ks_vc: array<u32>;
@group(1) @binding(4) var<uniform> ks_mc: MC;           // s0 k column stride, s1 v column stride (floats)
@compute @workgroup_size(64)
fn kv_store(@builtin(global_invocation_id) gid: vec3<u32>) {
  let w = gid.x; let col = gid.y;
  let kvw = cfg.kvDim / 2u;
  if (w >= kvw) { return; }
  let row = (frame.pos + col) * kvw + w;
  let ko = col * ks_mc.s0 + 2u * w; let vo = col * ks_mc.s1 + 2u * w;
  ks_kc[row] = pack2x16float(vec2<f32>(ks_k[ko], ks_k[ko + 1u]));
  ks_vc[row] = pack2x16float(vec2<f32>(ks_v[vo], ks_v[vo + 1u]));
}

// kv_store_q8: one thread per 32-value block of a K and a V row: scale = max|x| / 127, values
// rounded to int8, 4 per word. The flash kernel dequantises value * scale.
@group(1) @binding(0) var<storage, read> k8_k: array<f32>;
@group(1) @binding(1) var<storage, read> k8_v: array<f32>;
@group(1) @binding(2) var<storage, read_write> k8_kc: array<u32>;
@group(1) @binding(3) var<storage, read_write> k8_vc: array<u32>;
@group(1) @binding(4) var<storage, read_write> k8_ks: array<f32>;
@group(1) @binding(5) var<storage, read_write> k8_vs: array<f32>;
@group(1) @binding(6) var<uniform> k8_mc: MC;           // s0 k column stride, s1 v column stride
fn k8_block(x: ptr<function, array<f32, 32>>) -> vec2<f32> {   // (scale, 1/scale)
  var a: f32 = 0.0;
  for (var i: u32 = 0u; i < 32u; i++) { a = max(a, abs((*x)[i])); }
  let sc = a / 127.0;
  return vec2<f32>(sc, select(0.0, 1.0 / sc, sc > 0.0));
}
fn k8_pack(x: ptr<function, array<f32, 32>>, inv: f32, j: u32) -> u32 {
  var w: u32 = 0u;
  for (var b: u32 = 0u; b < 4u; b++) {
    let q = i32(clamp(round((*x)[4u * j + b] * inv), -127.0, 127.0));
    w = w | ((bitcast<u32>(q) & 255u) << (8u * b));
  }
  return w;
}
@compute @workgroup_size(64)
fn kv_store_q8(@builtin(global_invocation_id) gid: vec3<u32>) {
  let blk = gid.x; let col = gid.y;
  let nb = cfg.kvDim / 32u;
  if (blk >= nb) { return; }
  let p = frame.pos + col;
  var x: array<f32, 32>;
  for (var i: u32 = 0u; i < 32u; i++) { x[i] = k8_k[col * k8_mc.s0 + blk * 32u + i]; }
  var r = k8_block(&x);
  k8_ks[p * nb + blk] = r.x;
  for (var j: u32 = 0u; j < 8u; j++) { k8_kc[p * (cfg.kvDim / 4u) + blk * 8u + j] = k8_pack(&x, r.y, j); }
  for (var i: u32 = 0u; i < 32u; i++) { x[i] = k8_v[col * k8_mc.s1 + blk * 32u + i]; }
  r = k8_block(&x);
  k8_vs[p * nb + blk] = r.x;
  for (var j: u32 = 0u; j < 8u; j++) { k8_vc[p * (cfg.kvDim / 4u) + blk * 8u + j] = k8_pack(&x, r.y, j); }
}

// attn_flash: one 256-thread workgroup per (split of splitLen positions, column, kv head). The
// G = nH/nKV query heads sharing a kv head are done together, so each K/V row is read once for
// all of them. Inside a split, chunks of 64 positions: scores to workgroup memory, a running
// max / sum per head (online softmax), and thread i accumulates output dim i for every head.
// Writes per-split partials (max, sum, unnormalised output); attn_combine merges the splits.
// Every order is fixed by absolute position, so the decode (1 column) and batched (verify /
// prefill) passes give the same bits for a given position, and so do solo and split devices.
struct FA { s0: u32, s1: u32, splitLen: u32, maxSplits: u32 };   // q col stride, out col stride
var<workgroup> fa_qs: array<f32, 2048>;   // G * headDim <= 2048
var<workgroup> fa_sc: array<f32, 512>;    // G * 64 scores, then weights
var<workgroup> fa_m: array<f32, 8>;
var<workgroup> fa_l: array<f32, 8>;
var<workgroup> fa_a: array<f32, 8>;
${flashWGSL(false)}
${flashWGSL(true)}

// attn_flash_t2: attn_flash for two columns per workgroup (batched passes): each K and V row is
// read once for both columns' 2 x G query heads. Per (column, head) the scores, the running max /
// sum and the output accumulate in exactly attn_flash's order, so the partials are bit-identical
// to running the columns one by one (decode and verify still agree). Needs 2 * G * headDim <= 3072.
@group(1) @binding(0) var<storage, read> ft_q: array<f32>;
@group(1) @binding(1) var<storage, read> ft_k: array<u32>;
@group(1) @binding(2) var<storage, read> ft_v: array<u32>;
@group(1) @binding(3) var<storage, read_write> ft_o: array<f32>;
@group(1) @binding(4) var<storage, read_write> ft_ml: array<f32>;
@group(1) @binding(5) var<uniform> ft: FA;
var<workgroup> ft_qs: array<f32, 3072>;
var<workgroup> ft_sc: array<f32, 768>;
var<workgroup> ft_m: array<f32, 16>;
var<workgroup> ft_l: array<f32, 16>;
var<workgroup> ft_a: array<f32, 16>;
@compute @workgroup_size(256)
fn attn_flash_t2(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let sp = wg.x; let cb = wg.y * 2u; let g = wg.z; let tid = lid.x;
  let nc = max(frame.nCols, 1u);
  let hd = cfg.headDim; let G = cfg.nH / cfg.nKV;
  let hw = hd / 2u; let kvw = cfg.kvDim / 2u;
  let rs = sqrt(f32(hd));
  let t0 = sp * ft.splitLen;
  // where each of the two columns stops in this split (t0 = nothing to do)
  let s0 = frame.seqLen + cb; let s1 = s0 + 1u;
  let e0 = select(t0, min(s0, t0 + ft.splitLen), cb < nc && t0 < s0);
  let e1 = select(t0, min(s1, t0 + ft.splitLen), cb + 1u < nc && t0 < s1);
  let t1 = max(e0, e1);
  if (t1 <= t0) { return; }
  let GH = 2u * G;
  for (var w: u32 = tid; w < GH * hd; w += 256u) {
    let c = w / (G * hd);
    if (cb + c < nc) { ft_qs[w] = ft_q[(cb + c) * ft.s0 + g * G * hd + w % (G * hd)]; }
  }
  if (tid < GH) { ft_m[tid] = -3.0e38; ft_l[tid] = 0.0; }
  var acc: array<f32, 16>;
  for (var j: u32 = 0u; j < 16u; j++) { acc[j] = 0.0; }
  workgroupBarrier();
  for (var c0: u32 = t0; c0 < t1; c0 += 64u) {
    let n0 = select(0u, min(64u, e0 - c0), c0 < e0);
    let n1 = select(0u, min(64u, e1 - c0), c0 < e1);
    for (var w: u32 = tid; w < GH * 64u; w += 256u) {
      let j = w / 64u; let t = w % 64u;
      if (t < select(n0, n1, j >= G)) {
        let kb = (c0 + t) * kvw + g * hw;
        let qb = j * hd;
        var s: f32 = 0.0;
        for (var p: u32 = 0u; p < hw; p++) {
          let kk = unpack2x16float(ft_k[kb + p]);
          s += ft_qs[qb + 2u * p] * kk.x;
          s += ft_qs[qb + 2u * p + 1u] * kk.y;
        }
        ft_sc[w] = s / rs;
      }
    }
    workgroupBarrier();
    if (tid < GH) {
      let n = select(n0, n1, tid >= G);
      if (n > 0u) {
        let b = tid * 64u;
        var cm = ft_m[tid];
        for (var t: u32 = 0u; t < n; t++) { cm = max(cm, ft_sc[b + t]); }
        let alpha = exp(ft_m[tid] - cm);
        var l = ft_l[tid] * alpha;
        for (var t: u32 = 0u; t < n; t++) { let e = exp(ft_sc[b + t] - cm); ft_sc[b + t] = e; l += e; }
        ft_m[tid] = cm; ft_l[tid] = l; ft_a[tid] = alpha;
      }
    }
    workgroupBarrier();
    if (tid < hd) {
      if (n0 > 0u) { for (var h: u32 = 0u; h < G; h++) { acc[h] *= ft_a[h]; } }
      if (n1 > 0u) { for (var h: u32 = 0u; h < G; h++) { acc[G + h] *= ft_a[G + h]; } }
      let nm = max(n0, n1);
      for (var t: u32 = 0u; t < nm; t++) {
        let v = unpack2x16float(ft_v[(c0 + t) * kvw + g * hw + tid / 2u])[tid & 1u];
        if (t < n0) { for (var h: u32 = 0u; h < G; h++) { acc[h] += ft_sc[h * 64u + t] * v; } }
        if (t < n1) { for (var h: u32 = 0u; h < G; h++) { acc[G + h] += ft_sc[(G + h) * 64u + t] * v; } }
      }
    }
    workgroupBarrier();
  }
  for (var c: u32 = 0u; c < 2u; c++) {
    if (select(e0, e1, c == 1u) > t0) {
      let col = cb + c;
      if (tid < hd) {
        for (var h: u32 = 0u; h < G; h++) { ft_o[((col * cfg.nH + g * G + h) * ft.maxSplits + sp) * hd + tid] = acc[c * G + h]; }
      }
      if (tid < G) {
        let b = (col * cfg.nH + g * G + tid) * ft.maxSplits + sp;
        ft_ml[b * 2u] = ft_m[c * G + tid]; ft_ml[b * 2u + 1u] = ft_l[c * G + tid];
      }
    }
  }
}

@group(1) @binding(0) var<storage, read> fc_o: array<f32>;
@group(1) @binding(1) var<storage, read> fc_ml: array<f32>;
@group(1) @binding(2) var<storage, read_write> fc_out: array<f32>;
@group(1) @binding(3) var<uniform> fc: FA;
@compute @workgroup_size(256)
fn attn_combine(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let qh = wg.x; let col = wg.y; let i = lid.x;
  let hd = cfg.headDim;
  if (qh >= cfg.nH || i >= hd) { return; }
  let seqLen = frame.seqLen + col;
  let ns = (seqLen + fc.splitLen - 1u) / fc.splitLen;
  let b0 = (col * cfg.nH + qh) * fc.maxSplits;
  var M: f32 = -3.0e38;
  for (var s: u32 = 0u; s < ns; s++) { M = max(M, fc_ml[(b0 + s) * 2u]); }
  var L: f32 = 0.0; var O: f32 = 0.0;
  for (var s: u32 = 0u; s < ns; s++) {
    let w = exp(fc_ml[(b0 + s) * 2u] - M);
    L += fc_ml[(b0 + s) * 2u + 1u] * w;
    O += fc_o[(b0 + s) * hd + i] * w;
  }
  fc_out[col * fc.s1 + qh * hd + i] = O / L;
}

// --- fused attention glue: qsplit + q/k head_norm + partial rope in one dispatch ---
// One workgroup per (head, column); heads [0, nH) are q heads (split out of q_full, gate
// written to g), heads [nH, nH+nKV) are k heads (in place). Same arithmetic in the same order
// as the five separate kernels: the sum of squares runs serially on thread 0, so the result
// is bit-identical to qsplit -> head_norm -> rope_part. Needs headDim <= 256.
@group(1) @binding(0) var<storage, read> ag_full: array<f32>;
@group(1) @binding(1) var<storage, read_write> ag_q: array<f32>;
@group(1) @binding(2) var<storage, read_write> ag_g: array<f32>;
@group(1) @binding(3) var<storage, read_write> ag_k: array<f32>;
@group(1) @binding(4) var<storage, read> ag_qw: array<f32>;
@group(1) @binding(5) var<storage, read> ag_kw: array<f32>;
@group(1) @binding(6) var<uniform> ag_mc: MC;          // n = k stride, s0 full stride, s1 q stride, s2 g stride
@group(1) @binding(7) var<uniform> ag_dn: DN;
var<workgroup> ag_v: array<f32, 256>;
var<workgroup> ag_inv: f32;
@compute @workgroup_size(64)
fn attn_glue(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let hd = ag_dn.hd;
  let isQ = wg.x < cfg.nH;
  let h = select(wg.x - cfg.nH, wg.x, isQ);
  let col = wg.y;
  let fo = col * ag_mc.s0 + h * 2u * hd;
  let qo = col * ag_mc.s1 + h * hd;
  let go = col * ag_mc.s2 + h * hd;
  let ko = col * ag_mc.n + h * hd;
  for (var i: u32 = lid.x; i < hd; i += 64u) {
    if (isQ) { ag_v[i] = ag_full[fo + i]; ag_g[go + i] = ag_full[fo + hd + i]; }
    else { ag_v[i] = ag_k[ko + i]; }
  }
  workgroupBarrier();
  if (lid.x == 0u) {
    var ss: f32 = 0.0;
    for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = ag_v[i]; ss += v * v; }
    ag_inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
  }
  workgroupBarrier();
  let inv = ag_inv;
  for (var i: u32 = lid.x; i < hd; i += 64u) {
    let w = select(ag_kw[i], ag_qw[i], isQ);
    ag_v[i] = ag_v[i] * (inv * w);
  }
  workgroupBarrier();
  let half = ag_dn.nRot / 2u;
  for (var i: u32 = lid.x; i < half; i += 64u) {
    let ang = f32(frame.pos + col) * pow(ag_dn.ropeTheta, -f32(2u * i) / f32(ag_dn.nRot));
    let c = cos(ang); let s = sin(ang);
    let a = ag_v[i]; let b = ag_v[i + half];
    ag_v[i] = a * c - b * s;
    ag_v[i + half] = b * c + a * s;
  }
  workgroupBarrier();
  for (var i: u32 = lid.x; i < hd; i += 64u) {
    if (isQ) { ag_q[qo + i] = ag_v[i]; } else { ag_k[ko + i] = ag_v[i]; }
  }
}
`;
