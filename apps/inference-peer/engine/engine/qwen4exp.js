// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/qwen4exp.js (MIT; see LICENSE-ENGINE).
// Qwen3.8-Flash-Next (llama.cpp arch "qwen4exp") on WebGPU: the decode path, weights resident; QSA past 2051 cells.
// Roadmap 33 (roadmap/33-qwen38-flash.md) PR plan item 4.
//
// A separate engine module, not a mode of Qwen35Engine: the residual is 4 streams wide (hyper-connections), every
// norm around the blocks is an HC mixer, layer 1 adds the n-gram (PLE) injection, the MoE has 512 experts / top-10
// and there is no output norm, so every one of Qwen35Engine's fused decode paths would need an HC branch. This file
// keeps the math in one readable place, one token at a time, with its own kernels (engine/wgsl/qwen4exp.js); the
// batched prefill / verify passes (engine/qwen4exp_batch.js, PR plan item 5) run on this engine's state and weight
// buffers, and MTP, offload and room plumbing (items 6-8) build on it.
//
// One token (forwardToken):
//   R = 4 copies of the token embedding                               (hc_init)
//   per layer: [PLE on its layer] -> HC mix -> DeltaNet or gated attention -> HC combine
//                                 -> HC mix -> MoE (top-10 of 512 + shared expert) -> HC combine
//   head: HC mix (output_hc_*, no inject) -> output.weight -> greedy id on the GPU (and the logits on request)
//
// The math is tests/reference/ref_q38flash.mjs's with f32 activations (its --act f32 mode). Attention: QSA
// (engine/qwen4exp_qsa.js, PR plan item 9) when the full-attention layers' indexer weights are loaded: the indexer
// state is kept from position 0, attention stays dense while a step has <= 2051 cells (QSA selects them all there)
// and reads the selected cells past that. Without the indexer weights (qsa: false, the room loader's dense phase)
// attention is dense and the context is capped at 2048.
//
// Long context (docs/kernels.md "Flash long context"): kv "f16" keeps the 12 attention layers' K / V caches as f16
// (half of f32's 48 KB per token; the kernels read them exactly and do the f32 arithmetic in the same order, so
// decode, prefill and verify still agree bit for bit), and QSA's raw indexer keys are a ring of qsaRing rows (pooling
// only reads the blocks a pass completes) instead of one row per position. qwen4expMemory (engine/qwen4exp_mem.js)
// gives the state's bytes per context length; stateBytes() is what this engine allocated, _stateParts(pos) what a
// checkpoint copies.
//
// Ops ({ pipe, bg, wg }, _encode) may instead carry dyn(pos, nCols) -> workgroups | null (skipped): QSA's ops, whose
// sizes and dense / gathered choice depend on the step's position. bg may be a list (groups 0, 1, ..).
//
// Expert offload (roadmap 33 item 7; engine/expert_store.js): layers whose routed experts the loader parked
// (qwen4expWeights(..., { experts })) split the token's command buffer after their router, as Qwen35Engine's
// _cutAt / _cutResolve do: the 10 ids are read back, the store copies the misses into the layer's slot pool and the
// same moe_gu / moe_dn kernels run on the pool with slot ids (B.offIds) in place of expert ids. One token at a time,
// a cut holds at most K experts, so the store is built without the whole-layer prefill region. moeTrace: the router's
// ids and weights of every layer are kept per token for readMoeTrace() (tests/trace_flash.js).
// The batched prefill / verify passes (engine/qwen4exp_batch.js) build their resident layers' expert ops with _wq,
// which always refuses a parked entry (notParked). An offloaded layer's expert ops never go through _wq: they bind
// the store's whole-layer region (regionEntry(i, part), the original expert ids), which exists only when the engine
// was created with expertRegion: true (Qwen4ExpBatch.create refuses offloaded layers without it). Per offloaded layer
// and pass (Qwen4ExpBatch._encodeLayers): end the compute pass after the router, then ExpertStore.loadLayer (wide
// passes) or loadExperts (narrow ones, after reading the ids back) into the region. this.offLayers lists the
// offloaded layers.
import { qwen4expConfig } from "./gguf.js";
import { NgramState } from "./ngram.js";
import { notParked } from "./expert_store.js";
import * as W from "./wgsl/qwen4exp.js";
import { attnDecMcWGSL } from "./wgsl/qwen4exp_batch.js";
import { qsaCheckConfig, qsaFlashPipelines, qsaShared, qsaScratch, qsaSetFrame, qsaLayerState, qsaBind, qsaStateOps, qsaSelectOps, qsaSparse,
  QSA_SEL_STRIDE, QSA_RAW_RING, QSA_KPOOL, QSA_ID } from "./qwen4exp_qsa.js";
import { qwen4expMemory } from "./qwen4exp_mem.js";

const U = GPUBufferUsage;
const fr = Math.fround;

export class Qwen4ExpEngine {
  static _ids = 0;
  static async create(opts) { const e = new Qwen4ExpEngine(); await e._init(opts); return e; }

  // weights: qwen4expWeights' structure (engine/gguf.js), each matrix with entry.gpu (streamed) or its CPU bytes.
  // The engine consumes them: fused projections replace their parts' GPU buffers, and the f32 tensors it joins
  // drop their CPU copies once uploaded, so one weights object builds one engine (a second create throws).
  // ngram: an NgramTable (engine/ngram.js) for the PLE layer's rows; required when layerRange holds a PLE layer.
  // layerRange [lo, hi): the trunk layers this engine runs; hasEmbed / hasHead as Qwen35Engine.
  // qsa: run QSA (default: when every full-attention layer in range has its indexer weights); maxSeq may then pass 2048.
  // moeTrace: keep every layer's router ids and weights per token (readMoeTrace). expertRegion: build the expert store
  // with its whole-layer region (ExpertStore.loadLayer / loadExperts / regionEntry), for a batched engine that routes
  // offloaded layers; off by default, since decode alone never needs it and its 1.4-1.8 GB goes to the pools instead.
  // kv: the attention layers' K / V cache format, "f32" (the default) or "f16" (half the memory). qsaRing: QSA's raw
  // indexer keys as a ring of this many rows (a power of two; batched passes of up to qsaRing - 3 columns), 0 to keep
  // one row per position.
  async _init({ device, meta, weights, layerRange = null, hasEmbed = true, hasHead = true, maxSeq = 2048, ngram = null, qsa = null, moeTrace = false, expertRegion = false,
    kv = "f32", qsaRing = QSA_RAW_RING }) {
    const cfg = this.cfg = qwen4expConfig(meta);
    this.device = device;
    const [lo, hi] = layerRange || [0, cfg.L];
    Object.assign(this, { lo, hi, hasEmbed, hasHead, pos: 0 });
    const qLayers = [];
    for (let i = lo; i < hi; i++) if (cfg.qsa[i]) qLayers.push(weights.layers[i - lo]);
    this.qsa = qsa ?? (qLayers.length > 0 && qLayers.every((L) => L.indexer));
    if (this.qsa && qLayers.some((L) => !L.indexer)) throw new Error("qwen4exp: qsa needs the indexer weights (qwen4expWeights qsa: true)");
    if (this.qsa) qsaCheckConfig(cfg);
    // which steps / passes read QSA's selected cells: those with a column past 2051 cells (qsaAlways: every one, for
    // the QSA == dense check, tests/test_q38flash_batch.js)
    this.qsaAlways = false;
    this.sparse = (pos, n) => this.qsaAlways || qsaSparse(pos, n);
    if (maxSeq > 2048 && qLayers.length && !this.qsa) throw new Error("qwen4exp: without QSA (the indexer weights) the context is capped at 2048 positions");
    this.maxSeq = maxSeq;
    W.kvCheck(kv);
    this.kv = kv;
    this.qsaRing = this.qsa ? qsaRing : 0;
    // each attention layer's K and V are one buffer each, bound whole: it must fit the device's binding limit (the
    // test devices ask for the adapter's; a browser's default is 128 MiB: 65536 positions in f32, 131072 in f16)
    this.kvBufBytes = maxSeq * cfg.nKVHeads * cfg.headDim * W.kvBytes(kv);
    if (qLayers.length) {
      const lim = Math.min(device.limits.maxStorageBufferBindingSize, device.limits.maxBufferSize);
      if (this.kvBufBytes > lim) throw new Error(`qwen4exp: context ${maxSeq} needs a ${(this.kvBufBytes / 2 ** 20).toFixed(0)} MiB ${kv} K / V buffer per attention layer; `
        + `this GPU device binds at most ${(lim / 2 ** 20).toFixed(0)} MiB (a shorter context${kv === "f32" ? ", or kv: \"f16\"" : ""})`);
    }
    const d = cfg.dim, hc = cfg.hc, HD = cfg.hDim, ss = cfg.ssm;
    // shapes the kernels hard-code (engine/wgsl/qwen4exp.js): the router's workgroup arrays, DeltaNet's 128 lanes
    if (cfg.nExp > 1024 || cfg.K > 32) throw new Error(`qwen4exp: the router kernel holds at most 1024 experts / top-32, not ${cfg.nExp} / ${cfg.K}`);
    if (ss.stateSize !== 128) throw new Error(`qwen4exp: the DeltaNet kernels assume state size 128, not ${ss.stateSize}`);
    this.dims = { dim: d, hDim: HD, vocab: hasHead ? weights.head.shape[0] : 0 };
    const pleLayers = (cfg.ple?.layers || []).filter((l) => l >= lo && l < hi);
    if (pleLayers.length && !ngram) throw new Error("qwen4exp: layers " + pleLayers + " need the n-gram table (ngram: an NgramTable)");
    this.ngram = ngram;
    this.ngs = ngram ? new NgramState(ngram.cfg) : null;
    this.embed = weights.embed || null;
    this.nVocab = this.embed ? (this.embed.shape?.[0] ?? this.embed.data.length / d) : 0;

    // ---- pipelines
    const pipe = async (code, label) => device.createComputePipelineAsync({ layout: "auto", label,
      compute: { module: device.createShaderModule({ code, label }), entryPoint: "main" } });
    const kinds = ["q4", "q8", "f32"];
    const P = this.P = {};
    await Promise.all([
      // the GEMVs come in one variant per row length (lanes per row): every one the model's shapes need
      ...kinds.flatMap((k) => [8, 16, 32, 64, 128, 256].flatMap((n) => (k === "f32" ? [1] : [1, 4]).map(async (R) => {
        P[`gemv_${k}_${n}_${R}`] = await pipe(W.gemvWGSL(k, n, R), `gemv_${k}_${n}_${R}`);
      }))),
      ...["q4", "q8"].map(async (k) => { P["gu_" + k] = await pipe(W.moeGuWGSL(k), "moe_gu_" + k); P["dn_" + k] = await pipe(W.moeDnWGSL(k), "moe_dn_" + k); }),
      ...["q4", "q8"].flatMap((k) => [8, 16, 32].map(async (n) => { P[`gemvmix_${k}_${n}`] = await pipe(W.gemvWGSL(k, n, 4, true), `gemvmix_${k}_${n}`); })),
      ...Object.entries({ route: W.MOE_ROUTE_WGSL, hcSite: W.HC_SITE_WGSL, hcMix: W.HC_MIX_WGSL, hcComb: W.HC_COMBINE_WGSL,
        dnConv: W.DN_CONV_WGSL, dnDelta: W.DN_DELTA_WGSL,
        attnPrep: W.attnPrepWGSL(kv), attnDec: W.attnDecWGSL(kv), ple: W.PLE_WGSL, am1: W.ARGMAX1_WGSL, am2: W.ARGMAX2_WGSL })
        .map(async ([k, code]) => { P[k] = await pipe(code, k); }),
      // QSA: the gathered attention over the selected rows (one column), and the indexer kernels
      ...(this.qsa ? [(async () => { P.attnGather = await pipe(attnDecMcWGSL({ gather: QSA_SEL_STRIDE, kv }), "attn_gather"); })(),
        (async () => { this.qsaK = await qsaFlashPipelines(device, { rawRing: this.qsaRing }); })()] : []),
    ]);

    // ---- scratch (shared by every layer: one token's ops run in order)
    const buf = (n, extra = 0) => device.createBuffer({ size: Math.max(16, Math.ceil(n * 4 / 16) * 16), usage: U.STORAGE | U.COPY_SRC | U.COPY_DST | extra });
    const convDim = 2 * ss.nKHeads * ss.stateSize + ss.inner, keyDim = ss.nKHeads * ss.stateSize;
    const K = cfg.K, nH = cfg.nHeads, nKV = cfg.nKVHeads, hd = cfg.headDim;
    this.dn = { convDim, keyDim };
    const B = this.B = {
      R: buf(HD), xn: buf(HD), lo: buf(cfg.hcRank), gate: buf(HD), mixed: buf(d), inj: buf(hc), out: buf(d),
      qkvz: buf(convDim + ss.inner), ba: buf(2 * ss.nVHeads), co: buf(convDim), o: buf(ss.inner), go: buf(ss.inner),
      qkvA: buf(2 * nH * hd + 2 * nKV * hd), qn: buf(nH * hd), ao: buf(nH * hd),
      logits: buf(cfg.nExp + 1), ids: buf(K), wts: buf(K), h: buf(K * cfg.expFF), moe: buf(d),
      shIds: buf(1), hs: buf(cfg.shFF), sh: buf(d),
      frame: device.createBuffer({ size: W.FRAME_FLOATS * 4, usage: U.UNIFORM | U.COPY_DST }),
    };
    if (pleLayers.length) Object.assign(B, { emb: buf(d), pk: buf(HD), pv: buf(d), dbg: buf(2 * HD + hc) });
    device.queue.writeBuffer(B.shIds, 0, new Uint32Array(4));
    if (this.qsa) {   // QSA's uniforms (shared with the batched passes) and its one-column scratch
      this.qsaSh = qsaShared(device, { dim: d, maxSeq, eps: cfg.eps, theta: cfg.ropeTheta, nRot: cfg.ropeDims });
      this.qsaS = qsaScratch(device, { maxCols: 1, maxBlocks: this.qsaSh.maxBlocks });
    }

    // ---- expert offload: the store the loader parked these layers' experts in (one store for all of them)
    this.experts = null;
    const offL = weights.layers.filter((L) => L.expGate?.offload);
    if (offL.length) {
      const st = offL[0].expGate.store;
      if (!offL.every((L) => L.expUp?.offload && L.expDown?.offload && L.expGate.store === st && L.expUp.store === st && L.expDown.store === st))
        throw new Error("qwen4exp: expert offload: a layer's experts are only partly parked, or in two stores");
      st.build({ K, region: expertRegion === "shared" ? "shared" : !!expertRegion });   // ("shared": the region borrowed from the pools)
      await st.checkAlloc();
      this.experts = st;
      this.offLayers = new Set(offL.map((L) => L.expGate.layer));
      B.offIds = buf(K + 1);   // the slot ids of the cut's experts (ExpertStore.plan's remap: K, then the shared slot)
      this.offStage = device.createBuffer({ size: Math.ceil(K * 4 / 16) * 16, usage: U.MAP_READ | U.COPY_DST });
    }
    // ---- router trace: every layer's ids and weights, per token
    this.moeTraceOn = !!moeTrace;
    if (this.moeTraceOn) {
      const n = (hi - lo) * K;
      B.trace = buf(2 * n);
      this.traceStage = device.createBuffer({ size: Math.ceil(2 * n * 4 / 16) * 16, usage: U.MAP_READ | U.COPY_DST });
    }

    // ---- weights to the GPU (entry.gpu when the loader streamed it; f32 tensors are uploaded here)
    const wq = (e, what) => {
      if (!e) throw new Error("qwen4exp: missing weight " + what);
      if (e.kind === "f32") {
        if (!e.gpuF32) {
          const src = e.data;
          const b = device.createBuffer({ size: Math.ceil(src.byteLength / 16) * 16, usage: U.STORAGE | U.COPY_DST, mappedAtCreation: true });
          new Float32Array(b.getMappedRange()).set(src);
          b.unmap();
          e.gpuF32 = b;
        }
        return { kind: "f32", buf: e.gpuF32, n: e.data.length };
      }
      notParked(e, what);   // an offloaded layer's experts have no GPU copy: engines that do not route them refuse
      if (e.gpu?.fused) throw new Error(`qwen4exp: weight ${what || ""} was fused into another engine's buffer (one weights object builds one engine)`);
      if (!e.gpu) {
        const mk = (data) => {
          const u8 = new Uint8Array(data.buffer, data.byteOffset, data.byteLength);
          const b = device.createBuffer({ size: Math.ceil(u8.byteLength / 4) * 4, usage: U.STORAGE | U.COPY_DST | U.COPY_SRC, mappedAtCreation: true });
          new Uint8Array(b.getMappedRange()).set(u8);
          b.unmap();
          return b;
        };
        e.gpu = { kind: e.kind, qs: mk(e.qs), sc: mk(e.scales) };
      }
      return { kind: e.kind, qs: e.gpu.qs, sc: e.gpu.sc, shape: e.shape };
    };

    // f32 tensors end to end, as one entry (kept per pair: the cache key is the entries themselves). The joined
    // copies and their parts' CPU data are released at the end of _init, once uploaded (~290 MB on the 48 layers)
    const cats = new Map(), catParts = new Set();
    const cat = (...es) => {
      const k = es.map((e) => (e.__id ||= ++Qwen4ExpEngine._ids)).join(",");
      if (!cats.has(k)) {
        if (es.some((e) => !e.data)) throw new Error("qwen4exp: an f32 weight was consumed by another engine (one weights object builds one engine)");
        es.forEach((e) => catParts.add(e));
        const data = new Float32Array(es.reduce((n, e) => n + e.data.length, 0));
        let o = 0;
        for (const e of es) { data.set(e.data, o); o += e.data.length; }
        cats.set(k, { kind: "f32", data });
      }
      return cats.get(k);
    };

    // ---- op builders: an op is { pipe, bg, wg: [x, y, z], tap?: [name, buffer, count, "f32" | "u32"] }
    const uni = (fields) => {
      const n = Math.max(4, Math.ceil(fields.length / 4) * 4), ab = new ArrayBuffer(n * 4), dv = new DataView(ab);
      fields.forEach((v, i) => (typeof v === "number" ? dv.setUint32(i * 4, v, true) : dv.setFloat32(i * 4, v.f, true)));
      const b = device.createBuffer({ size: ab.byteLength, usage: U.UNIFORM | U.COPY_DST, mappedAtCreation: true });
      new Uint8Array(b.getMappedRange()).set(new Uint8Array(ab));
      b.unmap();
      return b;
    };
    // list: buffers, or { buffer, offset, size } views (offsets multiples of 256 bytes)
    const bg = (p, list) => device.createBindGroup({ layout: p.getBindGroupLayout(0),
      entries: list.map((b, i) => (b ? { binding: i, resource: b.buffer ? b : { buffer: b } } : null)).filter(Boolean) });
    const view = (buffer, off, n) => ({ buffer, offset: off * 4, size: n * 4 });
    // Quantized matrices of one kind and width stacked into one (rows end to end), so projections that read the
    // same input run as one GEMV. The engine owns its weights: the parts' GPU buffers are freed and their entries
    // marked fused (wq on one then throws, rather than hand back the stacked buffer with the part's shape). null
    // when the kinds or widths differ.
    const fuseRows = (...es) => {
      const ms = es.map((e, j) => wq(e, "fused part " + j));
      if (ms.some((m) => m.kind === "f32" || m.kind !== ms[0].kind || m.shape[1] !== ms[0].shape[1])) return null;
      const cols = ms[0].shape[1], nb = cols / 32, qb = ms[0].kind === "q4" ? 16 : 32;
      const rows = ms.reduce((n, m) => n + m.shape[0], 0);
      if (ms.some((m) => (m.shape[0] * nb) % 2)) return null;   // scales are packed two per word
      const qs = device.createBuffer({ size: rows * nb * qb, usage: U.STORAGE | U.COPY_DST | U.COPY_SRC });
      const sc = device.createBuffer({ size: rows * nb * 2, usage: U.STORAGE | U.COPY_DST | U.COPY_SRC });
      const enc = device.createCommandEncoder();
      let r = 0;
      for (const m of ms) {
        enc.copyBufferToBuffer(m.qs, 0, qs, r * nb * qb, m.shape[0] * nb * qb);
        enc.copyBufferToBuffer(m.sc, 0, sc, r * nb * 2, m.shape[0] * nb * 2);
        r += m.shape[0];
      }
      device.queue.submit([enc.finish()]);
      for (const e of es) { e.gpu.qs.destroy(); e.gpu.sc.destroy(); e.gpu = { kind: e.kind, qs, sc, fused: true }; }
      return { kind: ms[0].kind, qs, sc, shape: [rows, cols] };
    };
    const f = (v) => ({ f: v });
    const eps = cfg.eps;
    // y[yOff..] = mat x (mode 1: silu(acc * scale)); dOut / dIn from the weight's shape unless given
    // f32 entries carry no shape: pass dIn (dOut is then their length / dIn)
    const gemv = (M, x, y, { dIn = M.shape?.[1], dOut = M.shape?.[0] ?? M.n / dIn, rowOff = 0, xOff = 0, yOff = 0, mode = 0, scale = 0, tap } = {}) => {
      if (!(dIn > 0 && dOut > 0 && Number.isInteger(dOut))) throw new Error(`qwen4exp: gemv shape ${dOut} x ${dIn}`);
      const lanes = W.gemvLanes(M.kind, dOut, dIn), R = W.gemvRows(M.kind, dOut), p = P[`gemv_${M.kind}_${lanes}_${R}`];
      const u = uni([dOut, dIn, rowOff, xOff / 4, yOff, mode, f(scale), 0]);
      const g = M.kind === "f32" ? bg(p, [M.buf, null, x, y, u]) : bg(p, [M.qs, M.sc, x, y, u]);
      const nwg = Math.ceil(dOut / (256 / lanes * R));
      return { pipe: p, bg: g, wg: [Math.min(nwg, 32768), Math.ceil(nwg / 32768), 1], tap };
    };
    const ewg = (n) => [Math.ceil(n / 256), 1, 1];
    // HC sites. A layer has two (before its token mixer, before its MoE) and the head one; site h's inject partials go
    // to inj[h % 2], so a site that folds the previous site's combine reads the one buffer and writes the other.
    // A half layer is { pre (the PLE ops), site, rest, comb }; _halfOps lays it out unfused (each half on its own:
    // the per-layer check forces R between halves) or fused (forwardToken: each combine folded into the next site).
    if (hc !== 4) throw new Error("qwen4exp: the HC site kernel is written for 4 streams");
    B.inj = [buf(hc * hc), buf(hc * hc)];
    let nSite = 0;
    // site: { w (norm), injW (null: the head), tag } -> the mixer after the norm: down, up (+ mix)
    const hcMixRest = (hw, tag, nm) => {
      const ops = [gemv(wq(hw.down, "hc down"), B.xn, B.lo, { dIn: HD, mode: 1, scale: 1 / hc })];
      const up = wq(hw.up, "hc up");
      if (up.kind !== "f32") {   // the up projection and the mix as one kernel (rows i, d + i, 2d + i, 3d + i per lane group)
        const lanes = W.gemvLanes(up.kind, HD, cfg.hcRank), p = P[`gemvmix_${up.kind}_${lanes}`];
        const u = uni([HD, cfg.hcRank, 0, 0, 0, 0, f(0), d]);
        ops.push({ pipe: p, bg: bg(p, [up.qs, up.sc, B.lo, B.gate, u, B.xn, B.mixed]), wg: [Math.ceil(d / (256 / lanes)), 1, 1],
          taps: [["hc_gate" + tag, B.gate, HD], [nm, B.mixed, d]] });
      } else {
        ops.push(gemv(up, B.lo, B.gate, { dIn: cfg.hcRank, tap: ["hc_gate" + tag, B.gate, HD] }));
        ops.push({ pipe: P.hcMix, bg: bg(P.hcMix, [B.xn, B.gate, B.mixed, uni([d, hc, 0, 0])]), wg: ewg(d), tap: [nm, B.mixed, d] });
      }
      return ops;
    };
    const site = (hw, tag, inject = true) => ({ h: nSite++, w: wq(hw.norm, "hc norm").buf, injW: inject ? wq(hw.inject, "hc inject").buf : null, tag });
    // the site's kernel, folding comb (the previous half's { a, b, name, h }) when given
    const siteOp = (S, comb) => {
      const out = B.inj[S.h % 2], prev = comb ? B.inj[comb.h % 2] : B.inj[(S.h + 1) % 2];
      const u = uni([d, hc, f(eps), comb ? (comb.b ? 2 : 1) : 0, S.injW ? 1 : 0, 0, 0, 0]);
      const taps = [];
      if (comb) taps.push([comb.name, B.R, HD]);
      if (S.injW) taps.push(["hc_inject" + S.tag, out, hc * hc, "sum4"]);
      return { pipe: P.hcSite, bg: bg(P.hcSite, [B.R, comb ? comb.a : B.mixed, comb?.b || B.mixed, prev, S.w, B.xn, S.injW || S.w, out, u]), wg: [hc, 1, 1], taps };
    };
    const combOp = (C) => ({ pipe: P.hcComb, bg: bg(P.hcComb, [B.R, C.a, C.b || C.a, B.inj[C.h % 2], uni([d, hc, C.b ? 1 : 0, 0])]), wg: ewg(HD), tap: [C.name, B.R, HD] });
    this._halfOps = (halves, fused) => {
      const lists = halves.map(() => []);
      halves.forEach((H, k) => {
        const prev = k > 0 ? halves[k - 1] : null;
        const fold = fused && prev && prev.comb && !H.pre.length;   // the previous combine folds into this site
        lists[k].push(...H.pre, siteOp(H.site, fold ? prev.comb : null), ...H.rest);
        const next = halves[k + 1];
        if (H.comb && !(fused && next && !next.pre.length)) lists[k].push(combOp(H.comb));
      });
      return lists;
    };

    // ---- per layer state and ops
    const frameU = B.frame;
    this.layers = [];
    for (let i = lo; i < hi; i++) {
      // st.w: the weights this engine joined or fused (their parts are consumed), for engine/qwen4exp_batch.js
      const L = weights.layers[i - lo], isQ = cfg.qsa[i], st = { w: {} }, attnOps = [], ffnOps = [];
      if (!!L.isFull !== !!isQ) throw new Error(`qwen4exp: layer ${i} type does not match the config`);
      // PLE (n-gram) injection, before the layer's first mixer
      if (pleLayers.includes(i)) {
        const pc = L.ple, K1 = pc.conv.data.length / HD;
        const slots = (K1 - 1) * cfg.ple.ngram + 1;
        st.ring = buf(slots * HD);
        attnOps.push(gemv(wq(pc.key, "ple key"), B.emb, B.pk, { tap: ["ple_embd", B.emb, d] }));
        attnOps.push(gemv(wq(pc.value, "ple value"), B.emb, B.pv));
        // the PLE norms and the conv kernel in one buffer (8 storage buffers per stage is WebGPU's default limit)
        const pw = new Float32Array(3 * HD + HD * K1);
        [pc.normKey, pc.normQuery, pc.normConv, pc.conv].forEach((e, j) => pw.set(e.data, j * HD));
        st.w.pw = wq({ kind: "f32", data: pw, shape: [pw.length] }).buf;
        st.pleU = { K1, slots };
        attnOps.push({ pipe: P.ple, bg: bg(P.ple, [B.R, B.pk, B.pv, st.w.pw,
          st.ring, B.dbg, uni([d, K1, cfg.ple.ngram, slots, f(eps), hc, 0, 0]), frameU]), wg: [hc, 1, 1],
          taps: [[`ple_gate-${i}`, B.dbg, hc, "f32", 2 * HD], [`ple_gated_value-${i}`, B.dbg, HD], [`ple_conv_out-${i}`, B.dbg, HD, "f32", HD]] });
      }
      const pre = attnOps.splice(0), sA = site(L.hcAttn, `-${i}`);
      attnOps.push(...hcMixRest(L.hcAttn, `-${i}`, `hc_mixed-${i}`));
      if (isQ) attnOps.push(...this._attnOps(L, i, st, { gemv, wq, bg, uni, f, view, fuseRows }));
      else {
        st.conv = buf(convDim * (ss.convK - 1)); st.S = buf(ss.nVHeads * ss.stateSize * ss.stateSize);
        // [qkv | z]: one GEMV when both have one type (Q4_0 in every layer of the bartowski file)
        const zTap = { tap: [`z-${i}`, B.qkvz, ss.inner, "f32", convDim] }, fz = st.w.qkvz = fuseRows(L.wqkv, L.wz);
        if (fz) attnOps.push(gemv(fz, B.mixed, B.qkvz, zTap));
        else attnOps.push(gemv(wq(L.wqkv, "qkv"), B.mixed, B.qkvz), gemv(wq(L.wz, "z"), B.mixed, B.qkvz, { yOff: convDim, ...zTap }));
        // beta and alpha as one GEMV, dt_bias and A as one buffer (cat: CPU copies of the f32 tensors)
        attnOps.push(gemv(st.w.ba = wq(cat(L.wBeta, L.wAlpha)), B.mixed, B.ba, { dIn: d }));
        attnOps.push({ pipe: P.dnConv, bg: bg(P.dnConv, [view(B.qkvz, 0, convDim), st.conv, wq(L.conv, "conv").buf, B.co, uni([ss.convK, 2 * ss.nKHeads, ss.stateSize, f(eps)])]),
          wg: [convDim / ss.stateSize, 1, 1] });
        attnOps.push({ pipe: P.dnDelta, bg: bg(P.dnDelta, [B.co, st.S, B.ba, st.w.dta = wq(cat(L.dtBias, L.ssmA)).buf, view(B.qkvz, convDim, ss.inner), wq(L.ssmNorm).buf, B.o, B.go,
          uni([ss.nKHeads, ss.stateSize, keyDim, ss.nVHeads, f(eps), 0, 0, 0])]), wg: [ss.nVHeads, 1, 1],
          taps: [[`attn_output-${i}`, B.o, ss.inner], [`final_output-${i}`, B.go, ss.inner]] });
        attnOps.push(gemv(wq(L.wOut, "ssm_out"), B.go, B.out, { tap: [`linear_attn_out-${i}`, B.out, d] }));
      }
      const combA = { a: B.out, b: null, name: `hc_combine-${i}`, h: sA.h };
      // MoE: router, top-K, routed experts, shared expert (gated by sigmoid(shRouter . x))
      const sF = site(L.hcFfn, `-${i}`);
      ffnOps.push(...hcMixRest(L.hcFfn, `-${i}`, `hc_mixed-${i}`));
      // the router and the shared expert's gate as one GEMV: logits[nExp] is the shared gate's
      ffnOps.push(gemv(st.w.router = wq(cat(L.router, L.shRouter)), B.mixed, B.logits, { dIn: d }));
      ffnOps.push({ pipe: P.route, bg: bg(P.route, [B.logits, B.ids, B.wts, uni([cfg.nExp, K, 0, 0])]), wg: [1, 1, 1],
        taps: [[`ffn_moe_topk-${i}`, B.ids, K, "u32"], [`ffn_moe_weights_norm-${i}`, B.wts, K]] });
      // the routed experts: resident, or (offloaded) the layer's slot pool read through the slot ids, after the cut
      const off = !!L.expGate.offload, ids = off ? B.offIds : B.ids;
      const ex = (k, part) => (off ? this.experts.poolEntry(i, part) : wq(L[k], "experts " + part));
      const eg = ex("expGate", "gate"), eu = ex("expUp", "up"), ed = ex("expDown", "down");
      if (eg.kind !== eu.kind) throw new Error(`qwen4exp: layer ${i} gate/up experts differ in type`);
      const ff = cfg.expFF, nRoute = ffnOps.length;
      ffnOps.push({ pipe: P["gu_" + eg.kind], bg: bg(P["gu_" + eg.kind], [eg.qs, eg.sc, eu.qs, eu.sc, B.mixed, B.h, ids, uni([ff, d, K, 0, 0, 0, 0, 0])]), wg: [Math.ceil(ff / 8), K, 1] });
      ffnOps.push({ pipe: P["dn_" + ed.kind], bg: bg(P["dn_" + ed.kind], [ed.qs, ed.sc, B.h, B.moe, ids, B.wts, uni([d, ff, K, 0, 0, 0, 0, 0])]), wg: [Math.ceil(d / 8), 1, 1],
        tap: [`ffn_moe_out-${i}`, B.moe, d] });
      const sgw = wq(L.shGate, "shared gate"), suw = wq(L.shUp, "shared up"), sdw = wq(L.shDown, "shared down"), sff = cfg.shFF;
      if (sgw.kind !== suw.kind) throw new Error(`qwen4exp: layer ${i} shared gate/up differ in type`);
      ffnOps.push({ pipe: P["gu_" + sgw.kind], bg: bg(P["gu_" + sgw.kind], [sgw.qs, sgw.sc, suw.qs, suw.sc, B.mixed, B.hs, B.shIds, uni([sff, d, 1, 0, 0, 0, 0, 0])]), wg: [Math.ceil(sff / 8), 1, 1] });
      ffnOps.push({ pipe: P["dn_" + sdw.kind], bg: bg(P["dn_" + sdw.kind], [sdw.qs, sdw.sc, B.hs, B.sh, B.shIds, B.logits, uni([d, sff, 1, 1, cfg.nExp, 0, 0, 0])]), wg: [Math.ceil(d / 8), 1, 1],
        tap: [`ffn_shexp_gated-${i}`, B.sh, d] });
      ffnOps[nRoute - 1].cut = true;   // the router: an offloaded layer cuts after it, and the trace copies its ids
      const combF = { a: B.moe, b: B.sh, name: `l_last-${i}`, h: sF.h };
      this.layers.push({ i, isQ, st, off, halves: [{ pre, site: sA, rest: attnOps, comb: combA }, { pre: [], site: sF, rest: ffnOps, comb: combF }] });
    }

    // ---- head: the last HC mixer (no inject) stands in for the output norm, then the LM head and a GPU argmax
    if (hasHead) {
      const V = this.dims.vocab, per = 1024, nPart = Math.ceil(V / per);
      Object.assign(B, { hl: buf(V), part: buf(nPart * 2), am: buf(4) });
      this.headHalf = { pre: [], site: site(weights.headHC, "", false), comb: null, rest: [...hcMixRest(weights.headHC, "", "result_norm"),
        gemv(wq(weights.head, "output"), B.mixed, B.hl, { tap: ["result_output", B.hl, V] }),
        { pipe: P.am1, bg: bg(P.am1, [B.hl, B.part, uni([V, per, 0, 0])]), wg: [nPart, 1, 1] },
        { pipe: P.am2, bg: bg(P.am2, [B.part, B.am, uni([nPart, 0, 0, 0])]), wg: [1, 1, 1] }] };
      this.amStage = device.createBuffer({ size: 16, usage: U.MAP_READ | U.COPY_DST });
      this.logitStage = device.createBuffer({ size: Math.ceil(V * 4 / 16) * 16, usage: U.MAP_READ | U.COPY_DST });
    }
    this.hStage = device.createBuffer({ size: HD * 4, usage: U.MAP_READ | U.COPY_DST });
    this._wq = wq;   // engine/qwen4exp_batch.js builds its ops on the same weight buffers (st.w: the joined ones)
    // op lists: per half unfused (debugToken), per layer fused (forwardToken)
    const halves = [...this.layers.flatMap((l) => l.halves), ...(hasHead ? [this.headHalf] : [])];
    const dbg = this._halfOps(halves, false), fast = this._halfOps(halves, true);
    this.layers.forEach((l, k) => { l.attnOps = dbg[2 * k]; l.ffnOps = dbg[2 * k + 1]; l.fast = [...fast[2 * k], ...fast[2 * k + 1]]; });
    // a cut layer's ops split after its router: ffnRoute / ffnExp (debugToken), fastRoute / fastExp (forwardToken)
    const at = (ops) => ops.findIndex((o) => o.cut) + 1;
    for (const l of this.layers) {
      [l.ffnRoute, l.ffnExp] = [l.ffnOps.slice(0, at(l.ffnOps)), l.ffnOps.slice(at(l.ffnOps))];
      [l.fastRoute, l.fastExp] = [l.fast.slice(0, at(l.fast)), l.fast.slice(at(l.fast))];
    }
    if (hasHead) { this.headOps = dbg[dbg.length - 1]; this.headFast = fast[fast.length - 1]; }
    this._ropeTs = fr(Math.pow(cfg.ropeTheta, -2 / cfg.ropeDims));
    // the CPU copies behind the joined f32 tensors are on the GPU now (the op closures keep this scope alive)
    cats.clear();
    for (const e of catParts) e.data = null;
    this.reset();
  }

  // Gated full attention. With QSA (engine/qwen4exp_qsa.js): the indexer state update (proj, prep, pool) on every
  // step, then dense attention while the step has <= 2051 cells, or score / select and the gathered attention over the
  // selected cells past that (dyn ops: _encode asks them per step).
  _attnOps(L, i, st, { gemv, wq, bg, uni, f, view, fuseRows }) {
    const { cfg, B, P, device } = this, nH = cfg.nHeads, nKV = cfg.nKVHeads, hd = cfg.headDim;
    if (hd !== 256) throw new Error("qwen4exp: attn kernels assume head dim 256");
    const sz = this.kvBufBytes;
    // COPY_SRC: a checkpoint copies the KV rows out on the GPU, as Qwen35Engine.saveSlot does (the DeltaNet state,
    // conv window and PLE ring are scratch buf()s, which already have it)
    st.kc = device.createBuffer({ size: sz, usage: U.STORAGE | U.COPY_SRC | U.COPY_DST });
    st.vc = device.createBuffer({ size: sz, usage: U.STORAGE | U.COPY_SRC | U.COPY_DST });
    const ap = st.ap = uni([nH, nKV, hd, cfg.ropeDims, f(cfg.eps), 0, 0, 0]);
    const nQ = 2 * nH * hd, nK = nKV * hd, qf = view(B.qkvA, 0, nQ), fq = st.w.qkvA = fuseRows(L.wq, L.wk, L.wv);
    const dense = { pipe: P.attnDec, bg: bg(P.attnDec, [B.qn, st.kc, st.vc, qf, B.ao, ap, B.frame]), wg: [nH, 1, 1], tap: [`attn_gated-${i}`, B.ao, nH * hd] };
    let qsaState = [], qsaSel = [], gather = [];
    if (this.qsa) {
      st.qsa = qsaLayerState(device, L, { maxSeq: this.maxSeq, dim: cfg.dim, rawRing: this.qsaRing });
      const G = qsaBind(device, this.qsaK, this.qsaSh, this.qsaS, st.qsa, B.mixed);
      qsaState = qsaStateOps(this.qsaK, G);
      qsaSel = qsaSelectOps(this.qsaK, G, this.sparse);
      dense.dyn = (pos, n) => (this.sparse(pos, n) ? null : [nH, 1, 1]);
      // the batch kernel with one column: the gate at column 0 of qkvA (MS.qs unused)
      gather = [{ pipe: P.attnGather, bg: bg(P.attnGather, [B.qn, st.kc, st.vc, B.qkvA, B.ao, ap, null, uni([0, nQ, nQ + nK, 0]), this.qsaS.sel]),
        dyn: (pos, n) => (this.sparse(pos, n) ? [nH, 1, 1] : null), tap: [`attn_gated-${i}`, B.ao, nH * hd] }];
    }
    return [
      // [q|gate, k, v] into one buffer: one GEMV when the three share a type (Q6_K and Q8_0 both load as q8)
      ...(fq ? [gemv(fq, B.mixed, B.qkvA)] : [gemv(wq(L.wq, "attn_q"), B.mixed, B.qkvA), gemv(wq(L.wk, "attn_k"), B.mixed, B.qkvA, { yOff: nQ }),
        gemv(wq(L.wv, "attn_v"), B.mixed, B.qkvA, { yOff: nQ + nK })]),
      ...qsaState,
      { pipe: P.attnPrep, bg: bg(P.attnPrep, [qf, view(B.qkvA, nQ, nK), view(B.qkvA, nQ + nK, nK), B.qn, st.kc, st.vc, wq(L.qNorm).buf, wq(L.kNorm).buf, ap, B.frame]), wg: [nH + nKV, 1, 1] },
      ...qsaSel,
      dense,
      ...gather,
      gemv(wq(L.wo, "attn_output"), B.ao, B.out, { tap: [`attn_output-${i}`, B.out, cfg.dim] }),
    ];
  }

  // ---- per-sequence state: sizes, checkpoints
  // What a checkpoint copies at pos (as Qwen35Engine._stateParts): per attention layer the K / V rows [0, pos) and,
  // with QSA, the raw-key ring (whole: it holds the open block's rows) and the pooled keys of the complete blocks
  // [0, floor(pos / 4)); per DeltaNet layer S and the conv window; the PLE ring. Every buffer has COPY_SRC (#361).
  // The CPU part (pos, the n-gram predecessors: ngs.export()) is the caller's. -> [{ buf, bytes }]
  _stateParts(pos = this.pos) {
    const row = this.cfg.nKVHeads * this.cfg.headDim * W.kvBytes(this.kv), parts = [];
    for (const { st } of this.layers) {
      if (st.kc) {
        parts.push({ buf: st.kc, bytes: pos * row }, { buf: st.vc, bytes: pos * row });
        if (st.qsa) {
          parts.push({ buf: st.qsa.rawK, bytes: st.qsa.rawRing ? st.qsa.rawK.size : pos * QSA_ID * 2 });
          parts.push({ buf: st.qsa.pooled, bytes: Math.floor(pos / QSA_KPOOL) * QSA_ID * 2 });
        }
      }
      for (const b of [st.S, st.conv, st.ring]) if (b) parts.push({ buf: b, bytes: b.size });
    }
    return parts;
  }
  // What a saved state must match to be loaded here
  stateSignature() {
    return { v: 1, arch: "qwen4exp", lo: this.lo, hi: this.hi, kv: this.kv, qsa: !!this.qsa, qsaRing: this.qsaRing, dim: this.cfg.dim };
  }
  // The bytes of per-sequence state this engine allocated, by kind (qwen4expMemory's split; scores: this engine's
  // one-column QSA scratch only)
  stateBytes() {
    const b = { kv: 0, rawK: 0, pooled: 0, deltaNet: 0, ple: 0 };
    for (const { st } of this.layers) {
      if (st.kc) b.kv += st.kc.size + st.vc.size;
      if (st.qsa) { b.rawK += st.qsa.rawK.size; b.pooled += st.qsa.pooled.size; }
      if (st.S) b.deltaNet += st.S.size + st.conv.size;
      if (st.ring) b.ple += st.ring.size;
    }
    return { bytes: b, state: b.kv + b.rawK + b.pooled + b.deltaNet + b.ple };
  }
  // qwen4expMemory for this engine's layers and formats
  memory(opts = {}) {
    return qwen4expMemory(this.cfg, { ctx: this.maxSeq, kv: this.kv, rawRing: this.qsaRing, qsa: !!this.qsa, layerRange: [this.lo, this.hi], ...opts });
  }

  // reset() also clears a failed step (_failed): a step that throws part way may have advanced layer 0's DeltaNet
  // state and the n-gram predecessors without pos, so the engine refuses further tokens until reset.
  // It also bumps _posGen (as every step does: a batched verify's snapshot is then stale, Qwen4ExpBatch.accept) and
  // forgets which layer the expert store's region holds (a failed batched pass may have encoded a region load that
  // never ran).
  reset() {
    this.pos = 0;
    this._posGen = (this._posGen || 0) + 1;
    this._failed = null;
    if (this.experts?.region) this.experts.region.layer = -1;
    this.ngs?.reset();
    const enc = this.device.createCommandEncoder();
    for (const l of this.layers) for (const b of [l.st.conv, l.st.S, l.st.ring]) if (b) enc.clearBuffer(b);
    this.device.queue.submit([enc.finish()]);
  }

  // The token's embedding row (Q4_0 on the CPU, dequantized as dequantF32 does)
  _embedRow(id) {
    const e = this.embed, d = this.cfg.dim, nb = d / 32, out = new Float32Array(d);
    if (e.kind === "f32") { out.set(e.data.subarray(id * d, (id + 1) * d)); return out; }
    const sc16 = new Uint16Array(e.scales.buffer, e.scales.byteOffset, e.scales.byteLength / 2);
    for (let b = 0; b < nb; b++) {
      const blk = id * nb + b, s = f16(sc16[blk]);
      if (e.kind === "q4") {
        const o = blk * 16;
        for (let j = 0; j < 16; j++) { const q = e.qs[o + j]; out[b * 32 + j] = s * ((q & 15) - 8); out[b * 32 + j + 16] = s * ((q >> 4) - 8); }
      } else {
        const o = blk * 32;
        for (let j = 0; j < 32; j++) { const q = e.qs[o + j]; out[b * 32 + j] = s * (q > 127 ? q - 256 : q); }
      }
    }
    return out;
  }

  // Everything one token needs before its layers run: the residual (from the embedding), the n-gram rows, the frame.
  // The n-gram rows are read without waiting (rows: the pending upload): forwardToken runs the layers before the
  // PLE layer while they land; debugToken awaits them here.
  // The checks before a step changes any state (a throw here leaves the engine as it was).
  _check(id) {
    if (this._failed) throw new Error("qwen4exp: an earlier step failed (" + this._failed.message + "); reset() first");
    if (this.pos >= this.maxSeq) throw new Error(`qwen4exp: context full (${this.maxSeq})`);
    if (this.hasEmbed && !(Number.isInteger(id) && id >= 0 && id < this.nVocab)) throw new Error(`qwen4exp: token id ${id} outside the vocab (${this.nVocab})`);
  }

  async _begin(id, { defer = false } = {}) {
    this._check(id);
    const q = this.device.queue, { cfg, B } = this;
    if (this.hasEmbed) {
      const e = this._embedRow(id), R = new Float32Array(cfg.hDim);
      for (let c = 0; c < cfg.hc; c++) R.set(e, c * cfg.dim);
      q.writeBuffer(B.R, 0, R);
    }
    let rows = null;
    if (B.emb) {
      const r = this.lastRows = this.ngs.push([id]);
      rows = this.ngram.rows(r).then((emb) => q.writeBuffer(B.emb, 0, emb));
    } else this.ngs?.push([id]);
    const F = new Float32Array(W.FRAME_FLOATS), Fu = new Uint32Array(F.buffer);
    Fu[0] = this.pos; Fu[1] = this.pos + 1;
    let th = fr(this.pos);
    for (let i = 0; i < cfg.ropeDims / 2; i++) { F[4 + i * 4] = fr(Math.cos(th)); F[5 + i * 4] = fr(Math.sin(th)); th = fr(th * this._ropeTs); }
    q.writeBuffer(B.frame, 0, F);
    if (this.qsa) qsaSetFrame(this.device, this.qsaS, this.pos, 1);
    if (!defer && rows) await rows;
    return rows;
  }

  // ops at position pos (the step's; a batched pass: its first) over nCols columns
  _encode(pass, ops, pos = this.pos, nCols = 1) {
    let last = null;
    for (const o of ops) {
      const wg = o.dyn ? o.dyn(pos, nCols) : o.wg;
      if (!wg) continue;
      if (o.pipe !== last) { pass.setPipeline(o.pipe); last = o.pipe; }
      if (Array.isArray(o.bg)) o.bg.forEach((g, k) => pass.setBindGroup(k, g)); else pass.setBindGroup(0, o.bg);
      pass.dispatchWorkgroups(wg[0], wg[1], wg[2]);
    }
  }

  // After layer l's router, outside a compute pass: the trace copy, and for an offloaded layer the cut: submit what
  // was encoded, read the K ids back, let the store copy the misses into the layer's pool (a disk layer's experts are
  // read first: ExpertStore.prepare, roadmap 32) and write the slot ids. -> the encoder to continue in
  async _afterRoute(enc, l) {
    const { B, cfg } = this, K = cfg.K;
    if (this.moeTraceOn) {
      const at = (l.i - this.lo) * K * 4, n = (this.hi - this.lo) * K * 4;
      enc.copyBufferToBuffer(B.ids, 0, B.trace, at, K * 4);
      enc.copyBufferToBuffer(B.wts, 0, B.trace, n + at, K * 4);
    }
    if (!l.off) return enc;
    const q = this.device.queue, st = this.experts, stage = this.offStage;
    enc.copyBufferToBuffer(B.ids, 0, stage, 0, K * 4);
    q.submit([enc.finish()]);
    await stage.mapAsync(GPUMapMode.READ, 0, K * 4);
    const sel = new Uint32Array(K + 1);   // ExpertStore's layout: K routed ids, then the shared expert's slot (unused)
    sel.set(new Uint32Array(stage.getMappedRange(0, K * 4)));
    stage.unmap();
    if (st.isDisk?.(l.i)) await st.prepare(l.i, sel, 1, K + 1);
    const e2 = this.device.createCommandEncoder(), r = st.plan(e2, l.i, sel, 1, K + 1);
    if (!r.pool) throw new Error(`qwen4exp: layer ${l.i}: the expert pool holds fewer than one token's ${K} experts`);
    q.writeBuffer(B.offIds, 0, r.remap);   // lands before e2's kernels (queue order)
    return e2;
  }

  // the router's ids and weights of every layer for the last token (moeTrace) -> [{ sel: Uint32Array(K), w: Float32Array(K) }]
  async readMoeTrace() {
    if (!this.moeTraceOn) throw new Error("readMoeTrace: create the engine with moeTrace: true");
    const n = (this.hi - this.lo) * this.cfg.K, K = this.cfg.K, enc = this.device.createCommandEncoder();
    enc.copyBufferToBuffer(this.B.trace, 0, this.traceStage, 0, 2 * n * 4);
    this.device.queue.submit([enc.finish()]);
    await this.traceStage.mapAsync(GPUMapMode.READ, 0, 2 * n * 4);
    const ab = this.traceStage.getMappedRange(0, 2 * n * 4).slice(0);
    this.traceStage.unmap();
    const ids = new Uint32Array(ab, 0, n), w = new Float32Array(ab, n * 4, n);
    return this.layers.map((_, j) => ({ sel: ids.slice(j * K, (j + 1) * K), w: w.slice(j * K, (j + 1) * K) }));
  }

  // One decode step. -> { id (greedy, when the engine has the head), logits (wantLogits) }
  // The layers go in a few submits: the GPU starts on the first while the rest are encoded, and the layers before
  // the PLE layer run while its n-gram rows are read.
  async forwardToken(id, opts) {
    this._check(id);
    this._posGen++;
    try { return await this._forward(id, opts); } catch (e) { this._failed = e; throw e; }
  }

  async _forward(id, { wantLogits = false } = {}) {
    const rows = await this._begin(id, { defer: true }), dev = this.device;
    const firstPle = this.cfg.ple ? this.layers.findIndex((l) => this.cfg.ple.layers.includes(l.i)) : -1;
    // ls into a new encoder; an offloaded layer (or every layer, with moeTrace) cuts after its router -> { enc, pass }
    const encode = async (ls) => {
      let enc = dev.createCommandEncoder(), pass = enc.beginComputePass();
      for (const l of ls) {
        if (!l.off && !this.moeTraceOn) { this._encode(pass, l.fast); continue; }
        this._encode(pass, l.fastRoute);
        pass.end();
        enc = await this._afterRoute(enc, l);
        pass = enc.beginComputePass();
        this._encode(pass, l.fastExp);
      }
      return { enc, pass };
    };
    const run = async (ls) => {
      const { enc, pass } = await encode(ls);
      pass.end();
      dev.queue.submit([enc.finish()]);
    };
    let at = 0;
    if (rows) { if (firstPle > 0) await run(this.layers.slice(0, firstPle)); at = Math.max(firstPle, 0); await rows; }
    for (const step = 12; at + step < this.layers.length; at += step) await run(this.layers.slice(at, at + step));
    const { enc, pass } = await encode(this.layers.slice(at));
    if (this.hasHead) this._encode(pass, this.headFast);
    pass.end();
    if (this.hasHead) {
      enc.copyBufferToBuffer(this.B.am, 0, this.amStage, 0, 16);
      if (wantLogits) enc.copyBufferToBuffer(this.B.hl, 0, this.logitStage, 0, this.dims.vocab * 4);
    } else enc.copyBufferToBuffer(this.B.R, 0, this.hStage, 0, this.cfg.hDim * 4);
    this.device.queue.submit([enc.finish()]);
    this.pos++;
    const out = {};
    if (this.hasHead) {
      await this.amStage.mapAsync(GPUMapMode.READ);
      out.id = new Uint32Array(this.amStage.getMappedRange())[0];
      this.amStage.unmap();
      // the argmax's "no index" (every logit NaN or below -3e38)
      if (out.id >= this.dims.vocab) throw new Error(`qwen4exp: no greedy id at position ${this.pos - 1} (non-finite logits)`);
      if (wantLogits) {
        await this.logitStage.mapAsync(GPUMapMode.READ);
        out.logits = new Float32Array(this.logitStage.getMappedRange(), 0, this.dims.vocab).slice();
        this.logitStage.unmap();
      }
    } else {
      await this.hStage.mapAsync(GPUMapMode.READ);
      out.hidden = new Float32Array(this.hStage.getMappedRange()).slice();
      this.hStage.unmap();
    }
    return out;
  }

  // Prompt tokens one at a time (the batched prefill: engine/qwen4exp_batch.js). Returns the last step's result.
  async prefillTokens(ids, opts) { let r = null; for (const t of ids) r = await this.forwardToken(t, opts); return r; }

  // Per-layer check (tests/test_q38flash.js): one token, each half layer submitted on its own with every tapped
  // tensor read back. force(name) -> Float32Array | null: the residual to start a half layer from ("in-<i>" before
  // layer i's first mixer, "mid-<i>" before its MoE mixer; the dumps' hc_init / l_last-(i-1) / hc_combine-i), for a
  // teacher-forced run. -> Map of "<name>.<k>" (k counts repeats within the token, as the dumps do) -> typed array.
  async debugToken(id, force) {
    this._check(id);
    this._posGen++;
    try { return await this._debug(id, force); } catch (e) { this._failed = e; throw e; }
  }

  async _debug(id, force = () => null) {
    await this._begin(id);
    const out = new Map(), seen = new Map(), dev = this.device;
    const run = async (ops, Rin) => {
      if (Rin) dev.queue.writeBuffer(this.B.R, 0, Rin);
      const enc = dev.createCommandEncoder(), reads = [];
      for (const o of ops) {
        if (o.dyn && !o.dyn(this.pos, 1)) continue;   // not run at this position (QSA's dense / gathered choice)
        const pass = enc.beginComputePass();
        this._encode(pass, [o]);
        pass.end();
        for (const [name, b, n, type = "f32", off = 0] of o.taps || (o.tap ? [o.tap] : [])) {
          const s = dev.createBuffer({ size: n * 4, usage: U.MAP_READ | U.COPY_DST });
          enc.copyBufferToBuffer(b, off * 4, s, 0, n * 4);
          reads.push([name, s, n, type]);
        }
      }
      dev.queue.submit([enc.finish()]);
      for (const [name, s, n, type] of reads) {
        await s.mapAsync(GPUMapMode.READ);
        let a = type === "u32" ? new Int32Array(s.getMappedRange().slice(0, n * 4)) : new Float32Array(s.getMappedRange().slice(0, n * 4));
        if (type === "sum4") {   // the HC site's inject partials [row][stream] -> the inject values
          const h = Math.sqrt(n), v = new Float32Array(h);
          for (let r = 0; r < h; r++) for (let c = 0; c < h; c++) v[r] += a[r * h + c];
          a = v;
        }
        s.unmap(); s.destroy();
        const k = seen.get(name) || 0; seen.set(name, k + 1);
        out.set(`${name}.${k}`, a);
      }
    };
    for (const l of this.layers) {
      await run(l.attnOps, force(`in-${l.i}`));
      if (l.off || this.moeTraceOn) {   // offloaded: the router, the cut, then the experts on the pool (moeTrace:
        // every layer splits here too, so the trace holds every layer's ids, as in _forward)
        await run(l.ffnRoute, force(`mid-${l.i}`));
        dev.queue.submit([(await this._afterRoute(dev.createCommandEncoder(), l)).finish()]);
        await run(l.ffnExp, null);
      } else await run(l.ffnOps, force(`mid-${l.i}`));
    }
    if (this.hasHead) await run(this.headOps, force("head"));
    this.pos++;
    return out;
  }
}

function f16(h) {
  const s = h & 0x8000 ? -1 : 1, e = (h >> 10) & 0x1f, m = h & 0x3ff;
  return e === 0 ? s * m * 2 ** -24 : e === 31 ? (m ? NaN : s * Infinity) : s * (1 + m / 1024) * 2 ** (e - 15);
}
