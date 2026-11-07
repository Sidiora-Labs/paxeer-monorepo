// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/topk.js (MIT; see LICENSE-ENGINE).
// GPU sampling helpers: the host side of the topk_a / topk_b kernels (engine/wgsl/qwen35.js) and a
// CPU model of the same two-stage index math (tests/unit/topk_test.js checks it against a plain
// sort and against the host samplers, since the kernels themselves need a GPU).
//
// Order everywhere: value descending, index ascending on ties (the host greedy's rule: the lowest
// index of the largest value). NaN and -Inf are never picked; non-finite values (NaN, +-Inf) are
// counted as `bad`. Per column the kernels write k x [idx, bits(value)] then [bad, 0]; a missing
// entry (fewer than k finite values above -Inf) has idx NONE. With no finite value at all, the
// first entry is index 0 (what the host greedy returns for a column of -Inf).

export const TOPK_MAX = 64;        // workgroup memory and the result buffers are sized for this
export const TOPK_SLICE = 4096;    // logits per stage-a workgroup: 256 threads x 16 registers
export const NONE = 0xffffffff;

// sampler descriptor -> k: { kind: "greedy" } is 1; { kind: "topk", k } is clamped to 1..64
export function topkK(desc) {
  return desc && desc.kind === "topk" ? Math.max(1, Math.min(TOPK_MAX, desc.k | 0 || 1)) : 1;
}

// one column's result (k x [idx, bits], [bad, 0]) at u32 offset `off` -> { ids, vals, bad }, with
// only the pairs that exist. Copies out of `u` (a mapped range is detached after unmap).
export function readCands(u, off, k) {
  const f = new Float32Array(u.buffer, u.byteOffset, u.length);
  let m = 0;
  while (m < k && u[off + 2 * m] !== NONE) m++;
  const ids = new Uint32Array(m), vals = new Float32Array(m);
  for (let j = 0; j < m; j++) { ids[j] = u[off + 2 * j]; vals[j] = f[off + 2 * j + 1]; }
  return { ids, vals, bad: u[off + 2 * k] };
}

export const isCands = (x) => !!x && x.ids instanceof Uint32Array && x.vals instanceof Float32Array;

// ---- CPU model of the kernels ----
const F32_LOW = Math.fround(-3.402823e38);   // the kernels' starting value (-FLT_MAX)
const f32 = new Float32Array(1), u32 = new Uint32Array(f32.buffer);
const bitsOf = (v) => { f32[0] = v; return u32[0]; };
const better = (v, i, bv, bi) => v > bv || (v === bv && i < bi);
const after = (v, i, lv, li) => v < lv || (v === lv && i > li);
// the workgroup tree reduction (256 slots -> slot 0), exactly as tk_reduce
function reduce(V, I) {
  for (let s = 128; s > 0; s >>= 1)
    for (let t = 0; t < s; t++) if (better(V[t + s], I[t + s], V[t], I[t])) { V[t] = V[t + s]; I[t] = I[t + s]; }
  return [I[0], V[0]];
}

// Stage a + stage b for one column of x (floats; the column starts at col * stride) as the kernels
// run them, thread by thread. -> Uint32Array(2k + 2): the kernels' output for that column.
export function topkTwoStage(x, { n, stride = 0, col = 0, k = 1 }) {
  const nw = Math.ceil(n / TOPK_SLICE), R = 2 * k + 2, base = col * stride;
  const part = new Uint32Array(nw * R), pf = new Float32Array(part.buffer);
  const V = new Float32Array(256), I = new Uint32Array(256);
  for (let w = 0; w < nw; w++) {
    const s0 = w * TOPK_SLICE, rec = w * R;
    let bad = 0;
    for (let t = 0; t < 256; t++) for (let j = 0; j < 16; j++) {
      const i = s0 + j * 256 + t;
      if (i < n && (bitsOf(x[base + i]) & 0x7f800000) === 0x7f800000) bad++;
    }
    let lv = 0, li = NONE;
    for (let r = 0; r < k; r++) {
      for (let t = 0; t < 256; t++) {
        let bv = F32_LOW, bi = NONE;
        for (let j = 0; j < 16; j++) {
          const i = s0 + j * 256 + t;
          if (i >= n) continue;
          const v = x[base + i];
          if ((r === 0 || after(v, i, lv, li)) && better(v, i, bv, bi)) { bv = v; bi = i; }
        }
        V[t] = bv; I[t] = bi;
      }
      [li, lv] = reduce(V, I);
      part[rec + 2 * r] = li; pf[rec + 2 * r + 1] = lv;
    }
    part[rec + 2 * k] = bad;
  }
  const out = new Uint32Array(R), of = new Float32Array(out.buffer), m = nw * k;
  let bad = 0;
  for (let w = 0; w < nw; w++) bad += part[w * R + 2 * k];
  let lv = 0, li = NONE;
  for (let r = 0; r < k; r++) {
    for (let t = 0; t < 256; t++) {
      let bv = F32_LOW, bi = NONE;
      for (let e = t; e < m; e += 256) {
        const o = Math.floor(e / k) * R + 2 * (e % k), i = part[o];
        if (i === NONE) continue;
        const v = pf[o + 1];
        if ((r === 0 || after(v, i, lv, li)) && better(v, i, bv, bi)) { bv = v; bi = i; }
      }
      V[t] = bv; I[t] = bi;
    }
    [li, lv] = reduce(V, I);
    out[2 * r] = r === 0 && li === NONE ? 0 : li; of[2 * r + 1] = lv;
  }
  out[2 * k] = bad;
  return out;
}

// The same answer by a plain sort (the reference the two-stage model is checked against).
export function topkNaive(x, { n, stride = 0, col = 0, k = 1 }) {
  const base = col * stride, R = 2 * k + 2, out = new Uint32Array(R), of = new Float32Array(out.buffer);
  const all = [];
  let bad = 0;
  for (let i = 0; i < n; i++) {
    const v = x[base + i];
    if (!Number.isFinite(v)) bad++;
    if (v === v && v !== -Infinity) all.push(i);   // NaN and -Inf never picked
  }
  all.sort((a, b) => (x[base + b] - x[base + a]) || (a - b));
  for (let r = 0; r < k; r++) {
    if (r < all.length) { out[2 * r] = all[r]; of[2 * r + 1] = x[base + all[r]]; }
    else { out[2 * r] = r === 0 ? 0 : NONE; of[2 * r + 1] = F32_LOW; }
  }
  out[2 * k] = bad;
  return out;
}
