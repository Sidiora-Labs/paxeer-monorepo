// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/layer_fuse.js (MIT; see LICENSE-ENGINE).
// Decode-only layer fusion (engine option layerFuse): fewer dispatches per layer, same bits.
// Each kernel here replaces two dispatches of the one-token path with one, keeping every
// arithmetic expression and its order, so the outputs are bit-identical to the separate kernels:
//   dn_delta_gnp  = dn_pre (q/k L2 norm + beta/decay gates) folded into dn_delta_gn
//   attn_glue_kv  = attn_glue + kv_store (f16 KV cache), gate left in q_full
//   attn_combine_g = attn_combine + sigmoid_mul (the gate read straight from q_full)
//   (attnDecode v2's twin, attn_dec_combine_g = attn_dec_combine + sigmoid_mul, is in attn_dec.js)
// The DeltaNet conv is folded into the [qkv | z] GEMV's epilogue (coop.js, CV = true).
// Batched passes (verify, prefill) keep their _mc kernels. Appended to the module after WGSL2
// (uses its DN, MC and FA structs).
import { dnTwoPass } from "./qwen35.js";

// dn_delta_gn with the pre-pass inside. Per value head h: beta and decay come from the raw
// GEMV outputs with dn_pre's expressions, and the q and k heads of key head kh = h % nKH are
// L2-normalised from workgroup memory with dn_pre's in-order sum (8 loads, then 8 adds, per step).
// dn_pre wrote the normalised q/k back to the conv output in place; nothing else reads them in the
// one-token path, so here they only live in workgroup memory. Requires dState = 128.
function dnDeltaGnpWGSL() {
  const tp = dnTwoPass("dp_s", "dp1_k", "dp1_q");
  // pp_l2's sum, over a workgroup array instead of the conv output
  const l2 = (A) => `
    var ss: f32 = 0.0;
    for (var i: u32 = 0u; i < 128u; i += 8u) {
      let v0 = ${A}[i]; let v1 = ${A}[i + 1u]; let v2 = ${A}[i + 2u]; let v3 = ${A}[i + 3u];
      let v4 = ${A}[i + 4u]; let v5 = ${A}[i + 5u]; let v6 = ${A}[i + 6u]; let v7 = ${A}[i + 7u];
      ss += v0 * v0; ss += v1 * v1; ss += v2 * v2; ss += v3 * v3; ss += v4 * v4; ss += v5 * v5; ss += v6 * v6; ss += v7 * v7;
    }
    dp_inv = 1.0 / max(sqrt(ss), dp_dn.eps2);`;
  return `
@group(1) @binding(0) var<storage, read> dp_c: array<f32>;      // conv output [q | k | v], not yet normalised
@group(1) @binding(1) var<storage, read> dp_braw: array<f32>;   // beta before the sigmoid [nVH]
@group(1) @binding(2) var<storage, read> dp_alpha: array<f32>;  // [nVH]
@group(1) @binding(3) var<storage, read> dp_dta: array<f32>;    // [dt bias (nVH) | A (nVH)]
@group(1) @binding(4) var<storage, read_write> dp_s: array<f32>;
@group(1) @binding(5) var<storage, read> dp_z: array<f32>;
@group(1) @binding(6) var<storage, read> dp_w: array<f32>;
@group(1) @binding(7) var<storage, read_write> dp_y: array<f32>;
@group(1) @binding(8) var<uniform> dp_dn: DN;
var<workgroup> dp1_k: array<f32, 128>;
var<workgroup> dp1_q: array<f32, 128>;
var<workgroup> dp_partial: array<f32, 128>;
var<workgroup> dp_invq: f32;
var<workgroup> dp_invk: f32;
@compute @workgroup_size(128)
fn dn_delta_gnp(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let h = wg.x; let j = lid.x;
  let kh = h % dp_dn.nKH;
  let kOff = kh * 128u; let vOff = h * 128u; let Sb = h * 16384u;
  let beta = 1.0 / (1.0 + exp(-dp_braw[h]));
  let av = dp_alpha[h] + dp_dta[h];
  var sp: f32;
  if (av > 20.0) { sp = av; } else { sp = log(1.0 + exp(av)); }
  let decay = exp(sp * dp_dta[dp_dn.nVH + h]);
  let scale = inverseSqrt(f32(dp_dn.dState));
  dp1_k[j] = dp_c[dp_dn.keyDim + kOff + j]; dp1_q[j] = dp_c[kOff + j];
  workgroupBarrier();
  if (j == 0u) {${l2("dp1_q").replace("dp_inv =", "dp_invq =")}
  } else if (j == 64u) {${l2("dp1_k").replace("dp_inv =", "dp_invk =")}
  }
  workgroupBarrier();
  dp1_q[j] = dp1_q[j] * dp_invq; dp1_k[j] = dp1_k[j] * dp_invk;
  workgroupBarrier();
  var vh: f32 = 0.0; var sq: f32 = 0.0; var kq: f32 = 0.0;
  ${tp.pass1}
  let d = (dp_c[2u * dp_dn.keyDim + vOff + j] - vh) * beta;
  ${tp.pass2}
  let o = (sq + d * kq) * scale;
  dp_partial[j] = o * o;
  workgroupBarrier();
  var stride: u32 = 64u;
  while (stride > 0u) {
    if (j < stride) { dp_partial[j] += dp_partial[j + stride]; }
    workgroupBarrier();
    stride = stride / 2u;
  }
  let inv = inverseSqrt(dp_partial[0] / f32(dp_dn.dState) + cfg.eps);
  let z = dp_z[vOff + j];
  dp_y[vOff + j] = o * inv * dp_w[j] * (z / (1.0 + exp(-z)));
}`;
}

export function layerFuseWGSL() {
  return `
// ================= decode layer fusion (engine/wgsl/layer_fuse.js) =================
${dnDeltaGnpWGSL()}

// attn_glue + kv_store for one token (f16 KV cache): q heads as attn_glue (q normed and roped,
// the gate is not copied: attn_combine_g reads it from q_full), k heads normed and roped in
// workgroup memory and written to the cache as f16 pairs together with the v head, exactly as
// kv_store packs them. The roped k is not written back to the k buffer (only kv_store read it).
@group(1) @binding(0) var<storage, read> gk_full: array<f32>;
@group(1) @binding(1) var<storage, read_write> gk_q: array<f32>;
@group(1) @binding(2) var<storage, read> gk_k: array<f32>;
@group(1) @binding(3) var<storage, read> gk_v: array<f32>;
@group(1) @binding(4) var<storage, read> gk_qw: array<f32>;
@group(1) @binding(5) var<storage, read> gk_kw: array<f32>;
@group(1) @binding(6) var<storage, read_write> gk_kc: array<u32>;
@group(1) @binding(7) var<storage, read_write> gk_vc: array<u32>;
@group(1) @binding(8) var<uniform> gk_dn: DN;
var<workgroup> gk_vs: array<f32, 256>;
var<workgroup> gk_inv: f32;
@compute @workgroup_size(64)
fn attn_glue_kv(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let hd = gk_dn.hd;
  let isQ = wg.x < cfg.nH;
  let h = select(wg.x - cfg.nH, wg.x, isQ);
  let fo = h * 2u * hd;
  let qo = h * hd;
  let ko = h * hd;
  for (var i: u32 = lid.x; i < hd; i += 64u) {
    if (isQ) { gk_vs[i] = gk_full[fo + i]; }
    else { gk_vs[i] = gk_k[ko + i]; }
  }
  workgroupBarrier();
  if (lid.x == 0u) {
    var ss: f32 = 0.0;
    for (var i: u32 = 0u; i < cfg.headDim; i++) { let v = gk_vs[i]; ss += v * v; }
    gk_inv = inverseSqrt(ss / f32(cfg.headDim) + cfg.eps);
  }
  workgroupBarrier();
  let inv = gk_inv;
  for (var i: u32 = lid.x; i < hd; i += 64u) {
    let w = select(gk_kw[i], gk_qw[i], isQ);
    gk_vs[i] = gk_vs[i] * (inv * w);
  }
  workgroupBarrier();
  let half = gk_dn.nRot / 2u;
  for (var i: u32 = lid.x; i < half; i += 64u) {
    let ang = f32(frame.pos) * pow(gk_dn.ropeTheta, -f32(2u * i) / f32(gk_dn.nRot));
    let c = cos(ang); let s = sin(ang);
    let a = gk_vs[i]; let b = gk_vs[i + half];
    gk_vs[i] = a * c - b * s;
    gk_vs[i + half] = b * c + a * s;
  }
  workgroupBarrier();
  if (isQ) {
    for (var i: u32 = lid.x; i < hd; i += 64u) { gk_q[qo + i] = gk_vs[i]; }
  } else {
    let kvw = cfg.kvDim / 2u;
    let row = frame.pos * kvw + h * (hd / 2u);
    for (var w: u32 = lid.x; w < hd / 2u; w += 64u) {
      gk_kc[row + w] = pack2x16float(vec2<f32>(gk_vs[2u * w], gk_vs[2u * w + 1u]));
      gk_vc[row + w] = pack2x16float(vec2<f32>(gk_v[ko + 2u * w], gk_v[ko + 2u * w + 1u]));
    }
  }
}

// attn_combine + sigmoid_mul for one token. The gate of q head qh is q_full[qh * 2 * hd + hd + i] (the
// values attn_glue copies into the gate buffer). The quotient O / L goes through workgroup memory before the
// gate multiply: written as one expression, Metal folds the two (new bits on the M5 Max; the GB10 was unchanged).
@group(1) @binding(0) var<storage, read> fcg_o: array<f32>;
@group(1) @binding(1) var<storage, read> fcg_ml: array<f32>;
@group(1) @binding(2) var<storage, read_write> fcg_out: array<f32>;
@group(1) @binding(3) var<storage, read> fcg_full: array<f32>;
@group(1) @binding(4) var<uniform> fcg: FA;
var<workgroup> fcg_t: array<f32, 256>;
@compute @workgroup_size(256)
fn attn_combine_g(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let qh = wg.x; let i = lid.x;
  let hd = cfg.headDim;
  let live = qh < cfg.nH && i < hd;   // no early return: the barrier below needs uniform control flow
  if (live) {
    let seqLen = frame.seqLen;
    let ns = (seqLen + fcg.splitLen - 1u) / fcg.splitLen;
    let b0 = qh * fcg.maxSplits;
    var M: f32 = -3.0e38;
    for (var s: u32 = 0u; s < ns; s++) { M = max(M, fcg_ml[(b0 + s) * 2u]); }
    var L: f32 = 0.0; var O: f32 = 0.0;
    for (var s: u32 = 0u; s < ns; s++) {
      let w = exp(fcg_ml[(b0 + s) * 2u] - M);
      L += fcg_ml[(b0 + s) * 2u + 1u] * w;
      O += fcg_o[(b0 + s) * hd + i] * w;
    }
    fcg_t[i] = O / L;
  }
  workgroupBarrier();   // the quotient goes through workgroup memory: the multiply cannot be folded into it
  if (live) {
    let g = fcg_full[qh * 2u * hd + hd + i];
    fcg_out[qh * hd + i] = fcg_t[i] * (1.0 / (1.0 + exp(-g)));
  }
}

`;
}
