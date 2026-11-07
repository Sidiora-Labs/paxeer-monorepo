// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/selftest.js (MIT; see LICENSE-ENGINE).
// GPU self-test and kernel micro-tests run at load to catch broken drivers early.
import { DenseEngine } from "./dense.js";
import { quantizeQ4, dequantQ4, dequantQ8 } from "./quant.js";
import { quantizeQ8 } from "./gguf.js";

export async function gpuSelfTest(device) {
  const cfg = { hidden_size: 64, num_attention_heads: 4, num_key_value_heads: 2, head_dim: 16,
    intermediate_size: 128, vocab_size: 96, num_hidden_layers: 2, rms_norm_eps: 1e-5, rope_theta: 10000 };
  const { hidden_size: dim, num_attention_heads: nH, num_key_value_heads: nKV, head_dim: hd,
    intermediate_size: inter, vocab_size: vocab, num_hidden_layers: L, rms_norm_eps: eps, rope_theta: theta } = cfg;
  const qDim = nH * hd, kvDim = nKV * hd;
  let seed = 12345;
  const rnd = () => { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return seed / 0x7fffffff - 0.5; };
  const mat = (r, c, s) => { const a = new Float32Array(r * c); for (let i = 0; i < a.length; i++) a[i] = rnd() * s; return a; };
  const vec1 = (n) => { const a = new Float32Array(n); for (let i = 0; i < n; i++) a[i] = 1 + rnd() * 0.2; return a; };
  const raw = { embed: mat(vocab, dim, 1), finalNorm: vec1(dim), layers: [] };
  for (let l = 0; l < L; l++) raw.layers.push({
    inNorm: vec1(dim), postNorm: vec1(dim),
    q: mat(qDim, dim, 0.3), k: mat(kvDim, dim, 0.3), v: mat(kvDim, dim, 0.3), o: mat(dim, qDim, 0.3),
    gate: mat(inter, dim, 0.3), up: mat(inter, dim, 0.3), down: mat(dim, inter, 0.3),
  });
  const ids = [3, 17, 42];

  // CPU reference on the (possibly dequantized) weights
  const cpuForward = (W) => {
    const rms = (x, w) => { let ss = 0; for (const v of x) ss += v * v; const inv = 1 / Math.sqrt(ss / x.length + eps); return x.map((v, i) => v * inv * w[i]); };
    const mm = (M, x, dO, dI) => { const o = new Float32Array(dO); for (let r = 0; r < dO; r++) { let a = 0; for (let c = 0; c < dI; c++) a += M[r * dI + c] * x[c]; o[r] = a; } return o; };
    const rope = (v, heads, pos) => { const half = hd / 2; for (let h = 0; h < heads; h++) for (let i = 0; i < half; i++) {
      const ang = pos * Math.pow(theta, -(2 * i) / hd), c = Math.cos(ang), s = Math.sin(ang);
      const a = v[h * hd + i], b = v[h * hd + i + half]; v[h * hd + i] = a * c - b * s; v[h * hd + i + half] = b * c + a * s; } };
    const kc = W.layers.map(() => []), vc = W.layers.map(() => []);
    let logits = null;
    ids.forEach((id, pos) => {
      const x = Float32Array.from(W.embed.subarray(id * dim, (id + 1) * dim));
      W.layers.forEach((Wl, li) => {
        const xn = rms(x, Wl.inNorm);
        const q = mm(Wl.q, xn, qDim, dim), k = mm(Wl.k, xn, kvDim, dim), v = mm(Wl.v, xn, kvDim, dim);
        rope(q, nH, pos); rope(k, nKV, pos); kc[li].push(k); vc[li].push(v);
        const att = new Float32Array(qDim), T = kc[li].length, sc = new Float32Array(T);
        for (let h = 0; h < nH; h++) {
          const kvH = Math.floor(h / (nH / nKV));
          for (let t = 0; t < T; t++) { let a = 0; for (let i = 0; i < hd; i++) a += q[h * hd + i] * kc[li][t][kvH * hd + i]; sc[t] = a / Math.sqrt(hd); }
          let mx = -Infinity; for (let t = 0; t < T; t++) mx = Math.max(mx, sc[t]);
          let sum = 0; for (let t = 0; t < T; t++) { sc[t] = Math.exp(sc[t] - mx); sum += sc[t]; }
          for (let t = 0; t < T; t++) for (let i = 0; i < hd; i++) att[h * hd + i] += (sc[t] / sum) * vc[li][t][kvH * hd + i];
        }
        const pr = mm(Wl.o, att, dim, qDim); for (let i = 0; i < dim; i++) x[i] += pr[i];
        const xn2 = rms(x, Wl.postNorm);
        const g = mm(Wl.gate, xn2, inter, dim), u = mm(Wl.up, xn2, inter, dim);
        for (let i = 0; i < inter; i++) g[i] = (g[i] / (1 + Math.exp(-g[i]))) * u[i];
        const d = mm(Wl.down, g, dim, inter); for (let i = 0; i < dim; i++) x[i] += d[i];
      });
      logits = mm(W.embed, rms(x, W.finalNorm), vocab, dim);
    });
    return logits;
  };

  const results = {};
  for (const kind of ["f32", "q8", "q4"]) {
    const conv = (m, r, c) => kind === "f32" ? { entry: { kind: "f32", data: m }, deq: m }
      : kind === "q8" ? (() => { const q = quantizeQ8(m); return { entry: { kind: "q8", ...q, shape: [r, c] }, deq: dequantQ8(q, m.length) }; })()
      : (() => { const q = quantizeQ4(m); return { entry: { kind: "q4", ...q, shape: [r, c] }, deq: dequantQ4(q, m.length) }; })();
    const E = conv(raw.embed, vocab, dim);
    const weights = { embed: E.entry, finalNorm: { kind: "f32", data: raw.finalNorm }, layers: [] };
    const deq = { embed: E.deq, finalNorm: raw.finalNorm, layers: [] };
    for (const l of raw.layers) {
      const Q = conv(l.q, qDim, dim), K = conv(l.k, kvDim, dim), V = conv(l.v, kvDim, dim), O = conv(l.o, dim, qDim);
      const G2 = conv(l.gate, inter, dim), U = conv(l.up, inter, dim), D = conv(l.down, dim, inter);
      weights.layers.push({ inNorm: { kind: "f32", data: l.inNorm }, postNorm: { kind: "f32", data: l.postNorm },
        q: Q.entry, k: K.entry, v: V.entry, o: O.entry, gate: G2.entry, up: U.entry, down: D.entry });
      deq.layers.push({ inNorm: l.inNorm, postNorm: l.postNorm, q: Q.deq, k: K.deq, v: V.deq, o: O.deq, gate: G2.deq, up: U.deq, down: D.deq });
    }
    const ref = cpuForward(deq);
    let gpu = null, err = null;
    try {
      const eng = await DenseEngine.create({ device, cfg, weights, maxSeq: 8 });
      for (const id of ids) gpu = await eng.forwardToken(id);
    } catch (e) { err = e.message; }
    let maxDiff = Infinity, nan = false;
    if (gpu) {
      maxDiff = 0; let scale = 1e-6;
      for (let i = 0; i < ref.length; i++) { if (!Number.isFinite(gpu[i])) nan = true; maxDiff = Math.max(maxDiff, Math.abs(gpu[i] - ref[i])); scale = Math.max(scale, Math.abs(ref[i])); }
      maxDiff /= scale;
    }
    results[kind] = { ok: !err && !nan && maxDiff < 2e-3, maxDiff: +maxDiff.toFixed(5), nan, err };
  }
  const ok = results.f32.ok && results.q8.ok && results.q4.ok;
  const detail = ["f32", "q8", "q4"].map(k => `${k}:${results[k].ok ? "ok" : (results[k].err ? "ERR " + results[k].err : results[k].nan ? "NaN" : "diff " + results[k].maxDiff)}`).join(" · ");
  return { ok, ...results, detail };
}

// ---------- per-kernel micro-tests ----------
// Builds a tiny qwen-shaped engine (QK-norm, head_dim != dim/nH) and checks
// every kernel individually against CPU math. Returns the first failing kernel.
export async function kernelMicroTests(device) {
  const cfg = { hidden_size: 128, num_attention_heads: 4, num_key_value_heads: 2, head_dim: 64,
    intermediate_size: 256, vocab_size: 512, num_hidden_layers: 1, rms_norm_eps: 1e-6, rope_theta: 1000000 };
  const dim = 128, nH = 4, nKV = 2, hd = 64, qDim = 256, kvDim = 128, inter = 256, vocab = 512, eps = 1e-6, theta = 1e6;
  let seed = 777;
  const rnd = () => { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return seed / 0x7fffffff - 0.5; };
  const mat = (r, c, s) => { const a = new Float32Array(r * c); for (let i = 0; i < a.length; i++) a[i] = rnd() * s; return a; };
  const vec1 = (n) => { const a = new Float32Array(n); for (let i = 0; i < n; i++) a[i] = 1 + rnd() * 0.2; return a; };
  const q8 = (m, r, c) => { const q = quantizeQ8(m); return { entry: { kind: "q8", ...q, shape: [r, c] }, deq: dequantQ8(q, m.length) }; };
  const Wq = q8(mat(qDim, dim, 0.3), qDim, dim), Wk = q8(mat(kvDim, dim, 0.3), kvDim, dim), Wv = q8(mat(kvDim, dim, 0.3), kvDim, dim);
  const Wo = q8(mat(dim, qDim, 0.3), dim, qDim), Wg = q8(mat(inter, dim, 0.3), inter, dim), Wu = q8(mat(inter, dim, 0.3), inter, dim), Wd = q8(mat(dim, inter, 0.3), dim, inter);
  const inNorm = vec1(dim), postNorm = vec1(dim), qNorm = vec1(hd), kNorm = vec1(hd), finalNorm = vec1(dim);
  const E = q8(mat(vocab, dim, 1), vocab, dim);
  const weights = { embed: E.entry, finalNorm: { kind: "f32", data: finalNorm }, layers: [{
    inNorm: { kind: "f32", data: inNorm }, postNorm: { kind: "f32", data: postNorm },
    qNorm: { kind: "f32", data: qNorm }, kNorm: { kind: "f32", data: kNorm },
    q: Wq.entry, k: Wk.entry, v: Wv.entry, o: Wo.entry, gate: Wg.entry, up: Wu.entry, down: Wd.entry }] };
  const eng = await DenseEngine.create({ device, cfg, weights, maxSeq: 8 });
  const BG = eng.layerBGs[0];
  const stage = (n) => device.createBuffer({ size: n * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
  const run = (fn) => { const enc = device.createCommandEncoder(); const p = enc.beginComputePass(); fn(p); p.end(); device.queue.submit([enc.finish()]); };
  // q/k/v may be views into one merged buffer (DenseEngine mergeQKV): read and write at the view's offset
  const rb = async (buf, n) => {
    const [b, off] = DenseEngine._raw(buf), st = stage(n);
    const enc = device.createCommandEncoder(); enc.copyBufferToBuffer(b, off, st, 0, n * 4); device.queue.submit([enc.finish()]);
    await st.mapAsync(GPUMapMode.READ); const out = Float32Array.from(new Float32Array(st.getMappedRange(), 0, n)); st.unmap(); return out;
  };
  const wr = (buf, arr) => { const [b, off] = DenseEngine._raw(buf); device.queue.writeBuffer(b, off, arr); };
  const cmp = (got, want) => {
    let md = 0, sc = 1e-6, nan = false;
    for (let i = 0; i < want.length; i++) { if (!Number.isFinite(got[i])) nan = true; md = Math.max(md, Math.abs(got[i] - want[i])); sc = Math.max(sc, Math.abs(want[i])); }
    return nan ? "NaN" : (md / sc < 2e-3 ? "ok" : "diff " + (md / sc).toFixed(4));
  };
  const cpuRms = (x, w, n, off = 0) => { let ss = 0; for (let i = 0; i < n; i++) ss += x[off + i] * x[off + i]; const inv = 1 / Math.sqrt(ss / n + eps); const o = new Float32Array(n); for (let i = 0; i < n; i++) o[i] = x[off + i] * inv * w[i]; return o; };
  const cpuMm = (M, x, dO, dI) => { const o = new Float32Array(dO); for (let r = 0; r < dO; r++) { let a = 0; for (let c = 0; c < dI; c++) a += M[r * dI + c] * x[c]; o[r] = a; } return o; };
  const results = {};
  const X = mat(1, dim, 1), Q = mat(1, qDim, 1), G2 = mat(1, inter, 1), U = mat(1, inter, 1), T2 = mat(1, dim, 1);
  try {
    // rmsnorm
    wr(eng.x, X); run((p) => eng._dispatch(p, "rmsnorm", BG.norm1, 256, 256));
    results.rmsnorm = cmp(await rb(eng.xn, dim), cpuRms(X, inNorm, dim));
    // matvec_q8 (q projection from xn)
    const xn = cpuRms(X, inNorm, dim);
    run((p) => eng._dispatchOp(p, BG.q));
    results.matvec_q8 = cmp(await rb(eng.q, qDim), cpuMm(Wq.deq, xn, qDim, dim));
    // head_norm on q
    wr(eng.q, Q); run((p) => eng._dispatch(p, "head_norm", BG.qNorm, nH, 32));
    const hnWant = new Float32Array(qDim);
    for (let h = 0; h < nH; h++) hnWant.set(cpuRms(Q, qNorm, hd, h * hd), h * hd);
    results.head_norm = cmp(await rb(eng.q, qDim), hnWant);
    // rope at pos 3
    wr(eng.q, Q); eng._setFrame(3, 4); run((p) => eng._dispatch(p, "rope", eng.bgRopeQ, nH * hd / 2));
    const rWant = Float32Array.from(Q);
    for (let h = 0; h < nH; h++) for (let i = 0; i < hd / 2; i++) {
      const ang = 3 * Math.pow(theta, -(2 * i) / hd), c = Math.cos(ang), s = Math.sin(ang);
      const a = Q[h * hd + i], b = Q[h * hd + i + hd / 2];
      rWant[h * hd + i] = a * c - b * s; rWant[h * hd + i + hd / 2] = b * c + a * s;
    }
    results.rope = cmp(await rb(eng.q, qDim), rWant);
    // softmax over seqLen 5 (scores laid out h*maxSeq + t)
    const S = new Float32Array(nH * 8); for (let i = 0; i < S.length; i++) S[i] = rnd() * 4;
    wr(eng.scores, S); eng._setFrame(4, 5); run((p) => eng._dispatch(p, "attn_softmax", BG.softmax, nH, 1));
    const smWant = Float32Array.from(S);
    for (let h = 0; h < nH; h++) { let mx = -Infinity; for (let t = 0; t < 5; t++) mx = Math.max(mx, S[h * 8 + t]); let sum = 0; for (let t = 0; t < 5; t++) { smWant[h * 8 + t] = Math.exp(S[h * 8 + t] - mx); sum += smWant[h * 8 + t]; } for (let t = 0; t < 5; t++) smWant[h * 8 + t] /= sum; }
    const smGot = await rb(eng.scores, nH * 8);
    let smOk = "ok"; for (let h = 0; h < nH; h++) for (let t = 0; t < 5; t++) { const g = smGot[h * 8 + t], w = smWant[h * 8 + t]; if (!Number.isFinite(g)) smOk = "NaN"; else if (Math.abs(g - w) > 2e-3 && smOk === "ok") smOk = "diff"; }
    results.attn_softmax = smOk;
    // silu_mul
    wr(eng.g, G2); wr(eng.u, U); run((p) => eng._dispatch(p, "silu_mul", eng.bgSilu, inter));
    results.silu_mul = cmp(await rb(eng.g, inter), G2.map((g, i) => (g / (1 + Math.exp(-g))) * U[i]));
    // add_res
    wr(eng.x, X); wr(eng.tmpDim, T2); run((p) => eng._dispatch(p, "add_res", eng.bgAddTmp, dim));
    results.add_res = cmp(await rb(eng.x, dim), X.map((v, i) => v + T2[i]));
    // full attention block end-to-end at pos 0 via forwardToken (covers scores/attn_out/kv copy)
    eng.pos = 0; const lg = await eng.forwardToken(5);
    results.full_layer = lg.some((v) => !Number.isFinite(v)) ? "NaN" : "ok";
  } catch (e) { results.error = e.message; }
  const firstFail = Object.entries(results).find(([, v]) => v !== "ok");
  return { ok: !firstFail, firstFail: firstFail ? firstFail[0] + ": " + firstFail[1] : null,
    detail: Object.entries(results).map(([k, v]) => `${k}:${v}`).join(" · ") };
}
