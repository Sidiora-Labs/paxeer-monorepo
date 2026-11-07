// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/qwen4exp_batch.js (MIT; see LICENSE-ENGINE).
// Batched passes for Qwen3.8-Flash-Next (engine/qwen4exp.js): prompt prefill in chunks of up to maxCols tokens and
// speculative verify passes. Roadmap 33 PR plan items 5 (batched / wide prefill, verify) and 9 (QSA in them).
//
// A pass runs nCols consecutive tokens through every layer at once, on the decode engine's own state (DeltaNet conv
// and S, the PLE ring, the K / V cache, QSA's rawK / pooled keys, the n-gram history and pos) and its weight buffers
// (the joined / fused ones from each layer's st.w), so prefill and decode interleave freely. Two op lists:
//   exact  the multi-column decode kernels below: a pass leaves the engine exactly where nCols decode steps would,
//          bit for bit. Verify passes and passes narrower than FAST_MIN_COLS use them, and every pass with exact: true.
//   wide   prefill passes of FAST_MIN_COLS+ columns (maxCols 1024 by default, 1021 within QSA's default raw-key
//          ring): the kernels of engine/wgsl/qwen4exp_wide.js, tiled GEMMs for the projections, the expert-grouped
//          MoE, attention per KV head and column group (on the engine's f32 or f16 K / V cache), DeltaNet's norm out
//          of its sequential loop. Another summation order than decode, so
//          within a tolerance of it: f32, one pass's rounding is ~1e-6 relative on the residual (4 layers: p99 3e-6,
//          max 2e-4 per column), but over long prompts on the whole model a router (or QSA block score) near tie flips
//          at some column and the recurrent state / attention carry it on: past ~2K positions the median column is
//          ~3e-3 from the exact kernels from ~layer 20 on and single columns up to ~1e-1, as much as two wide widths
//          differ from each other (docs/kernels.md). dp4a: true, llama.cpp's int8-activation numerics, ~1e-2 per pass.
//          Checked per layer by tests/test_q38flash_batch.js and on the goldens (FULL=1).
// The op lists mirror the decode engine's: per layer two halves
// { pre, site, rest, comb }, laid out fused as forwardToken runs them (each HC combine folded into the next site,
// _halfOps), with the multi-column kernels of engine/wgsl/qwen4exp_batch.js (the decode kernels with a column index;
// gemv_mc shares each weight read across GEMV_MC_COLS columns). Ops carry dyn(pos, nCols) and run through the
// engine's _encode. QSA: engine/qwen4exp_qsa.js, the decode path's rule per pass (dense while every column has
// <= 2051 cells, the selected cells past that), on this module's own nCols-wide scratch.
//
//   const pb = await Qwen4ExpBatch.create(engine, weights);          // { maxCols, exact, dp4a, verifyCols }
//   const r = await pb.prefill(promptIds, { wantLogits: true });    // { id, logits } (head) or { hidden }
//   const v = await pb.verify([last, ...draft]);                     // { ids }: a greedy id per column
//   await pb.accept(n);                                              // keep the first n columns of that pass
//
// Verify keeps a copy of the recurrent state (DeltaNet conv + S, the PLE ring) from before the pass; accept(n < N)
// restores it, rolls the n-gram history back and re-runs the first n columns (no head). The K / V cache and QSA's
// rawK need no copy (the re-run rewrites them; the raw-key ring's rows of the block open before the pass are not
// touched by it, since a pass has at most ring - 3 columns) and a pooled key is re-pooled when its block completes again.
// Cost: the re-run is the whole trunk for n columns (MoE expert reads included), about one more pass, and every
// verify copies ~115 MB of state. Deferred (#348 review finding 3): replace it with the Qwen3.5 engine's replay
// rollback (save each column's DeltaNet inputs, replay only the delta step from S before the pass) before MTP
// drafting is built on verify (docs/kernels.md).
//
// Expert offload (engine/expert_store.js, engine created with expertRegion: true): an offloaded layer's pass splits
// after its router (op.cut). A wide pass (nCols x K >= half the experts) copies the whole layer into the store's region
// (ExpertStore.loadLayer, no readback); a narrow one (verify) reads the nCols x K ids back and copies only those
// experts (loadExperts). The expert kernels (exact or grouped) then run on regionEntry() with the original expert ids:
// the same kernels on the same bytes at another address, so an offloaded pass gives the resident bits. Without the
// region create() refuses offloaded layers (the engine's _wq would too: notParked). A disk-tier layer's experts are
// read from its file (ExpertStore.loadLayerAsync / loadExpertsAsync) after a submit, so a 1024-column pass reads each
// disk layer once instead of once per column.
import * as MC from "./wgsl/qwen4exp_batch.js";
import * as WX from "./wgsl/qwen4exp_wide.js";
import { probeDp4a } from "./wgsl/gemm_wide.js";
import { qsaScratch, qsaSetFrame, qsaBind, qsaStateOps, qsaSelectOps, qsaRingCols, QSA_SEL_STRIDE } from "./qwen4exp_qsa.js";
import { QSA_PROJ } from "./wgsl/qsa.js";
import { qsaMaxCols } from "./qsa.js";

const U = GPUBufferUsage;
const fr = Math.fround;
const SUBMIT_LAYERS = 12;   // layers per submit, as forwardToken
const WIDE_COLS = 1024;     // default columns per pass with the wide kernels (fewer for long contexts: the scratch bindings)
const LAYERED_COLS = 4096;  // tokens a layered prompt (disk-tier layers: passesLayered) takes through each layer at once
// A layered prompt over layers parked in RAM (a room node's shared region) never waits on anything real: each layer's
// experts go to the GPU by writeBuffer, and every await resolves at once, so a 4096-token group held the event loop
// for its whole run (~7 s a group on an RTX 5070 at 31 layers, longer deep into a 32K prompt) and the host dropped the
// node as silent (room/liveness.js SILENT_MS, 2026-10-14 Mac + PC run). It gives the loop a turn every YIELD_MS, as
// the disk path's reads already do.
const YIELD_MS = 50;
const yieldLoop = () => new Promise((r) => (typeof setImmediate === "function" ? setImmediate(r) : setTimeout(r, 0)));

export class Qwen4ExpBatch {
  static async create(engine, weights, opts = {}) { const b = new Qwen4ExpBatch(); await b._init(engine, weights, opts); return b; }

  // weights: the object the engine was built from (its unfused entries; the joined / fused ones come from st.w).
  // maxCols: columns per pass (longer prompts run in several); verifyCols: the most columns a pass with the head on
  // every column (verify) takes, which sizes its logits (vocab x 4 B per column).
  // exact: only the bit-exact kernels (every pass == the decode steps, bit for bit; the debugging reference). Otherwise
  // passes of at least FAST_MIN_COLS columns that do not take the head on every column (prefill, not verify) run the
  // wide kernels (engine/wgsl/qwen4exp_wide.js): tiled GEMMs and the expert-grouped MoE, within a tolerance of decode.
  // dp4a: the wide GEMMs and the grouped MoE on activations quantized per 32 values (llama.cpp's MMQ numerics; about
  // 1.3x faster on the GB10, the residual ~1e-2 from decode): true, "moe" (the routed experts only) or "dense" (the
  // projections only), when the device compiles dot4I8Packed. Default false: f32 tiles (one pass ~1e-6 from decode; see the header on long prompts).
  // maxBinding: a lower binding limit than the device's (tests: the maxCols cap of a smaller device / a longer maxSeq).
  // layeredCols: with disk-tier layers, the tokens passesLayered takes through each layer at once (their residuals,
  // layeredCols x hDim x 4 B, wait in one buffer; 0: pass after pass through every layer, as without disk layers)
  async _init(eng, W, { maxCols = null, verifyCols = 8, exact = false, dp4a = false, maxBinding = Infinity, layeredCols = LAYERED_COLS } = {}) {
    this.layeredCols = Math.max(0, +layeredCols || 0);
    // the default width also fits QSA's raw-key ring (engine qsaRing: a pass completes blocks of up to maxCols + 3
    // rows, 1021 columns with the default 1024-row ring); an explicit maxCols past the ring is refused below
    if (maxCols == null && eng.qsa) maxCols = Math.min(exact ? 64 : WIDE_COLS, qsaRingCols(eng.qsaRing));
    maxCols ??= exact ? 64 : WIDE_COLS;
    {   // the widest scratch bindings must fit the device: QSA's scores (maxCols x maxSeq / 4), the experts' outputs
      const lim = Math.min(eng.device.limits?.maxStorageBufferBindingSize ?? 128 << 20, maxBinding), c = eng.cfg;
      maxCols = Math.min(maxCols, Math.floor(lim / (c.K * c.dim * 4)), ...(eng.qsa ? [qsaMaxCols({ maxSeq: eng.maxSeq, maxBinding: lim })] : []));
    }
    if (dp4a && !exact && !await probeDp4a(eng.device)) dp4a = false;
    // which wide kernels run on dp4a: true (all), "moe" (the routed experts only), "dense" (the projections only)
    this.dpMoe = !exact && (dp4a === true || dp4a === "moe");
    this.dpDense = !exact && (dp4a === true || dp4a === "dense");
    this.dp4a = this.dpMoe || this.dpDense;
    const offSet = eng.offLayers || new Set();
    if (offSet.size && !eng.experts?.region) throw new Error(`qwen4exp batch: layers ${[...offSet].join(",")} have offloaded experts and the store has no `
      + "whole-layer region: create the engine with expertRegion: true (batched passes route offloaded layers through ExpertStore.loadLayer / loadExperts)");
    // the disk tier (#349): a disk layer fills the region with loadLayerAsync / loadExpertsAsync after a submit
    // (_encodeLayers), as the Qwen3.5 engine's grouped prefill does (engine/qwen35.js _regionLoad / _regionLoadDisk)
    const { device, cfg } = eng, wq = eng._wq;
    if (!wq) throw new Error("qwen4exp batch: the engine does not expose its weights (_wq)");
    Object.assign(this, { eng, device, cfg, maxCols, verifyCols: Math.min(verifyCols, maxCols), exact });
    // QSA's raw-key ring holds the rows of the blocks a pass completes: up to maxCols + 3 (engine/wgsl/qsa.js). The
    // default width is capped to the ring above; an explicit maxCols past it is refused
    if (eng.qsa && maxCols > qsaRingCols(eng.qsaRing))
      throw new Error(`qwen4exp batch: passes of ${maxCols} columns need a QSA raw-key ring of >= ${maxCols + 3} rows; the engine has ${eng.qsaRing} (Qwen4ExpEngine qsaRing)`);
    const d = cfg.dim, hc = cfg.hc, HD = cfg.hDim, ss = cfg.ssm, K = cfg.K, nH = cfg.nHeads, nKV = cfg.nKVHeads, hd = cfg.headDim;
    const { convDim, keyDim } = eng.dn, N = maxCols, eps = cfg.eps, C = MC.GEMV_MC_COLS, rank = cfg.hcRank;
    const nQ = 2 * nH * hd, nK = nKV * hd, qs = nQ + 2 * nK, qkvzS = convDim + ss.inner, ls = cfg.nExp + 1;
    if (rank % 4) throw new Error("qwen4exp batch: the HC rank must be a multiple of 4");

    // ---- pipelines (the GEMVs per lanes-per-row variant, compiled on first use)
    const pipe = (code, label) => device.createComputePipelineAsync({ layout: "auto", label,
      compute: { module: device.createShaderModule({ code, label }), entryPoint: "main" } });
    const P = this.P = {};
    await Promise.all([
      ...["q4", "q8"].map(async (k) => { P["gu_" + k] = await pipe(MC.moeGuMcWGSL(k), "moe_gu_mc_" + k); P["dn_" + k] = await pipe(MC.moeDnMcWGSL(k), "moe_dn_mc_" + k); }),
      ...Object.entries({ route: MC.MOE_ROUTE_MC_WGSL, hcSite: MC.HC_SITE_MC_WGSL, hcMix: MC.HC_MIX_MC_WGSL, hcComb: MC.HC_COMBINE_MC_WGSL,
        dnConv: MC.DN_CONV_MC_WGSL, dnDelta: MC.DN_DELTA_MC_WGSL, attnPrep: MC.attnPrepMcWGSL(eng.kv),
        attnDense: MC.attnDecMcWGSL({ kv: eng.kv }), attnGather: MC.attnDecMcWGSL({ gather: QSA_SEL_STRIDE, kv: eng.kv }),
        ple: MC.PLE_MC_WGSL, am1: MC.ARGMAX1_MC_WGSL, am2: MC.ARGMAX2_MC_WGSL,
        ...(exact ? {} : { moeSort: WX.moeSortWWGSL(), moeComb: WX.MOE_COMB_W_WGSL, guW_q4: WX.moeGuWWGSL("q4"), guW_q8: WX.moeGuWWGSL("q8"),
          dnW_q4: WX.moeDnWWGSL("q4"), dnW_q8: WX.moeDnWWGSL("q8"),
          dnDeltaW: WX.DN_DELTA_W_WGSL, dnGnormW: WX.DN_GNORM_W_WGSL, dnConvW: WX.DN_CONV_W_WGSL, dnConvSt: WX.DN_CONV_ST_W_WGSL,
          attnWD: WX.attnWWGSL({ G: nH / nKV, hd, kv: eng.kv }), attnWG: WX.attnWWGSL({ gather: QSA_SEL_STRIDE, G: nH / nKV, hd, kv: eng.kv }),
          ...(this.dpMoe ? { guD_q4: WX.moeGuCWGSL("q4"), guD_q8: WX.moeGuCWGSL("q8"), dnD_q4: WX.moeDnCWGSL("q4"), dnD_q8: WX.moeDnCWGSL("q8") } : {}) }) })
        .map(async ([k, code]) => { P[k] = await pipe(code, k); }),
    ]);
    const lazy = new Map();
    const gemvPipe = (key, code) => { if (!lazy.has(key)) lazy.set(key, pipe(code, key)); return lazy.get(key); };

    // ---- scratch: N columns of each of the decode engine's buffers
    const buf = (n) => device.createBuffer({ size: Math.max(16, Math.ceil(n * 4 / 16) * 16), usage: U.STORAGE | U.COPY_SRC | U.COPY_DST });
    const B = this.B = {
      R: buf(N * HD), xn: buf(N * HD), lo: buf(N * rank), gate: buf(N * HD), mixed: buf(N * d), out: buf(N * d),
      inj: [buf(N * hc * hc), buf(N * hc * hc)], xnTmp: buf(HD),
      qkvz: buf(N * qkvzS), ba: buf(N * 2 * ss.nVHeads), co: buf(N * convDim), o: buf(N * ss.inner), go: buf(N * ss.inner),
      qkvA: buf(N * qs), qn: buf(N * nH * hd), ao: buf(N * nH * hd),
      logits: buf(N * ls), ids: buf(N * K), wts: buf(N * K), h: buf(N * K * cfg.expFF), moe: buf(N * d),
      shIds: buf(4), hs: buf(N * cfg.shFF), sh: buf(N * d),
      F: device.createBuffer({ size: 16, usage: U.UNIFORM | U.COPY_DST }), cs: buf(N * MC.MC_FRAME_COL),
      // the head-on-the-last-column frame (pos + n - 1, 1 column), copied into F inside the pass's encoder (_run)
      F1: device.createBuffer({ size: 16, usage: U.COPY_SRC | U.COPY_DST }),
    };
    device.queue.writeBuffer(B.shIds, 0, new Uint32Array(4));
    const sortS = WX.moeSortWSizes({ maxCols: N, K, nExp: cfg.nExp });
    if (!exact) Object.assign(B, { hsg: buf(N * cfg.shFF), yK: buf(N * K * d), grp: buf(sortS.words), dummy: buf(4) });
    if (this.dp4a) Object.assign(B, { xq: buf(N * HD / 4), xd: buf(N * HD / 32), xqM: buf(N * d / 4), xdM: buf(N * d / 32),
      hq: buf(N * K * cfg.expFF / 4), hd: buf(N * K * cfg.expFF / 32) });
    if (eng.B.emb) Object.assign(B, { emb: buf(N * d), pk: buf(N * HD), pv: buf(N * d), dbg: buf(2 * HD + hc) });
    if (eng.qsa) this.qsaS = qsaScratch(device, { maxCols: N, maxBlocks: eng.qsaSh.maxBlocks });

    const uni = (fields) => {
      const n = Math.max(4, Math.ceil(fields.length / 4) * 4), ab = new ArrayBuffer(n * 4), dv = new DataView(ab);
      fields.forEach((v, i) => (typeof v === "number" ? dv.setUint32(i * 4, v, true) : dv.setFloat32(i * 4, v.f, true)));
      const b = device.createBuffer({ size: ab.byteLength, usage: U.UNIFORM | U.COPY_DST, mappedAtCreation: true });
      new Uint8Array(b.getMappedRange()).set(new Uint8Array(ab));
      b.unmap();
      return b;
    };
    const bg = (p, list) => device.createBindGroup({ layout: p.getBindGroupLayout(0),
      entries: list.map((b, i) => (b ? { binding: i, resource: { buffer: b } } : null)).filter(Boolean) });
    const f = (v) => ({ f: v });
    const cdiv = (a, b) => Math.ceil(a / b);
    const pending = [];   // ops whose pipeline is still compiling: { op, p (promise), list }
    const later = (pp, list, dyn, extra = {}) => { const op = { pipe: null, bg: null, dyn, ...extra }; pending.push({ op, pp, list }); return op; };

    // y (column stride ys) = mat x (column stride xs): the decode GEMV's lanes per row (gemvLanes of its shape)
    const gemv = (M, x, y, { dIn = M.shape?.[1], dOut = M.shape?.[0] ?? M.n / dIn, xs = dIn, ys = dOut, xOff = 0, yOff = 0, mode = 0, scale = 0, cut = false } = {}) => {
      if (!(dIn > 0 && dOut > 0 && Number.isInteger(dOut))) throw new Error(`qwen4exp batch: gemv shape ${dOut} x ${dIn}`);
      if (xs % 4 || xOff % 4) throw new Error("qwen4exp batch: gemv column stride / offset must be a multiple of 4");
      const lanes = MC.gemvLanes(M.kind, dOut, dIn), nwg = cdiv(dOut, 256 / lanes);
      const u = uni([dOut, dIn, 0, xOff / 4, yOff, mode, f(scale), xs / 4, ys, 0, 0, 0]);
      const list = M.kind === "f32" ? [M.buf, null, x, y, u, B.F] : [M.qs, M.sc, x, y, u, B.F];
      return later(gemvPipe(`gemv_mc_${M.kind}_${lanes}`, MC.gemvMcWGSL(M.kind, lanes)), list,
        (pos, n) => [Math.min(nwg, 32768), cdiv(nwg, 32768), cdiv(n, C)], cut ? { cut } : {});
    };
    const ewg = (n) => (pos, c) => [cdiv(n, 256), c, 1];

    // The wide GEMM (fast lists): y[col * ys + yOff + r] = epi(M x[col]) over the pass's columns -> [ops]. mode 1:
    // silu(acc * scale); mode 2: silu(aux) * acc. Quantized weights on dp4a (gemm_d, after a quant_x of x unless xq
    // gives x already quantized: { q, d }) or else f32 tiles (gemm_t, also every f32 weight). A shape with few output
    // tiles at maxCols columns splits K (workgroup z) into the part buffer, summed in order by split_sum.
    let partFloats = 0;
    const partOf = () => B.kpart ??= buf(partFloats);   // (B.part is the head's argmax partials)
    const quantX = (x, dIn, { xs = dIn, xOff = 0, mult = 1, q = B.xq, dq = B.xd } = {}) => later(gemvPipe("quant_x", WX.QUANT_X_WGSL),
      [x, q, dq, uni([dIn, xOff / 4, xs / 4, mult]), B.F], (pos, n) => [cdiv(dIn / 32, 32), n * mult, 1]);
    const gemmW = (M, x, y, { dIn = M.shape?.[1], dOut = M.shape?.[0] ?? M.n / dIn, xs = dIn, ys = dOut, xOff = 0, yOff = 0, mode = 0, scale = 0, aux = null, xq = null, f32 = false } = {}) => {
      if (!(dIn > 0 && dOut > 0 && Number.isInteger(dOut) && dIn % 32 === 0)) throw new Error(`qwen4exp batch: gemm shape ${dOut} x ${dIn}`);
      if (xs % 4 || xOff % 4) throw new Error("qwen4exp batch: gemm column stride / offset must be a multiple of 4");
      // gemm_d steps KB blocks per stage (no tail guard): a row of dIn / 32 blocks not a multiple of KB takes gemm_t
      const flt = M.kind === "f32" || M.kind === "bf16", nb = dIn / 32;
      const D = this.dpDense && !f32 && !flt && nb % WX.DP4A_T.KB === 0, T = D ? WX.DP4A_T : WX.WIDE_T, KB = D ? T.KB : 1;
      const tiles = cdiv(dOut, T.BM) * cdiv(N, T.BN);
      let kPer = nb;
      if (tiles < 160) kPer = Math.max(KB * 2, Math.ceil(Math.ceil(nb / Math.min(16, Math.round(320 / tiles))) / KB) * KB);
      const S = cdiv(nb, kPer), ops = [];
      if (S > 1) partFloats = Math.max(partFloats, S * N * dOut);
      const out = S > 1 ? null : y;   // null: the split-K buffer, bound once its size is known (B.kpart, partOf)
      const u = uni([dOut, dIn, xOff / 4, xs / 4, yOff, ys, mode, f(scale), kPer, S > 1 ? 1 : 0, 0, 0]);
      if (D && !xq) { ops.push(quantX(x, dIn, { xs, xOff })); xq = { q: B.xq, d: B.xd }; }
      const grid = (pos, n) => [cdiv(dOut, T.BM), cdiv(n, T.BN), S];
      ops.push(D ? later(gemvPipe(`gemm_d_${M.kind}`, WX.gemmDWGSL(M.kind)), [M.qs, M.sc, xq.q, xq.d, out, u, B.F, aux || B.dummy], grid)
        : later(gemvPipe(`gemm_t_${M.kind}`, WX.gemmTWGSL(M.kind)), flt ? [M.buf, null, x, out, u, B.F, aux || B.dummy] : [M.qs, M.sc, x, out, u, B.F, aux || B.dummy], grid));
      if (S > 1) {
        ops.push(later(gemvPipe("split_sum", WX.SPLIT_SUM_WGSL), [null, y, uni([dOut, dIn, 0, 0, yOff, ys, mode, f(scale), S, 0, 0, 0]), B.F, aux || B.dummy],
          (pos, n) => [cdiv(dOut, 256), n, 1]));
        ops.at(-2).partAt = D ? 4 : 3; ops.at(-1).partAt = 0;
      }
      return ops;
    };

    // The op lists, exact (the multi-column decode kernels) or fast (the wide kernels for the projections and the
    // routed / shared experts; every other kernel as exact). -> { layers: [{ i, st, off, ops }], headSite, headRest }
    const build = (fast) => {
      const proj = (...a) => (fast ? gemmW(...a) : [gemv(...a)]);
      // dp4a: the layer input (mixed) quantized once per half, after the HC mix, for every projection that reads it
      const XM = fast && this.dp4a ? { q: B.xqM, d: B.xdM } : null;
      // HC sites, as the engine's: site h's inject partials go to inj[h % 2]
      let nSite = 0;
      const site = (hw, inject = true) => ({ h: nSite++, w: wq(hw.norm, "hc norm").buf, injW: inject ? wq(hw.inject, "hc inject").buf : null });
      const siteOp = (S, comb) => {
        const out = B.inj[S.h % 2], prev = comb ? B.inj[comb.h % 2] : B.inj[(S.h + 1) % 2];
        const u = uni([d, hc, f(eps), comb ? (comb.b ? 2 : 1) : 0, S.injW ? 1 : 0, 0, 0, 0]);
        return { pipe: P.hcSite, bg: bg(P.hcSite, [B.R, comb ? comb.a : B.mixed, comb?.b || B.mixed, prev, S.w, B.xn, S.injW || S.w, out, u]), dyn: (pos, n) => [hc, n, 1], site: true };
      };
      const combOp = (Cb) => ({ pipe: P.hcComb, bg: bg(P.hcComb, [B.R, Cb.a, Cb.b || Cb.a, B.inj[Cb.h % 2], uni([d, hc, Cb.b ? 1 : 0, 0])]), dyn: ewg(HD) });
      const hcMixRest = (hw, wide = fast) => {
        const P2 = (...a) => (wide ? gemmW(...a) : [gemv(...a)]);
        // the HC projections stay on f32 tiles with dp4a too: the up projection's outputs are the streams' gates, and
        // with quantized inputs it alone moved the residual by ~1e-2 (p99 6e-2 -> 2e-2 on 4 layers); both are small
        const ops = [...P2(wq(hw.down, "hc down"), B.xn, B.lo, { dIn: HD, ys: rank, mode: 1, scale: 1 / hc, f32: true })];
        const up = wq(hw.up, "hc up");
        if (up.kind !== "f32" && !wide) {
          const lanes = MC.gemvLanes(up.kind, HD, rank), u = uni([HD, rank, 0, 0, 0, 0, f(0), d, rank / 4, HD, 0, 0]);
          ops.push(later(gemvPipe(`gemvmix_mc_${up.kind}_${lanes}`, MC.gemvMixMcWGSL(up.kind, lanes)), [up.qs, up.sc, B.lo, B.gate, u, B.xn, B.mixed],
            (pos, n) => [cdiv(d, 256 / lanes), 1, n]));
        } else {
          ops.push(...P2(up, B.lo, B.gate, { dIn: rank, ys: HD, f32: true }));
          ops.push({ pipe: P.hcMix, bg: bg(P.hcMix, [B.xn, B.gate, B.mixed, uni([d, hc, 0, 0])]), dyn: ewg(d) });
        }
        if (wide && XM) ops.push(quantX(B.mixed, d, { q: XM.q, dq: XM.d }));
        return ops;
      };
      const halfOps = (halves) => halves.map((H, k) => {
        const prev = k > 0 ? halves[k - 1] : null, next = halves[k + 1];
        const fold = prev && prev.comb && !H.pre.length;
        const list = [...H.pre, siteOp(H.site, fold ? prev.comb : null), ...H.rest];
        if (H.comb && !(next && !next.pre.length)) list.push(combOp(H.comb));
        return list;
      });

      // ---- per layer ops, on the engine's per-layer state and weights
      const halves = [], lmeta = [];
      for (const el of eng.layers) {
        const i = el.i, L = W.layers[i - eng.lo], st = el.st, w = st.w, pre = [], rest = [];
        if (w.pw) {   // PLE: key / value projections of the n-gram rows, then the injection over the columns in order
          const pc = L.ple, { K1, slots } = st.pleU;
          pre.push(...proj(wq(pc.key, "ple key"), B.emb, B.pk));
          pre.push(...proj(wq(pc.value, "ple value"), B.emb, B.pv));
          pre.push({ pipe: P.ple, bg: bg(P.ple, [B.R, B.pk, B.pv, w.pw, st.ring, B.dbg, uni([d, K1, cfg.ple.ngram, slots, f(eps), hc, 0, 0]), B.F]), dyn: () => [hc, 1, 1] });
        }
        const sA = site(L.hcAttn);
        rest.push(...hcMixRest(L.hcAttn));
        if (el.isQ) {
          if (w.qkvA) rest.push(...proj(w.qkvA, B.mixed, B.qkvA, { ys: qs, xq: XM }));
          else rest.push(...proj(wq(L.wq, "attn_q"), B.mixed, B.qkvA, { ys: qs, xq: XM }), ...proj(wq(L.wk, "attn_k"), B.mixed, B.qkvA, { ys: qs, yOff: nQ, xq: XM }),
            ...proj(wq(L.wv, "attn_v"), B.mixed, B.qkvA, { ys: qs, yOff: nQ + nK, xq: XM }));
          let sel = [];
          if (eng.qsa) {
            const G = qsaBind(device, eng.qsaK, eng.qsaSh, this.qsaS, st.qsa, B.mixed), so = qsaStateOps(eng.qsaK, G);
            // the indexer projections as a wide GEMM in the wide lists, on the layer's own BF16 weights (st.qsa.w,
            // [q ; k] rows as u32 pairs: the decode kernel's buffer, no second copy), widened to f32 in the kernel
            if (fast) so.splice(0, 1, ...gemmW({ kind: "bf16", buf: st.qsa.w }, B.mixed, this.qsaS.proj, { dIn: d, dOut: QSA_PROJ }));
            rest.push(...so);
            sel = qsaSelectOps(eng.qsaK, G, eng.sparse);
          }
          const ms = uni([qs, nQ, nQ + nK, 0]);
          rest.push({ pipe: P.attnPrep, bg: bg(P.attnPrep, [B.qkvA, B.qn, st.kc, st.vc, wq(L.qNorm).buf, wq(L.kNorm).buf, B.cs, st.ap, ms, B.F]),
            dyn: (pos, n) => [nH + nKV, n, 1] });
          rest.push(...sel);
          if (fast) {   // the tiled attention: a KV head's query heads (and in the dense pass, ATTN_W.dense.CW columns) per workgroup
            rest.push({ pipe: P.attnWD, bg: bg(P.attnWD, [B.qn, st.kc, st.vc, B.qkvA, B.ao, st.ap, B.F, ms]),
              dyn: (pos, n) => (eng.qsa && eng.sparse(pos, n) ? null : [nKV, cdiv(n, WX.ATTN_W.dense.CW), 1]) });
            if (eng.qsa) rest.push({ pipe: P.attnWG, bg: bg(P.attnWG, [B.qn, st.kc, st.vc, B.qkvA, B.ao, st.ap, B.F, ms, this.qsaS.sel]),
              dyn: (pos, n) => (eng.sparse(pos, n) ? [nKV, n, 1] : null) });
          } else {
            rest.push({ pipe: P.attnDense, bg: bg(P.attnDense, [B.qn, st.kc, st.vc, B.qkvA, B.ao, st.ap, B.F, ms]),
              dyn: (pos, n) => (eng.qsa && eng.sparse(pos, n) ? null : [nH, n, 1]) });
            if (eng.qsa) rest.push({ pipe: P.attnGather, bg: bg(P.attnGather, [B.qn, st.kc, st.vc, B.qkvA, B.ao, st.ap, null, ms, this.qsaS.sel]),
              dyn: (pos, n) => (eng.sparse(pos, n) ? [nH, n, 1] : null) });
          }
          rest.push(...proj(wq(L.wo, "attn_output"), B.ao, B.out));
        } else {
          if (w.qkvz) rest.push(...proj(w.qkvz, B.mixed, B.qkvz, { ys: qkvzS, xq: XM }));
          else rest.push(...proj(wq(L.wqkv, "qkv"), B.mixed, B.qkvz, { ys: qkvzS, xq: XM }), ...proj(wq(L.wz, "z"), B.mixed, B.qkvz, { ys: qkvzS, yOff: convDim, xq: XM }));
          rest.push(...proj(w.ba, B.mixed, B.ba, { dIn: d, xq: XM }));
          const cv = wq(L.conv, "conv").buf, nCH = convDim / ss.stateSize;
          if (fast && ss.convK <= 9) {   // every (head, column) at once (the conv reads only inputs), then the state
            rest.push({ pipe: P.dnConvW, bg: bg(P.dnConvW, [B.qkvz, st.conv, cv, B.co, uni([ss.convK, 2 * ss.nKHeads, ss.stateSize, f(eps), qkvzS, convDim, 0, 0])]),
              dyn: (pos, n) => [nCH, n, 1] });
            rest.push({ pipe: P.dnConvSt, bg: bg(P.dnConvSt, [B.qkvz, st.conv, null, null, uni([ss.convK, 2 * ss.nKHeads, ss.stateSize, f(eps), qkvzS, convDim, nCH, 0]), B.F]),
              dyn: () => [cdiv(convDim, 256), 1, 1] });
          } else rest.push({ pipe: P.dnConv, bg: bg(P.dnConv, [B.qkvz, st.conv, cv, B.co, uni([ss.convK, 2 * ss.nKHeads, ss.stateSize, f(eps), qkvzS, convDim, 0, 0]), B.F]),
            dyn: () => [nCH, 1, 1] });
          const ep = uni([ss.nKHeads, ss.stateSize, keyDim, ss.nVHeads, f(eps), convDim, 2 * ss.nVHeads, qkvzS, convDim, ss.inner, 0, 0]);
          if (fast) {   // the recurrence, then the gated norm over every (head, column) in parallel: the exact kernel's bits
            rest.push({ pipe: P.dnDeltaW, bg: bg(P.dnDeltaW, [B.co, st.S, B.ba, w.dta, B.o, ep, B.F]), dyn: () => [ss.nVHeads, 1, 1] });
            rest.push({ pipe: P.dnGnormW, bg: bg(P.dnGnormW, [B.o, B.qkvz, wq(L.ssmNorm).buf, B.go, ep]), dyn: (pos, n) => [ss.nVHeads, n, 1] });
          } else rest.push({ pipe: P.dnDelta, bg: bg(P.dnDelta, [B.co, st.S, B.ba, w.dta, B.qkvz, wq(L.ssmNorm).buf, B.o, B.go, ep, B.F]), dyn: () => [ss.nVHeads, 1, 1] });
          rest.push(...proj(wq(L.wOut, "ssm_out"), B.go, B.out));
        }
        const combA = { a: B.out, b: null, h: sA.h };
        // MoE: router + shared gate (one projection), top-K per column, routed experts, shared expert
        const sF = site(L.hcFfn), ffn = [...hcMixRest(L.hcFfn)];
        ffn.push(...proj(w.router, B.mixed, B.logits, { dIn: d, xq: XM }));
        ffn.push({ pipe: P.route, bg: bg(P.route, [B.logits, B.ids, B.wts, uni([cfg.nExp, K, ls, 0])]), dyn: (pos, n) => [n, 1, 1], cut: true });
        // the routed experts: resident, or (offloaded) the store's region, filled after the cut
        const off = offSet.has(i), ex = (k, part) => (off ? eng.experts.regionEntry(i, part) : wq(L[k], "experts " + part));
        const eg = ex("expGate", "gate"), eu = ex("expUp", "up"), ed = ex("expDown", "down"), ff = cfg.expFF;
        const sgw = wq(L.shGate, "shared gate"), suw = wq(L.shUp, "shared up"), sdw = wq(L.shDown, "shared down"), sff = cfg.shFF;
        let combF;
        if (fast) {
          // expert-grouped: sort the (column, slot) pairs by expert, gate/up and down per tile of one expert's pairs,
          // the shared expert as GEMMs, then the weighted sum (+ the gated shared expert) per column
          const maxTiles = (n) => Math.ceil(n * K / WX.MOE_BP) + Math.min(cfg.nExp, n * K);
          ffn.push({ pipe: P.moeSort, bg: bg(P.moeSort, [B.ids, B.grp, uni([cfg.nExp, K, sortS.PO, 0]), B.F]), dyn: () => [1, 1, 1] });
          const RW = this.dpMoe ? WX.MOE_RC : WX.MOE_RW;
          const guG = (pos, n) => [cdiv(ff, RW), maxTiles(n), 1], dnG = (pos, n) => [cdiv(d, RW), maxTiles(n), 1];
          if (this.dpMoe) {   // on the quantized layer input (XM); h quantized per pair for the down projection
            ffn.push({ pipe: P["guD_" + eg.kind], bg: bg(P["guD_" + eg.kind], [eg.qs, eg.sc, eu.qs, eu.sc, XM.q, XM.d, B.h, B.grp, uni([ff, d, K, sortS.PO, ff, 0, 0, 0])]), dyn: guG });
            ffn.push(quantX(B.h, ff, { mult: K, q: B.hq, dq: B.hd }));
            ffn.push({ pipe: P["dnD_" + ed.kind], bg: bg(P["dnD_" + ed.kind], [ed.qs, ed.sc, B.hq, B.hd, B.yK, B.grp, uni([d, ff, K, sortS.PO, 0, 0, 0, 0])]), dyn: dnG });
          } else {
            ffn.push({ pipe: P["guW_" + eg.kind], bg: bg(P["guW_" + eg.kind], [eg.qs, eg.sc, eu.qs, eu.sc, B.mixed, B.h, B.grp, uni([ff, d, K, sortS.PO, d / 4, ff, 0, 0])]), dyn: guG });
            ffn.push({ pipe: P["dnW_" + ed.kind], bg: bg(P["dnW_" + ed.kind], [ed.qs, ed.sc, B.h, B.yK, B.grp, uni([d, ff, K, sortS.PO, 0, 0, 0, 0])]), dyn: dnG });
          }
          ffn.push(...gemmW(sgw, B.mixed, B.hsg, { dIn: d, dOut: sff, xq: XM }));
          ffn.push(...gemmW(suw, B.mixed, B.hs, { dIn: d, dOut: sff, mode: 2, aux: B.hsg, xq: XM }));
          ffn.push(...gemmW(sdw, B.hs, B.sh, { dIn: sff, dOut: d }));
          ffn.push({ pipe: P.moeComb, bg: bg(P.moeComb, [B.yK, B.wts, B.sh, B.logits, B.moe, uni([d, K, ls, cfg.nExp])]), dyn: (pos, n) => [cdiv(d, 256), n, 1] });
          combF = { a: B.moe, b: null, h: sF.h };
        } else {
          ffn.push({ pipe: P["gu_" + eg.kind], bg: bg(P["gu_" + eg.kind], [eg.qs, eg.sc, eu.qs, eu.sc, B.mixed, B.h, B.ids, uni([ff, d, K, d / 4, K * ff, K, 0, 0])]),
            dyn: (pos, n) => [cdiv(ff, 8), K, n] });
          ffn.push({ pipe: P["dn_" + ed.kind], bg: bg(P["dn_" + ed.kind], [ed.qs, ed.sc, B.h, B.moe, B.ids, B.wts, uni([d, ff, K, 0, 0, K, K, K * ff / 4])]),
            dyn: (pos, n) => [cdiv(d, 8), 1, n] });
          ffn.push({ pipe: P["gu_" + sgw.kind], bg: bg(P["gu_" + sgw.kind], [sgw.qs, sgw.sc, suw.qs, suw.sc, B.mixed, B.hs, B.shIds, uni([sff, d, 1, d / 4, sff, 0, 0, 0])]),
            dyn: (pos, n) => [cdiv(sff, 8), 1, n] });
          ffn.push({ pipe: P["dn_" + sdw.kind], bg: bg(P["dn_" + sdw.kind], [sdw.qs, sdw.sc, B.hs, B.sh, B.shIds, B.logits, uni([d, sff, 1, 1, cfg.nExp, ls, 0, sff / 4])]),
            dyn: (pos, n) => [cdiv(d, 8), 1, n] });
          combF = { a: B.moe, b: B.sh, h: sF.h };
        }
        halves.push({ pre, site: sA, rest, comb: combA }, { pre: [], site: sF, rest: ffn, comb: combF });
        lmeta.push({ i, st, off });
      }

      // ---- head: the head mixer's site (which folds the last layer's combine) on every column, then the mixer, the
      // LM head and a greedy id per column ("all": verify), or on the last column only ("last": prefill). The mixer and
      // head stay on the exact kernels in both lists (one column, or a verify pass)
      if (eng.hasHead) {
        const V = eng.dims.vocab, per = 1024, nPart = cdiv(V, per);
        halves.push({ pre: [], site: site(W.headHC, false), comb: null, rest: [...hcMixRest(W.headHC, false),
          gemv(wq(W.head, "output"), B.mixed, B.hl, { dIn: d }),
          { pipe: P.am1, bg: bg(P.am1, [B.hl, B.part, uni([V, per, nPart, 0])]), dyn: (pos, n) => [nPart, n, 1] },
          { pipe: P.am2, bg: bg(P.am2, [B.part, B.am, uni([V, per, nPart, 0])]), dyn: (pos, n) => [n, 1, 1] }] });
      }
      return () => {   // once the pipelines are in: the lists, with every layer's split after its router
        const lists = halfOps(halves);
        const out = { layers: lmeta.map((m, k) => ({ ...m, ops: [...lists[2 * k], ...lists[2 * k + 1]] })) };
        if (eng.hasHead) { const h = lists[lists.length - 1]; out.headSite = h.slice(0, 1); out.headRest = h.slice(1); }
        for (const l of out.layers) if (l.off) { const at = l.ops.findIndex((o) => o.cut) + 1; l.route = l.ops.slice(0, at); l.exp = l.ops.slice(at); }
        return out;
      };
    };
    if (eng.hasHead) {
      const V = eng.dims.vocab, nPart = cdiv(V, 1024), NV = this.verifyCols;
      Object.assign(B, { hl: buf(NV * V), part: buf(NV * nPart * 2), am: buf(NV * 2) });
      this.amStage = device.createBuffer({ size: NV * 8, usage: U.MAP_READ | U.COPY_DST });
      this.logitStage = device.createBuffer({ size: Math.ceil(V * 4 / 16) * 16, usage: U.MAP_READ | U.COPY_DST });
    }
    const exactL = build(false), fastL = exact ? null : build(true);
    await Promise.all(pending.map(async ({ op, pp, list }) => {
      op.pipe = await pp;
      if (op.partAt !== undefined) { list = list.slice(); list[op.partAt] = partOf(); }
      op.bg = bg(op.pipe, list);
    }));
    this.L = { exact: exactL(), fast: fastL ? fastL() : null };
    if (offSet.size) this.idStage = device.createBuffer({ size: N * K * 4, usage: U.MAP_READ | U.COPY_DST });
    this.hStage = device.createBuffer({ size: HD * 4, usage: U.MAP_READ | U.COPY_DST });
    this._stages = new Map();
    this._snap = null;
  }

  // whether a pass of n columns (head: as _run) runs the wide kernels
  fastPass(n, head = "last") { return !this.exact && head !== "all" && n >= WX.FAST_MIN_COLS; }

  _stage(bytes) {
    let s = this._stages.get(bytes);
    if (!s) { s = this.device.createBuffer({ size: bytes, usage: U.MAP_READ | U.COPY_DST }); this._stages.set(bytes, s); }
    return s;
  }
  async _read(s, bytes, Type = Float32Array) {
    await s.mapAsync(GPUMapMode.READ, 0, bytes);
    const a = new Type(s.getMappedRange(0, bytes).slice(0));
    s.unmap();
    return a;
  }

  // One pass of n <= maxCols tokens at the engine's position. head: "last" (the head on the last column, or its
  // hidden when the engine has no head), "all" (a greedy id per column; n <= verifyCols), "none".
  // hidden: the input residual (n x hDim) when the engine has no embedding (a later room stage).
  // -> { id, logits } | { hidden } | { ids }, plus hiddens (n x hDim, the output residual) when wantHidden.
  async _pass(ids, opts) {
    const eng = this.eng;
    eng._check(ids[0]);
    eng._posGen++;   // a verify snapshot taken before this pass is stale from here (accept checks it)
    try { return await this._run(ids, opts); } catch (e) {
      eng._failed = e;
      // a region load encoded in a pass that was never submitted (or only partly) must not count as loaded
      if (eng.experts?.region) eng.experts.region.layer = -1;
      throw e;
    }
  }

  // exact: this pass on the exact kernels whatever its width. taps: also return taps (one Float32Array of n x hDim per
  // layer: the residual after that layer, read where the next layer's first HC site has folded in its last combine,
  // so the PLE layer's injection shows in the layer before it; the last one after the final combine) for the
  // per-layer comparison of the wide kernels against the exact ones (tests/test_q38flash_batch.js).
  async _run(ids, { head = "last", wantLogits = false, wantHidden = false, hidden = null, exact = false, taps = false } = {}) {
    const { eng, device, cfg, B } = this, q = device.queue, n = ids.length, pos = eng.pos, HD = cfg.hDim, d = cfg.dim;
    if (!(n >= 1 && n <= this.maxCols)) throw new Error(`qwen4exp batch: ${n} columns (1 .. ${this.maxCols})`);
    if (pos + n > eng.maxSeq) throw new Error(`qwen4exp: context full (${eng.maxSeq})`);
    if (eng.hasEmbed && !ids.every((id) => Number.isInteger(id) && id >= 0 && id < eng.nVocab)) throw new Error("qwen4exp batch: token id outside the vocab");
    if (head === "all" && !eng.hasHead) head = "none";
    if (head === "all" && n > this.verifyCols) throw new Error(`qwen4exp batch: a verify pass takes at most ${this.verifyCols} columns`);
    if (!eng.hasEmbed && !(hidden && hidden.length === n * HD)) throw new Error("qwen4exp batch: this stage needs the input residual (hidden, n x hDim)");
    // the residual
    if (eng.hasEmbed) {
      const R = new Float32Array(n * HD);
      for (let c = 0; c < n; c++) { const e = eng._embedRow(ids[c]); for (let s = 0; s < cfg.hc; s++) R.set(e, c * HD + s * d); }
      q.writeBuffer(B.R, 0, R);
    } else q.writeBuffer(B.R, 0, hidden);
    // the n-gram rows (the PLE layer's input); the history either way
    if (B.emb) q.writeBuffer(B.emb, 0, await eng.ngram.rows(eng.ngs.push(ids)));
    else eng.ngs?.push(ids);
    // the frame: pos and n, and every column's rope table (the decode frame's recurrence)
    q.writeBuffer(B.F, 0, new Uint32Array([pos, n, 0, 0]));
    if (head === "last" && eng.hasHead) q.writeBuffer(B.F1, 0, new Uint32Array([pos + n - 1, 1, 0, 0]));
    const cs = new Float32Array(n * MC.MC_FRAME_COL);
    for (let c = 0; c < n; c++) {
      let th = fr(pos + c);
      for (let i = 0; i < cfg.ropeDims / 2; i++) { cs[c * MC.MC_FRAME_COL + i * 4] = fr(Math.cos(th)); cs[c * MC.MC_FRAME_COL + i * 4 + 1] = fr(Math.sin(th)); th = fr(th * eng._ropeTs); }
    }
    q.writeBuffer(B.cs, 0, cs);
    if (eng.qsa) qsaSetFrame(device, this.qsaS, pos, n);

    // the wide kernels for a prefill pass of at least FAST_MIN_COLS columns (not with exact, not for verify)
    const Ls = !exact && this.fastPass(n, head) ? this.L.fast : this.L.exact;
    const tapBufs = [];
    this._tap = taps ? (enc) => {
      const b = device.createBuffer({ size: n * HD * 4, usage: U.MAP_READ | U.COPY_DST });
      enc.copyBufferToBuffer(B.R, 0, b, 0, n * HD * 4);
      tapBufs.push(b);
    } : null;
    let at = 0;
    for (; at + SUBMIT_LAYERS < Ls.layers.length; at += SUBMIT_LAYERS) {
      const r = await this._encodeLayers(Ls.layers.slice(at, at + SUBMIT_LAYERS), pos, n);
      r.pass.end();
      q.submit([r.enc.finish()]);
    }
    let { enc, pass } = await this._encodeLayers(Ls.layers.slice(at), pos, n);
    if (eng.hasHead) eng._encode(pass, Ls.headSite, pos, n);   // the last layer's combine, on every column
    if (this._tap) { pass.end(); this._tap(enc); pass = enc.beginComputePass(); }   // the last layer's output (its combine: the head site's, or its own last op)
    if (head === "all") eng._encode(pass, Ls.headRest, pos, n);
    pass.end();
    const out = {}, HB = HD * 4;
    if (head === "last") {
      if (eng.hasHead) {   // the head on the last column: its normed residual into column 0, then one column
        if (n > 1) { enc.copyBufferToBuffer(B.xn, (n - 1) * HB, B.xnTmp, 0, HB); enc.copyBufferToBuffer(B.xnTmp, 0, B.xn, 0, HB); }
        // and the frame's nCols to 1: gemv_mc guards its GEMV_MC_COLS columns with nCols, so with the pass's n it
        // would compute the head on columns 0..3 and write them into B.hl, which holds only verifyCols columns (an
        // encoded copy, not a writeBuffer: that would land before the layers above run)
        enc.copyBufferToBuffer(B.F1, 0, B.F, 0, 16);
        pass = enc.beginComputePass();
        eng._encode(pass, Ls.headRest, pos + n - 1, 1);
        pass.end();
        enc.copyBufferToBuffer(B.am, 0, this.amStage, 0, 8);
        if (wantLogits) enc.copyBufferToBuffer(B.hl, 0, this.logitStage, 0, eng.dims.vocab * 4);
      } else enc.copyBufferToBuffer(B.R, (n - 1) * HB, this.hStage, 0, HB);
    }
    if (head === "all") enc.copyBufferToBuffer(B.am, 0, this.amStage, 0, n * 8);
    let hs = null;
    if (wantHidden) { hs = this._stage(this.maxCols * HB); enc.copyBufferToBuffer(B.R, 0, hs, 0, n * HB); }
    q.submit([enc.finish()]);
    eng.pos += n;
    if (head === "last") {
      if (eng.hasHead) {
        const a = await this._read(this.amStage, 8, Uint32Array);
        out.id = a[0];
        if (out.id >= eng.dims.vocab) throw new Error(`qwen4exp: no greedy id at position ${eng.pos - 1} (non-finite logits)`);
        if (wantLogits) out.logits = await this._read(this.logitStage, eng.dims.vocab * 4);
      } else out.hidden = await this._read(this.hStage, HB);
    }
    if (head === "all") { const a = await this._read(this.amStage, n * 8, Uint32Array); out.ids = Array.from({ length: n }, (_, c) => a[2 * c]); }
    if (wantHidden) out.hiddens = await this._read(hs, n * HB);
    if (head === "none" && !wantHidden) await q.onSubmittedWorkDone();
    if (taps) { out.taps = []; for (const b of tapBufs) { out.taps.push(await this._read(b, n * HD * 4)); b.destroy(); } }
    this._tap = null;
    return out;
  }

  // layers ls into a new encoder -> { enc, pass } (open). An offloaded layer cuts after its router: the experts it
  // needs go into the store's region (encoded copies; a narrow pass reads its ids back first), then its expert ops.
  async _encodeLayers(ls, pos, n) {
    const { eng, device, cfg } = this, K = cfg.K, st = eng.experts;
    let enc = device.createCommandEncoder(), pass = enc.beginComputePass();
    // with a tap: the residual of the layer before l, once l's first HC site has folded in its combine
    const tapped = (ops) => {
      if (!this._tap) return ops;
      const at = ops.findIndex((o) => o.site) + 1;
      eng._encode(pass, ops.slice(0, at), pos, n);
      pass.end(); this._tap(enc); pass = enc.beginComputePass();
      return ops.slice(at);
    };
    for (const l of ls) {
      const first = l.i === eng.lo;
      // (the ops first: tapped() replaces the pass)
      const ops = first ? (l.off ? l.route : l.ops) : tapped(l.off ? l.route : l.ops);
      eng._encode(pass, ops, pos, n);
      if (!l.off) continue;
      pass.end();
      // a disk layer's experts are read after a submit of everything encoded so far (it may still use the region):
      // the store's writes then land in queue order, before the next encoder's kernels
      const disk = !!st.isDisk?.(l.i);
      if (2 * n * K >= cfg.nExp) {
        if (!disk) st.loadLayer(enc, l.i);
        else if (st.region.layer !== l.i) {
          device.queue.submit([enc.finish()]);
          await st.loadLayerAsync(l.i);
          enc = device.createCommandEncoder();
        }
      } else {
        enc.copyBufferToBuffer(this.B.ids, 0, this.idStage, 0, n * K * 4);
        device.queue.submit([enc.finish()]);
        await this.idStage.mapAsync(GPUMapMode.READ, 0, n * K * 4);
        const ids = new Uint32Array(this.idStage.getMappedRange(0, n * K * 4)), sel = new Uint32Array(n * (K + 1));
        for (let c = 0; c < n; c++) { sel.set(ids.subarray(c * K, (c + 1) * K), c * (K + 1)); sel[c * (K + 1) + K] = cfg.nExp; }
        this.idStage.unmap();
        if (disk) await st.loadExpertsAsync(l.i, sel, n, K + 1);
        enc = device.createCommandEncoder();
        if (!disk) st.loadExperts(enc, l.i, sel, n, K + 1);
      }
      pass = enc.beginComputePass();
      eng._encode(pass, l.exp, pos, n);
    }
    return { enc, pass };
  }

  // Layer by layer (an engine with disk-tier layers, or a shared region): the columns of up to LAYERED_COLS tokens go through one layer
  // pass by pass before the next layer, so each disk layer's experts are read from its file once for all of them
  // (loadLayerAsync), not once per pass. The same passes (maxCols columns each, the wide or exact kernels as _run
  // picks) in the same order within every layer, so the bits are _passes's: a layer's state (DeltaNet, K / V, QSA)
  // sees its columns in order; each pass's residual waits in a buffer between layers. Head "none" only (a room's
  // prompt frames and a solo prompt: room/flash.js _passes). hidden: the input residuals (n x hDim) on a device
  // without the embedding; wantHidden: -> { hiddens } (n x hDim), the output residuals.
  // Also with the region borrowed from the cache (ExpertStore shared: a room node whose cache is about a region's size):
  // its layers in RAM copy into the region once for the group, not once a pass.
  layered(n) { const st = this.eng.experts; return !!(st?.diskSet?.size || st?.region?.shared) && n > this.maxCols && this.layeredCols > this.maxCols; }
  async passesLayered(ids, { hidden = null, wantHidden = false } = {}) {
    const eng = this.eng;
    eng._check(ids[0]);
    eng._posGen++;
    try { return await this._layered(ids, hidden, wantHidden); } catch (e) {
      eng._failed = e;
      if (eng.experts?.region) eng.experts.region.layer = -1;
      throw e;
    } finally { this._lmBuf?.destroy(); this._lmBuf = null; }
  }
  async _layered(ids, hidden, wantHidden) {
    const { eng, device, cfg, B } = this, q = device.queue, HD = cfg.hDim, HB = HD * 4, d = cfg.dim, N = ids.length;
    if (eng.pos + N > eng.maxSeq) throw new Error(`qwen4exp: context full (${eng.maxSeq})`);
    if (eng.hasEmbed && !ids.every((id) => Number.isInteger(id) && id >= 0 && id < eng.nVocab)) throw new Error("qwen4exp batch: token id outside the vocab");
    if (!eng.hasEmbed && !(hidden && hidden.length === N * HD)) throw new Error("qwen4exp batch: this stage needs the input residual (hidden, n x hDim)");
    const out = wantHidden ? new Float32Array(N * HD) : null, span = Math.max(this.maxCols, Math.floor(this.layeredCols / this.maxCols) * this.maxCols);
    // what a pass carries from one layer to the next (per column): the residual, and the last FFN half's combine,
    // which the next layer's first HC site folds in (build(): comb { a: moe, b: sh }, its inject partials in inj)
    const carry = [[B.R, HD], [B.moe, d], [B.sh, d], [B.inj[0], cfg.hc * cfg.hc], [B.inj[1], cfg.hc * cfg.hc]];
    const colB = carry.reduce((a, [, w]) => a + w * 4, 0);
    let yieldAt = performance.now() + YIELD_MS;
    for (let g0 = 0; g0 < N; g0 += span) {
      const gN = Math.min(span, N - g0), pos0 = eng.pos;
      if (!this._lmBuf || this._lmBuf.size < gN * colB) { this._lmBuf?.destroy(); this._lmBuf = device.createBuffer({ size: gN * colB, usage: U.STORAGE | U.COPY_SRC | U.COPY_DST }); }
      const Rg = this._lmBuf;
      // the group's columns of carry[k] at base[k] (the residuals first, at 0)
      const base = []; carry.reduce((a, [, w]) => { base.push(a); return a + gN * w * 4; }, 0);
      const move = (enc, p, save) => carry.forEach(([b, w], k) => (save ? enc.copyBufferToBuffer(b, 0, Rg, base[k] + p.o * w * 4, p.n * w * 4)
        : enc.copyBufferToBuffer(Rg, base[k] + p.o * w * 4, b, 0, p.n * w * 4)));
      // the passes: their columns, frame, rope table and n-gram rows (the history pushed in order, as _run does)
      const P = [];
      for (let o = 0; o < gN; o += this.maxCols) {
        const n = Math.min(this.maxCols, gN - o), pid = ids.slice(g0 + o, g0 + o + n), pos = pos0 + o;
        const cs = new Float32Array(n * MC.MC_FRAME_COL);
        for (let c = 0; c < n; c++) {
          let th = fr(pos + c);
          for (let i = 0; i < cfg.ropeDims / 2; i++) { cs[c * MC.MC_FRAME_COL + i * 4] = fr(Math.cos(th)); cs[c * MC.MC_FRAME_COL + i * 4 + 1] = fr(Math.sin(th)); th = fr(th * eng._ropeTs); }
        }
        const emb = B.emb ? await eng.ngram.rows(eng.ngs.push(pid)) : (eng.ngs?.push(pid), null);
        P.push({ o, n, pos, cs, emb, Ls: this.fastPass(n, "none") ? this.L.fast : this.L.exact });
      }
      // the input residuals of every column
      if (eng.hasEmbed) {
        const R = new Float32Array(gN * HD);
        for (let c = 0; c < gN; c++) { const e = eng._embedRow(ids[g0 + c]); for (let s = 0; s < cfg.hc; s++) R.set(e, c * HD + s * d); }
        q.writeBuffer(Rg, 0, R);
      } else q.writeBuffer(Rg, 0, hidden.subarray(g0 * HD, (g0 + gN) * HD));
      const nL = P[0].Ls.layers.length;
      for (let li = 0; li < nL; li++) {
        for (const p of P) {
          // this pass's frame (queue order: after the last pass's submit, before this one's)
          q.writeBuffer(B.F, 0, new Uint32Array([p.pos, p.n, 0, 0]));
          q.writeBuffer(B.cs, 0, p.cs);
          if (eng.qsa) qsaSetFrame(device, this.qsaS, p.pos, p.n);
          if (p.emb) q.writeBuffer(B.emb, 0, p.emb);
          // (the first layer: only the residuals; the rest is what the last pass left, as pass after pass)
          const e0 = device.createCommandEncoder();
          if (li) move(e0, p, false);
          else e0.copyBufferToBuffer(Rg, p.o * HB, B.R, 0, p.n * HB);
          q.submit([e0.finish()]);
          // (a disk layer: the first pass reads it into the region, the others find it there)
          const { enc, pass } = await this._encodeLayers([p.Ls.layers[li]], p.pos, p.n);
          if (li === nL - 1 && eng.hasHead) eng._encode(pass, p.Ls.headSite, p.pos, p.n);
          pass.end();
          move(enc, p, true);
          q.submit([enc.finish()]);
          if (performance.now() >= yieldAt) { await yieldLoop(); yieldAt = performance.now() + YIELD_MS; }
        }
      }
      eng.pos = pos0 + gN;
      if (wantHidden) {
        const st = device.createBuffer({ size: gN * HB, usage: U.MAP_READ | U.COPY_DST }), e = device.createCommandEncoder();
        e.copyBufferToBuffer(Rg, 0, st, 0, gN * HB);
        q.submit([e.finish()]);
        try { out.set(await this._read(st, gN * HB), g0 * HD); } finally { st.destroy(); }
      }
      else await q.onSubmittedWorkDone();
    }
    return { hiddens: out };
  }

  // Prompt tokens in passes of maxCols (the last pass gets the head). Returns the last pass's result (as _pass).
  // opts.chunks: explicit pass sizes (tests), repeated until the ids run out; opts.onPass(result, offset, size).
  async prefill(ids, { chunks = null, onPass = null, ...opts } = {}) {
    this._pending = null;   // a verify before this prefill can no longer be accepted
    let r = null, o = 0, k = 0;
    while (o < ids.length) {
      const sz = Math.min(chunks ? chunks[k++ % chunks.length] : this.maxCols, this.maxCols, ids.length - o);
      const end = o + sz >= ids.length;
      const res = await this._pass(ids.slice(o, o + sz), end ? opts : { ...opts, head: "none", hidden: opts.hidden?.subarray(o * this.cfg.hDim, (o + sz) * this.cfg.hDim) });
      if (onPass) onPass(res, o, sz);
      r = res; o += sz;
    }
    return r;
  }

  // A verify pass: ids = [the last accepted token, ...draft]; -> { ids: greedy id per column } (head), and hiddens
  // when wantHidden. The state before the pass is kept for accept().
  async verify(ids, { wantHidden = false } = {}) {
    this._pending = null;
    this._save();
    const pos0 = this.eng.pos;
    // exact: true, not only head "all": on an engine without the head (a later room stage) _run turns "all" into
    // "none", which alone would send a verify of FAST_MIN_COLS+ columns to the wide kernels
    const r = await this._pass(ids, { head: "all", wantHidden, exact: true });
    this._pending = { ids: ids.slice(), pos0, gen: this.eng._posGen };
    return r;
  }

  // Keep the first n columns of the last verify pass (n = its length: nothing to do). Only right after that pass:
  // any step that moved the engine since (forwardToken, debugToken, a prefill or another pass, reset()) bumps its
  // _posGen, and the snapshot (the state before the verify) would then roll back over that step, so accept refuses.
  async accept(n) {
    const v = this._pending;
    if (!v) throw new Error("qwen4exp batch: accept without a verify pass");
    this._pending = null;
    if (v.gen !== this.eng._posGen || this.eng.pos !== v.pos0 + v.ids.length)
      throw new Error("qwen4exp batch: accept after the engine moved past its verify pass (forwardToken, prefill or reset since); verify again");
    if (!(Number.isInteger(n) && n >= 0 && n <= v.ids.length)) throw new Error(`qwen4exp batch: accept(${n}) of ${v.ids.length}`);
    if (n === v.ids.length) return;
    this._restore();
    this.eng.pos = v.pos0;
    this.eng.ngs?.rollback(v.ids.length);
    // on the exact kernels whatever n: the state kept must be the one the verify pass computed and scored (a replay of
    // FAST_MIN_COLS+ columns would otherwise take the wide kernels and sum in another order)
    if (n) await this._pass(v.ids.slice(0, n), { head: "none", exact: true });
  }

  // the recurrent state (DeltaNet conv + S, the PLE ring), copied on the GPU
  _bufs() { return this.eng.layers.flatMap((l) => [l.st.conv, l.st.S, l.st.ring].filter(Boolean)); }
  _save() {
    const src = this._bufs();
    if (!this._snap) this._snap = src.map((b) => this.device.createBuffer({ size: b.size, usage: U.COPY_DST | U.COPY_SRC }));
    const enc = this.device.createCommandEncoder();
    src.forEach((b, k) => enc.copyBufferToBuffer(b, 0, this._snap[k], 0, b.size));
    this.device.queue.submit([enc.finish()]);
  }
  _restore() {
    const enc = this.device.createCommandEncoder();
    this._bufs().forEach((b, k) => enc.copyBufferToBuffer(this._snap[k], 0, b, 0, b.size));
    this.device.queue.submit([enc.finish()]);
  }
}
