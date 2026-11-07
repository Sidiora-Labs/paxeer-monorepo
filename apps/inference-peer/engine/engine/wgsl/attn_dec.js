// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/attn_dec.js (MIT; see LICENSE-ENGINE).
// attn_dec + attn_dec_combine: split-K ("flash-decoding") attention for decode and verify passes at long
// context, opt-in (Qwen35Engine option attnDecode: "v2"; docs/research/long-context-2026-09.md).
//
// attn_flash (engine/wgsl/qwen35.js) runs one workgroup per (split of faSplit positions, column, kv head)
// and, per 64-position chunk, has every thread do a serial 256-long dot product over u32 loads of its
// own K row, then G threads run the online softmax serially, then every thread walks the 64 V rows one
// load at a time. Measured on the GB10, that is latency bound: attention at 32K costs several times
// the time it takes to stream the K/V bytes.
//
// attn_dec keeps the same outer shape (one workgroup per (split, column, kv head), per-split partials
// merged by a combine pass) and changes the inside:
//   - split length from the column's own context length: splitLen(s) = max(64, up64(ceil(s / S))), so
//     every column has at most S splits (S = attnDecodeSplits, default 256 / nKV workgroups per kv
//     head), short contexts still fill the GPU, and long ones get longer splits. A column's splits
//     depend only on its absolute position, so a verify column and a decode step at the same position
//     give the same bits (spec == plain), and so do solo and split rooms.
//   - 128-position chunks. Scores: two adjacent threads per K row, each with half of the row's 16-byte
//     vec4<u32> loads (interleaved, so a thread pair reads one whole 32-byte sector per load), eight
//     loads issued before use; q is in workgroup memory as vec4s (broadcast reads). The pair's two
//     partial dots are summed in a fixed order (lo + hi) through workgroup memory.
//   - softmax: 32 threads per head, each over 4 positions, maxima and sums of the 32 partials in a
//     fixed order; online rescale across chunks as in attn_flash.
//   - V: 32 threads per V row (one vec4<u32> = 8 dims each, a warp reads a whole 512-byte row), 8 row
//     groups, 4 rows per group in flight; the 8 row groups are summed in a fixed order at the end.
// Every sum has a fixed order that depends only on the position, so the kernel is deterministic. It is
// not bit-identical to attn_flash (different split boundaries and summation orders): tolerance, not
// bits, against the f32 reference (tests/test_attn_dec.js), same greedy tokens as attn_flash.
//
// Shape limits: headDim 256, G = nH / nKV <= 8 per workgroup, f16 KV (not kvQ8). Workgroup memory <= 16 KB.
// R > 1: more query heads per kv head than that (Qwen3.5-122B: 16), so each kv head gets R workgroups of
// G = nH / nKV / R heads (wg.z runs over nKV * R; the engine passes that product as nKV and this G).
//
// gather = stride (QSA, engine/wgsl/qsa.js): each column attends to a list of KV rows instead of rows
// 0 .. seqLen-1. Column col's list is ix[col * stride] = n, then n row indices ascending at ix[col * stride + 1 ..].
// List position t stands where position t stands in the dense kernel (split lengths come from n), so with the
// list 0 .. seqLen-1 the gathered kernel does the same arithmetic in the same order: the same bits as the dense
// one. Entry points are attn_dec_qsa / attn_dec_qsa_combine(_g); binding 6 is the list. gather = 0 (default)
// emits exactly the dense module.

export function attnDecConfig({ hd, G, nKV, splits = 0, kvQ8 = false, R = 1 }) {
  if (kvQ8 || hd !== 256 || !(G >= 1 && G <= 8) || !(R >= 1)) return null;
  const S = splits > 0 ? splits | 0 : Math.max(16, Math.floor(256 / nKV));
  return { HD: hd, G, S, R };
}

// JavaScript mirror of fd_split / the number of splits a column with s positions uses
export const decSplitLen = (s, S) => Math.max(64, Math.ceil(Math.ceil(s / S) / 64) * 64);
export const decSplits = (s, S) => Math.ceil(s / decSplitLen(s, S));

// combine: workgroups per head (dim slices); each has DEC_CQ slices of HD / DEC_CQ dims
export const DEC_CQ = 4;

const attnDecHeader = `
struct Config {
  dim: u32, kvDim: u32, nH: u32, nKV: u32,
  headDim: u32, inter: u32, vocab: u32, maxSeq: u32,
  eps: f32, theta: f32, qDim: u32,
};
struct Frame { pos: u32, seqLen: u32, nCols: u32, snap: u32 };
struct FD { s0: u32, s1: u32, S: u32, slots: u32 };   // q col stride, out col stride (floats), target splits, slot stride
@group(0) @binding(0) var<uniform> cfg: Config;
@group(0) @binding(1) var<uniform> frame: Frame;

fn fd_split(s: u32, S: u32) -> u32 { return max(64u, ((s + S - 1u) / S + 63u) / 64u * 64u); }
`;

export function attnDecWGSL({ HD, G, S, R = 1, gather = 0 }) {
  const X = gather ? "_qsa" : "";
  // KV row of list position e (a WGSL u32 expression)
  const row = (e) => gather ? `ad_ix[ib + ${e}]` : `(${e})`;
  const CQ = DEC_CQ, CD = HD / CQ, CG = 256 / (CD / 4);   // slices, dims per slice, split groups
  const H = [...Array(G).keys()];
  const QV = HD / 4;                 // vec4s of q per head
  const RED = Math.max(G * QV, 512); // q (G heads) then the 8 row groups' partial outputs (8 x 64 vec4)
  // scores: thread pair (r, hf) takes vec4 indices 2k + hf, k < 16, of row r; loads in two batches of 8
  const scoreBatch = (b) => {
    const ks = [...Array(8).keys()];
    return ks.map((i) => `let kw${i} = ad_k[kb + ${2 * (8 * b + i)}u + hf];`).join("\n        ") + "\n        " +
      ks.map((i) => {
        const k = 8 * b + i;
        return `{ let ka = vec4<f32>(unpack2x16float(kw${i}.x), unpack2x16float(kw${i}.y)); let kc = vec4<f32>(unpack2x16float(kw${i}.z), unpack2x16float(kw${i}.w)); let qd = ${4 * k}u + 2u * hf;\n          ` +
          H.map((h) => `s${h} += dot(ad_qs[${h * QV}u + qd], ka) + dot(ad_qs[${h * QV}u + qd + 1u], kc);`).join(" ") + " }";
      }).join("\n        ");
  };
  const vrow = (v, t) => `{ let va = vec4<f32>(unpack2x16float(${v}.x), unpack2x16float(${v}.y)); let vc = vec4<f32>(unpack2x16float(${v}.z), unpack2x16float(${v}.w));
          ${H.map((h) => `{ let p = ad_sc[${h * 128}u + ${t}]; oa${h} += p * va; ob${h} += p * vc; }`).join(" ")} }`;
  return /* wgsl */ `${attnDecHeader}
@group(1) @binding(0) var<storage, read> ad_q: array<f32>;
@group(1) @binding(1) var<storage, read> ad_k: array<vec4<u32>>;   // f16 K rows, 8 per vec4
@group(1) @binding(2) var<storage, read> ad_v: array<vec4<u32>>;
@group(1) @binding(3) var<storage, read_write> ad_o: array<f32>;
@group(1) @binding(4) var<storage, read_write> ad_ml: array<f32>;
@group(1) @binding(5) var<uniform> adu: FD;${gather ? "\n@group(1) @binding(6) var<storage, read> ad_ix: array<u32>;   // per column: n, then n KV rows" : ""}
var<workgroup> ad_qs: array<vec4<f32>, ${RED}>;
var<workgroup> ad_sc: array<f32, ${G * 128}>;
var<workgroup> ad_r1: array<f32, ${G * 32}>;
var<workgroup> ad_r2: array<f32, ${G * 32}>;
var<workgroup> ad_m: array<f32, ${G}>;
var<workgroup> ad_l: array<f32, ${G}>;
var<workgroup> ad_a: array<f32, ${G}>;

@compute @workgroup_size(256)
fn attn_dec${X}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let sp = wg.x; let col = wg.y; let g = wg.z; let tid = lid.x;
  ${gather ? `let seqLen = ad_ix[col * ${gather}u]; let ib = col * ${gather}u + 1u;` : "let seqLen = frame.seqLen + col;"}
  let SL = fd_split(seqLen, adu.S);
  let t0 = sp * SL;
  if (t0 >= seqLen) { return; }
  let t1 = min(seqLen, t0 + SL);
  let rs = sqrt(f32(${HD}));
  let qb = col * adu.s0 + g * ${G * HD}u;
  for (var w: u32 = tid; w < ${G * QV}u; w += 256u) {
    ad_qs[w] = vec4<f32>(ad_q[qb + 4u * w], ad_q[qb + 4u * w + 1u], ad_q[qb + 4u * w + 2u], ad_q[qb + 4u * w + 3u]);
  }
  if (tid < ${G}u) { ad_m[tid] = -3.0e38; ad_l[tid] = 0.0; }
  ${H.map((h) => `var oa${h} = vec4<f32>(0.0); var ob${h} = vec4<f32>(0.0);`).join(" ")}
  let r = tid >> 1u; let hf = tid & 1u;
  let rg = tid >> 5u; let j = tid & 31u;
  let kvRow = cfg.kvDim / 8u;            // vec4<u32> per position (all kv heads)
  let gOff = ${R > 1 ? `(g / ${R}u)` : "g"} * ${HD / 8}u;
  let sh = tid >> 5u; let ln = tid & 31u; // softmax: head, lane
  var Mh: f32 = -3.0e38;                  // this head's new running max (softmax threads)
  workgroupBarrier();
  for (var c0: u32 = t0; c0 < t1; c0 += 128u) {
    let n = min(128u, t1 - c0);
    // scores
    ${H.map((h) => `var s${h}: f32 = 0.0;`).join(" ")}
    if (r < n) {
      let kb = ${row("c0 + r")} * kvRow + gOff;
      {
        ${scoreBatch(0)}
      }
      {
        ${scoreBatch(1)}
      }
    }
    if (hf == 1u && r < n) { ${H.map((h) => `ad_sc[${h * 128}u + r] = s${h};`).join(" ")} }
    workgroupBarrier();
    if (hf == 0u && r < n) { ${H.map((h) => `ad_sc[${h * 128}u + r] = (s${h} + ad_sc[${h * 128}u + r]) / rs;`).join(" ")} }
    workgroupBarrier();
    // online softmax: 32 threads per head
    if (sh < ${G}u) {
      var lm: f32 = -3.0e38;
      for (var i: u32 = 0u; i < 4u; i++) { let t = ln + 32u * i; if (t < n) { lm = max(lm, ad_sc[sh * 128u + t]); } }
      ad_r1[tid] = lm;
    }
    workgroupBarrier();
    if (sh < ${G}u) {
      Mh = ad_m[sh];
      for (var i: u32 = 0u; i < 32u; i++) { Mh = max(Mh, ad_r1[sh * 32u + i]); }
      let M = Mh;
      var ls: f32 = 0.0;
      for (var i: u32 = 0u; i < 4u; i++) {
        let t = ln + 32u * i;
        if (t < n) { let e = exp(ad_sc[sh * 128u + t] - M); ad_sc[sh * 128u + t] = e; ls += e; }
      }
      ad_r2[tid] = ls;
    }
    workgroupBarrier();
    if (sh < ${G}u && ln == 0u) {
      let M = Mh;
      let alpha = exp(ad_m[sh] - M);
      var L = ad_l[sh] * alpha;
      for (var i: u32 = 0u; i < 32u; i++) { L += ad_r2[sh * 32u + i]; }
      ad_m[sh] = M; ad_l[sh] = L; ad_a[sh] = alpha;
    }
    workgroupBarrier();
    // V: row group rg takes rows rg, rg + 8, ...; lane j takes dims 8j .. 8j + 7
    ${H.map((h) => `{ let a = ad_a[${h}u]; oa${h} *= a; ob${h} *= a; }`).join(" ")}
    ${gather ? "let vb = gOff + j;" : "let vb = c0 * kvRow + gOff + j;"}
    var t = rg;
    for (; t + 24u < n; t += 32u) {
      ${gather ? "let v0 = ad_v[vb + ad_ix[ib + c0 + t] * kvRow]; let v1 = ad_v[vb + ad_ix[ib + c0 + t + 8u] * kvRow]; let v2 = ad_v[vb + ad_ix[ib + c0 + t + 16u] * kvRow]; let v3 = ad_v[vb + ad_ix[ib + c0 + t + 24u] * kvRow];"
        : "let v0 = ad_v[vb + t * kvRow]; let v1 = ad_v[vb + (t + 8u) * kvRow]; let v2 = ad_v[vb + (t + 16u) * kvRow]; let v3 = ad_v[vb + (t + 24u) * kvRow];"}
      ${vrow("v0", "t")}
      ${vrow("v1", "t + 8u")}
      ${vrow("v2", "t + 16u")}
      ${vrow("v3", "t + 24u")}
    }
    for (; t < n; t += 8u) {
      let v0 = ad_v[vb + ${gather ? "ad_ix[ib + c0 + t]" : "t"} * kvRow];
      ${vrow("v0", "t")}
    }
    workgroupBarrier();
  }
  // sum the 8 row groups in order, one head at a time (q's workgroup memory is free now)
  ${H.map((h) => `ad_qs[rg * 64u + 2u * j] = oa${h}; ad_qs[rg * 64u + 2u * j + 1u] = ob${h};
  workgroupBarrier();
  if (tid < 64u) {
    var o = ad_qs[tid];
    for (var i: u32 = 1u; i < 8u; i++) { o += ad_qs[i * 64u + tid]; }
    let ob = ((col * cfg.nH + g * ${G}u + ${h}u) * adu.slots + sp) * ${HD}u + 4u * tid;
    ad_o[ob] = o.x; ad_o[ob + 1u] = o.y; ad_o[ob + 2u] = o.z; ad_o[ob + 3u] = o.w;
  }
  workgroupBarrier();`).join("\n  ")}
  if (tid < ${G}u) {
    let b = (col * cfg.nH + g * ${G}u + tid) * adu.slots + sp;
    ad_ml[b * 2u] = ad_m[tid]; ad_ml[b * 2u + 1u] = ad_l[tid];
  }
}

${attnDecCombineWGSL({ HD, S, gather })}`;
}

// attn_dec_combine_g in a module of its own (optional, like layerFuse's kernels: a compile failure leaves
// attnDecode v2 running with the separate sigmoid_mul). One token only (column 0: the gate offset has no
// column stride).
export function attnDecCombineGWSL({ HD, S, gather = 0 }) { return attnDecHeader + attnDecCombineWGSL({ HD, S, gate: true, gather }); }

// attn_dec_combine, and (gate = true) attn_dec_combine_g: the same combine with layerFuse.comb's sigmoid gate
// for one token (sigmoid_mul folded in). The gate of q head qh is q_full[qh * 2 * hd + hd + i], the value
// attn_glue copies into the gate buffer. The quotient goes through workgroup memory before the gate multiply,
// as in attn_combine_g (layer_fuse.js): written as one expression, Metal may fold the two. Same bits as
// attn_dec_combine + sigmoid_mul. No early return in the gated one: its barrier needs uniform control flow.
function attnDecCombineWGSL({ HD, S, gate = false, gather = 0 }) {
  const CQ = DEC_CQ, CD = HD / CQ, CG = 256 / (CD / 4);
  const F = (gather ? "attn_dec_qsa_combine" : "attn_dec_combine") + (gate ? "_g" : "");
  const U = gate ? "adcg" : "adc";
  // store the vec4 r at dims 4 * (dq * CD / 4 + v) of head qh; gated: into workgroup memory first
  const put = (v) => gate ? `adcg_t[4u * ${v}] = r.x; adcg_t[4u * ${v} + 1u] = r.y; adcg_t[4u * ${v} + 2u] = r.z; adcg_t[4u * ${v} + 3u] = r.w;`
    : `let ob = col * adc.s1 + qh * ${HD}u + 4u * (dq * ${CD / 4}u + ${v});
      adc_out[ob] = r.x; adc_out[ob + 1u] = r.y; adc_out[ob + 2u] = r.z; adc_out[ob + 3u] = r.w;`;
  const decl = (gate ? `@group(1) @binding(3) var<storage, read> adcg_full: array<f32>;
@group(1) @binding(4) var<uniform> adcg: FD;
var<workgroup> adcg_t: array<f32, ${CD}>;   // this slice's quotients O / L` : `@group(1) @binding(3) var<uniform> adc: FD;`) + `
var<workgroup> adc_w: array<f32, ${S}>;      // per-split weight exp(m_s - M)
var<workgroup> adc_r: array<f32, 256>;       // reductions
var<workgroup> adc_p: array<vec4<f32>, 256>; // split groups' partial outputs` + (gather ? `
@group(1) @binding(6) var<storage, read> adc_ix: array<u32>;   // per column: n, then the KV rows` : "");
  const one = `if (tid < ${CD / 4}u) {
      let r = adc_o[b0 * ${HD / 4}u + vi] / adc_ml[b0 * 2u + 1u];
      ${put("lane")}
    }`;
  return `
@group(1) @binding(0) var<storage, read> adc_o: array<vec4<f32>>;
@group(1) @binding(1) var<storage, read> adc_ml: array<f32>;
@group(1) @binding(2) var<storage, read_write> adc_out: array<f32>;
${decl}
// one workgroup per (head, ${HD / CD}-dim slice, column); every split read in parallel: the maximum and the
// weighted sum of l by fixed-order tree reductions, then ${CG} split groups (16 threads x vec4 = ${CD} dims each)
// over strided splits, summed in group order. Deterministic; depends only on the column's own position.
@compute @workgroup_size(256)
fn ${F}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_id) lid: vec3<u32>) {
  let qh = wg.x / ${CQ}u; let dq = wg.x % ${CQ}u; let col = wg.y; let tid = lid.x;
  if (qh >= cfg.nH) { return; }
  let seqLen = ${gather ? `adc_ix[col * ${gather}u]` : "frame.seqLen + col"};
  let SL = fd_split(seqLen, ${U}.S);
  let ns = (seqLen + SL - 1u) / SL;
  let b0 = (col * cfg.nH + qh) * ${U}.slots;
  let lane = tid & ${CD / 4 - 1}u; let grp = tid / ${CD / 4}u;
  let vi = dq * ${CD / 4}u + lane;      // vec4 index within the head
  if (ns == 1u) {   // one split (short contexts): the same bits as below (weight exp(0) = 1, sums with zeros)
    ${one}${gate ? "" : "\n    return;"}
  }${gate ? " else {" : ""}
  var lm: f32 = -3.0e38;
  for (var s: u32 = tid; s < ns; s += 256u) { lm = max(lm, adc_ml[(b0 + s) * 2u]); }
  adc_r[tid] = lm;
  workgroupBarrier();
  for (var k: u32 = 128u; k > 0u; k >>= 1u) { if (tid < k) { adc_r[tid] = max(adc_r[tid], adc_r[tid + k]); } workgroupBarrier(); }
  let M = adc_r[0];
  workgroupBarrier();
  var ll: f32 = 0.0;
  for (var s: u32 = tid; s < ns; s += 256u) { let w = exp(adc_ml[(b0 + s) * 2u] - M); adc_w[s] = w; ll += adc_ml[(b0 + s) * 2u + 1u] * w; }
  adc_r[tid] = ll;
  workgroupBarrier();
  for (var k: u32 = 128u; k > 0u; k >>= 1u) { if (tid < k) { adc_r[tid] = adc_r[tid] + adc_r[tid + k]; } workgroupBarrier(); }
  let L = adc_r[0];
  var o = vec4<f32>(0.0);
  for (var s: u32 = grp; s < ns; s += ${CG}u) { o += adc_o[(b0 + s) * ${HD / 4}u + vi] * adc_w[s]; }
  adc_p[tid] = o;
  workgroupBarrier();
  if (tid < ${CD / 4}u) {
    var t = adc_p[tid];
    for (var i: u32 = 1u; i < ${CG}u; i++) { t += adc_p[i * ${CD / 4}u + tid]; }
    let r = t / L;
    ${put("tid")}
  }${gate ? `
  }
  workgroupBarrier();   // the quotient goes through workgroup memory: the multiply cannot be folded into it
  if (tid < ${CD}u) {
    let d = dq * ${CD}u + tid;
    let g = adcg_full[qh * ${2 * HD}u + ${HD}u + d];
    adc_out[qh * ${HD}u + d] = adcg_t[tid] * (1.0 / (1.0 + exp(-g)));
  }` : ""}
}
`;
}
