// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/dense.js (MIT; see LICENSE-ENGINE).
// DenseEngine: WebGPU inference for dense Llama-architecture models (Qwen3, SmolLM), layer-shardable.
import { weightsFromSafetensors } from "./safetensors.js";
import { WGSL } from "./wgsl/base.js";
import { compilePipeline } from "./compile.js";
import { moduleSet } from "./wgsl/prune.js";
import { probeUnpack, coopWGSL } from "./wgsl/coop.js";
import { f16ToF32 } from "./gguf.js";
import { denseAttnWGSL, DENSE_GLUE_WGSL, DENSE_GLUE3_WGSL } from "./wgsl/dense.js";

export class DenseEngine {
  // opts: { device, cfg, tensors?|weights?, layerRange, hasEmbed, hasHead, maxSeq }
  reset() { this.pos = 0; }   // fresh context; the KV cache is overwritten from position 0

  // GPU-side checkpoints (the same calls as Qwen35Engine's saveSlot / loadSlot / dropSlot): a copy
  // of this device's KV rows [0, pos) per layer. A dense model has no other state that depends on
  // the tokens so far (the next token is embedded fresh), so this is exactly the context. No
  // stateSignature / exportSlot: these are GPU copies only (no disk copies). A browser host keeps
  // its checkpoints for the qwen35 engine (hostCkpt, room.js ckptEngine); the room node uses these.
  get hostCkpt() { return false; }
  saveSlot(name) {
    this.dropSlot(name);
    const bytes = this.pos * this.dims.kvDim * 4;
    const enc = this.device.createCommandEncoder();
    const bufs = [];
    for (const L of this.layers) for (const src of [L.kCache, L.vCache]) {
      const b = this.device.createBuffer({ size: Math.max(4, bytes), usage: GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST });
      if (bytes) enc.copyBufferToBuffer(src, 0, b, 0, bytes);
      bufs.push(b);
    }
    this.device.queue.submit([enc.finish()]);
    (this.slots = this.slots || new Map()).set(name, { pos: this.pos, bufs });
  }
  loadSlot(name) {
    const sl = this.slots?.get(name);
    if (!sl) throw new Error("no saved slot " + name);
    const bytes = sl.pos * this.dims.kvDim * 4;
    const enc = this.device.createCommandEncoder();
    let i = 0;
    for (const L of this.layers) for (const dst of [L.kCache, L.vCache]) { if (bytes) enc.copyBufferToBuffer(sl.bufs[i], 0, dst, 0, bytes); i++; }
    this.device.queue.submit([enc.finish()]);
    this.pos = sl.pos;
  }
  dropSlot(name) {
    const sl = this.slots?.get(name);
    if (sl) { for (const b of sl.bufs) b.destroy(); this.slots.delete(name); }
  }
  dropAllSlots() { for (const k of [...(this.slots?.keys() || [])]) this.dropSlot(k); }

  static async create(opts) {
    const e = new DenseEngine();
    await e._init(opts);
    return e;
  }

  async _init({ device, cfg, tensors, weights, layerRange, hasEmbed = true, hasHead = true, maxSeq = 512, matvecVariant = "coop", coopWG = 256, coopRows = 4, attnFast = true, fuse = true, fuseGlue = false, glue3 = true, mergeQKV = true, headRows = 8, batchWG = 64, verifyWG = coopWG }) {
    this.device = device;
    this.cfg = cfg;
    this.maxSeq = maxSeq;
    this.mvVariant = matvecVariant;
    this.coopWG = coopWG; this.coopRows = coopRows;
    const dim = cfg.hidden_size;
    const nH = cfg.num_attention_heads;
    const nKV = cfg.num_key_value_heads;
    const headDim = cfg.head_dim || dim / nH;
    const qDim = nH * headDim;
    const kvDim = nKV * headDim;
    const inter = cfg.intermediate_size;
    const vocab = cfg.vocab_size;
    this.dims = { dim, nH, nKV, headDim, qDim, kvDim, inter, vocab };
    const [lo, hi] = layerRange || [0, cfg.num_hidden_layers];
    this.lo = lo; this.hi = hi;
    this.hasEmbed = hasEmbed; this.hasHead = hasHead;
    this.pos = 0;
    this.maxDrafts = 7;   // longest draft run one verify takes (8 columns: two batched passes per device)

    const W = weights || weightsFromSafetensors(tensors, { lo, hi, hasEmbed, hasHead });

    this.rowsB = 4;   // rows per workgroup of the 4-column batched kernels (ROWSB below), NOT coopRows
    // attention kernels that are not latency-bound at long context (engine/wgsl/dense.js), bit-identical
    // to attn_scores / attn_softmax / attn_out; engine.attnFast = false switches back at runtime (A/B)
    const G = nH / nKV;
    this.attnFastOn = attnFast !== false && Number.isInteger(G) && headDim % 32 === 0 && G * 64 <= 256 && G * headDim <= 1024;
    this.attnFast = this.attnFastOn;
    // fused layer glue (engine/wgsl/dense.js DENSE_GLUE_WGSL), bit-identical to the dispatches it
    // replaces: q/k head_norm + rope + the KV cache writes in one dispatch, residual adds folded into
    // the o / down GEMVs (_acc), one rmsnorm dispatch for all batch columns; the layer is one compute
    // pass. engine.fuse = false restores the old dispatches at runtime (A/B).
    this.fuseOn = fuse !== false && matvecVariant === "coop" && headDim % 2 === 0 && headDim / 2 <= 64;
    this.fuse = this.fuseOn;
    // The glue kernel (qk-norm + rope + cache writes in one dispatch) is NOT bit-identical on the GB10
    // (NVIDIA Vulkan): the same rope expression compiles to differently rounded code in the bigger
    // kernel. So by default the reference head_norm / rope dispatches and cache copies stay, and only the
    // exact parts are fused (residual adds in the GEMVs, one rmsnorm for all columns). fuseGlue: true
    // turns the glue on (about -0.5 ms per token, changes the bits). fuseAcc / fuseRms = false for A/B.
    this.fuseGlue = fuseGlue === true;
    this.glue3 = glue3 !== false && !this.fuseGlue;   // the glue as 3 reference-shaped kernels (see DENSE_GLUE3_WGSL)
    // one module per entry point, pruned to what it uses (engine/wgsl/prune.js)
    const mod = moduleSet(device, WGSL + coopWGSL(coopWG, coopRows, batchWG, 4, this.rowsB, await probeUnpack(device))
      + (this.attnFastOn ? denseAttnWGSL({ G, hd: headDim }) : "") + (this.fuseOn ? DENSE_GLUE_WGSL + DENSE_GLUE3_WGSL : ""));
    const C = GPUShaderStage.COMPUTE;
    const layout0 = device.createBindGroupLayout({
      entries: [
        { binding: 0, visibility: C, buffer: { type: "uniform" } },
        { binding: 1, visibility: C, buffer: { type: "uniform" } },
      ],
    });
    const G1 = {
      matvec: ["ro", "ro", "rw", "u"], matvec_q8: ["ro", "ro", "ro", "rw", "u"],
      matvec_q4: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop: ["ro", "ro", "rw", "u"], matvec_q8_coop: ["ro", "ro", "ro", "rw", "u"],
      matvec_q4_coop: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop_acc: ["ro", "ro", "rw", "u"], matvec_q8_coop_acc: ["ro", "ro", "ro", "rw", "u"], matvec_q4_coop_acc: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop_b_acc: ["ro", "ro", "rw", "u"], matvec_q8_coop_b_acc: ["ro", "ro", "ro", "rw", "u"], matvec_q4_coop_b_acc: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop_b: ["ro", "ro", "rw", "u"], matvec_q8_coop_b: ["ro", "ro", "ro", "rw", "u"],
      matvec_q4_coop_b: ["ro", "ro", "ro", "rw", "u"],
      matvec_gu: ["ro", "ro", "ro", "rw", "u"], matvec_gu_b: ["ro", "ro", "ro", "rw", "u"],
      matvec_q8_gu: ["ro", "ro", "ro", "ro", "ro", "rw", "u"], matvec_q8_gu_b: ["ro", "ro", "ro", "ro", "ro", "rw", "u"],
      matvec_q4_gu: ["ro", "ro", "ro", "ro", "ro", "rw", "u"], matvec_q4_gu_b: ["ro", "ro", "ro", "ro", "ro", "rw", "u"],
      rmsnorm: ["ro", "ro", "rw", "u"], head_norm: ["rw", "ro", "u"],
      rope: ["rw", "u"], attn_scores: ["ro", "ro", "rw"], attn_softmax: ["rw"],
      attn_out: ["ro", "ro", "rw"], silu_mul: ["rw", "ro"], add_res: ["rw", "ro"],
    };
    if (this.attnFastOn) Object.assign(G1, { attn_scores_d: ["ro", "ro", "rw", "u"], attn_softmax_d: ["rw"], attn_out_d: ["ro", "ro", "rw", "u"] });
    if (this.fuseOn) Object.assign(G1, { attn_glue_d: ["rw", "ro", "ro", "ro", "ro", "rw", "rw", "u"], rmsnorm_dmc: ["ro", "ro", "rw", "u"],
      head_norm_dmc: ["rw", "rw", "ro", "ro", "u"], rope_dmc: ["rw", "rw", "u"], kv_store_d: ["ro", "ro", "rw", "rw", "u"],
      head_norm_dmc1: ["rw", "ro", "ro", "u"], rope_dmc1: ["rw", "u"] });
    const bufType = { u: "uniform", ro: "read-only-storage", rw: "storage" };
    this.pipes = {};
    // compile every pipeline in parallel (async): overlaps shader compilation
    // with the rest of setup instead of serializing 20+ compiles
    await Promise.all(Object.entries(G1).map(async ([name, spec]) => {
      const layout1 = device.createBindGroupLayout({
        entries: spec.map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })),
      });
      this.pipes[name] = await compilePipeline(device, {
        layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }),
        compute: { module: mod(name), entryPoint: name },
      });
    }));

    // LM head GEMV with its own rows-per-workgroup (headRows): rows per workgroup never enter a row's
    // arithmetic (same per-thread sums, same tree over the WG slots), so this is bit-identical to the
    // coopRows kernel; at dIn = 2048 the head is ~20% faster with 8 rows (tests/bench_wide.js q17)
    this.headRows = hasHead && matvecVariant === "coop" && headRows > 0 && headRows !== coopRows ? headRows : 0;
    if (this.headRows) {
      const modH = moduleSet(device, WGSL + coopWGSL(coopWG, this.headRows, 64, 4, this.rowsB, await probeUnpack(device)));
      for (const [name, spec] of [["matvec_q8_coop", G1.matvec_q8_coop], ["matvec_q4_coop", G1.matvec_q4_coop], ["matvec_coop", G1.matvec_coop]]) {
        const layout1 = device.createBindGroupLayout({ entries: spec.map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })) });
        this.pipes[name + "_h"] = await compilePipeline(device, { layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), compute: { module: modH(name), entryPoint: name } });
      }
    }
    // Exact verify ops: the batched GEMVs again, built with the single-token kernels' workgroup size
    // (verifyWG = coopWG), so a column's sums run in the same order as matvec_*_coop's (the same
    // per-thread strides and reduction tree) and its bits match single-token decoding; the batched
    // prefill keeps its smaller, faster workgroups. Speculative verifies use these (embedRunBatch /
    // runHiddenBatch with verify), so a verified token is exactly a plainly decoded one.
    // tests/test_dense_spec.js checks the bits.
    this.verifyX = matvecVariant === "coop" && verifyWG !== batchWG;
    if (this.verifyX) {
      const modX = moduleSet(device, WGSL + coopWGSL(coopWG, coopRows, verifyWG, 4, this.rowsB, await probeUnpack(device)));
      const xNames = Object.keys(G1).filter((k) => /_b(_acc)?$/.test(k) && k.startsWith("matvec"));
      await Promise.all(xNames.map(async (name) => {
        const layout1 = device.createBindGroupLayout({ entries: G1[name].map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })) });
        this.pipes[name + "_x"] = await compilePipeline(device, { layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), compute: { module: modX(name), entryPoint: name } });
      }));
    }
    // uniforms
    const cfgData = new ArrayBuffer(48);
    const cu = new Uint32Array(cfgData), cf = new Float32Array(cfgData);
    cu.set([dim, kvDim, nH, nKV, headDim, inter, vocab, maxSeq], 0);
    cf[8] = cfg.rms_norm_eps; cf[9] = cfg.rope_theta; cu[10] = qDim;
    this.cfgBuf = this._buf(cfgData, GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST);
    this.frameBuf = device.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
    this._shapes = {};
    this.nBufDim = this._buf(new Uint32Array([dim]), GPUBufferUsage.UNIFORM);
    this.nHBuf = this._buf(new Uint32Array([nH]), GPUBufferUsage.UNIFORM);
    this.nKVBuf = this._buf(new Uint32Array([nKV]), GPUBufferUsage.UNIFORM);

    // working buffers
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC;
    this.x = device.createBuffer({ size: dim * 4, usage: S });
    this.xn = device.createBuffer({ size: dim * 4, usage: S });
    // mergeQKV: q, k and v come out of ONE GEMV over the row-concatenated [wq | wk | wv] (each row keeps
    // its kernel and its arithmetic, so the bits are the same; rows per workgroup never enter a row's
    // sums). q / k / v are then 256-byte aligned views of one output buffer. Needs Q8/Q4 q/k/v of one
    // kind and 256-byte aligned segment sizes (true for every Qwen3 size).
    this.mergeQKV = mergeQKV !== false && !this.fuseGlue && matvecVariant === "coop" && (qDim * 4) % 256 === 0 && (kvDim * 4) % 256 === 0;
    if (this.mergeQKV) {
      const kinds = new Set(W.layers.map((l) => [l.q, l.k, l.v].map((e) => e && e.kind).join()));
      if (kinds.size !== 1 || ![...kinds][0].split(",").every((k) => k === "q8" || k === "q4") || new Set([...kinds][0].split(",")).size !== 1) this.mergeQKV = false;
    }
    if (this.mergeQKV) {
      this.qkvBuf = device.createBuffer({ size: (qDim + 2 * kvDim) * 4, usage: S });
      this.q = DenseEngine._view(this.qkvBuf, 0, qDim * 4);
      this.k = DenseEngine._view(this.qkvBuf, qDim * 4, kvDim * 4);
      this.v = DenseEngine._view(this.qkvBuf, (qDim + kvDim) * 4, kvDim * 4);
    } else {
      this.q = device.createBuffer({ size: qDim * 4, usage: S });
      this.k = device.createBuffer({ size: kvDim * 4, usage: S });
      this.v = device.createBuffer({ size: kvDim * 4, usage: S });
    }
    this.attnOut = device.createBuffer({ size: qDim * 4, usage: S });
    this.tmpDim = device.createBuffer({ size: dim * 4, usage: S });
    this.g = device.createBuffer({ size: inter * 4, usage: S });
    this.u = device.createBuffer({ size: inter * 4, usage: S });
    // 4 column slots: the batched attention kernels keep one [nH][maxSeq] score block per column
    this.scores = device.createBuffer({ size: (this.attnFastOn ? 4 : 1) * nH * maxSeq * 4, usage: S });
    this.uDMC0 = this._buf(new Uint32Array([0, 0, 0, 0]), GPUBufferUsage.UNIFORM);
    this.stageX = device.createBuffer({ size: dim * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });

    // weight upload
    const up = (e) => {
      if (!e) return null;
      if (e.gpu) return e.gpu;            // already streamed onto the GPU during download
      let r;
      if (e.kind === "q8" || e.kind === "q4") r = { kind: e.kind, qs: this._buf(e.qs, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC), sc: this._buf(e.scales, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
      else r = { kind: "f32", buf: this._buf(e.data, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
      if (e !== this.cpuEmbed) e.qs = e.scales = e.data = null; // release CPU copy (embed rows stay for lookups)
      return r;
    };
    this.layers = W.layers.map((l) => ({
      inNorm: up(l.inNorm), postNorm: up(l.postNorm),
      qNorm: up(l.qNorm), kNorm: up(l.kNorm),
      wq: up(l.q), wk: up(l.k), wv: up(l.v), wo: up(l.o),
      wgate: up(l.gate), wup: up(l.up), wdown: up(l.down),
      kCache: device.createBuffer({ size: maxSeq * kvDim * 4, usage: S }),
      vCache: device.createBuffer({ size: maxSeq * kvDim * 4, usage: S }),
    }));

    if (this.mergeQKV) {   // row-concatenate [wq | wk | wv] on the GPU, free the parts
      const kind = this.layers[0].wq.kind, rb = kind === "q4" ? [dim / 2, dim / 16] : [dim, dim / 16];   // bytes per row: qs, f16 scales
      const rows = [qDim, kvDim, kvDim], U = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST;
      const enc = device.createCommandEncoder();
      for (const L2 of this.layers) {
        const dst = rb.map((x) => device.createBuffer({ size: Math.ceil((qDim + 2 * kvDim) * x / 4) * 4, usage: U }));
        let r0 = 0;
        [L2.wq, L2.wk, L2.wv].forEach((w, i) => {
          [w.qs, w.sc].forEach((b, j) => enc.copyBufferToBuffer(b, 0, dst[j], r0 * rb[j], rows[i] * rb[j]));
          r0 += rows[i];
        });
        L2.wqkv = { kind, qs: dst[0], sc: dst[1], _free: [L2.wq, L2.wk, L2.wv] };
      }
      device.queue.submit([enc.finish()]);
      for (const L2 of this.layers) { for (const w of L2.wqkv._free) { w.qs.destroy(); w.sc.destroy(); } L2.wqkv._free = null; L2.wq = L2.wk = L2.wv = null; }
    }

    if (hasEmbed || hasHead) {
      if (W.embed.kind === "f32") {
        this.embedGPU = this._buf(W.embed.data, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC);
        this.headEntry = { kind: "f32", buf: this.embedGPU };
      } else {
        this.cpuEmbed = W.embed; // dequant rows on CPU per token
        if (hasHead) this.headEntry = up(W.embed);
      }
      if (W.head) this.headEntry = up(W.head); // untied lm_head
    }
    if (hasHead) {
      this.finalNorm = up(W.finalNorm);
      this.logits = device.createBuffer({ size: vocab * 4, usage: S });
      this.stageLogits = device.createBuffer({ size: vocab * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    }

    // bind groups
    this.bgCommonFor = {};
    for (const [k2, p] of Object.entries(this.pipes))
      this.bgCommonFor[k2] = this._bg(p, 0, [this.cfgBuf, this.frameBuf]);

    const coop = this.mvVariant === "coop";
    const mv = (w, x, y, dOut, dIn, acc = false) => {   // acc: y += W x (coop only)
      const base = w.kind === "q8" ? "matvec_q8" : w.kind === "q4" ? "matvec_q4" : "matvec";
      const pipe = coop ? base + "_coop" + (acc ? "_acc" : "") : base;
      const bufs = w.kind === "f32" ? [w.buf, x, y, this._shape(dOut, dIn)] : [w.qs, w.sc, x, y, this._shape(dOut, dIn)];
      return { pipe, wgs: coop ? Math.ceil(dOut / this.coopRows) : Math.ceil(dOut / 64), bg: this._bg(this.pipes[pipe], 1, bufs) };
    };
    this._mv = mv;
    // fused gate/up(+SiLU) op; null when kinds differ (fallback: unfused path)
    const guOp = (wg2, wu2, x, y, dOut, dIn, xB, yB, sfx = "") => {
      if (!coop || !wg2 || !wu2 || wg2.kind !== wu2.kind) return null;
      const base = wg2.kind === "q8" ? "matvec_q8_gu" : wg2.kind === "q4" ? "matvec_q4_gu" : "matvec_gu";
      const pipe = xB ? base + "_b" + sfx : base;
      const shp = xB ? this._shapeB(dOut, dIn, xB.stride / 16, yB.stride / 4) : this._shapeB(dOut, dIn, 0, 0);
      const bufs = wg2.kind === "f32" ? [wg2.buf, wu2.buf, x, y, shp] : [wg2.qs, wg2.sc, wu2.qs, wu2.sc, x, y, shp];
      return { pipe, wgs: Math.ceil(dOut / (xB ? this.rowsB : this.coopRows)), bg: this._bg(this.pipes[pipe], 1, bufs) };
    };
    this._guOp = guOp;
    const bgNorm = (x, w, y) => this._bg(this.pipes.rmsnorm, 1, [x, w.buf, y, this.nBufDim]);

    const U = (a) => this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM);
    const uGlue = this.fuseOn ? U([0, 0, 0, this.layers[0]?.qNorm ? 1 : 0]) : null;
    const uGlueNoNorm = this.fuseOn ? U([0, 0, 0, 0]) : null;
    const uGlue1 = this.fuseOn && this.mergeQKV ? U([0, 0, 0, 0, qDim, 0, 0, 0]) : null;   // one q | k | v binding, k at qDim
    this.layerBGs = this.layers.map((L2) => ({
      glue: this.fuseOn ? this._bg(this.pipes.attn_glue_d, 1, [this.q, this.k, this.v, (L2.qNorm || { buf: this.x }).buf, (L2.kNorm || { buf: this.x }).buf, L2.kCache, L2.vCache, uGlue]) : null,
      glueNN: this.fuseOn ? this._bg(this.pipes.attn_glue_d, 1, [this.q, this.k, this.v, (L2.qNorm || { buf: this.x }).buf, (L2.kNorm || { buf: this.x }).buf, L2.kCache, L2.vCache, uGlueNoNorm]) : null,
      hn3: !this.fuseOn || !L2.qNorm ? null : this.mergeQKV ? this._bg(this.pipes.head_norm_dmc1, 1, [this.qkvBuf, L2.qNorm.buf, L2.kNorm.buf, uGlue1])
        : this._bg(this.pipes.head_norm_dmc, 1, [this.q, this.k, L2.qNorm.buf, L2.kNorm.buf, uGlue]),
      rope3: !this.fuseOn ? null : this.mergeQKV ? this._bg(this.pipes.rope_dmc1, 1, [this.qkvBuf, uGlue1]) : this._bg(this.pipes.rope_dmc, 1, [this.q, this.k, uGlue]),
      kv3: this.fuseOn ? this._bg(this.pipes.kv_store_d, 1, [this.k, this.v, L2.kCache, L2.vCache, uGlue]) : null,
      oAcc: this.fuseOn ? mv(L2.wo, this.attnOut, this.x, dim, qDim, true) : null,
      downAcc: this.fuseOn ? mv(L2.wdown, this.g, this.x, dim, inter, true) : null,
      norm1: bgNorm(this.x, L2.inNorm, this.xn),
      q: this.mergeQKV ? mv(L2.wqkv, this.xn, this.qkvBuf, qDim + 2 * kvDim, dim) : mv(L2.wq, this.xn, this.q, qDim, dim),
      k: this.mergeQKV ? null : mv(L2.wk, this.xn, this.k, kvDim, dim),
      v: this.mergeQKV ? null : mv(L2.wv, this.xn, this.v, kvDim, dim),
      qNorm: L2.qNorm ? this._bg(this.pipes.head_norm, 1, [this.q, L2.qNorm.buf, this.nHBuf]) : null,
      kNorm: L2.kNorm ? this._bg(this.pipes.head_norm, 1, [this.k, L2.kNorm.buf, this.nKVBuf]) : null,
      scores: this._bg(this.pipes.attn_scores, 1, [this.q, L2.kCache, this.scores]),
      softmax: this._bg(this.pipes.attn_softmax, 1, [this.scores]),
      attnOut: this._bg(this.pipes.attn_out, 1, [this.scores, L2.vCache, this.attnOut]),
      scoresD: this.attnFastOn ? this._bg(this.pipes.attn_scores_d, 1, [this.q, L2.kCache, this.scores, this.uDMC0]) : null,
      softmaxD: this.attnFastOn ? this._bg(this.pipes.attn_softmax_d, 1, [this.scores]) : null,
      outD: this.attnFastOn ? this._bg(this.pipes.attn_out_d, 1, [this.scores, L2.vCache, this.attnOut, this.uDMC0]) : null,
      o: mv(L2.wo, this.attnOut, this.tmpDim, dim, qDim),
      norm2: bgNorm(this.x, L2.postNorm, this.xn),
      gate: mv(L2.wgate, this.xn, this.g, inter, dim),
      up: mv(L2.wup, this.xn, this.u, inter, dim),
      gu: guOp(L2.wgate, L2.wup, this.xn, this.g, inter, dim),
      down: mv(L2.wdown, this.g, this.tmpDim, dim, inter),
    }));
    this.bgRopeQ = this._bg(this.pipes.rope, 1, [this.q, this.nHBuf]);
    this.bgRopeK = this._bg(this.pipes.rope, 1, [this.k, this.nKVBuf]);
    this.bgSilu = this._bg(this.pipes.silu_mul, 1, [this.g, this.u]);
    this.bgAddTmp = this._bg(this.pipes.add_res, 1, [this.x, this.tmpDim]);
    if (hasHead) {
      this.bgFinalNorm = bgNorm(this.x, this.finalNorm, this.xn);
      this.headOp = mv(this.headEntry, this.xn, this.logits, vocab, dim);
      if (this.headRows) this.headOp = { pipe: this.headOp.pipe + "_h", wgs: Math.ceil(vocab / this.headRows), bg: this._bg(this.pipes[this.headOp.pipe + "_h"], 1, this.headEntry.kind === "f32" ? [this.headEntry.buf, this.xn, this.logits, this._shape(vocab, dim)] : [this.headEntry.qs, this.headEntry.sc, this.xn, this.logits, this._shape(vocab, dim)]) };
    }
  }

  _shape(dOut, dIn) {
    const key = dOut + "," + dIn;
    if (!this._shapes[key])
      this._shapes[key] = this._buf(new Uint32Array([dOut, dIn, 0, 0]), GPUBufferUsage.UNIFORM);
    return this._shapes[key];
  }

  _shapeB(dOut, dIn, xs4, ys) {
    const key = "b" + dOut + "," + dIn + "," + xs4 + "," + ys;
    if (!this._shapes[key])
      this._shapes[key] = this._buf(new Uint32Array([dOut, dIn, xs4, ys]), GPUBufferUsage.UNIFORM);
    return this._shapes[key];
  }

  _buf(data, usage) {
    const src = ArrayBuffer.isView(data) ? data : new Uint8Array(data);
    const size = Math.ceil(src.byteLength / 4) * 4;
    const buf = this.device.createBuffer({ size, usage, mappedAtCreation: true });
    new Uint8Array(buf.getMappedRange()).set(
      new Uint8Array(src.buffer, src.byteOffset, src.byteLength));
    buf.unmap();
    return buf;
  }

  _bg(pipe, group, buffers) {
    return this.device.createBindGroup({
      layout: pipe.getBindGroupLayout(group),
      entries: buffers.map((b, i) => ({ binding: i, resource: DenseEngine._res(b) })),
    });
  }
  // a view is a 256-byte aligned range of a buffer that binds like a buffer of its own
  static _view(buffer, offset, size) { return { __view: true, buffer, offset, size }; }
  static _res(b) { return b && b.__view ? { buffer: b.buffer, offset: b.offset, size: b.size } : { buffer: b }; }
  static _raw(b) { return b && b.__view ? [b.buffer, b.offset] : [b, 0]; }

  _dispatch(pass, pipeName, bg, threads, wgSize = 64) {
    pass.setPipeline(this.pipes[pipeName]);
    pass.setBindGroup(0, this.bgCommonFor[pipeName]);
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroups(Math.ceil(threads / wgSize));
  }

  _dispatchOp(pass, op) {
    pass.setPipeline(this.pipes[op.pipe]);
    pass.setBindGroup(0, this.bgCommonFor[op.pipe]);
    pass.setBindGroup(1, op.bg);
    if (op.wgs > 32768) pass.dispatchWorkgroups(32768, Math.ceil(op.wgs / 32768)); else pass.dispatchWorkgroups(op.wgs);   // > 65535 per dimension is silently dropped
  }

  _setFrame(pos, seqLen) {
    this.device.queue.writeBuffer(this.frameBuf, 0, new Uint32Array([pos, seqLen]));
  }

  _encodeLayer(enc, i) {
    const { qDim, nH, nKV, headDim, kvDim, inter, dim } = this.dims;
    const L = this.layers[i], BG = this.layerBGs[i];
    const seqLen = this.pos + 1;
    if (this.fuse && BG.glue) {   // one pass: the glue kernel writes the K/V cache rows itself
      let pass = enc.beginComputePass();
      this._dispatch(pass, "rmsnorm", BG.norm1, 256, 256);
      this._dispatchOp(pass, BG.q);
      if (BG.k) this._dispatchOp(pass, BG.k);
      if (BG.v) this._dispatchOp(pass, BG.v);
      if (this.glue3 !== false && BG.rope3) {   // three exact multi-head kernels, no copies, one pass
        const g3 = (name, bg, x) => { pass.setPipeline(this.pipes[name]); pass.setBindGroup(0, this.bgCommonFor[name]); pass.setBindGroup(1, bg); pass.dispatchWorkgroups(x, 1); };
        const sfx = this.mergeQKV ? "1" : "";
        if (BG.hn3) g3("head_norm_dmc" + sfx, BG.hn3, Math.ceil((nH + nKV) / 32));
        g3("rope_dmc" + sfx, BG.rope3, Math.ceil((nH + nKV) * headDim / 2 / 64));
        g3("kv_store_d", BG.kv3, Math.ceil(kvDim / 64));
      } else if (this.fuseGlue !== false) {
        if (this.fuseNorm === false) {
          if (BG.qNorm) this._dispatch(pass, "head_norm", BG.qNorm, nH, 32);
          if (BG.kNorm) this._dispatch(pass, "head_norm", BG.kNorm, nKV, 32);
        }
        pass.setPipeline(this.pipes.attn_glue_d); pass.setBindGroup(0, this.bgCommonFor.attn_glue_d); pass.setBindGroup(1, this.fuseNorm === false ? BG.glueNN : BG.glue);
        pass.dispatchWorkgroups(nH + nKV, 1);
      } else {
        if (BG.qNorm) this._dispatch(pass, "head_norm", BG.qNorm, nH, 32);
        if (BG.kNorm) this._dispatch(pass, "head_norm", BG.kNorm, nKV, 32);
        this._dispatch(pass, "rope", this.bgRopeQ, nH * headDim / 2);
        this._dispatch(pass, "rope", this.bgRopeK, nKV * headDim / 2);
        pass.end();
        enc.copyBufferToBuffer(...DenseEngine._raw(this.k), L.kCache, this.pos * kvDim * 4, kvDim * 4);
        enc.copyBufferToBuffer(...DenseEngine._raw(this.v), L.vCache, this.pos * kvDim * 4, kvDim * 4);
        pass = enc.beginComputePass();
      }
      this._encodeAttn(pass, BG, seqLen);
      if (this.fuseAcc !== false) this._dispatchOp(pass, BG.oAcc);
      else { this._dispatchOp(pass, BG.o); this._dispatch(pass, "add_res", this.bgAddTmp, dim); }
      this._dispatch(pass, "rmsnorm", BG.norm2, 256, 256);
      if (BG.gu) this._dispatchOp(pass, BG.gu);
      else {
        this._dispatchOp(pass, BG.gate);
        this._dispatchOp(pass, BG.up);
        this._dispatch(pass, "silu_mul", this.bgSilu, inter);
      }
      if (this.fuseAcc !== false) this._dispatchOp(pass, BG.downAcc);
      else { this._dispatchOp(pass, BG.down); this._dispatch(pass, "add_res", this.bgAddTmp, dim); }
      pass.end();
      return;
    }
    {
      const pass = enc.beginComputePass();
      this._dispatch(pass, "rmsnorm", BG.norm1, 256, 256);
      this._dispatchOp(pass, BG.q);
      if (BG.k) this._dispatchOp(pass, BG.k);
      if (BG.v) this._dispatchOp(pass, BG.v);
      if (BG.qNorm) this._dispatch(pass, "head_norm", BG.qNorm, nH, 32);
      if (BG.kNorm) this._dispatch(pass, "head_norm", BG.kNorm, nKV, 32);
      this._dispatch(pass, "rope", this.bgRopeQ, nH * headDim / 2);
      this._dispatch(pass, "rope", this.bgRopeK, nKV * headDim / 2);
      pass.end();
    }
    enc.copyBufferToBuffer(...DenseEngine._raw(this.k), L.kCache, this.pos * kvDim * 4, kvDim * 4);
    enc.copyBufferToBuffer(...DenseEngine._raw(this.v), L.vCache, this.pos * kvDim * 4, kvDim * 4);
    {
      const pass = enc.beginComputePass();
      this._encodeAttn(pass, BG, seqLen);
      this._dispatchOp(pass, BG.o);
      this._dispatch(pass, "add_res", this.bgAddTmp, dim);
      this._dispatch(pass, "rmsnorm", BG.norm2, 256, 256);
      if (BG.gu) this._dispatchOp(pass, BG.gu);
      else {
        this._dispatchOp(pass, BG.gate);
        this._dispatchOp(pass, BG.up);
        this._dispatch(pass, "silu_mul", this.bgSilu, inter);
      }
      this._dispatchOp(pass, BG.down);
      this._dispatch(pass, "add_res", this.bgAddTmp, dim);
      pass.end();
    }
  }
  _encodeAttn(pass, BG, seqLen) {
    const { qDim, nH } = this.dims;
    if (this.attnFast && BG.scoresD) { this._encodeAttnD(pass, BG.scoresD, BG.softmaxD, BG.outD, seqLen, 1, this.bgCommonFor); return; }
    this._dispatch(pass, "attn_scores", BG.scores, nH * seqLen);
    this._dispatch(pass, "attn_softmax", BG.softmax, nH, 1);
    this._dispatch(pass, "attn_out", BG.attnOut, qDim);
  }

  // exact fast attention (engine/wgsl/dense.js) for nCols columns at once; `common` is the group-0
  // bind group set whose frame is column 0's (seqLen = column 0's length, column c adds c)
  _encodeAttnD(pass, bgS, bgM, bgO, seqLen0, nCols, common) {
    const { nH, nKV, headDim } = this.dims;
    const go = (name, bg, x, y, z) => { pass.setPipeline(this.pipes[name]); pass.setBindGroup(0, common[name]); pass.setBindGroup(1, bg); pass.dispatchWorkgroups(x, y, z); };
    go("attn_scores_d", bgS, Math.ceil((seqLen0 + nCols - 1) / 64), nKV, nCols);
    go("attn_softmax_d", bgM, nH, nCols, 1);
    go("attn_out_d", bgO, nKV * headDim / 32, nCols, 1);
  }

  _embedRowF32(id) {
    const { dim } = this.dims;
    const e = this.cpuEmbed;
    if (e.kind === "f32") return e.data.subarray(id * dim, (id + 1) * dim);
    const nb = dim / 32;
    const out = new Float32Array(dim);
    const rowB = id * nb;
    for (let b = 0; b < nb; b++) {
      const si = rowB + b;
      const s = f16ToF32((e.scales[si >> 1] >>> ((si & 1) * 16)) & 0xFFFF);
      if (e.kind === "q4") {
        const qBase = (rowB + b) * 16;
        for (let j = 0; j < 16; j++) {
          const q = e.qs[qBase + j];
          out[b * 32 + j] = s * ((q & 0xF) - 8);
          out[b * 32 + j + 16] = s * ((q >> 4) - 8);
        }
      } else {
        const qBase = (rowB + b) * 32;
        for (let i = 0; i < 32; i++) {
          const q = e.qs[qBase + i];
          out[b * 32 + i] = s * (q > 127 ? q - 256 : q);
        }
      }
    }
    return out;
  }

  _stageEmbed(tokenId) {
    const { dim } = this.dims;
    if (this.embedGPU) {
      const enc = this.device.createCommandEncoder();
      enc.copyBufferToBuffer(this.embedGPU, tokenId * dim * 4, this.x, 0, dim * 4);
      this.device.queue.submit([enc.finish()]);
    } else {
      this.device.queue.writeBuffer(this.x, 0, this._embedRowF32(tokenId));
    }
  }

  async _readback(srcBuf, stageBuf, n) {
    const enc = this.device.createCommandEncoder();
    enc.copyBufferToBuffer(srcBuf, 0, stageBuf, 0, n * 4);
    this.device.queue.submit([enc.finish()]);
    await stageBuf.mapAsync(GPUMapMode.READ);
    const out = Float32Array.from(new Float32Array(stageBuf.getMappedRange(), 0, n));
    stageBuf.unmap();
    return out;
  }

  // host peer: full forward for one token -> logits.
  // Whole token (all layers + head + the logits copy) is recorded into ONE command encoder and
  // submitted once. Encode-ahead: while the GPU runs token N and we wait for its logits, the command
  // buffer for position N + 1 is recorded too (a command buffer depends only on the position and the
  // runtime switches, never on the token id: the embedding row and the frame uniform are written with
  // queue.writeBuffer at call time), so the next call submits at once instead of spending the CPU
  // encode time on the critical path. Same commands, same bits. engine.encodeAhead = false for A/B.
  _fwdKey() { return [this.attnFast, this.fuse, this.fuseGlue, this.fuseAcc, this.fuseRms, this.fuseNorm, this.glue3].join(); }
  _encodeForward(pos) {
    const { vocab } = this.dims;
    const save = this.pos;
    this.pos = pos;
    const enc = this.device.createCommandEncoder();
    for (let i = 0; i < this.layers.length; i++) this._encodeLayer(enc, i);
    const pass = enc.beginComputePass();
    this._dispatch(pass, "rmsnorm", this.bgFinalNorm, 256, 256);
    this._dispatchOp(pass, this.headOp);
    pass.end();
    this.pos = save;
    this._stageI = (this._stageI || 0) ^ 1;   // alternate staging buffers: one can still be mapped
    const stage = this._stageI ? (this.stageLogits2 ||= this.device.createBuffer({ size: vocab * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ })) : this.stageLogits;
    enc.copyBufferToBuffer(this.logits, 0, stage, 0, vocab * 4);
    return { pos, key: this._fwdKey(), cb: enc.finish(), stage };
  }
  async forwardToken(tokenId, debugCapture) {
    const { dim, vocab } = this.dims;
    this._setFrame(this.pos, this.pos + 1);
    this._stageEmbed(tokenId);
    if (debugCapture) {           // slow path: per-layer readback for tests
      this._fwdPre = null;
      for (let i = 0; i < this.layers.length; i++) {
        const e2 = this.device.createCommandEncoder();
        this._encodeLayer(e2, i);
        this.device.queue.submit([e2.finish()]);
        debugCapture[this.lo + i] = (await this._readback(this.x, this.stageX, dim)).slice(0, 8);
      }
      const e3 = this.device.createCommandEncoder();
      const pass = e3.beginComputePass();
      this._dispatch(pass, "rmsnorm", this.bgFinalNorm, 256, 256);
      this._dispatchOp(pass, this.headOp);
      pass.end();
      this.device.queue.submit([e3.finish()]);
      const logits = await this._readback(this.logits, this.stageLogits, vocab);
      this.pos++;
      return logits;
    }
    const pre = this._fwdPre;
    this._fwdPre = null;
    const job = pre && pre.pos === this.pos && pre.key === this._fwdKey() ? pre : this._encodeForward(this.pos);
    this.device.queue.submit([job.cb]);
    const mapped = job.stage.mapAsync(GPUMapMode.READ);
    if (this.encodeAhead !== false && this.pos + 1 < this.maxSeq) this._fwdPre = this._encodeForward(this.pos + 1);
    await mapped;
    const logits = Float32Array.from(new Float32Array(job.stage.getMappedRange(), 0, vocab));
    job.stage.unmap();
    this.pos++;
    return logits;
  }

  // ---- batched prefill (4 prompt tokens per pass; weights read once per 4) ----
  _initBatch() {
    const { dim, qDim, kvDim, inter, nH, nKV } = this.dims;
    const dev = this.device;
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC;
    const al = (n) => Math.ceil(n * 4 / 256) * 256;         // 256-aligned slice stride, bytes
    const mkB = (n) => ({ buf: dev.createBuffer({ size: 4 * al(n), usage: S }), stride: al(n), n });
    // mergeQKV: q | k | v of a column are one row block of the merged GEMV's output; B.q/k/v are views
    // (column stride = the merged block's), so every kernel indexes them exactly as before
    const qkvB = this.mergeQKV ? mkB(qDim + 2 * kvDim) : null;
    const viewB = (off, n) => ({ buf: DenseEngine._view(qkvB.buf, off, qkvB.buf.size - off), stride: qkvB.stride, n, off });
    const B = this.B = {
      x: mkB(dim), xn: mkB(dim), ...(qkvB ? { qkv: qkvB, q: viewB(0, qDim), k: viewB(qDim * 4, kvDim), v: viewB((qDim + kvDim) * 4, kvDim) } : { q: mkB(qDim), k: mkB(kvDim), v: mkB(kvDim) }),
      attnOut: mkB(qDim), tmpDim: mkB(dim), g: mkB(inter), u: mkB(inter),
    };
    this.stageXB = dev.createBuffer({ size: 4 * dim * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    const slice = (b, c) => { const [buf, o] = DenseEngine._raw(b.buf); return { buffer: buf, offset: o + c * b.stride, size: b.n * 4 }; };
    this._bslice = slice;
    // per-column frame uniforms + per-column group0 for the per-token kernels
    this.frameBufsB = [0, 1, 2, 3].map(() => dev.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST }));
    const colPipes = ["rmsnorm", "head_norm", "rope", "attn_scores", "attn_softmax", "attn_out", "silu_mul", "add_res",
      ...(this.attnFastOn ? ["attn_scores_d", "attn_softmax_d", "attn_out_d"] : []), ...(this.fuseOn ? ["attn_glue_d", "rmsnorm_dmc", "head_norm_dmc", "rope_dmc", "kv_store_d", "head_norm_dmc1", "rope_dmc1"] : [])];
    this.bgCommonB = [0, 1, 2, 3].map((c) => {
      const m = {};
      for (const name of colPipes)
        m[name] = this._bg2g0(this.pipes[name], [{ buffer: this.cfgBuf }, { buffer: this.frameBufsB[c] }]);
      return m;
    });
    if (this.attnFastOn) {   // column strides (f32s) of the q input and the attention output
      this.uDMCq = this._buf(new Uint32Array([B.q.stride / 4, 0, 0, 0]), GPUBufferUsage.UNIFORM);
      this.uDMCo = this._buf(new Uint32Array([B.attnOut.stride / 4, 0, 0, 0]), GPUBufferUsage.UNIFORM);
    }
    // batched matvec op builder: whole B-buffers bound, strides in the uniform
    const mvB = (w, xB, yB, dOut, dIn, acc = false, sfx = "") => {   // acc: y += W x; sfx "_x": the exact verify pipes
      const base = w.kind === "q8" ? "matvec_q8" : w.kind === "q4" ? "matvec_q4" : "matvec";
      const pipe = base + "_coop_b" + (acc ? "_acc" : "") + sfx;
      const shp = this._shapeB(dOut, dIn, xB.stride / 16, yB.stride / 4);
      const bufs = w.kind === "f32" ? [w.buf, xB.buf, yB.buf, shp] : [w.qs, w.sc, xB.buf, yB.buf, shp];
      return { pipe, wgs: Math.ceil(dOut / this.rowsB), bg: this._bg(this.pipes[pipe], 1, bufs) };
    };
    const U = (a) => this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM);
    const uGlueB = this.fuseOn ? U([B.q.stride / 4, B.k.stride / 4, B.v.stride / 4, this.layers[0]?.qNorm ? 1 : 0]) : null;
    const uGlueBNN = this.fuseOn ? U([B.q.stride / 4, B.k.stride / 4, B.v.stride / 4, 0]) : null;
    const uGlueB1 = this.fuseOn && this.mergeQKV ? U([B.q.stride / 4, B.k.stride / 4, B.v.stride / 4, 0, qDim, 0, 0, 0]) : null;
    const uNormB = this.fuseOn ? U([dim, B.x.stride / 4, B.xn.stride / 4, 0]) : null;
    // the verify head: final norm + LM head for 4 columns with one read of the head weights, with the
    // exact ops (bit-identical per column to headFromHidden, like the layers' verify ops)
    if (this.hasHead && this.verifyX && this.fuseOn) {
      B.logits = mkB(this.dims.vocab);
      this.headVX = { norm: this._bg(this.pipes.rmsnorm_dmc, 1, [B.x.buf, this.finalNorm.buf, B.xn.buf, uNormB]), op: mvB(this.headEntry, B.xn, B.logits, this.dims.vocab, dim, false, "_x") };
    }
    // per-layer batched resources
    this.layerB = this.layers.map((L) => {
      const bgNormC = (xB, w, yB, c) => this._bg2res(this.pipes.rmsnorm,
        [slice(xB, c), { buffer: w.buf }, slice(yB, c), { buffer: this.nBufDim }]);
      return {
        glue: this.fuseOn ? this._bg(this.pipes.attn_glue_d, 1, [B.q.buf, B.k.buf, B.v.buf, (L.qNorm || { buf: this.x }).buf, (L.kNorm || { buf: this.x }).buf, L.kCache, L.vCache, uGlueB]) : null,
        glueNN: this.fuseOn ? this._bg(this.pipes.attn_glue_d, 1, [B.q.buf, B.k.buf, B.v.buf, (L.qNorm || { buf: this.x }).buf, (L.kNorm || { buf: this.x }).buf, L.kCache, L.vCache, uGlueBNN]) : null,
        hn3: !this.fuseOn || !L.qNorm ? null : this.mergeQKV ? this._bg(this.pipes.head_norm_dmc1, 1, [B.qkv.buf, L.qNorm.buf, L.kNorm.buf, uGlueB1])
          : this._bg(this.pipes.head_norm_dmc, 1, [B.q.buf, B.k.buf, L.qNorm.buf, L.kNorm.buf, uGlueB]),
        rope3: !this.fuseOn ? null : this.mergeQKV ? this._bg(this.pipes.rope_dmc1, 1, [B.qkv.buf, uGlueB1]) : this._bg(this.pipes.rope_dmc, 1, [B.q.buf, B.k.buf, uGlueB]),
        kv3: this.fuseOn ? this._bg(this.pipes.kv_store_d, 1, [B.k.buf, B.v.buf, L.kCache, L.vCache, uGlueB]) : null,
        norm1MC: this.fuseOn ? this._bg(this.pipes.rmsnorm_dmc, 1, [B.x.buf, L.inNorm.buf, B.xn.buf, uNormB]) : null,
        norm2MC: this.fuseOn ? this._bg(this.pipes.rmsnorm_dmc, 1, [B.x.buf, L.postNorm.buf, B.xn.buf, uNormB]) : null,
        oAcc: this.fuseOn ? mvB(L.wo, B.attnOut, B.x, dim, qDim, true) : null,
        downAcc: this.fuseOn ? mvB(L.wdown, B.g, B.x, dim, inter, true) : null,
        qkv: this.mergeQKV ? [mvB(L.wqkv, B.xn, B.qkv, qDim + 2 * kvDim, dim)] : [mvB(L.wq, B.xn, B.q, qDim, dim), mvB(L.wk, B.xn, B.k, kvDim, dim), mvB(L.wv, B.xn, B.v, kvDim, dim)],
        o: mvB(L.wo, B.attnOut, B.tmpDim, dim, qDim),
        gateUp: [mvB(L.wgate, B.xn, B.g, inter, dim), mvB(L.wup, B.xn, B.u, inter, dim)],
        gu: this._guOp(L.wgate, L.wup, B.xn.buf, B.g.buf, inter, dim, B.xn, B.g),
        down: mvB(L.wdown, B.g, B.tmpDim, dim, inter),
        cols: [0, 1, 2, 3].map((c) => ({
          norm1: bgNormC(B.x, L.inNorm, B.xn, c),
          norm2: bgNormC(B.x, L.postNorm, B.xn, c),
          qNorm: L.qNorm ? this._bg2res(this.pipes.head_norm, [slice(B.q, c), { buffer: L.qNorm.buf }, { buffer: this.nHBuf }]) : null,
          kNorm: L.kNorm ? this._bg2res(this.pipes.head_norm, [slice(B.k, c), { buffer: L.kNorm.buf }, { buffer: this.nKVBuf }]) : null,
          ropeQ: this._bg2res(this.pipes.rope, [slice(B.q, c), { buffer: this.nHBuf }]),
          ropeK: this._bg2res(this.pipes.rope, [slice(B.k, c), { buffer: this.nKVBuf }]),
          scores: this._bg2res(this.pipes.attn_scores, [slice(B.q, c), { buffer: L.kCache }, { buffer: this.scores }]),
          softmax: this._bg2res(this.pipes.attn_softmax, [{ buffer: this.scores }]),
          attnOut: this._bg2res(this.pipes.attn_out, [{ buffer: this.scores }, { buffer: L.vCache }, slice(B.attnOut, c)]),
          addTmp: this._bg2res(this.pipes.add_res, [slice(B.x, c), slice(B.tmpDim, c)]),
          silu: this._bg2res(this.pipes.silu_mul, [slice(B.g, c), slice(B.u, c)]),
        })),
        scoresD: this.attnFastOn ? this._bg(this.pipes.attn_scores_d, 1, [B.q.buf, L.kCache, this.scores, this.uDMCq]) : null,
        softmaxD: this.attnFastOn ? this._bg(this.pipes.attn_softmax_d, 1, [this.scores]) : null,
        outD: this.attnFastOn ? this._bg(this.pipes.attn_out_d, 1, [this.scores, L.vCache, B.attnOut.buf, this.uDMCo]) : null,
        // the exact verify ops (see verifyX in _init): the same GEMVs with the single-token kernels' workgroups
        x: !this.verifyX ? null : {
          oAcc: this.fuseOn ? mvB(L.wo, B.attnOut, B.x, dim, qDim, true, "_x") : null,
          downAcc: this.fuseOn ? mvB(L.wdown, B.g, B.x, dim, inter, true, "_x") : null,
          qkv: this.mergeQKV ? [mvB(L.wqkv, B.xn, B.qkv, qDim + 2 * kvDim, dim, false, "_x")] : [mvB(L.wq, B.xn, B.q, qDim, dim, false, "_x"), mvB(L.wk, B.xn, B.k, kvDim, dim, false, "_x"), mvB(L.wv, B.xn, B.v, kvDim, dim, false, "_x")],
          o: mvB(L.wo, B.attnOut, B.tmpDim, dim, qDim, false, "_x"),
          gateUp: [mvB(L.wgate, B.xn, B.g, inter, dim, false, "_x"), mvB(L.wup, B.xn, B.u, inter, dim, false, "_x")],
          gu: this._guOp(L.wgate, L.wup, B.xn.buf, B.g.buf, inter, dim, B.xn, B.g, "_x"),
          down: mvB(L.wdown, B.g, B.tmpDim, dim, inter, false, "_x"),
        },
      };
    });
  }

  _bg2g0(pipe, resources) {
    return this.device.createBindGroup({
      layout: pipe.getBindGroupLayout(0),
      entries: resources.map((r, i) => ({ binding: i, resource: r })),
    });
  }
  _bg2res(pipe, resources) {
    return this.device.createBindGroup({
      layout: pipe.getBindGroupLayout(1),
      entries: resources.map((r, i) => ({ binding: i, resource: r })),
    });
  }
  _dCol(pass, name, col, bg, threads, wgSize = 64) {
    pass.setPipeline(this.pipes[name]);
    pass.setBindGroup(0, this.bgCommonB[col][name]);
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroups(Math.ceil(threads / wgSize));
  }

  // n: columns that are real tokens (1..4; the GEMVs always compute 4, the extra columns only touch
  // the batch buffers). exact: the verify ops (see _initBatch), whose columns are bit-identical to
  // single-token decoding.
  _encodeLayerBatch(enc, i, basePos, n = 4, exact = false) {
    const { qDim, nH, nKV, headDim, kvDim, inter, dim } = this.dims;
    const L = this.layers[i], LB0 = this.layerB[i], B = this.B;
    const LB = exact && LB0.x ? { ...LB0, ...LB0.x } : LB0;
    if (this.fuse && LB.glue) {   // one pass, one dispatch per stage for all 4 columns (glue writes the caches)
      let pass = enc.beginComputePass();
      const mc = (name, bg, x) => { pass.setPipeline(this.pipes[name]); pass.setBindGroup(0, this.bgCommonB[0][name]); pass.setBindGroup(1, bg); pass.dispatchWorkgroups(x, n); };
      const rms = (bgMC, which) => { if (this.fuseRms !== false) mc("rmsnorm_dmc", bgMC, 1); else for (let c = 0; c < n; c++) this._dCol(pass, "rmsnorm", c, LB.cols[c][which], 256, 256); };
      rms(LB.norm1MC, "norm1");
      for (const op of LB.qkv) this._dispatchOp(pass, op);
      if (this.glue3 !== false && LB.rope3) {   // three exact kernels for all columns, no copies
        const sfx = this.mergeQKV ? "1" : "";
        if (LB.hn3) mc("head_norm_dmc" + sfx, LB.hn3, Math.ceil((nH + nKV) / 32));
        mc("rope_dmc" + sfx, LB.rope3, Math.ceil((nH + nKV) * headDim / 2 / 64));
        mc("kv_store_d", LB.kv3, Math.ceil(kvDim / 64));
      } else if (this.fuseGlue === false) {   // the reference head_norm / rope dispatches and cache copies
        for (let c = 0; c < n; c++) {
          const C = LB.cols[c];
          if (C.qNorm) this._dCol(pass, "head_norm", c, C.qNorm, nH, 32);
          if (C.kNorm) this._dCol(pass, "head_norm", c, C.kNorm, nKV, 32);
          this._dCol(pass, "rope", c, C.ropeQ, nH * headDim / 2);
          this._dCol(pass, "rope", c, C.ropeK, nKV * headDim / 2);
        }
        pass.end();
        for (let c = 0; c < n; c++) {
          enc.copyBufferToBuffer(DenseEngine._raw(B.k.buf)[0], DenseEngine._raw(B.k.buf)[1] + c * B.k.stride, L.kCache, (basePos + c) * kvDim * 4, kvDim * 4);
          enc.copyBufferToBuffer(DenseEngine._raw(B.v.buf)[0], DenseEngine._raw(B.v.buf)[1] + c * B.v.stride, L.vCache, (basePos + c) * kvDim * 4, kvDim * 4);
        }
        pass = enc.beginComputePass();
      } else {
        if (this.fuseNorm === false) for (let c = 0; c < n; c++) {
          const C = LB.cols[c];
          if (C.qNorm) this._dCol(pass, "head_norm", c, C.qNorm, nH, 32);
          if (C.kNorm) this._dCol(pass, "head_norm", c, C.kNorm, nKV, 32);
        }
        mc("attn_glue_d", this.fuseNorm === false ? LB.glueNN : LB.glue, nH + nKV);
      }
      this._encodeAttnBatch(pass, LB, basePos, n);
      if (this.fuseAcc !== false) this._dispatchOp(pass, LB.oAcc);
      else { this._dispatchOp(pass, LB.o); for (let c = 0; c < n; c++) this._dCol(pass, "add_res", c, LB.cols[c].addTmp, dim); }
      rms(LB.norm2MC, "norm2");
      if (LB.gu) this._dispatchOp(pass, LB.gu);
      else {
        for (const op of LB.gateUp) this._dispatchOp(pass, op);
        for (let c = 0; c < n; c++) this._dCol(pass, "silu_mul", c, LB.cols[c].silu, inter);
      }
      if (this.fuseAcc !== false) this._dispatchOp(pass, LB.downAcc);
      else { this._dispatchOp(pass, LB.down); for (let c = 0; c < n; c++) this._dCol(pass, "add_res", c, LB.cols[c].addTmp, dim); }
      pass.end();
      return;
    }
    {
      const pass = enc.beginComputePass();
      for (let c = 0; c < n; c++) this._dCol(pass, "rmsnorm", c, LB.cols[c].norm1, 256, 256);
      for (const op of LB.qkv) this._dispatchOp(pass, op);
      for (let c = 0; c < n; c++) {
        const C = LB.cols[c];
        if (C.qNorm) this._dCol(pass, "head_norm", c, C.qNorm, nH, 32);
        if (C.kNorm) this._dCol(pass, "head_norm", c, C.kNorm, nKV, 32);
        this._dCol(pass, "rope", c, C.ropeQ, nH * headDim / 2);
        this._dCol(pass, "rope", c, C.ropeK, nKV * headDim / 2);
      }
      pass.end();
    }
    for (let c = 0; c < n; c++) {
      enc.copyBufferToBuffer(DenseEngine._raw(B.k.buf)[0], DenseEngine._raw(B.k.buf)[1] + c * B.k.stride, L.kCache, (basePos + c) * kvDim * 4, kvDim * 4);
      enc.copyBufferToBuffer(DenseEngine._raw(B.v.buf)[0], DenseEngine._raw(B.v.buf)[1] + c * B.v.stride, L.vCache, (basePos + c) * kvDim * 4, kvDim * 4);
    }
    {
      const pass = enc.beginComputePass();
      this._encodeAttnBatch(pass, LB, basePos, n);
      this._dispatchOp(pass, LB.o);
      for (let c = 0; c < n; c++) this._dCol(pass, "add_res", c, LB.cols[c].addTmp, dim);
      for (let c = 0; c < n; c++) this._dCol(pass, "rmsnorm", c, LB.cols[c].norm2, 256, 256);
      if (LB.gu) this._dispatchOp(pass, LB.gu);
      else {
        for (const op of LB.gateUp) this._dispatchOp(pass, op);
        for (let c = 0; c < n; c++) this._dCol(pass, "silu_mul", c, LB.cols[c].silu, inter);
      }
      this._dispatchOp(pass, LB.down);
      for (let c = 0; c < n; c++) this._dCol(pass, "add_res", c, LB.cols[c].addTmp, dim);
      pass.end();
    }
  }
  _encodeAttnBatch(pass, LB, basePos, n = 4) {
    const { qDim, nH } = this.dims;
    if (this.attnFast && LB.scoresD) { this._encodeAttnD(pass, LB.scoresD, LB.softmaxD, LB.outD, basePos + 1, n, this.bgCommonB[0]); return; }
    for (let c = 0; c < n; c++) {
      const C = LB.cols[c];
      this._dCol(pass, "attn_scores", c, C.scores, nH * (basePos + c + 1));
      this._dCol(pass, "attn_softmax", c, C.softmax, nH, 1);
      this._dCol(pass, "attn_out", c, C.attnOut, qDim);
    }
  }

  _stageEmbedBatchCol(enc, id, c) {
    const { dim } = this.dims;
    if (this.embedGPU) enc.copyBufferToBuffer(this.embedGPU, id * dim * 4, this.B.x.buf, c * this.B.x.stride, dim * 4);
    else this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, this._embedRowF32(id));
  }
  async _runBatchAndRead(basePos, n = 4, exact = false) {
    const { dim } = this.dims;
    const enc = this.device.createCommandEncoder();
    if (this._pendingEmbeds) { for (const [id, c] of this._pendingEmbeds) this._stageEmbedBatchCol(enc, id, c); this._pendingEmbeds = null; }
    for (let l = 0; l < this.layers.length; l++) this._encodeLayerBatch(enc, l, basePos, n, exact);
    for (let c = 0; c < n; c++) enc.copyBufferToBuffer(this.B.x.buf, c * this.B.x.stride, this.stageXB, c * dim * 4, dim * 4);
    this.device.queue.submit([enc.finish()]);
    await this.stageXB.mapAsync(GPUMapMode.READ, 0, n * dim * 4);
    const out = Float32Array.from(new Float32Array(this.stageXB.getMappedRange(0, n * dim * 4), 0, n * dim));
    this.stageXB.unmap();
    this.pos = basePos + n;
    return out;
  }
  // host, split mode: 1..4 tokens -> their hiddens for the next peer. verify (a speculative verify
  // frame; the room passes { base, total }): the exact ops, so every column is bit-identical to what
  // embedRun would give for that token (plain decoding), and a verified token is a decoded token.
  async embedRunBatch(ids, basePos, verify = false) {
    if (!this.B) this._initBatch();
    const n = ids.length;
    this.pos = basePos;
    this._pendingEmbeds = ids.map((id, c) => [id, c]);
    for (let c = 0; c < n; c++)
      this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c, basePos + c + 1]));
    if (verify) this._vBase = basePos - (typeof verify === "object" ? verify.base : 0);
    return this._runBatchAndRead(basePos, n, !!verify);
  }
  // worker, split mode: 1..4 hiddens in, my layers, as many hiddens out (verify: see embedRunBatch)
  async runHiddenBatch(xs, basePos, verify = false) {
    if (!this.B) this._initBatch();
    const { dim } = this.dims;
    const n = xs.length / dim;
    this.pos = basePos;
    for (let c = 0; c < n; c++) {
      this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c, basePos + c + 1]));
      this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, xs.subarray(c * dim, (c + 1) * dim));
    }
    if (verify) this._vBase = basePos - (typeof verify === "object" ? verify.base : 0);
    return this._runBatchAndRead(basePos, n, !!verify);
  }
  // a rejected speculative suffix: the caches past the accepted tokens are simply overwritten by the
  // next frame (attention reads rows [0, pos] only), so rolling back is setting pos (checkpoints
  // saved next then hold exactly the accepted context). The room sends it with the next frame.
  restoreDN(k) { if (this._vBase != null) this.pos = this._vBase + k + 1; }

  // consume prompt tokens (no logits): chunks of 4 through the batched path,
  // remainder through the single-token fast path.
  async prefillTokens(ids) {
    if (!this.B && this.hasEmbed) this._initBatch();
    let i = 0;
    let sinceSync = 0;
    while (this.B && ids.length - i >= 4) {
      const basePos = this.pos;
      for (let c = 0; c < 4; c++) {
        this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c, basePos + c + 1]));
        if (this.embedGPU) {
          const enc0 = this.device.createCommandEncoder();
          enc0.copyBufferToBuffer(this.embedGPU, ids[i + c] * this.dims.dim * 4, this.B.x.buf, c * this.B.x.stride, this.dims.dim * 4);
          this.device.queue.submit([enc0.finish()]);
        } else {
          this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, this._embedRowF32(ids[i + c]));
        }
      }
      const enc = this.device.createCommandEncoder();
      for (let l = 0; l < this.layers.length; l++) this._encodeLayerBatch(enc, l, basePos);
      // hidden of the last column becomes the running x for any tail tokens
      enc.copyBufferToBuffer(this.B.x.buf, 3 * this.B.x.stride, this.x, 0, this.dims.dim * 4);
      this.device.queue.submit([enc.finish()]);
      this.pos += 4;
      i += 4;
      if (++sinceSync >= 4) { await this.device.queue.onSubmittedWorkDone(); sinceSync = 0; }
    }
    for (; i < ids.length; i++) {
      await this.prefillToken(ids[i]);
      if (i % 8 === 7) await this.device.queue.onSubmittedWorkDone();
    }
    await this.device.queue.onSubmittedWorkDone();
  }

  // prefill fast path: run the layers for a prompt token, skip head + readback
  // (the head is ~11% of the weight traffic and the logits go unused).
  async prefillToken(tokenId) {
    this._setFrame(this.pos, this.pos + 1);
    this._stageEmbed(tokenId);
    const enc = this.device.createCommandEncoder();
    for (let i = 0; i < this.layers.length; i++) this._encodeLayer(enc, i);
    this.device.queue.submit([enc.finish()]);
    this.pos++;
    // fire-and-forget: the queue is ordered, so later work sees this token's
    // caches. Callers apply backpressure every few tokens via
    // device.queue.onSubmittedWorkDone() to bound queued work.
  }

  // host peer, split mode: embed + local layers -> hidden for next peer
  async embedRun(tokenId, pos) {
    const { dim } = this.dims;
    this.pos = pos;
    this._setFrame(pos, pos + 1);
    this._stageEmbed(tokenId);
    const enc = this.device.createCommandEncoder();
    for (let i = 0; i < this.layers.length; i++) this._encodeLayer(enc, i);
    this.device.queue.submit([enc.finish()]);
    return await this._readback(this.x, this.stageX, dim);
  }

  // host peer, split mode: final norm + lm_head over returned hidden
  async headFromHidden(xIn) {
    const { vocab } = this.dims;
    this.device.queue.writeBuffer(this.x, 0, xIn);
    const enc = this.device.createCommandEncoder();
    {
      const pass = enc.beginComputePass();
      this._dispatch(pass, "rmsnorm", this.bgFinalNorm, 256, 256);
      this._dispatchOp(pass, this.headOp);
      pass.end();
    }
    this.device.queue.submit([enc.finish()]);
    return await this._readback(this.logits, this.stageLogits, vocab);
  }

  // final norm + LM head for n hiddens (n * dim floats) in ONE submit and one readback -> array of n
  // logits vectors, each bit-identical to headFromHidden's. With the exact verify ops (verifyX) the
  // head weights are read once per 4 columns (headVX); otherwise column by column with the
  // single-token kernels. engine.batchHead = false forces the latter (A/B).
  async headBatch(hs, n = hs.length / this.dims.dim) {
    const { dim, vocab } = this.dims;
    const dev = this.device;
    if (!this.hsIn || this.hsIn.size < n * dim * 4) {
      this.hsIn?.destroy();
      this.hsIn = dev.createBuffer({ size: Math.max(8, n) * dim * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC });
    }
    if (!this.stageLogitsN || this.stageLogitsN.size < n * vocab * 4) {
      this.stageLogitsN?.destroy();
      this.stageLogitsN = dev.createBuffer({ size: Math.max(8, n) * vocab * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    }
    if (this.hasEmbed && !this.B) this._initBatch();
    dev.queue.writeBuffer(this.hsIn, 0, hs, 0, n * dim);
    const enc = dev.createCommandEncoder();
    if (this.headVX && this.batchHead !== false) {
      const B = this.B, V = this.headVX;
      for (let c0 = 0; c0 < n; c0 += 4) {
        const m = Math.min(4, n - c0);
        for (let c = 0; c < m; c++) enc.copyBufferToBuffer(this.hsIn, (c0 + c) * dim * 4, B.x.buf, c * B.x.stride, dim * 4);
        const pass = enc.beginComputePass();
        pass.setPipeline(this.pipes.rmsnorm_dmc); pass.setBindGroup(0, this.bgCommonB[0].rmsnorm_dmc); pass.setBindGroup(1, V.norm); pass.dispatchWorkgroups(1, m);
        this._dispatchOp(pass, V.op);
        pass.end();
        for (let c = 0; c < m; c++) enc.copyBufferToBuffer(B.logits.buf, c * B.logits.stride, this.stageLogitsN, (c0 + c) * vocab * 4, vocab * 4);
      }
    } else for (let c = 0; c < n; c++) {
      enc.copyBufferToBuffer(this.hsIn, c * dim * 4, this.x, 0, dim * 4);
      const pass = enc.beginComputePass();
      this._dispatch(pass, "rmsnorm", this.bgFinalNorm, 256, 256);
      this._dispatchOp(pass, this.headOp);
      pass.end();
      enc.copyBufferToBuffer(this.logits, 0, this.stageLogitsN, c * vocab * 4, vocab * 4);
    }
    dev.queue.submit([enc.finish()]);
    await this.stageLogitsN.mapAsync(GPUMapMode.READ, 0, n * vocab * 4);
    const all = new Float32Array(this.stageLogitsN.getMappedRange(0, n * vocab * 4)).slice();
    this.stageLogitsN.unmap();
    const out = [];
    for (let c = 0; c < n; c++) out.push(all.subarray(c * vocab, (c + 1) * vocab));
    return out;
  }

  // ---- speculative decoding with drafts from outside the model ----
  // A dense model has no draft head, so the drafts come from the caller: prompt lookup (tokens that
  // followed the same n-gram earlier in the context, room/lookup.js) or a small draft model of the
  // same tokenizer. One verify pass runs tNext and the drafts at positions pos .. pos+K (in a room:
  // ONE lap round the chain, runTrunk), and the usual acceptance rule keeps the drafts the model
  // itself would have sampled, so the output is exactly plain decoding's: the verify columns are
  // bit-identical to single-token decoding (the exact batch ops, headBatch), and a sampler draws the
  // same token from the same logits. Same call as Qwen35Engine.specStepDrafts.
  //   tNext: the token already chosen for this.pos. Returns 1..K+1 new tokens; the last one is the
  //   next tNext (not yet written), all before it were written into the caches.
  //   runTrunk(tokens, pos) -> the final hiddens of all columns (chain mode; the host's own layers
  //   run inside it); onReject(k): the other devices' rollback to column k (see restoreDN).
  async specStepDrafts(tNext, sample, drafts, { runTrunk = null, onReject = null } = {}) {
    const pos = this.pos, { dim } = this.dims;
    const K = Math.max(0, Math.min(this.maxDrafts, drafts.length, this.maxSeq - pos - 1));
    drafts = drafts.slice(0, K);
    const tokens = [tNext, ...drafts], n = K + 1;
    let hs;
    if (runTrunk) hs = await runTrunk(tokens, pos);
    else {
      hs = new Float32Array(n * dim);
      for (let c0 = 0; c0 < n; c0 += 4) {
        const m = Math.min(4, n - c0);
        hs.set(await this.embedRunBatch(tokens.slice(c0, c0 + m), pos + c0, { base: c0, total: n }), c0 * dim);
      }
    }
    const lgs = await this.headBatch(hs, n);
    const out = [];
    let a = 0;
    for (let k = 0; k <= K; k++) {
      const t = sample(lgs[k]);
      out.push(t);
      if (k < K && t === drafts[k]) a++; else break;
    }
    const st = this.specStats ||= { steps: 0, drafts: 0, accepted: 0 };
    st.steps++; st.drafts += K; st.accepted += a;
    this._vBase = pos;
    this.pos = pos + a + 1;
    this.lastHidden = hs.slice(a * dim, (a + 1) * dim);   // the final hidden of the last written token
    if (a < K && onReject) await onReject(a);
    return out;
  }

  // worker peer: hidden in, my layers, hidden out
  async runHidden(xIn, pos) {
    const { dim } = this.dims;
    this.pos = pos;
    this._setFrame(pos, pos + 1);
    this.device.queue.writeBuffer(this.x, 0, xIn);
    const enc = this.device.createCommandEncoder();
    for (let i = 0; i < this.layers.length; i++) this._encodeLayer(enc, i);
    this.device.queue.submit([enc.finish()]);
    const h = await this._readback(this.x, this.stageX, dim);
    this.pos = pos + 1;   // the caches now hold [0, pos]: a checkpoint saved before the next frame keeps this row (as the batched path does)
    return h;
  }
}
