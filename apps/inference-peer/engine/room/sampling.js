// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/room/sampling.js (MIT; see LICENSE-ENGINE).
// Top-k / temperature sampling over a logits vector (host CPU).

export function aiSample(logits, temp = 0.8, topk = 40) {
  // single-pass top-k selection: sorting all 248k logit indices cost tens of
  // milliseconds per token; this is O(n) with a tiny candidate table.
  const idx = new Int32Array(topk), val = new Float32Array(topk).fill(-Infinity);
  let min = -Infinity, minAt = 0;
  for (let i = 0; i < logits.length; i++) {
    const v = logits[i];
    if (v > min) {
      idx[minAt] = i; val[minAt] = v;
      min = val[0]; minAt = 0;
      for (let j = 1; j < topk; j++) if (val[j] < min) { min = val[j]; minAt = j; }
    }
  }
  const order = [...idx.keys()].sort((a, b) => val[b] - val[a]);
  const mx = val[order[0]];
  const ps = order.map((j) => Math.exp((val[j] - mx) / temp));
  const sum = ps.reduce((a, b) => a + b, 0);
  let r = Math.random() * sum;
  for (let i = 0; i < order.length; i++) { r -= ps[i]; if (r <= 0) return idx[order[i]]; }
  return idx[order[0]];
}

// Sampling presets the host picks for the room. "exact" is greedy: the same question gives the
// same answer on any room shape, which is also how the bit-exactness claims are checked.
export const SAMPLING = {
  creative: { label: "creative (t 0.8)", temp: 0.8, topk: 40 },
  focused: { label: "focused (t 0.4)", temp: 0.4, topk: 20 },
  exact: { label: "exact (greedy)", temp: 0, topk: 1 },
};

export function greedy(logits) {
  let best = 0, bv = -Infinity;
  for (let i = 0; i < logits.length; i++) if (logits[i] > bv) { bv = logits[i]; best = i; }
  return best;
}

// The same draw as aiSample, over the top pairs the GPU already picked (engine headFromHiddenIds:
// { ids, vals } sorted by value descending, index ascending on ties). With the same Math.random()
// it returns what aiSample returns on the full logits, except in exact ties at the cut.
export function aiSampleTop(cands, temp = 0.8) {
  const { ids, vals } = cands, n = ids.length;
  if (!n) return 0;
  const mx = vals[0];
  const ps = new Array(n);
  let sum = 0;
  for (let i = 0; i < n; i++) { ps[i] = Math.exp((vals[i] - mx) / temp); sum += ps[i]; }
  let r = Math.random() * sum;
  for (let i = 0; i < n; i++) { r -= ps[i]; if (r <= 0) return ids[i]; }
  return ids[0];
}

// candidates object (GPU sampling) or logits vector?
const isCands = (x) => !!x && x.ids instanceof Uint32Array && !!x.vals;

// logits -> token id for a temperature and top-k: temp 0 is greedy. The returned function carries
// .gpu = { kind: "greedy" } | { kind: "topk", k, temp }: an engine with gpuSample on then samples on
// the GPU and hands it { ids, vals, bad } (k pairs) instead of the logits, and it accepts either. A
// wrapper that masks the logits first (the tool-name constraint) is a new function without .gpu,
// so it still gets the full logits. topk is clamped to 1..64 (engine/topk.js TOPK_MAX) so GPU
// sampling keeps working for any value an API client asks for.
export const TOPK_LIMIT = 64;
export function makeSampler({ temp = 0.8, topK = 40 } = {}) {
  const t = Number.isFinite(+temp) && +temp > 0 ? +temp : 0;
  const k = Math.max(1, Math.min(TOPK_LIMIT, Math.round(+topK) || 40));
  const f = t === 0
    ? (x) => (isCands(x) ? x.ids[0] : greedy(x))
    : (x) => (isCands(x) ? aiSampleTop(x, t) : aiSample(x, t, k));
  f.gpu = t === 0 ? { kind: "greedy" } : { kind: "topk", k, temp: t };
  return f;
}

// logits -> token id for a preset key (unknown keys fall back to creative).
export function pickSampler(key) {
  const p = SAMPLING[key] || SAMPLING.creative;
  return makeSampler({ temp: p.temp, topK: p.topk });
}
